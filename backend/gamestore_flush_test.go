package backend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestGameStore_Flush(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "gamestore_flush_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, st)

	gameId := "test-game-1"
	g := &Game{
		ID:     gameId,
		Status: "active",
	}

	// 1. Test SaveGameInMemory (forceSync=false)
	if err := gs.SaveGameInMemory(g, false); err != nil {
		t.Fatalf("SaveGameInMemory failed: %v", err)
	}

	// Verify Cache has it
	if _, ok := gs.cache.Load(gameId); !ok {
		t.Error("Cache should contain game")
	}

	// Verify Dirty
	gs.dirtyMu.Lock()
	if !gs.dirty[gameId] {
		t.Error("Game should be marked dirty")
	}
	gs.dirtyMu.Unlock()

	// Verify Disk DOES NOT have it
	encodedId := "test-game-1" // url encoded is same
	path := filepath.Join(tmpDir, "games", encodedId+".json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("File should not exist on disk yet")
	}

	// 2. Test Flush
	if err := gs.Flush(gameId); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify Disk HAS it
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("File should exist on disk after flush")
	}

	// Verify Dirty cleared
	gs.dirtyMu.Lock()
	if gs.dirty[gameId] {
		t.Error("Game should not be dirty after flush")
	}
	gs.dirtyMu.Unlock()

	// 3. Test FlushAll
	g2 := &Game{ID: "test-game-2"}
	g3 := &Game{ID: "test-game-3"}
	gs.SaveGameInMemory(g2, false)
	gs.SaveGameInMemory(g3, false)

	if err := gs.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	path2 := filepath.Join(tmpDir, "games", "test-game-2.json")
	if _, err := os.Stat(path2); os.IsNotExist(err) {
		t.Error("Game 2 should exist on disk")
	}
	path3 := filepath.Join(tmpDir, "games", "test-game-3.json")
	if _, err := os.Stat(path3); os.IsNotExist(err) {
		t.Error("Game 3 should exist on disk")
	}
}

func TestGameStore_SaveGame_ClearsDirty(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "gamestore_flush_test_2")
	defer os.RemoveAll(tmpDir)
	st := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, st)

	gameId := "test-game-dirty"
	g := &Game{ID: gameId}

	// Mark dirty
	gs.SaveGameInMemory(g, false)
	gs.dirtyMu.Lock()
	if !gs.dirty[gameId] {
		t.Fatal("Should be dirty")
	}
	gs.dirtyMu.Unlock()

	// Direct SaveGame
	if err := gs.SaveGame(g); err != nil {
		t.Fatal(err)
	}

	// Verify not dirty
	gs.dirtyMu.Lock()
	if gs.dirty[gameId] {
		t.Error("SaveGame should clear dirty flag")
	}
	gs.dirtyMu.Unlock()
}
