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
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestProfileDeleteAll(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	runStep(t, ctx, "Login and Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "ProfileTestAway", "ProfileTestHome")
			return err
		}),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Navigate to Profile",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.WaitVisible("#sidebar-btn-profile"),
		JSClick("#sidebar-btn-profile"),
		chromedp.WaitVisible("#profile-view"),
	)

	runStep(t, ctx, "Verify Stats and Delete Remote Data",
		chromedp.WaitVisible("#profile-remote-games"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var count string
			if err := chromedp.Text("#profile-remote-games", &count).Do(ctx); err != nil {
				return err
			}
			// Should be at least 1
			if count == "0" || count == "--" {
				return fmt.Errorf("Expected > 0 remote games, got %s", count)
			}
			return nil
		}),
		// Click Delete
		chromedp.Click("#btn-profile-delete-remote"),
		// First Confirmation
		chromedp.WaitVisible("#custom-confirm-modal"),
		chromedp.Click("#btn-confirm-yes"),
		// Wait for second confirmation text to appear (modal might stay open or toggle fast)
		chromedp.WaitVisible(`//p[contains(text(), "confirm one last time")]`, chromedp.BySearch),
		chromedp.Click("#btn-confirm-yes"),
		// Success Alert text
		chromedp.WaitVisible(`//p[contains(text(), "You will now be logged out")]`, chromedp.BySearch),
		chromedp.Click("#btn-confirm-yes"), // Acknowledge success
		waitUntilDisplayNone("#custom-confirm-modal"),
	)

	runStep(t, ctx, "Verify Logout State",

		chromedp.WaitVisible("#profile-view"), // App reloads to same hash

		chromedp.WaitVisible("#profile-email"),

		chromedp.ActionFunc(func(ctx context.Context) error {

			var email string

			if err := chromedp.Text("#profile-email", &email).Do(ctx); err != nil {

				return err

			}
			if email != "Guest User" {
				return fmt.Errorf("Expected 'Guest User' after logout, got '%s'", email)
			}
			return nil
		}),
		chromedp.ActionFunc(OpenSidebar),
		chromedp.WaitVisible("#btn-login"), // Login button should be visible in sidebar
	)
}
