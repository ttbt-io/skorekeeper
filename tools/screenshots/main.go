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

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/chromedp/chromedp"
	"github.com/ttbt-io/skorekeeper/backend"
	"github.com/ttbt-io/skorekeeper/tools/e2ehelpers"
)

var (
	chromeURL = flag.String("chrome-url", "", "The url of the remote debugging port")
	outputDir = flag.String("output-dir", "/screenshots", "Directory to save screenshots")
)

func main() {
	flag.Parse()

	if *chromeURL == "" {
		log.Fatal("--chrome-url must be set")
	}

	baseURL := startServer()
	log.Printf("Server started at %s", baseURL)

	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), *chromeURL)
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx, chromedp.WithLogf(log.Printf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 180*time.Second) // very generous timeout
	defer cancel()

	// Ensure output dir exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output dir: %v", err)
	}

	log.Println("Starting screenshot generation...")

	if err := generateScreenshots(ctx, baseURL); err != nil {
		log.Fatalf("Failed to generate screenshots: %v", err)
	}

	if err := generateManualImages(ctx, baseURL); err != nil {
		log.Fatalf("Failed to generate manual images: %v", err)
	}

	log.Println("Screenshots generated successfully.")
}

func debugFailure(ctx context.Context, name string) {
	log.Printf("DEBUG: capturing failure info for %s", name)
	var htmlContent string
	if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &htmlContent)); err != nil {
		log.Printf("DEBUG: Failed to capture HTML: %v", err)
	} else {
		log.Printf("DEBUG: HTML Dump for %s:\n%s", name, htmlContent)
	}

	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err == nil {
		os.WriteFile(filepath.Join(*outputDir, fmt.Sprintf("debug-%s.png", name)), buf, 0644)
		log.Printf("DEBUG: Saved screenshot to debug-%s.png", name)
	} else {
		log.Printf("DEBUG: Failed to capture screenshot: %v", err)
	}
}

// runAction executes a chromedp action with a timeout and debug capture on failure.
func runAction(ctx context.Context, name string, action chromedp.Action, timeout time.Duration) error {
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- chromedp.Run(stepCtx, action)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			log.Printf("Action '%s' failed: %v", name, err)
			debugFailure(ctx, name+"-failed")
			return err
		}
		return nil
	case <-stepCtx.Done():
		log.Printf("Action '%s' timed out", name)
		debugFailure(ctx, name+"-timeout")
		return stepCtx.Err()
	}
}

