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
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/hashicorp/raft"
)

var ErrConflict = errors.New("conflict detected")

// FSM implements the raft.FSM interface.
type FSM struct {
	gs          *GameStore
	ts          *TeamStore
	us          *UserIndexStore
	r           *Registry
	hm          *HubManager
	storage     *storage.Storage
	initialized atomic.Bool
	rm          *RaftManager

	metricsMu sync.RWMutex
	metrics   *MetricsStore // Monitoring Data

	nodeMap          sync.Map // map[string]*NodeMeta
	lastAppliedIndex atomic.Uint64
}

// NewFSM creates a new FSM.
func NewFSM(gs *GameStore, ts *TeamStore, r *Registry, hm *HubManager, s *storage.Storage, us *UserIndexStore) *FSM {
	f := &FSM{
		gs:      gs,
		ts:      ts,
		us:      us,
		r:       r,
		hm:      hm,
		storage: s,
		metrics: NewMetricsStore(),
	}
	if s != nil {
		// We still need to check for existence using os.Stat because storage might not expose it easily.
		if _, err := os.Stat(filepath.Join(s.Dir(), "initialized")); err == nil {
			f.initialized.Store(true)
		}
		f.loadNodes()
	}
	return f
}

// LastAppliedIndex returns the index of the last applied log entry.
func (f *FSM) LastAppliedIndex() uint64 {
	return f.lastAppliedIndex.Load()
}

func (f *FSM) GetMetricsJSON() map[string]interface{} {
	f.metricsMu.RLock()
	defer f.metricsMu.RUnlock()
	return f.metrics.ToJSON()
}

func (f *FSM) GetTotalGames() int {
	return f.r.CountTotalGames()
}

func (f *FSM) GetTotalTeams() int {
	return f.r.CountTotalTeams()
}

func (f *FSM) GetActiveWSCount() int {
	if f.hm == nil {
		return 0
	}
	return f.hm.GetTotalConnectionCount()
}

func (f *FSM) GetLastMetricsTimestamp() int64 {
	f.metricsMu.RLock()
	defer f.metricsMu.RUnlock()

	ts := f.metrics.LastUpdate

	// If the last update was very recent (e.g. within 15s), it might be the current node's first report
	// clobbering the history. In that case, look for the previous point in the ring buffer.
	if ts > 0 && time.Since(time.Unix(ts, 0)) < 15*time.Second {
		if f.metrics.ClusterMetrics != nil {
			if series, ok := f.metrics.ClusterMetrics["nodeCount"]; ok {
				if buf, ok := series.Buffers["1m"]; ok {
					points := buf.GetPoints()
					// Look for a point strictly older than the bucket of the last update
					alignedLast := (ts / 60) * 60
					for i := len(points) - 1; i >= 0; i-- {
						if points[i].Timestamp < alignedLast {
							return points[i].Timestamp
						}
					}
				}
			}
		}
		// If no older point found, fall through to return ts (better than 0?)
		// If we return ts, gap is 0. If we return 0, gap is 0.
		// So it doesn't matter much, but let's return ts as best effort.
	}

	if ts > 0 {
		return ts
	}

	// Fallback for legacy data
	if f.metrics.ClusterMetrics == nil {
		return 0
	}
	if series, ok := f.metrics.ClusterMetrics["nodeCount"]; ok {
		if buf, ok := series.Buffers["1m"]; ok {
			points := buf.GetPoints()
			if len(points) > 0 {
				return points[len(points)-1].Timestamp
			}
		}
	}
	return 0
}

func (f *FSM) loadNodes() {
	if f.storage == nil {
		return
	}
	var nodes map[string]*NodeMeta
	if err := f.storage.ReadDataFile("nodes.json", &nodes); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("FSM Error: failed to read nodes.json: %v", err)
		}
		return
	}
	for k, v := range nodes {
		f.nodeMap.Store(k, v)
	}
}

