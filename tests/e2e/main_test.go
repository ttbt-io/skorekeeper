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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/ttbt-io/skorekeeper/backend"
)

var (
	withChromeDP = flag.String("with-chromedp", "", "The url of the remote debugging port")
	raftNodes    = flag.Int("raft-nodes", 3, "Number of Raft nodes to start")
	useMinify    = flag.Bool("minify", false, "Use minified assets")
)

func TestMain(m *testing.M) {
	flag.Parse()
	exitCode := m.Run()
	os.Exit(exitCode)
}

func startTestServer(t *testing.T) string {
	return startTestServerWithFlags(t, nil)
}

func startTestServerWithFlags(t *testing.T, flags []string) string {
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate self-signed cert: %v", err)
	}

	// Parse flags for bootstrap admin
	var bootstrapAdmin string
	for i, f := range flags {
		if f == "--admin" && i+1 < len(flags) {
			bootstrapAdmin = flags[i+1]
		}
	}

	nodeCount := *raftNodes
	if nodeCount < 1 {
		nodeCount = 1
	}

	var leaderURL string
	var leaderRM *backend.RaftManager
	rms := make([]*backend.RaftManager, nodeCount)
	clusterSecret := "test-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())

	for i := 0; i < nodeCount; i++ {
		// Unique temp dir for each node
		dataDir := t.TempDir()

		// Independent Stores for each node
		s := storage.New(dataDir, nil)
		gStore := backend.NewGameStore(dataDir, s)
		tStore := backend.NewTeamStore(dataDir, s)
		reg := backend.NewRegistry(gStore, tStore)

		// Listen on a random free port on all interfaces (IPv4 forced)
		l, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			t.Fatalf("Node %d failed to listen: %v", i, err)
		}
		_, port, _ := net.SplitHostPort(l.Addr().String())
		httpAddr := fmt.Sprintf("https://devtest.local:%s", port)

		// Get a free port for Raft
		raftL, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Node %d failed to listen raft: %v", i, err)
		}
		raftBind := raftL.Addr().String()
		raftL.Close() // Close it, Raft will open it

		clusterL, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Node %d failed to listen cluster: %v", i, err)
		}
		clusterAddr := clusterL.Addr().String()
		clusterL.Close()

		t.Cleanup(func() { l.Close() })

		rmChan := make(chan *backend.RaftManager, 1)

		opts := backend.Options{
			Addr:             httpAddr, // Passing full URL as Addr for internal forwarding
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
			RaftBootstrap:    i == 0,
			RaftManagerChan:  rmChan,
			DataDir:          dataDir,
			BootstrapAdmin:   bootstrapAdmin,
			MinifyMode:       *useMinify,
		}

		// Start backend with specific store and raft options
		server, err := backend.StartServer(opts)
		if err != nil {
			t.Fatalf("Node %d failed to start: %v", i, err)
		}
		t.Cleanup(func() {
			sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server.Shutdown(sdCtx)
		})

		// Capture RaftManager
		select {
		case rm := <-rmChan:
			rms[i] = rm
			if i == 0 {
				leaderRM = rm
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("Node %d RaftManager not received", i)
		}

		localURL := fmt.Sprintf("https://localhost:%s", port)
		if err := waitForServer(localURL, 5*time.Second); err != nil {
			t.Fatalf("Server %d failed to start: %v", i, err)
		}

		if i == 0 {
			leaderURL = httpAddr
		}
	}

	if nodeCount > 1 {
		// Wait for Leader Election
		t.Log("Waiting for leader election...")
		timeout := time.After(10 * time.Second)
		elected := false
		for {
			select {
			case <-timeout:
				t.Fatal("Timeout waiting for leader")
			default:
				if leaderRM.Raft.State().String() == "Leader" {
					elected = true
				}
			}
			if elected {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Join other nodes
		for i := 1; i < nodeCount; i++ {
			t.Logf("Joining node %d to leader...", i)
			// Calculate joining node's HTTP Addr
			joinHttpAddr := rms[i].ClusterAdvertise
			pubKey := base64.StdEncoding.EncodeToString(rms[i].PubKey)

			// Prime joining node with leader's public key so it trusts the leader's TLS cert
			rms[i].AddNodePubKey(rms[0].NodeID, rms[0].ClusterAdvertise, base64.StdEncoding.EncodeToString(rms[0].PubKey))

			err := leaderRM.Join(rms[i].NodeID, rms[i].Bind, joinHttpAddr, pubKey, false, backend.CurrentAppVersion, backend.CurrentProtocolVersion, backend.CurrentSchemaVersion)
			if err != nil {
				t.Fatalf("Failed to join node %d: %v", i, err)
			}
		}
		t.Logf("Raft cluster of %d nodes formed.", nodeCount)
	}

	return leaderURL
}

func generateSelfSignedCert() (*tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "devtest", "devtest.local", "devtest.public"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &cert, nil
}

func waitForServer(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: tr}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	for start := time.Now(); time.Since(start) < timeout; {
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Printf("Server at %s is ready!", url)
			return nil
		}
		log.Printf("waitForServer(%q): %v", url, err)
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return fmt.Errorf("timeout waiting for server at %s", url)
}

