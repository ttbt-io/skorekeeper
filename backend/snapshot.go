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
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"runtime"
	"strings"
	"sync"
)

type snapshotManifest struct {
	NodeMap     map[string]*NodeMeta `json:"nodeMap"`
	Initialized bool                 `json:"initialized"`
	RaftIndex   uint64               `json:"raftIndex"`
}

func (f *FSM) persist(sink io.WriteCloser) error {
	defer sink.Close()

	// 1. Gzip Layer
	gz := gzip.NewWriter(sink)
	defer gz.Close()

	// 2. Tar Layer
	tw := tar.NewWriter(gz)
	defer tw.Close()

	// 3. Write Manifest
	nodes := make(map[string]*NodeMeta)
	f.nodeMap.Range(func(key, value interface{}) bool {
		nodes[key.(string)] = value.(*NodeMeta)
		return true
	})
	manifest := snapshotManifest{
		NodeMap:     nodes,
		Initialized: f.initialized.Load(),
		RaftIndex:   f.LastAppliedIndex(),
	}
	manifestBytes, _ := json.Marshal(manifest)
	if err := writeFileToTar(tw, "manifest.json", manifestBytes); err != nil {
		return err
	}

	// 4. Write Games (Logical Export)
	for g, err := range f.gs.ListAllGames() {
		if err != nil {
			return err
		}
		data, err := json.Marshal(g)
		if err != nil {
			log.Printf("Snapshot Warning: failed to marshal game %s: %v", g.ID, err)
			continue
		}
		if err := writeFileToTar(tw, fmt.Sprintf("games/%s.json", g.ID), data); err != nil {
			return err
		}
	}

	// 5. Write Teams (Logical Export)
	for t, err := range f.ts.ListAllTeams() {
		if err != nil {
			return err
		}
		data, err := json.Marshal(t)
		if err != nil {
			log.Printf("Snapshot Warning: failed to marshal team %s: %v", t.ID, err)
			continue
		}
		if err := writeFileToTar(tw, fmt.Sprintf("teams/%s.json", t.ID), data); err != nil {
			return err
		}
	}

	// 6. Write User Indices
	if f.us != nil {
		users, _ := f.us.ListAllUserIndices()
		for _, idx := range users {
			data, _ := json.Marshal(idx)
			if err := writeFileToTar(tw, fmt.Sprintf("users/%s.json", idx.UserID), data); err != nil {
				return err
			}
		}

		teamGames, _ := f.us.ListAllTeamGames()
		for _, idx := range teamGames {
			data, _ := json.Marshal(idx)
			if err := writeFileToTar(tw, fmt.Sprintf("team_games/%s.json", idx.TeamID), data); err != nil {
				return err
			}
		}

		gameUsers, _ := f.us.ListAllGameUsers()
		for _, idx := range gameUsers {
			data, _ := json.Marshal(idx)
			if err := writeFileToTar(tw, fmt.Sprintf("game_users/%s.json", idx.GameID), data); err != nil {
				return err
			}
		}

		teamUsers, _ := f.us.ListAllTeamUsers()
		for _, idx := range teamUsers {
			data, _ := json.Marshal(idx)
			if err := writeFileToTar(tw, fmt.Sprintf("team_users/%s.json", idx.TeamID), data); err != nil {
				return err
			}
		}
	}

	return nil
}

