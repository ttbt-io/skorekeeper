package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestTeamSearch(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
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

	runStep(t, ctx, "Nav to Teams",
		chromedp.ActionFunc(OpenSidebar),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.WaitVisible(`#sidebar-btn-teams`),
		chromedp.Click(`#sidebar-btn-teams`), // standard click usually works if visible
		chromedp.WaitVisible(`#teams-view`),
		chromedp.Sleep(500*time.Millisecond),
	)

	runStep(t, ctx, "Create Teams",
		chromedp.ActionFunc(func(ctx context.Context) error {
			teams := []string{"Alpha Squad", "Beta Blockers", "Gamma Rays"}
			for _, name := range teams {
				if err := CreateTeam(ctx, name); err != nil {
					return err
				}
			}
			return nil
		}),
	)

	runStep(t, ctx, "Basic Search",
		chromedp.SendKeys(`#teams-search`, "Alpha"),
		chromedp.Sleep(1000*time.Millisecond), // Wait for debounce
		chromedp.ActionFunc(func(ctx context.Context) error {
			var text string
			if err := chromedp.Text(`#teams-list`, &text).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(text, "Alpha Squad") {
				return CustomError("Expected 'Alpha Squad'")
			}
			if strings.Contains(text, "Beta Blockers") {
				return CustomError("Did not expect 'Beta Blockers'")
			}
			return nil
		}),
	)

	runStep(t, ctx, "Clear Search",
		chromedp.Evaluate(`
			var input = document.getElementById('teams-search');
			input.value = '';
			input.dispatchEvent(new Event('input', { bubbles: true }));
		`, nil),
		chromedp.Sleep(1000*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var text string
			if err := chromedp.Text(`#teams-list`, &text).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(text, "Beta Blockers") {
				return CustomError("Expected 'Beta Blockers' after clear")
			}
			return nil
		}),
	)

	runStep(t, ctx, "Advanced Search Panel",
		chromedp.Click(`#btn-toggle-teams-advanced-search`),
		chromedp.WaitVisible(`#teams-advanced-search-panel`),
		chromedp.SendKeys(`#teams-adv-search-name`, "Gamma"),
		chromedp.Click(`#btn-teams-adv-apply`),
		chromedp.Sleep(1000*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var text string
			if err := chromedp.Text(`#teams-list`, &text).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(text, "Gamma Rays") {
				return CustomError("Expected 'Gamma Rays'")
			}
			if strings.Contains(text, "Alpha Squad") {
				return CustomError("Did not expect 'Alpha Squad'")
			}
			// Check that search box was updated
			var query string
			if err := chromedp.Value(`#teams-search`, &query).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(query, "name:Gamma") && !strings.Contains(query, "Gamma") {
				// buildTeamsAdvancedQuery uses filters, so name:Gamma
				return CustomError("Search box not updated correctly: " + query)
			}
			return nil
		}),
	)
}

func CreateTeam(ctx context.Context, name string) error {
	return chromedp.Run(ctx,
		chromedp.Click(`#btn-create-team`),
		chromedp.WaitVisible(`#team-modal`),
		chromedp.SetValue(`#team-name`, name),
		chromedp.Click(`#btn-save-team`),
		chromedp.WaitNotVisible(`#team-modal`),
		chromedp.Sleep(200*time.Millisecond),
	)
}
