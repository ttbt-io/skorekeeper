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
	"log"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/ttbt-io/skorekeeper/backend/search"
)

const tombstoneTTL = 30 * 24 * time.Hour

// Registry manages the global index of games and teams for all users.
// It allows efficient lookup of accessible entities without scanning all files.
// It relies on UserIndexStore for persistent, map-free indexing.
type Registry struct {
	gameStore *GameStore
	teamStore *TeamStore
	userStore *UserIndexStore

	mu sync.RWMutex

	// Metadata Cache for Sorting/Filtering (LRU)
	// Also acts as Tombstone cache (Status="deleted")
	gameMetadata *lru.Cache[string, GameMetadata]
	teamMetadata *lru.Cache[string, TeamMetadata]

	// Global Counts
	gameCount int
	teamCount int

	// Access Policy Cache
	accessPolicy *UserAccessPolicy
}

// NewRegistry creates a new Registry.
// If forceRebuild is true, it scans all files to rebuild indices.
// Otherwise, it trusts the persisted indices and just counts files for stats.
func NewRegistry(gs *GameStore, ts *TeamStore, us *UserIndexStore, forceRebuild bool) *Registry {
	gmCache, _ := lru.New[string, GameMetadata](5000)
	tmCache, _ := lru.New[string, TeamMetadata](2000)

	r := &Registry{
		gameStore:    gs,
		teamStore:    ts,
		userStore:    us,
		gameMetadata: gmCache,
		teamMetadata: tmCache,
	}

	if forceRebuild {
		r.Rebuild()
	} else {
		// Fast Path: Count files (Total Objects)
		if ids, err := gs.ListAllGameIDs(); err == nil {
			r.gameCount = len(ids)
		}
		if ids, err := ts.ListAllTeamIDs(); err == nil {
			r.teamCount = len(ids)
		}
		log.Printf("Registry: Fast startup. Found %d games, %d teams.", r.gameCount, r.teamCount)
	}

	return r
}

// UpdateAccessPolicy updates the cached access policy.
func (r *Registry) UpdateAccessPolicy(policy *UserAccessPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accessPolicy = policy
}

// GetAccessPolicy returns the current access policy.
func (r *Registry) GetAccessPolicy() *UserAccessPolicy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.accessPolicy
}

// Flush persists the registry state (indices).
func (r *Registry) Flush() error {
	// 1. Flush indices
	return r.userStore.FlushAll()
}

// Rebuild reconstructs the entire index by scanning the underlying stores.
func (r *Registry) Rebuild() {
	log.Println("Registry: Rebuild started...")

	r.mu.Lock()
	r.gameCount = 0
	r.teamCount = 0
	r.mu.Unlock()

	// 1. Index Teams
	for t, err := range r.teamStore.ListAllTeamMetadata() {
		if err != nil {
			log.Printf("Registry: Error listing teams: %v", err)
			break
		}
		r.indexTeam(t.ID, t, true)
	}

	// 2. Index Games
	for g, err := range r.gameStore.ListAllGameMetadata() {
		if err != nil {
			log.Printf("Registry: Error listing games: %v", err)
			break
		}
		r.indexGame(g.ID, g, true)
	}

	// 3. Persist
	if err := r.userStore.FlushAll(); err != nil {
		log.Printf("Registry: Warning: failed to flush user indices: %v", err)
	}

	r.mu.RLock()
	log.Printf("Registry: Rebuild complete. Indexed %d games, %d teams.", r.gameCount, r.teamCount)
	r.mu.RUnlock()
}

