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

describe('Game Reducer - PLAY_RESULT - Dropped 3rd Strike', () => {
    let initialState;

    beforeEach(() => {
        initialState = getInitialState();
        initialState.events['away-0-col-1-0'] = {
            balls: 0, strikes: 3, outcome: '', paths: [0, 0, 0, 0], pitchSequence: [{ type: 'strike', code: 'Dropped' }],
        };
        initialState.columns = [{ inning: 1, id: 'col-1-0' }];
    });

    const createDroppedAction = (bipState, runnerAdvancements = []) => ({
        type: ActionTypes.PLAY_RESULT,
        payload: {
            activeCtx: { b: 0, i: 1, col: 'col-1-0' },
            activeTeam: 'away',
            bipState,
            batterId: 'batter-id-1',
            bipMode: 'dropped',
            runnerAdvancements,
        },
    });

    test('Dropped 3rd Strike - Safe (D3) no sequence', () => {
        const action = createDroppedAction({ res: 'Safe', base: '1B', type: 'D3', seq: [] });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('D3');
        expect(event.paths).toEqual([1, 0, 0, 0]);
    });

    test('Dropped 3rd Strike - Safe (D3) with sequence', () => {
        const action = createDroppedAction({ res: 'Safe', base: '1B', type: 'D3', seq: ['2', '3'] });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('D3 2-3');
    });

    test('Dropped 3rd Strike - Safe FC with sequence', () => {
        const action = createDroppedAction({ res: 'Safe', base: '1B', type: 'FC', seq: ['2', '4'] });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('FC 2-4');
    });

    test('Dropped 3rd Strike - Out (K) no sequence', () => {
        const action = createDroppedAction({ res: 'Out', base: '', type: 'K', seq: [] });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('K');
        expect(event.outNum).toBe(1);
    });

    test('Dropped 3rd Strike - Out (K) with sequence', () => {
        const action = createDroppedAction({ res: 'Out', base: '', type: 'K', seq: ['2', '3'] });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('K 2-3');
    });

    test('Automatic DP Detection (Normal Mode)', () => {
        const action = {
            type: ActionTypes.PLAY_RESULT,
            payload: {
                activeCtx: { b: 0, i: 1, col: 'col-1-0' },
                activeTeam: 'away',
                bipState: { res: 'Ground', base: '', type: 'Ground', seq: ['6', '4', '3'] },
                batterId: 'batter-id-1',
                bipMode: 'normal',
                runnerAdvancements: [{ outcome: 'Out' }], // 1 runner out + batter out = 2
            },
        };
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('DP 6-4-3');
        expect(event.outNum).toBe(2);
    });

    test('Automatic TP Detection (Normal Mode)', () => {
        const action = {
            type: ActionTypes.PLAY_RESULT,
            payload: {
                activeCtx: { b: 0, i: 1, col: 'col-1-0' },
                activeTeam: 'away',
                bipState: { res: 'Line', base: '', type: 'Line', seq: ['4'] }, // Unassisted line drive
                batterId: 'batter-id-1',
                bipMode: 'normal',
                runnerAdvancements: [{ outcome: 'Out' }, { outcome: 'Out' }], // 2 runners out + batter out = 3
            },
        };
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('TP L4');
        expect(event.outNum).toBe(1); // Batter is the 1st out in a Line Drive TP
    });
});
