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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

const snapshotCryptoCtx = "raft-snapshot"

// DecryptedReadCloser wraps the underlying reader.
type DecryptedReadCloser struct {
	inner  io.ReadCloser
	stream crypto.StreamReader // io.Reader + io.Seeker + io.Closer
}

func (r *DecryptedReadCloser) Read(p []byte) (n int, err error) {
	return r.stream.Read(p)
}

func (r *DecryptedReadCloser) Seek(offset int64, whence int) (int64, error) {
	return r.stream.Seek(offset, whence)
}

func (r *DecryptedReadCloser) Close() error {
	// Close stream first
	r.stream.Close()
	// Then close underlying file
	return r.inner.Close()
}

type bufReadCloser struct {
	*bufio.Reader
	io.Closer
}

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

	var stream crypto.StreamWriter
	if s.ring != nil && s.ring.Active != nil {
		stream, err = s.ring.Active.Key.StartWriter([]byte(snapshotCryptoCtx), sink)
		if err != nil {
			sink.Cancel()
			return nil, err
		}
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
	}

	// 2. Peek to detect if this is a Remote Snapshot (GZIP TAR) or Local Snapshot (JSON Manifest)
	br := bufio.NewReader(decryptedRC)
	header, err := br.Peek(2)
	if err == nil && len(header) == 2 && header[0] == 0x1f && header[1] == 0x8b {
		// It's a GZIP stream (Remote Snapshot).
		// Return the buffered reader wrapped to preserve Close.
		return meta, &bufReadCloser{Reader: br, Closer: decryptedRC}, nil
	}

	// 3. If Local Snapshot (JSON), we must "hydrate" it into a TAR stream.
	// To ensure meta.Size matches the stream exactly, we generate the full GZipped TAR
	// into a temporary encrypted cache file, count its size, and then stream from there.

	// Read manifest bytes eagerly
	manifestBytes, err := io.ReadAll(br)
	if err != nil {
		decryptedRC.Close()
		return nil, nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	decryptedRC.Close() // Done with the original partial reader

	snapDir, err := s.resolveSnapshotPath(id)
	if err != nil {
		return nil, nil, err
	}

	cachePath := filepath.Join(snapDir, "replication.cache")
	sizePath := filepath.Join(snapDir, "replication.size")

	// Check if cache exists
	if _, err := os.Stat(cachePath); err == nil {
		if sizeBytes, err := os.ReadFile(sizePath); err == nil {
			var size int64
			if _, err := fmt.Sscanf(string(sizeBytes), "%d", &size); err == nil {
				// Cache hit
				f, err := os.Open(cachePath)
				if err != nil {
					return nil, nil, err
				}

				// Setup Decryption Reader
				var decReader crypto.StreamReader
				if s.ring != nil && s.ring.Active != nil {
					decReader, err = s.ring.Active.Key.StartReader([]byte(snapshotCryptoCtx), f)
				} else if s.masterKey != nil {
					decReader, err = s.masterKey.StartReader([]byte(snapshotCryptoCtx), f)
				} else {
					decReader = &nopStreamReader{f}
				}

				if err == nil {
					meta.Size = size
					return meta, &DecryptedReadCloser{inner: f, stream: decReader}, nil
				}
				f.Close()
			}
		}
	}

	// Regenerate Cache
	f, err := os.Create(cachePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create replication cache: %w", err)
	}

	// Setup Encryption Writer
	var streamW crypto.StreamWriter
	if s.ring != nil && s.ring.Active != nil {
		streamW, err = s.ring.Active.Key.StartWriter([]byte(snapshotCryptoCtx), writerOnly{f})
	} else if s.masterKey != nil {
		streamW, err = s.masterKey.StartWriter([]byte(snapshotCryptoCtx), writerOnly{f})
	} else {
		streamW = &nopStreamWriter{f}
	}
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("failed to start encryption writer: %w", err)
	}

	// Counter to track Decrypted Size (GZIP stream size)
	counter := &byteCounter{}

	// Pipeline: Content -> Tar -> Gzip -> Counter -> Encrypt -> File
	// Note: We want to count what goes INTO Encrypt, which is what comes OUT of Gzip.
	// So Writer is: Gzip -> MultiWriter(Counter, StreamW) -> File?
	// No, io.MultiWriter writes to both.

	// We want:
	// content -> tar -> gzip -> [Counter] -> streamW -> file
	// So gzip writes to Counter. Counter writes to streamW.

	counterW := &counterWriter{target: streamW, counter: counter}
	gz := gzip.NewWriter(counterW)
	tw := tar.NewWriter(gz)

	// Write Manifest
	if err := writeFileToTar(tw, "manifest.json", manifestBytes); err != nil {
		tw.Close()
		gz.Close()
		streamW.Close()
		f.Close()
		return nil, nil, err
	}

	// Write Entities
	err = s.walkSnapshotEntities(id, func(path string, data []byte) error {
		return writeFileToTar(tw, path, data)
	})
	if err != nil {
		tw.Close()
		gz.Close()
		streamW.Close()
		f.Close()
		return nil, nil, err
	}

	// Close Writers (Order matters!)
	if err := tw.Close(); err != nil {
		return nil, nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, nil, err
	}
	if err := streamW.Close(); err != nil {
		return nil, nil, err
	}
	if err := f.Close(); err != nil {
		return nil, nil, err
	}

	// Save Size
	size := counter.count
	if err := os.WriteFile(sizePath, []byte(fmt.Sprintf("%d", size)), 0644); err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("failed to write snapshot size cache: %w", err)
	}

	// Re-open for reading
	f, err = os.Open(cachePath)
	if err != nil {
		return nil, nil, err
	}

	var decReader crypto.StreamReader
	if s.ring != nil && s.ring.Active != nil {
		decReader, err = s.ring.Active.Key.StartReader([]byte(snapshotCryptoCtx), f)
	} else if s.masterKey != nil {
		decReader, err = s.masterKey.StartReader([]byte(snapshotCryptoCtx), f)
	} else {
		decReader = &nopStreamReader{f}
	}

	if err != nil {
		f.Close()
		return nil, nil, err
	}

	meta.Size = size
	return meta, &DecryptedReadCloser{inner: f, stream: decReader}, nil
}

