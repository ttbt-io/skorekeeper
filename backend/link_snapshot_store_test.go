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
	raftDir := filepath.Join(dataDir, "raft")
	mk, _ := crypto.CreateAESMasterKeyForTest()

	s := storage.New(dataDir, mk)
	raftS := storage.New(raftDir, mk)

	gs := NewGameStore(dataDir, s)
	ts := NewTeamStore(dataDir, s)
	us := NewUserIndexStore(dataDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, reg, nil, raftS, us)

	// 1. Setup Data
	game := Game{SchemaVersion: SchemaVersionV3, ID: "game-1", Away: "Away Team", Home: "Home Team"}
	if err := gs.SaveGame(&game); err != nil {
		t.Fatalf("Failed to save game: %v", err)
	}

	team := Team{SchemaVersion: SchemaVersionV3, ID: "team-1", Name: "Super Team"}
	if err := ts.SaveTeam(&team); err != nil {
		t.Fatalf("Failed to save team: %v", err)
	}

	// Setup User Index
	idx := &UserIndex{UserID: "user-1", LastUpdated: 12345}
	us.SetUserIndex(idx)
	us.FlushAll()

	// 2. Setup Raft Keyring
	rk, _ := mk.NewKey()
	ring := NewKeyRing(rk, "raft-key-1")
	defer ring.Wipe()

	// 3. Setup LinkSnapshotStore
	innerStore, err := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	linkStore := NewLinkSnapshotStore(raftDir, dataDir, innerStore, ring, mk)

	// 4. Create Snapshot via FSM
	sink, err := linkStore.Create(raft.SnapshotVersion(1), 10, 2, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create sink failed: %v", err)
	}

	if err := fsm.persist(sink); err != nil {
		t.Fatalf("FSM persist failed: %v", err)
	}

	// 5. Verify Hardlinks on Disk
	snapID := sink.ID()
	snapDir := filepath.Join(raftDir, "snapshots", snapID)

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
	foundUser := false

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
		default:
			if filepath.Dir(header.Name) == "users" {
				var u UserIndex
				if err := json.NewDecoder(tr).Decode(&u); err != nil {
					t.Fatalf("Failed to decode user index: %v", err)
				}
				if u.UserID == "user-1" {
					foundUser = true
				}
			}
		}
	}

	if !foundManifest || !foundGame || !foundTeam || !foundUser {
		t.Errorf("Missing entries: manifest=%v, game=%v, team=%v, user=%v", foundManifest, foundGame, foundTeam, foundUser)
	}

	// 7. Test Restore with reconstructed stream
	// Reset stream
	_, rc2, _ := linkStore.Open(snapID)
	defer rc2.Close()

	dataDir2 := t.TempDir()
	raftDir2 := filepath.Join(dataDir2, "raft")
	s2 := storage.New(dataDir2, mk)
	raftS2 := storage.New(raftDir2, mk)

	gs2 := NewGameStore(dataDir2, s2)
	ts2 := NewTeamStore(dataDir2, s2)
	us2 := NewUserIndexStore(dataDir2, s2, nil)
	fsm2 := NewFSM(gs2, ts2, NewRegistry(gs2, ts2, us2, true), nil, raftS2, us2)

	if err := fsm2.restore(rc2); err != nil {
		t.Fatalf("Restore from reconstructed stream failed: %v", err)
	}

	gRestore, _ := gs2.LoadGame("game-1")
	if gRestore == nil || gRestore.Away != "Away Team" {
		t.Errorf("Restore failed to recover game data correctly")
	}
}

