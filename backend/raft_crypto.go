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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

// EncryptedLogStore wraps a raft.LogStore to encrypt log entries.
type EncryptedLogStore struct {
	inner raft.LogStore
	mu    sync.RWMutex
	key   crypto.EncryptionKey
	prev  crypto.EncryptionKey
}

// NewEncryptedLogStore creates a new encrypted log store.
func NewEncryptedLogStore(inner raft.LogStore, key crypto.EncryptionKey) *EncryptedLogStore {
	return &EncryptedLogStore{
		inner: inner,
		key:   key,
	}
}

func (e *EncryptedLogStore) SetKeys(key, prev crypto.EncryptionKey) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.key = key
	e.prev = prev
}

func (e *EncryptedLogStore) FirstIndex() (uint64, error) {
	return e.inner.FirstIndex()
}

func (e *EncryptedLogStore) LastIndex() (uint64, error) {
	return e.inner.LastIndex()
}

func (e *EncryptedLogStore) GetLog(index uint64, log *raft.Log) error {
	if err := e.inner.GetLog(index, log); err != nil {
		return err
	}
	if len(log.Data) == 0 {
		return nil
	}
	e.mu.RLock()
	key := e.key
	prev := e.prev
	e.mu.RUnlock()

	if key != nil {
		decrypted, err := key.Decrypt(log.Data)
		if err == nil {
			log.Data = decrypted
			return nil
		}
		if prev != nil && errors.Is(err, crypto.ErrDecryptFailed) {
			if dec, err := prev.Decrypt(log.Data); err == nil {
				log.Data = dec
				return nil
			}
		}
		return fmt.Errorf("failed to decrypt log index %d: %w", index, err)
	}
	return nil
}

func (e *EncryptedLogStore) StoreLog(log *raft.Log) error {
	e.mu.RLock()
	key := e.key
	e.mu.RUnlock()

	if key != nil && len(log.Data) > 0 {
		encrypted, err := key.Encrypt(log.Data)
		if err != nil {
			return fmt.Errorf("failed to encrypt log: %w", err)
		}
		newLog := *log
		newLog.Data = encrypted
		return e.inner.StoreLog(&newLog)
	}
	return e.inner.StoreLog(log)
}

func (e *EncryptedLogStore) StoreLogs(logs []*raft.Log) error {
	e.mu.RLock()
	key := e.key
	e.mu.RUnlock()

	if key == nil {
		return e.inner.StoreLogs(logs)
	}

	newLogs := make([]*raft.Log, len(logs))
	for i, l := range logs {
		if len(l.Data) > 0 {
			encrypted, err := key.Encrypt(l.Data)
			if err != nil {
				return fmt.Errorf("failed to encrypt log batch index %d: %w", i, err)
			}
			nl := *l
			nl.Data = encrypted
			newLogs[i] = &nl
		} else {
			newLogs[i] = l
		}
	}
	return e.inner.StoreLogs(newLogs)
}

func (e *EncryptedLogStore) DeleteRange(min, max uint64) error {
	return e.inner.DeleteRange(min, max)
}

func (e *EncryptedLogStore) Close() error {
	if c, ok := e.inner.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// EncryptedStableStore wraps a raft.StableStore to encrypt key-values.
type EncryptedStableStore struct {
	inner raft.StableStore
	mu    sync.RWMutex
	key   crypto.EncryptionKey
	prev  crypto.EncryptionKey
}

// NewEncryptedStableStore creates a new encrypted stable store.
func NewEncryptedStableStore(inner raft.StableStore, key crypto.EncryptionKey) *EncryptedStableStore {
	return &EncryptedStableStore{
		inner: inner,
		key:   key,
	}
}

func (e *EncryptedStableStore) SetKeys(key, prev crypto.EncryptionKey) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.key = key
	e.prev = prev
}

func (e *EncryptedStableStore) Set(key []byte, val []byte) error {
	e.mu.RLock()
	ekey := e.key
	e.mu.RUnlock()

	if ekey != nil {
		encrypted, err := ekey.Encrypt(val)
		if err != nil {
			return fmt.Errorf("failed to encrypt stable set: %w", err)
		}
		val = encrypted
	}
	return e.inner.Set(key, val)
}

func (e *EncryptedStableStore) Get(key []byte) ([]byte, error) {
	val, err := e.inner.Get(key)
	if err != nil {
		return nil, err
	}
	if len(val) == 0 {
		return val, nil
	}
	e.mu.RLock()
	ekey := e.key
	eprev := e.prev
	e.mu.RUnlock()

	if ekey != nil {
		decrypted, err := ekey.Decrypt(val)
		if err == nil {
			return decrypted, nil
		}
		if eprev != nil && errors.Is(err, crypto.ErrDecryptFailed) {
			if dec, err := eprev.Decrypt(val); err == nil {
				return dec, nil
			}
		}
		return nil, fmt.Errorf("failed to decrypt stable get: %w", err)
	}
	return val, nil
}

func (e *EncryptedStableStore) SetUint64(key []byte, val uint64) error {
	// We must store as encrypted bytes.
	// The inner store's SetUint64 might not support arbitrary length or encryption.
	// We simply rely on Set() to store the 8 bytes encrypted.
	// But wait, Raft might call GetUint64 later.
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, val)
	return e.Set(key, b)
}

func (e *EncryptedStableStore) GetUint64(key []byte) (uint64, error) {
	val, err := e.Get(key) // This calls our Get(), which decrypts
	if err != nil {
		return 0, err
	}
	if len(val) == 0 {
		return 0, fmt.Errorf("not found")
	}
	if len(val) != 8 {
		return 0, fmt.Errorf("unexpected value length: %d", len(val))
	}
	return binary.BigEndian.Uint64(val), nil
}

func (e *EncryptedStableStore) Close() error {
	if c, ok := e.inner.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
