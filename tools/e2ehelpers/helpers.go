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

package e2ehelpers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Logger interface allows passing *testing.T or log.Printf
type Logger interface {
	Logf(format string, args ...any)
}

// CaptureScreenshot captures a screenshot and saves it to the specified filename.
func CaptureScreenshot(ctx context.Context, filename string) error {
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("failed to create directory for screenshot: %w", err)
	}

	if err := os.WriteFile(filename, buf, 0644); err != nil {
		return fmt.Errorf("failed to write screenshot to file: %w", err)
	}
	log.Printf("Saved screenshot to %s", filename)
	return nil
}

func DisableCSSAnimations() chromedp.ActionFunc {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.Evaluate(`
                        const style = document.createElement('style');
                        style.innerHTML = '*{-webkit-transition-duration:0s!important;transition-duration:0s!important;-webkit-animation-duration:0s!important;animation-duration:0s!important;}';
                        document.head.appendChild(style);
                `, nil).Do(ctx)
	})
}

func WaitAnyVisible(sel string, match *string, timeout time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		ticker := time.NewTicker(200 * time.Millisecond) // Check frequently
		defer ticker.Stop()

		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		for {
			select {
			case <-ticker.C:
				err := chromedp.Evaluate(fmt.Sprintf(
					`(function(selectors) {
					const elements = document.querySelectorAll(selectors);
					for (let i = 0; i < elements.length; i++) {
						const el = elements[i];
						const style = window.getComputedStyle(el);
						if (el.offsetHeight !== 0 && style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0') {
							return el.tagName.toLowerCase() + (el.id ? '#' + el.id : ''); 
						}
					}
					return '';
				})('%s')`, sel), match).Do(ctx)
				if err == nil && *match != "" {
					return nil
				}
			case <-timeoutCtx.Done():
				return fmt.Errorf("timeout waiting for any element from list to become visible: %w", timeoutCtx.Err())
			}
		}
	})
}

// --- Navigation & Auth ---

// OpenSidebar clicks the hamburger menu to open the sidebar.
func OpenSidebar(ctx context.Context) error {
	var buttonSel string
	if err := chromedp.Run(ctx, WaitAnyVisible(`#btn-menu-dashboard, #btn-menu-scoresheet, #btn-menu-teams`, &buttonSel, 5*time.Second)); err != nil {
		return err
	}
	log.Printf("OpenSidebar found %s", buttonSel)
	if err := chromedp.Run(ctx, chromedp.Click(buttonSel)); err != nil {
		log.Printf("OpenSidebar click(%s): %v", buttonSel, err)
		return err
	}
	log.Print("OpenSidebar waiting for #app-sidebar")
	return chromedp.Run(ctx, chromedp.WaitVisible(`#app-sidebar`))
}

// Login performs the login flow: Open sidebar, click Login, wait for auth.
// It assumes the user is NOT logged in initially.
func Login(ctx context.Context, baseURL string) error {
	done := make(chan bool)
	defer close(done)
	go func() {
		select {
		case <-done:
			return
		case <-time.After(10 * time.Second):
			CaptureScreenshot(ctx, "/demo/debug-login-taking-too-long.png")
		}
	}()
	log.Print("Login: clearing cookies")
	if err := chromedp.Run(ctx, network.ClearBrowserCookies()); err != nil {
		return err
	}
	log.Printf("Login: opening %s/", baseURL)
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL+"/")); err != nil {
		return err
	}
	log.Print("Login: opening sidebar")
	if err := chromedp.Run(ctx, chromedp.ActionFunc(OpenSidebar)); err != nil {
		return err
	}
	if err := chromedp.Run(ctx, chromedp.Sleep(1*time.Second)); err != nil {
		return err
	}
	var sidebarHTML string
	return chromedp.Run(ctx,
		chromedp.OuterHTML(`#sidebar-auth`, &sidebarHTML),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("Sidebar Auth HTML: %s", sidebarHTML)
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Check for Login button first
			ctx1, cancel1 := context.WithTimeout(ctx, 2*time.Second)
			defer cancel1()
			if err := chromedp.Run(ctx1, chromedp.WaitVisible(`#btn-login`)); err == nil {
				log.Printf("Login: Clicking #btn-login")
				return chromedp.Click(`#btn-login`).Do(ctx)
			}
			// If not found, check for Session Expired
			log.Printf("Login: #btn-login not found, checking for #btn-session-expired")
			ctx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
			defer cancel2()
			if err := chromedp.Run(ctx2, chromedp.WaitVisible(`#btn-session-expired`)); err == nil {
				log.Printf("Login: Clicking #btn-session-expired")
				return chromedp.Click(`#btn-session-expired`).Do(ctx)
			}
			return fmt.Errorf("neither login button found")
		}),
		chromedp.Sleep(1*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("Login: Waiting for #dashboard-view")
			return nil
		}),
		chromedp.WaitVisible(`#dashboard-view`, chromedp.ByQuery),
	)
}

