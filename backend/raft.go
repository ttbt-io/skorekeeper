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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/c2FmZQ/storage/crypto"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

var ErrNotLeader = errors.New("not leader")

type RaftManager struct {
	Raft                  *raft.Raft
	FSM                   *FSM
	DataDir               string
	Bind                  string // "host:port" for Raft transport
	Advertise             string // "host:port" for advertising to other nodes
	ClusterAdvertise      string // "host:port" for advertising to other nodes (secure internal API)
	ClusterAddr           string // "host:port" for internal secure cluster API
	NodeID                string
	Secret                string
	MasterKey             crypto.MasterKey
	NodeKey               ed25519.PrivateKey
	PubKey                ed25519.PublicKey
	Cert                  *tls.Certificate
	Bootstrap             bool
	UseProductionTimeouts bool

	nodeAddrMap sync.Map // map[raft.ServerID]string (ClusterAdvertise Addr)

	shutdownCh     chan struct{}
	shutdownOnce   sync.Once
	readyCh        chan struct{}
	internalServer *http.Server
	httpClient     *http.Client
	AuthMiddleware func(http.Handler) http.Handler
	tofuCallback   func(nodeID string) // For testing

	logStore     raft.LogStore
	stableStore  raft.StableStore
	logStoreEnc  *EncryptedLogStore
	stabStoreEnc *EncryptedStableStore
	snapStoreEnc *LinkSnapshotStore
	keyRing      *KeyRing
	logKeyMu     sync.Mutex

	LogOutput  io.Writer // Optional: Redirect Raft logs
	UseGob     bool      // Optional: Use GOB encoding for log entries
	AppHandler http.Handler
	listener   net.Listener

	countersMu   sync.Mutex
	nodeCounters map[string]uint64
	pendingGapMS int64 // Atomic
	startTime    time.Time

	latencyMu          sync.Mutex
	latencyAccumulator *Histogram
}

func NewRaftManager(dataDir, bind, advertise, clusterAdvertise, clusterAddr, secret string, masterKey crypto.MasterKey, fsm *FSM) *RaftManager {
	// clusterAdvertise is now mandatory and validated in main.go, but for library usage we can fallback or error.
	// Since main.go handles it, we assume it's set. If not, it might break auto-config.
	rm := &RaftManager{
		DataDir:            dataDir,
		Bind:               bind,
		Advertise:          advertise,
		ClusterAdvertise:   clusterAdvertise,
		ClusterAddr:        clusterAddr,
		Secret:             secret,
		MasterKey:          masterKey,
		FSM:                fsm,
		shutdownCh:         make(chan struct{}),
		readyCh:            make(chan struct{}),
		LogOutput:          os.Stderr, // Default
		nodeCounters:       make(map[string]uint64),
		latencyAccumulator: &Histogram{},
	}
	// Note: nodeAddrMap and NodeID derivation will happen in Start() after key loading
	if fsm != nil {
		fsm.rm = rm
	}
	return rm
}

func (rm *RaftManager) loadKeyRing() error {
	if rm.MasterKey == nil {
		return nil
	}

	keysDir := filepath.Join(rm.DataDir, "keys")
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys dir: %v", err)
	}

	// Migration: Move legacy log.key and log.key.old to keysDir
	legacyKey := filepath.Join(rm.DataDir, "log.key")
	legacyOld := filepath.Join(rm.DataDir, "log.key.old")

	if _, err := os.Stat(legacyKey); err == nil {
		// Timestamp roughly now for migration
		ts := time.Now().Add(-1 * time.Minute).UnixNano()
		newName := filepath.Join(keysDir, fmt.Sprintf("%d.key", ts))
		if err := os.Rename(legacyKey, newName); err != nil {
			return fmt.Errorf("failed to migrate log.key: %v", err)
		}
		log.Printf("Migrated legacy log.key to %s", newName)
	}

	if _, err := os.Stat(legacyOld); err == nil {
		// Older timestamp
		ts := time.Now().Add(-2 * time.Minute).UnixNano()
		newName := filepath.Join(keysDir, fmt.Sprintf("%d.key", ts))
		if err := os.Rename(legacyOld, newName); err != nil {
			return fmt.Errorf("failed to migrate log.key.old: %v", err)
		}
		log.Printf("Migrated legacy log.key.old to %s", newName)
	}

	// Load all keys
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return fmt.Errorf("failed to read keys dir: %v", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".key") {
			files = append(files, filepath.Join(keysDir, e.Name()))
		}
	}

	// Ensure consistent sorting by length then content to handle timestamp rollover if needed
	// (Though UnixNano length is fixed for a long time, this is safer)
	sort.Slice(files, func(i, j int) bool {
		if len(files[i]) != len(files[j]) {
			return len(files[i]) < len(files[j])
		}
		return files[i] < files[j]
	})

	var active *KeyInfo
	var old []*KeyInfo

	// Read newest to oldest
	for i := len(files) - 1; i >= 0; i-- {
		path := files[i]
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open key %s: %v", path, err)
		}
		defer f.Close()
		key, err := rm.MasterKey.ReadEncryptedKey(f)
		if err != nil {
			return fmt.Errorf("failed to read key %s: %v", path, err)
		}

		info := &KeyInfo{Key: key, ID: filepath.Base(path)}

		if active == nil {
			active = info
		} else {
			old = append(old, info)
		}
	}

	if active == nil {
		// Generate first key
		log.Printf("Generating initial Raft encryption key...")
		k, err := rm.MasterKey.NewKey()
		if err != nil {
			return fmt.Errorf("failed to generate new key: %v", err)
		}
		ts := time.Now().UnixNano()
		// Initial key uses index 0
		path := filepath.Join(keysDir, fmt.Sprintf("idx-%020d-%d.key", 0, ts))
		if err := rm.saveLogKey(k, path); err != nil {
			return err
		}
		active = &KeyInfo{Key: k, ID: filepath.Base(path)}
	}

	rm.keyRing = &KeyRing{}
	rm.keyRing.SetKeys(active, old)
	return nil
}

func parseKeyIndex(id string) uint64 {
	if !strings.HasPrefix(id, "idx-") {
		return 0
	}
	parts := strings.Split(id, "-")
	if len(parts) < 2 {
		return 0
	}
	var res uint64
	fmt.Sscanf(parts[1], "%d", &res)
	return res
}

func (rm *RaftManager) RotateLogKey() error {
	if rm.MasterKey == nil {
		return nil
	}

	newKey, err := rm.MasterKey.NewKey()
	if err != nil {
		return fmt.Errorf("failed to generate new log key: %v", err)
	}

	keysDir := filepath.Join(rm.DataDir, "keys")
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("failed to ensure keys dir: %v", err)
	}

	var lastIdx uint64
	if rm.logStore != nil {
		if idx, err := rm.logStore.LastIndex(); err == nil {
			lastIdx = idx
		}
	}

	ts := time.Now().UnixNano()
	path := filepath.Join(keysDir, fmt.Sprintf("idx-%020d-%d.key", lastIdx, ts))

	if err := rm.saveLogKey(newKey, path); err != nil {
		return fmt.Errorf("failed to save new log key: %v", err)
	}

	if rm.keyRing != nil {
		rm.keyRing.Rotate(newKey, filepath.Base(path))
	} else {
		rm.keyRing = NewKeyRing(newKey, filepath.Base(path))
	}

	log.Printf("Raft log key rotated successfully. New key: %s", filepath.Base(path))
	return rm.GarbageCollectKeys()
}

