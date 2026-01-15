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

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/ttbt-io/skorekeeper/backend"
	"github.com/ttbt-io/skorekeeper/tools/e2ehelpers"
)

var (
	chromeURL    = flag.String("chrome-url", "", "The url of the remote debugging port")
	outputDir    = flag.String("output-dir", "tools/website-assets/output", "Directory to save screenshots")
	generateOnly = flag.Bool("generate-only", false, "Only generate the demo game JSON and exit")
)

func main() {
	flag.Parse()

	if *generateOnly {
		if err := os.MkdirAll(*outputDir, 0755); err != nil {
			log.Fatalf("Failed to create output dir: %v", err)
		}
		gameJSON, err := constructDemoGame()
		if err != nil {
			log.Fatalf("Failed to construct demo game: %v", err)
		}
		jsonPath := filepath.Join(*outputDir, "demo-game.json")
		if err := os.WriteFile(jsonPath, gameJSON, 0644); err != nil {
			log.Fatalf("Failed to write demo-game.json: %v", err)
		}
		log.Printf("Generated demo-game.json (%d bytes)", len(gameJSON))
		return
	}

	if *chromeURL == "" {
		log.Fatal("--chrome-url must be set")
	}

	baseURL := startServer()
	log.Printf("Server started at %s", baseURL)

	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), *chromeURL)
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx, chromedp.WithLogf(log.Printf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 55*time.Second)
	defer cancel()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output dir: %v", err)
	}

	if err := chromedp.Run(ctx, chromedp.EmulateViewport(768, 1024)); err != nil {
		log.Fatalf("Failed to set viewport: %v", err)
	}

	log.Println("Starting generation...")

	// 1. Construct the Demo Game JSON (Option 3c: PLAY_RESULT with Runner Advancements)
	gameJSON, err := constructDemoGame()
	if err != nil {
		log.Fatalf("Failed to construct demo game: %v", err)
	}

	// Save it
	jsonPath := filepath.Join(*outputDir, "demo-game.json")
	if err := os.WriteFile(jsonPath, gameJSON, 0644); err != nil {
		log.Fatalf("Failed to write demo-game.json: %v", err)
	}
	log.Printf("Generated demo-game.json (%d bytes)", len(gameJSON))

	// 2. Login
	if err := e2ehelpers.Login(ctx, baseURL); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	if err := chromedp.Run(ctx, e2ehelpers.DisableCSSAnimations()); err != nil {
		log.Fatalf("Failed to disable animations: %v", err)
	}

	// 3. Verify the Game Logic
	if err := verifyDemoGame(ctx, baseURL); err != nil {
		log.Fatalf("Verification FAILED: %v", err)
	}
	log.Println("Verification PASSED.")

	// 4. Capture Screenshots
	if err := captureScreenshots(ctx, baseURL); err != nil {
		log.Printf("Screenshot capture failed: %v", err)
	}

	log.Println("Asset generation complete.")
}

