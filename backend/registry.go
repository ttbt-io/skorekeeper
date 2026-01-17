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
	"sort"
	"strings"
	"sync"
	"time"
)

const tombstoneTTL = 30 * 24 * time.Hour

// Registry manages the global index of games and teams for all users.
// It allows efficient lookup of accessible entities without scanning all files.
type Registry struct {
	gameStore *GameStore
	teamStore *TeamStore

	mu sync.RWMutex
	// Maps for efficient discovery
	// UserID -> Set of Game IDs they can access
	userGames map[string]map[string]bool
	// UserID -> Set of Team IDs they can access
	userTeams map[string]map[string]bool
	// GameID -> OwnerID (for fast path lookup)
	gameOwners map[string]string
	// TeamID -> OwnerID
	teamOwners map[string]string
	// TeamID -> Set of Team members (all roles)
	teamMembers map[string]map[string]bool
	// TeamID -> Set of Game IDs referencing it
	teamGames map[string]map[string]bool

	// Metadata Cache for Sorting/Filtering
	gameMetadata map[string]GameMetadata
	teamMetadata map[string]TeamMetadata

	// Deleted Tombstones
	deletedGames map[string]int64
	deletedTeams map[string]int64

	// Access Policy Cache
	accessPolicy *UserAccessPolicy
}

