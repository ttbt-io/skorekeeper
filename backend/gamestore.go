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

// Player represents a player in the game roster (v3 standardized).
type Player struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
	Pos    string `json:"pos"`
}

// RosterSlot represents a position in the batting order.
type RosterSlot struct {
	Slot    int      `json:"slot"`
	Starter Player   `json:"starter"`
	Current Player   `json:"current"`
	History []Player `json:"history"`
}

// Permissions defines access control for a game.
type Permissions struct {
	Public string            `json:"public"` // "none", "read"
	Users  map[string]string `json:"users"`  // "email": "read"|"write"
}

// Game represents the full game state as stored on disk.
type Game struct {
	ID            string            `json:"id"`
	SchemaVersion int               `json:"schemaVersion"`
	Date          string            `json:"date,omitempty"`
	Location      string            `json:"location,omitempty"`
	Event         string            `json:"event,omitempty"`
	Away          string            `json:"away,omitempty"`
	Home          string            `json:"home,omitempty"`
	Status        string            `json:"status"`
	OwnerID       string            `json:"ownerId"`
	Permissions   Permissions       `json:"permissions,omitempty"`
	AwayTeamID    string            `json:"awayTeamId,omitempty"`
	HomeTeamID    string            `json:"homeTeamId,omitempty"`
	ActionLog     []json.RawMessage `json:"actionLog,omitempty"`

	// DeletedAt is the timestamp (Unix Nano) when the game was deleted.
	DeletedAt int64 `json:"deletedAt,omitempty"`

	// LastRaftIndex tracks the index of the last Raft log entry applied to this game.
	// Used for idempotency during log replay.
	LastRaftIndex uint64 `json:"lastRaftIndex,omitempty"`

	// Roster and Subs are now strictly typed.
	// We need custom Unmarshal to handle migration from v2 (n/u/p) to v3 (name/number/pos).
	// For now, we'll use a map structure for intermediate loading or dual fields?
	// To strictly follow the plan, we update the struct.
	// But JSON unmarshal will fail if fields don't match.
	// So we need an Alias or custom UnmarshalJSON.
	Roster map[string][]RosterSlot `json:"roster,omitempty"`
	Subs   map[string][]Player     `json:"subs,omitempty"`
}

func (g *Game) normalize() {
	if g.SchemaVersion == 0 {
		g.SchemaVersion = CurrentSchemaVersion
	}
	if g.Permissions.Users == nil {
		g.Permissions.Users = make(map[string]string)
	}
	if g.ActionLog == nil {
		g.ActionLog = make([]json.RawMessage, 0)
	}
	if g.Roster == nil {
		g.Roster = make(map[string][]RosterSlot)
	}
	if g.Subs == nil {
		g.Subs = make(map[string][]Player)
	}
}

// GameStore manages game persistence to disk.
type GameStore struct {
	DataDir string
	Debug   bool
	storage *storage.Storage
	mu      sync.Map // Stores *sync.RWMutex for each gameId to protect writes and reads
	cache   sync.Map // Stores the latest []byte (JSON) for each gameId (for backward compat / cache)

	dirtyMu sync.Mutex
	dirty   map[string]bool
}

// NewGameStore creates a new GameStore.
func NewGameStore(dataDir string, s *storage.Storage) *GameStore {
	return &GameStore{
		DataDir: dataDir,
		storage: s,
		mu:      sync.Map{},
		cache:   sync.Map{},
		dirty:   make(map[string]bool),
	}
}

