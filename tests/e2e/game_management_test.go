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

func TestGameMetadataEditing(t *testing.T) {
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
		}
	})

	var gameID string
	var cardText string

	runStep(t, ctx, "Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			id, err := CreateGame(ctx, "EditAway", "EditHome")
			gameID = id
			return err
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Return to Dashboard",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Click("#sidebar-btn-dashboard"),
		chromedp.WaitVisible("#dashboard-view"),
	)

	runStep(t, ctx, "Open Edit Modal via Context Menu",
		chromedp.WaitVisible(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
		RightClick(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
		chromedp.WaitVisible("#game-context-menu"),
		chromedp.Click("#btn-menu-edit-game"),
		chromedp.WaitVisible("#edit-game-modal"),
	)

	runStep(t, ctx, "Edit Game Details",
		chromedp.SetValue("#edit-game-event", "Championship Final"),
		chromedp.SetValue("#edit-game-location", "Central Park"),
		chromedp.SetValue("#edit-team-away", "Updated Away"),
		chromedp.SetValue("#edit-team-home", "Updated Home"),
		chromedp.Click("#btn-save-edit-game"),
		waitUntilDisplayNone("#edit-game-modal"),
	)

	runStep(t, ctx, "Verify Changes on Dashboard",
		chromedp.Text(fmt.Sprintf(`div[data-game-id="%s"]`, gameID), &cardText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(cardText, "Updated Away") {
				return fmt.Errorf("Expected 'Updated Away', got %q", cardText)
			}
			if !strings.Contains(cardText, "Updated Home") {
				return fmt.Errorf("Expected 'Updated Home', got %q", cardText)
			}
			if !strings.Contains(cardText, "Championship Final") {
				return fmt.Errorf("Expected 'Championship Final', got %q", cardText)
			}
			if !strings.Contains(cardText, "Central Park") {
				return fmt.Errorf("Expected 'Central Park', got %q", cardText)
			}
			return nil
		}),
	)
}

func TestDeleteGame(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var gameID string

	runStep(t, ctx, "Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			id, err := CreateGame(ctx, "DeleteMe", "Stays")
			gameID = id
			return err
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Return to Dashboard",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Click("#sidebar-btn-dashboard"),
		chromedp.WaitVisible("#dashboard-view"),
	)

	runStep(t, ctx, "Delete Game via Context Menu",
		chromedp.WaitVisible(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
		RightClick(fmt.Sprintf(`div[data-game-id="%s"]`, gameID)),
		chromedp.WaitVisible("#game-context-menu"),
		chromedp.Click("#btn-menu-delete-game"),
		chromedp.WaitVisible("#custom-confirm-modal"),
		chromedp.Click("#btn-confirm-yes"),
		waitUntilDisplayNone("#custom-confirm-modal"),
	)

	runStep(t, ctx, "Verify Game is Removed",
		chromedp.WaitVisible("#dashboard-view"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var exists bool
			err := chromedp.Evaluate(fmt.Sprintf(`!!document.querySelector('div[data-game-id="%s"]')`, gameID), &exists).Do(ctx)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("Game %s still exists on dashboard after deletion", gameID)
			}
			return nil
		}),
	)
}
