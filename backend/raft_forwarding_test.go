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
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRaftForwarding(t *testing.T) {
	nodeCount := 2
	dataDirs := make([]string, nodeCount)
	raftPorts := make([]int, nodeCount)
	clusterPorts := make([]int, nodeCount)

	for i := 0; i < nodeCount; i++ {
		dir := t.TempDir()
		dataDirs[i] = dir

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

	// Find Leader and Follower
	leaderIdx := waitForLeader(t, rms)
	followerIdx := (leaderIdx + 1) % nodeCount
	followerURL := servers[followerIdx].URL

	t.Logf("Leader: %d, Follower: %d (%s)", leaderIdx, followerIdx, followerURL)

	// Wait for follower to know leader's HTTP address
	timeout := time.After(10 * time.Second)
	for {
		addr := rms[followerIdx].GetLeaderHTTPAddr()
		if addr != "" {
			break
		}
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for follower to discover leader address")
		case <-time.After(100 * time.Millisecond):
		}
	}

	// Create Game on Leader first (to ensure it exists)
	gameID := makeUUID(5555)
	userID := "user@forward.com"
	startPayload := fmt.Sprintf(`{"id":"%s","timestamp":1,"type":"GAME_START","payload":{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"B","ownerId":"%s"}}`, makeUUID(0), gameID, userID)

	// Send to Follower (should be forwarded)
	msg := Message{
		Type:   MsgTypeAction,
		GameId: gameID,
		Action: json.RawMessage(startPayload),
	}
	msgBytes, _ := json.Marshal(msg)

	req, _ := http.NewRequest("POST", followerURL+"/api/action", bytes.NewReader(msgBytes))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send action to follower: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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

	t.Log("Success: Action sent to follower was forwarded and applied.")
}
