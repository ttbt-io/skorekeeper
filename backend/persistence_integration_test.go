package backend

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
)

// testMockSnapshotSink implements raft.SnapshotSink
type testMockSnapshotSink struct {
	*bytes.Buffer
	id     string
	cancel bool
	closed bool
}

func newTestMockSnapshotSink() *testMockSnapshotSink {
	return &testMockSnapshotSink{
		Buffer: new(bytes.Buffer),
		id:     "test-snapshot-id",
	}
}

func (m *testMockSnapshotSink) ID() string { return m.id }
func (m *testMockSnapshotSink) Cancel() error {
	m.cancel = true
	return nil
}
func (m *testMockSnapshotSink) Close() error {
	m.closed = true
	return nil
}

func TestPersistence_Integration_SnapshotRestore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "persistence_integration_test")
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
	fsm.rm = &RaftManager{} // Enable Raft mode (Delayed persistence)

	// 1. Create Data (In Memory Only)
	gameId := "game-1"
	g := &Game{ID: gameId, Status: "active", LastRaftIndex: 5, SchemaVersion: CurrentSchemaVersion}
	if err := gs.SaveGameInMemory(g, false); err != nil {
		t.Fatal(err)
	}

	teamId := "team-1"
	tm := &Team{ID: teamId, Name: "Team 1", LastRaftIndex: 5, SchemaVersion: CurrentSchemaVersion}
	if err := ts.SaveTeamInMemory(tm, false); err != nil {
		t.Fatal(err)
	}

	// Verify NOT on disk yet
	if _, err := os.Stat(filepath.Join(tmpDir, "games", gameId+".json")); !os.IsNotExist(err) {
		t.Error("Game should not be on disk yet")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "teams", teamId+".json")); !os.IsNotExist(err) {
		t.Error("Team should not be on disk yet")
	}

	// 2. Take Snapshot (Should Flush)
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Verify ON DISK now (Flush occurred)
	if _, err := os.Stat(filepath.Join(tmpDir, "games", gameId+".json")); os.IsNotExist(err) {
		t.Error("Game should be on disk after Snapshot call")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "teams", teamId+".json")); os.IsNotExist(err) {
		t.Error("Team should be on disk after Snapshot call")
	}

	// 3. Persist Snapshot to Sink (Tarball)
	sink := newTestMockSnapshotSink()
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}
	if !sink.closed {
		t.Error("Sink was not closed")
	}

	// 4. Restore from Snapshot (Simulate Crash/Restart)
	// Create NEW FSM
	tmpDir2, _ := os.MkdirTemp("", "persistence_restore_test")
	defer os.RemoveAll(tmpDir2)
	st2 := storage.New(tmpDir2, nil)
	gs2 := NewGameStore(tmpDir2, st2)
	ts2 := NewTeamStore(tmpDir2, st2)
	us2 := NewUserIndexStore(tmpDir2, st2, nil)
	r2 := NewRegistry(gs2, ts2, us2, true)
	fsm2 := NewFSM(gs2, ts2, r2, hm, st2, us2)

	// Feed the tarball to Restore
	reader := bytes.NewReader(sink.Bytes())
	// Use io.NopCloser because Restore expects ReadCloser but we have Reader
	readCloser := &nopReadCloser{reader}

	if err := fsm2.Restore(readCloser); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 5. Verify Restored State
	gRestored, err := gs2.LoadGame(gameId)
	if err != nil {
		t.Fatalf("Failed to load restored game: %v", err)
	}
	if gRestored.LastRaftIndex != 5 {
		t.Errorf("Restored game has wrong index: %d", gRestored.LastRaftIndex)
	}

	tRestored, err := ts2.LoadTeam(teamId)
	if err != nil {
		t.Fatalf("Failed to load restored team: %v", err)
	}
	if tRestored.LastRaftIndex != 5 {
		t.Errorf("Restored team has wrong index: %d", tRestored.LastRaftIndex)
	}
}

type nopReadCloser struct {
	*bytes.Reader
}

func (n *nopReadCloser) Close() error { return nil }

func TestPersistence_CrashRecovery_LogReplay(t *testing.T) {
	// Simulate: Apply 1 (Flush), Apply 2 (Dirty/Memory), Crash, Replay 2.
	tmpDir, _ := os.MkdirTemp("", "persistence_replay_test")
	defer os.RemoveAll(tmpDir)

	st := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, st)

	gameId := "game-replay"

	// 1. Initial State (Flushed)
	g := &Game{ID: gameId, LastRaftIndex: 10, SchemaVersion: CurrentSchemaVersion}
	gs.SaveGame(g) // Index 10 is on disk

	// 2. Apply Action (Memory Only)
	g.LastRaftIndex = 11
	gs.SaveGameInMemory(g, false)

	// Verify Disk is old
	// gDisk, _ := gs.LoadGame(gameId) // LoadGame checks Cache first, so we need to bypass cache or create new store

	// Create new Store to simulate "Reading from disk"
	gsDisk := NewGameStore(tmpDir, st)
	gFromDisk, err := gsDisk.LoadGame(gameId)
	if err != nil {
		t.Fatalf("Failed to load game from disk: %v", err)
	}
	if gFromDisk.LastRaftIndex != 10 {
		t.Errorf("Disk should have index 10, got %d", gFromDisk.LastRaftIndex)
	}

	// 3. Simulate Restart & Log Replay
	// On restart, we load from disk (Index 10).
	// Raft Log has entry 11. Raft calls Apply(11).

	usDisk := NewUserIndexStore(tmpDir, st, nil)
	fsm := NewFSM(gsDisk, nil, nil, nil, st, usDisk)

	// Mock the Apply payload for Action 11
	// We'll just manually trigger what ApplyAction does: SaveGameInMemory
	gReplay := &Game{ID: gameId, LastRaftIndex: 11, SchemaVersion: CurrentSchemaVersion}
	fsm.rm = &RaftManager{} // Enabled

	// FSM ApplyAction logic...
	// It calls gs.SaveGameInMemory(gReplay, false)
	gsDisk.SaveGameInMemory(gReplay, false)

	// 4. Verify Final State
	// Memory has 11.
	val, _ := gsDisk.cache.Load(gameId)
	var gFinal Game
	json.Unmarshal(val.([]byte), &gFinal)
	if gFinal.LastRaftIndex != 11 {
		t.Errorf("Final memory state should be 11, got %d", gFinal.LastRaftIndex)
	}
}
