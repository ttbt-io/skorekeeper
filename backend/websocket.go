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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/raft"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == r.Host
	},
}

// Message types for WebSocket communication
const (
	MsgTypeJoin       = "JOIN"
	MsgTypeAck        = "ACK"
	MsgTypeAction     = "ACTION"
	MsgTypeSyncUpdate = "SYNC_UPDATE"
	MsgTypeConflict   = "CONFLICT"
	MsgTypeError      = "ERROR"
)

// Message represents a WebSocket message
type Message struct {
	Type         string            `json:"type"`
	GameId       string            `json:"gameId,omitempty"`
	LastRevision string            `json:"lastRevision,omitempty"`
	BaseRevision string            `json:"baseRevision,omitempty"`
	Action       json.RawMessage   `json:"action,omitempty"`
	Actions      []json.RawMessage `json:"actions,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// HubRequest types
const (
	ReqTypeWSJoin     = "WS_JOIN"
	ReqTypeHTTPLoad   = "HTTP_LOAD"
	ReqTypeHTTPSave   = "HTTP_SAVE"
	ReqTypeHTTPAction = "HTTP_ACTION"
	ReqTypeBroadcast  = "BROADCAST"
)

// HubRequest represents a request to the Hub
type HubRequest struct {
	Type          string
	Client        *wsClient        // For WS requests
	UserId        string           // For HTTP requests
	Headers       http.Header      // For forwarding cookies/auth
	Message       Message          // For WS/HTTP requests
	Payload       []byte           // For HTTP Save/Broadcast
	SkipBroadcast bool             // For Broadcast (overwrites)
	NumActions    int              // For Broadcast: number of actions to broadcast from the end
	Reply         chan HubResponse // For HTTP requests (and potentially WS sync)
}

// HubResponse represents a response from the Hub
type HubResponse struct {
	Data  []byte // For HTTP Load
	Error error  // For HTTP Save/Load errors
}

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	resourceId string
	isTeam     bool // True if this hub manages a team, false for a game

	// Registered clients.
	clients map[*wsClient]bool

	// Inbound requests
	requests chan HubRequest

	// Register requests from the clients.
	register chan *wsClient

	// Unregister requests from clients.
	unregister chan *wsClient

	// In-memory state
	gameData *Game
	teamData *Team

	// Throttling for conflicts
	lastConflict map[string]time.Time // userId -> last conflict sent
	conflictMu   sync.Mutex

	gs *GameStore
	ts *TeamStore
	r  *Registry
	hm *HubManager
	rm *RaftManager
}

func newHub(id string, isTeam bool, gs *GameStore, ts *TeamStore, r *Registry, hm *HubManager, rm *RaftManager) *Hub {
	return &Hub{
		resourceId:   id,
		isTeam:       isTeam,
		requests:     make(chan HubRequest, 64), // Buffered to prevent dropping FSM updates
		register:     make(chan *wsClient),
		unregister:   make(chan *wsClient),
		clients:      make(map[*wsClient]bool),
		lastConflict: make(map[string]time.Time),
		gs:           gs,
		ts:           ts,
		r:            r,
		hm:           hm,
		rm:           rm,
	}
}

func (h *Hub) run() {
	idleTimer := time.NewTicker(5 * time.Minute)
	defer idleTimer.Stop()

	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case req := <-h.requests:
			if h.isTeam {
				h.ensureTeamLoaded(req.Reply)
			} else {
				h.ensureLoaded(req.Reply)
			}

			// If loading failed, stop processing.
			if (h.isTeam && h.teamData == nil) || (!h.isTeam && h.gameData == nil) {
				if req.Client != nil {
					req.Client.sendJSON(Message{Type: MsgTypeError, Error: "Server error loading resource"})
				}
				continue
			}

			switch req.Type {
			case ReqTypeWSJoin:
				if req.Client != nil && !h.clients[req.Client] {
					continue
				}
				if !h.isTeam {
					h.handleWSJoin(req.Client, req.Message)
				}
			case ReqTypeHTTPAction:
				if !h.isTeam {
					h.handleHTTPAction(req)
				}
			case ReqTypeHTTPLoad:
				h.handleHTTPLoad(req.Reply)
			case ReqTypeHTTPSave:
				h.handleHTTPSave(req.Payload, req.Reply)
			case ReqTypeBroadcast:
				h.handleBroadcast(req.Payload, req.SkipBroadcast, req.NumActions)
			}
		case <-idleTimer.C:
			if len(h.clients) == 0 {
				h.hm.RemoveHub(h.resourceId, h.isTeam)
				return
			}
		}
	}
}

func (h *Hub) handleBroadcast(data []byte, skipBroadcast bool, numActions int) {
	var g Game
	if err := json.Unmarshal(data, &g); err != nil {
		log.Printf("handleBroadcast: Error unmarshaling game data: %v", err)
		return
	}

	// Update Hub's in-memory state
	h.gameData = &g

	if skipBroadcast {
		return
	}

	if numActions <= 0 {
		numActions = 1
	}

	// Extract actions to broadcast
	if len(g.ActionLog) > 0 {
		if numActions > len(g.ActionLog) {
			numActions = len(g.ActionLog)
		}
		actions := g.ActionLog[len(g.ActionLog)-numActions:]
		for _, action := range actions {
			h.broadcast(Message{Type: MsgTypeAction, Action: action})
		}
	}
}

func (h *Hub) ensureLoaded(reply chan HubResponse) {
	if h.gameData != nil {
		return
	}
	// gs.LoadGame now returns *Game
	g, err := h.gs.LoadGame(h.resourceId)
	if err != nil {
		if os.IsNotExist(err) {
			h.gameData = &Game{ID: h.resourceId}
			return
		}
		log.Printf("Hub: Error loading game %s: %v", h.resourceId, err)
		if reply != nil {
			reply <- HubResponse{Error: err}
		}
		return
	}
	h.gameData = g
}

func (h *Hub) ensureTeamLoaded(reply chan HubResponse) {
	if h.teamData != nil {
		return
	}
	// ts.LoadTeam now returns *Team
	t, err := h.ts.LoadTeam(h.resourceId)
	if err != nil {
		if os.IsNotExist(err) {
			h.teamData = &Team{ID: h.resourceId}
			return
		}
		log.Printf("Hub: Error loading team %s: %v", h.resourceId, err)
		if reply != nil {
			reply <- HubResponse{Error: err}
		}
		return
	}
	h.teamData = t
}

// HubManager manages hubs for different games/teams
type HubManager struct {
	hubs map[string]*Hub
	mu   sync.Mutex
	rm   *RaftManager
}

func NewHubManager() *HubManager {
	return &HubManager{
		hubs: make(map[string]*Hub),
	}
}

func (hm *HubManager) SetRaftManager(rm *RaftManager) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.rm = rm
}

func (hm *HubManager) GetHub(id string, isTeam bool, gs *GameStore, ts *TeamStore, r *Registry) *Hub {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	key := id
	if isTeam {
		key = "team:" + id
	}

	if hub, ok := hm.hubs[key]; ok {
		return hub
	}

	hub := newHub(id, isTeam, gs, ts, r, hm, hm.rm)
	hm.hubs[key] = hub
	go hub.run()
	return hub
}

func (hm *HubManager) RemoveHub(id string, isTeam bool) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	key := id
	if isTeam {
		key = "team:" + id
	}
	delete(hm.hubs, key)
}

func (hm *HubManager) Clear() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.hubs = make(map[string]*Hub)
}

func (hm *HubManager) BroadcastToGame(gameId string, data []byte, skipBroadcast bool, numActions int) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hub, ok := hm.hubs[gameId]
	if !ok {
		return
	}

	// Send update via channel to serialize with Hub goroutine
	select {
	case hub.requests <- HubRequest{
		Type:          ReqTypeBroadcast,
		Payload:       data,
		SkipBroadcast: skipBroadcast,
		NumActions:    numActions,
	}:
	default:
		// Handle full channel - for now, we just log it or drop it to prevent blocking Raft FSM
		log.Printf("Warning: Hub channel full, dropping broadcast for game %s", gameId)
	}
}

// var resourceHubs = NewHubManager() // Removed global

// wsClient is a middleman between the websocket connection and the hub.
type wsClient struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan Message

	userId string
	gameId string
	gs     *GameStore
	ts     *TeamStore
	r      *Registry
}

// readPump pumps messages from the websocket connection to the hub.
func (c *wsClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		switch msg.Type {
		case MsgTypeJoin:
			c.hub.requests <- HubRequest{Type: ReqTypeWSJoin, Client: c, Message: msg}
		case "PING":
			c.sendJSON(Message{Type: "PONG"})
		default:
			log.Printf("Unknown message type: %s", msg.Type)
			c.sendJSON(Message{Type: MsgTypeError, Error: "Unknown message type"})
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *wsClient) sendJSON(msg Message) {
	select {
	case c.send <- msg:
	default:
		// Channel full, drop connection?
	}
}

func (h *Hub) handleWSJoin(c *wsClient, msg Message) {
	// Authorization Check
	if len(h.gameData.ActionLog) > 0 || h.gameData.OwnerID != "" {
		access := GetGameAccess(c.userId, *h.gameData, h.r.teamStore)
		if access < AccessRead {
			log.Printf("Forbidden: User %s attempted to join game %s without permissions", maskEmail(c.userId), h.resourceId)
			c.sendJSON(Message{Type: MsgTypeError, Error: "Forbidden: You do not have access to this game"})
			return
		}
	} else if msg.LastRevision != "" {
		log.Printf("Conflict: Client joining game %s with revision %s, but game empty on server", h.resourceId, msg.LastRevision)
		c.sendJSON(Message{Type: MsgTypeConflict, Error: "Game not found on server", BaseRevision: ""})
		return
	}

	serverRevision := getCurrentRevision(h.gameData.ActionLog)

	if msg.LastRevision == "" || msg.LastRevision == serverRevision {
		c.sendJSON(Message{Type: MsgTypeAck})
		return
	}

	missingActions := getActionsSince(h.gameData.ActionLog, msg.LastRevision)
	if missingActions == nil && msg.LastRevision != "" {
		if len(h.gameData.ActionLog) > 0 {
			c.sendJSON(Message{Type: MsgTypeConflict, Error: "Client history is divergent from server", BaseRevision: serverRevision})
			return
		}
	}

	if missingActions == nil {
		c.sendJSON(Message{Type: MsgTypeAck})
		return
	}

	c.sendJSON(Message{Type: MsgTypeSyncUpdate, Actions: missingActions})
}

func (h *Hub) handleHTTPAction(req HubRequest) {
	response, broadcasts, err := h.processAction(req.Message, req.UserId)
	if err != nil {
		if errors.Is(err, ErrNotLeader) {
			h.forwardToLeader(req)
			return
		}
		if req.Reply != nil {
			req.Reply <- HubResponse{Error: err}
		}
		return
	}

	// For HTTP, we return the response message as Data
	data, _ := json.Marshal(response)

	for _, b := range broadcasts {
		h.broadcast(b)
	}

	if req.Reply != nil {
		req.Reply <- HubResponse{Data: data}
	}
}

func (h *Hub) forwardToLeader(req HubRequest) {
	leaderAddr := h.rm.GetLeaderHTTPAddr()

	// Prevent forwarding to self if split-brain or stale metadata
	if leaderAddr == h.rm.ClusterAdvertise {
		if req.Reply != nil {
			req.Reply <- HubResponse{Error: fmt.Errorf("local node listed as leader but not in leader state")}
		}
		return
	}

	if leaderAddr == "" {
		if req.Reply != nil {
			req.Reply <- HubResponse{Error: fmt.Errorf("leader not found")}
		}
		return
	}

	// Ensure leaderAddr has protocol
	if !strings.HasPrefix(leaderAddr, "http") {
		leaderAddr = "https://" + leaderAddr // Assumes HTTPS
	}

	url := leaderAddr + "/api/cluster/action"
	body, _ := json.Marshal(req.Message)
	forwardReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		if req.Reply != nil {
			req.Reply <- HubResponse{Error: err}
		}
		return
	}

	// Copy authentication and content headers from the original request
	for _, h := range []string{"Cookie", "Authorization", "Content-Type"} {
		if v := req.Headers.Get(h); v != "" {
			forwardReq.Header.Set(h, v)
		}
	}

	// Update X-Raft-Forwarded
	forwarded := forwardReq.Header.Get("X-Raft-Forwarded")
	if forwarded != "" {
		forwarded += "," + h.rm.NodeID
	} else {
		forwarded = h.rm.NodeID
	}
	forwardReq.Header.Set("X-Raft-Forwarded", forwarded)

	// Ensure secret is set
	if h.rm.Secret != "" {
		forwardReq.Header.Set("X-Raft-Secret", h.rm.Secret)
	}

	// Use secure mTLS transport for internal forwarding
	client := h.rm.GetHTTPClient()
	resp, err := client.Do(forwardReq)
	if err != nil {
		if req.Reply != nil {
			req.Reply <- HubResponse{Error: err}
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		if req.Reply != nil {
			req.Reply <- HubResponse{Error: fmt.Errorf("leader returned %d: %s", resp.StatusCode, string(respBody))}
		}
		return
	}

	data, err := io.ReadAll(resp.Body)
	if req.Reply != nil {
		req.Reply <- HubResponse{Data: data, Error: err}
	}
}

func (h *Hub) processAction(msg Message, userId string) (response *Message, broadcasts []Message, err error) {
	var actions []json.RawMessage
	if len(msg.Actions) > 0 {
		if len(msg.Actions) > 100 {
			return &Message{Type: MsgTypeError, Error: "Batch size too large (max 100)"}, nil, nil
		}
		actions = msg.Actions
		if err := ValidateActions(actions); err != nil {
			log.Printf("Invalid actions payload from user %s: %v", maskEmail(userId), err)
			return &Message{Type: MsgTypeError, Error: "Malformed actions: " + err.Error()}, nil, nil
		}
	} else {
		actions = []json.RawMessage{msg.Action}
		if err := ValidateAction(msg.Action); err != nil {
			log.Printf("Invalid action payload from user %s: %v", maskEmail(userId), err)
			return &Message{Type: MsgTypeError, Error: "Malformed action: " + err.Error()}, nil, nil
		}
	}

	gameExists := len(h.gameData.ActionLog) > 0 || h.gameData.OwnerID != ""

	// Authorization Check
	baseAccess := GetGameAccess(userId, *h.gameData, h.r.teamStore)
	effectiveAccess := baseAccess

	isGameStart := false
	for _, raw := range actions {
		var meta struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			continue // Malformed meta, but base validation already caught serious issues
		}

		actionAccess := effectiveAccess

		if meta.Type == "GAME_START" {
			isGameStart = true
			if !gameExists {
				var p struct {
					OwnerID string `json:"ownerId"`
				}
				if err := json.Unmarshal(meta.Payload, &p); err == nil {
					if normalizeEmail(p.OwnerID) == userId {
						actionAccess = AccessWrite
						effectiveAccess = AccessWrite // Elevate for subsequent actions in batch
					}
				}
			}
		}

		if meta.Type == ActionGameMetadataUpdate {
			if actionAccess < AccessAdmin {
				return &Message{Type: MsgTypeError, Error: "Forbidden: Only admins can update game metadata"}, nil, nil
			}
		} else {
			if actionAccess < AccessWrite {
				log.Printf("Forbidden: User %s attempted to write action %s to game %s", maskEmail(userId), meta.Type, h.resourceId)
				if userId == "" {
					return &Message{Type: MsgTypeError, Error: "Unauthenticated: Login required"}, nil, nil
				} else {
					return &Message{Type: MsgTypeError, Error: "Forbidden: You do not have write access to this game"}, nil, nil
				}
			}
		}
	}

	if !gameExists && !isGameStart {
		log.Printf("Conflict: User %s sending action for non-existent game %s", maskEmail(userId), h.resourceId)
		return &Message{Type: MsgTypeConflict, Error: "Game not found on server", BaseRevision: ""}, nil, nil
	}

	// If Raft is enabled, ensure we are the leader before checking state against local (possibly stale) cache.
	if h.rm != nil && h.rm.Raft.State() != raft.Leader {
		return nil, nil, ErrNotLeader
	}

	currentServerRevision := getCurrentRevision(h.gameData.ActionLog)

	if len(h.gameData.ActionLog) > 0 && msg.BaseRevision != currentServerRevision {
		// Attempt reload from disk to clear stale cache
		if g, err := h.gs.LoadGame(h.resourceId); err == nil {
			h.gameData = g
			currentServerRevision = getCurrentRevision(h.gameData.ActionLog)
		}

		if len(h.gameData.ActionLog) > 0 && msg.BaseRevision != currentServerRevision {
			// Log Prefix Matching Logic to handle partial retries
			matchIndex := -1
			if msg.BaseRevision == "" {
				// Client claiming start of game, but server has log.
				// This is valid only if the batch matches the log from the beginning.
				matchIndex = -1
			} else {
				// Find BaseRevision in log
				found := false
				// Search backwards optimized for recent actions
				for i := len(h.gameData.ActionLog) - 1; i >= 0; i-- {
					var a struct {
						ID string `json:"id"`
					}
					if err := json.Unmarshal(h.gameData.ActionLog[i], &a); err == nil {
						if a.ID == msg.BaseRevision {
							matchIndex = i
							found = true
							break
						}
					}
				}
				if !found {
					// Base not found in log: Real Conflict (Fork)
					log.Printf("Conflict: Base revision %s not found in log (Head: %s) for user %s", msg.BaseRevision, currentServerRevision, maskEmail(userId))
					h.conflictMu.Lock()
					defer h.conflictMu.Unlock()
					h.lastConflict[userId] = time.Now()
					return &Message{Type: MsgTypeConflict, Error: "Base revision not found", BaseRevision: currentServerRevision}, nil, nil
				}
			}

			// Validate overlap: Ensure actions in batch match server log starting after BaseRevision
			serverIdx := matchIndex + 1
			batchIdx := 0
			conflict := false

			for serverIdx < len(h.gameData.ActionLog) && batchIdx < len(actions) {
				var sAction struct {
					ID string `json:"id"`
				}
				json.Unmarshal(h.gameData.ActionLog[serverIdx], &sAction)

				var bAction struct {
					ID string `json:"id"`
				}
				json.Unmarshal(actions[batchIdx], &bAction)

				if sAction.ID != bAction.ID {
					conflict = true
					break
				}
				serverIdx++
				batchIdx++
			}

			if conflict {
				h.conflictMu.Lock()
				defer h.conflictMu.Unlock()
				h.lastConflict[userId] = time.Now()
				return &Message{Type: MsgTypeConflict, Error: "History divergence", BaseRevision: currentServerRevision}, nil, nil
			}

			// If we exhausted the batch, everything was idempotent
			if batchIdx == len(actions) {
				return &Message{Type: MsgTypeAck}, nil, nil
			}

			// If we exhausted server log but have more actions, apply the remainder
			actions = actions[batchIdx:]
		}
	}

	if h.rm != nil {
		// Propose to Raft
		actionPayload := &ActionPayload{
			GameID:  h.resourceId,
			Action:  msg.Action,
			Actions: msg.Actions,
			UserID:  userId,
		}
		cmd := RaftCommand{
			Type:   CmdApplyAction,
			ID:     h.resourceId,
			Action: actionPayload,
		}
		if _, err := h.rm.Propose(cmd); err != nil {
			return nil, nil, err
		}
		// Success!
		return &Message{Type: MsgTypeAck}, nil, nil
	}

	// Non-Consensus Path: Apply to a clone to prevent in-memory corruption on failure
	var clone Game
	gameBytes, _ := json.Marshal(*h.gameData)
	json.Unmarshal(gameBytes, &clone)

	changed := false
	if len(msg.Actions) > 0 {
		changed, err = ApplyActions(&clone, msg.Actions)
		if err != nil {
			return &Message{Type: MsgTypeError, Error: "Server error applying actions: " + err.Error()}, nil, nil
		}
	} else {
		changed, err = ApplyAction(&clone, msg.Action)
		if err != nil {
			return &Message{Type: MsgTypeError, Error: "Server error applying action: " + err.Error()}, nil, nil
		}
	}

	if !changed {
		// Already applied, just acknowledge
		return &Message{Type: MsgTypeAck}, nil, nil
	}

	if err := h.gs.SaveGame(&clone); err != nil {
		return &Message{Type: MsgTypeError, Error: "Server error saving action"}, nil, nil
	}

	// Success: commit to Hub cache and Registry
	*h.gameData = clone
	h.r.UpdateGame(*h.gameData)

	// Collect broadcast messages
	var msgs []Message
	if len(msg.Actions) > 0 {
		log.Printf("Broadcasting batch of %d actions for game %s", len(msg.Actions), h.resourceId)
		for _, a := range msg.Actions {
			msgs = append(msgs, Message{Type: MsgTypeAction, Action: a})
		}
	} else if msg.Action != nil {
		log.Printf("Broadcasting single action for game %s", h.resourceId)
		msgs = append(msgs, Message{Type: MsgTypeAction, Action: msg.Action})
	}

	return &Message{Type: MsgTypeAck}, msgs, nil
}

func (h *Hub) broadcast(msg Message) {
	for client := range h.clients {
		select {
		case client.send <- msg:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}

func (h *Hub) handleHTTPLoad(reply chan HubResponse) {
	var data []byte
	var err error
	if h.isTeam {
		data, err = json.Marshal(h.teamData)
	} else {
		data, err = json.Marshal(h.gameData)
	}
	reply <- HubResponse{Data: data, Error: err}
}

func (h *Hub) handleHTTPSave(payload []byte, reply chan HubResponse) {
	if h.rm != nil {
		cmdType := CmdSaveGame
		var cmd RaftCommand
		if h.isTeam {
			cmdType = CmdSaveTeam
			// Re-wrap raw payload?
			// RaftCommand expects *json.RawMessage.
			// payload is []byte.
			raw := json.RawMessage(payload)
			cmd = RaftCommand{
				Type:     cmdType,
				ID:       h.resourceId,
				TeamData: &raw,
			}
		} else {
			raw := json.RawMessage(payload)
			cmd = RaftCommand{
				Type:     cmdType,
				ID:       h.resourceId,
				GameData: &raw,
			}
		}

		if _, err := h.rm.Propose(cmd); err != nil {
			reply <- HubResponse{Error: err}
			return
		}

		reply <- HubResponse{Error: nil}
		return
	}

	if h.isTeam {
		var newTeam Team
		if err := json.Unmarshal(payload, &newTeam); err != nil {
			reply <- HubResponse{Error: err}
			return
		}
		h.teamData = &newTeam
		if err := h.ts.SaveTeam(h.teamData); err != nil {
			reply <- HubResponse{Error: err}
			return
		}
		h.r.UpdateTeam(*h.teamData)
	} else {
		var newGame Game
		if err := json.Unmarshal(payload, &newGame); err != nil {
			reply <- HubResponse{Error: err}
			return
		}
		h.gameData = &newGame
		if err := h.gs.SaveGame(h.gameData); err != nil {
			reply <- HubResponse{Error: err}
			return
		}
		h.r.UpdateGame(*h.gameData)

		// NOTE: We do NOT broadcast the update here.
	}
	reply <- HubResponse{Error: nil}
}

func loadGameData(store *GameStore, gameId string) (Game, error) {
	game, err := store.LoadGame(gameId) // Returns *Game
	if err != nil {
		return Game{}, err
	}
	return *game, nil
}

func getCurrentRevision(log []json.RawMessage) string {
	if len(log) == 0 {
		return ""
	}
	last := log[len(log)-1]
	var action struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(last, &action); err != nil {
		return ""
	}
	return action.ID
}

func getActionsSince(log []json.RawMessage, revision string) []json.RawMessage {
	if revision == "" {
		return log
	}
	for i, raw := range log {
		var action struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &action); err != nil {
			continue
		}
		if action.ID == revision {
			return log[i+1:]
		}
	}
	return nil
}

// ServeWS handles websocket requests from the peer.
func ServeWS(gs *GameStore, ts *TeamStore, r *Registry, hm *HubManager, w http.ResponseWriter, r_req *http.Request, debugf func(string, ...any)) {
	userId := getUserID(r_req)

	gameId := r_req.URL.Query().Get("gameId")
	if gameId == "" || !isValidUUID(gameId) {
		http.Error(w, "Invalid gameId", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r_req, nil)
	if err != nil {
		log.Println(err)
		return
	}

	hub := hm.GetHub(gameId, false, gs, ts, r)
	client := &wsClient{hub: hub, conn: conn, send: make(chan Message, 256), userId: userId, gameId: gameId, gs: gs, ts: ts, r: r}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}
