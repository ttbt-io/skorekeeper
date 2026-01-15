# Visual System & UI Design Specification

This document defines the visual design system, user experience (UX) principles, and specific UI component behaviors for the Skorekeeper PWA.

## 1. Visual Design System

### 1.1 Color Palette
The application uses a high-contrast theme combining "Pitch" (dark interface elements) and "Paper" (light scoresheet area).

| Semantic Name | Hex Code | Tailwind Class | Usage |
| :--- | :--- | :--- | :--- |
| **Pitch (Dark)** | `#111827` | `bg-slate-900` | Headers, Modals, Scoreboard |
| **Paper (Light)** | `#fefcfb` | `bg-fefcfb` | Scoresheet Grid Background |
| **Ink (Main)** | `#222222` | `text-gray-900` | Primary Text, Grid Lines |
| **Ink (Red)** | `#dc2626` | `text-red-600` | Strike Buttons, Out Status |
| **Ink (Blue)** | `#2563eb` | `text-blue-600` | Ball Buttons, Primary Actions |
| **Highlight** | `#facc15` | `bg-yellow-400` | Active CSO Dots, Selected Runners |
| **Grass** | `#166534` | `bg-green-800` | Fielding Diamond Background |
| **Dirt** | `#9a6f44` | `fill-[#9a6f44]` | Fielding Diamond Infield |

### 1.2 Typography
*   **Font Family**: System Sans-Serif stack (e.g., `-apple-system`, `Roboto`, `Helvetica`).
*   **Outcome Text**: Extra Bold (`font-black`), `text-2xl` or larger for maximum legibility.
*   **Player Names**: Bold, `text-sm`.
*   **Labels**: Uppercase, tracking-wide, `text-xs`.

## 2. Interface Components

### 2.1 Dashboard & Search
*   **Search Logic**: Filters game cards based on Team Name, Event Name, Location, or Date.
*   **Game Cards**: Grouped chronologically. Finalized games appear with reduced opacity (80%).

### 2.2 Teams & Roster Hydration
*   **Smart Population**: Selecting a saved team for a new game automatically populates the initial lineup with the team's stored names, numbers, and default positions.
*   **Persistent IDs**: Each player has a stable UUID to maintain statistical integrity across multiple games.

### 2.3 The Scoresheet Grid
*   **Cell Geometry**: 65x65px squares.
*   **Visual Layers**:
    1.  **Path Layer (Bottom)**: Inactive paths are `stroke-slate-300`.
    2.  **Base Layer**: Rectangles for 1B, 2B, 3B; polygon for Home.
    3.  **Hit Path Layer**: SVG lines or curves representing the ball's trajectory (mapped from spray chart data).
    4.  **Marker Layer (Top)**: Squares for safe reach, 'X' for outs.
    5.  **Air Out Marker**: A large 'X' at the hit location for balls caught in the air.
*   **Scroll Behavior**: The grid supports independent overflow (auto) on both X and Y axes, with the lineup column pinned to the left.

### 2.4 Contextual Scoring Overlay (CSO)
*   **Reset on Open**: Re-opening the CSO for a specific batter always resets the view to the default "At-Bat" pitching screen.
*   **Thumb-Zone Optimization**: Critical buttons (Ball, Strike, Out) are sized and positioned for easy access on mobile devices.
*   **Center-Gravitating Menus**: Context menus (long-press/right-click) use logic to ensure they remain entirely within the viewport regardless of where the trigger occurred.

## 3. Specific UI Logic

### 3.1 Out-Ordering (Multi-Out Plays)
For plays resulting in multiple outs (DP/TP), the visual order of outs is deterministic:
*   **Air Outs**: The batter is recorded as the first out.
*   **Ground/Force Outs**: Lead runners are recorded as out first, followed by the batter.

### 3.2 Score Overrides
*   **Visual Cue**: Manually overridden inning scores are displayed as **Yellow, Bold, and Underlined** to distinguish them from derived scores.

## 4. Accessibility & UX Standards

*   **Hit Areas**: All interactive elements (buttons, tabs, cells) maintain a minimum hit area of **44x44px**.
*   **Color Redundancy**: Critical status changes are conveyed through both color and distinct icons (e.g., ✅ for Sync, ⚠️ for Conflict).
*   **Device Parity**: All features tucked behind context menus are accessible via both Long-Press (touch) and Right-Click (mouse).
