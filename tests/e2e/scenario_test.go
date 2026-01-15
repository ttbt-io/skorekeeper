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

func TestGameScenario1(t *testing.T) {
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

	reportRunners := func(ctx context.Context, step string) error {
		states, err := e2ehelpers.GetInningRunnerStates(ctx)
		if err != nil {
			return err
		}
		t.Logf("Runner States after %s: %v", step, states)
		return nil
	}

	runStep := func(description string, actions ...chromedp.Action) {
		t.Helper()
		runStep(t, ctx, description, actions...)
	}

	// Helper to record play with trajectory support
	// base: "1B", "2B", "3B", "Home" (or empty for default)
	// loc: "1"-"9" for field position (Hit Location)
	// traj: "Ground", "Line", "Fly", "Pop"
	// seq: Fielder sequence (e.g. "6", "3" for 6-3)
	recordPlay := func(ctx context.Context, res, base, typ, loc string, traj string, seq ...string) error {
		return chromedp.Run(ctx,
			chromedp.Click(`#btn-show-bip`),
			chromedp.WaitVisible(`#cso-bip-view`),
			cycleTo(t, "#btn-res", res),
			chromedp.ActionFunc(func(ctx context.Context) error {
				if base != "" {
					return cycleTo(t, "#btn-base", base).Do(ctx)
				}
				return nil
			}),
			cycleTo(t, "#btn-type", typ),
			chromedp.ActionFunc(func(ctx context.Context) error {
				if traj != "" {
					// Enable Location Mode
					if err := chromedp.Click(`#btn-loc`).Do(ctx); err != nil {
						return err
					}

					// Click somewhere on field based on location
					x, y := 100, 100 // Default Infield
					switch loc {
					case "7":
						x, y = 40, 80 // LF
					case "8":
						x, y = 100, 50 // CF
					case "9":
						x, y = 160, 80 // RF
					case "6":
						x, y = 70, 100 // SS
					case "4":
						x, y = 130, 100 // 2B
					case "5":
						x, y = 40, 130 // 3B
					case "3":
						x, y = 160, 130 // 1B
					case "2":
						x, y = 100, 150 // C/Home area
					case "1":
						x, y = 100, 120 // P
					}

					// Click using Evaluate with Sprintf
					return chromedp.Evaluate(fmt.Sprintf(`(function(x, y) {
                        const svg = document.querySelector('.field-svg-keyboard svg');
                        const rect = svg.getBoundingClientRect();
                        const clientX = rect.left + (x / 200) * rect.width;
                        const clientY = rect.top + (y / 200) * rect.height;
                        // Click on the container div which has the handler
                        const container = document.querySelector('.field-svg-keyboard');
                        container.dispatchEvent(new MouseEvent('click', {
                            bubbles: true, clientX: clientX, clientY: clientY
                        }));
                    })(%d, %d)`, x, y), nil).Do(ctx)
				}
				return nil
			}),
			chromedp.ActionFunc(func(ctx context.Context) error {
				if traj != "" {
					// Cycle trajectory
					targetChar := traj[0:1] // "G", "L", "F", "P"
					return cycleTo(t, "#btn-traj", targetChar).Do(ctx)
				}
				return nil
			}),
			chromedp.ActionFunc(func(ctx context.Context) error {
				for _, s := range seq {
					if err := chromedp.Click(fmt.Sprintf(".pos-key[data-pos=\"%s\"]", s)).Do(ctx); err != nil {
						return err
					}
				}
				return nil
			}),
			chromedp.Click(`#btn-save-bip`),
		)
	}

	// Initial Setup
	// 1. Navigate to App and Start Game
	runStep("Start new game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "Firebirds", "Icebreakers")
			return err
		}),
		DisableCSSAnimations(),
	)

	runStep("Add 2 additional innings (Total 7)",
		chromedp.Click("#btn-menu-scoresheet", chromedp.ByID),
		chromedp.WaitVisible("#sidebar-btn-add-inning", chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('sidebar-btn-add-inning').click()`, nil),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click("#btn-menu-scoresheet", chromedp.ByID),
		chromedp.WaitVisible("#sidebar-btn-add-inning", chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('sidebar-btn-add-inning').click()`, nil),
	)

	// Rename Players - Firebirds
	firebirds := []struct{ n, u string }{
		{"Smith", "1"}, {"Davis", "2"}, {"Brown", "3"}, {"Taylor", "4"},
		{"Anderson", "5"}, {"Thomas", "6"}, {"Martinez", "7"}, {"Robinson", "8"},
		{"Rodriguez", "9"}, {"Lee", "10"}, {"Hall", "11"}, {"Allen", "12"},
	}

	// Helper to edit lineup via modal
	runStep("Edit Lineup Away",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return EditLineup(ctx, "away", firebirds)
		}),
	)

	// Rename Players - Icebreakers
	icebreakers := []struct{ n, u string }{
		{"Jones", "13"}, {"Wilson", "14"}, {"Miller", "15"}, {"Moore", "16"},
		{"Jackson", "17"}, {"White", "18"}, {"Harris", "19"}, {"Clark", "20"},
		{"Lewis", "21"}, {"Walker", "22"}, {"Young", "23"}, {"King", "24"},
	}
	runStep("Edit Lineup Home",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return EditLineup(ctx, "home", icebreakers)
		}),
	)

	// --- Inning 1 ---
	runStep("Top 1: Switch to Away team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)
	runStep("Top 1: #1 Smith: Single (1B) Line Drive Center",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 1); err != nil {
				return err
			}
			return recordPlay(ctx, "Safe", "1B", "HIT", "8", "Line")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 1: #2 Davis: Double (2B) Fly Left",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 1); err != nil {
				return err
			}
			return recordPlay(ctx, "Safe", "2B", "HIT", "7", "Fly")
		}),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(FinishTurn),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 1: #3 Brown: Strikeout (K)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 1); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 1: #4 Taylor: Walk (BB)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 1); err != nil {
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
	runStep("Top 1: #5 Anderson: Grand Slam (HR) Fly",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 1); err != nil {
				return err
			}
			return recordPlay(ctx, "Safe", "Home", "HIT", "8", "Fly")
		}),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(FinishTurn),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 1: #6 Thomas: Ground Out (6-3)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 1); err != nil {
				return err
			}
			return recordPlay(ctx, "Ground", "1B", "OUT", "", "Ground", "6", "3")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 1: #7 Martinez: Fly Out (F8)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 1); err != nil {
				return err
			}
			return recordPlay(ctx, "Fly", "1B", "OUT", "8", "Fly")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 1: Check Score (Firebirds 4, Icebreakers 0)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertScore(ctx, "4", "0")
		}),
	)

	runStep("Bottom 1: Switch to Home team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)
	runStep("Bottom 1: #13 Jones: Walk (BB)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 1); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 1: #14 Wilson: Error (E6) Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 1); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "ERR", "6", "Ground"); err != nil {
				return err
			}
			if err := chromedp.WaitVisible(`#cso-runner-advance-view`).Do(ctx); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Jones", "To 2nd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)
	runStep("Bottom 1: #15 Miller: Strikeout Looking",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 1); err != nil {
				return err
			}
			if err := RecordPitch(ctx, "strike"); err != nil {
				return err
			}
			if err := RecordPitch(ctx, "strike"); err != nil {
				return err
			}
			// Right click for called
			return chromedp.Evaluate(`
				(() => {
					const el = document.querySelector('#btn-strike');
					const event = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2, buttons: 2 });
					el.dispatchEvent(event);
				})()
			`, nil).Do(ctx)
		}),
		chromedp.WaitVisible(`#cso-long-press-submenu`),
		chromedp.Click(`//button[text()="Called"]`, chromedp.BySearch),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 1: #16 Moore: FC Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 1); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "FC", "5", "Ground"); err != nil {
				return err
			}
			if err := chromedp.WaitVisible(`#cso-runner-advance-view`).Do(ctx); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Jones", "Out"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Wilson", "To 2nd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)
	runStep("Bottom 1: #17 Jackson: Line Out (L4)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 1); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Line", "1B", "OUT", "4", "Line"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)
	runStep("Bottom 1: Check Score (Icebreakers 0)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertScore(ctx, "4", "0")
		}),
	)

	// --- Inning 2 ---
	runStep("Top 2: Switch to Away team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)
	runStep("Top 2: #8 Robinson: Single Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 2); err != nil {
				return err
			}
			return recordPlay(ctx, "Safe", "1B", "HIT", "6", "Ground")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 2: #9 Rodriguez: Sac Bunt (SH) Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 2); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Ground", "1B", "SH", "1", "Ground", "3"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)
	runStep("Top 2: #10 Lee: K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 2); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 2: #11 Hall: HBP",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 2); err != nil {
				return err
			}
			if err := chromedp.Click(`[data-auto-advance="HBP"]`).Do(ctx); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)
	runStep("Top 2: #12 Allen: Ground Out 4-3",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 2); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Ground", "1B", "OUT", "", "Ground", "4", "3"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Robinson", "Stay"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Hall", "Stay"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)
	runStep("Top 2: Check Score remains 4",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertScore(ctx, "4", "0")
		}),
	)

	runStep("Bottom 2: Switch to Home team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)
	runStep("Bottom 2: #18 White: 1B Line",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 2); err != nil {
				return err
			}
			return recordPlay(ctx, "Safe", "1B", "HIT", "8", "Line")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 2: SB (White)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 2); err != nil {
				return err
			}
			return HandleRunnerAction(ctx, "White", "SB")
		}),
	)
	runStep("Bottom 2: #19 Harris: BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
	)
	runStep("Bottom 2: #20 Clark: 3B Line",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 2); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "3B", "HIT", "9", "Line"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
	)
	runStep("Bottom 2: #21 Lewis: F9 Fly (Clark Scores)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 2); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Fly", "1B", "OUT", "9", "Fly"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Clark", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 2: #22 Walker: K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 2); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 2: #23 Young: Ground Out 1-3",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 2); err != nil {
				return err
			}
			return recordPlay(ctx, "Ground", "1B", "OUT", "", "Ground", "1", "3")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 2: Check Score (Icebreakers 3)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertScore(ctx, "4", "3")
		}),
	)

	// --- Inning 3 Top ---
	runStep("Top 3: Switch to Away team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)
	runStep("Top 3: #1 Smith: D3",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 3); err != nil {
				return err
			}
			if err := chromedp.Run(ctx,
				chromedp.Click(`#btn-strike`), chromedp.Click(`#btn-strike`),
				chromedp.Evaluate(`
					(() => {
						const el = document.querySelector('#btn-strike');
						const event = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2, buttons: 2 });
						el.dispatchEvent(event);
					})()
				`, nil),
				chromedp.WaitVisible(`#cso-long-press-submenu`),
				chromedp.Click(`//button[text()="Dropped"]`),
				chromedp.WaitVisible(`#cso-bip-view`),
				chromedp.Click(`#btn-save-bip`),
			); err != nil {
				return err
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 3: Smith CS",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 3); err != nil {
				return err
			}
			return HandleRunnerAction(ctx, "Smith", "CS")
		}),
	)
	runStep("Top 3: #2 Davis: BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 3: Davis PO",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 3); err != nil {
				return err
			}
			return HandleRunnerAction(ctx, "Davis", "PO")
		}),
	)
	runStep("Top 3: #3 Brown: 1B Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Already in CSO
			return recordPlay(ctx, "Safe", "1B", "HIT", "6", "Ground")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 3: Brown Adv (WP)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 3); err != nil {
				return err
			}
			return HandleRunnerAction(ctx, "Brown", "Adv")
		}),
	)
	runStep("Top 3: #4 Taylor: IBB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := chromedp.Run(ctx,
				chromedp.Click(`[data-auto-advance="IBB"]`),
				chromedp.WaitVisible(`#cso-runner-advance-view`),
			); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 3: #5 Anderson: IFF Pop",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 3); err != nil {
				return err
			}
			if err := recordPlay(ctx, "IFF", "1B", "OUT", "6", "Pop"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 3: Switch to Home team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)
	runStep("Bottom 3: #24 King: 1B Line",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 3); err != nil {
				return err
			}
			return recordPlay(ctx, "Safe", "1B", "HIT", "8", "Line")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 3: #13 Jones: DP Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 3); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Ground", "1B", "OUT", "", "Ground", "6", "4", "3"); err != nil {
				return err
			}
			return nil
		}),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return SetRunnerOutcome(ctx, "King", "Out")
		}),
		chromedp.ActionFunc(FinishTurn),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 3: #14 Wilson: F7 Fly",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 3); err != nil {
				return err
			}
			return recordPlay(ctx, "Fly", "1B", "OUT", "7", "Fly")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Top 4: Switch to Away team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)
	runStep("Top 4: #6 Thomas: 1B Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 4); err != nil {
				return err
			}
			return recordPlay(ctx, "Safe", "1B", "HIT", "5", "Ground")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 4: #7 Martinez: 2B Line",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 4); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "2B", "HIT", "8", "Line"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 4: #8 Robinson: SF8 Fly",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 4); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Fly", "1B", "SF", "8", "Fly"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Martinez", "Stay"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Thomas", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 4: #9 Rodriguez: K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 4); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 4: #10 Lee: 1B Line",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 4); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "HIT", "9", "Line"); err != nil {
				return err
			}
			if err := chromedp.WaitVisible(`#cso-runner-advance-view`).Do(ctx); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Martinez", "Score"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 4: #11 Hall: Ground Out 5-3",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 4); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Ground", "1B", "OUT", "5", "Ground", "5", "3"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Lee", "Stay"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 4: Switch to Home team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)
	runStep("Bottom 4: Pitcher Change: #99 Johnson",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 4); err != nil {
				return err
			}
			return chromedp.Run(ctx,
				chromedp.Sleep(100*time.Millisecond),          // Wait for modal to load after click
				chromedp.Evaluate(`app.changePitcher()`, nil), // Directly call the JS function
				chromedp.WaitVisible(`#custom-prompt-modal`),
				chromedp.SetValue(`#custom-prompt-modal [data-test="custom-prompt-input"]`, "99"),
				chromedp.Click(`#custom-prompt-modal [data-test="custom-prompt-ok-btn"]`),
				waitUntilDisplayNone(`#custom-prompt-modal`),
			)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertPitcher(ctx, "99")
		}),
		chromedp.Click(`#btn-close-cso`), // Explicitly close modal
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 4: #15 Miller: HR Fly",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 4); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "Home", "HIT", "8", "Fly"); err != nil {
				return err
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 4: #16 Moore: BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 4); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 4: #17 Jackson: CI",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 4); err != nil {
				return err
			}
			if err := chromedp.Click(`[data-auto-advance="CI"]`).Do(ctx); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 4: #18 White: K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 4); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 4: #19 Harris: FC Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 4); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "FC", "5", "Ground"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Moore", "Out"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Jackson", "To 2nd"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 4: #20 Clark: L6 Line",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 4); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Line", "1B", "OUT", "6", "Line"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Top 5
	runStep("Top 5: Switch to Away team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)
	runStep("Top 5: #12 Allen: BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 5); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Top 5: #1 Smith: 1B Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 5); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "HIT", "6", "Ground"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 5: #2 Davis: K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 5); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 5: #3 Brown: INT Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 5); err != nil {
				return err
			}
			if err := HandleRunnerAction(ctx, "Allen", "INT"); err != nil {
				return err
			}
			// Brown reaches on FC (or just safe)
			if err := recordPlay(ctx, "Safe", "1B", "FC", "", "Ground"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 5: #4 (Sub 99): K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 5); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 5: Switch to Home team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)
	runStep("Bottom 5: #21 1B Line",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 5); err != nil {
				return err
			}
			return recordPlay(ctx, "Safe", "1B", "HIT", "8", "Line")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 5: #22 BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 5); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 5: #23 1B Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 5); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "HIT", "6", "Ground"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 5: #24 Force Out Home Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 5); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "FC", "5", "Ground"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Lewis", "Out"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 5: #13 HR Fly",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 5); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "Home", "HIT", "8", "Fly"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 5: #14 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 5); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Bottom 5: #15 Ground Out 3-1",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 5); err != nil {
				return err
			}
			return recordPlay(ctx, "Ground", "1B", "OUT", "", "Ground", "3", "1")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// --- Inning 6 Top ---
	runStep("Top 6: Switch to Away team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)
	runStep("Top 6: Pinch Hitter #25 for #5 (Green)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
                (() => {
                    const el = document.querySelectorAll('.lineup-cell')[4];
                    const event = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2, buttons: 2 });
                    el.dispatchEvent(event);
                })()
            `, nil).Do(ctx)
		}),
		chromedp.WaitVisible(`#player-context-menu`),
		chromedp.Click(`#btn-open-sub`),
		chromedp.WaitVisible(`#substitution-modal`),
		chromedp.SetValue(`#sub-incoming-num`, "25"),
		chromedp.SetValue(`#sub-incoming-name`, "Green"),
		chromedp.Click(`#btn-confirm-sub`), waitUntilDisplayNone(`#substitution-modal`),
	)
	runStep("Top 6: #25 Green: 1B Line",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 6); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "HIT", "8", "Line"); err != nil {
				return err
			}
			return reportRunners(ctx, "#25 Green 1B Line")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 6: #6 Thomas: 2B Fly",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 6); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "2B", "HIT", "7", "Fly"); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}
			return reportRunners(ctx, "#6 Thomas 2B Fly")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 6: #7 Martinez: BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 6); err != nil {
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
			return reportRunners(ctx, "#7 Martinez BB")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 6: #8 Robinson: K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 6); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return reportRunners(ctx, "#8 Robinson K")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 6: #9 Rodriguez: FC Home Ground (Green Out)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 6); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "FC", "5", "Ground"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Green", "Out"); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}
			return reportRunners(ctx, "#9 Rodriguez FC Ground")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 6: #10 Lee: K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 6); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 6: Switch to Home team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)
	runStep("Bottom 6: #16 BB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 6); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return nil
		}),
		chromedp.ActionFunc(FinishTurn),
	)
	runStep("Bottom 6: Runner Left Early",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 6); err != nil {
				return err
			}
			return HandleRunnerAction(ctx, "Moore", "Left Early")
		}),
		// Stay in CSO
	)
	runStep("Bottom 6: #17 1B Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Already in CSO
			return recordPlay(ctx, "Safe", "1B", "HIT", "6", "Ground")
		}),
		chromedp.ActionFunc(FinishTurn),
	)
	runStep("Bottom 6: #18 F8 Fly",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 6, 6); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Fly", "1B", "OUT", "8", "Fly"); err != nil {
				return err
			}
			return nil
		}),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(FinishTurn),
	)
	runStep("Bottom 6: #19 Ground Out 4-3",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 7, 6); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Ground", "1B", "OUT", "", "Ground", "4", "3"); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Jackson", "Stay"); err != nil {
				return err
			}
			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// --- Inning 7 Top ---
	runStep("Top 7: Switch to Away team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "away")
		}),
	)
	// #11 BB, #12 HR, #1 2B, #2 1B
	runStep("Top 7: #11 BB, #12 HR Fly, #1 2B Line, #2 1B Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			// #11 BB
			if err := SelectCell(ctx, 11, 7); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),

		chromedp.ActionFunc(func(ctx context.Context) error {
			// #12 HR Fly
			if err := SelectCell(ctx, 12, 7); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "Home", "HIT", "8", "Fly"); err != nil {
				return err
			}

			return FinishTurn(ctx)
		}),
		waitUntilDisplayNone(`#cso-modal`),

		chromedp.ActionFunc(func(ctx context.Context) error {
			// #1 2B Line
			if err := SelectCell(ctx, 1, 7); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "2B", "HIT", "7", "Line"); err != nil {
				return err
			}

			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),

		chromedp.ActionFunc(func(ctx context.Context) error {
			// #2 1B Ground
			if err := SelectCell(ctx, 2, 7); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "HIT", "6", "Ground"); err != nil {
				return err
			}
			// Smith (2B) to 3rd?
			// Scenario says "Smith to 3rd".
			return nil
		}),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return SetRunnerOutcome(ctx, "Smith", "To 3rd")
		}),
		chromedp.ActionFunc(FinishTurn),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 7: // #3 SF Fly",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 3, 7); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Fly", "1B", "SF", "9", "Fly"); err != nil {
				return err
			}
			return nil
		}),
		chromedp.WaitVisible(`#cso-runner-advance-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return SetRunnerOutcome(ctx, "Smith", "Score")
		}),
		chromedp.ActionFunc(FinishTurn),
		waitUntilDisplayNone(`#cso-modal`),
	)
	runStep("Top 7: #4(99) K, #25 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 4, 7); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),

		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 5, 7); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 7: Switch to Home team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)
	runStep("Bottom 7: Switch to Home team",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return e2ehelpers.SwitchToTeam(ctx, "home")
		}),
	)

	runStep("Bottom 7: #20 1B Line",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 8, 7); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "HIT", "8", "Line"); err != nil {
				return err
			}
			return reportRunners(ctx, "#20 Clark 1B Line")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 7: #21 SH Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 9, 7); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Out", "", "SH", "1", "Ground"); err != nil {
				return err
			}
			if err := chromedp.WaitVisible(`#cso-runner-advance-view`).Do(ctx); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Clark", "To 2nd"); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}
			return reportRunners(ctx, "#21 SH Ground")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 7: #22 1B Ground",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 10, 7); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "HIT", "6", "Ground"); err != nil {
				return err
			}
			if err := chromedp.WaitVisible(`#cso-runner-advance-view`).Do(ctx); err != nil {
				return err
			}
			// Clark to 3rd
			if err := SetRunnerOutcome(ctx, "Clark", "To 3rd"); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}
			return reportRunners(ctx, "#22 1B Ground")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 7: #23 IBB",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 11, 7); err != nil {
				return err
			}
			if err := chromedp.Click(`[data-auto-advance="IBB"]`).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.WaitVisible(`#cso-runner-advance-view`).Do(ctx); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}
			return reportRunners(ctx, "#23 IBB")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 7: #24 K",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 12, 7); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := RecordPitch(ctx, "strike"); err != nil {
					return err
				}
			}
			if err := reportRunners(ctx, "#24 K"); err != nil {
				return err
			}
			return nil
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 7: #13 BB (Tie Game)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 1, 7); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := RecordPitch(ctx, "ball"); err != nil {
					return err
				}
			}
			if err := chromedp.WaitVisible(`#cso-runner-advance-view`).Do(ctx); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Clark", "Score"); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}
			return reportRunners(ctx, "#13 BB (Tie Game)")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep("Bottom 7: #14 1B Line (Win)",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := SelectCell(ctx, 2, 7); err != nil {
				return err
			}
			if err := recordPlay(ctx, "Safe", "1B", "HIT", "8", "Line"); err != nil {
				return err
			}
			if err := chromedp.WaitVisible(`#cso-runner-advance-view`).Do(ctx); err != nil {
				return err
			}
			if err := SetRunnerOutcome(ctx, "Walker", "Score"); err != nil {
				return err
			}
			if err := FinishTurn(ctx); err != nil {
				return err
			}
			return reportRunners(ctx, "#14 1B Line (Win)")
		}),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// Check final score
	runStep("Final Score Check: 9-10",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return AssertScore(ctx, "9", "10")
		}),
	)

	// Capture Screenshot
	runStep("Capture Screenshot",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return CaptureScreenshot(ctx, "/demo/scenario1.png")
		}),
	)

	runStep("Finalize", chromedp.ActionFunc(func(ctx context.Context) error {
		return e2ehelpers.FinalizeGame(ctx)
	}))

	// Verify Narrative Golden
	VerifyNarrative(t, ctx, "scenario1.txt")
}
