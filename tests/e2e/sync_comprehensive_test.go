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

func TestDivergentConflict(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	// 1. Setup Contexts (Client A and Client B)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, *withChromeDP)
	defer cancel()

	opts := []chromedp.ContextOption{
		chromedp.WithLogf(log.Printf),
		chromedp.WithErrorf(log.Printf),
	}

	ctxA, cancelA := chromedp.NewContext(allocCtx, opts...)
	defer cancelA()

	ctxB, cancelB := chromedp.NewContext(allocCtx, opts...)
	defer cancelB()

	// Error Listener Helper
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
	var conflictVisible bool

	// 2. Client A: Init & Login & Create Game
	runStep(t, ctxA, "Client A: Navigate & Login & Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctxA, baseURL); err != nil {
				return err
			}
			id, err := CreateGame(ctxA, "ConflictAway", "ConflictHome")
			gameID = id
			return err
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)
	t.Logf("Game ID: %s", gameID)

	// 3. Client B: Join Game
	runStep(t, ctxB, "Client B: Join Game",
		chromedp.Navigate(fmt.Sprintf(`%s/#game/%s`, baseURL, gameID)),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		chromedp.WaitVisible(`#scoresheet-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)

	// 4. Client A: Go Offline
	runStep(t, ctxA, "Client A: Go Offline",
		chromedp.Evaluate(`app.sync.disconnect(true)`, nil), // Stop retrying
		chromedp.Sleep(100*time.Millisecond),
	)

	// 5. Client B: Record Ball (Server Rev 1)
	runStep(t, ctxB, "Client B: Record Ball",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return RecordPitch(ctx, "ball")
		}),
		CSOBallCount(&countB),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countB != 1 {
				return fmt.Errorf("Client B: Expected 1 ball, got %d", countB)
			}
			return nil
		}),
		// Close CSO to save state/ensure sync completes (though recordPitch syncs too)
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// 6. Client A: Record Strike (Local Rev 1' -> Divergent)
	runStep(t, ctxA, "Client A: Record Strike (Offline)",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return RecordPitch(ctx, "strike")
		}),
		CSOStrikeCount(&countA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countA != 1 {
				return fmt.Errorf("Client A: Expected 1 strike, got %d", countA)
			}
			return nil
		}),
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// 7. Client A: Reload to trigger Reconnect & Sync
	// Reloading forces a fresh connection logic which usually exposes JOIN issues best
	runStep(t, ctxA, "Client A: Reload",
		chromedp.Reload(),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
	)

	// 8. Client A: Verify Conflict Modal
	// Wait for modal to appear. It might take a moment after sync attempt.
	runStep(t, ctxA, "Client A: Verify Conflict Modal",
		chromedp.WaitVisible(`#conflict-resolution-modal`),
		chromedp.Evaluate(`!document.getElementById('conflict-resolution-modal').classList.contains('hidden')`, &conflictVisible),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !conflictVisible {
				return fmt.Errorf("Conflict modal did not appear")
			}
			return nil
		}),
	)

	// 9. Client A: Resolve (Overwrite)
	// We choose to discard local changes (Strike) and take server state (Ball).
	runStep(t, ctxA, "Client A: Overwrite",
		chromedp.Click(`#btn-conflict-overwrite`),
		waitUntilDisplayNone(`#conflict-resolution-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)

	// 10. Client A: Verify State matches B (Ball=1, Strike=0)
	runStep(t, ctxA, "Client A: Verify Re-Sync",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		CSOBallCount(&countA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countA != 1 {
				return fmt.Errorf("Client A: Expected 1 ball (from server), got %d", countA)
			}
			return nil
		}),
		CSOStrikeCount(&countA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countA != 0 {
				return fmt.Errorf("Client A: Expected 0 strikes (local discarded), got %d", countA)
			}
			return nil
		}),
	)
}

func TestForkConflict(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	// 1. Setup Contexts
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, *withChromeDP)
	defer cancel()

	opts := []chromedp.ContextOption{
		chromedp.WithLogf(log.Printf),
		chromedp.WithErrorf(log.Printf),
	}

	ctxA, cancelA := chromedp.NewContext(allocCtx, opts...)
	defer cancelA()

	ctxB, cancelB := chromedp.NewContext(allocCtx, opts...)
	defer cancelB()

	// Error Listener
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

	var gameID, newGameID string
	var countA, countB int
	var conflictVisible bool

	// 2. Client A: Init & Login & Create Game
	runStep(t, ctxA, "Client A: Navigate & Login & Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctxA, baseURL); err != nil {
				return err
			}
			id, err := CreateGame(ctxA, "ForkAway", "ForkHome")
			gameID = id
			return err
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)
	t.Logf("Original Game ID: %s", gameID)

	// 3. Client B: Join Game
	runStep(t, ctxB, "Client B: Join Game",
		chromedp.Navigate(fmt.Sprintf(`%s/#game/%s`, baseURL, gameID)),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		chromedp.WaitVisible(`#scoresheet-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)

	// 4. Client A: Go Offline
	runStep(t, ctxA, "Client A: Go Offline",
		chromedp.Evaluate(`app.sync.disconnect(true)`, nil),
		chromedp.Sleep(100*time.Millisecond),
	)

	// 5. Client B: Record Ball (Server Rev 1)
	runStep(t, ctxB, "Client B: Record Ball",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return RecordPitch(ctx, "ball")
		}),
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// 6. Client A: Record Strike (Local Rev 1' -> Divergent)
	runStep(t, ctxA, "Client A: Record Strike (Offline)",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return RecordPitch(ctx, "strike")
		}),
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// 7. Client A: Reload to trigger Reconnect & Sync
	runStep(t, ctxA, "Client A: Reload",
		chromedp.Reload(),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
	)

	// 8. Client A: Verify Conflict Modal
	runStep(t, ctxA, "Client A: Verify Conflict Modal",
		chromedp.WaitVisible(`#conflict-resolution-modal`),
		chromedp.Evaluate(`!document.getElementById('conflict-resolution-modal').classList.contains('hidden')`, &conflictVisible),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if !conflictVisible {
				return fmt.Errorf("Conflict modal did not appear")
			}
			return nil
		}),
	)

	// 9. Client A: Resolve (Fork)
	// We choose to save our local state as a NEW game.
	runStep(t, ctxA, "Client A: Fork",
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Click(`#btn-conflict-fork`),
		waitUntilDisplayNone(`#conflict-resolution-modal`),
		chromedp.Sleep(500*time.Millisecond), // Wait for navigation
		chromedp.Evaluate(`window.location.hash.substring(6)`, &newGameID),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if newGameID == "" || newGameID == gameID {
				return fmt.Errorf("Expected new game ID, got %q", newGameID)
			}
			t.Logf("New Game ID (Fork): %s", newGameID)
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)

	// 10. Client A: Verify State (Strike preserved)
	runStep(t, ctxA, "Client A: Verify Forked State",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		CSOStrikeCount(&countA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countA != 1 {
				return fmt.Errorf("Client A: Expected 1 strike (preserved), got %d", countA)
			}
			return nil
		}),
		CSOBallCount(&countA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countA != 0 {
				return fmt.Errorf("Client A: Expected 0 balls (divergent), got %d", countA)
			}
			return nil
		}),
	)

	// 11. Client B: Verify Original State (Ball preserved)
	runStep(t, ctxB, "Client B: Verify Original State",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		CSOBallCount(&countB),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countB != 1 {
				return fmt.Errorf("Client B: Expected 1 ball, got %d", countB)
			}
			return nil
		}),
	)
}

