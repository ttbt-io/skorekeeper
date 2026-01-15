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

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestDeleteSync(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)
	// Viewer uses devtest hostname to avoid cookie sharing with devtest.local (User A)
	viewerURL := strings.Replace(baseURL, "devtest.local", "devtest", 1)

	// Context A: Device A
	ctxA, cancelA := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancelA()
	ctxA, cancelA = chromedp.NewContext(ctxA, chromedp.WithLogf(log.Printf))
	defer cancelA()
	ctxA, cancelA = context.WithTimeout(ctxA, 60*time.Second)
	defer cancelA()

	// Context B: Device B
	ctxB, cancelB := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancelB()
	ctxB, cancelB = chromedp.NewContext(ctxB, chromedp.WithLogf(log.Printf))
	defer cancelB()
	ctxB, cancelB = context.WithTimeout(ctxB, 60*time.Second)
	defer cancelB()

	chromedp.ListenTarget(ctxA, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			t.Logf("DEVICE A: %s", ev.Args[0].Value)
		}
	})
	chromedp.ListenTarget(ctxB, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			t.Logf("DEVICE B: %s", ev.Args[0].Value)
		}
	})

	var gameID string

	// 1. Device A creates a game
	runStep(t, ctxA, "Device A: Login and Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			id, err := CreateGame(ctx, "DeleteSyncAway", "DeleteSyncHome")
			gameID = id
			return err
		}),
		DisableCSSAnimations(),
	)

	// 2. Device B logs in and syncs the game
	runStep(t, ctxB, "Device B: Login and Sync",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, viewerURL)
		}),
		DisableCSSAnimations(),
		chromedp.WaitVisible(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
	)

	// 3. Device A deletes the game
	runStep(t, ctxA, "Device A: Delete Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Re-login to ensure session is active (avoid 403 flakes)
			return Login(ctx, baseURL)
		}),
		chromedp.Navigate(baseURL+"#"),
		chromedp.WaitVisible(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
		RightClick(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
		chromedp.WaitVisible("#game-context-menu"),
		chromedp.Click("#btn-menu-delete-game"),
		chromedp.WaitVisible("#custom-confirm-modal"),
		chromedp.Click("#btn-confirm-yes"),
		waitUntilDisplayNone("#custom-confirm-modal"),
		waitUntilDisplayNone(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
	)

	// 4. Device B refreshes/navigates to dashboard and verifies deletion
	// We force a dashboard reload/navigation to trigger the sync logic
	runStep(t, ctxB, "Device B: Verify Deletion Sync",
		chromedp.Navigate(viewerURL+"#"),      // Navigate to root to trigger loadDashboard
		chromedp.Sleep(1000*time.Millisecond), // Wait for sync
		chromedp.ActionFunc(func(ctx context.Context) error {
			var exists bool
			err := chromedp.Evaluate(fmt.Sprintf(`!!document.querySelector('div[data-game-id="%s"]')`, gameID), &exists).Do(ctx)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("Game %s should have been removed from Device B", gameID)
			}
			return nil
		}),
	)
}