// NewRegistry creates a new Registry and rebuilds the index.
func NewRegistry(gs *GameStore, ts *TeamStore) *Registry {
	r := &Registry{
		gameStore:    gs,
		teamStore:    ts,
		userGames:    make(map[string]map[string]bool),
		userTeams:    make(map[string]map[string]bool),
		gameOwners:   make(map[string]string),
		teamOwners:   make(map[string]string),
		teamMembers:  make(map[string]map[string]bool),
		teamGames:    make(map[string]map[string]bool),
		gameMetadata: make(map[string]GameMetadata),
		teamMetadata: make(map[string]TeamMetadata),
		deletedGames: make(map[string]int64),
		deletedTeams: make(map[string]int64),
	}
	// Note: We don't load policy here; FSM will push it via UpdateAccessPolicy on startup/restore.
	r.Rebuild()
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

// Rebuild reconstructs the entire index by scanning the underlying stores.
func (r *Registry) Rebuild() {
	log.Println("Registry: Rebuild started...")

	// Use local maps to build state atomically
	userGames := make(map[string]map[string]bool)
	userTeams := make(map[string]map[string]bool)
	gameOwners := make(map[string]string)
	teamOwners := make(map[string]string)
	teamMembers := make(map[string]map[string]bool)
	teamGames := make(map[string]map[string]bool)
	gameMetadata := make(map[string]GameMetadata)
	teamMetadata := make(map[string]TeamMetadata)
	deletedGames := make(map[string]int64)
	deletedTeams := make(map[string]int64)

	now := time.Now()

	// Helper closures
	addUserGame := func(userId, gameId string) {
		if userGames[userId] == nil {
			userGames[userId] = make(map[string]bool)
		}
		userGames[userId][gameId] = true
	}

	addUserTeam := func(userId, teamId string) {
		if userTeams[userId] == nil {
			userTeams[userId] = make(map[string]bool)
		}
		userTeams[userId][teamId] = true
	}

	addTeamGamesToIndex := func(teamId, gameId string) {
		if teamId == "" {
			return
		}
		if teamGames[teamId] == nil {
			teamGames[teamId] = make(map[string]bool)
		}
		teamGames[teamId][gameId] = true

		members := teamMembers[teamId]
		for u := range members {
			addUserGame(u, gameId)
		}
	}

	// 1. Index Teams
	for t, err := range r.teamStore.ListAllTeamMetadata() {
		if err != nil {
			log.Printf("Registry: Error listing teams: %v", err)
			break
		}

		if t.Status == "deleted" {
			// Garbage Collection
			deletedTime := time.Unix(0, t.DeletedAt)
			if now.Sub(deletedTime) > tombstoneTTL {
				log.Printf("Registry: Purging expired team tombstone %s", t.ID)
				if err := r.teamStore.PurgeTeam(t.ID); err != nil {
					log.Printf("Registry: Failed to purge team %s: %v", t.ID, err)
				}
				continue
			}
			deletedTeams[t.ID] = t.DeletedAt
			continue
		}

		// Cache Metadata
		teamMetadata[t.ID] = t

		// Inlined indexTeamMetadata logic
		teamOwners[t.ID] = t.OwnerID
		addUserTeam(t.OwnerID, t.ID)

		members := make(map[string]bool)
		members[t.OwnerID] = true
		for _, u := range t.Roles.Admins {
			addUserTeam(u, t.ID)
			members[u] = true
		}
		for _, u := range t.Roles.Scorekeepers {
			addUserTeam(u, t.ID)
			members[u] = true
		}
		for _, u := range t.Roles.Spectators {
			addUserTeam(u, t.ID)
			members[u] = true
		}
		teamMembers[t.ID] = members
	}

	// 2. Index Games
	for g, err := range r.gameStore.ListAllGameMetadata() {
		if err != nil {
			log.Printf("Registry: Error listing games: %v", err)
			break
		}

		if g.Status == "deleted" {
			// Garbage Collection
			deletedTime := time.Unix(0, g.DeletedAt)
			if now.Sub(deletedTime) > tombstoneTTL {
				log.Printf("Registry: Purging expired game tombstone %s", g.ID)
				if err := r.gameStore.PurgeGame(g.ID); err != nil {
					log.Printf("Registry: Failed to purge game %s: %v", g.ID, err)
				}
				continue
			}
			deletedGames[g.ID] = g.DeletedAt
			continue
		}

		// Cache Metadata
		gameMetadata[g.ID] = g

		// Inlined indexGameMetadata logic
		gameOwners[g.ID] = g.OwnerID
		addUserGame(g.OwnerID, g.ID)

		for u := range g.Permissions.Users {
			addUserGame(u, g.ID)
		}

		if g.Permissions.Public != "" {
			addUserGame("", g.ID)
		}

		addTeamGamesToIndex(g.AwayTeamID, g.ID)
		addTeamGamesToIndex(g.HomeTeamID, g.ID)
	}

	// Swap safely
	r.mu.Lock()
	r.userGames = userGames
	r.userTeams = userTeams
	r.gameOwners = gameOwners
	r.teamOwners = teamOwners
	r.teamMembers = teamMembers
	r.teamGames = teamGames
	r.gameMetadata = gameMetadata
	r.teamMetadata = teamMetadata
	r.deletedGames = deletedGames
	r.deletedTeams = deletedTeams
	r.mu.Unlock()

	log.Printf("Registry: Rebuild complete. Indexed %d games, %d teams, %d deleted games, %d deleted teams.",
		len(gameOwners), len(teamOwners), len(deletedGames), len(deletedTeams))
}

// indexTeamMetadata adds a team's ownership and memberships to the index.
func (r *Registry) indexTeamMetadata(t TeamMetadata) {
	if t.Status == "deleted" {
		r.deletedTeams[t.ID] = t.DeletedAt
		delete(r.teamMetadata, t.ID)
		return
	}
	// Ensure removed from deleted set if it was there (undelete?)
	delete(r.deletedTeams, t.ID)
	r.teamMetadata[t.ID] = t

	r.teamOwners[t.ID] = t.OwnerID
	r.addUserTeam(t.OwnerID, t.ID)

	// Index Roles
	members := make(map[string]bool)
	members[t.OwnerID] = true
	for _, u := range t.Roles.Admins {
		r.addUserTeam(u, t.ID)
		members[u] = true
	}
	for _, u := range t.Roles.Scorekeepers {
		r.addUserTeam(u, t.ID)
		members[u] = true
	}
	for _, u := range t.Roles.Spectators {
		r.addUserTeam(u, t.ID)
		members[u] = true
	}
	r.teamMembers[t.ID] = members
}

// indexGameMetadata adds a game's ownership and permissions to the index.
func (r *Registry) indexGameMetadata(g GameMetadata) {
	if g.Status == "deleted" {
		r.deletedGames[g.ID] = g.DeletedAt
		delete(r.gameMetadata, g.ID)
		return
	}
	delete(r.deletedGames, g.ID)
	r.gameMetadata[g.ID] = g

	r.gameOwners[g.ID] = g.OwnerID
	r.addUserGame(g.OwnerID, g.ID)

	// Index Ad-hoc Users
	for u := range g.Permissions.Users {
		r.addUserGame(u, g.ID)
	}

	// Index Public Access
	if g.Permissions.Public != "" {
		r.addUserGame("", g.ID)
	}

	// Index users who have access via linked teams
	r.addTeamGamesToIndex(g.AwayTeamID, g.ID)
	r.addTeamGamesToIndex(g.HomeTeamID, g.ID)
}

// indexTeam adds a team's ownership and memberships to the index.
// Must be called with r.mu locked.
func (r *Registry) indexTeam(t Team) {
	r.indexTeamMetadata(TeamMetadata{
		ID:        t.ID,
		Name:      t.Name,
		OwnerID:   t.OwnerID,
		Roles:     t.Roles,
		UpdatedAt: t.UpdatedAt,
		Status:    t.Status,
		DeletedAt: t.DeletedAt,
	})
}

// indexGame adds a game's ownership and permissions to the index.
// Must be called with r.mu locked.
func (r *Registry) indexGame(g Game) {
	r.indexGameMetadata(GameMetadata{
		ID:            g.ID,
		SchemaVersion: g.SchemaVersion,
		Date:          g.Date,
		Location:      g.Location,
		Event:         g.Event,
		Away:          g.Away,
		Home:          g.Home,
		OwnerID:       g.OwnerID,
		Permissions:   g.Permissions,
		AwayTeamID:    g.AwayTeamID,
		HomeTeamID:    g.HomeTeamID,
		Status:        g.Status,
		DeletedAt:     g.DeletedAt,
	})
}

func (r *Registry) addUserGame(userId, gameId string) {
	if r.userGames[userId] == nil {
		r.userGames[userId] = make(map[string]bool)
	}
	r.userGames[userId][gameId] = true
}

func (r *Registry) addUserTeam(userId, teamId string) {
	if r.userTeams[userId] == nil {
		r.userTeams[userId] = make(map[string]bool)
	}
	r.userTeams[userId][teamId] = true
}

func (r *Registry) addTeamGamesToIndex(teamId, gameId string) {
	if teamId == "" {
		return
	}
	if r.teamGames[teamId] == nil {
		r.teamGames[teamId] = make(map[string]bool)
	}
	r.teamGames[teamId][gameId] = true

	members := r.teamMembers[teamId]
	for u := range members {
		r.addUserGame(u, gameId)
	}
}

// containsCaseInsensitive checks if s contains substr, case-insensitive.
func containsCaseInsensitive(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// ListGames returns the IDs of all games accessible by the user, sorted and filtered.
func (r *Registry) ListGames(userId, sortBy, order, query string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var ids []string
	for id := range r.userGames[userId] {
		// Filter
		if query != "" {
			meta, ok := r.gameMetadata[id]
			if !ok {
				continue
			}
			match := containsCaseInsensitive(meta.Event, query) ||
				containsCaseInsensitive(meta.Location, query) ||
				containsCaseInsensitive(meta.Away, query) ||
				containsCaseInsensitive(meta.Home, query)
			if !match {
				continue
			}
		}
		ids = append(ids, id)
	}

	// Sort
	// Default: Date Desc
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

	sort.Slice(ids, func(i, j int) bool {
		id1, id2 := ids[i], ids[j]
		m1, ok1 := r.gameMetadata[id1]
		m2, ok2 := r.gameMetadata[id2]

		if !ok1 || !ok2 {
			return id1 < id2 // Fallback to ID
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
			less = m1.Event < m2.Event
		case "location":
			less = m1.Location < m2.Location
		default:
			less = id1 < id2
		}

		if order == "desc" {
			return !less
		}
		return less
	})

	return ids
}

// ListTeams returns the IDs of all teams accessible by the user, sorted and filtered.
func (r *Registry) ListTeams(userId, sortBy, order, query string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var ids []string
	for id := range r.userTeams[userId] {
		// Filter
		if query != "" {
			meta, ok := r.teamMetadata[id]
			if !ok {
				continue
			}
			if !containsCaseInsensitive(meta.Name, query) {
				continue
			}
		}
		ids = append(ids, id)
	}

	// Sort
	// Default: Name Asc
	if sortBy == "" {
		sortBy = "name"
	}
	if order == "" {
		order = "asc"
	}

	sort.Slice(ids, func(i, j int) bool {
		id1, id2 := ids[i], ids[j]
		m1, ok1 := r.teamMetadata[id1]
		m2, ok2 := r.teamMetadata[id2]

		if !ok1 || !ok2 {
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
			less = m1.UpdatedAt < m2.UpdatedAt
		default:
			less = id1 < id2
		}

		if order == "desc" {
			return !less
		}
		return less
	})

	return ids
}

// UpdateTeam re-indexes a specific team and affected games.
func (r *Registry) UpdateTeam(t Team) {
	r.mu.Lock()
	// 1. Identify users who were in the team before but might be removed now
	oldMembers := make(map[string]bool)
	if m, ok := r.teamMembers[t.ID]; ok {
		for u := range m {
			oldMembers[u] = true
		}
	}

	// 2. Re-index the team (this updates r.teamMembers[t.ID] with current members)
	r.indexTeam(t)

	newMembers := r.teamMembers[t.ID]

	// 3. Find users who were REMOVED
	removedUsers := make([]string, 0)
	for u := range oldMembers {
		if !newMembers[u] {
			removedUsers = append(removedUsers, u)
		}
	}

	affectedGames := make([]string, 0)
	for gId := range r.teamGames[t.ID] {
		affectedGames = append(affectedGames, gId)
	}
	r.mu.Unlock()

	// 4. For removed users, we must re-evaluate their access to all games linked to this team.
	// If they no longer have access via other means, remove them from userGames map.
	for _, gId := range affectedGames {
		// Load each affected game ONCE
		g, err := r.gameStore.LoadGame(gId)
		if err != nil {
			continue
		}

		r.mu.Lock()
		for _, u := range removedUsers {
			stillHasAccess := false
			if normalizeEmail(g.OwnerID) == u {
				stillHasAccess = true
			} else if g.Permissions.Users != nil {
				for gu, role := range g.Permissions.Users {
					if normalizeEmail(gu) == u && role != "" {
						stillHasAccess = true
						break
					}
				}
			}

			if !stillHasAccess {
				// Check OTHER teams linked to this game
				otherTeams := []string{g.AwayTeamID, g.HomeTeamID}
				for _, otId := range otherTeams {
					if otId == "" || otId == t.ID {
						continue
					}
					if members, ok := r.teamMembers[otId]; ok && members[u] {
						stillHasAccess = true
						break
					}
				}
			}

			if !stillHasAccess {
				if r.userGames[u] != nil {
					delete(r.userGames[u], gId)
				}
			}
		}
		r.mu.Unlock()
	}

	// 5. Cleanup userTeams for removed users
	for _, u := range removedUsers {
		r.mu.Lock()
		if r.userTeams[u] != nil {
			delete(r.userTeams[u], t.ID)
		}
		r.mu.Unlock()
	}

	// 6. Re-index all games that use this team to ensure NEW members gain access
	for _, gId := range affectedGames {
		g, err := r.gameStore.LoadGame(gId)
		if err != nil {
			continue
		}
		r.mu.Lock()
		r.indexGame(*g)
		r.mu.Unlock()
	}
}

// UpdateGame re-indexes a specific game.
func (r *Registry) UpdateGame(g Game) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.indexGame(g)
}

// DeleteGame removes a game from the index and marks it as deleted.
func (r *Registry) DeleteGame(gameId string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deletedGames[gameId] = time.Now().UnixNano()
	delete(r.gameMetadata, gameId)

	delete(r.gameOwners, gameId)
	for _, games := range r.userGames {
		delete(games, gameId)
	}
	for _, games := range r.teamGames {
		delete(games, gameId)
	}
}

// DeleteTeam removes a team from the index and marks it as deleted.
func (r *Registry) DeleteTeam(teamId string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deletedTeams[teamId] = time.Now().UnixNano()
	delete(r.teamMetadata, teamId)

	delete(r.teamOwners, teamId)
	delete(r.teamMembers, teamId)
	delete(r.teamGames, teamId)
	for _, teams := range r.userTeams {
		delete(teams, teamId)
	}
}

// IsGameDeleted checks if a game ID has a deletion tombstone.
func (r *Registry) IsGameDeleted(gameId string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.deletedGames[gameId]
	return ok
}

// IsTeamDeleted checks if a team ID has a deletion tombstone.
func (r *Registry) IsTeamDeleted(teamId string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.deletedTeams[teamId]
	return ok
}

// HasGameAccess checks if the user has read access to the game.
func (r *Registry) HasGameAccess(userId, gameId string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if games, ok := r.userGames[userId]; ok && games[gameId] {
		return true
	}
	// Also check public access
	if games, ok := r.userGames[""]; ok {
		return games[gameId]
	}
	return false
}

// HasTeamAccess checks if the user has read access to the team.
func (r *Registry) HasTeamAccess(userId, teamId string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if teams, ok := r.userTeams[userId]; ok {
		return teams[teamId]
	}
	return false
}

// GameExists checks if a game ID is known to the registry.
func (r *Registry) GameExists(gameId string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.gameOwners[gameId]
	return ok
}

// TeamExists checks if a team ID is known to the registry.
func (r *Registry) TeamExists(teamId string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.teamOwners[teamId]
	return ok
}

// CountOwnedGames returns the number of games owned by the user.
func (r *Registry) CountOwnedGames(userId string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	// Iterate only over games the user can access to find ones they own
	for gId := range r.userGames[userId] {
		if r.gameOwners[gId] == userId {
			count++
		}
	}
	return count
}

// CountOwnedTeams returns the number of teams owned by the user.
func (r *Registry) CountOwnedTeams(userId string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for tId := range r.userTeams[userId] {
		if r.teamOwners[tId] == userId {
			count++
		}
	}
	return count
}