func (rm *RaftManager) GarbageCollectKeys() error {
	if rm.keyRing == nil || rm.snapStoreEnc == nil {
		return nil // Encryption disabled
	}

	// 1. Re-encrypt Stable Store (Term & Vote) to ensure they use the active key.
	// This prevents deletion of an old key that is still needed for stable store metadata.
	if rm.stableStore != nil {
		// Known keys used by hashicorp/raft
		knownKeys := [][]byte{
			[]byte("CurrentTerm"),
			[]byte("LastVoteCand"),
			[]byte("LastVoteTerm"),
		}
		for _, key := range knownKeys {
			// Get will try all keys (Active + Old)
			val, err := rm.stableStore.Get(key)
			if err == nil && len(val) > 0 {
				// Set will encrypt with Active key
				if err := rm.stableStore.Set(key, val); err != nil {
					log.Printf("Warning: Failed to re-encrypt stable key %s: %v", string(key), err)
				}
			}
		}
	}

	// 2. Determine the minimum Raft index we need to support.
	// Log entry N is encrypted with the key active at that time, which is
	// the key with the largest idx such that idx < N.
	// Snapshot S is encrypted with the key active at that time, which is
	// the key with the largest idx such that idx <= S (due to rotation in Snapshot()).
	snapshots, err := rm.snapStoreEnc.List()
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %v", err)
	}

	firstIdx, err := rm.logStore.FirstIndex()
	if err != nil {
		return fmt.Errorf("failed to get log first index: %w", err)
	}
	if firstIdx == 0 {
		firstIdx = ^uint64(0)
	}

	// We need to cover log index firstIdx, so we need a key with idx <= firstIdx-1.
	minNeededIdx := firstIdx - 1
	if firstIdx == 0 {
		minNeededIdx = ^uint64(0)
	}

	for _, snap := range snapshots {
		if snap.Index < minNeededIdx {
			minNeededIdx = snap.Index
		}
	}

	rm.keyRing.mu.RLock()
	oldKeys := make([]*KeyInfo, len(rm.keyRing.Old))
	copy(oldKeys, rm.keyRing.Old)
	activeID := rm.keyRing.Active.ID
	rm.keyRing.mu.RUnlock()

	// Find the oldest key in rm.keyRing.Old that we need to keep.
	// Index 0 is newest, Index N is oldest.
	maxUsedIndex := -1

	if parseKeyIndex(activeID) > minNeededIdx {
		// Active key is too new, we need at least some old keys to cover minNeededIdx.
		for i, k := range oldKeys {
			maxUsedIndex = i
			if parseKeyIndex(k.ID) <= minNeededIdx {
				// This key covers the minNeededIdx.
				break
			}
		}
	}

	// Calculate cutoff
	// We keep 0..maxUsedIndex.
	// We delete maxUsedIndex+1..End.
	cutoff := maxUsedIndex + 1
	if cutoff >= len(oldKeys) {
		return nil // Nothing to delete
	}

	keysToDelete := oldKeys[cutoff:]
	log.Printf("GC: Found %d keys to delete (older than oldest used key index %d)", len(keysToDelete), maxUsedIndex)

	// 3. Delete from Disk
	keysDir := filepath.Join(rm.DataDir, "keys")
	for _, k := range keysToDelete {
		path := filepath.Join(keysDir, k.ID)
		if err := os.Remove(path); err != nil {
			log.Printf("Warning: Failed to delete key file %s: %v", path, err)
			// Continue to remove from memory anyway?
			// Yes, consistency.
		} else {
			log.Printf("Deleted old key: %s", k.ID)
		}
	}

	// 4. Update Ring
	rm.keyRing.mu.Lock()
	// We hold lock now.
	// But between our read of Old and now, Rotate might have happened.
	// Rotate prepends to Old.
	// If Rotate happened:
	// Old became [NewActive, Old[0], Old[1]...]
	// Our 'keysToKeep' corresponds to Old[0]...Old[cutoff].
	// We want to keep the NEW ones too.

	// Better approach: filter the *current* Old list.
	// Remove keys whose IDs are in keysToDelete.

	// Let's rebuild the list to be safe against concurrent rotation.
	newOld := make([]*KeyInfo, 0)
	deleteMap := make(map[string]bool)
	for _, k := range keysToDelete {
		deleteMap[k.ID] = true
	}

	for _, k := range rm.keyRing.Old {
		if !deleteMap[k.ID] {
			newOld = append(newOld, k)
		} else {
			k.Key.Wipe() // Wipe from memory
		}
	}
	rm.keyRing.Old = newOld
	rm.keyRing.mu.Unlock()

	return nil
}

func (rm *RaftManager) saveLogKey(key crypto.EncryptionKey, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log key file: %v", err)
	}
	defer f.Close()
	if err := key.WriteEncryptedKey(f); err != nil {
		return fmt.Errorf("failed to write log key: %v", err)
	}
	return nil
}

