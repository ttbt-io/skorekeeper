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

    // 1. Identify Tombstones (UNDO logic)
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
    // We append them to the end as a fallback.
    if (pendingInsertions.length > 0) {
        timeline.push(...pendingInsertions);
    }

    return timeline;
}
