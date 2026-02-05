# Smart Snapshots & Fast Restore

The **Smart Snapshot** system optimizes the startup process for Raft nodes by intelligently skipping unnecessary snapshot restorations and parallelizing the restore process when it is required.

## 1. Persistence & Startup Strategy

The Skorekeeper FSM is natively **disk-backed**. All Game and Team data is written to persistent storage (`data/games/`, `data/teams/`) immediately upon application (via the `GameStore` and `TeamStore`).

Because the FSM state is durable, restoring data from a Raft snapshot during a clean startup is redundant and wasteful. It would simply overwrite existing files with the same data.

## 2. NoSnapshotRestoreOnStart

To optimize startup, we configure the Raft node with `NoSnapshotRestoreOnStart = true`.

This changes the startup behavior as follows:
1.  **Load Metadata:** Raft loads the latest snapshot metadata (Index, Term, Configuration) to initialize its internal state.
2.  **Skip Restore:** Raft **does not** call `fsm.Restore()`. The expensive process of reading the snapshot and writing data to disk is completely bypassed.
3.  **Log Replay:** Raft replays any Raft logs that have committed since the snapshot was taken (the "Trailing Logs").
4.  **Result:** Startup time is effectively **O(1)** relative to the dataset size, dominated only by the time to replay a small number of recent logs.

**Note:** The `fsm.Restore()` method is now invoked *only* in two scenarios:
*   **Replication (`InstallSnapshot`):** When a follower node falls too far behind the leader and needs to catch up via a full snapshot transfer.
*   **Disaster Recovery:** When manually restoring the cluster from a backup.

## 3. Fast Parallel Restore (The "Fast Path")

When a restore IS necessary (e.g., fresh node join via `InstallSnapshot`), the process is optimized for speed and memory safety.

### Parallel Write-Through
Instead of loading all data into memory and then saving it (which risks OOM), or processing serially (slow), the restore process uses a **Worker Pool**.

*   **Concurrency:** A pool of workers (sized to `runtime.NumCPU()`) handles incoming data items.
*   **Write-Through:** Items are written directly to disk using `RestoreGame`/`RestoreTeam`, bypassing the in-memory cache. This keeps memory usage low even when restoring gigabytes of data.
*   **Streaming:** The tar stream is read sequentially, but the expensive JSON marshaling, encryption, and disk I/O happen in parallel.

### Optimized Cleanup
After restoring, the system must delete local files that were not present in the snapshot ("zombies").
*   **Optimization:** Instead of loading every file to check its ID, `ListAllGameIDs` uses `os.ReadDir` to scan filenames only. This avoids unnecessary I/O during the cleanup phase.