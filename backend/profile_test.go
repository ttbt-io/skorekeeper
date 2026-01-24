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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDeleteAllEndpoint(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "backend_profile_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	opts := Options{
		DataDir:     tempDir,
		UseMockAuth: true,
	}

	_, _, handler := NewServerHandler(opts)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 1. Create a Game owned by user1
	t.Run("DeleteAll", func(t *testing.T) {
		ownerId := "user1@example.com"
		otherId := "user2@example.com"

		// Helper to make request
		doReq := func(method, path, user string, body string) *http.Response {
			req := httptest.NewRequest(method, path, nil)
			if body != "" {
				req.Body = io.NopCloser(strings.NewReader(body))
			}
			req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: user})
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			return rec.Result()
		}

		// Create Game 1 (Owned by user1)
		game1Id := "11111111-1111-4111-8111-111111111111"
		g1Body := fmt.Sprintf(`{"id":"%s","away":"A","home":"B","date":"2025-01-01"}`, game1Id)
		resp := doReq("POST", "/api/save", ownerId, g1Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Failed to create game 1: %v", resp.Status)
		}

		// Create Game 2 (Owned by user2)
		game2Id := "22222222-2222-4222-8222-222222222222"
		g2Body := fmt.Sprintf(`{"id":"%s","away":"C","home":"D","date":"2025-01-01"}`, game2Id)
		resp = doReq("POST", "/api/save", otherId, g2Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Failed to create game 2: %v", resp.Status)
		}

		// Create Team 1 (Owned by user1)
		team1Id := "33333333-3333-4333-8333-333333333333"
		t1Body := fmt.Sprintf(`{"id":"%s","name":"Team1"}`, team1Id)
		resp = doReq("POST", "/api/save-team", ownerId, t1Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Failed to create team 1: %v", resp.Status)
		}

		// Verify User 1 can see Game 1
		resp = doReq("GET", "/api/load/"+game1Id, ownerId, "")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("User 1 should see Game 1 before delete")
		}

		// Call Delete All for User 1
		resp = doReq("POST", "/api/delete-all", ownerId, "")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Delete All failed: %v", resp.Status)
		}

		// Verify Game 1 is marked deleted (Soft Delete / Tombstone)
		resp = doReq("GET", "/api/load/"+game1Id, ownerId, "")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Game 1 should return 200 OK (Tombstone), got %v", resp.Status)
		} else {
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), `"status":"deleted"`) {
				t.Errorf("Game 1 should have status: deleted, got %s", string(body))
			}
		}

		// Verify Team 1 is marked deleted
		resp = doReq("GET", "/api/load-team/"+team1Id, ownerId, "")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Team 1 should return 200 OK (Tombstone), got %v", resp.Status)
		} else {
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), `"status":"deleted"`) {
				t.Errorf("Team 1 should have status: deleted, got %s", string(body))
			}
		}

		// Verify Game 2 (User 2) still exists and is ACTIVE
		resp = doReq("GET", "/api/load/"+game2Id, otherId, "")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Game 2 should still exist for User 2")
		}
	})
}