func (f *FSM) restore(rc io.Reader) error {
	gz, err := gzip.NewReader(rc)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	processedGames := make(map[string]bool)
	processedTeams := make(map[string]bool)
	shouldSkipRestore := false

	// Worker Pool Setup (for heavy Game/Team restore)
	numWorkers := runtime.NumCPU()
	jobs := make(chan interface{}, numWorkers)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-errCh:
					return
				default:
				}
				switch v := job.(type) {
				case *Game:
					if err := f.gs.RestoreGame(v); err != nil {
						select {
						case errCh <- err:
						default:
						}
					}
				case *Team:
					if err := f.ts.RestoreTeam(v); err != nil {
						select {
						case errCh <- err:
						default:
						}
					}
				}
			}
		}()
	}

	teardown := func() { close(jobs); wg.Wait() }

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			teardown()
			return err
		}

		select {
		case err := <-errCh:
			teardown()
			return err
		default:
		}

		if header.Size > 10*1024*1024 {
			teardown()
			return fmt.Errorf("snapshot entry %s too large: %d bytes", header.Name, header.Size)
		}

		if header.Name == "manifest.json" {
			var manifest snapshotManifest
			if err := json.NewDecoder(tr).Decode(&manifest); err != nil {
				teardown()
				return err
			}
			for k, v := range manifest.NodeMap {
				f.nodeMap.Store(k, v)
			}
			if manifest.Initialized {
				f.setInitialized()
			}

			// Smart Snapshot Check
			if f.IsInitialized() && f.storage != nil {
				var state map[string]any
				if err := f.storage.ReadDataFile("fsm_state.json", &state); err == nil {
					var localIndex uint64
					if v, ok := state["lastAppliedIndex"]; ok {
						// ... conversion logic ...
						switch val := v.(type) {
						case float64:
							localIndex = uint64(val)
						case int:
							localIndex = uint64(val)
						case int64:
							localIndex = uint64(val)
						case uint64:
							localIndex = val
						}
					}
					if localIndex >= manifest.RaftIndex && manifest.RaftIndex > 0 {
						log.Printf("Smart Restore: Local state (Index %d) is fresh enough. Skipping.", localIndex)
						shouldSkipRestore = true
					}
				}
			}
			continue
		}

		if shouldSkipRestore {
			continue
		}

		if strings.HasPrefix(header.Name, "games/") {
			var g Game
			if err := json.NewDecoder(tr).Decode(&g); err != nil {
				continue
			}
			processedGames[g.ID] = true
			select {
			case jobs <- &g:
			case err := <-errCh:
				teardown()
				return err
			}
		} else if strings.HasPrefix(header.Name, "teams/") {
			var t Team
			if err := json.NewDecoder(tr).Decode(&t); err != nil {
				continue
			}
			processedTeams[t.ID] = true
			select {
			case jobs <- &t:
			case err := <-errCh:
				teardown()
				return err
			}
		} else if strings.HasPrefix(header.Name, "users/") {
			// Restore User Index directly
			var idx UserIndex
			if err := json.NewDecoder(tr).Decode(&idx); err == nil {
				f.us.RestoreUserIndex(&idx)
			}
		} else if strings.HasPrefix(header.Name, "team_games/") {
			var idx TeamGamesIndex
			if err := json.NewDecoder(tr).Decode(&idx); err == nil {
				f.us.RestoreTeamGames(&idx)
			}
		} else if strings.HasPrefix(header.Name, "game_users/") {
			var idx GameUsersIndex
			if err := json.NewDecoder(tr).Decode(&idx); err == nil {
				f.us.RestoreGameUsers(&idx)
			}
		} else if strings.HasPrefix(header.Name, "team_users/") {
			var idx TeamUsersIndex
			if err := json.NewDecoder(tr).Decode(&idx); err == nil {
				f.us.RestoreTeamUsers(&idx)
			}
		}
	}

	teardown()
	select {
	case err := <-errCh:
		return err
	default:
	}

	f.saveNodes()

	if shouldSkipRestore {
		return nil
	}

	// Cleanup Zombies (games/teams only, indices cleaned up by overwrite or we accept staleness until next update)
	// Ideally we clean up indices too, but listing all files is expensive.
	// Since we overwrote active ones, stale ones might remain on disk but won't be in active set?
	// Actually, if we restore, we might want to clear directory first?
	// Raft usually snapshots FULL state.
	// Current cleanup only handles games/teams.
	// For indices, if a user was deleted, their index file remains.
	// It's acceptable for indices to be "additive" or we need to list and delete.
	// Given "Scalability", listing all users to delete zombies is slow.
	// We'll accept zombie index files for now (they are just disk space, not logically reachable if user doesn't exist).
	// But wait, if user exists but lost access, the file is overwritten.
	// If user is completely gone?
	// We don't have a list of "active users" in memory to check against.
	// So we skip zombie cleanup for users for now.

	gameIDs, err := f.gs.ListAllGameIDs()
	if err == nil {
		for _, id := range gameIDs {
			if !processedGames[id] {
				f.gs.DeleteGame(id)
			}
		}
	}
	teamIDs, err := f.ts.ListAllTeamIDs()
	if err == nil {
		for _, id := range teamIDs {
			if !processedTeams[id] {
				f.ts.DeleteTeam(id)
			}
		}
	}

	// Trigger Registry Rebuild/Reload?
	// Since we updated indices directly via userStore, Registry caches are invalid.
	// Registry Rebuild() rescans everything.
	// We should probably just invalidate Registry caches.
	// But Registry doesn't expose InvalidateAll.
	// Rebuild() is safe. But slow.
	// If we trust snapshot indices, we don't need Rebuild().
	// But Registry in-memory metadata cache is stale.
	// We should clear the cache.
	// f.registry.Rebuild() ?
	// Yes, to be safe and update counts/metadata.
	// But Rebuild() rescans.
	// If we want to avoid rescan, we need `Registry.Refresh()` that trusts the store?
	// For now, let's leave it. The caches will be populated on demand.
	// The only issue is `deletedGames` map in Registry.
	// Rebuild() rebuilds it.
	// So we MUST call Rebuild().
	f.r.Rebuild()

	return nil
}

func writeFileToTar(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name: name,
		Size: int64(len(data)),
		Mode: 0644,
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
