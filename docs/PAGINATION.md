# Pagination and Data Loading

This document describes the pagination and data loading strategy used in the Skorekeeper application to ensure a responsive UI while handling large datasets and network synchronization.

## Architecture: Local-First Async Merging

The application prioritizes immediate UI responsiveness by loading local data first and then asynchronously merging remote data. It avoids blocking the main thread or the UI render cycle with network requests.

### Key Components

1.  **Local Buffer:**
    *   Stores data loaded from the local IndexedDB.
    *   Loaded immediately upon view initialization.
    *   Ensures the user sees data instantly, even if offline or on a slow connection.

2.  **Remote Buffer:**
    *   Stores data fetched from the backend API.
    *   Fetched in pages (batches) asynchronously in the background.
    *   Used to update the local view with the latest server state and to fill in gaps (e.g., historical data not cached locally).

3.  **Merge & Render Logic:**
    *   Combines the Local and Remote buffers.
    *   **Deduplication:** Items present in both buffers are merged. Remote data typically takes precedence for metadata (authoritative source), while local data determines the `syncStatus` (e.g., if local has pending edits).
    *   **Sorting:** The combined list is sorted in-memory (e.g., Games by Date Descending, Teams by Name Ascending).
    *   **Pagination:** The sorted list is sliced based on a dynamic `displayLimit` to prevent rendering DOM for thousands of items at once.

4.  **Infinite Scroll:**
    *   Scrolling to the bottom of the list increases the `displayLimit` (e.g., +20 items).
    *   If the buffers are running low on data relative to the new limit, a background fetch for the next remote page is triggered.

### Data Flow

1.  **View Init:**
    *   `DashboardController` / `TeamController` initializes empty buffers.
    *   Calls `render()` immediately (renders empty state/spinner).
    *   Triggers `loadAllLocalGames()` (async) -> Populates `localBuffer` -> Calls `mergeAndRender()`.
    *   Triggers `fetchNextRemoteBatch()` (async) -> Populates `remoteBuffer` -> Calls `mergeAndRender()`.
    *   Triggers `checkDeletions()` (async) -> Removes stale items -> Calls `mergeAndRender()`.

2.  **User Scrolls:**
    *   `handleScroll` detects proximity to bottom.
    *   Increases `displayLimit`.
    *   Calls `mergeAndRender()` to show more items from the existing buffers.
    *   If `remoteBuffer` is near exhaustion, triggers `fetchNextRemoteBatch()`.

3.  **Sync/Updates:**
    *   When an item is saved or synced, the local buffer is updated.
    *   `mergeAndRender()` is called to reflect the change immediately.

## Deprecated: StreamMerger

The previous `StreamMerger` class, which attempted to interleave async streams using generators and locking, has been removed. It introduced unnecessary blocking behavior that degraded the user experience. The current approach fully decouples data fetching from rendering.
