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
	"iter"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/c2FmZQ/storage"
)

// TeamRoles defines the members of a team by their role.
type TeamRoles struct {
	Admins       []string `json:"admins"`
	Scorekeepers []string `json:"scorekeepers"`
	Spectators   []string `json:"spectators"`
}

func (r *TeamRoles) normalize() {
	if r.Admins == nil {
		r.Admins = make([]string, 0)
	}
	if r.Scorekeepers == nil {
		r.Scorekeepers = make([]string, 0)
	}
	if r.Spectators == nil {
		r.Spectators = make([]string, 0)
	}
}

// Team represents a persistent team roster and its permissions.
type Team struct {
	ID            string    `json:"id"`
	SchemaVersion int       `json:"schemaVersion"`
	Name          string    `json:"name,omitempty"`
	ShortName     string    `json:"shortName,omitempty"`
	Color         string    `json:"color,omitempty"`
	Roster        []Player  `json:"roster,omitempty"`
	OwnerID       string    `json:"ownerId"`
	Roles         TeamRoles `json:"roles,omitempty"`
	UpdatedAt     int64     `json:"updatedAt,omitempty"`

	// Status can be "active" (default/empty) or "deleted"
	Status string `json:"status,omitempty"`
	// DeletedAt is the timestamp (Unix Nano) when the team was deleted.
	DeletedAt int64 `json:"deletedAt,omitempty"`

	// LastRaftIndex tracks the index of the last Raft log entry applied to this team.
	// Used for idempotency during log replay.
	LastRaftIndex uint64 `json:"lastRaftIndex,omitempty"`
}

func (t *Team) normalize() {
	if t.SchemaVersion == 0 {
		t.SchemaVersion = CurrentSchemaVersion
	}
	if t.Roster == nil {
		t.Roster = make([]Player, 0)
	}
	t.Roles.normalize()
}

// TeamStore manages team persistence to disk.
type TeamStore struct {
	DataDir string
	storage *storage.Storage
	mu      sync.Map // Stores *sync.Mutex for each teamId to protect writes
}

// NewTeamStore creates a new TeamStore.
func NewTeamStore(dataDir string, s *storage.Storage) *TeamStore {
	return &TeamStore{
		DataDir: dataDir,
		storage: s,
		mu:      sync.Map{},
	}
}

// SaveTeam saves the team data atomically.
func (ts *TeamStore) SaveTeam(team *Team) error {
	teamId := team.ID
	// Get or create a mutex for this specific team
	m, _ := ts.mu.LoadOrStore(teamId, &sync.Mutex{})
	mutex := m.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()

	encodedTeamId := url.PathEscape(teamId)
	filename := filepath.Join("teams", fmt.Sprintf("%s.json", encodedTeamId))

	if err := ts.storage.SaveDataFile(filename, team); err != nil {
		return fmt.Errorf("storage.SaveDataFile: %w", err)
	}
	return nil
}

// LoadTeam loads the team data by ID.
// Handles migration from legacy JSON.
func (ts *TeamStore) LoadTeam(teamId string) (*Team, error) {
	encodedTeamId := url.PathEscape(teamId)
	filename := filepath.Join("teams", fmt.Sprintf("%s.json", encodedTeamId))

	var t Team
	err := ts.storage.ReadDataFile(filename, &t)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("ReadDataFile: %w", err)
	}
	if t.SchemaVersion < SchemaVersionV3 {
		return nil, fmt.Errorf("legacy schema version %d no longer supported", t.SchemaVersion)
	}
	t.normalize()

	return &t, nil
}

// LoadTeamAsJSON is a helper for backward compatibility.
func (ts *TeamStore) LoadTeamAsJSON(teamId string) ([]byte, error) {
	t, err := ts.LoadTeam(teamId)
	if err != nil {
		return nil, err
	}
	return json.Marshal(t)
}

