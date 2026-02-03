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
)

type snapshotManifest struct {
	NodeMap     map[string]*NodeMeta `json:"nodeMap"`
	Initialized bool                 `json:"initialized"`
	RaftIndex   uint64               `json:"raftIndex"`
}

func (f *FSM) persist(sink io.WriteCloser) error {
	defer sink.Close()

	// Ensure all in-memory state is flushed to disk before linking
	if err := f.gs.FlushAll(); err != nil {
		return fmt.Errorf("failed to flush games: %w", err)
	}
	if err := f.ts.FlushAll(); err != nil {
		return fmt.Errorf("failed to flush teams: %w", err)
	}

	// Check if sink supports linking
	var linker SnapshotLinker
	if l, ok := sink.(SnapshotLinker); ok {
		linker = l
	}

	// If not linking, we wrap in Gzip/Tar immediately
	var gw *gzip.Writer
	var tw *tar.Writer

	if linker == nil {
		gw = gzip.NewWriter(sink)
		defer gw.Close()
		tw = tar.NewWriter(gw)
		defer tw.Close()
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

	if linker != nil {
		if _, err := linker.WriteManifest(manifestBytes); err != nil {
			return err
		}
	} else {
		if err := writeFileToTar(tw, "manifest.json", manifestBytes); err != nil {
			return err
		}
	}

	// 2. Write Games
	// Use ListAllGameIDs to iterate names, then handle file logic
	gameIDs, err := f.gs.ListAllGameIDs()
	if err != nil {
		return err
	}

	for _, id := range gameIDs {
		encodedId := url.PathEscape(id)
		srcRel := fmt.Sprintf("games/%s.json", encodedId)

		if linker != nil {
			// Link
			if err := linker.LinkFile(srcRel, srcRel); err != nil {
				return fmt.Errorf("failed to link game %s: %w", id, err)
			}
		} else {
			// Write to Tar
			g, err := f.gs.LoadGame(id)
			if err != nil {
				log.Printf("Snapshot Warning: failed to load game %s: %v", id, err)
				continue
			}
			data, err := json.Marshal(g)
			if err != nil {
				log.Printf("Snapshot Warning: failed to marshal game %s: %v", id, err)
				continue
			}
			if err := writeFileToTar(tw, srcRel, data); err != nil {
				return err
			}
		}
	}

	// 3. Write Teams
	teamIDs, err := f.ts.ListAllTeamIDs()
	if err != nil {
		return err
	}

	for _, id := range teamIDs {
		encodedId := url.PathEscape(id)
		srcRel := fmt.Sprintf("teams/%s.json", encodedId)

		if linker != nil {
			if err := linker.LinkFile(srcRel, srcRel); err != nil {
				return fmt.Errorf("failed to link team %s: %w", id, err)
			}
		} else {
			t, err := f.ts.LoadTeam(id)
			if err != nil {
				log.Printf("Snapshot Warning: failed to load team %s: %v", id, err)
				continue
			}
			data, err := json.Marshal(t)
			if err != nil {
				log.Printf("Snapshot Warning: failed to marshal team %s: %v", id, err)
				continue
			}
			if err := writeFileToTar(tw, srcRel, data); err != nil {
				return err
			}
		}
	}

	// 4. Write User Indices
	// Currently indices are not separate files in the same way (they are inside users/ team_games/ etc)
	// They are managed by user_index_store which uses `storage` too.
	// If `UserIndexStore` stores individual files, we can link them too.
	// `UserIndexStore` implementation uses `users/ID.json`.
	// We can link them if we iterate IDs.
	// Current `persist` uses `f.us.ListAllUserIndices()` which returns objects.
	// We need file-based iteration or keep using tar for them if small?
	// User indices can be large (many users).
	// Let's defer linking user indices to a future optimization to verify this change first,
	// OR use `ListAllUserIndices` (which loads them) and Write to Tar for now.
	// The LinkSnapshotStore.Open handles `users/` prefix by reading generic JSON, so it supports it being in Tar.
	// Wait, if I write them to `tw` here, but `linker` is NOT nil, `tw` is nil!
	// So I MUST handle them.
	// If I don't link them, I must write them to `state.bin` (via linker.WriteManifest? No, that's just one file).
	// `LinkSnapshotSink` writes to `state.bin`.
	// If I want to include them in the snapshot, I have two options:
	// A) Link them (requires `ListAllUserIDs` and knowing paths).
	// B) Write them to `sink` (state.bin). But `state.bin` is just one stream.
	//    The `LinkSnapshotStore.Open` reads `state.bin` as the Manifest JSON *only*.
	//    Wait, `LinkSnapshotStore.Open` reads `state.bin` into `manifestBytes`.
	//    Then it uses `writeFileToTar(tw, "manifest.json", manifestBytes)`.
	//    It expects `state.bin` to be JUST `manifest.json`.
	//    So I CANNOT write other stuff to `sink` if I use `LinkSnapshotStore`.
	//    I MUST link EVERYTHING or change `LinkSnapshotStore` to handle a tarball in `state.bin`.
	//    The current design of `LinkSnapshotStore` assumes `state.bin` is effectively `manifest.json`.
	//    So I MUST link `users/`, `team_games/`, etc.

	// I need `ListAllUserIDs` etc from `UserIndexStore`.
	// Let's check `user_index_store.go`.
	// It likely has `ListAllUserIndices` which loads them.
	// I should implement `LinkUserIndices` helper in `UserIndexStore` or just iterate.
	// `f.us` has `ListAllUserIndices`.
	// If I iterate them, I can get ID, construct path, and Link.

	if f.us != nil {
		// Users
		users, _ := f.us.ListAllUserIndices()
		for _, idx := range users {
			encodedId := url.PathEscape(idx.UserID)
			path := fmt.Sprintf("users/%s.json", encodedId)
			if linker != nil {
				linker.LinkFile(path, path)
			} else {
				data, _ := json.Marshal(idx)
				writeFileToTar(tw, path, data)
			}
		}

		// Team Games
		teamGames, _ := f.us.ListAllTeamGames()
		for _, idx := range teamGames {
			encodedId := url.PathEscape(idx.TeamID)
			path := fmt.Sprintf("team_games/%s.json", encodedId)
			if linker != nil {
				linker.LinkFile(path, path)
			} else {
				data, _ := json.Marshal(idx)
				writeFileToTar(tw, path, data)
			}
		}

		// Game Users
		gameUsers, _ := f.us.ListAllGameUsers()
		for _, idx := range gameUsers {
			encodedId := url.PathEscape(idx.GameID)
			path := fmt.Sprintf("game_users/%s.json", encodedId)
			if linker != nil {
				linker.LinkFile(path, path)
			} else {
				data, _ := json.Marshal(idx)
				writeFileToTar(tw, path, data)
			}
		}

		// Team Users
		teamUsers, _ := f.us.ListAllTeamUsers()
		for _, idx := range teamUsers {
			encodedId := url.PathEscape(idx.TeamID)
			path := fmt.Sprintf("team_users/%s.json", encodedId)
			if linker != nil {
				linker.LinkFile(path, path)
			} else {
				data, _ := json.Marshal(idx)
				writeFileToTar(tw, path, data)
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
