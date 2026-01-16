package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestGameStore_MetadataSidecar(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gamestore_meta_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, st)

	gameId := "test-game-meta"
	g := &Game{
		ID:            gameId,
		Status:        "active",
		SchemaVersion: SchemaVersionV3,
		ActionLog:     []json.RawMessage{json.RawMessage(`{}`)}, // Add dummy action to avoid warning
	}

	// 1. Save Game
	if err := gs.SaveGame(g); err != nil {
		t.Fatalf("SaveGame failed: %v", err)
	}

	// 2. Verify .meta.json existence
	metaPath := filepath.Join(tmpDir, "games", fmt.Sprintf("%s.meta.json", gameId))
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("Metadata file %s was not created", metaPath)
	}

	// 3. Verify Content
	relMetaPath := filepath.Join("games", fmt.Sprintf("%s.meta.json", gameId))
	var meta GameMetadata
	if err := st.ReadDataFile(relMetaPath, &meta); err != nil {
		t.Fatalf("Failed to read metadata via storage: %v", err)
	}
	if meta.ID != gameId {
		t.Errorf("Metadata ID mismatch: got %s, want %s", meta.ID, gameId)
	}

	// 4. Verify ListAllGameMetadata uses it
	// We can't easily "spy" on the internal read, but we can verify correctness
	found := false
	for md, err := range gs.ListAllGameMetadata() {
		if err != nil {
			t.Errorf("ListAllGameMetadata error: %v", err)
		}
		if md.ID == gameId {
			found = true
			if md.Status != "active" {
				t.Errorf("Metadata status mismatch: got %s, want active", md.Status)
			}
		}
	}
	if !found {
		t.Error("ListAllGameMetadata failed to return the game")
	}

	// 5. Delete Game (Tombstone)
	if err := gs.DeleteGame(gameId); err != nil {
		t.Fatal(err)
	}

	// 6. Verify Meta Tombstone
	if err := st.ReadDataFile(relMetaPath, &meta); err != nil {
		t.Fatalf("Failed to read metadata tombstone via storage: %v", err)
	}
	if meta.Status != "deleted" {
		t.Errorf("Metadata tombstone status mismatch: got %s, want deleted", meta.Status)
	}

	// 7. Purge Game
	if err := gs.PurgeGame(gameId); err != nil {
		t.Fatal(err)
	}

	// 8. Verify Meta Deleted
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Error("Metadata file was not deleted after PurgeGame")
	}
	jsonPath := filepath.Join(tmpDir, "games", fmt.Sprintf("%s.json", gameId))
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("Game JSON file was not deleted after PurgeGame")
	}
}
