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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/hashicorp/raft"
)

func TestRingBuffer_AddAndGet(t *testing.T) {
	cfg := ResolutionConfig{
		Name:       "1m",
		Resolution: 60 * time.Second,
		Buckets:    5,
	}
	rb := NewRingBuffer[float64](cfg)

	// Add points
	baseTime := int64(1000000) // arbitrary start
	// Add 1st point
	rb.Add(baseTime, 10.0)
	points := rb.GetPoints()
	if len(points) != 1 {
		t.Errorf("Expected 1 point, got %d", len(points))
	}
	if points[0].Value != 10.0 {
		t.Errorf("Expected value 10.0, got %f", points[0].Value)
	}

	// Add 2nd point (next minute)
	rb.Add(baseTime+60, 20.0)
	points = rb.GetPoints()
	if len(points) != 2 {
		t.Errorf("Expected 2 points, got %d", len(points))
	}

	// Update existing point (same timestamp)
	rb.Add(baseTime+60, 25.0)
	points = rb.GetPoints()
	if len(points) != 2 {
		t.Errorf("Expected 2 points after update, got %d", len(points))
	}
	if points[1].Value != 25.0 {
		t.Errorf("Expected updated value 25.0, got %f", points[1].Value)
	}

	// Fill buffer
	rb.Add(baseTime+120, 30.0)
	rb.Add(baseTime+180, 40.0)
	rb.Add(baseTime+240, 50.0)
	// Now has 5 points: 10, 25, 30, 40, 50

	// Wrap around (overwrite first point)
	rb.Add(baseTime+300, 60.0)
	points = rb.GetPoints()
	if len(points) != 5 {
		t.Errorf("Expected 5 points after wrap, got %d", len(points))
	}
	// Oldest should now be baseTime+60 (value 25.0)
	if points[0].Timestamp != ((baseTime + 60) / 60 * 60) {
		t.Errorf("Expected oldest timestamp %d, got %d", (baseTime+60)/60*60, points[0].Timestamp)
	}
	if points[4].Value != 60.0 {
		t.Errorf("Expected newest value 60.0, got %f", points[4].Value)
	}
}

func TestMetricSeries_IngestAggregation(t *testing.T) {
	// Setup series with default resolutions
	ms := NewMetricSeries("test_metric", "Avg")

	baseTime := int64(1700000000) // rounded start

	// 1. Ingest 5 minutes of data (1m resolution)
	// 10, 20, 30, 40, 50
	// 5m Average should be 30.

	inputs := []float64{10, 20, 30, 40, 50}
	for i, v := range inputs {
		ms.Ingest(baseTime+int64(i*60), v)
	}

	// Check 1m buffer
	points1m := ms.Buffers["1m"].GetPoints()
	if len(points1m) != 5 {
		t.Errorf("Expected 5 points in 1m buffer, got %d", len(points1m))
	}

	// Check 5m buffer
	// All these points fall into the same 5m bucket
	baseTime = 3000 // Multiple of 60 and 300.

	ms = NewMetricSeries("test_metric_aligned", "Avg")
	inputs = []float64{10, 20, 30, 40, 50}

	for i, v := range inputs {
		ms.Ingest(baseTime+int64(i*60), v)
	}

	points5m := ms.Buffers["5m"].GetPoints()
	if len(points5m) != 1 {
		t.Errorf("Expected 1 point in 5m buffer, got %d", len(points5m))
	} else {
		// Value should be average of 10,20,30,40,50 = 30
		if points5m[0].Value != 30.0 {
			t.Errorf("Expected 5m average 30.0, got %f", points5m[0].Value)
		}
	}

	// Add data for next bucket
	ms.Ingest(baseTime+300, 100.0) // Time 3300
	points5m = ms.Buffers["5m"].GetPoints()
	if len(points5m) != 2 {
		t.Errorf("Expected 2 points in 5m buffer, got %d", len(points5m))
	}
	if points5m[1].Value != 100.0 {
		t.Errorf("Expected 2nd bucket value 100.0, got %f", points5m[1].Value)
	}
}