// LoginWithUser performs login and sets the mock user cookie.
func LoginWithUser(ctx context.Context, baseURL, email string) error {
	// Extract domain from baseURL
	// baseURL format: https://devtest.local:port or http://localhost:port
	// We need "devtest.local" or "localhost"
	parts := strings.Split(baseURL, "//")
	if len(parts) < 2 {
		return fmt.Errorf("invalid baseURL format: %s", baseURL)
	}
	hostPort := parts[1]
	host := strings.Split(hostPort, ":")[0]

	// Directly set cookie and reload to simulate login
	return chromedp.Run(ctx,
		network.ClearBrowserCookies(),
		network.SetCookie("mock_auth_user", email).
			WithDomain(host).
			WithPath("/").
			WithSecure(true),
		chromedp.Navigate(baseURL+"/"),
		chromedp.WaitVisible(`#dashboard-view`, chromedp.ByQuery),
	)
}

// Logout performs the logout flow.
func Logout(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.ActionFunc(OpenSidebar),
		chromedp.WaitVisible(`#sidebar-auth`),
		chromedp.Click(`//div[@id="sidebar-auth"]//button[text()="Logout"]`, chromedp.BySearch),
		chromedp.WaitReady(`body[data-app-ready="true"]`), // App reload
	)
}

// --- Game Management ---

// OpenNewGameModal opens the "New Game" modal from the dashboard.
func OpenNewGameModal(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(`#btn-new-game`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Animation stability
		chromedp.Click(`#btn-new-game`, chromedp.ByQuery),
		chromedp.WaitVisible(`#new-game-modal`),
	)
}

// CreateGame fills the new game form and starts the game. Returns the Game ID.
func CreateGame(ctx context.Context, teamAway, teamHome string) (string, error) {
	var gameID string
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(OpenNewGameModal),
		chromedp.WaitVisible(`#team-away-input`),
		chromedp.SetValue(`#team-away-input`, teamAway),
		chromedp.SetValue(`#team-home-input`, teamHome),
		chromedp.Click(`#btn-start-new-game`),
		chromedp.WaitVisible(`#scoresheet-view`),
		chromedp.Evaluate(`window.location.hash.substring(6)`, &gameID),
	)
	return gameID, err
}

// AddInning adds an inning to the game via the sidebar.
func AddInning(ctx context.Context) error {
	var initialColCount int
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(OpenSidebar),                                  // Ensure sidebar is open
		chromedp.WaitVisible(`#sidebar-btn-add-inning`, chromedp.ByQuery), // Wait for the button to be visible
		// Get initial column count
		chromedp.ActionFunc(func(ctx context.Context) error {
			err := chromedp.Evaluate(`document.querySelectorAll('#scoresheet-grid .grid-header').length`, &initialColCount).Do(ctx)
			if err != nil {
				log.Printf("Error getting initial column count: %v", err)
				return err
			}
			log.Printf("AddInning: Initial column count: %d", initialColCount)
			return nil
		}),
		chromedp.Evaluate(`document.getElementById('sidebar-btn-add-inning').click()`, nil), // Click using JS Evaluate
		// Wait for the column count to increase
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Poll(fmt.Sprintf(`
				(() => {
					const currentCount = document.querySelectorAll('#scoresheet-grid .grid-header').length;
					return currentCount > %d;
				})()
			`, initialColCount), nil, chromedp.WithPollingInterval(100*time.Millisecond), chromedp.WithPollingTimeout(5*time.Second)).Do(ctx)
		}),
	)
	return err
}

