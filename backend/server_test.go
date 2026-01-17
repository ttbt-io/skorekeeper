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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
)

func TestGameStore(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gamestore_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	store := NewGameStore(tempDir, s)
	gameId := "11111111-1111-4111-8111-111111111111"
	game := Game{SchemaVersion: SchemaVersionV3, ID: gameId, Date: "2025-01-01"}

	// Test SaveGame
	t.Run("SaveGame", func(t *testing.T) {
		err := store.SaveGame(&game)
		if err != nil {
			t.Errorf("SaveGame failed: %v", err)
		}

		// Verify file exists
		expectedPath := filepath.Join(tempDir, "games", gameId+".json")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("Game file not created at %s", expectedPath)
		}
	})

	// Test LoadGame
	t.Run("LoadGame", func(t *testing.T) {
		loaded, err := store.LoadGame(gameId)
		if err != nil {
			t.Fatalf("LoadGame failed: %v", err)
		}

		if loaded.ID != gameId {
			t.Errorf("Loaded data mismatch. Got %v, want %v", loaded.ID, gameId)
		}
	})

	// Test LoadGameAsJSON
	t.Run("LoadGameAsJSON", func(t *testing.T) {
		data, err := store.LoadGameAsJSON(gameId)
		if err != nil {
			t.Fatalf("LoadGameAsJSON failed: %v", err)
		}
		if len(data) == 0 {
			t.Error("LoadGameAsJSON returned empty data")
		}
		var g Game
		if err := json.Unmarshal(data, &g); err != nil {
			t.Errorf("Failed to unmarshal JSON data: %v", err)
		}
		if g.ID != gameId {
			t.Errorf("JSON data mismatch. Got %v, want %v", g.ID, gameId)
		}
	})

	// Test LoadGame Not Found
	t.Run("LoadGameNotFound", func(t *testing.T) {
		_, err := store.LoadGame("33333333-3333-4333-8333-333333333333")
		if !os.IsNotExist(err) {
			t.Errorf("Expected os.ErrNotExist, got %v", err)
		}
	})

	// Test DeleteGame
	t.Run("DeleteGame", func(t *testing.T) {
		err := store.DeleteGame(gameId)
		if err != nil {
			t.Fatalf("DeleteGame failed: %v", err)
		}

		// Verify file still exists but is marked deleted
		loaded, err := store.LoadGame(gameId)
		if err != nil {
			t.Fatalf("LoadGame failed: %v", err)
		}
		if loaded.Status != "deleted" {
			t.Errorf("Expected status 'deleted', got %s", loaded.Status)
		}
	})

	// Test PurgeGame
	t.Run("PurgeGame", func(t *testing.T) {
		err := store.PurgeGame(gameId)
		if err != nil {
			t.Fatalf("PurgeGame failed: %v", err)
		}

		// Verify file is gone
		expectedPath := filepath.Join(tempDir, "games", gameId+".json")
		if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
			t.Errorf("Game file still exists after purge at %s", expectedPath)
		}
	})
}

