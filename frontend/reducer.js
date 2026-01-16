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

/**
 * Reducer function to manage game state transitions based on actions.
 * Follows the Redux pattern: (state, action) => newState
 */

export const ActionTypes = {
    GAME_START: 'GAME_START',
    LINEUP_UPDATE: 'LINEUP_UPDATE',
    SUBSTITUTION: 'SUBSTITUTION',
    PITCH: 'PITCH',
    PLAY_RESULT: 'PLAY_RESULT',
    RUNNER_ADVANCE: 'RUNNER_ADVANCE',
    SCORE_OVERRIDE: 'SCORE_OVERRIDE',
    RBI_EDIT: 'RBI_EDIT',
    GAME_IMPORT: 'GAME_IMPORT',
    PITCHER_UPDATE: 'PITCHER_UPDATE',
    MOVE_PLAY: 'MOVE_PLAY',
    CLEAR_DATA: 'CLEAR_DATA',
    RUNNER_BATCH_UPDATE: 'RUNNER_BATCH_UPDATE',
    UNDO: 'UNDO',
    ADD_INNING: 'ADD_INNING',
    ADD_COLUMN: 'ADD_COLUMN',
    REMOVE_COLUMN: 'REMOVE_COLUMN',
    GAME_METADATA_UPDATE: 'GAME_METADATA_UPDATE',
    SET_INNING_LEAD: 'SET_INNING_LEAD',
    GAME_FINALIZE: 'GAME_FINALIZE',
    OUT_NUM_UPDATE: 'OUT_NUM_UPDATE',
    MANUAL_PATH_OVERRIDE: 'MANUAL_PATH_OVERRIDE',
};

import {
    CurrentSchemaVersion,
    GameStatusOngoing,
    GameStatusFinal,
    TeamAway,
    TeamHome,
    PitchTypeBall,
    PitchTypeStrike,
    PitchTypeFoul,
    PitchCodeCalled,
    PitchCodeDropped,
    BiPResultGround,
    BiPResultOut,
    RunnerActionScore,
    RunnerActionOut,
    RunnerActionStay,
} from './constants.js';

/**
 * Returns the initial state for a new game.
 * @returns {object} The initial game state.
 */
export function getInitialState() {
    return {
        id: '',
        schemaVersion: CurrentSchemaVersion,
        date: '',
        location: '',
        event: '',
        away: '',
        home: '',
        status: GameStatusOngoing,
        ownerId: '',
        awayTeamId: '',
        homeTeamId: '',
        pitchers: { away: '', home: '' },
        overrides: { away: {}, home: {} },
        events: {},
        columns: [],
        pitchLog: [],
        actionLog: [],
        permissions: { public: 'none', users: {} },
        roster: { away: [], home: [] },
        subs: { away: [], home: [] },
    };
}

/**
 * Computes the current game state from the action log, handling Append-Only Undo.
 * @param {Array} log - The full array of actions.
 * @returns {object} The computed state.
 */
export function computeStateFromLog(log) {
    if (!log || log.length === 0) {
        return getInitialState();
    }

    // Pass 1: Identify Tombstones
    // Iterate through the log to determine the validity of each action.
    // An UNDO action invalidates its target ('refId').
    // We maintain a validity map where actions are valid by default unless targeted by an active UNDO.
    // Note: This logic assumes a linear history where UNDO actions target previous actions.

    const validMap = new Map(); // ActionID -> Boolean (Is Valid?)

    log.forEach(action => {
        if (action.type === ActionTypes.UNDO) {
            // An UNDO action is valid by default (unless we allow undoing undos later)
            // If this UNDO targets an existing action, we mark that target as invalid.
            if (action.payload && action.payload.refId) {
                // Mark the target as invalid
                validMap.set(action.payload.refId, false);
                // The UNDO action itself is "valid" in the sense that it occurred,
                // but it doesn't affect state directly, only via side-effect on map.
                validMap.set(action.id, true);
            }
        } else {
            // Normal action, valid by default
            validMap.set(action.id, true);
        }
    });

    // Now reduce, skipping invalid actions
    // Note: This simple map approach works for:
    // 1. Action A
    // 2. Undo A (A becomes invalid)
    // 3. Redo A (We need to Undo the Undo A).
    //    If Redo is just "Undo(UndoA)", then:
    //    3. Undo (refId: IdOfAction2).
    //    Map processing:
    //    1. A: Valid
    //    2. UndoA: Valid. Sets A -> Invalid.
    //    3. UndoUndoA: Valid. Sets UndoA -> Invalid.
    //    Result: A is Invalid?
    //    Wait, if UndoA is Invalid, it should NOT have invalidated A.
    //    So we DO need to process validity carefully.

    // Correct logic:
    // We need to determine if an action is effective.
    // An action is effective if it is NOT targeted by an *effective* UNDO.
    // This is recursive.
    // Since actions can only target *past* actions, we can resolve this?
    // No, future actions invalidate past actions.
    // A <- Undo A <- Undo Undo A
    // We need to process from End to Start?
    // If we see UndoUndoA (active), it invalidates UndoA.
    // If UndoA is invalid, it DOES NOT invalidate A.
    // Yes! Reverse iteration is the key.

    const effectivelyUndone = new Set(); // IDs of actions that are effectively undone

    // Iterate backwards
    for (let i = log.length - 1; i >= 0; i--) {
        const action = log[i];
        if (effectivelyUndone.has(action.id)) {
            // This action (whether regular or UNDO) is already neutralized.
            // It cannot affect anything else.
            continue;
        }

        if (action.type === ActionTypes.UNDO && action.payload && action.payload.refId) {
            // This is an ACTIVE Undo action.
            // It neutralizes its target.
            effectivelyUndone.add(action.payload.refId);
        }
    }

    // Pass 2: Reduce forward, skipping tombstoned (effectivelyUndone) actions
    // Also skip UNDO actions themselves as they have no state effect other than the tombstone logic we just handled.

    let state = getInitialState();

    // Preserve the full log in the state for future appends
    state.actionLog = log;

    log.forEach(action => {
        if (!effectivelyUndone.has(action.id) && action.type !== ActionTypes.UNDO) {
            state = gameReducer(state, action);
        }
    });

    // Ensure actionLog is attached (gameReducer might spread it out or create new object without it if not careful,
    // though our gameReducer implementation tries to keep it or we re-attach it)
    state.actionLog = log;

    return state;
}

