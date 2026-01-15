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
	"fmt"
	"strings"
	"sync"
)

// AccessControl manages user permissions and quotas.
type AccessControl struct {
	r *Registry
	// Bootstrap admin email (from flag)
	bootstrapAdmin string
	mu             sync.RWMutex
}

// NewAccessControl creates a new AccessControl service.
func NewAccessControl(r *Registry, bootstrapAdmin string) *AccessControl {
	return &AccessControl{
		r:              r,
		bootstrapAdmin: strings.ToLower(bootstrapAdmin),
	}
}

// IsAllowed checks if a user is allowed to access the service.
// Returns allowed status and a denial message (if denied).
func (ac *AccessControl) IsAllowed(email string) (bool, string) {
	if email == "" {
		return false, "Authentication required"
	}
	email = strings.ToLower(email)

	// 1. Check Bootstrap Admin
	if ac.bootstrapAdmin != "" && email == ac.bootstrapAdmin {
		return true, ""
	}

	policy := ac.r.GetAccessPolicy()
	if policy == nil {
		// Default open.
		return true, ""
	}

	// 2. Check Permanent Admins
	for _, admin := range policy.Admins {
		if strings.EqualFold(admin, email) {
			return true, ""
		}
	}

	// 3. User Override
	if override, ok := policy.Users[email]; ok {
		if override.Access == "deny" {
			return false, policy.DefaultDenyMessage
		}
		return true, ""
	}

	// 4. Default Policy
	if policy.DefaultPolicy == "deny" {
		return false, policy.DefaultDenyMessage
	}

	return true, ""
}

// IsAdmin checks if a user has admin privileges.
func (ac *AccessControl) IsAdmin(email string) bool {
	if email == "" {
		return false
	}
	email = strings.ToLower(email)

	if ac.bootstrapAdmin != "" && email == ac.bootstrapAdmin {
		return true
	}

	policy := ac.r.GetAccessPolicy()
	if policy == nil {
		return false
	}

	for _, admin := range policy.Admins {
		if strings.EqualFold(admin, email) {
			return true
		}
	}
	return false
}

// CheckGameQuota verifies if a user can create a new game.
func (ac *AccessControl) CheckGameQuota(email string, currentCount int) error {
	policy := ac.r.GetAccessPolicy()
	if policy == nil {
		return nil // No limits
	}

	limit := policy.DefaultMaxGames
	if override, ok := policy.Users[strings.ToLower(email)]; ok {
		if override.MaxGames != 0 {
			limit = override.MaxGames
		}
	}

	// A limit of 0 means unlimited. A negative limit means none.
	if limit != 0 && currentCount >= limit {
		return fmt.Errorf("game limit reached (%d)", limit)
	}
	return nil
}

// CheckTeamQuota verifies if a user can create a new team.
func (ac *AccessControl) CheckTeamQuota(email string, currentCount int) error {
	policy := ac.r.GetAccessPolicy()
	if policy == nil {
		return nil
	}

	limit := policy.DefaultMaxTeams
	if override, ok := policy.Users[strings.ToLower(email)]; ok {
		if override.MaxTeams != 0 {
			limit = override.MaxTeams
		}
	}

	// A limit of 0 means unlimited. A negative limit means none.
	if limit != 0 && currentCount >= limit {
		return fmt.Errorf("team limit reached (%d)", limit)
	}
	return nil
}

// GetUserQuotas returns the effective max games and teams for a user.
func (ac *AccessControl) GetUserQuotas(email string) (maxGames, maxTeams int) {
	policy := ac.r.GetAccessPolicy()
	if policy == nil {
		return 0, 0
	}

	maxGames = policy.DefaultMaxGames
	maxTeams = policy.DefaultMaxTeams

	if override, ok := policy.Users[strings.ToLower(email)]; ok {
		if override.MaxGames != 0 {
			maxGames = override.MaxGames
		}
		if override.MaxTeams != 0 {
			maxTeams = override.MaxTeams
		}
	}
	return
}
