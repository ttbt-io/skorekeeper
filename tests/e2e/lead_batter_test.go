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
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestLeadBatterIndicator(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := startTestServer(t)

	// 1. Create Game
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := CreateGame(ctx, "Away Team", "Home Team")
			return err
		}),
	)
	if err != nil {
		t.Fatalf("CreateGame failed: %v", err)
	}

	// 2. Add Inning 2
	if err := AddInning(ctx); err != nil {
		t.Fatalf("AddInning failed: %v", err)
	}

	// 3. Record 3 Strikeouts
	// Batter 1
	if err := SelectCell(ctx, 1, 1); err != nil {
		t.Fatalf("SelectCell 1,1 failed: %v", err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, waitUntilDisplayNone(`#cso-modal`)); err != nil {
		t.Fatal(err)
	}

	// Batter 2
	if err := SelectCell(ctx, 2, 1); err != nil {
		t.Fatalf("SelectCell 2,1 failed: %v", err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, waitUntilDisplayNone(`#cso-modal`)); err != nil {
		t.Fatal(err)
	}

	// Batter 3 -> 3rd Out. Logic should set Batter 4 (idx 3) as lead for Inning 2.
	if err := SelectCell(ctx, 3, 1); err != nil {
		t.Fatalf("SelectCell 3,1 failed: %v", err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, waitUntilDisplayNone(`#cso-modal`)); err != nil {
		t.Fatal(err)
	}

	// 4. Verify Inning 2 (col-2-0), Batter 4 (idx 3) has the indicator.
	// We check by right-clicking and looking for "Unset Lead"
	err = chromedp.Run(ctx,
		chromedp.Sleep(500*time.Millisecond), // Wait for state update/render
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Right click cell: Inning 2 (col-2-0), Batter 4 (idx 3)
			selector := `div[data-col-id="col-2-0"][data-player-idx="3"]`
			js := `document.querySelector('` + selector + `').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, view: window}));`
			return chromedp.Evaluate(js, nil).Do(ctx)
		}),
		chromedp.WaitVisible(`#column-context-menu`, chromedp.ByID),
		chromedp.WaitVisible(`//button[text()="Unset Lead"]`, chromedp.BySearch),
	)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}
}

func TestLeadBatterRunnerAction(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := startTestServer(t)

	// 1. Create Game
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := CreateGame(ctx, "Away Team", "Home Team")
			return err
		}),
	)
	if err != nil {
		t.Fatalf("CreateGame failed: %v", err)
	}

	// 2. Add Inning 2
	if err := AddInning(ctx); err != nil {
		t.Fatalf("AddInning failed: %v", err)
	}

	// 3. Record 2 Outs first (Batter 1 & 2 K)
	if err := SelectCell(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, waitUntilDisplayNone(`#cso-modal`)); err != nil {
		t.Fatal(err)
	}

	if err := SelectCell(ctx, 2, 1); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#btn-strike`, chromedp.ByID)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, waitUntilDisplayNone(`#cso-modal`)); err != nil {
		t.Fatal(err)
	}

	// Batter 3 gets on base (Single)
	if err := SelectCell(ctx, 3, 1); err != nil {
		t.Fatal(err)
	}
	if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, waitUntilDisplayNone(`#cso-modal`)); err != nil {
		t.Fatal(err)
	}

	// Now Batter 4 is UP. But we use Runner Actions to pick off Batter 3 (Runner on 1B).
	// SelectCell opens CSO for Batter 4
	if err := SelectCell(ctx, 4, 1); err != nil {
		t.Fatal(err)
	}

	// Open Runner Actions
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`#btn-runner-actions`),
		chromedp.Click(`#btn-runner-actions`),
		chromedp.WaitVisible(`#cso-runner-action-view`),
		// Cycle runner action to 'PO' (Pickoff) -> Out
		// Initial: Stay -> SB -> CS -> PO
		chromedp.Click(`#runner-action-list button`, chromedp.ByQuery), // SB
		chromedp.Click(`#runner-action-list button`, chromedp.ByQuery), // CS
		chromedp.Click(`#runner-action-list button`, chromedp.ByQuery), // PO (Out)
		chromedp.Click(`#btn-save-runner-actions`, chromedp.ByID),
		waitUntilDisplayNone(`#cso-runner-action-view`),
	); err != nil {
		t.Fatal(err)
	}

	// Verify Inning 2 (col-2-0), Batter 4 (idx 3) has the indicator.
	err = chromedp.Run(ctx,
		chromedp.Sleep(500*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			selector := `div[data-col-id="col-2-0"][data-player-idx="3"]`
			js := `document.querySelector('` + selector + `').dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, view: window}));`
			return chromedp.Evaluate(js, nil).Do(ctx)
		}),
		chromedp.WaitVisible(`#column-context-menu`, chromedp.ByID),
		chromedp.WaitVisible(`//button[text()="Unset Lead"]`, chromedp.BySearch),
	)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}
}
