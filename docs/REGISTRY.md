# Registry Design: Disk-Based User Indices

This document outlines the architecture of `backend.Registry`, which supports disk-based user indices to reduce memory footprint and ensure scalability.

## 1. Design Overview

The Registry utilizes a **Disk-Based User Index** strategy to manage user permissions and entity relationships. This approach eliminates the need for monolithic in-memory maps that grow linearly with the user base.

*   **Scalability**: Supports an unlimited number of users and teams; memory usage is bounded by LRU cache sizes.
*   **Persistence**: All indices are persisted to disk using authenticated encryption, ensuring data survival across restarts.
*   **Performance**: Frequently accessed indices are cached in memory (LRU) to maintain sub-millisecond lookup times for active users.
*   **Runtime Inheritance**: Team-based game access is resolved at runtime by intersecting user team membership with team-linked games, ensuring that updating a team's roster is an $O(members)$ operation regardless of the number of games.

## 2. Store Component: `UserIndexStore`

The `UserIndexStore` is the unified persistence layer responsible for managing user access indices and team-game relationships.

*   **File Paths**:
    *   User Index: `data/users/<url_escaped_email>.json`
    *   Team Games Index: `data/team_games/<team_id>.json`
    *   Game Users Index: `data/game_users/<game_id>.json`
    *   Team Users Index: `data/team_users/<team_id>.json`
*   **Data Structures**:
    ```go
    type UserIndex struct {
        UserID      string                 `json:"userId"`
        GameAccess  map[string]AccessLevel `json:"gameAccess"` // Direct Game Access
        TeamAccess  map[string]AccessLevel `json:"teamAccess"` // Team Membership
    }

    type TeamGamesIndex struct {
        TeamID      string          `json:"teamId"`
        GameIDs     map[string]bool `json:"gameIds"`
    }

    type GameUsersIndex struct {
        GameID      string          `json:"gameId"`
        UserIDs     map[string]bool `json:"userIds"` // Direct access users
    }

    type TeamUsersIndex struct {
        TeamID      string          `json:"teamId"`
        UserIDs     map[string]bool `json:"userIds"`
    }
    ```
*   **Caching Strategy (LRU)**:
    *   **User Cache**: 1,000 items.
    *   **Team-Games Cache**: 500 items.
    *   **Game-Users Cache**: 1,000 items.
    *   **Team-Users Cache**: 500 items.
*   **Write-Behind Persistence**:
    *   Updates are applied immediately to the in-memory cache and marked as "dirty".
    *   `FlushAll()` (called during snapshots or shutdown) writes all dirty entries to disk.
    *   Cache eviction triggers an automatic save of the evicted item if it is dirty.

## 3. Registry Architecture

The `Registry` acts as the high-level interface for querying and modifying these indices. It delegates index management entirely to `UserIndexStore` and maintains its own LRU cache for entity metadata.

### Logic Flow

#### Reading (`ListGames`, `HasGameAccess`, `GetAccessLevel`)
1.  **Load Direct Index**: Calls `UserIndexStore.GetUserIndex(userId)` to get direct game access and team memberships.
2.  **Resolve Inheritance**: For every team the user belongs to, it loads the `TeamGamesIndex` to check if the target game is linked to that team.
3.  **Calculate Effective Level**: Returns the highest access level found among direct access and all team-inherited paths.
4.  **Fetch Metadata**: Resolves entity metadata (Name, Date) using the Registry's internal metadata cache (5,000 games, 2,000 teams) or loading from the respective stores (`GameStore`, `TeamStore`).

#### Writing (`UpdateTeam`, `UpdateGame`)
1.  **Team Update**:
    *   Determines new members and their roles.
    *   Identifies removed members and removes the team from their `UserIndex`.
    *   Updates the `UserIndex` for all current members to reflect their membership.
    *   Updates `TeamUsersIndex` for the team.
2.  **Game Update**:
    *   Updates direct access in `UserIndex` for all users listed in the game permissions.
    *   Updates `GameUsersIndex` for the game.
    *   Links the game to the Home and Away teams in their respective `TeamGamesIndex`.

#### Deleting
1.  **Game Deletion**:
    *   Marks game as "deleted" in metadata cache (tombstone).
    *   Removes game from the `UserIndex` of all users who had direct access.
    *   Deletes the `GameUsersIndex`.
    *   *Note*: The game remains in `TeamGamesIndex` files but is filtered out at runtime by the deletion check.
2.  **Team Deletion**:
    *   Marks team as "deleted" in metadata cache.
    *   Removes team membership from the `UserIndex` of all members.
    *   Deletes `TeamUsersIndex` and `TeamGamesIndex`.

## 4. Startup and Rebuild

To ensure high performance and consistency, the Registry supports two startup paths:

*   **Fast Path (Normal)**:
    *   Counts total games and teams by listing files in `data/games` and `data/teams`.
    *   Trusts the persisted indices in `data/users`, `data/team_games`, etc.
*   **Rebuild Path (Recovery/Force)**:
    *   Triggered if indices are missing or consistency is in question.
    *   Scans all Game and Team files.
    *   Regenerates all indices from scratch.
    *   Optimized to use local counters and a single lock to avoid contention during large reconstructions.