// indexTeam processes a team for indexing (Rebuild/Update).
func (r *Registry) indexTeam(teamId string, t TeamMetadata, isRebuild bool) {
	// Cache metadata (even if deleted)
	r.teamMetadata.Add(teamId, t)

	if t.Status == "deleted" {
		// Ensure user indices are cleaned up
		oldIdx, _ := r.userStore.GetTeamUsers(teamId)
		for u := range oldIdx.UserIDs {
			r.updateUserTeamAccess(u, teamId, AccessNone)
		}
		r.userStore.DeleteTeamUsers(teamId)
		// We should also probably delete TeamGamesIndex?
		// indexTeam is called during Rebuild.
		// If it's deleted, we do cleanup.
		// Rebuild logic iterates ALL teams.
		// If status is deleted, we cleanup.
		// But indexTeam doesn't touch TeamGamesIndex directly (addTeamGame does).
		// Wait, DeleteTeam calls DeleteTeamGames.
		// If indexTeam is called with deleted status (e.g. from Rebuild), should it delete TeamGames?
		// Rebuild scans all files. If it finds a deleted team, it should ensure index is gone.
		// Yes.
		r.userStore.DeleteTeamGames(teamId)
		return
	}

	// Update TeamUsersIndex
	newMembers := make(map[string]bool)
	newMembers[t.OwnerID] = true
	for _, u := range t.Roles.Admins {
		newMembers[u] = true
	}
	for _, u := range t.Roles.Scorekeepers {
		newMembers[u] = true
	}
	for _, u := range t.Roles.Spectators {
		newMembers[u] = true
	}

	oldIdx, _ := r.userStore.GetTeamUsers(teamId)
	isNew := len(oldIdx.UserIDs) == 0

	// Identify Removed
	for u := range oldIdx.UserIDs {
		if !newMembers[u] {
			r.updateUserTeamAccess(u, teamId, AccessNone)
		}
	}

	// Identify Added/Updated
	getLevel := func(u string) AccessLevel {
		if u == t.OwnerID {
			return AccessAdmin
		}
		for _, a := range t.Roles.Admins {
			if a == u {
				return AccessAdmin
			}
		}
		for _, a := range t.Roles.Scorekeepers {
			if a == u {
				return AccessWrite
			}
		}
		for _, a := range t.Roles.Spectators {
			if a == u {
				return AccessRead
			}
		}
		return AccessNone
	}

	for u := range newMembers {
		level := getLevel(u)
		r.updateUserTeamAccess(u, teamId, level)
	}

	if !maps.Equal(oldIdx.UserIDs, newMembers) {
		oldIdx.UserIDs = newMembers
		r.userStore.SetTeamUsers(oldIdx)
	}

	if isNew || isRebuild {
		r.mu.Lock()
		r.teamCount++
		r.mu.Unlock()
	}
}

// indexGame processes a game for indexing (Rebuild/Update).
func (r *Registry) indexGame(gameId string, g GameMetadata, isRebuild bool) {
	// Cache metadata (even if deleted)
	r.gameMetadata.Add(gameId, g)

	if g.Status == "deleted" {
		// Ensure user indices are cleaned up
		oldIdx, _ := r.userStore.GetGameUsers(gameId)
		for u := range oldIdx.UserIDs {
			r.updateUserGameAccess(u, gameId, AccessNone)
		}
		r.userStore.DeleteGameUsers(gameId)
		return
	}

	// Update GameUsersIndex
	newUsers := make(map[string]bool)
	newUsers[g.OwnerID] = true
	for u := range g.Permissions.Users {
		newUsers[u] = true
	}
	if g.Permissions.Public != "" {
		newUsers[""] = true
	}

	oldIdx, _ := r.userStore.GetGameUsers(gameId)
	isNew := len(oldIdx.UserIDs) == 0

	// Removed
	for u := range oldIdx.UserIDs {
		if !newUsers[u] {
			r.updateUserGameAccess(u, gameId, AccessNone)
		}
	}

	// Added/Updated
	getLevel := func(u string) AccessLevel {
		if u == g.OwnerID {
			return AccessAdmin
		}
		role, ok := g.Permissions.Users[u]
		if ok {
			switch role {
			case "admin":
				return AccessAdmin
			case "write":
				return AccessWrite
			case "read":
				return AccessRead
			}
		}
		if g.Permissions.Public != "" {
			switch g.Permissions.Public {
			case "write":
				return AccessWrite
			case "read":
				return AccessRead
			}
		}
		return AccessNone
	}

	for u := range newUsers {
		level := getLevel(u)
		r.updateUserGameAccess(u, gameId, level)
	}

	if !maps.Equal(oldIdx.UserIDs, newUsers) {
		oldIdx.UserIDs = newUsers
		r.userStore.SetGameUsers(oldIdx)
	}

	// Update TeamGamesIndex
	r.addTeamGame(g.AwayTeamID, gameId, g)
	r.addTeamGame(g.HomeTeamID, gameId, g)

	if isNew || isRebuild {
		r.mu.Lock()
		r.gameCount++
		r.mu.Unlock()
	}
}

