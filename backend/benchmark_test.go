package backend

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/hashicorp/raft"
)

func BenchmarkRaftScaling(b *testing.B) {
	// Configuration
	numGames := 1000
	numTeams := 100
	gamesPerTeam := numGames / numTeams

	// Data Generators
	generateData := func(dir string) error {
		// Create Teams
		for i := 0; i < numTeams; i++ {
			teamID := fmt.Sprintf("team-%04d", i)
			team := Team{
				ID:      teamID,
				Name:    fmt.Sprintf("Team %d", i),
				OwnerID: "bench@benchmark.com",
			}
			data, _ := json.Marshal(team)
			if err := os.WriteFile(filepath.Join(dir, "teams", teamID+".json"), data, 0644); err != nil {
				return err
			}
		}

		// Create Games
		for i := 0; i < numTeams; i++ {
			teamID := fmt.Sprintf("team-%04d", i)
			for j := 0; j < gamesPerTeam; j++ {
				gameID := fmt.Sprintf("game-%s-%04d", teamID, j)
				game := Game{
					ID:         gameID,
					AwayTeamID: teamID,
					HomeTeamID: "team-opponent",
					Date:       time.Now().Format(time.RFC3339),
					Status:     "scheduled",
					OwnerID:    "bench@benchmark.com",
				}
				data, _ := json.Marshal(game)
				if err := os.WriteFile(filepath.Join(dir, "games", gameID+".json"), data, 0644); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Scenario A: Storage Boot & Indexing
	// This benchmark is fast enough to run b.N times
	b.Run("Scenario A: Storage Boot", func(b *testing.B) {
		// We can't reuse the same directory easily because stores lock files.
		// We must setup/teardown per iteration.
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			dir := b.TempDir()
			os.MkdirAll(filepath.Join(dir, "games"), 0755)
			os.MkdirAll(filepath.Join(dir, "teams"), 0755)
			if err := generateData(dir); err != nil {
				b.Fatalf("Failed to generate data: %v", err)
			}
			b.StartTimer()

			s := storage.New(dir, nil)
			gs := NewGameStore(dir, s)
			ts := NewTeamStore(dir, s)
			reg := NewRegistry(gs, ts)

			// Access something to ensure lazy loading/indexing is complete if applicable
			games := reg.ListGames("bench@benchmark.com", "", "", "")
			if len(games) != numGames {
				b.Errorf("Expected %d games, got %d", numGames, len(games))
			}
		}
	})

	// Helpers for Raft scenarios
	setupRaftNode := func(dir string, bootstrap bool, existingFSM *FSM, useGob bool) (*RaftManager, *FSM) {
		dataDir := filepath.Join(dir, "data")
		raftDir := filepath.Join(dir, "raft")

		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		bind := ln.Addr().String()
		ln.Close()

		cln, _ := net.Listen("tcp", "127.0.0.1:0")
		clusterAddr := cln.Addr().String()
		cln.Close()

		s := storage.New(dataDir, nil)
		rs := storage.New(raftDir, nil)
		gs := NewGameStore(dataDir, s)
		ts := NewTeamStore(dataDir, s)
		reg := NewRegistry(gs, ts)
		hm := NewHubManager()

		var fsm *FSM
		if existingFSM != nil {
			fsm = existingFSM
		} else {
			fsm = NewFSM(gs, ts, reg, hm, rs)
		}
		rm := NewRaftManager(raftDir, bind, bind, clusterAddr, clusterAddr, "bench-secret", nil, fsm)
		rm.LogOutput = io.Discard // Discount log I/O overhead
		rm.UseGob = useGob

		if err := rm.Start(bootstrap); err != nil {
			b.Fatalf("Failed to start node %s: %v", rm.NodeID, err)
		}

		return rm, fsm
	}

	waitForLeader := func(rm *RaftManager) {
		timeout := time.After(10 * time.Second)
		for {
			select {
			case <-timeout:
				b.Fatal("Timed out waiting for leader")
			default:
				if rm.Raft.State() == raft.Leader {
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	for _, encoding := range []string{"JSON", "GOB"} {
		useGob := encoding == "GOB"

		// Scenario B: Raft Log Replication (Log Replay)
		b.Run("Scenario B: Log Replication/"+encoding, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				dir1 := b.TempDir()
				dir2 := b.TempDir()

				// 1. Start Leader
				rm1, _ := setupRaftNode(dir1, true, nil, useGob)
				waitForLeader(rm1)

				// 2. Pump Data (Propose)
				var wg sync.WaitGroup
				concurrency := 10
				wg.Add(concurrency)
				for w := 0; w < concurrency; w++ {
					go func(workerID int) {
						defer wg.Done()
						for k := workerID; k < numGames; k += concurrency {
							game := Game{
								ID:      fmt.Sprintf("log-game-%d", k),
								OwnerID: "bench",
							}
							data, _ := json.Marshal(game)
							raw := json.RawMessage(data)
							cmd := RaftCommand{
								Type:     CmdSaveGame,
								ID:       game.ID,
								GameData: &raw,
							}
							if _, err := rm1.Propose(cmd); err != nil {
								return
							}
						}
					}(w)
				}
				wg.Wait()

				// 3. Start Follower (Fresh)
				rm2, fsm2 := setupRaftNode(dir2, false, nil, useGob)

				// 4. Join & Measure
				pubKey := base64.StdEncoding.EncodeToString(rm2.PubKey)

				b.StartTimer()
				if err := rm1.Join("node-2", rm2.Bind, rm2.ClusterAdvertise, pubKey, false, CurrentAppVersion, CurrentProtocolVersion, CurrentSchemaVersion); err != nil {
					b.Fatalf("Join failed: %v", err)
				}

				timeout := time.After(120 * time.Second)
				synced := false
				for !synced {
					select {
					case <-timeout:
						b.Fatalf("Timeout waiting for sync")
					default:
						count := 0
						for range fsm2.gs.ListAllGames() {
							count++
						}
						if count >= numGames {
							synced = true
						} else {
							time.Sleep(500 * time.Millisecond)
						}
					}
				}

				// Verify Data Integrity
				verifyData := func(rm *RaftManager, fsm *FSM) {
					gs, _ := fsm.GetStores()
					count := 0
					for g, _ := range gs.ListAllGames() {
						count++
						if g.OwnerID != "bench@benchmark.com" && g.OwnerID != "bench" {
							b.Errorf("Unexpected OwnerID for game %s: %s", g.ID, g.OwnerID)
						}
					}
					if count != numGames {
						b.Errorf("Data verification failed. Expected %d games, got %d", numGames, count)
					}
				}
				verifyData(rm2, fsm2)

				// Cleanup
				rm1.Shutdown()
				rm2.Shutdown()
			}
		})

		// Scenario C: Snapshot Restore
		b.Run("Scenario C: Snapshot Restore/"+encoding, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				dir1 := b.TempDir()
				dir2 := b.TempDir()

				// 1. Start Leader
				rm1, _ := setupRaftNode(dir1, true, nil, useGob)
				waitForLeader(rm1)

				// 2. Pump Data
				var wg sync.WaitGroup
				concurrency := 10
				wg.Add(concurrency)
				for w := 0; w < concurrency; w++ {
					go func(workerID int) {
						defer wg.Done()
						for k := workerID; k < numGames; k += concurrency {
							game := Game{
								ID:      fmt.Sprintf("snap-game-%d", k),
								OwnerID: "bench",
							}
							data, _ := json.Marshal(game)
							raw := json.RawMessage(data)
							cmd := RaftCommand{
								Type:     CmdSaveGame,
								ID:       game.ID,
								GameData: &raw,
							}
							_, _ = rm1.Propose(cmd)
						}
					}(w)
				}
				wg.Wait()

				// 3. Force Snapshot
				future := rm1.Raft.Snapshot()
				if err := future.Error(); err != nil {
					b.Fatalf("Snapshot failed: %v", err)
				}

				// 4. Start Follower
				rm2, fsm2 := setupRaftNode(dir2, false, nil, useGob)

				// 5. Join & Measure
				pubKey := base64.StdEncoding.EncodeToString(rm2.PubKey)

				b.StartTimer()
				if err := rm1.Join("node-2", rm2.Bind, rm2.ClusterAdvertise, pubKey, false, CurrentAppVersion, CurrentProtocolVersion, CurrentSchemaVersion); err != nil {
					b.Fatalf("Join failed: %v", err)
				}

				timeout := time.After(120 * time.Second)
				synced := false
				for !synced {
					select {
					case <-timeout:
						b.Fatalf("Timeout waiting for snapshot sync")
					default:
						count := 0
						for _, _ = range fsm2.gs.ListAllGames() {
							count++
						}
						if count >= numGames {
							synced = true
						} else {
							time.Sleep(500 * time.Millisecond)
						}
					}
				}

				// Verify Data Integrity
				verifyData := func(rm *RaftManager, fsm *FSM) {
					gs, _ := fsm.GetStores()
					count := 0
					for g, _ := range gs.ListAllGames() {
						count++
						if g.OwnerID != "bench" { // Snapshots use "bench" from loop above
							b.Errorf("Unexpected OwnerID for game %s: %s", g.ID, g.OwnerID)
						}
					}
					if count != numGames {
						b.Errorf("Data verification failed. Expected %d games, got %d", numGames, count)
					}
				}
				verifyData(rm2, fsm2)

				rm1.Shutdown()
				rm2.Shutdown()
			}
		})
	}
}