// SaveGame saves the game data atomically.
func (gs *GameStore) SaveGame(game *Game) error {
	gameId := game.ID
	// Get or create a mutex for this specific game
	m, _ := gs.mu.LoadOrStore(gameId, &sync.RWMutex{})
	mutex := m.(*sync.RWMutex)

	mutex.Lock()
	defer mutex.Unlock()

	encodedGameId := url.PathEscape(gameId)
	filename := filepath.Join("games", fmt.Sprintf("%s.json", encodedGameId))

	if len(game.ActionLog) == 0 {
		log.Printf("SaveGame WARNING: Saving game %s with 0 actions!", gameId)
	}

	if err := gs.storage.SaveDataFile(filename, game); err != nil {
		return fmt.Errorf("storage.SaveDataFile: %w", err)
	}

	// Update cache with JSON representation (for callers that might need bytes, or just to keep it warm)
	// We might eventually remove this cache if we switch fully to structs.
	// For now, let's keep it consistent.
	if jsonBytes, err := json.Marshal(game); err == nil {
		gs.cache.Store(gameId, jsonBytes)
	}

	gs.dirtyMu.Lock()
	delete(gs.dirty, gameId)
	gs.dirtyMu.Unlock()

	return nil
}

// SaveGameInMemory updates the in-memory cache and marks the game as dirty.
// If forceSync is true, it writes to disk immediately (behaving like SaveGame).
func (gs *GameStore) SaveGameInMemory(game *Game, forceSync bool) error {
	// 1. Update Cache (Authoritative)
	jsonBytes, err := json.Marshal(game)
	if err != nil {
		return err
	}
	gs.cache.Store(game.ID, jsonBytes)

	// 2. Handle Persistence
	if forceSync {
		return gs.SaveGame(game)
	}

	// 3. Mark as Dirty
	gs.dirtyMu.Lock()
	gs.dirty[game.ID] = true
	gs.dirtyMu.Unlock()

	return nil
}

// Flush persists a specific game to disk if it is dirty.
func (gs *GameStore) Flush(gameId string) error {
	gs.dirtyMu.Lock()
	if !gs.dirty[gameId] {
		gs.dirtyMu.Unlock()
		return nil
	}
	gs.dirtyMu.Unlock()

	// Load from cache (Authoritative)
	val, ok := gs.cache.Load(gameId)
	if !ok {
		// If it's not in cache but marked dirty, that's an issue.
		// However, it might have been evicted (if we had eviction).
		// For now, assume it's fine or already saved?
		// Better to be safe: check if we can load it?
		// If we can't load from cache, we can't flush what we don't have.
		// We should clear the dirty flag?
		gs.dirtyMu.Lock()
		delete(gs.dirty, gameId)
		gs.dirtyMu.Unlock()
		return fmt.Errorf("game %s marked dirty but not found in cache", gameId)
	}

	var g Game
	if err := json.Unmarshal(val.([]byte), &g); err != nil {
		return fmt.Errorf("failed to unmarshal game from cache for flush: %w", err)
	}

	// SaveGame will clear the dirty flag
	return gs.SaveGame(&g)
}

// FlushAll persists all dirty games to disk.
func (gs *GameStore) FlushAll() error {
	gs.dirtyMu.Lock()
	// Copy dirty keys to slice to release lock while flushing
	dirtyIds := make([]string, 0, len(gs.dirty))
	for id := range gs.dirty {
		dirtyIds = append(dirtyIds, id)
	}
	gs.dirtyMu.Unlock()

	for _, id := range dirtyIds {
		if err := gs.Flush(id); err != nil {
			return fmt.Errorf("failed to flush game %s: %w", id, err)
		}
	}
	return nil
}

