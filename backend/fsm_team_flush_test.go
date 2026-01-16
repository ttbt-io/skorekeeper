package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestFSM_SaveTeam_DelayedPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fsm_team_delayed_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, st)
	ts := NewTeamStore(tmpDir, st)
	r := NewRegistry(gs, ts)
	hm := NewHubManager()

	fsm := NewFSM(gs, ts, r, hm, st)
	fsm.rm = &RaftManager{} // Mock Raft mode

	teamId := "test-team-fsm"
	team := &Team{
		ID:   teamId,
		Name: "FSM Team",
	}
	teamBytes, _ := json.Marshal(team)

	// applySaveTeam
	if err := fsm.applySaveTeam(teamId, teamBytes, 1); err != nil {
		t.Fatalf("applySaveTeam failed: %v", err)
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
		t.Error("File should NOT exist on disk before flush")
	}

	// Trigger Snapshot
	if _, err := fsm.Snapshot(); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Verify Disk
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("File should exist on disk after snapshot")
	}

	// Test processTeamJob batching
	teamId2 := "test-team-batch"
	team2 := &Team{ID: teamId2}
	teamBytes2, _ := json.Marshal(team2)
	raw2 := json.RawMessage(teamBytes2)

	cmd := RaftCommand{
		Type:     CmdSaveTeam,
		ID:       teamId2,
		TeamData: &raw2,
	}

	results := make([]interface{}, 1)
	j := &resourceJob{
		id:     teamId2,
		isTeam: true,
		items:  []batchItem{{index: 0, raftIndex: 2, cmd: cmd}},
	}

	fsm.processTeamJob(j, results)

	if results[0] != nil {
		t.Fatalf("processTeamJob failed: %v", results[0])
	}

	// Verify Dirty
	ts.dirtyMu.Lock()
	if !ts.dirty[teamId2] {
		t.Error("Team 2 should be marked dirty")
	}
	ts.dirtyMu.Unlock()
}
