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
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/hashicorp/raft"
)

func TestFSMInitialization(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "fsm_init_test")
	defer os.RemoveAll(tempDir)
	s := storage.New(tempDir, nil)
	us := NewUserIndexStore(tempDir, s, nil)

	// Create FSM
	fsm := NewFSM(nil, nil, nil, nil, s, us)

	// Create a mock RaftManager for the FSM to get the selfID

	// Use a real RaftManager to ensure NodeID is derived correctly

	raftDir := filepath.Join(tempDir, "raft")

	rm := NewRaftManager(raftDir, "127.0.0.1:0", "127.0.0.1:0", "127.0.0.1:0", "127.0.0.1:0", "secret", nil, fsm)

	fsm.rm = rm // Link FSM to its RM

	// Start RM to derive NodeID and ensure it's set
	if err := rm.Start(false); err != nil {
		t.Fatalf("Failed to start mock RaftManager: %v", err)
	}
	defer rm.Shutdown()

	selfNodeID := rm.NodeID

	if fsm.IsInitialized() {
		t.Error("FSM should not be initialized initially")
	}

	// Apply metadata for self (using the derived selfNodeID)
	metaSelf := NodeMeta{NodeID: selfNodeID, HttpAddr: "127.0.0.1:8080", AppVersion: CurrentAppVersion, ProtocolVersion: CurrentProtocolVersion, SchemaVersion: CurrentSchemaVersion}
	cmdSelf := RaftCommand{
		Type:     CmdNodeMeta,
		NodeMeta: &metaSelf,
	}
	cmdSelfBytes, _ := json.Marshal(cmdSelf)

	fsm.Apply(&raft.Log{Data: cmdSelfBytes})

	if fsm.IsInitialized() {
		t.Error("FSM should not be initialized after applying self metadata")
	}

	// Apply metadata for another node
	otherNodeID := "node-other-manual" // Use a distinct ID
	metaOther := NodeMeta{NodeID: otherNodeID, HttpAddr: "127.0.0.1:8081", AppVersion: CurrentAppVersion, ProtocolVersion: CurrentProtocolVersion, SchemaVersion: CurrentSchemaVersion}
	cmdOther := RaftCommand{
		Type:     CmdNodeMeta,
		NodeMeta: &metaOther,
	}
	cmdOtherBytes, _ := json.Marshal(cmdOther)

	fsm.Apply(&raft.Log{Data: cmdOtherBytes})

	if !fsm.IsInitialized() {
		t.Error("FSM should be initialized after applying other node metadata")
	}
}

func TestFSMSnapshotInitialization(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "snapshot_init_test")
	defer os.RemoveAll(tempDir)
	s := storage.New(tempDir, nil)
	gs := NewGameStore(tempDir, s)
	ts := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)

	fsm := NewFSM(gs, ts, reg, nil, s, us)
	fsm.initialized.Store(true)
	fsm.nodeMap.Store("node-1", &NodeMeta{NodeID: "node-1"})

	// Snapshot
	snap, _ := fsm.Snapshot()
	sink := &mockSnapshotSink{Buffer: &bytes.Buffer{}}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Restore
	tempDir2, _ := os.MkdirTemp("", "snapshot_init_test_2")
	defer os.RemoveAll(tempDir2)
	s2 := storage.New(tempDir2, nil)
	gs2 := NewGameStore(tempDir2, s2)
	ts2 := NewTeamStore(tempDir2, s2)
	us2 := NewUserIndexStore(tempDir2, s2, nil)
	reg2 := NewRegistry(gs2, ts2, us2, true)

	fsm2 := NewFSM(gs2, ts2, reg2, nil, s2, us2)
	if err := fsm2.Restore(&mockReadCloser{sink.Buffer}); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	if !fsm2.IsInitialized() {
		t.Error("FSM should be initialized after restore")
	}
}

type mockSnapshotSink struct {
	*bytes.Buffer
}

func (m *mockSnapshotSink) Write(p []byte) (n int, err error) { return m.Buffer.Write(p) }
func (m *mockSnapshotSink) Close() error                      { return nil }
func (m *mockSnapshotSink) ID() string                        { return "id" }
func (m *mockSnapshotSink) Cancel() error                     { return nil }

type mockReadCloser struct {
	*bytes.Buffer
}

func (m *mockReadCloser) Close() error { return nil }
