# Skorekeeper Versioning & Interoperability Design

This document outlines the strategy for managing rolling updates, backward compatibility, and data interoperability across different versions of the Skorekeeper application, backend nodes, and persistent data.

## 1. Versioning Strategy

We decouple infrastructure, communication, and data to allow independent evolution of components.

*   **Application Version (SemVer):** The overall release version (e.g., `v1.2.3`). Used for identifying the release and tracking changes.
*   **Protocol Version:** Versions for API and WebSocket communication (e.g., `Proto-v2`). Defines the structure of messages and endpoints.
*   **Schema Version:** Versions for persistent entities: `Game`, `Team`, and `Action` (e.g., `Schema-v3`). Defines the structure of data on disk and in the Action Log.

## 2. Schema Evolution

*   **Schema Version 3 Required:** The backend and frontend strictly enforce Schema Version 3 for all operations.
*   **No Legacy Support:** Support for schema versions prior to 3 has been removed. Data must be in Version 3 format to be loaded.
*   **Strict Writing:** Data is always written using the latest schema version.

## 3. Raft Cluster Interoperability

In a distributed environment, different nodes may be at different release levels during a rollout.

### 3.1 Version Handshake
*   The `NodeMeta` structure is expanded to include the node's `AppVersion` and its supported `SchemaRange`.
*   During the Raft join process, the Leader verifies that the joining node is compatible.
*   Nodes outside the minimum supported version are rejected or relegated to read-only "Observer" status.

### 3.2 Consensus-Aware Feature Flags
*   Changes to state reduction logic (e.g., how a specific play is scored) must be deterministic across all nodes.
*   New logic is gated by a cluster-wide "Active Schema Version." This version is only bumped once the Leader confirms that all nodes in the cluster have been upgraded to the required level.

## 4. Client-Server Compatibility

### 4.1 Capabilities Negotiation
*   Upon initial connection (WebSocket `JOIN` or `GET /api/cluster/status`), the server communicates its `AppVersion` and `ProtocolVersion`.
*   The client checks its own version against the server. If the client is too old, it triggers a forced refresh via the Service Worker to fetch the latest frontend assets.

### 4.2 Action Integrity
*   The server's `ValidateAction` layer enforces strict Version 3 compliance for all incoming actions. Actions in older formats are rejected.

## 5. Backup & Restore Interoperability

*   **Manifest Metadata:** The JSONL backup header includes the `AppVersion` and `SchemaVersion` at the time of export.
*   **Portable Reducer:** The system maintains a "Migration Path" for actions. If an action's structure changes radically, the `computeStateFromLog` logic includes legacy paths to handle old actions correctly.

## 6. Implementation Roadmap

1.  **[DONE] Add `schemaVersion`** to `Game`, `Team`, and `BaseAction` structs.
2.  **[DONE] Enhance `NodeMeta`** with versioning fields.
3.  **[DONE] Implement `UpgradeSchema()`** utilities in `backend/validation.go` and `frontend/reducer.js`.
4.  **[DONE] Update `BackupManager`** to include version headers in exports.
