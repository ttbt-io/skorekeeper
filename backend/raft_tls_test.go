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
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/hashicorp/raft"
)

// generateSelfSignedCert is a helper to create a self-signed cert and CA pool for testing.
func generateSelfSignedCert() (*tls.Certificate, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "leader-node",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "127.0.0.1"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, pub, priv)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privBytes, _ := x509.MarshalPKCS8PrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, err
	}

	return &cert, pub, nil
}

// TestRaftTLSConfig verifies that RaftManager correctly configures TLS with dynamic keys.
func TestRaftTLSConfig(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "raft_tls_test")
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)
	hm := NewHubManager()
	fsm := NewFSM(gStore, tStore, reg, hm, s, us)

	// Use random port
	rm := NewRaftManager(tempDir, "127.0.0.1:0", "", "http://localhost", "127.0.0.1:0", "secret", nil, fsm)

	// Start Raft (bootstrapping single node)
	if err := rm.Start(true); err != nil {
		t.Fatalf("RaftManager.Start() failed: %v", err)
	}

	// We can't easily inspect the internal Transport via public API, but we can verify it accepted the certs
	// and started without error. The fact that Start succeeded with Cert set means it tried to use tlsStreamLayer.

	// Wait for Leader to confirm operational
	timeout := time.After(5 * time.Second)
	for {
		if rm.Raft != nil && rm.Raft.State().String() == "Leader" {
			// Once leader, propose own NodeMeta.
			// This is essential as forwardToLeader uses rm.FSM.GetNodeAddr which relies on NodeMeta.
			cmd := RaftCommand{
				Type: CmdNodeMeta,
				NodeMeta: &NodeMeta{
					NodeID:          rm.NodeID,
					HttpAddr:        rm.ClusterAdvertise,
					PubKey:          base64.StdEncoding.EncodeToString(rm.PubKey),
					AppVersion:      CurrentAppVersion,
					ProtocolVersion: CurrentProtocolVersion,
					SchemaVersion:   CurrentSchemaVersion,
				},
			}
			if _, err := rm.Propose(cmd); err != nil {
				t.Fatalf("Failed to propose leader NodeMeta: %v", err)
			}
			break
		}
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for leader election in TLS mode")
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Verify that GetLeaderHTTPAddr works (which relies on FSM metadata applied via TLS consensus)
	// We need to wait for the metadata command to be applied
	time.Sleep(500 * time.Millisecond)

	addr := rm.GetLeaderHTTPAddr()
	if addr != "http://localhost" {
		t.Errorf("GetLeaderHTTPAddr() = %s, want http://localhost", addr)
	}
}

