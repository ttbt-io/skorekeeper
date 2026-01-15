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

func TestOfflineSyncConflictRepro(t *testing.T) {
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
		}
	})

	var conflictVisible bool

	runStep(t, ctx, "Create Game and Wait for Sync",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "OfflineAway", "OfflineHome")
			return err
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}),
	)

	runStep(t, ctx, "Go Offline (Disconnect SyncManager)",
		chromedp.Evaluate(`app.sync.disconnect(true)`, nil), // true = stop retrying
		chromedp.Sleep(100*time.Millisecond),
	)

	runStep(t, ctx, "Perform Offline Action (Ball)",
		chromedp.Click(`.grid-cell`), // Open CSO
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.Click(`#btn-ball`), // Record Ball
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Click(`#btn-close-cso`),
		waitUntilDisplayNone(`#cso-modal`),
	)

	runStep(t, ctx, "Reload Page (Simulate Reconnect/Refresh)",
		chromedp.Reload(),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return WaitForSync(ctx)
		}), // Wait for sync to re-establish
	)

	runStep(t, ctx, "Check for Conflict Modal",
		chromedp.Evaluate(`!document.getElementById('conflict-resolution-modal').classList.contains('hidden')`, &conflictVisible),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if conflictVisible {
				return fmt.Errorf("Conflict modal appeared unexpectedly!")
			}
			t.Log("No conflict detected.")
			return nil
		}),
	)
}