func (r *Registry) updateUserTeamAccess(userId, teamId string, level AccessLevel) {
	idx, _ := r.userStore.GetUserIndex(userId)
	changed := false
	if level == AccessNone {
		if _, ok := idx.TeamAccess[teamId]; ok {
			delete(idx.TeamAccess, teamId)
			changed = true
		}
	} else {
		if idx.TeamAccess[teamId] != level {
			idx.TeamAccess[teamId] = level
			changed = true
		}
	}
	if changed {
		r.userStore.SetUserIndex(idx)
	}
}

func (r *Registry) updateUserGameAccess(userId, gameId string, level AccessLevel) {
	idx, _ := r.userStore.GetUserIndex(userId)
	changed := false
	if level == AccessNone {
		if _, ok := idx.GameAccess[gameId]; ok {
			delete(idx.GameAccess, gameId)
			changed = true
		}
	} else {
		if idx.GameAccess[gameId] != level {
			idx.GameAccess[gameId] = level
			changed = true
		}
	}
	if changed {
		r.userStore.SetUserIndex(idx)
	}
}

func (r *Registry) addTeamGame(teamId, gameId string, gMeta GameMetadata) {
	if teamId == "" {
		return
	}
	idx, _ := r.userStore.GetTeamGames(teamId)

	// Check change
	if !idx.GameIDs[gameId] {
		idx.GameIDs[gameId] = true
		r.userStore.SetTeamGames(idx)
	}

	// Update Members
	tuIdx, _ := r.userStore.GetTeamUsers(teamId)

	// Construct partial game for permission check
	g := Game{
		ID:          gMeta.ID,
		OwnerID:     gMeta.OwnerID,
		Permissions: gMeta.Permissions,
		AwayTeamID:  gMeta.AwayTeamID,
		HomeTeamID:  gMeta.HomeTeamID,
	}

	for u := range tuIdx.UserIDs {
		level := GetGameAccess(u, g, r.teamStore)
		r.updateUserGameAccess(u, gameId, level)
	}
}

func (r *Registry) UpdateTeam(t Team) {
	oldTeamUsers, _ := r.userStore.GetTeamUsers(t.ID)
	oldMembers := make(map[string]bool)
	for u := range oldTeamUsers.UserIDs {
		oldMembers[u] = true
	}

	r.indexTeam(t.ID, TeamMetadata{
		ID: t.ID, Name: t.Name, OwnerID: t.OwnerID, Roles: t.Roles,
		UpdatedAt: t.UpdatedAt, Status: t.Status, DeletedAt: t.DeletedAt,
	}, false)

	newTeamUsers, _ := r.userStore.GetTeamUsers(t.ID)
	removedUsers := make([]string, 0)
	for u := range oldMembers {
		if !newTeamUsers.UserIDs[u] {
			removedUsers = append(removedUsers, u)
		}
	}

	linkedGamesIdx, _ := r.userStore.GetTeamGames(t.ID)
	for gId := range linkedGamesIdx.GameIDs {
		g, err := r.gameStore.LoadGame(gId)
		if err != nil {
			continue
		}
		for _, u := range removedUsers {
			level := GetGameAccess(u, *g, r.teamStore)
			r.updateUserGameAccess(u, gId, level)
		}
		r.indexGame(gId, *g.Metadata(), false)
	}
}

func (r *Registry) UpdateGame(g Game) {
	r.indexGame(g.ID, *g.Metadata(), false)
}

func (r *Registry) DeleteGame(gameId string) {
	r.markGameDeleted(gameId, time.Now().UnixNano())
	guIdx, _ := r.userStore.GetGameUsers(gameId)
	for u := range guIdx.UserIDs {
		r.updateUserGameAccess(u, gameId, AccessNone)
	}
	r.userStore.DeleteGameUsers(gameId)
}

