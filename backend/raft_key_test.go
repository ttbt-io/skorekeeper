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
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage/crypto"
)

func TestNodeKeyEncryption(t *testing.T) {
	tempDir := t.TempDir()

	passphrase := "test-passphrase"
	keyFile := filepath.Join(tempDir, "master.key")
	mk, err := crypto.CreateMasterKey()
	if err != nil {
		t.Fatalf("Failed to create master key: %v", err)
	}
	if err := mk.Save([]byte(passphrase), keyFile); err != nil {
		t.Fatalf("Failed to save master key: %v", err)
	}

	rm := &RaftManager{
		DataDir:   tempDir,
		NodeID:    "test-node",
		MasterKey: mk,
		FSM:       &FSM{}, // Mock FSM
	}

	// 1. Generate encrypted key
	if err := rm.loadOrGenerateNodeKey(); err != nil {
		t.Fatalf("Failed to generate node key: %v", err)
	}

	nodeKey1 := rm.NodeKey
	keyPath := filepath.Join(tempDir, "node.key")

	// Verify file content is encrypted (not raw 64 bytes)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read node.key: %v", err)
	}
	if len(data) == ed25519.PrivateKeySize {
		t.Errorf("node.key should be encrypted, but size is %d", len(data))
	}

	// 2. Load encrypted key
	rm2 := &RaftManager{
		DataDir:   tempDir,
		NodeID:    "test-node",
		MasterKey: mk,
		FSM:       &FSM{}, // Mock FSM
	}
	if err := rm2.loadOrGenerateNodeKey(); err != nil {
		t.Fatalf("Failed to load node key: %v", err)
	}
	if string(rm2.NodeKey) != string(nodeKey1) {
		t.Error("Loaded node key does not match original")
	}

	// 3. Migration: Start unencrypted, then encrypt
	tempDir2 := t.TempDir()

	rm3 := &RaftManager{

		DataDir: tempDir2,
		NodeID:  "test-node",
		FSM:     &FSM{}, // Mock FSM
	}
	if err := rm3.loadOrGenerateNodeKey(); err != nil {
		t.Fatalf("Failed to generate unencrypted node key: %v", err)
	}
	nodeKey3 := rm3.NodeKey
	keyPath2 := filepath.Join(tempDir2, "node.key")

	// Verify unencrypted
	data2, _ := os.ReadFile(keyPath2)
	if len(data2) != ed25519.PrivateKeySize {
		t.Errorf("node.key should be unencrypted, but size is %d", len(data2))
	}

	// Re-load with MasterKey
	rm4 := &RaftManager{
		DataDir:   tempDir2,
		NodeID:    "test-node",
		MasterKey: mk,
		FSM:       &FSM{}, // Mock FSM
	}
	if err := rm4.loadOrGenerateNodeKey(); err != nil {
		t.Fatalf("Failed to load/migrate node key: %v", err)
	}
	if string(rm4.NodeKey) != string(nodeKey3) {
		t.Error("Migrated node key does not match original")
	}

	// Verify it is now encrypted on disk
	data3, _ := os.ReadFile(keyPath2)
	if len(data3) == ed25519.PrivateKeySize {
		t.Error("node.key should have been migrated to encrypted format")
	}
}
