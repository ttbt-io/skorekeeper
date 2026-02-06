# Encryption at Rest: Design & Implementation

This document details the architectural design and implementation of encryption at rest for the Skorekeeper application.

## 1. Overview

The system uses authenticated encryption (AES-256-GCM) to secure all persistent data on disk. This ensures that sensitive game and team data cannot be read if the physical storage media is compromised or if files are leaked.

The implementation relies on the `github.com/c2FmZQ/storage` library, which provides a high-level, secure API for encrypting data at rest and streaming encrypted data.

## 2. Scope of Encryption

Encryption is applied to the following data components:

*   **Entity Data:**
    *   Game files (`data/games/*.json`)
    *   Team files (`data/teams/*.json`)
*   **Consensus State:**
    *   Raft Log (`data/raft/raft-log.bolt`)
    *   Raft Stable Store (`data/raft/raft-stable.bolt`)
*   **Snapshots:**
    *   Raft Snapshots stored on disk.

## 3. Key Management

The system employs a **Node-Local Key Management** strategy with support for key rotation.

*   **Root Secret:** A master passphrase is provided to the application via the environment variable `SK_MASTER_KEY`.
*   **Master Key Derivation:**
    *   On startup, the node checks for a `data/master.key` file.
    *   If the file exists, it is decrypted using the provided passphrase to load the `crypto.MasterKey`.
    *   If the file does not exist, a new random Master Key is generated, encrypted with the passphrase, and saved to `data/master.key`.
*   **KeyRing Architecture:**
    *   Raft encryption keys are stored in `data/raft/keys/`.
    *   Keys are managed by a `KeyRing` structure which maintains an **Active Key** (for new writes) and a list of **Old Keys** (for decrypting older data).
    *   **Rotation:** Keys can be rotated programmatically. The new key becomes Active, and the previous Active key is demoted to Old. This ensures immediate write protection with new keys while maintaining read access to historical data.
    *   **Garbage Collection:** A background process runs periodically (every 15 minutes) to securely delete obsolete keys.
        *   It scans all retained snapshots **and** the oldest existing Raft Log to identify the oldest key currently in use.
        *   The retention horizon is the **older** of the oldest snapshot key and the oldest log key.
        *   It re-encrypts critical Stable Store metadata with the Active Key.
        *   Any keys strictly older than the retention horizon are permanently deleted from disk and memory.
*   **Isolation:** Each node in a cluster maintains its own unique `master.key` and encryption passphrase. Encryption keys are **never** shared across the network.
    *   This design simplifies rotation and decommissioning of nodes.
    *   Data replicated via Raft (Logs/Snapshots) is decrypted by the sender and re-encrypted by the receiver using their own local keys. Transport security is handled by mTLS.

## 4. Architecture Components

### 4.1 Entity Stores (`GameStore` & `TeamStore`)
The stores have been refactored to abstract file I/O through the `storage` library.

*   **Write Path:** When `SaveGame` or `SaveTeam` is called, the struct is serialized and encrypted using the node's Master Key before being written to disk atomically.
*   **Read Path:** `LoadGame` and `LoadTeam` decrypt and unmarshal the data on the fly.
*   **Lazy Migration:** To support upgrading existing deployments, the read path includes a fallback mechanism. If decryption fails (indicating a legacy plaintext file), the system attempts to read the file as standard JSON. If successful, the data is loaded, and subsequent writes will transparently encrypt the file.

### 4.2 Encrypted Raft Storage (`backend/raft_crypto.go`)
Since the Raft library (`hashicorp/raft`) manages its own persistence via BoltDB, we implemented the Decorator Pattern to inject encryption transparently using the `KeyRing`.

*   **`EncryptedLogStore`:** Wraps the standard `raft.LogStore`. It encrypts the `Data` payload of `raft.Log` entries with the **Active Key** before storing them. When retrieving, it attempts decryption with the Active Key, falling back to Old Keys if necessary.
*   **`EncryptedStableStore`:** Wraps `raft.StableStore`. It encrypts values (e.g., `CurrentTerm`, `LastVote`) stored in the key-value store. Critical values are automatically re-encrypted with the Active Key during garbage collection cycles.

### 4.3 Encrypted Snapshots (`backend/link_snapshot_store.go`)
Snapshots utilize filesystem hardlinks to minimize I/O and storage overhead (`LinkSnapshotStore`).

*   **Storage:**
    *   **Writes:** New snapshots create hardlinks to the active Game and Team files in the snapshot directory. These files remain encrypted at rest with the **Master Key**.
    *   **Manifest:** The snapshot metadata (`state.bin`) is encrypted using the **Active Raft Key**.
    *   **Reads:** `Open` decrypts the manifest with the Raft KeyRing. It then creates a reconstructed tar stream on-the-fly by reading the hardlinked files (decrypting them via the Master Key) and streaming the plaintext JSON.
*   **Replication:** When a snapshot is sent to another node (InstallSnapshot), the `Open` method provides a *decrypted* stream. This plaintext stream is sent over the secure mTLS transport. The receiving node's FSM then persists the data using its own local encryption keys.

## 5. Security Guarantees

*   **Confidentiality:** All data at rest is encrypted with AES-256-GCM.
*   **Integrity:** The GCM mode provides authenticated encryption, ensuring that data modification on disk is detected (decryption will fail).
*   **Key Isolation:** Compromise of one node's storage key does not compromise the encrypted storage of other nodes in the cluster.

## 6. Operational Considerations

*   **Environment Variables:** The `SK_MASTER_KEY` must be set in the production environment.
    *   If `SK_MASTER_KEY` is omitted and no `data/master.key` exists, the system will log a warning and fallback to plaintext storage.
    *   **CRITICAL:** If `data/master.key` exists but `SK_MASTER_KEY` is NOT provided, the application will exit with a fatal error immediately to prevent accidental unencrypted access or data exposure.
*   **Backups:** The `data/master.key` file MUST be backed up if you intend to restore raw data files to a new machine. Losing the `master.key` renders the encrypted data files unreadable, even if you know the passphrase.
*   **Performance:**
    *   Encryption adds a small CPU overhead to I/O operations.
    *   Listing metadata (`Registry.Rebuild`) has been optimized to use streaming iterators, but still requires decrypting file headers/content.
