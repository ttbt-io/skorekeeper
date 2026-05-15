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
	"github.com/ttbt-io/skorekeeper/tools/e2ehelpers"
)

func TestDualTimelineInsertionAndEdit(t *testing.T) {
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
	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
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

	runStep(t, ctx, "Game Initialization",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "Away", "Home")
			return err
		}),
		DisableCSSAnimations(),
	)

	// Play 1: Ground Out
	runStep(t, ctx, "Batter 1: Ground Out",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SelectCell(ctx, 1, 1)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.RecordBallInPlay(ctx, "Ground", "OUT", "6")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Play 2: Single
	runStep(t, ctx, "Batter 2: Single",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SelectCell(ctx, 2, 1)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.RecordBallInPlay(ctx, "Safe", "HIT", "")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Verify initial state
	runStep(t, ctx, "Verify Initial State",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.AssertScore(ctx, "0", "0")
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var outs int
			err := chromedp.Evaluate(`window.app.calculateStats().currentPA.outs`, &outs).Do(ctx)
			if err != nil {
				return err
			}
			if outs != 1 {
				return fmt.Errorf("Expected 1 out, got %d", outs)
			}
			return nil
		}),
	)

	// Context menu on Batter 1 to insert
	runStep(t, ctx, "Insert PA After Batter 1",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Find cell for slot=1, inning=1 using the same nth-child logic
			var js = fmt.Sprintf(`
				(() => {
					const headers = document.querySelectorAll('#scoresheet-grid > .grid-header');
					if (!headers.length) return false;
					const stride = headers.length;
					const index = stride + (%d - 1) * stride + 1 + %d;
					const el = document.querySelector('#scoresheet-grid > div:nth-child(' + index + ')');
					if (el) {
						el.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true }));
						return true;
					}
					return false;
				})()
			`, 1, 1)
			var success bool
			if err := chromedp.Evaluate(js, &success).Do(ctx); err != nil {
				return err
			}
			if !success {
				return fmt.Errorf("could not right-click cell 1-1")
			}
			return nil
		}),
		chromedp.WaitVisible(`#column-context-menu`),
		chromedp.Click(`//button[text()="Insert PA After"]`, chromedp.BySearch),
		chromedp.Sleep(500*time.Millisecond), // Wait for grid re-render and UI shift
	)

	// Now, record Walk in the newly inserted cell (which shifted into slot 2)
	runStep(t, ctx, "Record Walk in Inserted Cell",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SelectCell(ctx, 2, 1) // Slot 2 is the new empty cell
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			for i := 0; i < 4; i++ {
				if err := e2ehelpers.RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return e2ehelpers.FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// The single was pushed to slot 3. Let's edit it.
	runStep(t, ctx, "Edit shifted Play 3 (formerly Play 2)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SelectCell(ctx, 3, 1) // Slot 3 is the old Single
		}),
		chromedp.WaitVisible(`#btn-toggle-action`),
		chromedp.Click(`#btn-toggle-action`), // Enter Edit Mode
		chromedp.WaitVisible(`#btn-show-bip`),
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Change from HIT to ERR
			return chromedp.Evaluate(`
                (() => {
                    const errBtn = Array.from(document.querySelectorAll('.cso-option-btn')).find(el => el.textContent === 'ERR');
					if(errBtn) errBtn.click();
                    document.getElementById('btn-save-bip').click();
                })()
            `, nil).Do(ctx)
		}),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(e2ehelpers.FinishTurn),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Verify the logical timeline correctly applied everything
	runStep(t, ctx, "Verify Final State",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Outs should still be 1: Ground out (1), Walk (0), Error (0)
			var outs int
			err := chromedp.Evaluate(`window.app.calculateStats().currentPA.outs`, &outs).Do(ctx)
			if err != nil {
				return err
			}
			if outs != 1 {
				return fmt.Errorf("Expected 1 out, got %d", outs)
			}
			return nil
		}),
	)
}
