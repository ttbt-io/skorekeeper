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

func TestTeamScreen(t *testing.T) {
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

	teamName := "Screen Test Team"
	teamShortName := "STT"

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
		chromedp.SetValue(`#team-roster-container > div:nth-child(1) input[name="number"]`, "99"),
		chromedp.SetValue(`#team-roster-container > div:nth-child(1) input[name="name"]`, "Test Player"),
		chromedp.Click("#btn-save-team"),
		waitUntilDisplayNone("#team-modal"),
		chromedp.WaitVisible("#teams-list"),
	)

	runStep(t, ctx, "Open Team Detail",
		// Wait for the team to appear in the list
		chromedp.WaitVisible(fmt.Sprintf(`//h3[contains(text(), "%s")]`, teamName)),
		chromedp.Sleep(500*time.Millisecond), // Let UI settle
		// Click the card container
		chromedp.Click(fmt.Sprintf(`//div[contains(@class, "bg-white") and .//h3[contains(text(), "%s")]]`, teamName)),
		chromedp.WaitVisible("#team-view"),
		chromedp.WaitVisible("#team-detail-name"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var text string
			if err := chromedp.Text("#team-detail-name", &text).Do(ctx); err != nil {
				return err
			}
			if text != teamName {
				return fmt.Errorf("expected detail title %q, got %q", teamName, text)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Verify Detail View Elements",
		chromedp.WaitVisible("#btn-team-detail-edit"),
		chromedp.WaitVisible("#btn-team-detail-delete"),
		chromedp.WaitVisible("#tab-team-detail-roster"),
		chromedp.WaitVisible("#tab-team-detail-members"),
		chromedp.WaitVisible("#btn-team-detail-stats"),
	)

	runStep(t, ctx, "Verify Roster Tab Active",
		// Wait for roster content to be populated (verifies render and visibility)
		chromedp.WaitVisible(`#team-detail-roster-view div.bg-white`),
		// Verify members view is hidden
		chromedp.WaitNotVisible("#team-detail-members-view"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var text string
			if err := chromedp.Text("#team-detail-roster-view", &text).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(text, "Test Player") {
				return fmt.Errorf("expected roster to contain 'Test Player', got %q", text)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Switch to Members Tab",
		chromedp.Click("#tab-team-detail-members"),
		chromedp.WaitVisible("#team-detail-members-view:not(.hidden)"),
		chromedp.WaitNotVisible("#team-detail-roster-view"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var text string
			if err := chromedp.Text("#team-detail-members-view", &text).Do(ctx); err != nil {
				return err
			}
			// Should contain Admin (current user email)
			// Since we use Login helper, the email is default "user@example.com" or similar from AuthManager defaults if we didn't specify.
			// The Login helper uses dev mode which might auto-login or use "owner@example.com" if we check helpers.
			// Let's just check for 'Admins' label.
			if !strings.Contains(text, "ADMINS") {
				return fmt.Errorf("expected members view to contain 'ADMINS' section, got %q", text)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Test View Stats Link",
		chromedp.Click("#btn-team-detail-stats"),
		chromedp.WaitVisible("#statistics-view"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var val string
			if err := chromedp.Value("#stats-search", &val).Do(ctx); err != nil {
				return err
			}
			if !strings.HasPrefix(val, "team:") {
				return fmt.Errorf("expected stats filter to start with 'team:', got %q", val)
			}
			teamId := strings.TrimPrefix(val, "team:")

			// Verify dropdown has this team
			// Wait for any option to populate (besides "All Teams")
			if err := chromedp.WaitReady(`#stats-adv-search-team option:not([value=""])`).Do(ctx); err != nil {
				return err
			}
			// Use Evaluate to get textContent directly
			var jsText string
			jsSel := fmt.Sprintf(`document.querySelector('#stats-adv-search-team option[value="%s"]').textContent`, teamId)
			if err := chromedp.Evaluate(jsSel, &jsText).Do(ctx); err != nil {
				return fmt.Errorf("failed to get text content for option value %q: %w", teamId, err)
			}
			if strings.TrimSpace(jsText) != teamName {
				return fmt.Errorf("expected option text %q, got %q (teamId: %s)", teamName, jsText, teamId)
			}
			return nil
		}), // Go back to team detail
		chromedp.Evaluate("window.history.back()", nil),
		chromedp.WaitVisible("#team-view"),
	)

	runStep(t, ctx, "Test Back Button",
		chromedp.Click("#btn-team-detail-back"),
		chromedp.WaitVisible("#teams-view"),
		// Re-open for delete test
		chromedp.Click(fmt.Sprintf(`//div[contains(@class, "bg-white") and .//h3[contains(text(), "%s")]]`, teamName)),
		chromedp.WaitVisible("#team-view"),
	)

	runStep(t, ctx, "Delete Team from Detail View",
		chromedp.Click("#btn-team-detail-delete"),
		chromedp.WaitVisible("#custom-confirm-modal"),
		chromedp.Click("#btn-confirm-yes"),
		waitUntilDisplayNone("#custom-confirm-modal"),
		chromedp.WaitVisible("#teams-view"), // Should navigate back to list
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Verify team is gone
			var listText string
			if err := chromedp.Text("#teams-list", &listText).Do(ctx); err != nil {
				return err
			}
			if strings.Contains(listText, teamName) {
				return fmt.Errorf("expected team %q to be deleted, but still found in list", teamName)
			}
			return nil
		}),
	)
}
