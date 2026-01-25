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
	"fmt"
	"testing"

	"github.com/c2FmZQ/storage"
)

func makeAction(id string, payload string) json.RawMessage {
	// payload string is ignored, we use standard PITCH payload
	return json.RawMessage(fmt.Sprintf(`{"id":"%s","type":"PITCH","payload":{"type":"ball","activeTeam":"away","activeCtx":{"b":0,"i":1,"col":"col-1-0"}}}`, id))
}

func TestConflictResolution_PrefixMatch(t *testing.T) {
	// Setup
	s := storage.New(t.TempDir(), nil)
	gs := NewGameStore(t.TempDir(), s)
	ts := NewTeamStore(t.TempDir(), s)
	us := NewUserIndexStore(t.TempDir(), s, nil)
	reg := NewRegistry(gs, ts, us, true)
	hm := NewHubManager()

	// Create Game
	gameID := makeUUID(999)
	g := &Game{ID: gameID, OwnerID: "user1"}
	gs.SaveGame(g)

	// Initialize Hub
	hub := hm.GetHub(gameID, false, gs, ts, reg)
	hub.ensureLoaded(nil)

	// 1. Apply Action A directly to state (simulate previous success)
	idA := makeUUID(1)
	actionA := makeAction(idA, "first")
	g.ActionLog = append(g.ActionLog, actionA)
	hub.gameData = g // Update hub state

	// 2. Prepare Batch [A, B] with Base "" (claiming start of log)
	// Server has [A]. Head=A.
	// Client sends [A, B]. Base=""
	idB := makeUUID(2)
	actionB := makeAction(idB, "second")

	msg := Message{
		Type:         MsgTypeAction,
		Actions:      []json.RawMessage{actionA, actionB},
		BaseRevision: "", // Claiming start of log
	}

	// 3. Process
	resp, _, err := hub.processAction(msg, "user1")
	if err != nil {
		t.Fatalf("processAction failed: %v", err)
	}

	if resp.Type != MsgTypeAck {
		t.Errorf("Expected ACK, got %s: %s", resp.Type, resp.Error)
	}

	// 4. Verify B is applied
	if len(hub.gameData.ActionLog) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(hub.gameData.ActionLog))
	}
	last := getCurrentRevision(hub.gameData.ActionLog)
	if last != idB {
		t.Errorf("Expected last revision %s, got %s", idB, last)
	}
}

func TestConflictResolution_PartialOverlap(t *testing.T) {
	// Setup
	s := storage.New(t.TempDir(), nil)
	gs := NewGameStore(t.TempDir(), s)
	ts := NewTeamStore(t.TempDir(), s)
	us := NewUserIndexStore(t.TempDir(), s, nil)
	reg := NewRegistry(gs, ts, us, true)
	hm := NewHubManager()

	gameID := makeUUID(998)
	g := &Game{ID: gameID, OwnerID: "user1"}
	idA := makeUUID(10)
	idB := makeUUID(11)
	actionA := makeAction(idA, "first")
	actionB := makeAction(idB, "second")
	g.ActionLog = append(g.ActionLog, actionA, actionB)
	gs.SaveGame(g)

	hub := hm.GetHub(gameID, false, gs, ts, reg)
	hub.ensureLoaded(nil)

	// Server has [A, B].
	// Client sends [B, C]. Base="A".
	idC := makeUUID(12)
	actionC := makeAction(idC, "third")

	msg := Message{
		Type:         MsgTypeAction,
		Actions:      []json.RawMessage{actionB, actionC},
		BaseRevision: idA,
	}

	// Process
	resp, _, err := hub.processAction(msg, "user1")
	if err != nil {
		t.Fatalf("processAction failed: %v", err)
	}

	if resp.Type != MsgTypeAck {
		t.Errorf("Expected ACK, got %s: %s", resp.Type, resp.Error)
	}

	// Verify C is applied
	if len(hub.gameData.ActionLog) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(hub.gameData.ActionLog))
	}
	last := getCurrentRevision(hub.gameData.ActionLog)
	if last != idC {
		t.Errorf("Expected last revision %s, got %s", idC, last)
	}
}