func runStep(t *testing.T, ctx context.Context, description string, actions ...chromedp.Action) {
	t.Helper()
	t.Logf("STEP: %s", description)
	runAction := func(i int, action chromedp.Action) {
		t.Helper()
		done := make(chan bool)
		defer close(done)
		go func() {
			d, ok := ctx.Deadline()
			if !ok {
				return
			}
			left := time.Until(d) - 5*time.Second
			select {
			case <-done:
				return
			case <-time.After(left):
				CaptureScreenshot(ctx, "/demo/debug-5-sec-left.png")
			case <-time.After(10 * time.Second):
				t.Logf("STEP %s [Action#%d]: single action took more than 10 sec", description, i)
				CaptureScreenshot(ctx, "/demo/debug-single-action-timeout.png")
			}
		}()
		if err := chromedp.Run(ctx, action); err != nil {
			CaptureScreenshot(ctx, "/demo/debug-failed-action.png")
			t.Fatalf("STEP FAILED: %s [Action#%d]: %v", description, i, err)
		}
	}
	for i, action := range actions {
		runAction(i, action)
	}
}

// ... Tests ...

func TestComprehensiveWorkflow(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

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

	var pitcherText string
	var bCount, sCount int
	var cellText string
	var scoreText string

	runStep(t, ctx, "Navigate and Start Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "Dragons", "Knights")
			return err
		}),
	)

	runStep(t, ctx, "Pitcher Tracking Check",
		chromedp.Click(`#scoresheet-grid > .grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Text(`#cso-pitcher-num`, &pitcherText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if pitcherText != "" {
				t.Errorf("Expected empty pitcher, got %s", pitcherText)
				return fmt.Errorf("Pitcher tracking mismatch")
			}
			t.Log("Pitcher Tracking verified (empty initially)")
			return nil
		}),
	)

	runStep(t, ctx, "At-Bat & Undo",
		chromedp.Click(`#btn-ball`),
		chromedp.Click(`#btn-ball`),
		chromedp.Click(`#btn-strike`),
		CSOBallCount(&bCount),
		CSOStrikeCount(&sCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if bCount != 2 || sCount != 1 {
				t.Errorf("Count mismatch: %d-%d", bCount, sCount)
				return fmt.Errorf("At-Bat count mismatch")
			}
			return nil
		}),

		chromedp.Click(`#btn-undo-pitch`),
		CSOStrikeCount(&sCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if sCount != 0 {
				t.Errorf("Undo failed, strikes: %d", sCount)
				return fmt.Errorf("Undo failed")
			}
			t.Log("At-Bat Undo verified")
			return nil
		}),
	)

	runStep(t, ctx, "Strikeout",
		chromedp.Click(`#btn-strike`),
		chromedp.Click(`#btn-strike`),
		chromedp.Click(`#btn-strike`),
		waitUntilDisplayNone(`#cso-modal`),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("Strikeout recorded")
			return nil
		}),
		chromedp.Sleep(100*time.Millisecond),
	)

	runStep(t, ctx, "Substitution",
		chromedp.Evaluate(`
			const ev = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 100, clientY: 100 });
			document.querySelector('.lineup-cell').dispatchEvent(ev);
		`, nil),
		chromedp.WaitVisible(`#player-context-menu`),
		chromedp.Click(`#btn-open-sub`),
		chromedp.WaitVisible(`#substitution-modal`),
		chromedp.SetValue(`#sub-incoming-num`, "99"),
		chromedp.Click(`#btn-confirm-sub`),
		waitUntilDisplayNone(`#substitution-modal`),
		chromedp.Text(`.lineup-cell`, &cellText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(cellText, "Sub: #99") {
				t.Errorf("Substitution not displayed correctly. Got: %s", cellText)
				return fmt.Errorf("Substitution verification failed")
			}
			t.Log("Substitution verified")
			return nil
		}),
	)

	runStep(t, ctx, "Manual Score Override",
		chromedp.Evaluate(`
			const contextEv = new MouseEvent('contextmenu', { bubbles: true, cancelable: true });
			document.querySelector('#sb-innings-away .sb-cell').dispatchEvent(contextEv);
		`, nil),
		chromedp.WaitVisible(`#custom-prompt-modal`),
		chromedp.SetValue(`[data-test="custom-prompt-input"]`, "5"),
		chromedp.Click(`[data-test="custom-prompt-ok-btn"]`),
		waitUntilDisplayNone(`#custom-prompt-modal`),
		chromedp.WaitVisible(`#scoresheet-view`),
		chromedp.Text(`#sb-innings-away .sb-cell`, &scoreText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if scoreText != "5" {
				t.Errorf("Expected overridden score 5, got %s", scoreText)
				return fmt.Errorf("Score override mismatch")
			}
			t.Log("Score Override verified")
			return nil
		}),
	)

	runStep(t, ctx, "Global Undo/Redo",
		chromedp.Click(`#btn-undo`),
		chromedp.Sleep(50*time.Millisecond),
		chromedp.Text(`#sb-innings-away .sb-cell`, &scoreText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if scoreText == "5" {
				t.Error("Undo failed, score is still 5")
				return fmt.Errorf("Undo failed")
			}
			return nil
		}),

		chromedp.Click(`#btn-redo`),
		chromedp.Sleep(50*time.Millisecond),
		chromedp.Text(`#sb-innings-away .sb-cell`, &scoreText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if scoreText != "5" {
				t.Errorf("Redo failed, expected 5, got %s", scoreText)
				return fmt.Errorf("Redo failed")
			}
			t.Log("Global Undo/Redo verified")
			return nil
		}),
	)

	runStep(t, ctx, "Change Pitcher: Open CSO Modal",
		chromedp.Click(`#scoresheet-grid > .grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
	)
	runStep(t, ctx, "Change Pitcher: Click button",
		chromedp.Click(`#btn-change-pitcher`),
	)
	runStep(t, ctx, "Change Pitcher: Wait for modal",
		chromedp.WaitVisible(`#custom-prompt-modal`),
	)
	runStep(t, ctx, "Change Pitcher: Set value",
		chromedp.SetValue(`[data-test="custom-prompt-input"]`, "P3"),
	)
	runStep(t, ctx, "Change Pitcher: Click OK",
		chromedp.Click(`[data-test="custom-prompt-ok-btn"]`),
	)
	runStep(t, ctx, "Change Pitcher: Wait for modal to disappear",
		waitUntilDisplayNone(`#custom-prompt-modal`),
	)
	runStep(t, ctx, "Change Pitcher: Wait for UI update and verify text",
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Text(`#cso-pitcher-num`, &pitcherText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if pitcherText != "P3" {
				t.Errorf("Expected pitcher P3, got %s", pitcherText)
				return fmt.Errorf("Change Pitcher verification failed")
			}
			t.Log("Change Pitcher verified")
			return nil
		}),
	)
}

