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

func TestNarrativeFeed(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), *withChromeDP)
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
		case *runtime.EventExceptionThrown:
			t.Logf("JS EXCEPTION: %s", ev.ExceptionDetails.Text)
		}
	})

	if err := e2ehelpers.Login(ctx, baseURL); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if err := chromedp.Run(ctx, e2ehelpers.DisableCSSAnimations()); err != nil {
		t.Fatalf("Failed to disable CSS animations: %v", err)
	}

	// Create a new game
	gameID, err := e2ehelpers.CreateGame(ctx, "Away Team", "Home Team")
	if err != nil {
		t.Fatalf("Failed to create game: %v", err)
	}
	t.Logf("Created game %s", gameID)

	// Record some pitches
	if err := e2ehelpers.SelectCell(ctx, 1, 1); err != nil {
		t.Fatalf("Failed to select cell: %v", err)
	}

	if err := e2ehelpers.RecordPitch(ctx, "ball"); err != nil {
		t.Fatalf("Failed to record pitch: %v", err)
	}
	if err := e2ehelpers.RecordPitch(ctx, "strike"); err != nil {
		t.Fatalf("Failed to record pitch: %v", err)
	}

	// Record a BIP (Single)
	if err := e2ehelpers.RecordBallInPlay(ctx, "Safe", "HIT", "8"); err != nil {
		t.Fatalf("Failed to record BIP: %v", err)
	}

	// Steal 2nd (Runner action during next batter's turn)
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		t.Fatalf("Failed to finish turn after BIP: %v", err)
	}

	// Batter 2 is up. Runner on 1st. Record Steal.
	if err := e2ehelpers.SelectCell(ctx, 2, 1); err != nil {
		t.Fatalf("Failed to select batter 2 cell: %v", err)
	}

	if err := chromedp.Run(ctx,
		chromedp.WaitVisible("#cso-modal"),
		chromedp.WaitVisible("#btn-runner-actions"),
		chromedp.Click("#btn-runner-actions"),
		chromedp.WaitVisible("#cso-runner-action-view"),
		// Click the first action button (Steal 2nd for runner on 1st)
		chromedp.Click("#runner-action-list button"),
		chromedp.Click("#btn-save-runner-actions"),
		waitUntilDisplayNone("#cso-runner-action-view"),
	); err != nil {
		t.Fatalf("Failed to record steal: %v", err)
	}

	// Record manual advance on error for Player 1 (now on 2nd) to 3rd
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible("#cso-main-view"),
		chromedp.Sleep(200*time.Millisecond),
		// 1. Click on 3rd base (target base) to advance them from 2nd
		chromedp.Evaluate(`document.getElementById("zoom-base-3b").dispatchEvent(new MouseEvent("click", {bubbles: true}))`, nil),
		// 2. Click 'Err' in the runner menu
		// Use Evaluate with setTimeout to trigger the click without blocking (since it opens a prompt)
		chromedp.WaitReady("#runner-menu-options"),
		chromedp.Evaluate(`setTimeout(() => {
		            const btn = [...document.querySelectorAll("#runner-menu-options button")].find(b => b.textContent === "Err");
		            if (btn) btn.click();
		        }, 100)`, nil),
		// 3. Handle the error attribution prompt (Fielder Pos 6)
		chromedp.WaitVisible("input[data-test='custom-prompt-input']"),
		chromedp.SendKeys("input[data-test='custom-prompt-input']", "6"),
		chromedp.Click("#btn-prompt-ok"),
		waitUntilDisplayNone("#custom-prompt-modal"),
		// 4. Save the event data (actually we just close CSO as it's already dispatched)
		chromedp.Click("#btn-close-cso"),
		waitUntilDisplayNone("#cso-modal"),
	); err != nil {
		t.Fatalf("Failed to record manual error: %v", err)
	}

	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		t.Fatalf("Failed to finish turn: %v", err)
	}

	// Switch to Feed view via sidebar
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-menu-scoresheet"),
		chromedp.WaitVisible("#sidebar-btn-view-feed"),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click("#sidebar-btn-view-feed"),
		chromedp.WaitVisible("#feed-container"),
	); err != nil {
		t.Fatalf("Failed to switch to feed view: %v", err)
	}

	// Verify narrative text
	var narrativeText string
	if err := chromedp.Run(ctx,
		chromedp.Text("#feed-container", &narrativeText),
	); err != nil {
		t.Fatalf("Failed to get narrative text: %v", err)
	}

	t.Logf("Narrative Text: %q", narrativeText)
	// Flexible checks for dynamic flavor text
	if !strings.Contains(narrativeText, "Player 1") || !strings.Contains(narrativeText, "Center Field") ||
		(!strings.Contains(narrativeText, "single") && !strings.Contains(narrativeText, "hit")) {
		t.Errorf("Narrative missing play description. Got: %q", narrativeText)
	}
	// New format uses bullets and separate lines
	if !strings.Contains(narrativeText, "Ball") || !strings.Contains(narrativeText, "Strike") {
		t.Errorf("Narrative missing pitch sequence. Got: %q", narrativeText)
	}
	if !strings.Contains(narrativeText, "🏃 Player 1 steals 2nd!") {
		t.Errorf("Narrative missing runner action (steal). Got: %q", narrativeText)
	}
	if !strings.Contains(narrativeText, "⚠️ Player 1 advances to 3rd on an error by Shortstop.") {
		t.Errorf("Narrative missing manual error description. Got: %q", narrativeText)
	}

	// Verify team stats show 1 error in the scoreboard
	var homeErrors string
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-menu-scoresheet"),
		chromedp.WaitVisible("#sidebar-btn-view-grid"),
		chromedp.Click("#sidebar-btn-view-grid"),
		chromedp.WaitVisible("#scoresheet-grid"),
		chromedp.Text("#sb-e-home", &homeErrors),
	); err != nil {
		t.Fatalf("Failed to read scoreboard errors: %v", err)
	}
	t.Logf("Home Team Errors: %q", homeErrors)
	if homeErrors != "1" {
		t.Errorf("Expected 1 error for Home team in scoreboard, got %q", homeErrors)
	}
}