func (f *FSM) saveNodes() {
	if f.storage == nil {
		return
	}
	nodes := make(map[string]*NodeMeta)
	f.nodeMap.Range(func(k, v interface{}) bool {
		nodes[k.(string)] = v.(*NodeMeta)
		return true
	})
	if err := f.storage.SaveDataFile("nodes.json", nodes); err != nil {
		log.Printf("FSM Error: failed to save nodes.json: %v", err)
	}
}

// IsInitialized returns true if the node has joined a cluster (processed a NodeMeta from another node).
func (f *FSM) IsInitialized() bool {
	return f.initialized.Load()
}

func (f *FSM) setInitialized() {
	if f.initialized.Swap(true) {
		return
	}
	if f.storage != nil {
		if err := f.storage.SaveDataFile("initialized", "true"); err != nil {
			log.Printf("FSM Error: failed to save initialized state: %v", err)
		}
	}
}

// Apply applies a Raft log entry to the key-value store.
func (f *FSM) Apply(l *raft.Log) interface{} {
	if len(l.Data) == 0 {
		return nil
	}
	var cmd RaftCommand
	var err error

	if f.rm != nil && f.rm.UseGob {
		dec := gob.NewDecoder(bytes.NewReader(l.Data))
		err = dec.Decode(&cmd)
	} else {
		err = json.Unmarshal(l.Data, &cmd)
	}

	if err != nil {
		log.Printf("FSM Apply Error: failed to decode command (gob=%v): %v", f.rm != nil && f.rm.UseGob, err)
		return err
	}

	res := f.applyCommand(cmd, l.Index)
	f.lastAppliedIndex.Store(l.Index)
	return res
}

func (f *FSM) GetHubManager() *HubManager {
	return f.hm
}

func (f *FSM) GetHub(id string, isTeam bool) *Hub {
	return f.hm.GetHub(id, isTeam, f.gs, f.ts, f.r)
}

func (f *FSM) GetStores() (*GameStore, *TeamStore) {
	return f.gs, f.ts
}

func (f *FSM) GetNodeCount() int {
	count := 0
	f.nodeMap.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func (f *FSM) GetAllNodes() map[string]string {
	nodes := make(map[string]string)
	f.nodeMap.Range(func(key, value interface{}) bool {
		if meta, ok := value.(*NodeMeta); ok {
			nodes[key.(string)] = meta.HttpAddr
		}
		return true
	})
	return nodes
}

func (f *FSM) GetNodeAddr(nodeID string) string {
	if val, ok := f.nodeMap.Load(nodeID); ok {
		if meta, ok := val.(*NodeMeta); ok {
			return meta.HttpAddr
		}
	}
	return ""
}

func (f *FSM) GetNodePubKey(nodeID string) string {
	if val, ok := f.nodeMap.Load(nodeID); ok {
		if meta, ok := val.(*NodeMeta); ok {
			return meta.PubKey
		}
	}
	return ""
}

func (f *FSM) GetNodeMeta(nodeID string) *NodeMeta {
	if val, ok := f.nodeMap.Load(nodeID); ok {
		if meta, ok := val.(*NodeMeta); ok {
			return meta
		}
	}
	return nil
}

func (f *FSM) applyNodeMeta(nodeID string, nodeInfo []byte) error {
	var meta NodeMeta
	if err := json.Unmarshal(nodeInfo, &meta); err != nil {
		return err
	}
	f.nodeMap.Store(nodeID, &meta)
	f.saveNodes()
	if f.rm != nil && nodeID != f.rm.NodeID {
		f.setInitialized()
	}
	return nil
}

func (f *FSM) applyAction(gameId string, data []byte, index uint64) error {
	g, err := f.gs.LoadGame(gameId)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to load game %s: %w", gameId, err)
		}
		g = &Game{ID: gameId}
	} else {
		if g.ID != gameId {
			return fmt.Errorf("data consistency error: loaded game ID %s does not match expected %s", g.ID, gameId)
		}
	}

	if index > 0 && index <= g.LastRaftIndex {
		return nil // Already applied
	}

	changed, err := ApplyAction(g, data)
	if err != nil {
		return err
	}

	if index > 0 {
		g.LastRaftIndex = index
		// Always save if index updated, even if action didn't change game state (though unlikely)
		// Actually if !changed but index > LastRaftIndex, we should save LastRaftIndex update?
		// Yes, to avoid re-processing in future.
	} else if !changed {
		return nil
	}

	if err := f.gs.SaveGameInMemory(g, f.rm == nil); err != nil {
		return err
	}
	newBytes, _ := json.Marshal(g)
	f.r.UpdateGame(*g)
	f.broadcastGameUpdate(gameId, newBytes, false, 1) // false = broadcast action
	return nil
}

