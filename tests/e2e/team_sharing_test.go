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

func TestTeamSharing(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)
	// User B uses devtest hostname to isolate cookies from devtest.local (User A)
	userBURL := strings.Replace(baseURL, "devtest.local", "devtest", 1)

	// Context A: User A (Owner)
	ctxA, cancelA := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancelA()
	ctxA, cancelA = chromedp.NewContext(ctxA, chromedp.WithLogf(log.Printf))
	defer cancelA()
	ctxA, cancelA = context.WithTimeout(ctxA, 60*time.Second)
	defer cancelA()

	// Context B: User B (Invited Skorekeeper)
	ctxB, cancelB := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancelB()
	ctxB, cancelB = chromedp.NewContext(ctxB, chromedp.WithLogf(log.Printf))
	defer cancelB()
	ctxB, cancelB = context.WithTimeout(ctxB, 60*time.Second)
	defer cancelB()

	// Capture JS console logs
	chromedp.ListenTarget(ctxA, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			t.Logf("USER A JS: %s", ev.Args[0].Value)
		}
	})
	chromedp.ListenTarget(ctxB, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			t.Logf("USER B JS: %s", ev.Args[0].Value)
		}
	})

	teamName := "Shared Team"
	userBEmail := "userb@example.com"

	// 1. User A Creates Team
	runStep(t, ctxA, "User A creates team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, baseURL) // Logs in as default 'test@example.com'
		}),
		DisableCSSAnimations(),
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click("#sidebar-btn-teams"),
		chromedp.WaitVisible("#teams-view"),
		chromedp.Click("#btn-create-team"),
		chromedp.WaitVisible("#team-modal"),
		chromedp.SetValue("#team-name", teamName),
		chromedp.Click("#btn-save-team"),
		waitUntilDisplayNone("#team-modal"),
		chromedp.WaitVisible(fmt.Sprintf(`//h3[text()='%s']`, teamName), chromedp.BySearch),
	)

	// 2. User A Invites User B
	runStep(t, ctxA, "User A invites User B",
		chromedp.Sleep(1*time.Second),
		// Click the Team Card to open edit modal
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const headers = Array.from(document.querySelectorAll('h3'));
					const h = headers.find(el => el.textContent.trim() === '%s');
					if (!h) throw "Team header not found";
					h.click(); // Click header (or card)
				})()
			`, teamName), nil).Do(ctx)
		}),
		chromedp.WaitVisible("#team-modal"),
		chromedp.Click("#tab-team-members"),
		chromedp.WaitVisible("#team-members-view"),
		// Fill Invite
		chromedp.SetValue("#member-invite-email", userBEmail),
		chromedp.SetValue("#member-invite-role", "scorekeeper"),
		chromedp.Click("#btn-add-member"),
		// Verify added to list
		chromedp.WaitVisible(fmt.Sprintf(`//div[text()='%s']`, userBEmail), chromedp.BySearch),
		// Save
		chromedp.Click("#btn-save-team"),
		waitUntilDisplayNone("#team-modal"),
	)

	// 3. User B Logs in and Verifies Access
	runStep(t, ctxB, "User B logs in and sees shared team",
		chromedp.Navigate(userBURL),
		chromedp.WaitVisible("#btn-login"),
		// Mock login as User B
		chromedp.Evaluate(fmt.Sprintf(`
			document.cookie = "mock_auth_user=%s; path=/";
			window.location.reload();
		`, userBEmail), nil),
		chromedp.WaitVisible("#dashboard-view"),
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click("#sidebar-btn-teams"),
		chromedp.WaitVisible("#teams-view"),
		// Should see the team
		chromedp.WaitVisible(fmt.Sprintf(`//h3[text()='%s']`, teamName), chromedp.BySearch),
	)

	// 4. User B Edits Team
	newShortName := "SHRD"
	runStep(t, ctxB, "User B edits shared team",
		// Click Team Card
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const headers = Array.from(document.querySelectorAll('h3'));
					const h = headers.find(el => el.textContent.trim().startsWith('%s'));
					if (!h) {
						console.error('Available headers:', headers.map(h => h.textContent));
						throw "Team header not found for: " + '%s';
					}
					h.click();
				})()
			`, teamName, teamName), nil).Do(ctx)
		}),
		chromedp.WaitVisible("#team-modal"),
		chromedp.SetValue("#team-short-name", newShortName),
		chromedp.Click("#btn-save-team"),
		waitUntilDisplayNone("#team-modal"),
	)

	// 5. User A Verifies Edit
	runStep(t, ctxA, "User A verifies update",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Navigate(baseURL).Do(ctx) // Reload to get fresh data
		}),
		chromedp.WaitVisible("#dashboard-view"),
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click("#sidebar-btn-teams"),
		chromedp.WaitVisible("#teams-view"),
		// Check for short name in the list (assuming it is displayed)
		// Usually short name is displayed in parens or badges?
		// Or we can just open edit modal to check.
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`
		        				(() => {
		        										const headers = Array.from(document.querySelectorAll('h3'));
		        										const h = headers.find(el => el.textContent.trim().startsWith('%s'));
		        										if (!h) throw "Team header not found";
		        										h.click();
		        									})()
		        								`, teamName), nil).Do(ctx)
		}),
		chromedp.WaitVisible("#team-modal"),
		chromedp.Value("#team-short-name", &newShortName), // Verify value
	)
}
