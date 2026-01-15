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

describe('Game Reducer - PLAY_RESULT', () => {
    let initialState;

    beforeEach(() => {
        initialState = getInitialState();
        // Mock state setup
        initialState.events['away-0-col-1-0'] = {
            balls: 0, strikes: 0, outcome: '', paths: [0, 0, 0, 0], pitchSequence: [],
        };
        // Set up context
        initialState.columns = [{ inning: 1, id: 'col-1-0' }];
    });

    const createAction = (bipState, activeTeam = 'away') => ({
        type: ActionTypes.PLAY_RESULT,
        payload: {
            activeCtx: { b: 0, i: 1, col: 'col-1-0' },
            activeTeam,
            bipState,
            batterId: 'batter-id-1',
        },
    });

    test('Safe 1B (Single)', () => {
        const action = createAction({ res: 'Safe', base: '1B', type: 'Ground', seq: '7' });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('1B');
        expect(event.paths).toEqual([1, 0, 0, 0]);
    });

    test('Safe 2B (Double)', () => {
        const action = createAction({ res: 'Safe', base: '2B', type: 'Line', seq: '8' });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('2B');
        expect(event.paths).toEqual([1, 1, 0, 0]);
    });

    test('Safe HR', () => {
        const action = createAction({ res: 'Safe', base: 'Home', type: 'Fly', seq: '9' });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('HR');
        expect(event.paths).toEqual([1, 1, 1, 1]);
        expect(event.scoreInfo).toEqual({ rbiCreditedTo: 'batter-id-1' });
    });

    test('Fly Out (F8)', () => {
        const action = createAction({ res: 'Fly', base: '', type: 'Fly', seq: '8' });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('F8');
        expect(event.outNum).toBe(1);
    });

    test('Ground Out (6-3)', () => {
        const action = createAction({ res: 'Ground', base: '', type: 'Ground', seq: '6-3' });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('6-3');
        expect(event.outNum).toBe(1);
    });

    test('Error (E5)', () => {
        const action = createAction({ res: 'Safe', base: '1B', type: 'ERR', seq: '5' });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('E-5'); // Assuming logic matches 'E' + '-seq'
        expect(event.paths).toEqual([1, 0, 0, 0]);
    });

    test('Sacrifice Fly (SF8)', () => {
        const action = createAction({ res: 'Fly', base: '', type: 'SF', seq: '8' });
        const state = gameReducer(initialState, action);
        const event = state.events['away-0-col-1-0'];

        expect(event.outcome).toBe('SF8');
        expect(event.outNum).toBe(1);
    });
});
