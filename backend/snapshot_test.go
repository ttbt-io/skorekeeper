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
	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

func TestFSMSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	mk, _ := crypto.CreateAESMasterKeyForTest()

	s := storage.New(dataDir, mk)
	raftS := storage.New(raftDir, mk)

	gs := NewGameStore(dataDir, s)
	ts := NewTeamStore(dataDir, s)
	us := NewUserIndexStore(dataDir, s, nil)
	reg := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, reg, nil, raftS, us)
	// 1. Add some data
	gameId := "game-1"
	game := Game{SchemaVersion: SchemaVersionV3, ID: gameId, Away: "A", Home: "B"}
	gs.SaveGame(&game)

	teamId := "team-1"
	team := Team{SchemaVersion: SchemaVersionV3, ID: teamId, Name: "Team One"}
	ts.SaveTeam(&team)

	fsm.nodeMap.Store("node-1", &NodeMeta{NodeID: "node-1", HttpAddr: "127.0.0.1:8080"})

	// 2. Snapshot using LinkSnapshotStore
	innerStore, err := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	// Create a dummy keyring for test
	ring := NewKeyRing(mk, "test-key")
	linkStore := NewLinkSnapshotStore(raftDir, dataDir, innerStore, ring, mk)

	sink, err := linkStore.Create(1, 10, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create sink failed: %v", err)
	}

	if err := fsm.persist(sink); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	// persist closes the sink

	// 3. Restore to new dir
	dataDir2 := t.TempDir()
	raftDir2 := filepath.Join(dataDir2, "raft")
	s2 := storage.New(dataDir2, mk)
	raftS2 := storage.New(raftDir2, mk)

	gs2 := NewGameStore(dataDir2, s2)
	ts2 := NewTeamStore(dataDir2, s2)
	us2 := NewUserIndexStore(dataDir2, s2, nil)
	reg2 := NewRegistry(gs2, ts2, us2, true)
	fsm2 := NewFSM(gs2, ts2, reg2, nil, raftS2, us2)

	_, rc, err := linkStore.Open(sink.ID())
	if err != nil {
		t.Fatalf("Open snapshot failed: %v", err)
	}
	defer rc.Close()

	if err := fsm2.restore(rc); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 4. Verify
	g2, err := gs2.LoadGame(gameId)
	if err != nil {
		t.Fatalf("Game not found after restore: %v", err)
	}
	if g2.ID != gameId || g2.Away != "A" {
		t.Errorf("Game data mismatch. Expected %+v, got %+v", game, g2)
	}

	t2, err := ts2.LoadTeam(teamId)
	if err != nil {
		t.Fatalf("Team not found after restore: %v", err)
	}
	if t2.ID != teamId || t2.Name != "Team One" {
		t.Errorf("Team data mismatch. Expected %+v, got %+v", team, t2)
	}

	addr := fsm2.GetNodeAddr("node-1")
	if addr != "127.0.0.1:8080" {
		t.Errorf("NodeAddr mismatch. Expected 127.0.0.1:8080, got %s", addr)
	}
}

func TestSmartSnapshot_IndexTracking(t *testing.T) {
	// Setup FSM
	tmpDir := t.TempDir()
	raftDir := filepath.Join(tmpDir, "raft")
	s := storage.New(tmpDir, nil)
	raftS := storage.New(raftDir, nil)

	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, nil)
	r := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, r, nil, raftS, us)
	// verify initial index
	if fsm.LastAppliedIndex() != 0 {
		t.Fatalf("Expected 0, got %d", fsm.LastAppliedIndex())
	}

	// Apply a log
	logEntry := &raft.Log{
		Index: 100,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  []byte(`{"type": "UNKNOWN"}`), // Unknown command returns error but we track index?
	}
	// Note: Apply checks command type. If unknown, it returns error.
	// Check if FSM updates index even on error?
	// The code: res := f.applyCommand(...); f.lastAppliedIndex.Store(l.Index); return res
	// Yes, it updates index.

	fsm.Apply(logEntry)

	if fsm.LastAppliedIndex() != 100 {
		t.Fatalf("Expected 100, got %d", fsm.LastAppliedIndex())
	}

	// Snapshot
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	defer snap.Release()

	// Check Manifest in Snapshot
	mk, _ := crypto.CreateAESMasterKeyForTest()
	innerStore, _ := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	ring := NewKeyRing(mk, "test-key")
	linkStore := NewLinkSnapshotStore(raftDir, tmpDir, innerStore, ring, mk)

	sink, _ := linkStore.Create(1, 100, 1, raft.Configuration{}, 1, nil)
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	_, rc, err := linkStore.Open(sink.ID())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()

	// Read tar to find manifest.json
	gz, _ := gzip.NewReader(rc)
	tr := tar.NewReader(gz)
	found := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if h.Name == "manifest.json" {
			var m snapshotManifest
			json.NewDecoder(tr).Decode(&m)
			if m.RaftIndex != 100 {
				t.Errorf("Manifest index mismatch. Expected 100, got %d", m.RaftIndex)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("Manifest not found in snapshot")
	}
}

