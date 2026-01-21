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
	"encoding/json"
)

// CommandType represents the type of operation to perform on the FSM.
type CommandType string

const (
	CmdSaveGame           CommandType = "SAVE_GAME"
	CmdDeleteGame         CommandType = "DELETE_GAME"
	CmdApplyAction        CommandType = "APPLY_ACTION"
	CmdSaveTeam           CommandType = "SAVE_TEAM"
	CmdDeleteTeam         CommandType = "DELETE_TEAM"
	CmdNodeMeta           CommandType = "NODE_META"
	CmdNodeLeft           CommandType = "NODE_LEFT"
	CmdUpdateAccessPolicy CommandType = "UPDATE_ACCESS_POLICY"
	CmdMetricsUpdate      CommandType = "METRICS_UPDATE"
)

// RaftCommand is a unified structure for all Raft log entries.
type RaftCommand struct {
	Type           CommandType       `json:"type"`
	NodeMeta       *NodeMeta         `json:"nodeMeta,omitempty"`
	Action         *ActionPayload    `json:"action,omitempty"`
	GameData       *json.RawMessage  `json:"gameData,omitempty"`
	TeamData       *json.RawMessage  `json:"teamData,omitempty"`
	PolicyData     *UserAccessPolicy `json:"policyData,omitempty"`
	MetricsPayload *MetricsPayload   `json:"metricsPayload,omitempty"`
	ID             string            `json:"id,omitempty"`
	Force          bool              `json:"force,omitempty"`
}

// UserAccessPolicy defines global access rules and quotas.
type UserAccessPolicy struct {
	DefaultPolicy      string                  `json:"defaultPolicy"` // "allow" or "deny"
	DefaultMaxTeams    int                     `json:"defaultMaxTeams"`
	DefaultMaxGames    int                     `json:"defaultMaxGames"`
	DefaultDenyMessage string                  `json:"defaultDenyMessage"`
	Admins             []string                `json:"admins"` // List of admin emails
	Users              map[string]UserOverride `json:"users"`
}

// UserOverride defines specific access rules for a single user.
type UserOverride struct {
	Access   string `json:"access"` // "allow" or "deny"
	MaxTeams int    `json:"maxTeams"`
	MaxGames int    `json:"maxGames"`
}

// NodeMeta contains metadata about a cluster node.
type NodeMeta struct {
	NodeID          string `json:"nodeId"`
	HttpAddr        string `json:"httpAddr"`
	PubKey          string `json:"pubKey"` // Base64-encoded Ed25519 public key
	AppVersion      string `json:"appVersion,omitempty"`
	ProtocolVersion int    `json:"protocolVersion,omitempty"`
	SchemaVersion   int    `json:"schemaVersion,omitempty"`
}

// ActionPayload contains details for CmdApplyAction
type ActionPayload struct {
	GameID  string            `json:"gameId"`
	Action  json.RawMessage   `json:"action,omitempty"`
	Actions []json.RawMessage `json:"actions,omitempty"`
	UserID  string            `json:"userId"`
}