func (f *FSM) broadcastGameUpdate(gameId string, data []byte, skipBroadcast bool, numActions int) {
	f.hm.BroadcastToGame(gameId, data, skipBroadcast, numActions)
}

func (f *FSM) applyActions(gameId string, actions []json.RawMessage, index uint64) error {
	g, err := f.gs.LoadGame(gameId)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to load game %s: %w", gameId, err)
		}
		g = &Game{ID: gameId}
	} else {
		if g.ID != gameId {
			return fmt.Errorf("data consistency error: loaded game ID %s does not match expected %s", g.ID, gameId)
		}
	}

	if index > 0 && index <= g.LastRaftIndex {
		return nil // Already applied
	}

	changed, err := ApplyActions(g, actions)
	if err != nil {
		return err
	}

	if index > 0 {
		g.LastRaftIndex = index
	} else if !changed {
		return nil
	}

	if err := f.gs.SaveGameInMemory(g, f.rm == nil); err != nil {
		return err
	}

	newBytes, _ := json.Marshal(g)

	f.r.UpdateGame(*g)
	f.broadcastGameUpdate(gameId, newBytes, false, len(actions))
	return nil
}

func (f *FSM) checkGameConflict(incoming *Game, existing *Game) error {
	if len(incoming.ActionLog) < len(existing.ActionLog) {
		return fmt.Errorf("incoming game state is older or forked (log length %d < %d): %w", len(incoming.ActionLog), len(existing.ActionLog), ErrConflict)
	}

	for i := 0; i < len(existing.ActionLog); i++ {
		var exID, inID struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(existing.ActionLog[i], &exID); err != nil {
			log.Printf("Warning: failed to unmarshal existing action ID at index %d: %v", i, err)
			continue
		}
		if err := json.Unmarshal(incoming.ActionLog[i], &inID); err != nil {
			log.Printf("Warning: failed to unmarshal incoming action ID at index %d: %v", i, err)
			continue
		}
		if exID.ID != inID.ID {
			return fmt.Errorf("history divergence at index %d (%s vs %s): %w", i, exID.ID, inID.ID, ErrConflict)
		}
	}
	return nil
}

func (f *FSM) repairLastActionID(g *Game) {
	if g.LastActionID == "" && len(g.ActionLog) > 0 {
		var act struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(g.ActionLog[len(g.ActionLog)-1], &act); err == nil {
			g.LastActionID = act.ID
		}
	}
}

func (f *FSM) applySaveGame(id string, data []byte, index uint64, force bool) error {
	// Optimization: Load header to check index? Or just unmarshal and overwrite?
	// If overwrite is older than current state (replayed), we should SKIP?
	// Yes, strict linearizability.

	var g Game
	if err := json.Unmarshal(data, &g); err != nil {
		return fmt.Errorf("failed to unmarshal game data: %w", err)
	}

	// We must check existing game index
	existing, err := f.gs.LoadGame(id)
	if err == nil {
		if index > 0 && index <= existing.LastRaftIndex {
			return nil
		}

		// Conflict Detection
		// If not forced, ensure strictly strictly forward history.
		if !force {
			if err := f.checkGameConflict(&g, existing); err != nil {
				return err
			}
		}
	}

	if index > 0 {
		g.LastRaftIndex = index
	}

	// Ensure LastActionID is set (self-repair)
	f.repairLastActionID(&g)

	if err := f.gs.SaveGame(&g); err != nil {
		return err
	}

	f.r.UpdateGame(g)
	f.broadcastGameUpdate(id, data, true, 0) // true = skip broadcast (overwrite)
	return nil
}