func TestScoresheetInteractions(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

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
			args := make([]string, len(ev.Args))
			for i, arg := range ev.Args {
				args[i] = string(arg.Value)
			}
			t.Logf("JS CONSOLE (%s): %s", ev.Type, strings.Join(args, " "))
			if ev.Type == runtime.APITypeError {
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

	var elementCount int
	var buttonText string
	var scoreText string
	var displayStyle string

	runStep(t, ctx, "Game Initialization",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "AwayTeam", "HomeTeam")
			return err
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Nav Bar: Verify buttons presence",
		chromedp.ActionFunc(func(ctx context.Context) error {
			selectors := []string{"#btn-menu-scoresheet", "#header-game-title", "#btn-undo", "#btn-redo"}
			for _, sel := range selectors {
				var count int
				err := chromedp.Evaluate(fmt.Sprintf(`document.querySelectorAll('%s').length`, sel), &count).Do(ctx)
				if err != nil {
					return err
				}
				if count == 0 {
					return fmt.Errorf("Nav bar element %s not found", sel)
				}
			}
			t.Log("Nav Bar elements verified")
			return nil
		}),
	)

	runStep(t, ctx, "Scoresheet: Verify Inning Scores display (empty cells)",
		chromedp.Evaluate(`document.querySelectorAll('#sb-innings-away .sb-cell').length`, &elementCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if elementCount == 0 {
				return fmt.Errorf("Expected inning score cells to be rendered, found %d", elementCount)
			}
			return nil
		}),
	)

	runStep(t, ctx, "CSO Reset: Re-opening CSO resets view but keeps data",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-ball`),
		CSOBallCount(&elementCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if elementCount != 1 {
				return fmt.Errorf("Expected 1 ball after click, got %d", elementCount)
			}
			return nil
		}),
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),

		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),

		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),

		CSOBallCount(&elementCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if elementCount != 1 {
				return fmt.Errorf("Data lost on re-open: Expected 1 ball, got %d", elementCount)
			}
			return nil
		}),

		chromedp.Evaluate(`window.getComputedStyle(document.getElementById('cso-bip-view')).display`, &displayStyle),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if displayStyle != "none" {
				return fmt.Errorf("View not reset: BiP view is still visible")
			}
			t.Log("CSO View Reset & Data Persistence verified")
			return nil
		}),
	)

	runStep(t, ctx, "Play Recorded: UI switches when outcome defined",
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),
		chromedp.Click(`#btn-save-bip`),
		waitUntilDisplayNone(`#cso-modal`),

		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.WaitVisible(`#action-area-recorded`),

		chromedp.Evaluate(`window.getComputedStyle(document.querySelector('#action-area-recorded')).display`, &displayStyle),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if displayStyle == "none" {
				return fmt.Errorf("Play Recorded view not visible")
			}
			return nil
		}),
		chromedp.Click(`#btn-toggle-action`),
		chromedp.WaitVisible(`#action-area-pitch`),
		chromedp.Evaluate(`window.getComputedStyle(document.querySelector('#action-area-pitch')).display`, &displayStyle),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if displayStyle == "none" {
				return fmt.Errorf("Pitch Pad view not visible after tap to edit")
			}
			t.Log("Play Recorded UI switch verified")
			return nil
		}),
	)

	runStep(t, ctx, "BiP Mode: Full cycle for btn-res and btn-type",
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),

		cycleTo(t, "#btn-res", "Out"),
		cycleTo(t, "#btn-res", "Ground"),
		cycleTo(t, "#btn-res", "Fly"),
		cycleTo(t, "#btn-res", "Line"),
		cycleTo(t, "#btn-res", "IFF"),
		cycleTo(t, "#btn-res", "Safe"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("btn-res cycle verified")
			return nil
		}),

		chromedp.Text(`#btn-type`, &buttonText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if buttonText != "HIT" {
				return fmt.Errorf("Expected HIT, got %s", buttonText)
			}
			return nil
		}),
		cycleTo(t, "#btn-type", "ERR"),
		cycleTo(t, "#btn-type", "FC"),
		cycleTo(t, "#btn-type", "HIT"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("btn-type (SAFE) cycle verified")
			return nil
		}),

		cycleTo(t, "#btn-res", "Fly"),
		chromedp.Sleep(50*time.Millisecond), chromedp.Text(`#btn-type`, &buttonText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if buttonText != "OUT" {
				return fmt.Errorf("Expected OUT, got %s", buttonText)
			}
			return nil
		}),
		cycleTo(t, "#btn-type", "SF"),
		cycleTo(t, "#btn-type", "OUT"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("btn-type (FLY) cycle verified")
			return nil
		}),

		cycleTo(t, "#btn-res", "Ground"),
		chromedp.Text(`#btn-type`, &buttonText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if buttonText != "OUT" {
				return fmt.Errorf("Expected OUT, got %s", buttonText)
			}
			return nil
		}),
		cycleTo(t, "#btn-type", "SH"),
		cycleTo(t, "#btn-type", "OUT"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("btn-type (GROUND) cycle verified")
			return nil
		}),

		chromedp.Click(`#btn-cancel-bip`),
		waitUntilDisplayNone(`#cso-bip-view`),
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep(t, ctx, "Dropped 3rd Strike workflow",
		chromedp.Click(`#scoresheet-grid > div:nth-child(13)`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-ball`),
		chromedp.Click(`#btn-strike`),
		chromedp.Click(`#btn-strike`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var x, y float64
			err := chromedp.Evaluate(`
				(() => {
					const el = document.querySelector('#btn-strike');
					const rect = el.getBoundingClientRect();
					return [rect.left + rect.width / 2, rect.top + rect.height / 2];
				})()
			`, &[]interface{}{&x, &y}).Do(ctx)
			if err != nil {
				return err
			}

			err = chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const el = document.querySelector('#btn-strike');
					const event = new MouseEvent('contextmenu', {
						bubbles: true,
						cancelable: true,
						clientX: %f,
						clientY: %f,
						button: 2,
						buttons: 2,
					});
					el.dispatchEvent(event);
				})()
			`, x, y), nil).Do(ctx)
			if err != nil {
				return err
			}
			return nil
		}),

		chromedp.WaitVisible(`#cso-long-press-submenu`),
		chromedp.Click(`//div[@id="submenu-content"]//button[text()="Dropped"]`),
		chromedp.WaitVisible(`#cso-bip-view`),
		chromedp.Text(`#sequence-display`, &scoreText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if scoreText != "_" {
				return fmt.Errorf("Dropped 3rd Strike initial sequence should be empty, got: %s", scoreText)
			}
			return nil
		}),
		chromedp.Text(`#btn-type`, &buttonText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if buttonText != "D3" {
				return fmt.Errorf("Expected D3, got %s", buttonText)
			}
			return nil
		}),
		cycleTo(t, "#btn-type", "FC"),
		cycleTo(t, "#btn-type", "D3"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("Dropped 3rd Strike btn-type cycle verified")
			return nil
		}),
		chromedp.Click(`#btn-save-bip`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep(t, ctx, "Forced Advance (Walk) - Needs a runner on base",
		chromedp.Click(`#scoresheet-grid > div:nth-child(14)`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-ball`),
		chromedp.Click(`#btn-ball`),
		chromedp.Click(`#btn-ball`),
		chromedp.Click(`#btn-ball`),
		waitUntilDisplayNone(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("Forced advance (Walk) - test requires checking grid for runner advancement logic.")
			return nil
		}),
	)

	runStep(t, ctx, "Inning End (3 Outs) - Using Inning 5 (Col 5) to ensure clean state",
		chromedp.Click(`#scoresheet-grid > div:nth-child(16)`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-show-bip`),
		chromedp.Click(`#btn-cancel-bip`),
		chromedp.Click(`#btn-strike`), chromedp.Click(`#btn-strike`), chromedp.Click(`#btn-strike`),
		waitUntilDisplayNone(`#cso-modal`),

		chromedp.Click(`#scoresheet-grid > div:nth-child(26)`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-strike`), chromedp.Click(`#btn-strike`), chromedp.Click(`#btn-strike`),
		waitUntilDisplayNone(`#cso-modal`),

		chromedp.Click(`#scoresheet-grid > div:nth-child(36)`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-strike`), chromedp.Click(`#btn-strike`), chromedp.Click(`#btn-strike`),
		waitUntilDisplayNone(`#cso-modal`),
	)
}

func TestRunnerActions(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

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
			}
		}
	})

	var runnerListHTML string

	runStep(t, ctx, "Init Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "Runners", "Fielders")
			return err
		}),
	)

	runStep(t, ctx, "Batter 1: Single",
		chromedp.Click(`#scoresheet-grid > div:nth-child(12)`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),
		chromedp.Click(`#btn-save-bip`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep(t, ctx, "Batter 2: Strikeout",
		chromedp.Click(`#scoresheet-grid > div:nth-child(22)`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-strike`),
		chromedp.Click(`#btn-strike`),
		chromedp.Click(`#btn-strike`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep(t, ctx, "Batter 3: Check Runner Actions",
		chromedp.Click(`#scoresheet-grid > div:nth-child(32)`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.WaitVisible(`.ghost-runner[data-base-idx="0"]`),

		chromedp.WaitVisible(`#btn-runner-actions`),
		chromedp.Click(`#btn-runner-actions`),
		chromedp.WaitVisible(`#cso-runner-action-view`),

		chromedp.OuterHTML(`#runner-action-list`, &runnerListHTML),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(runnerListHTML, "Stay") {
				return fmt.Errorf("Runner Action List missing 'Stay' button")
			}
			return nil
		}),
	)

	runStep(t, ctx, "Execute Steal",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`document.querySelector('#runner-action-list button').click()`, nil).Do(ctx)
		}),
		chromedp.Click(`#btn-save-runner-actions`),
		waitUntilDisplayNone(`#cso-runner-action-view`),

		chromedp.WaitVisible(`.ghost-runner[data-base-idx="1"]`),
		chromedp.WaitNotPresent(`.ghost-runner[data-base-idx="0"]`),
	)
}

