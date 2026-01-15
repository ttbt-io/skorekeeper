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

func TestBroadcastOverlay(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			args := make([]string, len(ev.Args))
			for i, arg := range ev.Args {
				args[i] = string(arg.Value)
			}
			t.Logf("JS CONSOLE (%s): %s", ev.Type, strings.Join(args, " "))
		case *runtime.EventExceptionThrown:
			t.Logf("JS EXCEPTION: %s", ev.ExceptionDetails.Text)
		}
	})

	if err := e2ehelpers.Login(ctx, baseURL); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Create a new game
	gameID, err := e2ehelpers.CreateGame(ctx, "Dragons", "Knights")
	if err != nil {
		t.Fatalf("Failed to create game: %v", err)
	}

	// Navigate to broadcast view
	broadcastURL := fmt.Sprintf("%s/#broadcast/%s", baseURL, gameID)
	if err := chromedp.Run(ctx,
		chromedp.Navigate(broadcastURL),
		chromedp.WaitVisible("#scorebug"),
	); err != nil {
		t.Fatalf("Failed to load broadcast view: %v", err)
	}

	// Verify initial bug state
	var awayName, homeName, score, count string
	if err := chromedp.Run(ctx,
		chromedp.Text("#bug-away-name", &awayName),
		chromedp.Text("#bug-home-name", &homeName),
		chromedp.Text("#bug-away-score", &score),
		chromedp.Text("#bug-count", &count),
	); err != nil {
		t.Fatalf("Failed to verify initial bug state: %v", err)
	}

	if !strings.Contains(awayName, "DRA") || !strings.Contains(homeName, "KNI") {
		t.Errorf("Team names mismatch. Away: %q, Home: %q", awayName, homeName)
	}
	if score != "0" || count != "0-0" {
		t.Errorf("Score or count mismatch. Score: %q, Count: %q", score, count)
	}

	// Open a SECOND context to simulate scorekeeper actions
	ctx2, cancel2 := chromedp.NewContext(ctx)
	defer cancel2()

	if err := e2ehelpers.Login(ctx2, baseURL); err != nil {
		t.Fatalf("Login for scorekeeper failed: %v", err)
	}

	// Navigate to scoresheet
	if err := chromedp.Run(ctx2,
		chromedp.Navigate(fmt.Sprintf("%s/#game/%s", baseURL, gameID)),
		chromedp.WaitVisible("#scoresheet-view"),
	); err != nil {
		t.Fatalf("Skorekeeper failed to load game: %v", err)
	}

	// Record a Ball
	if err := e2ehelpers.SelectCell(ctx2, 1, 1); err != nil {
		t.Fatalf("Skorekeeper failed to select cell: %v", err)
	}
	if err := e2ehelpers.RecordPitch(ctx2, "ball"); err != nil {
		t.Fatalf("Skorekeeper failed to record ball: %v", err)
	}

	// Check broadcast view updates automatically
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible("#bug-count"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Poll(`document.getElementById('bug-count').innerText === '1-0'`, nil).Do(ctx)
		}),
	); err != nil {
		t.Errorf("Broadcast view did not update count to 1-0: %v", err)
	}

	// Record a BIP (Home Run)
	if err := chromedp.Run(ctx2,
		chromedp.Click("#btn-show-bip"),
		chromedp.WaitVisible("#cso-bip-view"),
		e2ehelpers.CycleTo(nil, "#btn-res", "Safe"),
		e2ehelpers.CycleTo(nil, "#btn-type", "HIT"),
		e2ehelpers.CycleTo(nil, "#btn-base", "Home"),
		chromedp.Click("#btn-save-bip"),
	); err != nil {
		t.Fatalf("Skorekeeper failed to record HR: %v", err)
	}
	if err := e2ehelpers.FinishTurn(ctx2); err != nil {
		t.Fatalf("Skorekeeper failed to finish turn: %v", err)
	}

	// Check broadcast view updates score
	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Poll(`document.getElementById('bug-away-score').innerText === '1'`, nil).Do(ctx)
		}),
	); err != nil {
		t.Errorf("Broadcast view did not update score to 1: %v", err)
	}
}