func TestFSMMetricsPersistence(t *testing.T) {
	// Setup FSM
	tmpDir := t.TempDir()
	s := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, nil)
	r := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, r, NewHubManager(), s, us)

	// Apply Metrics Update
	payload := &MetricsPayload{
		Timestamp: 1000,
		Nodes: []NodeMetric{
			{NodeID: "n1", RPS: 5.5},
		},
		Cluster: &ClusterMetric{
			NodeCount: 3,
		},
	}

	// Apply
	if res := fsm.applyMetricsUpdate(payload); res != nil {
		t.Fatalf("Failed to apply metrics: %v", res)
	}

	// Verify in memory
	series := fsm.metrics.GetNodeSeries("n1")
	points := series.Buffers["1m"].GetPoints()
	if len(points) == 0 || points[0].Value != 5.5 {
		t.Errorf("Metrics not stored in memory")
	}

	// Snapshot
	sink := &monitoringSnapshotSink{data: make([]byte, 0)}
	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if err := snapshot.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Verify that `metrics.json` exists on disk.
	metricsPath := filepath.Join(tmpDir, "metrics.json")
	if _, err := os.Stat(metricsPath); os.IsNotExist(err) {
		t.Fatalf("metrics.json not found on disk after Snapshot")
	}

	// Restore Verification
	var m2 MetricsStore
	if err := s.ReadDataFile("metrics.json", &m2); err != nil {
		t.Fatalf("Failed to read metrics.json back: %v", err)
	}

	series2 := m2.GetNodeSeries("n1")
	points2 := series2.Buffers["1m"].GetPoints()
	if len(points2) == 0 || points2[0].Value != 5.5 {
		t.Errorf("Restored metrics mismatch: %+v", points2)
	}
}

type monitoringSnapshotSink struct {
	data []byte
}

func (m *monitoringSnapshotSink) Write(p []byte) (n int, err error) {
	m.data = append(m.data, p...)
	return len(p), nil
}
func (m *monitoringSnapshotSink) Close() error  { return nil }
func (m *monitoringSnapshotSink) ID() string    { return "mock" }
func (m *monitoringSnapshotSink) Cancel() error { return nil }

func TestFSM_ApplyBatch_Metrics(t *testing.T) {
	// Setup FSM
	tmpDir := t.TempDir()
	s := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, nil)
	r := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, r, NewHubManager(), s, us)

	// Construct CmdMetricsUpdate
	payload := &MetricsPayload{
		Timestamp: 2000,
		Nodes: []NodeMetric{
			{NodeID: "batchNode", RPS: 10.0, ActiveWS: 5},
		},
		Cluster: &ClusterMetric{
			NodeCount: 5,
		},
	}
	cmd := RaftCommand{
		Type:           CmdMetricsUpdate,
		MetricsPayload: payload,
	}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	// Create Raft Log
	logEntry := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  cmdBytes,
	}

	// Execute ApplyBatch
	results := fsm.ApplyBatch([]*raft.Log{logEntry})
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0] != nil {
		t.Fatalf("ApplyBatch returned error: %v", results[0])
	}

	// Verify Metrics Applied
	series := fsm.metrics.GetNodeSeries("batchNode")
	points := series.Buffers["1m"].GetPoints()
	if len(points) == 0 {
		t.Fatalf("Metrics not applied from batch")
	}
	if points[0].Value != 10.0 {
		t.Errorf("Expected RPS 10.0, got %f", points[0].Value)
	}

	// Verify WS metric
	wsSeries := fsm.metrics.GetNodeSeries("batchNode:ws")
	wsPoints := wsSeries.Buffers["1m"].GetPoints()
	if len(wsPoints) == 0 {
		t.Fatalf("WS Metrics not applied from batch")
	}
	if wsPoints[0].Value != 5.0 {
		t.Errorf("Expected WS 5.0, got %f", wsPoints[0].Value)
	}
}

