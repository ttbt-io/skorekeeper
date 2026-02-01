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

// KeyInfo wraps an encryption key with its identifier (filename).
type KeyInfo struct {
	Key crypto.EncryptionKey
	ID  string
}

// KeyRing manages a collection of encryption keys.
type KeyRing struct {
	mu     sync.RWMutex
	Active *KeyInfo
	Old    []*KeyInfo
}

// NewKeyRing creates a new KeyRing with an initial active key.
func NewKeyRing(active crypto.EncryptionKey, id string) *KeyRing {
	return &KeyRing{
		Active: &KeyInfo{Key: active, ID: id},
		Old:    make([]*KeyInfo, 0),
	}
}

// SetKeys sets the key ring keys.
func (k *KeyRing) SetKeys(active *KeyInfo, old []*KeyInfo) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.Active = active
	k.Old = old
}

// Rotate adds a new active key and moves the current active key to the old list.
func (k *KeyRing) Rotate(newKey crypto.EncryptionKey, id string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.Active != nil {
		// Prepend to keep newest first
		k.Old = append([]*KeyInfo{k.Active}, k.Old...)
	}
	k.Active = &KeyInfo{Key: newKey, ID: id}
}

// Wipe wipes all keys in the ring from memory.
func (k *KeyRing) Wipe() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.Active != nil {
		k.Active.Key.Wipe()
		k.Active = nil
	}
	for _, info := range k.Old {
		if info != nil {
			info.Key.Wipe()
		}
	}
	k.Old = nil
}

// Encrypt encrypts data using the active key.
func (k *KeyRing) Encrypt(data []byte) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.Active == nil {
		return nil, fmt.Errorf("no active key")
	}
	return k.Active.Key.Encrypt(data)
}

// Decrypt tries to decrypt data using the active key, then old keys.
func (k *KeyRing) Decrypt(data []byte) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Try active key first
	if k.Active != nil {
		dec, err := k.Active.Key.Decrypt(data)
		if err == nil {
			return dec, nil
		}
		if !errors.Is(err, crypto.ErrDecryptFailed) {
			return nil, err
		}
	}

	// Try old keys
	for _, info := range k.Old {
		dec, err := info.Key.Decrypt(data)
		if err == nil {
			return dec, nil
		}
		if !errors.Is(err, crypto.ErrDecryptFailed) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("failed to decrypt with any key: %w", crypto.ErrDecryptFailed)
}

// EncryptedLogStore wraps a raft.LogStore to encrypt log entries.
type EncryptedLogStore struct {
	inner raft.LogStore
	ring  *KeyRing
}

// NewEncryptedLogStore creates a new encrypted log store.
func NewEncryptedLogStore(inner raft.LogStore, ring *KeyRing) *EncryptedLogStore {
	return &EncryptedLogStore{
		inner: inner,
		ring:  ring,
	}
}

// SetKeyRing updates the key ring reference (if ring pointer changes, though typically we just update inside ring).
func (e *EncryptedLogStore) SetKeyRing(ring *KeyRing) {
	e.ring = ring
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

	if e.ring != nil {
		decrypted, err := e.ring.Decrypt(log.Data)
		if err == nil {
			log.Data = decrypted
			return nil
		}
		return fmt.Errorf("failed to decrypt log index %d: %w", index, err)
	}
	return nil
}

func (e *EncryptedLogStore) StoreLog(log *raft.Log) error {
	if e.ring != nil && len(log.Data) > 0 {
		encrypted, err := e.ring.Encrypt(log.Data)
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
	if e.ring == nil {
		return e.inner.StoreLogs(logs)
	}

	newLogs := make([]*raft.Log, len(logs))
	for i, l := range logs {
		if len(l.Data) > 0 {
			encrypted, err := e.ring.Encrypt(l.Data)
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
	ring  *KeyRing
}

// NewEncryptedStableStore creates a new encrypted stable store.
func NewEncryptedStableStore(inner raft.StableStore, ring *KeyRing) *EncryptedStableStore {
	return &EncryptedStableStore{
		inner: inner,
		ring:  ring,
	}
}

func (e *EncryptedStableStore) SetKeyRing(ring *KeyRing) {
	e.ring = ring
}

func (e *EncryptedStableStore) Set(key []byte, val []byte) error {
	if e.ring != nil {
		encrypted, err := e.ring.Encrypt(val)
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

	if e.ring != nil {
		decrypted, err := e.ring.Decrypt(val)
		if err == nil {
			return decrypted, nil
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
