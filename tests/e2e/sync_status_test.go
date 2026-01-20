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

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func TestSyncStatusIndicators(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx, chromedp.WithLogf(log.Printf))
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	gameID := ""

	// Scenario A: Online Sync
	runStep(t, ctx, "Scenario A: Online Sync",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, baseURL)
		}),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		DisableCSSAnimations(),
		// Create Game
		chromedp.WaitVisible("#btn-new-game"),
		chromedp.Evaluate(`document.getElementById('btn-new-game').click()`, nil),
		chromedp.WaitVisible("#team-away-input"),
		chromedp.SetValue("#team-away-input", "Away A"),
		chromedp.SetValue("#team-home-input", "Home A"),
		chromedp.Click("#btn-start-new-game"),
		chromedp.WaitVisible("#scoresheet-view"),
		// Extract Game ID
		chromedp.ActionFunc(func(ctx context.Context) error {
			var hash string
			if err := chromedp.Evaluate("window.location.hash", &hash).Do(ctx); err != nil {
				return err
			}
			gameID = hash[6:] // remove #game/
			return nil
		}),
		// Go to Dashboard
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Click("#sidebar-btn-dashboard"),
		chromedp.WaitVisible("#game-list"),
		// Check Status: Synced (Green check)
		chromedp.WaitVisible(fmt.Sprintf(`//div[contains(@class, 'bg-white')]//h3[contains(text(), 'Away A')]`), chromedp.BySearch),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Find the card for this game and check button text
			return chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const card = document.querySelector('div[data-game-id="%s"]');
					if (!card) throw "Card not found";
					const btn = card.querySelector('button');
					if (!btn) throw "Sync button not found";
					if (btn.textContent !== '✅') throw "Expected ✅, got " + btn.textContent;
				})()
			`, gameID), nil).Do(ctx)
		}),
	)

	// Scenario B: Offline Edits
	runStep(t, ctx, "Scenario B: Offline Edits",
		network.Enable(),
		// Go Offline
		network.EmulateNetworkConditions(true, 0, 0, 0),
		// Navigate back to the game while offline
		chromedp.Navigate(baseURL+"#game/"+gameID),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		// Edit Game metadata via dispatch (will trigger saveGame with dirty=true)
		chromedp.Evaluate(fmt.Sprintf(`window.app.dispatch({type: 'GAME_METADATA_UPDATE', payload: {id: "%s", location: 'Offline Loc'}})`, gameID), nil),
		chromedp.Sleep(1*time.Second), // Wait for save
		// Go to Dashboard via hash
		chromedp.Evaluate(`window.location.hash = ''`, nil),
		chromedp.WaitVisible("#dashboard-view"),
		// Check Status: Unsynced (☁️⬆️)
		// Note: Since we are offline and remote buffer is empty, it appears as Local Only (☁️⬆️).
		chromedp.ActionFunc(func(ctx context.Context) error {
			var icon string
			err := chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const card = document.querySelector('div[data-game-id="%s"]');
					if (!card) throw "Card not found";
					const btn = card.querySelector('button');
					if (!btn) throw "Btn not found";
					return btn.textContent;
				})()
			`, gameID), &icon).Do(ctx)
			if err != nil {
				return err
			}
			if !strings.Contains(icon, "☁️⬆️") {
				return fmt.Errorf("Expected ☁️⬆️, got %q", icon)
			}
			return nil
		}),
	)

	// Scenario C: Reconnect and Sync
	runStep(t, ctx, "Scenario C: Reconnect",
		network.EmulateNetworkConditions(false, 0, 0, 0),
		// Trigger Sync
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const card = document.querySelector('div[data-game-id="%s"]');
					const btn = card.querySelector('button');
					btn.click(); // Trigger sync
				})()
			`, gameID), nil).Do(ctx)
		}),
		chromedp.WaitVisible(fmt.Sprintf(`//div[@data-game-id='%s']//button[contains(., '✅')]`, gameID), chromedp.BySearch),
		// Check Status: Synced (✅)
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const card = document.querySelector('div[data-game-id="%s"]');
					const btn = card.querySelector('button');
					if (btn.textContent !== '✅') throw "Expected ✅, got " + btn.textContent;
				})()
			`, gameID), nil).Do(ctx)
		}),
	)

	// Scenario D: Logged Out
	runStep(t, ctx, "Scenario D: Logged Out",
		// Logout
		chromedp.Evaluate(`window.app.auth.logout()`, nil),
		chromedp.WaitVisible("#btn-login"),
		// Check Dashboard: Buttons should be GONE
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const card = document.querySelector('div[data-game-id="%s"]');
					if (!card) throw "Card not found";
					const btn = card.querySelector('button');
					if (btn) throw "Sync button should be hidden for unauthenticated users";
				})()
			`, gameID), nil).Do(ctx)
		}),
	)
}