// SwitchToTeam clicks the tab for the specified team and waits for the app state to update.
func SwitchToTeam(ctx context.Context, teamSide string) error {
	selector := fmt.Sprintf("#tab-%s", teamSide)
	activeSelector := selector + ".active"
	return chromedp.Run(ctx,
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.WaitVisible(activeSelector, chromedp.ByQuery),
	)
}

// EditLineup updates the lineup for a team.
// players is a slice of structs {n: Name, u: Uniform}.
// We define a simple struct for players here to avoid complex imports
type PlayerInfo struct {
	N string
	U string
}

func EditLineup(ctx context.Context, teamSide string, players []PlayerInfo) error {
	selector := fmt.Sprintf(`#tab-%s`, teamSide)
	return chromedp.Run(ctx,
		chromedp.Click(selector), // Activate tab
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Context menu to open lineup editor
			return chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const el = document.querySelector('%s');
					const ev = new MouseEvent('contextmenu', { bubbles: true, cancelable: true });
				el.dispatchEvent(ev);
				})()
			`, selector), nil).Do(ctx)
		}),
		chromedp.WaitVisible(`#edit-lineup-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if len(players) > 9 {
				for i := len(players) - 9; i >= 0; i-- {
					chromedp.Click("#btn-add-starter-row").Do(ctx)
				}
			}
			// Efficiently set values via JS to avoid many round trips
			var js strings.Builder
			js.WriteString(`(() => {`)
			for i, p := range players {
				// i+1 because nth-child is 1-based
				js.WriteString(fmt.Sprintf(`
					var row = document.querySelector('#lineup-starters-container > div:nth-child(%d)');
					if (row) {
						var uIn = row.querySelector('input[name="number"]');
						var nIn = row.querySelector('input[name="name"]');
						if (uIn) { uIn.value = '%s'; uIn.dispatchEvent(new Event('input', { bubbles: true })); }
						if (nIn) { nIn.value = '%s'; nIn.dispatchEvent(new Event('input', { bubbles: true })); }
					}
				`, i+1, p.U, p.N))
			}
			js.WriteString(`})()`)
			return chromedp.Evaluate(js.String(), nil).Do(ctx)
		}),
		chromedp.Click(`#btn-save-lineup`),
		WaitUntilDisplayNone(`#edit-lineup-modal`),
	)
}

// --- Gameplay ---

// SelectCell clicks a grid cell to open the CSO.
// slot is 1-based (batter lineup order). inning is 1-based.
func SelectCell(ctx context.Context, slot, inning int) error {
	return chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var js = fmt.Sprintf(`
				(() => {
					// Calculate stride (total columns) by counting headers
					const headers = document.querySelectorAll('#scoresheet-grid > .grid-header');
					if (!headers.length) return false;
					const stride = headers.length;
					
					// Calculate index (nth-child is 1-based)
					// Header Row takes 'stride' slots.
					// Slot 1 (Row 1) starts at stride + 1.
					// In that row: 
					// 1: Name
					// 2: Inning 1
					// 3: Inning 2 ...
					// So Inning K is at offset (1 + K).
					
					// Formula: stride + (slot-1)*stride + 1 + inning
					const index = stride + (%d - 1) * stride + 1 + %d;
					
					const el = document.querySelector('#scoresheet-grid > div:nth-child(' + index + ')');
					if (el) {
						el.click();
						return true;
					}
					return false;
				})()
			`, slot, inning)
			var found bool
			if err := chromedp.Evaluate(js, &found).Do(ctx); err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("cell not found for slot %d, inning %d", slot, inning)
			}
			return nil
		}),
		chromedp.WaitVisible(`#cso-modal`),
	)
}