func (f *FSM) applyDeleteGame(id string, index uint64) error {
	existing, err := f.gs.LoadGame(id)
	if err == nil {
		if index > 0 && index <= existing.LastRaftIndex {
			return nil
		}
	}

	if err := f.gs.DeleteGame(id); err != nil {
		return err
	}
	f.r.DeleteGame(id)
	f.hm.RemoveHub(id, false)
	return nil
}

func (f *FSM) applySaveTeam(id string, data []byte, index uint64) error {
	var t Team
	if err := json.Unmarshal(data, &t); err != nil {
		return fmt.Errorf("failed to unmarshal team data: %w", err)
	}

	existing, err := f.ts.LoadTeam(id)
	if err == nil {
		if index > 0 && index <= existing.LastRaftIndex {
			return nil
		}
	}

	if index > 0 {
		t.LastRaftIndex = index
	}

	if err := f.ts.SaveTeamInMemory(&t, f.rm == nil); err != nil {
		return err
	}
	f.r.UpdateTeam(t)
	return nil
}

func (f *FSM) applyDeleteTeam(id string, index uint64) error {
	existing, err := f.ts.LoadTeam(id)
	if err == nil {
		if index > 0 && index <= existing.LastRaftIndex {
			return nil
		}
	}

	if err := f.ts.DeleteTeam(id); err != nil {
		return err
	}
	f.r.DeleteTeam(id)
	f.hm.RemoveHub(id, true)
	return nil
}

func (f *FSM) applyDeleteAllUser(userId string, index uint64) error {
	// 1. Delete Games
	// Registry.ListGames uses ReadLock, so it's safe to use for lookup.
	// We iterate all games accessible to user, then check ownership.
	// Note: In Raft, every node has the same Registry state (eventually).
	// ListGames returns ID list.
	gameIds := f.r.ListGames(userId, "", "", "")
	for _, id := range gameIds {
		g, err := f.gs.LoadGame(id)
		if err == nil && g.OwnerID == userId {
			f.applyDeleteGame(id, index)
		}
	}

	// 2. Delete Teams
	teamIds := f.r.ListTeams(userId, "", "", "")
	for _, id := range teamIds {
		t, err := f.ts.LoadTeam(id)
		if err == nil && t.OwnerID == userId {
			f.applyDeleteTeam(id, index)
		}
	}
	return nil
}

type batchItem struct {
	index     int // Original index in the []*raft.Log slice
	raftIndex uint64
	cmd       RaftCommand
}

type resourceJob struct {
	id       string
	isTeam   bool
	isSystem bool
	items    []batchItem

	// Output
	game          *Game
	team          *Team
	deleted       bool
	dirty         bool
	skipBroadcast bool
	totalActions  int
}

