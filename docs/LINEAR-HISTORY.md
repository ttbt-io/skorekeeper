# Linear Game History Design

This document defines the strategy for transforming the raw, potentially non-linear `actionLog` into a stable, chronological, and authoritative `LinearHistory`. This representation serves as the primary data source for the narrative feed and other history-dependent UI components.

## 1. Data Structure

The `LinearHistory` is an array of `HistoryItem` objects.

### HistoryItem Fields
*   **ctxKey**: The unique identifier for the context (e.g., `Inning-Team-BatterIdx-Column`).
*   **type**: The category of the item (`PLAY`, `SUBSTITUTION`, `INNING_HEADER`, `SUMMARY`).
*   **isStricken**: Boolean. True if this play was subsequently edited or corrected.
*   **isCorrection**: Boolean. True if this item represents a "new version" of a previous play.
*   **events**: An array of discrete occurrences (pitches, runner advancements, play results).
*   **stateBefore**: A snapshot of the game state (outs, runners, score) immediately *before* this item occurred.
*   **stateAfter**: A snapshot of the game state immediately *after* this item occurred (only for non-stricken items).

## 2. The Generation Process

To ensure data integrity, especially when historical plays are edited, the history is generated using the core **Dual-Timeline** pipeline.

### Phase 1: Build Chronological Timeline
The system parses the raw `actionLog` using `buildTimeline` to resolve all `UNDO` tombstones, in-place edits (via `refId`), and timeline insertions (via `insertAfterId`). This produces a single, chronological 1D array of effective generative actions.

### Phase 2: Grouping and Replay
The linear timeline is iterated to build the `LinearHistory` array:
1.  **Plate Appearance Detection**: Actions are grouped into Plate Appearances (PAs) based on their sequential progression.
2.  **Context Contextualization**: The `HistoryManager` groups events under a stable `ctxKey`.

### Phase 3: State Propagation & High-Fidelity Resolution
Iterate through the `LinearHistory` array from beginning to end to calculate and attach game states using the core `gameReducer`:

1.  Maintain a running "Authoritative State" initialized with `getInitialState()` and the game roster.
2.  For every item:
    *   Set `stateBefore` equal to a narrative-mapped snapshot of the current Authoritative State.
    *   **Batter Resolution:** Resolve the batter's name using the roster state at this exact moment in history.
    *   **Augmentation:** Replay the item's events against a temporary clone of the state to resolve runner names (e.g., mapping "Runner on 1st" to "Alice").
3.  Update the running Authoritative State by applying the events via `gameReducer`.
4.  Set `stateAfter` equal to the updated Authoritative State.

## 3. Narrative Feed Integration Strategy

The `LinearHistory` serves as the authoritative view-model source for the narrative feed, enabling stable and transparent UI updates.

### Stable Identification
Each `HistoryItem` is assigned a stable, unique ID derived from its `ctxKey` and a correction sequence number. This allows the UI to perform efficient DOM diffing and maintain the user's scroll position when history is recalculated.

### View-Model Generation
The `NarrativeEngine` is refactored into a stateless transformer:
1.  **Input**: The `LinearHistory` array and current `rosters`.
2.  **Output**: A structured Feed Model (Inning Blocks -> PA Blocks -> Event Lines).
3.  **Context Resolution**: The engine uses the pre-attached `stateBefore` snapshots to generate phrasing (e.g., "Runners on 1st and 3rd") without needing to track state during the render loop.

### Transparent UI Updates ("Smart Refresh")
The `ScoresheetRenderer` implements a key-based update mechanism:
*   **Chronological Re-rendering**: When a play is edited or inserted, the Dual-Timeline pipeline completely recalculates the game state. The renderer updates the feed immediately, naturally reflecting the new chronological order without needing visual hacks like "strike-through" text.
*   **Reactive State Propagation**: If a historical edit changes the context of subsequent plays (e.g., an Out becomes a Hit), the renderer updates the context headers of following plays without re-rendering their entire event lists, minimizing visual churn.
*   **Dispatch Integration**: The app automatically re-generates the narrative whenever the `actionLog` is updated (locally or via sync), ensuring real-time authoritative consistency.
