# Raft Consensus Design & Architecture

This document details the implementation of the distributed consensus system in Skorekeeper, powered by `hashicorp/raft`. This architecture provides High Availability (HA), strong consistency for game data, and automatic failover.

## 1. Architecture Overview

Skorekeeper implements the **Embedded Raft** pattern. The application itself acts as the database node, managing both the consensus protocol and the state machine.

### 1.1 Core Components

*   **Raft Node (`RaftManager`):** The core component managing leader election, log replication, and peer communication. It uses a TCP transport layer secured by mutual TLS.
*   **Finite State Machine (`FSM`):** The bridge between the Raft log and the application's data. It is responsible for applying committed log entries to the local storage.
*   **Storage Layer:**
    *   **Raft Log:** A BoltDB database (`raft-log.bolt`) stores the sequential log of commands. Encrypted via `EncryptedLogStore`.
    *   **Stable Store:** A BoltDB database (`raft-stable.bolt`) stores election metadata (term, vote). Encrypted via `EncryptedStableStore`.
    *   **Application State:** The existing `GameStore` and `TeamStore` (JSON files on disk) serve as the materialized view of the FSM. Secured via authenticated encryption (see `docs/ENCRYPT-AT-REST.md`).

### 1.2 Data Flow

All state changes in the cluster must go through the Raft Leader.

1.  **Request:** A client sends a write request (e.g., `POST /api/action`) to *any* node.
2.  **Forwarding:**
    *   If the node is **Leader**: It processes the request directly.
    *   If the node is **Follower**: It identifies the Leader via internal metadata and proxies the HTTP request to the Leader's HTTP address, preserving the authentication token (JWT) and authenticating with the Cluster Secret.
3.  **Proposal:** The Leader serializes the request into a `RaftCommand` and proposes it to the Raft cluster.
4.  **Consensus:** The command is replicated to Followers. Once a quorum is reached, the command is "committed".
5.  **Application (`Apply`):**
    *   On **every node** (Leader and Followers), the committed command is passed to the `FSM`.
    *   The `FSM` updates the local JSON files (`data/games/*.json`).
    *   **Broadcast:** The `FSM` triggers the `HubManager` to broadcast the update via WebSockets to all locally connected clients.

## 2. Security Model: Zero-Trust & TOFU

The cluster uses a custom Zero-Trust security model for inter-node communication, ensuring that only authorized nodes can replicate data.

### 2.1 Identity
*   **Node Key:** On first startup, every node generates a persistent **Ed25519** private key (`node.key`) in its data directory.
*   **Node ID:** The Raft Node ID is automatically derived from the first 8 bytes of the Ed25519 public key (hex-encoded). This ensures a unique, deterministic, and cryptographically verifiable identity.
*   **Ephemeral TLS:** Nodes generate ephemeral self-signed X.509 certificates signed by their `node.key` for the Raft transport layer.

### 2.2 Mutual TLS & Authorization
*   **Authorized Keys:** The Raft FSM maintains a list of authorized public keys (`NodeMeta`).
*   **Handshake:** When two nodes connect, they exchange certificates. The connection is accepted **only if** the peer's Ed25519 public key matches an authorized key in the local FSM.

### 2.3 Bootstrap: Trust On First Use (TOFU)
To solve the "chicken and egg" problem of initial trust:
1.  **Strict Mode:** If a node is part of a cluster (knows >1 peer), it strictly enforces the authorized key list.
2.  **TOFU Mode:** If a node is "fresh" (has never joined a cluster, `initialized == false`) and is *not* the bootstrap leader, it effectively operates in "Trust On First Use" mode. It will accept a connection from an unknown peer (the Leader) to allow the initial join and replication of the cluster state (which contains the trusted keys). Once it has received metadata from any other node, it becomes "initialized" and strictly enforces the authorized key list for all future connections, even after restarts.

## 3. Operational Guide

### 3.1 Startup Flags