func (r *Registry) DeleteTeam(teamId string) {
	r.markTeamDeleted(teamId, time.Now().UnixNano())
	tuIdx, _ := r.userStore.GetTeamUsers(teamId)
	for u := range tuIdx.UserIDs {
		r.updateUserTeamAccess(u, teamId, AccessNone)
	}
	r.userStore.DeleteTeamUsers(teamId)

	tgIdx, _ := r.userStore.GetTeamGames(teamId)
	for gId := range tgIdx.GameIDs {
		g, err := r.gameStore.LoadGame(gId)
		if err != nil {
			continue
		}
		r.indexGame(gId, *g.Metadata(), false)
	}
	r.userStore.DeleteTeamGames(teamId)
}

func (r *Registry) markGameDeleted(id string, ts int64) {
	r.mu.Lock()
	r.gameCount--
	r.mu.Unlock()

	// Cache tombstone
	r.gameMetadata.Add(id, GameMetadata{
		ID: id, Status: "deleted", DeletedAt: ts,
	})
}

func (r *Registry) markTeamDeleted(id string, ts int64) {
	r.mu.Lock()
	r.teamCount--
	r.mu.Unlock()

	r.teamMetadata.Add(id, TeamMetadata{
		ID: id, Status: "deleted", DeletedAt: ts,
	})
}

func (r *Registry) IsGameDeleted(id string) bool {
	if m, ok := r.gameMetadata.Get(id); ok {
		return m.Status == "deleted"
	}
	g, err := r.gameStore.LoadGame(id)
	if err == nil {
		r.gameMetadata.Add(id, *g.Metadata())
		return g.Status == "deleted"
	}
	return false
}

func (r *Registry) IsTeamDeleted(id string) bool {
	if m, ok := r.teamMetadata.Get(id); ok {
		return m.Status == "deleted"
	}
	t, err := r.teamStore.LoadTeam(id)
	if err == nil {
		m := TeamMetadata{ID: t.ID, Status: t.Status, DeletedAt: t.DeletedAt}
		r.teamMetadata.Add(id, m)
		return t.Status == "deleted"
	}
	return false
}

func (r *Registry) HasGameAccess(userId, gameId string) bool {
	idx, err := r.userStore.GetUserIndex(userId)
	if err == nil {
		if idx.GameAccess[gameId] >= AccessRead {
			return true
		}
	}
	pIdx, err := r.userStore.GetUserIndex("")
	if err == nil {
		if pIdx.GameAccess[gameId] >= AccessRead {
			return true
		}
	}
	return false
}

func (r *Registry) HasTeamAccess(userId, teamId string) bool {
	idx, err := r.userStore.GetUserIndex(userId)
	if err == nil {
		if idx.TeamAccess[teamId] >= AccessRead {
			return true
		}
	}
	return false
}

func (r *Registry) GameExists(id string) bool {
	if m, ok := r.gameMetadata.Get(id); ok {
		return m.Status != "deleted"
	}
	g, err := r.gameStore.LoadGame(id)
	return err == nil && g.Status != "deleted"
}

func (r *Registry) TeamExists(id string) bool {
	if m, ok := r.teamMetadata.Get(id); ok {
		return m.Status != "deleted"
	}
	t, err := r.teamStore.LoadTeam(id)
	return err == nil && t.Status != "deleted"
}

func (r *Registry) CountOwnedGames(userId string) int {
	idx, err := r.userStore.GetUserIndex(userId)
	if err != nil {
		return 0
	}
	count := 0
	for gId, level := range idx.GameAccess {
		if level < AccessAdmin {
			continue
		}
		if m, ok := r.gameMetadata.Get(gId); ok {
			if m.OwnerID == userId && m.Status != "deleted" {
				count++
			}
		} else {
			if g, err := r.gameStore.LoadGame(gId); err == nil && g.Status != "deleted" {
				r.gameMetadata.Add(gId, *g.Metadata())
				if g.OwnerID == userId {
					count++
				}
			}
		}
	}
	return count
}

