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

import { ActionTypes, gameReducer, getInitialState } from '../reducer.js';

/**
 * HistoryManager transforms the raw action log into a stable, linear representation
 * of the game's events, handling undos and corrections robustly.
 */
export class HistoryManager {
    constructor({ dispatch }) {
        this.dispatch = dispatch;
    }

    /**
     * Generates a linear history from the game's action log.
     * @param {object} game - The game object containing the action log and roster.
     * @returns {Array} An array of HistoryItem objects.
     */
    generateLinearHistory(game) {
        if (!game || !game.actionLog) {
            return [];
        }

        // Pass 1: Undo Reduction
        const effectiveLog = this.getEffectiveLog(game.actionLog);

        // Pass 2: Replay and Insertion
        let history = [];
        const contextMap = new Map(); // ctxKey -> number of items seen for this context
        let currentCtxKey = null;

        effectiveLog.forEach(action => {
            const payload = action.payload || {};

            if (action.type === ActionTypes.MOVE_PLAY) {
                // MOVE_PLAY strikes the source and generates in the target.
                // We need to resolve full ctxKeys for both.
                const findFullCtxKey = (key) => {
                    const item = history.find(h => h.ctxKey.endsWith('-' + key));
                    return item ? item.ctxKey : null;
                };

                const sourceCtxKey = findFullCtxKey(payload.sourceKey);
                if (sourceCtxKey) {
                    const sourceItems = history.filter(h => h.ctxKey === sourceCtxKey);
                    if (sourceItems.length > 0) {
                        sourceItems[sourceItems.length - 1].isStricken = true;
                    }
                }

                // Treat target as a new generative action
                // targetKey format: team-b-col. We need to prepend inning.
                // Use currentCtxKey's inning as a guess if not found?
                // Actually, MOVE_PLAY should probably carry activeCtx for the target.
                // If it doesn't, we infer from sourceCtxKey.
                let targetCtxKey = payload.targetKey ? findFullCtxKey(payload.targetKey) : null;
                if (!targetCtxKey && sourceCtxKey && payload.targetKey) {
                    const inning = sourceCtxKey.split('-')[0];
                    targetCtxKey = `${inning}-${payload.targetKey}`;
                }

                if (targetCtxKey) {
                    const count = contextMap.get(targetCtxKey) || 0;
                    if (count > 0) {
                        const contextItems = history.filter(h => h.ctxKey === targetCtxKey);
                        const latestItem = contextItems[contextItems.length - 1];
                        latestItem.isStricken = true;
                    }
                    const newItem = this._createHistoryItem(targetCtxKey, [action], count, count > 0);
                    history.push(newItem);
                    contextMap.set(targetCtxKey, count + 1);
                    currentCtxKey = targetCtxKey;
                }
                return;
            }

            let ctxKey = payload.activeCtx
                ? `${payload.activeCtx.i}-${payload.activeTeam}-${payload.activeCtx.b}-${payload.activeCtx.col}`
                : currentCtxKey;

            if (!ctxKey) {
                return;
            }

            currentCtxKey = ctxKey;
            const count = contextMap.get(ctxKey) || 0;

            if (count === 0) {
                const item = this._createHistoryItem(ctxKey, [action], 0, false);
                history.push(item);
                contextMap.set(ctxKey, 1);
            } else {
                const isGenerativeAction = (a) => a.type === ActionTypes.PLAY_RESULT || a.type === ActionTypes.CLEAR_DATA;

                const isGenerative = isGenerativeAction(action);
                const contextItems = history.filter(h => h.ctxKey === ctxKey);
                const latestItem = contextItems[contextItems.length - 1];

                if (!latestItem) {
                    // Should not happen if count > 0, but safe guard
                    const item = this._createHistoryItem(ctxKey, [action], 0, false);
                    history.push(item);
                    contextMap.set(ctxKey, 1);
                    return;
                }

                const hasGenerative = latestItem.events.some(a => isGenerativeAction(a));
                const latestIsCleared = latestItem.events.some(e => e.type === ActionTypes.CLEAR_DATA);

                if ((isGenerative && hasGenerative) || latestIsCleared) {
                    // Only strike the previous play if the NEW action is what's clearing it.
                    // If we already have a ActionTypes.CLEAR_DATA block, we don't strike IT, we just move on.
                    if (isGenerative && hasGenerative && !latestIsCleared) {
                        latestItem.isStricken = true;
                        if (action.type === ActionTypes.CLEAR_DATA) {
                            latestItem.wasCleared = true;
                        }
                    }

                    const newItem = this._createHistoryItem(ctxKey, [action], count, true);
                    history.push(newItem);
                    contextMap.set(ctxKey, count + 1);
                } else {
                    latestItem.events.push(action);
                }
            }
        });

        history = this.insertHeaders(history, game);
        this.propagateStates(history, game);

        if (game.status === 'final') {
            const lastItem = history[history.length - 1];
            const finalState = lastItem ? (lastItem.stateAfter || lastItem.stateBefore) : { score: { away: 0, home: 0 } };
            history.push({
                id: 'final-summary',
                ctxKey: 'FINAL',
                type: 'SUMMARY',
                isStricken: false,
                isCorrection: false,
                actions: [],
                stateBefore: finalState,
                stateAfter: finalState,
            });
        }

        return history;
    }