func constructDemoGame() ([]byte, error) {
	gameId := "demo-game-001"
	rosters := getHeroSimpleRosters()

	var actions []map[string]interface{}

	addAction := func(aType string, payload map[string]interface{}) {
		actions = append(actions, map[string]interface{}{
			"id":        uuid.New().String(),
			"type":      aType,
			"timestamp": time.Now().UnixMilli(),
			"payload":   payload,
		})
	}

	addSetLead := func(team, colId string, rowId int) {
		addAction("SET_INNING_LEAD", map[string]interface{}{
			"team": team, "colId": colId, "rowId": rowId,
		})
	}

	addPitch := func(team string, b, i int, colId, batterId, pType string) {
		addAction("PITCH", map[string]interface{}{
			"activeCtx":  map[string]interface{}{"i": i, "b": b, "col": colId},
			"activeTeam": team,
			"batterId":   batterId,
			"type":       pType,
		})
	}

	// addPlay updated to accept interface{} for seq
	addPlay := func(team string, b, i int, colId, batterId, res, base, bipType string, seq interface{}, traj string, x, y float64, runners []map[string]interface{}) {
		addAction("PLAY_RESULT", map[string]interface{}{
			"activeCtx":  map[string]interface{}{"i": i, "b": b, "col": colId},
			"activeTeam": team,
			"batterId":   batterId,
			"bipMode":    "normal",
			"bipState": map[string]interface{}{
				"res": res, "base": base, "type": bipType, "seq": seq,
				"hitData": map[string]interface{}{
					"trajectory": traj,
					"location":   map[string]float64{"x": x, "y": y},
				},
			},
			"hitData": map[string]interface{}{
				"trajectory": traj,
				"location":   map[string]float64{"x": x, "y": y},
			},
			"runnerAdvancements": runners,
		})
	}

	makeKey := func(team string, b int, colId string) string {
		return fmt.Sprintf("%s-%d-%s", team, b, colId)
	}

	// 1. GAME_START
	addAction("GAME_START", map[string]interface{}{
		"id":             gameId,
		"away":           "Rockets",
		"home":           "Aviators",
		"date":           "2026-01-01T12:00:00-08:00",
		"location":       "Riverfront Stadium",
		"event":          "DEMO",
		"ownerId":        "DEMO_OWNER",
		"schemaVersion":  3,
		"initialRosters": rosters,
	})

	// --- Top 1 (Rockets) ---
	addSetLead("away", "col-1-0", 0)
	// Speedy 1B
	addPlay("away", 0, 1, "col-1-0", "p1", "Safe", "1B", "HIT", "", "Line", 0.2, 0.3, nil)
	// Flash 2B -> Scores Speedy
	addPlay("away", 1, 1, "col-1-0", "p2", "Safe", "2B", "HIT", "", "Fly", 0.8, 0.1, []map[string]interface{}{
		{"key": makeKey("away", 0, "col-1-0"), "base": 0, "outcome": "Score"},
	})
	// Slugger HR -> Scores Flash
	addPlay("away", 2, 1, "col-1-0", "p3", "Safe", "Home", "HIT", "", "Fly", 0.5, 0.05, []map[string]interface{}{
		{"key": makeKey("away", 1, "col-1-0"), "base": 1, "outcome": "Score"},
	})
	// Cannon K
	addPitch("away", 3, 1, "col-1-0", "p4", "strike")
	addPitch("away", 3, 1, "col-1-0", "p4", "strike")
	addPitch("away", 3, 1, "col-1-0", "p4", "strike")
	// Ace Fly Out
	addPlay("away", 4, 1, "col-1-0", "p5", "Fly", "1B", "OUT", "8", "Fly", 0.5, 0.2, nil)
	// Lefty Ground Out (6-3)
	addPlay("away", 5, 1, "col-1-0", "p6", "Ground", "1B", "OUT", []string{"6", "3"}, "Ground", 0.35, 0.4, nil)

	// --- Bot 1 (Aviators) ---
	addSetLead("home", "col-1-0", 0)
	// Pilot K
	addPitch("home", 0, 1, "col-1-0", "h1", "strike")
	addPitch("home", 0, 1, "col-1-0", "h1", "strike")
	addPitch("home", 0, 1, "col-1-0", "h1", "strike")
	// Wingman 1B
	addPlay("home", 1, 1, "col-1-0", "h2", "Safe", "1B", "HIT", "", "Ground", 0.65, 0.4, nil)
	// Bomber DP -> Wingman Out (Force) (4-6-3)
	addPlay("home", 2, 1, "col-1-0", "h3", "Ground", "1B", "OUT", []string{"4", "6", "3"}, "Ground", 0.6, 0.4, []map[string]interface{}{
		{"key": makeKey("home", 1, "col-1-0"), "base": 0, "outcome": "Out"},
	})

	// --- Top 2 (Rockets) ---
	addSetLead("away", "col-2-0", 6)
	// Stretch Fly Out
	addPlay("away", 6, 2, "col-2-0", "p7", "Fly", "1B", "OUT", "9", "Fly", 0.8, 0.2, nil)
	// Glove 1B
	addPlay("away", 7, 2, "col-2-0", "p8", "Safe", "1B", "HIT", "", "Line", 0.5, 0.3, nil)
	// Rookie K
	addPitch("away", 8, 2, "col-2-0", "p9", "strike")
	addPitch("away", 8, 2, "col-2-0", "p9", "strike")
	addPitch("away", 8, 2, "col-2-0", "p9", "strike")
	// Speedy Ground Out (5-3) -> Glove To 2nd
	addPlay("away", 0, 2, "col-2-0", "p1", "Ground", "1B", "OUT", []string{"5", "3"}, "Ground", 0.3, 0.45, []map[string]interface{}{
		{"key": makeKey("away", 7, "col-2-0"), "base": 0, "outcome": "To 2nd"},
	})

	// --- Bot 2 (Aviators) ---
	addSetLead("home", "col-2-0", 3)
	// Zoom HR
	addPlay("home", 3, 2, "col-2-0", "h4", "Safe", "Home", "HIT", "", "Fly", 0.85, 0.05, nil)
	// Slider K
	addPitch("home", 4, 2, "col-2-0", "h5", "strike")
	addPitch("home", 4, 2, "col-2-0", "h5", "strike")
	addPitch("home", 4, 2, "col-2-0", "h5", "strike")
	// Curve Fly Out
	addPlay("home", 5, 2, "col-2-0", "h6", "Fly", "1B", "OUT", "7", "Fly", 0.2, 0.2, nil)
	// Knuckle Line Out (4)
	addPlay("home", 6, 2, "col-2-0", "h7", "Line", "1B", "OUT", "4", "Line", 0.6, 0.35, nil)

	// --- Top 3 (Rockets) ---
	addSetLead("away", "col-3-0", 1)
	// Flash BB
	addPitch("away", 1, 3, "col-3-0", "p2", "ball")
	addPitch("away", 1, 3, "col-3-0", "p2", "ball")
	addPitch("away", 1, 3, "col-3-0", "p2", "ball")
	addPitch("away", 1, 3, "col-3-0", "p2", "ball")
	// Slugger 2B -> Flash Scores
	addPlay("away", 2, 3, "col-3-0", "p3", "Safe", "2B", "HIT", "", "Line", 0.1, 0.15, []map[string]interface{}{
		{"key": makeKey("away", 1, "col-3-0"), "base": 0, "outcome": "Score"},
	})
	// Cannon Fly Out -> Slugger Stay
	addPlay("away", 3, 3, "col-3-0", "p4", "Fly", "1B", "OUT", "8", "Fly", 0.5, 0.15, nil)
	// Ace K
	addPitch("away", 4, 3, "col-3-0", "p5", "strike")
	addPitch("away", 4, 3, "col-3-0", "p5", "strike")
	addPitch("away", 4, 3, "col-3-0", "p5", "strike")
	// Lefty Ground Out (6-3) -> Slugger Stay
	addPlay("away", 5, 3, "col-3-0", "p6", "Ground", "1B", "OUT", []string{"6", "3"}, "Ground", 0.4, 0.4, nil)

	// --- Bot 3 (Aviators) ---
	addSetLead("home", "col-3-0", 7)
	// Changeup BB
	addPitch("home", 7, 3, "col-3-0", "h8", "ball")
	addPitch("home", 7, 3, "col-3-0", "h8", "ball")
	addPitch("home", 7, 3, "col-3-0", "h8", "ball")
	addPitch("home", 7, 3, "col-3-0", "h8", "ball")
	// Fastball 1B -> Changeup To 2nd
	addPlay("home", 8, 3, "col-3-0", "h9", "Safe", "1B", "HIT", "", "Line", 0.5, 0.25, []map[string]interface{}{
		{"key": makeKey("home", 7, "col-3-0"), "base": 0, "outcome": "To 2nd"},
	})
	// Pilot K
	addPitch("home", 0, 3, "col-3-0", "h1", "strike")
	addPitch("home", 0, 3, "col-3-0", "h1", "strike")
	addPitch("home", 0, 3, "col-3-0", "h1", "strike")

	// Wingman 2B (Double) -> Scores Changeup, Fastball to 3rd
	addPlay("home", 1, 3, "col-3-0", "h2", "Safe", "2B", "HIT", "", "Line", 0.2, 0.1, []map[string]interface{}{
		{"key": makeKey("home", 7, "col-3-0"), "base": 1, "outcome": "Score"},
		{"key": makeKey("home", 8, "col-3-0"), "base": 0, "outcome": "To 3rd"},
	})

	// Bomber Fly Out to SS (3rd Out)
	addPlay("home", 2, 3, "col-3-0", "h3", "Fly", "1B", "OUT", "6", "Fly", 0.45, 0.35, nil)

	// --- Top 4 (Rockets) ---
	addSetLead("away", "col-4-0", 6)
	addPitch("away", 6, 4, "col-4-0", "p7", "strike")
	addPitch("away", 6, 4, "col-4-0", "p7", "strike")
	addPitch("away", 6, 4, "col-4-0", "p7", "strike")
	addPitch("away", 7, 4, "col-4-0", "p8", "strike")
	addPitch("away", 7, 4, "col-4-0", "p8", "strike")
	addPitch("away", 7, 4, "col-4-0", "p8", "strike")
	addPitch("away", 8, 4, "col-4-0", "p9", "strike")
	addPitch("away", 8, 4, "col-4-0", "p9", "strike")
	addPitch("away", 8, 4, "col-4-0", "p9", "strike")

	// --- Bot 4 (Aviators) ---
	addSetLead("home", "col-4-0", 3)
	// Zoom Fly Out (Was scheduled for Bot 3, now leading off Bot 4)
	addPlay("home", 3, 4, "col-4-0", "h4", "Fly", "1B", "OUT", "9", "Fly", 0.9, 0.2, nil)
	// Slider K
	addPitch("home", 4, 4, "col-4-0", "h5", "strike")
	addPitch("home", 4, 4, "col-4-0", "h5", "strike")
	addPitch("home", 4, 4, "col-4-0", "h5", "strike")
	// Curve Fly Out
	addPlay("home", 5, 4, "col-4-0", "h6", "Fly", "1B", "OUT", "7", "Fly", 0.2, 0.2, nil)

	// --- Top 5 (Rockets) ---
	addSetLead("away", "col-5-0", 0)
	addPitch("away", 0, 5, "col-5-0", "p1", "ball")
	addPitch("away", 0, 5, "col-5-0", "p1", "ball")
	addPitch("away", 0, 5, "col-5-0", "p1", "ball")
	addPitch("away", 0, 5, "col-5-0", "p1", "ball")
	addPitch("away", 1, 5, "col-5-0", "p2", "strike")
	addPitch("away", 1, 5, "col-5-0", "p2", "strike")
	addPitch("away", 1, 5, "col-5-0", "p2", "strike")
	addPlay("away", 2, 5, "col-5-0", "p3", "Safe", "1B", "HIT", "", "Line", 0.5, 0.3, []map[string]interface{}{
		{"key": makeKey("away", 0, "col-5-0"), "base": 0, "outcome": "To 2nd"},
	})

	return json.MarshalIndent(map[string]interface{}{
		"id":            gameId,
		"away":          "Rockets",
		"home":          "Aviators",
		"date":          "2026-01-01T12:00:00-08:00",
		"location":      "Riverfront Stadium",
		"event":         "DEMO",
		"ownerId":       "DEMO_OWNER",
		"schemaVersion": 3,
		"actionLog":     actions,
	}, "", "  ")
}