// LoadGame loads the game data by game ID.
func (gs *GameStore) LoadGame(gameId string) (*Game, error) {
	// Cache check? The cache stores []byte.
	// If we return *Game, we need to unmarshal from cache.
	if val, ok := gs.cache.Load(gameId); ok {
		var g Game
		if err := json.Unmarshal(val.([]byte), &g); err == nil {
			if gs.Debug {
				log.Printf("[CACHE] Hit for game %s", gameId)
			}
			g.normalize()
			return &g, nil
		}
		// If unmarshal fails, proceed to load from disk
		gs.cache.Delete(gameId)
	}
	if gs.Debug {
		log.Printf("[CACHE] Miss for game %s", gameId)
	}

	m, _ := gs.mu.LoadOrStore(gameId, &sync.RWMutex{})
	mutex := m.(*sync.RWMutex)

	mutex.RLock()
	defer mutex.RUnlock()

	encodedGameId := url.PathEscape(gameId)
	filename := filepath.Join("games", fmt.Sprintf("%s.json", encodedGameId))

	var g Game
	err := gs.storage.ReadDataFile(filename, &g)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("ReadDataFile: %w", err)
	}
	if g.SchemaVersion < SchemaVersionV3 {
		return nil, fmt.Errorf("legacy schema version %d no longer supported", g.SchemaVersion)
	}
	g.normalize()

	// Update cache
	if jsonBytes, err := json.Marshal(&g); err == nil {
		gs.cache.Store(gameId, jsonBytes)
	}

	if len(g.ActionLog) == 0 {
		fi, _ := os.Stat(filepath.Join(gs.DataDir, filename))
		size := int64(-1)
		if fi != nil {
			size = fi.Size()
		}
		log.Printf("LoadGame WARNING: Loaded game %s with 0 actions! File size: %d bytes. Path: %s", gameId, size, filename)
	}

	return &g, nil
}

// LoadGameAsJSON is a helper for backward compatibility or API handlers that just want bytes.
func (gs *GameStore) LoadGameAsJSON(gameId string) ([]byte, error) {
	g, err := gs.LoadGame(gameId)
	if err != nil {
		return nil, err
	}
	return json.Marshal(g)
}

// DeleteGame deletes a specific game by overwriting it with a tombstone.
func (gs *GameStore) DeleteGame(gameId string) error {
	// Load first to get OwnerID
	g, err := gs.LoadGame(gameId)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	m, _ := gs.mu.LoadOrStore(gameId, &sync.RWMutex{})
	mutex := m.(*sync.RWMutex)

	mutex.Lock()
	defer mutex.Unlock()

	// Create tombstone
	tombstone := &Game{
		ID:            gameId,
		SchemaVersion: CurrentSchemaVersion,
		Status:        "deleted",
		OwnerID:       g.OwnerID,
		DeletedAt:     time.Now().UnixNano(),
	}

	encodedGameId := url.PathEscape(gameId)
	filename := filepath.Join("games", fmt.Sprintf("%s.json", encodedGameId))

	if err := gs.storage.SaveDataFile(filename, tombstone); err != nil {
		return fmt.Errorf("storage.SaveDataFile (tombstone): %w", err)
	}

	// Update cache with tombstone
	if jsonBytes, err := json.Marshal(tombstone); err == nil {
		gs.cache.Store(gameId, jsonBytes)
	}

	return nil
}

// PurgeGame permanently deletes the game file.
func (gs *GameStore) PurgeGame(gameId string) error {
	m, _ := gs.mu.LoadOrStore(gameId, &sync.RWMutex{})
	mutex := m.(*sync.RWMutex)

	mutex.Lock()
	defer mutex.Unlock()

	gs.cache.Delete(gameId)

	encodedGameId := url.PathEscape(gameId)
	filename := filepath.Join("games", fmt.Sprintf("%s.json", encodedGameId))
	fullPath := filepath.Join(gs.DataDir, filename)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("could not purge game file: %w", err)
	}
	return nil
}

// GameSummary represents a summary of a game.
type GameSummary struct {
	ID       string `json:"id"`
	Date     string `json:"date"`
	Location string `json:"location"`
	Event    string `json:"event"`
	Away     string `json:"away"`
	Home     string `json:"home"`
	Revision string `json:"revision"`
	Status   string `json:"status"`
	OwnerID  string `json:"ownerId"`
}

// GameMetadata contains only the fields needed for indexing.
type GameMetadata struct {
	ID          string      `json:"id"`
	OwnerID     string      `json:"ownerId"`
	Permissions Permissions `json:"permissions"`
	AwayTeamID  string      `json:"awayTeamId"`
	HomeTeamID  string      `json:"homeTeamId"`
	Status      string      `json:"status"`
	DeletedAt   int64       `json:"deletedAt"`
}

