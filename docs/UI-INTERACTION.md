# UI & Interaction Design Specification

This document details the user interface principles and interaction models that enable efficient game recording and consumption in Skorekeeper.

## 1. The Scoresheet Grid

The scoresheet is the primary interface for game management, modeled after traditional paper scoresheets but enhanced with digital capabilities.

### 1.1 Structural Layout
*   **Rows**: Represent the batting order (Slots 1-9, plus any extra hitters).
*   **Columns**: Represent game innings.
*   **Cells**: The intersection of a batter and an inning. Each cell represents a single Plate Appearance (PA).

### 1.2 Visual Language
*   **Empty Cells**: Indicate pending or future at-bats.
*   **Active Cell**: Highlighted with a pulse or distinct border, indicating where the next action will be recorded.
*   **Completed Cells**: Display a summary of the PA (e.g., the final outcome code like `1B` or `K`) and a visualization of the diamond showing runner movement.

## 2. Contextual Scoring Overlay (CSO)

The CSO is a modal interface that appears when a cell is selected. It is designed for fast, thumb-friendly input.

### 2.1 State-Driven Options
The CSO adapts its buttons and menus based on the current state of the PA and the game:
*   **Count Management**: Standard buttons for Ball, Strike, and Foul.
*   **Auto-Calculations**: The system automatically triggers outcomes like Walk (4 balls) or Strikeout (3 strikes).
*   **In-Play Transition**: Clicking "Ball in Play" opens the detailed BiP sub-view.

### 2.2 Adaptive Menus
*   **Long-Press/Right-Click**: Provides advanced options (e.g., Dropped 3rd Strike, Batter Out of Order, Catcher's Interference) without cluttering the primary interface.
*   **Smart Defaults**: Pre-selects the most likely next action based on context (e.g., defaulting to a "Single" when recording a hit).

## 3. Ball-in-Play (BiP) Interaction

The BiP sub-view handles the complex logic of batted balls.
*   **Spray Chart mapping**: Users can tap on a field diagram to record hit location and trajectory.
*   **Defensive Sequence**: A numeric keypad (positions 1-9) allows for rapid entry of defensive plays (e.g., `6-4-3`).
*   **Validation**: The interface prevents impossible selections based on current rules (e.g., you cannot select "Sacrifice Fly" if there are no runners on base).

## 4. Runner Advancement Flow

A distinct interaction layer handles multi-runner scenarios.
*   **Intercept Pattern**: When a play ends, if runners are on base, the system interrupts the flow with an "Update Runners" screen.
*   **Predictive Defaults**: Based on the hit type (e.g., a Double), the system suggests advancements for all runners.
*   **Manual Override**: Users can cycle through outcomes (Stay, Advance, Out) or use a context menu for specific actions (Stolen Base, Caught Stealing).

## 5. Responsive Design Principles

### 5.1 Mobile-First
*   Large touch targets for all critical scoring actions.
*   Side-menus and overlays to maximize usable screen real estate.
*   Sticky headers for scoreboards and navigation.

### 5.2 Accessibility
*   **Color Semantics**: Consistent use of color (Green for Safe/Positive, Red for Out/Negative, Yellow for Error/Warning).
*   **Redundancy**: Critical information is conveyed through both icons and text.
*   **Touch/Mouse Parity**: All critical interactions (like context menus) are accessible via both long-press (touch) and right-click (mouse).

## 6. Real-Time Feedback

*   **Optimistic Updates**: The UI responds instantly to inputs, with background synchronization handled transparently.
*   **Status Indicators**: Real-time visual feedback on synchronization status and network health.
*   **Narrative Synchronization**: The play-by-play feed updates in real-time as actions are recorded, providing immediate confirmation of the recorded event.

## 7. Team Management

Team management uses a hierarchical navigation model to balance quick access with deep editing.

### 7.1 Navigation Flow
*   **Teams List**: High-level summary cards for all accessible teams.
*   **Team Screen**: Detailed view of a specific team, featuring tabs for **Roster** and **Members**.
*   **Edit Modal**: A dedicated interface for modifying team metadata, roster rows, and member permissions.

### 7.2 The Team Screen
*   **Roster Tab**: Lists all players with their jersey numbers and default positions. Clicking a player opens their **Player Profile** (Stats).
*   **Members Tab**: Displays all users with access to the team (Admins, Scorekeepers, Spectators).
*   **View Stats Link**: Provides a direct shortcut to the Statistics view, automatically filtered for the current team using its unique ID.

