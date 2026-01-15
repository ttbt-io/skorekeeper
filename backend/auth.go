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
	"net/http"
	"strings"
)

type contextKey struct{}

// userIDKey is the context key for the authenticated user's ID (email).
// The associated value is always a string.
var userIDKey contextKey

// getUserID returns the UserID from the request context, if present.
func getUserID(r *http.Request) string {
	if val := r.Context().Value(userIDKey); val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// normalizeEmail ensures consistent casing and whitespace for User IDs.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// maskEmail obscures an email address for safe logging.
// e.g. "user@example.com" -> "u***@example.com"
func maskEmail(email string) string {
	if email == "" {
		return "<empty>"
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 || len(parts[0]) < 1 {
		return "****"
	}
	return string(parts[0][0]) + "***@" + parts[1]
}

type AccessLevel int

const (
	AccessNone AccessLevel = iota
	AccessRead
	AccessWrite
	AccessAdmin
)

// GetGameAccess calculates the effective access level for a user on a game.
func GetGameAccess(userId string, game Game, tStore *TeamStore) AccessLevel {
	userId = normalizeEmail(userId)
	ownerId := normalizeEmail(game.OwnerID)

	log.Printf("[AUTH] Checking access for user=%s, gameId=%s, gameOwner=%s", maskEmail(userId), game.ID, maskEmail(ownerId))
	// 1. Owner has full access
	if userId != "" && ownerId == userId {
		log.Printf("[AUTH] User is owner")
		return AccessAdmin
	}

	// 2. Check direct permissions
	if userId != "" && game.Permissions.Users != nil {
		for u, role := range game.Permissions.Users {
			if normalizeEmail(u) == userId {
				switch role {
				case "write":
					return AccessWrite
				case "read":
					return AccessRead
				}
			}
		}
	}

	// 3. Check Team Inheritance
	level := AccessNone
	if userId != "" {
		// Helper to check team roles
		checkTeam := func(teamId string) {
			if teamId == "" {
				return
			}
			t, err := tStore.LoadTeam(teamId)
			if err != nil {
				return
			}

			// Admin of linked team -> Admin of game
			for _, u := range t.Roles.Admins {
				if normalizeEmail(u) == userId {
					if AccessAdmin > level {
						level = AccessAdmin
					}
					return
				}
			}
			// Scorekeeper of linked team -> Write access to game
			for _, u := range t.Roles.Scorekeepers {
				if normalizeEmail(u) == userId {
					if AccessWrite > level {
						level = AccessWrite
					}
					return
				}
			}
			// Spectator of linked team -> Read access to game
			for _, u := range t.Roles.Spectators {
				if normalizeEmail(u) == userId {
					if AccessRead > level {
						level = AccessRead
					}
					return
				}
			}
		}

		checkTeam(game.AwayTeamID)
		if level < AccessAdmin {
			checkTeam(game.HomeTeamID)
		}
	}

	if level > AccessNone {
		return level
	}

	// 4. Check Public Access
	if game.Permissions.Public == "read" {
		return AccessRead
	}

	return AccessNone
}

// GetTeamAccess calculates the effective access level for a user on a team.
func GetTeamAccess(userId string, team Team) AccessLevel {
	userId = normalizeEmail(userId)
	if userId == "" {
		return AccessNone
	}
	if normalizeEmail(team.OwnerID) == userId {
		return AccessAdmin
	}

	for _, u := range team.Roles.Admins {
		if normalizeEmail(u) == userId {
			return AccessAdmin
		}
	}
	for _, u := range team.Roles.Scorekeepers {
		if normalizeEmail(u) == userId {
			return AccessWrite
		}
	}
	for _, u := range team.Roles.Spectators {
		if normalizeEmail(u) == userId {
			return AccessRead
		}
	}

	return AccessNone
}
