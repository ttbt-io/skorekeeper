# Skorekeeper Design Overview

Skorekeeper is a modern, real-time Progressive Web Application (PWA) designed for recording and broadcasting baseball and softball games. It prioritizes data integrity, collaboration, and a responsive user experience across both online and offline environments.

## 1. Core Architecture: Event Sourcing

The application's fundamental design principle is **Event Sourcing**. Instead of storing the current state of a game as a set of mutable values, the system maintains an authoritative, append-only **Action Log**.

*   **Single Source of Truth**: Every game event (pitch, hit, substitution, undo) is recorded as a discrete action in the log.
*   **Deterministic State Derivation**: The entire game state—including the current score, number of outs, and positions of runners—is derived by replaying the Action Log through a pure, deterministic reducer function.
*   **Undo/Redo**: Implemented naturally by appending "UNDO" actions to the log that target specific previous action IDs, preserving a complete history of the game.

## 2. Real-Time Synchronization

Skorekeeper is built for collaboration. The synchronization layer ensures that multiple users can interact with the same game simultaneously.

*   **WebSocket Communication**: Changes are broadcast in real-time to all connected clients.
*   **Optimistic UI**: Clients apply actions locally and re-render the UI immediately while awaiting confirmation from the server.
*   **Conflict Resolution**: If history diverges between a client and the server, the system provides clear paths for resolution (Overwrite, Catch-up, or Fork).

## 3. Key Components

The application is composed of several specialized engines and managers:

### 3.1 Contextual Scoring Overlay (CSO)
The primary user interface for inputting game data. It adapts dynamically based on the game context (e.g., current count, runners on base) to provide valid scoring options.

### 3.2 Ball-in-Play (BiP) Engine
Handles the complex logic of batted balls, defensive sequences, and automated runner advancements.
*   *Reference*: [Ball-in-Play Design Specification](./BIP-DESIGN.md)

### 3.3 Narrative Engine
Translates the structured data from the Action Log into a human-readable play-by-play feed, providing flavor and clarity to the game history.

### 3.4 Stats Engine
Aggregates live data into comprehensive player and team statistics (e.g., AVG, OBP, ERA, WHIP), updating in real-time as plays are recorded.

### 3.5 Synchronization & Persistence
Manages Service Workers for offline availability, IndexedDB for local storage, and WebSocket protocols for server communication.

## 4. Security & Authorization

Access control is governed by a **Team-Centric Authorization** model. Permissions (Admin, Scorekeeper, Spectator) are typically inherited from a user's relationship with a team, granting access to all games associated with that team.

---

*This overview is part of the comprehensive Skorekeeper Engineering Design Document. Detailed specifications for Synchronization, Statistics, and Authorization are available in their respective design files.*
