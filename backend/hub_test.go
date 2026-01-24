// Copyright (c) 2026 TTBT Enterprises LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/gorilla/websocket"
)

func makeUUID(i int) string {
	return fmt.Sprintf("%08x-0000-0000-0000-000000000000", i)
}

func TestHubConcurrency(t *testing.T) {
	// Setup Server
	tempDir, err := os.MkdirTemp("", "hub_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)

	_, handler := NewServerHandler(Options{
		GameStore:      gStore,
		TeamStore:      tStore,
		Storage:        s,
		Registry:       reg,
		UserIndexStore: us,
		UseMockAuth:    true,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	userId := "concurrent@example.com"
	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Add("Cookie", "mock_auth_user="+userId)

	getWSURL := func(gId string) string {
		u, _ := url.Parse(server.URL)
		u.Scheme = "ws"
		u.Path = "/api/ws"
		if gId != "" {
			q := u.Query()
			q.Set("gameId", gId)
			u.RawQuery = q.Encode()
		}
		return u.String()
	}

	gameId := "10000000-0000-4000-8000-000000000001"

	// Bootstrap
	bootstrap := func() {
		// Use HTTP for bootstrap action
		actionId := makeUUID(1)
		action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":123,"type":"GAME_START","payload":{"id":"%s","date":"2025-12-25T12:00:00Z","away":"A","home":"B","ownerId":"%s"}}`, actionId, gameId, userId))
		msg := Message{Type: MsgTypeAction, Action: action, GameId: gameId}
		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("Bootstrap failed: %v", err)
		}
		resp.Body.Close()
	}
	bootstrap()

	// Concurrency Test
	// Spawn N clients, each sending M actions sequentially.
	// Hub serialization ensures they are processed one by one.
	clientCount := 5
	actionsPerClient := 10
	var wg sync.WaitGroup
	wg.Add(clientCount)

	errors := make(chan error, clientCount*actionsPerClient)

	for i := 0; i < clientCount; i++ {
		go func(clientId int) {
			defer wg.Done()
			conn, _, err := dialer.Dial(getWSURL(gameId), header)
			if err != nil {
				errors <- fmt.Errorf("Client %d dial failed: %v", clientId, err)
				return
			}
			defer conn.Close()

			wsMsgs := make(chan Message, 100)
			go func() {
				defer close(wsMsgs)
				for {
					var m Message
					if err := conn.ReadJSON(&m); err != nil {
						return
					}
					wsMsgs <- m
				}
			}()

			// Join first to get current revision
			conn.WriteJSON(Message{Type: MsgTypeJoin, GameId: gameId})

			var resp Message
			select {
			case resp = <-wsMsgs:
			case <-time.After(5 * time.Second):
				errors <- fmt.Errorf("Client %d join timeout", clientId)
				return
			}

			currentRev := makeUUID(1) // Start ID
			if resp.Type == MsgTypeSyncUpdate && len(resp.Actions) > 0 {
				last := resp.Actions[len(resp.Actions)-1]
				var a struct {
					ID string `json:"id"`
				}
				json.Unmarshal(last, &a)
				currentRev = a.ID
			}

			for j := 0; j < actionsPerClient; j++ {
				// Generate a deterministic valid UUID for this action
				actionId := makeUUID(20000000 + (clientId * 1000) + j)
				action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":%d,"type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, actionId, time.Now().UnixMilli()))

				// Retry loop for revision mismatch
				for {
					msg := Message{Type: MsgTypeAction, Action: action, BaseRevision: currentRev, GameId: gameId}

					// Send via HTTP
					body, _ := json.Marshal(msg)
					req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})

					httpResp, err := http.DefaultClient.Do(req)
					if err != nil {
						errors <- fmt.Errorf("Client %d http failed: %v", clientId, err)
						return
					}

					var apiResp Message
					json.NewDecoder(httpResp.Body).Decode(&apiResp)
					httpResp.Body.Close()

					if apiResp.Type == MsgTypeAck {
						// Success, wait for broadcast to update currentRev
						found := false
						timeout := time.After(5 * time.Second)
						for !found {
							select {
							case m := <-wsMsgs:
								if m.Type == MsgTypeAction {
									var a struct {
										ID string `json:"id"`
									}
									json.Unmarshal(m.Action, &a)
									currentRev = a.ID
									if a.ID == actionId {
										found = true
									}
								}
							case <-timeout:
								errors <- fmt.Errorf("Client %d timed out waiting for broadcast %s", clientId, actionId)
								return
							}
						}
						break // Outer loop (next action)
					} else if apiResp.Type == MsgTypeConflict {
						currentRev = apiResp.BaseRevision
						// Also drain any pending WS actions to catch up
					drainLoop:
						for {
							select {
							case m := <-wsMsgs:
								if m.Type == MsgTypeAction {
									var a struct {
										ID string `json:"id"`
									}
									json.Unmarshal(m.Action, &a)
									currentRev = a.ID
								}
							default:
								break drainLoop
							}
						}
					} else if apiResp.Type == MsgTypeError {
						if strings.Contains(apiResp.Error, "throttled") {
							time.Sleep(100 * time.Millisecond)
							continue
						} else {
							errors <- fmt.Errorf("Client %d got error: %s", clientId, apiResp.Error)
							return
						}
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrency error: %v", err)
	}

	// Verify Log Length
	game, _ := gStore.LoadGame(gameId)

	expected := 1 + (clientCount * actionsPerClient)
	if len(game.ActionLog) != expected {
		t.Errorf("Expected %d actions, got %d", expected, len(game.ActionLog))
	}
}

func TestIdempotency(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "idempotency_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)

	_, handler := NewServerHandler(Options{
		GameStore:      gStore,
		TeamStore:      tStore,
		Storage:        s,
		Registry:       reg,
		UserIndexStore: us,
		UseMockAuth:    true,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	userId := "idempotent@example.com"
	gameId := "10000000-0000-4000-8000-000000000005"

	// 1. Bootstrap
	startId := makeUUID(1)
	action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":1,"type":"GAME_START","payload":{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"B","ownerId":"%s"}}`, startId, gameId, userId))
	msg := Message{Type: MsgTypeAction, Action: action, GameId: gameId}
	body, _ := json.Marshal(msg)
	req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("Bootstrap failed")
	}
	resp.Body.Close()

	// 2. Send Action A
	actionAId := makeUUID(2)
	actionA := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":2,"type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, actionAId))
	msgA := Message{Type: MsgTypeAction, Action: actionA, BaseRevision: startId, GameId: gameId}
	bodyA, _ := json.Marshal(msgA)
	reqA, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(bodyA))
	reqA.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})

	respA, err := http.DefaultClient.Do(reqA)
	if err != nil {
		t.Fatalf("Action A failed: %v", err)
	}
	respA.Body.Close()

	// 3. Resend Action A (Simulate Retry)
	// Same ID, Same BaseRevision (which is now Stale because Server has A)
	reqRetry, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(bodyA))
	reqRetry.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
	respRetry, err := http.DefaultClient.Do(reqRetry)
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}
	defer respRetry.Body.Close()

	var apiResp Message
	json.NewDecoder(respRetry.Body).Decode(&apiResp)

	if apiResp.Type == MsgTypeConflict {
		t.Errorf("Expected ACK for idempotent retry, got Conflict: %s", apiResp.Error)
	}
	if apiResp.Type != MsgTypeAck {
		t.Errorf("Expected ACK, got %s", apiResp.Type)
	}
}