func verifyDemoGame(ctx context.Context, baseURL string) error {
	log.Println("Verifying generated demo game...")
	return chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(baseURL + "/#game/demo"),
		chromedp.WaitVisible(".grid-cell", chromedp.ByQuery),
		chromedp.Sleep(1000 * time.Millisecond),
		chromedp.Evaluate(`
			(() => {
				const game = app.state.activeGame;
				if (!game) throw new Error("Game not loaded in app state");
				
				const stats = app.calculateStats();
				console.log("Verified Rockets Score:", stats.score.away.R);
				console.log("Verified Aviators Score:", stats.score.home.R);

				if (stats.score.away.R !== 4) throw new Error("Rockets score mismatch: expected 4, got " + stats.score.away.R);
				if (stats.score.home.R !== 2) throw new Error("Aviators score mismatch: expected 2, got " + stats.score.home.R);
				
				console.log("Verification checks successful");
			})()
		`, nil),
	})
}

func captureScreenshots(ctx context.Context, baseURL string) error {
	log.Println("Capturing screenshots...")

	// Hero Scorecard (Use Real Demo Game)
	err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(baseURL + "/#game/demo-game-001"),
		chromedp.WaitVisible(".grid-cell", chromedp.ByQuery),
		chromedp.Evaluate(`
			app.state.activeTeam = 'away';
			app.state.activeCtx = { i: 5, b: 3, col: 'col-5-0' };
			app.render();
		`, nil),
		captureScreenshot("website-hero-scorecard.png"),
	})
	if err != nil {
		return err
	}

	// Season Stats (Use Mock Data for ideal visualization)
	mockAggStats := map[string]interface{}{
		"players": map[string]map[string]interface{}{
			"p3": {"id": "p3", "name": "Slugger", "pa": 120, "ab": 100, "h": 45, "singles": 20, "doubles": 10, "triples": 0, "hr": 15, "r": 35, "rbi": 50, "bb": 18, "k": 20, "hbp": 2, "sf": 0, "sh": 0, "sb": 5, "flyouts": 15, "lineouts": 10, "groundouts": 10, "otherOuts": 0, "roe": 2, "calledStrikes": 15, "games": 25},
			"p1": {"id": "p1", "name": "Speedy", "pa": 125, "ab": 110, "h": 38, "singles": 30, "doubles": 5, "triples": 3, "hr": 0, "r": 40, "rbi": 15, "bb": 15, "k": 10, "hbp": 0, "sf": 0, "sh": 0, "sb": 25, "flyouts": 25, "lineouts": 15, "groundouts": 22, "otherOuts": 0, "roe": 5, "calledStrikes": 8, "games": 25},
			"p5": {"id": "p5", "name": "Ace", "pa": 115, "ab": 105, "h": 32, "singles": 20, "doubles": 8, "triples": 1, "hr": 3, "r": 20, "rbi": 25, "bb": 8, "k": 15, "hbp": 2, "sf": 0, "sh": 0, "sb": 2, "flyouts": 20, "lineouts": 12, "groundouts": 26, "otherOuts": 0, "roe": 1, "calledStrikes": 12, "games": 24},
			"p2": {"id": "p2", "name": "Flash", "pa": 118, "ab": 108, "h": 28, "singles": 20, "doubles": 6, "triples": 2, "hr": 0, "r": 25, "rbi": 18, "bb": 10, "k": 18, "hbp": 0, "sf": 0, "sh": 0, "sb": 12, "flyouts": 22, "lineouts": 10, "groundouts": 30, "otherOuts": 0, "roe": 3, "calledStrikes": 10, "games": 25},
			"p9": {"id": "p9", "name": "Rookie", "pa": 50, "ab": 45, "h": 15, "singles": 10, "doubles": 3, "triples": 0, "hr": 2, "r": 8, "rbi": 10, "bb": 5, "k": 12, "hbp": 0, "sf": 0, "sh": 0, "sb": 1, "flyouts": 8, "lineouts": 4, "groundouts": 6, "otherOuts": 0, "roe": 1, "calledStrikes": 14, "games": 12},
		},
		"pitchers": map[string]map[string]interface{}{
			"p5": {"id": "p5", "name": "Ace", "ipOuts": 150, "h": 35, "bb": 15, "er": 10, "k": 60, "hbp": 2, "pitches": 750, "strikes": 500, "balls": 250, "bf": 200, "defensiveOuts": 90, "errors": 1, "games": 10},
			"h9": {"id": "h9", "name": "Fastball", "ipOuts": 120, "h": 45, "bb": 20, "er": 18, "k": 45, "hbp": 3, "pitches": 650, "strikes": 400, "balls": 250, "bf": 180, "defensiveOuts": 75, "errors": 3, "games": 8},
			"p9": {"id": "p9", "name": "Rookie", "ipOuts": 40, "h": 15, "bb": 5, "er": 5, "k": 12, "hbp": 1, "pitches": 200, "strikes": 130, "balls": 70, "bf": 60, "defensiveOuts": 28, "errors": 0, "games": 4},
		},
		"teams": map[string]map[string]interface{}{
			"t1": {"w": 20, "l": 5, "t": 0, "rs": 150, "ra": 50, "games": 25, "name": "Rockets"},
			"t2": {"w": 18, "l": 7, "t": 0, "rs": 130, "ra": 80, "games": 25, "name": "Aviators"},
			"t3": {"w": 10, "l": 15, "t": 0, "rs": 90, "ra": 110, "games": 25, "name": "Dragons"},
			"t4": {"w": 2, "l": 23, "t": 0, "rs": 40, "ra": 180, "games": 25, "name": "Wolves"},
		},
	}
	statsJSON, _ := json.Marshal(mockAggStats)

	err = chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(baseURL + "/#stats"),
		chromedp.WaitVisible("#stats-content", chromedp.ByQuery),
		chromedp.Evaluate(fmt.Sprintf(`
			// Disable the automatic load to prevent overwriting our mock data
			const originalLoad = app.loadStatisticsView;
			app.loadStatisticsView = async () => { app.state.view = 'statistics'; };

			app.state.aggregatedStats = %s;
			app.state.allGames = [
				{ id: 'g1', away: 'Rockets', home: 'Aviators', awayTeamId: 't1', homeTeamId: 't2', status: 'final' },
				{ id: 'g2', away: 'Dragons', home: 'Wolves', awayTeamId: 't3', homeTeamId: 't4', status: 'final' }
			];
			app.render();
			
			// Restore after render
			app.loadStatisticsView = originalLoad;
		`, statsJSON), nil),
		chromedp.Sleep(1000 * time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var hasContent bool
			err := chromedp.Evaluate(`document.querySelectorAll("tbody tr").length > 0 && document.body.innerText.includes("Slugger")`, &hasContent).Do(ctx)
			if err != nil {
				return err
			}
			if !hasContent {
				return fmt.Errorf("Season Stats verification failed: No rows or missing 'Slugger'")
			}
			return nil
		}),
		captureScreenshot("website-season-stats.png"),
	})
	if err != nil {
		return err
	}

	// Player Profile (Use Mock Data)
	sprayChart := []map[string]interface{}{
		{"type": "Hit", "x": 165, "y": 45},  // HR RF
		{"type": "Hit", "x": 150, "y": 55},  // HR RF
		{"type": "Hit", "x": 175, "y": 70},  // 2B RF line
		{"type": "Hit", "x": 125, "y": 85},  // 1B RC
		{"type": "Hit", "x": 135, "y": 35},  // HR RC
		{"type": "Out", "x": 100, "y": 45},  // F8
		{"type": "Out", "x": 125, "y": 155}, // 4-3
		{"type": "Out", "x": 165, "y": 135}, // F9
	}
	sprayJSON, _ := json.Marshal(sprayChart)

	gameLog := []map[string]interface{}{
		{"date": "2025-04-01", "opponent": "Aviators", "performance": "3-4, 2 HR, 5 RBI"},
		{"date": "2025-03-28", "opponent": "Dragons", "performance": "1-3, 2B, BB"},
		{"date": "2025-03-25", "opponent": "Wolves", "performance": "2-4, 1B, HR"},
	}
	logJSON, _ := json.Marshal(gameLog)

	err = chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Evaluate(fmt.Sprintf(`
			app.state.aggregatedStats = %s;
			const p = { id: 'p3', name: 'Slugger', number: '3' };
			const stats = app.state.aggregatedStats['players']['p3'];
			
			// Inline StatsEngine.getDerivedHittingStats logic
			const avg = stats.ab > 0 ? stats.h / stats.ab : 0;
			const obpDenom = stats.ab + stats.bb + stats.hbp + stats.sf;
			const obp = obpDenom > 0 ? (stats.h + stats.bb + stats.hbp) / obpDenom : 0;
			const slg = stats.ab > 0 ? (stats.singles + 2 * stats.doubles + 3 * stats.triples + 4 * stats.hr) / stats.ab : 0;
			const derived = { avg: avg.toFixed(3), obp: obp.toFixed(3), slg: slg.toFixed(3), ops: (obp + slg).toFixed(3) };

			const spray = %s;
			const logs = %s;

			document.getElementById('profile-name').textContent = p.name;
			document.getElementById('profile-subtitle').textContent = '#' + p.number + ' • ' + stats.games + ' Games • ' + stats.pa + ' PA • ' + stats.h + ' Hits • ' + stats.hr + ' HR';

			const card = document.getElementById('profile-stats-card');
			card.innerHTML = '';
			const items = [
				['AVG', derived.avg], ['OPS', derived.ops],
				['Hits', stats.h], ['HR', stats.hr], ['RBI', stats.rbi], ['AB', stats.ab]
			];
			items.forEach(([label, val]) => {
				const div = document.createElement('div');
				div.className = 'text-center';
				div.innerHTML = '<div class="text-xs text-gray-500 uppercase">' + label + '</div><div class="text-xl font-bold">' + val + '</div>';
				card.appendChild(div);
			});

			// Detailed Hitting Breakdown
			const hittingBreakdown = document.createElement('div');
			hittingBreakdown.className = 'mt-6 profile-breakdown';
			hittingBreakdown.innerHTML = '<h4 class="text-sm font-bold text-gray-400 uppercase tracking-widest mb-2">Hitting Breakdown</h4>';
			const hGrid = document.createElement('div');
			hGrid.className = 'grid grid-cols-3 gap-2';
			[
				['K', stats.k], ['BB', stats.bb], ['HBP', stats.hbp],
				['ROE', stats.roe], ['Fly', stats.flyouts], ['Line', stats.lineouts],
				['Gnd', stats.groundouts], ['Other', stats.otherOuts], ['CS', stats.calledStrikes]
			].forEach(([l, v]) => {
				const div = document.createElement('div');
				div.className = 'bg-white p-2 rounded border border-gray-100 text-center';
				div.innerHTML = '<div class="text-[10px] text-gray-400 uppercase">' + l + '</div><div class="text-sm font-bold text-gray-700">' + v + '</div>';
				hGrid.appendChild(div);
			});
			hittingBreakdown.appendChild(hGrid);
			
			// Remove any previous ones
			const prevBreakdowns = card.parentElement.querySelectorAll('.profile-breakdown');
			prevBreakdowns.forEach(b => b.remove());
			
			card.parentElement.appendChild(hittingBreakdown);

			const tbody = document.getElementById('profile-game-log');
			tbody.innerHTML = '';
			logs.forEach(g => {
				const tr = document.createElement('tr');
				tr.innerHTML = '<td class="p-2 text-gray-700 whitespace-nowrap">' + g.date + '</td><td class="p-2 font-bold">' + g.opponent + '</td><td class="p-2 text-center">' + g.performance + '</td>';
				tbody.appendChild(tr);
			});

			const container = document.getElementById('spray-markers');
			if (container) {
				container.innerHTML = '';
				const SVG_NS = 'http://www.w3.org/2000/svg';
				spray.forEach(pt => {
					const circle = document.createElementNS(SVG_NS, 'circle');
					circle.setAttribute('cx', pt.x);
					circle.setAttribute('cy', pt.y);
					circle.setAttribute('r', 3);
					if (pt.type === 'Hit') {
						circle.setAttribute('fill', '#4ade80'); // Green
					} else {
						circle.setAttribute('fill', '#ef4444'); // Red
					}
					container.appendChild(circle);
				});
			}

			document.getElementById('player-profile-modal').classList.remove('hidden');
		`, statsJSON, sprayJSON, logJSON), nil),
		chromedp.Sleep(500 * time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var name string
			err := chromedp.Evaluate(`document.getElementById("profile-name").textContent`, &name).Do(ctx)
			if err != nil {
				return err
			}
			if name != "Slugger" {
				return fmt.Errorf("Player Profile verification failed: Expected 'Slugger', got '%s'", name)
			}
			return nil
		}),
		captureScreenshot("website-player-profile.png"),
	})
	return err
}

