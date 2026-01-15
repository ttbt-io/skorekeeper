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
*   `timestamp`: When the action occurred.
*   `userId`: The user who performed the action.

## 2. Deterministic State Derivation (The Reducer)

The game state is derived by replaying the Action Log through a pure, deterministic **Reducer Function**.

### 2.1 Pure Function
The reducer follows the pattern `(state, action) => newState`. It must be pure:
*   No side effects (no network calls, no random numbers).
*   Given the same state and action, it must always return the same resulting state.
*   This allows any client to reconstruct the exact same game state by replaying the same log.

### 2.2 Replay Logic
To compute the current state:
1.  Initialize a fresh state object (Initial State).
2.  Iterate through the Action Log from oldest to newest.
3.  Apply each valid action sequentially to the state.
4.  The final result is the "Snapshot" used for rendering and UI logic.

## 3. Append-Only Undo Mechanism

Skorekeeper implements Undo/Redo without mutating the historical log.

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
