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
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/ttbt-io/skorekeeper/tools/e2ehelpers"
)

var DisableCSSAnimations = e2ehelpers.DisableCSSAnimations
var WaitAnyVisible = e2ehelpers.WaitAnyVisible
var OpenSidebar = e2ehelpers.OpenSidebar
var Login = e2ehelpers.Login
var LoginWithUser = e2ehelpers.LoginWithUser
var Logout = e2ehelpers.Logout
var OpenNewGameModal = e2ehelpers.OpenNewGameModal
var CreateGame = e2ehelpers.CreateGame
var AddInning = e2ehelpers.AddInning
var SelectCell = e2ehelpers.SelectCell
var RecordPitch = e2ehelpers.RecordPitch
var RecordBallInPlay = e2ehelpers.RecordBallInPlay
var HandleRunnerAction = e2ehelpers.HandleRunnerAction
var FinishTurn = e2ehelpers.FinishTurn
var SetRunnerOutcome = e2ehelpers.SetRunnerOutcome
var AssertScore = e2ehelpers.AssertScore
var AssertPitcher = e2ehelpers.AssertPitcher
var RightClick = e2ehelpers.RightClick
var JSClick = e2ehelpers.JSClick
var WaitForSync = e2ehelpers.WaitForSync
var CaptureScreenshot = e2ehelpers.CaptureScreenshot

// Wrapper functions to match signatures if needed or just aliases

func CSOBallCount(count *int) chromedp.Action {
	return chromedp.Evaluate(`document.querySelectorAll('#cso-modal .cell-content-layer .count-display .count-dots:first-child .filled-black').length`, count)
}

func CSOStrikeCount(count *int) chromedp.Action {
	return chromedp.Evaluate(`document.querySelectorAll('#cso-modal .cell-content-layer .count-display .count-dots:last-child .filled-black').length`, count)
}

func EditLineup(ctx context.Context, teamSide string, players []struct{ n, u string }) error {
	// Convert anonymous struct to e2ehelpers.PlayerInfo
	pInfos := make([]e2ehelpers.PlayerInfo, len(players))
	for i, p := range players {
		pInfos[i] = e2ehelpers.PlayerInfo{N: p.n, U: p.u}
	}
	return e2ehelpers.EditLineup(ctx, teamSide, pInfos)
}

func cycleTo(t *testing.T, selector, value string) chromedp.Action {
	var l e2ehelpers.Logger
	if t != nil {
		l = t
	}
	return e2ehelpers.CycleTo(l, selector, value)
}

func waitUntilDisplayNone(selector string) chromedp.Tasks {
	return e2ehelpers.WaitUntilDisplayNone(selector)
}

// LoginAndCreateGame is the legacy helper
func LoginAndCreateGame(ctx context.Context, baseURL, teamAway, teamHome string) (string, error) {
	if err := Login(ctx, baseURL); err != nil {
		return "", fmt.Errorf("Login failed: %w", err)
	}
	return CreateGame(ctx, teamAway, teamHome)
}