type byteCounter struct {
	count int64
}

type counterWriter struct {
	target  io.Writer
	counter *byteCounter
}

func (c *counterWriter) Write(p []byte) (int, error) {
	n, err := c.target.Write(p)
	// Count bytes successfully written
	if n > 0 {
		c.counter.count += int64(n)
	}
	return n, err
}

type nopStreamReader struct {
	*os.File
}

func (n *nopStreamReader) Close() error { return nil }

type nopStreamWriter struct {
	io.Writer
}

func (n *nopStreamWriter) Close() error { return nil }

type writerOnly struct {
	io.Writer
}

func (s *LinkSnapshotStore) walkSnapshotEntities(id string, visitor func(relPath string, data []byte) error) error {
	snapDir, err := s.resolveSnapshotPath(id)
	if err != nil {
		return err
	}

	tempStore := storage.New(snapDir, s.masterKey)

	return filepath.Walk(snapDir, func(path string, info os.FileInfo, err error) error {
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

		if relPath == "meta.json" || relPath == "state.bin" || relPath == "replication.cache" || relPath == "replication.size" {
			return nil
		}

		if relPath == "sys_access_policy" {
			obj := &UserAccessPolicy{}
			if err := tempStore.ReadDataFile(relPath, obj); err != nil {
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

		if relPath == "metrics.json" {
			obj := &MetricsStore{}
			if err := tempStore.ReadDataFile(relPath, obj); err != nil {
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

		if relPath == "nodes.json" {
			var obj map[string]*NodeMeta
			if err := tempStore.ReadDataFile(relPath, &obj); err != nil {
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

		if relPath == "fsm_state.json" {
			var obj map[string]any
			if err := tempStore.ReadDataFile(relPath, &obj); err != nil {
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

		handlers := []struct {
			prefix  string
			factory func() any
		}{
			{"games/", func() any { return &Game{} }},
			{"teams/", func() any { return &Team{} }},
			{"users/", func() any { return &UserIndex{} }},
			{"team_games/", func() any { return &TeamGamesIndex{} }},
			{"game_users/", func() any { return &GameUsersIndex{} }},
			{"team_users/", func() any { return &TeamUsersIndex{} }},
		}

		for _, h := range handlers {
			if strings.HasPrefix(relPath, h.prefix) {
				obj := h.factory()
				if err := tempStore.ReadDataFile(relPath, obj); err != nil {
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
		}
		return nil
	})
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
	inner     raft.SnapshotSink
	snapDir   string
	sourceDir string
	stream    crypto.StreamWriter
}

func (s *LinkSnapshotSink) Write(p []byte) (n int, err error) {
	if s.stream != nil {
		return s.stream.Write(p)
	}
	return s.inner.Write(p)
}

func (s *LinkSnapshotSink) Close() error {
	if s.stream != nil {
		if err := s.stream.Close(); err != nil {
			s.inner.Cancel()
			return err
		}
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
