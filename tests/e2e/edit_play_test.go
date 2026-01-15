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

func TestEditPlayFeature(t *testing.T) {
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
		case *runtime.EventExceptionThrown:
			t.Logf("JS EXCEPTION: %s", ev.ExceptionDetails.Text)
			t.Fail()
			cancel() // Stop the test on JS exception
		}
	})

	var btnResText, btnBaseText, btnTypeText string
	var markerPresence string

	runStep(t, ctx, "Game Initialization",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "EditPlayAway", "EditPlayHome")
			return err
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Record Initial Play (Safe 1B)",
		chromedp.Click(`#scoresheet-grid > .grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var sideControlsHTML, btnShowBipHTML string
			var sideControlsComputedDisplay, btnShowBipComputedDisplay string
			chromedp.OuterHTML(`#side-controls`, &sideControlsHTML).Do(ctx)
			chromedp.Evaluate(`window.getComputedStyle(document.getElementById('side-controls')).display`, &sideControlsComputedDisplay).Do(ctx)
			chromedp.OuterHTML(`#btn-show-bip`, &btnShowBipHTML).Do(ctx)
			chromedp.Evaluate(`window.getComputedStyle(document.getElementById('btn-show-bip')).display`, &btnShowBipComputedDisplay).Do(ctx)
			t.Logf("DEBUG: #side-controls HTML: %s", sideControlsHTML)
			t.Logf("DEBUG: #side-controls Computed Display: %s", sideControlsComputedDisplay)
			t.Logf("DEBUG: #btn-show-bip HTML: %s", btnShowBipHTML)
			t.Logf("DEBUG: #btn-show-bip Computed Display: %s", btnShowBipComputedDisplay)
			return nil
		}),
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),
		// Default is Safe 1B HIT. Let's add hit location.
		chromedp.Click(`#btn-loc`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(() => {
					const el = document.querySelector('.field-svg-keyboard > svg');
					const rect = el.getBoundingClientRect();
					const x = rect.left + 100;
					const y = rect.top + 100;
					const event = new MouseEvent('click', {
						bubbles: true,
						cancelable: true,
						clientX: x,
						clientY: y,
					});
					el.dispatchEvent(event);
				})()
			`, nil).Do(ctx)
		}),
		chromedp.Click(`#btn-save-bip`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep(t, ctx, "Verify Play Recorded UI",
		chromedp.Click(`#scoresheet-grid > .grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.WaitVisible(`#btn-toggle-action`),
		// Verify action-area-recorded is visible (by proxy of btn-toggle-action) and pitch is hidden
		chromedp.ActionFunc(func(ctx context.Context) error {
			var classes string
			if err := chromedp.AttributeValue(`#action-area-pitch`, "class", &classes, nil).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(classes, "hidden") {
				return fmt.Errorf("Pitch buttons should be hidden, got classes: %s", classes)
			}

			// --- DEBUG START ---
			var sideControlsHTML string
			var sideControlsComputedStyle string
			chromedp.OuterHTML(`#side-controls`, &sideControlsHTML).Do(ctx)
			chromedp.Evaluate(`window.getComputedStyle(document.getElementById('side-controls')).display`, &sideControlsComputedStyle).Do(ctx)
			t.Logf("DEBUG: #side-controls HTML: %s", sideControlsHTML)
			t.Logf("DEBUG: #side-controls Computed Display: %s", sideControlsComputedStyle)
			// --- DEBUG END ---

			return nil
		}),
	)

	runStep(t, ctx, "Enter Edit Mode",
		chromedp.Click(`#btn-toggle-action`), // Click "PLAY RECORDED"
		chromedp.WaitVisible(`#action-area-pitch`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var classes string
			if err := chromedp.AttributeValue(`#action-area-recorded`, "class", &classes, nil).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(classes, "hidden") {
				return fmt.Errorf("Play Recorded button should be hidden after click, got classes: %s", classes)
			}

			// --- DEBUG START ---
			var sideControlsHTML string
			var sideControlsComputedStyle string
			chromedp.OuterHTML(`#side-controls`, &sideControlsHTML).Do(ctx)
			chromedp.Evaluate(`window.getComputedStyle(document.getElementById('side-controls')).display`, &sideControlsComputedStyle).Do(ctx)
			t.Logf("DEBUG: #side-controls HTML: %s", sideControlsHTML)
			t.Logf("DEBUG: #side-controls Computed Display: %s", sideControlsComputedStyle)
			// --- DEBUG END ---

			return nil
		}),
	)

	runStep(t, ctx, "Open BiP and Verify State",
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),
		chromedp.Text(`#btn-res`, &btnResText),
		chromedp.Text(`#btn-base`, &btnBaseText),
		chromedp.Text(`#btn-type`, &btnTypeText),
		chromedp.Evaluate(`document.querySelector('.field-svg-keyboard svg .hit-location-marker') ? 'present' : 'absent'`, &markerPresence),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if btnResText != "Safe" {
				return fmt.Errorf("Expected Safe, got %s", btnResText)
			}
			if btnBaseText != "1B" {
				return fmt.Errorf("Expected 1B, got %s", btnBaseText)
			}
			if btnTypeText != "HIT" {
				return fmt.Errorf("Expected HIT, got %s", btnTypeText)
			}
			if markerPresence != "present" {
				return fmt.Errorf("Hit location marker should be present")
			}
			return nil
		}),
	)

	runStep(t, ctx, "Modify Play to 2B and Save",
		chromedp.Click(`#btn-base`), // Cycle to 2B
		chromedp.Text(`#btn-base`, &btnBaseText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if btnBaseText != "2B" {
				return fmt.Errorf("Expected 2B after click, got %s", btnBaseText)
			}
			return nil
		}),
		chromedp.Click(`#btn-save-bip`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep(t, ctx, "Verify Updated Grid Outcome",
		chromedp.Text(`.grid-cell[data-player-idx="0"] .outcome-text`, &btnResText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(btnResText, "2B") {
				return fmt.Errorf("Expected grid outcome to contain 2B, got %s", btnResText)
			}
			return nil
		}),
	)
}
