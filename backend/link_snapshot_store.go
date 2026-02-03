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
	baseDir   string
	inner     *raft.FileSnapshotStore
	ring      *KeyRing
	masterKey crypto.MasterKey
}

// NewLinkSnapshotStore creates a new LinkSnapshotStore.
func NewLinkSnapshotStore(baseDir string, inner *raft.FileSnapshotStore, ring *KeyRing, masterKey crypto.MasterKey) *LinkSnapshotStore {
	return &LinkSnapshotStore{
		baseDir:   baseDir,
		inner:     inner,
		ring:      ring,
		masterKey: masterKey,
	}
}

func (s *LinkSnapshotStore) Create(version raft.SnapshotVersion, index, term uint64, configuration raft.Configuration, snapshotSize uint64, trans raft.Transport) (raft.SnapshotSink, error) {
	sink, err := s.inner.Create(version, index, term, configuration, snapshotSize, trans)
	if err != nil {
		return nil, err
	}

	// FileSnapshotStore typically puts snapshots in baseDir/snapshots/ID.tmp during creation
	snapDir := filepath.Join(s.baseDir, "snapshots", sink.ID())
	if _, err := os.Stat(snapDir); os.IsNotExist(err) {
		snapDir += ".tmp"
	}

	if _, err := os.Stat(snapDir); os.IsNotExist(err) {
		// Try without "snapshots" subdir as well (depending on raft version/config)
		altDir := filepath.Join(s.baseDir, sink.ID())
		if _, err := os.Stat(altDir); os.IsNotExist(err) {
			altDir += ".tmp"
		}
		if _, err := os.Stat(altDir); err == nil {
			snapDir = altDir
		} else {
			sink.Cancel()
			return nil, fmt.Errorf("snapshot directory not found for ID %s in %s", sink.ID(), s.baseDir)
		}
	}

	var stream crypto.StreamWriter
	if s.ring != nil && s.ring.Active != nil {
		stream, err = s.ring.Active.Key.StartWriter([]byte(snapshotCryptoCtx), sink)
		if err != nil {
			sink.Cancel()
			return nil, err
		}
	}

	return &LinkSnapshotSink{
		inner:   sink,
		snapDir: snapDir,
		dataDir: s.baseDir,
		stream:  stream,
	}, nil
}

func (s *LinkSnapshotStore) List() ([]*raft.SnapshotMeta, error) {
	return s.inner.List()
}

// GetSnapshotKeyID attempts to identify which key ID decrypts the snapshot.
func (s *LinkSnapshotStore) GetSnapshotKeyID(id string) (string, error) {
	if s.ring == nil {
		return "", nil
	}
	s.ring.mu.RLock()
	keys := make([]*KeyInfo, 0, 1+len(s.ring.Old))
	if s.ring.Active != nil {
		keys = append(keys, s.ring.Active)
	}
	keys = append(keys, s.ring.Old...)
	s.ring.mu.RUnlock()

	for _, info := range keys {
		if info == nil {
			continue
		}
		_, rc, err := s.inner.Open(id)
		if err != nil {
			return "", err
		}
		_, err = info.Key.StartReader([]byte(snapshotCryptoCtx), rc)
		rc.Close()
		if err == nil {
			return info.ID, nil
		}
	}
	return "", fmt.Errorf("no key found for snapshot %s", id)
}

