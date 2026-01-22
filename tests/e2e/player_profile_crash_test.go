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
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestPlayerProfileCrash(t *testing.T) {
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
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Capture JS exceptions
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventExceptionThrown:
			t.Logf("JS EXCEPTION: %s", ev.ExceptionDetails.Text)
			if ev.ExceptionDetails.Exception != nil {
				t.Logf("JS EXCEPTION DETAILS: %s", ev.ExceptionDetails.Exception.Description)
			}
		case *runtime.EventConsoleAPICalled:
			args := make([]string, len(ev.Args))
			for i, arg := range ev.Args {
				args[i] = string(arg.Value)
			}
			// t.Logf("JS CONSOLE (%s): %s", ev.Type, strings.Join(args, " "))
		}
	})

	teamName := "Crash Test Team"
	playerName := "Crash Dummy"

	runStep(t, ctx, "Login",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, baseURL)
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Create Team with Player",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.WaitVisible("#sidebar-btn-teams"),
		chromedp.Click("#sidebar-btn-teams"),
		chromedp.WaitVisible("#teams-view"),
		chromedp.Click("#btn-create-team"),
		chromedp.WaitVisible("#team-modal"),
		chromedp.SetValue("#team-name", teamName),
		chromedp.SetValue(`#team-roster-container > div:nth-child(1) input[name="name"]`, playerName),
		chromedp.SetValue(`#team-roster-container > div:nth-child(1) input[name="number"]`, "00"),
		chromedp.Click("#btn-save-team"),
		waitUntilDisplayNone("#team-modal"),
	)

	runStep(t, ctx, "Open Team Detail and Click Player",
		chromedp.WaitVisible(fmt.Sprintf(`//h3[contains(text(), "%s")]`, teamName)),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Click(fmt.Sprintf(`//div[contains(@class, "bg-white") and .//h3[contains(text(), "%s")]]`, teamName)),
		chromedp.WaitVisible("#team-view"),
		// Click the player row
		chromedp.Click(fmt.Sprintf(`//span[contains(text(), "%s")]/ancestor::div[contains(@class, "hover:bg-gray-50")]`, playerName)),
		// Verify Profile Modal opens and name is correct
		chromedp.WaitVisible("#player-profile-modal"),
		chromedp.WaitVisible("#profile-name"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var text string
			if err := chromedp.Text("#profile-name", &text).Do(ctx); err != nil {
				return err
			}
			if text != playerName {
				return fmt.Errorf("expected profile name %q, got %q", playerName, text)
			}
			return nil
		}),
	)
}
