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
	"testing"

	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

type mockLogStore struct {
	raft.LogStore
	logs map[uint64]*raft.Log
}

func (m *mockLogStore) FirstIndex() (uint64, error) {
	if len(m.logs) == 0 {
		return 0, nil
	}
	var min uint64 = ^uint64(0)
	for i := range m.logs {
		if i < min {
			min = i
		}
	}
	return min, nil
}

func (m *mockLogStore) LastIndex() (uint64, error) {
	if len(m.logs) == 0 {
		return 0, nil
	}
	var max uint64 = 0
	for i := range m.logs {
		if i > max {
			max = i
		}
	}
	return max, nil
}

func (m *mockLogStore) StoreLog(log *raft.Log) error {
	m.logs[log.Index] = log
	return nil
}

func (m *mockLogStore) StoreLogs(logs []*raft.Log) error {
	for _, l := range logs {
		m.StoreLog(l)
	}
	return nil
}

func (m *mockLogStore) GetLog(index uint64, log *raft.Log) error {
	l, ok := m.logs[index]
	if !ok {
		return raft.ErrLogNotFound
	}
	*log = *l
	return nil
}

func (m *mockLogStore) DeleteRange(min, max uint64) error {
	for i := min; i <= max; i++ {
		delete(m.logs, i)
	}
	return nil
}

func TestLogKeyRotation(t *testing.T) {
	tempDir := t.TempDir()

	mk, _ := crypto.CreateAESMasterKeyForTest()

	rm := &RaftManager{
		DataDir:   tempDir,
		MasterKey: mk,
	}

	if err := rm.loadKeyRing(); err != nil {
		t.Fatalf("Failed to load key ring: %v", err)
	}
	defer func() {
		if rm.keyRing != nil {
			rm.keyRing.Wipe()
		}
	}()

	inner := &mockLogStore{logs: make(map[uint64]*raft.Log)}
	logStore := NewEncryptedLogStore(inner, rm.keyRing)
	rm.logStoreEnc = logStore

	// 1. Store log with key 1
	log1 := &raft.Log{Index: 1, Data: []byte("log entry 1")}
	if err := logStore.StoreLog(log1); err != nil {
		t.Fatalf("Failed to store log 1: %v", err)
	}

	// Verify we can read it
	var out1 raft.Log
	if err := logStore.GetLog(1, &out1); err != nil {
		t.Fatalf("Failed to read log 1: %v", err)
	}
	if string(out1.Data) != "log entry 1" {
		t.Errorf("Unexpected log data: %s", string(out1.Data))
	}

	// 2. Rotate key
	if err := rm.RotateLogKey(); err != nil {
		t.Fatalf("Failed to rotate log key: %v", err)
	}

	// Verify we can STILL read log 1 (using prev key)
	var out1b raft.Log
	if err := logStore.GetLog(1, &out1b); err != nil {
		t.Fatalf("Failed to read log 1 after rotation: %v", err)
	}
	if string(out1b.Data) != "log entry 1" {
		t.Errorf("Unexpected log data after rotation: %s", string(out1b.Data))
	}

	// 3. Store log with key 2
	log2 := &raft.Log{Index: 2, Data: []byte("log entry 2")}
	if err := logStore.StoreLog(log2); err != nil {
		t.Fatalf("Failed to store log 2: %v", err)
	}

	// Verify we can read it
	var out2 raft.Log
	if err := logStore.GetLog(2, &out2); err != nil {
		t.Fatalf("Failed to read log 2: %v", err)
	}
	if string(out2.Data) != "log entry 2" {
		t.Errorf("Unexpected log data 2: %s", string(out2.Data))
	}

	// 4. Rotate key again
	if err := rm.RotateLogKey(); err != nil {
		t.Fatalf("Failed to rotate log key again: %v", err)
	}

	// Now log 1 should STILL be readable because we keep ALL keys
	if err := logStore.GetLog(1, &out1); err != nil {
		t.Errorf("Log 1 should still be readable after two rotations: %v", err)
	}

	// Log 2 should still be readable
	if err := logStore.GetLog(2, &out2); err != nil {
		t.Errorf("Log 2 should be readable: %v", err)
	}
}

func TestEncryptedLogStoreExtra(t *testing.T) {
	tempDir := t.TempDir()
	mk, _ := crypto.CreateAESMasterKeyForTest()
	rm := &RaftManager{MasterKey: mk, DataDir: tempDir}
	rm.loadKeyRing()
	defer func() {
		if rm.keyRing != nil {
			rm.keyRing.Wipe()
		}
	}()

	inner := &mockLogStore{logs: make(map[uint64]*raft.Log)}
	logStore := NewEncryptedLogStore(inner, rm.keyRing)

	// 1. StoreLogs
	logs := []*raft.Log{
		{Index: 10, Data: []byte("ten")},
		{Index: 11, Data: []byte("eleven")},
	}
	if err := logStore.StoreLogs(logs); err != nil {
		t.Fatalf("StoreLogs failed: %v", err)
	}

	// 2. FirstIndex / LastIndex
	f, _ := logStore.FirstIndex()
	l, _ := logStore.LastIndex()
	if f != 10 || l != 11 {
		t.Errorf("Expected indices (10, 11), got (%d, %d)", f, l)
	}

	// 3. DeleteRange
	if err := logStore.DeleteRange(10, 10); err != nil {
		t.Fatalf("DeleteRange failed: %v", err)
	}
	f, _ = logStore.FirstIndex()
	if f != 11 {
		t.Errorf("Expected first index 11 after delete, got %d", f)
	}
}

