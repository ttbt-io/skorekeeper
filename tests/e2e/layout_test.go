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
	"math"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestVisualLayout(t *testing.T) {
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
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var tasks []chromedp.Action

	// 1. Setup Game
	tasks = append(tasks,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := LoginAndCreateGame(ctx, baseURL, "LayoutAway", "LayoutHome")
			return err
		}),
	)

	// 2. Mobile-First Touch Targets (>= 44px)
	// Checking a few critical buttons
	buttons := []string{"#btn-menu-scoresheet", "#btn-undo", "#btn-redo"}
	for _, sel := range buttons {
		tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
			var w, h float64
			err := chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const rect = document.querySelector('%s').getBoundingClientRect();
					return [rect.width, rect.height];
				})()
			`, sel), &[]interface{}{&w, &h}).Do(ctx)
			if err != nil {
				return err
			}
			if w < 44 || h < 44 {
				return fmt.Errorf("Element %s dimensions %.2fx%.2f are less than 44px", sel, w, h)
			}
			return nil
		}))
	}

	// 3. Grid Dimensions
	// Lineup Column (Col 1) ~ 120px
	// Using .lineup-cell as proxy for column width
	tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
		var w float64
		err := chromedp.Evaluate(`document.querySelector('.lineup-cell').getBoundingClientRect().width`, &w).Do(ctx)
		if err != nil {
			return err
		}
		// Allow some variance, e.g. 115-125
		if math.Abs(w-120) > 10 {
			return fmt.Errorf("Lineup column width %.2f is not approx 120px", w)
		}
		return nil
	}))

	// Score Cells ~ 65px
	tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
		var w, h float64
		err := chromedp.Evaluate(`
			(() => {
				const rect = document.querySelector('.grid-cell').getBoundingClientRect();
				return [rect.width, rect.height];
			})()
		`, &[]interface{}{&w, &h}).Do(ctx)
		if err != nil {
			return err
		}
		if math.Abs(w-75) > 5 || math.Abs(h-65) > 5 {
			return fmt.Errorf("Score cell dimensions %.2fx%.2f are not approx 75x65px", w, h)
		}
		return nil
	}))

	// 4. Sticky Header Behavior
	// Vertical Scroll: Header top should remain at top of container (or visible)
	// Horizontal Scroll: Lineup left should remain at left of container
	tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
		var initialTop, scrolledTop float64
		var initialLeft, scrolledLeft float64

		// Vertical Scroll Test
		// Get header top
		err := chromedp.Evaluate(`document.querySelector('.grid-header').getBoundingClientRect().top`, &initialTop).Do(ctx)
		if err != nil {
			return err
		}

		// Scroll container down
		err = chromedp.Evaluate(`document.getElementById('scoresheet-grid').scrollTop = 100`, nil).Do(ctx)
		if err != nil {
			return err
		}
		// Wait a bit for scroll/render
		time.Sleep(100 * time.Millisecond)

		// Get header top again
		err = chromedp.Evaluate(`document.querySelector('.grid-header').getBoundingClientRect().top`, &scrolledTop).Do(ctx)
		if err != nil {
			return err
		}

		// Should be roughly same (sticky)
		if math.Abs(initialTop-scrolledTop) > 5 {
			return fmt.Errorf("Header moved significantly on scroll! Initial: %.2f, Scrolled: %.2f", initialTop, scrolledTop)
		}

		// Horizontal Scroll Test
		// Get lineup left
		err = chromedp.Evaluate(`document.querySelector('.lineup-cell').getBoundingClientRect().left`, &initialLeft).Do(ctx)
		if err != nil {
			return err
		}

		// Scroll container right
		err = chromedp.Evaluate(`document.getElementById('scoresheet-grid').scrollLeft = 100`, nil).Do(ctx)
		if err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)

		// Get lineup left again
		err = chromedp.Evaluate(`document.querySelector('.lineup-cell').getBoundingClientRect().left`, &scrolledLeft).Do(ctx)
		if err != nil {
			return err
		}

		if math.Abs(initialLeft-scrolledLeft) > 5 {
			return fmt.Errorf("Lineup moved significantly on scroll! Initial: %.2f, Scrolled: %.2f", initialLeft, scrolledLeft)
		}

		return nil
	}))

	// 5. CSO Modal Layout
	tasks = append(tasks,
		chromedp.Click(`.grid-cell`),
		chromedp.WaitVisible(`#cso-modal`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var mw, mh, vw, vh float64
			err := chromedp.Evaluate(`
				(() => {
					const m = document.getElementById('cso-modal');
					const mr = m.getBoundingClientRect();
					return [mr.width, mr.height, window.innerWidth, window.innerHeight];
				})()
			`, &[]interface{}{&mw, &mh, &vw, &vh}).Do(ctx)
			if err != nil {
				return err
			}
			// Check if modal covers full screen
			if math.Abs(mw-vw) > 2 || math.Abs(mh-vh) > 2 {
				return fmt.Errorf("CSO Modal dimensions %.2fx%.2f do not match viewport %.2fx%.2f", mw, mh, vw, vh)
			}
			return nil
		}),
	)

	if err := chromedp.Run(ctx, tasks...); err != nil {
		t.Fatalf("Visual Layout Test Failed: %v", err)
	}
}
