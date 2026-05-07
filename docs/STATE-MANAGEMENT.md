# State Management Design Specification

This document details the event-sourcing architecture and state management principles used in the Skorekeeper application.

## 1. Event Sourcing: The Action Log

The core of the Skorekeeper architecture is the **Action Log**, an authoritative, append-only chronological list of every event that has occurred in a game.

### 1.1 Single Source of Truth
The current state of a game is never stored directly as a mutable object in the database. Instead, only the Action Log is persistent. This ensures:
*   **Auditability**: Every change is traceable to a specific user and timestamp.
*   **Reliability**: History can be perfectly reconstructed at any time.
*   **Collaboration**: Real-time synchronization is achieved by broadcasting discrete actions rather than full state snapshots.

### 1.2 Action Structure
Each action in the log is a discrete object containing:
*   `id`: A unique identifier (UUID).
*   `type`: The category of the event (e.g., `PITCH`, `SUBSTITUTION`).
*   `payload`: The data specific to that action.
*   `refId`: (Optional) The ID of a previous action this action is modifying.
*   `insertAfterId`: (Optional) The ID of a previous action this action should be logically inserted after.
*   `timestamp`: When the action occurred.
*   `userId`: The user who performed the action.

## 2. Deterministic State Derivation (Dual-Timeline Pipeline)

The game state is derived by processing the Action Log through a three-phase **Dual-Timeline Pipeline** (`computeStateFromLog`).

### 2.1 Phase 1: Timeline Builder (`buildTimeline`)
Transforms the append-only Action Log (the Scorekeeper Timeline) into a 1D, chronological array of game events (the Game Timeline).
*   **Tombstones:** Filters out actions targeted by `UNDO`.
*   **Edits:** Replaces original actions with their latest edited versions (using `refId`).
*   **Insertions:** Logically inserts actions into the timeline at their requested historical position (using `insertAfterId`).

### 2.2 Phase 2: Logical Reducer (`reassignGridCoordinates`)
Iterates through the 1D Game Timeline to calculate the logical flow of the game (innings, outs, batter index) without relying on grid coordinates. It dynamically tracks actual roster sizes and handles batting around the order.

### 2.3 Phase 3: Grid Projection (`gameReducer`)
Applies the logically corrected actions sequentially to generate the 2D grid structure (`state.events` and `state.columns`) required by the legacy UI and scoresheet renderers.

## 3. Append-Only Undo and Edit Mechanism

Skorekeeper implements Undo/Redo and historical edits without mutating the historical log.

### 3.1 The UNDO Action
An `UNDO` is a standard action type that targets a specific historical action ID (`refId`).
*   Instead of removing an action from the log, an `UNDO` is appended to the *end* of the log.
*   During state derivation, the system performs a multi-pass scan to identify "effectively undone" actions.

### 3.2 Tombstoning & Redo
*   **Neutralization**: An action is ignored during reduction if it is targeted by an active `UNDO` action.
*   **Undo-Undo (Redo)**: If an `UNDO` action is itself targeted by a subsequent `UNDO`, the original action is "re-validated" and included in the reduction again. This provides an infinitely recursive Undo/Redo capability while maintaining an immutable history.

## 4. State Persistence

### 4.1 Distributed Server Persistence (Raft Consensus)
The backend uses **Hashicorp Raft** to replicate the Action Log across a cluster of nodes.
*   **Strong Consistency:** All writes (`SAVE_GAME`, `DELETE_GAME`, etc.) are proposed to the Raft cluster.
*   **Leader Election:** One node is elected Leader. All writes must go through the Leader.
*   **FSM (Finite State Machine):** Each node has an FSM that applies the committed Raft log entries to its local disk storage (GameStore). This ensures that all nodes have an identical copy of the data.
*   **Broadcast:** When the FSM applies a change, it triggers a WebSocket broadcast to all clients connected to that specific node. This allows for scalable read/broadcast traffic while maintaining a single source of truth for writes.

The underlying storage remains a flat-file/database structure indexed by `gameId` on each node, managed by the FSM.

### 4.2 Client Persistence (Offline Support)
Clients use **IndexedDB** to store a local copy of the Action Log.
*   This allows the app to function entirely offline.
*   When connection is restored, the client syncs its local actions with the server (see [Synchronization Design Specification](./SYNC-OFFLINE.md)).

## 5. Architectural Benefits

*   **Time Travel Debugging**: Developers can inspect the game at any point in its history by replaying the log up to a specific index.
*   **Simplified Logic**: State transition logic is centralized in one place (the reducer), making it easy to test and reason about.
*   **Optimistic UI**: Clients can immediately apply an action to their local state and re-render the UI, providing a lag-free experience while the action is synced in the background.