func TestBootstrapRace(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "race_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)

	_, handler := NewServerHandler(Options{
		GameStore:      gStore,
		TeamStore:      tStore,
		Storage:        s,
		Registry:       reg,
		UserIndexStore: us,
		UseMockAuth:    true,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	userId := "racer@example.com"
	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Add("Cookie", "mock_auth_user="+userId)

	gameId := "10000000-0000-4000-8000-000000000002"
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws?gameId=" + gameId

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	// Send GAME_START via HTTP
	startId := makeUUID(100)
	startAction := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":1,"type":"GAME_START","payload":{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"H","ownerId":"%s"}}`, startId, gameId, userId))

	msg := Message{Type: MsgTypeAction, Action: startAction, GameId: gameId}
	body, _ := json.Marshal(msg)
	req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
	http.DefaultClient.Do(req)

	// Immediately send PITCH via HTTP (without waiting for echo)
	pitchId := makeUUID(101)
	pitchAction := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":2,"type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, pitchId))

	msg2 := Message{Type: MsgTypeAction, Action: pitchAction, BaseRevision: startId, GameId: gameId}
	body2, _ := json.Marshal(msg2)
	req2, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body2))
	req2.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
	http.DefaultClient.Do(req2)

	var resp Message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// We expect broadcasts for both actions on the WS connection
	// Since we used HTTP, we don't get ACKs on WS, only ACTION broadcasts.

	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read start broadcast: %v", err)
	}
	if resp.Type != MsgTypeAction {
		t.Errorf("Expected start broadcast, got %s", resp.Type)
	}

	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("Failed to read pitch broadcast: %v", err)
	}
	if resp.Type != MsgTypeAction {
		t.Errorf("Expected pitch broadcast, got %s", resp.Type)
	}
}