func TestSmartSnapshot_SkipRestore(t *testing.T) {
	// 1. Setup Local State (High Index)
	tmpDir := t.TempDir()
	raftDir := filepath.Join(tmpDir, "raft")
	s := storage.New(tmpDir, nil)

	raftS := storage.New(raftDir, nil)

	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, nil)
	r := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, r, nil, raftS, us)

	// Set initialized
	fsm.setInitialized()
	// Create "Local Game A"
	gameA := &Game{ID: "gameA", ActionLog: []json.RawMessage{}}
	gs.SaveGame(gameA)

	// 2. Create a Snapshot (Low Index)
	// We need to craft a snapshot manually or use FSM to generate one.
	// Using FSM is easier, but FSM writes *its* current state.
	// So let's create a separate FSM2 with Low Index.

	tmpDir2 := t.TempDir()
	raftDir2 := filepath.Join(tmpDir2, "raft")
	s2 := storage.New(tmpDir2, nil)
	raftS2 := storage.New(raftDir2, nil)

	gs2 := NewGameStore(tmpDir2, s2)
	ts2 := NewTeamStore(tmpDir2, s2)
	us2 := NewUserIndexStore(tmpDir2, s2, nil)
	r2 := NewRegistry(gs2, ts2, us2, true)
	fsm2 := NewFSM(gs2, ts2, r2, nil, raftS2, us2)

	// Set Index 100 on FSM2
	fsm2.lastAppliedIndex.Store(100)
	fsm2.setInitialized()

	// Create "Snapshot Game B"
	gameB := &Game{ID: "gameB", ActionLog: []json.RawMessage{}}
	gs2.SaveGame(gameB)

	// Take Snapshot from FSM2
	snap, err := fsm2.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot creation failed: %v", err)
	}

	// Persist to LinkSnapshotStore
	mk, _ := crypto.CreateAESMasterKeyForTest()
	innerStore, _ := raft.NewFileSnapshotStore(raftDir2, 1, io.Discard)
	ring := NewKeyRing(mk, "test-key")
	linkStore := NewLinkSnapshotStore(raftDir2, tmpDir2, innerStore, ring, mk)

	sink, _ := linkStore.Create(1, 100, 1, raft.Configuration{}, 1, nil)
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// 3. Restore FSM1 from Snapshot
	// FSM1 has Index 200. Snapshot is Index 100.
	// Should SKIP.
	_, rc, err := linkStore.Open(sink.ID())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()

	if err := fsm.Restore(rc); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 4. Verify
	// Game A should exist (not cleaned up)
	if _, err := gs.LoadGame("gameA"); err != nil {
		t.Errorf("Game A should still exist (Restore should have been skipped): %v", err)
	}
	// Game B should NOT exist
	if _, err := gs.LoadGame("gameB"); err == nil {
		t.Errorf("Game B should NOT exist (Restore should have been skipped)")
	}
}

