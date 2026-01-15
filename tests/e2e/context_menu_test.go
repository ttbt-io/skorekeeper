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
	"log"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestContextMenuTimer(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	// Create a new allocator context
	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	// Create a new browser context
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithErrorf(log.Printf),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Start Game
	if _, err := LoginAndCreateGame(ctx, baseURL, "Away", "Home"); err != nil {
		t.Fatalf("Failed to start game: %v", err)
	}

	t.Log("Game started, testing Context Menu Timer")

	// 1. Open Context Menu
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`{
			const headers = document.querySelectorAll('.grid-header.cursor-pointer');
			if (headers.length > 0) {
				headers[0].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }));
			}
		}`, nil),
		chromedp.WaitVisible("#column-context-menu", chromedp.ByID),
	); err != nil {
		t.Fatalf("Failed to open context menu: %v", err)
	}

	// 2. Simulate Mouse Enter (Hover)
	t.Log("Simulating MouseEnter (Hover) on context menu")
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`{
			const menu = document.getElementById('column-context-menu');
			menu.dispatchEvent(new MouseEvent('mouseenter'));
		}`, nil),
	); err != nil {
		t.Fatalf("Failed to dispatch mouseenter: %v", err)
	}

	// 3. Wait for > 3 seconds (Default timer is 3s)
	t.Log("Waiting 3.5s...")
	time.Sleep(3500 * time.Millisecond)

	// 4. Verify menu is STILL visible
	var isVisible bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`!document.getElementById('column-context-menu').classList.contains('hidden')`, &isVisible),
	); err != nil {
		t.Fatalf("Failed to check menu visibility: %v", err)
	}

	if !isVisible {
		t.Fatal("Context menu closed despite hover/mouseenter!")
	}
	t.Log("Context menu remained open during hover.")

	// 5. Simulate Mouse Leave
	t.Log("Simulating MouseLeave on context menu")
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`{
			const menu = document.getElementById('column-context-menu');
			menu.dispatchEvent(new MouseEvent('mouseleave'));
		}`, nil),
	); err != nil {
		t.Fatalf("Failed to dispatch mouseleave: %v", err)
	}

	// 6. Wait for > 3 seconds
	t.Log("Waiting 3.5s for auto-close...")
	time.Sleep(3500 * time.Millisecond)

	// 7. Verify menu is HIDDEN
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`!document.getElementById('column-context-menu').classList.contains('hidden')`, &isVisible),
	); err != nil {
		t.Fatalf("Failed to check menu visibility: %v", err)
	}

	if isVisible {
		t.Fatal("Context menu did NOT close after mouseleave and timeout!")
	}
	t.Log("Context menu closed automatically.")
}
