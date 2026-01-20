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
	"context"
	"crypto/sha256"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/c2FmZQ/storage"
	"github.com/c2FmZQ/storage/crypto"
	"github.com/ttbt-io/skorekeeper/frontend"
)

func generateETag(data []byte) string {
	return fmt.Sprintf("\"%x\"", sha256.Sum256(data))
}

func hubBusyResponse(w http.ResponseWriter, retryAfter string) {
	w.Header().Set("Retry-After", retryAfter)
	http.Error(w, "Too Many Requests: Server is busy", http.StatusTooManyRequests)
}

func parsePagination(r *http.Request) (int, int, string, string, string) {
	limit := 50
	offset := 0
	sortBy := r.URL.Query().Get("sortBy")
	order := r.URL.Query().Get("order")
	query := r.URL.Query().Get("q")

	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			offset = val
		}
	}

	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return limit, offset, sortBy, order, query
}

// Options represent server options.
type Options struct {
	Addr             string
	ClusterAdvertise string
	ClusterAddr      string
	Cert             *tls.Certificate
	DataDir          string
	UseMockAuth      bool
	Debug            bool
	GameStore        *GameStore
	TeamStore        *TeamStore
	Storage          *storage.Storage
	MasterKey        crypto.MasterKey
	Registry         *Registry
	Listener         net.Listener

	// Raft Options
	RaftEnabled           bool
	RaftBind              string
	RaftAdvertise         string
	RaftSecret            string
	RaftJoin              string // Address of leader to join
	RaftBootstrap         bool
	RaftManager           *RaftManager      // Allow injecting pre-configured RaftManager
	RaftManagerChan       chan *RaftManager // For testing: receive the created RaftManager
	UseProductionTimeouts bool              // Set to true to use longer timeouts (e.g. for production)

	// Auth Options
	AuthCookieName string
	AuthJWKSURL    string

	// Access Control Options
	BootstrapAdmin string

	MinifyMode bool
}

//go:embed cluster_dashboard.html
var clusterDashboardHTML []byte

//go:embed cluster_dashboard.js
var clusterDashboardJS []byte

//go:embed admin_dashboard.html
var adminDashboardHTML []byte

//go:embed admin_dashboard.js
var adminDashboardJS []byte

const (
	retryAfterLoad   = "2"
	retryAfterSave   = "10"
	retryAfterAction = "5"
)

// Server represents the running server instance.
type Server struct {
	httpServer *http.Server
	raftMgr    *RaftManager
}

// Shutdown gracefully shuts down the server and Raft node.
func (s *Server) Shutdown(ctx context.Context) error {
	var errs []string

	flush := func() {
		if s.raftMgr != nil {
			if err := s.raftMgr.Shutdown(); err != nil {
				errs = append(errs, fmt.Sprintf("raft: %v", err))
			}
			// Ensure any dirty FSM state is flushed to disk on shutdown
			if s.raftMgr.FSM != nil {
				if err := s.raftMgr.FSM.FlushAll(); err != nil {
					errs = append(errs, fmt.Sprintf("fsm flush: %v", err))
				}
			}
		}
	}
	flush()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Sprintf("http: %v", err))
	}
	flush()

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errs, ", "))
	}
	return nil
}

// StartServer starts the web server and registers the API handlers.
func StartServer(opts Options) (*Server, error) {
	raftMgr, handler := NewServerHandler(opts)

	if raftMgr != nil {
		// Wait for Raft to replay log and catch up to ensure data consistency
		// before starting the public HTTP server.
		if err := raftMgr.WaitForSync(30 * time.Second); err != nil {
			log.Printf("Warning: Raft sync timed out: %v", err)
		}
	}

	httpServer := &http.Server{
		Addr:    opts.Addr,
		Handler: handler,
	}

	// TLS Config
	if opts.Cert != nil {
		httpServer.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{*opts.Cert},
		}
	} else if _, err := os.Stat("certs/cert.pem"); err == nil {
		// Only load certs if not provided in opts and files exist
		httpServer.TLSConfig = &tls.Config{
			// Certificates will be loaded by ListenAndServeTLS
		}
	}

	// Start Server
	go func() {
		var err error
		if opts.Listener != nil {
			if httpServer.TLSConfig != nil {
				log.Printf("Starting HTTPS server on provided listener %s...", opts.Listener.Addr())
				err = httpServer.ServeTLS(opts.Listener, "", "")
			} else {
				log.Printf("Starting HTTP server on provided listener %s...", opts.Listener.Addr())
				err = httpServer.Serve(opts.Listener)
			}
		} else {
			// Legacy/Default path
			log.Printf("Server starting on port %s...\n", opts.Addr)
			if opts.Cert != nil {
				err = httpServer.ListenAndServeTLS("", "")
			} else if _, statErr := os.Stat("certs/cert.pem"); statErr == nil {
				log.Println("Starting HTTPS server using certs/cert.pem...")
				err = httpServer.ListenAndServeTLS("certs/cert.pem", "certs/key.pem")
			} else {
				log.Println("Starting HTTP server...")
				err = httpServer.ListenAndServe()
			}
		}

		if err != nil && !errors.Is(err, net.ErrClosed) && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	return &Server{
			httpServer: httpServer,
			raftMgr:    raftMgr,
		},
		nil
}

