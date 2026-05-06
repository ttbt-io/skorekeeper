# Dual-Timeline Architecture Plan

This document outlines the strategy for refactoring the state management of Skorekeeper to distinguish between the **Scorekeeper Timeline** (Action Log) and the **Game Timeline** (Logical History). This enables robust editing and insertion of events in the past without breaking the grid-based UI or E2E tests.

## 1. The Problem
Currently, the application state uses grid coordinates (`activeCtx`: batter index, inning, column ID) as the primary key for all game events.
*   **Grid-Centric Coupling:** If a scorekeeper needs to insert a missed batter in the 2nd inning, all subsequent batters in the entire game must have their `activeCtx` shifted. This is extremely brittle and complex.
*   **The Goal:** The grid should be a *projection* (a view) of the game, not the foundational data model. The foundational model should be a chronological timeline of baseball events.

## 2. The Core Concept: Dual Timelines
*   **The Scorekeeper Timeline (Action Log):** An append-only list of actions taken by the user (e.g., "Added a pitch", "Fixed an error from inning 1"). This is our synchronization primitive.
*   **The Game Timeline (Logical History):** A 1D, chronological array of **Plate Appearances (PAs)** and **Game Events**. This represents what actually happened on the field.

## 3. The New Reducer Pipeline
We will refactor `computeStateFromLog` (and `gameReducer`) to process state in three distinct phases:

### Phase 1: Build the Game Timeline
Parse the `actionLog` to build the 1D chronological array of `PlateAppearance` objects.
*   **Stable IDs:** Each PA and Game Event will have a stable ID (derived from the action that created it).
*   **Edits & Insertions:** Instead of complex shifting, a new action in the log can say "Insert this pitch after Event X" or "Replace the outcome of PA Y". Phase 1 processes these instructions to assemble the correct 1D array.

### Phase 2: Derive the Logical Baseball State
Iterate through the 1D Game Timeline from start to finish to calculate the baseball state (outs, score, runners on base, current inning).
*   Because the timeline is linear and chronological, this logic becomes straightforward and deterministic.

### Phase 3: Project to the Grid (Backward Compatibility)
Take the 1D Game Timeline and the Logical State, and map them back into the 2D `events` and `columns` structure currently expected by the UI.
*   **Grid Projection:** The system figures out which row (batter index) and column each PA belongs to, automatically handling inning breaks and batting around.
*   **E2E Tests:** By preserving the output shape (`state.events["away-0-col-1-0"]`), existing components, renderers, and E2E tests will continue to work with minimal modifications. The `activeCtx` simply becomes a rendered coordinate rather than a primary key.

## 4. UI and Controller Updates
*   **Stable References:** When a user clicks a cell on the scoresheet to make an edit, the UI will look up the underlying `paId` for that cell.
*   **Dispatching Edits:** The dispatched action will target the `paId` (e.g., `UPDATE_PA_RESULT`) rather than relying on the grid coordinate (`activeCtx`).
*   **Insertions:** We can introduce UI options to "Insert PA Before/After", which will dispatch actions with an `insertAfterId` targeting the Game Timeline.

## 5. Implementation Steps
1.  **Draft Data Models:** Define the internal shapes for `PlateAppearance` and `GameEvent`.
2.  **Timeline Builder (`timeline.js`):** Implement Phase 1 (`buildTimeline(actionLog)`).
3.  **Logical Reducer (`baseballLogic.js`):** Implement Phase 2 to calculate outs, scores, and runners sequentially.
4.  **Grid Projector (`gridProjector.js`):** Implement Phase 3 to output `state.events` and `state.columns`.
5.  **Integration:** Wire the pipeline into `computeStateFromLog` inside `reducer.js`.
6.  **Refactor Action Payloads:** Update UI components to dispatch edits using stable PA/Event IDs.
7.  **Test Verification:** Run the E2E suite to ensure the grid projection behaves identically to the old grid-centric logic for standard flows.