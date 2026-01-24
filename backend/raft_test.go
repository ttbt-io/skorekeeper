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
	"os"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
)

func TestRaftSingleNode(t *testing.T) {
	// Setup unique temp dirs
	dataDir, err := os.MkdirTemp("", "raft_test_data")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	// Ports
	raftBind := "127.0.0.1:50001" // Random port might be better, but this works for single run

	s := storage.New(dataDir, nil)
	gStore := NewGameStore(dataDir, s)
	tStore := NewTeamStore(dataDir, s)
	us := NewUserIndexStore(dataDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)

	// Channel to capture RaftManager
	rmChan := make(chan *RaftManager, 1)

	opts := Options{
		DataDir:          dataDir,
		GameStore:        gStore,
		TeamStore:        tStore,
		Storage:          s,
		Registry:         reg,
		RaftEnabled:      true,
		RaftBind:         raftBind,
		RaftAdvertise:    raftBind,
		ClusterAdvertise: "127.0.0.1:0",
		ClusterAddr:      "127.0.0.1:0",
		RaftSecret:       "test-secret",
		RaftBootstrap:    true,
		RaftManagerChan:  rmChan,
		UseMockAuth:      true,
	}

	_, _, handler := NewServerHandler(opts)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Wait for Raft Leader
	var rm *RaftManager
	select {
	case rm = <-rmChan:
	case <-time.After(5 * time.Second):
		t.Fatal("RaftManager not initialized")
	}

	timeout := time.After(10 * time.Second)
	leaderReady := false
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for leader election")
		default:
			if rm.Raft.State().String() == "Leader" {
				leaderReady = true
			}
		}
		if leaderReady {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Create Game via HTTP Action
	gameId := "10000000-0000-0000-0000-000000000001"
	actionId := "20000000-0000-0000-0000-000000000001"
	userId := "raft-user@example.com"

	payload := fmt.Sprintf(`{"id":"%s","timestamp":1,"type":"GAME_START","payload":{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"B","ownerId":"%s"}}`, actionId, gameId, userId)
	msg := Message{
		Type:   MsgTypeAction,
		GameId: gameId,
		Action: json.RawMessage(payload),
	}
	msgBytes, _ := json.Marshal(msg)

	req, _ := http.NewRequest("POST", server.URL+"/api/action", bytes.NewReader(msgBytes))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send action: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Action failed status: %d", resp.StatusCode)
	}

	// Verify Data on Disk (applied via FSM)
	// Allow some time for FSM Apply if async (though Propose waits, Apply calls SaveGame synchronously)

	g, err := gStore.LoadGame(gameId)
	if err != nil {
		t.Fatalf("Failed to load game from store: %v", err)
	}

	if g.OwnerID != userId {
		t.Errorf("Expected owner %s, got %s", userId, g.OwnerID)
	}

	if len(g.ActionLog) != 1 {
		t.Errorf("Expected 1 action, got %d", len(g.ActionLog))
	}
}

func TestRaftGetters(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "raft_getters")
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gs := NewGameStore(tempDir, s)
	ts := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)
	hm := NewHubManager()
	fsm := NewFSM(gs, ts, reg, hm, s, us)

	if fsm.GetHubManager() != hm {
		t.Error("GetHubManager mismatch")
	}

	gStore, tStore := fsm.GetStores()
	if gStore != gs || tStore != ts {
		t.Error("GetStores mismatch")
	}

	if fsm.GetNodeCount() != 0 {
		t.Error("GetNodeCount should be 0")
	}

	if len(fsm.GetAllNodes()) != 0 {
		t.Error("GetAllNodes should be empty")
	}

	if fsm.GetNodeAddr("fake") != "" {
		t.Error("GetNodeAddr should be empty")
	}

	if fsm.GetNodePubKey("fake") != "" {
		t.Error("GetNodePubKey should be empty")
	}

	if fsm.IsInitialized() {
		t.Error("IsInitialized should be false")
	}

	// Test GetHub (mocking internal logic partially by creating a game first)
	// Actually GetHub creates a new Hub if not exists, so it's safe.
	hub := fsm.GetHub("game-1", false)
	if hub == nil {
		t.Error("GetHub returned nil")
	}

	// Test RaftManager GetHTTPClient
	rm := &RaftManager{httpClient: &http.Client{}}
	if rm.GetHTTPClient() == nil {
		t.Error("GetHTTPClient returned nil")
	}
}
