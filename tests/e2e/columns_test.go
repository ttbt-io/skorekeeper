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
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestColumnManagement(t *testing.T) {
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
			if ev.Type == runtime.APITypeError || ev.Type == runtime.APITypeWarning {
				args := make([]string, len(ev.Args))
				for i, arg := range ev.Args {
					args[i] = string(arg.Value)
				}
				t.Logf("JS CONSOLE (%s): %s", ev.Type, strings.Join(args, " "))
			}
		case *runtime.EventExceptionThrown:
			t.Logf("JS EXCEPTION: %s", ev.ExceptionDetails.Text)
		}
	})

	t.Log("Starting game...")
	_, err := LoginAndCreateGame(ctx, baseURL, "Away Team", "Home Team")
	if err != nil {
		t.Fatalf("Failed to start game: %v", err)
	}

	// Helper to count columns
	countColumns := func(ctx context.Context) int {
		var n int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('.grid-cell[data-player-idx="0"]:not(.stats-cell)').length`, &n),
		); err != nil {
			t.Fatalf("Failed to count columns: %v", err)
		}
		return n
	}

	// Helper to switch team
	switchTeam := func(ctx context.Context, team string) error {
		return chromedp.Run(ctx,
			chromedp.Click("#tab-"+team, chromedp.ByID),
			chromedp.Sleep(200*time.Millisecond),
		)
	}

	// Initial State
	initialCols := countColumns(ctx)
	if initialCols != 7 {
		t.Errorf("Expected 7 initial columns, got %d", initialCols)
	}

	// --- TEST 1: Per-Team Addition ---
	t.Log("Adding a column to Inning 1 for Away")
	if err := chromedp.Run(ctx,
		// Open context menu on first header (col-1-0)
		chromedp.Evaluate(`{
			const headers = document.querySelectorAll('.grid-header.cursor-pointer');
			if (headers.length > 0) {
				headers[0].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }));
			}
		}`, nil),
		chromedp.WaitVisible("#column-context-menu", chromedp.ByID),
		chromedp.Sleep(200*time.Millisecond),
		// Click Add Column
		chromedp.Evaluate(`{
			const menu = document.getElementById('column-context-menu');
			const buttons = menu.getElementsByTagName('button');
			for (let b of buttons) {
				if (b.textContent.includes('Add Column')) {
					b.click();
					break;
				}
			}
		}`, nil),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		t.Fatalf("Failed to add column: %v", err)
	}

	// Verify Away has 8 columns
	if n := countColumns(ctx); n != 8 {
		t.Errorf("Expected Away to have 6 columns, got %d", n)
	}

	// Verify Home still has 7
	t.Log("Switching to Home to verify column count")
	if err := switchTeam(ctx, "home"); err != nil {
		t.Fatalf("Failed to switch to home: %v", err)
	}
	if n := countColumns(ctx); n != 7 {
		t.Errorf("Expected Home to have 5 columns, got %d", n)
	}

	// --- TEST 2: Per-Team Removal (Shared Column) ---
	t.Log("Switching back to Away")
	if err := switchTeam(ctx, "away"); err != nil {
		t.Fatalf("Failed to switch to away: %v", err)
	}

	// We have col-1-0 (shared) and col-1-1 (away only).
	// Remove col-1-0. It should succeed for Away (hidden) but stay for Home.
	t.Log("Removing the first column (shared col-1-0) for Away")
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`{
			const headers = document.querySelectorAll('.grid-header.cursor-pointer');
			headers[0].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }));
		}`, nil),
		chromedp.WaitVisible("#column-context-menu", chromedp.ByID),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Evaluate(`{
			const menu = document.getElementById('column-context-menu');
			const buttons = menu.getElementsByTagName('button');
			for (let b of buttons) {
				if (b.textContent.includes('Remove Column')) {
					b.click();
					break;
				}
			}
		}`, nil),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		t.Fatalf("Failed to remove column: %v", err)
	}

	// Verify Away has 5 columns (removed 1)
	if n := countColumns(ctx); n != 7 {
		t.Errorf("Expected Away to have 5 columns after removal, got %d", n)
	}

	// Verify Home still has 5 columns (col-1-0 persists for Home)
	t.Log("Switching to Home to verify col-1-0 persistence")
	if err := switchTeam(ctx, "home"); err != nil {
		t.Fatalf("Failed to switch to home: %v", err)
	}
	if n := countColumns(ctx); n != 7 {
		t.Errorf("Expected Home to have 5 columns, got %d", n)
	}
	// Verify header is '1' (col-1-0)
	var headerText string
	if err := chromedp.Run(ctx, chromedp.Text(`.grid-header.cursor-pointer`, &headerText)); err != nil {
		t.Fatalf("Failed to read header text: %v", err)
	}
	if headerText != "1" {
		t.Errorf("Expected first column to be '1', got %s", headerText)
	}

	// --- TEST 3: Last Column Constraint ---
	t.Log("Switching back to Away")
	if err := switchTeam(ctx, "away"); err != nil {
		t.Fatalf("Failed to switch to away: %v", err)
	}

	// Away currently has col-1-1 as the ONLY column for Inning 1.
	// Attempt to remove it should fail.
	t.Log("Attempting to remove the last remaining column for Inning 1 (col-1-1)")
	if err := chromedp.Run(ctx,
		// Header index 0 is now Inning 1 (col-1-1)
		chromedp.Evaluate(`{
			const headers = document.querySelectorAll('.grid-header.cursor-pointer');
			headers[0].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }));
		}`, nil),
		chromedp.WaitVisible("#column-context-menu", chromedp.ByID),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Evaluate(`{
			const menu = document.getElementById('column-context-menu');
			const buttons = menu.getElementsByTagName('button');
			for (let b of buttons) {
				if (b.textContent.includes('Remove Column')) {
					b.click();
					break;
				}
			}
		}`, nil),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		t.Fatalf("Failed to click Remove Column: %v", err)
	}

	// Count should remain 7
	if n := countColumns(ctx); n != 7 {
		t.Errorf("Expected column count to remain 5 (Last Column Constraint), got %d", n)
	} else {
		t.Log("Verified Last Column Constraint: Column not removed.")
	}

	// --- TEST 4: Data Constraint ---
	t.Log("Testing data constraint on removal")

	// Add another column (col-1-2) so we have 2 again, allowing removal attempt
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`{
			const headers = document.querySelectorAll('.grid-header.cursor-pointer');
			headers[0].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }));
		}`, nil),
		chromedp.WaitVisible("#column-context-menu", chromedp.ByID),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Evaluate(`{
			const menu = document.getElementById('column-context-menu');
			const buttons = menu.getElementsByTagName('button');
			for (let b of buttons) {
				if (b.textContent.includes('Add Column')) {
					b.click();
					break;
				}
			}
		}`, nil),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		t.Fatalf("Failed to add column back: %v", err)
	}

	// Away now has 2 columns for Inning 1.

	// Add data to the last column of Inning 1
	t.Log("Adding data to the new column")
	var newColId string
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const cells = document.querySelectorAll('.grid-cell[data-player-idx="0"]:not(.stats-cell)');
			return cells[1].dataset.colId; // Index 1 is the second column of Inning 1
		})()`, &newColId),
	); err != nil {
		t.Fatalf("Failed to resolve new column ID: %v", err)
	}

	if err := chromedp.Run(ctx,
		// Click cell in second column
		chromedp.Click(`.grid-cell[data-col-id="`+newColId+`"][data-player-idx="0"]`, chromedp.ByQuery),
		chromedp.WaitVisible("#cso-modal", chromedp.ByID),
		chromedp.Click("#btn-ball", chromedp.ByID),
		chromedp.Click("#btn-close-cso", chromedp.ByID),
		waitUntilDisplayNone("#cso-modal"),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		t.Fatalf("Failed to add data to new column: %v", err)
	}

	// Attempt to remove the column (which has data)
	t.Log("Attempting to remove column with data")
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`{
			const headers = document.querySelectorAll('.grid-header.cursor-pointer');
			// Inning 1 is header 0
			headers[0].dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }));
		}`, nil),
		chromedp.WaitVisible("#column-context-menu", chromedp.ByID),
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Evaluate(`{
			const menu = document.getElementById('column-context-menu');
			const buttons = menu.getElementsByTagName('button');
			for (let b of buttons) {
				if (b.textContent.includes('Remove Column')) {
					b.click();
					break;
				}
			}
		}`, nil),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		t.Fatalf("Failed to click Remove Column: %v", err)
	}

	// Count should remain 8
	if n := countColumns(ctx); n != 8 {
		t.Errorf("Expected column count to remain 6 (Data Constraint), got %d", n)
	} else {
		t.Log("Verified Data Constraint: Column not removed.")
	}
}