func (rm *RaftManager) loadOrGenerateNodeKey() error {
	if err := os.MkdirAll(rm.DataDir, 0755); err != nil {
		return err
	}
	keyPath := filepath.Join(rm.DataDir, "node.key")
	if data, err := os.ReadFile(keyPath); err == nil {
		var priv ed25519.PrivateKey
		if len(data) == ed25519.PrivateKeySize {
			priv = ed25519.PrivateKey(data)
			// Migration: encrypt it if we have a MasterKey
			if rm.MasterKey != nil {
				if encrypted, err := rm.MasterKey.Encrypt(data); err == nil {
					if err := os.WriteFile(keyPath, encrypted, 0600); err != nil {
						log.Printf("Warning: failed to encrypt node.key during migration: %v", err)
					} else {
						log.Printf("Successfully encrypted node.key during migration")
					}
				}
			}
		} else if rm.MasterKey != nil {
			if decrypted, err := rm.MasterKey.Decrypt(data); err == nil && len(decrypted) == ed25519.PrivateKeySize {
				priv = ed25519.PrivateKey(decrypted)
			}
		}

		if priv != nil {
			rm.NodeKey = priv
			rm.PubKey = priv.Public().(ed25519.PublicKey)
			return nil
		}
		return fmt.Errorf("failed to load existing node key from %s", keyPath)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	rm.NodeKey = priv
	rm.PubKey = pub

	saveData := []byte(priv)
	if rm.MasterKey != nil {
		if encrypted, err := rm.MasterKey.Encrypt(saveData); err == nil {
			saveData = encrypted
		} else {
			return fmt.Errorf("failed to encrypt node key: %v", err)
		}
	}

	return os.WriteFile(keyPath, saveData, 0600)
}

func (rm *RaftManager) generateEphemeralCert() (*tls.Certificate, error) {
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: rm.NodeID,
		},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, rm.PubKey, rm.NodeKey)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalPKCS8PrivateKey(rm.NodeKey)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func (rm *RaftManager) Start(bootstrap bool) error {
	rm.Bootstrap = bootstrap
	rm.startTime = time.Now()
	if err := rm.loadKeyRing(); err != nil {
		return err
	}
	if err := rm.loadOrGenerateNodeKey(); err != nil {
		return fmt.Errorf("failed to load node key: %v", err)
	}

	// Derive NodeID from PubKey if not already set (or always?)
	// To be safe and consistent with the plan:
	rm.NodeID = hex.EncodeToString(rm.PubKey[:8])
	log.Printf("NodeID: %s", rm.NodeID)
	log.Printf("PublicKey: %s", base64.StdEncoding.EncodeToString(rm.PubKey))
	rm.nodeAddrMap.Store(raft.ServerID(rm.NodeID), rm.ClusterAdvertise)

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(rm.NodeID)
	// Optimized for WAN / High Latency / Low Idle Traffic
	if rm.UseProductionTimeouts {
		config.HeartbeatTimeout = 5 * time.Second
		config.ElectionTimeout = 20 * time.Second
		config.LeaderLeaseTimeout = 5 * time.Second
	} else {
		// Faster timeouts for tests
		config.HeartbeatTimeout = 1000 * time.Millisecond
		config.ElectionTimeout = 1000 * time.Millisecond
		config.LeaderLeaseTimeout = 500 * time.Millisecond
	}
	config.CommitTimeout = 500 * time.Millisecond

	config.SnapshotInterval = 120 * time.Second
	config.SnapshotThreshold = 20480
	//config.SnapshotThreshold = 200 // Testing

	//config.ShutdownOnRemove = true
	//config.NoSnapshotRestoreOnStart = true
	config.LogLevel = "INFO"
	config.MaxAppendEntries = 200
	if rm.LogOutput != nil {
		config.LogOutput = rm.LogOutput
	}

	notifyCh := make(chan bool, 1)
	config.NotifyCh = notifyCh

	// Setup Transport
	cert, err := rm.generateEphemeralCert()
	if err != nil {
		return fmt.Errorf("failed to generate ephemeral cert: %v", err)
	}
	rm.Cert = cert

	// Initialize reusable HTTP client with mTLS
	rm.httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:          []tls.Certificate{*rm.Cert},
				InsecureSkipVerify:    true, // Verification is done by VerifyPeerCertificate against FSM
				VerifyPeerCertificate: rm.verifyPeerCertificate,
			},
		},
	}

	sl := &tlsStreamLayer{
		rm:   rm,
		cert: cert,
	}
	if err := sl.Listen(rm.Bind); err != nil {
		return err
	}
	rm.listener = sl

	transport := raft.NewNetworkTransport(sl, 3, 10*time.Second, rm.LogOutput)

	// Setup Stores
	if err := os.MkdirAll(rm.DataDir, 0755); err != nil {
		return err
	}
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(rm.DataDir, "raft-log.bolt"))
	if err != nil {
		return err
	}
	rm.logStore = logStore // Assign immediately for cleanup
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(rm.DataDir, "raft-stable.bolt"))
	if err != nil {
		return err
	}
	rm.stableStore = stableStore // Assign immediately for cleanup

	var raftLogStore raft.LogStore = logStore
	var raftStableStore raft.StableStore = stableStore

	if rm.keyRing != nil {
		rm.logStoreEnc = NewEncryptedLogStore(logStore, rm.keyRing)
		rm.stabStoreEnc = NewEncryptedStableStore(stableStore, rm.keyRing)
		raftLogStore = rm.logStoreEnc
		raftStableStore = rm.stabStoreEnc

		// Update references to wrapped stores for proper Close() via io.Closer
		rm.logStore = raftLogStore
		rm.stableStore = raftStableStore
	}

	snapshotStore, err := raft.NewFileSnapshotStore(rm.DataDir, 1, rm.LogOutput)
	if err != nil {
		return err
	}

	var raftSnapshotStore raft.SnapshotStore = snapshotStore
	if rm.keyRing != nil {
		// Use KeyRing for snapshots with Linking (Hardlink optimization)
		var sourceDir string
		if rm.FSM != nil {
			if gs, _ := rm.FSM.GetStores(); gs != nil {
				sourceDir = gs.DataDir
			}
		}

		if sourceDir == "" {
			return fmt.Errorf("failed to determine source directory for snapshot linking: GameStore not available")
		}

		// LinkSnapshotStore wraps FileSnapshotStore but links data files instead of copying
		rm.snapStoreEnc = NewLinkSnapshotStore(rm.DataDir, sourceDir, snapshotStore, rm.keyRing, rm.MasterKey)
		raftSnapshotStore = rm.snapStoreEnc
	} else if rm.MasterKey != nil {
		// Fallback for transition/testing if KeyRing not loaded?
		// Actually loadKeyRing ensures KeyRing exists if MasterKey exists.
		// If MasterKey is nil, we don't encrypt.
	}

	r, err := raft.NewRaft(config, rm.FSM, raftLogStore, raftStableStore, raftSnapshotStore, transport)
	if err != nil {
		return err
	}
	rm.Raft = r
	close(rm.readyCh)

	if bootstrap {
		log.Printf("Bootstrapping Raft cluster with NodeID: %s", rm.NodeID)
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		f := r.BootstrapCluster(configuration)
		if err := f.Error(); err != nil {
			log.Printf("Bootstrap error (might be already bootstrapped): %v", err)
		}

		// Propose own metadata once leader
		go func() {
			for {
				if r.State() == raft.Leader {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			cmd := RaftCommand{
				Type: CmdNodeMeta,
				NodeMeta: &NodeMeta{
					NodeID:          rm.NodeID,
					HttpAddr:        rm.ClusterAdvertise,
					PubKey:          base64.StdEncoding.EncodeToString(rm.PubKey),
					AppVersion:      CurrentAppVersion,
					ProtocolVersion: CurrentProtocolVersion,
					SchemaVersion:   CurrentSchemaVersion,
				},
			}
			if _, err := rm.Propose(cmd); err != nil {
				log.Printf("Failed to propose bootstrap metadata: %v", err)
			}

			// Ingest existing data into Raft log (Migration from standalone)
			log.Printf("Ingesting existing data into Raft log...")
			gs, ts := rm.FSM.GetStores()

			for g, err := range gs.ListAllGames() {
				if err != nil {
					log.Printf("Failed to list games for ingestion: %v", err)
					break
				}
				// Reset LastRaftIndex on disk so the FSM accepts the new log entry
				g.LastRaftIndex = 0
				if err := gs.SaveGame(g); err != nil {
					log.Printf("Failed to reset index for game %s: %v", g.ID, err)
				}

				data, _ := json.Marshal(g)
				raw := json.RawMessage(data)
				cmd := RaftCommand{
					Type:     CmdSaveGame,
					ID:       g.ID,
					GameData: &raw,
					Force:    true,
				}
				if _, err := rm.Propose(cmd); err != nil {
					log.Printf("Failed to ingest game %s: %v", g.ID, err)
				}
			}

			for t, err := range ts.ListAllTeams() {
				if err != nil {
					log.Printf("Failed to list teams for ingestion: %v", err)
					break
				}
				// Reset LastRaftIndex on disk so the FSM accepts the new log entry
				t.LastRaftIndex = 0
				if err := ts.SaveTeam(t); err != nil {
					log.Printf("Failed to reset index for team %s: %v", t.ID, err)
				}

				data, _ := json.Marshal(t)
				raw := json.RawMessage(data)
				cmd := RaftCommand{
					Type:     CmdSaveTeam,
					ID:       t.ID,
					TeamData: &raw,
					Force:    true,
				}
				if _, err := rm.Propose(cmd); err != nil {
					log.Printf("Failed to ingest team %s: %v", t.ID, err)
				}
			}
			log.Printf("Ingestion complete.")
		}()
	}
	// Start Internal Secure Server
	if rm.ClusterAddr != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/cluster/status", rm.handleStatus)
		mux.HandleFunc("/api/cluster/join", rm.handleJoin)
		mux.HandleFunc("/api/cluster/remove", rm.handleRemove)
		mux.HandleFunc("/api/cluster/action", rm.handleAction)

		if rm.AppHandler != nil {
			mux.Handle("/", rm.AppHandler)
		}

		var handler http.Handler = mux
		if rm.AuthMiddleware != nil {
			handler = rm.AuthMiddleware(mux)
		}

		ln, err := net.Listen("tcp", rm.ClusterAddr)
		if err != nil {
			return fmt.Errorf("failed to listen on cluster addr %s: %v", rm.ClusterAddr, err)
		}

		// Update ClusterAdvertise if we bound to a random port
		if strings.HasSuffix(rm.ClusterAdvertise, ":0") {
			_, port, _ := net.SplitHostPort(ln.Addr().String())
			host, _, _ := net.SplitHostPort(rm.ClusterAdvertise)
			rm.ClusterAdvertise = net.JoinHostPort(host, port)
			// Also update the stored map
			rm.nodeAddrMap.Store(raft.ServerID(rm.NodeID), rm.ClusterAdvertise)
		}

		server := &http.Server{
			Handler: handler,
			TLSConfig: &tls.Config{
				Certificates:          []tls.Certificate{*cert},
				ClientAuth:            tls.RequireAnyClientCert,
				VerifyPeerCertificate: rm.verifyPeerCertificate,
			},
		}
		rm.internalServer = server

		go func() {
			log.Printf("Starting Internal Secure Cluster API on %s...", ln.Addr())
			if err := server.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
				log.Printf("Internal Server Error: %v", err)
			}
		}()
	}

	// Store own HTTP address locally as fallback/immediate
	// Note: We store ClusterAddr as the HttpAddr for internal communication
	metaJSON := fmt.Sprintf(`{"httpAddr":"%s","pubKey":"%s"}`,
		rm.ClusterAdvertise, base64.StdEncoding.EncodeToString(rm.PubKey))
	rm.FSM.applyNodeMeta(rm.NodeID, []byte(metaJSON))
	go rm.monitorConfiguration()
	go rm.monitorMetrics()
	go rm.monitorLeadership(notifyCh)

	return nil
}

