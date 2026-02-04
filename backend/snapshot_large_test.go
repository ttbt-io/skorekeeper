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
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

func TestSnapshot_LargeDataset_Eviction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset test in short mode")
	}

	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	mk, _ := crypto.CreateAESMasterKeyForTest()

	s := storage.New(dataDir, mk)
	raftS := storage.New(raftDir, mk)

	gs := NewGameStore(dataDir, s)
	ts := NewTeamStore(dataDir, s)
	us := NewUserIndexStore(dataDir, s, mk)
	reg := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, reg, nil, raftS, us)

	// UserIndexStore cache size is 1000. We'll create 1500 items.
	numItems := 1500

	t.Logf("Generating %d items to force eviction...", numItems)

	for i := 0; i < numItems; i++ {
		uid := fmt.Sprintf("user-%d@example.com", i)
		gid := fmt.Sprintf("game-%d", i)

		// Create Game (Directly to store to avoid overhead, but we need Registry updated)
		// Registry.UpdateGame handles UserIndex.
		g := Game{ID: gid, OwnerID: uid, SchemaVersion: 3}
		gs.SaveGame(&g)
		reg.UpdateGame(g)
	}

	// Take Snapshot
	t.Log("Taking snapshot...")
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

	// Restore to new dir
	dataDir2 := t.TempDir()
	raftDir2 := filepath.Join(dataDir2, "raft")
	s2 := storage.New(dataDir2, mk)
	raftS2 := storage.New(raftDir2, mk)

	gs2 := NewGameStore(dataDir2, s2)
	ts2 := NewTeamStore(dataDir2, s2)
	us2 := NewUserIndexStore(dataDir2, s2, mk)
	reg2 := NewRegistry(gs2, ts2, us2, true)
	fsm2 := NewFSM(gs2, ts2, reg2, nil, raftS2, us2)

	_, rc, err := linkStore.Open(sink.ID())
	if err != nil {
		t.Fatalf("Open snapshot failed: %v", err)
	}
	defer rc.Close()

	if err := fsm2.restore(rc); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify
	t.Log("Verifying restored data...")

	// Check random items (start, middle, end) to ensure no gaps
	indicesToCheck := []int{0, 500, 1000, 1499}
	for _, i := range indicesToCheck {
		uid := fmt.Sprintf("user-%d@example.com", i)
		gid := fmt.Sprintf("game-%d", i)

		// Verify Game
		g, err := gs2.LoadGame(gid)
		if err != nil {
			t.Errorf("Game %s missing after restore", gid)
		} else if g.OwnerID != uid {
			t.Errorf("Game %s owner mismatch. Want %s, got %s", gid, uid, g.OwnerID)
		}

		// Verify User Index (Registry Access)
		if !reg2.HasGameAccess(uid, gid) {
			t.Errorf("User %s lost access to game %s after restore", uid, gid)
		}
	}

	// Verify Counts
	if reg2.CountTotalGames() != numItems {
		t.Errorf("Total games mismatch. Want %d, got %d", numItems, reg2.CountTotalGames())
	}
}