/**
 * The main reducer function.
 * @param {object} state - The current game state.
 * @param {object} action - The action to apply.
 * @returns {object} The new game state.
 */
export function gameReducer(state, action) {
    // Use structural sharing instead of deep clone for performance
    const newState = { ...state };
    // Ensure actionLog exists (for existing states migrating)
    if (!newState.actionLog) {
        newState.actionLog = [];
    }

    switch (action.type) {
        case ActionTypes.GAME_IMPORT:
            return applyGameImport(newState, action.payload);

        case ActionTypes.GAME_START:
            // GAME_START replaces the state mostly, so deep clone or fresh object is fine/expected there
            // But let's stick to the function contract
            return applyGameStart(newState, action.payload);

        case ActionTypes.PITCH:
            return applyPitch(newState, action.payload);

        case ActionTypes.PLAY_RESULT:
            return applyPlayResult(newState, action.payload);

        case ActionTypes.RUNNER_ADVANCE:
            return applyRunnerAdvance(newState, action.payload);

        case ActionTypes.SUBSTITUTION:
            return applySubstitution(newState, action.payload);

        case ActionTypes.LINEUP_UPDATE:
            return applyLineupUpdate(newState, action.payload);

        case ActionTypes.SCORE_OVERRIDE:
            return applyScoreOverride(newState, action.payload);

        case ActionTypes.PITCHER_UPDATE:
            return applyPitcherUpdate(newState, action.payload);

        case ActionTypes.MOVE_PLAY:
            return applyMovePlay(newState, action.payload);

        case ActionTypes.CLEAR_DATA:
            return applyClearData(newState, action.payload);

        case ActionTypes.RUNNER_BATCH_UPDATE:
            return applyRunnerBatchUpdate(newState, action.payload);

        case ActionTypes.ADD_INNING:
            return applyAddInning(newState, action.payload);

        case ActionTypes.ADD_COLUMN:
            return applyAddColumn(newState, action.payload);

        case ActionTypes.REMOVE_COLUMN:
            return applyRemoveColumn(newState, action.payload);

        case ActionTypes.GAME_METADATA_UPDATE:
            return applyGameMetadataUpdate(newState, action.payload);

        case ActionTypes.SET_INNING_LEAD:
            return applySetInningLead(newState, action.payload);

        case ActionTypes.GAME_FINALIZE:
            return applyGameFinalize(newState, action.payload);

        case ActionTypes.RBI_EDIT:
            return applyRBIEdit(newState, action.payload);

        case ActionTypes.OUT_NUM_UPDATE:
            return applyOutNumUpdate(newState, action.payload);

        case ActionTypes.MANUAL_PATH_OVERRIDE:
            return applyManualPathOverride(newState, action.payload);

        case ActionTypes.UNDO:
            // UNDO is handled by computeStateFromLog, but if passed directly here (shouldn't be in normal flow), return state.
            return newState;

        default:
            console.warn('Unknown action type:', action.type);
            return newState;
    }
}

function applyGameFinalize(state, _payload) {
    state.status = GameStatusFinal;
    return state;
}

