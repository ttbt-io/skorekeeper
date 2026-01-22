# Monitoring System Design

This document details the design for the internal self-monitoring system of Skorekeeper. The system tracks request rates and latency across the cluster using a distributed, consensus-backed approach.

## 1. Objectives

*   **Self-contained**: No external dependencies (Prometheus, Grafana, etc.) required for basic monitoring.
*   **Distributed**: Metrics are collected from all nodes and aggregated via Raft to ensure a consistent view.
*   **Multi-resolution**: Data is stored with varying granularities to support both real-time analysis and long-term trending.

## 2. Metric Specification

The system tracks two categories of metrics: **Node Metrics** (per-node) and **Cluster Metrics** (global).

### 2.1 Node Metrics
*   **HTTP Request Rate**: Requests per second (RPS).
*   **Active WebSockets**: Current number of connected clients.
*   **Request Latency**: Histogram of request durations (excluding WebSockets).

### 2.2 Cluster Metrics (Raft)
*   **Node Count**: Total active nodes in the cluster.
*   **Election Count**: Total number of leader elections since startup.
*   **Log Index**: The current Raft log index.
*   **Snapshot Count**: Total number of FSM snapshots performed.
*   **Leader Unavailable Time**: Cumulative milliseconds the cluster has spent without an active leader.
*   **Total Games/Teams**: Count of persistent entities.

### 2.3 Resolution & Retention Policies
All metrics use the same RRD-style time resolutions:

| Resolution | Retention | Buckets |
| :--- | :--- | :--- |
| 1 Minute | 2 Hours | 120 |
| 5 Minutes | 6 Hours | 72 |
| 15 Minutes | 1 Day | 96 |
| 1 Hour | 1 Month (31d) | 744 |
| 1 Day | 6 Months (183d) | 183 |

## 3. Histogram Design (Latency)

To accurately track latency distribution without storing individual request points, we use a fixed-bucket histogram.

### 3.1 Buckets
We use **41 buckets** to cover the range from 0ms to 5000ms+, with linear sizing for simplicity and efficient mapping.

*   **Bucket Size**: 125ms
*   **Range**: [0, 5000ms]
*   **Overflow**: The last bucket captures all requests > 5000ms.

**Mapping:** `BucketIndex = min(40, floor(Duration / 125ms))`

### 3.2 Aggregation
Histograms are aggregated by **Summing** corresponding buckets.
*   **Temporal Aggregation**: When downsampling from 1-minute to 5-minute resolution, the 5 histograms are merged bucket-by-bucket.
*   **Spatial Aggregation**: (Optional) Cluster-wide latency can be derived by merging histograms from all nodes for a given timestamp.

### 3.3 Derived Metrics
From the histogram, we approximate percentiles (P50, P90, P95, P99) by iterating through the buckets to find the count threshold.

## 4. Architecture & Data Flow

### 4.1 Local Collection
*   **Scalar Counters**: `uint64` counters for Total Requests and Active WebSockets.
*   **Latency Accumulator**: A thread-safe Histogram struct accumulating requests since the last report.
    *   **Exclude**: WebSocket upgrades/frames are excluded from latency tracking to avoid skewing data with long-lived connections.
*   **Middleware**: Intercepts HTTP requests to update counters and the histogram.

### 4.2 Reporting (Node -> Leader)
*   **Interval**: Every 60 seconds (aligned to the minute).
*   **Mechanism**: `POST /api/cluster/metrics`
*   **Payload**: Includes scalars and the accumulated Histogram.
*   **Reset**: The local Accumulator is reset to zero after each successful report.

### 4.3 Storage & Application (FSM)
*   **MetricsStore**:
    *   `NodeMetrics`: Map of `NodeID` -> `MetricSeries` (Scalars) and `HistogramSeries`.
    *   `ClusterMetrics`: Map of `MetricName` -> `MetricSeries`.
*   **Generic RingBuffer**: The storage backend is refactored to support generic types (`float64` for scalars, `Histogram` struct for latency).

## 5. Storage & Memory Estimates

Estimates per Node, assuming full retention (all buckets filled).

### 5.1 Scalar Metric Cost
*   **Storage**: `float64` (8 bytes) + `int64` timestamp (8 bytes) = 16 bytes per point.
*   **Total Points**: 120 + 72 + 96 + 744 + 183 = 1215 points per series.
*   **Cost**: ~19.4 KB per metric series.
*   **Per Node**: RPS + ActiveWS = ~39 KB.

### 5.2 Histogram Metric Cost
*   **Structure**: 
    *   `Buckets`: 41 * `uint64` (8 bytes) = 328 bytes.
    *   `Count`, `Sum`: 16 bytes.
    *   `Timestamp`: 8 bytes.
    *   **Total per Point**: ~352 bytes.
*   **Total Points**: 1215 points.
*   **Cost**: 1215 * 352 bytes ≈ **428 KB** per node.

### 5.3 Total System Cost
*   **Cluster Overhead** (Cluster Metrics): ~10 scalars * 19.4 KB ≈ 200 KB.
*   **Per-Node Overhead**: 39 KB (Scalars) + 428 KB (Histogram) ≈ **467 KB**.

**Example: 5-Node Cluster**
*   **RAM/Disk Usage**: 200 KB + (5 * 467 KB) ≈ **2.5 MB** total for full monitoring history.
*   This is negligible for modern systems.

## 6. Implementation Details

### 6.1 Raft Payload
```go
type MetricsPayload struct {
    Timestamp int64           `json:"timestamp"`
    Nodes     []NodeMetric    `json:"nodes,omitempty"`
    Cluster   *ClusterMetric  `json:"cluster,omitempty"`
}

type NodeMetric struct {
    NodeID   string     `json:"nodeId"`
    RPS      float64    `json:"rps"`
    ActiveWS int        `json:"activeWS"`
    Latency  *Histogram `json:"latency,omitempty"` // New field
}

type Histogram struct {
    Buckets [41]uint64 `json:"b"` // Concise JSON key
    Count   uint64     `json:"c"`
    Sum     float64    `json:"s"`
}
```

### 6.2 Visualization
*   **Heatmap**: A heatmap visualization is ideal for histograms over time, but for the MVP, we will compute and plot P50, P90, and P99 lines on a standard line chart.
*   **Computation**: Percentiles are computed on the client-side (Javascript) from the raw histogram data to allow dynamic toggling without backend re-calculation.