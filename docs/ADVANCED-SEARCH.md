# Advanced Search Implementation Plan

> **STATUS: DRAFT / NOT IMPLEMENTED**
> This document outlines the proposed design for the Advanced Search feature. It serves as a roadmap for implementation.

## Overview
This document outlines the plan to implement a structured search query language (DSL) for Games and Teams, supporting filters like `is:local`, `event:"..."`, and date ranges. This includes a shared query syntax for frontend/backend and a UI panel for constructing queries visually.

## 1. Search Query Syntax (DSL)
We will support a Google-style search syntax:
*   **Keywords:** `key:value` or `key:"value with spaces"`.
*   **Flags:** `is:local`, `is:remote` (Frontend only).
*   **Comparators:** `date:2025-01-01`, `date:>=2025-01-01`, `date:2025-01..2025-02`.
*   **Free Text:** Any tokens not matching `key:value` are treated as free text search across default fields.

**Supported Keys:**
*   **Games:** `event`, `location`, `away`, `home`, `date`.
*   **Teams:** `name`, `city` (if applicable), `coach`.

## 2. Backend Implementation (Go)

### Step 2.1: Search Parser Package
Create `backend/search/parser.go`.
*   **Structs:** `Query`, `Filter`.
*   **Function:** `Parse(input string) Query`.
    *   Tokenize string by spaces (respecting quotes).
    *   Extract keys and values.
    *   Identify date ranges.

### Step 2.2: Update Registry
Refactor `backend/registry.go` to use the parser.
*   **`ListGames` & `ListTeams`:**
    *   Call `search.Parse(query)`.
    *   Iterate through metadata (`gameMetadata`, `teamMetadata`).
    *   Apply filters:
        *   Exact/Substring match for string fields.
        *   Range check for Date fields.
        *   Free text search across all indexed text fields.

### Step 2.3: Backend Tests
*   **`backend/search/parser_test.go`:** Unit tests for tokenization, quoting, and edge cases.
*   **`backend/pagination_test.go`:** Add test cases for `q=event:Finals`, `q=date:>=2025`, etc.

## 3. Frontend Implementation (JS)

### Step 3.1: Search Parser Utility
Create `frontend/utils/searchParser.js`.
*   **Function:** `parseQuery(queryString)`.
*   **Returns:** `{ tokens: [], filters: { event: "...", is: [] } }`.
*   **Function:** `buildQuery(parsedObj)` -> String (for UI two-way binding).

### Step 3.2: StreamMerger & Controller Logic
Update `frontend/controllers/DashboardController.js` and `TeamController.js`.
*   **Parsing:** In `search(query)`, parse the string immediately.
*   **Source Filtering:**
    *   If `is:local` set, do NOT fetch remote.
    *   If `is:remote` set, do NOT fetch local (empty local stream).
*   **Local Filtering:**
    *   Update the local filtering logic (which currently does simple string `includes`) to use the parsed object and match specific fields (`event`, `date`, etc.) mirroring backend logic.

### Step 3.3: Frontend Tests
*   **`tests/unit/searchParser.test.js`:** Verify parsing logic.
*   **`tests/unit/dashboardController.test.js`:** Verify `is:local` prevents remote fetch.

## 4. UI Implementation (Advanced Panel)

### Step 4.1: HTML Structure
Update `frontend/index.html` (Dashboard & Teams views).
*   Add "Advanced" toggle button next to search input.
*   Add `#advanced-search-panel` (hidden by default).
    *   **Inputs:** Event, Location, Team, Date Start, Date End.
    *   **Checkboxes:** Local Only, Remote Only.
    *   **Buttons:** Apply, Clear.

### Step 4.2: AppController Logic
Update `frontend/controllers/AppController.js`.
*   **Toggle:** Show/Hide panel.
*   **Sync:**
    *   **Input -> Panel:** When user types `event:Test` in main box, update the "Event" input in the panel.
    *   **Panel -> Input:** When user clicks Apply, construct string `event:Test` and update main box + trigger search.

### Step 4.3: E2E Tests
*   **`tests/e2e/advanced_search_test.go`:**
    *   Open panel.
    *   Fill fields (e.g., "Local Only").
    *   Apply.
    *   Verify URL/Search Box contains `is:local`.
    *   Verify results are filtered.

## 5. Execution Order
1.  **Backend Parser & Registry:** Implement Go parser and integrate into Registry. Add backend tests.
2.  **Frontend Parser:** Implement JS parser and unit tests.
3.  **Controller Logic:** Integrate JS parser into Dashboard/Team controllers for local filtering and source selection (`is:local`).
4.  **UI Panel:** Build the HTML and AppController logic for the Advanced Search panel.
5.  **E2E Testing:** Verify the full flow.
6.  **Finalize:** Rewrite this document (`docs/ADVANCED-SEARCH.md`) to serve as the authoritative documentation for the implemented feature, removing the "Plan" structure.