// NewServerHandler creates and configures the HTTP handler for the server.
func NewServerHandler(opts Options) (*RaftManager, http.Handler) {
	if opts.DataDir == "" {
		opts.DataDir = "data"
	}

	if opts.Storage == nil {
		opts.Storage = storage.New(opts.DataDir, nil)
	}

	store := opts.GameStore
	if store == nil {
		store = NewGameStore(opts.DataDir, opts.Storage)
	}
	tStore := opts.TeamStore
	if tStore == nil {
		tStore = NewTeamStore(opts.DataDir, opts.Storage)
	}

	registry := opts.Registry
	if registry == nil {
		registry = NewRegistry(store, tStore)
	}

	accessControl := NewAccessControl(registry, opts.BootstrapAdmin)

	var raftMgr *RaftManager
	hm := NewHubManager()

	if opts.RaftEnabled {
		if opts.RaftManager != nil {
			raftMgr = opts.RaftManager
		} else {
			raftDataDir := filepath.Join(opts.DataDir, "raft")
			if err := os.MkdirAll(raftDataDir, 0755); err != nil {
				log.Fatalf("Failed to create Raft data directory: %v", err)
			}
			raftStorage := storage.New(raftDataDir, opts.MasterKey)
			fsm := NewFSM(store, tStore, registry, hm, raftStorage)

			raftMgr = NewRaftManager(raftDataDir, opts.RaftBind, opts.RaftAdvertise, opts.ClusterAdvertise, opts.ClusterAddr, opts.RaftSecret, opts.MasterKey, fsm)
			raftMgr.UseProductionTimeouts = opts.UseProductionTimeouts

			if opts.UseMockAuth {
				raftMgr.AuthMiddleware = func(next http.Handler) http.Handler {
					return mockAuthMiddleware(opts, next)
				}
			} else {
				raftMgr.AuthMiddleware = func(next http.Handler) http.Handler {
					return jwtAuthMiddleware(opts, next)
				}
			}
		}

		if opts.RaftManagerChan != nil {
			go func() { opts.RaftManagerChan <- raftMgr }()
		}
		hm.SetRaftManager(raftMgr)
	}

	debugf := func(string, ...any) {}
	if opts.Debug {
		debugf = func(f string, a ...any) {
			log.Printf("[DEBUG BACKEND] "+f, a...)
		}
	}
	mux := http.NewServeMux()

	// Cluster Dashboard
	mux.HandleFunc("/api/cluster", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(clusterDashboardHTML)
	})

	mux.HandleFunc("/api/cluster/script.js", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.Write(clusterDashboardJS)
	})

	// Admin Dashboard
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		userId := getUserID(r)
		if !accessControl.IsAdmin(userId) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(adminDashboardHTML)
	})

	mux.HandleFunc("/api/admin/script.js", func(w http.ResponseWriter, r *http.Request) {
		userId := getUserID(r)
		if !accessControl.IsAdmin(userId) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.Write(adminDashboardJS)
	})

	// Cluster Join Handler (Public API - Secured by Secret)
	mux.HandleFunc("/api/cluster/join", func(w http.ResponseWriter, r *http.Request) {
		if raftMgr == nil {
			http.Error(w, "Raft is not enabled on this node", http.StatusBadRequest)
			return
		}
		raftMgr.handleJoin(w, r)
	})
	// Cluster Leave/Remove Handler (Public API - Secured by Secret)
	mux.HandleFunc("/api/cluster/remove", func(w http.ResponseWriter, r *http.Request) {
		if raftMgr == nil {
			http.Error(w, "Raft is not enabled on this node", http.StatusBadRequest)
			return
		}
		raftMgr.handleRemove(w, r)
	})
	// Cluster Status Handler (Public/Protected)
	mux.HandleFunc("/api/cluster/status", func(w http.ResponseWriter, r *http.Request) {
		if raftMgr == nil || !opts.RaftEnabled {
			http.Error(w, "Raft is not enabled on this node", http.StatusNotImplemented)
			return
		}
		raftMgr.handleStatus(w, r)
	})

	// Admin API - Get/Update Policy
	mux.HandleFunc("/api/admin/policy", func(w http.ResponseWriter, r *http.Request) {
		userId := getUserID(r)
		if !accessControl.IsAdmin(userId) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		if r.Method == http.MethodGet {
			policy := registry.GetAccessPolicy()
			if policy == nil {
				policy = &UserAccessPolicy{
					DefaultPolicy:   "allow",
					DefaultMaxTeams: 0,
					DefaultMaxGames: 0,
					Admins:          []string{},
					Users:           make(map[string]UserOverride),
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(policy)
			return
		}

		if r.Method == http.MethodPost {
			var newPolicy UserAccessPolicy
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&newPolicy); err != nil {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}

			// Normalize user emails to lowercase to ensure case-insensitive matching
			normalizedUsers := make(map[string]UserOverride)
			for email, override := range newPolicy.Users {
				normalizedUsers[strings.ToLower(email)] = override
			}
			newPolicy.Users = normalizedUsers

			if newPolicy.DefaultPolicy != "allow" && newPolicy.DefaultPolicy != "deny" {
				http.Error(w, "Invalid default policy", http.StatusBadRequest)
				return
			}

			if raftMgr != nil {
				cmd := RaftCommand{
					Type:       CmdUpdateAccessPolicy,
					PolicyData: &newPolicy,
				}
				if _, err := raftMgr.Propose(cmd); err != nil {
					if errors.Is(err, ErrNotLeader) {
						// Re-marshal body to forward
						body, _ := json.Marshal(newPolicy)
						r.Body = io.NopCloser(bytes.NewReader(body))
						raftMgr.forwardRequestToLeader(w, r)
						return
					}
					log.Printf("Raft Propose Error: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			} else {
				// Standalone mode
				if opts.Storage != nil {
					if err := opts.Storage.SaveDataFile("sys_access_policy", &newPolicy); err != nil {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						return
					}
				}
				registry.UpdateAccessPolicy(&newPolicy)
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	// User Status & Quota Endpoint
	mux.HandleFunc("/api/me", func(w http.ResponseWriter, r *http.Request) {
		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Unauthenticated", http.StatusForbidden)
			return
		}

		allowed, msg := accessControl.IsAllowed(userId)
		maxGames, maxTeams := accessControl.GetUserQuotas(userId)
		ownedGames := registry.CountOwnedGames(userId)
		ownedTeams := registry.CountOwnedTeams(userId)

		resp := map[string]interface{}{
			"id":      userId,
			"allowed": allowed,
			"message": msg,
			"quotas": map[string]int{
				"maxGames":  maxGames,
				"maxTeams":  maxTeams,
				"gamesUsed": ownedGames,
				"teamsUsed": ownedTeams,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Unauthenticated", http.StatusForbidden)
			return
		}
		if allowed, msg := accessControl.IsAllowed(userId); !allowed {
			http.Error(w, "Forbidden: "+msg, http.StatusForbidden)
			return
		}

		var msg Message
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&msg); err != nil {
			http.Error(w, "Bad Request: Malformed JSON", http.StatusBadRequest)
			return
		}

		gameId := msg.GameId
		if gameId == "" || !isValidUUID(gameId) {
			// Fallback: check query param (though typically payload should have it)
			gameId = r.URL.Query().Get("gameId")
			if gameId == "" || !isValidUUID(gameId) {
				http.Error(w, "Bad Request: gameId is missing or invalid", http.StatusBadRequest)
				return
			}
			msg.GameId = gameId // Ensure it's in the message
		}

		// Serialize through Hub
		hub := hm.GetHub(gameId, false, store, tStore, registry)
		reply := make(chan HubResponse, 1)
		select {
		case hub.requests <- HubRequest{
			Type:    ReqTypeHTTPAction,
			UserId:  userId,
			Headers: r.Header,
			Message: msg,
			Reply:   reply,
		}:
			select {
			case resp := <-reply:
				if resp.Error != nil {
					log.Printf("Error processing HTTP action: %v", resp.Error)
					// Map specific errors to HTTP codes if possible, otherwise 500
					// Currently hub returns generic errors, maybe improve later.
					// If error string contains "Forbidden", return 403
					// If "Conflict", return 409
					errStr := resp.Error.Error()
					if strings.Contains(errStr, "Forbidden") {
						http.Error(w, errStr, http.StatusForbidden)
					} else if strings.Contains(errStr, "Conflict") {
						http.Error(w, errStr, http.StatusConflict)
					} else if strings.Contains(errStr, "Unauthenticated") {
						http.Error(w, errStr, http.StatusForbidden)
					} else {
						http.Error(w, errStr, http.StatusInternalServerError)
					}
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.Write(resp.Data)
			case <-r.Context().Done():
				return
			}
		default:
			hubBusyResponse(w, retryAfterAction)
		}
	})

	mux.HandleFunc("/api/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Forbidden: Invalid User ID", http.StatusForbidden)
			return
		}

		if allowed, msg := accessControl.IsAllowed(userId); !allowed {
			http.Error(w, "Forbidden: "+msg, http.StatusForbidden)
			return
		}

		var g Game
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 20*1048576)).Decode(&g); err != nil {
			http.Error(w, "Bad Request: Malformed JSON", http.StatusBadRequest)
			return
		}

		gameId := g.ID
		if gameId == "" || !isValidUUID(gameId) {
			http.Error(w, "Bad Request: gameId is missing or invalid", http.StatusBadRequest)
			return
		}

		// Authorization Check
		existingGame, err := store.LoadGame(gameId)
		if err == nil {
			// Updating existing game
			level := GetGameAccess(userId, *existingGame, tStore)
			if level < AccessWrite {
				http.Error(w, "Forbidden: You do not have write access to this game", http.StatusForbidden)
				return
			}
			// Enforce existing ownership
			g.OwnerID = existingGame.OwnerID
		} else if errors.Is(err, os.ErrNotExist) {
			// New game: Set owner to current user
			g.OwnerID = userId

			// Quota Check
			ownedCount := registry.CountOwnedGames(userId)
			if err := accessControl.CheckGameQuota(userId, ownedCount); err != nil {
				http.Error(w, "Forbidden: "+err.Error(), http.StatusForbidden)
				return
			}
		} else {
			log.Printf("Error checking existing game %s: %v", gameId, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Enforce Schema Version
		g.SchemaVersion = SchemaVersionV3

		// Re-marshal to bytes for validation and storage
		body, err := json.Marshal(g)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Validate the entire game data structure
		if err := ValidateGameData(body); err != nil {
			log.Printf("Validation error for game %s: %v", gameId, err)
			http.Error(w, fmt.Sprintf("Bad Request: Data validation failed: %v", err), http.StatusBadRequest)
			return
		}

		// Serialize through Hub
		force := r.URL.Query().Get("force") == "true"
		hub := hm.GetHub(gameId, false, store, tStore, registry)
		reply := make(chan HubResponse, 1)
		select {
		case hub.requests <- HubRequest{
			Type:    ReqTypeHTTPSave,
			Payload: body,
			Reply:   reply,
			Force:   force, // Add Force field to HubRequest
		}:
			select {
			case resp := <-reply:
				if resp.Error != nil {
					if errors.Is(resp.Error, ErrNotLeader) && raftMgr != nil {
						r.Body = io.NopCloser(bytes.NewReader(body))
						raftMgr.forwardRequestToLeader(w, r)
						return
					}
					log.Printf("Internal Server Error during Hub Save: %v", resp.Error)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "Game %s saved successfully", gameId)
			case <-r.Context().Done():
				return
			}
		default:
			hubBusyResponse(w, retryAfterSave)
		}
	})

	mux.HandleFunc("/api/load/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		userId := getUserID(r)

		if userId != "" {
			if allowed, msg := accessControl.IsAllowed(userId); !allowed {
				http.Error(w, "Forbidden: "+msg, http.StatusForbidden)
				return
			}
		}

		gameId := strings.TrimPrefix(r.URL.Path, "/api/load/")
		if gameId == "" || !isValidUUID(gameId) {
			http.Error(w, "Bad Request: gameId is missing or invalid", http.StatusBadRequest)
			return
		}

		// Serialize through Hub
		hub := hm.GetHub(gameId, false, store, tStore, registry)
		reply := make(chan HubResponse, 1)
		select {
		case hub.requests <- HubRequest{
			Type:  ReqTypeHTTPLoad,
			Reply: reply,
		}:
			select {
			case resp := <-reply:
				if resp.Error != nil {
					if os.IsNotExist(resp.Error) {
						http.Error(w, "Not Found: Game not found", http.StatusNotFound)
					} else {
						log.Printf("Internal Server Error during Hub Load: %v", resp.Error)
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
					return
				}
				data := resp.Data

				// Authorization Check
				var g Game
				if err := json.Unmarshal(data, &g); err != nil {
					log.Printf("Error unmarshaling game data for auth check: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				if GetGameAccess(userId, g, tStore) < AccessRead {
					http.Error(w, "Forbidden: You do not have access to this game", http.StatusForbidden)
					return
				}

				etag := generateETag(data)
				if r.Header.Get("If-None-Match") == etag {
					w.WriteHeader(http.StatusNotModified)
					return
				}

				w.Header().Set("ETag", etag)
				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
			case <-r.Context().Done():
				return
			}
		default:
			hubBusyResponse(w, retryAfterLoad)
		}
	})

	mux.HandleFunc("/api/list-games", func(w http.ResponseWriter, r *http.Request) {
		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Forbidden: Invalid User ID", http.StatusForbidden)
			return
		}

		if allowed, msg := accessControl.IsAllowed(userId); !allowed {
			http.Error(w, "Forbidden: "+msg, http.StatusForbidden)
			return
		}

		var knownIds []string
		if r.Method == http.MethodPost {
			var body struct {
				KnownIds []string `json:"knownIds"`
			}
			// Ignore error on decode if body empty, just treat as empty list
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&body); err == nil {
				knownIds = body.KnownIds
			}
		} else if r.Method != http.MethodGet {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		limit, offset, sortBy, order, query := parsePagination(r)
		accessibleIds := registry.ListGames(userId, sortBy, order, query)
		total := len(accessibleIds)

		// Pagination Logic
		var pageIds []string
		if offset < total {
			end := offset + limit
			if end > total {
				end = total
			}
			pageIds = accessibleIds[offset:end]
		}

		games := make([]GameSummary, 0)

		for _, gid := range pageIds {
			gf, err := store.LoadGame(gid)
			if err != nil {
				continue
			}

			revision := ""
			if len(gf.ActionLog) > 0 {
				var lastAction struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(gf.ActionLog[len(gf.ActionLog)-1], &lastAction); err == nil {
					revision = lastAction.ID
				}
			}

			games = append(games, GameSummary{
				ID:       gf.ID,
				Date:     gf.Date,
				Location: gf.Location,
				Event:    gf.Event,
				Away:     gf.Away,
				Home:     gf.Home,
				Revision: revision,
				Status:   gf.Status,
				OwnerID:  gf.OwnerID,
			})
		}

		// Check for deleted games among known IDs
		for _, kid := range knownIds {
			if registry.IsGameDeleted(kid) {
				// Add tombstone summary
				games = append(games, GameSummary{
					ID:     kid,
					Status: "deleted",
				})
			}
		}

		respData := struct {
			Data []GameSummary `json:"data"`
			Meta struct {
				Total  int `json:"total"`
				Offset int `json:"offset"`
				Limit  int `json:"limit"`
			} `json:"meta"`
		}{
			Data: games,
		}
		respData.Meta.Total = total
		respData.Meta.Offset = offset
		respData.Meta.Limit = limit

		response, err := json.Marshal(respData)
		if err != nil {
			log.Printf("Internal Server Error during JSON Marshal: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// ETag handling (skip for POST as it depends on body content which ETag logic here doesn't account for well,
		// or just generate ETag based on response content which is fine)
		etag := generateETag(response)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	})

	mux.HandleFunc("/api/save-team", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Forbidden: Invalid User ID", http.StatusForbidden)
			return
		}

		if allowed, msg := accessControl.IsAllowed(userId); !allowed {
			http.Error(w, "Forbidden: "+msg, http.StatusForbidden)
			return
		}

		var t Team
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&t); err != nil {
			http.Error(w, "Bad Request: Malformed JSON", http.StatusBadRequest)
			return
		}

		teamId := t.ID
		if teamId == "" || !isValidUUID(teamId) {
			http.Error(w, "Bad Request: teamId is missing or invalid", http.StatusBadRequest)
			return
		}

		// Authorization Check
		existingTeam, err := tStore.LoadTeam(teamId)
		if err == nil {
			if GetTeamAccess(userId, *existingTeam) < AccessWrite {
				http.Error(w, "Forbidden: You do not have permission to manage this team", http.StatusForbidden)
				return
			}
			// Enforce existing ownership
			t.OwnerID = existingTeam.OwnerID
		} else if errors.Is(err, os.ErrNotExist) {
			// New team: set owner to current user
			t.OwnerID = userId

			// Quota Check
			ownedCount := registry.CountOwnedTeams(userId)
			if err := accessControl.CheckTeamQuota(userId, ownedCount); err != nil {
				http.Error(w, "Forbidden: "+err.Error(), http.StatusForbidden)
				return
			}
		}

		// Enforce Schema Version
		t.SchemaVersion = SchemaVersionV3

		// Re-marshal to enforce server-side fields
		body, err := json.Marshal(t)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Serialize through Hub
		hub := hm.GetHub(teamId, true, store, tStore, registry)
		reply := make(chan HubResponse, 1)
		select {
		case hub.requests <- HubRequest{
			Type:    ReqTypeHTTPSave,
			Payload: body,
			Reply:   reply,
		}:
			select {
			case resp := <-reply:
				if resp.Error != nil {
					if errors.Is(resp.Error, ErrNotLeader) && raftMgr != nil {
						r.Body = io.NopCloser(bytes.NewReader(body))
						raftMgr.forwardRequestToLeader(w, r)
						return
					}
					log.Printf("Internal Server Error during Hub SaveTeam: %v", resp.Error)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "Team %s saved successfully", teamId)
			case <-r.Context().Done():
				return
			}
		default:
			hubBusyResponse(w, retryAfterSave)
		}
	})

	mux.HandleFunc("/api/list-teams", func(w http.ResponseWriter, r *http.Request) {
		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Forbidden: Invalid User ID", http.StatusForbidden)
			return
		}

		if allowed, msg := accessControl.IsAllowed(userId); !allowed {
			http.Error(w, "Forbidden: "+msg, http.StatusForbidden)
			return
		}

		var knownIds []string
		if r.Method == http.MethodPost {
			var body struct {
				KnownIds []string `json:"knownIds"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&body); err == nil {
				knownIds = body.KnownIds
			}
		} else if r.Method != http.MethodGet {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		limit, offset, sortBy, order, query := parsePagination(r)
		accessibleIds := registry.ListTeams(userId, sortBy, order, query)
		total := len(accessibleIds)

		// Pagination Logic
		var pageIds []string
		if offset < total {
			end := offset + limit
			if end > total {
				end = total
			}
			pageIds = accessibleIds[offset:end]
		}

		teams := make([]json.RawMessage, 0)

		for _, tid := range pageIds {
			t, err := tStore.LoadTeam(tid)
			if err != nil {
				continue
			}
			// Marshalling struct to JSON for list response
			data, _ := json.Marshal(t)
			teams = append(teams, json.RawMessage(data))
		}

		// Check for deleted teams
		for _, kid := range knownIds {
			if registry.IsTeamDeleted(kid) {
				// Minimal tombstone json
				tombstone := map[string]string{
					"id":     kid,
					"status": "deleted",
				}
				data, _ := json.Marshal(tombstone)
				teams = append(teams, json.RawMessage(data))
			}
		}

		respData := struct {
			Data []json.RawMessage `json:"data"`
			Meta struct {
				Total  int `json:"total"`
				Offset int `json:"offset"`
				Limit  int `json:"limit"`
			} `json:"meta"`
		}{
			Data: teams,
		}
		respData.Meta.Total = total
		respData.Meta.Offset = offset
		respData.Meta.Limit = limit

		response, err := json.Marshal(respData)
		if err != nil {
			log.Printf("Internal Server Error during JSON Marshal: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		etag := generateETag(response)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	})

	mux.HandleFunc("/api/load-team/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		userId := getUserID(r)
		if allowed, msg := accessControl.IsAllowed(userId); !allowed {
			http.Error(w, "Forbidden: "+msg, http.StatusForbidden)
			return
		}

		teamId := strings.TrimPrefix(r.URL.Path, "/api/load-team/")
		if teamId == "" || !isValidUUID(teamId) {
			http.Error(w, "Bad Request: teamId is missing or invalid", http.StatusBadRequest)
			return
		}

		// Serialize through Hub
		hub := hm.GetHub(teamId, true, store, tStore, registry)
		reply := make(chan HubResponse, 1)
		select {
		case hub.requests <- HubRequest{
			Type:  ReqTypeHTTPLoad,
			Reply: reply,
		}:
			select {
			case resp := <-reply:
				if resp.Error != nil {
					if os.IsNotExist(resp.Error) {
						http.Error(w, "Not Found: Team not found", http.StatusNotFound)
					} else {
						log.Printf("Internal Server Error during Hub LoadTeam: %v", resp.Error)
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
					return
				}
				data := resp.Data

				// Authorization Check
				var t Team
				if err := json.Unmarshal(data, &t); err != nil {
					log.Printf("Error unmarshaling team data for auth check: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				if GetTeamAccess(userId, t) < AccessRead {
					http.Error(w, "Forbidden: You do not have access to this team", http.StatusForbidden)
					return
				}

				etag := generateETag(data)
				if r.Header.Get("If-None-Match") == etag {
					w.WriteHeader(http.StatusNotModified)
					return
				}

				w.Header().Set("ETag", etag)
				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
			case <-r.Context().Done():
				return
			}
		default:
			hubBusyResponse(w, retryAfterLoad)
		}
	})

	mux.HandleFunc("/api/delete-team", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Forbidden: Invalid User ID", http.StatusForbidden)
			return
		}

		var data struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&data); err != nil {
			http.Error(w, "Bad Request: Malformed JSON", http.StatusBadRequest)
			return
		}

		teamId := data.ID
		if teamId == "" || !isValidUUID(teamId) {
			http.Error(w, "Bad Request: teamId is missing or invalid", http.StatusBadRequest)
			return
		}

		// Authorization Check
		existingTeam, err := tStore.LoadTeam(teamId)
		if err == nil {
			if GetTeamAccess(userId, *existingTeam) < AccessAdmin {
				http.Error(w, "Forbidden: You do not have permission to delete this team", http.StatusForbidden)
				return
			}
		}

		if err := tStore.DeleteTeam(teamId); err != nil {
			log.Printf("Internal Server Error during DeleteTeam: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Update registry
		registry.DeleteTeam(teamId)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Team %s deleted successfully", teamId)
	})

	mux.HandleFunc("/api/team/members", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		var req struct {
			TeamId string    `json:"teamId"`
			Roles  TeamRoles `json:"roles"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Serialize through Hub
		hub := hm.GetHub(req.TeamId, true, store, tStore, registry)
		reply := make(chan HubResponse, 1)
		select {
		case hub.requests <- HubRequest{
			Type:  ReqTypeHTTPLoad,
			Reply: reply,
		}:
			select {
			case resp := <-reply:
				if resp.Error != nil {
					if os.IsNotExist(resp.Error) {
						http.Error(w, "Not Found", http.StatusNotFound)
					} else {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
					return
				}

				var t Team
				if err := json.Unmarshal(resp.Data, &t); err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				if GetTeamAccess(userId, t) < AccessAdmin {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}

				// Update Roles
				t.Roles = req.Roles
				updatedBytes, _ := json.Marshal(t)

				replySave := make(chan HubResponse, 1)
				select {
				case hub.requests <- HubRequest{
					Type:    ReqTypeHTTPSave,
					Payload: updatedBytes,
					Reply:   replySave,
				}:
					select {
					case respSave := <-replySave:
						if respSave.Error != nil {
							http.Error(w, "Internal Server Error", http.StatusInternalServerError)
							return
						}

						w.WriteHeader(http.StatusOK)
					case <-r.Context().Done():
						return
					}
				default:
					hubBusyResponse(w, retryAfterSave)
				}
			case <-r.Context().Done():
				return
			}
		default:
			hubBusyResponse(w, retryAfterLoad)
		}
	})

	mux.HandleFunc("/api/delete-game", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Forbidden: Invalid User ID", http.StatusForbidden)
			return
		}

		var data struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&data); err != nil {
			http.Error(w, "Bad Request: Malformed JSON", http.StatusBadRequest)
			return
		}

		gameId := data.ID
		if gameId == "" || !isValidUUID(gameId) {
			http.Error(w, "Bad Request: gameId is missing or invalid", http.StatusBadRequest)
			return
		}

		// Authorization Check
		g, err := store.LoadGame(gameId)
		if err == nil {
			if GetGameAccess(userId, *g, tStore) < AccessAdmin {
				http.Error(w, "Forbidden: Only the owner can delete this game", http.StatusForbidden)
				return
			}
		}

		if err := store.DeleteGame(gameId); err != nil {
			log.Printf("Internal Server Error during DeleteGame: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		registry.DeleteGame(gameId)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Game %s deleted successfully", gameId)
	})

	mux.HandleFunc("/api/check-deletions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		userId := getUserID(r)
		if userId == "" || !isValidEmail(userId) {
			http.Error(w, "Forbidden: Invalid User ID", http.StatusForbidden)
			return
		}

		if allowed, msg := accessControl.IsAllowed(userId); !allowed {
			http.Error(w, "Forbidden: "+msg, http.StatusForbidden)
			return
		}

		var req struct {
			GameIDs []string `json:"gameIds"`
			TeamIDs []string `json:"teamIds"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1048576)).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var resp struct {
			DeletedGameIDs []string `json:"deletedGameIds"`
			DeletedTeamIDs []string `json:"deletedTeamIds"`
		}
		resp.DeletedGameIDs = make([]string, 0)
		resp.DeletedTeamIDs = make([]string, 0)

		for _, gid := range req.GameIDs {
			// Report as deleted if explicitly tombstoned OR if it exists but is no longer accessible
			if registry.IsGameDeleted(gid) || (registry.GameExists(gid) && !registry.HasGameAccess(userId, gid)) {
				resp.DeletedGameIDs = append(resp.DeletedGameIDs, gid)
			}
		}
		for _, tid := range req.TeamIDs {
			if registry.IsTeamDeleted(tid) || (registry.TeamExists(tid) && !registry.HasTeamAccess(userId, tid)) {
				resp.DeletedTeamIDs = append(resp.DeletedTeamIDs, tid)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		userId := getUserID(r)
		if userId != "" {
			if allowed, msg := accessControl.IsAllowed(userId); !allowed {
				http.Error(w, "Forbidden: "+msg, http.StatusForbidden)
				return
			}
		}
		ServeWS(store, tStore, registry, hm, w, r, debugf)
	})

	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if opts.UseMockAuth {
			http.SetCookie(w, &http.Cookie{
				Name:  "mock_auth_user",
				Value: "test@example.com",
				Path:  "/",
			})
		} else if userId := getUserID(r); userId == "" || !isValidEmail(userId) {
			http.Error(w, "Forbidden: Invalid User ID", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><head><script src='/login-success.js'></script></head><body>Login successful. Closing window...</body></html>"))
	})

	// Mock SSO endpoints for local development
	if opts.UseMockAuth {
		mux.HandleFunc("/.sso/{$}", func(w http.ResponseWriter, r *http.Request) {
			ssoStatusHandler(registry, w, r)
		})
		mux.HandleFunc("/.sso/logout", ssoLogoutHandler)
	}

	// Serve embedded frontend
	contentStatic, err := fs.Sub(frontend.FS, ".")
	if err != nil {
		log.Fatal(err)
	}
	var fs http.Handler = http.FileServerFS(contentStatic)

	if opts.MinifyMode {
		originalFS := fs
		fs = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" || r.URL.Path == "/index.html" {
				r.URL.Path = "/dist/index.min.html"
			} else if r.URL.Path == "/sw.js" {
				r.URL.Path = "/dist/sw.js"
			}
			originalFS.ServeHTTP(w, r)
		})
	}

	mux.Handle("/", contentTypeMiddleware(fs))

	handler := http.Handler(mux)
	if opts.UseMockAuth {
		handler = mockAuthMiddleware(opts, handler)
	} else {
		handler = jwtAuthMiddleware(opts, handler)
	}
	handler = loggingMiddleware(handler)
	handler = securityMiddleware(handler)
	handler = cacheControlMiddleware(handler)

	if raftMgr != nil {
		raftMgr.AppHandler = handler
		if err := raftMgr.Start(opts.RaftBootstrap); err != nil {
			log.Fatalf("Failed to start Raft: %v", err)
		}
	}

	return raftMgr, handler
}

// cacheControlMiddleware adds Cache-Control headers optimized for PWA reliability behind a proxy.
func cacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/.sso/") {
			w.Header().Set("Cache-Control", "private, no-cache, no-transform")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=300, proxy-revalidate, no-transform")
		}
		next.ServeHTTP(w, r)
	})
}

// securityMiddleware adds HTTP security headers to responses.
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content-Security-Policy: restrict sources to 'self' and allow inline scripts/styles (for now)
		// We might need to adjust this for images or external resources if used.
		// w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:")
		// NOTE: Strict CSP might break the current app if it relies on inline event handlers or styles heavily.
		// Given the prototype nature, I'll use a slightly permissive one but still useful.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// mockAuthMiddleware simulates TLSProxy by checking for a cookie and setting the UserID context.
func mockAuthMiddleware(opts Options, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieName := "mock_auth_user"
		cookie, err := r.Cookie(cookieName)
		if err == nil && cookie.Value != "" {
			// Simulate TLSProxy adding the UserID from a cookie
			ctx := context.WithValue(r.Context(), userIDKey, normalizeEmail(cookie.Value))
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ssoStatusHandler returns the current user status.
func ssoStatusHandler(registry *Registry, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	userId := getUserID(r)
	if userId == "" {
		w.Write([]byte("null\n"))
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"email": userId,
		"name":  "Test User",
	})
}

// ssoLogoutHandler logs the user out (clears cookie).
func ssoLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "mock_auth_user",
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
		MaxAge:  -1,
	})
	w.WriteHeader(http.StatusOK)
}

// contentTypeMiddleware ensures that files are served with the correct MIME type.
func contentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := filepath.Ext(r.URL.Path)
		switch ext {
		case ".js", ".mjs":
			w.Header().Set("Content-Type", "application/javascript")
		case ".css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case ".html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".json":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs the method and URL path of every incoming HTTP request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
