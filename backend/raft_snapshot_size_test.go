package backend

import (
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
)

func TestLinkSnapshotStore_Replication_SizeCorrectness(t *testing.T) {
	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	mk, _ := crypto.CreateAESMasterKeyForTest()

	s := storage.New(dataDir, mk)
	
	// 1. Create dummy data file
	dummyGame := &Game{
		ID: "game-1",
		Away: "Away Team",
		Home: "Home Team",
		// Add padding to make it large
		ActionLog: make([]json.RawMessage, 100),
	}
	// Fill with dummy data
	for i := 0; i < 100; i++ {
		dummyGame.ActionLog[i] = json.RawMessage(`{"type":"PITCH"}`)
	}

	if err := s.SaveDataFile("games/game-1.json", dummyGame); err != nil {
		t.Fatalf("SaveDataFile failed: %v", err)
	}

	// 2. Setup LinkSnapshotStore
	innerStore, err := raft.NewFileSnapshotStore(raftDir, 1, io.Discard)
	if err != nil {
		t.Fatalf("Failed to create file snapshot store: %v", err)
	}
	linkStore := NewLinkSnapshotStore(raftDir, dataDir, innerStore, nil, mk)

	// 3. Create Snapshot
	sink, err := linkStore.Create(1, 10, 1, raft.Configuration{}, 1, nil)
	if err != nil {
		t.Fatalf("Create sink failed: %v", err)
	}

	linker := sink.(SnapshotLinker)
	linker.LinkFile("games/game-1.json", "games/game-1.json")
	
	manifest := snapshotManifest{
		RaftIndex: 10,
	}
	manifestBytes, _ := json.Marshal(manifest)
	linker.WriteManifest(manifestBytes)
	
	if err := sink.Close(); err != nil {
		t.Fatalf("Sink close failed: %v", err)
	}
	snapID := sink.ID()

	// 4. Open Snapshot
	meta, rc, err := linkStore.Open(snapID)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer rc.Close()

	// 5. Read all data
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	realSize := int64(len(data))
	
	t.Logf("Meta Size: %d", meta.Size)
	t.Logf("Real Stream Size: %d", realSize)

	if meta.Size != realSize {
		t.Errorf("Size mismatch: Meta says %d, Stream is %d", meta.Size, realSize)
	} else {
		t.Logf("Sizes match: %d", meta.Size)
	}
}
