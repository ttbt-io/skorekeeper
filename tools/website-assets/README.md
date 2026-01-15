# Website Assets Generator

This tool automates the creation of high-quality, realistic screenshots for the `skorekeeper.org` website and user manual.

## Design Goals

*   **Realism**: Scenarios should look like a real game in progress, not a test harness.
*   **Aesthetic**: Data should be populated to show off the UI density and layout effectively.
*   **Automation**: Assets should be regeneratable with a single command.
*   **Mocking**: Use state injection where possible to avoid brittle or slow UI-driving scripts for complex states (like full season stats).

## Configuration

*   **Viewport**: 768x1024 (iPad Mini Portrait). This aspect ratio shows the responsiveness of the grid and is a good middle ground between phone and desktop.
*   **Output Directory**: `website-assets/output/` (or configurable).

## Scenarios

### 1. The "Hero" Scorecard (Mid-Game)
*   **Context**: A game in progress (Top of the 5th inning).
*   **Teams**: "Rockets" (Away) vs "Aviators" (Home).
*   **State**:
    *   Score: Rockets 4, Aviators 2.
    *   Situation: Runners on 1st and 3rd, 1 Out.
    *   Grid: Populated with a mix of hits (1B, 2B, HR), outs (K, F8, 6-3), and some color (RBI diamonds).
*   **Capture**: The main `scoresheet-view` showing the grid, header, and scoreboard.

### 2. Season Statistics (The "Moneyball" Shot)
*   **Context**: A completed season view.
*   **Data**: Injected `aggregatedStats` with ~15-20 players.
*   **Visuals**:
    *   Sort by AVG or OPS to show high numbers.
    *   Mix of good and average stats for realism.
*   **Capture**: The `statistics-view` table.

### 3. Player Profile & Spray Chart
*   **Context**: Deep dive into a specific player ("Slugger").
*   **Data**: Injected game log and spray chart data.
*   **Visuals**:
    *   Spray chart showing a pull-hitter tendency (lots of dots in RF/RC for a lefty or LF/LC for a righty).
    *   Recent games log populated.
*   **Capture**: The `player-profile-modal`.

## Implementation Details

*   **Language**: Go (using `chromedp` and `e2ehelpers`).
*   **Mocking Strategy**:
    *   For the *Scorecard*, we will script a "fast-forward" function or directly inject a constructed `Game` object into `app.state.activeGame` and trigger a render. Direct injection is preferred for precision and speed.
    *   For *Stats*, we will inject `app.state.aggregatedStats`.
*   **Entry Point**: `tools/website-assets/main.go`.

## Usage

To generate the screenshots, run the following command from the project root:

```bash
./tools/website-assets/generate-assets.sh
```

This will:
1.  Build the generator tool.
2.  Spin up a Docker container with Headless Chrome (simulating an iPad Mini).
3.  Execute the scenarios defined in `main.go`.
4.  Save the resulting PNG files to `tools/website-assets/output/`.

## Generated Assets

*   `website-hero-scorecard.png`: Mid-game scorecard view showing a busy game state.
*   `website-season-stats.png`: Full statistics table with simulated season data.
*   `website-player-profile.png`: Player profile modal with spray chart and game log.
