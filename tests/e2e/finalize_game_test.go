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

func TestFinalizeGame(t *testing.T) {
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

	// Capture JS console errors
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

	var gameID string
	var cardText string

	runStep(t, ctx, "Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			id, err := CreateGame(ctx, "FinalAway", "FinalHome")
			gameID = id
			return err
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Score Some Plays",
		// Already on scoresheet after CreateGame
		chromedp.WaitVisible("#scoresheet-view"),

		// Record an out
		chromedp.Click(`.grid-cell[data-player-idx="0"]`),
		chromedp.WaitVisible("#cso-modal"),
		chromedp.Click("#btn-out"),
		waitUntilDisplayNone("#cso-modal"),
	)

	runStep(t, ctx, "Finalize Game",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.WaitVisible("#sidebar-btn-end-game"),
		chromedp.Click("#sidebar-btn-end-game"),
		// Confirm modal
		chromedp.WaitVisible("#custom-confirm-modal"),
		chromedp.Click("#btn-confirm-yes"),
		waitUntilDisplayNone("#custom-confirm-modal"),
		// Wait for render
		chromedp.WaitVisible("#game-status-indicator"), // FINAL indicator
	)

	runStep(t, ctx, "Verify Read-Only",
		// Try to open CSO (should fail/do nothing)
		// We can't easily assert "nothing happened" without waiting a timeout,
		// but we can assert the grid has pointer-events: none style?
		chromedp.AttributeValue("#scoresheet-grid", "class", &cardText, nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(cardText, "pointer-events-none") {
				return fmt.Errorf("Expected grid to have CLASS pointer-events-none, got %s", cardText)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Verify Dashboard Status",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Click("#sidebar-btn-dashboard"),
		chromedp.WaitVisible("#dashboard-view"),
		// Check that it's in Finalized section?
		// We can check if the card has opacity class
		chromedp.AttributeValue(fmt.Sprintf(`div[data-game-id="%s"]`, gameID), "class", &cardText, nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(cardText, "opacity-80") { // Updated opacity class
				return fmt.Errorf("Expected card to have opacity-80, got %s", cardText)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Undo Finalize",
		// Go back to game
		chromedp.Click(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
		chromedp.WaitVisible("#scoresheet-view"),
		// Click Undo
		chromedp.Click("#btn-undo"),
		// Wait for FINAL indicator to disappear
		waitUntilDisplayNone("#game-status-indicator"),
		// Verify interactive again
		chromedp.AttributeValue("#scoresheet-grid", "class", &cardText, nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if strings.Contains(cardText, "pointer-events-none") {
				return fmt.Errorf("Expected grid NOT to have pointer-events-none class, got %s", cardText)
			}
			return nil
		}),
	)
}
