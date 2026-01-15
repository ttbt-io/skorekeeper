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

func TestAccessPolicyEnforcement(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	// 1. Start Server with Bootstrap Admin
	adminEmail := "admin@example.com"
	baseURL := startTestServerWithFlags(t, []string{"--admin", adminEmail})

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
			// t.Logf("JS CONSOLE (%s): %s", ev.Type, strings.Join(args, " "))
		}
	})

	// 2. Set Policy via Admin API (Backend direct call simulator)
	// Since we don't have a UI for this yet, we simulate the API call the admin would make.
	// We need to log in as admin first.
	runStep(t, ctx, "Login as Admin",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return LoginWithUser(ctx, baseURL, adminEmail)
		}),
	)

	// Apply Policy: Restrict games to 1
	runStep(t, ctx, "Apply Strict Policy via API",
		chromedp.Evaluate(`
			fetch('/api/admin/policy', {
				method: 'POST',
				headers: {'Content-Type': 'application/json'},
				body: JSON.stringify({
					defaultPolicy: 'allow',
					defaultMaxGames: 1,
					defaultMaxTeams: 1,
					defaultDenyMessage: 'Quota Reached',
					users: {},
					admins: []
				})
			}).then(r => { if(!r.ok) throw new Error(r.statusText) })
		`, nil),
	)

	// 3. Login as Restricted User
	userEmail := "user@restricted.com"
	runStep(t, ctx, "Login as Restricted User",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Logout first
			if err := chromedp.Evaluate(`fetch('/.sso/logout', {method: 'POST'})`, nil).Do(ctx); err != nil {
				return err
			}
			return LoginWithUser(ctx, baseURL, userEmail)
		}),
	)

	// 4. Create Game 1 (Should Succeed)
	runStep(t, ctx, "Create Game 1",
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := CreateGame(ctx, "Team A", "Team B")
			return err
		}),
	)

	// 5. Verify Quota Reached UI
	runStep(t, ctx, "Verify New Game Button Disabled",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#sidebar-btn-dashboard`),
		chromedp.WaitVisible(`#game-list`),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var disabled bool
			var title string
			if err := chromedp.Evaluate(`document.getElementById('btn-new-game').disabled`, &disabled).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Evaluate(`document.getElementById('btn-new-game').title`, &title).Do(ctx); err != nil {
				return err
			}

			if !disabled {
				return fmt.Errorf("New Game button should be disabled")
			}
			if !strings.Contains(title, "Quota Reached") {
				return fmt.Errorf("Tooltip should say Quota Reached, got: %s", title)
			}
			return nil
		}),
	)
}

func TestAdminDashboardAccess(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	adminEmail := "admin@example.com"
	baseURL := startTestServerWithFlags(t, []string{"--admin", adminEmail})

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 1. Unauthenticated Access
	runStep(t, ctx, "Verify Unauthenticated Access Denied",
		chromedp.Navigate(baseURL+"/admin"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Should return 403 Forbidden plain text or error page
			// We can check the body text
			var body string
			err := chromedp.Text(`body`, &body).Do(ctx)
			if err != nil {
				return err
			}
			if !strings.Contains(body, "Forbidden") {
				return fmt.Errorf("Expected Forbidden, got: %s", body)
			}
			return nil
		}),
	)

	// 2. Admin Access
	runStep(t, ctx, "Login as Admin and Access Dashboard",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return LoginWithUser(ctx, baseURL, adminEmail)
		}),
		chromedp.Navigate(baseURL+"/admin"),
		chromedp.WaitVisible(`#policyForm`), // Form inside admin_dashboard.html
	)

	// 3. Non-Admin Access
	runStep(t, ctx, "Login as Non-Admin and Verify Denial",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Logout
			if err := chromedp.Evaluate(`fetch('/.sso/logout', {method: 'POST'})`, nil).Do(ctx); err != nil {
				return err
			}
			return LoginWithUser(ctx, baseURL, "user@normal.com")
		}),
		chromedp.Navigate(baseURL+"/admin"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var body string
			err := chromedp.Text(`body`, &body).Do(ctx)
			if err != nil {
				return err
			}
			if !strings.Contains(body, "Forbidden") {
				return fmt.Errorf("Expected Forbidden for non-admin, got: %s", body)
			}
			return nil
		}),
	)
}
