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

func TestGameScenario2(t *testing.T) {
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
	ctx, cancel = context.WithTimeout(ctx, 240*time.Second) // Increased timeout for long scenario
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
	runStep("Start New Game (Rockets vs Aviators)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := LoginAndCreateGame(ctx, baseURL, "Rockets", "Aviators")
			return err
		}),
		DisableCSSAnimations(),
	)

	// Add 2 innings (Total 7)
	runStep("Add innings 6 and 7",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := AddInning(ctx); err != nil {
				return err
			}
			return AddInning(ctx)
		}),
	)

	// Rename Players - Rockets (Away)
	rockets := []struct{ n, u string }{
		{"Speedy", "1"}, {"Flash", "2"}, {"Slugger", "3"}, {"Cannon", "4"},
		{"Ace", "5"}, {"Lefty", "6"}, {"Stretch", "7"}, {"Glove", "8"},
		{"Hotcorner", "9"}, {"Rover", "10"}, {"Power", "11"}, {"Rookie", "12"},
	}
	runStep("Edit Rockets Lineup",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return EditLineup(ctx, "away", rockets)
		}),
	)

	// Rename Players - Aviators (Home)
	aviators := []struct{ n, u string }{
		{"Pilot", "11"}, {"Wingman", "12"}, {"Bomber", "13"}, {"Zoom", "14"},
		{"Slider", "15"}, {"Curve", "16"}, {"Knuckle", "17"}, {"Changeup", "18"},
		{"Fastball", "19"}, {"Cutter", "20"}, {"Sinker", "21"}, {"Glider", "22"},
	}
	runStep("Edit Aviators Lineup",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return EditLineup(ctx, "home", aviators)
		}),
	)

	// --- Inning 1 ---
	runStep("Switch to Rockets (Away)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	// Top 1 Rockets
	runStep("Top 1: #1 Open CSO",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return SelectCell(ctx, 1, 1)
		}),
	)

	runStep("Top 1: Verify CSO Title",
		chromedp.ActionFunc(func(ctx context.Context) error {
			var title string
			if err := chromedp.Text(`#cso-title`, &title).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(title, "Speedy") {
				return fmt.Errorf("expected CSO title to contain 'Speedy', got %q", title)
			}
			return nil
		}),
	)

	runStep("Top 1: #1 BB (Walk)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 1: #2 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 1); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 1: #3 HR (Mercy Limit)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 1); err != nil {
				return err
			}
			// Manual HR recording to handle base cycling if needed, or helper
			// RecordBallInPlay doesn't support cycling bases yet easily.
			// Let's do it manually for HR.
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery), // Home
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Switch to Aviators",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 1: #11 2B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 1); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), // 2B
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 1: #12 1B (Pilot Scores)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 1); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Pilot", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 1: #13 2B (Wingman Scores)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 1); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), // 2B
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Wingman", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 1: #14 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 1); err != nil {
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

	runStep("Bottom 1: #15 L4",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 1); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Line", "OUT", "4"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	// --- Inning 2 ---
	runStep("Top 2: Switch to Rockets",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 2: #4 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 2); err != nil {
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

	runStep("Top 2: #5 Ground Out (6-3)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 2); err != nil {
				return err
			}
			// Manual BiP for multiple locations
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				cycleTo(nil, "#btn-res", "Ground"),
				chromedp.Click(`.pos-key[data-pos="6"]`, chromedp.ByQuery), chromedp.Click(`.pos-key[data-pos="3"]`, chromedp.ByQuery),
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Top 2: #6 F8",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 2); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Fly", "OUT", "8"); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Check Score End Top 2",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertScore(ctx, "3", "2") // Rockets 3, Aviators 2 (from Inn 1)
		}),
	)

	runStep("Bottom 2: Switch to Aviators",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 2: #17 BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 2); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 2: #18 SH",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 2); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Ground", "SH", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Knuckle", "To 2nd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 2: #19 1B (Knuckle to 3rd)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 2); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Knuckle", "To 3rd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 2: #20 SF (Knuckle scores)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 2); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Fly", "SF", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Knuckle", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 2: #21 Ground Out (6-3)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 2); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				cycleTo(nil, "#btn-res", "Ground"),
				chromedp.Click(`.pos-key[data-pos="6"]`, chromedp.ByQuery), chromedp.Click(`.pos-key[data-pos="3"]`, chromedp.ByQuery),
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Fastball", "Out"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	// --- Inning 3 ---
	runStep("Top 3: Switch to Rockets",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 3: #7 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Scroll to center using JS
			if err := chromedp.Evaluate(`document.querySelector('#scoresheet-grid > div:nth-child(32)').scrollIntoView({block: 'center'})`, nil).Do(ctx); err != nil {
				// Ignore scroll error
			}
			if err := SelectCell(ctx, 7, 3); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Top 3: #8 1B (Stretch scores)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 3); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Stretch", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 3: #9 2B (Glove scores)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 3); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), // 2B
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Glove", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 3: #10 F7",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 3); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Fly", "OUT", "7"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Hotcorner", "Stay"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 3: #11 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 3); err != nil {
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

	runStep("Top 3: #12 Ground Out",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 3); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Ground", "OUT", ""); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Check Score End Top 3",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertScore(ctx, "5", "3") // Rockets 5 (3+2), Aviators 3
		}),
	)

	runStep("Switch to Aviators",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 3: #22 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 3); err != nil {
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

	runStep("Bottom 3: #11 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 3); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 3: #12 HR (Pilot scores)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 3); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery), // Home
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Pilot", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 3: #13 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 3); err != nil {
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

	runStep("Bottom 3: #14 Ground Out",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 3); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Ground", "OUT", ""); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	// --- Inning 4 (Mercy Override) ---
	runStep("Top 4: Switch to Rockets",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 4: #1 BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 4); err != nil {
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

	runStep("Top 4: #2 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 4); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Speedy", "To 2nd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 4: #3 BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 4); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 4: #4 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 4); err != nil {
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

	runStep("Top 4: #5 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 4); err != nil {
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

	runStep("Top 4: #6 Grand Slam",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 4); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery), chromedp.Click(`#btn-base`, chromedp.ByQuery), // Home
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Speedy", "Score"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Flash", "Score"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Slugger", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	// Manual Adjustment
	runStep("Manual Score Adjustment Top 4 -> 3 Runs",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				const el = document.querySelectorAll('#sb-innings-away .sb-cell')[3];
				const ev = new MouseEvent('contextmenu', { bubbles: true, cancelable: true });
				el.dispatchEvent(ev);
			`, nil).Do(ctx)
		}),
		chromedp.WaitVisible(`#custom-prompt-modal`, chromedp.ByQuery),
		chromedp.SetValue(`[data-test="custom-prompt-input"]`, "3", chromedp.ByQuery),
		chromedp.Click(`[data-test="custom-prompt-ok-btn"]`, chromedp.ByQuery),
		waitUntilDisplayNone(`#custom-prompt-modal`),
	)

	runStep("Check Score End Top 4",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertScore(ctx, "8", "5")
		}),
	)

	runStep("Switch to Aviators",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 4: #15 BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 4); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 4: #16 BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 4); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			if err := SetRunnerOutcome(ctx, "Slider", "To 2nd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 4: #17 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 4); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Slider", "Score"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Curve", "To 3rd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 4: #18 2B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 4); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), // 2B
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Curve", "Score"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Knuckle", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	// --- Inning 5 ---
	runStep("Top 5: Switch to Rockets",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 5: #7 Ground Out",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 5); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Ground", "OUT", ""); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Top 5: #8 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 5); err != nil {
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

	runStep("Top 5: #9 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 5); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Top 5: #9 PO",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 5); err != nil {
				return err
			}
			if err := HandleRunnerAction(ctx, "Hotcorner", "PO"); err != nil {
				return err
			}
			if err := chromedp.Click(`#btn-close-cso`, chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Switch to Aviators",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 5: #19 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 5); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 5: #20 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 5); err != nil {
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

	runStep("Bottom 5: #21 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 5); err != nil {
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

	runStep("Bottom 5: #22 F9",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 5); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Fly", "OUT", "9"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	// --- Inning 6 (ITB) ---
	runStep("Top 6: Switch to Rockets",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)

	runStep("Top 6: ITB Setup",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 6); err != nil {
				return err
			}
			// Place on 2nd
			if err := chromedp.Click(`#zoom-base-1b`, chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Click(`#btn-close-runner-menu`, chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Click(`#btn-close-cso`, chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Top 6: #10 SH",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 6); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Ground", "SH", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Hotcorner", "To 3rd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 6: #11 SF",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 6); err != nil {
				return err
			}
			// Toggle 3rd base safe (manual placement hack from original test)
			// Wait, the original test clicked base 3B to toggle safe status before recording?
			// "chromedp.Click(`#zoom-base-3b`)"
			// This is because Hotcorner is on 3rd.
			// Let's assume the previous play moved him there correctly.
			if err := RecordBallInPlay(ctx, "Fly", "SF", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Hotcorner", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Top 6: #12 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 6); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Top 6: #12 PO",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 6); err != nil {
				return err
			}
			if err := HandleRunnerAction(ctx, "Rookie", "PO"); err != nil {
				return err
			}
			return nil
		}),
	)

	runStep("Top 6: #1 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			//if err := SelectCell(ctx, 1, 6); err != nil {
			//	return err
			//}
			// Still in the same CSO
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	// Bottom 6 ITB
	runStep("Switch to Aviators",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 6: ITB Setup",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 6); err != nil {
				return err
			}
			if err := chromedp.Click(`#zoom-base-1b`, chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Click(`#btn-close-runner-menu`, chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Click(`#btn-close-cso`, chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
			return waitUntilDisplayNone(`#cso-modal`).Do(ctx)
		}),
	)

	runStep("Bottom 6: #11 1B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 6); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Glider", "To 3rd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 6: #12 1B (Tie)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 6); err != nil {
				return err
			}
			if err := RecordBallInPlay(ctx, "Safe", "HIT", ""); err != nil {
				return err
			}
			// Glider scores, Pilot to 2nd (default)
			return FinishTurn(ctx)
		}),
	)

	runStep("Bottom 6: #13 2B (Win)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 6); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-show-bip`, chromedp.ByQuery), chromedp.WaitVisible(`#cso-bip-view`, chromedp.ByQuery),
				chromedp.Click(`#btn-base`, chromedp.ByQuery), // 2B
				chromedp.Click(`#btn-save-bip`, chromedp.ByQuery),
			); err != nil {
				return err
			}
			// Pilot scores (Winning run)
			return FinishTurn(ctx)
		}),
	)

	runStep("Final Score Check",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertScore(ctx, "9", "10")
		}),
	)

	runStep("Success", chromedp.ActionFunc(func(ctx context.Context) error {
		t.Log("Scenario 2 Completed")
		return nil
	}))

	// Capture Screenshot
	runStep("Capture Screenshot",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return CaptureScreenshot(ctx, "/demo/scenario2.png")
		}),
	)

	runStep("Finalize", chromedp.ActionFunc(func(ctx context.Context) error {
		return e2ehelpers.FinalizeGame(ctx)
	}))

	// Verify Narrative Golden
	VerifyNarrative(t, ctx, "scenario2.txt")
}