func TestSmartSnapshot_FastRestore(t *testing.T) {
	// Setup FSM
	tmpDir := t.TempDir()
	raftDir := filepath.Join(tmpDir, "raft")
	s := storage.New(tmpDir, nil)
	raftS := storage.New(raftDir, nil)

	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, nil)
	r := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, r, nil, raftS, us)

	// Create Games
	numGames := 10
	for i := 0; i < numGames; i++ {
		// We use gs.SaveGame directly to bypass FSM log/index but populate disk
		g := &Game{ID: fmt.Sprintf("game-%d", i), ActionLog: []json.RawMessage{}, SchemaVersion: 3}
		gs.SaveGame(g)
	}

	// Snapshot
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	mk, _ := crypto.CreateAESMasterKeyForTest()
	innerStore, _ := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	ring := NewKeyRing(mk, "test-key")
	linkStore := NewLinkSnapshotStore(raftDir, tmpDir, innerStore, ring, mk)

	sink, _ := linkStore.Create(1, 10, 1, raft.Configuration{}, 1, nil)
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// New FSM

	tmpDir2 := t.TempDir()

	raftDir2 := filepath.Join(tmpDir2, "raft")

	s2 := storage.New(tmpDir2, mk)

	raftS2 := storage.New(raftDir2, mk)

	gs2 := NewGameStore(tmpDir2, s2)

	ts2 := NewTeamStore(tmpDir2, s2)

	us2 := NewUserIndexStore(tmpDir2, s2, nil)

	r2 := NewRegistry(gs2, ts2, us2, true)

	fsm2 := NewFSM(gs2, ts2, r2, nil, raftS2, us2)

	// Restore
	_, rc, err := linkStore.Open(sink.ID())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()

	if err := fsm2.Restore(rc); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify all games exist
	for i := 0; i < numGames; i++ {
		id := fmt.Sprintf("game-%d", i)
		if _, err := gs2.LoadGame(id); err != nil {
			t.Errorf("Game %s missing after restore: %v", id, err)
		}
	}
}

func TestSnapshot_LargeDataset_Eviction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset test in short mode")
	}

	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	mk, _ := crypto.CreateAESMasterKeyForTest()

	s := storage.New(dataDir, mk)

	gs := NewGameStore(dataDir, s)

	ts := NewTeamStore(dataDir, s)

	us := NewUserIndexStore(dataDir, s, mk)

	reg := NewRegistry(gs, ts, us, true)

	fsm := NewFSM(gs, ts, reg, nil, s, us)

	// UserIndexStore cache size is 1000. We'll create 1500 items.
	numItems := 1500

	t.Logf("Generating %d items to force eviction...", numItems)

	for i := 0; i < numItems; i++ {
		uid := fmt.Sprintf("user-%d@example.com", i)
		gid := fmt.Sprintf("game-%d", i)

		// Create Game (Directly to store to avoid overhead, but we need Registry updated)
		// Registry.UpdateGame handles UserIndex.
		g := Game{ID: gid, OwnerID: uid, SchemaVersion: 3}
		gs.SaveGame(&g)
		reg.UpdateGame(g)
	}

	// Take Snapshot
	t.Log("Taking snapshot...")
	innerStore, err := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	ring := NewKeyRing(mk, "test-key")
	linkStore := NewLinkSnapshotStore(raftDir, dataDir, innerStore, ring, mk)

	sink, err := linkStore.Create(1, 10, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create sink failed: %v", err)
	}

	if err := fsm.persist(sink); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Restore to new dir

	dataDir2 := t.TempDir()
	s2 := storage.New(dataDir2, mk)

	gs2 := NewGameStore(dataDir2, s2)
	ts2 := NewTeamStore(dataDir2, s2)
	us2 := NewUserIndexStore(dataDir2, s2, mk)
	reg2 := NewRegistry(gs2, ts2, us2, true)
	fsm2 := NewFSM(gs2, ts2, reg2, nil, s2, us2)

	_, rc, err := linkStore.Open(sink.ID())
	if err != nil {
		t.Fatalf("Open snapshot failed: %v", err)
	}
	defer rc.Close()

	if err := fsm2.restore(rc); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify
	t.Log("Verifying restored data...")

	// Check random items (start, middle, end) to ensure no gaps
	indicesToCheck := []int{0, 500, 1000, 1499}
	for _, i := range indicesToCheck {
		uid := fmt.Sprintf("user-%d@example.com", i)
		gid := fmt.Sprintf("game-%d", i)

		// Verify Game
		g, err := gs2.LoadGame(gid)
		if err != nil {
			t.Errorf("Game %s missing after restore", gid)
		} else if g.OwnerID != uid {
			t.Errorf("Game %s owner mismatch. Want %s, got %s", gid, uid, g.OwnerID)
		}

		// Verify User Index (Registry Access)
		if !reg2.HasGameAccess(uid, gid) {
			t.Errorf("User %s lost access to game %s after restore", uid, gid)
		}
	}

	// Verify Counts
	if reg2.CountTotalGames() != numItems {
		t.Errorf("Total games mismatch. Want %d, got %d", numItems, reg2.CountTotalGames())
	}
}

