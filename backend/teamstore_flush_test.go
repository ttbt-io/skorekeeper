package backend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestTeamStore_Flush(t *testing.T) {
	tmpDir := t.TempDir()
	st := storage.New(tmpDir, nil)

	ts := NewTeamStore(tmpDir, st)

	teamId := "test-team-1"
	team := &Team{
		ID:   teamId,
		Name: "Test Team",
	}

	// 1. SaveTeamInMemory (forceSync=false)
	if err := ts.SaveTeamInMemory(team, false); err != nil {
		t.Fatalf("SaveTeamInMemory failed: %v", err)
	}

	// Verify Cache
	if _, ok := ts.cache.Load(teamId); !ok {
		t.Error("Cache should contain team")
	}

	// Verify Dirty
	ts.dirtyMu.Lock()
	if !ts.dirty[teamId] {
		t.Error("Team should be marked dirty")
	}
	ts.dirtyMu.Unlock()

	// Verify Disk (Should not exist)
	path := filepath.Join(tmpDir, "teams", teamId+".json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("File should not exist on disk yet")
	}

	// 2. Flush
	if err := ts.Flush(teamId); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify Disk
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("File should exist on disk after flush")
	}

	// Verify Dirty cleared
	ts.dirtyMu.Lock()
	if ts.dirty[teamId] {
		t.Error("Team should not be dirty after flush")
	}
	ts.dirtyMu.Unlock()
}
