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
)

func TestValidateAction(t *testing.T) {
	validUUID := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"

	tests := []struct {
		name    string
		action  string
		wantErr bool
	}{
		{
			name: "Valid GAME_START",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "GAME_START",
				"payload": {
					"id": "%s",
					"date": "2025-12-18T14:57:39Z",
					"away": "Team A",
					"home": "Team B"
				}
			}`, validUUID, validUUID),
			wantErr: false,
		},
		{
			name: "GAME_START with invalid team IDs",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "GAME_START",
				"payload": {
					"id": "%s",
					"date": "2025-12-18T14:57:39Z",
					"away": "Team A",
					"home": "Team B",
					"awayTeamId": "-- Select Team (Optional) --",
					"homeTeamId": "-- Select Team (Optional) --"
				}
			}`, validUUID, validUUID),
			wantErr: false,
		},
		{
			name: "Valid PITCH",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "PITCH",
				"payload": {
					"type": "ball",
					"activeTeam": "away",
					"activeCtx": {"b": 0, "i": 1, "col": "col-1-0"}
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Invalid Action ID",
			action: `{
				"id": "invalid",
				"type": "GAME_START",
				"payload": {}
			}`,
			wantErr: true,
		},
		{
			name: "Unknown Action Type",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "UNKNOWN_ACTION",
				"payload": {}
			}`, validUUID),
			wantErr: true,
		},
		{
			name: "Valid UNDO",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "UNDO",
				"payload": {
					"refId": "%s"
				}
			}`, validUUID, validUUID),
			wantErr: false,
		},
		{
			name: "Valid GAME_METADATA_UPDATE",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "GAME_METADATA_UPDATE",
				"payload": {
					"id": "%s",
					"permissions": {
						"public": "read",
						"users": {"user@example.com": "write"}
					}
				}
			}`, validUUID, validUUID),
			wantErr: false,
		},
		{
			name: "Valid GAME_METADATA_UPDATE Full",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "GAME_METADATA_UPDATE",
				"payload": {
					"id": "%s",
					"date": "2024-01-01",
					"location": "Stadium",
					"event": "Championship",
					"away": "Team A",
					"home": "Team B",
					"awayTeamId": "%s",
					"homeTeamId": "%s"
				}
			}`, validUUID, validUUID, validUUID, validUUID),
			wantErr: false,
		},
		{
			name: "Valid PLAY_RESULT",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "PLAY_RESULT",
				"payload": {
					"activeTeam": "home",
					"activeCtx": {"b": 2, "i": 3, "col": "c1"},
					"bipState": {"res": "Safe", "base": "1B", "type": "HIT", "seq": [6]}
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid SUBSTITUTION",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "SUBSTITUTION",
				"payload": {
					"team": "away",
					"rosterIndex": 5,
					"subParams": {"n": "New Player", "u": "99", "p": "P", "id": "%s"}
				}
			}`, validUUID, validUUID),
			wantErr: false,
		},
		{
			name: "Valid RUNNER_ADVANCE",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "RUNNER_ADVANCE",
				"payload": {
					"activeTeam": "away",
					"updates": [{"key": "away-0-c1", "base": 1, "action": "to 2nd"}]
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "GAME_METADATA_UPDATE with invalid team IDs",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "GAME_METADATA_UPDATE",
				"payload": {
					"id": "%s",
					"awayTeamId": "-- Select Team (Optional) --",
					"homeTeamId": "-- Select Team (Optional) --"
				}
			}`, validUUID, validUUID),
			wantErr: false,
		},
		{
			name: "Valid GAME_FINALIZE",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "GAME_FINALIZE",
				"payload": {
					"finalScore": {"away": 5, "home": 2},
					"stats": {},
					"timestamp": 123456789
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid LINEUP_UPDATE",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "LINEUP_UPDATE",
				"payload": {
					"team": "away",
					"teamName": "Alpha",
					"roster": [{"current": {"n": "P1", "u": "1", "id": "%s"}}]
				}
			}`, validUUID, validUUID),
			wantErr: false,
		},
		{
			name: "Valid SCORE_OVERRIDE",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "SCORE_OVERRIDE",
				"payload": {
					"team": "home",
					"inning": 1,
					"score": "5"
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid PITCHER_UPDATE",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "PITCHER_UPDATE",
				"payload": {
					"team": "away",
					"pitcher": "Starter"
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid MOVE_PLAY",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "MOVE_PLAY",
				"payload": {
					"sourceKey": "away-0-col-1-0",
					"targetKey": "away-1-col-1-0"
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid CLEAR_DATA",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "CLEAR_DATA",
				"payload": {
					"activeTeam": "away",
					"activeCtx": {"b": 0, "i": 1, "col": "c1"}
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid RUNNER_BATCH_UPDATE",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "RUNNER_BATCH_UPDATE",
				"payload": {
					"updates": [{"key": "away-0-c1", "action": "to 2nd", "base": 1}]
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid ADD_COLUMN",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "ADD_COLUMN",
				"payload": {
					"targetInning": 6,
					"team": "away"
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid REMOVE_COLUMN",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "REMOVE_COLUMN",
				"payload": {
					"colId": "col-1-1",
					"team": "away"
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid SET_INNING_LEAD",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "SET_INNING_LEAD",
				"payload": {
					"team": "away",
					"colId": "col-1-0"
				}
			}`, validUUID),
			wantErr: false,
		},
		{
			name: "Valid GAME_IMPORT",
			action: fmt.Sprintf(`{
				"id": "%s",
				"type": "GAME_IMPORT",
				"payload": {
					"id": "%s"
				}
			}`, validUUID, validUUID),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAction(json.RawMessage(tt.action))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateActions(t *testing.T) {
	validUUID1 := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
	validUUID2 := "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb"

	tests := []struct {
		name    string
		actions []string
		wantErr bool
	}{
		{
			name: "Valid Batch",
			actions: []string{
				fmt.Sprintf(`{"id": "%s", "type": "PITCH", "payload": {"type": "ball", "activeTeam": "away", "activeCtx": {"b": 0, "i": 1, "col": "c1"}}}`, validUUID1),
				fmt.Sprintf(`{"id": "%s", "type": "PITCH", "payload": {"type": "strike", "activeTeam": "away", "activeCtx": {"b": 0, "i": 1, "col": "c1"}}}`, validUUID2),
			},
			wantErr: false,
		},
		{
			name: "Invalid Action in Batch",
			actions: []string{
				fmt.Sprintf(`{"id": "%s", "type": "PITCH", "payload": {"type": "ball", "activeTeam": "away", "activeCtx": {"b": 0, "i": 1, "col": "c1"}}}`, validUUID1),
				`{"id": "invalid", "type": "PITCH", "payload": {}}`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raws []json.RawMessage
			for _, a := range tt.actions {
				raws = append(raws, json.RawMessage(a))
			}
			err := ValidateActions(raws)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateActions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyActions(t *testing.T) {
	g := &Game{
		ID:        "game1",
		ActionLog: []json.RawMessage{},
	}
	validUUID1 := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
	validUUID2 := "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb"

	actions := []json.RawMessage{
		json.RawMessage(fmt.Sprintf(`{"id": "%s", "type": "PITCH", "payload": {"type": "ball"}}`, validUUID1)),
		json.RawMessage(fmt.Sprintf(`{"id": "%s", "type": "PITCH", "payload": {"type": "strike"}}`, validUUID2)),
	}

	// Initial apply
	changed, err := ApplyActions(g, actions)
	if err != nil {
		t.Fatalf("Unexpected error applying actions: %v", err)
	}
	if !changed {
		t.Error("Expected changed=true on initial apply")
	}
	if len(g.ActionLog) != 2 {
		t.Errorf("Expected ActionLog length 2, got %d", len(g.ActionLog))
	}

	// Idempotent apply (batch containing already applied actions)
	changed, err = ApplyActions(g, actions)
	if err != nil {
		t.Fatalf("Unexpected error applying same actions: %v", err)
	}
	if changed {
		t.Error("Expected changed=false on idempotent apply")
	}
	if len(g.ActionLog) != 2 {
		t.Errorf("Expected ActionLog length 2 after idempotent apply, got %d", len(g.ActionLog))
	}

	// Partial idempotent apply
	validUUID3 := "cccccccc-cccc-4ccc-cccc-cccccccccccc"
	newActions := []json.RawMessage{
		actions[1], // Duplicate
		json.RawMessage(fmt.Sprintf(`{"id": "%s", "type": "PITCH", "payload": {"type": "foul"}}`, validUUID3)),
	}
	changed, err = ApplyActions(g, newActions)
	if err != nil {
		t.Fatalf("Unexpected error applying partial duplicate batch: %v", err)
	}
	if !changed {
		t.Error("Expected changed=true on partial idempotent apply")
	}
	if len(g.ActionLog) != 3 {
		t.Errorf("Expected ActionLog length 3, got %d", len(g.ActionLog))
	}
}

func TestSpecificValidators(t *testing.T) {
	// Testing helper functions and edge cases in payload validators

	t.Run("validateStringLen", func(t *testing.T) {
		if err := validateStringLen("short", 10, "test"); err != nil {
			t.Errorf("Unexpected error for short string: %v", err)
		}
		if err := validateStringLen("way too long", 5, "test"); err == nil {
			t.Error("Expected error for long string, got nil")
		}
	})

	t.Run("validateContext", func(t *testing.T) {
		validCtx := Context{B: 0, I: 1, Col: "col-1-0"}
		if err := validateContext(validCtx); err != nil {
			t.Errorf("Unexpected error for valid context: %v", err)
		}

		invalidB := Context{B: -1, I: 1, Col: "c"}
		if err := validateContext(invalidB); err == nil {
			t.Error("Expected error for negative batter index")
		}

		invalidI := Context{B: 0, I: 0, Col: "c"}
		if err := validateContext(invalidI); err == nil {
			t.Error("Expected error for inning < 1")
		}
	})
}

func TestApplyAction_MetadataUpdate(t *testing.T) {
	g := &Game{
		ID:        "game-meta-test",
		ActionLog: []json.RawMessage{},
	}
	validUUID := "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"

	// Action with full metadata
	actionJSON := fmt.Sprintf(`{
		"id": "%s",
		"type": "GAME_METADATA_UPDATE",
		"payload": {
			"date": "2024-05-20",
			"location": "Main Field",
			"event": "Playoffs",
			"away": "Visitors",
			"home": "Hosts"
		}
	}`, validUUID)

	changed, err := ApplyAction(g, json.RawMessage(actionJSON))
	if err != nil {
		t.Fatalf("ApplyAction failed: %v", err)
	}
	if !changed {
		t.Error("Expected changed=true")
	}

	if g.Date != "2024-05-20" {
		t.Errorf("Date not updated, got %s", g.Date)
	}
	if g.Location != "Main Field" {
		t.Errorf("Location not updated, got %s", g.Location)
	}
	if g.Event != "Playoffs" {
		t.Errorf("Event not updated, got %s", g.Event)
	}
	if g.Away != "Visitors" {
		t.Errorf("Away not updated, got %s", g.Away)
	}
	if g.Home != "Hosts" {
		t.Errorf("Home not updated, got %s", g.Home)
	}
}
