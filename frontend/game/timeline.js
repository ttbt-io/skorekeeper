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

/**
 * Migrates a legacy action log to the new Dual-Timeline format by inferring
 * `refId` for historical edits (e.g. repeated PLAY_RESULT actions for the same grid context).
 * 
 * BACKWARD COMPATIBILITY NOTE:
 * This function handles data created before the Dual-Timeline update (May 2026).
 * It will be safe to remove this logic ONLY after:
 * 1. A full production database migration has been performed to explicitly add refIds to all logs.
 * 2. OR enough time has passed (e.g., 24 months) that legacy data is no longer supported.
 * 
 * TO REMOVE: 
 * - Delete this function.
 * - Remove calls in reducer.js (computeStateFromLog) and historyManager.js (propagateStates).
 *
 * @param {Array} log - The raw action log.
 * @returns {Array} A new log array with retrofitted `refId`s where appropriate.
 */
export function migrateLegacyActionLog(log) {
    if (!log || log.length === 0) {
        return [];
    }

    const lastGenerativeMap = new Map(); // legacyCtxKey -> action.id
    let requiresMigration = false;

    // First pass to check if any migration is needed to avoid unnecessary cloning
    for (const action of log) {
        if (!action.payload || action.type === ActionTypes.UNDO) {
            continue;
        }
        const team = action.payload.activeTeam || action.payload.team;
        if (!team || !action.payload.activeCtx) {
            continue;
        }

        const ctxKey = `${team}-${action.payload.activeCtx.b}-${action.payload.activeCtx.col}`;
        if (action.type === ActionTypes.PLAY_RESULT || action.type === ActionTypes.CLEAR_DATA) {
            const prevId = lastGenerativeMap.get(ctxKey);
            if (prevId && !action.refId && !action.insertAfterId) {
                requiresMigration = true;
                break;
            }
            lastGenerativeMap.set(ctxKey, action.id);
        }
    }

    if (!requiresMigration) {
        return log;
    }

    lastGenerativeMap.clear();

    return log.map(action => {
        if (!action.payload || action.type === ActionTypes.UNDO) {
            return action;
        }

        const team = action.payload.activeTeam || action.payload.team;
        if (!team || !action.payload.activeCtx) {
            return action;
        }

        const ctxKey = `${team}-${action.payload.activeCtx.b}-${action.payload.activeCtx.col}`;

        if (action.type === ActionTypes.PLAY_RESULT || action.type === ActionTypes.CLEAR_DATA) {
            const prevId = lastGenerativeMap.get(ctxKey);

            if (prevId && !action.refId && !action.insertAfterId) {
                const migratedAction = { ...action, refId: prevId };
                lastGenerativeMap.set(ctxKey, migratedAction.id);
                return migratedAction;
            }

            lastGenerativeMap.set(ctxKey, action.id);
        }

        return action;
    });
}

/**
 * Phase 1: Build the Game Timeline from the Action Log.
 * This function resolves UNDO tombstones, in-place edits (refId),
 * and insertions (insertAfterId) to produce a 1D chronological array
 * of valid game actions.
 *
 * @param {Array} log - The raw append-only action log.
 * @returns {Array} The ordered timeline of effective actions.
 */
export function buildTimeline(log) {
    if (!log || log.length === 0) {
        return [];
    }

    // 1. Identify Tombstones (UNDO logic and CLEAR_DATA wipes)
    const effectivelyUndone = new Set();
    const clearedContexts = new Set(); // tracks contexts cleared by a later CLEAR_DATA

    for (let i = log.length - 1; i >= 0; i--) {
        const action = log[i];
        if (effectivelyUndone.has(action.id)) {
            continue;
        }

        if (action.type === ActionTypes.UNDO && action.payload && action.payload.refId) {
            effectivelyUndone.add(action.payload.refId);
        } else if (action.type === ActionTypes.CLEAR_DATA && !action.insertAfterId) {
            // Wipe everything before this in the same context
            const p = action.payload;
            if (p && p.activeCtx) {
                const team = p.activeTeam || p.team;
                if (team) {
                    clearedContexts.add(`${team}-${p.activeCtx.b}-${p.activeCtx.col}`);
                }
            }
            // The CLEAR_DATA itself should also be omitted from the final timeline
            // as its only purpose is to act as a tombstone for prior actions.
            effectivelyUndone.add(action.id);
        } else if (action.payload && action.payload.activeCtx) {
            const p = action.payload;
            const team = p.activeTeam || p.team;
            if (team) {
                const ctxKey = `${team}-${p.activeCtx.b}-${p.activeCtx.col}`;
                if (clearedContexts.has(ctxKey)) {
                    effectivelyUndone.add(action.id);
                }
            }
        }
    }

    // 2. Resolve Edits and Base Ordering
    // We maintain a map of the "latest version" of an action if it was edited.
    // An edit is an action with a `refId` that is NOT an UNDO.
    const latestVersionMap = new Map(); // original action ID -> edited action
    const actionsToPlace = [];

    log.forEach(action => {
        if (effectivelyUndone.has(action.id) || action.type === ActionTypes.UNDO) {
            return; // Skip tombstones and UNDOs
        }

        if (action.refId) {
            // This is an edit of a previous action
            latestVersionMap.set(action.refId, action);
        } else {
            actionsToPlace.push(action);
        }
    });

    // Replace original actions with their latest edited versions
    const replacedActions = actionsToPlace.map(action => {
        let current = action;
        while (latestVersionMap.has(current.id)) {
            current = latestVersionMap.get(current.id);
        }
        return current;
    });

    // 3. Resolve Insertions
    // We build a linked list or use topological sort based on insertAfterId.
    // For simplicity, we can do a multi-pass array insertion.
    let timeline = [];
    const pendingInsertions = [];

    replacedActions.forEach(action => {
        if (action.insertAfterId) {
            pendingInsertions.push(action);
        } else {
            timeline.push(action);
        }
    });

    // Insert pending actions after their target IDs
    // We loop until no more insertions can be made (resolves chains of insertions)
    let madeProgress = true;
    while (pendingInsertions.length > 0 && madeProgress) {
        madeProgress = false;
        for (let i = 0; i < pendingInsertions.length; i++) {
            const action = pendingInsertions[i];
            const targetIndex = timeline.findIndex(a => a.id === action.insertAfterId);

            if (targetIndex !== -1) {
                // Insert right after the target
                timeline.splice(targetIndex + 1, 0, action);
                pendingInsertions.splice(i, 1);
                madeProgress = true;
                break; // restart loop after mutation
            }
        }
    }

    // If there are still pending insertions but no progress, they refer to missing IDs.
    if (pendingInsertions.length > 0) {
        console.warn('Timeline: missing insertion targets', pendingInsertions.map(a => a.insertAfterId));
        timeline.push(...pendingInsertions);
    }

    return timeline;
}
