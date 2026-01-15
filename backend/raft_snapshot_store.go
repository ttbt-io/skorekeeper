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
	"io"

	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

const snapshotCryptoCtx = "raft-snapshot"

// EncryptedSnapshotStore wraps a raft.SnapshotStore to encrypt snapshots on disk
// but serve them as plaintext (decrypted) via Open() for streaming/restore.
type EncryptedSnapshotStore struct {
	inner raft.SnapshotStore
	key   crypto.EncryptionKey
}

func NewEncryptedSnapshotStore(inner raft.SnapshotStore, key crypto.EncryptionKey) *EncryptedSnapshotStore {
	return &EncryptedSnapshotStore{
		inner: inner,
		key:   key,
	}
}

func (e *EncryptedSnapshotStore) Create(version raft.SnapshotVersion, index, term uint64, configuration raft.Configuration, snapshotSize uint64, trans raft.Transport) (raft.SnapshotSink, error) {
	sink, err := e.inner.Create(version, index, term, configuration, snapshotSize, trans)
	if err != nil {
		return nil, err
	}

	if e.key == nil {
		return sink, nil
	}

	// Create encryption stream wrapper
	// We wrap the sink. Writing to 'encryptedWriter' encrypts and writes to 'sink'.
	encryptedWriter, err := e.key.StartWriter([]byte(snapshotCryptoCtx), sink)
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

func (e *EncryptedSnapshotStore) Open(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	meta, rc, err := e.inner.Open(id)
	if err != nil {
		return nil, nil, err
	}

	if e.key == nil {
		return meta, rc, nil
	}

	// Create decryption stream wrapper
	// Reading from 'decryptedReader' reads from 'rc' and decrypts.
	decryptedReader, err := e.key.StartReader([]byte(snapshotCryptoCtx), rc)
	if err != nil {
		rc.Close()
		return nil, nil, err
	}

	return meta, &DecryptedReadCloser{
		inner:  rc,
		stream: decryptedReader,
	}, nil
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
