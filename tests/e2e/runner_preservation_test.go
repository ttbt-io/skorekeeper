// Copyright (c) 2026 TTBT Enterprises LLC
package e2e

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ttbt-io/skorekeeper/tools/e2ehelpers"
)

func TestRunnerPreservationOnEdit(t *testing.T) {
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

	// Play 1: Single
	runStep(t, ctx, "Batter 1: Single",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SelectCell(ctx, 1, 1)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.RecordBallInPlay(ctx, "Safe", "HIT", "")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Play 2: Single, advance P1 to 2nd
	runStep(t, ctx, "Batter 2: Single",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SelectCell(ctx, 2, 1)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.RecordBallInPlay(ctx, "Safe", "HIT", "")
		}),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		// P1 is on 1st base (idx=0). Advance him to 2nd.
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(() => {
					const btns = document.querySelectorAll('.runner-outcome-btn');
					// The second button is usually "To 2nd"
					const to2nd = Array.from(btns).find(b => b.textContent.includes('To 2nd'));
					if(to2nd) to2nd.click();
				})()
			`, nil).Do(ctx)
		}),
		chromedp.ActionFunc(e2ehelpers.FinishTurn),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Edit P1
	runStep(t, ctx, "Edit P1 to 1B ERR",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SelectCell(ctx, 1, 1)
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
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Verify P1 is on 2nd Base
	runStep(t, ctx, "Verify P1 on 2nd",
		chromedp.ActionFunc(func(ctx context.Context) error {
			var paths []int
			err := chromedp.Evaluate(`window.app.state.activeGame.events['away-0-col-1-0'].paths`, &paths).Do(ctx)
			if err != nil {
				return err
			}
			if len(paths) < 2 || paths[1] != 1 {
				return fmt.Errorf("Expected P1 on 2nd base (paths[1] == 1), got paths: %v", paths)
			}
			return nil
		}),
	)
}
