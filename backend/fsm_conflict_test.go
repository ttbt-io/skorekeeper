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
	"strings"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/hashicorp/raft"
)

func TestFSM_ConflictDetection(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	st := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, st)
	ts := NewTeamStore(tmpDir, st)
	r := NewRegistry(gs, ts)
	hm := NewHubManager()
	fsm := NewFSM(gs, ts, r, hm, st)

	// Helper to create Action
	createAction := func(id, gameId, typeStr string) json.RawMessage {
		act := BaseAction{
			ID:   id,
			Type: typeStr,
		}
		b, _ := json.Marshal(act)
		return b
	}

	gameID := "conflict-test-game"
	act1 := createAction("act1", gameID, "GAME_START")
	act2 := createAction("act2", gameID, "ACTION_2")
	act3 := createAction("act3", gameID, "ACTION_3")

	// 1. Initial Save (Game with Act1)
	g1 := Game{
		ID:        gameID,
		ActionLog: []json.RawMessage{act1},
	}
	g1Bytes, _ := json.Marshal(g1)
	cmd1 := RaftCommand{Type: CmdSaveGame, ID: gameID, GameData: (*json.RawMessage)(&g1Bytes)}
	cmd1Bytes, _ := json.Marshal(cmd1)

	// Apply Cmd1
	fsm.Apply(&raft.Log{Index: 1, Data: cmd1Bytes})

	// Verify State
	loaded, _ := gs.LoadGame(gameID)
	if len(loaded.ActionLog) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(loaded.ActionLog))
	}

	// 2. Fast-Forward (Game with Act1, Act2)
	g2 := Game{
		ID:        gameID,
		ActionLog: []json.RawMessage{act1, act2},
	}
	g2Bytes, _ := json.Marshal(g2)
	cmd2 := RaftCommand{Type: CmdSaveGame, ID: gameID, GameData: (*json.RawMessage)(&g2Bytes)}
	cmd2Bytes, _ := json.Marshal(cmd2)

	// Apply Cmd2
	fsm.Apply(&raft.Log{Index: 2, Data: cmd2Bytes})

	// Verify State
	loaded, _ = gs.LoadGame(gameID)
	if len(loaded.ActionLog) != 2 {
		t.Fatalf("Expected 2 actions, got %d", len(loaded.ActionLog))
	}

	// 3. Conflict (Game with Act1, Act3) - Divergence
	// Server has [Act1, Act2]
	// Incoming has [Act1, Act3]
	g3 := Game{
		ID:        gameID,
		ActionLog: []json.RawMessage{act1, act3},
	}
	g3Bytes, _ := json.Marshal(g3)
	cmd3 := RaftCommand{Type: CmdSaveGame, ID: gameID, GameData: (*json.RawMessage)(&g3Bytes)}
	cmd3Bytes, _ := json.Marshal(cmd3)

	// Apply Cmd3 -> Should Return Error
	res := fsm.Apply(&raft.Log{Index: 3, Data: cmd3Bytes})
	if res == nil {
		t.Fatal("Expected conflict error, got nil")
	}
	err, ok := res.(error)
	if !ok {
		t.Fatal("Expected error type")
	}
	if !strings.Contains(err.Error(), "conflict detected") {
		t.Fatalf("Expected conflict error, got: %v", err)
	}

	// Verify State Unchanged
	loaded, _ = gs.LoadGame(gameID)
	if len(loaded.ActionLog) != 2 {
		t.Fatalf("Expected state unchanged (2 actions), got %d", len(loaded.ActionLog))
	}

	// 4. Force Overwrite (Game with Act1, Act3, Force=true)
	cmd4 := RaftCommand{Type: CmdSaveGame, ID: gameID, GameData: (*json.RawMessage)(&g3Bytes), Force: true}
	cmd4Bytes, _ := json.Marshal(cmd4)

	// Apply Cmd4 -> Should Succeed
	res = fsm.Apply(&raft.Log{Index: 4, Data: cmd4Bytes})
	if res != nil {
		t.Fatalf("Expected success with force, got error: %v", res)
	}

	// Verify State Overwritten
	loaded, _ = gs.LoadGame(gameID)
	if len(loaded.ActionLog) != 2 {
		t.Fatalf("Expected 2 actions, got %d", len(loaded.ActionLog))
	}
	// Check last action is act3
	var lastAct BaseAction
	json.Unmarshal(loaded.ActionLog[1], &lastAct)
	if lastAct.ID != "act3" {
		t.Fatalf("Expected last action ID 'act3', got %s", lastAct.ID)
	}
}
