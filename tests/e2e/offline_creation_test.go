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

package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/ttbt-io/skorekeeper/backend"
)

func TestOfflineCreation(t *testing.T) {
	// 1. Setup Standalone Server
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}

	dataDir := t.TempDir()

	// Start standalone
	standaloneL, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	standalonePort := standaloneL.Addr().(*net.TCPAddr).Port
	standaloneURL := fmt.Sprintf("https://localhost:%d", standalonePort)

	s := storage.New(dataDir, nil)
	gStore := backend.NewGameStore(dataDir, s)
	tStore := backend.NewTeamStore(dataDir, s)
	reg := backend.NewRegistry(gStore, tStore)

	opts := backend.Options{
		Addr:        standaloneURL,
		Listener:    standaloneL,
		Cert:        cert,
		UseMockAuth: true,
		Debug:       true,
		GameStore:   gStore,
		TeamStore:   tStore,
		Registry:    reg,
		DataDir:     dataDir,
	}

	server, err := backend.StartServer(opts)
	if err != nil {
		t.Fatalf("Failed to start standalone server: %v", err)
	}

	// Helper cleanup
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	if err := waitForServer(standaloneURL, 5*time.Second); err != nil {
		t.Fatal("Standalone server failed to start")
	}

	// 2. Create Data (Game)
	userId := "user@example.com"
	gameId := "10000000-0000-0000-0000-000000000001"
	actionId := "20000000-0000-0000-0000-000000000001"

	// Create Game via HTTP
	payload := fmt.Sprintf(`{"id":"%s","timestamp":1,"type":"GAME_START","schemaVersion":3,"payload":{"id":"%s","date":"2025-01-01T00:00:00Z","away":"A","home":"B","ownerId":"%s"}}`, actionId, gameId, userId)
	msg := backend.Message{
		Type:   backend.MsgTypeAction,
		GameId: gameId,
		Action: json.RawMessage(payload),
	}
	msgBytes, _ := json.Marshal(msg)

	// Use secure client (skipping verify for test certs)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, _ := http.NewRequest("POST", standaloneURL+"/api/action", bytes.NewReader(msgBytes))
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create game on standalone: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Create game failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Stop Standalone Server
	sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(sdCtx)
	standaloneL.Close() // Release port (though we'll likely use new ports for Raft)

	t.Log("Standalone server stopped. Data persisted.")

	// 4. Start as Raft Leader (bootstrapping with existing data)
	clusterSecret := "migration-secret"

	// Network for Node 1
	l1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port1 := l1.Addr().(*net.TCPAddr).Port
	url1 := fmt.Sprintf("https://localhost:%d", port1)

	raftL1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	raftBind1 := raftL1.Addr().String()
	raftL1.Close()

	clusterL1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clusterAddr1 := clusterL1.Addr().String()
	clusterL1.Close()

	rmChan1 := make(chan *backend.RaftManager, 1)

	// Re-use Stores (pointing to same DataDir)
	s1 := storage.New(dataDir, nil)
	gStore1 := backend.NewGameStore(dataDir, s1)
	tStore1 := backend.NewTeamStore(dataDir, s1)
	reg1 := backend.NewRegistry(gStore1, tStore1)

	opts1 := backend.Options{
		Addr:             url1,
		ClusterAdvertise: clusterAddr1,
		ClusterAddr:      clusterAddr1,
		Listener:         l1,
		Cert:             cert,
		UseMockAuth:      true,
		Debug:            true,
		GameStore:        gStore1,
		TeamStore:        tStore1,
		Registry:         reg1,
		RaftEnabled:      true,
		RaftBind:         raftBind1,
		RaftAdvertise:    raftBind1,
		RaftSecret:       clusterSecret,
		RaftBootstrap:    true, // Bootstrap!
		RaftManagerChan:  rmChan1,
		DataDir:          dataDir,
	}

	server1, err := backend.StartServer(opts1)
	if err != nil {
		t.Fatalf("Failed to start Raft Leader: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server1.Shutdown(ctx)
	})

	var rm1 *backend.RaftManager
	select {
	case rm1 = <-rmChan1:
	case <-time.After(5 * time.Second):
		t.Fatal("RaftManager 1 not received")
	}

	// Wait for Leader
	t.Log("Waiting for leader election...")
	waitForLeader(t, rm1)

	// 5. Start Node 2 (Follower) with EMPTY data dir
	dataDir2 := t.TempDir()

	l2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port2 := l2.Addr().(*net.TCPAddr).Port
	url2 := fmt.Sprintf("https://localhost:%d", port2)

	raftL2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	raftBind2 := raftL2.Addr().String()
	raftL2.Close()

	clusterL2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clusterAddr2 := clusterL2.Addr().String()
	clusterL2.Close()

	rmChan2 := make(chan *backend.RaftManager, 1)

	s2 := storage.New(dataDir2, nil)
	gStore2 := backend.NewGameStore(dataDir2, s2)
	tStore2 := backend.NewTeamStore(dataDir2, s2)
	reg2 := backend.NewRegistry(gStore2, tStore2)

	opts2 := backend.Options{
		Addr:             url2,
		ClusterAdvertise: clusterAddr2,
		ClusterAddr:      clusterAddr2,
		Listener:         l2,
		Cert:             cert,
		UseMockAuth:      true,
		Debug:            true,
		GameStore:        gStore2,
		TeamStore:        tStore2,
		Registry:         reg2,
		RaftEnabled:      true,
		RaftBind:         raftBind2,
		RaftAdvertise:    raftBind2,
		RaftSecret:       clusterSecret,
		RaftBootstrap:    false,
		RaftManagerChan:  rmChan2,
		DataDir:          dataDir2,
	}

	server2, err := backend.StartServer(opts2)
	if err != nil {
		t.Fatalf("Failed to start Raft Follower: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server2.Shutdown(ctx)
	})

	var rm2 *backend.RaftManager
	select {
	case rm2 = <-rmChan2:
	case <-time.After(5 * time.Second):
		t.Fatal("RaftManager 2 not received")
	}

	// 6. Join Node 2
	t.Log("Joining Node 2...")
	rm2.AddNodePubKey(rm1.NodeID, rm1.ClusterAdvertise, base64.StdEncoding.EncodeToString(rm1.PubKey))
	if err := rm1.Join(rm2.NodeID, rm2.Bind, rm2.ClusterAdvertise, base64.StdEncoding.EncodeToString(rm2.PubKey), false, backend.CurrentAppVersion, backend.CurrentProtocolVersion, backend.CurrentSchemaVersion); err != nil {
		t.Fatalf("Join failed: %v", err)
	}

	// 7. Verify Data on Node 2
	t.Log("Verifying data on Node 2...")
	// We need to wait for snapshot replication and restore.
	// We can poll the GameStore on Node 2.

	success := false
	for i := 0; i < 20; i++ {
		_, err := gStore2.LoadGame(gameId)
		if err == nil {
			success = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !success {
		t.Fatal("Node 2 failed to receive game data via snapshot")
	}
	t.Log("Success! Data migrated to Raft cluster.")
}
