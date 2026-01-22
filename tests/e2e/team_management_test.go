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

func TestTeamManagement(t *testing.T) {
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

	teamName := "E2E Test Team"
	teamShortName := "E2ET"

	runStep(t, ctx, "Login",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, baseURL)
		}),
		DisableCSSAnimations(),
		chromedp.Sleep(1000*time.Millisecond),
	)

	runStep(t, ctx, "Navigate to Teams",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.WaitVisible("#sidebar-btn-teams"),
		chromedp.Click("#sidebar-btn-teams"),
		chromedp.WaitVisible("#teams-view"),
	)

	runStep(t, ctx, "Create Team",
		chromedp.Click("#btn-create-team"),
		chromedp.WaitVisible("#team-modal"),
		chromedp.SetValue("#team-name", teamName),
		chromedp.SetValue("#team-short-name", teamShortName),
		// Fill first player row
		chromedp.SetValue(`#team-roster-container > div:nth-child(1) input[name="number"]`, "10"),
		chromedp.SetValue(`#team-roster-container > div:nth-child(1) input[name="name"]`, "Player Ten"),
		chromedp.Click("#btn-save-team"),
		waitUntilDisplayNone("#team-modal"),
	)

	runStep(t, ctx, "Verify Team in List",
		chromedp.WaitVisible("#teams-list"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var listText string
			if err := chromedp.Text("#teams-list", &listText).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(listText, teamName) {
				return fmt.Errorf("expected team name %q in list, but not found", teamName)
			}
			if !strings.Contains(listText, teamShortName) {
				return fmt.Errorf("expected team short name %q in list, but not found", teamShortName)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Use Team in New Game",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click("#sidebar-btn-dashboard"),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.WaitVisible("#dashboard-view"),
		chromedp.ActionFunc(OpenNewGameModal),
		// Select our team for Away
		chromedp.WaitVisible("#team-away-select"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var value string
			// Find option value by text
			err := chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const sel = document.getElementById('team-away-select');
					const opt = Array.from(sel.options).find(o => o.text === '%s');
					return opt ? opt.value : '';
				})()
			`, teamName), &value).Do(ctx)
			if err != nil {
				return err
			}
			if value == "" {
				return fmt.Errorf("could not find team %q in dropdown", teamName)
			}
			return chromedp.SetValue("#team-away-select", value).Do(ctx)
		}),
		chromedp.SetValue("#team-home-input", "Opponents"),
		chromedp.Click("#btn-start-new-game"),
		chromedp.WaitVisible("#scoresheet-view"),
	)

	runStep(t, ctx, "Verify Roster Hydration",
		chromedp.ActionFunc(func(ctx context.Context) error {
			var lineupText string
			// Check first batter in grid lineup column
			if err := chromedp.Text(`.lineup-cell[data-player-idx="0"]`, &lineupText).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(lineupText, "Player Ten") {
				return fmt.Errorf("expected first batter to be 'Player Ten', but got %q", lineupText)
			}
			if !strings.Contains(lineupText, "#10") {
				return fmt.Errorf("expected jersey #10, but got %q", lineupText)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Delete Team",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Click("#sidebar-btn-teams"),
		chromedp.WaitVisible("#teams-view"),
		// Click the card to open detail
		chromedp.Click(fmt.Sprintf(`//div[contains(@class, "bg-white") and .//h3[contains(text(), "%s")]]`, teamName)),
		chromedp.WaitVisible("#team-view"),
		// Click delete button in detail header
		chromedp.Click("#btn-team-detail-delete"),
		chromedp.WaitVisible("#custom-confirm-modal"),
		chromedp.Click("#btn-confirm-yes"),
		waitUntilDisplayNone("#custom-confirm-modal"),
		// Should return to list
		chromedp.WaitVisible("#teams-list"),
	)
}