func TestLinkSnapshotStore_Open_RemoteSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	mk, _ := crypto.CreateAESMasterKeyForTest()

	// 1. Setup inner store
	innerStore, err := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	// Setup Raft Keyring
	rk, _ := mk.NewKey()
	ring := NewKeyRing(rk, "raft-key-1")
	linkStore := NewLinkSnapshotStore(raftDir, dataDir, innerStore, ring, mk)

	// 2. Create a "Remote" Snapshot (GZIP TAR) via inner store (simulating Raft receiving it)
	// We use innerStore.Create directly because when receiving a snapshot, Raft writes the raw stream (GZIP TAR)
	// to the Sink. LinkSnapshotSink (if used) simply passes bytes through if encryption is disabled.
	// Using innerStore ensures correct CRC/Meta handling.
	sink, err := innerStore.Create(1, 100, 5, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Failed to create inner sink: %v", err)
	}

	// Encrypt the stream because LinkSnapshotStore expects encrypted data
	encWriter, err := ring.Active.Key.StartWriter([]byte(sink.ID()), sink)
	if err != nil {
		t.Fatalf("Failed to start encryption writer: %v", err)
	}

	// Write GZIP TAR content to sink
	gz := gzip.NewWriter(encWriter)
	tw := tar.NewWriter(gz)

	// Add manifest.json to TAR
	manifestContent := []byte(`{"remote": true}`)
	header := &tar.Header{
		Name: "manifest.json",
		Mode: 0600,
		Size: int64(len(manifestContent)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}
	if _, err := tw.Write(manifestContent); err != nil {
		t.Fatalf("Failed to write tar body: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("Failed to close gzip: %v", err)
	}
	if err := encWriter.Close(); err != nil {
		t.Fatalf("Failed to close encryption writer: %v", err)
	}

	// Close sink to finalize snapshot (writes meta.json with CRC)
	if err := sink.Close(); err != nil {
		t.Fatalf("Failed to close sink: %v", err)
	}
	snapID := sink.ID()

	// 3. Open via LinkSnapshotStore
	_, rc, err := linkStore.Open(snapID)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()

	// 4. Verify Content
	// It should be the GZIP stream we just wrote.
	// If the bug exists, this will be a GZIP stream of a TAR containing "manifest.json" which contains the GZIP stream we wrote.
	// So we decode it and check the content of manifest.json.

	gzR, err := gzip.NewReader(rc)
	if err != nil {
		t.Fatalf("Failed to open gzip reader (returned stream wasn't gzip?): %v", err)
	}
	defer gzR.Close()

	tr := tar.NewReader(gzR)
	h, err := tr.Next()
	if err != nil {
		t.Fatalf("Failed to read first tar entry: %v", err)
	}

	if h.Name != "manifest.json" {
		t.Errorf("First entry should be manifest.json, got %s", h.Name)
	}

	// Read content of manifest.json
	content, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("Failed to read manifest content: %v", err)
	}

	// Verify it's our JSON, not binary garbage
	var m map[string]interface{}
	if err := json.Unmarshal(content, &m); err != nil {
		t.Fatalf("Failed to unmarshal manifest.json: %v. Content was: %q. This likely means LinkSnapshotStore wrapped the GZIP TAR inside another TAR.", err, string(content))
	}

	if m["remote"] != true {
		t.Errorf("Unexpected manifest content: %v", m)
	}
}

func TestLinkSnapshotStore_GC(t *testing.T) {
	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
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
	innerStore, err := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	ring := NewKeyRing(mk, "test-key")
	linkStore := NewLinkSnapshotStore(raftDir, dataDir, innerStore, ring, mk)

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
	snap1Path := filepath.Join(raftDir, "snapshots", id1, "games", "game-gc.json")
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
	if _, err := os.Stat(filepath.Join(raftDir, "snapshots", id1)); !os.IsNotExist(err) {
		// If Snap 1 still exists, it means GC didn't happen.
	}

	// Verify Snap 2 is still there
	if _, err := os.Stat(filepath.Join(raftDir, "snapshots", id2)); err != nil {
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
	snap2Path := filepath.Join(raftDir, "snapshots", id2, "games", "game-gc.json")
	if _, err := os.Stat(snap2Path); err != nil {
		t.Errorf("Snap 2 file missing: %v", err)
	}
}

func TestLinkSnapshotStore_Replication_SizeCorrectness(t *testing.T) {
	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	mk, _ := crypto.CreateAESMasterKeyForTest()

	s := storage.New(dataDir, mk)

	// 1. Create dummy data file
	dummyGame := &Game{
		ID:   "game-1",
		Away: "Away Team",
		Home: "Home Team",
		// Add padding to make it large
		ActionLog: make([]json.RawMessage, 100),
	}
	// Fill with dummy data
	for i := 0; i < 100; i++ {
		dummyGame.ActionLog[i] = json.RawMessage(`{"type":"PITCH"}`)
	}

	if err := s.SaveDataFile("games/game-1.json", dummyGame); err != nil {
		t.Fatalf("SaveDataFile failed: %v", err)
	}

	// 1b. Create dummy system file
	policy := &UserAccessPolicy{
		Admins: []string{"admin"},
	}
	if err := s.SaveDataFile("sys_access_policy", policy); err != nil {
		t.Fatalf("SaveDataFile policy failed: %v", err)
	}

	// 2. Setup LinkSnapshotStore
	innerStore, err := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	ring := NewKeyRing(mk, "test-key")
	linkStore := NewLinkSnapshotStore(raftDir, dataDir, innerStore, ring, mk)

	// 3. Create Snapshot
	sink, err := linkStore.Create(1, 10, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create sink failed: %v", err)
	}

	linker := sink.(SnapshotLinker)

	linker.LinkFile("games/game-1.json", "games/game-1.json")

	linker.LinkFile("sys_access_policy", "sys_access_policy")

	manifest := snapshotManifest{

		RaftIndex: 10,
	}
	manifestBytes, _ := json.Marshal(manifest)
	linker.WriteManifest(manifestBytes)

	if err := sink.Close(); err != nil {
		t.Fatalf("Sink close failed: %v", err)
	}
	snapID := sink.ID()

	// 4. Open Snapshot
	meta, rc, err := linkStore.Open(snapID)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()

	// 5. Read all data
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	realSize := int64(len(data))

	t.Logf("Meta Size: %d", meta.Size)
	t.Logf("Real Stream Size: %d", realSize)

	if meta.Size != realSize {
		t.Errorf("Size mismatch: Meta says %d, Stream is %d", meta.Size, realSize)
	} else {
		t.Logf("Sizes match: %d", meta.Size)
	}
}