function applyGameMetadataUpdate(state, payload) {
    return {
        ...state,
        away: payload.away || state.away,
        home: payload.home || state.home,
        awayTeamId: payload.awayTeamId !== undefined ? payload.awayTeamId : state.awayTeamId,
        homeTeamId: payload.homeTeamId !== undefined ? payload.homeTeamId : state.homeTeamId,
        date: payload.date || state.date,
        event: payload.event !== undefined ? payload.event : state.event,
        location: payload.location !== undefined ? payload.location : state.location,
        permissions: payload.permissions || state.permissions,
    };
}

function applyRunnerBatchUpdate(state, payload) {
    const { updates, activeCtx, activeTeam } = payload;
    // updates: [{ key, action, base }]
    state.events = { ...state.events };
    // Get current inning stats to determine out number baseline
    // We need to know how many outs were BEFORE this batch of updates.
    // However, the batch updates happen simultaneously conceptually.
    // Each OUT action increments the out count.
    const inningColIds = state.columns.filter(c => c.inning === activeCtx.i).map(c => c.id);
    let maxOutNum = 0;
    Object.keys(state.events).forEach(k => {
        const parts = k.split('-');
        if (parts[0] === activeTeam) {
            const colId = parts.slice(2).join('-');
            if (inningColIds.includes(colId)) {
                // Check if this key is in our updates list
                const isUpdating = updates.some(u => u.key === k);
                if (!isUpdating) {
                    const ev = state.events[k];
                    if (ev.outNum) {
                        maxOutNum = Math.max(maxOutNum, ev.outNum);
                    }
                }
            }
        }
    });

    let runningOutNum = maxOutNum;
    updates.forEach(({ key, action, base }) => {
        if (!state.events[key]) {
            state.events[key] = {
                outcome: '',
                balls: 0,
                strikes: 0,
                outNum: 0,
                paths: [0, 0, 0, 0],
                pathInfo: ['', '', '', ''],
                pitchSequence: [],
                pId: payload.batterId,
            };
        }

        // Clone event
        state.events[key] = { ...state.events[key] };
        const evt = state.events[key];

        // Clone arrays
        evt.paths = [...evt.paths];
        evt.pathInfo = [...evt.pathInfo];
        if (evt.outPos) {
            evt.outPos = [...evt.outPos];
        }
        else {
            evt.outPos = [0.5, 0.5, 0.5, 0.5];
        }

        const nextPathIdx = base + 1;
        if (nextPathIdx > 3) {
            return;
        }

        if (action === 'SB' || action === 'Adv' || action === 'Place' || action.startsWith('CR') || action.startsWith('E')) {
            evt.paths[nextPathIdx] = 1; // Safe
            evt.pathInfo[nextPathIdx] = action;
        } else if (['CS', RunnerActionOut, 'PO', 'LE', 'LB', 'INT', 'Left Early', 'Look Back', 'Int'].includes(action)) {
            // Out cases
            evt.paths[nextPathIdx] = 2; // Out
            evt.pathInfo[nextPathIdx] = action;

            let pos = 0.5;
            if (action === 'CS' || action === 'Tag' || action === 'Force') {
                pos = 0.8;
            } else if (['PO', 'LE', 'LB', 'INT', 'Left Early', 'Look Back', 'Int'].includes(action)) {
                pos = 0.2;
            }
            evt.outPos[nextPathIdx] = pos;

            runningOutNum++;
            evt.outNum = Math.min(3, runningOutNum);
        }
    });

    return state;
}

function applyClearData(state, payload) {
    const { activeCtx, activeTeam, batterId } = payload;
    const key = `${activeTeam}-${activeCtx.b}-${activeCtx.col}`;

    state.events = { ...state.events };

    // Reset the event to its initial empty state
    state.events[key] = {
        outcome: '',
        balls: 0,
        strikes: 0,
        outNum: 0,
        paths: [0, 0, 0, 0],
        pathInfo: ['', '', '', ''],
        pitchSequence: [],
        pId: batterId, // Preserve the batter ID
    };

    return state;
}

function applyMovePlay(state, payload) {
    const { sourceKey, targetKey, eventData, newColumn } = payload;

    // If a new column was created during the move logic, add it to state
    if (newColumn) {
        state.columns = [...state.columns];
        state.columns.push(newColumn);
        state.columns.sort((a, b) => {
            if (a.inning !== b.inning) {
                return a.inning - b.inning;
            }
            const aSub = parseInt(a.id.split('-')[2]);
            const bSub = parseInt(b.id.split('-')[2]);
            return aSub - bSub;
        });
    }

    state.events = { ...state.events };
    // Copy event data to target
    state.events[targetKey] = { ...eventData };
    // Clear source event
    delete state.events[sourceKey];
    return state;
}

