package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func TestClientSidePaginationAndFiltering(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	runStep(t, ctx, "Clear State",
		network.ClearBrowserCookies(),
		chromedp.Navigate(baseURL),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		chromedp.Evaluate(`(async () => {
			if (window.app && window.app.db) {
				window.app.db.close();
			}
			const req = indexedDB.deleteDatabase('scorekeeper');
			await new Promise((resolve, reject) => {
				req.onsuccess = resolve;
				req.onerror = resolve; // Ignore errors, just try
				req.onblocked = resolve;
			});
		})()`, nil),
		// Reload to re-init DB
		chromedp.Navigate(baseURL),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
	)

	runStep(t, ctx, "Inject Data and Set Limit",
		chromedp.Evaluate(`(async () => {
            // 1. Inject 3 Games
            const games = [
                { id: 'g1', date: '2025-01-03T12:00:00Z', away: 'Cherry', home: 'Team C' },
                { id: 'g2', date: '2025-01-02T12:00:00Z', away: 'Banana', home: 'Team B' },
                { id: 'g3', date: '2025-01-01T12:00:00Z', away: 'Apple',  home: 'Team A' }
            ];
            for (const g of games) {
                await window.app.db.saveGame({
                    id: g.id,
                    date: g.date,
                    away: g.away,
                    home: g.home,
                    actionLog: []
                });
            }
            
            // 2. Force Offline Mode (simulate failure) by mocking
            window.app.sync.fetchGameList = async () => { throw new Error('Offline'); };
            
            // 3. Set Low Limit
            window.app.dashboardController.limit = 2;
            
            // 4. Reload Dashboard
            await window.app.dashboardController.loadDashboard();
		})()`, nil),
	)

	runStep(t, ctx, "Verify Page 1",
		chromedp.WaitVisible(`div[data-game-id="g1"]`),
		chromedp.WaitVisible(`div[data-game-id="g2"]`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var exists bool
			err := chromedp.Evaluate(`!!document.querySelector('div[data-game-id="g3"]')`, &exists).Do(ctx)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("Game g3 should not be visible on page 1")
			}
			return nil
		}),
	)

	runStep(t, ctx, "Load More",
		// Click the button that contains text "Load More"
		chromedp.Click(`//button[contains(text(), 'Load More')]`, chromedp.BySearch),
		chromedp.WaitVisible(`div[data-game-id="g3"]`),
	)

	runStep(t, ctx, "Search",
		chromedp.SendKeys(`#dashboard-search`, "Apple"),
		chromedp.Sleep(1000*time.Millisecond), // Wait for debounce
		chromedp.WaitVisible(`div[data-game-id="g3"]`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var count int
			// Count visible game cards
			err := chromedp.Evaluate(`document.querySelectorAll('div[data-game-id]').length`, &count).Do(ctx)
			if err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("Expected 1 game after search, found %d", count)
			}
			return nil
		}),
	)
}
