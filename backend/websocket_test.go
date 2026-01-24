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
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/gorilla/websocket"
)

func TestWebSocket(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ws_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)

	_, _, handler := NewServerHandler(Options{
		GameStore:      gStore,
		TeamStore:      tStore,
		Storage:        s,
		Registry:       reg,
		UserIndexStore: us,
		UseMockAuth:    true,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	userId := "wsuser@example.com"
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

	// Helper to bootstrap a game
	bootstrap := func(t *testing.T, gId string) {
		// Valid UUID for action ID
		actionId := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
		action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":123,"type":"GAME_START","payload":{"id":"%s","date":"2025-12-18T14:57:39Z","away":"A","home":"B","ownerId":"%s"}}`, actionId, gId, userId))

		msg := Message{Type: MsgTypeAction, Action: action, GameId: gId}
		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("Bootstrap failed: %v", err)
		}
		resp.Body.Close()
	}

	// Test 1: Connect and Join (Existing Game)
	t.Run("ConnectAndJoin", func(t *testing.T) {
		gId := "10000000-0000-4000-8000-000000000001"
		bootstrap(t, gId)

		conn, _, err := dialer.Dial(getWSURL(gId), header)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		joinMsg := Message{Type: MsgTypeJoin, GameId: gId}
		conn.WriteJSON(joinMsg)

		var resp Message
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := conn.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}
		if resp.Type != MsgTypeSyncUpdate && resp.Type != MsgTypeAck {
			t.Errorf("Expected SYNC_UPDATE or ACK, got %s: %s", resp.Type, resp.Error)
		}
	})

	// Test 2: Send Action
	t.Run("SendAction", func(t *testing.T) {
		gId := "10000000-0000-4000-8000-000000000002"
		bootstrap(t, gId)

		conn, _, err := dialer.Dial(getWSURL(gId), header)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		actionId := "20000000-0000-4000-8000-000000000001"
		// BaseRevision matches the bootstrap action ID
		baseRev := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
		action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":123,"type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, actionId))
		msg := Message{Type: MsgTypeAction, Action: action, BaseRevision: baseRev, GameId: gId}

		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		httpResp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("HTTP action failed: %v", err)
		}
		defer httpResp.Body.Close()

		var apiResp Message
		json.NewDecoder(httpResp.Body).Decode(&apiResp)
		if apiResp.Type != MsgTypeAck {
			t.Errorf("Expected ACK from HTTP, got %s: %s", apiResp.Type, apiResp.Error)
		}

		var resp Message
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := conn.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read broadcast: %v", err)
		}
		if resp.Type != MsgTypeAction {
			t.Errorf("Expected broadcasted action, got %s: %s", resp.Type, resp.Error)
		}
	})

	// Test 3: Sync Catch-up
	t.Run("SyncCatchUp", func(t *testing.T) {
		gId := "10000000-0000-4000-8000-000000000003"
		bootstrap(t, gId)

		connA, _, err := dialer.Dial(getWSURL(gId), header)
		if err != nil {
			t.Fatalf("Failed to connect A: %v", err)
		}
		a2Id := "30000000-0000-4000-8000-000000000002"
		baseRev := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
		action2 := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":124,"type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, a2Id))

		msg := Message{Type: MsgTypeAction, Action: action2, BaseRevision: baseRev, GameId: gId}
		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		http.DefaultClient.Do(req)

		// Consume broadcast on connA (optional, but good practice to verify)
		connA.SetReadDeadline(time.Now().Add(1 * time.Second))
		var dummy Message
		connA.ReadJSON(&dummy) // Broadcast
		connA.Close()

		connB, _, err := dialer.Dial(getWSURL(gId), header)
		if err != nil {
			t.Fatalf("Failed to connect B: %v", err)
		}
		defer connB.Close()
		joinMsg := Message{Type: MsgTypeJoin, GameId: gId, LastRevision: baseRev}
		connB.WriteJSON(joinMsg)

		var resp Message
		connB.ReadJSON(&resp)
		if resp.Type != MsgTypeSyncUpdate || len(resp.Actions) != 1 {
			t.Errorf("Expected SyncUpdate with 1 action, got Type=%s, Count=%d", resp.Type, len(resp.Actions))
		}
	})

	// Test 4: Conflict Detection
	t.Run("ConflictDetection", func(t *testing.T) {
		gId := "10000000-0000-4000-8000-000000000004"
		bootstrap(t, gId)

		conn, _, err := dialer.Dial(getWSURL(gId), header)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		actionId := "40000000-0000-4000-8000-000000000001"
		action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":125,"type":"PITCH","payload":{"type":"strike","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, actionId))
		msg := Message{Type: MsgTypeAction, Action: action, BaseRevision: "stale", GameId: gId}

		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		httpResp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("HTTP action failed: %v", err)
		}
		defer httpResp.Body.Close()

		var resp Message
		json.NewDecoder(httpResp.Body).Decode(&resp)

		if resp.Type != MsgTypeConflict {
			t.Errorf("Expected Conflict, got %s", resp.Type)
		}
	})

	// Test 5: Unauthorized Action
	t.Run("UnauthorizedWrite", func(t *testing.T) {
		gId := "10000000-0000-4000-8000-000000000005"
		bootstrap(t, gId)

		otherHeader := http.Header{}
		otherHeader.Add("Cookie", "mock_auth_user=attacker@example.com")
		conn, _, err := dialer.Dial(getWSURL(gId), otherHeader)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		actionId := "50000000-0000-4000-8000-000000000001"
		baseRev := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
		action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":126,"type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, actionId))
		msg := Message{Type: MsgTypeAction, Action: action, BaseRevision: baseRev, GameId: gId}

		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: "attacker@example.com"})
		httpResp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("HTTP action failed: %v", err)
		}
		defer httpResp.Body.Close()

		// Hub returns 200 OK but with Error in JSON for permission denial
		if httpResp.StatusCode != 200 {
			t.Errorf("Expected 200 OK with error body, got %d", httpResp.StatusCode)
		}

		var resp Message
		json.NewDecoder(httpResp.Body).Decode(&resp)
		if resp.Type != MsgTypeError || !strings.Contains(resp.Error, "Forbidden") {
			t.Errorf("Expected Forbidden error in JSON, got Type=%s, Error=%s", resp.Type, resp.Error)
		}
	})

	t.Run("MalformedMessage", func(t *testing.T) {
		gId := "00000000-0000-4000-8000-000000000008"
		conn, _, err := dialer.Dial(getWSURL(gId), header)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()
		conn.WriteMessage(websocket.TextMessage, []byte(`invalid`))
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, _, err = conn.ReadMessage()
		if err == nil {
			t.Error("Expected closure for malformed JSON")
		}
	})

	t.Run("JoinNonExistent", func(t *testing.T) {
		gId := "00000000-0000-4000-8000-000000000008"
		conn, _, err := dialer.Dial(getWSURL(gId), header)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()
		joinMsg := Message{Type: MsgTypeJoin, GameId: gId}
		conn.WriteJSON(joinMsg)
		var resp Message
		conn.ReadJSON(&resp)
		if resp.Type != MsgTypeAck {
			t.Errorf("Expected ACK for join-non-existent (bootstrap), got %s", resp.Type)
		}
	})

	t.Run("UnauthorizedJoin", func(t *testing.T) {
		gId := "10000000-0000-4000-8000-000000000006"
		bootstrap(t, gId)

		otherHeader := http.Header{}
		otherHeader.Add("Cookie", "mock_auth_user=attacker@example.com")
		conn, _, err := dialer.Dial(getWSURL(gId), otherHeader)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		joinMsg := Message{Type: MsgTypeJoin, GameId: gId}
		conn.WriteJSON(joinMsg)
		var resp Message
		conn.ReadJSON(&resp)
		if resp.Type != MsgTypeError || !strings.Contains(resp.Error, "Forbidden") {
			t.Errorf("Expected Forbidden error, got %s", resp.Error)
		}
	})

	t.Run("DivergentSync", func(t *testing.T) {
		gId := "10000000-0000-4000-8000-000000000009"
		bootstrap(t, gId)

		conn, _, err := dialer.Dial(getWSURL(gId), header)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		joinMsg := Message{Type: MsgTypeJoin, GameId: gId, LastRevision: "non-existent"}
		conn.WriteJSON(joinMsg)

		var resp Message
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		conn.ReadJSON(&resp)
		if resp.Type != MsgTypeConflict || !strings.Contains(resp.Error, "divergent") {
			t.Errorf("Expected Conflict for divergent sync, got %s", resp.Type)
		}
	})

	t.Run("MalformedActionPayload", func(t *testing.T) {
		gId := "10000000-0000-4000-8000-000000000010"
		bootstrap(t, gId)

		conn, _, err := dialer.Dial(getWSURL(gId), header)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		msg := Message{
			Type:   MsgTypeAction,
			Action: json.RawMessage(`{"id":"bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb","type":"PITCH","payload":{invalid}}`),
			GameId: gId,
		}

		body, _ := json.Marshal(msg)
		// We need to inject raw malformed JSON inside the message structure?
		// json.Marshal will fail if I construct it with invalid struct content?
		// Wait, json.RawMessage is just []byte. It won't fail Marshal.
		// But api/action decodes Request Body into Message struct.
		// Then it passes msg to Hub.
		// Hub handles it.

		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		httpResp, _ := http.DefaultClient.Do(req)

		var resp Message
		json.NewDecoder(httpResp.Body).Decode(&resp)

		// HTTP handler returns 400 for malformed top-level JSON.
		// But here the "Action" payload is malformed JSON.
		// Hub might return Error.

		if resp.Type != MsgTypeError || !strings.Contains(resp.Error, "malformed") {
			// If status code 400 is returned, we accept that too?
			if httpResp.StatusCode != 400 {
				t.Errorf("Expected Malformed JSON error or 400, got %d %s", httpResp.StatusCode, resp.Error)
			}
		}
	})

	t.Run("PublicJoin", func(t *testing.T) {
		gId := "00000000-0000-4000-8000-000000000011"
		// Bootstrap public game via HTTP
		actionId := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
		action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":123,"type":"GAME_START","payload":{"id":"%s","date":"2025-12-18T14:57:39Z","away":"A","home":"B","ownerId":"%s","permissions":{"public":"read"}}}`, actionId, gId, userId))

		msg := Message{Type: MsgTypeAction, Action: action, GameId: gId}
		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		http.DefaultClient.Do(req)

		// Anonymous join (no cookie)
		connB, _, err := dialer.Dial(getWSURL(gId), nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer connB.Close()

		joinMsg := Message{Type: MsgTypeJoin, GameId: gId}
		connB.WriteJSON(joinMsg)

		var resp Message
		connB.SetReadDeadline(time.Now().Add(1 * time.Second))
		if err := connB.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}
		if resp.Type != MsgTypeAck && resp.Type != MsgTypeSyncUpdate {
			t.Errorf("Expected ACK or SYNC_UPDATE for public JOIN, got %s", resp.Type)
		}
	})

	t.Run("MetadataUpdateSync", func(t *testing.T) {
		gId := "00000000-0000-4000-8000-000000000012"
		bootstrap(t, gId)

		conn, _, _ := dialer.Dial(getWSURL(gId), header)
		defer conn.Close()

		// Send metadata update to make it public via HTTP
		actionId := "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb"
		baseRev := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
		action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":124,"type":"GAME_METADATA_UPDATE","payload":{"id":"%s","permissions":{"public":"read"}}}`, actionId, gId))
		msg := Message{Type: MsgTypeAction, Action: action, BaseRevision: baseRev, GameId: gId}

		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		http.DefaultClient.Do(req)

		var resp Message
		// Expect broadcast on WS
		conn.ReadJSON(&resp)

		// Verify registry is updated
		games := reg.ListGames("", "", "", "") // List public games
		found := false
		for _, id := range games {
			if id == gId {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Public game not found in registry after WebSocket metadata update")
		}
	})

	t.Run("UnauthorizedMetadataUpdate", func(t *testing.T) {
		gId := "00000000-0000-4000-8000-000000000013"
		// User has Write access via team scorekeeper role, but not Admin
		// Need to set up a game and team such that user is a scorekeeper
		// For simplicity, let's just use direct permissions if supported, or bootstrap.
		// Actually, standard bootstrap gives Owner (Admin).
		// Let's bootstrap as User A (Owner), then User B (Editor) tries to update metadata.

		ownerId := "owner@example.com"
		editorId := "editor@example.com"

		// Bootstrap game via HTTP
		actionId := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
		action := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":123,"type":"GAME_START","payload":{"id":"%s","date":"2025-12-18T14:57:39Z","away":"A","home":"B","ownerId":"%s"}}`, actionId, gId, ownerId))

		msg := Message{Type: MsgTypeAction, Action: action, GameId: gId}
		body, _ := json.Marshal(msg)
		req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: ownerId})
		http.DefaultClient.Do(req)

		// Grant Write access to editor via HTTP
		metaId := "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb"
		metaAction := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":124,"type":"GAME_METADATA_UPDATE","payload":{"id":"%s","permissions":{"users":{"%s":"write"}}}}`, metaId, gId, editorId))

		msg2 := Message{Type: MsgTypeAction, Action: metaAction, BaseRevision: actionId, GameId: gId}
		body2, _ := json.Marshal(msg2)
		req2, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body2))
		req2.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: ownerId})
		http.DefaultClient.Do(req2)

		// Editor tries to update metadata (e.g. make public) via HTTP
		attackId := "cccccccc-cccc-4ccc-cccc-cccccccccccc"
		attackAction := json.RawMessage(fmt.Sprintf(`{"id":"%s","timestamp":125,"type":"GAME_METADATA_UPDATE","payload":{"id":"%s","permissions":{"public":"read"}}}`, attackId, gId))

		msg3 := Message{Type: MsgTypeAction, Action: attackAction, BaseRevision: metaId, GameId: gId}
		body3, _ := json.Marshal(msg3)
		req3, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(body3))
		req3.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: editorId})
		respHTTP, err := http.DefaultClient.Do(req3)
		if err != nil {
			t.Fatalf("HTTP action failed: %v", err)
		}
		defer respHTTP.Body.Close()

		// Hub returns 200 OK but with Error in JSON for permission denial
		if respHTTP.StatusCode != 200 {
			t.Errorf("Expected 200 OK with error body, got %d", respHTTP.StatusCode)
		}

		var resp Message
		json.NewDecoder(respHTTP.Body).Decode(&resp)
		if resp.Type != MsgTypeError || !strings.Contains(resp.Error, "Forbidden") {
			t.Errorf("Expected Forbidden for non-admin metadata update in JSON, got Type=%s, Error=%s", resp.Type, resp.Error)
		}
	})
}

