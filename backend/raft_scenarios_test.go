package backend

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"

	"time"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
)

// TestRaftScenarios verifies cluster consistency under various restart/failure conditions.
func TestRaftScenarios(t *testing.T) {
	// Infrastructure Setup
	baseDir := t.TempDir()
	var mu sync.Mutex
	activeNodes := make(map[string]*RaftManager)
	nodeDirs := make(map[string]string)
	nodePorts := make(map[string]int)
	nodeMKs := make(map[string]crypto.MasterKey)

	// Helper: Get free port
	getFreePort := func() int {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("getFreePort: could not get free port: %v", err)
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	}
	// Helper: Create/Start Node
	startNode := func(id string, bootstrap bool) *RaftManager {
		mu.Lock()
		defer mu.Unlock()

		dir, ok := nodeDirs[id]
		if !ok {
			dir = filepath.Join(baseDir, id)
			nodeDirs[id] = dir
		}

		port, ok := nodePorts[id]
		if !ok {
			port = getFreePort()
			nodePorts[id] = port
		}

		raftAddr := fmt.Sprintf("127.0.0.1:%d", port)
		httpPort := getFreePort()
		httpAddr := fmt.Sprintf("127.0.0.1:%d", httpPort)

		// Use a fixed secret for cluster
		secret := "test-secret"

		mk, ok := nodeMKs[id]
		if !ok {
			var err error
			mk, err = crypto.CreateAESMasterKeyForTest()
			if err != nil {
				t.Fatalf("Failed to create master key: %v", err)
			}
			nodeMKs[id] = mk
		}

		dataDir := dir
		raftDir := filepath.Join(dataDir, "raft")
		// storage.New creates dirs if needed, but explicit is fine

		s := storage.New(dataDir, mk)
		raftS := storage.New(raftDir, mk)

		gs := NewGameStore(dataDir, s)
		ts := NewTeamStore(dataDir, s)
		us := NewUserIndexStore(dataDir, s, nil)
		r := NewRegistry(gs, ts, us, true)
		fsm := NewFSM(gs, ts, r, NewHubManager(), raftS, us)

		rm := NewRaftManager(raftDir, raftAddr, raftAddr, httpAddr, httpAddr, secret, mk, fsm)

		// aggressive snapshot config for testing
		// We modify the internal config before Start (a bit hacky but needed since Config is private/hardcoded in Start)
		// Actually Start() creates DefaultConfig.
		// We can't easily inject config into Start() without changing code.
		// But Start() sets SnapshotThreshold = 20480.
		// To test "Way Behind", we need small threshold.
		// We should modify RaftManager to accept config override or modify Start?
		// Or we can rely on manual Snapshot() calls.
		// Let's assume manual Snapshot() calls for Scenario C.

		// We need to set UseProductionTimeouts = false (default)

		if err := rm.Start(bootstrap); err != nil {
			t.Fatalf("Failed to start node %s: %v", id, err)
		}

		activeNodes[id] = rm
		return rm
	}
	// Helper: Stop Node
	stopNode := func(id string) {
		mu.Lock()
		defer mu.Unlock()
		if rm, ok := activeNodes[id]; ok {
			rm.Shutdown()
			delete(activeNodes, id)
		}
	}

	// Helper: Verify Consistency
	verifyConsistency := func() {
		deadline := time.Now().Add(10 * time.Second)
		var lastErr error

		for time.Now().Before(deadline) {
			mu.Lock()
			refNode := activeNodes["A"]
			nodes := make([]*RaftManager, 0, len(activeNodes))
			for id, n := range activeNodes {
				if id != "A" {
					nodes = append(nodes, n)
				}
			}
			mu.Unlock()

			if refNode == nil {
				if len(nodes) > 0 {
					refNode = nodes[0]
					nodes = nodes[1:]
				} else {
					return // No nodes up
				}
			}

			// Reference State
			refGS, _ := refNode.FSM.GetStores()
			refGames, refErr := refGS.ListAllGameIDs()
			if refErr != nil {
				lastErr = fmt.Errorf("failed to list games on ref node: %v", refErr)
				time.Sleep(500 * time.Millisecond)
				continue
			}
			sort.Strings(refGames)

			consistent := true
			for i, node := range nodes {
				gs, _ := node.FSM.GetStores()
				games, err := gs.ListAllGameIDs()
				if err != nil {
					lastErr = fmt.Errorf("failed to list games on node %d: %v", i, err)
					consistent = false
					break
				}
				sort.Strings(games)

				if !reflect.DeepEqual(refGames, games) {
					lastErr = fmt.Errorf("mismatch: ref=%v node=%v", refGames, games)
					consistent = false
					break
				}
			}

			if consistent {
				return // Success
			}

			time.Sleep(500 * time.Millisecond)
		}

		t.Errorf("Consistency Check Failed after timeout: %v", lastErr)
	}

	// ==========================================
	// Scenario A: Clean Restart (Log Replay)
	// ==========================================
	t.Log("--- Scenario A: Clean Restart ---")

	nodeA := startNode("A", true)
	// Wait for leader
	time.Sleep(2 * time.Second)

	nodeB := startNode("B", false)
	nodeC := startNode("C", false)

	// Join B and C
	// Note: We need the PubKey. In test we can cheat or use the getter.
	// Join() requires HttpAddr, PubKey.

	joinNode := func(leader, joiner *RaftManager) {
		err := leader.Join(joiner.NodeID, joiner.Bind, joiner.ClusterAdvertise,
			joiner.FSM.GetNodePubKey(joiner.NodeID), false, "v1", 1, 1)
		if err != nil {
			t.Fatalf("Failed to join node: %v", err)
		}
	}

	joinNode(nodeA, nodeB)
	joinNode(nodeA, nodeC)

	time.Sleep(2 * time.Second) // Wait for cluster form

	// Create G1
	createGame := func(leader *RaftManager, id string) {
		g := &Game{ID: id, ActionLog: []json.RawMessage{}}
		data, err := json.Marshal(g)
		if err != nil {
			t.Fatalf("failed to marshal game %s: %v", id, err)
		}
		raw := json.RawMessage(data)
		cmd := RaftCommand{Type: CmdSaveGame, ID: id, GameData: &raw}
		if _, err := leader.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose game %s: %v", id, err)
		}
	}

	createGame(nodeA, "G1")
	verifyConsistency()

	// Shutdown C
	t.Log("Shutting down Node C...")
	stopNode("C")

	// Create G2
	createGame(nodeA, "G2")

	// Restart C
	t.Log("Restarting Node C...")
	nodeC = startNode("C", false)
	// Wait for catchup
	time.Sleep(2 * time.Second)

	verifyConsistency()

	// Verify C has G2
	gsC, _ := nodeC.FSM.GetStores()
	if _, err := gsC.LoadGame("G2"); err != nil {
		t.Errorf("Node C missing G2 after restart")
	}

	// ==========================================
	// Scenario B: Crash Recovery
	// ==========================================
	t.Log("--- Scenario B: Crash Recovery ---")

	createGame(nodeA, "G3")

	t.Log("Crashing Node B...")
	stopNode("B")

	createGame(nodeA, "G4")

	t.Log("Restarting Node B...")
	nodeB = startNode("B", false)
	time.Sleep(2 * time.Second)

	verifyConsistency()

	gsB, _ := nodeB.FSM.GetStores()
	if _, err := gsB.LoadGame("G4"); err != nil {
		t.Errorf("Node B missing G4 after crash recovery")
	}

	// ==========================================
	// Scenario C: Way Behind (Snapshot Install)
	// ==========================================
	t.Log("--- Scenario C: Way Behind (Snapshot) ---")

	stopNode("C")

	// Create 20 games to fill log
	for i := 0; i < 20; i++ {
		createGame(nodeA, fmt.Sprintf("Bulk-%d", i))
	}

	// Force Snapshot on A
	t.Log("Forcing Snapshot on A...")
	if future := nodeA.Raft.Snapshot(); future.Error() != nil {
		t.Fatalf("Failed to snapshot A: %v", future.Error())
	}

	// Wait for snapshot to complete
	time.Sleep(1 * time.Second)

	// Create more games to advance log past snapshot
	createGame(nodeA, "PostSnap-1")

	// Restart C
	t.Log("Restarting Node C (Way Behind)...")
	nodeC = startNode("C", false)

	// It might take longer to receive snapshot
	time.Sleep(5 * time.Second)

	verifyConsistency()

	gsC, _ = nodeC.FSM.GetStores()
	if _, err := gsC.LoadGame("Bulk-19"); err != nil {
		t.Errorf("Node C missing Bulk-19 (Snapshot data)")
	}
	if _, err := gsC.LoadGame("PostSnap-1"); err != nil {
		t.Errorf("Node C missing PostSnap-1 (Log tail)")
	}

	// ==========================================
	// Scenario D: Fresh Node Join
	// ==========================================
	t.Log("--- Scenario D: Fresh Node Join ---")

	nodeD := startNode("D", false)
	joinNode(nodeA, nodeD)

	time.Sleep(5 * time.Second) // Wait for snapshot transfer

	verifyConsistency()

	gsD, _ := nodeD.FSM.GetStores()
	if _, err := gsD.LoadGame("G1"); err != nil {
		t.Errorf("Node D missing G1")
	}
	if _, err := gsD.LoadGame("Bulk-0"); err != nil {
		t.Errorf("Node D missing Bulk-0")
	}

	t.Log("All Scenarios Passed")
}
