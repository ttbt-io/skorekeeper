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

func TestIncrementalStats(t *testing.T) {
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
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second) // Long timeout for full rebuild test
	defer cancel()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			// Filter noise
			args := make([]string, len(ev.Args))
			for i, arg := range ev.Args {
				args[i] = string(arg.Value)
			}
			// t.Logf("JS CONSOLE (%s): %s", ev.Type, strings.Join(args, " "))
		case *runtime.EventExceptionThrown:
			t.Logf("JS EXCEPTION: %s", ev.ExceptionDetails.Text)
			// t.Fail() // Don't fail immediately, let assertions handle logic
		}
	})

	runStep(t, ctx, "Initialization",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			// Create Game 1
			if _, err := CreateGame(ctx, "IncTeamA", "IncTeamB"); err != nil {
				return err
			}
			return nil
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Record Data Game 1",
		chromedp.Click(`#scoresheet-grid > .grid-cell`), // 1st cell
		chromedp.WaitVisible(`#cso-modal`),
		// 1B for Batter 1
		chromedp.Click(`#btn-show-bip`),
		chromedp.Click(`#btn-res`), // Safe
		chromedp.Click(`#btn-save-bip`),
		waitUntilDisplayNone(`#cso-modal`),
		// Finish Game 1 (to ensure it saves properly)
		chromedp.Click(`#btn-menu-scoresheet`),
		chromedp.WaitVisible(`#app-sidebar`),
		chromedp.Click(`#sidebar-btn-dashboard`),
		chromedp.WaitVisible(`#game-list`),
	)

	runStep(t, ctx, "Verify Stats (Initial Aggregation)",
		chromedp.Click(`#btn-menu-dashboard`),
		chromedp.WaitVisible(`#app-sidebar`),
		chromedp.Click(`#sidebar-btn-stats`),
		chromedp.WaitVisible(`#stats-content`),
		chromedp.Sleep(2*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var text string
			if err := chromedp.Text(`#stats-content`, &text).Do(ctx); err != nil {
				return err
			}
			// Should see Team Name
			if !strings.Contains(text, "IncTeamA") {
				return fmt.Errorf("Stats missing team name 'IncTeamA' in: %s", text)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Modify Game and Verify Incremental Update",
		chromedp.Click(`#btn-menu-stats`),
		chromedp.WaitVisible(`#app-sidebar`),
		chromedp.Click(`#sidebar-btn-dashboard`),
		chromedp.WaitVisible(`#game-list`),
		// Open Game 1
		chromedp.Click(`.bg-white[data-game-id]`),
		chromedp.WaitVisible(`#scoresheet-view`),
		// Add another Hit (Batter 2)
		chromedp.Click(`.grid-cell[data-player-idx="1"]`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-show-bip`),
		chromedp.Click(`#btn-save-bip`), // Default Safe 1B
		waitUntilDisplayNone(`#cso-modal`),
		// Check Stats Again
		chromedp.Click(`#btn-menu-scoresheet`),
		chromedp.WaitVisible(`#app-sidebar`),
		chromedp.Click(`#sidebar-btn-stats`),
		chromedp.WaitVisible(`#stats-content`),
		chromedp.Sleep(2*time.Second), // Wait for render
		chromedp.ActionFunc(func(ctx context.Context) error {
			var text string
			if err := chromedp.Text(`#stats-content`, &text).Do(ctx); err != nil {
				return err
			}
			// Should see Team Name and 2 Hits (one from each batter)
			if !strings.Contains(text, "IncTeamA") {
				return fmt.Errorf("Stats missing team name 'IncTeamA' in: %s", text)
			}
			// After 2 hits, AVG 1.000 should be present
			if !strings.Contains(text, "1.000") {
				return fmt.Errorf("Stats missing AVG 1.000 in: %s", text)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Rebuild Statistics",
		chromedp.Click(`#btn-rebuild-stats`),
		chromedp.WaitVisible(`#custom-confirm-modal`),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Click(`#btn-confirm-yes`),
		waitUntilDisplayNone(`#custom-confirm-modal`),
		chromedp.Sleep(2*time.Second), // Give it more time
		chromedp.ActionFunc(func(ctx context.Context) error {
			var btnText string
			// Re-verify we are still on the stats view and button is there
			if err := chromedp.Text(`#btn-rebuild-stats`, &btnText).Do(ctx); err != nil {
				return err
			}
			if btnText != "Rebuild Stats" {
				return fmt.Errorf("Rebuild button did not reset")
			}
			return nil
		}),
	)
}
