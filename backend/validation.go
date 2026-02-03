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
	"fmt"
	"net/mail"
	"regexp"
	"time"
)

// uuidRegex is a regex for standard UUIDs (8-4-4-4-12 hex digits)
var uuidRegex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// isValidUUID checks if the string is a valid UUID.
func isValidUUID(id string) bool {
	return uuidRegex.MatchString(id)
}

// isValidEmail checks if the string is a valid email address.
func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

const (
	CurrentSchemaVersion   = 3
	CurrentProtocolVersion = 1
	CurrentAppVersion      = "0.2.5"
)

// ActionTypes constants
const (
	ActionGameStart          = "GAME_START"
	ActionLineupUpdate       = "LINEUP_UPDATE"
	ActionSubstitution       = "SUBSTITUTION"
	ActionPitch              = "PITCH"
	ActionPlayResult         = "PLAY_RESULT"
	ActionRunnerAdvance      = "RUNNER_ADVANCE"
	ActionScoreOverride      = "SCORE_OVERRIDE"
	ActionGameImport         = "GAME_IMPORT"
	ActionPitcherUpdate      = "PITCHER_UPDATE"
	ActionMovePlay           = "MOVE_PLAY"
	ActionClearData          = "CLEAR_DATA"
	ActionRunnerBatchUpdate  = "RUNNER_BATCH_UPDATE"
	ActionUndo               = "UNDO"
	ActionAddInning          = "ADD_INNING"
	ActionAddColumn          = "ADD_COLUMN"
	ActionRemoveColumn       = "REMOVE_COLUMN"
	ActionGameMetadataUpdate = "GAME_METADATA_UPDATE"
	ActionSetInningLead      = "SET_INNING_LEAD"
	ActionGameFinalize       = "GAME_FINALIZE"
	ActionManualPathOverride = "MANUAL_PATH_OVERRIDE"
	ActionOutNumUpdate       = "OUT_NUM_UPDATE"
	ActionRBIEdit            = "RBI_EDIT"
)

// BaseAction represents the common fields of an action.
type BaseAction struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Payload       json.RawMessage `json:"payload"`
	Timestamp     int64           `json:"timestamp"`
	SchemaVersion int             `json:"schemaVersion,omitempty"`
}

// ValidateGameData validates the entire game data structure including the action log.
func ValidateGameData(data []byte) error {
	var game struct {
		ID        string            `json:"id"`
		ActionLog []json.RawMessage `json:"actionLog"`
	}
	if err := json.Unmarshal(data, &game); err != nil {
		return fmt.Errorf("invalid game JSON: %w", err)
	}

	if !isValidUUID(game.ID) {
		return fmt.Errorf("invalid game ID format: %s", game.ID)
	}

	for i, rawAction := range game.ActionLog {
		if err := ValidateAction(rawAction); err != nil {
			return fmt.Errorf("invalid action at index %d: %w", i, err)
		}
	}

	return nil
}

// ValidateAction validates a single action from raw JSON.
func ValidateAction(raw json.RawMessage) error {
	var action BaseAction
	if err := json.Unmarshal(raw, &action); err != nil {
		return fmt.Errorf("malformed action JSON")
	}

	if !isValidUUID(action.ID) {
		return fmt.Errorf("invalid action ID: %s", action.ID)
	}
	if action.Type == "" {
		return fmt.Errorf("missing action type")
	}

	return validateActionPayload(action.Type, action.Payload)
}

