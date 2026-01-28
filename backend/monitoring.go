// Copyright (c) 2026 TTBT Enterprises LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"time"
)

// --- API / Raft Payloads ---

// MetricsPayload is the data carried by CmdMetricsUpdate.
type MetricsPayload struct {
	Timestamp int64          `json:"timestamp"` // Unix timestamp of the update
	Nodes     []NodeMetric   `json:"nodes,omitempty"`
	Cluster   *ClusterMetric `json:"cluster,omitempty"`
}

type NodeMetric struct {
	NodeID   string     `json:"nodeId"`
	RPS      float64    `json:"rps"`
	ActiveWS int        `json:"activeWS"`
	Latency  *Histogram `json:"latency,omitempty"`
}

type ClusterMetric struct {
	NodeCount    int    `json:"nodeCount"`
	Elections    uint64 `json:"elections"`
	LastLogIndex uint64 `json:"lastLogIndex"`
	Snapshots    uint64 `json:"snapshots"`
	LeaderGapMS  uint64 `json:"leaderGapMs"`
	TotalGames   int    `json:"totalGames"`
	TotalTeams   int    `json:"totalTeams"`
}

const LatencyBuckets = 101
const LatencyBucketSize = 50 * time.Millisecond

type Histogram struct {
	Buckets [LatencyBuckets]uint64 `json:"b2"`
	Count   uint64                 `json:"c"`
	Sum     float64                `json:"s"` // Sum of durations in milliseconds
}

func (h *Histogram) Add(d time.Duration) {
	ms := float64(d.Milliseconds())
	idx := int(d / LatencyBucketSize)
	if idx >= LatencyBuckets {
		idx = LatencyBuckets - 1
	}
	h.Buckets[idx]++
	h.Count++
	h.Sum += ms
}

func (h *Histogram) Merge(other *Histogram) {
	if other == nil {
		return
	}
	for i := 0; i < LatencyBuckets; i++ {
		h.Buckets[i] += other.Buckets[i]
	}
	h.Count += other.Count
	h.Sum += other.Sum
}

// --- Internal Storage (RRD) ---

// ResolutionConfig defines the policy for a single RRD bucket set.
type ResolutionConfig struct {
	Name       string        `json:"name"`
	Resolution time.Duration `json:"resolution"`
	Retention  time.Duration `json:"retention"`
	Buckets    int           `json:"buckets"`
}

var DefaultResolutions = []ResolutionConfig{
	{"1m", 1 * time.Minute, 2 * time.Hour, 120},
	{"5m", 5 * time.Minute, 6 * time.Hour, 72},
	{"15m", 15 * time.Minute, 24 * time.Hour, 96},
	{"1h", 1 * time.Hour, 31 * 24 * time.Hour, 744},
	{"1d", 24 * time.Hour, 183 * 24 * time.Hour, 183},
}

// Point represents a single data point in a time series.
type Point[T any] struct {
	Timestamp int64 `json:"t"`
	Value     T     `json:"v"`
}

// RingBuffer is a fixed-size circular buffer for storing time series data.
type RingBuffer[T any] struct {
	Config ResolutionConfig `json:"config"`
	Data   []Point[T]       `json:"data"`
	Head   int              `json:"head"` // Points to the *next* write position
}

func NewRingBuffer[T any](cfg ResolutionConfig) *RingBuffer[T] {
	return &RingBuffer[T]{
		Config: cfg,
		Data:   make([]Point[T], cfg.Buckets),
		Head:   0,
	}
}

// Add appends a point to the ring buffer.
func (rb *RingBuffer[T]) Add(timestamp int64, value T) {
	// Align timestamp to resolution
	resSec := int64(rb.Config.Resolution.Seconds())
	alignedTs := (timestamp / resSec) * resSec

	// Check if we just overwrote the last point (update in place) or if this is new
	prevIdx := (rb.Head - 1 + len(rb.Data)) % len(rb.Data)
	if rb.Data[prevIdx].Timestamp == alignedTs {
		rb.Data[prevIdx].Value = value
		return
	}

	rb.Data[rb.Head] = Point[T]{Timestamp: alignedTs, Value: value}
	rb.Head = (rb.Head + 1) % len(rb.Data)
}

// GetPoints returns the data points sorted by time.
func (rb *RingBuffer[T]) GetPoints() []Point[T] {
	points := make([]Point[T], 0, len(rb.Data))
	for i := 0; i < len(rb.Data); i++ {
		idx := (rb.Head + i) % len(rb.Data)
		if rb.Data[idx].Timestamp > 0 {
			points = append(points, rb.Data[idx])
		}
	}
	return points
}

// MetricSeries holds all resolutions for a specific metric.
type MetricSeries struct {
	Name            string                          `json:"name"`
	AggregationType string                          `json:"aggType"` // "Avg" or "Sum"
	Buffers         map[string]*RingBuffer[float64] `json:"buffers"`
}

func NewMetricSeries(name string, aggType string) *MetricSeries {
	if aggType == "" {
		aggType = "Avg"
	}
	buffers := make(map[string]*RingBuffer[float64])
	for _, cfg := range DefaultResolutions {
		buffers[cfg.Name] = NewRingBuffer[float64](cfg)
	}
	return &MetricSeries{
		Name:            name,
		AggregationType: aggType,
		Buffers:         buffers,
	}
}

