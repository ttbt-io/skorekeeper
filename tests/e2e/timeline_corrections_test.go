// Copyright (c) 2026 TTBT Enterprises LLC
package e2e

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ttbt-io/skorekeeper/tools/e2ehelpers"
)

func TestTimeline_ClearPhantomPlay(t *testing.T) {
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

	// B1: 1B
	runStep(t, ctx, "B1: 1B",
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.SelectCell(ctx, 1, 1) }),
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.RecordBallInPlay(ctx, "Safe", "HIT", "") }),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// B2: Strikeout (Phantom)
	runStep(t, ctx, "B2: K",
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.SelectCell(ctx, 2, 1) }),
		chromedp.ActionFunc(func(ctx context.Context) error {
			for i := 0; i < 3; i++ {
				if err := e2ehelpers.RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// B3: 1B
	runStep(t, ctx, "B3: 1B",
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.SelectCell(ctx, 3, 1) }),
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.RecordBallInPlay(ctx, "Safe", "HIT", "") }),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(e2ehelpers.FinishTurn),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Clear B2
	runStep(t, ctx, "Clear B2",
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.SelectCell(ctx, 2, 1) }),
		chromedp.WaitVisible(`#btn-toggle-action`),
		chromedp.Click(`#btn-toggle-action`), // Edit mode
		chromedp.WaitVisible(`#btn-clear-all`),
		chromedp.Click(`#btn-clear-all`),
		chromedp.WaitVisible(`#custom-confirm-modal`),
		chromedp.Click(`#btn-confirm-yes`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Verify B3 shifted to B2
	runStep(t, ctx, "Verify B3 shifted to B2",
		chromedp.ActionFunc(func(ctx context.Context) error {
			var paths []int
			// The original B3 (which was at away-2-col-1-0) should now be at away-1-col-1-0
			err := chromedp.Evaluate(`window.app.state.activeGame.events['away-1-col-1-0'].paths`, &paths).Do(ctx)
			if err != nil {
				return fmt.Errorf("Failed to read shifted event: %v", err)
			}
			if paths[0] != 1 {
				return fmt.Errorf("Expected 1B at B2 slot, got paths: %v", paths)
			}
			return nil
		}),
	)
}

func TestTimeline_DPtoFC(t *testing.T) {
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

	// B1: 1B
	runStep(t, ctx, "B1: 1B",
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.SelectCell(ctx, 1, 1) }),
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.RecordBallInPlay(ctx, "Safe", "HIT", "") }),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// B2: DP (2 outs)
	runStep(t, ctx, "B2: DP",
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.SelectCell(ctx, 2, 1) }),
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.RecordBallInPlay(ctx, "Out", "DP", "6") }),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// B3: Groundout (3 outs)
	runStep(t, ctx, "B3: Groundout",
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.SelectCell(ctx, 3, 1) }),
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.RecordBallInPlay(ctx, "Ground", "OUT", "5") }),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// B4: 1B (Starts Top of 2nd Inning)
	runStep(t, ctx, "B4: 1B (Top 2nd)",
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.SelectCell(ctx, 4, 2) }), // visual col 2
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.RecordBallInPlay(ctx, "Safe", "HIT", "") }),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Edit B2: DP to FC
	runStep(t, ctx, "Edit B2: DP to FC",
		chromedp.ActionFunc(func(ctx context.Context) error { return e2ehelpers.SelectCell(ctx, 2, 1) }),
		chromedp.WaitVisible(`#btn-toggle-action`),
		chromedp.Click(`#btn-toggle-action`), // Edit mode
		chromedp.WaitVisible(`#btn-show-bip`),
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(() => {
					const fcBtn = Array.from(document.querySelectorAll('.cso-option-btn')).find(el => el.textContent === 'FC');
					if(fcBtn) fcBtn.click();
					const safeBtn = Array.from(document.querySelectorAll('#btn-res')).find(el => el.textContent === 'Safe');
					if(safeBtn) safeBtn.click();
					document.getElementById('btn-save-bip').click();
				})()
			`, nil).Do(ctx)
		}),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Find the runner out button
			return chromedp.Evaluate(`
				(() => {
					const btns = document.querySelectorAll('.runner-outcome-btn');
					const outBtn = Array.from(btns).find(b => b.textContent.includes('Out'));
					if(outBtn) outBtn.click();
				})()
			`, nil).Do(ctx)
		}),
		chromedp.ActionFunc(e2ehelpers.FinishTurn),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Verify B4 shifted back to Inning 1
	runStep(t, ctx, "Verify B4 is in Inning 1",
		chromedp.ActionFunc(func(ctx context.Context) error {
			var evts string
			err := chromedp.Evaluate(`JSON.stringify(Object.keys(window.app.state.activeGame.events))`, &evts).Do(ctx)
			if err != nil {
				return err
			}
			if !strings.Contains(evts, "away-3-col-1-0") {
				return fmt.Errorf("B4 did not shift to col-1-0. Events: %s", evts)
			}
			return nil
		}),
	)
}
