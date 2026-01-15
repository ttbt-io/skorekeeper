# Narrative Engine & Statistics Design Specification

This document details how Skorekeeper transforms raw game events into human-readable stories and comprehensive baseball/softball statistics.

## 1. The Narrative Engine

The Narrative Engine acts as a translator, converting the discrete actions in the log into a chronological play-by-play feed.

### 1.1 State Reconstruction
Because the narrative must be chronological and context-aware (e.g., "Runners on 1st and 3rd"), the engine re-plays the action log internally to maintain a "narrative state" that tracks:
*   Current inning and half (Top/Bottom).
*   Current outs and score.
*   Current runners on base.
*   Active roster and any substitutions.

### 1.2 Narrative Generation Rules
*   **Plate Appearances**: Each PA is grouped as a block, starting with the batter's name and the current game context (outs/runners).
*   **Player Identification**: Generic terms like "Batter" or "Runner" must be replaced with the actual player's name whenever known from the roster state.
*   **Dynamic Flavor Text**: Play descriptions (e.g., hits, outs) utilize a template system with randomized verbs and adjectives based on trajectory and hit type to avoid robotic repetition (e.g., "rips a line drive" vs. "laces a base hit").
*   **High-Leverage Context**: Critical situations (late innings, close score, RISP) trigger a "Narrative Pre-Roll" or "Clutch Alert" to build anticipation before the events are listed.
*   **Detailed Pitch Sequences**: Long at-bats are summarized with "Battle" logic (e.g., highlighting foul balls with two strikes, announcing "Pitch #8").
*   **Smart Score Announcements**: Run scoring events contextualize the impact on the game state (e.g., "Tie Game!", "Lead Change!", "Walk-off Win!") rather than just stating a run scored.
*   **Inning Recaps**: A summary block is generated at the end of each half-inning, detailing runs, hits, and runners left on base (LOB).
*   **Runner Actions**: Base running events (stolen bases, advances on wild pitches) are listed as sub-events within the active PA.
*   **Icons**: Semantic icons are used to provide quick visual cues (⚾ for hits, ❌ for strikeouts, 💎 for runs).

### 1.3 Effective Log Processing
The engine ignores actions that have been "undone" in the log, ensuring the feed always reflects the current authoritative version of the game.

## 2. The Stats Engine

The Stats Engine aggregates data from the Action Log to derive standard baseball/softball metrics for players and teams.

### 2.1 Calculated Metrics
The engine calculates a wide range of statistics, including but not limited to:

#### Hitting
*   **Standard**: PA, AB, R, H, RBI, BB, K, HBP.
*   **Power**: 1B, 2B, 3B, HR.
*   **Efficiency**: AVG (Batting Average), OBP (On-Base Percentage), SLG (Slugging), OPS (OBP + SLG).
*   **Baserunning**: SB (Stolen Bases).

#### Pitching
*   **Workload**: IP (Innings Pitched), BF (Batters Faced), NP (Number of Pitches), Striking Percentage.
*   **Effectiveness**: H, R, ER, BB, K, ERA (Earned Run Average), WHIP.

### 2.2 Derivation Logic
Stats are derived through a single pass over the action log:
1.  **Event Categorization**: Each play result is categorized based on its code (e.g., `BB` is a Walk, not an At-Bat).
2.  **Player Mapping**: Stats are attributed to specific player IDs. The engine handles substitutions by tracking which player was "active" in a roster slot at the time of each action.
3.  **Inning Aggregation**: Stats are also grouped by inning to calculate line scores and team-wide totals.

## 3. Real-Time Updates

Both the Narrative and Stats engines are designed for real-time operation.
*   **Incremental Re-calculation**: Whenever a new action is added to the log, the engines re-process the log to update the UI immediately.
*   **Snapshot Accuracy**: Because the calculation is deterministic, statistics viewed by a spectator will always match those computed by the scorekeeper.

## 4. Implementation Independence

The formulas used (e.g., `AVG = H / AB`) follow standard baseball/softball rules. The engines are agnostic of the underlying storage (IndexedDB vs. Server) and focus strictly on the logic of the Action Log.
