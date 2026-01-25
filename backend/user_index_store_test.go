package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
)

func TestUserIndexStore(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "userindex_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	os.MkdirAll(filepath.Join(tmpDir, "users"), 0755)

	masterKey, _ := crypto.CreateAESMasterKeyForTest()
	s := storage.New(tmpDir, masterKey)
	store := NewUserIndexStore(tmpDir, s, masterKey)

	// 1. Get New User (Should be empty)
	userId := "user@example.com"
	idx, err := store.GetUserIndex(userId)
	if err != nil {
		t.Fatalf("Get new user failed: %v", err)
	}
	if len(idx.GameAccess) != 0 || len(idx.TeamAccess) != 0 {
		t.Errorf("Expected empty index, got: %+v", idx)
	}

	// 2. Modify and Flush
	gameId := "game-123"
	idx.GameAccess[gameId] = AccessRead
	store.SetUserIndex(idx)

	// Verify Dirty
	store.dirtyMu.Lock()
	if !store.dirtyU[userId] {
		t.Error("User should be marked dirty")
	}
	store.dirtyMu.Unlock()

	// Flush
	if err := store.saveUserToDisk(userId); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Verify persisted file exists and path is hashed
	hashBytes := masterKey.Hash([]byte(userId))
	hash := hex.EncodeToString(hashBytes)
	expectedPath := filepath.Join(tmpDir, "users", hash+".json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected persisted file at %s", expectedPath)
	}

	// 3. Invalidate and Reload
	store.InvalidateUser(userId)

	// Ensure cache miss
	if _, ok := store.userCache.Get(userId); ok {
		t.Error("Cache should be empty after invalidate")
	}

	loadedIdx, err := store.GetUserIndex(userId)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if loadedIdx.GameAccess[gameId] != AccessRead {
		t.Error("Expected game access to be persisted")
	}
}

func TestUserIndexStore_NoMasterKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "userindex_nomk_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	os.MkdirAll(filepath.Join(tmpDir, "users"), 0755)

	s := storage.New(tmpDir, nil)
	store := NewUserIndexStore(tmpDir, s, nil)

	userId := "plain@example.com"
	idx, _ := store.GetUserIndex(userId)
	idx.TeamAccess["team-1"] = AccessRead
	store.SetUserIndex(idx)
	store.saveUserToDisk(userId)

	// Verify SHA256 fallback
	h := sha256.Sum256([]byte(userId))
	hash := hex.EncodeToString(h[:])
	expectedPath := filepath.Join(tmpDir, "users", hash+".json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected persisted file at %s (sha256 fallback)", expectedPath)
	}
}
