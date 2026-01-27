package backend

import (
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestGameStore_ListAllGames_IncludesDirty(t *testing.T) {
	tmpDir := t.TempDir()
	st := storage.New(tmpDir, nil)

	gs := NewGameStore(tmpDir, st)

	// 1. Create a game on DISK (Synchronous save)
	g1 := &Game{ID: "game-disk", Status: "active"}
	if err := gs.SaveGame(g1); err != nil {
		t.Fatal(err)
	}

	// 2. Create a game in MEMORY (Dirty)
	g2 := &Game{ID: "game-dirty", Status: "pending"}
	if err := gs.SaveGameInMemory(g2, false); err != nil {
		t.Fatal(err)
	}

	// 3. Verify ListAllGames finds BOTH
	foundDisk := false
	foundDirty := false

	for g, err := range gs.ListAllGames() {
		if err != nil {
			t.Fatalf("ListAllGames error: %v", err)
		}
		if g.ID == "game-disk" {
			foundDisk = true
		}
		if g.ID == "game-dirty" {
			foundDirty = true
		}
	}

	if !foundDisk {
		t.Error("Failed to find disk game")
	}
	if !foundDirty {
		t.Error("Failed to find dirty game")
	}

	// 4. Verify Metadata List also finds both
	foundDiskMeta := false
	foundDirtyMeta := false

	for md, err := range gs.ListAllGameMetadata() {
		if err != nil {
			t.Fatalf("ListAllGameMetadata error: %v", err)
		}
		if md.ID == "game-disk" {
			foundDiskMeta = true
		}
		if md.ID == "game-dirty" {
			foundDirtyMeta = true
		}
	}

	if !foundDiskMeta {
		t.Error("Failed to find disk game metadata")
	}
	if !foundDirtyMeta {
		t.Error("Failed to find dirty game metadata")
	}
}

func TestTeamStore_ListAllTeams_IncludesDirty(t *testing.T) {
	tmpDir := t.TempDir()
	st := storage.New(tmpDir, nil)

	ts := NewTeamStore(tmpDir, st)

	// 1. Create a team on DISK
	t1 := &Team{ID: "team-disk", Name: "Disk Team"}
	if err := ts.SaveTeam(t1); err != nil {
		t.Fatal(err)
	}

	// 2. Create a team in MEMORY
	t2 := &Team{ID: "team-dirty", Name: "Dirty Team"}
	if err := ts.SaveTeamInMemory(t2, false); err != nil {
		t.Fatal(err)
	}

	// 3. Verify ListAllTeams
	foundDisk := false
	foundDirty := false

	for tm, err := range ts.ListAllTeams() {
		if err != nil {
			t.Fatalf("ListAllTeams error: %v", err)
		}
		if tm.ID == "team-disk" {
			foundDisk = true
		}
		if tm.ID == "team-dirty" {
			foundDirty = true
		}
	}

	if !foundDisk {
		t.Error("Failed to find disk team")
	}
	if !foundDirty {
		t.Error("Failed to find dirty team")
	}
}
