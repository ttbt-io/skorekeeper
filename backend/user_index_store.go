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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	lru "github.com/hashicorp/golang-lru/v2"
)

// UserIndex represents the set of entities accessible by a user.
type UserIndex struct {
	UserID      string                 `json:"userId"`
	GameAccess  map[string]AccessLevel `json:"gameAccess"` // GameID -> AccessLevel
	TeamAccess  map[string]AccessLevel `json:"teamAccess"` // TeamID -> AccessLevel
	LastUpdated int64                  `json:"lastUpdated"`
}

// TeamGamesIndex represents the set of games associated with a team.
type TeamGamesIndex struct {
	TeamID      string          `json:"teamId"`
	GameIDs     map[string]bool `json:"gameIds"`
	LastUpdated int64           `json:"lastUpdated"`
}

// GameUsersIndex represents the set of users with direct access to a game.
type GameUsersIndex struct {
	GameID      string          `json:"gameId"`
	UserIDs     map[string]bool `json:"userIds"`
	LastUpdated int64           `json:"lastUpdated"`
}

// TeamUsersIndex represents the set of users who are members of a team.
type TeamUsersIndex struct {
	TeamID      string          `json:"teamId"`
	UserIDs     map[string]bool `json:"userIds"`
	LastUpdated int64           `json:"lastUpdated"`
}

// UserIndexStore manages persistence and caching of various Registry-related indices.
type UserIndexStore struct {
	DataDir   string
	storage   *storage.Storage
	masterKey crypto.MasterKey

	userCache     *lru.Cache[string, *UserIndex]      // Key: UserID
	teamGameCache *lru.Cache[string, *TeamGamesIndex] // Key: TeamID
	gameUserCache *lru.Cache[string, *GameUsersIndex] // Key: GameID
	teamUserCache *lru.Cache[string, *TeamUsersIndex] // Key: TeamID

	dirtyMu sync.Mutex
	dirtyU  map[string]bool // UserID
	dirtyTG map[string]bool // TeamID (Games)
	dirtyGU map[string]bool // GameID (Users)
	dirtyTU map[string]bool // TeamID (Users)

	muU  sync.Map
	muTG sync.Map
	muGU sync.Map
	muTU sync.Map
}

// NewUserIndexStore creates a new store for registry indices.
func NewUserIndexStore(dataDir string, s *storage.Storage, mk crypto.MasterKey) *UserIndexStore {
	store := &UserIndexStore{
		DataDir:   dataDir,
		storage:   s,
		masterKey: mk,
		dirtyU:    make(map[string]bool),
		dirtyTG:   make(map[string]bool),
		dirtyGU:   make(map[string]bool),
		dirtyTU:   make(map[string]bool),
	}

	// Define Eviction Callbacks
	onUserEvict := func(key string, value *UserIndex) {
		store.dirtyMu.Lock()
		isDirty := store.dirtyU[key]
		if isDirty {
			delete(store.dirtyU, key)
		}
		store.dirtyMu.Unlock()

		if isDirty {
			store.persistUserIndex(value)
		}
	}

	onTeamGameEvict := func(key string, value *TeamGamesIndex) {
		store.dirtyMu.Lock()
		isDirty := store.dirtyTG[key]
		if isDirty {
			delete(store.dirtyTG, key)
		}
		store.dirtyMu.Unlock()

		if isDirty {
			store.persistTeamGamesIndex(value)
		}
	}

	onGameUserEvict := func(key string, value *GameUsersIndex) {
		store.dirtyMu.Lock()
		isDirty := store.dirtyGU[key]
		if isDirty {
			delete(store.dirtyGU, key)
		}
		store.dirtyMu.Unlock()

		if isDirty {
			store.persistGameUsersIndex(value)
		}
	}

	onTeamUserEvict := func(key string, value *TeamUsersIndex) {
		store.dirtyMu.Lock()
		isDirty := store.dirtyTU[key]
		if isDirty {
			delete(store.dirtyTU, key)
		}
		store.dirtyMu.Unlock()

		if isDirty {
			store.persistTeamUsersIndex(value)
		}
	}

	uCache, _ := lru.NewWithEvict[string, *UserIndex](1000, onUserEvict)
	tgCache, _ := lru.NewWithEvict[string, *TeamGamesIndex](500, onTeamGameEvict)
	guCache, _ := lru.NewWithEvict[string, *GameUsersIndex](1000, onGameUserEvict)
	tuCache, _ := lru.NewWithEvict[string, *TeamUsersIndex](500, onTeamUserEvict)

	store.userCache = uCache
	store.teamGameCache = tgCache
	store.gameUserCache = guCache
	store.teamUserCache = tuCache

	return store
}

