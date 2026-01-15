# Development Guide

This document provides instructions for building, testing, and contributing to the Skorekeeper PWA. Adhering to these guidelines ensures code quality, architectural consistency, and reliable synchronization across all clients.

## 1. Getting Started

### Prerequisites
*   **Node.js**: For managing frontend dependencies and Tailwind CSS.
*   **Go (1.25+)**: For the backend server and E2E test execution.
*   **Docker**: Required for running the headless E2E test environment.

### Build Instructions
To install dependencies, compile CSS, and build the backend executable:
```bash
./build.sh
```
This script performs the following:
1.  `npm install`: Installs JavaScript dependencies.
2.  `npm run build:css`: Compiles Tailwind CSS.
3.  `go build`: Compiles the Go backend.
4.  `docker build`: Creates the production Docker image.

## 2. Running Locally

### Development Server
Start the server with mock authentication and debug logging enabled:
```bash
go run . --use-mock-auth --debug
```
The application will be accessible at `https://devtest.local:8080/` (or `localhost:8080` depending on your environment).

### Running Raft Cluster (HA Mode)
To run a local 3-node Raft cluster for development:

    # Node 1 (Leader):
    go run . --use-mock-auth --addr :8080 --raft --raft-bootstrap --raft-bind :5001 --cluster-addr :9091 --cluster-advertise localhost:9091 --data-dir data/node1 --raft-secret secret
    
    # Node 2 (Follower):
    go run . --use-mock-auth --addr :8081 --raft --raft-bind :5002 --cluster-addr :9092 --cluster-advertise localhost:9092 --data-dir data/node2 --raft-secret secret
    *Note: You must manually join Node 2 to the cluster using the Cluster Dashboard or API.*
    
    *For E2E tests, the test runner handles this automatically.*

### Mock Authentication
When `--use-mock-auth` is enabled, the system looks for a `mock_auth_user` cookie. This is used extensively in E2E tests to simulate multiple users (e.g., Owner vs. Viewer).

## 3. Testing Suite

Skorekeeper maintains a high-fidelity testing environment. All changes must pass the full test suite.

### JavaScript Checks (Linting & Unit Tests)
```bash
./tests/run-js-checks.sh
```
*   **Linting**: ESLint ensures code style consistency.
*   **Unit Tests**: Jest tests for pure functions (like the reducer) and manager logic.

### Go Checks (Vet & Unit Tests)
```bash
go vet ./...
go test ./...
```

### End-to-End (E2E) Headless Tests
```bash
./tests/e2e/run-headless-tests.sh
```
*   These tests use `chromedp` to simulate user interactions in a headless browser.
*   They run within Docker containers to ensure environment consistency.
*   **Golden Files**: Narrative outputs are compared against "golden" snapshots in `tests/e2e/goldens/`. Use the `-update-goldens` flag to update them if narrative logic changes intentionally.

## 4. Architectural Rules

### Event Sourcing & Reducer
*   **Immutability**: Never mutate the game state directly. All state changes MUST be derived from the `actionLog` via the `reducer.js`.
*   **Pure Reducer**: The reducer must remain a pure function. No `Math.random()`, `Date.now()`, or network calls inside the reducer.
*   **Undo/Redo**: Implemented by appending `UNDO` actions. Generative actions performed after an undo act as a linear history barrier.

### Database & Sync
*   **Dependency Injection**: `SkorekeeperApp` accepts optional `dbManager` and `historyManager` instances to facilitate deterministic testing.
*   **Optimistic UI**: Update local state and re-render synchronously before awaiting server persistence to keep the UI responsive.

## 5. Style Guide & Coding Standards

### JavaScript (Frontend)
*   **DOM Safety**: NEVER set `innerHTML` to anything other than `''`. Use `textContent` or explicit DOM manipulation with `sanitizeHTML()`.
*   **Event Handlers**: Use explicit `onclick` or `oncontextmenu` handlers. Avoid HTML `<form onsubmit>` or `<button type="submit">`.
*   **Native UI**: Do not use `window.alert()`, `confirm()`, or `prompt()`. Use the custom modal system (`modalPrompt.js`).
*   **Indentation**: 4 spaces.
*   **Quotes**: Single quotes (`'`).
*   **Semicolons**: Required.

### Go (Backend)
*   Standard Go formatting (`go fmt`).
*   Strict authorization checks: The server must never trust the request payload for permissions; always load existing data from disk to verify roles.

## 6. Maintenance & Deployment

### Atomic Updates
When overwriting large or critical files (like `index.html`), always write to a `.tmp` file and then use `mv` to prevent file corruption in case of a crash during the write process.

### Finalization
Once a game is marked as `final`, the scoresheet becomes read-only (`pointer-events: none`) and destructive UI elements are hidden. Ensure all new UI features respect this `isReadOnly` state.

