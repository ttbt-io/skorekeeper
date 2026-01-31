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
	"github.com/ttbt-io/skorekeeper/tools/e2ehelpers"
)

func TestBalk(t *testing.T) {
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
		}
	})

	runStep := func(description string, actions ...chromedp.Action) {
		t.Helper()
		runStep(t, ctx, description, actions...)
	}

	cycleTo := func(t *testing.T, sel, targetText string) chromedp.Action {
		return e2ehelpers.CycleTo(t, sel, targetText)
	}

	t.Log("Starting Balk Test...")

	// 1. Setup Game
	runStep("Setup Game",
		e2ehelpers.DisableCSSAnimations(),
		chromedp.Navigate(baseURL),
		chromedp.WaitVisible(`#btn-new-game`, chromedp.ByID),
		chromedp.Click(`#btn-new-game`, chromedp.ByID),
		chromedp.WaitVisible(`#new-game-modal`, chromedp.ByID),
		chromedp.Click(`#btn-start-new-game`, chromedp.ByID),
		chromedp.WaitVisible(`#scoresheet-view`, chromedp.ByID),
	)

	// 2. Batter 1: Single to get on base
	runStep("Batter 1: Single",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SelectCell(ctx, 1, 1)
		}),
		chromedp.Click(`#btn-show-bip`, chromedp.ByID),
		chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByID),
		cycleTo(t, "#btn-res", "Safe"),
		cycleTo(t, "#btn-base", "1B"),
		cycleTo(t, "#btn-type", "HIT"),
		chromedp.Click(`#btn-save-bip`, chromedp.ByID),
		chromedp.WaitNotVisible(`#cso-bip-view`, chromedp.ByID),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.FinishTurn(ctx)
		}),
	)

	// 3. Batter 2: Balk
	runStep("Check BK Button Visibility",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SelectCell(ctx, 2, 1)
		}),
		chromedp.WaitVisible(`#btn-balk`, chromedp.ByID), // Should be visible
	)

	// 4. Record Balk
	runStep("Record Balk",
		chromedp.Click(`#btn-balk`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond), // Wait for render
	)

	// 5. Verify Runner on 2nd
	runStep("Verify Runner on 2nd",
		chromedp.WaitVisible(`.ghost-runner[data-base-idx="1"]`, chromedp.ByQuery),
	)

	// 6. Close CSO
	runStep("Close CSO",
		chromedp.Click(`#btn-close-cso`, chromedp.ByID),
		chromedp.WaitNotVisible(`#cso-modal`, chromedp.ByID),
	)

	// 7. Verify Narrative Feed
	runStep("Verify Narrative Feed",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.OpenSidebar(ctx)
		}),
		chromedp.WaitVisible(`#sidebar-btn-view-feed`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#sidebar-btn-view-feed`, chromedp.ByID),
		chromedp.WaitVisible(`#narrative-feed`, chromedp.ByID),
		chromedp.WaitVisible(`//div[contains(text(), "advances on a balk")]`, chromedp.BySearch),
	)

	// 8. Return to Grid
	runStep("Return to Grid",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.OpenSidebar(ctx)
		}),
		chromedp.WaitVisible(`#sidebar-btn-view-grid`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#sidebar-btn-view-grid`, chromedp.ByID),
		chromedp.WaitVisible(`#scoresheet-grid`, chromedp.ByID),
	)
}