// validateActionPayload validates the payload based on the action type.
func validateActionPayload(actionType string, payload json.RawMessage) error {
	switch actionType {
	case ActionGameStart:
		return validateGameStart(payload)
	case ActionPitch:
		return validatePitch(payload)
	case ActionPlayResult:
		return validatePlayResult(payload)
	case ActionRunnerAdvance:
		return validateRunnerAdvance(payload)
	case ActionSubstitution:
		return validateSubstitution(payload)
	case ActionLineupUpdate:
		return validateLineupUpdate(payload)
	case ActionScoreOverride:
		return validateScoreOverride(payload)
	case ActionGameImport:
		return validateGameImport(payload)
	case ActionPitcherUpdate:
		return validatePitcherUpdate(payload)
	case ActionMovePlay:
		return validateMovePlay(payload)
	case ActionClearData:
		return validateClearData(payload)
	case ActionRunnerBatchUpdate:
		return validateRunnerBatchUpdate(payload)
	case ActionAddInning:
		return nil // No payload validation needed for now
	case ActionAddColumn:
		return validateAddColumn(payload)
	case ActionRemoveColumn:
		return validateRemoveColumn(payload)
	case ActionGameMetadataUpdate:
		return validateGameMetadataUpdate(payload)
	case ActionSetInningLead:
		return validateSetInningLead(payload)
	case ActionGameFinalize:
		return validateGameFinalize(payload)
	case ActionUndo:
		return validateUndo(payload)
	case ActionManualPathOverride:
		return nil // Basic pass-through
	case ActionOutNumUpdate:
		return nil // Basic pass-through
	case ActionRBIEdit:
		return nil // Basic pass-through
	default:
		return fmt.Errorf("unknown action type: %s", actionType)
	}
}

// validateStringLen checks if the string length is within the limit.
func validateStringLen(s string, max int, name string) error {
	if len(s) > max {
		return fmt.Errorf("%s too long (max %d chars)", name, max)
	}
	return nil
}

// --- Specific Payload Validators ---

type Context struct {
	B   int    `json:"b"`
	I   int    `json:"i"`
	Col string `json:"col"`
}

func validateContext(ctx Context) error {
	if ctx.B < 0 || ctx.B > 999 {
		return fmt.Errorf("invalid batter index: %d", ctx.B)
	}
	if ctx.I < 1 {
		return fmt.Errorf("invalid inning: %d", ctx.I)
	}
	if err := validateStringLen(ctx.Col, 20, "col"); err != nil {
		return err
	}
	return nil
}