func (ms *MetricSeries) Ingest(timestamp int64, value float64) {
	for _, cfg := range DefaultResolutions {
		buf, ok := ms.Buffers[cfg.Name]
		if !ok {
			continue
		}
		resSec := int64(cfg.Resolution.Seconds())
		alignedTs := (timestamp / resSec) * resSec
		prevIdx := (buf.Head - 1 + len(buf.Data)) % len(buf.Data)

		if buf.Data[prevIdx].Timestamp == alignedTs {
			if ms.AggregationType == "Sum" {
				buf.Data[prevIdx].Value += value
			} else if cfg.Name == "1m" {
				buf.Data[prevIdx].Value = value
			} else {
				// Running Average
				offset := timestamp - alignedTs
				n := (offset / 60) + 1
				oldAvg := buf.Data[prevIdx].Value
				buf.Data[prevIdx].Value = ((oldAvg * float64(n-1)) + value) / float64(n)
			}
		} else {
			buf.Add(timestamp, value)
		}
	}
}

// HistogramSeries holds all resolutions for a histogram metric.
type HistogramSeries struct {
	Name    string                            `json:"name"`
	Buffers map[string]*RingBuffer[Histogram] `json:"buffers"`
}

func NewHistogramSeries(name string) *HistogramSeries {
	buffers := make(map[string]*RingBuffer[Histogram])
	for _, cfg := range DefaultResolutions {
		buffers[cfg.Name] = NewRingBuffer[Histogram](cfg)
	}
	return &HistogramSeries{
		Name:    name,
		Buffers: buffers,
	}
}

func (hs *HistogramSeries) Ingest(timestamp int64, h *Histogram) {
	if h == nil {
		return
	}
	for _, cfg := range DefaultResolutions {
		buf, ok := hs.Buffers[cfg.Name]
		if !ok {
			continue
		}
		resSec := int64(cfg.Resolution.Seconds())
		alignedTs := (timestamp / resSec) * resSec
		prevIdx := (buf.Head - 1 + len(buf.Data)) % len(buf.Data)

		if buf.Data[prevIdx].Timestamp == alignedTs {
			// Merge into existing bucket
			buf.Data[prevIdx].Value.Merge(h)
		} else {
			buf.Add(timestamp, *h)
		}
	}
}

// MetricsStore is the top-level container in the FSM.
type MetricsStore struct {
	NodeMetrics    map[string]*MetricSeries    `json:"nodes"`
	NodeLatencies  map[string]*HistogramSeries `json:"latencies"`
	ClusterMetrics map[string]*MetricSeries    `json:"cluster"`
	LastUpdate     int64                       `json:"lastUpdate"`
}

func NewMetricsStore() *MetricsStore {
	return &MetricsStore{
		NodeMetrics:    make(map[string]*MetricSeries),
		NodeLatencies:  make(map[string]*HistogramSeries),
		ClusterMetrics: make(map[string]*MetricSeries),
	}
}

func (s *MetricsStore) GetNodeSeries(nodeID string) *MetricSeries {
	if s.NodeMetrics == nil {
		s.NodeMetrics = make(map[string]*MetricSeries)
	}
	if _, ok := s.NodeMetrics[nodeID]; !ok {
		s.NodeMetrics[nodeID] = NewMetricSeries("node:"+nodeID+":rps", "Avg")
	}
	return s.NodeMetrics[nodeID]
}

func (s *MetricsStore) GetNodeLatencySeries(nodeID string) *HistogramSeries {
	if s.NodeLatencies == nil {
		s.NodeLatencies = make(map[string]*HistogramSeries)
	}
	if _, ok := s.NodeLatencies[nodeID]; !ok {
		s.NodeLatencies[nodeID] = NewHistogramSeries("node:" + nodeID + ":latency")
	}
	return s.NodeLatencies[nodeID]
}

func (s *MetricsStore) GetClusterSeries(metricName string) *MetricSeries {
	if s.ClusterMetrics == nil {
		s.ClusterMetrics = make(map[string]*MetricSeries)
	}
	if _, ok := s.ClusterMetrics[metricName]; !ok {
		aggType := "Avg"
		if metricName == "leaderGapMs" {
			aggType = "Sum"
		}
		s.ClusterMetrics[metricName] = NewMetricSeries("cluster:"+metricName, aggType)
	}
	return s.ClusterMetrics[metricName]
}

func (s *MetricsStore) Hydrate() {
	if s.NodeMetrics == nil {
		s.NodeMetrics = make(map[string]*MetricSeries)
	}
	for _, series := range s.NodeMetrics {
		series.Hydrate()
	}
	if s.NodeLatencies == nil {
		s.NodeLatencies = make(map[string]*HistogramSeries)
	}
	for _, series := range s.NodeLatencies {
		series.Hydrate()
	}
	if s.ClusterMetrics == nil {
		s.ClusterMetrics = make(map[string]*MetricSeries)
	}
	for _, series := range s.ClusterMetrics {
		series.Hydrate()
	}
}

func (ms *MetricSeries) Hydrate() {
	if ms.Buffers == nil {
		ms.Buffers = make(map[string]*RingBuffer[float64])
	}
	for _, cfg := range DefaultResolutions {
		if _, ok := ms.Buffers[cfg.Name]; !ok {
			ms.Buffers[cfg.Name] = NewRingBuffer[float64](cfg)
		}
	}
}

func (hs *HistogramSeries) Hydrate() {
	if hs.Buffers == nil {
		hs.Buffers = make(map[string]*RingBuffer[Histogram])
	}
	for _, cfg := range DefaultResolutions {
		if _, ok := hs.Buffers[cfg.Name]; !ok {
			hs.Buffers[cfg.Name] = NewRingBuffer[Histogram](cfg)
		}
	}
}

func (s *MetricsStore) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"nodes":      s.NodeMetrics,
		"latencies":  s.NodeLatencies,
		"cluster":    s.ClusterMetrics,
		"lastUpdate": s.LastUpdate,
	}
}
