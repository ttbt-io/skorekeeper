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
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

func TestLinkSnapshotStore_EndToEnd(t *testing.T) {
	dataDir := t.TempDir()
	mk, _ := crypto.CreateAESMasterKeyForTest()
	s := storage.New(dataDir, mk)

	gs := NewGameStore(dataDir, s)
	ts := NewTeamStore(dataDir, s)
	us := NewUserIndexStore(dataDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, reg, nil, s, us)

	// 1. Setup Data
	game := Game{SchemaVersion: SchemaVersionV3, ID: "game-1", Away: "Away Team", Home: "Home Team"}
	if err := gs.SaveGame(&game); err != nil {
		t.Fatalf("Failed to save game: %v", err)
	}

	team := Team{SchemaVersion: SchemaVersionV3, ID: "team-1", Name: "Super Team"}
	if err := ts.SaveTeam(&team); err != nil {
		t.Fatalf("Failed to save team: %v", err)
	}

	// 2. Setup Raft Keyring
	rk, _ := mk.NewKey()
	ring := NewKeyRing(rk, "raft-key-1")
	defer ring.Wipe()

	// 3. Setup LinkSnapshotStore
	innerStore, err := raft.NewFileSnapshotStore(dataDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	linkStore := NewLinkSnapshotStore(dataDir, innerStore, ring, mk)

	// 4. Create Snapshot via FSM
	sink, err := linkStore.Create(raft.SnapshotVersion(1), 10, 2, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create sink failed: %v", err)
	}

	if err := fsm.persist(sink); err != nil {
		t.Fatalf("FSM persist failed: %v", err)
	}
	// persist calls sink.Close() because of the defer I added

	// 5. Verify Hardlinks on Disk
	snapID := sink.ID()
	snapDir := filepath.Join(dataDir, "snapshots", snapID)

	gamePath := filepath.Join(snapDir, "games", "game-1.json")
	if _, err := os.Stat(gamePath); err != nil {
		t.Errorf("Linked game file not found: %v", err)
	}

	teamPath := filepath.Join(snapDir, "teams", "team-1.json")
	if _, err := os.Stat(teamPath); err != nil {
		t.Errorf("Linked team file not found: %v", err)
	}

	// Verify they are indeed hardlinks (same Inode)
	fi1, _ := os.Stat(filepath.Join(dataDir, "games", "game-1.json"))
	fi2, _ := os.Stat(gamePath)
	if os.SameFile(fi1, fi2) == false {
		t.Errorf("Game file in snapshot is not a hardlink")
	}

	// 6. Open Snapshot and Verify Tar Stream
	_, rc, err := linkStore.Open(snapID)
	if err != nil {
		t.Fatalf("Open snapshot failed: %v", err)
	}
	defer rc.Close()

	gz, err := gzip.NewReader(rc)
	if err != nil {
		t.Fatalf("Gzip reader failed: %v", err)
	}
	tr := tar.NewReader(gz)

	foundManifest := false
	foundGame := false
	foundTeam := false

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Tar error: %v", err)
		}

		switch header.Name {
		case "manifest.json":
			foundManifest = true
			var m snapshotManifest
			if err := json.NewDecoder(tr).Decode(&m); err != nil {
				t.Fatalf("Failed to decode manifest: %v", err)
			}
			if m.RaftIndex != fsm.LastAppliedIndex() {
				t.Errorf("Manifest index mismatch. Got %d, want %d", m.RaftIndex, fsm.LastAppliedIndex())
			}
		case "games/game-1.json":
			foundGame = true
			var g Game
			if err := json.NewDecoder(tr).Decode(&g); err != nil {
				t.Fatalf("Failed to decode game from tar: %v", err)
			}
			if g.ID != "game-1" || g.Away != "Away Team" {
				t.Errorf("Decoded game mismatch: %+v", g)
			}
		case "teams/team-1.json":
			foundTeam = true
			var t2 Team
			if err := json.NewDecoder(tr).Decode(&t2); err != nil {
				t.Fatalf("Failed to decode team from tar: %v", err)
			}
			if t2.ID != "team-1" || t2.Name != "Super Team" {
				t.Errorf("Decoded team mismatch: %+v", t2)
			}
		}
	}

	if !foundManifest || !foundGame || !foundTeam {
		t.Errorf("Missing entries in tar stream: manifest=%v, game=%v, team=%v", foundManifest, foundGame, foundTeam)
	}

	// 7. Test Restore with reconstructed stream
	// Reset stream
	_, rc2, _ := linkStore.Open(snapID)
	defer rc2.Close()

	dataDir2 := t.TempDir()
	s2 := storage.New(dataDir2, mk)
	gs2 := NewGameStore(dataDir2, s2)
	ts2 := NewTeamStore(dataDir2, s2)
	us2 := NewUserIndexStore(dataDir2, s2, nil)
	fsm2 := NewFSM(gs2, ts2, NewRegistry(gs2, ts2, us2, true), nil, s2, us2)

	if err := fsm2.restore(rc2); err != nil {
		t.Fatalf("Restore from reconstructed stream failed: %v", err)
	}

	gRestore, _ := gs2.LoadGame("game-1")
	if gRestore == nil || gRestore.Away != "Away Team" {
		t.Errorf("Restore failed to recover game data correctly")
	}
}
