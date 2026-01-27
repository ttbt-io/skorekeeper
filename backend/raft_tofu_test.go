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
	"encoding/base64"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
)

func getFreePort() string {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	return ln.Addr().String()
}

func TestRaftTOFU(t *testing.T) {

	tempDir := t.TempDir()

	dataDir1 := filepath.Join(tempDir, "node1", "data")

	raftDir1 := filepath.Join(tempDir, "node1", "raft")
	dataDir2 := filepath.Join(tempDir, "node2", "data")
	raftDir2 := filepath.Join(tempDir, "node2", "raft")

	bind1 := getFreePort()
	cluster1 := getFreePort()

	s1 := storage.New(dataDir1, nil)
	rs1 := storage.New(raftDir1, nil)
	gs1 := NewGameStore(dataDir1, s1)
	ts1 := NewTeamStore(dataDir1, s1)
	us1 := NewUserIndexStore(dataDir1, s1, nil)
	reg1 := NewRegistry(gs1, ts1, us1, true)
	fsm1 := NewFSM(gs1, ts1, reg1, NewHubManager(), rs1, us1)
	rm1 := NewRaftManager(raftDir1, bind1, bind1, cluster1, cluster1, "secret", nil, fsm1)

	var tofuCount1 atomic.Int32
	rm1.tofuCallback = func(nodeID string) {
		t.Logf("Node 1: TOFU accepted for node %s", nodeID)
		tofuCount1.Add(1)
	}

	if err := rm1.Start(true); err != nil {
		t.Fatalf("Failed to start Node 1: %v", err)
	}
	defer rm1.Shutdown()

	// Wait for Leader
	for rm1.Raft.State().String() != "Leader" {
		time.Sleep(100 * time.Millisecond)
	}

	// Start Node 2
	bind2 := getFreePort()
	cluster2 := getFreePort()

	s2 := storage.New(dataDir2, nil)
	rs2 := storage.New(raftDir2, nil)
	gs2 := NewGameStore(dataDir2, s2)
	ts2 := NewTeamStore(dataDir2, s2)
	us2 := NewUserIndexStore(dataDir2, s2, nil)
	reg2 := NewRegistry(gs2, ts2, us2, true)
	fsm2 := NewFSM(gs2, ts2, reg2, NewHubManager(), rs2, us2)
	rm2 := NewRaftManager(raftDir2, bind2, bind2, cluster2, cluster2, "secret", nil, fsm2)

	var tofuCount2 atomic.Int32
	rm2.tofuCallback = func(nodeID string) {
		t.Logf("Node 2: TOFU accepted for node %s", nodeID)
		tofuCount2.Add(1)
	}

	if err := rm2.Start(false); err != nil {
		t.Fatalf("Failed to start Node 2: %v", err)
	}

	// Join Node 2 to Node 1
	// Priming Node 2 with Node 1's pub key manually? No, we WANT to test TOFU.
	// But we need to know where to connect.

	t.Log("Joining Node 2 to Node 1...")
	if err := rm1.Join(rm2.NodeID, bind2, cluster2, base64.StdEncoding.EncodeToString(rm2.PubKey), false, CurrentAppVersion, CurrentProtocolVersion, CurrentSchemaVersion); err != nil {
		t.Fatalf("Failed to join Node 2: %v", err)
	}

	// Wait for heartbeats.
	// Node 1 (Leader) will connect to Node 2 (Follower).
	// Node 2 doesn't know Node 1's pubkey yet, so it should use TOFU.
	time.Sleep(2 * time.Second)

	if tofuCount2.Load() == 0 {
		t.Error("Node 2 should have used TOFU for Node 1's connection")
	}
	t.Logf("Initial TOFU count for Node 2: %d", tofuCount2.Load())

	// Restart Node 2
	t.Log("Restarting Node 2...")
	rm2.Shutdown()
	time.Sleep(500 * time.Millisecond) // Give OS some time to release file locks
	tofuCount2.Store(0)

	// New RaftManager instance with same directories
	gs2b := NewGameStore(dataDir2, s2)
	ts2b := NewTeamStore(dataDir2, s2)
	us2b := NewUserIndexStore(dataDir2, s2, nil)
	reg2b := NewRegistry(gs2b, ts2b, us2b, true)
	rs2b := storage.New(raftDir2, nil)
	fsm2b := NewFSM(gs2b, ts2b, reg2b, NewHubManager(), rs2b, us2b)
	if !fsm2b.IsInitialized() {
		t.Error("FSM should be initialized after restart")
	}

	rm2b := NewRaftManager(raftDir2, bind2, bind2, cluster2, cluster2, "secret", nil, fsm2b)
	rm2b.tofuCallback = func(nodeID string) {
		t.Logf("Node 2 (Restarted): TOFU accepted for node %s", nodeID)
		tofuCount2.Add(1)
	}

	if err := rm2b.Start(false); err != nil {
		t.Fatalf("Failed to start Node 2b: %v", err)
	}

	// Verify that it eventually connects successfully without TOFU
	success := false
	for i := 0; i < 50; i++ {
		if rm2b.Raft.State().String() != "Follower" && rm2b.Raft.State().String() != "Leader" {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		// Check if it has Node 1 in its configuration
		cfg := rm2b.Raft.GetConfiguration()
		if err := cfg.Error(); err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		found := false
		for _, s := range cfg.Configuration().Servers {
			if string(s.ID) == rm1.NodeID {
				found = true
				break
			}
		}
		if found {
			success = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !success {
		t.Error("Node 2 failed to reconnect to Node 1 after restart")
	}

	if tofuCount2.Load() > 0 {
		t.Errorf("Node 2 should NOT have used TOFU after restart, got %d times", tofuCount2.Load())
	}

	// Restart Node 1
	t.Log("Restarting Node 1...")
	rm1.Shutdown()
	time.Sleep(500 * time.Millisecond)
	tofuCount1.Store(0)

	rs1b := storage.New(raftDir1, nil)
	us1b := NewUserIndexStore(dataDir1, s1, nil)
	fsm1b := NewFSM(gs1, ts1, reg1, NewHubManager(), rs1b, us1b)
	rm1b := NewRaftManager(raftDir1, bind1, bind1, cluster1, cluster1, "secret", nil, fsm1b)
	rm1b.tofuCallback = func(nodeID string) {
		t.Logf("Node 1 (Restarted): TOFU accepted for node %s", nodeID)
		tofuCount1.Add(1)
	}

	// Restarting Node 1 with bootstrap=false because it's already bootstrapped on disk
	if err := rm1b.Start(false); err != nil {
		t.Fatalf("Failed to start Node 1b: %v", err)
	}
	defer rm1b.Shutdown()

	time.Sleep(2 * time.Second)

	if tofuCount1.Load() > 0 {
		t.Errorf("Node 1 should NOT have used TOFU after restart, got %d times", tofuCount1.Load())
	}
}