// TestForwardRequestToLeader verifies forwarding for handleJoin/handleRemove.
func TestForwardRequestToLeader(t *testing.T) {
	// 1. Get ports
	l1, _ := net.Listen("tcp", "127.0.0.1:0")
	leaderAddr := l1.Addr().String()
	l1.Close()

	l2, _ := net.Listen("tcp", "127.0.0.1:0")
	followerAddr := l2.Addr().String()
	l2.Close()

	// 2. Start Leader
	dir1, _ := os.MkdirTemp("", "leader")
	defer os.RemoveAll(dir1)

	// We need fsm, stores etc.
	s1 := storage.New(dir1, nil)
	gStore1 := NewGameStore(dir1, s1)
	tStore1 := NewTeamStore(dir1, s1)
	us1 := NewUserIndexStore(dir1, s1, nil)
	reg1 := NewRegistry(gStore1, tStore1, us1, true)
	fsm1 := NewFSM(gStore1, tStore1, reg1, NewHubManager(), s1, us1)

	// Use leaderAddr for BOTH Raft and ClusterHTTP for simplicity, or separate?
	// Start() uses Bind for Raft, ClusterAddr for HTTP. They must be different ports usually?
	// If they are same, they might conflict if same protocol? No, Raft is custom TCP, HTTP is TCP.
	// They cannot bind same port.

	r1, _ := net.Listen("tcp", "127.0.0.1:0")
	leaderRaft := r1.Addr().String()
	r1.Close()

	rm1 := NewRaftManager(dir1, leaderRaft, leaderRaft, leaderAddr, leaderAddr, "secret", nil, fsm1)
	if err := rm1.Start(true); err != nil {
		t.Fatalf("Leader start: %v", err)
	}

	// Wait for leader
	for {
		if rm1.Raft != nil && rm1.Raft.State().String() == "Leader" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 3. Start Follower
	dir2, _ := os.MkdirTemp("", "follower")
	defer os.RemoveAll(dir2)

	r2, _ := net.Listen("tcp", "127.0.0.1:0")
	followerRaft := r2.Addr().String()
	r2.Close()

	s2 := storage.New(dir2, nil)
	gStore2 := NewGameStore(dir2, s2)
	tStore2 := NewTeamStore(dir2, s2)
	us2 := NewUserIndexStore(dir2, s2, nil)
	reg2 := NewRegistry(gStore2, tStore2, us2, true)
	fsm2 := NewFSM(gStore2, tStore2, reg2, NewHubManager(), s2, us2)

	rm2 := NewRaftManager(dir2, followerRaft, followerRaft, followerAddr, followerAddr, "secret", nil, fsm2)
	if err := rm2.Start(false); err != nil {
		t.Fatalf("Follower start: %v", err)
	}

	// 4. Join Follower to Cluster (bootstrap the cluster)
	// We do this manually on Leader so Follower knows Leader exists.
	// Add Follower's pubkey to Leader FSM so Leader accepts Follower's Raft connection
	rm1.AddNodePubKey(rm2.NodeID, followerAddr, base64.StdEncoding.EncodeToString(rm2.PubKey))

	// Add Leader's pubkey to Follower so Follower trusts Leader (for forwarding!)
	rm2.AddNodePubKey(rm1.NodeID, leaderAddr, base64.StdEncoding.EncodeToString(rm1.PubKey))

	// Call Join on Leader to add Follower
	if err := rm1.Join(rm2.NodeID, followerRaft, followerAddr, base64.StdEncoding.EncodeToString(rm2.PubKey), false, CurrentAppVersion, CurrentProtocolVersion, CurrentSchemaVersion); err != nil {
		t.Fatalf("Join failed: %v", err)
	}

	// Wait for Follower to see Leader
	time.Sleep(1 * time.Second) // Let Raft replicate config

	// 5. Test Forwarding: Send "Join Node 3" request to FOLLOWER
	// This request should be forwarded to Leader.

	body := fmt.Sprintf(`{"nodeId":"node3", "raftAddr":"127.0.0.1:9999", "httpAddr":"127.0.0.1:8888", "pubKey":"dummykey", "appVersion":"%s", "protocolVersion":%d, "schemaVersion":%d}`, CurrentAppVersion, CurrentProtocolVersion, CurrentSchemaVersion)
	req := httptest.NewRequest("POST", "/api/cluster/join", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Raft-Secret", "secret") // Must match

	w := httptest.NewRecorder()

	// Call handleJoin on Follower
	rm2.handleJoin(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK from forwarded join, got %d. Body: %s", resp.StatusCode, w.Body.String())
	}

	// Verify Node 3 is in Leader's list (conceptually) or log.
	// Ideally we check rm1.FSM or rm1.Raft config.
	future := rm1.Raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range future.Configuration().Servers {
		if s.ID == "node3" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Node 3 not found in Leader configuration after forwarded join")
	}
}

func TestJoinNonVoter(t *testing.T) {
	dir, _ := os.MkdirTemp("", "nonvoter_test")
	defer os.RemoveAll(dir)

	s := storage.New(dir, nil)
	gStore := NewGameStore(dir, s)
	tStore := NewTeamStore(dir, s)
	us := NewUserIndexStore(dir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)
	fsm := NewFSM(gStore, tStore, reg, NewHubManager(), s, us)

	r1, _ := net.Listen("tcp", "127.0.0.1:0")
	leaderRaft := r1.Addr().String()
	r1.Close()

	rm := NewRaftManager(dir, leaderRaft, leaderRaft, "127.0.0.1:0", "127.0.0.1:0", "secret", nil, fsm)
	if err := rm.Start(true); err != nil {
		t.Fatalf("Leader start: %v", err)
	}

	// Wait for leader
	for {
		if rm.Raft != nil && rm.Raft.State().String() == "Leader" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Join a non-voter
	if err := rm.Join("nonvoter-node", "127.0.0.1:9999", "127.0.0.1:8888", "dummykey", true, CurrentAppVersion, CurrentProtocolVersion, CurrentSchemaVersion); err != nil {
		t.Fatalf("Join non-voter failed: %v", err)
	}

	// Verify configuration
	future := rm.Raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, s := range future.Configuration().Servers {
		if s.ID == "nonvoter-node" {
			found = true
			if s.Suffrage != raft.Nonvoter {
				t.Errorf("Expected nonvoter-node to be Nonvoter, got %v", s.Suffrage)
			}
			break
		}
	}
	if !found {
		t.Error("nonvoter-node not found in configuration")
	}
}

func TestForwardingLoop(t *testing.T) {
	dir, _ := os.MkdirTemp("", "loop_test")
	defer os.RemoveAll(dir)

	rm := NewRaftManager(dir, "127.0.0.1:0", "", "127.0.0.1:0", "127.0.0.1:0", "secret", nil, nil)
	// We need to load/generate keys to get the NodeID
	rm.loadOrGenerateNodeKey()
	// And derive ID
	rm.NodeID = hex.EncodeToString(rm.PubKey[:8])

	req := httptest.NewRequest("POST", "/api/cluster/join", nil)
	req.Header.Set("X-Raft-Forwarded", "node-a,"+rm.NodeID) // loop-node is us
	req.Header.Set("X-Raft-Secret", "secret")
	w := httptest.NewRecorder()

	rm.handleJoin(w, req)

	if w.Code != http.StatusLoopDetected {
		t.Errorf("Expected 508 Loop Detected, got %d", w.Code)
	}
}

func TestForwardAppRequest(t *testing.T) {
	// 1. Setup Leader
	dir1, _ := os.MkdirTemp("", "leader_app")
	defer os.RemoveAll(dir1)

	// Get two free ports for Leader
	l1, _ := net.Listen("tcp", "127.0.0.1:0")
	leaderHTTP := l1.Addr().String()
	l1.Close()

	r1, _ := net.Listen("tcp", "127.0.0.1:0")
	leaderRaft := r1.Addr().String()
	r1.Close()

	s1 := storage.New(dir1, nil)
	// Minimal deps
	gStore1 := NewGameStore(dir1, s1)
	tStore1 := NewTeamStore(dir1, s1)
	us1 := NewUserIndexStore(dir1, s1, nil)
	fsm1 := NewFSM(gStore1, tStore1, NewRegistry(gStore1, tStore1, us1, true), NewHubManager(), s1, us1)

	// Mock App Handler on Leader
	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/test" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("handled"))
			return
		}
		http.NotFound(w, r)
	})

	rm1 := NewRaftManager(dir1, leaderRaft, leaderRaft, leaderHTTP, leaderHTTP, "secret", nil, fsm1)
	rm1.AppHandler = appHandler
	if err := rm1.Start(true); err != nil {
		t.Fatalf("Leader start: %v", err)
	}

	// Wait for leader
	for {
		if rm1.Raft != nil && rm1.Raft.State().String() == "Leader" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 2. Setup Follower
	dir2, _ := os.MkdirTemp("", "follower_app")
	defer os.RemoveAll(dir2)

	// Get two free ports for Follower
	l2, _ := net.Listen("tcp", "127.0.0.1:0")
	followerHTTP := l2.Addr().String()
	l2.Close()

	r2, _ := net.Listen("tcp", "127.0.0.1:0")
	followerRaft := r2.Addr().String()
	r2.Close()

	s2 := storage.New(dir2, nil)
	gStore2 := NewGameStore(dir2, s2)
	tStore2 := NewTeamStore(dir2, s2)
	us2 := NewUserIndexStore(dir2, s2, nil)
	fsm2 := NewFSM(gStore2, tStore2, NewRegistry(gStore2, tStore2, us2, true), NewHubManager(), s2, us2)

	rm2 := NewRaftManager(dir2, followerRaft, followerRaft, followerHTTP, followerHTTP, "secret", nil, fsm2)
	if err := rm2.Start(false); err != nil {
		t.Fatalf("Follower start: %v", err)
	}

	// Join Follower to Leader
	rm1.AddNodePubKey(rm2.NodeID, followerHTTP, base64.StdEncoding.EncodeToString(rm2.PubKey))
	rm2.AddNodePubKey(rm1.NodeID, leaderHTTP, base64.StdEncoding.EncodeToString(rm1.PubKey))

	if err := rm1.Join(rm2.NodeID, followerRaft, followerHTTP, base64.StdEncoding.EncodeToString(rm2.PubKey), false, CurrentAppVersion, CurrentProtocolVersion, CurrentSchemaVersion); err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	time.Sleep(1 * time.Second)

	// 3. Test Forwarding
	// We simulate a request arriving at the Follower that needs forwarding
	req := httptest.NewRequest("POST", "/api/test", bytes.NewBufferString("test body"))
	w := httptest.NewRecorder()

	// Manually invoke forwarding logic (simulating what server.go does)
	rm2.forwardRequestToLeader(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}
	if body := w.Body.String(); body != "handled" {
		t.Errorf("Expected body 'handled', got %q", body)
	}
}
