# Pagination Implementation Plan

## Overview
This document outlines the design for "Auto-Scroll" pagination with a merged data stream. The goal is to provide an infinite scroll experience for Games and Teams that seamlessly interleaves data from the remote server (via API) and the local IndexedDB, respecting sort order (e.g., Date Descending) regardless of where the data originates.

## Backend Changes (Complete)
- **`Registry`:** Updated to cache/index metadata for efficient sorting/filtering.
- **API (`list-games`, `list-teams`):** Updated to accept `limit`, `offset`, `sortBy`, `order`, `q`.

## Frontend Architecture: Stream & Merge

### 1. Merge Engine (Client-Side)
Instead of simple page concatenation, the Controller will manage two "Streams" of data:
1.  **Local Stream:** All items from IndexedDB, loaded into memory and sorted (e.g., by Date Desc).
    - *Note:* IDB is fast enough for ~10k items to load/sort in memory.
2.  **Remote Stream:** An async iterator/buffer that fetches pages from the API (e.g., page size 50) on demand.

**Logic:**
- `fetchNextBatch(size)`:
    - Compare the head of the Local Stream vs. the head of the Remote Stream.
    - Pick the "winner" based on the current Sort Order (e.g., newer Game first).
    - **Deduplicate:** If the winner's ID is in `seenIds`, discard it and pick again.
    - Add to batch. Repeat until batch is full.
    - Return batch.

**Offline Fallback:**
- If the Remote Stream encounters an error (network/auth), it "closes" gracefully.
- The Merge Engine continues serving only from the Local Stream.

### 2. UI Behavior: Auto-Fill & Infinite Scroll
- **Auto-Fill:** On load, the Controller fetches batches and renders them until the content height is ~2x the viewport height. This ensures the user has enough content to scroll immediately.
- **Infinite Scroll:**
    - A **Sentinel Element** (loading spinner/div) is appended to the bottom of the list.
    - An `IntersectionObserver` (or scroll event listener) watches the sentinel.
    - When the sentinel becomes visible (or scroll reaches bottom), `fetchNextBatch` is triggered, and results are appended.

### 3. Controller Updates
- **`DashboardController.js` & `TeamController.js`:**
    - Remove "Load More" button logic.
    - Implement `MergeStream` class (or helper) to handle the two sources.
    - Implement `handleScroll` logic.
    - Update `search(query)` to reset both streams and `seenIds`.

## Test Plan

### E2E Tests (`tests/e2e/pagination_test.go`)
- **Auto-Fill:** Verify that on load, multiple "pages" of data are fetched if the viewport is large or items are small.
- **Scroll:** Simulate scrolling (or `scrollTo`) to trigger the observer and verify new items appear.
- **Merge Logic:**
    - Inject a "Local Only" game dated *between* two remote games.
    - Verify it appears in the correct sorted position, not just at the top/bottom.
- **Offline:** Verify that when network is blocked, the list continues to populate from local data seamlessly.
