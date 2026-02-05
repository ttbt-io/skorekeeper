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
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

// SnapshotLinker is the interface that FSM.Persist uses to link files.
type SnapshotLinker interface {
	LinkFile(srcRelPath string, dstRelPath string) error
	// WriteManifest writes the manifest data to the snapshot state file.
	// This is equivalent to Write() but explicitly named for clarity in the mixed mode.
	WriteManifest(p []byte) (int, error)
}

// LinkSnapshotStore implements raft.SnapshotStore using hardlinks for data files.
type LinkSnapshotStore struct {
	baseDir   string // Directory for Raft snapshots (e.g. /data/raft)
	sourceDir string // Directory for Source Data (e.g. /data)
	inner     *raft.FileSnapshotStore
	ring      *KeyRing
	masterKey crypto.MasterKey
}

// NewLinkSnapshotStore creates a new LinkSnapshotStore.
func NewLinkSnapshotStore(baseDir, sourceDir string, inner *raft.FileSnapshotStore, ring *KeyRing, masterKey crypto.MasterKey) *LinkSnapshotStore {
	return &LinkSnapshotStore{
		baseDir:   baseDir,
		sourceDir: sourceDir,
		inner:     inner,
		ring:      ring,
		masterKey: masterKey,
	}
}

func (s *LinkSnapshotStore) resolveSnapshotPath(id string) (string, error) {
	candidates := []string{
		filepath.Join(s.baseDir, "snapshots", id),
		filepath.Join(s.baseDir, "snapshots", id+".tmp"),
		filepath.Join(s.baseDir, id),
		filepath.Join(s.baseDir, id+".tmp"),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("snapshot directory not found for ID %s in %s", id, s.baseDir)
}

func (s *LinkSnapshotStore) Create(version raft.SnapshotVersion, index, term uint64, configuration raft.Configuration, snapshotSize uint64, trans raft.Transport) (raft.SnapshotSink, error) {
	sink, err := s.inner.Create(version, index, term, configuration, snapshotSize, trans)
	if err != nil {
		return nil, err
	}

	snapDir, err := s.resolveSnapshotPath(sink.ID())
	if err != nil {
		sink.Cancel()
		return nil, err
	}

	stream, err := s.ring.Active.Key.StartWriter([]byte(sink.ID()), sink)
	if err != nil {
		sink.Cancel()
		return nil, err
	}

	return &LinkSnapshotSink{
		inner:     sink,
		snapDir:   snapDir,
		sourceDir: s.sourceDir,
		stream:    stream,
	}, nil
}

func (s *LinkSnapshotStore) List() ([]*raft.SnapshotMeta, error) {
	return s.inner.List()
}

func (s *LinkSnapshotStore) openEncrypted(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	s.ring.mu.RLock()
	keys := make([]*KeyInfo, 0, 1+len(s.ring.Old))
	if s.ring.Active != nil {
		keys = append(keys, s.ring.Active)
	}
	keys = append(keys, s.ring.Old...)
	s.ring.mu.RUnlock()

	var lastErr error
	for _, info := range keys {
		if info == nil {
			continue
		}
		meta, rc, err := s.inner.Open(id)
		if err != nil {
			return nil, nil, err
		}

		reader, err := info.Key.StartReader([]byte(id), rc)
		if err != nil {
			rc.Close()
			lastErr = err
			continue
		}
		return meta, reader, nil
	}
	return nil, nil, fmt.Errorf("failed to open snapshot with any key: %w", lastErr)
}

func (s *LinkSnapshotStore) Open(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	meta, rc, err := s.openEncrypted(id)
	if err != nil {
		return nil, nil, err
	}

	br := bufio.NewReader(rc)
	header, err := br.Peek(2)
	if err == nil && len(header) == 2 && header[0] == 0x1f && header[1] == 0x8b {
		// It's a GZIP stream (Remote Snapshot).
		// Return the buffered reader wrapped to preserve Close.
		type bufReadCloser struct {
			*bufio.Reader
			io.Closer
		}
		return meta, &bufReadCloser{Reader: br, Closer: rc}, nil
	}

	manifestBytes, err := io.ReadAll(br)
	rc.Close() // Done with the original reader
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	snapDir, err := s.resolveSnapshotPath(id)
	if err != nil {
		return nil, nil, err
	}

	store := storage.New(snapDir, s.masterKey)
	const fullSnapshotName = "full-snapshot"

	reader, err := store.OpenBlobRead(fullSnapshotName)
	if errors.Is(err, os.ErrNotExist) {
		if err := s.createTar(store, manifestBytes, fullSnapshotName); err != nil {
			return nil, nil, err
		}
		reader, err = store.OpenBlobRead(fullSnapshotName)
	}
	if err != nil {
		return nil, nil, err
	}

	size, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, nil, err
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	meta.Size = size
	return meta, reader, nil
}

func (s *LinkSnapshotStore) createTar(store *storage.Storage, manifestBytes []byte, filename string) error {
	enc, err := store.OpenBlobWrite(filename, filename)
	if err != nil {
		return err
	}

	gz := gzip.NewWriter(enc)
	tw := tar.NewWriter(gz)

	if err := writeFileToTar(tw, "manifest.json", manifestBytes); err != nil {
		tw.Close()
		gz.Close()
		enc.Close()
		return err
	}

	if err := s.walkSnapshotEntities(store, func(path string, data []byte) error {
		return writeFileToTar(tw, path, data)
	}); err != nil {
		tw.Close()
		gz.Close()
		enc.Close()
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return enc.Close()
}

func (s *LinkSnapshotStore) walkSnapshotEntities(store *storage.Storage, visitor func(relPath string, data []byte) error) error {
	dir := store.Dir()
	tempRaftStore := storage.New(filepath.Join(store.Dir(), "raft"), s.masterKey)

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if strings.HasPrefix(relPath, "raft/") {
			raftPath := relPath[5:]
			var obj any

			switch raftPath {
			case "sys_access_policy":
				obj = &UserAccessPolicy{}
			case "metrics.json":
				obj = &MetricsStore{}
			case "nodes.json":
				var o map[string]*NodeMeta
				obj = &o
			default:
				return nil
			}

			if err := tempRaftStore.ReadDataFile(raftPath, obj); err != nil {
				return fmt.Errorf("failed to read %s: %w", relPath, err)
			}
			data, err := json.Marshal(obj)
			if err != nil {
				return fmt.Errorf("failed to marshal %s: %w", relPath, err)
			}
			if err := visitor(relPath, data); err != nil {
				return err
			}
			return nil
		}

		var obj any
		switch {
		case strings.HasPrefix(relPath, "games/"):
			obj = &Game{}
		case strings.HasPrefix(relPath, "teams/"):
			obj = &Team{}
		case strings.HasPrefix(relPath, "users/"):
			obj = &UserIndex{}
		case strings.HasPrefix(relPath, "team_games/"):
			obj = &TeamGamesIndex{}
		case strings.HasPrefix(relPath, "game_users/"):
			obj = &GameUsersIndex{}
		case strings.HasPrefix(relPath, "team_users/"):
			obj = &TeamUsersIndex{}
		default:
			return nil
		}
		if err := store.ReadDataFile(relPath, obj); err != nil {
			return fmt.Errorf("failed to read %s: %w", relPath, err)
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal %s: %w", relPath, err)
		}

		if err := visitor(relPath, data); err != nil {
			return err
		}
		return nil
	})
}

// LinkSnapshotSink implements raft.SnapshotSink
type LinkSnapshotSink struct {
	inner     raft.SnapshotSink
	snapDir   string
	sourceDir string
	stream    crypto.StreamWriter
}

func (s *LinkSnapshotSink) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

func (s *LinkSnapshotSink) Close() error {
	if err := s.stream.Close(); err != nil {
		s.inner.Cancel()
		return err
	}
	return nil
}

func (s *LinkSnapshotSink) ID() string {
	return s.inner.ID()
}

func (s *LinkSnapshotSink) Cancel() error {
	s.stream.Close()
	return s.inner.Cancel()
}

func (s *LinkSnapshotSink) LinkFile(srcRelPath string, dstRelPath string) error {
	src := filepath.Join(s.sourceDir, srcRelPath)
	dst := filepath.Join(s.snapDir, dstRelPath)

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return os.Link(src, dst)
}

func (s *LinkSnapshotSink) WriteManifest(p []byte) (int, error) {
	return s.Write(p)
}