// ApplyBatch implements the raft.BatchingFSM interface.
func (f *FSM) ApplyBatch(logs []*raft.Log) []interface{} {
	results := make([]interface{}, len(logs))
	jobs := make(map[string]*resourceJob)

	// 1. Decode and Group
	for i, l := range logs {
		if l.Type != raft.LogCommand || len(l.Data) == 0 {
			continue
		}

		var cmd RaftCommand
		var err error

		if f.rm != nil && f.rm.UseGob {
			dec := gob.NewDecoder(bytes.NewReader(l.Data))
			err = dec.Decode(&cmd)
		} else {
			err = json.Unmarshal(l.Data, &cmd)
		}

		if err != nil {
			log.Printf("FSM ApplyBatch Error: failed to decode command (gob=%v): %v", f.rm != nil && f.rm.UseGob, err)
			results[i] = err
			continue
		}

		// Identify key (e.g., "game:123" or "team:456" or "sys:global")
		var key string
		var isTeam bool
		var isSystem bool
		switch cmd.Type {
		case CmdSaveGame, CmdDeleteGame:
			key = "game:" + cmd.ID
		case CmdApplyAction:
			if cmd.Action != nil {
				key = "game:" + cmd.Action.GameID
			}
		case CmdSaveTeam, CmdDeleteTeam:
			key = "team:" + cmd.ID
			isTeam = true
		case CmdNodeMeta, CmdNodeLeft, CmdUpdateAccessPolicy, CmdMetricsUpdate, CmdDeleteAllUser:
			key = "sys:global"
			isSystem = true
		default:
			results[i] = fmt.Errorf("unknown command type: %s", cmd.Type)
			continue
		}

		if key == "" {
			results[i] = fmt.Errorf("could not determine resource key for command type %s", cmd.Type)
			continue
		}

		if _, ok := jobs[key]; !ok {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) != 2 {
				results[i] = fmt.Errorf("malformed internal key: %s", key)
				continue
			}
			jobs[key] = &resourceJob{
				id:       parts[1],
				isTeam:   isTeam,
				isSystem: isSystem,
				items:    make([]batchItem, 0),
			}
		}
		jobs[key].items = append(jobs[key].items, batchItem{index: i, raftIndex: l.Index, cmd: cmd})
	}

	// 2. Execute Parallel (I/O and reduction)
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func(j *resourceJob) {
			defer wg.Done()
			f.processJob(j, results)
		}(job)
	}

	wg.Wait()

	// 3. Process Side Effects Sequentially (Registry and Broadcast)
	// This avoids deadlocks between resource locks and registry lock.
	for _, job := range jobs {
		if !job.dirty {
			continue
		}
		if job.isTeam {
			if job.deleted {
				f.r.DeleteTeam(job.id)
			} else if job.team != nil {
				f.r.UpdateTeam(*job.team)
			}
		} else if !job.isSystem {
			if job.deleted {
				f.r.DeleteGame(job.id)
			} else if job.game != nil {
				newBytes, err := json.Marshal(job.game)
				if err != nil {
					log.Printf("FSM ApplyBatch Error: failed to marshal game %s for broadcast: %v", job.id, err)
					continue
				}
				f.r.UpdateGame(*job.game)
				f.broadcastGameUpdate(job.id, newBytes, job.skipBroadcast, job.totalActions)
			}
		}
	}

	if len(logs) > 0 {
		f.lastAppliedIndex.Store(logs[len(logs)-1].Index)
	}

	return results
}

