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

func TestHitLocationFeature(t *testing.T) {
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
			if ev.Type == runtime.APITypeError {
				t.Logf("JS CONSOLE ERROR: %s", strings.Join(args, " "))
				t.Fail()
				cancel()
			}
		case *runtime.EventExceptionThrown:
			t.Logf("JS EXCEPTION: %s", ev.ExceptionDetails.Text)
			t.Fail()
			cancel()
		}
	})

	var btnLocClass, divTrajControlsDisplay, btnTrajText, markerPresence, posKeyVisibility string
	var hitPathClass string

	runStep(t, ctx, "Game Initialization",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "HitLocAway", "HitLocHome")
			return err
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Open CSO and Ball In Play",
		chromedp.Click(`#scoresheet-grid > .grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),
	)

	runStep(t, ctx, "Verify Default Location Mode State",
		chromedp.AttributeValue(`#btn-loc`, "class", &btnLocClass, nil),
		chromedp.AttributeValue(`#div-traj-controls`, "class", &divTrajControlsDisplay, nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if strings.Contains(btnLocClass, "active") {
				return fmt.Errorf("btn-loc should not be active by default, classes: %s", btnLocClass)
			}
			if !strings.Contains(divTrajControlsDisplay, "hidden") {
				return fmt.Errorf("div-traj-controls should be hidden by default, classes: %s", divTrajControlsDisplay)
			}
			t.Log("Default location mode state verified")
			return nil
		}),
	)

	runStep(t, ctx, "Activate Location Mode",
		chromedp.Click(`#btn-loc`),
		chromedp.AttributeValue(`#btn-loc`, "class", &btnLocClass, nil),
		chromedp.AttributeValue(`.field-svg-keyboard`, "class", &hitPathClass, nil), // Reusing hitPathClass var
		chromedp.Evaluate(`
			(() => {
				const keys = document.querySelectorAll('.pos-key');
				for (let k of keys) {
					if (k.style.display !== 'none') return 'visible';
				}
				return 'hidden';
			})()
		`, &posKeyVisibility),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(btnLocClass, "active") {
				return fmt.Errorf("btn-loc should be active, classes: %s", btnLocClass)
			}
			if !strings.Contains(hitPathClass, "location-mode-active") {
				return fmt.Errorf("field-svg-keyboard should be active, classes: %s", hitPathClass)
			}
			if posKeyVisibility != "hidden" {
				return fmt.Errorf("Pos keys should be hidden in location mode, got %s", posKeyVisibility)
			}
			t.Log("Location mode activated and UI updated")
			return nil
		}),
	)

	runStep(t, ctx, "Record Hit Location (Grounder)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(() => {
					const el = document.querySelector('.field-svg-keyboard > svg');
					const rect = el.getBoundingClientRect();
					const x = rect.left + 50; // 50px from left
					const y = rect.top + 50;  // 50px from top
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
		chromedp.AttributeValue(`#btn-loc`, "class", &btnLocClass, nil),
		chromedp.AttributeValue(`#div-traj-controls`, "class", &divTrajControlsDisplay, nil),
		chromedp.Text(`#btn-traj`, &btnTrajText),
		chromedp.Evaluate(`document.querySelector('.field-svg-keyboard svg .hit-location-marker') ? 'present' : 'absent'`, &markerPresence),
		chromedp.Evaluate(`
			(() => {
				const keys = document.querySelectorAll('.pos-key');
				for (let k of keys) {
					if (k.style.display !== 'none') return 'visible';
				}
				return 'hidden';
			})()
		`, &posKeyVisibility),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if strings.Contains(btnLocClass, "active") {
				return fmt.Errorf("btn-loc should be inactive after setting location, classes: %s", btnLocClass)
			}
			if strings.Contains(divTrajControlsDisplay, "hidden") {
				return fmt.Errorf("div-traj-controls should be visible, classes: %s", divTrajControlsDisplay)
			}
			if markerPresence != "present" {
				return fmt.Errorf("Hit location marker not found")
			}
			if posKeyVisibility != "visible" {
				return fmt.Errorf("Pos keys should be visible, got %s", posKeyVisibility)
			}
			if btnTrajText != "G" {
				return fmt.Errorf("Default trajectory should be G, got %s", btnTrajText)
			}
			t.Log("Hit location recorded and UI updated")
			return nil
		}),
	)

	runStep(t, ctx, "Cycle Trajectory",
		chromedp.Click(`#btn-traj`), // LINE
		chromedp.Text(`#btn-traj`, &btnTrajText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if btnTrajText != "L" {
				return fmt.Errorf("Expected L, got %s", btnTrajText)
			}
			return nil
		}),
		chromedp.Click(`#btn-traj`), // FLY
		chromedp.Text(`#btn-traj`, &btnTrajText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if btnTrajText != "F" {
				return fmt.Errorf("Expected F, got %s", btnTrajText)
			}
			return nil
		}),
		chromedp.Click(`#btn-traj`), // POP
		chromedp.Text(`#btn-traj`, &btnTrajText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if btnTrajText != "P" {
				return fmt.Errorf("Expected P, got %s", btnTrajText)
			}
			return nil
		}),
		chromedp.Click(`#btn-traj`), // GROUND
		chromedp.Text(`#btn-traj`, &btnTrajText),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if btnTrajText != "G" {
				return fmt.Errorf("Expected G, got %s", btnTrajText)
			}
			t.Log("Trajectory cycled verified")
			return nil
		}),
	)

	runStep(t, ctx, "Clear Hit Location",
		chromedp.Click(`#btn-clear-loc`),
		chromedp.AttributeValue(`#div-traj-controls`, "class", &divTrajControlsDisplay, nil),
		chromedp.Evaluate(`document.querySelector('.field-svg-keyboard svg .hit-location-marker') ? 'present' : 'absent'`, &markerPresence),
		chromedp.Evaluate(`
			(() => {
				const keys = document.querySelectorAll('.pos-key');
				for (let k of keys) {
					if (k.style.display === 'none') return 'hidden';
				}
				return 'visible';
			})()
		`, &posKeyVisibility),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(divTrajControlsDisplay, "hidden") {
				return fmt.Errorf("div-traj-controls should be hidden, classes: %s", divTrajControlsDisplay)
			}
			if markerPresence != "absent" {
				return fmt.Errorf("Hit location marker should be absent")
			}
			if posKeyVisibility != "visible" {
				return fmt.Errorf("Pos keys should be visible, got %s", posKeyVisibility)
			}
			t.Log("Hit location cleared verified")
			return nil
		}),
	)

	runStep(t, ctx, "Record Hit Location and Commit Play (Fly Ball)",
		chromedp.Click(`#btn-loc`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(() => {
					const el = document.querySelector('.field-svg-keyboard > svg');
					const rect = el.getBoundingClientRect();
					const x = rect.left + 100; // 100px from left
					const y = rect.top + 20;   // 20px from top
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
		chromedp.Click(`#btn-traj`), // Default is GROUND, click to LINE
		chromedp.Click(`#btn-traj`), // LINE to FLY
		chromedp.Click(`#btn-save-bip`),
		waitUntilDisplayNone(`#cso-modal`),
		chromedp.Sleep(100*time.Millisecond), // Give renderGrid a moment
	)

	runStep(t, ctx, "Verify Hit Path on Grid Cell (Fly Ball)",
		chromedp.Click(`#scoresheet-grid > .grid-cell`), // Re-open CSO to ensure data is there if needed
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),
		chromedp.Sleep(100*time.Millisecond), // Wait for CSO to close

		// Now inspect the grid cell's SVG for the rendered path
		// Use specific data attributes to target the correct cell (Batter 0, Col col-1-0)
		chromedp.AttributeValue(`.grid-cell[data-player-idx="0"] > svg > .hit-path.fly`, "class", &hitPathClass, nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !strings.Contains(hitPathClass, "hit-path") || !strings.Contains(hitPathClass, "fly") {
				return fmt.Errorf("Fly ball hit path not found or incorrect class. Got: %s", hitPathClass)
			}
			t.Log("Fly ball hit path rendered on grid verified")
			return nil
		}),
	)
}
