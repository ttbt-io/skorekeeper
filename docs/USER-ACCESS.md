# User Access Policy Design

This document outlines the design for the robust, Raft-replicated, and encrypted-at-rest User Access Policy implementation in Skorekeeper.

## 1. Requirements Recap

*   **Global Policy:** Default allow/deny for new users.
*   **Default Quotas:** Max teams and max games per user.
*   **Custom Denial Message:** Message shown to users when access is denied.
*   **User Overrides:** Specific access levels and quotas per email.
*   **Raft Replication:** Policy must be consistent across the cluster.
*   **Standalone Support:** Feature must fully function in non-Raft mode, persisting to local disk.
*   **Encryption at Rest:** Policy data must be stored using the authenticated encryption layer.
*   **Frontend integration:** UI must handle denials and respect quotas.

## 2. Backend Implementation (Go)

### 2.1. Data Model
*   Defined in `backend/access_control.go`.
*   `UserAccessPolicy` struct:
    ```go
    type UserAccessPolicy struct {
        DefaultPolicy      string `json:"defaultPolicy"` // "allow" or "deny"
        DefaultMaxTeams    int    `json:"defaultMaxTeams"`
        DefaultMaxGames    int    `json:"defaultMaxGames"`
        DefaultDenyMessage string `json:"defaultDenyMessage"`
        Admins             []string `json:"admins"` // List of admin emails
        Users              map[string]UserOverride `json:"users"`
    }

    type UserOverride struct {
        Access   string `json:"access"` // "allow" or "deny"
        MaxTeams int    `json:"maxTeams"`
        MaxGames int    `json:"maxGames"`
    }
    ```

### 2.2. Persistence & Raft (FSM)
*   **Storage Key:** `sys_access_policy` in the encrypted store.
*   **FSM Command:** `CommandUpdateAccessPolicy` in `backend/raft_types.go`.
*   **FSM Apply:** The `Apply` method in `backend/fsm.go` handles the command and persists to disk via the encrypted storage layer.
*   **Registry Integration:** The `Registry` caches the current policy in memory for fast lookups.

### 2.3. Access Control Service
*   Implemented in `backend/access_control.go`.
*   Methods:
    *   `IsAllowed(email string) (bool, string)`: Returns allowed status and denial message.
    *   `CheckQuota(email string, teamCount, gameCount int) error`: Returns error if limits exceeded.

### 2.4. Server Integration
*   **Authentication:** Login handler checks `IsAllowed`. If denied, returns `403 Forbidden` with the custom message in the JSON body.
*   **Resource Creation:** `CreateGame` and `CreateTeam` handlers verify quotas before proceeding.
*   **Admin API:**
    *   `GET /api/admin/policy`: Retrieves current policy (Admin access required).
    *   `POST /api/admin/policy`: Proposes a Raft command to update the policy.

### 2.5. Bootstrapping
*   Server binary accepts an `--admin` string flag.
*   On startup, if `--admin` is set:
    *   Temporarily injects an in-memory override granting "Admin" status to that email.
    *   This allows the user to call `POST /api/admin/policy` to set up the initial permanent configuration.
    *   Logs a warning that temporary admin access is enabled.

## 3. Frontend Implementation (JS)

### 3.1. Auth & State
*   `frontend/services/authManager.js` stores quota information returned in the auth status or login response.
*   `SkorekeeperApp.js` catches `403 Forbidden` during initialization or login.

### 3.2. UI Components
*   **Access Denied Modal:** A specialized modal displays the `DefaultDenyMessage`.
*   **Quota Enforcement:**
    *   `DashboardRenderer`: Disables the "New Game" button and adds a "Quota Reached" tooltip if the limit is hit.
    *   `TeamsRenderer`: Disables the "New Team" button if the limit is hit.