function applyPitcherUpdate(state, payload) {
    const { team, pitcher } = payload;
    state.pitchers = { ...state.pitchers };
    state.pitchers[team] = pitcher;
    return state;
}

function applyScoreOverride(state, payload) {
    const { team, inning, score } = payload;
    state.overrides = { ...state.overrides };
    state.overrides[team] = { ...state.overrides[team] };
    state.overrides[team][inning] = score;
    return state;
}

function applyLineupUpdate(state, payload) {
    const { team, teamName, roster, subs } = payload;

    // Update team name if provided
    if (teamName) {
        state[team] = teamName;
    }

    state.roster = { ...state.roster };
    state.subs = { ...state.subs };

    // Deep clone roster and subs
    state.roster[team] = JSON.parse(JSON.stringify(roster));
    state.subs[team] = JSON.parse(JSON.stringify(subs));

    return state;
}

function applySubstitution(state, payload) {
    const { team, rosterIndex, subParams } = payload;
    // console.log('DEBUG: applySubstitution', team, rosterIndex, subParams.name);
    // Structural sharing for Roster
    state.roster = { ...state.roster };
    state.roster[team] = [...state.roster[team]];

    const slot = { ...state.roster[team][rosterIndex] };
    if (!slot.history) {
        slot.history = [];
    } else {
        slot.history = [...slot.history];
    }
    // Deep clone current before pushing to history

    slot.history.push(JSON.parse(JSON.stringify(slot.current)));

    slot.current = {
        id: subParams.id,
        name: subParams.name,
        number: subParams.number,
        pos: subParams.pos,
    };

    state.roster[team][rosterIndex] = slot;

    return state;
}

function applyRunnerAdvance(state, payload) {
    const { runners, batterId, rbiEligible, outSequencing, activeCtx, activeTeam } = payload;

    state.events = { ...state.events };

    let batterKey = null;
    let batterEvent = null;
    let inningColIds = [];

    if (activeCtx && activeTeam) {
        batterKey = `${activeTeam}-${activeCtx.b}-${activeCtx.col}`;
        batterEvent = state.events[batterKey];
        inningColIds = state.columns.filter(c => c.inning === activeCtx.i).map(c => c.id);
    }

    const runnersOut = runners.filter(r => r.outcome === RunnerActionOut);
    // Determine where the runners first out occurred
    let runnerOutStart = 0;
    if (outSequencing === 'RunnersFirst' && batterEvent && batterEvent.outNum > 0 && runnersOut.length > 0) {
        runnerOutStart = batterEvent.outNum; // Start runners at batter's original out number

        // Shift batter's out number to be after all runners
        state.events[batterKey] = {
            ...batterEvent,
            outNum: batterEvent.outNum + runnersOut.length,
        };
        // Ensure capped at 3? No, let standard logic handle cap or display handles it.
        if (state.events[batterKey].outNum > 3) {
            state.events[batterKey].outNum = 3;
        }
    }

    runners.forEach((r) => {
        const key = r.key;
        if (!state.events[key]) {
            return;
        }

        state.events[key] = { ...state.events[key] };
        const event = state.events[key];

        event.paths = [...event.paths];
        if (event.pathInfo) {
            event.pathInfo = [...event.pathInfo];
        }

        if (r.outcome === RunnerActionStay) {
            return;
        }
        if (r.outcome === RunnerActionOut) {
            event.paths[r.base + 1] = 2;
            if (event.pathInfo) {
                event.pathInfo[r.base + 1] = '';
            }

            // Assign Out Number
            if (outSequencing === 'RunnersFirst' && runnerOutStart > 0) {
                event.outNum = runnerOutStart;
                runnerOutStart++;
            } else if (activeCtx && activeTeam) {
                // Standard Sequential Order (BatterFirst or generic)
                // Calculate max out currently in inning (including the batter who is already processed)
                const inningStats = calculateInningOutsExclude(state, activeTeam, inningColIds, key);
                event.outNum = Math.min(3, inningStats + 1);
            }

        } else if (r.outcome === 'To 2nd') {
            event.paths[1] = 1;
        } else if (r.outcome === 'To 3rd') {
            if (r.base === 0) {
                event.paths[1] = 1;
            }
            event.paths[2] = 1;
        } else if (r.outcome === RunnerActionScore) {
            for (let b = r.base + 1; b <= 3; b++) {
                event.paths[b] = 1;
            }
            if (rbiEligible && batterId) {
                event.scoreInfo = { rbiCreditedTo: batterId };
            }
        }
    });

    return state;
}