func TestServerOverwriteConflict(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	// 1. Setup Contexts
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, *withChromeDP)
	defer cancel()

	opts := []chromedp.ContextOption{
		chromedp.WithLogf(log.Printf),
		chromedp.WithErrorf(log.Printf),
	}

	ctxA, cancelA := chromedp.NewContext(allocCtx, opts...)
	defer cancelA()

	ctxB, cancelB := chromedp.NewContext(allocCtx, opts...)
	defer cancelB()

	var gameID string
	var countA, countB int

	// 1. Client A: Create Game
	runStep(t, ctxA, "Client A: Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			id, err := CreateGame(ctx, "TeamA", "TeamB")
			gameID = id
			return err
		}),
		chromedp.WaitVisible(`#scoresheet-view`),
	)

	// 2. Client B: Join Game
	runStep(t, ctxB, "Client B: Join Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			return chromedp.Navigate(fmt.Sprintf("%s/#game/%s", baseURL, gameID)).Do(ctx)
		}),
		chromedp.WaitVisible(`#scoresheet-view`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)

	// 3. Client A: Go Offline
	runStep(t, ctxA, "Client A: Go Offline",
		chromedp.Evaluate(`app.sync.disconnect(true)`, nil),
		chromedp.Sleep(100*time.Millisecond),
	)

	// 4. Client B: Record Ball (Server State)
	runStep(t, ctxB, "Client B: Record Ball",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return RecordPitch(ctx, "ball")
		}),
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// 5. Client A: Record Strike (Local State - Divergent)
	runStep(t, ctxA, "Client A: Record Strike (Offline)",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return RecordPitch(ctx, "strike")
		}),
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	// 6. Client A: Reload to trigger Conflict
	runStep(t, ctxA, "Client A: Reload",
		chromedp.Reload(),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		chromedp.WaitVisible(`#conflict-resolution-modal`),
	)

	// 7. Client A: Resolve via "Overwrite Server"
	runStep(t, ctxA, "Client A: Overwrite Server",
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Click(`#btn-conflict-force-save`),
		waitUntilDisplayNone(`#conflict-resolution-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)

	// 8. Client A: Verify State (Strike is there)
	runStep(t, ctxA, "Client A: Verify State",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		CSOStrikeCount(&countA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countA != 1 {
				return fmt.Errorf("Client A: Expected 1 strike, got %d", countA)
			}
			return nil
		}),
		CSOBallCount(&countA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countA != 0 {
				return fmt.Errorf("Client A: Expected 0 balls (server discarded), got %d", countA)
			}
			return nil
		}),
	)

	// 9. Client B: Perform Action -> Triggers Conflict (since server history changed)
	runStep(t, ctxB, "Client B: Action -> New Conflict",
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return RecordPitch(ctx, "ball")
		}),
		// Action will fail due to revision mismatch, triggering CONFLICT
		chromedp.WaitVisible(`#conflict-resolution-modal`),
	)

	// 10. Client B: Overwrite Local (Catch up to A)
	runStep(t, ctxB, "Client B: Catch up to A",
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Click(`#btn-conflict-overwrite`),
		waitUntilDisplayNone(`#conflict-resolution-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
		// Verify state now matches A
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		CSOStrikeCount(&countB),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if countB != 1 {
				return fmt.Errorf("Client B: Expected 1 strike (from A), got %d", countB)
			}
			return nil
		}),
	)
}