func TestMixedTraffic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mixed_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)

	_, handler := NewServerHandler(Options{
		GameStore:      gStore,
		TeamStore:      tStore,
		Storage:        s,
		Registry:       reg,
		UserIndexStore: us,
		UseMockAuth:    true,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	userId := "mixed@example.com"
	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Add("Cookie", "mock_auth_user="+userId)

	gameId := "10000000-0000-4000-8000-000000000003"
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws?gameId=" + gameId

	startId := makeUUID(200)
	gameData := fmt.Sprintf(`{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"H","ownerId":"%s","actionLog":[{"id":"%s","type":"GAME_START","payload":{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"H","ownerId":"%s"}}]}`, gameId, userId, startId, gameId, userId)

	req, _ := http.NewRequest("POST", server.URL+"/api/save", strings.NewReader(gameData))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("Initial save failed: %v, status %d", err, resp.StatusCode)
	}

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		actionId := makeUUID(201)
		action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":2,"type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, actionId))

		msg := Message{Type: MsgTypeAction, Action: action, BaseRevision: startId, GameId: gameId}
		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		http.DefaultClient.Do(req)

		var r Message
		for {
			conn.ReadJSON(&r)
			if r.Type == MsgTypeAction {
				var a struct {
					ID string `json:"id"`
				}
				json.Unmarshal(r.Action, &a)
				if a.ID == actionId {
					break
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		req, _ := http.NewRequest("GET", server.URL+"/api/load/"+gameId, nil)
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		r, err := http.DefaultClient.Do(req)
		if err != nil || r.StatusCode != 200 {
			t.Errorf("Load failed: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		saveId := makeUUID(202)
		saveData := fmt.Sprintf(`{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"H","ownerId":"%s","actionLog":[{"id":"%s","type":"GAME_START","payload":{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"H","ownerId":"%s"}},{"id":"%s","type":"PITCH","payload":{"type":"strike","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}]}`, gameId, userId, startId, gameId, userId, saveId)
		req, _ := http.NewRequest("POST", server.URL+"/api/save", strings.NewReader(saveData))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		r, err := http.DefaultClient.Do(req)
		if err != nil || r.StatusCode != 200 {
			t.Errorf("Save failed: %v", err)
		}
	}()

	wg.Wait()
}

func TestTeamHubConcurrency(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "team_hub_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)

	_, handler := NewServerHandler(Options{
		GameStore:      gStore,
		TeamStore:      tStore,
		Storage:        s,
		Registry:       reg,
		UserIndexStore: us,
		UseMockAuth:    true,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	userId := "team@example.com"
	teamId := "20000000-0000-4000-8000-000000000001"

	// Concurrent HTTP Save/Load for Team
	clientCount := 10
	var wg sync.WaitGroup
	wg.Add(clientCount)

	for i := 0; i < clientCount; i++ {
		go func(clientId int) {
			defer wg.Done()
			// Save
			teamData := fmt.Sprintf(`{"id":"%s","name":"Team %d","ownerId":"%s"}`, teamId, clientId, userId)
			req, _ := http.NewRequest("POST", server.URL+"/api/save-team", strings.NewReader(teamData))
			req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
			r, err := http.DefaultClient.Do(req)
			if err != nil || r.StatusCode != 200 {
				t.Errorf("Team Save %d failed: %v", clientId, err)
			}

			// Load
			reqL, _ := http.NewRequest("GET", server.URL+"/api/load-team/"+teamId, nil)
			reqL.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
			rL, err := http.DefaultClient.Do(reqL)
			if err != nil || rL.StatusCode != 200 {
				t.Errorf("Team Load %d failed: %v", clientId, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestStaleCacheRecovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "stale_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)

	_, handler := NewServerHandler(Options{
		GameStore:      gStore,
		TeamStore:      tStore,
		Storage:        s,
		Registry:       reg,
		UserIndexStore: us,
		UseMockAuth:    true,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	userId := "stale@example.com"
	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Add("Cookie", "mock_auth_user="+userId)

	gameId := "10000000-0000-4000-8000-000000000004"
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws?gameId=" + gameId

	// 1. Initial Game State (Rev 1)
	startId := makeUUID(1)
	game := Game{
		ID:            gameId,
		SchemaVersion: SchemaVersionV3,
		OwnerID:       userId,
		ActionLog: []json.RawMessage{
			json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":1,"type":"GAME_START","payload":{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"B","ownerId":"%s"}}`, startId, gameId, userId)),
		},
	}
	if err := gStore.SaveGame(&game); err != nil {
		t.Fatalf("Initial save failed: %v", err)
	}

	// 2. Connect Client (caches Rev 1 in Hub)
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	// Wait for JOIN/ACK
	conn.WriteJSON(Message{Type: MsgTypeJoin, GameId: gameId, LastRevision: startId})
	var msg Message
	conn.ReadJSON(&msg) // Expect ACK or SYNC_UPDATE

	// 3. Simulate External Update (Rev 2) - Bypassing Hub Broadcast
	// This makes Hub cache (Rev 1) stale compared to Disk (Rev 2)
	actionId2 := makeUUID(2)
	game.ActionLog = append(game.ActionLog, json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":2,"type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, actionId2)))
	if err := gStore.SaveGame(&game); err != nil {
		t.Fatalf("External save failed: %v", err)
	}

	// 4. Client sends Action building on Rev 2
	actionId3 := makeUUID(3)
	action3 := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":3,"type":"PITCH","payload":{"type":"strike","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, actionId3))

	// Client sends action with BaseRevision = Rev 2
	// If Hub doesn't reload, it sees Rev 1 vs Rev 2 mismatch -> CONFLICT
	// If Hub reloads, it sees Rev 2 vs Rev 2 match -> ACK
	sendMsg := Message{
		Type:         MsgTypeAction,
		Action:       action3,
		BaseRevision: actionId2, // Rev 2
		GameId:       gameId,
	}

	body, _ := json.Marshal(sendMsg)
	req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP action failed: %v", err)
	}
	defer resp.Body.Close()

	var apiResp Message
	json.NewDecoder(resp.Body).Decode(&apiResp)

	if apiResp.Type == MsgTypeConflict {
		t.Fatalf("Got CONFLICT, expected ACK. Stale cache recovery failed. Error: %s, BaseRev: %s", apiResp.Error, apiResp.BaseRevision)
	}
	if apiResp.Type != MsgTypeAck {
		t.Fatalf("Expected ACK, got %s: %s", apiResp.Type, apiResp.Error)
	}
}