function applyPlayResult(state, payload) {
    const { activeCtx, activeTeam, bipState, batterId, bipMode, hitData, runnerAdvancements } = payload;
    const key = `${activeTeam}-${activeCtx.b}-${activeCtx.col}`;
    const { res, base, type, seq } = bipState;
    // Structural Sharing
    state.events = { ...state.events };
    if (!state.events[key]) {
        state.events[key] = {
            outcome: '',
            balls: 0,
            strikes: 0,
            outNum: 0,
            paths: [0, 0, 0, 0],
            pathInfo: ['', '', '', ''],
            pitchSequence: [],
            pId: batterId,
            hitData: null, // Initialize hitData
        };
    } else {
        state.events[key] = { ...state.events[key] };
    }
    const event = state.events[key];
    if (!event.pId && batterId) {
        event.pId = batterId;
    }
    // Copy mutable arrays
    event.paths = [...event.paths];
    if (event.outPos) {
        event.outPos = [...event.outPos];
    }


    event.hitData = hitData;
    event.bipState = bipState;

    // Find batter name for pitchLog
    let batterName = 'Unknown';
    const roster = state.roster[activeTeam];
    if (roster) {
        for (const slot of roster) {
            if (slot.current && slot.current.id === batterId) {
                batterName = slot.current.name; break;
            }
            if (slot.starter && slot.starter.id === batterId) {
                batterName = slot.starter.name; break;
            }
            const hist = (slot.history || []).find(h => h.id === batterId);
            if (hist) {
                batterName = hist.name; break;
            }
        }
    }

    // Maintain pitchLog for BIP
    state.pitchLog = [...(state.pitchLog || [])];
    const defenseTeam = activeTeam === TeamAway ? TeamHome : TeamAway;
    const pitcherName = (state.pitchers && state.pitchers[defenseTeam]) ? state.pitchers[defenseTeam] : '';

    state.pitchLog.push({
        inning: activeCtx.i,
        team: activeTeam,
        batter: batterName,
        pitcher: pitcherName,
        type: 'bip',
        code: res,
        count: `${event.balls}-${event.strikes}`,
    });

    const seqStr = Array.isArray(seq) ? seq.join('-') : (seq || '');
    const seqClean = seqStr.replace(/[^0-9]/g, '');
    let out = '';

    const isAirOut = (
        res === 'Fly' ||
        res === 'Line' ||
        res === 'IFF' ||
        (res === BiPResultOut && type === 'SF') ||
        ((type === 'DP' || type === 'TP') && (res === 'Fly' || res === 'Line'))
    );

    // Count total outs in this play result (batter + existing runners)
    const runnerOutsCount = (runnerAdvancements || []).filter(a => a.outcome === RunnerActionOut).length;
    // Note: Strikeout is typically handled by PITCH, but if it comes here (Dropped 3rd), it counts.
    const isBatterOut = res !== 'Safe';
    const totalOuts = (isBatterOut ? 1 : 0) + runnerOutsCount;
    const isDP = totalOuts === 2;
    const isTP = totalOuts === 3;

    // Calculate outNum for the event if the batter is out
    if (isBatterOut) {
        const inningColIds = state.columns.filter(c => c.inning === activeCtx.i).map(c => c.id);
        const inningStats = calculateInningOutsExclude(state, activeTeam, inningColIds, key);
        if (isAirOut) {
            event.outNum = Math.min(3, inningStats + 1);
        } else {
            event.outNum = Math.min(3, inningStats + totalOuts);
        }
    } else {
        event.outNum = 0;
    }

    if (bipMode === 'dropped') {
        if (res === 'Safe') {
            out = type; // D3 or FC
            if (seqStr) {
                out += ' ' + seqStr;
            }
        } else {
            out = 'K';
            if (seqStr) {
                out += ' ' + seqStr;
            }
        }
    }
    else if (res === 'Safe') {
        if (type === 'ERR') {
            out = 'E' + (seqStr ? '-' + seqClean : '');
        }
        else if (type === 'FC') {
            out = 'FC' + (seqStr ? '-' + seqClean : '');
        }
        else if (['HBP', 'IBB', 'CI'].includes(type)) {
            out = type;
        }
        else if (base === '1B') {
            out = '1B';
        }
        else if (base === '2B') {
            out = '2B';
        }
        else if (base === '3B') {
            out = '3B';
        }
        else {
            out = 'HR';
        }
    } else {
        if (res === 'Fly') {
            out = (type === 'SF' ? 'SF' : 'F') + seqClean;
        }
        else if (res === 'Line') {
            out = 'L' + seqClean;
        }
        else if (res === 'IFF') {
            out = 'IFF' + seqClean;
        }
        else if (res === BiPResultGround || res === BiPResultOut) {
            if (['BOO', 'Int', 'SO'].includes(type)) {
                out = type;
            } else {
                out = (type === 'SH' ? 'SH' : (type === 'SF' ? 'SF' : '')) + seqStr;
            }
        }
    }

    if (isTP) {
        out = 'TP ' + out;
    }
    else if (isDP) {
        out = 'DP ' + out;
    }

    if (res === 'Safe') {
        if (base === '1B') {
            event.paths = [1, 0, 0, 0];
        }
        else if (base === '2B') {
            event.paths = [1, 1, 0, 0];
        }
        else if (base === '3B') {
            event.paths = [1, 1, 1, 0];
        }
        else if (base === 'Home') {
            event.paths = [1, 1, 1, 1];
        }
    } else if (!isAirOut) {
        if (base === '1B') {
            event.paths = [2, 0, 0, 0]; event.outPos = [0.75, 0, 0, 0];
        }
        else if (base === '2B') {
            event.paths = [1, 2, 0, 0]; event.outPos = [0, 0.75, 0, 0];
        }
        else if (base === '3B') {
            event.paths = [1, 1, 2, 0]; event.outPos = [0, 0, 0.75, 0];
        }
        else if (base === 'Home') {
            event.paths = [1, 1, 1, 2]; event.outPos = [0, 0, 0, 0.75];
        }
    }

    event.outcome = out;

    if (res === 'Safe' && base === 'Home') {
        if (!out.includes('E') && batterId) {
            event.scoreInfo = { rbiCreditedTo: batterId };
        }
    }

    if (runnerAdvancements && Array.isArray(runnerAdvancements)) {
        const outSequencing = isAirOut ? 'BatterFirst' : 'RunnersFirst';
        let runnerOutStart = 0;
        const runnersOut = runnerAdvancements.filter(r => r.outcome === RunnerActionOut);
        if (outSequencing === 'RunnersFirst' && event.outNum > 0 && runnersOut.length > 0) {
            runnerOutStart = event.outNum - runnersOut.length;
        }

        runnerAdvancements.forEach((r) => {
            const rKey = r.key;
            if (!state.events[rKey]) {
                return;
            }
            state.events[rKey] = { ...state.events[rKey] };
            const rev = state.events[rKey];
            rev.paths = [...rev.paths];

            if (r.outcome === RunnerActionOut) {
                rev.paths[r.base + 1] = 2;
                if (outSequencing === 'RunnersFirst' && runnerOutStart > 0) {
                    rev.outNum = runnerOutStart++;
                }
                else {
                    const inningColIds = state.columns.filter(c => c.inning === activeCtx.i).map(c => c.id);
                    const inningStats = calculateInningOutsExclude(state, activeTeam, inningColIds, rKey);
                    rev.outNum = Math.min(3, inningStats + 1);
                }
            } else if (r.outcome === 'To 2nd') {
                rev.paths[1] = 1;
                //rev.pathInfo[1] = 'Adv';
            }
            else if (r.outcome === 'To 3rd') {
                if (r.base === 0) {
                    rev.paths[1] = 1;
                }
                rev.paths[2] = 1;
                //rev.pathInfo[2] = 'Adv';
            } else if (r.outcome === RunnerActionScore) {
                for (let b = r.base + 1; b <= 3; b++) {
                    rev.paths[b] = 1;
                }
                //rev.pathInfo[3] = 'Adv';
                const rbiEligible = res === 'Safe' || !(isDP || isTP);
                if (rbiEligible && batterId) {
                    rev.scoreInfo = { rbiCreditedTo: batterId };
                }
            }
        });
    }

    return state;
}