// ListAllGameMetadata returns metadata for all games without loading full action logs.
func (gs *GameStore) ListAllGameMetadata() iter.Seq2[GameMetadata, error] {
	return func(yield func(GameMetadata, error) bool) {
		// 1. Scan Disk
		gamesDir := filepath.Join(gs.DataDir, "games")
		files, err := os.ReadDir(gamesDir)
		if err != nil && !os.IsNotExist(err) {
			yield(GameMetadata{}, fmt.Errorf("could not read games directory: %w", err))
			return
		}

		seen := make(map[string]bool)

		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				encodedGameId := strings.TrimSuffix(file.Name(), ".json")
				gameId, err := url.PathUnescape(encodedGameId)
				if err != nil {
					continue
				}

				seen[gameId] = true

				g, err := gs.LoadGame(gameId)
				if err != nil {
					log.Printf("Registry Warning: failed to load game %s from disk: %v", gameId, err)
					continue
				}

				if !yield(GameMetadata{
					ID:          g.ID,
					OwnerID:     g.OwnerID,
					Permissions: g.Permissions,
					AwayTeamID:  g.AwayTeamID,
					HomeTeamID:  g.HomeTeamID,
					Status:      g.Status,
					DeletedAt:   g.DeletedAt,
				}, nil) {
					return
				}
			}
		}

		// 2. Scan Dirty Cache (for games created in memory but not yet flushed)
		gs.dirtyMu.Lock()
		dirtyIds := make([]string, 0, len(gs.dirty))
		for id := range gs.dirty {
			dirtyIds = append(dirtyIds, id)
		}
		gs.dirtyMu.Unlock()

		for _, id := range dirtyIds {
			if seen[id] {
				continue // Already processed from disk scan
			}

			// Must verify existence (LoadGame handles cache lookup)
			g, err := gs.LoadGame(id)
			if err != nil {
				log.Printf("Error: Failed to load dirty game %s: %v", id, err)
				continue
			}

			if !yield(GameMetadata{
				ID:          g.ID,
				OwnerID:     g.OwnerID,
				Permissions: g.Permissions,
				AwayTeamID:  g.AwayTeamID,
				HomeTeamID:  g.HomeTeamID,
				Status:      g.Status,
				DeletedAt:   g.DeletedAt,
			}, nil) {
				return
			}
		}
	}
}

// ListAllGames returns an iterator over all games found in the flat games directory.
func (gs *GameStore) ListAllGames() iter.Seq2[*Game, error] {
	return func(yield func(*Game, error) bool) {
		// 1. Scan Disk
		gamesDir := filepath.Join(gs.DataDir, "games")
		files, err := os.ReadDir(gamesDir)
		if err != nil && !os.IsNotExist(err) {
			yield(nil, fmt.Errorf("could not read games directory: %w", err))
			return
		}

		seen := make(map[string]bool)

		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				encodedGameId := strings.TrimSuffix(file.Name(), ".json")
				gameId, err := url.PathUnescape(encodedGameId)
				if err != nil {
					continue
				}

				seen[gameId] = true

				g, err := gs.LoadGame(gameId)
				if err != nil {
					log.Printf("Warning: could not load game '%s': %v", gameId, err)
					continue
				}
				g.normalize()
				if !yield(g, nil) {
					return
				}
			}
		}

		// 2. Scan Dirty Cache (New games not yet on disk)
		gs.dirtyMu.Lock()
		dirtyIds := make([]string, 0, len(gs.dirty))
		for id := range gs.dirty {
			dirtyIds = append(dirtyIds, id)
		}
		gs.dirtyMu.Unlock()

		for _, id := range dirtyIds {
			if seen[id] {
				continue
			}

			g, err := gs.LoadGame(id)
			if err != nil {
				log.Printf("Error: Failed to load dirty game %s: %v", id, err)
				continue
			}
			g.normalize()
			if !yield(g, nil) {
				return
			}
		}
	}
}