// getHashPath calculates the storage path for a given index key and type.
func (s *UserIndexStore) getHashPath(key, prefix string) string {
	var hash string
	if s.masterKey != nil {
		hash = hex.EncodeToString(s.masterKey.Hash([]byte(key)))
	} else {
		h := sha256.Sum256([]byte(key))
		hash = hex.EncodeToString(h[:])
	}
	return filepath.Join(prefix, fmt.Sprintf("%s.json", hash))
}

// --- User Index Methods ---

func (s *UserIndexStore) GetUserIndex(userId string) (*UserIndex, error) {
	if idx, ok := s.userCache.Get(userId); ok {
		return idx, nil
	}
	idx, err := s.loadUserFromDisk(userId)
	if err != nil {
		if os.IsNotExist(err) {
			return &UserIndex{
				UserID:     userId,
				GameAccess: make(map[string]AccessLevel),
				TeamAccess: make(map[string]AccessLevel),
			}, nil
		}
		return nil, err
	}
	s.userCache.Add(userId, idx)
	return idx, nil
}

func (s *UserIndexStore) SetUserIndex(idx *UserIndex) {
	s.userCache.Add(idx.UserID, idx)
	s.dirtyMu.Lock()
	s.dirtyU[idx.UserID] = true
	s.dirtyMu.Unlock()
}

