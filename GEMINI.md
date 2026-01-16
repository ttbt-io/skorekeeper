# Persona and Guiding Principles

You are a highly skilled senior software engineer. You are meticulous and follow sound software engineering principles. You MUST adhere to these core principles:

*   **Clarity & Authoritative Documentation:** Software requirements and design are clearly documented. Documentation is the source of truth. All features and logic MUST strictly adhere to the corresponding design documents in the `docs/` directory.
*   **Documentation Maintenance:** All relevant documentation must be kept up-to-date. If a requirement or logic change is needed, the documentation MUST be updated first or in tandem with the code changes.
*   **Ambiguity Resolution:** When something is ambiguous or deviates from the design, you stop and ask for clarification, clearly explaining the discrepancy and the proposed options.
*   **Testing:** All features are thoroughly tested via unit tests and e2e tests. Code changes are tested to ensure they do not bring unexpected regressions and that they fulfill the requirements defined in the documentation.

## Code Quality

* Before completing a task, always make sure all the tests pass and that the documentation is up to date.
* Always run a code review before completing a task.

## Authoritative Design Documentation

The following documents in the `docs/` directory serve as the authoritative reference for the application's architecture and logic. Code MUST strictly follow these specifications:

1.  **[High-Level Overview](./docs/OVERVIEW.md)**: Introduction to the core architecture and components.
2.  **[Ball-in-Play (BiP) Design](./docs/BIP-DESIGN.md)**: Authoritative logic for batted balls, DP/TP automation, and runner advancements.
3.  **[State Management](./docs/STATE-MANAGEMENT.md)**: Event sourcing, Action Log, and deterministic reduction logic.
4.  **[Real-Time Synchronization](./docs/SYNC-OFFLINE.md)**: WebSocket protocols and offline persistence strategy.
5.  **[Authorization & Security](./docs/AUTH-SECURITY.md)**: Team-centric permissions and server-side enforcement.
6.  **[Visual System & UI Design](./docs/VISUAL-UI.md)**: Color palette, typography, and specific UI component behaviors.
7.  **[Definitions & Statistics](./docs/DEFINITIONS.md)**: Glossary of terms and statistical formulas.
8.  **[Development Guide](./DEVELOPMENT.md)**: Build instructions, testing procedures, and coding standards.
9.  **[Encryption at Rest](./docs/ENCRYPT-AT-REST.md)**: Design and implementation of node-local authenticated encryption for all persistent data.

# Gemini Session Notes - Skorekeeper PWA

This document records architectural decisions, critical learnings, and operational rules established during the development of the Scorekeeper Progressive Web Application.

## 1. Architectural Core: Event Sourcing

The application uses an **authoritative server-side action log** for state management.

*   **Single Source of Truth:** `activeGame.actionLog` contains all events (e.g., `PITCH`, `UNDO`, `GAME_METADATA_UPDATE`).
*   **State Derivation:** All UI and logic are derived by replaying the log through a pure reducer (`reducer.js`).
*   **Undo/Redo:** Implemented by appending `UNDO` actions that target previous action IDs. 
*   **Linear History Barrier:** Any new "Generative Action" performed after an Undo acts as a barrier, clearing future redo steps to maintain a predictable linear history.
*   **Conflict Resolution:** Three paths are provided when client/server history diverges:
    1.  **Overwrite Server (Authoritative):** Force-pushes local state using `POST /api/save`.
    2.  **Overwrite Local (Safe):** Discards local unsynced changes and catches up to the server.
    3.  **Fork:** Clones the local state into a new game with unique IDs.

## 2. Collaboration & Security Model

The system uses a **Team-Centric Authorization** model defined in `docs/AUTH-SECURITY.md`.

*   **Access Inheritance:** Game permissions are derived from a user's role within the Home or Away team (`Admin`, `Scorekeeper`, `Spectator`).
*   **Dynamic Indexing:** The `Registry` service tracks team-to-game relationships, ensuring that adding a member to a team immediately grants them access to all linked games.
*   **WebSocket Security:** Bootstrap (`GAME_START`) is allowed for new games if the sender is the owner. Subsequent metadata updates require `AccessAdmin`.
*   **Data Integrity:** The server never trusts the request payload for authorization; it always loads existing data from disk to verify permissions before allowing updates.

## 3. Testing Infrastructure

### E2E Testing (Chromedp + Docker)
*   **Robust Environment:** Application server and headless browser run in separate Docker containers (`chromedp/headless-shell`).
    * Run individual e2e tests with: ./run-tests-gemini.sh <TestName>
    * Run all tests with: ./run-tests-gemini.sh
*   **Session Isolation:** Multiple users (e.g., Owner and Viewer) are tested using different hostnames (`devtest.local` vs `devtest`) to ensure cookie/session isolation.
*   **Timing:** Use `WaitVisible` for elements in the current view and implement short sleeps (200ms) before critical interactions (like conflict resolution clicks) to handle modal transitions reliably.

