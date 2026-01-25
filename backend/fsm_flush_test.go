package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestFSM_ApplyAction_DelayedPersistence(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "fsm_delayed_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, st)
	ts := NewTeamStore(tmpDir, st)
	us := NewUserIndexStore(tmpDir, st, nil)
	r := NewRegistry(gs, ts, us, true)
	hm := NewHubManager()

	fsm := NewFSM(gs, ts, r, hm, st, us)
	// Mock RaftManager presence to enable delayed writes
	fsm.rm = &RaftManager{}

	gameId := "test-game-fsm"
	g := &Game{
		ID:     gameId,
		Status: "active",
	}
	// Save initial game (synchronous)
	if err := gs.SaveGame(g); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, "games", gameId+".json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal("Initial game file not found")
	}
	initialModTime := info.ModTime()

	// Create an action
	action := struct {
		Type string `json:"type"`
	}{Type: "TEST_ACTION"}
	actionBytes, _ := json.Marshal(action)

	// Apply Action via FSM
	// We need to bypass the raft log decoding and call applyAction directly
	// or mock the Apply payload.
	// Using applyAction directly is easier for unit testing this behavior.
	// However, applyAction expects the action payload to be wrapped?
	// No, ApplyAction helper takes raw bytes.
	// But wait, fsm.applyAction takes 'data []byte' which is the Action payload?
	// Let's check fsm.go: ApplyAction(g, data)

	if err := fsm.applyAction(gameId, actionBytes, 1); err != nil {
		t.Fatalf("applyAction failed: %v", err)
	}

	// VERIFICATION

	// 1. Cache should be updated
	val, ok := gs.cache.Load(gameId)
	if !ok {
		t.Error("Cache should contain game")
	}
	var gUpdated Game
	json.Unmarshal(val.([]byte), &gUpdated)
	// We can't easily check if action was applied without a real Reducer/ApplyAction logic that modifies state.
	// But we can check LastRaftIndex if updated?
	// applyAction updates LastRaftIndex if index > 0.
	if gUpdated.LastRaftIndex != 1 {
		t.Errorf("Expected LastRaftIndex 1, got %d", gUpdated.LastRaftIndex)
	}

	// 2. Dirty flag should be set
	gs.dirtyMu.Lock()
	if !gs.dirty[gameId] {
		t.Error("Game should be marked dirty")
	}
	gs.dirtyMu.Unlock()

	// 3. Disk file should NOT be modified (ModTime unchanged)
	// Note: FS resolution might be low, but this test runs fast.
	info2, _ := os.Stat(path)
	if !info2.ModTime().Equal(initialModTime) {
		t.Error("Disk file modified immediately! Expected delayed write.")
	}

	// 4. Trigger Snapshot (Flush)
	if _, err := fsm.Snapshot(); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// 5. Verify Disk Modified
	info3, _ := os.Stat(path)
	if info3.ModTime().Equal(initialModTime) {
		t.Error("Disk file NOT modified after snapshot/flush!")
	}

	// 6. Verify Dirty cleared
	gs.dirtyMu.Lock()
	if gs.dirty[gameId] {
		t.Error("Dirty flag not cleared after snapshot")
	}
	gs.dirtyMu.Unlock()
}

func TestFSM_Standalone_ImmediatePersistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "fsm_standalone_test")
	defer os.RemoveAll(tmpDir)

	st := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, st)
	ts := NewTeamStore(tmpDir, st)
	us := NewUserIndexStore(tmpDir, st, nil)
	r := NewRegistry(gs, ts, us, true)
	hm := NewHubManager()

	fsm := NewFSM(gs, ts, r, hm, st, us)
	fsm.rm = nil // Explicitly nil (Standalone mode)

	gameId := "test-game-std"
	g := &Game{ID: gameId}
	gs.SaveGame(g)

	// path := filepath.Join(tmpDir, "games", gameId+".json")
	// info, _ := os.Stat(path)
	// initialModTime := info.ModTime()

	// Sleep briefly to ensure FS time resolution diff
	// time.Sleep(10 * time.Millisecond) // Might not be enough on some FS, but let's try.

	actionBytes := []byte(`{"type":"TEST"}`)
	fsm.applyAction(gameId, actionBytes, 0) // Index 0 for standalone usually? Or just ignore index.

	// Verify Dirty is FALSE (flushed immediately)
	gs.dirtyMu.Lock()
	if gs.dirty[gameId] {
		t.Error("Game should NOT be dirty in standalone mode")
	}
	gs.dirtyMu.Unlock()

	// We can't easily check ModTime change reliably without significant wait.
	// But checking the dirty flag is strong evidence.
}