func (s *UserIndexStore) DeleteUserIndex(userId string) error {
	s.dirtyMu.Lock()
	delete(s.dirtyU, userId)
	s.dirtyMu.Unlock()
	s.userCache.Remove(userId)

	path := s.getHashPath(userId, "users")
	m, _ := s.muU.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	err := os.Remove(filepath.Join(s.DataDir, path))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// --- Team Games Index Methods ---

func (s *UserIndexStore) GetTeamGames(teamId string) (*TeamGamesIndex, error) {
	if idx, ok := s.teamGameCache.Get(teamId); ok {
		return idx, nil
	}
	idx, err := s.loadTeamGamesFromDisk(teamId)
	if err != nil {
		if os.IsNotExist(err) {
			return &TeamGamesIndex{TeamID: teamId, GameIDs: make(map[string]bool)}, nil
		}
		return nil, err
	}
	s.teamGameCache.Add(teamId, idx)
	return idx, nil
}

func (s *UserIndexStore) SetTeamGames(idx *TeamGamesIndex) {
	s.teamGameCache.Add(idx.TeamID, idx)
	s.dirtyMu.Lock()
	s.dirtyTG[idx.TeamID] = true
	s.dirtyMu.Unlock()
}

func (s *UserIndexStore) DeleteTeamGames(teamId string) error {
	s.dirtyMu.Lock()
	delete(s.dirtyTG, teamId)
	s.dirtyMu.Unlock()
	s.teamGameCache.Remove(teamId)

	path := s.getHashPath(teamId, "team_games")
	m, _ := s.muTG.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	err := os.Remove(filepath.Join(s.DataDir, path))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// --- Game Users Index Methods ---

func (s *UserIndexStore) GetGameUsers(gameId string) (*GameUsersIndex, error) {
	if idx, ok := s.gameUserCache.Get(gameId); ok {
		return idx, nil
	}
	idx, err := s.loadGameUsersFromDisk(gameId)
	if err != nil {
		if os.IsNotExist(err) {
			return &GameUsersIndex{GameID: gameId, UserIDs: make(map[string]bool)}, nil
		}
		return nil, err
	}
	s.gameUserCache.Add(gameId, idx)
	return idx, nil
}

func (s *UserIndexStore) SetGameUsers(idx *GameUsersIndex) {
	s.gameUserCache.Add(idx.GameID, idx)
	s.dirtyMu.Lock()
	s.dirtyGU[idx.GameID] = true
	s.dirtyMu.Unlock()
}

func (s *UserIndexStore) DeleteGameUsers(gameId string) error {
	s.dirtyMu.Lock()
	delete(s.dirtyGU, gameId)
	s.dirtyMu.Unlock()
	s.gameUserCache.Remove(gameId)

	path := s.getHashPath(gameId, "game_users")
	m, _ := s.muGU.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	err := os.Remove(filepath.Join(s.DataDir, path))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// --- Team Users Index Methods ---

func (s *UserIndexStore) GetTeamUsers(teamId string) (*TeamUsersIndex, error) {
	if idx, ok := s.teamUserCache.Get(teamId); ok {
		return idx, nil
	}
	idx, err := s.loadTeamUsersFromDisk(teamId)
	if err != nil {
		if os.IsNotExist(err) {
			return &TeamUsersIndex{TeamID: teamId, UserIDs: make(map[string]bool)}, nil
		}
		return nil, err
	}
	s.teamUserCache.Add(teamId, idx)
	return idx, nil
}

func (s *UserIndexStore) SetTeamUsers(idx *TeamUsersIndex) {
	s.teamUserCache.Add(idx.TeamID, idx)
	s.dirtyMu.Lock()
	s.dirtyTU[idx.TeamID] = true
	s.dirtyMu.Unlock()
}

func (s *UserIndexStore) DeleteTeamUsers(teamId string) error {
	s.dirtyMu.Lock()
	delete(s.dirtyTU, teamId)
	s.dirtyMu.Unlock()
	s.teamUserCache.Remove(teamId)

	path := s.getHashPath(teamId, "team_users")
	m, _ := s.muTU.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	err := os.Remove(filepath.Join(s.DataDir, path))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// --- Persistence Methods ---

func (s *UserIndexStore) FlushAll() error {
	s.dirtyMu.Lock()
	users := make([]string, 0, len(s.dirtyU))
	for k := range s.dirtyU {
		users = append(users, k)
	}
	teamGames := make([]string, 0, len(s.dirtyTG))
	for k := range s.dirtyTG {
		teamGames = append(teamGames, k)
	}
	gameUsers := make([]string, 0, len(s.dirtyGU))
	for k := range s.dirtyGU {
		gameUsers = append(gameUsers, k)
	}
	teamUsers := make([]string, 0, len(s.dirtyTU))
	for k := range s.dirtyTU {
		teamUsers = append(teamUsers, k)
	}
	s.dirtyMu.Unlock()

	for _, id := range users {
		s.saveUserToDisk(id)
	}
	for _, id := range teamGames {
		s.saveTeamGamesToDisk(id)
	}
	for _, id := range gameUsers {
		s.saveGameUsersToDisk(id)
	}
	for _, id := range teamUsers {
		s.saveTeamUsersToDisk(id)
	}
	return nil
}

// -- Load/Save Helpers --

// Persist helpers (internal, takes object)
func (s *UserIndexStore) persistUserIndex(idx *UserIndex) error {
	path := s.getHashPath(idx.UserID, "users")
	m, _ := s.muU.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()
	return s.storage.SaveDataFile(path, idx)
}

func (s *UserIndexStore) persistTeamGamesIndex(idx *TeamGamesIndex) error {
	path := s.getHashPath(idx.TeamID, "team_games")
	m, _ := s.muTG.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()
	return s.storage.SaveDataFile(path, idx)
}

func (s *UserIndexStore) persistGameUsersIndex(idx *GameUsersIndex) error {
	path := s.getHashPath(idx.GameID, "game_users")
	m, _ := s.muGU.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()
	return s.storage.SaveDataFile(path, idx)
}

func (s *UserIndexStore) persistTeamUsersIndex(idx *TeamUsersIndex) error {
	path := s.getHashPath(idx.TeamID, "team_users")
	m, _ := s.muTU.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()
	return s.storage.SaveDataFile(path, idx)
}

// Public Load/Save (handles cache/dirty logic)

func (s *UserIndexStore) loadUserFromDisk(id string) (*UserIndex, error) {
	path := s.getHashPath(id, "users")
	m, _ := s.muU.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	var idx UserIndex
	if err := s.storage.ReadDataFile(path, &idx); err != nil {
		return nil, err
	}
	if idx.GameAccess == nil {
		idx.GameAccess = make(map[string]AccessLevel)
	}
	if idx.TeamAccess == nil {
		idx.TeamAccess = make(map[string]AccessLevel)
	}
	return &idx, nil
}

func (s *UserIndexStore) saveUserToDisk(id string) error {
	s.dirtyMu.Lock()
	if !s.dirtyU[id] {
		s.dirtyMu.Unlock()
		return nil
	}
	idx, ok := s.userCache.Get(id)
	if !ok {
		s.dirtyMu.Unlock()
		return nil
	} // If evicted, it was already saved by onEvict

	delete(s.dirtyU, id)
	s.dirtyMu.Unlock()

	return s.persistUserIndex(idx)
}

func (s *UserIndexStore) loadTeamGamesFromDisk(id string) (*TeamGamesIndex, error) {
	path := s.getHashPath(id, "team_games")
	m, _ := s.muTG.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	var idx TeamGamesIndex
	if err := s.storage.ReadDataFile(path, &idx); err != nil {
		return nil, err
	}
	if idx.GameIDs == nil {
		idx.GameIDs = make(map[string]bool)
	}
	return &idx, nil
}

func (s *UserIndexStore) saveTeamGamesToDisk(id string) error {
	s.dirtyMu.Lock()
	if !s.dirtyTG[id] {
		s.dirtyMu.Unlock()
		return nil
	}
	idx, ok := s.teamGameCache.Get(id)
	if !ok {
		s.dirtyMu.Unlock()
		return nil
	}

	delete(s.dirtyTG, id)
	s.dirtyMu.Unlock()

	return s.persistTeamGamesIndex(idx)
}

func (s *UserIndexStore) loadGameUsersFromDisk(id string) (*GameUsersIndex, error) {
	path := s.getHashPath(id, "game_users")
	m, _ := s.muGU.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	var idx GameUsersIndex
	if err := s.storage.ReadDataFile(path, &idx); err != nil {
		return nil, err
	}
	if idx.UserIDs == nil {
		idx.UserIDs = make(map[string]bool)
	}
	return &idx, nil
}

func (s *UserIndexStore) saveGameUsersToDisk(id string) error {
	s.dirtyMu.Lock()
	if !s.dirtyGU[id] {
		s.dirtyMu.Unlock()
		return nil
	}
	idx, ok := s.gameUserCache.Get(id)
	if !ok {
		s.dirtyMu.Unlock()
		return nil
	}

	delete(s.dirtyGU, id)
	s.dirtyMu.Unlock()

	return s.persistGameUsersIndex(idx)
}

func (s *UserIndexStore) loadTeamUsersFromDisk(id string) (*TeamUsersIndex, error) {
	path := s.getHashPath(id, "team_users")
	m, _ := s.muTU.LoadOrStore(path, &sync.Mutex{})
	mutex := m.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	var idx TeamUsersIndex
	if err := s.storage.ReadDataFile(path, &idx); err != nil {
		return nil, err
	}
	if idx.UserIDs == nil {
		idx.UserIDs = make(map[string]bool)
	}
	return &idx, nil
}

func (s *UserIndexStore) saveTeamUsersToDisk(id string) error {
	s.dirtyMu.Lock()
	if !s.dirtyTU[id] {
		s.dirtyMu.Unlock()
		return nil
	}
	idx, ok := s.teamUserCache.Get(id)
	if !ok {
		s.dirtyMu.Unlock()
		return nil
	}

	delete(s.dirtyTU, id)
	s.dirtyMu.Unlock()

	return s.persistTeamUsersIndex(idx)
}

// Invalidation
func (s *UserIndexStore) InvalidateUser(id string)      { s.userCache.Remove(id) }
func (s *UserIndexStore) InvalidateTeamGames(id string) { s.teamGameCache.Remove(id) }
func (s *UserIndexStore) InvalidateGameUsers(id string) { s.gameUserCache.Remove(id) }
func (s *UserIndexStore) InvalidateTeamUsers(id string) { s.teamUserCache.Remove(id) }

// --- Snapshot Helpers ---

func (s *UserIndexStore) listIndexFiles(subDir string) ([]string, error) {
	dir := filepath.Join(s.DataDir, subDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			files = append(files, filepath.Join(subDir, e.Name()))
		}
	}
	return files, nil
}

func (s *UserIndexStore) ListUserIndexFiles() ([]string, error) {
	return s.listIndexFiles("users")
}

func (s *UserIndexStore) ListTeamGamesFiles() ([]string, error) {
	return s.listIndexFiles("team_games")
}

func (s *UserIndexStore) ListGameUsersFiles() ([]string, error) {
	return s.listIndexFiles("game_users")
}

func (s *UserIndexStore) ListTeamUsersFiles() ([]string, error) {
	return s.listIndexFiles("team_users")
}

func (s *UserIndexStore) ListAllUserIndices() ([]*UserIndex, error) {
	return s.ListAllUserIndicesWithDirty()
}

func (s *UserIndexStore) ListAllUserIndicesWithDirty() ([]*UserIndex, error) {
	s.dirtyMu.Lock()
	dirtySet := make(map[string]bool, len(s.dirtyU))
	for k := range s.dirtyU {
		dirtySet[k] = true
	}
	s.dirtyMu.Unlock()

	// 1. Scan Disk
	dir := filepath.Join(s.DataDir, "users")
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	resMap := make(map[string]*UserIndex)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			var idx UserIndex
			if err := s.storage.ReadDataFile(filepath.Join("users", e.Name()), &idx); err == nil {
				resMap[idx.UserID] = &idx
			}
		}
	}

	// 2. Merge Dirty
	for id := range dirtySet {
		if val, ok := s.userCache.Peek(id); ok {
			resMap[id] = val
		}
	}

	res := make([]*UserIndex, 0, len(resMap))
	for _, v := range resMap {
		res = append(res, v)
	}
	return res, nil
}

func (s *UserIndexStore) ListAllTeamGames() ([]*TeamGamesIndex, error) {
	s.dirtyMu.Lock()
	dirtySet := make(map[string]bool, len(s.dirtyTG))
	for k := range s.dirtyTG {
		dirtySet[k] = true
	}
	s.dirtyMu.Unlock()

	dir := filepath.Join(s.DataDir, "team_games")
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	resMap := make(map[string]*TeamGamesIndex)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			var idx TeamGamesIndex
			if err := s.storage.ReadDataFile(filepath.Join("team_games", e.Name()), &idx); err == nil {
				resMap[idx.TeamID] = &idx
			}
		}
	}

	for id := range dirtySet {
		if val, ok := s.teamGameCache.Peek(id); ok {
			resMap[id] = val
		}
	}

	res := make([]*TeamGamesIndex, 0, len(resMap))
	for _, v := range resMap {
		res = append(res, v)
	}
	return res, nil
}