func TestDBPersistence(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
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
			}
		}
	})

	var gameID string
	var loadedGameJSON string
	var directDBJSON string

	runStep(t, ctx, "Init Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			id, err := CreateGame(ctx, "PersistAway", "PersistHome")
			gameID = id
			return err
		}),
	)

	runStep(t, ctx, "Get Game ID from hash",
		chromedp.Evaluate(`window.location.hash.substring(6)`, &gameID),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Logf("Created Game ID: %s", gameID)
			return nil
		}),
	)

	runStep(t, ctx, "Read back via App DB Class",
		chromedp.ActionFunc(func(ctx context.Context) error {
			cmd := fmt.Sprintf(`
				(async () => {
					window._dbTestRes1 = null;
					const g = await app.db.loadGame('%s');
					window._dbTestRes1 = JSON.stringify(g);
				})()
			`, gameID)
			if err := chromedp.Evaluate(cmd, nil).Do(ctx); err != nil {
				return err
			}
			return chromedp.Poll(`window._dbTestRes1`, &loadedGameJSON).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(loadedGameJSON, "PersistAway") {
				return fmt.Errorf("DBManager load failed: %s", loadedGameJSON)
			}
			t.Log("DBManager load verified")
			return nil
		}),
	)

	runStep(t, ctx, "Read back DIRECTLY via IndexedDB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			cmd := fmt.Sprintf(`
				(async () => {
					window._dbTestRes2 = null;
					const req = indexedDB.open('SkorekeeperDB', 3);
					req.onsuccess = (e) => {
						const db = e.target.result;
						const tx = db.transaction(['games'], 'readonly');
						const store = tx.objectStore('games');
						const getReq = store.get('%s');
						getReq.onsuccess = (ev) => {
							window._dbTestRes2 = JSON.stringify(ev.target.result);
						};
						getReq.onerror = (err) => { window._dbTestRes2 = "ERROR"; };
					};
					req.onerror = (err) => { window._dbTestRes2 = "ERROR"; };
				})()
			`, gameID)
			if err := chromedp.Evaluate(cmd, nil).Do(ctx); err != nil {
				return err
			}
			return chromedp.Poll(`window._dbTestRes2`, &directDBJSON).Do(ctx)
		}),
	)
}

func TestUndoRedoBarrier(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Capture JS errors
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
			}
		case *runtime.EventExceptionThrown:
			t.Logf("JS EXCEPTION: %s", ev.ExceptionDetails.Text)
			t.Fail()
		}
	})

	var bCount, sCount int
	var redoOpacity string

	runStep(t, ctx, "Init Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "BarrierTest", "FencePost")
			return err
		}),
	)

	runStep(t, ctx, "Action A: Ball",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-ball`),
		CSOBallCount(&bCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if bCount != 1 {
				return fmt.Errorf("Expected 1 Ball, got %d", bCount)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Action B: Strike",
		chromedp.Click(`#btn-strike`),
		CSOStrikeCount(&sCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if sCount != 1 {
				return fmt.Errorf("Expected 1 Strike, got %d", sCount)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Undo B",
		chromedp.Click(`#btn-undo-pitch`), // Uses in-CSO undo which calls global undo
		CSOStrikeCount(&sCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if sCount != 0 {
				return fmt.Errorf("Expected 0 Strikes after undo, got %d", sCount)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Action C: Ball (Barrier)",
		chromedp.Click(`#btn-ball`),
		CSOBallCount(&bCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if bCount != 2 {
				return fmt.Errorf("Expected 2 Balls, got %d", bCount)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Verify Redo Disabled (Linear History enforced)",
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),

		chromedp.Evaluate(`document.getElementById('btn-redo').style.opacity`, &redoOpacity),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if redoOpacity == "1" {
				return fmt.Errorf("Redo should be disabled (opacity != 1) after barrier action, got %s", redoOpacity)
			}
			return nil
		}),
	)
}

func TestDashboardRemoteFetch(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
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
			}
		}
	})

	var gameID string
	var cardCount int

	runStep(t, ctx, "Create and Sync Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			id, err := CreateGame(ctx, "RemoteFetchAway", "RemoteFetchHome")
			gameID = id
			// Wait for sync (connection + ACK)
			if err == nil {
				err = WaitForSync(ctx)
			}
			return err
		}),
	)

	runStep(t, ctx, "Clear Local Storage & Reload",
		chromedp.Evaluate(`
			(async () => {
				// Close active connection
				if (window.app && window.app.db && window.app.db.db) {
					window.app.db.db.close();
				}
				// Delete DB
				const req = indexedDB.deleteDatabase('SkorekeeperDB');
				req.onsuccess = () => resolve();
				req.onerror = () => reject(req.error);
				req.onblocked = () => reject('blocked');
			})()
		`, nil),
		chromedp.Navigate(baseURL),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		// Cookie should persist, so just wait for auth UI
		chromedp.Click("#btn-menu-dashboard", chromedp.ByID),
		chromedp.WaitVisible(`#sidebar-auth span.font-mono`, chromedp.ByQuery),
	)

	runStep(t, ctx, "Verify Game Appears with Metadata (Fetched from Remote)",
		chromedp.WaitVisible(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
		chromedp.Evaluate(`document.querySelectorAll('#game-list > div[data-game-id]').length`, &cardCount),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if cardCount == 0 {
				return fmt.Errorf("Expected at least 1 game card, got 0")
			}
			return nil
		}),
		// Check that metadata (Team Name) is present
		chromedp.ActionFunc(func(ctx context.Context) error {
			var cardText string
			err := chromedp.Text(fmt.Sprintf(`div[data-game-id="%s"]`, gameID), &cardText).Do(ctx)
			if err != nil {
				return err
			}
			if !strings.Contains(cardText, "RemoteFetchAway") {
				return fmt.Errorf("Game card missing metadata (Team Name 'RemoteFetchAway'). Got: %s", cardText)
			}
			t.Logf("Found game %s with correct metadata", gameID)
			return nil
		}),
	)
}