// RecordPitch clicks the corresponding pitch button in CSO.
// pitchType: "ball", "strike", "foul"
func RecordPitch(ctx context.Context, pitchType string) error {
	btnID := fmt.Sprintf("#btn-%s", strings.ToLower(pitchType))
	return chromedp.Run(ctx, chromedp.Click(btnID, chromedp.ByQuery))
}

// RecordBallInPlay handles the BiP flow.
// location can be empty string if not applicable.
func RecordBallInPlay(ctx context.Context, result, playType, location string) error {
	return chromedp.Run(ctx,
		chromedp.Click(`#btn-show-bip`),
		chromedp.WaitVisible(`#cso-bip-view`),
		// Set Result
		CycleTo(nil, "#btn-res", result),
		// Set Type
		CycleTo(nil, "#btn-type", playType),
		// Set Location (if provided)
		chromedp.ActionFunc(func(ctx context.Context) error {
			if location != "" {
				// Location buttons are .pos-key[data-pos="X"]
				return chromedp.Click(fmt.Sprintf(".pos-key[data-pos=\"%s\"]", location)).Do(ctx)
			}
			return nil
		}),
		chromedp.Click(`#btn-save-bip`),
	)
}

// HandleRunnerAction selects an action from the runner menu.
func HandleRunnerAction(ctx context.Context, runnerName, action string) error {
	return chromedp.Run(ctx,
		chromedp.Click(`#btn-runner-actions`),
		chromedp.WaitVisible(`#cso-runner-action-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Find index of runner in pending state
			var index int
			err := chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					// Access global app state
					return app.state.pendingRunnerState.findIndex(r => r.name === '%s');
				})()
			`, runnerName), &index).Do(ctx)
			if err != nil {
				return err
			}
			if index == -1 {
				return fmt.Errorf("runner %s not found in pending state", runnerName)
			}

			// Selector for action button: #runner-action-list > div:nth-child(index+1) > button
			selector := fmt.Sprintf(`#runner-action-list > div:nth-child(%d) > button`, index+1)

			if err := CycleTo(nil, selector, action).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
		chromedp.Click(`#btn-save-runner-actions`),
		WaitUntilDisplayNone(`#cso-runner-action-view`),
		chromedp.WaitVisible(`#cso-modal`),
	)
}

// FinishTurn clicks the Finish Turn button in Runner Advance view.
func FinishTurn(ctx context.Context) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		for {
			select {
			case <-ticker.C:
				var state string
				err := chromedp.Evaluate(
					`(function() {
					const runnerAdvanceView = document.getElementById('cso-runner-advance-view');
					const isRAVisible = runnerAdvanceView && window.getComputedStyle(runnerAdvanceView).display !== 'none';
					
					if (isRAVisible) {
						document.getElementById('btn-finish-turn').click();
						return 'clicked';
					} else {
						const csoModal = document.getElementById('cso-modal');
						const isCSOVisible = csoModal && window.getComputedStyle(csoModal).display !== 'none';
						if (!isCSOVisible) {
							return 'done';
						}
						
						// If back to Pitch View, consider turn finished
						const pitchArea = document.getElementById('action-area-pitch');
						const isPitchVisible = pitchArea && window.getComputedStyle(pitchArea).display !== 'none';
						if (isPitchVisible) {
							return 'done';
						}

						// If in "Play Recorded" view, close the modal to finish
						const recordedArea = document.getElementById('action-area-recorded');
						const isRecordedVisible = recordedArea && window.getComputedStyle(recordedArea).display !== 'none';
						if (isRecordedVisible) {
							const closeBtn = document.getElementById('btn-close-cso');
							if (closeBtn) closeBtn.click();
							return 'closing';
						}
					}
					return 'waiting';
				})()`, &state).Do(ctx)
				if err != nil {
					log.Printf("FinishTurn error: %v", err)
				}
				if err == nil && state == "done" {
					return nil
				}
			case <-timeoutCtx.Done():
				return fmt.Errorf("timeout finishing turn: %w", timeoutCtx.Err())
			}
		}
	}))
}

