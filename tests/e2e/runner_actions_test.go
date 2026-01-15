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
	"log"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestRunnerActionsBatch(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	// Create a new allocator context
	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	// Create a new browser context
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// 1. Setup Game
	gameID, err := LoginAndCreateGame(ctx, baseURL, "Away Team", "Home Team")
	if err != nil {
		t.Fatalf("CreateGame failed: %v", err)
	}
	t.Logf("Game created with ID: %s", gameID)

	// 2. Put a runner on 1st (Batter 1: Single)
	if err := SelectCell(ctx, 1, 1); err != nil { // Slot 1, Inning 1
		t.Fatalf("SelectCell(1,1) failed: %v", err)
	}
	if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
		t.Fatalf("RecordBallInPlay failed: %v", err)
	}
	// Wait for modal to close (since no runners were on base, it closes automatically)
	if err := chromedp.Run(ctx, waitUntilDisplayNone(`#cso-modal`)); err != nil {
		t.Fatalf("Failed waiting for CSO to close: %v", err)
	}

	// 3. Open next batter (Batter 2)
	if err := SelectCell(ctx, 2, 1); err != nil {
		t.Fatalf("SelectCell(2,1) failed: %v", err)
	}

	// 4. Open Runner Actions
	// The runner from Batter 1 (Away Team) should be on 1st.
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`#btn-runner-actions`),
		chromedp.Click(`#btn-runner-actions`),
		chromedp.WaitVisible(`#cso-runner-action-view`),
	); err != nil {
		t.Fatalf("Failed to open Runner Actions: %v", err)
	}

	// 5. Interact with new Batched UI
	// Cycle the first runner's action button to "SB"
	if err := chromedp.Run(ctx,
		cycleTo(t, "#runner-action-list > div:nth-child(1) > button", "SB"),
	); err != nil {
		t.Fatalf("Failed to cycle runner action: %v", err)
	}

	// 6. Save Batch
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`#btn-save-runner-actions`),
		chromedp.Click(`#btn-save-runner-actions`),
		waitUntilDisplayNone(`#cso-runner-action-view`),
		chromedp.WaitVisible(`#cso-modal`),
	); err != nil {
		t.Fatalf("Failed to save runner actions: %v", err)
	}

	// 7. Verify Outcome
	// Verify we see a ghost runner on 2nd base (idx 1).
	// Selector: .ghost-runner[data-base-idx="1"]
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`.ghost-runner[data-base-idx="1"]`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Ghost runner on 2B not found: %v", err)
	}
}
