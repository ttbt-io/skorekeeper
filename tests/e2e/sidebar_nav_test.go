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

func TestSidebarNavigation(t *testing.T) {
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
		}
	})

	if err := e2ehelpers.Login(ctx, baseURL); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Create a new game
	_, err := e2ehelpers.CreateGame(ctx, "SidebarAway", "SidebarHome")
	if err != nil {
		t.Fatalf("Failed to create game: %v", err)
	}

	// Helper to check visibility
	isVisible := func(sel string) bool {
		var display string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`window.getComputedStyle(document.querySelector('%s')).display`, sel), &display),
		); err != nil {
			return false
		}
		return display != "none"
	}

	// Initially Grid should be visible
	if !isVisible("#grid-container") {
		t.Fatal("Grid container should be visible initially")
	}
	if isVisible("#feed-container") {
		t.Fatal("Feed container should be hidden initially")
	}

	// Open Sidebar
	if err := chromedp.Run(ctx, chromedp.Click("#btn-menu-scoresheet")); err != nil {
		t.Fatalf("Failed to open sidebar: %v", err)
	}

	// Click Feed
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible("#sidebar-btn-view-feed"),
		chromedp.Sleep(500*time.Millisecond), // Wait for sidebar animation
		chromedp.Click("#sidebar-btn-view-feed"),
		chromedp.Sleep(500*time.Millisecond), // Wait for transition
	); err != nil {
		t.Fatalf("Failed to click Feed button: %v", err)
	}

	// Feed should be visible
	if !isVisible("#feed-container") {
		t.Fatal("Feed container should be visible")
	}
	if isVisible("#grid-container") {
		t.Fatal("Grid container should be hidden")
	}

	// Sidebar should be closed (backdrop hidden/removed)
	// Check if sidebar has -translate-x-full
	var sidebarClass string
	if err := chromedp.Run(ctx,
		chromedp.AttributeValue("#app-sidebar", "class", &sidebarClass, nil),
	); err != nil {
		t.Fatalf("Failed to get sidebar class: %v", err)
	}
	if !strings.Contains(sidebarClass, "-translate-x-full") {
		t.Errorf("Sidebar should have -translate-x-full class, got: %s", sidebarClass)
	}

	// Open Sidebar again
	if err := chromedp.Run(ctx, chromedp.Click("#btn-menu-scoresheet")); err != nil {
		t.Fatalf("Failed to open sidebar: %v", err)
	}

	// Click Scorecard
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible("#sidebar-btn-view-grid"),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click("#sidebar-btn-view-grid"),
		chromedp.Sleep(200*time.Millisecond),
	); err != nil {
		t.Fatalf("Failed to click Scorecard button: %v", err)
	}

	// Grid should be visible
	if !isVisible("#grid-container") {
		t.Fatal("Grid container should be visible")
	}
	if isVisible("#feed-container") {
		t.Fatal("Feed container should be hidden")
	}
}