// GetHTTPClient returns the reusable HTTP client for internal cluster communication.
func (rm *RaftManager) GetHTTPClient() *http.Client {
	return rm.httpClient
}

// WaitForSync blocks until the Raft FSM has applied all entries currently in the log.
// This prevents serving stale data immediately after a restart while the log is being replayed.
func (rm *RaftManager) WaitForSync(timeout time.Duration) error {
	if rm.Raft == nil {
		return nil
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return fmt.Errorf("timeout waiting for Raft sync (applied: %d, last: %d)", rm.Raft.AppliedIndex(), rm.Raft.LastIndex())
		case <-ticker.C:
			last := rm.Raft.LastIndex()
			applied := rm.Raft.AppliedIndex()
			if applied >= last {
				return nil
			}
		}
	}
}

// Propose proposes a command to the Raft cluster.
func (rm *RaftManager) Propose(cmd RaftCommand) (uint64, error) {
	if rm.Raft.State() != raft.Leader {
		return 0, ErrNotLeader
	}

	var data []byte
	var err error

	if rm.UseGob {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(cmd); err != nil {
			return 0, err
		}
		data = buf.Bytes()
	} else {
		data, err = json.Marshal(cmd)
		if err != nil {
			return 0, err
		}
	}

	f := rm.Raft.Apply(data, 5*time.Second)
	if err := f.Error(); err != nil {
		return 0, err
	}

	// f.Response() returns what FSM.Apply returns.
	// In our FSM, we return `error` or `nil`.
	resp := f.Response()
	if resp != nil {
		if err, ok := resp.(error); ok {
			return f.Index(), err
		}
	}
	return f.Index(), nil
}

// Join adds a new node to the cluster.
func (rm *RaftManager) Join(nodeID, raftAddr, httpAddr, pubKey string, nonVoter bool, appVer string, protoVer, schemaVer int) error {
	if rm.Raft.State() != raft.Leader {
		return ErrNotLeader
	}
	log.Printf("Received join request for remote node %s at Raft:%s, HTTP:%s (nonVoter: %v)", nodeID, raftAddr, httpAddr, nonVoter)

	// Store public key first so the node can connect via TLS transport
	cmd := RaftCommand{
		Type: CmdNodeMeta,
		NodeMeta: &NodeMeta{
			NodeID:          nodeID,
			HttpAddr:        httpAddr,
			PubKey:          pubKey,
			AppVersion:      appVer,
			ProtocolVersion: protoVer,
			SchemaVersion:   schemaVer,
		},
	}
	if _, err := rm.Propose(cmd); err != nil {
		return fmt.Errorf("failed to store node metadata: %v", err)
	}

	var f raft.IndexFuture
	if nonVoter {
		f = rm.Raft.AddNonvoter(raft.ServerID(nodeID), raft.ServerAddress(raftAddr), 0, 0)
	} else {
		f = rm.Raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(raftAddr), 0, 0)
	}

	if err := f.Error(); err != nil {
		return err
	}

	rm.nodeAddrMap.Store(raft.ServerID(nodeID), httpAddr)
	log.Printf("Node %s joined successfully", nodeID)
	return nil
}

// AddNodePubKey manually adds a node's public key to the authorized list.
// This is useful for priming the cluster or for the initial join handshake.
func (rm *RaftManager) AddNodePubKey(nodeID, httpAddr, pubKey string) {
	rm.FSM.nodeMap.Store(nodeID, &NodeMeta{
		NodeID:   nodeID,
		HttpAddr: httpAddr,
		PubKey:   pubKey,
	})
}

