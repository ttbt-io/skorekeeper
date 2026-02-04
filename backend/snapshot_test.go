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
	"io"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

func TestFSMSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	mk, _ := crypto.CreateAESMasterKeyForTest()

	s := storage.New(dataDir, mk)

	gs := NewGameStore(dataDir, s)
	ts := NewTeamStore(dataDir, s)
	us := NewUserIndexStore(dataDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, reg, nil, s, us)
	// 1. Add some data
	gameId := "game-1"
	game := Game{SchemaVersion: SchemaVersionV3, ID: gameId, Away: "A", Home: "B"}
	gs.SaveGame(&game)

	teamId := "team-1"
	team := Team{SchemaVersion: SchemaVersionV3, ID: teamId, Name: "Team One"}
	ts.SaveTeam(&team)

	fsm.nodeMap.Store("node-1", &NodeMeta{NodeID: "node-1", HttpAddr: "127.0.0.1:8080"})

	// 2. Snapshot using LinkSnapshotStore
	innerStore, err := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	linkStore := NewLinkSnapshotStore(raftDir, dataDir, innerStore, nil, mk)

	sink, err := linkStore.Create(1, 10, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create sink failed: %v", err)
	}

	if err := fsm.persist(sink); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	// persist closes the sink

	// 3. Restore to new dir

	dataDir2 := t.TempDir()

	s2 := storage.New(dataDir2, mk)

	gs2 := NewGameStore(dataDir2, s2)

	ts2 := NewTeamStore(dataDir2, s2)
	us2 := NewUserIndexStore(dataDir2, s2, nil)
	reg2 := NewRegistry(gs2, ts2, us2, true)
	fsm2 := NewFSM(gs2, ts2, reg2, nil, s2, us2)
	_, rc, err := linkStore.Open(sink.ID())
	if err != nil {
		t.Fatalf("Open snapshot failed: %v", err)
	}
	defer rc.Close()

	if err := fsm2.restore(rc); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 4. Verify
	g2, err := gs2.LoadGame(gameId)
	if err != nil {
		t.Fatalf("Game not found after restore: %v", err)
	}
	if g2.ID != gameId || g2.Away != "A" {
		t.Errorf("Game data mismatch. Expected %+v, got %+v", game, g2)
	}

	t2, err := ts2.LoadTeam(teamId)
	if err != nil {
		t.Fatalf("Team not found after restore: %v", err)
	}
	if t2.ID != teamId || t2.Name != "Team One" {
		t.Errorf("Team data mismatch. Expected %+v, got %+v", team, t2)
	}

	addr := fsm2.GetNodeAddr("node-1")
	if addr != "127.0.0.1:8080" {
		t.Errorf("NodeAddr mismatch. Expected 127.0.0.1:8080, got %s", addr)
	}
}
