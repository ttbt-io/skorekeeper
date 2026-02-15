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

import { HistoryManager } from '../../frontend/game/historyManager.js';
import { ActionTypes } from '../../frontend/reducer.js';

describe('HistoryManager Undo/Redo Exhaustive Edge Cases', () => {
    let historyManager;

    beforeEach(() => {
        historyManager = new HistoryManager({ dispatch: jest.fn() });
    });

    const createAction = (id, type = ActionTypes.PITCH) => ({
        type,
        id,
        payload: {},
        timestamp: Date.now(),
    });

    const createUndo = (id, refId) => ({
        type: ActionTypes.UNDO,
        id,
        payload: { refId },
        timestamp: Date.now(),
    });

    test('Empty log should return null for both', () => {
        expect(historyManager.getUndoTargetId([])).toBeNull();
        expect(historyManager.getRedoTargetId([])).toBeNull();
        expect(historyManager.getUndoTargetId(null)).toBeNull();
        expect(historyManager.getRedoTargetId(null)).toBeNull();
    });

    test('Single action should be undoable but not redoable', () => {
        const log = [createAction('A')];
        expect(historyManager.getUndoTargetId(log)).toBe('A');
        expect(historyManager.getRedoTargetId(log)).toBeNull();
    });

    test('Undo of single action should be redoable but not undoable', () => {
        const log = [
            createAction('A'),
            createUndo('UA', 'A'),
        ];
        expect(historyManager.getUndoTargetId(log)).toBeNull();
        expect(historyManager.getRedoTargetId(log)).toBe('UA');
    });

    test('Redo of an undo should restore undoability', () => {
        const log = [
            createAction('A'),
            createUndo('UA', 'A'),
            createUndo('RUA', 'UA'),
        ];
        expect(historyManager.getUndoTargetId(log)).toBe('A');
        expect(historyManager.getRedoTargetId(log)).toBeNull();
    });

    test('Undo-Redo-Undo-Redo sequence', () => {
        const log = [
            createAction('A'),
            createUndo('U1', 'A'),    // Undo A
            createUndo('R1', 'U1'),   // Redo A
            createUndo('U2', 'A'),    // Undo A again
            createUndo('R2', 'U2'),    // Redo A again
        ];
        expect(historyManager.getUndoTargetId(log)).toBe('A');
        expect(historyManager.getRedoTargetId(log)).toBeNull();
    });

    test('Triple nested undos (Undo of Redo)', () => {
        // [A, UA, RUA, RUUA(ref RUA)]
        // UA cancels A.
        // RUA cancels UA (so A is restored).
        // RUUA cancels RUA (so UA is active again, cancelling A).
        const log = [
            createAction('A'),
            createUndo('UA', 'A'),
            createUndo('RUA', 'UA'),
            createUndo('RUUA', 'RUA'),
        ];
        // State: A is undone.
        // Next Undo: nothing (A is already undone).
        // Next Redo: UA (to redo A).
        expect(historyManager.getUndoTargetId(log)).toBeNull();
        expect(historyManager.getRedoTargetId(log)).toBe('UA');
    });

    test('Multiple actions, all undone, redone one by one', () => {
        const log = [
            createAction('A'),
            createAction('B'),
            createUndo('UB', 'B'),
            createUndo('UA', 'A'),
        ];
        // Both undone.
        expect(historyManager.getUndoTargetId(log)).toBeNull();
        expect(historyManager.getRedoTargetId(log)).toBe('UA');

        // Redo A
        log.push(createUndo('RA', 'UA'));
        expect(historyManager.getUndoTargetId(log)).toBe('A');
        expect(historyManager.getRedoTargetId(log)).toBe('UB');

        // Redo B
        log.push(createUndo('RB', 'UB'));
        expect(historyManager.getUndoTargetId(log)).toBe('B');
        expect(historyManager.getRedoTargetId(log)).toBeNull();
    });

    test('Linear History Barrier: New action blocks past redos', () => {
        const log = [
            createAction('A'),
            createAction('B'),
            createUndo('UB', 'B'),
            createAction('C'),
        ];
        // State: A, C. B is undone.
        // C happened AFTER B was undone.
        // Redo B should be disabled.
        expect(historyManager.getRedoTargetId(log)).toBeNull();
        // Undo should target C.
        expect(historyManager.getUndoTargetId(log)).toBe('C');
    });

    test('Linear History Barrier with mixed action types', () => {
        const log = [
            createAction('A', ActionTypes.PITCH),
            createUndo('UA', 'A'),
            createAction('L1', ActionTypes.LINEUP_UPDATE),
        ];
        // Even a LINEUP_UPDATE acts as a barrier for previous redos.
        expect(historyManager.getRedoTargetId(log)).toBeNull();
        expect(historyManager.getUndoTargetId(log)).toBe('L1');
    });

    test('Barrier after partial redo', () => {
        const log = [
            createAction('A'),
            createAction('B'),
            createUndo('UB', 'B'),
            createUndo('UA', 'A'),
            createUndo('RA', 'UA'), // Redo A
            createAction('C'),       // New action
        ];
        // State: A, C. B is still undone.
        // UA is "dead" (undone by RA).
        // UB is "alive" but behind barrier C.
        expect(historyManager.getRedoTargetId(log)).toBeNull();
        expect(historyManager.getUndoTargetId(log)).toBe('C');
    });

    test('Undoing an action that was redone multiple times', () => {
        const log = [
            createAction('A'),
            createUndo('U1', 'A'),
            createUndo('R1', 'U1'),
            createUndo('U2', 'A'),
            createUndo('R2', 'U2'),
        ];
        // A is active.
        expect(historyManager.getUndoTargetId(log)).toBe('A');

        log.push(createUndo('U3', 'A'));
        // A is undone. Next redo should be U3.
        expect(historyManager.getUndoTargetId(log)).toBeNull();
        expect(historyManager.getRedoTargetId(log)).toBe('U3');
    });

    test('Redoing an action that was never undone (robustness)', () => {
        const log = [
            createAction('A'),
            createUndo('R1', 'A'), // "Redo" A even though it was never undone
        ];
        // This is weird usage but getEffectivelyUndoneSet treats it as "Undo A".
        // getUndoTargetId: A is effectively undone. returns null.
        // getRedoTargetId: R1 is UNDO. targets A. A is NOT UNDO. returns R1.
        expect(historyManager.getUndoTargetId(log)).toBeNull();
        expect(historyManager.getRedoTargetId(log)).toBe('R1');
    });

    test('Actions interleaved with undos of other actions', () => {
        const log = [
            createAction('A'),
            createAction('B'),
            createUndo('UA', 'A'), // Undo A. B is still live and after UA in log?
            // Wait, in standard Skorekeeper usage, Undo targets the LAST live action.
            // If user explicitly undid A (e.g. from a history feed), what happens?
        ];
        // current logic:
        // getUndoTargetId: scans backwards.
        // i=2: UA is UNDO. skip.
        // i=1: B is NOT UNDO, not effectivelyUndone. returns B.
        expect(historyManager.getUndoTargetId(log)).toBe('B');

        // getRedoTargetId:
        // i=2: UA is UNDO. targets A. returns UA.
        expect(historyManager.getRedoTargetId(log)).toBe('UA');
        // Note: Skorekeeper design usually prefers reverse-chronological undo,
        // but the core logic supports arbitrary undos from the feed.
        // Even if A is undone, B is still "live" and is the LATEST live action.
    });
});
