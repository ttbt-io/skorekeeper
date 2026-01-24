package backend

import (
	"os"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
)

func TestRegistry_Permissions(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "registry_test")
	defer os.RemoveAll(tmpDir)

	mk, _ := crypto.CreateAESMasterKeyForTest()
	s := storage.New(tmpDir, mk)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, mk)

	r := NewRegistry(gs, ts, us, true)

	owner := "owner@example.com"
	viewer := "viewer@example.com"

	// 1. Create a Game
	gameId := "game-1"
	g := &Game{
		ID:      gameId,
		OwnerID: owner,
		Permissions: Permissions{
			Users: map[string]string{
				viewer: "read",
			},
		},
	}
	gs.SaveGame(g)
	r.UpdateGame(*g)

	// 2. Verify Access
	if !r.HasGameAccess(owner, gameId) {
		t.Errorf("Owner should have access")
	}
	if !r.HasGameAccess(viewer, gameId) {
		t.Errorf("Viewer should have access")
	}
	if r.HasGameAccess("other@example.com", gameId) {
		t.Errorf("Other should NOT have access")
	}

	// 3. Update Permissions (Remove Viewer)
	g.Permissions.Users = make(map[string]string)
	gs.SaveGame(g)
	r.UpdateGame(*g)

	if r.HasGameAccess(viewer, gameId) {
		t.Errorf("Viewer should NOT have access after removal")
	}
	if !r.HasGameAccess(owner, gameId) {
		t.Errorf("Owner should still have access")
	}
}

func TestRegistry_Rebuild(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "registry_rebuild_test")
	defer os.RemoveAll(tmpDir)

	mk, _ := crypto.CreateAESMasterKeyForTest()
	s := storage.New(tmpDir, mk)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, mk)

	owner := "owner@example.com"
	gameId := "game-rebuild"

	// 1. Save data directly to Store without Registry
	g := &Game{
		ID:            gameId,
		OwnerID:       owner,
		SchemaVersion: SchemaVersionV3,
	}
	gs.SaveGame(g)

	// 2. Initialize Registry and Rebuild
	r := NewRegistry(gs, ts, us, true) // NewRegistry calls Rebuild()

	// 3. Verify access (Lazy loading from rebuild indices)
	if !r.HasGameAccess(owner, gameId) {
		t.Errorf("Owner should have access after rebuild")
	}
}

func TestRegistry_TeamAccess(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "registry_team_test")
	defer os.RemoveAll(tmpDir)

	mk, _ := crypto.CreateAESMasterKeyForTest()
	s := storage.New(tmpDir, mk)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, mk)

	r := NewRegistry(gs, ts, us, true)

	owner := "owner@example.com"
	member := "member@example.com"

	// 1. Create Team
	teamId := "team-1"
	team := &Team{
		ID:      teamId,
		OwnerID: owner,
		Roles: TeamRoles{
			Admins: []string{member},
		},
	}
	ts.SaveTeam(team)
	r.UpdateTeam(*team)

	// 2. Create Game linked to Team
	gameId := "game-team-1"
	g := &Game{
		ID:         gameId,
		OwnerID:    owner,
		HomeTeamID: teamId,
	}
	gs.SaveGame(g)
	r.UpdateGame(*g)

	// 3. Verify Access via Team
	if !r.HasGameAccess(member, gameId) {
		t.Errorf("Team member should have access to game linked to team")
	}

	// 4. Remove from Team
	team.Roles.Admins = []string{}
	ts.SaveTeam(team)
	r.UpdateTeam(*team)

	if r.HasGameAccess(member, gameId) {
		t.Errorf("Removed team member should NOT have access to game")
	}
}
