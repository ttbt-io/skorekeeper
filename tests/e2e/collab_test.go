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

func TestPublicSharing(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)
	// Viewer uses devtest hostname to avoid cookie sharing with devtest.local (User A)
	// (Assuming devtest and devtest.local resolve to the same IP but are treated as distinct origins)

	// Context A: User A (Owner)
	ctxA, cancelA := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancelA()
	ctxA, cancelA = chromedp.NewContext(ctxA, chromedp.WithLogf(log.Printf))
	defer cancelA()
	ctxA, cancelA = context.WithTimeout(ctxA, 60*time.Second)
	defer cancelA()

	// Context B: Public Viewer (No Auth)
	ctxB, cancelB := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancelB()
	ctxB, cancelB = chromedp.NewContext(ctxB, chromedp.WithLogf(log.Printf))
	defer cancelB()
	ctxB, cancelB = context.WithTimeout(ctxB, 60*time.Second)
	defer cancelB()

	chromedp.ListenTarget(ctxA, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			t.Logf("USER A: %s", ev.Args[0].Value)
		}
	})
	chromedp.ListenTarget(ctxB, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			t.Logf("VIEWER: %s", ev.Args[0].Value)
		}
	})

	var gameURL string

	runStep(t, ctxA, "User A creates game and enables Public Link",
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := LoginAndCreateGame(ctx, baseURL, "Team Alpha", "Team Beta")
			return err
		}),
		chromedp.Click(`#btn-share-game`),
		chromedp.WaitVisible(`#share-modal`),
		chromedp.Click(`#share-public-toggle`),
		chromedp.WaitVisible(`#public-link-container`), // Confirm toggle worked
		chromedp.Sleep(2000*time.Millisecond),          // Ensure save completes
		chromedp.Value(`#public-share-url`, &gameURL),
		chromedp.Click(`#btn-close-share`),
		waitUntilDisplayNone(`#share-modal`),
	)

	// Fix gameURL domain for Viewer
	gameURL = strings.Replace(gameURL, "devtest.local", "devtest", 1)

	runStep(t, ctxB, "Viewer opens public link",
		chromedp.Navigate(gameURL),
		chromedp.WaitVisible(`#scoresheet-view`),
		chromedp.WaitVisible(`#header-game-title`),
	)

	runStep(t, ctxB, "Viewer verifies Read-Only UI",
		chromedp.ActionFunc(func(ctx context.Context) error {
			var hasClass bool
			// Check if grid is locked
			err := chromedp.Evaluate(`document.getElementById('scoresheet-grid').classList.contains('pointer-events-none')`, &hasClass).Do(ctx)
			if err != nil {
				return err
			}
			if !hasClass {
				return fmt.Errorf("Grid should be locked (pointer-events-none) for public viewer")
			}
			return nil
		}),
		// Try clicking a cell (should be ignored by UI, but we can verify CSO doesn't appear)
		chromedp.Click(`#scoresheet-grid > .grid-cell:nth-child(14)`),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var display string
			err := chromedp.Evaluate(`window.getComputedStyle(document.getElementById('cso-modal')).display`, &display).Do(ctx)
			if err != nil {
				return err
			}
			if display != "none" {
				return fmt.Errorf("CSO modal appeared despite read-only mode")
			}
			return nil
		}),
	)

	runStep(t, ctxA, "User A records a pitch",
		// Open CSO to set active context
		chromedp.Click(`#scoresheet-grid .grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		// Record pitch
		chromedp.Click(`#btn-ball`),
		chromedp.Sleep(500*time.Millisecond),
	)

	runStep(t, ctxB, "Viewer verifies real-time update",
		chromedp.WaitVisible(`#scoresheet-grid .count-display .count-dots:first-child .dot.filled-black`),
	)
}
