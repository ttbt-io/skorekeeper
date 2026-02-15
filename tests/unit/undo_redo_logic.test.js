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


import { AppController } from '../../frontend/controllers/AppController.js';
import { ActionTypes } from '../../frontend/reducer.js';

// Mock dependencies
jest.mock('../../frontend/services/dbManager.js');
jest.mock('../../frontend/services/authManager.js');
jest.mock('../../frontend/services/syncManager.js');

describe('Undo/Redo Logic', () => {
    let app;

    beforeEach(() => {
        // Prevent DOM access during constructor
        jest.spyOn(AppController.prototype, 'bindEvents').mockImplementation(() => {
        });
        jest.spyOn(AppController.prototype, 'init').mockResolvedValue();

        app = new AppController();
        // Setup a dummy activeGame
        app.state.activeGame = {
            actionLog: [],
        };
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    const createAction = (id, type = 'TEST') => ({
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

    test('Basic Undo/Redo Cycle', () => {
        // Log: [A]
        const A = createAction('A');
        app.state.activeGame.actionLog = [A];

        // Should be able to Undo A
        expect(app.historyManager.getUndoTargetId(app.state.activeGame.actionLog)).toBe('A');
        expect(app.historyManager.getRedoTargetId(app.state.activeGame.actionLog)).toBeNull();

        // Log: [A, Undo(A)]
        const UA = createUndo('UA', 'A');
        app.state.activeGame.actionLog = [A, UA];

        // Should NOT be able to Undo (A is undone).
        // Should be able to Redo (Undo the Undo).
        expect(app.historyManager.getUndoTargetId(app.state.activeGame.actionLog)).toBeNull();
        expect(app.historyManager.getRedoTargetId(app.state.activeGame.actionLog)).toBe('UA');

        // Log: [A, UA, RUA]
        const RUA = createUndo('RUA', 'UA');
        app.state.activeGame.actionLog = [A, UA, RUA];

        // RUA is an Undo of UA. UA is an Undo of A.
        // Effectively state is A.
        // UA is now effectively undone by RUA.
        // A is NOT targeted by any non-undone Undo.
        // So Undo should target A again?
        // Wait, the current logic in AppController (and restored in historyManager) is:
        // Identify all UNDO actions and their targets. (UA targets A, RUA targets UA).
        // Scan backwards for first action that is NOT an UNDO and NOT targeted.
        // 1. RUA is UNDO. Skip.
        // 2. UA is UNDO. Skip.
        // 3. A is targeted by UA. But UA is targeted by RUA (effectively undone).
        // So A is NOT targeted by an effective undo.
        // Result: 'A'.
        expect(app.historyManager.getUndoTargetId(app.state.activeGame.actionLog)).toBe('A');
        expect(app.historyManager.getRedoTargetId(app.state.activeGame.actionLog)).toBeNull();
    });

    test('Linear Barrier (New Action clears Redo)', () => {
        // Log: [A, Undo(A)]
        const A = createAction('A');
        const UA = createUndo('UA', 'A');
        app.state.activeGame.actionLog = [A, UA];

        // Pre-check: Redo is available
        expect(app.historyManager.getRedoTargetId(app.state.activeGame.actionLog)).toBe('UA');

        // User performs Action B.
        // Log: [A, Undo(A), B]
        const B = createAction('B');
        app.state.activeGame.actionLog = [A, UA, B];

        // Check: Redo should be GONE.
        // We cannot Redo A because B has established a new timeline.
        expect(app.historyManager.getRedoTargetId(app.state.activeGame.actionLog)).toBeNull();

        // Check: Undo should target B.
        expect(app.historyManager.getUndoTargetId(app.state.activeGame.actionLog)).toBe('B');
    });

    test('Multiple Undo Levels', () => {
        // Log: [A, B]
        const A = createAction('A');
        const B = createAction('B');
        app.state.activeGame.actionLog = [A, B];

        // 1. Undo B
        expect(app.historyManager.getUndoTargetId(app.state.activeGame.actionLog)).toBe('B');
        const UB = createUndo('UB', 'B');
        app.state.activeGame.actionLog.push(UB);

        // 2. Undo A
        // Current state effectively: A. (B is undone).
        // We can Undo A.
        expect(app.historyManager.getUndoTargetId(app.state.activeGame.actionLog)).toBe('A');
        const UA = createUndo('UA', 'A');
        app.state.activeGame.actionLog.push(UA);

        // 3. Redo A
        expect(app.historyManager.getRedoTargetId(app.state.activeGame.actionLog)).toBe('UA');
        const RUA = createUndo('RUA', 'UA');
        app.state.activeGame.actionLog.push(RUA);

        // Current state effectively: A. (B is still undone).
        // Can Undo: RUA (to go back to empty).
        // Can Redo: UB (to restore B)?
        // Wait, Redo Target Logic:
        // We have [A, B, UB, UA, RUA].
        // RUA cancels UA. A is active.
        // UB cancels B. B is inactive.
        // Is UB a valid Redo target?
        // historyManager.getRedoTargetId scans backwards.
        // It sees RUA (Generative-like? No, Redo).
        // It skips RUA.
        // It sees UA (Effective? No, targeted by RUA).
        // It sees UB (Effective? Yes).
        // Is UB blocked?
        // RUA is an Undo(Undo). It behaves like Generative in that it restores state.
        // Does RUA block UB?
        // If I restored A, I should be able to restore B next?
        // Yes, standard stack: [A, B] -> Undo B -> [A] -> Undo A -> [] -> Redo A -> [A] -> Redo B -> [A, B].
        // So RUA should NOT block UB.

        // Let's trace historyManager.getRedoTargetId for [A, B, UB, UA, RUA]:
        // 1. RUA (Undo of Undo). Type=UNDO. RefId=UA. Target(UA) is UNDO.
        //    Logic: "It is a Redo... Continue." (Does not block).
        // 2. UA (Targeted by RUA). In 'effectivelyUndone'. Skip.
        // 3. UB (Primary Undo). Target(B) is Generative.
        //    Logic: "Primary Undo... Return ID".
        // Expect: UB.
        expect(app.historyManager.getRedoTargetId(app.state.activeGame.actionLog)).toBe('UB');
    });

    test('Complex Barrier', () => {
        // [A, B, Undo(B), C] -> Redo(B) should be impossible.
        const A = createAction('A');
        const B = createAction('B');
        const UB = createUndo('UB', 'B');
        const C = createAction('C');

        app.state.activeGame.actionLog = [A, B, UB, C];

        expect(app.historyManager.getRedoTargetId(app.state.activeGame.actionLog)).toBeNull();
        expect(app.historyManager.getUndoTargetId(app.state.activeGame.actionLog)).toBe('C');
    });
});
