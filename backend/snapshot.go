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
	"net/url"
	"runtime"
	"strings"
	"sync"

	"github.com/hashicorp/raft"
)

type snapshotManifest struct {
	NodeMap     map[string]*NodeMeta `json:"nodeMap"`
	Initialized bool                 `json:"initialized"`
	RaftIndex   uint64               `json:"raftIndex"`
}

func (f *FSM) persist(sink io.WriteCloser) (err error) {
	defer func() {
		if s, ok := sink.(raft.SnapshotSink); ok && err != nil {
			s.Cancel()
			return
		}
		sink.Close()
	}()

	// Ensure all in-memory state is flushed to disk before linking
	if err := f.gs.FlushAll(); err != nil {
		return fmt.Errorf("failed to flush games: %w", err)
	}
	if err := f.ts.FlushAll(); err != nil {
		return fmt.Errorf("failed to flush teams: %w", err)
	}
	if f.us != nil {
		if err := f.us.FlushAll(); err != nil {
			return fmt.Errorf("failed to flush user indices: %w", err)
		}
	}

	// Check if sink supports linking
	linker, ok := sink.(SnapshotLinker)
	if !ok {
		return fmt.Errorf("sink does not support SnapshotLinker interface")
	}

	// Helper to link files
	link := func(relPath string) error {
		if err := linker.LinkFile(relPath, relPath); err != nil {
			return fmt.Errorf("failed to link %s: %w", relPath, err)
		}
		return nil
	}

	// 1. Prepare Manifest
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

	if _, err := linker.WriteManifest(manifestBytes); err != nil {
		return err
	}

	// 2. Write Games
	gameIDs, err := f.gs.ListAllGameIDs()
	if err != nil {
		return err
	}
	for _, id := range gameIDs {
		rel := fmt.Sprintf("games/%s.json", url.PathEscape(id))
		if err := link(rel); err != nil {
			return err
		}
	}

	// 3. Write Teams
	teamIDs, err := f.ts.ListAllTeamIDs()
	if err != nil {
		return err
	}
	for _, id := range teamIDs {
		rel := fmt.Sprintf("teams/%s.json", url.PathEscape(id))
		if err := link(rel); err != nil {
			return err
		}
	}

	// 4. Write User Indices
	if f.us != nil {
		linkGroup := func(listFiles func() ([]string, error)) error {
			files, err := listFiles()
			if err != nil {
				return err
			}
			for _, path := range files {
				if err := link(path); err != nil {
					return err
				}
			}
			return nil
		}

		if err := linkGroup(f.us.ListUserIndexFiles); err != nil {
			return err
		}
		if err := linkGroup(f.us.ListTeamGamesFiles); err != nil {
			return err
		}
		if err := linkGroup(f.us.ListGameUsersFiles); err != nil {
			return err
		}
		if err := linkGroup(f.us.ListTeamUsersFiles); err != nil {
			return err
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
			if err := json.NewDecoder(tr).Decode(&idx); err != nil {
				log.Printf("Restore Warning: failed to unmarshal user index %s: %v", header.Name, err)
				continue
			}
			f.us.RestoreUserIndex(&idx)
		} else if strings.HasPrefix(header.Name, "team_games/") {
			var idx TeamGamesIndex
			if err := json.NewDecoder(tr).Decode(&idx); err != nil {
				log.Printf("Restore Warning: failed to unmarshal team_games index %s: %v", header.Name, err)
				continue
			}
			f.us.RestoreTeamGames(&idx)
		} else if strings.HasPrefix(header.Name, "game_users/") {
			var idx GameUsersIndex
			if err := json.NewDecoder(tr).Decode(&idx); err != nil {
				log.Printf("Restore Warning: failed to unmarshal game_users index %s: %v", header.Name, err)
				continue
			}
			f.us.RestoreGameUsers(&idx)
		} else if strings.HasPrefix(header.Name, "team_users/") {
			var idx TeamUsersIndex
			if err := json.NewDecoder(tr).Decode(&idx); err != nil {
				log.Printf("Restore Warning: failed to unmarshal team_users index %s: %v", header.Name, err)
				continue
			}
			f.us.RestoreTeamUsers(&idx)
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

	// Cleanup Zombies (Games and Teams only).
	// We delete any local entities that were not present in the snapshot to maintain consistency.
	// User index cleanup is currently skipped as listing all users is prohibitively expensive
	// for large datasets; zombie index files remain on disk but are logically unreachable.
	gameIDs, err := f.gs.ListAllGameIDs()
	if err == nil {
		for _, id := range gameIDs {
			if !processedGames[id] {
				f.gs.DeleteGame(id)
			}
		}
	} else {
		log.Printf("Restore Cleanup Warning: failed to list games for zombie cleanup: %v", err)
	}
	teamIDs, err := f.ts.ListAllTeamIDs()
	if err == nil {
		for _, id := range teamIDs {
			if !processedTeams[id] {
				f.ts.DeleteTeam(id)
			}
		}
	} else {
		log.Printf("Restore Cleanup Warning: failed to list teams for zombie cleanup: %v", err)
	}

	// Re-initialize the registry to use the restored on-disk indices
	// without performing a full, expensive rebuild.
	// We just refresh the file counts so stats are correct.
	// The existing Registry instance is preserved, keeping external references valid.
	f.r.RefreshCounts()

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
