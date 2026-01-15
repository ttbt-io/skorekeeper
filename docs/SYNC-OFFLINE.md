# Real-Time Synchronization & Offline Strategy

This document describes the protocols and strategies used by Skorekeeper to ensure real-time collaboration, reliable operation in intermittent network conditions, and system resilience during high-load recovery.

## 1. Real-Time Synchronization (WebSockets)

Skorekeeper uses a full-duplex WebSocket connection to synchronize the Action Log between the client and the server.

### 1.1 The Synchronization Protocol (Hybrid HTTP/WebSocket)
1.  **Handshake (`JOIN`)**: Upon WebSocket connection, the client sends a `JOIN` message containing its `lastRevision`. The server sends missing actions (`SYNC_UPDATE`).
2.  **Writing (`HTTP POST`)**: When a client performs an action, it sends it via `POST /api/action`.
    *   This is a stateless request that can be easily forwarded to the Raft Leader.
    *   The client queues these requests to ensure order.
3.  **Broadcasting (`WS ACTION`)**: When an action is committed (persisted by Raft), the server broadcasts it via WebSocket to **all** connected clients (including the sender, as confirmation).
4.  **Reconciliation**: The client uses the broadcasted `ACTION` as an ACK. If the HTTP request fails with `409 Conflict`, the client enters conflict resolution mode.

### 1.2 Action Batching
To reduce the overhead of individual HTTP requests and Raft proposals, especially after extended offline periods, the system supports batched action synchronization.
*   **Client Logic**: The `SyncManager` monitors its outgoing `httpQueue`. When multiple actions are pending, it drains the entire queue into a single `POST /api/action` request containing an `actions` array.
*   **Server Logic**: The server processes batches atomically within the Raft cluster, maintaining idempotency by checking individual action IDs against the authoritative log. Successful batches are broadcasted to all connected clients.

### 1.3 "Catch-up First" Policy
To minimize `409 Conflict` errors caused by stale `baseRevision` IDs, the synchronization protocol enforces strict ordering:
1.  **Handshake Barrier**: On connection, the client blocks outgoing HTTP requests.
2.  **Completion Signal**: The block is cleared only after the client receives a `JOIN ACK` or a `SYNC_UPDATE` (catch-up) message from the server.
3.  **Resumption**: This ensures the first pushed action uses the most recent possible `baseRevision`.

## 2. Team Synchronization
Unlike the real-time action log for games, Teams are synchronized as monolithic objects.
1.  **Adoption**: Teams created while anonymous are automatically "adopted" by the user upon login, updating the `ownerId` from a local ID to the user's email.
2.  **Auto-Sync**: When viewing the Teams list, the application automatically identifies `local_only` teams and attempts to push them to the server in the background.
3.  **Visual Indicators**: The UI displays status icons (✅, ☁️⬆️, ⏳, ❌) to convey synchronization health.

## 3. Offline Strategy

Skorekeeper is designed as a "Local-First" application, capable of full functionality without an active internet connection.

### 3.1 Local Persistence (IndexedDB)
The entire Action Log for active games is mirrored in the browser's **IndexedDB**.
*   **Durability**: Game data persists across browser sessions and device restarts.
*   **Instant Load**: On startup, the app renders immediately from IndexedDB while the WebSocket attempts to connect.

### 3.2 Progressive Web App (PWA) Features
*   **Service Workers**: Assets (HTML, JS, CSS) are cached locally, enabling the application to load even when completely offline.
*   **Manifest**: Allows the application to be "installed" on mobile and desktop devices.

### 3.3 Optimistic UI
Clients apply actions to their local state and re-render the UI *immediately* before sending them to the server, ensuring a responsive experience.

## 4. Resilience & Load Management

### 4.1 Adaptive Reconnection Jitter
To prevent "Thundering Herd" failures during server recovery, the system implements a scaled jitter algorithm for reconnections.
*   **Algorithm**: `delay = base * 1.5^attempts + random(min(10000, attempts * 1000))`
*   **Impact**: As the number of failed attempts increases, the jitter window grows up to 10 seconds, spreading the connection load across the cluster.

### 4.2 Load Shedding (HTTP 429)
The backend implements active load shedding to protect its internal processing queues.
*   **Trigger**: When a Hub's request channel is saturated, the server returns `HTTP 429 Too Many Requests`.
*   **Retry-After**: Responses include a `Retry-After` header (e.g., 2s for Loads, 10s for Saves) which the client explicitly respects by pausing its outgoing queue.

### 4.3 Proxy Optimization (Caching)
To ensure high availability and reliable PWA updates when deployed behind a proxy like Cloudflare, the server enforces strict `Cache-Control` policies:
*   **API & SSO (`/api/*`, `/.sso/*`)**: `private, no-cache, no-store, no-transform`. Prevents sensitive data from being cached by shared proxies.
*   **Frontend Assets**: `public, no-cache, proxy-revalidate, no-transform`. Allows proxies to cache assets but **forces** revalidation with the origin via ETags on every request. This ensures that Service Worker updates and application logic changes are detected immediately.

## 5. Conflict Resolution

When histories diverge, the application provides three resolution strategies:
1.  **Overwrite Server (Force Push)**: The client's local history is declared authoritative.
2.  **Overwrite Local (Catch-up)**: The server's history is declared authoritative; local unsynced changes are discarded.
3.  **Fork**: The client's current state is cloned into a *new* game with a unique ID, preserving both versions.

---
*This document is part of the authoritative Skorekeeper Engineering Design Document.*