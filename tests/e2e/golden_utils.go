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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pmezard/go-difflib/difflib"
)

// VerifyNarrative captures the feed text and compares it to a golden file.
// If UPDATE_GOLDENS is true, it writes the file instead.
func VerifyNarrative(t *testing.T, ctx context.Context, goldenFilename string) {
	// 1. Switch to Feed View
	if err := chromedp.Run(ctx,
		chromedp.Click("#btn-menu-scoresheet"),
		chromedp.WaitVisible("#sidebar-btn-view-feed"),
		chromedp.Click("#sidebar-btn-view-feed"),
		chromedp.WaitVisible("#feed-container"),
	); err != nil {
		t.Fatalf("Failed to switch to feed view: %v", err)
	}

	// 2. Capture Text
	var actual string
	if err := chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second), // Wait for render
		chromedp.Text("#narrative-feed", &actual),
	); err != nil {
		t.Fatalf("Failed to capture feed text: %v", err)
	}

	// Clean up whitespace for consistent comparison
	actual = strings.TrimSpace(actual)
	if len(actual) == 0 {
		t.Fatal("Narrative feed is empty")
	}

	// 3. Determine Golden Path
	// We are running inside /app in container, which is root of repo.
	goldenPath := filepath.Join("tests/e2e/goldens", goldenFilename)

	// 4. Update or Compare
	if os.Getenv("UPDATE_GOLDENS") == "true" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("Failed to create golden directory: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0644); err != nil {
			t.Fatalf("Failed to write golden file %s: %v", goldenPath, err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
	} else {
		expectedBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			if os.IsNotExist(err) {
				t.Errorf("Golden file missing: %s. Run with -update-goldens to create it.\nActual Content:\n%s", goldenPath, actual)
				return
			}
			t.Fatalf("Failed to read golden file %s: %v", goldenPath, err)
		}
		expected := string(expectedBytes)
		expected = strings.TrimSpace(expected)

		if actual != expected {
			diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(expected),
				B:        difflib.SplitLines(actual),
				FromFile: "Expected",
				ToFile:   "Actual",
				Context:  3,
			})
			t.Errorf("Narrative mismatch for %s:\n%s", goldenFilename, diff)
		}
	}
}