// Leave removes a node from the cluster.
func (rm *RaftManager) Leave(nodeID string) error {
	if rm.Raft.State() != raft.Leader {
		return ErrNotLeader
	}
	log.Printf("Received leave request for node %s", nodeID)

	f := rm.Raft.RemoveServer(raft.ServerID(nodeID), 0, 0)
	if err := f.Error(); err != nil {
		return err
	}

	// Broadcast node removal to cluster map
	cmd := RaftCommand{
		Type: CmdNodeLeft,
		NodeMeta: &NodeMeta{
			NodeID: nodeID,
		},
	}
	if _, err := rm.Propose(cmd); err != nil {
		log.Printf("Warning: Failed to broadcast node removal: %v", err)
	}

	rm.nodeAddrMap.Delete(raft.ServerID(nodeID))
	log.Printf("Node %s removed successfully", nodeID)
	return nil
}

func (rm *RaftManager) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Require Secret for status to prevent leaking topology.
	secret := r.Header.Get("X-Raft-Secret")
	if rm.Secret == "" || secret != rm.Secret {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	leaderAddr := rm.GetLeaderHTTPAddr()
	_, leaderID := rm.Raft.LeaderWithID()

	status := map[string]any{
		"nodeId":          rm.NodeID,
		"state":           rm.Raft.State().String(),
		"leaderId":        string(leaderID),
		"leaderAddr":      leaderAddr,
		"raftAddr":        rm.Advertise,
		"pubKey":          base64.StdEncoding.EncodeToString(rm.PubKey),
		"appVersion":      CurrentAppVersion,
		"protocolVersion": CurrentProtocolVersion,
		"schemaVersion":   CurrentSchemaVersion,
	}
	if status["raftAddr"] == "" {
		status["raftAddr"] = rm.Bind
	}

	configFuture := rm.Raft.GetConfiguration()
	if err := configFuture.Error(); err == nil {
		var nodes []map[string]any
		for _, s := range configFuture.Configuration().Servers {
			node := map[string]any{
				"id":       string(s.ID),
				"raftAddr": string(s.Address),
				"httpAddr": rm.FSM.GetNodeAddr(string(s.ID)),
				"pubKey":   rm.FSM.GetNodePubKey(string(s.ID)),
				"suffrage": s.Suffrage.String(),
			}
			if meta := rm.FSM.GetNodeMeta(string(s.ID)); meta != nil {
				node["appVersion"] = meta.AppVersion
				node["protocolVersion"] = meta.ProtocolVersion
				node["schemaVersion"] = meta.SchemaVersion
			}
			nodes = append(nodes, node)
		}
		status["nodes"] = nodes
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (rm *RaftManager) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Loop detection
	if forwarded := r.Header.Get("X-Raft-Forwarded"); forwarded != "" {
		for _, id := range strings.Split(forwarded, ",") {
			if strings.TrimSpace(id) == rm.NodeID {
				http.Error(w, "Forwarding loop detected", http.StatusLoopDetected)
				return
			}
		}
	}

	// Authentication (Already mTLS verified, but checking secret adds defense-in-depth)
	secret := r.Header.Get("X-Raft-Secret")
	if rm.Secret == "" || secret != rm.Secret {
		http.Error(w, "Forbidden: Invalid Cluster Secret", http.StatusForbidden)
		return
	}

	if rm.Raft.State() != raft.Leader {
		rm.forwardRequestToLeader(w, r)
		return
	}

	var data struct {
		NodeID          string `json:"nodeId"`
		RaftAddr        string `json:"raftAddr"`
		HttpAddr        string `json:"httpAddr"`
		PubKey          string `json:"pubKey"`
		NonVoter        bool   `json:"nonVoter"`
		AppVersion      string `json:"appVersion"`
		ProtocolVersion int    `json:"protocolVersion"`
		SchemaVersion   int    `json:"schemaVersion"`
	}
	// We decode into a fresh struct, so we can't reuse body if we forwarded.
	// But forwarding happens before decode.
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if data.HttpAddr == "" || data.PubKey == "" {
		http.Error(w, "Missing required fields: httpAddr and pubKey are required", http.StatusBadRequest)
		return
	}

	if data.NodeID == "" {
		// Attempt Discovery
		status, err := rm.discoverNode(data.HttpAddr, data.PubKey)
		if err != nil {
			log.Printf("Discovery failed for %s: %v", data.HttpAddr, err)
			http.Error(w, fmt.Sprintf("Discovery failed: %v", err), http.StatusBadGateway)
			return
		}

		// Fill in discovered details
		var ok bool
		if data.NodeID, ok = status["nodeId"].(string); !ok || data.NodeID == "" {
			http.Error(w, "Discovery failed: missing nodeId in response", http.StatusBadGateway)
			return
		}
		if data.RaftAddr, ok = status["raftAddr"].(string); !ok || data.RaftAddr == "" {
			http.Error(w, "Discovery failed: missing raftAddr in response", http.StatusBadGateway)
			return
		}
		if data.AppVersion, ok = status["appVersion"].(string); !ok {
			data.AppVersion = ""
		}
		if v, ok := status["protocolVersion"].(float64); ok {
			data.ProtocolVersion = int(v)
		}
		if v, ok := status["schemaVersion"].(float64); ok {
			data.SchemaVersion = int(v)
		}
	}

	// Validate Address Formats
	if _, _, err := net.SplitHostPort(data.RaftAddr); err != nil {
		http.Error(w, "Invalid RaftAddr: must be host:port", http.StatusBadRequest)
		return
	}
	if _, _, err := net.SplitHostPort(data.HttpAddr); err != nil {
		// Not a standard host:port, check if it's a valid URL
		u, pErr := url.Parse(data.HttpAddr)
		if pErr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			http.Error(w, "Invalid HttpAddr: must be host:port or valid URL", http.StatusBadRequest)
			return
		}
	}

	// Validate PubKey (Base64)
	if _, err := base64.StdEncoding.DecodeString(data.PubKey); err != nil {
		http.Error(w, "Invalid PubKey: must be base64", http.StatusBadRequest)
		return
	}

	if err := rm.Join(data.NodeID, data.RaftAddr, data.HttpAddr, data.PubKey, data.NonVoter, data.AppVersion, data.ProtocolVersion, data.SchemaVersion); err != nil {
		http.Error(w, fmt.Sprintf("Failed to join: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Node %s joined cluster", data.NodeID)
}

func (rm *RaftManager) discoverNode(targetAddr, expectedPubKeyBase64 string) (map[string]any, error) {
	if !strings.HasPrefix(targetAddr, "http") {
		targetAddr = "https://" + targetAddr
	}
	u, err := url.Parse(targetAddr)
	if err != nil {
		return nil, err
	}
	u.Path = "/api/cluster/status"
	url := u.String()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{*rm.Cert},
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				if len(rawCerts) == 0 {
					return fmt.Errorf("no peer certificate")
				}
				cert, err := x509.ParseCertificate(rawCerts[0])
				if err != nil {
					return err
				}
				pubKey, ok := cert.PublicKey.(ed25519.PublicKey)
				if !ok {
					return fmt.Errorf("peer public key is not ed25519")
				}
				expectedPubKey, err := base64.StdEncoding.DecodeString(expectedPubKeyBase64)
				if err != nil {
					return err
				}
				if !ed25519.PublicKey(expectedPubKey).Equal(pubKey) {
					return fmt.Errorf("public key mismatch: expected %s", expectedPubKeyBase64)
				}
				return nil
			},
		},
	}
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Raft-Secret", rm.Secret)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discoverNode(%q): %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("node returned status %d: %s", resp.StatusCode, string(body))
	}

	var status map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	// Verify that the pubKey in the JSON also matches
	if pk, ok := status["pubKey"].(string); !ok || pk != expectedPubKeyBase64 {
		return nil, fmt.Errorf("discovered public key mismatch in JSON response")
	}

	return status, nil
}

