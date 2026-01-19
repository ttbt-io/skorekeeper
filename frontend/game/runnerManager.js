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

import {
    ActionTypes,
} from '../reducer.js';
import {
    RunnerActionScore,
    RunnerActionStay,
    RunnerActionOut,
    BiPResultFly,
    BiPResultLine,
    BiPResultIFF,
} from '../constants.js';

/**
 * Manages runner state and the Runner Action/Advance views.
 */
export class RunnerManager {
    /**
     * @param {object} options
     * @param {Function} options.dispatch - App dispatch function.
     * @param {Function} options.renderCSO - Re-render CSO.
     * @param {Function} options.getBatterId - Helper to get current batter ID.
     */
    constructor({ dispatch, renderCSO, getBatterId }) {
        this.dispatch = dispatch;
        this.renderCSO = renderCSO;
        this.getBatterId = getBatterId;
    }

    /**
     * Applies a selected runner action, handling prompts and dispatching updates.
     * @param {object} state - Current app state.
     * @param {number} idx - Base index.
     * @param {string} code - Action code.
     * @param {Function} modalPromptFn - Function to show modal prompt.
     * @returns {Promise<boolean>} True if action applied, false if cancelled.
     */
    async applyRunnerAction(state, idx, code, modalPromptFn) {
        if (state.isReadOnly) {
            return false;
        }
        let final = code;
        if (code === 'CR') {
            const n = await modalPromptFn('CR #:', '');
            if (n !== null && n.trim() !== '') {
                final = `CR ${n.trim()}`;
            } else {
                return false;
            }
        } else if (code === 'Err') {
            const pos = await modalPromptFn('Error Attribution (Fielder Pos #):', '');
            if (pos !== null && pos.trim() !== '') {
                final = `E-${pos.trim()}`;
            } else if (pos === null) {
                return false;
            } else {
                final = 'E';
            }
        }

        const k = `${state.activeTeam}-${state.activeCtx.b}-${state.activeCtx.col}`;

        // Determine if this action refers to the path TO the current base (idx)
        // or the path FROM the current base TO the next one (idx+1).
        // Reducer always adds +1 to 'base' to get nextPathIdx.
        // So if we want to update pathInfo[idx], we need base = idx - 1.

        const baseVal = idx - 1;

        await this.dispatch({
            type: ActionTypes.RUNNER_BATCH_UPDATE,
            payload: {
                updates: [{
                    key: k,
                    action: final,
                    base: baseVal,
                }],
                activeCtx: state.activeCtx,
                activeTeam: state.activeTeam,
                batterId: this.getBatterId ? this.getBatterId() : null,
            },
        });

        return true;
    }

    /**
     * Identifies which runners are currently on base.
     */
    /**
     * Identifies which runners are currently on base.
     */
    getRunnersOnBase(game, team, ctx) {
        const runners = [];
        const roster = game.roster[team];
        const inningCols = game.columns
            .filter(c => c.inning === ctx.i)
            .map(c => c.id);


        roster.forEach((p, idx) => {
            if (idx === ctx.b) {
                return;
            }

            let d = null;
            let key = '';
            for (let i = inningCols.length - 1; i >= 0; i--) {
                const k = `${team}-${idx}-${inningCols[i]}`;
                if (game.events[k]) {
                    d = game.events[k];
                    key = k;
                    break;
                }
            }

            if (!d) {
                return;
            }


            let base = -1;
            if (d.paths[3] === 1 || d.paths[3] === 2) {
                return;
            }

            if (d.paths[2] === 1) {
                if (d.paths[3] === 0) {
                    base = 2;
                }
            } else if (d.paths[2] === 2) {
                return;
            }

            if (base === -1) {
                if (d.paths[1] === 1) {
                    if (d.paths[2] === 0) {
                        base = 1;
                    }
                } else if (d.paths[1] === 2) {
                    return;
                }
            }

            if (base === -1) {
                if (d.paths[0] === 1) {
                    if (d.paths[1] === 0) {
                        base = 0;
                    }
                } else if (d.paths[0] === 2) {
                    return;
                }
            }

            if (base >= 0) {
                runners.push({ idx, name: p.current.name, base, key: key });
            }
        });
        // Sort runners by base (descending) so lead runner appears first regardless of lineup order
        runners.sort((a, b) => b.base - a.base);
        return runners;
    }

    /**
     * Cycles through possible outcomes for a runner in the advance view.
     */
    cycleAdvance(idx, pendingRunnerState, options) {
        const r = pendingRunnerState[idx];
        const curIdx = options.indexOf(r.outcome);
        r.outcome = options[(curIdx + 1) % options.length];
    }

    /**
     * Cycles through possible actions for a runner in the action menu.
     */
    cycleAction(idx, pendingRunnerState, options) {
        const r = pendingRunnerState[idx];
        const curIdx = options.indexOf(r.action);
        r.action = options[(curIdx + 1) % options.length];
    }

