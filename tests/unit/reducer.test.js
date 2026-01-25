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

describe('Game Reducer', () => {
    let initialState;

    beforeEach(() => {
        initialState = getInitialState();
    });

    test('should return initial state', () => {
        // When state is undefined, it should probably return default state if we implemented default params,
        // but our reducer expects a state object.
        // Let's test with the result of getInitialState
        const stateWithInit = gameReducer(getInitialState(), {});
        expect(stateWithInit).toEqual(expect.objectContaining({
            actionLog: [],
            roster: { away: [], home: [] },
        }));
    });

    test('GAME_START should initialize game metadata and roster', () => {
        const initialRosterIds = { away: [], home: [] };
        for (let i = 0; i < 9; i++) {
            initialRosterIds.away.push(`away-id-${i}`);
            initialRosterIds.home.push(`home-id-${i}`);
        }

        const action = {
            type: ActionTypes.GAME_START,
            payload: {
                id: 'game-123',
                date: '2023-10-27',
                location: 'Stadium',
                event: 'Championship',
                away: 'Team A',
                home: 'Team B',
                initialRosterIds,
            },
        };

        const newState = gameReducer(initialState, action);

        expect(newState.id).toBe('game-123');
        expect(newState.away).toBe('Team A');
        expect(newState.columns.length).toBe(7);
        expect(newState.roster.away[0].starter.id).toBe('away-id-0');
        expect(newState.roster.home[8].starter.id).toBe('home-id-8');
    });

    test('PITCH (Ball) should increment balls and set BB on 4th ball', () => {
        const activeCtx = { b: 0, i: 1, col: 'col-1-0' };
        const action = {
            type: ActionTypes.PITCH,
            payload: {
                activeCtx,
                type: 'ball',
                code: '',
                activeTeam: 'away',
            },
        };

        // 1st Ball
        let state = gameReducer(initialState, action);
        const eventKey = 'away-0-col-1-0';
        expect(state.events[eventKey].balls).toBe(1);
        expect(state.events[eventKey].outcome).toBe('');

        // 2nd, 3rd, 4th
        state = gameReducer(state, action);
        state = gameReducer(state, action);
        state = gameReducer(state, action);

        expect(state.events[eventKey].balls).toBe(4);
        expect(state.events[eventKey].outcome).toBe('BB');
        expect(state.events[eventKey].paths[0]).toBe(1); // Safe at 1st
    });

    test('PITCH (Strike) should set K on 3rd strike', () => {
        const activeCtx = { b: 1, i: 1, col: 'col-1-0' };
        const action = {
            type: ActionTypes.PITCH,
            payload: {
                activeCtx,
                type: 'strike',
                code: 'Swinging',
                activeTeam: 'away',
            },
        };

        let state = gameReducer(initialState, action);
        const eventKey = 'away-1-col-1-0';

        state = gameReducer(state, action); // Strike 2
        state = gameReducer(state, action); // Strike 3

        expect(state.events[eventKey].strikes).toBe(3);
        expect(state.events[eventKey].outcome).toBe('K');
        expect(state.events[eventKey].outNum).toBe(1); // 1st out
    });

    test('PLAY_RESULT should correctly store hitData', () => {
        const activeCtx = { b: 0, i: 1, col: 'col-1-0' };
        const batterId = 'player-123';
        const hitData = { location: { x: 0.5, y: 0.5 }, trajectory: 'Line' };

        const action = {
            type: ActionTypes.PLAY_RESULT,
            payload: {
                activeCtx,
                activeTeam: 'away',
                bipState: { res: 'Safe', base: '1B', type: 'HIT', seq: '8' },
                batterId,
                bipMode: 'normal',
                hitData,
            },
        };

        const newState = gameReducer(initialState, action);
        const eventKey = 'away-0-col-1-0';

        expect(newState.events[eventKey].hitData).toEqual(hitData);
    });
});