func (rm *RaftManager) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Loop detection
	if forwarded := r.Header.Get("X-Raft-Forwarded"); forwarded != "" {
		for _, id := range strings.Split(forwarded, ",") {
			if strings.TrimSpace(id) == rm.NodeID {
				http.Error(w, "Forwarding loop detected", http.StatusLoopDetected)
				return
			}
		}
	}

	secret := r.Header.Get("X-Raft-Secret")
	if rm.Secret == "" || secret != rm.Secret {
		http.Error(w, "Forbidden: Invalid Cluster Secret", http.StatusForbidden)
		return
	}

	if rm.Raft.State() != raft.Leader {
		rm.forwardRequestToLeader(w, r)
		return
	}

	var data struct {
		NodeID string `json:"nodeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := rm.Leave(data.NodeID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to remove node: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Node %s removed from cluster", data.NodeID)
}

func (rm *RaftManager) forwardRequestToLeader(w http.ResponseWriter, r *http.Request) {
	leaderAddr := rm.GetLeaderHTTPAddr()
	if leaderAddr == "" {
		http.Error(w, "No leader found", http.StatusServiceUnavailable)
		return
	}

	if strings.HasPrefix(leaderAddr, "http://") {
		leaderAddr = "https://" + strings.TrimPrefix(leaderAddr, "http://")
	} else if !strings.HasPrefix(leaderAddr, "https://") {
		leaderAddr = "https://" + leaderAddr
	}

	url := leaderAddr + r.URL.Path
	// We need to buffer the body to forward it
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest(r.Method, url, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Failed to create forward request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for k, v := range r.Header {
		req.Header[k] = v
	}
	req.Host = r.Host

	// Update X-Raft-Forwarded
	forwarded := req.Header.Get("X-Raft-Forwarded")
	if forwarded != "" {
		forwarded += "," + rm.NodeID
	} else {
		forwarded = rm.NodeID
	}
	req.Header.Set("X-Raft-Forwarded", forwarded)

	// Ensure secret is set
	if rm.Secret != "" {
		req.Header.Set("X-Raft-Secret", rm.Secret)
	}

	resp, err := rm.httpClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (rm *RaftManager) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Loop detection
	if forwarded := r.Header.Get("X-Raft-Forwarded"); forwarded != "" {
		for _, id := range strings.Split(forwarded, ",") {
			if strings.TrimSpace(id) == rm.NodeID {
				http.Error(w, "Forwarding loop detected", http.StatusLoopDetected)
				return
			}
		}
	}

	secret := r.Header.Get("X-Raft-Secret")
	if rm.Secret == "" || secret != rm.Secret {
		http.Error(w, "Forbidden: Invalid Cluster Secret", http.StatusForbidden)
		return
	}

	userId := getUserID(r)

	var msg Message
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&msg); err != nil {
		http.Error(w, "Bad Request: Malformed JSON", http.StatusBadRequest)
		return
	}

	gameId := msg.GameId
	if gameId == "" {
		http.Error(w, "Bad Request: gameId is missing", http.StatusBadRequest)
		return
	}

	// Serialize through Hub
	hub := rm.FSM.GetHub(gameId, false)
	reply := make(chan HubResponse)
	hub.requests <- HubRequest{
		Type:    ReqTypeHTTPAction,
		UserId:  userId,
		Headers: r.Header,
		Message: msg,
		Reply:   reply,
	}
	resp := <-reply

	if resp.Error != nil {
		log.Printf("Error processing forwarded HTTP action: %v", resp.Error)
		http.Error(w, resp.Error.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp.Data)
}

// GetLeaderHTTPAddr returns the HTTP address of the current leader.
func (rm *RaftManager) GetLeaderHTTPAddr() string {
	_, leaderID := rm.Raft.LeaderWithID()
	if leaderID == "" {
		return ""
	}
	return rm.FSM.GetNodeAddr(string(leaderID))
}

// Shutdown gracefully shuts down the Raft node.
func (rm *RaftManager) Shutdown() error {
	rm.shutdownOnce.Do(func() {
		close(rm.shutdownCh)
	})

	if rm.internalServer != nil {
		rm.internalServer.Close()
	}

	if rm.listener != nil {
		rm.listener.Close()
	}

	if rm.Raft == nil {
		rm.closeStores()
		return nil
	}

	// Attempt graceful leadership transfer if leader
	if rm.Raft.State() == raft.Leader {
		log.Printf("Attempting leadership transfer before shutdown...")
		f := rm.Raft.LeadershipTransfer()

		// Wait for transfer with timeout
		done := make(chan error, 1)
		go func() { done <- f.Error() }()

		select {
		case err := <-done:
			if err != nil {
				log.Printf("Leadership transfer failed (continuing): %v", err)
			} else {
				log.Printf("Leadership transfer successful.")
			}
		case <-time.After(5 * time.Second):
			log.Printf("Leadership transfer timed out (continuing).")
		}
	}

	raftErr := rm.Raft.Shutdown().Error()
	rm.closeStores()
	return raftErr
}

func (rm *RaftManager) closeStores() {
	rm.logKeyMu.Lock()
	defer rm.logKeyMu.Unlock()

	if rm.logStore != nil {
		if c, ok := rm.logStore.(io.Closer); ok {
			c.Close()
		}
		rm.logStore = nil
	}
	if rm.stableStore != nil {
		if c, ok := rm.stableStore.(io.Closer); ok {
			c.Close()
		}
		rm.stableStore = nil
	}

	if rm.keyRing != nil {
		rm.keyRing.Wipe()
		rm.keyRing = nil
	}
}

func (rm *RaftManager) monitorConfiguration() {
	// Wait for leader
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rm.shutdownCh:
			return
		case <-ticker.C:
			// 1. Check if we have a leader
			_, leaderID := rm.Raft.LeaderWithID()

			// 2. Identify if we are Leader
			if leaderID == raft.ServerID(rm.NodeID) {
				// We are Leader: Update own metadata if needed
				currentHttpAddr := rm.FSM.GetNodeAddr(rm.NodeID)
				if currentHttpAddr != rm.ClusterAdvertise {
					log.Printf("[AutoConfig] Updating own HTTP address from %q to %q", currentHttpAddr, rm.ClusterAdvertise)
					cmd := RaftCommand{
						Type: CmdNodeMeta,
						NodeMeta: &NodeMeta{
							NodeID:          rm.NodeID,
							HttpAddr:        rm.ClusterAdvertise,
							PubKey:          base64.StdEncoding.EncodeToString(rm.PubKey),
							AppVersion:      CurrentAppVersion,
							ProtocolVersion: CurrentProtocolVersion,
							SchemaVersion:   CurrentSchemaVersion,
						},
					}
					if _, err := rm.Propose(cmd); err != nil {
						log.Printf("[AutoConfig] Failed to update own metadata: %v", err)
					}
				}
				// Update Raft Address if needed
				cfg := rm.Raft.GetConfiguration()
				if err := cfg.Error(); err == nil {
					for _, s := range cfg.Configuration().Servers {
						if s.ID == raft.ServerID(rm.NodeID) {
							advertiseAddr := rm.Advertise
							if advertiseAddr == "" {
								advertiseAddr = rm.Bind
							}
							if string(s.Address) != advertiseAddr {
								log.Printf("[AutoConfig] Updating own Raft address from %q to %q", s.Address, advertiseAddr)
								if f := rm.Raft.AddVoter(s.ID, raft.ServerAddress(advertiseAddr), 0, 0); f.Error() != nil {
									log.Printf("[AutoConfig] Failed to update own Raft address: %v", f.Error())
								}
							}
							break
						}
					}
				}
				continue
			}

			// We are Follower (or Lost Candidate).
			// Try to contact Leader, or if unknown, any known peer.
			var targetHTTP string
			if leaderID != "" {
				targetHTTP = rm.FSM.GetNodeAddr(string(leaderID))
			} else if rm.FSM.GetNodeCount() > 1 {
				// Fallback: Try to contact any peer to re-announce ourselves
				allNodes := rm.FSM.GetAllNodes()
				for id, addr := range allNodes {
					if id != rm.NodeID && addr != "" {
						targetHTTP = addr
						break
					}
				}
			}

			if targetHTTP == "" {
				continue
			}

			// Ensure leaderHTTP has protocol
			if !strings.HasPrefix(targetHTTP, "http") {
				targetHTTP = "https://" + targetHTTP
			}

			raftAddr := rm.Advertise
			if raftAddr == "" {
				raftAddr = rm.Bind
			}

			payload := map[string]any{
				"nodeId":          rm.NodeID,
				"raftAddr":        raftAddr,
				"httpAddr":        rm.ClusterAdvertise,
				"pubKey":          base64.StdEncoding.EncodeToString(rm.PubKey),
				"appVersion":      CurrentAppVersion,
				"protocolVersion": CurrentProtocolVersion,
				"schemaVersion":   CurrentSchemaVersion,
			}

			// Check if we are currently a Nonvoter to preserve that status
			cfg := rm.Raft.GetConfiguration()
			if err := cfg.Error(); err == nil {
				for _, s := range cfg.Configuration().Servers {
					if s.ID == raft.ServerID(rm.NodeID) && s.Suffrage == raft.Nonvoter {
						payload["nonVoter"] = true
						break
					}
				}
			}

			data, _ := json.Marshal(payload)

			tr := &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates:          []tls.Certificate{*rm.Cert},
					InsecureSkipVerify:    true, // Verification is done by VerifyPeerCertificate against FSM
					VerifyPeerCertificate: rm.verifyPeerCertificate,
				},
			}
			client := &http.Client{Transport: tr, Timeout: 5 * time.Second}

			req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/cluster/join", targetHTTP), bytes.NewBuffer(data))
			if err != nil {
				log.Printf("[AutoConfig] Failed to create join request: %v", err)
				return
			}
			req.Header.Set("X-Raft-Secret", rm.Secret)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[AutoConfig] Failed to contact node at %s: %v", targetHTTP, err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				log.Printf("[AutoConfig] Successfully registered with node at %s", targetHTTP)
				return
			}

			log.Printf("[AutoConfig] Registration failed: HTTP %d", resp.StatusCode)
		}
	}
}

type tlsStreamLayer struct {
	rm   *RaftManager
	ln   net.Listener
	cert *tls.Certificate
}

func (t *tlsStreamLayer) Listen(bind string) error {
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		return err
	}
	t.ln = ln
	return nil
}

func (t *tlsStreamLayer) Accept() (net.Conn, error) {
	conn, err := t.ln.Accept()
	if err != nil {
		return nil, err
	}

	// Block incoming connections until Raft is fully initialized (FSM restored).
	// This prevents the TOFU check from running against an empty FSM during restart.
	select {
	case <-t.rm.readyCh:
	case <-t.rm.shutdownCh:
		conn.Close()
		return nil, fmt.Errorf("raft manager shutting down")
	}

	return tls.Server(conn, &tls.Config{
		Certificates:          []tls.Certificate{*t.cert},
		ClientAuth:            tls.RequireAnyClientCert,
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: t.rm.verifyPeerCertificate,
	}), nil
}

func (t *tlsStreamLayer) Close() error {
	return t.ln.Close()
}

func (t *tlsStreamLayer) Addr() net.Addr {
	if t.rm.Advertise != "" {
		return raftAddress{addr: t.rm.Advertise}
	}
	return t.ln.Addr()
}

func (t *tlsStreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	config := &tls.Config{
		Certificates:          []tls.Certificate{*t.cert},
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: t.rm.verifyPeerCertificate,
	}
	return tls.DialWithDialer(dialer, "tcp", string(address), config)
}

func (rm *RaftManager) verifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("no peer certificate")
	}
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return err
	}

	nodeID := cert.Subject.CommonName
	pubKey, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf("peer public key is not ed25519, got %T", cert.PublicKey)
	}

	expectedPubKeyBase64 := rm.FSM.GetNodePubKey(nodeID)
	if expectedPubKeyBase64 == "" {
		// TOFU: Trust On First Use
		// If we are NOT the bootstrap node (Leader), and we have never joined a cluster (initialized == false),
		// we allow the connection to proceed. This allows the Leader to join us and replicate
		// the cluster state (including valid public keys).
		if !rm.Bootstrap && !rm.FSM.IsInitialized() {
			log.Printf("Security Warning: TOFU accepted for node %s (initial join)", nodeID)
			if rm.tofuCallback != nil {
				rm.tofuCallback(nodeID)
			}
			return nil
		}
		// Debugging unknown node
		known := rm.FSM.GetAllNodes()
		keys := make([]string, 0, len(known))
		for k := range known {
			keys = append(keys, k)
		}
		return fmt.Errorf("unknown node %s (known: %v, initialized: %v, bootstrap: %v)", nodeID, keys, rm.FSM.IsInitialized(), rm.Bootstrap)
	}

	expectedPubKey, err := base64.StdEncoding.DecodeString(expectedPubKeyBase64)
	if err != nil {
		return err
	}

	if !ed25519.PublicKey(expectedPubKey).Equal(pubKey) {
		return fmt.Errorf("public key mismatch for node %s", nodeID)
	}

	return nil
}

type raftAddress struct {
	addr string
}

func (a raftAddress) Network() string {
	return "tcp"
}

func (a raftAddress) String() string {
	return a.addr
}

func (rm *RaftManager) monitorLeadership(notifyCh <-chan bool) {
	for {
		select {
		case <-rm.shutdownCh:
			return
		case isLeader := <-notifyCh:
			if isLeader {
				// We became leader. Calculate gap since last contact.
				var gap time.Duration
				last := rm.Raft.LastContact()
				if !last.IsZero() {
					gap = time.Since(last)
					log.Printf("Leadership acquired. Gap since last contact: %v", gap)
				} else {
					// Fallback: Check FSM for last metrics timestamp (handling restart)
					// Wait for FSM to catch up with logs to ensure we have the latest metrics
					// We use a longer timeout here to ensure full replay on large logs.
					syncStart := time.Now()
					if err := rm.WaitForSync(30 * time.Second); err != nil {
						log.Printf("Warning: WaitForSync timed out in monitorLeadership: %v", err)
					}
					log.Printf("FSM sync completed in %v", time.Since(syncStart))

					lastTs := rm.FSM.GetLastMetricsTimestamp()
					if lastTs > 0 {
						gap = time.Since(time.Unix(lastTs, 0))
						log.Printf("Leadership acquired. LastContact was zero. Calculated gap since FSM timestamp %d: %v", lastTs, gap)
					} else if !rm.startTime.IsZero() {
						// Fallback 2: Gap since node start (Cold Start)
						gap = time.Since(rm.startTime)
						log.Printf("Leadership acquired. Cold start detected. Gap since node start: %v", gap)
					}
				}

				if gap > 0 {
					atomic.AddInt64(&rm.pendingGapMS, gap.Milliseconds())
				} else {
					log.Printf("Leadership acquired. No gap detected (gap=%v)", gap)
				}
			}
		}
	}
}

func (rm *RaftManager) monitorMetrics() {
	// Align to the next minute boundary for cleaner charts
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	time.Sleep(time.Until(nextMinute))

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Initial report immediately (after alignment)
	rm.reportMetrics()

	for {
		select {
		case <-rm.shutdownCh:
			return
		case <-ticker.C:
			rm.reportMetrics()
		}
	}
}

func (rm *RaftManager) reportMetrics() {
	count := GlobalRequestCounter.Load()
	activeWS := 0
	if rm.FSM != nil {
		activeWS = rm.FSM.GetActiveWSCount()
	}
	timestamp := time.Now().Unix()

	// Capture and reset latency histogram
	rm.latencyMu.Lock()
	latency := rm.latencyAccumulator
	rm.latencyAccumulator = &Histogram{}
	rm.latencyMu.Unlock()

	// Prepare payload
	payload := map[string]any{
		"nodeId":    rm.NodeID,
		"timestamp": timestamp,
		"total":     count,
		"activeWS":  activeWS,
		"latency":   latency,
	}
	data, _ := json.Marshal(payload)

	// Send to Leader
	leaderAddr := rm.GetLeaderHTTPAddr()
	if leaderAddr == "" {
		log.Printf("Metrics: No leader address found, skipping report (NodeID: %s)", rm.NodeID)
		return
	}

	// Ensure protocol
	if !strings.HasPrefix(leaderAddr, "http") {
		leaderAddr = "https://" + leaderAddr
	}
	url := fmt.Sprintf("%s/api/cluster/metrics", leaderAddr)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Metrics Error: failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Raft-Secret", rm.Secret)

	resp, err := rm.httpClient.Do(req)
	if err != nil {
		log.Printf("Metrics Error: failed to send report to %s: %v", leaderAddr, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Metrics Error: Leader returned %d: %s", resp.StatusCode, string(body))
	}
}

func (rm *RaftManager) handleMetricsReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	secret := r.Header.Get("X-Raft-Secret")
	if rm.Secret == "" || secret != rm.Secret {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if rm.Raft.State() != raft.Leader {
		rm.forwardRequestToLeader(w, r)
		return
	}

	var req struct {
		NodeID    string     `json:"nodeId"`
		Timestamp int64      `json:"timestamp"`
		Total     uint64     `json:"total"`
		ActiveWS  int        `json:"activeWS"`
		Latency   *Histogram `json:"latency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Calculate Delta / RPS
	rm.countersMu.Lock()
	last, exists := rm.nodeCounters[req.NodeID]
	rm.nodeCounters[req.NodeID] = req.Total
	rm.countersMu.Unlock()

	var delta uint64
	if !exists || req.Total < last {
		// First report or restart detected
		delta = req.Total
	} else {
		delta = req.Total - last
	}

	rps := float64(delta) / 60.0

	// Construct Raft Command
	metricsCmd := &MetricsPayload{
		Timestamp: req.Timestamp,
		Nodes: []NodeMetric{
			{NodeID: req.NodeID, RPS: rps, ActiveWS: req.ActiveWS, Latency: req.Latency},
		},
	}

	// If sender is self (Leader), also append Cluster Metrics
	if req.NodeID == rm.NodeID {
		stats := rm.Raft.Stats()

		// Parse string stats to numbers (Raft returns strings)
		// Actually, let's just use what we can easily get or maintain.
		// "num_peers", "term", "commit_index", "last_log_index", "last_snapshot_index"
		// The `stats` map keys depend on implementation.
		// Hashicorp Raft stats: "applied_index", "commit_index", "fsm_pending", "last_contact", "last_log_index", "last_log_term", "last_snapshot_index", "last_snapshot_term", "latest_configuration", "latest_configuration_index", "num_peers", "protocol_version", "protocol_version_min", "protocol_version_max", "snapshot_version_min", "snapshot_version_max", "state", "term"

		parseUint := func(key string) uint64 {
			if v, ok := stats[key]; ok {
				var i uint64
				fmt.Sscanf(v, "%d", &i)
				return i
			}
			return 0
		}

		metricsCmd.Cluster = &ClusterMetric{
			NodeCount:    rm.FSM.GetNodeCount(),
			Elections:    parseUint("term"), // Approximation: Term is roughly election count
			LastLogIndex: rm.Raft.LastIndex(),
			Snapshots:    parseUint("last_snapshot_index"), // Not really count, but index. Close enough for visual.
			LeaderGapMS:  uint64(atomic.SwapInt64(&rm.pendingGapMS, 0)),
			TotalGames:   rm.FSM.GetTotalGames(),
			TotalTeams:   rm.FSM.GetTotalTeams(),
		}
	}

	cmd := RaftCommand{
		Type:           CmdMetricsUpdate,
		MetricsPayload: metricsCmd,
	}

	if _, err := rm.Propose(cmd); err != nil {
		log.Printf("Metrics Error: failed to propose update: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// log.Printf("Metrics: Proposed update for node %s (RPS: %.2f)", req.NodeID, rps) // Debug log
	w.WriteHeader(http.StatusOK)
}

func (rm *RaftManager) handleMetricsQuery(w http.ResponseWriter, r *http.Request) {
	// Allow any authenticated node/user to query?
	// The doc says: "The Admin Dashboard and the GET ... endpoint are available on any node"
	// So we assume this is protected by `server.go` routing (it is NOT protected by middleware there for public API?)
	// Wait, `server.go` put it under `/api/cluster/metrics`.
	// If it's for Admin Dashboard, maybe we should protect it?
	// The `handleMetricsQuery` checks for Secret if called inter-cluster, but for frontend?
	// `server.go` uses `jwtAuthMiddleware` for the whole handler.
	// So a user needs a JWT.
	// We also might want to check if user is Admin?
	// For now, let's keep it open to authenticated users.

	data := rm.FSM.GetMetricsJSON()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