func TestLogKeyPersistence(t *testing.T) {
	tempDir := t.TempDir()

	mk, _ := crypto.CreateAESMasterKeyForTest()

	rm := &RaftManager{
		DataDir:   tempDir,
		MasterKey: mk,
	}

	if err := rm.loadKeyRing(); err != nil {
		t.Fatalf("Failed to load log key: %v", err)
	}
	defer func() {
		if rm.keyRing != nil {
			rm.keyRing.Wipe()
		}
	}()
	key1 := rm.keyRing.Active

	// Rotate
	if err := rm.RotateLogKey(); err != nil {
		t.Fatalf("Failed to rotate: %v", err)
	}
	key2 := rm.keyRing.Active

	// Restart RaftManager
	rm2 := &RaftManager{
		DataDir:   tempDir,
		MasterKey: mk,
	}
	if err := rm2.loadKeyRing(); err != nil {
		t.Fatalf("Failed to load: %v", err)
	}
	defer func() {
		if rm2.keyRing != nil {
			rm2.keyRing.Wipe()
		}
	}()

	// Verify keys match
	// (EncryptionKey doesn't have equality check, but we can try to decrypt something)
	data, _ := key2.Key.Encrypt([]byte("test"))
	if _, err := rm2.keyRing.Decrypt(data); err != nil {
		t.Error("Current key not persisted correctly")
	}

	dataOld, _ := key1.Key.Encrypt([]byte("old test"))
	if _, err := rm2.keyRing.Decrypt(dataOld); err != nil {
		t.Error("Old key not persisted correctly")
	}
}

type mockStableStore struct {
	raft.StableStore
	data map[string][]byte
}

func (m *mockStableStore) Set(key []byte, val []byte) error {
	m.data[string(key)] = val
	return nil
}

func (m *mockStableStore) Get(key []byte) ([]byte, error) {
	val, ok := m.data[string(key)]
	if !ok {
		return nil, nil
	}
	return val, nil
}

func (m *mockStableStore) SetUint64(key []byte, val uint64) error {
	return m.Set(key, []byte{byte(val)}) // Dummy impl for test
}

func (m *mockStableStore) GetUint64(key []byte) (uint64, error) {
	val, err := m.Get(key)
	if err != nil || len(val) == 0 {
		return 0, err
	}
	return uint64(val[0]), nil
}

func TestEncryptedStableStore(t *testing.T) {
	mk, _ := crypto.CreateAESMasterKeyForTest()
	key, _ := mk.NewKey()
	ring := NewKeyRing(key, "test-key")
	defer ring.Wipe()

	inner := &mockStableStore{data: make(map[string][]byte)}
	store := NewEncryptedStableStore(inner, ring)

	// Test Set/Get
	if err := store.Set([]byte("key1"), []byte("val1")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := store.Get([]byte("key1"))
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "val1" {
		t.Errorf("Expected val1, got %s", string(val))
	}

	// Verify inner data is encrypted (not plain "val1")
	innerVal := inner.data["key1"]
	if string(innerVal) == "val1" {
		t.Error("Inner data is not encrypted!")
	}

	// Test Uint64
	if err := store.SetUint64([]byte("u64"), 12345); err != nil {
		t.Fatalf("SetUint64 failed: %v", err)
	}
	uval, err := store.GetUint64([]byte("u64"))
	if err != nil {
		t.Fatalf("GetUint64 failed: %v", err)
	}
	if uval != 12345 {
		t.Errorf("Expected 12345, got %d", uval)
	}
}

func TestStableStoreRotationPersistence(t *testing.T) {
	tempDir := t.TempDir()

	mk, _ := crypto.CreateAESMasterKeyForTest()

	rm := &RaftManager{
		DataDir:   tempDir,
		MasterKey: mk,
	}

	if err := rm.loadKeyRing(); err != nil {
		t.Fatalf("Failed to load log key: %v", err)
	}
	defer func() {
		if rm.keyRing != nil {
			rm.keyRing.Wipe()
		}
	}()

	// 1. Setup EncryptedStableStore
	inner := &mockStableStore{data: make(map[string][]byte)}
	store := NewEncryptedStableStore(inner, rm.keyRing)
	rm.stabStoreEnc = store

	// 2. Write "CurrentTerm" (simulating Raft)
	termKey := []byte("CurrentTerm")
	termVal := uint64(1)

	if err := store.SetUint64(termKey, termVal); err != nil {
		t.Fatalf("Failed to set CurrentTerm: %v", err)
	}

	// Verify we can read it
	v, err := store.GetUint64(termKey)
	if err != nil || v != termVal {
		t.Fatalf("Failed to read initial term: %v, %d", err, v)
	}

	// 3. Rotate Key (1 -> 2)
	if err := rm.RotateLogKey(); err != nil {
		t.Fatalf("Failed to rotate key 1: %v", err)
	}

	// Verify we can STILL read it (should use Prev key)
	v, err = store.GetUint64(termKey)
	if err != nil {
		t.Fatalf("Failed to read term after rotation 1: %v", err)
	}
	if v != termVal {
		t.Errorf("Value mismatch after rotation 1: got %d want %d", v, termVal)
	}

	// 4. Rotate Key (2 -> 3)
	if err := rm.RotateLogKey(); err != nil {
		t.Fatalf("Failed to rotate key 2: %v", err)
	}

	// 5. Verify read SUCCEEDS (because we keep ALL keys)
	v, err = store.GetUint64(termKey)
	if err != nil {
		t.Fatalf("Failed to read term after 2nd rotation (should use retained key): %v", err)
	}
	if v != termVal {
		t.Errorf("Value mismatch after 2nd rotation: got %d want %d", v, termVal)
	}
}