func TestRaft_Snapshot_FullRestore(t *testing.T) {
	// 1. Setup Infrastructure
	baseDir := t.TempDir()
	mk, _ := crypto.CreateAESMasterKeyForTest()

	// Helper to create a node using StartServer
	createNode := func(id string, bootstrap bool) (*Server, string, int, int) {
		dir := filepath.Join(baseDir, id)

		portHTTP := getPort(getFreePort())
		portRaft := getPort(getFreePort())
		portCluster := getPort(getFreePort())

		addrHTTP := fmt.Sprintf("127.0.0.1:%d", portHTTP)
		addrRaft := fmt.Sprintf("127.0.0.1:%d", portRaft)
		addrCluster := fmt.Sprintf("127.0.0.1:%d", portCluster)

		opts := Options{
			Addr:             addrHTTP,
			DataDir:          dir,
			MasterKey:        mk,
			RaftEnabled:      true,
			RaftBind:         addrRaft,
			RaftAdvertise:    addrRaft,
			ClusterAddr:      addrCluster,
			ClusterAdvertise: addrCluster,
			RaftBootstrap:    bootstrap,
			RaftSecret:       "secret",
			// Configure for fast testing and snapshot triggering
			UseProductionTimeouts: false,
			SnapshotThreshold:     100,
			TrailingLogs:          10,
		}

		srv, err := StartServer(opts)
		if err != nil {
			t.Fatalf("Failed to start server %s: %v", id, err)
		}

		return srv, dir, portRaft, portHTTP
	}

	// 2. Start Node A (Leader)
	serverA, _, _, _ := createNode("A", true)
	defer serverA.Shutdown(nil)
	nodeA := serverA.raftMgr

	// Wait for leader
	waitForLeader(t, []*RaftManager{nodeA})

	// 3. Populate Data on Node A
	t.Log("Populating data on Node A...")

	// Game
	game := &Game{ID: "game-1", Away: "Away Team", Home: "Home Team", SchemaVersion: CurrentSchemaVersion}
	gameData, _ := json.Marshal(game)
	gameRaw := json.RawMessage(gameData)
	if _, err := nodeA.Propose(RaftCommand{Type: CmdSaveGame, ID: "game-1", GameData: &gameRaw}); err != nil {
		t.Fatalf("Failed to save game: %v", err)
	}

	// Team
	team := &Team{ID: "team-1", Name: "Super Team", SchemaVersion: CurrentSchemaVersion}
	teamData, _ := json.Marshal(team)
	teamRaw := json.RawMessage(teamData)
	if _, err := nodeA.Propose(RaftCommand{Type: CmdSaveTeam, ID: "team-1", TeamData: &teamRaw}); err != nil {
		t.Fatalf("Failed to save team: %v", err)
	}

	// System Policy
	policy := &UserAccessPolicy{
		DefaultPolicy: "deny",
		Admins:        []string{"admin@example.com"},
	}
	if _, err := nodeA.Propose(RaftCommand{Type: CmdUpdateAccessPolicy, PolicyData: policy}); err != nil {
		t.Fatalf("Failed to save policy: %v", err)
	}

	// Metrics (Ingest directly into FSM to simulate accumulation)
	// We manually update FSM metrics because Propose(CmdMetricsUpdate) is async and complicated.
	nodeA.FSM.metricsMu.Lock()
	nodeA.FSM.metrics.GetClusterSeries("testCount").Ingest(time.Now().Unix(), 42.0)
	nodeA.FSM.metricsMu.Unlock()

	// Generate logs to force compaction (SnapshotThreshold is 100, TrailingLogs is 10)
	// We need enough logs so that Snapshot() actually deletes old logs.
	for i := 0; i < 150; i++ {
		metricsPayload := &MetricsPayload{Timestamp: int64(i)}
		nodeA.Propose(RaftCommand{Type: CmdMetricsUpdate, MetricsPayload: metricsPayload})
	}

	// 4. Force Snapshot on Node A
	t.Log("Forcing Snapshot on Node A...")

	if future := nodeA.Raft.Snapshot(); future.Error() != nil {
		t.Fatalf("Failed to snapshot A: %v", future.Error())
	}

	// Ensure snapshot is done
	time.Sleep(1 * time.Second)

	// 5. Start Node B (Fresh)
	serverB, dirB, portRaftB, portHTTPB := createNode("B", false)
	defer serverB.Shutdown(nil)
	nodeB := serverB.raftMgr

	// 6. Join Node B to Cluster
	t.Log("Joining Node B to Cluster...")

	pubKeyB := nodeB.PubKey

	// 7. Verify Restoration on Node B
	t.Log("Waiting for Node B to sync...")

	// Helper to join
	joinErr := nodeA.Join(
		nodeB.NodeID,
		fmt.Sprintf("127.0.0.1:%d", portRaftB),
		fmt.Sprintf("127.0.0.1:%d", portHTTPB),
		base64.StdEncoding.EncodeToString(pubKeyB),
		false,
		"v1",
		1,
		1,
	)
	if joinErr != nil {
		t.Fatalf("Failed to join B: %v", joinErr)
	}

	// Wait for B to have the game
	deadline := time.Now().Add(10 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		gsB, _ := nodeB.FSM.GetStores()
		if _, err := gsB.LoadGame("game-1"); err == nil {
			found = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !found {
		t.Fatalf("Node B failed to restore game-1 within timeout")
	}

	// Assertions
	gsB, tsB := nodeB.FSM.GetStores()

	// Game
	g, err := gsB.LoadGame("game-1")
	if err != nil {
		t.Errorf("Node B missing game-1: %v", err)
	} else if g.Away != "Away Team" {
		t.Errorf("Node B game-1 mismatch: %v", g)
	}

	// Team
	tm, err := tsB.LoadTeam("team-1")
	if err != nil {
		t.Errorf("Node B missing team-1: %v", err)
	} else if tm.Name != "Super Team" {
		t.Errorf("Node B team-1 mismatch: %v", tm)
	}

	// Check memory state (Policy)
	policyB := nodeB.FSM.r.GetAccessPolicy() // Access internal Registry
	if policyB == nil {
		t.Errorf("Node B Registry has no policy")
	} else {
		if policyB.DefaultPolicy != "deny" {
			t.Errorf("Node B Policy mismatch: got %s, want deny", policyB.DefaultPolicy)
		}
		if len(policyB.Admins) != 1 || policyB.Admins[0] != "admin@example.com" {
			t.Errorf("Node B Policy Admins mismatch: %v", policyB.Admins)
		}
	}

	// Verify System Files Restoration
	// We expect metrics.json and nodes.json.
	// fsm_state.json is deprecated/not used.
	sysFiles := []string{"metrics.json", "nodes.json"}
	for _, f := range sysFiles {
		// StartServer uses DataDir/raft for FSM storage. So files are in raft/ subdirectory.
		if _, err := os.Stat(filepath.Join(dirB, "raft", f)); err != nil {
			t.Errorf("Node B missing restored file %s: %v", f, err)
		}
	}

	// Verify Metrics Restoration in memory
	nodeB.FSM.metricsMu.RLock()
	seriesB, ok := nodeB.FSM.metrics.ClusterMetrics["testCount"]
	nodeB.FSM.metricsMu.RUnlock()
	if !ok {
		t.Errorf("Node B missing restored metrics in memory")
	} else {
		points := seriesB.Buffers["1m"].GetPoints()
		if len(points) == 0 || points[0].Value != 42.0 {
			t.Errorf("Node B metrics data mismatch: %v", points)
		}
	}

	t.Log("Test Complete")
}

func getPort(addr string) int {
	_, portStr, _ := net.SplitHostPort(addr)
	p, _ := strconv.Atoi(portStr)
	return p
}