func TestDashboardSyncStatus(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
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
			}
		}
	})

	var iconText string

	runStep(t, ctx, "Inject Local-Only Game",
		network.ClearBrowserCookies(),
		chromedp.Navigate(baseURL),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		chromedp.Evaluate(`(async () => {
			const game = {
				id: '88888888-8888-4888-8888-888888888888',
				away: 'Local',
				home: 'Only',
				ownerId: 'test@example.com',
				date: new Date().toISOString(),
				actionLog: [{
					type: 'GAME_START',
					id: 'aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa',
					payload: { id: '88888888-8888-4888-8888-888888888888', date: new Date().toISOString(), away: 'Local', home: 'Only', initialRosterIds: { away: [], home: [] } }
				}]
			};
			await window.app.db.saveGame(game);
		})()`, nil),
	)

	runStep(t, ctx, "Login and Check Dashboard",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, baseURL)
		}),
		chromedp.WaitVisible(`div[data-game-id="88888888-8888-4888-8888-888888888888"]`),
		// Check for "Upload" icon (Cloud Up arrow)
		// The icon text is ☁️⬆️
		chromedp.Text(`#sync-btn-88888888-8888-4888-8888-888888888888`, &iconText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(iconText, "☁️⬆️") {
				return fmt.Errorf("Expected upload icon, got %s", iconText)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Trigger Upload",
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('sync-btn-88888888-8888-4888-8888-888888888888').click()`, nil),
		// Wait for icon to change to Checkmark
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Poll(`
				(() => {
					const el = document.querySelector('#sync-btn-88888888-8888-4888-8888-888888888888');
					return el && el.innerText.includes('✅') ? el.innerText : false;
				})()
			`, &iconText,
				chromedp.WithPollingInterval(500*time.Millisecond),
				chromedp.WithPollingTimeout(10*time.Second),
			).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(iconText, "✅") {
				return fmt.Errorf("Expected synced icon after click, got %s", iconText)
			}
			return nil
		}),
	)
}
