# PDF Generation & Game Export Design Specification

This document details the architecture and implementation of the client-side PDF export system in Skorekeeper.

## 1. Overview

Skorekeeper provides users with the ability to export a comprehensive, high-fidelity report of any game. In alignment with our "Local-First" and "Zero-Trust" philosophies, PDF generation is performed entirely on the client device. This ensures the feature remains functional offline and eliminates the need for expensive server-side rendering infrastructure.

## 2. Orchestration Strategy: Browser Print Engine

The application leverages the browser's built-in print engine (`window.print()`) to generate PDFs. This approach ensures perfect rendering of complex SVG elements (base paths, spray charts) and maintains sharp, vector-quality output.

### 2.1 The "Shadow" Print View
To produce a professional document layout that differs from the interactive live view, the application uses a "Shadow" rendering technique:
1.  **Container Injection**: When a user triggers an export, a temporary, hidden container (`#print-view-container`) is injected into the DOM.
2.  **Linear Rendering**: Both the Home and Away team data are rendered sequentially into this container. This is a departure from the live view, which toggles between teams.
3.  **Static State**: Rendering components (Scoresheet, Stats) are invoked with a `isPrint: true` flag, which:
    *   Disables all interactive elements (buttons, click handlers, context menus).
    *   Optimizes the layout for fixed-width pages (e.g., Letter/A4).
    *   Expands scrollable regions to their full dimensions.

### 2.2 Orchestration Lifecycle
*   **Trigger**: User clicks "Export to PDF" in the sidebar.
*   **Preparation**: Sidebar is closed, and the Shadow View is populated with data.
*   **Rendering**: A small delay (500ms) allows the browser to settle the layout and render all SVG paths.
*   **Execution**: `window.print()` is called, opening the system print dialog.
*   **Cleanup**: Once the dialog is closed, the Shadow View is removed from the DOM to reclaim memory.

## 3. Document Structure

The exported document is structured for maximum readability and follows a deterministic sequence:

1.  **Main Game Header**: Clean, light-themed summary of the game (Teams, Event, Location, Date).
2.  **Summary Scoreboard**: A static table showing per-inning runs and R/H/E totals for both teams.
3.  **Home Team Section**:
    *   **Statistics Table**: Full box score including derived hitting and pitching metrics.
    *   **Scoresheet Grid**: The complete scoresheet visualization (starting on a new page).
4.  **Away Team Section**:
    *   **Statistics Table**.
    *   **Scoresheet Grid** (starting on a new page).

### 3.1 Mini-Header System
To ensure context is preserved across multi-page documents, every new section (Stats and Scoresheets) is preceded by a "Mini-Header" containing consistent game metadata and a clear section subtitle.

## 4. CSS Optimization (`@media print`)

The styling of the PDF is managed via specific print media queries in `frontend/css/input.css`:

*   **Visibility Control**: All standard UI elements (navigation bars, sync status icons, interactive buttons) are explicitly hidden using `display: none !important`.
*   **Page Break Enforcement**: 
    *   `break-before: page` is used to ensure major sections start on fresh pages.
    *   Logic is applied to avoid breaking in the middle of a player's row or the scoreboard.
*   **Color Preservation**: The `print-color-adjust: exact` property is used to ensure the signature "Ink" colors (Red for Outs, Blue for Safe) are preserved in the final document.
*   **High Contrast**: Dark backgrounds used in the app are replaced with white backgrounds and dark borders to save ink and improve professional aesthetics.

## 5. Technical Considerations

*   **SVG Resolution**: Browser print engines treat SVGs as vector data, ensuring the PDF remains crisp at any zoom level.
*   **Scaling**: The scoresheet grid uses CSS scaling (`scale-90`) in print mode to ensure that even games with a high number of innings or extensive rosters fit within standard page margins.
*   **Offline Functionality**: Because generation is entirely client-side, users can export games to PDF even in the absence of an internet connection.