// TeamMetadata contains only the fields needed for indexing.
type TeamMetadata struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"ownerId"`
	Roles     TeamRoles `json:"roles"`
	UpdatedAt int64     `json:"updatedAt"`
	Status    string    `json:"status"`
	DeletedAt int64     `json:"deletedAt"`
}

// ListAllTeamMetadata returns an iterator over metadata for all teams.
func (ts *TeamStore) ListAllTeamMetadata() iter.Seq2[TeamMetadata, error] {
	return func(yield func(TeamMetadata, error) bool) {
		teamsDir := filepath.Join(ts.DataDir, "teams")
		files, err := os.ReadDir(teamsDir)
		if err != nil {
			if !os.IsNotExist(err) {
				yield(TeamMetadata{}, fmt.Errorf("could not read teams directory: %w", err))
			}
			return
		}

		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				encodedTeamId := strings.TrimSuffix(file.Name(), ".json")
				teamId, err := url.PathUnescape(encodedTeamId)
				if err != nil {
					continue
				}

				t, err := ts.LoadTeam(teamId)
				if err != nil {
					continue
				}

				if !yield(TeamMetadata{
					ID:        t.ID,
					OwnerID:   t.OwnerID,
					Roles:     t.Roles,
					UpdatedAt: t.UpdatedAt,
					Status:    t.Status,
					DeletedAt: t.DeletedAt,
				}, nil) {
					return
				}
			}
		}
	}
}

// ListAllTeams returns an iterator over all teams found in the flat teams directory.
func (ts *TeamStore) ListAllTeams() iter.Seq2[*Team, error] {
	return func(yield func(*Team, error) bool) {
		teamsDir := filepath.Join(ts.DataDir, "teams")
		files, err := os.ReadDir(teamsDir)
		if err != nil {
			if !os.IsNotExist(err) {
				yield(nil, fmt.Errorf("could not read teams directory: %w", err))
			}
			return
		}

		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				encodedTeamId := strings.TrimSuffix(file.Name(), ".json")
				teamId, err := url.PathUnescape(encodedTeamId)
				if err != nil {
					continue
				}

				t, err := ts.LoadTeam(teamId)
				if err != nil {
					log.Printf("Warning: could not load team '%s': %v", teamId, err)
					continue
				}
				t.normalize()
				if !yield(t, nil) {
					return
				}
			}
		}
	}
}

// DeleteTeam deletes a specific team by overwriting it with a tombstone.
func (ts *TeamStore) DeleteTeam(teamId string) error {
	// Load first to get OwnerID
	t, err := ts.LoadTeam(teamId)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Get or create a mutex for this specific team
	m, _ := ts.mu.LoadOrStore(teamId, &sync.Mutex{})
	mutex := m.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()

	// Create Tombstone
	tombstone := &Team{
		ID:            teamId,
		SchemaVersion: CurrentSchemaVersion,
		OwnerID:       t.OwnerID,
		Status:        "deleted",
		DeletedAt:     time.Now().UnixNano(),
	}

	encodedTeamId := url.PathEscape(teamId)
	filename := filepath.Join("teams", fmt.Sprintf("%s.json", encodedTeamId))

	if err := ts.storage.SaveDataFile(filename, tombstone); err != nil {
		return fmt.Errorf("storage.SaveDataFile (tombstone): %w", err)
	}
	return nil
}

// PurgeTeam permanently deletes the team file.
func (ts *TeamStore) PurgeTeam(teamId string) error {
	// Get or create a mutex for this specific team
	m, _ := ts.mu.LoadOrStore(teamId, &sync.Mutex{})
	mutex := m.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()

	encodedTeamId := url.PathEscape(teamId)
	filename := filepath.Join("teams", fmt.Sprintf("%s.json", encodedTeamId))
	fullPath := filepath.Join(ts.DataDir, filename)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already gone
		}
		return fmt.Errorf("could not purge team file: %w", err)
	}
	return nil
}
