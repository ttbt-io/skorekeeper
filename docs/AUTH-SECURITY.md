# Authorization & Security Design Specification

This document details the security model and authorization logic used to protect data and ensure collaborative integrity within Skorekeeper.

## 1. Team-Centric Authorization Model

Skorekeeper uses a hierarchical, team-based permissions model. Access to a game is typically derived from a user's relationship with the participating teams.

### 1.1 Access Levels
| Level | Name | Description |
| :--- | :--- | :--- |
| 0 | `AccessNone` | No access. |
| 1 | `AccessRead` | **Spectator**: Can view live games, historical scores, and rosters. |
| 2 | `AccessWrite`| **Scorekeeper**: Can record pitches, manage lineups, and edit team rosters. |
| 3 | `AccessAdmin`| **Admin**: Full control, including managing permissions and game deletion. |

### 1.2 Permission Inheritance
A user's effective permission for a game is the *highest* level granted by any of the following rules:
1.  **Ownership**: The game creator is automatically an `Admin`.
2.  **Direct Grant**: Specific users can be granted `Read` or `Write` access directly on a per-game basis.
3.  **Team Relationship**: If a game is associated with a specific Team (Home or Away), users with roles on that Team inherit corresponding access to the game.
4.  **Public Access**: Games can be flagged for "Public Read", allowing anyone (including unauthenticated users) to view the live broadcast.

## 2. Server-Side Enforcement

The server is the ultimate authority for permission enforcement. It never trusts permission flags sent by the client.

### 2.1 API Validation
Every REST endpoint validates the authenticated user's permission against the resource on disk before performing any operation.
*   **Write Operations**: Require `AccessWrite`.
*   **Administrative Operations**: Require `AccessAdmin`.

### 2.2 WebSocket Security
Permissions are re-validated for every message sent over a WebSocket connection.
*   **`JOIN`**: Validated for `AccessRead`.
*   **`ACTION`**: Validated for `AccessWrite`.
*   **Bootstrap Actions**: Special logic allows the `GAME_START` action only if the sender is the intended owner.

## 3. Dynamic Indexing (The Registry)

The server maintains a **Registry** that tracks relationships between users, teams, and games.
*   When a user is added to a team, the Registry ensures they immediately gain access to all games linked to that team.
*   When a game is created, it is indexed by its participating team IDs to facilitate fast permission lookups.

## 4. Authentication Architecture

### 4.1 JWT Verification (Zero-Trust)
Skorekeeper uses a cryptographically verified authentication model based on **JSON Web Tokens (JWT)**. This eliminates the need to "trust" upstream headers and protects against identity spoofing.

*   **Token Source**: The server expects a JWT provided in a secure HTTP-only cookie (default: `skorekeeper_auth`).
*   **Verification (JWKS)**: On every request, the server validates the token's signature against a set of public keys retrieved from a trusted **JWKS (JSON Web Key Set)** endpoint.
*   **Identity Extraction**: Once verified, the user's identity is extracted from the `email` claim.
*   **Internal Routing**: The verified identity is injected into the request context (`UserID`) by the authentication middleware for consumption by downstream handlers. External headers are ignored in favor of the cryptographically verified identity.

### 4.2 Mock Authentication
For development and automated testing, a mock authentication mode can be enabled.
*   **Behavior**: When `UseMockAuth` is active, the server bypasses cryptographic verification and treats the value of the auth cookie as the user's unique ID directly.
*   **Safety**: This mode should NEVER be enabled in production environments.

### 4.3 Data Integrity
*   **Sanitization**: All user-supplied data is sanitized before storage or broadcast to prevent Cross-Site Scripting (XSS).
*   **Authoritative Log**: The append-only nature of the Action Log prevents historical tampering.

## 5. Privacy Considerations

*   **Personally Identifiable Information (PII)**: The system primarily handles player names and numbers. User IDs (emails) are used for internal authorization but are not exposed to spectators.
*   **Read-Only Finalization**: Once a game is marked as `final`, write access is disabled for all users except Admins, preventing accidental post-game modifications.