    /**
     * Logic for calculating default advancements when opening the advance view.
     */
    calculateDefaultAdvances(runners, bip, events, currentOuts = 0) {
        // 1. Initial Mapping & Edit-Mode Loading
        const pending = runners.map((r) => {
            let outcome = RunnerActionStay;
            const event = events[r.key];
            if (event && event.paths && (event.outcome || (event.pitchSequence && event.pitchSequence.length > 0))) {
                if (event.paths[3] === 1) {
                    outcome = RunnerActionScore;
                }
                else if (event.paths[2] === 1 && r.base < 2) {
                    outcome = 'To 3rd';
                }
                else if (event.paths[1] === 1 && r.base < 1) {
                    outcome = 'To 2nd';
                }
                else if (event.paths[r.base + 1] === 2) {
                    outcome = RunnerActionOut;
                }
            }
            return { ...r, outcome };
        });

        // Check for potential 3rd out on the batter (Fly, Line, IFF, Out)
        // Note: RunnerActionOut usually means Ground Out or similar where batter is out at 1st.
        // If batter is out and it makes the 3rd out, runners default to Stay.
        const batterIsOut = [BiPResultFly, BiPResultLine, BiPResultIFF, RunnerActionOut].includes(bip.res);
        if (batterIsOut && currentOuts >= 2) {
            return pending;
        }

        // 2. Default Advancements (only apply to RunnerActionStay runners)
        const getRS = rKey => pending.find(pr => pr.key === rKey);

        const isAirOut = [BiPResultFly, BiPResultLine, BiPResultIFF].includes(bip.res);

        if (['BB', 'IBB', 'HBP', 'CI'].includes(bip.type)) {
            const onBase = [null, null, null];
            runners.forEach(r => {
                if (r.base >= 0 && r.base <= 2) {
                    onBase[r.base] = r;
                }
            });

            if (onBase[0]) {
                const rs1 = getRS(onBase[0].key);
                if (rs1?.outcome === RunnerActionStay) {
                    rs1.outcome = 'To 2nd';
                }
                if (onBase[1]) {
                    const rs2 = getRS(onBase[1].key);
                    if (rs2?.outcome === RunnerActionStay) {
                        rs2.outcome = 'To 3rd';
                    }
                    if (onBase[2]) {
                        const rs3 = getRS(onBase[2].key);
                        if (rs3?.outcome === RunnerActionStay) {
                            rs3.outcome = RunnerActionScore;
                        }
                    }
                }
            }
        } else if (!isAirOut) {
            let advance = 1;

            if (bip.res === 'Safe' && ['1B', '2B', '3B', 'Home'].includes(bip.base)) {
                const map = { '1B': 1, '2B': 2, '3B': 3, 'Home': 4 };
                advance = map[bip.base];
            } else if (bip.type === 'SF' || bip.type === 'SH') {
                advance = 1;
            }

            // Identify Forced Runners for Out/Ground scenarios
            const onBase = [null, null, null];
            runners.forEach(r => {
                if (r.base < 3) {
                    onBase[r.base] = r;
                }
            });
            const forced = [false, false, false];

            // Force chain starts at 1st base for ground balls/safe hits
            if (onBase[0]) {
                forced[0] = true;
            }
            if (onBase[1] && forced[0]) {
                forced[1] = true;
            }
            if (onBase[2] && forced[1]) {
                forced[2] = true;
            }

            runners.forEach((r) => {
                const rs = getRS(r.key);
                if (rs?.outcome === RunnerActionStay) {
                    let shouldAdvance = false;

                    if (bip.res === 'Safe' || bip.type === 'SF' || bip.type === 'SH') {
                        shouldAdvance = true; // Hits/Sacrifices advance everyone by default
                    } else {
                        // Ground Outs / Dropped 3rd / FC / Error -> Only advance forced runners
                        // EXCEPTION: Generic Outs (Out, Int, BOO, etc) should NOT advance runners
                        // These are typically procedural outs where runners stay put unless explicit action is taken.
                        const isGenericOut = bip.res === 'Out' && ['Out', 'Int', 'BOO', 'SO', 'Interference'].includes(bip.type);

                        if (!isGenericOut && r.base >= 0 && r.base <= 2 && forced[r.base]) {
                            shouldAdvance = true;
                        }
                    }

                    if (shouldAdvance) {
                        const nb = r.base + advance;
                        if (nb === 1) {
                            rs.outcome = 'To 2nd';
                        }
                        else if (nb === 2) {
                            rs.outcome = 'To 3rd';
                        }
                        else if (nb >= 3) {
                            rs.outcome = RunnerActionScore;
                        }
                    }
                }
            });
        }

        return pending;
    }
}