func (s *LinkSnapshotStore) Open(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	meta, rc, err := s.inner.Open(id)
	if err != nil {
		return nil, nil, err
	}

	// 1. Decrypt Manifest Stream (state.bin)
	var decryptedRC io.ReadCloser = rc
	if s.ring != nil {
		decryptedRC, err = s.decryptManifestStream(id)
		if err != nil {
			rc.Close()
			return nil, nil, err
		}
		rc.Close()
	}

	// 2. Return a reader that streams the TAR (Manifest + Files)
	pr, pw := io.Pipe()

	go func() {
		defer decryptedRC.Close()
		defer pw.Close()

		gz := gzip.NewWriter(pw)
		defer gz.Close()

		tw := tar.NewWriter(gz)
		defer tw.Close()

		manifestBytes, err := io.ReadAll(decryptedRC)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("failed to read manifest: %w", err))
			return
		}

		if err := writeFileToTar(tw, "manifest.json", manifestBytes); err != nil {
			pw.CloseWithError(err)
			return
		}

		// FileSnapshotStore moves the directory to its final name (without .tmp) once closed.
		// Since we are Opening it, it should be in the final location.
		snapDir := filepath.Join(s.baseDir, "snapshots", id)
		if _, err := os.Stat(snapDir); os.IsNotExist(err) {
			snapDir = filepath.Join(s.baseDir, id)
		}

		tempStore := storage.New(snapDir, s.masterKey)

		filepath.Walk(snapDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relPath, err := filepath.Rel(snapDir, path)
			if err != nil {
				return err
			}

			if relPath == "meta.json" || relPath == "state.bin" {
				return nil
			}

			if strings.HasPrefix(relPath, "games/") {
				var g Game
				if err := tempStore.ReadDataFile(relPath, &g); err != nil {
					log.Printf("Snapshot Open Warning: failed to read game %s: %v", relPath, err)
					return nil
				}
				data, _ := json.Marshal(g)
				writeFileToTar(tw, relPath, data)
			} else if strings.HasPrefix(relPath, "teams/") {
				var t Team
				if err := tempStore.ReadDataFile(relPath, &t); err != nil {
					log.Printf("Snapshot Open Warning: failed to read team %s: %v", relPath, err)
					return nil
				}
				data, _ := json.Marshal(t)
				writeFileToTar(tw, relPath, data)
			} else if strings.HasPrefix(relPath, "users/") {
				var idx UserIndex
				if err := tempStore.ReadDataFile(relPath, &idx); err != nil {
					log.Printf("Snapshot Open Warning: failed to read user index %s: %v", relPath, err)
					return nil
				}
				data, _ := json.Marshal(idx)
				writeFileToTar(tw, relPath, data)
			} else if strings.HasPrefix(relPath, "team_games/") {
				var idx TeamGamesIndex
				if err := tempStore.ReadDataFile(relPath, &idx); err != nil {
					log.Printf("Snapshot Open Warning: failed to read team_games index %s: %v", relPath, err)
					return nil
				}
				data, _ := json.Marshal(idx)
				writeFileToTar(tw, relPath, data)
			} else if strings.HasPrefix(relPath, "game_users/") {
				var idx GameUsersIndex
				if err := tempStore.ReadDataFile(relPath, &idx); err != nil {
					log.Printf("Snapshot Open Warning: failed to read game_users index %s: %v", relPath, err)
					return nil
				}
				data, _ := json.Marshal(idx)
				writeFileToTar(tw, relPath, data)
			} else if strings.HasPrefix(relPath, "team_users/") {
				var idx TeamUsersIndex
				if err := tempStore.ReadDataFile(relPath, &idx); err != nil {
					log.Printf("Snapshot Open Warning: failed to read team_users index %s: %v", relPath, err)
					return nil
				}
				data, _ := json.Marshal(idx)
				writeFileToTar(tw, relPath, data)
			}

			return nil
		})
	}()

	return meta, pr, nil
}

func (s *LinkSnapshotStore) decryptManifestStream(id string) (io.ReadCloser, error) {
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
		_, rc, err := s.inner.Open(id)
		if err != nil {
			return nil, err
		}

		decryptedReader, err := info.Key.StartReader([]byte(snapshotCryptoCtx), rc)
		if err == nil {
			return &DecryptedReadCloser{
				inner:  rc,
				stream: decryptedReader,
			}, nil
		}
		rc.Close()
		lastErr = err
	}
	return nil, fmt.Errorf("failed to open snapshot with any key: %w", lastErr)
}

// LinkSnapshotSink implements raft.SnapshotSink
type LinkSnapshotSink struct {
	inner   raft.SnapshotSink
	snapDir string
	dataDir string
	stream  crypto.StreamWriter
}

func (s *LinkSnapshotSink) Write(p []byte) (n int, err error) {
	if s.stream != nil {
		return s.stream.Write(p)
	}
	return s.inner.Write(p)
}

func (s *LinkSnapshotSink) Close() error {
	if s.stream != nil {
		s.stream.Close()
	}
	return s.inner.Close()
}

func (s *LinkSnapshotSink) ID() string {
	return s.inner.ID()
}

func (s *LinkSnapshotSink) Cancel() error {
	if s.stream != nil {
		s.stream.Close()
	}
	return s.inner.Cancel()
}

func (s *LinkSnapshotSink) LinkFile(srcRelPath string, dstRelPath string) error {
	src := filepath.Join(s.dataDir, srcRelPath)
	dst := filepath.Join(s.snapDir, dstRelPath)

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return os.Link(src, dst)
}

func (s *LinkSnapshotSink) WriteManifest(p []byte) (int, error) {
	return s.Write(p)
}
