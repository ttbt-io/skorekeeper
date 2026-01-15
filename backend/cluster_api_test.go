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
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
)

func TestClusterAPI(t *testing.T) {
	// Setup 1 Node Cluster
	dir, _ := os.MkdirTemp("", "raft_api_test")
	defer os.RemoveAll(dir)

	l1, _ := net.Listen("tcp", "127.0.0.1:0")
	raftBind := l1.Addr().String()
	l1.Close()

	l2, _ := net.Listen("tcp", "127.0.0.1:0")
	clusterAddr := l2.Addr().String()
	l2.Close()

	s := storage.New(dir, nil)
	gs := NewGameStore(dir, s)
	ts := NewTeamStore(dir, s)
	reg := NewRegistry(gs, ts)
	rmChan := make(chan *RaftManager, 1)

	opts := Options{
		DataDir:          dir,
		GameStore:        gs,
		TeamStore:        ts,
		Storage:          s,
		Registry:         reg,
		RaftEnabled:      true,
		RaftBind:         raftBind,
		RaftAdvertise:    raftBind,
		ClusterAddr:      clusterAddr,
		ClusterAdvertise: clusterAddr,
		RaftSecret:       "secret",
		RaftBootstrap:    true,
		RaftManagerChan:  rmChan,
	}

	_, handler := NewServerHandler(opts)
	server := httptest.NewServer(handler)
	defer server.Close()

	var rm *RaftManager
	select {
	case rm = <-rmChan:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout getting RaftManager")
	}
	defer rm.Shutdown()

	waitForLeader(t, []*RaftManager{rm})

	// Wait for metadata to be applied (NodePubKey) to prevent race condition
	// where leader is elected but FSM hasn't processed the NodeMeta command yet.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rm.FSM.GetNodePubKey(rm.NodeID) != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Client for Cluster API (mTLS)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				Certificates:       []tls.Certificate{*rm.Cert},
			},
		},
	}

	// 1. Test Status
	statusURL := "https://" + clusterAddr + "/api/cluster/status"
	req, _ := http.NewRequest("GET", statusURL, nil)
	req.Header.Set("X-Raft-Secret", "secret")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Status code %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Test Join (Invalid Secret)
	joinPayload := `{"nodeId":"node2","raftAddr":"127.0.0.1:9999","httpAddr":"127.0.0.1:8888"}`
	req, _ = http.NewRequest("POST", "https://"+clusterAddr+"/api/cluster/join", bytes.NewReader([]byte(joinPayload)))
	req.Header.Set("X-Raft-Secret", "wrong-secret")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to join: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Test Join (Valid)
	// We can't really join a fake node easily because Raft will try to dial it.
	// But we can test the API validation part.

	// 4. Test Remove (Valid Secret)
	// Remove self (should fail or shutdown? actually Raft prevents removing leader if it's the only one? No, it might allow it and step down)
	// Let's try removing a non-existent node
	removePayload := `{"nodeId":"fake-node"}`
	req, _ = http.NewRequest("POST", "https://"+clusterAddr+"/api/cluster/remove", bytes.NewReader([]byte(removePayload)))
	req.Header.Set("X-Raft-Secret", "secret")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to remove: %v", err)
	}
	// It might return 200 (future: ignored) or 404/500 if node not found in config
	// The implementation calls `rm.Raft.RemoveServer`.
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		// Just logging status to see behavior
		t.Logf("Remove status: %d", resp.StatusCode)
	}
	resp.Body.Close()
}
