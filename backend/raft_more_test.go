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
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestRaftHandlersDirect(t *testing.T) {
	// Setup
	dir, _ := os.MkdirTemp("", "raft_handlers")
	defer os.RemoveAll(dir)

	s := storage.New(dir, nil)
	gs := NewGameStore(dir, s)
	ts := NewTeamStore(dir, s)
	reg := NewRegistry(gs, ts)

	// Create RaftManager without starting Raft (mocking Raft if needed, but for handlers we just need validation logic first)
	rm := &RaftManager{
		Secret: "secret",
		FSM:    NewFSM(gs, ts, reg, nil, s),
	}

	// 1. handleStatus (Direct)
	req := httptest.NewRequest("GET", "http://localhost/api/cluster/status", nil)
	req.Header.Set("X-Raft-Secret", "secret") // Pass auth to hit Raft logic
	w := httptest.NewRecorder()

	// Capture panic because rm.Raft is nil in this test setup
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	rm.handleStatus(w, req)

	// Invalid Secret
	reqInvalid := httptest.NewRequest("GET", "/api/cluster/status", nil)
	reqInvalid.Header.Set("X-Raft-Secret", "wrong")
	wInvalid := httptest.NewRecorder()
	rm.handleStatus(wInvalid, reqInvalid)
	if wInvalid.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", wInvalid.Code)
	}

	// 2. handleRemove (Direct)
	// Invalid Method
	reqRemoveBad := httptest.NewRequest("GET", "/api/cluster/remove", nil)
	wRemoveBad := httptest.NewRecorder()
	rm.handleRemove(wRemoveBad, reqRemoveBad)
	if wRemoveBad.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", wRemoveBad.Code)
	}

	// Valid Method, Invalid Secret
	reqRemoveSec := httptest.NewRequest("POST", "/api/cluster/remove", nil)
	wRemoveSec := httptest.NewRecorder()
	rm.handleRemove(wRemoveSec, reqRemoveSec)
	if wRemoveSec.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", wRemoveSec.Code)
	}

	// Valid Request (Raft nil -> panic, need to recover)
	func() {
		defer func() {
			if r := recover(); r != nil {
				// expected panic due to rm.Raft nil
			}
		}()
		reqRemove := httptest.NewRequest("POST", "/api/cluster/remove", bytes.NewReader([]byte(`{"nodeId":"n1"}`)))
		reqRemove.Header.Set("X-Raft-Secret", "secret")
		wRemove := httptest.NewRecorder()
		rm.handleRemove(wRemove, reqRemove)
	}()

	// 3. handleJoin (Direct)
	// Invalid Secret
	reqJoinSec := httptest.NewRequest("POST", "/api/cluster/join", nil)
	wJoinSec := httptest.NewRecorder()
	rm.handleJoin(wJoinSec, reqJoinSec)
	if wJoinSec.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", wJoinSec.Code)
	}

	// 4. Leave (Direct)
	// Note: We only test error paths here as mocking the Hashicorp Raft struct for success paths is complex.
	// See raft_test.go for integration tests.
}

func TestRaftDiscoverNode(t *testing.T) {
	// Start a mock server for the target node
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cluster/status" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		// Return dummy status
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"nodeId":"target-node","state":"Follower"}`))
	}))
	defer ts.Close()

	rm := &RaftManager{
		NodeID: "local-node",
		Secret: "secret",
	}

	// Generate valid Ed25519 keys
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rm.NodeKey = priv
	rm.PubKey = pub

	tsTLS := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"nodeId":"target-node"}`))
	}))
	defer tsTLS.Close()

	cert, err := rm.generateEphemeralCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	rm.Cert = cert

	// Strip https://
	target := tsTLS.URL[8:]

	_, err = rm.discoverNode(target, "bad-key")
	if err == nil {
		t.Error("Expected error due to key mismatch")
	}
}

func TestRaftLeave(t *testing.T) {
	rm := &RaftManager{}
	otherNodeID := "node-other"
	// Test Leave with nil Raft (Expect Panic)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	rm.Leave(otherNodeID)
}
