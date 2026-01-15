# Authorization Policy

This document defines the authorization rules for the Skorekeeper PWA backend.

## Access Levels

Permissions are calculated based on the user's relationship to the resource (Game or Team).

| Level | Name | Capabilities |
|-------|------|--------------|
| 0 | `AccessNone` | No access. |
| 1 | `AccessRead` | Can view data, roster, and live updates. |
| 2 | `AccessWrite`| Can record pitches, manage lineups, and edit team rosters. |
| 3 | `AccessAdmin`| Full control, including permission management and deletion. |

---

## Game Authorization

A user's access to a **Game** is the highest level granted by any of these rules:

1.  **Ownership:** If `Game.OwnerID == UserID` -> `AccessAdmin`.
2.  **Direct Permission:**
    *   `Game.Permissions.Users[UserID] == "write"` -> `AccessWrite`.
    *   `Game.Permissions.Users[UserID] == "read"` -> `AccessRead`.
3.  **Team Inheritance:** If the game is linked to a Team (Away or Home):
    *   User is Team Admin -> `AccessAdmin`.
    *   User is Team Scorekeeper -> `AccessWrite`.
    *   User is Team Spectator -> `AccessRead`.
4.  **Public Sharing:**
    *   If `Game.Permissions.Public == "read"` -> `AccessRead` (applies to anonymous users).

---

## Team Authorization

A user's access to a **Team** is determined by:

1.  **Ownership:** If `Team.OwnerID == UserID` -> `AccessAdmin`.
2.  **Explicit Roles:**
    *   `Team.Roles.Admins` contains UserID -> `AccessAdmin`.
    *   `Team.Roles.Scorekeepers` contains UserID -> `AccessWrite`.
    *   `Team.Roles.Spectators` contains UserID -> `AccessRead`.

---

## Endpoint Policy Mapping

### Game API

| Endpoint | Method | Required Access | Operation |
|----------|--------|-----------------|-----------|
| `/api/save` | `POST` | `AccessWrite` | Create or update game data and action log. |
| `/api/load/{id}` | `GET` | `AccessRead` | Fetch full game data. |
| `/api/list-games` | `GET` | Authenticated | List all games where User has `AccessRead`. |

### Team API

| Endpoint | Method | Required Access | Operation |
|----------|--------|-----------------|-----------|
| `/api/save-team` | `POST` | `AccessWrite` | Create or update team metadata and roster. |
| `/api/load-team/{id}` | `GET` | `AccessRead` | Fetch full team data. |
| `/api/list-teams` | `GET` | Authenticated | List all teams where User has `AccessRead`. |
| `/api/delete-team` | `POST` | `AccessAdmin` | Permanently remove a team. |
| `/api/team/members`| `POST` | `AccessAdmin` | Manage team member roles. |

### Real-Time Sync (WebSocket)

Connections to `/api/ws` are upgraded for any authenticated user. Authorization is checked per message:

| Message Type | Required Access | Operation |
|--------------|-----------------|-----------|
| `JOIN` | `AccessRead` | Join a game session and receive missing actions. |
| `ACTION` | `AccessWrite` | Submit a new play/metadata change to the log. |

---

## Authentication

The backend currently uses a **Mock Authentication** system for local development and E2E testing:
*   Checks for a `mock_auth_user` cookie.
*   Validates the JWT token from the cookie.
*   Populates the `UserID` in the request context, which is used for all permission checks.
*   In production, this header is provided by an authenticating reverse proxy (e.g., TLSProxy).

## User Access Policy (Global Gatekeeper)

Before checking resource-specific permissions (Game/Team), the system enforces a global **User Access Policy**:

1.  **Service Access:** Is the user allowed to use Skorekeeper at all? (Controlled by Allow/Deny lists).
2.  **Resource Quotas:** Has the user reached their limit of created Games or Teams?

If the User Access Policy denies the request, the server returns `403 Forbidden` with a policy-specific error message, and no resource authorization checks are performed.
