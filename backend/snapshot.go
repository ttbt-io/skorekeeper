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
	"strings"
)

type snapshotManifest struct {
	NodeMap     map[string]*NodeMeta `json:"nodeMap"`
	Initialized bool                 `json:"initialized"`
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
		if len(g.ActionLog) == 0 {
			log.Printf("Snapshot: Persisting game %s with EMPTY ActionLog!", g.ID)
		} else {
			if len(g.ActionLog)%500 == 0 {
				log.Printf("Snapshot: Persisting game %s with %d actions", g.ID, len(g.ActionLog))
			}
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

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Sanity check: limit entry size to 10MB to prevent OOM/Zip Bomb
		if header.Size > 10*1024*1024 {
			return fmt.Errorf("snapshot entry %s too large: %d bytes", header.Name, header.Size)
		}

		if header.Name == "manifest.json" {
			var manifest snapshotManifest
			if err := json.NewDecoder(tr).Decode(&manifest); err != nil {
				return err
			}
			for k, v := range manifest.NodeMap {
				f.nodeMap.Store(k, v)
			}
			if manifest.Initialized {
				f.setInitialized()
			}
			continue
		}

		if strings.HasPrefix(header.Name, "games/") {
			var g Game
			if err := json.NewDecoder(tr).Decode(&g); err != nil {
				log.Printf("Restore Warning: failed to unmarshal game %s: %v", header.Name, err)
				continue
			}
			if len(g.ActionLog) == 0 {
				log.Printf("Restore: Loaded game %s with EMPTY ActionLog from snapshot!", g.ID)
			} else {
				log.Printf("Restore: Loaded game %s with %d actions from snapshot", g.ID, len(g.ActionLog))
			}
			processedGames[g.ID] = true
			if err := f.gs.SaveGame(&g); err != nil {
				return err
			}
		} else if strings.HasPrefix(header.Name, "teams/") {
			var t Team
			if err := json.NewDecoder(tr).Decode(&t); err != nil {
				log.Printf("Restore Warning: failed to unmarshal team %s: %v", header.Name, err)
				continue
			}
			processedTeams[t.ID] = true
			if err := f.ts.SaveTeam(&t); err != nil {
				return err
			}
		}
	}

	// Persist learned node metadata to disk to ensure peer availability immediately
	// upon next restart, even before Raft log replay.
	f.saveNodes()

	// 6. Cleanup: Delete local files not in snapshot
	for g, err := range f.gs.ListAllGames() {
		if err == nil {
			if !processedGames[g.ID] {
				log.Printf("Cleanup: Deleting zombie game %s after restore", g.ID)
				f.gs.DeleteGame(g.ID)
			}
		}
	}

	for t, err := range f.ts.ListAllTeams() {
		if err == nil {
			if !processedTeams[t.ID] {
				log.Printf("Cleanup: Deleting zombie team %s after restore", t.ID)
				f.ts.DeleteTeam(t.ID)
			}
		}
	}

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
