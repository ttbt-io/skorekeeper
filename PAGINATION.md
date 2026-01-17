# Pagination Implementation Plan

## Overview
This document outlines the plan to refactor the `list-games` and `list-teams` API endpoints to support pagination, sorting, and filtering. This change addresses performance issues caused by loading and returning the entire dataset for a user.

## Backend Changes

### 1. `backend/teamstore.go`
- **Update `TeamMetadata` struct:** Add `Name` field.
- **Update `ListAllTeamMetadata`:** Populate `Name` from the loaded `Team` object.

### 2. `backend/registry.go`
- **Update `Registry` struct:**
    - Add `gameMetadata map[string]GameMetadata` to cache sortable fields.
    - Add `teamMetadata map[string]TeamMetadata` to cache sortable fields.
- **Update `Rebuild`:** Populate these maps during initialization.
- **Update `indexGameMetadata` / `indexTeamMetadata`:** Keep these maps in sync.
- **Update `ListGames`:**
    - Accept `sortBy` (default: "date"), `order` (default: "desc"), and `query` (filter).
    - Implement filtering (case-insensitive contains on Event, Location, Away, Home).
    - Implement sorting based on cached metadata.
- **Update `ListTeams`:**
    - Accept `sortBy` (default: "name"), `order` (default: "asc"), and `query` (filter).
    - Implement filtering (case-insensitive contains on Name).
    - Implement sorting.

### 3. `backend/server.go`
- **Update `parsePagination`:** Return `limit`, `offset`, `sortBy`, `order`, `query`.
    - `limit`: default 50, max 100.
    - `offset`: default 0.
    - `sortBy`: default "" (handled by Registry defaults).
    - `order`: default "desc" (or "asc" depending on context, passed as string).
    - `query`: default "".
- **Update `list-games` Handler:**
    - Extract params.
    - Call `registry.ListGames(userId, sortBy, order, query)`.
    - Pagination (slicing) happens on the result *after* sorting/filtering (Registry returns full filtered sorted list? Or Registry handles pagination? The prompt said "Slice the ID list... before interacting with file system". So Registry should return the *full sorted list of IDs*, and the Handler slices it. This keeps Registry simple).
- **Update `list-teams` Handler:**
    - Extract params.
    - Call `registry.ListTeams(userId, sortBy, order, query)`.
    - Slice and load.

## Frontend Changes

### 1. Service Layer (`frontend/services/`)
- **`SyncManager.js` (`fetchGameList`):**
    - Accept `offset`, `limit`, `sortBy`, `order`, `query`.
    - Return `{ data, meta }`.
- **`TeamSyncManager.js` (`fetchTeamList`):**
    - Accept `offset`, `limit`, `sortBy`, `order`, `query`.
    - Return `{ data, meta }`.

### 2. Controller Layer (`frontend/controllers/DashboardController.js`)
- **`loadDashboard`:**
    - Initial load: Fetch page 0.
- **`loadMoreGames`:**
    - Fetch next page.
- **UI:**
    - Add search bar.
    - Add sort dropdowns (optional for now, can stick to defaults).

## Test Plan

### Backend Tests (`backend/pagination_test.go`)
1.  **Pagination:** (Existing)
2.  **Sorting:**
    - Create games with different dates.
    - Request sort by date asc/desc. Verify order.
    - Create teams with different names.
    - Request sort by name asc/desc. Verify order.
3.  **Filtering:**
    - Create games "Yankees vs Red Sox", "Mets vs Rays".
    - Query "Yankees". Verify only 1 result.
    - Query "vs". Verify 2 results.