# Advanced Search

The Advanced Search feature provides a structured query language (DSL) and a visual interface for filtering Games and Teams. It allows for precise filtering by metadata fields, dates, and synchronization status.

## 1. Search Syntax

The search bar supports a Google-style syntax. Queries are composed of space-separated tokens.

### Key-Value Filters
Use `key:value` to filter by specific fields.
*   **Exact Match:** `event:Finals`
*   **Quoted Values:** `location:"Central Park"` (Supports values with spaces)
*   **Case Insensitive:** `event:finals` matches "Finals", "finals", "FINALS".

**Supported Keys:**
*   `event` - Game event name.
*   `location` - Game location.
*   `away` - Away team name.
*   `home` - Home team name.
*   `team` - Matches either Away or Home (supports Name or Team ID).
*   `name` - Team name (Teams view only).

### Date Filtering
Filter items by date using operators.
*   **Exact Date:** `date:2025-01-01`
*   **After:** `date:>=2025-01-01` or `date:>2025-01-01`
*   **Before:** `date:<=2025-12-31` or `date:<2025-12-31`
*   **Range:** `date:2025-01..2025-03` (Inclusive)

### Source Flags
Control where data is fetched from (Frontend Only).
*   `is:local` - Only show data from the local database (Offline mode). Do not fetch from server.
*   `is:remote` - Only show data fetched from the server. (Useful for debugging sync).

### Free Text
Any token that doesn't match the `key:value` pattern is treated as free text.
*   `Yankees` -> Searches for "Yankees" in Event, Location, Away, Home, etc.
*   `"Game 1"` -> Searches for the exact phrase "Game 1".

## 2. Advanced Search Panel

The UI includes a collapsible panel for constructing queries visually without memorizing syntax.

*   **Inputs:** Event, Location, Team, Date Range.
*   **Toggles:** Local Only, Remote Only.
*   **Behavior:**
    *   Entering values in the panel updates the main search bar in real-time with the corresponding DSL.
    *   Clicking **Apply** triggers the search.
    *   Clicking **Clear** resets all fields and the search bar.

## 3. Implementation Details

### Parser
*   **Backend (Go):** `backend/search/parser.go` - Parses queries for `Registry` filtering.
*   **Frontend (JS):** `frontend/utils/searchParser.js` - Parses queries for local filtering and UI binding.
*   Both parsers share the same logic for tokenization and operator handling.

### Filtering Logic
1.  **Frontend (Local):** `DashboardController` and `TeamController` use the parsed query to filter the **Local Buffer** in-memory.
2.  **Backend (Remote):** The parsed query is passed to the backend API (`/api/list-games?q=...`), which filters results at the Registry level before pagination.
3.  **Merge:** The results from both sources are merged, deduplicated, and sorted by the Controller.

### Source Control
*   `is:local`: The Controller skips the `fetchRemotePage()` call.
*   `is:remote`: The Controller clears the `localBuffer` before rendering.

## 4. Tests
*   **Unit Tests:**
    *   `backend/search/parser_test.go`: Verifies DSL parsing.
    *   `frontend/utils/searchParser.test.js`: Verifies JS parsing and query reconstruction.
    *   `tests/unit/dashboardController.test.js`: Verifies filtering integration.
*   **E2E Tests:**
    *   `tests/e2e/advanced_search_test.go`: Verifies the UI panel, syntax application, and result filtering in a real browser environment.