    /**
     * Helper to create a new HistoryItem object.
     * @private
     */
    _createHistoryItem(ctxKey, events, count, isCorrection) {
        return {
            id: `${ctxKey}-${count}`,
            ctxKey,
            type: 'PLAY',
            isStricken: false,
            isCorrection: isCorrection,
            events: [...events],
            stateBefore: null,
            stateAfter: null,
        };
    }

    insertHeaders(history, game) {
        const result = [];
        let currentInning = -1;
        let currentTeam = '';

        history.forEach(item => {
            const parts = item.ctxKey.split('-');
            const inn = parseInt(parts[0], 10);
            const team = parts[1];

            if (inn !== currentInning || team !== currentTeam) {
                result.push({
                    id: `header-${inn}-${team}`,
                    ctxKey: `${inn}-${team}-header`,
                    type: 'INNING_HEADER',
                    inning: inn,
                    team: team === 'away' ? (game.away || 'Away') : (game.home || 'Home'),
                    side: team === 'away' ? 'Top' : 'Bottom',
                    isStricken: false,
                    isCorrection: false,
                    actions: [],
                    stateBefore: null,
                    stateAfter: null,
                });
                currentInning = inn;
                currentTeam = team;
            }
            result.push(item);
        });
        return result;
    }

    getEffectiveLog(log) {
        const effectivelyUndone = new Set();
        for (let i = log.length - 1; i >= 0; i--) {
            const action = log[i];
            if (effectivelyUndone.has(action.id)) {
                continue;
            }
            if (action.type === ActionTypes.UNDO && action.payload && action.payload.refId) {
                effectivelyUndone.add(action.payload.refId);
            }
        }
        return log.filter(a => !effectivelyUndone.has(a.id) && a.type !== ActionTypes.UNDO);
    }

    /**
     * Replays the history to attach authoritative state snapshots.
     * Reuses the main gameReducer for high-fidelity state tracking.
     */
    propagateStates(history, game) {
        let currentState = getInitialState();
        // Roster is essential for state transitions and name resolution
        if (game.roster) {
            currentState.roster = JSON.parse(JSON.stringify(game.roster));
            // Reset roster to starters for accurate replay
            ['away', 'home'].forEach(team => {
                if (currentState.roster[team]) {
                    currentState.roster[team].forEach(slot => {
                        if (slot.starter) {
                            slot.current = JSON.parse(JSON.stringify(slot.starter));
                            slot.history = []; // Clear history as we will rebuild it
                        }
                    });
                }
            });
        }

        history.forEach(item => {
            const parts = item.ctxKey.split('-');
            const inn = parseInt(parts[0], 10);
            const team = parts[1]; // 'away' or 'home'

            // stateBefore is always the state at the start of this history item
            item.stateBefore = this.mapInternalStateToNarrative(currentState, team, inn);

            // Resolve batter for this play item (even if stricken)
            const bIdx = parts.length > 2 ? parseInt(parts[2], 10) : -1;
            if (bIdx !== -1 && currentState.roster[team] && currentState.roster[team][bIdx]) {
                item.batter = JSON.parse(JSON.stringify(currentState.roster[team][bIdx].current));
            }

            // Headers and Summaries don't have events to reduce
            if (item.type === 'INNING_HEADER' || item.type === 'SUMMARY') {
                return;
            }

            let itemState = JSON.parse(JSON.stringify(currentState));

            // Apply events to itemState and augment them
            item.events.forEach(action => {
                // Infer columns from context if missing (robustness for tests/partial data)
                if (action.payload && action.payload.activeCtx) {
                    const { i, col } = action.payload.activeCtx;
                    if (!itemState.columns.find(c => c.id === col)) {
                        itemState.columns.push({ id: col, inning: i });
                    }
                }

                const prevState = JSON.parse(JSON.stringify(itemState));
                itemState = gameReducer(itemState, action);

                // Augment action with resolved names based on prevState
                this.augmentActionWithNames(action, prevState, team, inn);
            });

            // stateAfter is the state after all events in this item have been applied
            if (!item.isStricken) {
                currentState = itemState;
                item.stateAfter = this.mapInternalStateToNarrative(currentState, team, inn);
            }
        });
    }

