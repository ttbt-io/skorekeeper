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
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/hashicorp/raft"
)

func TestNonVoterRemainsNonVoter(t *testing.T) {
	// 1. Setup Leader
	dataDir1 := t.TempDir()
	raftDir1 := filepath.Join(dataDir1, "raft")

	l1, _ := net.Listen("tcp", "127.0.0.1:0")
	leaderRaft := l1.Addr().String()
	l1.Close()

	l2, _ := net.Listen("tcp", "127.0.0.1:0")
	leaderCluster := l2.Addr().String()
	l2.Close()

	s1 := storage.New(dataDir1, nil)
	rs1 := storage.New(raftDir1, nil)
	gs1 := NewGameStore(dataDir1, s1)
	ts1 := NewTeamStore(dataDir1, s1)
	us1 := NewUserIndexStore(dataDir1, s1, nil)
	reg1 := NewRegistry(gs1, ts1, us1, true)
	fsm1 := NewFSM(gs1, ts1, reg1, NewHubManager(), rs1, us1)

	rm1 := NewRaftManager(raftDir1, leaderRaft, leaderRaft, leaderCluster, leaderCluster, "secret", nil, fsm1)
	if err := rm1.Start(true); err != nil {
		t.Fatal(err)
	}
	defer rm1.Shutdown()

	waitForLeader(t, []*RaftManager{rm1})

	// 2. Follower Setup
	dataDir2 := t.TempDir()
	raftDir2 := filepath.Join(dataDir2, "raft")

	l3, _ := net.Listen("tcp", "127.0.0.1:0")
	followerRaft := l3.Addr().String()
	l3.Close()

	l4, _ := net.Listen("tcp", "127.0.0.1:0")
	followerCluster := l4.Addr().String()
	l4.Close()

	s2 := storage.New(dataDir2, nil)
	rs2 := storage.New(raftDir2, nil)
	gs2 := NewGameStore(dataDir2, s2)
	ts2 := NewTeamStore(dataDir2, s2)
	us2 := NewUserIndexStore(dataDir2, s2, nil)
	reg2 := NewRegistry(gs2, ts2, us2, true)
	fsm2 := NewFSM(gs2, ts2, reg2, NewHubManager(), rs2, us2)

	rm2 := NewRaftManager(raftDir2, followerRaft, followerRaft, followerCluster, followerCluster, "secret", nil, fsm2)
	if err := rm2.Start(false); err != nil {
		t.Fatal(err)
	}
	defer rm2.Shutdown()

	// Wait for keys generation
	time.Sleep(500 * time.Millisecond)

	// 3. Join Follower as NonVoter
	pubKey2 := base64.StdEncoding.EncodeToString(rm2.PubKey)
	t.Logf("Joining NonVoter: %s", rm2.NodeID)

	// We join manually on Leader
	err := rm1.Join(rm2.NodeID, followerRaft, followerCluster, pubKey2, true, CurrentAppVersion, CurrentProtocolVersion, CurrentSchemaVersion)
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}

	// 4. Wait for Join to propagate
	time.Sleep(1 * time.Second)

	// Verify Initial Status on Leader
	verifySuffrage(t, rm1, rm2.NodeID, raft.Nonvoter)

	// 5. Wait for AutoConfig (monitorConfiguration) to run on Follower
	// It runs every 2s. We wait 3s.
	// Also ensure Follower knows Leader address so it can send Join request.
	// Join on Leader broadcasts NodeMeta, so Follower should receive it via Raft replication.
	// But as Nonvoter, does it receive logs? Yes.
	t.Log("Waiting for AutoConfig...")
	time.Sleep(3 * time.Second)

	// 6. Verify Status Again (Should still be Nonvoter)
	verifySuffrage(t, rm1, rm2.NodeID, raft.Nonvoter)
}

func verifySuffrage(t *testing.T, rm *RaftManager, nodeID string, expected raft.ServerSuffrage) {
	future := rm.Raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range future.Configuration().Servers {
		if s.ID == raft.ServerID(nodeID) {
			found = true
			if s.Suffrage != expected {
				t.Fatalf("Node %s suffrage mismatch: expected %v, got %v", nodeID, expected, s.Suffrage)
			}
			break
		}
	}
	if !found {
		t.Fatalf("Node %s not found in configuration", nodeID)
	}
}
