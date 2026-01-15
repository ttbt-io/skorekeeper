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

	"github.com/chromedp/chromedp"
)

// runStep, cycleTo, waitUntilDisplayNone are defined in main_test.go and accessible in the same package.

func TestStats(t *testing.T) {
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

	var batter1RBI, batter2RBI string

	runStep(t, ctx, "Navigate to App and create new game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := LoginAndCreateGame(ctx, baseURL, "Stats Away", "Stats Home")
			return err
		}),
	)

	// Batter 1: 1B
	runStep(t, ctx, "Batter 1: Single (1B)",
		// Row 0, Col 1. Grid index 12.
		chromedp.Click("#scoresheet-grid > div:nth-child(12)"),
		chromedp.WaitVisible("#cso-modal", chromedp.ByID),
		chromedp.Click("#btn-show-bip", chromedp.ByID),
		chromedp.Click("#btn-save-bip", chromedp.ByID), // Default 1B Safe
		// No runners on base, so CSO closes immediately
		waitUntilDisplayNone("#cso-modal"), // Using the helper
	)

	// Batter 2: HR
	runStep(t, ctx, "Batter 2: Home Run (HR)",
		// Row 1, Col 1. Grid index 22.
		chromedp.Click("#scoresheet-grid > div:nth-child(22)"),
		chromedp.WaitVisible("#cso-modal", chromedp.ByID),
		chromedp.Click("#btn-show-bip", chromedp.ByID),
		cycleTo(t, "#btn-res", "Safe"),                 // Ensure "Safe"
		cycleTo(t, "#btn-base", "Home"),                // Cycle base to Home
		chromedp.Click("#btn-save-bip", chromedp.ByID), // HR

		// Runner Advance (Batter 1 Scores - P1 is on 1B from previous play)
		chromedp.WaitVisible("#cso-runner-advance-view", chromedp.ByID),
		chromedp.Click("#btn-finish-turn", chromedp.ByID),
		waitUntilDisplayNone("#cso-modal"), // Using the helper
	)

	// Verify Stats
	runStep(t, ctx, "Verify Player RBIs",
		// Row 0 RBI: Index 20.
		chromedp.Text("#scoresheet-grid > div:nth-child(20)", &batter1RBI),
		// Row 1 RBI: Index 30.
		chromedp.Text("#scoresheet-grid > div:nth-child(30)", &batter2RBI),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if batter1RBI != "0" {
				return fmt.Errorf("Expected Batter 1 RBI to be 0, got %s", batter1RBI)
			}
			if batter2RBI != "2" {
				return fmt.Errorf("Expected Batter 2 RBI to be 2, got %s", batter2RBI)
			}
			t.Log("Player RBIs verified")
			return nil
		}),
	)
}