    /**
     * Augments an action payload with resolved player names based on the state BEFORE the action.
     */
    augmentActionWithNames(action, state, team, inning) {
        const payload = action.payload || {};
        const inningColIds = state.columns.filter(c => c.inning === inning).map(c => c.id);

        if (action.type === ActionTypes.PLAY_RESULT) {
            if (payload.runnerAdvancements) {
                payload.runnerAdvancements.forEach(adv => {
                    let name = adv.name;
                    if (!name && adv.key && state.events[adv.key]?.pId) {
                        const bIdx = parseInt(adv.key.split('-')[1], 10);
                        name = state.roster[team][bIdx]?.current?.name;
                    }
                    if (!name && typeof adv.base === 'number') {
                        // Find who was on that base in 'state'
                        Object.keys(state.events).forEach(key => {
                            const [evtTeam, bIdx, ...colParts] = key.split('-');
                            const colId = colParts.join('-');
                            if (evtTeam === team && inningColIds.includes(colId)) {
                                const evt = state.events[key];
                                if (evt.paths[adv.base] === 1 && evt.paths[adv.base + 1] === 0) {
                                    name = state.roster[team][parseInt(bIdx, 10)]?.current?.name;
                                }
                            }
                        });
                    }
                    // Fallback: Parse key if available
                    if (!name && adv.key) {
                        const parts = adv.key.split('-');
                        if (parts.length >= 2) {
                            const bIdx = parseInt(parts[1], 10);
                            name = state.roster[team][bIdx]?.current?.name;
                        }
                    }

                    if (!name && (adv.base === -1 || adv.base == null)) {
                        // It's the batter
                        const bIdx = action.payload?.activeCtx?.b;
                        if (bIdx !== undefined) {
                            name = state.roster[team][bIdx]?.current?.name;
                        }
                    }
                    adv.resolvedName = name || 'Runner';
                });
            }
        } else if (action.type === ActionTypes.RUNNER_ADVANCE || action.type === ActionTypes.RUNNER_BATCH_UPDATE) {
            const updates = action.type === ActionTypes.RUNNER_BATCH_UPDATE ? payload.updates : payload.runners;
            updates.forEach(u => {
                let name = u.name;
                const originBase = typeof u.base === 'number' ? u.base : (u.baseIdx !== undefined ? u.baseIdx : -1);

                if (!name) {
                    if (u.key && state.events[u.key]?.pId) {
                        const bIdx = parseInt(u.key.split('-')[1], 10);
                        name = state.roster[team][bIdx]?.current?.name;
                    }
                }

                if (!name) {
                    if (originBase === -1) {
                        const bIdx = action.payload?.activeCtx?.b;
                        if (bIdx !== undefined) {
                            name = state.roster[team][bIdx]?.current?.name;
                        }
                    } else {
                        // Find who was on originBase
                        Object.keys(state.events).forEach(key => {
                            const [evtTeam, bIdx, ...colParts] = key.split('-');
                            const colId = colParts.join('-');
                            if (evtTeam === team && inningColIds.includes(colId)) {
                                const evt = state.events[key];
                                if (evt.paths[originBase] === 1 && evt.paths[originBase + 1] === 0) {
                                    name = state.roster[team][parseInt(bIdx, 10)]?.current?.name;
                                }
                            }
                        });
                    }
                }
                // Fallback: Parse key if available
                if (!name && u.key) {
                    const parts = u.key.split('-');
                    if (parts.length >= 2) {
                        const bIdx = parseInt(parts[1], 10);
                        name = state.roster[team][bIdx]?.current?.name;
                    }
                }
                u.resolvedName = name || 'Runner';
            });
        }
    }

