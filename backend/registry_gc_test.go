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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
)

func TestRegistry_GC(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "registry_gc_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gs := NewGameStore(tempDir, s)
	ts := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)

	// Create Registry without immediate rebuild
	r := NewRegistry(gs, ts, us, false)
	defer r.StopGC()

	now := time.Now()
	expiredCutoff := now.Add(-tombstoneTTL - time.Hour).UnixNano()
	freshCutoff := now.Add(-tombstoneTTL + time.Hour).UnixNano()

	// 1. Setup Expired Tombstones
	expiredGameID := "expired-game"
	expiredTeamID := "expired-team"

	gs.storage.SaveDataFile(filepath.Join("games", expiredGameID+".json"), &Game{
		ID: expiredGameID, Status: "deleted", DeletedAt: expiredCutoff, SchemaVersion: SchemaVersionV3,
	})
	gs.storage.SaveDataFile(filepath.Join("games", expiredGameID+".meta.json"), &GameMetadata{
		ID: expiredGameID, Status: "deleted", DeletedAt: expiredCutoff, SchemaVersion: SchemaVersionV3,
	})
	ts.storage.SaveDataFile(filepath.Join("teams", expiredTeamID+".json"), &Team{
		ID: expiredTeamID, Status: "deleted", DeletedAt: expiredCutoff, SchemaVersion: SchemaVersionV3,
	})

	// 2. Setup Fresh Tombstones
	freshGameID := "fresh-game"
	freshTeamID := "fresh-team"

	gs.storage.SaveDataFile(filepath.Join("games", freshGameID+".json"), &Game{
		ID: freshGameID, Status: "deleted", DeletedAt: freshCutoff, SchemaVersion: SchemaVersionV3,
	})
	gs.storage.SaveDataFile(filepath.Join("games", freshGameID+".meta.json"), &GameMetadata{
		ID: freshGameID, Status: "deleted", DeletedAt: freshCutoff, SchemaVersion: SchemaVersionV3,
	})
	ts.storage.SaveDataFile(filepath.Join("teams", freshTeamID+".json"), &Team{
		ID: freshTeamID, Status: "deleted", DeletedAt: freshCutoff, SchemaVersion: SchemaVersionV3,
	})

	// 3. Setup Active Entities
	activeGameID := "active-game"
	gs.storage.SaveDataFile(filepath.Join("games", activeGameID+".json"), &Game{
		ID: activeGameID, Status: "active", SchemaVersion: SchemaVersionV3,
	})

	// Run GC
	r.PurgeOldTombstones()

	// Verify Results
	checkExists := func(path string, shouldExist bool) {
		_, err := os.Stat(filepath.Join(tempDir, path))
		exists := !os.IsNotExist(err)
		if exists != shouldExist {
			t.Errorf("File %s exists=%v, want %v", path, exists, shouldExist)
		}
	}

	// Expired should be gone
	checkExists(filepath.Join("games", expiredGameID+".json"), false)
	checkExists(filepath.Join("games", expiredGameID+".meta.json"), false)
	checkExists(filepath.Join("teams", expiredTeamID+".json"), false)

	// Fresh should remain
	checkExists(filepath.Join("games", freshGameID+".json"), true)
	checkExists(filepath.Join("games", freshGameID+".meta.json"), true)
	checkExists(filepath.Join("teams", freshTeamID+".json"), true)

	// Active should remain
	checkExists(filepath.Join("games", activeGameID+".json"), true)
}

func TestRegistry_Rebuild_WithGC(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "registry_rebuild_gc_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gs := NewGameStore(tempDir, s)
	ts := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)

	now := time.Now()
	expiredCutoff := now.Add(-tombstoneTTL - time.Hour).UnixNano()

	expiredGameID := "expired-game-rebuild"
	gs.storage.SaveDataFile(filepath.Join("games", expiredGameID+".json"), &Game{
		ID: expiredGameID, Status: "deleted", DeletedAt: expiredCutoff, SchemaVersion: SchemaVersionV3,
	})
	gs.storage.SaveDataFile(filepath.Join("games", expiredGameID+".meta.json"), &GameMetadata{
		ID: expiredGameID, Status: "deleted", DeletedAt: expiredCutoff, SchemaVersion: SchemaVersionV3,
	})

	// Create Registry with forceRebuild=true
	r := NewRegistry(gs, ts, us, true)
	defer r.StopGC()

	// Verify expired is gone
	_, err = os.Stat(filepath.Join(tempDir, "games", expiredGameID+".json"))
	if !os.IsNotExist(err) {
		t.Errorf("Expired game should have been purged during Rebuild")
	}

	if r.CountTotalGames() != 0 {
		t.Errorf("Registry should have 0 games, got %d", r.CountTotalGames())
	}
}