func getHeroSimpleRosters() map[string][]map[string]string {
	return map[string][]map[string]string{
		"away": {
			{"id": "p1", "name": "Speedy", "number": "1", "pos": "SS"},
			{"id": "p2", "name": "Flash", "number": "2", "pos": "2B"},
			{"id": "p3", "name": "Slugger", "number": "3", "pos": "1B"},
			{"id": "p4", "name": "Cannon", "number": "4", "pos": "RF"},
			{"id": "p5", "name": "Ace", "number": "5", "pos": "CF"},
			{"id": "p6", "name": "Lefty", "number": "6", "pos": "LF"},
			{"id": "p7", "name": "Stretch", "number": "7", "pos": "3B"},
			{"id": "p8", "name": "Glove", "number": "8", "pos": "C"},
			{"id": "p9", "name": "Rookie", "number": "9", "pos": "P"},
		},
		"home": {
			{"id": "h1", "name": "Pilot", "number": "11", "pos": "CF"},
			{"id": "h2", "name": "Wingman", "number": "12", "pos": "SS"},
			{"id": "h3", "name": "Bomber", "number": "13", "pos": "1B"},
			{"id": "h4", "name": "Zoom", "number": "14", "pos": "LF"},
			{"id": "h5", "name": "Slider", "number": "15", "pos": "RF"},
			{"id": "h6", "name": "Curve", "number": "16", "pos": "3B"},
			{"id": "h7", "name": "Knuckle", "number": "17", "pos": "2B"},
			{"id": "h8", "name": "Changeup", "number": "18", "pos": "C"},
			{"id": "h9", "name": "Fastball", "number": "19", "pos": "P"},
		},
	}
}

func startServer() string {
	cert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("Failed to generate cert: %v", err)
	}
	dataDir := os.TempDir()
	s := storage.New(dataDir, nil)
	store := backend.NewGameStore(dataDir, s)
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	go backend.StartServer(backend.Options{
		Listener:    l,
		Cert:        cert,
		UseMockAuth: true,
		Debug:       false,
		GameStore:   store,
	})
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return fmt.Sprintf("https://devtest.local:%s", port)
}

func generateSelfSignedCert() (*tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Test Org"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "devtest", "devtest.local"},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.X509KeyPair(crtPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func captureScreenshot(filename string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		var buf []byte
		if err := chromedp.CaptureScreenshot(&buf).Do(ctx); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(*outputDir, filename), buf, 0644)
	}
}
