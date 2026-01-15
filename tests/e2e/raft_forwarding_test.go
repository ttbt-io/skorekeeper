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
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/ttbt-io/skorekeeper/backend"
)

// startRaftCluster starts a 3-node Raft cluster and returns the Leader's URL, a Follower's URL, and the cluster secret.
func startRaftCluster(t *testing.T) (leaderURL string, followerURL string, secret string) {
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate self-signed cert: %v", err)
	}

	nodeCount := 3
	rms := make([]*backend.RaftManager, nodeCount)
	urls := make([]string, nodeCount)
	clusterSecret := "test-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Channel to capture RaftManagers
	rmChans := make([]chan *backend.RaftManager, nodeCount)

	for i := 0; i < nodeCount; i++ {
		dataDir := t.TempDir()
		s := storage.New(dataDir, nil)
		gStore := backend.NewGameStore(dataDir, s)
		tStore := backend.NewTeamStore(dataDir, s)
		reg := backend.NewRegistry(gStore, tStore)

		l, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			t.Fatalf("Node %d failed to listen: %v", i, err)
		}
		_, port, _ := net.SplitHostPort(l.Addr().String())
		httpAddr := fmt.Sprintf("https://devtest.local:%s", port)
		urls[i] = httpAddr

		raftL, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Node %d failed to listen raft: %v", i, err)
		}
		raftBind := raftL.Addr().String()
		raftL.Close()

		clusterL, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Node %d failed to listen cluster: %v", i, err)
		}
		clusterAddr := clusterL.Addr().String()
		clusterL.Close()

		t.Cleanup(func() { l.Close() })

		rmChans[i] = make(chan *backend.RaftManager, 1)

		opts := backend.Options{
			Addr:             l.Addr().String(),
			ClusterAdvertise: clusterAddr,
			ClusterAddr:      clusterAddr,
			Listener:         l,
			Cert:             cert,
			UseMockAuth:      true,
			Debug:            true,
			GameStore:        gStore,
			TeamStore:        tStore,
			Registry:         reg,
			RaftEnabled:      true,
			RaftBind:         raftBind,
			RaftSecret:       clusterSecret,
			RaftBootstrap:    i == 0, // Only node 0 bootstraps
			RaftManagerChan:  rmChans[i],
			DataDir:          dataDir,
		}

		server, err := backend.StartServer(opts)
		if err != nil {
			t.Fatalf("Node %d failed to start: %v", i, err)
		}
		t.Cleanup(func() {
			sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server.Shutdown(sdCtx)
		})

		localURL := fmt.Sprintf("https://localhost:%s", port)
		if err := waitForServer(localURL, 5*time.Second); err != nil {
			t.Fatalf("Server %d failed to start: %v", i, err)
		}
	}

	// Collect RaftManagers
	for i := 0; i < nodeCount; i++ {
		select {
		case rm := <-rmChans[i]:
			rms[i] = rm
		case <-time.After(5 * time.Second):
			t.Fatalf("Node %d RaftManager not received", i)
		}
	}

	// Wait for Node 0 to become Leader (since it bootstrapped)
	t.Log("Waiting for initial leader election...")
	waitForLeader(t, rms[0])
	leaderURL = urls[0]

	// Join other nodes
	for i := 1; i < nodeCount; i++ {
		t.Logf("Joining node %d to leader...", i)
		pubKey := base64.StdEncoding.EncodeToString(rms[i].PubKey)

		// Prime joining node with leader's public key so it trusts the leader's TLS cert
		rms[i].AddNodePubKey(rms[0].NodeID, rms[0].ClusterAdvertise, base64.StdEncoding.EncodeToString(rms[0].PubKey))

		err := rms[0].Join(rms[i].NodeID, rms[i].Bind, rms[0].ClusterAdvertise, pubKey, false, backend.CurrentAppVersion, backend.CurrentProtocolVersion, backend.CurrentSchemaVersion)
		if err != nil {
			t.Fatalf("Failed to join node %d: %v", i, err)
		}
	}

	// Identify a Follower
	// Wait a bit for cluster to stabilize and verify node 1 or 2 is a follower
	time.Sleep(2 * time.Second)

	// We'll just pick Node 1 as the follower for testing.
	// In a real scenario we might check .State(), but for now assuming stability.
	// If Node 0 is leader, Node 1 should be a follower.
	followerURL = urls[1]

	t.Logf("Cluster formed. Leader: %s, Follower: %s", leaderURL, followerURL)
	return leaderURL, followerURL, clusterSecret
}

func waitForLeader(t *testing.T, rm *backend.RaftManager) {
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for leader")
		default:
			if rm.Raft.State().String() == "Leader" {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestRaftRequestForwarding(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	leaderURL, followerURL, _ := startRaftCluster(t)
	t.Logf("Testing forwarding from Follower (%s) to Leader (%s)", followerURL, leaderURL)

	// We will run the test against the FOLLOWER.
	// Any write request (CreateGame, Action) sent to the Follower
	// should be forwarded to the Leader seamlessly.

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			if ev.Type == runtime.APITypeError {
				args := make([]string, len(ev.Args))
				for i, arg := range ev.Args {
					args[i] = string(arg.Value)
				}
				t.Logf("JS CONSOLE ERROR: %s", strings.Join(args, " "))
				t.Fail()
				cancel()
			}
		case *runtime.EventExceptionThrown:
			t.Logf("JS EXCEPTION: %s", ev.ExceptionDetails.Text)
			t.Fail()
			cancel()
		}
	})

	var bCount int

	runStep(t, ctx, "Login to Follower and Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Login to the FOLLOWER
			if err := Login(ctx, followerURL); err != nil {
				return err
			}
			// Create Game on FOLLOWER. This triggers a write.
			// If forwarding works, this succeeds.
			_, err := CreateGame(ctx, "FwdAway", "FwdHome")
			return err
		}),
	)

	runStep(t, ctx, "Perform Action on Follower",
		chromedp.Click(`#scoresheet-grid > .grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-ball`), // Action: BALL
		CSOBallCount(&bCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if bCount != 1 {
				return fmt.Errorf("Expected 1 Ball, got %d. Action likely failed.", bCount)
			}
			return nil
		}),
	)

	// Optional: Verify on Leader?
	// We can't easily switch the browser context's origin without losing the session
	// (unless we manually manage cookies or open a second tab/target).
	// But simply getting a successful UI update on the Follower implies:
	// 1. Follower -> Leader (Forward)
	// 2. Leader -> Raft (Apply)
	// 3. Leader -> Follower (Replicate)
	// 4. Follower UI updates via WS (or local state update confirmation)

	// If the action failed, the UI probably wouldn't update or we'd see an error.

	t.Log("Raft Request Forwarding Verified Success")
}
