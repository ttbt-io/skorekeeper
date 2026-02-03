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
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

func TestLinkSnapshotStore_GC(t *testing.T) {
	dataDir := t.TempDir()
	mk, _ := crypto.CreateAESMasterKeyForTest()
	s := storage.New(dataDir, mk)

	gs := NewGameStore(dataDir, s)
	fsm := NewFSM(gs, NewTeamStore(dataDir, s), nil, nil, s, nil)

	// 1. Setup Data
	game := Game{SchemaVersion: SchemaVersionV3, ID: "game-gc", Away: "A", Home: "B"}
	if err := gs.SaveGame(&game); err != nil {
		t.Fatalf("Failed to save game: %v", err)
	}

	gamePath := filepath.Join(dataDir, "games", "game-gc.json")
	info, err := os.Stat(gamePath)
	if err != nil {
		t.Fatalf("Game file missing: %v", err)
	}
	initialMode := info.Mode()

	// 2. Setup Store with Retention=1
	innerStore, err := raft.NewFileSnapshotStore(dataDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	linkStore := NewLinkSnapshotStore(dataDir, innerStore, nil, mk)

	// 3. Create Snapshot 1
	sink1, err := linkStore.Create(1, 10, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create 1 failed: %v", err)
	}
	if err := fsm.persist(sink1); err != nil {
		t.Fatalf("Persist 1 failed: %v", err)
	}
	id1 := sink1.ID()

	// Verify hardlink exists in Snap 1
	snap1Path := filepath.Join(dataDir, "snapshots", id1, "games", "game-gc.json")
	if _, err := os.Stat(snap1Path); err != nil {
		t.Errorf("Snap 1 file missing: %v", err)
	}

	// 4. Create Snapshot 2 (Should trigger GC of Snapshot 1 due to retain=1)
	sink2, err := linkStore.Create(1, 20, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create 2 failed: %v", err)
	}
	if err := fsm.persist(sink2); err != nil {
		t.Fatalf("Persist 2 failed: %v", err)
	}
	id2 := sink2.ID()

	// 5. Verify Snap 1 is reaped
	// FileSnapshotStore reaps snapshots during Create (before returning sink).
	// With retain=1, creating Snap 2 should leave us with 2 snapshots temporarily (Snap 1 and Snap 2).
	// We force a third Create (even if cancelled) to ensure Snap 1 is reaped.

	sink3, err := linkStore.Create(1, 30, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create 3 failed: %v", err)
	}
	sink3.Cancel() // Abort 3, just wanted to trigger GC

	// Verify Snap 1 is gone
	if _, err := os.Stat(filepath.Join(dataDir, "snapshots", id1)); !os.IsNotExist(err) {
		t.Errorf("Snapshot 1 %s should have been deleted", id1)
	}

	// Verify Snap 2 is still there
	if _, err := os.Stat(filepath.Join(dataDir, "snapshots", id2)); err != nil {
		t.Errorf("Snapshot 2 %s should exist", id2)
	}

	// 6. CRITICAL: Verify Source File is still there and valid
	infoAfter, err := os.Stat(gamePath)
	if err != nil {
		t.Fatalf("Source game file deleted by GC! %v", err)
	}
	if infoAfter.Mode() != initialMode {
		t.Errorf("Source file mode changed")
	}

	// 7. Verify Snap 2 file is valid
	snap2Path := filepath.Join(dataDir, "snapshots", id2, "games", "game-gc.json")
	if _, err := os.Stat(snap2Path); err != nil {
		t.Errorf("Snap 2 file missing: %v", err)
	}
}
