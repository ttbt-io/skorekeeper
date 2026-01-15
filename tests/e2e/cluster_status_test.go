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
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestClusterStatusEndpoint(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	leaderURL, followerURL, secret := startRaftCluster(t)
	t.Logf("Cluster started. Leader: %s, Follower: %s", leaderURL, followerURL)

	ctx, cancel := chromedp.NewRemoteAllocator(t.Context(), *withChromeDP)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 1. Login to Follower to have a valid session/origin
	if err := Login(ctx, followerURL); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// 2. Query Status on Follower WITH Secret
	var status map[string]any

	success := false
	var lastErr error
	for i := 0; i < 5; i++ {
		cmd := fmt.Sprintf(`(async () => {
			window._statusResult = null;
			try {
				const r = await fetch('/api/cluster/status', {
					headers: { 'X-Raft-Secret': '%s' }
				});
				if (!r.ok) {
					window._statusResult = { error: "HTTP " + r.status };
					return;
				}
				window._statusResult = await r.json();
			} catch (e) {
				window._statusResult = { error: e.toString() };
			}
		})()`, secret)

		err := chromedp.Run(ctx,
			chromedp.Evaluate(cmd, nil),
			chromedp.Poll(`window._statusResult`, &status, chromedp.WithPollingInterval(100*time.Millisecond)),
		)
		if err != nil {
			lastErr = err
			time.Sleep(1 * time.Second)
			continue
		}

		if errMsg, ok := status["error"].(string); ok && errMsg != "" {
			lastErr = fmt.Errorf("API Error: %s", errMsg)
			time.Sleep(1 * time.Second)
			continue
		}

		state, _ := status["state"].(string)
		leaderAddr, _ := status["leaderAddr"].(string)

		if state == "Follower" && leaderAddr != "" {
			// Verify HttpAdvertise is working (leaderAddr should be the public hostname)
			// startRaftCluster sets HttpAdvertise to include "devtest.local" or "127.0.0.1"
			if strings.Contains(leaderAddr, "devtest.local") || strings.Contains(leaderAddr, "127.0.0.1") {
				success = true
				break
			}
			t.Logf("leaderAddr %q does not contain expected host", leaderAddr)
		}

		t.Logf("Attempt %d: state=%v, leaderAddr=%v. Retrying...", i+1, status["state"], status["leaderAddr"])
		time.Sleep(1 * time.Second)
	}

	if !success {
		t.Fatalf("Failed to get valid status after retries. Last error: %v, Last state: %+v", lastErr, status)
	}

	t.Logf("Success! Follower status: %+v", status)

	// 3. Verify unauthorized access
	var jsonContent string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(followerURL+"/api/cluster/status"),
		chromedp.Text(`body`, &jsonContent),
	); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}
	// We expect "Forbidden"
	if jsonContent != "Forbidden\n" {
		t.Errorf("Expected 'Forbidden', got %q", jsonContent)
	}
}
