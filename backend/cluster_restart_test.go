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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
)

func TestClusterRestart(t *testing.T) {
	// 1. Setup Cluster
	nodeCount := 3
	dataDirs := make([]string, nodeCount)
	raftPorts := make([]int, nodeCount)
	clusterPorts := make([]int, nodeCount)

	// Pre-allocate dirs and ports
	for i := 0; i < nodeCount; i++ {
		dir := t.TempDir()
		dataDirs[i] = dir

		// Get free ports
		l1, _ := net.Listen("tcp", "127.0.0.1:0")
		raftPorts[i] = l1.Addr().(*net.TCPAddr).Port
		l1.Close()

		l2, _ := net.Listen("tcp", "127.0.0.1:0")
		clusterPorts[i] = l2.Addr().(*net.TCPAddr).Port
		l2.Close()
	}

	servers, rms := startCluster(t, nodeCount, dataDirs, raftPorts, clusterPorts, true)
	defer func() {
		for _, s := range servers {
			s.Close()
		}
		for _, rm := range rms {
			rm.Shutdown()
		}
	}()

	leaderIdx := waitForLeader(t, rms)
	leaderURL := servers[leaderIdx].URL

	// 2. Create Game & Populate
	gameID := makeUUID(9999)
	userID := "test-user@example.com"

	// Create Game
	startPayload := fmt.Sprintf(`{"id":"%s","timestamp":1,"type":"GAME_START","payload":{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"B","ownerId":"%s"}}`, makeUUID(0), gameID, userID)
	sendAction(t, leaderURL, gameID, startPayload, "", userID)

	// Populate 200 actions
	t.Log("Populating 200 actions...")
	lastActionID := makeUUID(0)
	for i := 1; i <= 200; i++ {
		actionID := makeUUID(i)
		payload := fmt.Sprintf(`{"id":"%s","timestamp":%d,"type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, actionID, time.Now().UnixMilli())
		lastActionID = sendAction(t, leaderURL, gameID, payload, lastActionID, userID)
		if i%10 == 0 {
			t.Logf("Sent %d actions...", i)
		}
	}

	// Ensure all nodes caught up
	t.Log("Waiting for replication...")
	for i, rm := range rms {
		if err := rm.WaitForSync(10 * time.Second); err != nil {
			t.Errorf("Node %d failed to sync: %v", i, err)
		}
	}

	// DEBUG: Check files
	entries, _ := os.ReadDir(filepath.Join(dataDirs[0], "raft"))
	t.Logf("Raft Dir Node 0: %v", entries)
	entries2, _ := os.ReadDir(filepath.Join(dataDirs[0], "games"))
	t.Logf("Games Dir Node 0: %v", entries2)
	// Check size of raft log
	if fi, err := os.Stat(filepath.Join(dataDirs[0], "raft", "raft-log.bolt")); err == nil {
		t.Logf("Raft Log Size Node 0: %d", fi.Size())
	}

	// 3. Restart Cluster
	t.Log("Restarting cluster...")
	for _, s := range servers {
		s.Close()
	}
	for _, rm := range rms {
		if rm.FSM != nil {
			if err := rm.FSM.FlushAll(); err != nil {
				t.Errorf("Node %s FlushAll failed: %v", rm.NodeID, err)
			}
		}
		rm.Shutdown()
	}
	// Give Raft time to shutdown ports
	time.Sleep(1 * time.Second)

	// Start again (bootstrap=false because data exists)
	servers, rms = startCluster(t, nodeCount, dataDirs, raftPorts, clusterPorts, false)
	defer func() {
		for _, s := range servers {
			s.Close()
		}
		for _, rm := range rms {
			rm.Shutdown()
		}
	}()

	leaderIdx = waitForLeader(t, rms)
	leaderURL = servers[leaderIdx].URL
	t.Logf("New Leader: Node %d (%s)", leaderIdx, leaderURL)

	// 4. Client Reconnect & Append
	// Client sends new action based on lastActionID
	newActionID := makeUUID(201)
	newPayload := fmt.Sprintf(`{"id":"%s","timestamp":%d,"type":"PITCH","payload":{"type":"strike","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, newActionID, time.Now().UnixMilli())

	msg := Message{
		Type:         MsgTypeAction,
		GameId:       gameID,
		Action:       json.RawMessage(newPayload),
		BaseRevision: lastActionID,
	}
	msgBytes, _ := json.Marshal(msg)

	req, _ := http.NewRequest("POST", leaderURL+"/api/action", bytes.NewReader(msgBytes))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send post-restart action: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		t.Fatalf("Got Conflict (409) after restart! The server likely reset its history or failed to load the full log.")
	}
	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		t.Fatalf("Action failed with status %d: %s", resp.StatusCode, buf.String())
	}

	var respMsg Message
	if err := json.NewDecoder(resp.Body).Decode(&respMsg); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if respMsg.Type == MsgTypeConflict {
		t.Fatalf("Got 200 OK but Type=CONFLICT: %s", respMsg.Error)
	}
	if respMsg.Type != MsgTypeAck {
		t.Fatalf("Expected ACK, got %s: %s", respMsg.Type, respMsg.Error)
	}

	t.Log("Success: Action accepted after restart without conflict.")
}

func startCluster(t *testing.T, count int, dataDirs []string, raftPorts, clusterPorts []int, bootstrap bool) ([]*httptest.Server, []*RaftManager) {
	servers := make([]*httptest.Server, count)
	rms := make([]*RaftManager, count)

	for i := 0; i < count; i++ {
		raftBind := fmt.Sprintf("127.0.0.1:%d", raftPorts[i])
		clusterAddr := fmt.Sprintf("127.0.0.1:%d", clusterPorts[i])

		s := storage.New(dataDirs[i], nil)
		gStore := NewGameStore(dataDirs[i], s)
		tStore := NewTeamStore(dataDirs[i], s)
		us := NewUserIndexStore(dataDirs[i], s, nil)
		reg := NewRegistry(gStore, tStore, us, true)

		rmChan := make(chan *RaftManager, 1)

		opts := Options{
			DataDir:          dataDirs[i],
			GameStore:        gStore,
			TeamStore:        tStore,
			Storage:          s,
			Registry:         reg,
			RaftEnabled:      true,
			RaftBind:         raftBind,
			RaftAdvertise:    raftBind,
			ClusterAddr:      clusterAddr,
			ClusterAdvertise: clusterAddr,
			RaftSecret:       "test-secret",
			RaftBootstrap:    bootstrap && i == 0,
			RaftManagerChan:  rmChan,
			UseMockAuth:      true,
		}

		_, _, handler := NewServerHandler(opts)
		server := httptest.NewServer(handler)
		servers[i] = server

		select {
		case rm := <-rmChan:
			rms[i] = rm
		case <-time.After(5 * time.Second):
			t.Fatalf("Node %d RaftManager not received", i)
		}
	}

	// If bootstrapping, join them
	if bootstrap {
		// Wait for Node 0 leader
		waitForLeader(t, []*RaftManager{rms[0]})

		for i := 1; i < count; i++ {
			// Join
			pubKey := base64.StdEncoding.EncodeToString(rms[i].PubKey)
			rms[i].AddNodePubKey(rms[0].NodeID, rms[0].ClusterAdvertise, base64.StdEncoding.EncodeToString(rms[0].PubKey))

			err := rms[0].Join(rms[i].NodeID, rms[i].Bind, rms[i].ClusterAdvertise, pubKey, false, "0.0.0", 1, 1)
			if err != nil {
				t.Fatalf("Failed to join node %d: %v", i, err)
			}
		}
	}

	return servers, rms
}

func waitForLeader(t *testing.T, rms []*RaftManager) int {
	timeout := time.After(30 * time.Second) // Increased timeout for log replay
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for leader election")
		default:
			for i, rm := range rms {
				if rm != nil && rm.Raft != nil && rm.Raft.State().String() == "Leader" {
					return i
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func sendAction(t *testing.T, url, gameID, payload, baseRevision, userID string) string {
	msg := Message{
		Type:         MsgTypeAction,
		GameId:       gameID,
		Action:       json.RawMessage(payload),
		BaseRevision: baseRevision,
	}
	msgBytes, _ := json.Marshal(msg)

	req, _ := http.NewRequest("POST", url+"/api/action", bytes.NewReader(msgBytes))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send action: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Action failed with status %d: %s", resp.StatusCode, string(body))
	}

	var respMsg Message
	if err := json.NewDecoder(resp.Body).Decode(&respMsg); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if respMsg.Type != MsgTypeAck {
		t.Fatalf("Expected ACK, got %s: %s", respMsg.Type, respMsg.Error)
	}

	// Extract ID from payload
	var action struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(payload), &action)
	return action.ID
}
