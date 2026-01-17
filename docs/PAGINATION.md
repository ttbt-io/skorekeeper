# Pagination & Infinite Scroll Architecture

## Overview
The Scorekeeper application uses a robust **Infinite Scroll** pattern backed by a **Stream Merging** architecture. This design ensures a seamless user experience that interleaves local (offline) data with remote (server) data while maintaining strict sort order and deduplication.

## Core Concepts

### 1. Stream Merging (`StreamMerger`)
The frontend does not simply append remote data to local data. Instead, it treats both sources as sorted streams and merges them on the fly.

*   **Local Stream:** All items from IndexedDB, loaded into memory and sorted (e.g., Date Descending).
*   **Remote Stream:** A buffered async iterator that fetches pages from the API (default batch size: 50).

**Merge Logic:**
The `StreamMerger` class (`frontend/services/streamMerger.js`) maintains pointers to both streams. To fetch the next batch:
1.  **Compare:** It compares the head of the Local Stream vs. the head of the Remote Buffer using the current sort comparator.
2.  **Pick Winner:** The item that comes "first" in the sort order is selected.
3.  **Deduplicate:** If the selected item's ID has already been seen (emitted), it is skipped. This handles cases where an item exists in both local and remote sources.
4.  **Merge Synced Items:** If the Local and Remote streams have the same ID at the head (e.g., a synced game with identical sort keys), they are merged into a single emission, ensuring the UI receives the most complete data (local state + remote sync status).

**Offline Fallback:**
If the Remote Stream encounters an error (e.g., network failure), it marks itself as "exhausted" but allows the Local Stream to continue serving data. This ensures the app remains functional offline.

### 2. Infinite Scroll UI
The application replaces the traditional "Load More" button with an automatic infinite scroll mechanism.

*   **Auto-Fill:** On view load, the Controller automatically fetches batches until the content fills the viewport (`clientHeight`). This prevents the user from seeing a blank screen if the first batch is empty or small.
*   **Scroll Event:** A scroll listener on the container detects when the user nears the bottom (threshold: 200px) and triggers `loadNextBatch`.
*   **Sentinel:** A passive visual indicator ("Loading...") is appended to the bottom of the list during fetches. It is hidden when the list is idle and displays "All items loaded" when both streams are exhausted.

## Backend API

The `/api/list-games` and `/api/list-teams` endpoints support standard pagination parameters:

*   `limit`: Number of items to return (default: 50, max: 100).
*   `offset`: Number of items to skip.
*   `sortBy`: Field to sort by (`date`, `name`, etc.).
*   `order`: `asc` or `desc`.
*   `q`: Search query string (filters by event, location, team names).

**Response Format:**
```json
{
  "data": [ ... ],
  "meta": {
    "total": 100,
    "limit": 50,
    "offset": 0
  }
}
```

## Performance Optimizations

*   **Parallel Persistence:** When a batch of remote items is processed by the Controller, their persistence to IndexedDB (for caching) is performed in parallel using `Promise.all`. This prevents database I/O from blocking the UI rendering loop during rapid scrolling.
*   **Metadata Indexing:** The Backend `Registry` maintains in-memory indices of metadata (Dates, Names) to allow for efficient O(1) or O(log N) sorting and filtering without loading full game histories from disk.

## Testing

*   **E2E Tests:** `tests/e2e/pagination_test.go` verifies the infinite scroll behavior by simulating scroll events and checking for the incremental appearance of items.
*   **Unit Tests:** Controllers and `StreamMerger` are unit tested with mocked data sources to verify sorting, merging, and offline fallback logic.