| Flag | Description | Default |
| :--- | :--- | :--- |
| `--raft` | Enable Raft consensus mode. | `false` |
| `--raft-bind` | `host:port` for internal Raft TCP transport. | `:8081` |
| `--raft-advertise` | **REQUIRED.** Public `host:port` for Raft traffic. | `""` |
| `--cluster-advertise` | **REQUIRED.** Public `host:port` for internal cluster mTLS API. | `""` |
| `--cluster-addr` | `host:port` to listen on for internal cluster mTLS API. | `:9090` |
| `--raft-vol` | Directory for Raft logs/keys (separate from data). | `data/raft` |
| `--raft-secret` | **REQUIRED** shared secret for API ops. | `""` |
| `--raft-bootstrap` | Initialize a new cluster (First node only). | `false` |
| `--addr` | Local TCP address to listen for HTTP (Client API). | `:8080` |

> **Note:** `--raft-advertise` and `--cluster-advertise` are mandatory when Raft is enabled. These flags ensure that other nodes know the exact address or hostname (including DNS and SNI support) to use for replication and request forwarding.

### 3.2 Auto-Configuration & Self-Healing

The system implements an automatic configuration mechanism. When a node starts:
1. It identifies the current cluster Leader.
2. It sends a "Self-Registration" request to the Leader with its current advertise addresses.
3. The Leader automatically updates the cluster configuration if the node's addresses have changed (e.g., after a container restart with a new IP).
4. **Bootstrapping Ingestion:** When a node bootstraps a new cluster, it automatically ingests any existing game and team data from disk into the Raft log, ensuring a smooth migration from standalone mode.

### 3.3 Bootstrapping a Cluster

**Node 1 (Leader):**
```bash
./skorekeeper --raft --raft-bootstrap --raft-secret "supersecret" \
  --addr :8080 \
  --raft-advertise raft1.example.com:5001 \
  --cluster-advertise api1.example.com:9090 \
  --raft-bind :5001 --cluster-addr :9090 --raft-vol data/node1
```

**Node 2 (Follower):**
```bash
./skorekeeper --raft --raft-secret "supersecret" \
  --addr :8081 \
  --raft-advertise raft2.example.com:5002 \
  --cluster-advertise api2.example.com:9091 \
  --raft-bind :5002 --cluster-addr :9091 --raft-vol data/node2
```

### 3.3 Joining Nodes

New nodes do not join automatically. You must instruct the *Leader* to add them. This exchanges public keys and updates the configuration.

**Via Dashboard:**
Navigate to `/api/cluster` on the Leader node. Use the "Add Node" form, enter the node's HTTP Address (e.g., `https://node-02-host:9091`) and Public Key fingerprint. The Leader will automatically discover the remaining configuration.

**Via API:**
```bash
curl -X POST https://leader-host/api/cluster/join \
  -H "X-Raft-Secret: supersecret" \
  -d '{
    "httpAddr": "https://node-02-host:9091",
    "pubKey": "<base64-pub-key-of-node-2>",
    "nonVoter": true
  }'
```
*Note: The `httpAddr` in the join request corresponds to the `--cluster-advertise` address of the joining node. The Leader will automatically fetch the node's ID and Raft address.*

> **Security Requirement:** The `--raft-secret` flag is **mandatory** when Raft is enabled. The server will fail to start if this secret is missing or empty. All cluster management endpoints strictly enforce this secret.*

## 4. Disaster Recovery

### 4.1 Snapshots
The system automatically snapshots the FSM (zipping `games/` and `teams/` directories) to prevent the Raft log from growing indefinitely. Snapshots are stored in the `--raft-vol` directory.

### 4.2 Restoring from Backup
If the cluster is lost:
1.  Stop all nodes.
2.  On one node, delete `raft-log.bolt` and `raft-stable.bolt` in the raft volume.
3.  Ensure the JSON data in `data/` is correct (restore from backup if needed).
4.  Start that node with `--raft-bootstrap` to force it to become a new Leader with term 1.
5.  Re-join other empty nodes.

## 5. Internals & Debugging

*   **Dashboard:** `/api/cluster` provides a real-time view of node status, leadership, and peers.
*   **Status API:** `GET /api/cluster/status` (requires Secret header) returns JSON topology.
*   **Logs:** Watch for `[Raft]` and `[FSM]` prefixes in stdout.