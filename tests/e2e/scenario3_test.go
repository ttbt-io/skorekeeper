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

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/ttbt-io/skorekeeper/tools/e2ehelpers"
)

func TestGameScenario3(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 240*time.Second)
	defer cancel()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			args := make([]string, len(ev.Args))
			for i, arg := range ev.Args {
				args[i] = string(arg.Value)
			}
			t.Logf("JS CONSOLE (%s): %s", ev.Type, strings.Join(args, " "))
		}
	})

	runStep := func(description string, actions ...chromedp.Action) {
		t.Helper()
		runStep(t, ctx, description, actions...)
	}

	// 1. Navigate to App and Start New Game
	runStep("Start new game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := LoginAndCreateGame(ctx, baseURL, "Oddballs", "Rulebenders")
			return err
		}),
		DisableCSSAnimations(),
	)

	oddballs := []struct{ n, u string }{
		{"Odd", "1"}, {"Weird", "2"}, {"Strange", "3"}, {"Quirk", "4"},
		{"Peculiar", "5"}, {"Bizarre", "6"}, {"Curious", "7"}, {"Rare", "8"},
		{"Unique", "9"}, {"Freak", "10"}, {"Unusual", "11"}, {"Wild", "12"},
	}
	runStep("Edit Oddballs Lineup",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return EditLineup(ctx, "away", oddballs)
		}),
	)

	rulebenders := []struct{ n, u string }{
		{"Bender", "13"}, {"Twist", "14"}, {"Loop", "15"}, {"Knot", "16"},
		{"Kink", "17"}, {"Warp", "18"}, {"Coil", "19"}, {"Spiral", "20"},
		{"Arc", "21"}, {"Curve", "22"}, {"Swerve", "23"}, {"Zigzag", "24"},
	}
	runStep("Edit Rulebenders Lineup",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return EditLineup(ctx, "home", rulebenders)
		}),
	)

	runStep("Top 1: Switch to Away",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 1: Bases Loaded singles",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// #1 Single
			if err := SelectCell(ctx, 1, 1); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			if err := waitUntilDisplayNone(`#cso-modal`).Do(ctx); err != nil {
				return err
			}

			// #2 Single
			if err := SelectCell(ctx, 2, 1); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}

			// #3 Single
			if err := SelectCell(ctx, 3, 1); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 1: Triple Play",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 1); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				cycleTo(nil, "#btn-res", "Line"),
				chromedp.Click(`.pos-key[data-pos="6"]`, chromedp.ByQuery),
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Odd", "Out"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Weird", "Out"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Strange", "Stay"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 1: Switch to Home",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 1: #13 Walk",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 1); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 1: #14 GDP 6-4-3",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 1); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				cycleTo(nil, "#btn-res", "Ground"),
				chromedp.Click(`.pos-key[data-pos="6"]`, chromedp.ByQuery), chromedp.Click(`.pos-key[data-pos="4"]`, chromedp.ByQuery), chromedp.Click(`.pos-key[data-pos="3"]`, chromedp.ByQuery),
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Bender", "Out"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 1: #15 Strikeout",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 1); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 2: Switch to Away",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 2: Correct Batter Mid-AB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 2); err != nil {
				return err
			}
			// 2 Balls
			if err := RecordPitch(ctx, "ball"); err != nil {
				return err
			}
			if err := RecordPitch(ctx, "ball"); err != nil {
				return err
			}

			// Correct Player Context Menu
			if err := RightClick("#cso-title").Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.WaitVisible(`#player-context-menu`, chromedp.ByQuery),
				chromedp.Click(`//button[text()="Correct Player in Slot"]`, chromedp.BySearch),
				chromedp.WaitVisible(`#custom-prompt-modal`, chromedp.ByQuery),
				chromedp.Click(`//div[@id="custom-prompt-options"]//button[contains(text(), "6 - Bizarre")]`, chromedp.BySearch),
				chromedp.WaitVisible(`#cso-title`, chromedp.ByQuery),
			); err != nil {
				return err
			}

			// Verify Title
			var title string
			if err := chromedp.Text(`#cso-title`, &title).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(title, "Bizarre") {
				return fmt.Errorf("Expected title to contain Bizarre, got %s", title)
			}
			return nil
		}),
	)

	runStep("Top 2: #6 Bizarre HR",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Already in CSO
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery), // Home
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Top 2: #7 K, #8 Ground Out, #9 F8",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// #7 K
			if err := SelectCell(ctx, 7, 2); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}

			// #8 Ground Out
			if err := SelectCell(ctx, 8, 2); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Ground", "OUT", ""); err != nil {
				return err
			}
			if err := waitUntilDisplayNone(`#cso-modal`).Do(ctx); err != nil {
				return err
			}

			// #9 F8
			if err := SelectCell(ctx, 9, 2); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Fly", "OUT", "8"); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 2: Switch to Home",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 2: BOO Setup (Record 1B for #17)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 2); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 2: Penalty Out on #16",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 2); err != nil {
				return err
			}
			// Right click OUT button
			if err := RightClick("#btn-out").Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.WaitVisible(`#cso-long-press-submenu`, chromedp.ByQuery),
				chromedp.Click(`//button[@data-c="BOO"]`, chromedp.BySearch),
				waitUntilDisplayNone(`#cso-modal`),
			); err != nil {
				return err
			}
			return nil
		}),
	)

	runStep("Bottom 2: Clear #17 illegal hit",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 2); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-toggle-action`, chromedp.ByQuery),
				chromedp.WaitVisible(`#btn-clear-all`, chromedp.ByQuery),
				chromedp.Click(`#btn-clear-all`, chromedp.ByQuery),
				chromedp.WaitVisible(`#custom-confirm-modal`, chromedp.ByQuery),
				chromedp.Click(`#custom-confirm-modal [data-test="custom-confirm-ok-btn"]`, chromedp.ByQuery),
				waitUntilDisplayNone(`#custom-confirm-modal`),
				waitUntilDisplayNone(`#cso-modal`),
			); err != nil {
				return err
			}
			return nil
		}),
	)

	runStep("Bottom 2: #17 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 2); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
	)

	runStep("Bottom 2: #18 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 2); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 2: #19 Ground Out (Warp to 2nd)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 2); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Ground", "OUT", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Warp", "To 2nd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 3: Switch to Away",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 3: Mistaken Record (2B in #11)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 3); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), // 2B
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
				waitUntilDisplayNone(`#cso-modal`),
			); err != nil {
				return err
			}
			return nil
		}),
	)

	runStep("Top 3: Move Play from #11 to #10",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// We need the selector for cell 11, 3
			// Helpers don't expose selector generation directly, so we use JS to find it or replicate logic
			// SelectCell clicks it. We need to right click it.
			// Let's use JS to find it and right click.
			err := chromedp.Evaluate(`
				(() => {
					// 11th player (index 10), 3rd inning (offset 3)
					// But grid index is complex.
					// Let's use data attributes if available, or just the same logic as SelectCell but RightClick
					// SelectCell uses nth-child.
					const stride = document.querySelectorAll('#scoresheet-grid > .grid-header').length;
					const index = stride + (11 - 1) * stride + 1 + 3;
					const el = document.querySelector('#scoresheet-grid > div:nth-child(' + index + ')');
					if (el) {
						const ev = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2, buttons: 2 });
						el.dispatchEvent(ev);
					} else {
						throw new Error("Cell not found");
					}
				})()
			`, nil).Do(ctx)
			if err != nil {
				return err
			}

			return chromedp.Run(ctx,
				chromedp.WaitVisible(`#column-context-menu`, chromedp.ByQuery),
				chromedp.Click(`//button[text()="Move Play To..."]`, chromedp.BySearch),
				chromedp.WaitVisible(`#custom-prompt-modal`, chromedp.ByQuery),
				chromedp.Click(`//div[@id="custom-prompt-options"]//button[contains(text(), "10 - Freak")]`, chromedp.BySearch),
			)
		}),
	)

	runStep("Top 3: Verify Move",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 3); err != nil {
				return err
			}
			var outcome string
			if err := chromedp.Text(`#zoom-outcome-text`, &outcome).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(outcome, "2B") {
				return fmt.Errorf("Expected outcome 2B in slot 10, got %s", outcome)
			}
			return chromedp.Click(`#btn-close-cso`, chromedp.ByQuery).Do(ctx)
		}),
	)

	runStep("Top 3: #11 K, #12 BB, #1 IFF, #2 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// #11 K
			if err := SelectCell(ctx, 11, 3); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}

			// #12 BB
			if err := SelectCell(ctx, 12, 3); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}

			// #1 IFF
			if err := SelectCell(ctx, 1, 3); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "IFF", "OUT", ""); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}

			// #2 K
			if err := SelectCell(ctx, 2, 3); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 3: Switch to Home",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 3: #21 3B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 3); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				cycleTo(t, "#btn-base", "3B"),
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 3: #22 Squeeze",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 3); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Arc", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 3: #23 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 3); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
	)

	runStep("Bottom 3: #24 DP (L3, 3U)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 3); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				cycleTo(nil, "#btn-res", "Line"),
				chromedp.Click(`.pos-key[data-pos="3"]`, chromedp.ByQuery),
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Curve", "Out"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 4: Switch to Away",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 4: #3 K (WP)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 4); err != nil {
				return err
			}
			if err := RecordPitch(ctx, "strike"); err != nil {
				return err
			}
			if err := RecordPitch(ctx, "strike"); err != nil {
				return err
			}

			// 3rd strike dropped
			if err := RightClick("#btn-strike").Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.WaitVisible(`#cso-long-press-submenu`, chromedp.ByQuery),
				chromedp.Click(`//button[text()="Dropped"]`, chromedp.BySearch),
				chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				// Type defaults to D3, so we just save
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
				waitUntilDisplayNone(`#cso-modal`),
			); err != nil {
				return err
			}
			return nil
		}),
	)

	runStep("Top 4: #4 F2",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 4); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Fly", "OUT", "2"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 4: Restore Peculiar",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 4); err != nil {
				return err
			}
			if err := RightClick("#cso-title").Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.WaitVisible(`#player-context-menu`, chromedp.ByQuery),
				chromedp.Click(`//button[text()="Correct Player in Slot"]`, chromedp.BySearch),
				chromedp.WaitVisible(`#custom-prompt-modal`, chromedp.ByQuery),
				chromedp.Click(`//div[@id="custom-prompt-options"]//button[contains(text(), "5 - Peculiar")]`, chromedp.BySearch),
				chromedp.WaitVisible(`#cso-title`, chromedp.ByQuery),
			); err != nil {
				return err
			}

			var title string
			if err := chromedp.Text(`#cso-title`, &title).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(title, "Peculiar") {
				return fmt.Errorf("Expected title to contain Peculiar, got %s", title)
			}
			return chromedp.Click(`#btn-close-cso`, chromedp.ByQuery).Do(ctx)
		}),
	)

	runStep("Top 4: #5 CI",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 4); err != nil {
				return err
			}
			if err := chromedp.Click(`[data-auto-advance="CI"]`, chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 4: #6 FC",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 4); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				cycleTo(nil, "#btn-type", "FC"),
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Strange", "Out"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Peculiar", "To 2nd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 4: #7 INT",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 4); err != nil {
				return err
			}
			if err := HandleRunnerAction(ctx, "Peculiar", "INT"); err != nil {
				return err
			}

			// FC for batter
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				cycleTo(nil, "#btn-type", "FC"),
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 4: Switch to Home",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 4: #13 HR",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 4); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery),
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 4: #14 BB, #15 HBP",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// #14 BB
			if err := SelectCell(ctx, 2, 4); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}

			// #15 HBP
			if err := SelectCell(ctx, 3, 4); err != nil {
				return err
			}
			if err := chromedp.Click(`[data-auto-advance="HBP"]`, chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 4: #16 F8, #17 F9, #18 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// #16 F8
			if err := SelectCell(ctx, 4, 4); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Fly", "OUT", "8"); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}

			// #17 F9
			if err := SelectCell(ctx, 5, 4); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Fly", "OUT", "9"); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}

			// #18 K
			if err := SelectCell(ctx, 6, 4); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 5: Switch to Away",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 5: #8 BB, #9 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// #8 BB
			if err := SelectCell(ctx, 8, 5); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}

			// #9 K
			if err := SelectCell(ctx, 9, 5); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 5: #10 DP (K)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 5); err != nil {
				return err
			}
			// 2 strikes
			if err := RecordPitch(ctx, "strike"); err != nil {
				return err
			}
			if err := RecordPitch(ctx, "strike"); err != nil {
				return err
			}
			// 3rd strike
			if err := RecordPitch(ctx, "strike"); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)
	// This closes CSO.
	// Now we need to record CS on #8 Rare (on 1st).
	// We have to open the runner's CSO or the batter's CSO to access runner actions?
	// The original test clicked cell 8, 5 (the runner's cell) which is already completed (BB).
	// Clicking a completed cell normally opens it for edit, but here we want to modify the runner state.
	// The original test logic:
	//   chromedp.Click(getCellSelector(8, 5)), ...
	//   jsClick(`#zoom-base-1b`), ...
	//   chromedp.Click(`//div[@id="runner-menu-options"]//button[text()="CS"]`)

	runStep("Top 5: Runner CS",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return SelectCell(ctx, 8, 5)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("JSClick #zoom-base-2b")
			if err := JSClick(`#zoom-base-2b`).Do(ctx); err != nil {
				return err
			}
			return chromedp.Sleep(time.Second).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("JSClick #zoom-base-2b")
			if err := JSClick(`#zoom-base-2b`).Do(ctx); err != nil {
				return err
			}
			return chromedp.Sleep(time.Second).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var foo string
			chromedp.OuterHTML(`#cso-runner-menu`, &foo).Do(ctx)
			t.Logf("#cso-runner-menu: %q", foo)
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("Wait for #cso-runner-menu")
			return chromedp.WaitVisible(`#cso-runner-menu`, chromedp.ByQuery).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("Click CS")
			return chromedp.Click(`//div[@id="runner-menu-options"]//button[text()="CS"]`, chromedp.BySearch).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("Wait for #cso-runner-menu to disappear")
			return waitUntilDisplayNone(`#cso-runner-menu`).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("Click #btn-close-cso")
			return chromedp.Click(`#btn-close-cso`, chromedp.ByQuery).Do(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Capture Screenshot
	runStep("Capture Screenshot",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return CaptureScreenshot(ctx, "/demo/scenario3.png")
		}),
	)

	chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.FinalizeGame(ctx)
		}),
	)

	// Verify Narrative Golden
	VerifyNarrative(t, ctx, "scenario3.txt")
}
