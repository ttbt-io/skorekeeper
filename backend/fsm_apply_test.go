// Copyright (c) 2026 TTBT Enterprises LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/hashicorp/raft"
)

func TestFSMApply(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "fsm_test")
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gs := NewGameStore(tempDir, s)
	ts := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)
	hm := NewHubManager()
	fsm := NewFSM(gs, ts, reg, hm, s, us)

	// 1. SaveGame
	game := Game{ID: "game-fsm", SchemaVersion: SchemaVersionV3, OwnerID: "owner@example.com"}
	gBytes, _ := json.Marshal(game)
	rawGame := json.RawMessage(gBytes)
	cmd := RaftCommand{Type: CmdSaveGame, GameData: &rawGame}
	cmdBytes, _ := json.Marshal(cmd)
	if resp := fsm.Apply(&raft.Log{Data: cmdBytes}); resp != nil {
		if err, ok := resp.(error); ok && err != nil {
			t.Fatalf("Apply SaveGame failed: %v", err)
		}
	}

	// Verify
	loaded, err := gs.LoadGame("game-fsm")
	if err != nil {
		t.Fatalf("LoadGame failed: %v", err)
	}
	if loaded.OwnerID != "owner@example.com" {
		t.Errorf("OwnerID mismatch")
	}

	// 2. SaveTeam
	team := Team{ID: "team-fsm", SchemaVersion: SchemaVersionV3, OwnerID: "owner@example.com"}
	tBytes, _ := json.Marshal(team)
	rawTeam := json.RawMessage(tBytes)
	cmdTeam := RaftCommand{Type: CmdSaveTeam, TeamData: &rawTeam}
	cmdTeamBytes, _ := json.Marshal(cmdTeam)
	if resp := fsm.Apply(&raft.Log{Data: cmdTeamBytes}); resp != nil {
		if err, ok := resp.(error); ok && err != nil {
			t.Fatalf("Apply SaveTeam failed: %v", err)
		}
	}

	// Verify
	loadedTeam, err := ts.LoadTeam("team-fsm")
	if err != nil {
		t.Fatalf("LoadTeam failed: %v", err)
	}
	if loadedTeam.OwnerID != "owner@example.com" {
		t.Errorf("OwnerID mismatch")
	}

	// 3. Action
	action := BaseAction{ID: "action-1", Type: "PITCH", Payload: json.RawMessage(`{"type":"ball"}`)}
	actionBytes, _ := json.Marshal(action)

	cmdAction := RaftCommand{
		Type: CmdApplyAction,
		Action: &ActionPayload{
			GameID: "game-fsm",
			Action: json.RawMessage(actionBytes),
			UserID: "user@example.com",
		},
	}
	cmdActionBytes, _ := json.Marshal(cmdAction)
	if resp := fsm.Apply(&raft.Log{Data: cmdActionBytes}); resp != nil {
		if err, ok := resp.(error); ok && err != nil {
			t.Fatalf("Apply Action failed: %v", err)
		}
	}

	// Verify Action Log
	loaded, _ = gs.LoadGame("game-fsm")
	if len(loaded.ActionLog) != 1 {
		t.Errorf("Expected 1 action, got %d", len(loaded.ActionLog))
	}

	// 4. Batch Action
	action2 := BaseAction{ID: "action-2", Type: "PITCH", Payload: json.RawMessage(`{"type":"strike"}`)}
	action2Bytes, _ := json.Marshal(action2)
	batch := []json.RawMessage{json.RawMessage(action2Bytes)}

	cmdBatch := RaftCommand{
		Type: CmdApplyAction,
		Action: &ActionPayload{
			GameID:  "game-fsm",
			Actions: batch,
			UserID:  "user@example.com",
		},
	}
	cmdBatchBytes, _ := json.Marshal(cmdBatch)
	fsm.Apply(&raft.Log{Data: cmdBatchBytes})

	loaded, _ = gs.LoadGame("game-fsm")
	if len(loaded.ActionLog) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(loaded.ActionLog))
	}

	// 5. DeleteGame
	cmdDel := RaftCommand{Type: CmdDeleteGame, ID: "game-fsm"}
	cmdDelBytes, _ := json.Marshal(cmdDel)
	fsm.Apply(&raft.Log{Data: cmdDelBytes})

	loaded, _ = gs.LoadGame("game-fsm")
	if loaded.Status != "deleted" {
		t.Error("Game should be deleted")
	}

	// 6. DeleteTeam
	cmdDelTeam := RaftCommand{Type: CmdDeleteTeam, ID: "team-fsm"}
	cmdDelTeamBytes, _ := json.Marshal(cmdDelTeam)
	fsm.Apply(&raft.Log{Data: cmdDelTeamBytes})

	loadedTeam, _ = ts.LoadTeam("team-fsm")
	if loadedTeam.Status != "deleted" {
		t.Error("Team should be deleted")
	}

	// 7. Update Access Policy
	policy := &UserAccessPolicy{DefaultMaxGames: 99}
	cmdPolicy := RaftCommand{Type: CmdUpdateAccessPolicy, PolicyData: policy}
	cmdPolicyBytes, _ := json.Marshal(cmdPolicy)
	fsm.Apply(&raft.Log{Data: cmdPolicyBytes})

	if reg.GetAccessPolicy().DefaultMaxGames != 99 {
		t.Error("Access policy not updated")
	}

	// 8. Batch Team Actions (SaveTeam)
	team2 := Team{ID: "team-batch", SchemaVersion: SchemaVersionV3, OwnerID: "owner@example.com"}
	team2Bytes, _ := json.Marshal(team2)
	rawTeam2 := json.RawMessage(team2Bytes)

	cmdBatchTeam := RaftCommand{Type: CmdSaveTeam, TeamData: &rawTeam2, ID: "team-batch"} // ID needed for batch keying

	// Create logs for ApplyBatch
	logs := []*raft.Log{
		{Type: raft.LogCommand, Data: func() []byte { b, _ := json.Marshal(cmdBatchTeam); return b }()},
	}

	fsm.ApplyBatch(logs)

	loadedTeam2, err := ts.LoadTeam("team-batch")
	if err != nil {
		t.Fatalf("LoadTeam (batch) failed: %v", err)
	}
	if loadedTeam2.ID != "team-batch" {
		t.Error("Batch SaveTeam failed")
	}
}
