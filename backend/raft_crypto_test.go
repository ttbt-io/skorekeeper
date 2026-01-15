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
	"os"
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
	tempDir, _ := os.MkdirTemp("", "logkey_rotation_test")
	defer os.RemoveAll(tempDir)

	mk, _ := crypto.CreateAESMasterKeyForTest()

	rm := &RaftManager{
		DataDir:   tempDir,
		MasterKey: mk,
	}

	if err := rm.loadOrGenerateLogKey(); err != nil {
		t.Fatalf("Failed to load log key: %v", err)
	}

	inner := &mockLogStore{logs: make(map[uint64]*raft.Log)}
	logStore := NewEncryptedLogStore(inner, rm.logKey)
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

	// Now log 1 should be UNREADABLE (too old)
	// (Actually, our fallback only goes back one level)
	if err := logStore.GetLog(1, &out1); err == nil {
		t.Error("Log 1 should be unreadable after two rotations")
	}

	// Log 2 should still be readable
	if err := logStore.GetLog(2, &out2); err != nil {
		t.Errorf("Log 2 should be readable: %v", err)
	}
}

func TestEncryptedLogStoreExtra(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "log_extra")
	defer os.RemoveAll(tempDir)

	mk, _ := crypto.CreateAESMasterKeyForTest()
	rm := &RaftManager{MasterKey: mk, DataDir: tempDir}
	rm.loadOrGenerateLogKey()

	inner := &mockLogStore{logs: make(map[uint64]*raft.Log)}
	logStore := NewEncryptedLogStore(inner, rm.logKey)

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
	tempDir, _ := os.MkdirTemp("", "logkey_persistence_test")
	defer os.RemoveAll(tempDir)

	mk, _ := crypto.CreateAESMasterKeyForTest()

	rm := &RaftManager{
		DataDir:   tempDir,
		MasterKey: mk,
	}

	if err := rm.loadOrGenerateLogKey(); err != nil {
		t.Fatalf("Failed to load log key: %v", err)
	}
	key1 := rm.logKey

	// Rotate
	if err := rm.RotateLogKey(); err != nil {
		t.Fatalf("Failed to rotate: %v", err)
	}
	key2 := rm.logKey

	// Restart RaftManager
	rm2 := &RaftManager{
		DataDir:   tempDir,
		MasterKey: mk,
	}
	if err := rm2.loadOrGenerateLogKey(); err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	// Verify keys match
	// (EncryptionKey doesn't have equality check, but we can try to decrypt something)
	data, _ := key2.Encrypt([]byte("test"))
	if _, err := rm2.logKey.Decrypt(data); err != nil {
		t.Error("Current key not persisted correctly")
	}

	dataOld, _ := key1.Encrypt([]byte("old test"))
	if _, err := rm2.prevLogKey.Decrypt(dataOld); err != nil {
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

	inner := &mockStableStore{data: make(map[string][]byte)}
	store := NewEncryptedStableStore(inner, key)

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
