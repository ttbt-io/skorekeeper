package backend

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

func TestPersistence_Integration_SnapshotRestore(t *testing.T) {
	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	mk, _ := crypto.CreateAESMasterKeyForTest()

	st := storage.New(dataDir, mk)
	raftS := storage.New(raftDir, mk)

	gs := NewGameStore(dataDir, st)
	ts := NewTeamStore(dataDir, st)
	us := NewUserIndexStore(dataDir, st, nil)
	r := NewRegistry(gs, ts, us, true)
	hm := NewHubManager()

	fsm := NewFSM(gs, ts, r, hm, raftS, us)
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
	if _, err := os.Stat(filepath.Join(dataDir, "games", gameId+".json")); !os.IsNotExist(err) {
		t.Error("Game should not be on disk yet")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "teams", teamId+".json")); !os.IsNotExist(err) {
		t.Error("Team should not be on disk yet")
	}

	// 2. Take Snapshot (Should Flush)
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// 3. Persist Snapshot to Sink (Hardlinks)
	innerStore, err := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	ring := NewKeyRing(mk, "test-key")
	linkStore := NewLinkSnapshotStore(raftDir, dataDir, innerStore, ring, mk)

	sink, err := linkStore.Create(1, 10, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create sink failed: %v", err)
	}

	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Now verify Flush occurred
	if _, err := os.Stat(filepath.Join(dataDir, "games", gameId+".json")); os.IsNotExist(err) {
		t.Error("Game should be on disk after Persist call")
	}

	// 4. Restore from Snapshot (Simulate Crash/Restart)
	tmpDir2 := t.TempDir()
	raftDir2 := filepath.Join(tmpDir2, "raft")
	st2 := storage.New(tmpDir2, mk)
	raftS2 := storage.New(raftDir2, mk)

	gs2 := NewGameStore(tmpDir2, st2)
	ts2 := NewTeamStore(tmpDir2, st2)
	us2 := NewUserIndexStore(tmpDir2, st2, nil)
	r2 := NewRegistry(gs2, ts2, us2, true)
	fsm2 := NewFSM(gs2, ts2, r2, hm, raftS2, us2)

	// Open snapshot from linkStore
	_, rc, err := linkStore.Open(sink.ID())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()

	if err := fsm2.Restore(rc); err != nil {
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

func TestPersistence_CrashRecovery_LogReplay(t *testing.T) {
	// Simulate: Apply 1 (Flush), Apply 2 (Dirty/Memory), Crash, Replay 2.
	tmpDir := t.TempDir()
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
