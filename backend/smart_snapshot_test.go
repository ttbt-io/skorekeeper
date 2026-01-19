package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/hashicorp/raft"
)

func TestSmartSnapshot_IndexTracking(t *testing.T) {
	// Setup FSM
	tmpDir, _ := os.MkdirTemp("", "smart_snap_test")
	defer os.RemoveAll(tmpDir)

	s := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	r := NewRegistry(gs, ts)
	fsm := NewFSM(gs, ts, r, nil, s)

	// verify initial index
	if fsm.LastAppliedIndex() != 0 {
		t.Fatalf("Expected 0, got %d", fsm.LastAppliedIndex())
	}

	// Apply a log
	logEntry := &raft.Log{
		Index: 100,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  []byte(`{"type": "UNKNOWN"}`), // Unknown command returns error but we track index?
	}
	// Note: Apply checks command type. If unknown, it returns error.
	// Check if FSM updates index even on error?
	// The code: res := f.applyCommand(...); f.lastAppliedIndex.Store(l.Index); return res
	// Yes, it updates index.

	fsm.Apply(logEntry)

	if fsm.LastAppliedIndex() != 100 {
		t.Fatalf("Expected 100, got %d", fsm.LastAppliedIndex())
	}

	// Snapshot
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	defer snap.Release()

	// Check fsm_state.json
	var state map[string]any
	if err := s.ReadDataFile("fsm_state.json", &state); err != nil {
		t.Fatalf("Failed to read fsm_state.json: %v", err)
	}

	val := state["lastAppliedIndex"]
	var idx uint64
	switch v := val.(type) {
	case float64:
		idx = uint64(v)
	case int:
		idx = uint64(v)
	case int64:
		idx = uint64(v)
	case uint64:
		idx = v
	default:
		t.Errorf("lastAppliedIndex has unexpected type %T: %v", v, v)
	}

	if idx != 100 {
		t.Errorf("fsm_state.json index mismatch: expected 100, got %v (type %T)", val, val)
	}

	// Check Manifest in Tarball
	sink := &testSmartSnapshotSink{Buffer: &bytes.Buffer{}}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Restore to check manifest
	// We can't easily peek inside without un-tarring.
	// Let's use fsm.restore (which reads manifest) on a fresh FSM
	// and verify it sees the index?
	// But FSM doesn't expose the manifest it read.
	// We can modify restore to log it (already does) or trust unit test logic.
	// Let's manually inspect the tarball using FSM.restore logic?
	// Actually, Phase 2 implements checking the manifest.
	// For Phase 1, we just ensure it compiles and writes the file.
}

func TestSmartSnapshot_SkipRestore(t *testing.T) {
	// 1. Setup Local State (High Index)
	tmpDir, _ := os.MkdirTemp("", "smart_snap_skip_test")
	defer os.RemoveAll(tmpDir)

	s := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	r := NewRegistry(gs, ts)
	fsm := NewFSM(gs, ts, r, nil, s)

	// Set initialized
	fsm.setInitialized()

	// Create "Local Game A"
	gameA := &Game{ID: "gameA", ActionLog: []json.RawMessage{}}
	gs.SaveGame(gameA)

	// Write High Water Mark (Index 200)
	state := map[string]any{
		"lastAppliedIndex": 200,
		"timestamp":        123456789,
	}
	s.SaveDataFile("fsm_state.json", state)

	// 2. Create a Snapshot (Low Index)
	// We need to craft a snapshot manually or use FSM to generate one.
	// Using FSM is easier, but FSM writes *its* current state.
	// So let's create a separate FSM2 with Low Index.

	tmpDir2, _ := os.MkdirTemp("", "smart_snap_source")
	defer os.RemoveAll(tmpDir2)
	s2 := storage.New(tmpDir2, nil)
	gs2 := NewGameStore(tmpDir2, s2)
	ts2 := NewTeamStore(tmpDir2, s2)
	r2 := NewRegistry(gs2, ts2)
	fsm2 := NewFSM(gs2, ts2, r2, nil, s2)

	// Set Index 100 on FSM2
	fsm2.lastAppliedIndex.Store(100)
	fsm2.setInitialized()

	// Create "Snapshot Game B"
	gameB := &Game{ID: "gameB", ActionLog: []json.RawMessage{}}
	gs2.SaveGame(gameB)

	// Take Snapshot from FSM2
	snap, err := fsm2.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot creation failed: %v", err)
	}

	// Persist to buffer
	var buf bytes.Buffer
	sink := &testSmartSnapshotSink{Buffer: &buf}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// 3. Restore FSM1 from Snapshot
	// FSM1 has Index 200. Snapshot is Index 100.
	// Should SKIP.
	if err := fsm.Restore(io.NopCloser(&buf)); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 4. Verify
	// Game A should exist (not cleaned up)
	if _, err := gs.LoadGame("gameA"); err != nil {
		t.Errorf("Game A should still exist (Restore should have been skipped): %v", err)
	}
	// Game B should NOT exist
	if _, err := gs.LoadGame("gameB"); err == nil {
		t.Errorf("Game B should NOT exist (Restore should have been skipped)")
	}
}

func TestSmartSnapshot_FastRestore(t *testing.T) {
	// Setup FSM
	tmpDir, _ := os.MkdirTemp("", "smart_snap_fast_test")
	defer os.RemoveAll(tmpDir)

	s := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	r := NewRegistry(gs, ts)
	fsm := NewFSM(gs, ts, r, nil, s)

	// Create Games
	numGames := 10
	for i := 0; i < numGames; i++ {
		// We use gs.SaveGame directly to bypass FSM log/index but populate disk
		g := &Game{ID: fmt.Sprintf("game-%d", i), ActionLog: []json.RawMessage{}}
		gs.SaveGame(g)
	}

	// Snapshot
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	var buf bytes.Buffer
	sink := &testSmartSnapshotSink{Buffer: &buf}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// New FSM
	tmpDir2, _ := os.MkdirTemp("", "smart_snap_fast_dest")
	defer os.RemoveAll(tmpDir2)
	s2 := storage.New(tmpDir2, nil)
	gs2 := NewGameStore(tmpDir2, s2)
	ts2 := NewTeamStore(tmpDir2, s2)
	r2 := NewRegistry(gs2, ts2)
	fsm2 := NewFSM(gs2, ts2, r2, nil, s2)

	// Restore
	if err := fsm2.Restore(io.NopCloser(&buf)); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify all games exist
	for i := 0; i < numGames; i++ {
		id := fmt.Sprintf("game-%d", i)
		if _, err := gs2.LoadGame(id); err != nil {
			t.Errorf("Game %s missing after restore: %v", id, err)
		}
	}
}

type testSmartSnapshotSink struct {
	*bytes.Buffer
}

func (m *testSmartSnapshotSink) ID() string    { return "mock" }
func (m *testSmartSnapshotSink) Cancel() error { return nil }
func (m *testSmartSnapshotSink) Close() error  { return nil }