func (f *FSM) applyCommand(cmd RaftCommand, index uint64) interface{} {
	switch cmd.Type {
	case CmdSaveGame:
		return f.applySaveGame(cmd.ID, *cmd.GameData, index, cmd.Force)
	case CmdApplyAction:
		if len(cmd.Action.Actions) > 0 {
			return f.applyActions(cmd.Action.GameID, cmd.Action.Actions, index)
		}
		return f.applyAction(cmd.Action.GameID, cmd.Action.Action, index)
	case CmdDeleteGame:
		return f.applyDeleteGame(cmd.ID, index)
	case CmdSaveTeam:
		return f.applySaveTeam(cmd.ID, *cmd.TeamData, index)
	case CmdDeleteTeam:
		return f.applyDeleteTeam(cmd.ID, index)
	case CmdDeleteAllUser:
		if cmd.Action == nil || cmd.Action.UserID == "" {
			return fmt.Errorf("missing user id for delete all")
		}
		return f.applyDeleteAllUser(cmd.Action.UserID, index)
	case CmdNodeMeta:
		if cmd.NodeMeta == nil {
			return fmt.Errorf("missing node meta")
		}
		f.nodeMap.Store(cmd.NodeMeta.NodeID, cmd.NodeMeta)
		f.saveNodes()
		if f.rm != nil && (cmd.NodeMeta.NodeID != f.rm.NodeID || f.rm.Bootstrap) {
			f.setInitialized()
		}
		return nil
	case CmdNodeLeft:
		if cmd.NodeMeta == nil {
			return fmt.Errorf("missing node meta for leave")
		}
		f.nodeMap.Delete(cmd.NodeMeta.NodeID)
		f.saveNodes()
		return nil
	case CmdUpdateAccessPolicy:
		if cmd.PolicyData == nil {
			return fmt.Errorf("missing policy data")
		}
		return f.applyUpdateAccessPolicy(cmd.PolicyData)
	case CmdMetricsUpdate:
		if cmd.MetricsPayload == nil {
			return nil
		}
		return f.applyMetricsUpdate(cmd.MetricsPayload)
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

func (f *FSM) applyMetricsUpdate(p *MetricsPayload) error {
	f.metricsMu.Lock()
	defer f.metricsMu.Unlock()

	f.metrics.LastUpdate = p.Timestamp

	// 1. Apply Node Metrics
	for _, nm := range p.Nodes {
		series := f.metrics.GetNodeSeries(nm.NodeID)
		series.Ingest(p.Timestamp, nm.RPS)
		f.metrics.GetNodeSeries(nm.NodeID+":ws").Ingest(p.Timestamp, float64(nm.ActiveWS))
		f.metrics.GetNodeLatencySeries(nm.NodeID).Ingest(p.Timestamp, nm.Latency)
	}

	// 2. Apply Cluster Metrics
	if p.Cluster != nil {
		f.metrics.GetClusterSeries("nodeCount").Ingest(p.Timestamp, float64(p.Cluster.NodeCount))
		f.metrics.GetClusterSeries("elections").Ingest(p.Timestamp, float64(p.Cluster.Elections))
		f.metrics.GetClusterSeries("lastLogIndex").Ingest(p.Timestamp, float64(p.Cluster.LastLogIndex))
		f.metrics.GetClusterSeries("snapshots").Ingest(p.Timestamp, float64(p.Cluster.Snapshots))
		f.metrics.GetClusterSeries("leaderGapMs").Ingest(p.Timestamp, float64(p.Cluster.LeaderGapMS))
		f.metrics.GetClusterSeries("totalGames").Ingest(p.Timestamp, float64(p.Cluster.TotalGames))
		f.metrics.GetClusterSeries("totalTeams").Ingest(p.Timestamp, float64(p.Cluster.TotalTeams))
	}

	return nil
}

func (f *FSM) applyUpdateAccessPolicy(policy *UserAccessPolicy) error {
	// Persist to encrypted storage
	if f.storage != nil {
		if err := f.storage.SaveDataFile("sys_access_policy", policy); err != nil {
			return fmt.Errorf("failed to save access policy: %w", err)
		}
	}
	// Update in-memory registry cache (assuming Registry has this method, adding it next)
	f.r.UpdateAccessPolicy(policy)
	return nil
}

func (f *FSM) processJob(j *resourceJob, results []interface{}) {
	if j.isSystem {
		for _, item := range j.items {
			results[item.index] = f.applyCommand(item.cmd, item.raftIndex)
		}
	} else if j.isTeam {
		f.processTeamJob(j, results)
	} else {
		f.processGameJob(j, results)
	}
}

func (f *FSM) processGameJob(j *resourceJob, results []interface{}) {
	// 1. Load Once
	g, err := f.gs.LoadGame(j.id)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			for _, item := range j.items {
				results[item.index] = fmt.Errorf("failed to load game %s: %w", j.id, err)
			}
			return
		}
		g = &Game{ID: j.id}
	}

	dirty := false
	deleted := false
	totalActions := 0
	forceDiskSave := false

	// 2. Apply Loop (In-Memory)
	for _, item := range j.items {
		if item.raftIndex > 0 && item.raftIndex <= g.LastRaftIndex {
			results[item.index] = nil
			continue
		}

		if deleted {
			if item.cmd.Type != CmdSaveGame {
				results[item.index] = fmt.Errorf("cannot apply command to deleted game %s", j.id)
				continue
			}
			g = &Game{ID: j.id}
			deleted = false
		}

		switch item.cmd.Type {
		case CmdSaveGame:
			var newG Game
			if err := json.Unmarshal(*item.cmd.GameData, &newG); err != nil {
				results[item.index] = err
				continue
			}

			// Conflict Detection (same as applySaveGame)
			if !item.cmd.Force {
				if err := f.checkGameConflict(&newG, g); err != nil {
					results[item.index] = err
					continue
				}
			}

			g = &newG
			g.LastRaftIndex = item.raftIndex

			// Repair LastActionID if needed
			f.repairLastActionID(g)

			dirty = true
			deleted = false
			forceDiskSave = true
			j.skipBroadcast = true
			results[item.index] = nil

		case CmdApplyAction:
			var changed bool
			var actionErr error
			if len(item.cmd.Action.Actions) > 0 {
				changed, actionErr = ApplyActions(g, item.cmd.Action.Actions)
				if changed && actionErr == nil {
					totalActions += len(item.cmd.Action.Actions)
				}
			} else {
				changed, actionErr = ApplyAction(g, item.cmd.Action.Action)
				if changed {
					totalActions++
				}
			}
			if actionErr != nil {
				results[item.index] = actionErr
			} else {
				g.LastRaftIndex = item.raftIndex
				if changed {
					dirty = true
					j.skipBroadcast = false
				}
				results[item.index] = nil
			}

		case CmdDeleteGame:
			deleted = true
			g.LastRaftIndex = item.raftIndex
			dirty = true
			forceDiskSave = true
			results[item.index] = nil
		}
	}

	// 3. Save Once (if dirty)
	if dirty {
		if deleted {
			if err := f.gs.DeleteGame(j.id); err != nil {
				log.Printf("Batch Error: failed to delete game %s: %v", j.id, err)
				for _, item := range j.items {
					if results[item.index] == nil {
						results[item.index] = err
					}
				}
			} else {
				j.deleted = true
				j.dirty = true
			}
		} else {
			var saveErr error
			if forceDiskSave {
				saveErr = f.gs.SaveGame(g)
			} else {
				saveErr = f.gs.SaveGameInMemory(g, f.rm == nil)
			}

			if saveErr != nil {
				log.Printf("Batch Error: failed to save game %s: %v", j.id, saveErr)
				for _, item := range j.items {
					if results[item.index] == nil {
						results[item.index] = saveErr
					}
				}
			} else {
				j.game = g
				j.dirty = true
				j.totalActions = totalActions
			}
		}
	}
}