func (r *Registry) CountOwnedTeams(userId string) int {
	idx, err := r.userStore.GetUserIndex(userId)
	if err != nil {
		return 0
	}
	count := 0
	for tId, level := range idx.TeamAccess {
		if level < AccessAdmin {
			continue
		}
		if m, ok := r.teamMetadata.Get(tId); ok {
			if m.OwnerID == userId && m.Status != "deleted" {
				count++
			}
		} else {
			if t, err := r.teamStore.LoadTeam(tId); err == nil && t.Status != "deleted" {
				m := TeamMetadata{ID: t.ID, Name: t.Name, OwnerID: t.OwnerID, Status: t.Status}
				r.teamMetadata.Add(tId, m)
				if t.OwnerID == userId {
					count++
				}
			}
		}
	}
	return count
}

func (r *Registry) CountTotalGames() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.gameCount
}

func (r *Registry) CountTotalTeams() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.teamCount
}

func (r *Registry) ListGames(userId, sortBy, order, query string) []string {
	// Defaults
	if sortBy == "" {
		sortBy = "date"
	}
	if order == "" {
		if sortBy == "date" {
			order = "desc"
		} else {
			order = "asc"
		}
	}

	q := search.Parse(query)
	for i, t := range q.FreeText {
		q.FreeText[i] = strings.ToLower(t)
	}
	for i, f := range q.Filters {
		if f.Key != "date" {
			q.Filters[i].Value = strings.ToLower(f.Value)
		}
	}

	idx, err := r.userStore.GetUserIndex(userId)
	if err != nil {
		return []string{}
	}

	var ids []string
	getMeta := func(id string) (GameMetadata, bool) {
		if m, ok := r.gameMetadata.Get(id); ok {
			return m, true
		}
		g, err := r.gameStore.LoadGame(id)
		if err != nil {
			return GameMetadata{}, false
		}
		m := *g.Metadata()
		r.gameMetadata.Add(id, m)
		return m, true
	}

	for id := range idx.GameAccess {
		meta, ok := getMeta(id)
		if !ok || meta.Status == "deleted" || !matchesGame(meta, q) {
			continue
		}
		ids = append(ids, id)
	}

	if userId != "" {
		pIdx, err := r.userStore.GetUserIndex("")
		if err == nil {
			for id := range pIdx.GameAccess {
				if _, ok := idx.GameAccess[id]; ok {
					continue
				}
				meta, ok := getMeta(id)
				if !ok || meta.Status == "deleted" || !matchesGame(meta, q) {
					continue
				}
				ids = append(ids, id)
			}
		}
	}

	sort.Slice(ids, func(i, j int) bool {
		id1, id2 := ids[i], ids[j]
		m1, ok1 := getMeta(id1)
		m2, ok2 := getMeta(id2)
		if !ok1 || !ok2 {
			if order == "desc" {
				return id1 > id2
			}
			return id1 < id2
		}
		var less bool
		switch sortBy {
		case "date":
			if m1.Date != m2.Date {
				less = m1.Date < m2.Date
			} else {
				less = id1 < id2
			}
		case "event":
			if m1.Event != m2.Event {
				less = m1.Event < m2.Event
			} else {
				less = id1 < id2
			}
		case "location":
			if m1.Location != m2.Location {
				less = m1.Location < m2.Location
			} else {
				less = id1 < id2
			}
		default:
			less = id1 < id2
		}
		if order == "desc" {
			switch sortBy {
			case "date":
				if m1.Date != m2.Date {
					return m1.Date > m2.Date
				}
				return id1 > id2
			case "event":
				if m1.Event != m2.Event {
					return m1.Event > m2.Event
				}
				return id1 > id2
			case "location":
				if m1.Location != m2.Location {
					return m1.Location > m2.Location
				}
				return id1 > id2
			default:
				return id1 > id2
			}
		}
		return less
	})
	return ids
}