func (s *UserIndexStore) ListAllGameUsers() ([]*GameUsersIndex, error) {
	s.dirtyMu.Lock()
	dirtySet := make(map[string]bool, len(s.dirtyGU))
	for k := range s.dirtyGU {
		dirtySet[k] = true
	}
	s.dirtyMu.Unlock()

	dir := filepath.Join(s.DataDir, "game_users")
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	resMap := make(map[string]*GameUsersIndex)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			var idx GameUsersIndex
			if err := s.storage.ReadDataFile(filepath.Join("game_users", e.Name()), &idx); err == nil {
				resMap[idx.GameID] = &idx
			}
		}
	}

	for id := range dirtySet {
		if val, ok := s.gameUserCache.Peek(id); ok {
			resMap[id] = val
		}
	}

	res := make([]*GameUsersIndex, 0, len(resMap))
	for _, v := range resMap {
		res = append(res, v)
	}
	return res, nil
}

func (s *UserIndexStore) ListAllTeamUsers() ([]*TeamUsersIndex, error) {
	s.dirtyMu.Lock()
	dirtySet := make(map[string]bool, len(s.dirtyTU))
	for k := range s.dirtyTU {
		dirtySet[k] = true
	}
	s.dirtyMu.Unlock()

	dir := filepath.Join(s.DataDir, "team_users")
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	resMap := make(map[string]*TeamUsersIndex)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			var idx TeamUsersIndex
			if err := s.storage.ReadDataFile(filepath.Join("team_users", e.Name()), &idx); err == nil {
				resMap[idx.TeamID] = &idx
			}
		}
	}

	for id := range dirtySet {
		if val, ok := s.teamUserCache.Peek(id); ok {
			resMap[id] = val
		}
	}

	res := make([]*TeamUsersIndex, 0, len(resMap))
	for _, v := range resMap {
		res = append(res, v)
	}
	return res, nil
}

// Restore helpers
func (s *UserIndexStore) RestoreUserIndex(idx *UserIndex) error {
	s.userCache.Remove(idx.UserID)
	return s.persistUserIndex(idx)
}
func (s *UserIndexStore) RestoreTeamGames(idx *TeamGamesIndex) error {
	s.teamGameCache.Remove(idx.TeamID)
	return s.persistTeamGamesIndex(idx)
}
func (s *UserIndexStore) RestoreGameUsers(idx *GameUsersIndex) error {
	s.gameUserCache.Remove(idx.GameID)
	return s.persistGameUsersIndex(idx)
}
func (s *UserIndexStore) RestoreTeamUsers(idx *TeamUsersIndex) error {
	s.teamUserCache.Remove(idx.TeamID)
	return s.persistTeamUsersIndex(idx)
}

// Legacy shims
func (s *UserIndexStore) Get(userId string) (*UserIndex, error) { return s.GetUserIndex(userId) }
func (s *UserIndexStore) Set(idx *UserIndex)                    { s.SetUserIndex(idx) }
func (s *UserIndexStore) Flush(userId string) error             { return s.saveUserToDisk(userId) }
