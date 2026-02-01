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
	"fmt"
	"io"

	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

const snapshotCryptoCtx = "raft-snapshot"

// EncryptedSnapshotStore wraps a raft.SnapshotStore to encrypt snapshots on disk
// but serve them as plaintext (decrypted) via Open() for streaming/restore.
type EncryptedSnapshotStore struct {
	inner raft.SnapshotStore
	ring  *KeyRing
}

func NewEncryptedSnapshotStore(inner raft.SnapshotStore, ring *KeyRing) *EncryptedSnapshotStore {
	return &EncryptedSnapshotStore{
		inner: inner,
		ring:  ring,
	}
}

func (e *EncryptedSnapshotStore) SetKeyRing(ring *KeyRing) {
	e.ring = ring
}

func (e *EncryptedSnapshotStore) Create(version raft.SnapshotVersion, index, term uint64, configuration raft.Configuration, snapshotSize uint64, trans raft.Transport) (raft.SnapshotSink, error) {
	sink, err := e.inner.Create(version, index, term, configuration, snapshotSize, trans)
	if err != nil {
		return nil, err
	}

	if e.ring == nil || e.ring.Active == nil {
		return sink, nil
	}

	// Use active key for writing new snapshot
	encryptedWriter, err := e.ring.Active.Key.StartWriter([]byte(snapshotCryptoCtx), sink)
	if err != nil {
		sink.Cancel()
		return nil, err
	}

	return &EncryptedSnapshotSink{
		inner:  sink,
		stream: encryptedWriter,
	}, nil
}

func (e *EncryptedSnapshotStore) List() ([]*raft.SnapshotMeta, error) {
	return e.inner.List()
}

// GetSnapshotKeyID attempts to identify which key ID decrypts the snapshot.
// It returns the ID of the key, or empty string if no key works or store is unencrypted.
func (e *EncryptedSnapshotStore) GetSnapshotKeyID(id string) (string, error) {
	if e.ring == nil {
		return "", nil
	}

	// We need to try keys just like Open, but return the ID.
	keys := append([]*KeyInfo{e.ring.Active}, e.ring.Old...)

	for _, info := range keys {
		if info == nil {
			continue
		}

		// Open logic: Re-open the snapshot stream for each key attempt.
		_, rc, err := e.inner.Open(id)
		if err != nil {
			return "", err
		}

		// Try to start reader (verifies header/tag)
		_, err = info.Key.StartReader([]byte(snapshotCryptoCtx), rc)
		rc.Close() // Close immediately

		if err == nil {
			return info.ID, nil
		}
	}

	return "", fmt.Errorf("no key found for snapshot %s", id)
}

func (e *EncryptedSnapshotStore) Open(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	if e.ring == nil {
		return e.inner.Open(id)
	}

	keys := append([]*KeyInfo{e.ring.Active}, e.ring.Old...)
	var lastErr error

	for _, info := range keys {
		if info == nil {
			continue
		}

		// Re-open the snapshot stream for each key attempt.
		// This is necessary because standard raft.FileSnapshotStore returns a non-seekable *raft.bufferedFile,
		// so we cannot rewind after a failed decryption attempt.
		meta, rc, err := e.inner.Open(id)
		if err != nil {
			return nil, nil, err
		}

		decryptedReader, err := info.Key.StartReader([]byte(snapshotCryptoCtx), rc)
		if err == nil {
			return meta, &DecryptedReadCloser{
				inner:  rc,
				stream: decryptedReader,
			}, nil
		}

		// Failed with this key, close and try next
		rc.Close()
		lastErr = err
	}

	return nil, nil, fmt.Errorf("failed to open snapshot with any key: %w", lastErr)
}

// EncryptedSnapshotSink wraps the underlying sink.
type EncryptedSnapshotSink struct {
	inner  raft.SnapshotSink
	stream crypto.StreamWriter // io.Writer + io.Closer
}

func (s *EncryptedSnapshotSink) Write(p []byte) (n int, err error) {
	return s.stream.Write(p)
}

func (s *EncryptedSnapshotSink) Close() error {
	// Close encryption stream first to flush tag
	if err := s.stream.Close(); err != nil {
		s.inner.Cancel()
		return err
	}
	// Then close underlying sink to commit file
	return s.inner.Close()
}

func (s *EncryptedSnapshotSink) ID() string {
	return s.inner.ID()
}

func (s *EncryptedSnapshotSink) Cancel() error {
	// We don't strictly need to close stream on cancel, but good practice?
	s.stream.Close()
	return s.inner.Cancel()
}

// DecryptedReadCloser wraps the underlying reader.
type DecryptedReadCloser struct {
	inner  io.ReadCloser
	stream crypto.StreamReader // io.Reader + io.Seeker + io.Closer
}

func (r *DecryptedReadCloser) Read(p []byte) (n int, err error) {
	return r.stream.Read(p)
}

func (r *DecryptedReadCloser) Close() error {
	// Close stream first
	r.stream.Close()
	// Then close underlying file
	return r.inner.Close()
}
