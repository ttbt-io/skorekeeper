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
            // 1. Inject 60 Games
            // We create enough games to ensure they don't all fit in one batch/screen
            for (let i = 1; i <= 60; i++) {
                // Date logic: g1 is newest (Jan 1 + 60 days), g60 is oldest
                const date = new Date('2025-01-01');
                date.setDate(date.getDate() + (60 - i));
                
                await window.app.db.saveGame({
                    id: 'g' + i,
                    date: date.toISOString(),
                    away: 'Away ' + i,
                    home: 'Home ' + i,
                    actionLog: []
                });
            }
            
            // 2. Force Offline Mode (simulate failure) by mocking fetch
            window.app.sync.fetchGameList = async () => { throw new Error('Offline'); };
            
            // 3. Set Display Limit (optional)
            // window.app.dashboardController.displayLimit = 10;
            
            // 4. Reload Dashboard
            await window.app.dashboardController.loadDashboard();
		})()`, nil),
	)

	runStep(t, ctx, "Verify Pagination (Initial Load)",
		chromedp.WaitVisible(`div[data-game-id="g1"]`),  // Newest
		chromedp.WaitVisible(`div[data-game-id="g10"]`), // 10th
		chromedp.ActionFunc(func(ctx context.Context) error {
			var exists bool
			// g60 (oldest) should NOT be visible yet
			err := chromedp.Evaluate(`!!document.querySelector('div[data-game-id="g60"]')`, &exists).Do(ctx)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("Game g60 should not be visible on initial load")
			}
			return nil
		}),
	)

	runStep(t, ctx, "Scroll to Load More",
		chromedp.Evaluate(`
            const container = document.getElementById('game-list-container');
            container.scrollTo(0, container.scrollHeight);
        `, nil),
		// Wait for more games to appear. We check for g20 or just count increase.
		// Let's wait for g15
		chromedp.WaitVisible(`div[data-game-id="g15"]`),
	)

	runStep(t, ctx, "Search",
		chromedp.SendKeys(`#dashboard-search`, "Home 60"),
		chromedp.Sleep(1000*time.Millisecond), // Wait for debounce
		chromedp.WaitVisible(`div[data-game-id="g60"]`),
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
