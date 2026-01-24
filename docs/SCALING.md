# Scaling Analysis and Potential

This document provides a principal estimation of the scaling potential for a Skorekeeper cluster consisting of **3 nodes** running on modern hardware (NVMe SSDs, high-frequency multi-core CPUs).

## 1. Write Throughput: Concurrent Games

The primary bottleneck for writes is the **Raft Consensus and BoltDB `fsync` latency**. Every pitch or runner advancement requires a round-trip to a quorum (2/3 nodes) and a synchronous write to the BoltDB log.

*   **IOPS Constraint:** A standard NVMe SSD handles BoltDB `fsync` operations at roughly 1ms–2ms latency per commit. This limits the cluster to approximately **500–1,000 committed actions per second** globally.
*   **Scoring Cadence:** In a live baseball game, a "scoring event" (pitch, play result) occurs roughly every 15–20 seconds per game.
*   **Capacity:** 500 actions/sec * 15 sec/action ≈ **7,500 games being scored simultaneously.**
*   **Optimization Note:** If multiple actions are batched (e.g., a rapid series of runner advancements), the throughput per game increases, potentially lowering the total game count to ~5,000.

## 2. Read Capacity: Spectators

Spectator traffic is handled via WebSockets and is **horizontally scalable across all nodes** because reads do not go through the Raft log; they are served directly from the FSM state.

*   **Memory Footprint:** Each WebSocket connection in Go consumes roughly 15KB–30KB of memory (goroutine stacks + buffers). 
*   **Broadcasting Overhead:** The `Hub` mechanism broadcasts every committed action to all connected clients. This is a CPU-bound operation (JSON marshaling + network stack).
*   **Capacity:** 32GB of RAM could theoretically hold ~1,000,000 connections. However, CPU context switching and network bandwidth usually limit this first.
*   **Estimation:** A 3-node cluster can comfortably support **150,000 to 200,000 concurrent spectators** across all active games.

## 3. Storage: Persistence

The application stores games as individual JSON files.

*   **Game Size:** An average 9-inning game with a full action log is approximately 50KB to 100KB.
*   **Capacity:** 1TB of SSD storage can hold **10 million to 20 million historical games.**
*   **Performance Limit:** As the number of files in `data/games/` grows into the millions, the OS filesystem performance (directory entries/inodes) will become a bottleneck before disk capacity does. 
*   **Raft Logs:** The implementation of key rotation and snapshotting ensures the BoltDB log remains lean, as old logs are truncated after being snapshotted into the FSM state.

## 4. Memory Usage: Active State

The `Registry` and `FSM` keep metadata and active games in memory.

*   **Registry Index:** In-memory footprint is bounded by fixed-size LRU caches (approx. 5–10MB). The bulk of user-to-game mappings (unlimited users) resides on disk and is loaded on-demand.
*   **Active Game Objects:** Keeping 5,000 "active" games in the FSM memory (to avoid disk hits during live scoring) takes ~500MB.
*   **Total Profile:** Under heavy load (5k games, 100k spectators), the backend process will likely hover around **4GB–8GB of RSS memory.**

---

## Summary Scaling Matrix (Estimated)

| Metric | Estimated Limit (3-Node Cluster) | Constraint |
| :--- | :--- | :--- |
| **Concurrent Active Games** | **5,000 – 7,500** | Raft Quorum Latency / SSD fsync |
| **Total Concurrent Spectators** | **200,000+** | CPU (Broadcasting) / Memory |
| **Total Stored Games** | **10,000,000+** | Filesystem Inodes / Disk Space |
| **Request Latency (UI)** | **< 50ms** | Local-First Optimistic UI |
| **Persistence Latency** | **~5ms – 10ms** | Raft consensus (Node-to-Node RTT) |

## Future Scaling Recommendations

1.  **Disk Performance:** Ensure the `data/` directory is on an NVMe with high **O_DIRECT/fsync** performance. Avoid networked storage (EBS/EFS) if possible, as it significantly degrades Raft throughput.
2.  **Spectator Nodes:** For high-profile games (e.g., 10k+ spectators on a single game), implement a "Spectator Node" role. These nodes join the cluster as a `NonVoter`, allowing them to serve massive read traffic without participating in the write quorum or adding to consensus latency.
3.  **Directory Sharding:** If historical game counts exceed 100,000, the `GameStore` should be refactored to shard the `data/games/` directory (e.g., `data/games/ab/cd/uuid.json`) to mitigate filesystem performance degradation.
