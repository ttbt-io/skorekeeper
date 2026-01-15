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
	"os"
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestGetGameAccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "auth_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	ts := NewTeamStore(tempDir, s)
	owner := "owner@example.com"
	editor := "editor@example.com"
	viewer := "viewer@example.com"
	stranger := "stranger@example.com"

	// Setup a Team
	teamId := "team-1"
	team := Team{
		ID:            teamId,
		SchemaVersion: SchemaVersionV3,
		OwnerID:       owner,
		Roles: TeamRoles{
			Admins:       []string{"admin@example.com"},
			Scorekeepers: []string{"sk@example.com"},
			Spectators:   []string{"spec@example.com"},
		},
	}
	ts.SaveTeam(&team)

	game := Game{
		ID:            "game-1",
		SchemaVersion: SchemaVersionV3,
		OwnerID:       owner,
		Permissions: Permissions{
			Public: "none",
			Users: map[string]string{
				editor: "write",
				viewer: "read",
			},
		},
		AwayTeamID: teamId,
	}

	tests := []struct {
		name   string
		userId string
		game   Game
		want   AccessLevel
	}{
		{"Owner", owner, game, AccessAdmin},
		{"Direct Editor", editor, game, AccessWrite},
		{"Direct Viewer", viewer, game, AccessRead},
		{"Stranger Private", stranger, game, AccessNone},
		{"Anonymous Private", "", game, AccessNone},
		{"Team Admin Inheritance", "admin@example.com", game, AccessAdmin},
		{"Team Scorekeeper Inheritance", "sk@example.com", game, AccessWrite},
		{"Team Spectator Inheritance", "spec@example.com", game, AccessRead},
		{"Public Read Access", "anon@example.com", Game{SchemaVersion: SchemaVersionV3, Permissions: Permissions{Public: "read"}}, AccessRead},
		{"Public Read Anonymous", "", Game{SchemaVersion: SchemaVersionV3, Permissions: Permissions{Public: "read"}}, AccessRead},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetGameAccess(tt.userId, tt.game, ts); got != tt.want {
				t.Errorf("GetGameAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTeamAccess(t *testing.T) {
	owner := "owner@example.com"
	admin := "admin@example.com"
	sk := "sk@example.com"
	spec := "spec@example.com"
	stranger := "stranger@example.com"

	team := Team{
		OwnerID: owner,
		Roles: TeamRoles{
			Admins:       []string{admin},
			Scorekeepers: []string{sk},
			Spectators:   []string{spec},
		},
	}

	tests := []struct {
		name   string
		userId string
		want   AccessLevel
	}{
		{"Owner", owner, AccessAdmin},
		{"Admin", admin, AccessAdmin},
		{"Scorekeeper", sk, AccessWrite},
		{"Spectator", spec, AccessRead},
		{"Stranger", stranger, AccessNone},
		{"Anonymous", "", AccessNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTeamAccess(tt.userId, team); got != tt.want {
				t.Errorf("GetTeamAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}
