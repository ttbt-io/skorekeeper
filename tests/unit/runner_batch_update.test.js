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

import { gameReducer, getInitialState, ActionTypes } from '../../frontend/reducer.js';

describe('Runner Batch Update', () => {
    let initialState;

    beforeEach(() => {
        initialState = getInitialState();
        // Setup a game with some initial events/runners
        initialState.id = 'game-1';
        initialState.away = 'Away';
        initialState.home = 'Home';
        initialState.columns = [{ inning: 1, id: 'col-1-0' }];
        initialState.roster.away = Array(9).fill(0).map((_, i) => ({
            slot: i,
            starter: { name: `P${i}`, number: `${i}`, pos: '', id: `p${i}` },
            current: { name: `P${i}`, number: `${i}`, pos: '', id: `p${i}` },
            history: [],
        }));

        // P0 on 1B
        initialState.events['away-0-col-1-0'] = {
            outcome: '1B',
            paths: [1, 0, 0, 0],
            pathInfo: ['', '', '', ''],
            outNum: 0,
        };

        // P1 on 2B
        initialState.events['away-1-col-1-0'] = {
            outcome: '2B',
            paths: [1, 1, 0, 0],
            pathInfo: ['', '', '', ''],
            outNum: 0,
        };
    });

    test('should apply multiple runner updates', () => {
        const updates = [
            {
                key: 'away-0-col-1-0', // P0 on 1B
                base: 0,
                action: 'SB', // Steal 2nd
            },
            {
                key: 'away-1-col-1-0', // P1 on 2B
                base: 1,
                action: 'CS', // Caught Stealing 3rd
            },
        ];

        const payload = {
            updates,
            activeCtx: { b: 2, i: 1, col: 'col-1-0' }, // Current batter is P2
            activeTeam: 'away',
        };

        const action = {
            type: ActionTypes.RUNNER_BATCH_UPDATE,
            payload,
        };

        const newState = gameReducer(initialState, action);

        // Check P0 (Steal 2nd)
        const e0 = newState.events['away-0-col-1-0'];
        expect(e0.paths[1]).toBe(1); // Safe at 2nd
        expect(e0.pathInfo[1]).toBe('SB');

        // Check P1 (Caught Stealing 3rd)
        const e1 = newState.events['away-1-col-1-0'];
        expect(e1.paths[2]).toBe(2); // Out at 3rd
        expect(e1.pathInfo[2]).toBe('CS');
        expect(e1.outNum).toBe(1); // Should be 1st out (assuming starts at 0)
    });

    test('should handle Picked Off (PO) correctly', () => {
        const updates = [
            {
                key: 'away-0-col-1-0', // P0 on 1B
                base: 0,
                action: 'PO', // Picked Off 1B (or 2B attempt?)
                // Usually PO means picked off at current base or while stealing.
                // The UI logic maps action to next base result.
                // So PO at 1B means out trying to go to 2B or picked off AT 1B?
                // In this system, paths[base+1] = 2 implies out advancing.
                // If picked off AT base, it's usually recorded as out on the path to next base in this grid model?
                // Or maybe the 'Out' logic handles positioning.
            },
        ];

        // Let's assume PO logic treats it as out on the path to next base (common for simple grid,
        // though strictly PO happens at the base).
        // The App logic sets pos=0.2 for PO.

        const payload = {
            updates,
            activeCtx: { b: 2, i: 1, col: 'col-1-0' },
            activeTeam: 'away',
        };

        const action = {
            type: ActionTypes.RUNNER_BATCH_UPDATE,
            payload,
        };

        const newState = gameReducer(initialState, action);
        const e0 = newState.events['away-0-col-1-0'];

        expect(e0.paths[1]).toBe(2); // Out
        expect(e0.pathInfo[1]).toBe('PO');
        expect(e0.outPos[1]).toBeCloseTo(0.2); // Check position logic
    });
});