func validateGameStart(payload json.RawMessage) error {
	var p struct {
		ID               string              `json:"id"`
		Date             string              `json:"date"`
		Away             string              `json:"away"`
		Home             string              `json:"home"`
		Event            string              `json:"event"`
		Location         string              `json:"location"`
		AwayTeamID       string              `json:"awayTeamId"`
		HomeTeamID       string              `json:"homeTeamId"`
		InitialRosterIds map[string][]string `json:"initialRosterIds"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if !isValidUUID(p.ID) {
		return fmt.Errorf("invalid game ID in payload")
	}
	if p.Away == "" || p.Home == "" {
		return fmt.Errorf("missing team names")
	}
	if err := validateStringLen(p.Away, 50, "away team"); err != nil {
		return err
	}
	if err := validateStringLen(p.Home, 50, "home team"); err != nil {
		return err
	}
	if err := validateStringLen(p.Event, 100, "event"); err != nil {
		return err
	}
	if err := validateStringLen(p.Location, 100, "location"); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, p.Date); err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}
	return nil
}

func validatePitch(payload json.RawMessage) error {
	var p struct {
		ActiveCtx  Context `json:"activeCtx"`
		Type       string  `json:"type"`
		ActiveTeam string  `json:"activeTeam"`
		BatterID   string  `json:"batterId"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if err := validateContext(p.ActiveCtx); err != nil {
		return err
	}
	if p.Type == "" {
		return fmt.Errorf("missing pitch type")
	}
	if err := validateStringLen(p.Type, 20, "pitch type"); err != nil {
		return err
	}
	if p.ActiveTeam != "away" && p.ActiveTeam != "home" {
		return fmt.Errorf("invalid active team")
	}
	return nil
}

func validatePlayResult(payload json.RawMessage) error {
	var p struct {
		ActiveCtx  Context `json:"activeCtx"`
		ActiveTeam string  `json:"activeTeam"`
		BipState   struct {
			Res  string `json:"res"`
			Base string `json:"base"`
			Type string `json:"type"`
		} `json:"bipState"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if err := validateContext(p.ActiveCtx); err != nil {
		return err
	}
	if p.BipState.Res == "" {
		return fmt.Errorf("missing bipState.res")
	}
	if err := validateStringLen(p.BipState.Res, 20, "res"); err != nil {
		return err
	}
	if err := validateStringLen(p.BipState.Base, 10, "base"); err != nil {
		return err
	}
	if err := validateStringLen(p.BipState.Type, 20, "type"); err != nil {
		return err
	}
	return nil
}

func validateRunnerAdvance(payload json.RawMessage) error {
	var p struct {
		Runners []struct {
			Key     string `json:"key"`
			Base    int    `json:"base"`
			Outcome string `json:"outcome"`
		} `json:"runners"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	for _, r := range p.Runners {
		if r.Base < 0 || r.Base > 3 {
			return fmt.Errorf("invalid base index")
		}
		if r.Outcome == "" {
			return fmt.Errorf("missing runner outcome")
		}
		if err := validateStringLen(r.Outcome, 20, "outcome"); err != nil {
			return err
		}
		if err := validateStringLen(r.Key, 50, "key"); err != nil {
			return err
		}
	}
	return nil
}

func validateSubstitution(payload json.RawMessage) error {
	var p struct {
		Team        string `json:"team"`
		RosterIndex int    `json:"rosterIndex"`
		SubParams   struct {
			Name   string `json:"name"`
			Number string `json:"number"`
			Pos    string `json:"pos"`
			ID     string `json:"id"`
		} `json:"subParams"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.Team != "away" && p.Team != "home" {
		return fmt.Errorf("invalid team")
	}
	if p.RosterIndex < 0 || p.RosterIndex > 99 {
		return fmt.Errorf("invalid roster index: %d", p.RosterIndex)
	}
	if !isValidUUID(p.SubParams.ID) {
		return fmt.Errorf("invalid player ID")
	}
	if err := validateStringLen(p.SubParams.Name, 50, "player name"); err != nil {
		return err
	}
	if err := validateStringLen(p.SubParams.Number, 10, "player number"); err != nil {
		return err
	}
	if err := validateStringLen(p.SubParams.Pos, 10, "position"); err != nil {
		return err
	}
	return nil
}

func validateLineupUpdate(payload json.RawMessage) error {
	var p struct {
		Team     string `json:"team"`
		TeamName string `json:"teamName"`
		Roster   []struct {
			Current struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Number string `json:"number"`
				Pos    string `json:"pos"`
			} `json:"current"`
		} `json:"roster"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.Team != "away" && p.Team != "home" {
		return fmt.Errorf("invalid team")
	}
	if err := validateStringLen(p.TeamName, 50, "team name"); err != nil {
		return err
	}
	for _, s := range p.Roster {
		if !isValidUUID(s.Current.ID) {
			return fmt.Errorf("invalid roster player ID")
		}
		if err := validateStringLen(s.Current.Name, 50, "player name"); err != nil {
			return err
		}
		if err := validateStringLen(s.Current.Number, 10, "player number"); err != nil {
			return err
		}
		if err := validateStringLen(s.Current.Pos, 10, "position"); err != nil {
			return err
		}
	}
	return nil
}

func validateScoreOverride(payload json.RawMessage) error {
	var p struct {
		Team   string `json:"team"`
		Inning int    `json:"inning"`
		Score  string `json:"score"` // Frontend sends string from prompt
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.Team != "away" && p.Team != "home" {
		return fmt.Errorf("invalid team")
	}
	if p.Inning < 1 {
		return fmt.Errorf("invalid inning")
	}
	if err := validateStringLen(p.Score, 5, "score"); err != nil {
		return err
	}
	return nil
}

func validateGameImport(payload json.RawMessage) error {
	// Full game object import. Recursive validation?
	// For now, just check basic fields exist.
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if !isValidUUID(p.ID) {
		return fmt.Errorf("invalid imported game ID")
	}
	return nil
}

func validatePitcherUpdate(payload json.RawMessage) error {
	var p struct {
		Team    string `json:"team"`
		Pitcher string `json:"pitcher"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if err := validateStringLen(p.Pitcher, 50, "pitcher name"); err != nil {
		return err
	}
	return nil
}

func validateMovePlay(payload json.RawMessage) error {
	var p struct {
		SourceKey string `json:"sourceKey"`
		TargetKey string `json:"targetKey"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.SourceKey == "" || p.TargetKey == "" {
		return fmt.Errorf("missing source/target keys")
	}
	if err := validateStringLen(p.SourceKey, 50, "sourceKey"); err != nil {
		return err
	}
	if err := validateStringLen(p.TargetKey, 50, "targetKey"); err != nil {
		return err
	}
	return nil
}

func validateClearData(payload json.RawMessage) error {
	var p struct {
		ActiveCtx  Context `json:"activeCtx"`
		ActiveTeam string  `json:"activeTeam"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	return validateContext(p.ActiveCtx)
}

func validateRunnerBatchUpdate(payload json.RawMessage) error {
	var p struct {
		Updates []struct {
			Key    string `json:"key"`
			Action string `json:"action"`
			Base   int    `json:"base"`
		} `json:"updates"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	for _, u := range p.Updates {
		if err := validateStringLen(u.Key, 50, "key"); err != nil {
			return err
		}
		if err := validateStringLen(u.Action, 20, "action"); err != nil {
			return err
		}
	}
	return nil
}

func validateAddColumn(payload json.RawMessage) error {
	var p struct {
		TargetInning int    `json:"targetInning"`
		Team         string `json:"team"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.TargetInning < 1 {
		return fmt.Errorf("invalid target inning")
	}
	return nil
}

func validateRemoveColumn(payload json.RawMessage) error {
	var p struct {
		ColId string `json:"colId"`
		Team  string `json:"team"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.ColId == "" {
		return fmt.Errorf("missing colId")
	}
	if err := validateStringLen(p.ColId, 20, "colId"); err != nil {
		return err
	}
	return nil
}

func validateGameMetadataUpdate(payload json.RawMessage) error {
	var p struct {
		ID          string       `json:"id"`
		Date        string       `json:"date"`
		Event       string       `json:"event"`
		Location    string       `json:"location"`
		Away        string       `json:"away"`
		Home        string       `json:"home"`
		AwayTeamID  string       `json:"awayTeamId"`
		HomeTeamID  string       `json:"homeTeamId"`
		Permissions *Permissions `json:"permissions"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if !isValidUUID(p.ID) {
		return fmt.Errorf("invalid game ID")
	}
	if err := validateStringLen(p.Date, 50, "date"); err != nil {
		return err
	}
	if err := validateStringLen(p.Event, 100, "event"); err != nil {
		return err
	}
	if err := validateStringLen(p.Location, 100, "location"); err != nil {
		return err
	}
	if err := validateStringLen(p.Away, 50, "away team"); err != nil {
		return err
	}
	if err := validateStringLen(p.Home, 50, "home team"); err != nil {
		return err
	}
	// We allow non-UUID values for team IDs to accommodate existing data with UI-placeholder values.
	return nil
}

func validateSetInningLead(payload json.RawMessage) error {
	var p struct {
		Team  string `json:"team"`
		ColId string `json:"colId"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.ColId == "" {
		return fmt.Errorf("missing colId")
	}
	if err := validateStringLen(p.ColId, 20, "colId"); err != nil {
		return err
	}
	return nil
}

func validateUndo(payload json.RawMessage) error {
	var p struct {
		RefId string `json:"refId"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if !isValidUUID(p.RefId) {
		return fmt.Errorf("invalid refId")
	}
	return nil
}

func validateGameFinalize(payload json.RawMessage) error {
	var p struct {
		FinalScore struct {
			Away int `json:"away"`
			Home int `json:"home"`
		} `json:"finalScore"`
		Stats     json.RawMessage `json:"stats"`
		Timestamp int64           `json:"timestamp"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if p.Stats == nil {
		return fmt.Errorf("missing stats")
	}
	return nil
}

// ValidateActions validates a list of actions.
func ValidateActions(actions []json.RawMessage) error {
	for i, raw := range actions {
		if err := ValidateAction(raw); err != nil {
			return fmt.Errorf("invalid action at index %d: %w", i, err)
		}
	}
	return nil
}

// ApplyActions appends multiple actions to the game state.
func ApplyActions(g *Game, actions []json.RawMessage) (bool, error) {
	anyChanged := false
	for _, raw := range actions {
		changed, err := ApplyAction(g, raw)
		if err != nil {
			return anyChanged, err
		}
		if changed {
			anyChanged = true
		}
	}
	return anyChanged, nil
}

// ApplyAction appends an action to the game state and updates metadata.
// It assumes validation and authorization have already been performed.
// Returns true if the action was applied, false if it was a duplicate.
func ApplyAction(g *Game, raw json.RawMessage) (bool, error) {
	var action BaseAction
	if err := json.Unmarshal(raw, &action); err != nil {
		return false, fmt.Errorf("failed to unmarshal action for apply: %w", err)
	}

	// Idempotency check: check if this action is already in the log.
	// We scan backwards through the action log to find duplicates.
	// A limit of 100 actions is used to maintain performance (O(K) where K=100) while effectively
	// catching duplicates from transient network retries or client double-submissions.
	// For massive logs, an O(N) scan would become a bottleneck.
	const maxScan = 100
	for i, count := len(g.ActionLog)-1, 0; i >= 0 && count < maxScan; i, count = i-1, count+1 {
		var existing BaseAction
		if err := json.Unmarshal(g.ActionLog[i], &existing); err == nil {
			if existing.ID == action.ID {
				return false, nil // Already applied
			}
		}
	}

	// Apply Metadata Updates
	if action.Type == ActionGameStart {
		g.SchemaVersion = action.SchemaVersion
		if g.SchemaVersion == 0 {
			g.SchemaVersion = CurrentSchemaVersion
		}
		var p struct {
			ID          string      `json:"id"`
			OwnerID     string      `json:"ownerId"`
			Date        string      `json:"date"`
			Location    string      `json:"location"`
			Event       string      `json:"event"`
			Away        string      `json:"away"`
			Home        string      `json:"home"`
			AwayTeamID  string      `json:"awayTeamId"`
			HomeTeamID  string      `json:"homeTeamId"`
			Permissions Permissions `json:"permissions"`
		}
		if err := json.Unmarshal(action.Payload, &p); err == nil {
			g.ID = p.ID
			g.OwnerID = p.OwnerID
			g.Date = p.Date
			g.Location = p.Location
			g.Event = p.Event
			g.Away = p.Away
			g.Home = p.Home
			g.AwayTeamID = p.AwayTeamID
			g.HomeTeamID = p.HomeTeamID
			g.Permissions = p.Permissions
		}
	} else if action.Type == ActionGameMetadataUpdate {
		var p struct {
			AwayTeamID  *string      `json:"awayTeamId"`
			HomeTeamID  *string      `json:"homeTeamId"`
			Permissions *Permissions `json:"permissions"`
			Date        *string      `json:"date"`
			Location    *string      `json:"location"`
			Event       *string      `json:"event"`
			Away        *string      `json:"away"`
			Home        *string      `json:"home"`
		}
		if err := json.Unmarshal(action.Payload, &p); err == nil {
			if p.AwayTeamID != nil {
				g.AwayTeamID = *p.AwayTeamID
			}
			if p.HomeTeamID != nil {
				g.HomeTeamID = *p.HomeTeamID
			}
			if p.Permissions != nil {
				g.Permissions = *p.Permissions
			}
			if p.Date != nil {
				g.Date = *p.Date
			}
			if p.Location != nil {
				g.Location = *p.Location
			}
			if p.Event != nil {
				g.Event = *p.Event
			}
			if p.Away != nil {
				g.Away = *p.Away
			}
			if p.Home != nil {
				g.Home = *p.Home
			}
		}
	} else if action.Type == ActionGameFinalize {
		g.Status = "final"
	}

	// Append to log
	g.ActionLog = append(g.ActionLog, raw)
	g.LastActionID = action.ID
	return true, nil
}