func TestHTTPHandlers(t *testing.T) {
	// Setup with temp dir
	tempDir, err := os.MkdirTemp("", "http_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	reg := NewRegistry(gStore, tStore)

	// Setup Handler using the factory function
	_, handler := NewServerHandler(Options{
		GameStore:   gStore,
		TeamStore:   tStore,
		Storage:     s,
		Registry:    reg,
		UseMockAuth: true,
	})

	userId := "user1@example.com"
	validGameId := "11111111-1111-4111-8111-111111111111"

	// Helper to make authenticated requests
	makeRequest := func(method, url, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, url, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}

	// Test Save Handler (Bootstrap - New Game)
	t.Run("SaveHandlerNewGame", func(t *testing.T) {
		// Minimum valid game data for ValidateGameData
		game := Game{
			ID:            validGameId,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       userId,
			Location:      "Test Field",
			Event:         "Test Event",
			ActionLog: []json.RawMessage{

				json.RawMessage(`{"id":"aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa","timestamp":123456789,"type":"GAME_START","payload":{"id":"` + validGameId + `","date":"2025-12-18T14:57:39Z","away":"A","home":"B"}}`),
			},
		}
		body, _ := json.Marshal(game)
		w := makeRequest("POST", "/api/save", string(body))

		if w.Code != http.StatusOK {
			t.Errorf("SaveHandler failed: %d - %s", w.Code, w.Body.String())
		}

		// Verify saved and indexed
		_, err := gStore.LoadGame(validGameId)
		if err != nil {
			t.Errorf("Game not saved to store: %v", err)
		}

		games := reg.ListGames(userId, "", "", "")
		found := false
		for _, id := range games {
			if id == validGameId {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Game not found in registry for user %s", userId)
		}
	})

	// Test Load Handler
	t.Run("LoadHandler", func(t *testing.T) {
		w := makeRequest("GET", "/api/load/"+validGameId, "")

		if w.Code != http.StatusOK {
			t.Errorf("LoadHandler failed: %d", w.Code)
		}

		var resp Game
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.ID != validGameId {
			t.Errorf("Loaded wrong game ID")
		}

		// Verify Security Headers
		if w.Header().Get("X-Frame-Options") != "DENY" {
			t.Errorf("Missing X-Frame-Options header")
		}
	})

	// Test Public Game Access (Anonymous)
	t.Run("PublicGameAccess", func(t *testing.T) {
		publicId := "dddddddd-0000-4000-8000-000000000001"
		game := Game{
			ID:            publicId,
			SchemaVersion: SchemaVersionV3,
			Permissions:   Permissions{Public: "read"},
		}
		gStore.SaveGame(&game)

		req := httptest.NewRequest("GET", "/api/load/"+publicId, nil)
		// No headers (anonymous)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for public game load, got %d - %s", w.Code, w.Body.String())
		}
	})

	// Test Games Handler
	t.Run("ListGamesHandler", func(t *testing.T) {
		w := makeRequest("GET", "/api/list-games", "")

		if w.Code != http.StatusOK {
			t.Errorf("GamesHandler failed: %d", w.Code)
		}

		var resp struct {
			Data []GameSummary `json:"data"`
			Meta struct {
				Total  int `json:"total"`
				Offset int `json:"offset"`
				Limit  int `json:"limit"`
			} `json:"meta"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		games := resp.Data

		if len(games) != 1 || games[0].ID != validGameId {
			t.Errorf("Games list incorrect: %v", games)
		}
		if games[0].Location != "Test Field" || games[0].Event != "Test Event" {
			t.Errorf("Game summary missing location/event: location=%q, event=%q", games[0].Location, games[0].Event)
		}
		if resp.Meta.Total != 1 {
			t.Errorf("Expected Total 1, got %d", resp.Meta.Total)
		}
	})

	// Test Team Handlers
	t.Run("TeamHandlers", func(t *testing.T) {
		teamId := "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb"
		team := Team{
			ID:            teamId,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       userId,
			Name:          "Test Team",
		}
		teamBody, _ := json.Marshal(team)

		// 1. Save Team
		w := makeRequest("POST", "/api/save-team", string(teamBody))
		if w.Code != http.StatusOK {
			t.Errorf("SaveTeam failed: %d - %s", w.Code, w.Body.String())
		}

		// 2. Load Team
		w = makeRequest("GET", "/api/load-team/"+teamId, "")
		if w.Code != http.StatusOK {
			t.Errorf("LoadTeam failed: %d - %s", w.Code, w.Body.String())
		}

		// 3. List Teams
		w = makeRequest("GET", "/api/list-teams", "")
		if w.Code != http.StatusOK {
			t.Errorf("ListTeams failed: %d", w.Code)
		}
		var resp struct {
			Data []json.RawMessage `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		teams := resp.Data

		if len(teams) != 1 {
			t.Errorf("Expected 1 team in list, got %d", len(teams))
		}
		if resp.Meta.Total != 1 {
			t.Errorf("Expected Total 1, got %d", resp.Meta.Total)
		}

		// 4. Delete Team
		deleteBody := `{"id": "` + teamId + `"}`
		w = makeRequest("POST", "/api/delete-team", deleteBody)
		if w.Code != http.StatusOK {
			t.Errorf("DeleteTeam failed: %d", w.Code)
		}
	})

	// Test /api/team/members (Admin Access)
	t.Run("TeamMembersHandler", func(t *testing.T) {
		teamId := "dddddddd-dddd-4ddd-dddd-dddddddddddd"
		team := Team{
			ID:            teamId,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       userId,
			Name:          "Admin Team",
		}
		tStore.SaveTeam(&team)
		reg.UpdateTeam(team)

		// 1. Successful Update (Owner)
		body := `{"teamId": "` + teamId + `", "roles": {"admins": ["admin@ex.com"]}}`
		w := makeRequest("POST", "/api/team/members", body)
		if w.Code != http.StatusOK {
			t.Errorf("TeamMembers update failed: %d - %s", w.Code, w.Body.String())
		}

		// 2. Unauthorized Update (Scorekeeper)
		skId := "sk@example.com"
		team.Roles.Scorekeepers = []string{skId}
		tStore.SaveTeam(&team)
		reg.UpdateTeam(team)

		req := httptest.NewRequest("POST", "/api/team/members", strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: skId})
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for Scorekeeper updating members, got %d", w.Code)
		}
	})

	// Test Unauthorized Deletion
	t.Run("UnauthorizedDelete", func(t *testing.T) {
		teamId := "eeeeeeee-eeee-4eee-eeee-eeeeeeeeeeee"
		team := Team{
			ID:            teamId,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       "other@example.com",
			Roles:         TeamRoles{Scorekeepers: []string{userId}},
		}
		tStore.SaveTeam(&team)
		reg.UpdateTeam(team)

		// userId is Scorekeeper, should not be able to delete
		deleteBody := `{"id": "` + teamId + `"}`
		w := makeRequest("POST", "/api/delete-team", deleteBody)
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for Scorekeeper deleting team, got %d", w.Code)
		}
	})

	// Test Unauthorized Team Save
	t.Run("UnauthorizedTeamSave", func(t *testing.T) {
		teamId := "ffffffff-ffff-4fff-ffff-ffffffffffff"
		team := Team{
			ID:            teamId,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       "other@example.com",
			Roles:         TeamRoles{Spectators: []string{userId}},
		}
		teamDataBytes, _ := json.Marshal(team)
		tStore.SaveTeam(&team)
		reg.UpdateTeam(team)

		// userId is Spectator, should not be able to save
		w := makeRequest("POST", "/api/save-team", string(teamDataBytes))
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for Spectator saving team, got %d", w.Code)
		}
	})

	// Test Unauthorized
	t.Run("Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/list-games", nil)
		// No auth cookie
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %d", w.Code)
		}
	})

	// Test Unauthorized Load (Private Game)
	t.Run("UnauthorizedLoadPrivate", func(t *testing.T) {
		privateId := "cccccccc-0000-4000-8000-000000000001"
		game := Game{
			ID:            privateId,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       "other@example.com",
		}
		gStore.SaveGame(&game)

		w := makeRequest("GET", "/api/load/"+privateId, "")
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for unauthorized load, got %d", w.Code)
		}
	})

	// Test Unauthorized Save (Hijack Attempt)
	t.Run("UnauthorizedSaveHijack", func(t *testing.T) {
		targetId := "cccccccc-0000-4000-8000-000000000002"
		game := Game{
			ID:            targetId,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       "real-owner@example.com",
		}
		gStore.SaveGame(&game)

		// Attacker tries to overwrite the game with themselves as owner
		attackGame := Game{
			ID:      targetId,
			OwnerID: userId, // Attacker's ID
			ActionLog: []json.RawMessage{
				json.RawMessage(`{"id":"h1h1h1h1-h1h1-4h11-h1h1-h1h1h1111111","timestamp":123,"type":"GAME_START","payload":{"id":"` + targetId + `","date":"2025-12-18T14:57:39Z","away":"A","home":"B"}}`),
			},
		}
		attackBody, _ := json.Marshal(attackGame)
		w := makeRequest("POST", "/api/save", string(attackBody))

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for ownership hijack attempt, got %d", w.Code)
		}
	})

	// Test Unauthorized Save (Viewer Role)
	t.Run("UnauthorizedSaveViewer", func(t *testing.T) {
		targetId := "cccccccc-0000-4000-8000-000000000003"
		game := Game{
			ID:            targetId,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       "owner@example.com",
			Permissions: Permissions{
				Users: map[string]string{userId: "read"},
			},
		}
		gStore.SaveGame(&game)

		// User has read-only access, tries to save
		gameDataBytes, _ := json.Marshal(game)
		w := makeRequest("POST", "/api/save", string(gameDataBytes))
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for viewer trying to save, got %d", w.Code)
		}
	})

	// Test Unauthorized Team Load
	t.Run("UnauthorizedTeamLoad", func(t *testing.T) {
		teamId := "cccccccc-0000-4000-8000-000000000004"
		team := Team{
			ID:            teamId,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       "other@example.com",
		}
		tStore.SaveTeam(&team)

		w := makeRequest("GET", "/api/load-team/"+teamId, "")
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for unauthorized team load, got %d", w.Code)
		}
	})

	// Test Mock Auth Middleware (Cookie)
	t.Run("MockAuthCookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/list-games", nil)
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		w := httptest.NewRecorder()

		// Use a handler with UseMockAuth enabled
		_, mockAuthHandler := NewServerHandler(Options{
			GameStore:   gStore,
			TeamStore:   tStore,
			Storage:     s,
			Registry:    reg,
			UseMockAuth: true,
		})
		mockAuthHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK from mock auth cookie, got %d - %s", w.Code, w.Body.String())
		}
	})

	// Test Content-Type Middleware
	t.Run("ContentTypeMiddleware", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/init.js", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/javascript") {
			t.Errorf("Expected application/javascript, got %s", w.Header().Get("Content-Type"))
		}
	})

	// Test SSO Status Handler
	t.Run("SSOStatusHandler", func(t *testing.T) {
		// Enabled via UseMockAuth: true
		_, mockAuthHandler := NewServerHandler(Options{
			GameStore:   gStore,
			TeamStore:   tStore,
			Storage:     s,
			Registry:    reg,
			UseMockAuth: true,
		})

		// 1. Authenticated
		req := httptest.NewRequest("POST", "/.sso/", nil)
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		w := httptest.NewRecorder()
		mockAuthHandler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("SSO status failed: %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), userId) {
			t.Errorf("Expected user ID in response")
		}

		// 2. Anonymous
		req = httptest.NewRequest("POST", "/.sso/", nil)
		w = httptest.NewRecorder()
		mockAuthHandler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("SSO status failed: %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "null") {
			t.Errorf("Expected null for anonymous status")
		}
	})

	// Test SSO Logout Handler
	t.Run("SSOLogoutHandler", func(t *testing.T) {
		_, mockAuthHandler := NewServerHandler(Options{UseMockAuth: true})
		req := httptest.NewRequest("POST", "/.sso/logout", nil)
		w := httptest.NewRecorder()
		mockAuthHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Logout failed: %d", w.Code)
		}
		// Verify cookie is cleared (expired)
		found := false
		for _, c := range w.Result().Cookies() {
			if c.Name == "mock_auth_user" {
				found = true
				if c.MaxAge != -1 {
					t.Errorf("Cookie not expired")
				}
			}
		}
		if !found {
			t.Errorf("Logout cookie not set")
		}
	})

	// Test Cluster Status Handler
	t.Run("ClusterStatusHandler", func(t *testing.T) {
		// 1. Test with Raft Disabled (Default handler from TestHTTPHandlers setup)
		// Ensure Raft is explicitly disabled for this test case
		optsDisabledRaft := Options{UseMockAuth: true, Debug: true, DataDir: t.TempDir(), RaftEnabled: false}
		var handlerDisabledRaft http.Handler
		_, handlerDisabledRaft = NewServerHandler(optsDisabledRaft)

		req := httptest.NewRequest("GET", "/api/cluster/status", nil)
		w := httptest.NewRecorder()
		handlerDisabledRaft.ServeHTTP(w, req)
		if w.Code != http.StatusNotImplemented {
			t.Errorf("Expected %d Not Implemented when Raft explicitly disabled, got %d", http.StatusNotImplemented, w.Code)
		}

		// 2. Test with Raft Enabled
		dataDir, _ := os.MkdirTemp("", "status_test_data")
		defer os.RemoveAll(dataDir)
		raftDir, _ := os.MkdirTemp("", "status_test_log")
		defer os.RemoveAll(raftDir)

		secret := "status-test-secret"
		rmChan := make(chan *RaftManager, 1)

		s := storage.New(dataDir, nil)
		gStore := NewGameStore(dataDir, s)
		tStore := NewTeamStore(dataDir, s)
		opts := Options{
			DataDir:          dataDir,
			GameStore:        gStore,
			TeamStore:        tStore,
			Storage:          s,
			Registry:         NewRegistry(gStore, tStore),
			RaftEnabled:      true,
			RaftBind:         "127.0.0.1:0", // Random port
			RaftAdvertise:    "127.0.0.1:0",
			ClusterAdvertise: "127.0.0.1:0",
			ClusterAddr:      "127.0.0.1:0",
			RaftSecret:       secret,
			RaftBootstrap:    true,
			RaftManagerChan:  rmChan,
		}

		var raftHandler http.Handler
		_, raftHandler = NewServerHandler(opts)
		select {
		case <-rmChan:
		case <-time.After(5 * time.Second):
			t.Fatal("RaftManager not initialized")
		}

		// 2a. Missing Secret
		req = httptest.NewRequest("GET", "/api/cluster/status", nil)
		w = httptest.NewRecorder()
		raftHandler.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden when secret missing, got %d", w.Code)
		}

		// 2b. Wrong Secret
		req = httptest.NewRequest("GET", "/api/cluster/status", nil)
		req.Header.Set("X-Raft-Secret", "wrong-secret")
		w = httptest.NewRecorder()
		raftHandler.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden when secret wrong, got %d", w.Code)
		}

		// 2c. Valid Secret
		req = httptest.NewRequest("GET", "/api/cluster/status", nil)
		req.Header.Set("X-Raft-Secret", secret)
		w = httptest.NewRecorder()
		raftHandler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK with valid secret, got %d - %s", w.Code, w.Body.String())
		}

		var status map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
			t.Fatalf("Failed to unmarshal status: %v", err)
		}

		if status["nodeId"].(string) == "" {
			t.Errorf("Expected nodeId to be non-empty, got empty string")
		}
		if status["state"].(string) == "" {
			t.Error("State is empty")
		}
	})
}

func TestDataDirConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "datadir_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup Handler with a specific DataDir
	_, handler := NewServerHandler(Options{
		DataDir:     tempDir,
		UseMockAuth: true,
	})

	// Perform a save operation
	gameId := "10000000-0000-4000-8000-000000000005"
	game := Game{
		ID:            gameId,
		SchemaVersion: SchemaVersionV3,
		OwnerID:       "test@example.com",
		ActionLog: []json.RawMessage{
			json.RawMessage(`{"id":"10000000-0000-0000-0000-000000000001","timestamp":1,"type":"GAME_START","payload":{"id":"10000000-0000-4000-8000-000000000005","date":"2025-12-18T14:57:39Z","away":"A","home":"B"}}`),
		},
	}
	body, _ := json.Marshal(game)

	req := httptest.NewRequest("POST", "/api/save", strings.NewReader(string(body)))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: "test@example.com"})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Save failed: %d - %s", w.Code, w.Body.String())
	}

	// Verify the file was created in the correct temp directory
	expectedPath := filepath.Join(tempDir, "games", gameId+".json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Game file not created at expected path: %s", expectedPath)
	}
}

func TestConcurrentSaves(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "concurrent_save_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	reg := NewRegistry(gStore, tStore)

	_, handler := NewServerHandler(Options{
		GameStore:   gStore,
		TeamStore:   tStore,
		Storage:     s,
		Registry:    reg,
		UseMockAuth: true,
	})

	userId := "user@example.com"
	gameId := "20000000-0000-4000-8000-000000000001"

	// Prepare a valid game
	game := Game{
		ID:            gameId,
		SchemaVersion: SchemaVersionV3,
		OwnerID:       userId,
		ActionLog: []json.RawMessage{
			json.RawMessage(`{"id":"20000000-0000-0000-0000-000000000001","timestamp":1,"type":"GAME_START","payload":{"id":"` + gameId + `","date":"2025-12-18T14:57:39Z","away":"A","home":"B"}}`),
		},
	}
	body, _ := json.Marshal(game)

	const numConcurrent = 10
	var wg sync.WaitGroup
	wg.Add(numConcurrent)

	errChan := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/api/save", strings.NewReader(string(body)))
			req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				errChan <- fmt.Errorf("concurrent save failed with code %d: %s", w.Code, w.Body.String())
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent save error: %v", err)
	}
}
