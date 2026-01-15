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
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

func TestEncryptedSnapshotStore(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "snapshot_test")
	defer os.RemoveAll(tempDir)

	mk, _ := crypto.CreateAESMasterKeyForTest()
	key, _ := mk.NewKey()

	inner, err := raft.NewFileSnapshotStore(tempDir, 1, nil)
	if err != nil {
		t.Fatalf("NewFileSnapshotStore failed: %v", err)
	}
	store := NewEncryptedSnapshotStore(inner, key)

	// 1. Create a snapshot
	data := []byte("snapshot data")
	sink, err := store.Create(raft.SnapshotVersion(1), 1, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := sink.Write(data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 2. List snapshots
	metas, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("Expected 1 snapshot, got %d", len(metas))
	}
	id := metas[0].ID

	// 3. Open and Read
	_, rc, err := store.Open(id)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()

	readData, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(readData, data) {
		t.Errorf("Expected %s, got %s", string(data), string(readData))
	}
}
