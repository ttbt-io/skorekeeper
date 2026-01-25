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
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestAccessControl(t *testing.T) {
	// Setup Mocks
	tmpDir := t.TempDir()
	s := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)
	ac := NewAccessControl(reg, "bootstrap@admin.com")

	// Test 1: No Policy (Default Allow)
	allowed, msg := ac.IsAllowed("user@example.com")
	if !allowed {
		t.Errorf("Expected allowed when no policy set, got %s", msg)
	}

	// Test 2: Bootstrap Admin
	if !ac.IsAdmin("bootstrap@admin.com") {
		t.Error("Bootstrap admin should be admin")
	}
	if ac.IsAdmin("user@example.com") {
		t.Error("Regular user should not be admin")
	}

	// Test 3: Apply Policy (Default Deny)
	policy := &UserAccessPolicy{
		DefaultPolicy:      "deny",
		DefaultDenyMessage: "Invite Only",
		Admins:             []string{"perm@admin.com"},
		Users: map[string]UserOverride{
			"allowed@user.com": {Access: "allow"},
			"banned@user.com":  {Access: "deny"},
		},
	}
	reg.UpdateAccessPolicy(policy)

	// Case A: Default Deny
	allowed, msg = ac.IsAllowed("random@user.com")
	if allowed {
		t.Error("Expected denied for random user under default deny")
	}
	if msg != "Invite Only" {
		t.Errorf("Expected 'Invite Only', got '%s'", msg)
	}

	// Case B: Explicit Allow
	allowed, _ = ac.IsAllowed("allowed@user.com")
	if !allowed {
		t.Error("Expected allowed for explicitly allowed user")
	}

	// Case C: Explicit Deny (overriding default allow if we had one, but here matches default)
	allowed, _ = ac.IsAllowed("banned@user.com")
	if allowed {
		t.Error("Expected denied for explicitly banned user")
	}

	// Case D: Permanent Admin
	if !ac.IsAdmin("perm@admin.com") {
		t.Error("Permanent admin should be admin")
	}
	allowed, _ = ac.IsAllowed("perm@admin.com")
	if !allowed {
		t.Error("Admin should be allowed")
	}

	// Test 4: Quotas
	policy.DefaultMaxGames = 2
	policy.Users["vip@user.com"] = UserOverride{MaxGames: 10}
	reg.UpdateAccessPolicy(policy)

	// Case A: Default Quota
	if err := ac.CheckGameQuota("random@user.com", 1); err != nil {
		t.Errorf("Unexpected error for count 1 (limit 2): %v", err)
	}
	if err := ac.CheckGameQuota("random@user.com", 2); err == nil {
		t.Error("Expected quota error for count 2 (limit 2)") // >= limit triggers error?
		// Check implementation: if currentCount >= limit.
		// So if limit is 2, and current is 2, we are AT limit.
		// Wait, CheckGameQuota usually checks if we can Create ONE MORE.
		// If I have 2 games, and limit is 2, can I create another? No.
		// So input 'currentCount' implies "count before creation".
		// Implementation says: if currentCount >= limit return error.
		// Correct.
	}

	// Case B: VIP Quota
	if err := ac.CheckGameQuota("vip@user.com", 5); err != nil {
		t.Errorf("Unexpected error for VIP count 5 (limit 10): %v", err)
	}
	if err := ac.CheckGameQuota("vip@user.com", 10); err == nil {
		t.Error("Expected quota error for VIP count 10 (limit 10)")
	}
}

func TestGetUserQuotas(t *testing.T) {
	tmpDir := t.TempDir()
	s := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)
	ac := NewAccessControl(reg, "admin@test.com")

	policy := &UserAccessPolicy{
		DefaultMaxGames: 5,
		DefaultMaxTeams: 3,
		Users: map[string]UserOverride{
			"vip@test.com": {MaxGames: 100, MaxTeams: 50},
		},
	}
	reg.UpdateAccessPolicy(policy)

	// Test Default Quotas
	maxGames, maxTeams := ac.GetUserQuotas("user@test.com")
	if maxGames != 5 || maxTeams != 3 {
		t.Errorf("Expected (5, 3), got (%d, %d)", maxGames, maxTeams)
	}

	// Test Override Quotas
	maxGames, maxTeams = ac.GetUserQuotas("vip@test.com")
	if maxGames != 100 || maxTeams != 50 {
		t.Errorf("Expected (100, 50), got (%d, %d)", maxGames, maxTeams)
	}
}

func TestCheckTeamQuota(t *testing.T) {
	tmpDir := t.TempDir()
	s := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)
	ac := NewAccessControl(reg, "admin@test.com")

	policy := &UserAccessPolicy{
		DefaultMaxTeams: 2,
		Users: map[string]UserOverride{
			"vip@test.com": {MaxTeams: 5},
		},
	}
	reg.UpdateAccessPolicy(policy)

	// Test Default Quota
	if err := ac.CheckTeamQuota("user@test.com", 1); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := ac.CheckTeamQuota("user@test.com", 2); err == nil {
		t.Error("Expected error for count 2 (limit 2)")
	}

	// Test VIP Quota
	if err := ac.CheckTeamQuota("vip@test.com", 4); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := ac.CheckTeamQuota("vip@test.com", 5); err == nil {
		t.Error("Expected error for count 5 (limit 5)")
	}
}
