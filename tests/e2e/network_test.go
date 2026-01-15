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
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestBurstActionsOffline(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, *withChromeDP)
	defer cancel()

	opts := []chromedp.ContextOption{
		chromedp.WithLogf(log.Printf),
		chromedp.WithErrorf(log.Printf),
	}

	// Client A
	ctxA, cancelA := chromedp.NewContext(allocCtx, opts...)
	defer cancelA()

	// Client B (Observer)
	ctxB, cancelB := chromedp.NewContext(allocCtx, opts...)
	defer cancelB()

	// Listen for errors
	listenErrors := func(ctx context.Context, name string) {
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *runtime.EventConsoleAPICalled:
				if ev.Type == runtime.APITypeError || ev.Type == runtime.APITypeWarning {
					args := make([]string, len(ev.Args))
					for i, arg := range ev.Args {
						args[i] = string(arg.Value)
					}
					// Only log real errors, not anticipated warnings
					if !strings.Contains(args[0], "WS Pong timeout") {
						// t.Logf("[%s] JS CONSOLE: %s", name, strings.Join(args, " "))
					}
				}
			}
		})
	}
	listenErrors(ctxA, "Client A")
	listenErrors(ctxB, "Client B")

	var gameID string

	// 1. Setup: A creates game
	runStep(t, ctxA, "Client A: Login & Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			id, err := LoginAndCreateGame(ctx, baseURL, "BurstAway", "BurstHome")
			gameID = id
			return err
		}),
	)
	t.Logf("Game ID: %s", gameID)

	// 2. Setup: B joins
	runStep(t, ctxB, "Client B: Join Game",
		chromedp.Navigate(fmt.Sprintf(`%s/#game/%s`, baseURL, gameID)),
		chromedp.WaitVisible(`#scoresheet-view`),
		chromedp.WaitVisible(`#sync-status-container svg.text-green-400`), // Wait for sync
	)

	// 3. Client A: Go Offline
	t.Log("Simulating Offline on Client A...")
	runStep(t, ctxA, "Client A: Go Offline",
		network.Enable(),
		network.EmulateNetworkConditions(true, 0, 0, 0), // Offline
	)

	// Wait for status to update (might take a moment for ping timeout or immediate error on send?)
	// Actually, sending fails immediately? No, we just queue.
	// We won't see "Offline" status until onclose/onerror.
	// But we are testing BURST actions before close.

	// 4. Client A: Record 3 actions (Ball, Strike, Ball)
	runStep(t, ctxA, "Client A: Record Burst Actions",
		chromedp.Click(`.grid-cell`), // Open CSO
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-ball`),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Click(`#btn-strike`),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Click(`#btn-ball`),
		// Verify local optimistic update
		chromedp.WaitVisible(`#cso-modal .count-dots:first-child .filled-black:nth-child(2)`),
		chromedp.WaitVisible(`#cso-modal .count-dots:last-child .filled-black:nth-child(1)`),
	)

	// 5. Client A: Go Online
	t.Log("Simulating Online on Client A...")
	runStep(t, ctxA, "Client A: Go Online",
		network.EmulateNetworkConditions(false, 0, 0, 0), // Online
	)

	// 6. Verification
	// Client A should eventually see "Synced" (Green Check)
	// Client B should see the same state (2 Balls, 1 Strike)

	runStep(t, ctxA, "Client A: Verify Re-Sync",
		chromedp.WaitVisible(`#sync-status-container svg.text-green-400`, chromedp.ByQuery),
	)

	runStep(t, ctxB, "Client B: Verify Sync Received",
		// B needs to open CSO to see details, or we check the grid cell.
		// Let's check the grid cell content on B.
		// The cell text should reflect "2-1"? The grid cell shows dots.
		// Let's open CSO on B.
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		// Wait for update
		chromedp.WaitVisible(`#cso-modal .count-dots:first-child .filled-black:nth-child(2)`),
		chromedp.WaitVisible(`#cso-modal .count-dots:last-child .filled-black:nth-child(1)`),
	)
}

func TestReconnectionLogic(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, *withChromeDP)
	defer cancel()

	opts := []chromedp.ContextOption{
		chromedp.WithLogf(log.Printf),
		chromedp.WithErrorf(log.Printf),
	}

	ctxA, cancelA := chromedp.NewContext(allocCtx, opts...)
	defer cancelA()

	runStep(t, ctxA, "Client A: Login & Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := LoginAndCreateGame(ctx, baseURL, "ReconnectAway", "ReconnectHome")
			// Wait for sync status as required by this test
			if err == nil {
				err = chromedp.WaitVisible(`#sync-status-container svg.text-green-400`).Do(ctx)
			}
			return err
		}),
	)

	// Simulate Disconnect
	t.Log("Severing connection...")
	// We can simulate this by manually closing the socket from JS?
	// Or network offline.
	runStep(t, ctxA, "Client A: Sever Connection",
		chromedp.Evaluate(`window.app.sync.socket.close()`, nil),
	)

	// Check that it detects and tries to reconnect.
	// Status should change to "Offline" or "Connecting" or "Error" briefly, then eventually back to Green.
	// The reconnect timer starts at 1s.

	t.Log("Waiting for auto-reconnect...")
	runStep(t, ctxA, "Client A: Verify Auto-Reconnect",
		// It might flash error/offline.
		// We just want to see it eventually go Green again.
		chromedp.WaitVisible(`#sync-status-container svg.text-green-400`),
	)
}
