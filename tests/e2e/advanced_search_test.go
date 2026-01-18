package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestAdvancedSearch(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 180*time.Second) // Generous timeout
	defer cancel()

	runStep(t, ctx, "App Init",
		chromedp.Sleep(2*time.Second),
		DisableCSSAnimations(),
	)

	runStep(t, ctx, "Login",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, baseURL)
		}),
	)

	runStep(t, ctx, "Create Game 1",
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := CreateGameWithMetadata(ctx, "World Series", "Stadium A", "Team A", "Team B")
			return err
		}),
		chromedp.Sleep(1*time.Second),
	)

	runStep(t, ctx, "Nav to Dashboard 1",
		chromedp.ActionFunc(NavToDashboard),
	)

	runStep(t, ctx, "Create Game 2",
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := CreateGameWithMetadata(ctx, "Playoff", "Stadium B", "Team C", "Team D")
			return err
		}),
		chromedp.Sleep(1*time.Second),
	)

	runStep(t, ctx, "Nav to Dashboard 2",
		chromedp.ActionFunc(NavToDashboard),
	)

	t.Run("TogglePanel", func(t *testing.T) {
		runStep(t, ctx, "Toggle Panel",
			chromedp.WaitVisible("#btn-toggle-advanced-search"),
			chromedp.Click("#btn-toggle-advanced-search"),
			chromedp.WaitVisible("#advanced-search-panel"),
		)
	})

	t.Run("FilterByEvent", func(t *testing.T) {
		runStep(t, ctx, "Filter By Event",
			chromedp.ActionFunc(EnsureAdvancedPanelOpen),
			chromedp.SendKeys("#adv-search-event", "Series"),
			chromedp.Click("#btn-adv-apply"),
			chromedp.Sleep(1*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				var text string
				if err := chromedp.Text("#game-list", &text).Do(ctx); err != nil {
					return err
				}
				if !strings.Contains(text, "World Series") {
					return CustomError("Expected 'World Series' in results")
				}
				if strings.Contains(text, "Playoff") {
					return CustomError("Did not expect 'Playoff' in results")
				}
				return nil
			}),
		)
	})

	t.Run("FilterByLocation", func(t *testing.T) {
		runStep(t, ctx, "Filter By Location",
			chromedp.ActionFunc(EnsureAdvancedPanelOpen),
			chromedp.Click("#btn-adv-clear"),
			chromedp.Sleep(1*time.Second),
			chromedp.SendKeys("#adv-search-location", "Stadium B"),
			chromedp.Click("#btn-adv-apply"),
			chromedp.Sleep(1*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				var text string
				if err := chromedp.Text("#game-list", &text).Do(ctx); err != nil {
					return err
				}
				if !strings.Contains(text, "Playoff") {
					return CustomError("Expected 'Playoff' (Stadium B) in results")
				}
				if strings.Contains(text, "World Series") {
					return CustomError("Did not expect 'World Series' (Stadium A) in results")
				}
				return nil
			}),
		)
	})

	t.Run("IsLocalFlag", func(t *testing.T) {
		runStep(t, ctx, "Filter By is:local",
			chromedp.ActionFunc(EnsureAdvancedPanelOpen),
			chromedp.Click("#btn-adv-clear"),
			chromedp.Sleep(1*time.Second),
			chromedp.Click("#adv-search-local"),
			chromedp.Click("#btn-adv-apply"),
			chromedp.Sleep(1*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				var query string
				if err := chromedp.Value("#dashboard-search", &query).Do(ctx); err != nil {
					return err
				}
				if !strings.Contains(query, "is:local") {
					return CustomError("Search box should contain is:local")
				}
				return nil
			}),
		)
	})

	t.Run("ClearSearch", func(t *testing.T) {
		runStep(t, ctx, "Clear Search",
			chromedp.Click("#btn-adv-clear"),
			chromedp.Sleep(1*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				var text string
				if err := chromedp.Text("#game-list", &text).Do(ctx); err != nil {
					return err
				}
				if !strings.Contains(text, "World Series") || !strings.Contains(text, "Playoff") {
					return CustomError("Expected both games after clear")
				}
				return nil
			}),
		)
	})
}

func CreateGameWithMetadata(ctx context.Context, event, location, teamAway, teamHome string) (string, error) {
	var gameID string
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(OpenNewGameModal),
		chromedp.WaitVisible(`#game-event-input`),
		chromedp.SetValue(`#game-event-input`, event),
		chromedp.SetValue(`#game-location-input`, location),
		chromedp.SetValue(`#team-away-input`, teamAway),
		chromedp.SetValue(`#team-home-input`, teamHome),
		chromedp.Click(`#btn-start-new-game`),
		chromedp.WaitVisible(`#scoresheet-view`),
		chromedp.Evaluate(`window.location.hash.substring(6)`, &gameID),
	)
	return gameID, err
}

func NavToDashboard(ctx context.Context) error {
	// Manually open sidebar to wait for backdrop instead of app-sidebar
	return chromedp.Run(ctx,
		chromedp.Click("#btn-menu-scoresheet", chromedp.NodeVisible),
		chromedp.WaitVisible("#sidebar-backdrop"),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Click("#sidebar-btn-dashboard"),
		chromedp.WaitVisible("#game-list-container"),
		chromedp.Sleep(1*time.Second),
	)
}

func EnsureAdvancedPanelOpen(ctx context.Context) error {
	var isHidden bool
	err := chromedp.Evaluate(`document.getElementById('advanced-search-panel').classList.contains('hidden')`, &isHidden).Do(ctx)
	if err != nil {
		return err
	}
	if isHidden {
		return chromedp.Click("#btn-toggle-advanced-search").Do(ctx)
	}
	return nil
}

type CustomError string

func (e CustomError) Error() string { return string(e) }