func TestGetCurrentRevision(t *testing.T) {
	a1Id := "10000000-0000-4000-8000-000000000001"
	a2Id := "10000000-0000-4000-8000-000000000002"
	tests := []struct {
		name string
		log  []json.RawMessage
		want string
	}{
		{"Empty Log", nil, ""},
		{"Single Action", []json.RawMessage{json.RawMessage(fmt.Sprintf(`{"id":"%s"}`, a1Id))}, a1Id},
		{"Multiple Actions", []json.RawMessage{
			json.RawMessage(fmt.Sprintf(`{"id":"%s"}`, a1Id)),
			json.RawMessage(fmt.Sprintf(`{"id":"%s"}`, a2Id)),
		}, a2Id},
		{"Malformed Action", []json.RawMessage{json.RawMessage(`invalid`)}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCurrentRevision(tt.log); got != tt.want {
				t.Errorf("getCurrentRevision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetActionsSince(t *testing.T) {
	a1Id := "10000000-0000-4000-8000-000000000001"
	a2Id := "10000000-0000-4000-8000-000000000002"
	a3Id := "10000000-0000-4000-8000-000000000003"
	log := []json.RawMessage{
		json.RawMessage(fmt.Sprintf(`{"id":"%s"}`, a1Id)),
		json.RawMessage(fmt.Sprintf(`{"id":"%s"}`, a2Id)),
		json.RawMessage(fmt.Sprintf(`{"id":"%s"}`, a3Id)),
	}

	tests := []struct {
		name     string
		revision string
		wantLen  int
	}{
		{"From Start", "", 3},
		{"From a1", a1Id, 2},
		{"From a2", a2Id, 1},
		{"From a3", a3Id, 0},
		{"Non-existent", "non-existent", 0}, // Returns nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getActionsSince(log, tt.revision)
			if tt.wantLen == 0 && got != nil && len(got) != 0 {
				t.Errorf("getActionsSince() = %v, want empty/nil", got)
			} else if tt.wantLen > 0 && len(got) != tt.wantLen {
				t.Errorf("getActionsSince() length = %v, want %v", len(got), tt.wantLen)
			}
		})
	}
}
