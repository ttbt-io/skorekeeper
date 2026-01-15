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
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestFlyBallCurve(t *testing.T) {
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

	var pathD string

	runStep(t, ctx, "Game Initialization",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "CurveTestAway", "CurveTestHome")
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

	runStep(t, ctx, "Record Straight Center Hit",
		chromedp.Click(`#btn-loc`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Click exactly in the horizontal center of the SVG
			return chromedp.Evaluate(`
				(() => {
					const el = document.querySelector('.field-svg-keyboard > svg');
					const rect = el.getBoundingClientRect();
					// SVG viewBox is 0 0 200 200.
					// We want x=100 (center).
					// Calculate scaling ratio
					const scaleX = rect.width / 200;
					const scaleY = rect.height / 200;
					
					const clickX = rect.left + (100 * scaleX);
					const clickY = rect.top + (20 * scaleY); // Deep center

					const event = new MouseEvent('click', {
						bubbles: true,
						cancelable: true,
						clientX: clickX,
						clientY: clickY,
					});
					el.dispatchEvent(event);
				})()
			`, nil).Do(ctx)
		}),
		chromedp.Click(`#btn-traj`), // G -> L
		chromedp.Click(`#btn-traj`), // L -> F
	)

	runStep(t, ctx, "Verify Field View Curve",
		chromedp.AttributeValue(`.field-svg-keyboard svg .hit-location-marker[d]`, "d", &pathD, nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Logf("Field View Path D: %s", pathD)
			// Expected: M 100 170 Q 125 ...
			// The control point X should be 125 because of the +25 offset for straight hits
			if !strings.Contains(pathD, "Q 125") {
				return fmt.Errorf("Expected curve control point at 125, got path: %s", pathD)
			}
			return nil
		}),
	)

	runStep(t, ctx, "Save and Verify Grid View Curve",
		chromedp.Click(`#btn-save-bip`),
		waitUntilDisplayNone(`#cso-modal`),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.AttributeValue(`.grid-cell[data-player-idx="0"] > svg > .hit-path.fly`, "d", &pathD, nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Logf("Grid View Path D: %s", pathD)
			// Expected: M 30 55 Q 40 ...
			// The control point X should be 40 because of the +10 offset (30 center + 10)
			// Depending on float precision, it might be 40 or 40.something or 39.something.
			// But since we used exact integers in calculation, it should be clean.
			// However, SVG serialization might vary.
			matched, _ := regexp.MatchString(`Q 40[\s,]`, pathD)
			if !matched {
				return fmt.Errorf("Expected curve control point around 40, got path: %s", pathD)
			}
			return nil
		}),
	)
}
