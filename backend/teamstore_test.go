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

func TestTeamStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "teamstore_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	store := NewTeamStore(tempDir, s)
	teamId := "team-1111-2222-3333"
	team := Team{
		ID:            teamId,
		SchemaVersion: SchemaVersionV3,
		Name:          "Test Team",
		Roles: TeamRoles{
			Admins: []string{"admin@example.com"},
		},
	}

	t.Run("SaveAndLoadTeam", func(t *testing.T) {
		if err := store.SaveTeam(&team); err != nil {
			t.Fatalf("SaveTeam failed: %v", err)
		}

		loaded, err := store.LoadTeam(teamId)
		if err != nil {
			t.Fatalf("LoadTeam failed: %v", err)
		}

		if loaded.Name != "Test Team" {
			t.Errorf("Expected Test Team, got %s", loaded.Name)
		}
	})

	t.Run("ListAllTeams", func(t *testing.T) {
		count := 0
		for _, err := range store.ListAllTeams() {
			if err != nil {
				t.Fatalf("ListAllTeams failed: %v", err)
			}
			count++
		}
		if count != 1 {
			t.Errorf("Expected 1 team, got %d", count)
		}
	})

	t.Run("DeleteTeam", func(t *testing.T) {
		if err := store.DeleteTeam(teamId); err != nil {
			t.Fatalf("DeleteTeam failed: %v", err)
		}
		loaded, err := store.LoadTeam(teamId)
		if err != nil {
			t.Errorf("Expected success (tombstone), got error: %v", err)
		}
		if loaded.Status != "deleted" {
			t.Errorf("Expected status 'deleted', got '%s'", loaded.Status)
		}
	})

	t.Run("PurgeTeam", func(t *testing.T) {
		if err := store.PurgeTeam(teamId); err != nil {
			t.Fatalf("PurgeTeam failed: %v", err)
		}
		_, err := store.LoadTeam(teamId)
		if !os.IsNotExist(err) {
			t.Errorf("Expected os.ErrNotExist after purge, got %v", err)
		}
	})

	t.Run("LoadTeamAsJSON", func(t *testing.T) {
		// Save it again first
		store.SaveTeam(&team)
		data, err := store.LoadTeamAsJSON(teamId)
		if err != nil {
			t.Fatalf("LoadTeamAsJSON failed: %v", err)
		}
		if len(data) == 0 {
			t.Error("Empty JSON data")
		}
	})
}