func (f *FSM) processTeamJob(j *resourceJob, results []interface{}) {
	t, err := f.ts.LoadTeam(j.id)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			for _, item := range j.items {
				results[item.index] = fmt.Errorf("failed to load team %s: %w", j.id, err)
			}
			return
		}
		t = &Team{ID: j.id}
	}

	dirty := false
	deleted := false
	forceDiskSave := false

	for _, item := range j.items {
		if item.raftIndex > 0 && item.raftIndex <= t.LastRaftIndex {
			results[item.index] = nil
			continue
		}

		if deleted {
			if item.cmd.Type != CmdSaveTeam {
				results[item.index] = fmt.Errorf("cannot apply command to deleted team %s", j.id)
				continue
			}
			t = &Team{ID: j.id}
			deleted = false
		}

		switch item.cmd.Type {
		case CmdSaveTeam:
			var newT Team
			if err := json.Unmarshal(*item.cmd.TeamData, &newT); err != nil {
				results[item.index] = err
				continue
			}
			t = &newT
			t.LastRaftIndex = item.raftIndex
			dirty = true
			j.skipBroadcast = true
			results[item.index] = nil
		case CmdDeleteTeam:
			deleted = true
			t.LastRaftIndex = item.raftIndex
			dirty = true
			forceDiskSave = true
			results[item.index] = nil
		}
	}

	if dirty {
		if deleted {
			if err := f.ts.DeleteTeam(j.id); err != nil {
				log.Printf("Batch Error: failed to delete team %s: %v", j.id, err)
				for _, item := range j.items {
					if results[item.index] == nil {
						results[item.index] = err
					}
				}
			} else {
				j.deleted = true
				j.dirty = true
			}
		} else {
			var saveErr error
			if forceDiskSave {
				saveErr = f.ts.SaveTeam(t)
			} else {
				saveErr = f.ts.SaveTeamInMemory(t, f.rm == nil)
			}

			if saveErr != nil {
				log.Printf("Batch Error: failed to save team %s: %v", j.id, saveErr)
				for _, item := range j.items {
					if results[item.index] == nil {
						results[item.index] = saveErr
					}
				}
			} else {
				j.team = t
				j.dirty = true
			}
		}
	}
}

