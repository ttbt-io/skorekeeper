package backend

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
)

func TestRaft_Snapshot_FullRestore(t *testing.T) {
	// 1. Setup Infrastructure
	baseDir := t.TempDir()
	mk, _ := crypto.CreateAESMasterKeyForTest()

	// Helper to create a node
	createNode := func(id string, portRaft int, portHTTP int, bootstrap bool) (*RaftManager, string) {
		dir := filepath.Join(baseDir, id)
		raftDir := filepath.Join(dir, "raft")

		addrRaft := fmt.Sprintf("127.0.0.1:%d", portRaft)
		addrHTTP := fmt.Sprintf("127.0.0.1:%d", portHTTP)

		s := storage.New(dir, mk)

		gs := NewGameStore(dir, s)
		ts := NewTeamStore(dir, s)
		us := NewUserIndexStore(dir, s, nil)
		r := NewRegistry(gs, ts, us, true)
		hm := NewHubManager()
		fsm := NewFSM(gs, ts, r, hm, s, us)

		rm := NewRaftManager(raftDir, addrRaft, addrRaft, addrHTTP, addrHTTP, "secret", mk, fsm)

		// Configure for fast testing

		rm.UseProductionTimeouts = false

		rm.SnapshotThreshold = 100

		rm.TrailingLogs = 10

		if err := rm.Start(bootstrap); err != nil {

			t.Fatalf("Failed to start node %s: %v", id, err)
		}

		return rm, dir
	}

	// 2. Start Node A (Leader)
	portRaftA := getPort(getFreePort())
	portHTTPA := getPort(getFreePort())
	nodeA, _ := createNode("A", portRaftA, portHTTPA, true)
	defer nodeA.Shutdown()

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

	// Generate logs to force compaction (SnapshotThreshold is 200? TrailingLogs is 100)
	// We need enough logs so that Snapshot() actually deletes old logs.
	for i := 0; i < 150; i++ {
		// We use CmdSaveGame with minimal data just to create log entries
		// Actually invalid game JSON might fail Apply, but Log is created.
		// Better use valid command. CmdMetricsUpdate is small.
		metricsPayload := &MetricsPayload{Timestamp: int64(i)}
		nodeA.Propose(RaftCommand{Type: CmdMetricsUpdate, MetricsPayload: metricsPayload})
	}

	// 4. Force Snapshot on Node A

	if future := nodeA.Raft.Snapshot(); future.Error() != nil {
		t.Fatalf("Failed to snapshot A: %v", future.Error())
	}

	// Ensure snapshot is done
	time.Sleep(1 * time.Second)

	// Create logs AFTER snapshot to force log replication + snapshot
	// (Actually snapshot is enough if we join a fresh node)

	// 5. Start Node B (Fresh)
	portRaftB := getPort(getFreePort())
	portHTTPB := getPort(getFreePort())
	nodeB, dirB := createNode("B", portRaftB, portHTTPB, false)
	defer nodeB.Shutdown()

	// 6. Join Node B to Cluster
	t.Log("Joining Node B to Cluster...")
	// We need Node B's pubkey
	pubKeyB := base64.StdEncoding.EncodeToString(nodeB.PubKey)

	err := nodeA.Join(nodeB.NodeID, nodeB.Bind, nodeB.ClusterAdvertise, pubKeyB, false, "v1", 1, 1)
	if err != nil {
		t.Fatalf("Failed to join B: %v", err)
	}

	// 7. Verify Restoration on Node B
	t.Log("Waiting for Node B to sync...")
	// Wait for B to have the game
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		gsB, _ := nodeB.FSM.GetStores()
		if _, err := gsB.LoadGame("game-1"); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
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

	// Policy (Crucial Check)
	// AccessControl reads from Registry. Registry reads from memory. FSM loads from disk on startup/restore.
	// Check if `sys_access_policy` file exists on B's disk.
	if _, err := os.Stat(filepath.Join(dirB, "sys_access_policy")); err != nil {
		t.Errorf("Node B missing sys_access_policy file: %v", err)
	}

	// Check memory state
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
	sysFiles := []string{"metrics.json", "nodes.json", "fsm_state.json"}
	for _, f := range sysFiles {
		if _, err := os.Stat(filepath.Join(dirB, f)); err != nil {
			t.Errorf("Node B missing restored file %s: %v", f, err)
		}
	}

	// Verify fsm_state.json content
	var stateB map[string]any
	// Using nodeB.FSM.storage might be better, but we already have dirB
	storageB := storage.New(dirB, mk)
	if err := storageB.ReadDataFile("fsm_state.json", &stateB); err != nil {
		t.Errorf("Failed to read node B fsm_state.json: %v", err)
	} else {
		idx, ok := stateB["lastAppliedIndex"].(float64) // JSON unmarshal uses float64 for numbers
		if !ok || uint64(idx) == 0 {
			t.Errorf("Node B fsm_state.json has invalid index: %v", stateB["lastAppliedIndex"])
		} else {
			t.Logf("Node B fsm_state.json index: %d", uint64(idx))
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
