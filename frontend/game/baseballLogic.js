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
 * BACKWARD COMPATIBILITY NOTE:
 * This rewriter allows the legacy grid-based UI to function with the new 1D timeline.
 * It will be safe to remove this logic ONLY after:
 * 1. The entire UI is refactored to use stable PA IDs (paId) instead of grid coordinates (team-b-col).
 * 2. All scoresheet and CSO components are updated to look up events by paId.
 *
 * TO REMOVE:
 * - Delete this function.
 * - Remove its usage in reducer.js (computeStateFromLog).
 *
 * @param {Array} timeline - The ordered timeline of effective actions.
 * @returns {Array} A new array of actions with rewritten payloads.
 */
export function reassignGridCoordinates(timeline) {
    // We need to track the logical state of the game to assign correct grid coordinates.
    const state = {
        away: { batterIndex: 0, inning: 1, outs: 0, runs: 0, columnId: 'col-1-0', strikes: 0, batterIsOut: false },
        home: { batterIndex: 0, inning: 1, outs: 0, runs: 0, columnId: 'col-1-0', strikes: 0, batterIsOut: false },
        activeTeam: TeamAway,
        currentInning: 1,
        rosterSizes: { away: 9, home: 9 },
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
    // Map of legacy runner keys (team-b-col) -> new runner keys (team-b-col)
    const runnerKeyMapping = new Map();

    return timeline.map(action => {
        // Deep clone the action so we can mutate its payload
        const newAction = JSON.parse(JSON.stringify(action));
        const payload = newAction.payload || {};

        if (newAction.type === ActionTypes.GAME_METADATA_UPDATE ||
            newAction.type === ActionTypes.PITCHER_UPDATE) {
            return newAction;
        }

        if (newAction.type === ActionTypes.GAME_START || newAction.type === ActionTypes.GAME_IMPORT) {
            ['away', 'home'].forEach(team => {
                if (payload.initialRosters && payload.initialRosters[team]) {
                    state.rosterSizes[team] = payload.initialRosters[team].length;
                } else {
                    // Default to 9 if roster is not explicitly provided in payload
                    state.rosterSizes[team] = 9;
                }
            });
            return newAction;
        }

        if (newAction.type === ActionTypes.LINEUP_UPDATE) {
            if (payload.team && payload.lineup) {
                state.rosterSizes[payload.team] = payload.lineup.length;
            }
            return newAction;
        }

        if (newAction.type === ActionTypes.ADD_COLUMN) {
            if (payload.team && payload.colId) {
                state[payload.team].columnId = payload.colId;
            }
            return newAction;
        }

        if (newAction.type === ActionTypes.SET_INNING_LEAD) {
            // Apply manual inning lead overrides to our tracker
            if (payload.team && payload.rowId !== undefined && payload.rowId !== null) {
                state[payload.team].batterIndex = payload.rowId;
                state[payload.team].justSetLead = true;
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
                    state[team].strikes = 0; // Reset strikes for new PA
                    state[team].batterIsOut = false; // Reset out flag for new PA

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
                        if (state[team].justSetLead) {
                            // Lead was explicitly set, do not auto-increment this time
                            state[team].justSetLead = false;
                        } else {
                            const prevIndex = state[team].batterIndex;
                            const rLen = state.rosterSizes[team] || 9;
                            state[team].batterIndex = (state[team].batterIndex + 1) % rLen;

                            if (state[team].batterIndex === 0 && prevIndex === rLen - 1) {
                                // Batted around, increment column suffix
                                const parts = state[team].columnId.split('-');
                                const suffix = parseInt(parts[2] || '0', 10) + 1;
                                state[team].columnId = `col-${state[team].inning}-${suffix}`;
                            }
                        }
                    }

                    // Save mapping
                    ctxMapping.set(legacyKey, {
                        b: state[team].batterIndex,
                        i: state[team].inning,
                        col: state[team].columnId,
                    });

                    const legacyRunnerKey = `${team}-${payload.activeCtx.b}-${payload.activeCtx.col}`;
                    const newRunnerKey = `${team}-${state[team].batterIndex}-${state[team].columnId}`;
                    runnerKeyMapping.set(legacyRunnerKey, newRunnerKey);

                    lastLegacyContext[team] = legacyKey;
                }
            }

            // Apply the mapped logical context
            const logicalCtx = ctxMapping.get(legacyKey);
            if (logicalCtx) {
                payload.activeCtx = { ...logicalCtx };
            }

            // Rewrite runner advancements keys to match the new logical coordinates
            if (payload.runnerAdvancements) {
                payload.runnerAdvancements.forEach(runner => {
                    const mappedKey = runnerKeyMapping.get(runner.key);
                    if (mappedKey) {
                        runner.key = mappedKey;
                    }
                });
            }

            // Track outs to know when inning ends
            if (newAction.type === ActionTypes.PITCH) {
                if (payload.type === 'strike') {
                    state[team].strikes++;
                } else if (payload.type === 'foul' && state[team].strikes < 2) {
                    state[team].strikes++;
                }

                // If strikes hit 3, count as out
                if (state[team].strikes === 3) {
                    state[team].outs++;
                    state[team].batterIsOut = true;
                    // We increment to 4 just to prevent it from counting multiple outs if there are bugged extra strikes
                    state[team].strikes++;
                }
            } else if (newAction.type === ActionTypes.PLAY_RESULT) {
                if (payload.bipState) {
                    const isBatterOut = payload.bipState.res !== 'Safe' && !state[team].batterIsOut;
                    if (isBatterOut) {
                        state[team].batterIsOut = true;
                    }
                    const runnerOuts = (payload.runnerAdvancements || []).filter(r => r.outcome === RunnerActionOut).length;
                    state[team].outs += (isBatterOut ? 1 : 0) + runnerOuts;
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