function applyGameImport(state, payload) {
    return {
        ...state,
        id: payload.id,
        date: payload.date,
        event: payload.event || '',
        location: payload.location || '',
        away: payload.away,
        home: payload.home,
        ...payload, // Import all other fields
    };
}


function applyGameStart(state, payload) {
    const newState = {
        ...state,
        id: payload.id,
        schemaVersion: payload.schemaVersion || CurrentSchemaVersion,
        date: payload.date,
        event: payload.event || '',
        location: payload.location || '',
        away: payload.away,
        home: payload.home,
        awayTeamId: payload.awayTeamId,
        homeTeamId: payload.homeTeamId,
        ownerId: payload.ownerId || null,
        permissions: payload.permissions || { public: 'none', users: {} },
    };

    // Initialize Columns
    newState.columns = [];
    for (let i = 1; i <= 5; i++) {
        newState.columns.push({ inning: i, id: `col-${i}-0` });
    }

    // Initialize Roster
    newState.roster = { away: [], home: [] };
    newState.subs = { away: [], home: [] };
    [TeamAway, TeamHome].forEach((team) => {
        const initialRoster = payload.initialRosters ? payload.initialRosters[team] : null;
        if (payload.initialSubs && payload.initialSubs[team]) {
            newState.subs[team] = JSON.parse(JSON.stringify(payload.initialSubs[team]));
        }

        newState.roster[team] = Array(9).fill(0).map((_, i) => {
            let pInfo = (initialRoster && initialRoster[i]) ? initialRoster[i] : null;

            const id = pInfo ? pInfo.id : (payload.initialRosterIds ? payload.initialRosterIds[team][i] : '');
            const name = pInfo ? pInfo.name : `${team === TeamAway ? 'Player' : 'H Player'} ${i + 1}`;
            const num = pInfo ? pInfo.number : `${i + 1}`;
            const pos = pInfo ? (pInfo.pos || '') : '';

            const playerObj = { id: id, name: name, number: num, pos: pos };
            return {
                slot: i + 1,
                starter: playerObj,
                current: playerObj,
                history: [],
            };
        });
    });

    return newState;
}