func TestConflictResolution_Divergence(t *testing.T) {
	// Setup
	s := storage.New(t.TempDir(), nil)
	gs := NewGameStore(t.TempDir(), s)
	ts := NewTeamStore(t.TempDir(), s)
	us := NewUserIndexStore(t.TempDir(), s, nil)
	reg := NewRegistry(gs, ts, us, true)
	hm := NewHubManager()

	gameID := makeUUID(997)
	g := &Game{ID: gameID, OwnerID: "user1"}
	idA := makeUUID(20)
	idB := makeUUID(21)
	actionA := makeAction(idA, "first")
	actionB := makeAction(idB, "second")
	g.ActionLog = append(g.ActionLog, actionA, actionB)
	gs.SaveGame(g)

	hub := hm.GetHub(gameID, false, gs, ts, reg)
	hub.ensureLoaded(nil)

	// Server has [A, B].
	// Client sends [X, C]. Base="A".
	// Expect Conflict because X != B.
	idX := makeUUID(99)
	idC := makeUUID(22)
	actionX := makeAction(idX, "divergent")
	actionC := makeAction(idC, "third")

	msg := Message{
		Type:         MsgTypeAction,
		Actions:      []json.RawMessage{actionX, actionC},
		BaseRevision: idA,
	}

	// Process
	resp, _, err := hub.processAction(msg, "user1")
	if err != nil {
		t.Fatalf("processAction failed: %v", err)
	}

	if resp.Type != MsgTypeConflict {
		t.Errorf("Expected CONFLICT, got %s", resp.Type)
	}
	if resp.Error != "History divergence" {
		t.Errorf("Expected 'History divergence', got '%s'", resp.Error)
	}
}

func TestConflictFix_StaleCache(t *testing.T) {
	// Setup
	s := storage.New(t.TempDir(), nil)
	gs := NewGameStore(t.TempDir(), s)
	ts := NewTeamStore(t.TempDir(), s)
	us := NewUserIndexStore(t.TempDir(), s, nil)
	reg := NewRegistry(gs, ts, us, true)
	hm := NewHubManager()

	gameID := makeUUID(888)
	g := &Game{ID: gameID, OwnerID: "user1"}
	idA := makeUUID(1)
	g.ActionLog = append(g.ActionLog, makeAction(idA, ""))
	gs.SaveGame(g)

	hub := hm.GetHub(gameID, false, gs, ts, reg)
	hub.ensureLoaded(nil)

	// Hub has [A]. Disk has [A].

	// 1. Update Disk "Behind the back" (Simulate FSM update that bypassed Hub somehow, or Hub stale)
	idB := makeUUID(2)
	g.ActionLog = append(g.ActionLog, makeAction(idB, ""))
	gs.SaveGame(g)

	// Hub has [A]. Disk has [A, B].
	// Client sends [C] with Base B.
	idC := makeUUID(3)
	msg := Message{
		Type:         MsgTypeAction,
		Action:       makeAction(idC, ""),
		BaseRevision: idB,
	}

	// 2. Process
	// Should detect mismatch (Hub Head A != Base B).
	// Should reload from Disk (get [A, B]).
	// Should match Base B.
	// Should apply C.

	resp, _, err := hub.processAction(msg, "user1")
	if err != nil {
		t.Fatalf("processAction failed: %v", err)
	}
	if resp.Type != MsgTypeAck {
		t.Errorf("Expected ACK, got %s: %s", resp.Type, resp.Error)
	}

	// Verify C in log
	gReload, _ := gs.LoadGame(gameID)
	if len(gReload.ActionLog) != 3 {
		t.Errorf("Expected 3 actions on disk, got %d", len(gReload.ActionLog))
	}
}
