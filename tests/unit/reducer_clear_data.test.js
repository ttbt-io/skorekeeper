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

describe('reducer: CLEAR_DATA', () => {
    let initialState;

    beforeEach(() => {
        initialState = getInitialState();
        // Setup a dummy event
        const key = 'away-0-col-1-0';
        initialState.events[key] = {
            outcome: '1B',
            balls: 2,
            strikes: 1,
            outNum: 0,
            paths: [1, 0, 0, 0],
            pathInfo: ['Hit', '', '', ''],
            pitchSequence: [{ type: 'ball' }, { type: 'ball' }, { type: 'strike' }],
            pId: 'player1',
        };
    });

    test('should clear event data but preserve pId', () => {
        const action = {
            type: ActionTypes.CLEAR_DATA,
            payload: {
                activeCtx: { b: 0, col: 'col-1-0', i: 1 },
                activeTeam: 'away',
                batterId: 'player1',
            },
        };

        const newState = gameReducer(initialState, action);
        const key = 'away-0-col-1-0';
        const event = newState.events[key];

        expect(event).toBeDefined();
        expect(event.outcome).toBe('');
        expect(event.balls).toBe(0);
        expect(event.strikes).toBe(0);
        expect(event.outNum).toBe(0);
        expect(event.paths).toEqual([0, 0, 0, 0]);
        expect(event.pitchSequence).toEqual([]);
        expect(event.pId).toBe('player1'); // pId should be preserved
    });

    test('should initialize event if it does not exist', () => {
        const action = {
            type: ActionTypes.CLEAR_DATA,
            payload: {
                activeCtx: { b: 1, col: 'col-1-0', i: 1 },
                activeTeam: 'away',
                batterId: 'player2',
            },
        };

        const newState = gameReducer(initialState, action);
        const key = 'away-1-col-1-0';
        const event = newState.events[key];

        expect(event).toBeDefined();
        expect(event.pId).toBe('player2');
        expect(event.outcome).toBe('');
    });
});