// FSMSnapshot represents a snapshot of the FSM state.
type FSMSnapshot struct {
	fsm *FSM
}

// Persist saves the snapshot to the given sink.
func (s *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	return s.fsm.persist(sink)
}

// Release releases the snapshot.
func (s *FSMSnapshot) Release() {}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	// 1. Flush all dirty state to disk so the snapshotter reads fresh data
	if err := f.gs.FlushAll(); err != nil {
		log.Printf("FSM Snapshot Error: flushing games failed: %v", err)
		return nil, err
	}
	if err := f.ts.FlushAll(); err != nil {
		log.Printf("FSM Snapshot Error: flushing teams failed: %v", err)
		return nil, err
	}
	if err := f.us.FlushAll(); err != nil {
		log.Printf("FSM Snapshot Error: flushing user indices failed: %v", err)
		return nil, err
	}

	if f.rm != nil {
		if err := f.rm.RotateLogKey(); err != nil {
			log.Printf("Warning: failed to rotate log key during snapshot: %v", err)
		}
	}

	// Persist local state marker
	state := map[string]any{
		"lastAppliedIndex": f.LastAppliedIndex(),
		"timestamp":        time.Now().UnixNano(),
	}
	if f.storage != nil {
		if err := f.storage.SaveDataFile("fsm_state.json", state); err != nil {
			log.Printf("Warning: failed to save fsm_state.json: %v", err)
		}
		// Persist Metrics
		f.metricsMu.RLock()
		if err := f.storage.SaveDataFile("metrics.json", f.metrics); err != nil {
			log.Printf("Warning: failed to save metrics.json: %v", err)
		}
		f.metricsMu.RUnlock()
	}

	return &FSMSnapshot{fsm: f}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	if err := f.restore(rc); err != nil {
		return err
	}
	// Re-build registry after restoration
	f.r.Rebuild()
	// Restore Metrics
	if f.storage != nil {
		var m MetricsStore
		if err := f.storage.ReadDataFile("metrics.json", &m); err == nil {
			m.Hydrate()
			f.metricsMu.Lock()
			f.metrics = &m
			f.metricsMu.Unlock()
		} else if !os.IsNotExist(err) {
			log.Printf("Warning: failed to restore metrics.json: %v", err)
		}
	}
	return nil
}

func (f *FSM) FlushAll() error {
	if err := f.gs.FlushAll(); err != nil {
		return err
	}
	if err := f.ts.FlushAll(); err != nil {
		return err
	}
	if err := f.us.FlushAll(); err != nil {
		return err
	}
	return nil
}