function applyPitch(state, payload) {
    const { activeCtx, type, code, activeTeam, batterId } = payload;
    const key = `${activeTeam}-${activeCtx.b}-${activeCtx.col}`;
    state.events = { ...state.events };
    if (!state.events[key]) {
        state.events[key] = {
            outcome: '',
            balls: 0,
            strikes: 0,
            outNum: 0,
            paths: [0, 0, 0, 0],
            pathInfo: ['', '', '', ''],
            pitchSequence: [],
            pId: batterId,
        };
    } else {
        state.events[key] = { ...state.events[key] };
    }

    const event = state.events[key];
    if (!event.pId && batterId) {
        event.pId = batterId;
    }
    // Copy mutable sub-objects
    event.pitchSequence = [...event.pitchSequence];
    event.paths = [...event.paths];
    event.pathInfo = [...event.pathInfo];
    // Determine pitcher (defense)
    const defense = activeTeam === TeamAway ? TeamHome : TeamAway;
    const pitcher = state.pitchers[defense] || ''; // Should be passed or in state

    // Find batter name for pitchLog
    let batterName = 'Unknown';
    const roster = state.roster[activeTeam];
    if (roster) {
        for (const slot of roster) {
            if (slot.current && slot.current.id === batterId) {
                batterName = slot.current.name; break;
            }
            if (slot.starter && slot.starter.id === batterId) {
                batterName = slot.starter.name; break;
            }
            const hist = (slot.history || []).find(h => h.id === batterId);
            if (hist) {
                batterName = hist.name; break;
            }
        }
    }

    event.pitchSequence.push({ type, code, pitcher });
    // Recalc Count
    let b = 0, s = 0;
    event.pitchSequence.forEach((p) => {
        if (p.type === PitchTypeBall) {
            if (b < 4) {
                b++;
            }
        } else if (p.type === PitchTypeStrike) {
            if (s < 3) {
                s++;
            }
        } else if (p.type === PitchTypeFoul) {
            if (s < 2) {
                s++;
            }
        }
    });

    event.balls = b;
    event.strikes = s;

    // Maintain pitchLog
    state.pitchLog = [...(state.pitchLog || [])];
    state.pitchLog.push({
        inning: activeCtx.i,
        team: activeTeam,
        batter: batterName,
        pitcher: pitcher,
        type: type,
        code: code,
        count: `${event.balls}-${event.strikes}`,
    });

    // Check Outcomes
    if (b >= 4) {
        event.outcome = 'BB';
        event.paths[0] = 1; // Safe at 1st
    } else if (s >= 3) {
        const last = event.pitchSequence[event.pitchSequence.length - 1];
        const isCalled = last && last.type === PitchTypeStrike && last.code === PitchCodeCalled;
        const isDropped = last && last.type === PitchTypeStrike && last.code === PitchCodeDropped;
        event.outcome = isCalled ? 'ꓘ' : 'K';

        if (!isDropped) {
            const inningStats = calculateInningOutsExclude(state, activeTeam, state.columns.filter(c => c.inning === activeCtx.i).map(c => c.id), key);
            event.outNum = Math.min(3, inningStats + 1);
        }
    }
    return state;
}

function applyAddInning(state) {
    state.columns = [...state.columns];
    // Find max inning
    let maxInning = 0;
    state.columns.forEach(c => {
        if (c.inning > maxInning) {
            maxInning = c.inning;
        }
    });
    const nextInning = maxInning + 1;
    state.columns.push({ inning: nextInning, id: `col-${nextInning}-0` });
    return state;
}