func (r *Registry) ListTeams(userId, sortBy, order, query string) []string {
	// Defaults
	if sortBy == "" {
		sortBy = "name"
	}
	if order == "" {
		order = "asc"
	}

	q := search.Parse(query)
	for i, t := range q.FreeText {
		q.FreeText[i] = strings.ToLower(t)
	}
	for i, f := range q.Filters {
		q.Filters[i].Value = strings.ToLower(f.Value)
	}

	idx, err := r.userStore.GetUserIndex(userId)
	if err != nil {
		return []string{}
	}

	var ids []string
	getMeta := func(id string) (TeamMetadata, bool) {
		if m, ok := r.teamMetadata.Get(id); ok {
			return m, true
		}
		t, err := r.teamStore.LoadTeam(id)
		if err != nil {
			return TeamMetadata{}, false
		}
		m := TeamMetadata{ID: t.ID, Name: t.Name, OwnerID: t.OwnerID, Roles: t.Roles, UpdatedAt: t.UpdatedAt, Status: t.Status, DeletedAt: t.DeletedAt}
		r.teamMetadata.Add(id, m)
		return m, true
	}

	for id := range idx.TeamAccess {
		meta, ok := getMeta(id)
		if !ok || meta.Status == "deleted" || !matchesTeam(meta, q) {
			continue
		}
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		id1, id2 := ids[i], ids[j]
		m1, ok1 := getMeta(id1)
		m2, ok2 := getMeta(id2)
		if !ok1 || !ok2 {
			if order == "desc" {
				return id1 > id2
			}
			return id1 < id2
		}
		var less bool
		switch sortBy {
		case "name":
			if m1.Name != m2.Name {
				less = m1.Name < m2.Name
			} else {
				less = id1 < id2
			}
		case "updated":
			if m1.UpdatedAt != m2.UpdatedAt {
				less = m1.UpdatedAt < m2.UpdatedAt
			} else {
				less = id1 < id2
			}
		default:
			less = id1 < id2
		}
		if order == "desc" {
			switch sortBy {
			case "name":
				if m1.Name != m2.Name {
					return m1.Name > m2.Name
				}
				return id1 > id2
			case "updated":
				if m1.UpdatedAt != m2.UpdatedAt {
					return m1.UpdatedAt > m2.UpdatedAt
				}
				return id1 > id2
			default:
				return id1 > id2
			}
		}
		return less
	})
	return ids
}

// --- Search Helpers ---

func containsLower(s, substrLower string) bool {
	return strings.Contains(strings.ToLower(s), substrLower)
}

func matchesGame(m GameMetadata, q search.Query) bool {
	for _, token := range q.FreeText {
		match := containsLower(m.Event, token) ||
			containsLower(m.Location, token) ||
			containsLower(m.Away, token) ||
			containsLower(m.Home, token)
		if !match {
			return false
		}
	}
	for _, f := range q.Filters {
		switch f.Key {
		case "event":
			if !containsLower(m.Event, f.Value) {
				return false
			}
		case "location":
			if !containsLower(m.Location, f.Value) {
				return false
			}
		case "away":
			if !containsLower(m.Away, f.Value) {
				return false
			}
		case "home":
			if !containsLower(m.Home, f.Value) {
				return false
			}
		case "date":
			if !checkDateFilter(m.Date, f) {
				return false
			}
		}
	}
	return true
}

func matchesTeam(m TeamMetadata, q search.Query) bool {
	for _, token := range q.FreeText {
		if !containsLower(m.Name, token) {
			return false
		}
	}
	for _, f := range q.Filters {
		switch f.Key {
		case "name":
			if !containsLower(m.Name, f.Value) {
				return false
			}
		}
	}
	return true
}

func checkDateFilter(dateVal string, f search.Filter) bool {
	switch f.Operator {
	case search.OpEqual:
		return strings.HasPrefix(dateVal, f.Value)
	case search.OpGreater:
		return dateVal > f.Value
	case search.OpGreaterOrEqual:
		return dateVal >= f.Value
	case search.OpLess:
		return dateVal < f.Value
	case search.OpLessOrEqual:
		return dateVal <= f.Value
	case search.OpRange:
		maxVal := f.MaxValue + "~"
		return dateVal >= f.Value && dateVal <= maxVal
	}
	return true
}