    /**
     * Maps the complex internal reducer state to the simplified snapshot format.
     */
    mapInternalStateToNarrative(state, team, inning) {
        const runners = [null, null, null];
        const inningColIds = state.columns.filter(c => c.inning === inning).map(c => c.id);

        let maxOutNum = 0;
        const score = { away: 0, home: 0 };

        Object.keys(state.events).forEach(key => {
            const [evtTeam, bIdx, ...colParts] = key.split('-');
            const colId = colParts.join('-');
            const evt = state.events[key];

            // Score calculation
            if (evt.paths[3] === 1) {
                score[evtTeam]++;
            }

            // Runners on base for this team and inning
            if (evtTeam === team && inningColIds.includes(colId)) {
                if (evt.outNum) {
                    maxOutNum = Math.max(maxOutNum, evt.outNum);
                }
                for (let b = 0; b < 3; b++) {
                    if (evt.paths[b] === 1 && evt.paths[b + 1] === 0) {
                        const bIdxInt = parseInt(bIdx, 10);
                        const player = state.roster[team][bIdxInt]?.current;
                        if (player) {
                            runners[b] = JSON.parse(JSON.stringify(player));
                        }
                    }
                }
            }
        });

        return {
            outs: maxOutNum % 3,
            runners,
            score,
            hits: { away: 0, home: 0 }, // Handled by Narrative Summary logic if needed
        };
    }

    getUndoTargetId(log) {
        if (!log || log.length === 0) {
            return null;
        }
        const effectivelyUndone = new Set();
        for (let i = log.length - 1; i >= 0; i--) {
            const action = log[i];
            if (action.type === ActionTypes.UNDO && action.payload && action.payload.refId) {
                effectivelyUndone.add(action.payload.refId);
            }
        }
        for (let i = log.length - 1; i >= 0; i--) {
            const action = log[i];
            if (action.type === ActionTypes.UNDO || effectivelyUndone.has(action.id)) {
                continue;
            }
            return action.id;
        }
        return null;
    }

    getRedoTargetId(log) {
        if (!log || log.length === 0) {
            return null;
        }
        const effectivelyUndone = new Set();
        const actionMap = new Map();
        log.forEach(a => actionMap.set(a.id, a));
        for (let i = log.length - 1; i >= 0; i--) {
            const action = log[i];
            if (action.type === ActionTypes.UNDO && action.payload && action.payload.refId) {
                effectivelyUndone.add(action.payload.refId);
            }
        }
        for (let i = log.length - 1; i >= 0; i--) {
            const action = log[i];
            if (action.type !== ActionTypes.UNDO && !effectivelyUndone.has(action.id)) {
                return null;
            }
            if (action.type === ActionTypes.UNDO) {
                if (effectivelyUndone.has(action.id)) {
                    continue;
                }
                const targetId = action.payload?.refId;
                const target = actionMap.get(targetId);
                if (target && target.type !== ActionTypes.UNDO) {
                    return action.id;
                }
            }
        }
        return null;
    }

    async undo(log, isReadOnly) {
        if (isReadOnly || !log) {
            return;
        }
        const targetId = this.getUndoTargetId(log);
        if (targetId) {
            await this.dispatch({ type: ActionTypes.UNDO, payload: { refId: targetId } });
        }
    }

    async redo(log, isReadOnly) {
        if (isReadOnly || !log) {
            return;
        }
        const targetId = this.getRedoTargetId(log);
        if (targetId) {
            await this.dispatch({ type: ActionTypes.UNDO, payload: { refId: targetId } });
        }
    }
}