// SetRunnerOutcome sets the outcome for a runner in the Runner Advance view (Finish Turn screen).
func SetRunnerOutcome(ctx context.Context, runnerName, outcome string) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var index int
			err := chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					let index = -1;
				if (app.state.pendingRunnerState) {
					app.state.pendingRunnerState.forEach((runner, i) => {
						if (runner.name === '%s') index = i;
					});
				}
				return index;
				})()
			`, runnerName), &index).Do(ctx)
			if err != nil {
				return err
			}
			if index == -1 {
				return fmt.Errorf("runner '%s' not found in pending state", runnerName)
			}
			buttonSelector := fmt.Sprintf("#btn-adv-%d", index)
			return CycleTo(nil, buttonSelector, outcome).Do(ctx)
		}),
	)
}

// --- Verification ---

// GetInningRunnerStates returns the status of all players who have batted in the current inning.
func GetInningRunnerStates(ctx context.Context) ([]map[string]any, error) {
	var results []map[string]any
	err := chromedp.Evaluate(`
		(() => {
			const team = app.state.activeTeam;
			const inning = app.state.activeCtx.i;
			const roster = app.state.activeGame.roster[team];
			const events = app.state.activeGame.events;
			const inningCols = app.state.activeGame.columns.filter(c => c.inning === inning).map(c => c.id);
			
			const list = [];
			roster.forEach((slot, idx) => {
				let latestEvent = null;
				let foundCol = -1;
				for (let i = 0; i < inningCols.length; i++) {
					const key = team + "-" + idx + "-" + inningCols[i];
					if (events[key]) {
						latestEvent = events[key];
						foundCol = i;
					}
				}
				
				if (latestEvent) {
					let status = "Stay";
					const paths = latestEvent.paths;
					if (paths[3] === 1) status = "Score";
					else if (paths[3] === 2) status = "OUT";
					else if (paths[2] === 1) status = "3B";
					else if (paths[2] === 2) status = "OUT";
					else if (paths[1] === 1) status = "2B";
					else if (paths[1] === 2) status = "OUT";
					else if (paths[0] === 1) status = "1B";
					else if (paths[0] === 2) status = "OUT";
					
					list.push({ n: slot.current.name, s: status, c: foundCol, i: idx });
				}
			});
			// Sort by column then roster index to approximate batting order
			list.sort((a, b) => (a.c - b.c) || (a.i - b.i));
			return list.map(item => ({ n: item.n, s: item.s }));
		})()
	`, &results).Do(ctx)
	return results, err
}

// AssertScore checks the scoreboard.
func AssertScore(ctx context.Context, away, home string) error {
	var a, h string
	err := chromedp.Run(ctx,
		chromedp.Text(`#sb-r-away`, &a),
		chromedp.Text(`#sb-r-home`, &h),
	)
	if err != nil {
		return err
	}
	if a != away || h != home {
		return fmt.Errorf("score mismatch: expected %s-%s, got %s-%s", away, home, a, h)
	}
	return nil
}

// AssertPitcher checks the active pitcher display in the CSO.
func AssertPitcher(ctx context.Context, pitcherNameOrNum string) error {
	return chromedp.Run(ctx,
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Try #cso-pitcher-num first
			var val string
			err := chromedp.Text(`#cso-pitcher-num`, &val).Do(ctx)
			if err == nil && strings.TrimSpace(val) == pitcherNameOrNum {
				return nil
			}
			// Maybe check name if num fails? For now just num as per test.
			if strings.TrimSpace(val) != pitcherNameOrNum {
				return fmt.Errorf("expected pitcher %q, got %q", pitcherNameOrNum, val)
			}
			return nil
		}),
	)
}

