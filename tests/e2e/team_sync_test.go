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

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestTeamOfflineSync(t *testing.T) {
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
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			args := make([]string, len(ev.Args))
			for i, arg := range ev.Args {
				args[i] = string(arg.Value)
			}
			t.Logf("JS CONSOLE (%s): %s", ev.Type, strings.Join(args, " "))
		}
	})

	teamName := "Offline Team"

	runStep(t, ctx, "Login",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, baseURL)
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Block Team Sync API",
		network.Enable(),
		network.SetBlockedURLs([]string{"*/api/save-team"}),
	)

	runStep(t, ctx, "Create Team while 'Offline'",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Click("#sidebar-btn-teams"),
		chromedp.WaitVisible("#teams-view"),
		chromedp.Click("#btn-create-team"),
		chromedp.WaitVisible("#team-modal"),
		chromedp.SetValue("#team-name", teamName),
		chromedp.Click("#btn-save-team"),
		waitUntilDisplayNone("#team-modal"),
	)

	runStep(t, ctx, "Verify 'Error' Icon (from failed auto-sync)",
		chromedp.WaitVisible("#teams-list"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// It might take a moment for auto-sync to fail and show error icon
			success := false
			for i := 0; i < 10; i++ {
				var listHTML string
				if err := chromedp.OuterHTML("#teams-list", &listHTML).Do(ctx); err != nil {
					return err
				}
				if strings.Contains(listHTML, "❌") {
					success = true
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			if !success {
				return fmt.Errorf("expected 'Sync Error' icon (❌) in team list, but not found.")
			}
			t.Log("Error icon verified.")
			return nil
		}),
	)

	runStep(t, ctx, "Unblock Team Sync API",
		network.SetBlockedURLs([]string{}),
	)

	runStep(t, ctx, "Trigger Manual Sync (to resolve error)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(async () => {
					const btns = Array.from(document.querySelectorAll('button[id^="team-sync-btn-"]'));
					if (btns.length === 0) throw "Sync button not found";
					btns[0].click();
				})()
			`, nil).Do(ctx)
		}),
	)

	runStep(t, ctx, "Verify 'Synced' Icon",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// It might take a moment for the fetch to complete
			success := false
			for i := 0; i < 10; i++ {
				var listHTML string
				if err := chromedp.OuterHTML("#teams-list", &listHTML).Do(ctx); err != nil {
					return err
				}
				if strings.Contains(listHTML, "✅") {
					success = true
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			if !success {
				return fmt.Errorf("team failed to sync manually. 'Synced' icon (✅) not found.")
			}
			t.Log("Synced icon verified.")
			return nil
		}),
	)
}
