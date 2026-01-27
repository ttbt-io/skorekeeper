package backend

import (
	"os"
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestFSMLoadAccessPolicyOnStartup(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "access_policy_test")
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)

	// 1. Save Policy manually
	policy := UserAccessPolicy{
		DefaultPolicy: "deny",
		Admins:        []string{"admin@example.com"},
	}
	if err := s.SaveDataFile("sys_access_policy", &policy); err != nil {
		t.Fatalf("Failed to save mock policy: %v", err)
	}

	// 2. Initialize FSM dependencies
	gs := NewGameStore(tempDir, s)
	ts := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	r := NewRegistry(gs, ts, us, false)

	// 3. Initialize FSM (Should load policy)
	NewFSM(gs, ts, r, nil, s, us)

	// 4. Verify
	loaded := r.GetAccessPolicy()
	if loaded == nil {
		t.Fatal("Access policy was not loaded on startup")
	}
	if loaded.DefaultPolicy != "deny" {
		t.Errorf("Expected DefaultPolicy='deny', got '%s'", loaded.DefaultPolicy)
	}
	if len(loaded.Admins) != 1 || loaded.Admins[0] != "admin@example.com" {
		t.Errorf("Admins mismatch: %v", loaded.Admins)
	}
}