// RightClick performs a right-click on the selected element.
func RightClick(selector string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// Escape selector for embedding in JavaScript string literals
		escapedSelector := strings.ReplaceAll(selector, "'", "\\'")

		return chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				if (!el) throw new Error('Element not found: %s');
				const rect = el.getBoundingClientRect();
				const x = rect.left + rect.width / 2;
				const y = rect.top + rect.height / 2;
				const ev = new MouseEvent('contextmenu', { 
					bubbles: true, 
					cancelable: true, 
					button: 2, 
					buttons: 2, 
					clientX: x, 
					clientY: y 
				});
				el.dispatchEvent(ev);
			})()
		`, escapedSelector, escapedSelector), nil).Do(ctx)
	})
}

// JSClick clicks an element using JavaScript. Useful for SVGs.
func JSClick(selector string) chromedp.Action {
	return chromedp.Evaluate(fmt.Sprintf(`
		(() => {
			const el = document.querySelector('%s');
			if (el) {
				el.dispatchEvent(new MouseEvent('click', {bubbles: true}));
			} else {
				throw new Error("JSClick: Element not found: " + '%s');
			}
		})()
	`, selector, selector), nil)
}

// WaitForSync waits for the sync status to be Green (Synced).
func WaitForSync(ctx context.Context) error {
	return chromedp.Poll(
		`
		(() => {
			const el = document.querySelector('#sync-status-container svg');
			return el && el.classList.contains('text-green-400');
		})()
	`, nil, chromedp.WithPollingInterval(200*time.Millisecond), chromedp.WithPollingTimeout(5*time.Second)).Do(ctx)
}

// FinalizeGame ends the game via the sidebar.
func FinalizeGame(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.ActionFunc(OpenSidebar),
		chromedp.WaitVisible(`#sidebar-btn-end-game`),
		chromedp.Click(`#sidebar-btn-end-game`),
		// Confirm modal
		chromedp.WaitVisible(`#custom-confirm-modal`),
		chromedp.Click(`#btn-confirm-yes`),
		WaitUntilDisplayNone(`#custom-confirm-modal`),
		// Wait for render
		chromedp.WaitVisible(`#game-status-indicator`), // FINAL indicator
	)
}

// --- Legacy / Internal Helpers ---

// CycleTo clicks a button until its text matches the target value.
// It attempts to use the right-click context menu first for efficiency.
// l is optional (can be nil) if not used for logging.
func CycleTo(l Logger, selector, value string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for i := 0; i < 15; i++ {
			var currentText string
			if err := chromedp.Text(selector, &currentText, chromedp.ByQuery).Do(ctx); err != nil {
				return fmt.Errorf("failed to get text of %s: %w", selector, err)
			}
			currentText = strings.TrimSpace(currentText)

			if l != nil {
				l.Logf("cycleTo: button %s current text: %q, target: %q", selector, currentText, value)
			}
			if currentText == value {
				return nil
			}
			if err := chromedp.Click(selector, chromedp.ByQuery).Do(ctx); err != nil {
				return fmt.Errorf("failed to click button %s: %w", selector, err)
			}
			// Small sleep to allow UI update
			time.Sleep(100 * time.Millisecond)
		}
		return fmt.Errorf("failed to cycle %s to %q", selector, value)
	})
}

// WaitUntilDisplayNone waits until the element is hidden (display: none) or removed.
func WaitUntilDisplayNone(selector string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("WaitUntilDisplayNone: %s", selector)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			timeout := time.After(10 * time.Second)
			for {
				select {
				case <-ctx.Done():
					return fmt.Errorf("context cancelled while waiting for %s to have display: none", selector)
				case <-ticker.C:
					var elementExists bool
					err := chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s') !== null`, selector), &elementExists).Do(ctx)
					if err != nil {
						if strings.Contains(err.Error(), "node for selector") {
							return nil
						}
						return fmt.Errorf("error checking existence of %s: %w", selector, err)
					}
					if !elementExists {
						return nil
					}

					var display string
					err = chromedp.Evaluate(fmt.Sprintf(`window.getComputedStyle(document.querySelector('%s')).display`, selector), &display).Do(ctx)
					if err != nil {
						return fmt.Errorf("error getting display style for %s: %w", selector, err)
					}
					if display == "none" {
						return nil
					}
				case <-timeout:
					return fmt.Errorf("timeout waiting for %s to have display: none (current display: visible)", selector)
				}
			}
		}),
	}
}