### Frontend Unit Testing (Jest)
*   **Dependency Injection:** `SkorekeeperApp` accepts optional `dbManager` and `historyManager` instances. This eliminates the need for complex module mocking and makes tests deterministic.
*   **Async Mocking:** When testing IndexedDB, manually trigger `onsuccess` handlers on mocked request objects to ensure reliable execution flow.

## 4. Hard Rules & Operational Guidelines

### Development Rules
*   **DOM Safety:** Never set `innerHTML` to anything other than `''`. Use `textContent` and explicit DOM manipulation with `sanitizeHTML()`.
*   **Event Handling:** Use explicit `onclick` or `oncontextmenu` handlers. Avoid HTML `<form onsubmit>` or `button type="submit"`.
*   **Native UI:** Do not use `window.alert()`, `confirm()`, or `prompt()`. Use the custom modal system (`modalPrompt.js`).
*   **Optimistic UI:** Update local state and re-render synchronously before awaiting persistence to keep the UI responsive.

### Usability & Accessibility
*   **Auto-ContextMenu:** Elements with `onclick` automatically get a default `oncontextmenu` handler triggering the same action, ensuring consistency across touch and mouse devices.
*   **Cycle Buttons:** Buttons that cycle through options (BiP Result, Runner Action) include a ↻ icon and support direct selection via right-click picker.

## 5. Deployment & Maintenance
*   **Atomic Updates:** When overwriting large files (like `index.html`), write to a `.tmp` file and `mv` it to prevent corruption.
*   **Service Worker:** Checks for updates on every page load. A "Clear Cache & Reload" button provides a manual force-refresh mechanism.
*   **Finalization:** Finalizing a game (`status: 'final'`) makes the scoresheet read-only via `pointer-events: none` and hides destructive UI elements.

## 6. Encryption at Rest
*   **Implementation:** The system uses `github.com/c2FmZQ/storage` for authenticated encryption (AES-256-GCM) of all persistent data.
*   **Scope:**
    *   **Entity Files:** Game and Team data in `data/games` and `data/teams`.
    *   **Raft Logs:** Consensus logs in `data/raft/raft-log.bolt`.
    *   **Snapshots:** Streaming snapshots between nodes.
*   **Key Management:**
    *   A master key is derived from the `SK_MASTER_KEY` environment variable.
    *   If `SK_MASTER_KEY` is not provided and no `master.key` file exists, data is stored UNENCRYPTED (warning logged).
    *   **Safety Check:** If `master.key` exists but `SK_MASTER_KEY` is NOT set, the application will exit with a fatal error to prevent accidental unencrypted access.
    *   **Node Isolation:** Each node in a cluster can (and should) have a unique, independent `SK_MASTER_KEY` and `master.key` file.
    *   **Snapshot Security:** While snapshots are encrypted at rest on disk, they are automatically decrypted by the `EncryptedSnapshotStore` layer when being streamed to peers. Security during transit is provided by the cluster's mTLS transport.
*   **Migration:** The system supports "Lazy Migration" for entity files. Legacy plaintext JSON files are read transparently and upgraded to encrypted format upon the next write. Raft logs are NOT backward compatible and must be cleared if enabling encryption on an existing node.

## 7. Write-Optimized Persistence (Checkpointing)
*   **Strategy:** The system employs a "Write-Behind" strategy for Game and Team updates to minimize I/O overhead.
*   **In-Memory Authority:** The `GameStore` and `TeamStore` caches are the authoritative sources of truth for the FSM. Updates are applied to memory immediately and marked as "dirty".
*   **Checkpointing:** Dirty state is flushed to disk only when:
    *   A Raft Snapshot is requested (Mandatory).
    *   The server is shutting down (Safety).
    *   (Future) A background timer triggers.
*   **Standalone Mode:** If Raft is disabled, the system defaults to "Write-Through" (immediate persistence) to ensure durability in the absence of a Raft log.
*   **Consistency:** The `ListAllGames` and `ListAllTeams` APIs automatically merge disk-based and in-memory (dirty) items to ensure clients always see the latest state.

## Gemini Added Memories
- For E2E tests involving multiple users (e.g., Owner and Viewer) sharing a single browser instance (cookies), use different hostnames (e.g., `devtest.local` vs `devtest` or `devtest.public`) to ensure session isolation. localhost doesn't work for cross-container communication.
- Do NOT modify `getRunnersOnBase()` to include the current active batter. Doing so causes issues like duplicate runners on base or ghost runners. `getRunnersOnBase` should strictly return runners from *previous* slots in the inning. To handle runner actions for the current batter, check `activeData.outcome` or `paths` separately.
