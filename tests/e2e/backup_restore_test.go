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
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupRestore(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	// Setup context with download behavior
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
			if ev.Type == runtime.APITypeError {
				args := make([]string, len(ev.Args))
				for i, arg := range ev.Args {
					args[i] = string(arg.Value)
				}
				t.Logf("JS CONSOLE ERROR: %s", strings.Join(args, " "))
				t.Fail()
			}
		}
	})

	// Configure downloads to /downloads (mounted volume)
	if err := chromedp.Run(ctx, browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllow).WithDownloadPath("/downloads")); err != nil {
		t.Fatalf("Failed to set download behavior: %v", err)
	}

	// 1. Seed Data
	runStep(t, ctx, "Create Seed Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			_, err := CreateGame(ctx, "BackupAway", "BackupHome")
			return err
		}),
	)

	// 2. Perform Backup
	var backupFile string
	runStep(t, ctx, "Perform Backup",
		chromedp.Click(`#btn-menu-scoresheet`), // Open Sidebar (from Game View)
		chromedp.WaitVisible(`#app-sidebar`),
		chromedp.Sleep(500*time.Millisecond), // Wait for transition
		chromedp.Click(`#sidebar-btn-backup`),
		chromedp.WaitVisible(`#backup-modal`),
		// Disable File System Access API to force Blob download (handled by SetDownloadBehavior)
		chromedp.Evaluate(`window.showSaveFilePicker = null;`, nil),
		// Ensure options are checked
		chromedp.Click(`#btn-start-backup`),
		chromedp.WaitVisible(`#backup-progress`),
		// Wait for download to complete
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Poll /downloads dir for a .jsonl file
			timeout := time.After(30 * time.Second)
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-timeout:
					return fmt.Errorf("Timeout waiting for backup file download")
				case <-ticker.C:
					files, err := os.ReadDir("/downloads")
					if err != nil {
						return err
					}
					for _, f := range files {
						if strings.HasSuffix(f.Name(), ".jsonl") {
							backupFile = filepath.Join("/downloads", f.Name())
							t.Logf("Found backup file: %s", backupFile)
							return nil // Success
						}
					}
				}
			}
		}),
		waitUntilDisplayNone(`#backup-modal`),
	)

	// 2b. Verify Backup File Content
	runStep(t, ctx, "Verify Backup File Content",
		chromedp.ActionFunc(func(ctx context.Context) error {
			content, err := os.ReadFile(backupFile)
			if err != nil {
				return fmt.Errorf("Failed to read backup file: %v", err)
			}
			fileContent := string(content)

			// Check for Header
			if !strings.Contains(fileContent, `"type":"header"`) {
				return fmt.Errorf("Backup file missing header")
			}
			// Check for Game Data
			if !strings.Contains(fileContent, `"type":"game"`) {
				return fmt.Errorf("Backup file missing game record")
			}
			if !strings.Contains(fileContent, `"away":"BackupAway"`) {
				return fmt.Errorf("Backup file missing specific game data (BackupAway)")
			}

			t.Logf("Backup file content verified (%d bytes)", len(fileContent))
			return nil
		}),
	)

	// 3. Wipe Data
	runStep(t, ctx, "Wipe Local Data and Logout",
		chromedp.Evaluate(`
			(async () => {
				console.log('Starting Wipe...');
				// 1. Clear LocalStorage
				localStorage.clear();

				// 2. Unregister Service Workers
				const regs = await navigator.serviceWorker.getRegistrations();
				for (const reg of regs) {
					await reg.unregister();
				}

				// 3. Close DB Connection
				if (window.app && window.app.db && window.app.db.db) {
					window.app.db.db.close();
				}
				
				// 4. Delete Database
				await new Promise((resolve, reject) => {
					const req = indexedDB.deleteDatabase('SkorekeeperDB');
					req.onsuccess = () => {
						console.log('DB Deleted');
						resolve();
					};
					req.onerror = () => reject(req.error);
					req.onblocked = () => {
						console.warn('DB Delete Blocked');
					};
				});
			})()
		`, nil),
		network.ClearBrowserCookies(),
		// Reload to apply changes
		chromedp.Navigate("about:blank"),
		chromedp.Navigate(baseURL),
		chromedp.WaitReady(`body[data-app-ready="true"]`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Verify game list is empty (check for game cards specifically)
			var count int
			err := chromedp.Evaluate(`document.querySelectorAll('#game-list > div[data-game-id]').length`, &count).Do(ctx)
			if err != nil {
				return err
			}
			if count > 0 {
				var html string
				chromedp.OuterHTML(`#game-list`, &html).Do(ctx)
				t.Logf("Unexpected Game List Content: %s", html)
				return fmt.Errorf("Expected 0 games after wipe and logout, found %d", count)
			}
			t.Log("Verified: Dashboard is empty.")
			return nil
		}),
	)

	// 4. Restore
	runStep(t, ctx, "Restore Backup",
		chromedp.Click(`#btn-menu-dashboard`), // Open Sidebar
		chromedp.WaitVisible(`#app-sidebar`),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#sidebar-btn-backup`),
		chromedp.WaitVisible(`#backup-modal`),
		chromedp.Click(`#btn-open-restore`),
		chromedp.WaitVisible(`#restore-modal`),

		// Upload File
		chromedp.SetUploadFiles(`#restore-file-input`, []string{backupFile}),
		chromedp.WaitVisible(`#restore-list-container > div`), // Wait for items to appear

		// Verify list content
		chromedp.ActionFunc(func(ctx context.Context) error {
			var html string
			if err := chromedp.OuterHTML(`#restore-list-container`, &html).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(html, "BackupAway") {
				return fmt.Errorf("Restore list missing expected game 'BackupAway'")
			}
			return nil
		}),

		// Select All and Confirm
		chromedp.Click(`#btn-restore-select-all`),
		chromedp.Click(`#btn-confirm-restore`),
		chromedp.WaitVisible(`#custom-confirm-modal`),         // Success alert
		chromedp.Click(`[data-test="custom-confirm-ok-btn"]`), // Dismiss alert
		waitUntilDisplayNone(`#restore-modal`),
		chromedp.WaitReady(`body[data-app-ready="true"]`), // Wait for reload
	)

	// 4b. Re-Login to see data (since we wiped/logged out in Step 3)
	// Restored game is owned by "test@example.com", so we must match.
	runStep(t, ctx, "Login to verify data",
		chromedp.ActionFunc(func(ctx context.Context) error {
			return LoginWithUser(ctx, baseURL, "test@example.com")
		}),
	)

	// 5. Verification
	runStep(t, ctx, "Verify Restored Data",
		chromedp.WaitVisible(`#game-list`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var html string
			if err := chromedp.OuterHTML(`#game-list`, &html).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(html, "BackupAway") {
				return fmt.Errorf("Dashboard missing restored game 'BackupAway'")
			}
			return nil
		}),
	)
}
