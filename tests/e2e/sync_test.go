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
)

func TestSyncWorkflow(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	// 1. Setup Contexts (Client A and Client B)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, *withChromeDP)
	defer cancel()

	// Enable logging for debugging
	opts := []chromedp.ContextOption{
		chromedp.WithLogf(log.Printf),
		chromedp.WithErrorf(log.Printf),
	}

	ctxA, cancelA := chromedp.NewContext(allocCtx, opts...)
	defer cancelA()

	ctxB, cancelB := chromedp.NewContext(allocCtx, opts...)
	defer cancelB()

	// Helper to attach error listener
	listenErrors := func(ctx context.Context, name string) {
		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *runtime.EventConsoleAPICalled:
				if ev.Type == runtime.APITypeError {
					args := make([]string, len(ev.Args))
					for i, arg := range ev.Args {
						args[i] = string(arg.Value)
					}
					t.Logf("[%s] JS CONSOLE ERROR: %s", name, strings.Join(args, " "))
				}
			case *runtime.EventExceptionThrown:
				t.Logf("[%s] JS EXCEPTION: %s", name, ev.ExceptionDetails.Text)
			}
		})
	}
	listenErrors(ctxA, "Client A")
	listenErrors(ctxB, "Client B")

	var gameID string
	var countA, countB int

	// 2. Client A: Init & Login
	runStep(t, ctxA, "Client A: Navigate & Login",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return Login(ctx, baseURL)
		}),
	)

	// 3. Client A: Create Game
	runStep(t, ctxA, "Client A: Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			id, err := CreateGame(ctx, "SyncAway", "SyncHome")
			gameID = id
			return err
		}),
	)
	t.Logf("Game ID: %s", gameID)

	// 4. Client B: Join Game
	runStep(t, ctxB, "Client B: Join Game",
		chromedp.Navigate(fmt.Sprintf(`%s/#game/%s`, baseURL, gameID)),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		chromedp.WaitVisible(`#scoresheet-view`),
		// Verify Login (should be auto via shared cookie)
		// Auth container is on dashboard, so it's hidden in game view.
		// Instead, check for sync status which implies we are in the game view.
		chromedp.WaitVisible(`#sync-status-container`),
	)

	// 5. Verify Sync Status UI
	runStep(t, ctxA, "Verify Sync Status A",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)
	runStep(t, ctxB, "Verify Sync Status B",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)

	// 6. Action A -> B (Ball)
	runStep(t, ctxA, "Client A: Record Ball",
		chromedp.Click(`.grid-cell`), // Open CSO (First available cell)
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return RecordPitch(ctx, "ball")
		}),
		// Verify local update
		CSOBallCount(&countA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countA != 1 {
				return fmt.Errorf("Client A: Expected 1 ball, got %d", countA)
			}
			return nil
		}),
	)

	runStep(t, ctxB, "Client B: Verify Sync (Ball)",
		chromedp.Sleep(100*time.Millisecond),
		// Open CSO on B to see the same cell.
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		// Wait/Poll for update
		CSOBallCount(&countB),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countB != 1 {
				return fmt.Errorf("Client B: Expected 1 ball, got %d", countB)
			}
			return nil
		}),
	)

	// 7. Action B -> A (Strike)
	runStep(t, ctxB, "Client B: Record Strike",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return RecordPitch(ctx, "strike")
		}),
		// Verify local
		CSOStrikeCount(&countB),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countB != 1 {
				return fmt.Errorf("Client B: Expected 1 strike, got %d", countB)
			}
			return nil
		}),
	)

	runStep(t, ctxA, "Client A: Verify Sync (Strike)",
		chromedp.Sleep(100*time.Millisecond),
		// A already has CSO open. Should see update live.
		CSOStrikeCount(&countA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countA != 1 {
				return fmt.Errorf("Client A: Expected 1 strike, got %d", countA)
			}
			return nil
		}),
	)
}
