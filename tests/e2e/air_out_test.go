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

func TestAirOutDP(t *testing.T) {
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
			cancel()
		}
	})

	runStep(t, ctx, "Game Initialization",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "AirOutAway", "AirOutHome")
			return err
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Walk First Batter",
		chromedp.Click(`#scoresheet-grid > .grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-ball`),
		chromedp.Click(`#btn-ball`),
		chromedp.Click(`#btn-ball`),
		chromedp.Click(`#btn-ball`), // Walk
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep(t, ctx, "Record Air DP (Fly Ball)",
		chromedp.Click(`.grid-cell[data-player-idx="1"]`), // Second batter
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),
		// Cycle Result to Fly
		chromedp.Click(`#btn-res`), // Out
		chromedp.Click(`#btn-res`), // Ground
		chromedp.Click(`#btn-res`), // Fly
		// Cycle Type to DP
		chromedp.Click(`#btn-type`), // OUT
		chromedp.Click(`#btn-type`), // SF
		chromedp.Click(`#btn-type`), // DP
		// Set Hit Location (Center Field)
		chromedp.Click(`#btn-loc`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(() => {
					const el = document.querySelector('.field-svg-keyboard');
					const rect = el.getBoundingClientRect();
					const x = rect.left + 100;
					const y = rect.top + 50; // High up
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
		// Runner Advance Screen should appear because of DP
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		// Runner on 1st: Defaults to Out for DP.
		// Batter: Out (implicit)
		chromedp.Click(`#btn-finish-turn`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep(t, ctx, "Verify Batter Paths",
		chromedp.ActionFunc(func(ctx context.Context) error {
			var paths []int
			// Batter is index 1
			err := chromedp.Evaluate(`
				(() => {
					// Access via state
                    // We need to find the event key.
                    // away-1-col-1-0 (Batter index 1)
                    const evt = window.app.state.activeGame.events['away-1-col-1-0'];
                    return evt ? evt.paths : null;
				})()
			`, &paths).Do(ctx)
			if err != nil {
				return err
			}
			if paths == nil {
				return fmt.Errorf("Event not found for batter 1")
			}
			t.Logf("Batter Paths: %v", paths)
			// paths[0] (Home->1st) should be 0 (Empty) for Air Out, NOT 2 (Out).
			if paths[0] != 0 {
				return fmt.Errorf("Expected paths[0] to be 0 (Empty) for Air Out DP, got %d", paths[0])
			}
			return nil
		}),
	)
}