## 7. Production Cluster Configuration

To deploy a high-availability (HA) Raft cluster in production:

1.  **Shared Secret**:
    A strong shared secret string is **required** for cluster security. All nodes must start with the same secret.
    *   Flag: `--raft-secret "your-secret-string"`

2.  **Node Identity & Persistence**:
    Each node requires a unique ID and a persistent data volume.
    *   `--data-dir`: Directory path for game data and Raft logs (e.g., `/mnt/data`).
    *   Node ID is automatically derived from the node's persistent key.

3.  **Networking**:
    *   **Public/HTTP Port** (`--addr`): Used for client traffic AND internal request forwarding. Must be accessible by other nodes.
    *   **Raft Port** (`--raft-bind`): Used for Raft consensus traffic. Must be accessible by other nodes but should remain private/internal.

4.  **Bootstrapping (First Node)**:
    Start the *first* node with `--raft-bootstrap`. This initializes the cluster.
    ```bash
    ./skorekeeper --raft --raft-bootstrap --raft-secret "prod-secret-123" \
      --addr :443 --raft-bind :5001 --data-dir /mnt/data/node01
    ```

5.  **Adding Nodes (Followers)**:
    Start subsequent nodes *without* `--raft-bootstrap`.
    ```bash
    ./skorekeeper --raft --raft-secret "prod-secret-123" \
      --addr :443 --raft-bind :5001 --data-dir /mnt/data/node02
    ```

6.  **Joining the Cluster**:
    New nodes do not join automatically. You must instruct the *Leader* to add them.
    Use the `curl` command to hit the Leader's internal join endpoint:
    ```bash
    curl -X POST https://<leader-host>/api/cluster/join \
      -H "X-Raft-Secret: prod-secret-123" \
      -d '{ 
        "httpAddr": "https://<node-02-host>",
        "pubKey": "<base64-pub-key-of-node-2>"
      }'
    ```
    *   `httpAddr`: The public URL where this node can be reached (including protocol). The Leader stores this to forward write requests to it.

    **Finding the Leader:**
    If you don't know which node is the leader, you can query the status of *any* node (requires the shared secret):
    ```bash
    curl -H "X-Raft-Secret: prod-secret-123" https://<any-node-host>/api/cluster/status
    ```
    Response:
    ```json
    {
      "nodeId": "node-02",
      "state": "Follower",
      "leaderId": "node-01",
      "leaderAddr": "https://node-01.example.com"
    }
    ```

    **Listing Games (API):**
    ```bash
    curl -b "skorekeeper_auth=<jwt-token>" https://<node-host>/api/list-games
    ```

7.  **Operational Notes**:

    *   **Request Forwarding**: Followers automatically proxy write requests (`/api/action`) to the Leader using the `X-Raft-Secret` header.

    *   **Metadata Sync**: HTTP addresses are propagated to all nodes via the Raft log.

    *   **Backups**: Regularly backup the `--data-dir` directory.

## 8. User Access & Policy Management

Skorekeeper implements a strict User Access Policy to control who can use the service and how many resources they can create.

### Bootstrapping Admin Access
To set up the initial policy or recover access, start the server with the `--admin` flag:

```bash
go run . --use-mock-auth --admin "admin@example.com"
```

This grants the specified email temporary, full administrative privileges *for this session only*.

### Configuring the Policy
Once logged in as the bootstrap admin (or a permanently configured admin), you can update the policy via the API:

**Endpoint:** `POST /api/admin/policy`

**Payload:**
```json
{
  "defaultPolicy": "allow",
  "defaultMaxTeams": 5,
  "defaultMaxGames": 10,
  "defaultDenyMessage": "Registration is currently by invitation only.",
  "admins": ["admin@example.com", "other@admin.com"],
  "users": {
    "vip@user.com": {
      "access": "allow",
      "maxTeams": 50,
      "maxGames": 100
    },
    "banned@user.com": {
      "access": "deny"
    }
  }
}
```

*   **defaultPolicy**: "allow" or "deny". Controls access for users not explicitly listed.
*   **admins**: List of emails with permanent admin access (can manage policy).
*   **users**: Map of email to override settings.
    *   **access**: "allow" or "deny".
    *   **maxTeams/maxGames**: Override default quotas. Set to `-1` for unlimited (not yet implemented, currently large number recommended).

### Admin Dashboard
For a graphical interface to manage the policy:
1.  Ensure you are logged in as an Admin (or started with `--admin`).
2.  Navigate to `/admin`.
3.  Use the dashboard to edit global defaults, manage the admin list, and configure user overrides.

### Verifying Policy
You can retrieve the current policy via:
`GET /api/admin/policy`