func generateManualImages(ctx context.Context, baseURL string) error {
	manualDir := *outputDir

	// 1. Fresh Game for Manual Examples
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL+"/")); err != nil {
		return err
	}
	gameID, err := e2ehelpers.CreateGame(ctx, "ManualAway", "ManualHome")
	if err != nil {
		return err
	}
	log.Printf("Manual: Created game %s", gameID)

	// Helper to capture a specific cell
	captureCell := func(playerIdx, colIdx int, filename string) error {
		selector := fmt.Sprintf(`.grid-cell[data-player-idx="%d"][data-col-id="col-%d-0"]`, playerIdx, colIdx)
		var buf []byte
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(selector),
			chromedp.ScrollIntoView(selector),
			chromedp.Sleep(200*time.Millisecond), // Wait for scroll to settle
			chromedp.Screenshot(selector, &buf),
		); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(manualDir, filename), buf, 0644)
	}

	log.Println("Manual: Empty Cell")
	if err := captureCell(1, 2, "cell-empty.png"); err != nil {
		return err
	}

	log.Println("Manual: Strikeout (Looking)")
	if err := e2ehelpers.SelectCell(ctx, 1, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`(async () => {
			try {
				await app.recordPitch('strike');
				await app.recordPitch('strike');
				await app.recordPitch('strike', 'Called');
				app.closeCSO();
			} catch(e) { console.error('JS Error in recordPitch:', e); }
		})()`, nil),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := captureCell(0, 1, "cell-strikeout.png"); err != nil {
		return err
	}

	log.Println("Manual: Walk")
	if err := e2ehelpers.SelectCell(ctx, 2, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`(async () => {
			try {
				for(let i=0; i<4; i++) await app.recordPitch('ball');
				app.closeCSO();
			} catch(e) { console.error('JS Error in recordPitch:', e); }
		})()`, nil),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := captureCell(1, 1, "cell-walk.png"); err != nil {
		return err
	}

	log.Println("Manual: Single")
	if err := e2ehelpers.SelectCell(ctx, 3, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "1B"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click(".pos-key[data-pos=\"8\"]"),
		chromedp.Click("#btn-save-bip"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		return err
	}
	if err := captureCell(2, 1, "cell-single.png"); err != nil {
		return err
	}

	log.Println("Manual: Double")
	if err := e2ehelpers.SelectCell(ctx, 4, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "2B"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click(".pos-key[data-pos=\"7\"]"),
		chromedp.Click("#btn-save-bip"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		return err
	}
	if err := captureCell(3, 1, "cell-double.png"); err != nil {
		return err
	}

	log.Println("Manual: Homerun")
	if err := e2ehelpers.SelectCell(ctx, 5, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "Home"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click(".pos-key[data-pos=\"9\"]"),
		chromedp.Click("#btn-save-bip"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		return err
	}
	if err := captureCell(4, 1, "cell-homerun.png"); err != nil {
		return err
	}

	log.Println("Manual: Fly Out")
	if err := e2ehelpers.SelectCell(ctx, 6, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Fly"),
		e2ehelpers.CycleTo(nil, "#btn-type", "OUT"),
		chromedp.Click(".pos-key[data-pos=\"8\"]"),
		chromedp.Click("#btn-save-bip"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		return err
	}
	if err := captureCell(5, 1, "cell-flyout.png"); err != nil {
		return err
	}

	log.Println("Manual: Ground Out")
	if err := e2ehelpers.SelectCell(ctx, 7, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Ground"),
		e2ehelpers.CycleTo(nil, "#btn-type", "OUT"),
		chromedp.Click(".pos-key[data-pos=\"6\"]"),
		chromedp.Click(".pos-key[data-pos=\"3\"]"),
		chromedp.Click("#btn-save-bip"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		return err
	}
	if err := captureCell(6, 1, "cell-groundout.png"); err != nil {
		return err
	}

	log.Println("Manual: Trajectory Examples")
	// Use the Home team slots for these to avoid creating a new game
	if err := e2ehelpers.SwitchToTeam(ctx, "home"); err != nil {
		return err
	}
	// Stability wait to ensure team switch is complete and state is settled
	time.Sleep(1 * time.Second)

	recordTrajectory := func(slot int, res, base, playType, traj, filename string) error {
		if err := e2ehelpers.SelectCell(ctx, slot, 1); err != nil {
			debugFailure(ctx, fmt.Sprintf("traj-%s-select", filename))
			return err
		}
		xFact, yFact := 0.4, 0.35
		if traj == "P" || traj == "L" {
			xFact, yFact = 0.45, 0.5
		}
		if err := runAction(ctx, fmt.Sprintf("recordTrajectory-%s", filename), chromedp.Tasks{
			chromedp.Click("#btn-show-bip"),
			chromedp.WaitVisible("#cso-bip-view"),
			e2ehelpers.CycleTo(nil, "#btn-res", res),
			e2ehelpers.CycleTo(nil, "#btn-base", base),
			e2ehelpers.CycleTo(nil, "#btn-type", playType),
			chromedp.Click("#btn-loc"),
			chromedp.Sleep(200 * time.Millisecond),
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const el = document.querySelector('.field-svg-keyboard > svg');
					const rect = el.getBoundingClientRect();
					const x = rect.left + rect.width * %.2f;
					const y = rect.top + rect.height * %.2f;
					el.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: x, clientY: y }));
				})()
			`, xFact, yFact), nil),
			e2ehelpers.CycleTo(nil, "#btn-traj", traj),
			chromedp.Click("#btn-save-bip"),
			chromedp.ActionFunc(func(c context.Context) error {
				return e2ehelpers.FinishTurn(c)
			}),
			chromedp.ActionFunc(func(c context.Context) error {
				return e2ehelpers.WaitForSync(c)
			}),
		}, 30*time.Second); err != nil {
			return err
		}
		return captureCell(slot-1, 1, filename)
	}

	// Batter 1: Grounder Hit
	if err := recordTrajectory(1, "Safe", "1B", "HIT", "G", "cell-grounder.png"); err != nil {
		return err
	}
	// Batter 2: Line Drive Hit
	if err := recordTrajectory(2, "Safe", "1B", "HIT", "L", "cell-linedrive.png"); err != nil {
		return err
	}
	// Batter 3: Fly Ball Hit
	if err := recordTrajectory(3, "Safe", "1B", "HIT", "F", "cell-flyball.png"); err != nil {
		return err
	}
	// Batter 4: Pop Fly Hit
	if err := recordTrajectory(4, "Fly", "1B", "OUT", "P", "cell-popfly.png"); err != nil {
		return err
	}

	log.Println("Manual: Runner Advance")
	if err := e2ehelpers.SwitchToTeam(ctx, "away"); err != nil {
		return err
	}
	// We've used up to slot 7 in Away team. Let's use slot 8 & 9.
	// Batter 8: Single
	if err := e2ehelpers.SelectCell(ctx, 8, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "1B"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click("#btn-save-bip"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		return err
	}

	// Batter 9: Double (Advances Batter 8 to 3rd)
	if err := e2ehelpers.SelectCell(ctx, 9, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "2B"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click("#btn-save-bip"),
		chromedp.WaitVisible("#cso-runner-advance-view"),
		e2ehelpers.CycleTo(nil, "#btn-adv-0", "To 3rd"),
		chromedp.Click("#btn-finish-turn"),
		e2ehelpers.WaitUntilDisplayNone("#cso-modal"),
	); err != nil {
		return err
	}
	if err := captureCell(7, 1, "cell-advance.png"); err != nil {
		return err
	}

	return nil
}

func generateScreenshots(ctx context.Context, baseURL string) error {
	if err := e2ehelpers.Login(ctx, baseURL); err != nil {
		return err
	}
	if err := chromedp.Run(ctx, e2ehelpers.DisableCSSAnimations()); err != nil {
		return err
	}

	if err := captureBasicUI(ctx); err != nil {
		return err
	}

	if err := captureTeams(ctx); err != nil {
		return err
	}

	if err := captureStats(ctx); err != nil {
		return err
	}

	if err := capturePitching(ctx, baseURL); err != nil {
		return err
	}
	if err := captureHits(ctx, baseURL); err != nil {
		return err
	}
	if err := captureOuts(ctx, baseURL); err != nil {
		return err
	}
	if err := captureBaseRunning(ctx, baseURL); err != nil {
		return err
	}
	if err := captureUnusual(ctx, baseURL); err != nil {
		return err
	}
	if err := captureCycleOptions(ctx); err != nil {
		return err
	}
	if err := captureBroadcast(ctx, baseURL); err != nil {
		return err
	}
	if err := captureConflicts(ctx, baseURL); err != nil {
		return err
	}

	return nil
}

func captureBasicUI(ctx context.Context) error {
	log.Println("Capturing: Sidebar")
	if err := runAction(ctx, "capture-sidebar", chromedp.Tasks{
		chromedp.ActionFunc(e2ehelpers.OpenSidebar),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.CaptureScreenshot(&[]byte{}),
	}, 20*time.Second); err != nil {
		return err
	}

	// Capture only the sidebar element
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.Screenshot("#app-sidebar", &buf)); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*outputDir, "sidebar.png"), buf, 0644); err != nil {
		return err
	}
	log.Printf("Saved %s", filepath.Join(*outputDir, "sidebar.png"))

	if err := chromedp.Run(ctx,
		chromedp.Click("#sidebar-backdrop"),
		chromedp.Sleep(200*time.Millisecond),
	); err != nil {
		return err
	}

	log.Println("Capturing: New Game Modal")
	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(e2ehelpers.OpenNewGameModal),
		chromedp.SetValue("#team-away-input", "Tigers"),
		chromedp.SetValue("#team-home-input", "Lions"),
		chromedp.SetValue("#game-event-input", "Spring Training"),
		chromedp.SetValue("#game-location-input", "Lakeland"),
		chromedp.Sleep(200*time.Millisecond),
	); err != nil {
		return err
	}
	if err := captureScreenshot(ctx, "new-game.png"); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-start-new-game"),
		chromedp.WaitVisible("#scoresheet-view"),
	); err != nil {
		return err
	}

	log.Println("Capturing: Scoresheet View")
	if err := captureScreenshot(ctx, "scoresheet.png"); err != nil {
		return err
	}

	log.Println("Capturing: Edit Lineup Modal")
	if err := chromedp.Run(ctx,
		e2ehelpers.RightClick("#tab-away"),
		chromedp.WaitVisible("#edit-lineup-modal"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := captureScreenshot(ctx, "edit-lineup.png"); err != nil {
		return err
	}
	if err := chromedp.Run(ctx, chromedp.Click("#btn-cancel-lineup")); err != nil {
		return err
	}

	log.Println("Capturing: Dashboard with Sync Status")
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`window.location.hash = ""`, nil),
		chromedp.WaitVisible("#dashboard-view"),
	); err != nil {
		return err
	}

	// Create another game to have multiple cards
	if _, err := e2ehelpers.CreateGame(ctx, "Dragons", "Eagles"); err != nil {
		return err
	}

	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`window.location.hash = ""`, nil),
		chromedp.WaitVisible("#dashboard-view"),
		// Inject some sync status icons for the screenshot
		chromedp.Evaluate(`
			const cards = document.querySelectorAll('.game-card');
			if (cards.length >= 2) {
				const s1 = cards[0].querySelector('.sync-status-icon');
				if (s1) s1.textContent = '✅';
				const s2 = cards[1].querySelector('.sync-status-icon');
				if (s2) s2.textContent = '⚠️';
			}
		`, nil),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		return err
	}
	if err := captureScreenshot(ctx, "dashboard.png"); err != nil {
		return err
	}

	return nil
}

func captureTeams(ctx context.Context) error {
	log.Println("Capturing: Team Management")
	if err := runAction(ctx, "capture-teams", chromedp.Tasks{
		chromedp.Evaluate(`window.location.hash = "teams"`, nil),
		chromedp.WaitVisible("#teams-view"),
		chromedp.Click("#btn-create-team"),
		chromedp.WaitVisible("#team-modal"),
		chromedp.SetValue("#team-name", "All-Stars"),
		chromedp.SetValue("#team-short-name", "AS"),
		chromedp.Click("#tab-team-members"),
		chromedp.WaitVisible("#team-members-container"),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "team-members.png")
		}),
		chromedp.Click("#btn-cancel-team"),
	}, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func captureStats(ctx context.Context) error {
	log.Println("Capturing: Statistics")
	if err := runAction(ctx, "capture-stats", chromedp.Tasks{
		chromedp.Evaluate(`window.location.hash = "stats"`, nil),
		chromedp.WaitVisible("#statistics-view"),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "statistics.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func capturePitching(ctx context.Context, baseURL string) error {
	log.Println("Capturing: CSO Pitch View")
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL+"/")); err != nil {
		return err
	}
	if _, err := e2ehelpers.CreateGame(ctx, "PitchAway", "PitchHome"); err != nil {
		return err
	}
	if err := e2ehelpers.SelectCell(ctx, 1, 1); err != nil {
		return err
	}
	if err := captureScreenshot(ctx, "cso-pitch.png"); err != nil {
		return err
	}

	log.Println("Capturing: Strikeout Sequence")
	// Use runAction for the strikeout sequence
	if err := runAction(ctx, "capture-strikeout", chromedp.Tasks{
		chromedp.Click("#btn-strike"),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.Click("#btn-strike"),
		chromedp.Sleep(200 * time.Millisecond),
		e2ehelpers.RightClick("#btn-strike"),
		chromedp.WaitVisible("#cso-long-press-submenu"),
		chromedp.Click(`//button[text()="Swinging"]`, chromedp.BySearch),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-strikeout-swinging.png")
		}),
		// Close logic inside runAction - conditionally close menu via JS to avoid blocking
		chromedp.Evaluate(`
			(() => {
				const btn = document.getElementById('btn-close-long-press-menu');
				const menu = document.getElementById('cso-long-press-submenu');
				if (btn && menu && !menu.classList.contains('hidden')) {
					btn.click();
				}
			})()
		`, nil),
		chromedp.Evaluate(`
			(() => {
				const btn = document.getElementById('btn-close-cso');
				if (btn && btn.offsetParent !== null) {
					btn.click();
				}
			})()
		`, nil),
	}, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func captureHits(ctx context.Context, baseURL string) error {
	log.Println("Starting captureHits")
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL+"/")); err != nil {
		return err
	}
	if _, err := e2ehelpers.CreateGame(ctx, "HitAway", "HitHome"); err != nil {
		return err
	}

	log.Println("Capturing: Hit Single")
	if err := e2ehelpers.SelectCell(ctx, 1, 1); err != nil {
		return err
	}
	if err := runAction(ctx, "capture-hit-single", chromedp.Tasks{
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "1B"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click(".pos-key[data-pos=\"8\"]"),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-hit-single.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	log.Println("Capturing: Homerun")
	// Step 1: Save the previous Single
	if err := runAction(ctx, "save-hit-single", chromedp.Tasks{
		chromedp.Click("#btn-save-bip"),
		chromedp.ActionFunc(func(c context.Context) error {
			return e2ehelpers.FinishTurn(c)
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	// Step 2: Setup Homerun
	if err := runAction(ctx, "setup-homerun", chromedp.Tasks{
		chromedp.ActionFunc(func(c context.Context) error {
			return e2ehelpers.SelectCell(c, 2, 1)
		}),
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "Home"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click(`.pos-key[data-pos="8"]`),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-homerun.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	// Step 3: Save Homerun
	if err := runAction(ctx, "save-homerun", chromedp.Tasks{
		chromedp.Click("#btn-save-bip"),
		chromedp.ActionFunc(func(c context.Context) error {
			return e2ehelpers.FinishTurn(c)
		}),
	}, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func captureOuts(ctx context.Context, baseURL string) error {
	log.Println("Starting captureOuts")
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL+"/")); err != nil {
		return err
	}
	if _, err := e2ehelpers.CreateGame(ctx, "OutAway", "OutHome"); err != nil {
		return err
	}

	log.Println("Capturing: Ground Out")
	if err := e2ehelpers.SelectCell(ctx, 1, 1); err != nil {
		return err
	}
	if err := runAction(ctx, "capture-ground-out", chromedp.Tasks{
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Ground"),
		e2ehelpers.CycleTo(nil, "#btn-type", "OUT"),
		chromedp.Click(".pos-key[data-pos=\"6\"]"),
		chromedp.Click(".pos-key[data-pos=\"3\"]"),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-ground-out.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	log.Println("Capturing: Fly Out")
	// Step 1: Save Ground Out
	if err := runAction(ctx, "save-ground-out", chromedp.Tasks{
		chromedp.Click("#btn-save-bip"),
		chromedp.ActionFunc(func(c context.Context) error {
			return e2ehelpers.FinishTurn(c)
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	// Step 2: Setup Fly Out
	if err := runAction(ctx, "setup-fly-out", chromedp.Tasks{
		chromedp.ActionFunc(func(c context.Context) error {
			return e2ehelpers.SelectCell(c, 2, 1)
		}),
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Fly"),
		e2ehelpers.CycleTo(nil, "#btn-type", "OUT"),
		chromedp.Click(`.pos-key[data-pos="8"]`),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-fly-out.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	// Step 3: Save Fly Out
	if err := runAction(ctx, "save-fly-out", chromedp.Tasks{
		chromedp.Click("#btn-save-bip"),
		chromedp.ActionFunc(func(c context.Context) error {
			return e2ehelpers.FinishTurn(c)
		}),
	}, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func captureBaseRunning(ctx context.Context, baseURL string) error {
	log.Println("Starting captureBaseRunning")
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL+"/")); err != nil {
		return err
	}
	if _, err := e2ehelpers.CreateGame(ctx, "BaseAway", "BaseHome"); err != nil {
		return err
	}
	log.Println("Game Created for BaseRunning")

	// Put a runner on 1st
	if err := e2ehelpers.SelectCell(ctx, 1, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "1B"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click("#btn-save-bip"),
	); err != nil {
		return err
	}
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		return err
	}

	log.Println("Capturing: Steal (Runner Actions)")
	if err := e2ehelpers.SelectCell(ctx, 2, 1); err != nil {
		return err
	}
	if err := runAction(ctx, "capture-steal", chromedp.Tasks{
		chromedp.Click("#btn-runner-actions"),
		chromedp.WaitVisible("#cso-runner-action-view"),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-steal.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	log.Println("Capturing: Runner Advance")
	if err := runAction(ctx, "capture-runner-advance", chromedp.Tasks{
		chromedp.Click("#btn-close-runner-action"),
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "1B"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click("#btn-save-bip"),
		chromedp.WaitVisible("#cso-runner-advance-view"),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-runner-advance.png")
		}),
		chromedp.ActionFunc(func(c context.Context) error {
			return e2ehelpers.FinishTurn(c)
		}),
	}, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func captureUnusual(ctx context.Context, baseURL string) error {
	log.Println("Starting captureUnusual")
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL+"/")); err != nil {
		return err
	}
	if _, err := e2ehelpers.CreateGame(ctx, "UnAway", "UnHome"); err != nil {
		return err
	}

	// Double Play
	if err := e2ehelpers.SelectCell(ctx, 1, 1); err != nil {
		return err
	}
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "1B"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Click("#btn-save-bip"),
	); err != nil {
		return err
	}
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		return err
	}
	if err := e2ehelpers.SelectCell(ctx, 2, 1); err != nil {
		return err
	}
	if err := runAction(ctx, "capture-dp", chromedp.Tasks{
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Ground"),
		e2ehelpers.CycleTo(nil, "#btn-type", "OUT"),
		chromedp.Click(".pos-key[data-pos=\"6\"]"),
		chromedp.Click(".pos-key[data-pos=\"4\"]"),
		chromedp.Click(".pos-key[data-pos=\"3\"]"),
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-dp.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	log.Println("Capturing: Dropped 3rd Strike View")
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-save-bip"),
	); err != nil {
		return err
	}
	if err := e2ehelpers.FinishTurn(ctx); err != nil {
		return err
	}
	if err := e2ehelpers.SelectCell(ctx, 3, 1); err != nil {
		return err
	}
	if err := runAction(ctx, "capture-dropped-3rd", chromedp.Tasks{
		chromedp.Click("#btn-strike"),
		chromedp.Click("#btn-strike"),
		e2ehelpers.RightClick("#btn-strike"),
		chromedp.WaitVisible("#cso-long-press-submenu"),
		chromedp.Click(`//button[text()="Dropped"]`, chromedp.BySearch),
		chromedp.WaitVisible("#cso-bip-view"),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-dropped-3rd.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	log.Println("Capturing: BOO Context Menu")
	if err := runAction(ctx, "capture-boo", chromedp.Tasks{
		chromedp.Click("#btn-cancel-bip"),
		e2ehelpers.RightClick("#btn-out"),
		chromedp.WaitVisible("#cso-long-press-submenu"),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-out-options.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}

	log.Println("Capturing: Move Play Context Menu")
	if err := runAction(ctx, "capture-move-play", chromedp.Tasks{
		chromedp.Evaluate(`
			(() => {
				const btn = document.getElementById('btn-close-cso');
				if (btn && btn.offsetParent !== null) btn.click();
			})()
		`, nil),
		e2ehelpers.WaitUntilDisplayNone("#cso-modal"),
		e2ehelpers.RightClick(`.grid-cell[data-player-idx="0"][data-col-id="col-1-0"]`),
		chromedp.WaitVisible("#column-context-menu"),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "play-context-move.png")
		}),
		chromedp.Click("#scoresheet-grid"),
	}, 30*time.Second); err != nil {
		return err
	}

	log.Println("Capturing: Correct Player in Slot")
	if err := runAction(ctx, "capture-correct-player", chromedp.Tasks{
		chromedp.Evaluate(`
			(() => {
				const btn = document.getElementById('btn-close-long-press-menu');
				if (btn && btn.offsetParent !== null) btn.click();
			})()
		`, nil),
		chromedp.Evaluate(`
			(() => {
				const btn = document.getElementById('btn-close-cso');
				if (btn && btn.offsetParent !== null) btn.click();
			})()
		`, nil),
		e2ehelpers.WaitUntilDisplayNone("#cso-modal"),
		e2ehelpers.RightClick(`.lineup-cell[data-player-idx="0"]`),
		chromedp.WaitVisible("#player-context-menu"),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "correct-batter.png")
		}),
		chromedp.Click("#scoresheet-grid"),
	}, 30*time.Second); err != nil {
		return err
	}

	return nil
}

func captureCycleOptions(ctx context.Context) error {
	log.Println("Capturing: Cycle Button Options")
	// Ensure clean state
	if err := runAction(ctx, "reset-cycle-options", chromedp.Tasks{
		chromedp.Evaluate(`
			(() => {
				const btn = document.getElementById('btn-close-cso');
				if (btn && btn.offsetParent !== null) btn.click();
			})()
		`, nil),
		e2ehelpers.WaitUntilDisplayNone("#cso-modal"),
	}, 10*time.Second); err != nil {
		return err
	}

	if err := e2ehelpers.SelectCell(ctx, 5, 1); err != nil {
		return err
	}

	if err := runAction(ctx, "capture-cycle-options", chromedp.Tasks{
		chromedp.Sleep(200 * time.Millisecond),
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		chromedp.WaitVisible("#btn-res"),
		chromedp.Sleep(200 * time.Millisecond),
		e2ehelpers.RightClick("#btn-res"),
		chromedp.WaitVisible("#options-context-menu"),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "cycle-options.png")
		}),
		chromedp.Click("#cso-modal"),
		chromedp.Click("#btn-close-cso"),
	}, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func captureBroadcast(ctx context.Context, baseURL string) error {
	log.Println("Capturing: Broadcast Overlay")
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL+"/")); err != nil {
		return err
	}
	if _, err := e2ehelpers.CreateGame(ctx, "BcastAway", "BcastHome"); err != nil {
		return err
	}
	if err := e2ehelpers.SelectCell(ctx, 1, 1); err != nil {
		return err
	}
	if err := runAction(ctx, "capture-broadcast", chromedp.Tasks{
		chromedp.Click("#btn-strike"),
		chromedp.Click("#btn-ball"),
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-base", "1B"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		chromedp.Evaluate(`
			app.state.activeGame.broadcast = {
				enabled: true,
				overlay: true,
				message: "GO TIGERS!"
			};
			app.render();
		`, nil),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "broadcast-overlay.png")
		}),
	}, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func captureConflicts(ctx context.Context, baseURL string) error {
	log.Println("Capturing: Conflict Resolution")
	// We need to trigger the conflict modal
	if err := runAction(ctx, "capture-conflict", chromedp.Tasks{
		chromedp.Navigate(baseURL + "/"),
		chromedp.Evaluate(`
			document.getElementById('conflict-resolution-modal').classList.remove('hidden');
		`, nil),
		chromedp.WaitVisible("#conflict-resolution-modal"),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			return captureScreenshot(c, "conflict-resolution.png")
		}),
		chromedp.Click("#btn-conflict-overwrite"), // Close modal
	}, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func captureScreenshot(ctx context.Context, filename string) error {
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(*outputDir, filename), buf, 0644)
}

func startServer() string {

	cert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("Failed to generate cert: %v", err)
	}
	dataDir := os.TempDir()
	s := storage.New(dataDir, nil)
	store := backend.NewGameStore(dataDir, s)
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	go backend.StartServer(backend.Options{
		Listener:    l,
		Cert:        cert,
		UseMockAuth: true,
		Debug:       false,
		GameStore:   store,
		MasterKey:   nil,
	})
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return fmt.Sprintf("https://devtest.local:%s", port)
}

func generateSelfSignedCert() (*tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Test Org"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "devtest", "devtest.local"},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.X509KeyPair(crtPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}
