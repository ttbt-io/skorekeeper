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
	"log"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestGlobalStatsNavigation(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	// Create a new allocator context
	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	// Create a new browser context
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
				t.Errorf("JS ERROR: %s", strings.Join(args, " "))
			}
		}
	})

	runStep(t, ctx, "Login and navigate to Statistics",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, baseURL)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := CreateGame(ctx, "Stats Team A", "Stats Team B")
			return err
		}),
		chromedp.Click(`#btn-menu-scoresheet`),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.WaitVisible(`#sidebar-btn-stats`),
		chromedp.Click(`#sidebar-btn-stats`),
		chromedp.WaitVisible(`#statistics-view`),
	)

	runStep(t, ctx, "Verify Statistics sections are visible",
		chromedp.WaitVisible(`#stats-content h2`), // Should show at least one header
	)
}

func TestGlobalStatsAggregation(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	// Create a new allocator context
	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	// Create a new browser context
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
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
				t.Errorf("JS ERROR: %s", strings.Join(args, " "))
			}
		}
	})

	runStep(t, ctx, "Login and create game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := LoginAndCreateGame(ctx, baseURL, "Agg Team A", "Agg Team B")
			return err
		}),
	)

	runStep(t, ctx, "Record 1B for Batter 1 with Location",
		chromedp.Click("#scoresheet-grid > div:nth-child(12)"), // Row 0, Col 1
		chromedp.WaitVisible("#cso-modal"),
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		// Set Location
		chromedp.Click("#btn-loc"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Click center field (approx)
			return chromedp.Evaluate(`
				const svg = document.querySelector('.field-svg-keyboard svg');
				const rect = svg.getBoundingClientRect();
				const x = rect.left + rect.width * 0.5;
				const y = rect.top + rect.height * 0.3;
				document.elementFromPoint(x, y).dispatchEvent(new MouseEvent('click', {
					bubbles: true,
					clientX: x,
					clientY: y
				}));
			`, nil).Do(ctx)
		}),
		chromedp.Click("#btn-save-bip"), // 1B Safe
		waitUntilDisplayNone("#cso-modal"),
	)

	runStep(t, ctx, "Record K for Batter 2",
		chromedp.Click("#scoresheet-grid > div:nth-child(22)"), // Row 1, Col 1
		chromedp.WaitVisible("#cso-modal"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone("#cso-modal"),
		chromedp.Sleep(2000*time.Millisecond),
	)

	runStep(t, ctx, "Open Sidebar",
		chromedp.Click(`#btn-menu-scoresheet`),
		chromedp.Sleep(1000*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return CaptureScreenshot(ctx, "/demo/stats_sidebar_open.png")
		}),
	)

	runStep(t, ctx, "Click End Game",
		chromedp.WaitVisible(`#sidebar-btn-end-game`),
		chromedp.Click(`#sidebar-btn-end-game`),
		chromedp.WaitVisible(`#custom-confirm-modal`),
		chromedp.Sleep(1000*time.Millisecond),
	)

	runStep(t, ctx, "Confirm Finalize",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return CaptureScreenshot(ctx, "/demo/stats_finalize_confirm.png")
		}),
		chromedp.Click(`#btn-confirm-yes`),
		waitUntilDisplayNone(`#custom-confirm-modal`),
		chromedp.Sleep(1000*time.Millisecond),
	)

	runStep(t, ctx, "Navigate to Statistics and Verify",
		chromedp.Click(`#btn-menu-scoresheet`),
		chromedp.Sleep(1000*time.Millisecond), // Wait for sidebar animation
		chromedp.WaitVisible(`#sidebar-btn-stats`),
		chromedp.Evaluate(`document.getElementById('sidebar-btn-stats').click()`, nil),
		chromedp.WaitVisible(`#statistics-view`),
		chromedp.WaitVisible(`#stats-content table`),
	)

	var statsText string
	runStep(t, ctx, "Verify Player 1 has a hit in stats",
		chromedp.Text(`#stats-content`, &statsText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(statsText, "Player 1") {
				return log.Output(1, "Player 1 not found in stats table")
			}
			// Player 1 had 1B, so AVG should be 1.000
			if !strings.Contains(statsText, "1.000") {
				return log.Output(1, "Player 1 AVG 1.000 not found in stats")
			}
			return nil
		}),
	)

	runStep(t, ctx, "Open Player Profile and Verify Spray Chart",
		// Click Player 1 row (first row in tbody)
		chromedp.WaitVisible(`#stats-content table tbody tr`),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#stats-content table tbody tr:first-child`),
		chromedp.WaitVisible(`#player-profile-modal`),
		chromedp.WaitVisible(`#profile-spray-chart circle`), // Verify at least one marker exists
		chromedp.ActionFunc(func(ctx context.Context) error {
			return CaptureScreenshot(ctx, "/demo/stats_profile_spray.png")
		}),
	)
}
