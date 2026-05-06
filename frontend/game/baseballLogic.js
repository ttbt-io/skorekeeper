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

import { ActionTypes } from '../reducer.js';
import { TeamAway, TeamHome, RunnerActionOut } from '../constants.js';

/**
 * Phase 2: Action Rewriter.
 * Takes the chronological 1D Game Timeline and recalculates the `activeCtx`
 * for all gameplay actions. This effectively translates the logical chronological
 * order back into grid coordinates for the legacy `gameReducer`.
 *
 * @param {Array} timeline - The ordered timeline of effective actions.
 * @returns {Array} A new array of actions with rewritten payloads.
 */
export function reassignGridCoordinates(timeline) {
    // We need to track the logical state of the game to assign correct grid coordinates.
    const state = {
        away: { batterIndex: 0, inning: 1, outs: 0, runs: 0, columnId: 'col-1-0' },
        home: { batterIndex: 0, inning: 1, outs: 0, runs: 0, columnId: 'col-1-0' },
        activeTeam: TeamAway,
        currentInning: 1,
    };

    // To group actions into Plate Appearances (PAs), we detect when the underlying
    // "intent" was to start a new PA.
    // We can assume a PA ends when the batter gets out, gets a hit, walks, or when 3 outs are reached.
    // However, the safest way to group legacy actions is by looking at when the legacy `activeCtx.b`
    // or `batterId` changes from the previous action.

    let lastLegacyContext = { away: null, home: null };

    // We maintain a map of Legacy Ctx String -> New Logical Ctx
    // This handles actions that belong to the same PA.
    const ctxMapping = new Map();

    return timeline.map(action => {
        // Deep clone the action so we can mutate its payload
        const newAction = JSON.parse(JSON.stringify(action));
        const payload = newAction.payload || {};

        if (newAction.type === ActionTypes.GAME_METADATA_UPDATE ||
            newAction.type === ActionTypes.LINEUP_UPDATE ||
            newAction.type === ActionTypes.GAME_START ||
            newAction.type === ActionTypes.GAME_IMPORT ||
            newAction.type === ActionTypes.PITCHER_UPDATE) {
            return newAction;
        }

        if (newAction.type === ActionTypes.SET_INNING_LEAD) {
            // Apply manual inning lead overrides to our tracker
            if (payload.team && payload.rowId !== undefined && payload.rowId !== null) {
                state[payload.team].batterIndex = payload.rowId;
            }
            return newAction;
        }

        // For gameplay actions, rewrite activeCtx
        if (payload.activeCtx && payload.activeTeam) {
            const team = payload.activeTeam;
            const legacyKey = `${payload.activeCtx.i}-${payload.activeCtx.col}-${payload.activeCtx.b}`;

            // Check if we need to advance to a new PA
            // We advance if this action has a different legacy context than the last one we saw for this team
            if (lastLegacyContext[team] !== legacyKey) {

                if (ctxMapping.has(legacyKey)) {
                    // This is an out-of-order edit to a Plate Appearance we already established!
                    // We just switch our tracker to point to it, without advancing the game state.
                    lastLegacyContext[team] = legacyKey;
                } else {
                    // This is a truly NEW Plate Appearance!

                    // Have we reached 3 outs? Move to next half-inning.
                    if (state[team].outs >= 3) {
                        state[team].outs = 0;
                        state[team].inning++;
                        state[team].columnId = `col-${state[team].inning}-0`;

                        if (team === TeamHome) {
                            state.currentInning++;
                        }
                    }

                    // If this isn't the very first PA, advance the batter index
                    if (lastLegacyContext[team] !== null) {
                        state[team].batterIndex = (state[team].batterIndex + 1) % 9;
                    }

                    // Save mapping
                    ctxMapping.set(legacyKey, {
                        b: state[team].batterIndex,
                        i: state[team].inning,
                        col: state[team].columnId,
                    });

                    lastLegacyContext[team] = legacyKey;
                }
            }

            // Apply the mapped logical context
            const logicalCtx = ctxMapping.get(legacyKey);
            if (logicalCtx) {
                payload.activeCtx = { ...logicalCtx };
            }

            // Track outs to know when inning ends
            if (newAction.type === ActionTypes.PITCH) {
                // Heuristic: If it's a Strikeout or out, increment out (for tracking only)
                // We don't need perfect out tracking here, just enough to break innings if
                // the user didn't explicitly use SET_INNING_LEAD.
                // Actually, the reducer calculates outs perfectly. We just need to know if
                // we should advance the inning.
            } else if (newAction.type === ActionTypes.PLAY_RESULT) {
                if (payload.bipState && payload.bipState.res !== 'Safe') {
                    // An out occurred
                    let outsOnPlay = 1;
                    if (payload.bipState.type === 'DP') {
                        outsOnPlay = 2;
                    }
                    if (payload.bipState.type === 'TP') {
                        outsOnPlay = 3;
                    }
                    state[team].outs += outsOnPlay;
                }
            } else if (newAction.type === ActionTypes.RUNNER_ADVANCE) {
                if (payload.runners) {
                    payload.runners.forEach(r => {
                        if (r.outcome === RunnerActionOut) {
                            state[team].outs++;
                        }
                    });
                }
            }
        }

        return newAction;
    });
}