func getFreePortMon() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestLeaderGap_Restart(t *testing.T) {
	// 1. Setup Data Dir
	dataDir := t.TempDir()

	// Pre-allocate ports to ensure stability across restarts
	raftPort := getFreePortMon()
	clusterPort := getFreePortMon()

	startNode := func(bootstrap bool) (*RaftManager, *httptest.Server) {
		raftBind := fmt.Sprintf("127.0.0.1:%d", raftPort)
		clusterAddr := fmt.Sprintf("127.0.0.1:%d", clusterPort)

		s := storage.New(dataDir, nil)
		gStore := NewGameStore(dataDir, s)
		tStore := NewTeamStore(dataDir, s)
		us := NewUserIndexStore(dataDir, s, nil)
		reg := NewRegistry(gStore, tStore, us, true)
		rmChan := make(chan *RaftManager, 1)

		opts := Options{
			DataDir:               dataDir,
			GameStore:             gStore,
			TeamStore:             tStore,
			Storage:               s,
			Registry:              reg,
			RaftEnabled:           true,
			RaftBind:              raftBind,
			RaftAdvertise:         raftBind,
			ClusterAdvertise:      clusterAddr,
			ClusterAddr:           clusterAddr,
			RaftSecret:            "secret",
			RaftBootstrap:         bootstrap,
			RaftManagerChan:       rmChan,
			UseMockAuth:           true,
			UseProductionTimeouts: false, // Fast election
		}

		// Ensure ports are free before starting
		for i := 0; i < 10; i++ {
			if l, err := net.Listen("tcp", clusterAddr); err == nil {
				l.Close()
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		for i := 0; i < 10; i++ {
			if l, err := net.Listen("tcp", raftBind); err == nil {
				l.Close()
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		_, _, handler := NewServerHandler(opts)
		ts := httptest.NewServer(handler)

		var rm *RaftManager
		select {
		case rm = <-rmChan:
		case <-time.After(5 * time.Second):
			t.Fatal("RaftManager not initialized")
		}

		timeout := time.After(10 * time.Second)
		for {
			select {
			case <-timeout:
				t.Fatal("Timeout waiting for leader election")
			default:
				if rm.Raft.State().String() == "Leader" {
					return rm, ts
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 2. Start First Time
	rm1, ts1 := startNode(true)
	defer ts1.Close()

	// 3. Inject Metrics
	lastTs := time.Now().Unix()
	payload := &MetricsPayload{
		Timestamp: lastTs,
		Nodes: []NodeMetric{
			{NodeID: rm1.NodeID, RPS: 1.0},
		},
		Cluster: &ClusterMetric{
			NodeCount: 1,
		},
	}
	cmd := RaftCommand{Type: CmdMetricsUpdate, MetricsPayload: payload}
	if _, err := rm1.Propose(cmd); err != nil {
		t.Fatalf("Failed to propose metrics: %v", err)
	}

	rm1.WaitForSync(2 * time.Second)

	if ts := rm1.FSM.GetLastMetricsTimestamp(); ts != lastTs {
		t.Fatalf("FSM did not record metric timestamp. Expected %d, got %d", lastTs, ts)
	}

	// 4. Shutdown
	rm1.Shutdown()
	ts1.Close()

	// 5. Sleep to create gap
	time.Sleep(2 * time.Second)

	// 6. Restart
	rm2, ts2 := startNode(false)
	defer ts2.Close()

	time.Sleep(500 * time.Millisecond)

	client := ts2.Client()
	req, _ := http.NewRequest("GET", ts2.URL+"/api/cluster/metrics", nil)
	req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: "test@example.com"})

	rm2.reportMetrics()
	rm2.WaitForSync(2 * time.Second)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to query metrics API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("API returned status %d", resp.StatusCode)
	}

	var data MetricsStore
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode metrics JSON: %v", err)
	}

	series := data.GetClusterSeries("leaderGapMs")
	points := series.Buffers["1m"].GetPoints()

	if len(points) == 0 {
		t.Fatalf("No leader gap recorded in FSM (API)")
	}

	gapVal := points[len(points)-1].Value
	if gapVal < 1000 || gapVal > 8000 {
		t.Errorf("Expected gap around 2000ms (1000-8000), got %f", gapVal)
	}

	rm2.Shutdown()
}

func TestLeaderGap_SelfClobber(t *testing.T) {
	tmpDir := t.TempDir()
	s := storage.New(tmpDir, nil)
	gs := NewGameStore(tmpDir, s)
	ts := NewTeamStore(tmpDir, s)
	us := NewUserIndexStore(tmpDir, s, nil)
	r := NewRegistry(gs, ts, us, true)
	fsm := NewFSM(gs, ts, r, NewHubManager(), s, us)

	now := time.Now()
	oldTs := now.Add(-60 * time.Second).Unix()

	payloadOld := &MetricsPayload{
		Timestamp: oldTs,
		Cluster:   &ClusterMetric{NodeCount: 1},
	}
	fsm.applyMetricsUpdate(payloadOld)

	if ts := fsm.metrics.LastUpdate; ts != oldTs {
		t.Fatalf("LastUpdate mismatch. Expected %d, got %d", oldTs, ts)
	}

	newTs := now.Unix()
	payloadNew := &MetricsPayload{
		Timestamp: newTs,
		Cluster:   &ClusterMetric{NodeCount: 1},
	}
	fsm.applyMetricsUpdate(payloadNew)

	if ts := fsm.metrics.LastUpdate; ts != newTs {
		t.Fatalf("LastUpdate not clobbered. Expected %d, got %d", newTs, ts)
	}

	retrievedTs := fsm.GetLastMetricsTimestamp()
	alignedOldTs := (oldTs / 60) * 60

	if retrievedTs != alignedOldTs {
		t.Errorf("GetLastMetricsTimestamp failed to ignore self-clobber. Expected %d (aligned old), got %d", alignedOldTs, retrievedTs)
	}
}

func TestHistogram_AddAndMerge(t *testing.T) {
	h := &Histogram{}
	h.Add(40 * time.Millisecond)  // Bucket 0 (0-49ms)
	h.Add(50 * time.Millisecond)  // Bucket 1 (50-99ms)
	h.Add(150 * time.Millisecond) // Bucket 3 (150-199ms)
	h.Add(6 * time.Second)        // Bucket 100 (>= 5000ms)

	if h.Count != 4 {
		t.Errorf("Expected count 4, got %d", h.Count)
	}
	if h.Buckets[0] != 1 {
		t.Errorf("Bucket 0 mismatch: %d", h.Buckets[0])
	}
	if h.Buckets[1] != 1 {
		t.Errorf("Bucket 1 mismatch: %d", h.Buckets[1])
	}
	if h.Buckets[3] != 1 {
		t.Errorf("Bucket 3 mismatch: %d", h.Buckets[3])
	}
	if h.Buckets[LatencyBuckets-1] != 1 {
		t.Errorf("Last Bucket mismatch: %d", h.Buckets[LatencyBuckets-1])
	}

	h2 := &Histogram{}
	h2.Add(100 * time.Millisecond) // Bucket 2
	h.Merge(h2)

	if h.Count != 5 || h.Buckets[2] != 1 {
		t.Errorf("Merge failed")
	}
}

func TestHistogramSeries_Ingest(t *testing.T) {
	hs := NewHistogramSeries("test_latency")
	baseTime := int64(6000)

	h1 := &Histogram{}
	h1.Add(100 * time.Millisecond)
	hs.Ingest(baseTime, h1)

	h2 := &Histogram{}
	h2.Add(200 * time.Millisecond)
	hs.Ingest(baseTime+10, h2)

	points1m := hs.Buffers["1m"].GetPoints()
	if len(points1m) != 1 || points1m[0].Value.Count != 2 {
		t.Fatalf("Expected 1 point with count 2")
	}
}
