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

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/ttbt-io/skorekeeper/backend"
)

var (
	addr             = flag.String("addr", ":8080", "The TCP address to listen to")
	useMockAuth      = flag.Bool("use-mock-auth", false, "Use Mock Authentication. For testing purposes only.")
	debugMode        = flag.Bool("debug", false, "Enable debug mode")
	raftEnabled      = flag.Bool("raft", false, "Enable Raft consensus")
	raftBind         = flag.String("raft-bind", ":8081", "Address for Raft TCP transport")
	raftAdvertise    = flag.String("raft-advertise", "", "Public address for Raft traffic (REQUIRED)")
	clusterAdvertise = flag.String("cluster-advertise", "", "Public address for internal cluster traffic (REQUIRED)")
	clusterAddr      = flag.String("cluster-addr", ":9090", "Address for internal secure cluster API (mTLS)")
	raftSecret       = flag.String("raft-secret", "", "Shared secret for cluster authentication")
	raftBootstrap    = flag.Bool("raft-bootstrap", false, "Bootstrap the Raft cluster (only for first node)")
	dataDir          = flag.String("data-dir", "data", "Directory for game and team data")
	tlsCert          = flag.String("tls-cert", "", "Path to main HTTP TLS certificate")
	tlsKey           = flag.String("tls-key", "", "Path to main HTTP TLS key")
	authCookieName   = flag.String("auth-cookie-name", "skorekeeper_auth", "Name of the cookie containing the JWT")
	authJWKSURL      = flag.String("auth-jwks-url", "", "Comma-separated list of [ISSUER=]URL for JWKS endpoints")
	bootstrapAdmin   = flag.String("admin", "", "Email of temporary admin user for bootstrapping access policy")
	minifyMode       = flag.Bool("minify", false, "Serve minified frontend assets from dist/")
	forceRebuild     = flag.Bool("force-rebuild", false, "Force rebuild of Registry indices on startup")
)

// main starts the web server and registers the API handlers.
func main() {
	flag.Parse()

	if *raftEnabled {
		if *raftAdvertise == "" {
			log.Fatal("--raft-advertise is required when Raft is enabled")
		}
		if *clusterAdvertise == "" {
			log.Fatal("--cluster-advertise is required when Raft is enabled")
		}
		if *raftSecret == "" {
			log.Fatal("--raft-secret is required when Raft is enabled")
		}
	}

	var mainTLSCert *tls.Certificate
	if *tlsCert != "" && *tlsKey != "" {
		cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		if err != nil {
			log.Fatalf("Failed to load main TLS cert/key: %v", err)
		}
		mainTLSCert = &cert
	}

	// Initialize Encryption Key and Storage
	var masterKey crypto.MasterKey
	if passphrase := os.Getenv("SK_MASTER_KEY"); passphrase != "" {
		keyFile := filepath.Join(*dataDir, "master.key")
		// Ensure data dir exists for key file
		os.MkdirAll(*dataDir, 0755)

		var err error
		masterKey, err = crypto.ReadMasterKey([]byte(passphrase), keyFile)
		if err != nil {
			if os.IsNotExist(err) {
				log.Println("Initializing new master encryption key...")
				masterKey, err = crypto.CreateMasterKey()
				if err != nil {
					log.Fatalf("Failed to create master key: %v", err)
				}
				if err := masterKey.Save([]byte(passphrase), keyFile); err != nil {
					log.Fatalf("Failed to save master key: %v", err)
				}
			} else {
				log.Fatalf("Failed to read master key: %v", err)
			}
		} else {
			log.Println("Loaded master encryption key.")
		}
	} else {
		keyFile := filepath.Join(*dataDir, "master.key")
		if _, err := os.Stat(keyFile); err == nil {
			log.Fatalf("Critical Security Error: %s exists but SK_MASTER_KEY is not set. Refusing to start in unencrypted mode to prevent data corruption or exposure.", keyFile)
		}
		log.Println("Warning: No SK_MASTER_KEY provided. Data will be stored UNENCRYPTED.")
	}

	store := storage.New(*dataDir, masterKey)
	store.EnableCompression(true)

	server, err := backend.StartServer(backend.Options{
		Addr:                  *addr,
		ClusterAdvertise:      *clusterAdvertise,
		ClusterAddr:           *clusterAddr,
		Cert:                  mainTLSCert,
		DataDir:               *dataDir,
		UseMockAuth:           *useMockAuth,
		Debug:                 *debugMode,
		Storage:               store,
		RaftEnabled:           *raftEnabled,
		RaftBind:              *raftBind,
		RaftAdvertise:         *raftAdvertise,
		RaftSecret:            *raftSecret,
		RaftBootstrap:         *raftBootstrap,
		UseProductionTimeouts: true,
		AuthCookieName:        *authCookieName,
		AuthJWKSURL:           *authJWKSURL,
		BootstrapAdmin:        *bootstrapAdmin,
		MinifyMode:            *minifyMode,
		ForceRebuild:          *forceRebuild,
	})
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	} else {
		log.Println("Gracefully stopped.")
	}
}
