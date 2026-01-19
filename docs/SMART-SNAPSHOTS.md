# Smart Snapshots & Fast Restore

The **Smart Snapshot** system optimizes the startup process for Raft nodes by intelligently skipping unnecessary snapshot restorations and parallelizing the restore process when it is required.

## 1. Context Awareness (Index Tracking)

To enable decision-making during startup, the FSM tracks the Raft Index of the last applied log entry.

*   **Runtime Tracking:** `FSM.lastAppliedIndex` (atomic) is updated on every `Apply` and `ApplyBatch`.
*   **Snapshot Manifest:** Every snapshot includes this index in its `manifest.json` as `raftIndex`.
*   **Local Persistence:** Whenever a snapshot is created, a lightweight `fsm_state.json` file is written to the data directory, recording the `lastAppliedIndex`.

## 2. Smart Restore Logic (The "Skip")

When a node restarts, Raft often provides the latest snapshot to ensure the FSM is up-to-date. However, if the node was shut down cleanly or crashed without data loss, its local disk state might already match or exceed the snapshot's state.

The `restore()` method performs the following check:
1.  Read `manifest.json` from the incoming snapshot stream.
2.  Read the local `fsm_state.json` file.
3.  **Condition:** If `local.lastAppliedIndex >= manifest.raftIndex` AND the node is already `Initialized`:
    *   **Action:** The restore of Game and Team data is **SKIPPED**.
    *   **Safety:** The `nodeMap` (cluster topology) from the manifest is still merged to ensure connectivity with peers is maintained/healed.
    *   **Benefit:** This turns an O(N) startup operation (where N is data size) into O(1), making restarts nearly instantaneous.

## 3. Fast Parallel Restore (The "Fast Path")

When a restore is necessary (e.g., fresh node join, data corruption recovery), the process is optimized for speed and memory safety.

### Parallel Write-Through
Instead of loading all data into memory and then saving it (which risks OOM), or processing serially (slow), the restore process uses a **Worker Pool**.

*   **Concurrency:** A pool of workers (sized to `runtime.NumCPU()`) handles incoming data items.
*   **Write-Through:** Items are written directly to disk using `RestoreGame`/`RestoreTeam`, bypassing the in-memory cache. This keeps memory usage low even when restoring gigabytes of data.
*   **Streaming:** The tar stream is read sequentially, but the expensive JSON marshaling, encryption, and disk I/O happen in parallel.

### Optimized Cleanup
After restoring, the system must delete local files that were not present in the snapshot ("zombies").
*   **Optimization:** Instead of loading every file to check its ID, `ListAllGameIDs` uses `os.ReadDir` to scan filenames only. This avoids unnecessary I/O during the cleanup phase.