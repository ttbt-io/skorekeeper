package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/ttbt-io/skorekeeper/backend"
)

var (
	dataDir = flag.String("data-dir", "data", "Directory for game and team data")
)

// main starts the web server and registers the API handlers.
func main() {
	flag.Parse()
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
			log.Fatalf("Critical Security Error: %s exists but SK_MASTER_KEY is not set. Refusing to read encrypted data in unencrypted mode.", keyFile)
		}
		log.Println("Warning: No SK_MASTER_KEY provided. Data will be stored UNENCRYPTED.")
	}
	store := storage.New(*dataDir, masterKey)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	for _, arg := range flag.Args() {
		arg = strings.TrimPrefix(arg, *dataDir)
		var obj any
		if strings.Contains(arg, "games") {
			obj = new(backend.Game)
		} else {
			obj = new(backend.Team)
		}
		if err := store.ReadDataFile(arg, obj); err != nil {
			log.Printf("%s: %v", arg, err)
			continue
		}
		fmt.Printf("=========== %s ===========\n", arg)
		if err := enc.Encode(obj); err != nil {
			log.Printf("JSON: %s: %v", arg, err)
		}
	}
}