function applyAddColumn(state, payload) {
    const { targetInning, team } = payload;
    state.columns = [...state.columns];

    // Find all columns for this inning to determine next sub-index
    const inningCols = state.columns.filter(c => c.inning === targetInning);
    let maxSub = -1;
    inningCols.forEach(c => {
        const parts = c.id.split('-');
        const sub = parseInt(parts[2] || 0);
        if (sub > maxSub) {
            maxSub = sub;
        }
    });

    const nextSub = maxSub + 1;
    const newCol = { inning: targetInning, id: `col-${targetInning}-${nextSub}`, team };

    state.columns.push(newCol);
    // Sort columns: Inning ASC, then Sub ASC
    state.columns.sort((a, b) => {
        if (a.inning !== b.inning) {
            return a.inning - b.inning;
        }
        const aSub = parseInt(a.id.split('-')[2] || 0);
        const bSub = parseInt(b.id.split('-')[2] || 0);
        return aSub - bSub;
    });

    return state;
}

function applyRemoveColumn(state, payload) {
    const { colId, team } = payload;

    // Find column
    const colIndex = state.columns.findIndex(c => c.id === colId);
    if (colIndex === -1) {
        return state;
    }
    const col = state.columns[colIndex];

    // Validation: Ensure at least one column remains for this inning for this team
    const inning = col.inning;
    const teamColsForInning = state.columns.filter(c =>
        c.inning === inning && (!c.team || c.team === team) && c.id !== colId,
    );

    if (teamColsForInning.length === 0) {
        // This is the last column for this team in this inning. Cannot remove.
        return state;
    }

    // Validate: Cannot remove if data exists for this team
    const hasRecordedData = Object.keys(state.events).some((key) => {
        const parts = key.split('-');
        // Check if event belongs to the active team
        if (parts[0] !== team) {
            return false;
        }

        const eventColId = parts.slice(2).join('-');
        if (eventColId !== colId) {
            return false;
        }
        const event = state.events[key];
        return event && (event.outcome || (event.pitchSequence && event.pitchSequence.length > 0));
    });

    if (hasRecordedData) {
        return state;
    }

    // Update columns
    state.columns = [...state.columns];
    if (col.team === team) {
        // Column is specific to this team: remove it
        state.columns.splice(colIndex, 1);
    } else if (!col.team) {
        // Column is shared: convert it to the other team
        const otherTeam = team === TeamAway ? TeamHome : TeamAway;
        state.columns[colIndex] = { ...col, team: otherTeam };
    } else {
        // Column belongs to other team (should not happen via UI)
        return state;
    }

    // Clean up empty events associated with this column for this team
    const newEvents = {};
    for (const key in state.events) {
        const parts = key.split('-');
        const eventTeam = parts[0];
        const eventColId = parts.slice(2).join('-');
        if (eventColId === colId && eventTeam === team) {
            // Remove event
            continue;
        }
        newEvents[key] = state.events[key];
    }
    state.events = newEvents;

    return state;
}

function calculateInningOutsExclude(state, team, inningColIds, excludeKey) {
    let outs = 0;
    Object.keys(state.events).forEach((k) => {
        const parts = k.split('-');
        if (parts[0] === team) {
            const colId = parts.slice(2).join('-');
            if (inningColIds.includes(colId)) {
                if (k !== excludeKey) {
                    const ev = state.events[k];
                    if (ev.outNum) {
                        outs = Math.max(outs, ev.outNum);
                    }
                }
            }
        }
    });
    return outs;
}

function applySetInningLead(state, payload) {
    const { team, colId, rowId } = payload;

    state.columns = state.columns.map(col => {
        if (col.id === colId) {
            const leadRow = col.leadRow ? { ...col.leadRow } : {};
            if (rowId !== null) {
                leadRow[team] = rowId;
            } else {
                delete leadRow[team];
            }
            return { ...col, leadRow };
        }
        return col;
    });

    return state;
}

function applyRBIEdit(state, payload) {
    const { key, rbiCreditedTo } = payload;
    if (!state.events[key]) {
        return state;
    }
    state.events = { ...state.events };
    state.events[key] = { ...state.events[key] };
    const evt = state.events[key];
    evt.scoreInfo = { ...(evt.scoreInfo || {}) };
    if (rbiCreditedTo) {
        evt.scoreInfo.rbiCreditedTo = rbiCreditedTo;
    } else {
        delete evt.scoreInfo.rbiCreditedTo;
    }
    return state;
}

function applyOutNumUpdate(state, payload) {
    const { key, outNum } = payload;
    if (!state.events[key]) {
        return state;
    }
    state.events = { ...state.events };
    state.events[key] = { ...state.events[key], outNum };
    return state;
}

function applyManualPathOverride(state, payload) {
    const { key, data } = payload;
    state.events = { ...state.events };
    if (!state.events[key]) {
        state.events[key] = {
            outcome: '',
            balls: 0,
            strikes: 0,
            outNum: 0,
            paths: [0, 0, 0, 0],
            pathInfo: ['', '', '', ''],
            pitchSequence: [],
            pId: '',
        };
    }
    state.events[key] = { ...state.events[key], ...data };
    if (data.pId) {
        state.events[key].pId = data.pId;
    }
    return state;
}

