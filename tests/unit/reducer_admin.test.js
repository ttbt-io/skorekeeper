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

describe('Reducer: Admin Actions (Substitution & Lineup)', () => {
    let initialState;

    beforeEach(() => {
        initialState = getInitialState();
        // Setup basic roster
        initialState.roster.away = Array(9).fill(0).map((_, i) => ({
            slot: i,
            starter: { name: `S${i}`, number: `${i}`, pos: 'P', id: `s-${i}` },
            current: { name: `S${i}`, number: `${i}`, pos: 'P', id: `s-${i}` },
            history: [],
        }));
        initialState.subs.away = [{ name: 'Sub1', number: '99', id: 'sub-1' }];
    });

    test('SUBSTITUTION action updates current player and history', () => {
        const action = {
            type: ActionTypes.SUBSTITUTION,
            payload: {
                team: 'away',
                rosterIndex: 0,
                subParams: {
                    id: 'new-id-123',
                    name: 'NewPlayer',
                    number: '50',
                    pos: 'C',
                },
            },
        };

        const newState = gameReducer(initialState, action);
        const slot = newState.roster.away[0];

        // Check current updated
        expect(slot.current.name).toBe('NewPlayer');
        expect(slot.current.number).toBe('50');
        expect(slot.current.pos).toBe('C');
        expect(slot.current.id).toBe('new-id-123');

        // Check history
        expect(slot.history.length).toBe(1);
        expect(slot.history[0].name).toBe('S0');
        expect(slot.history[0].id).toBe('s-0');
    });

    test('LINEUP_UPDATE action replaces roster and subs', () => {
        const newRoster = Array(9).fill(0).map((_, i) => ({
            slot: i,
            starter: { name: `NewS${i}`, number: `${i + 10}`, pos: 'LF', id: `news-${i}` },
            current: { name: `NewS${i}`, number: `${i + 10}`, pos: 'LF', id: `news-${i}` },
            history: [],
        }));
        const newSubs = [{ name: 'NewSub', number: '88', id: 'newsub-1' }];

        const action = {
            type: ActionTypes.LINEUP_UPDATE,
            payload: {
                team: 'away',
                teamName: 'New Team Name',
                roster: newRoster,
                subs: newSubs,
            },
        };

        const newState = gameReducer(initialState, action);

        expect(newState.away).toBe('New Team Name');
        expect(newState.roster.away[0].starter.name).toBe('NewS0');
        expect(newState.subs.away[0].name).toBe('NewSub');
    });

    test('SCORE_OVERRIDE action updates overrides', () => {
        const action = {
            type: ActionTypes.SCORE_OVERRIDE,
            payload: {
                team: 'away',
                inning: 1,
                score: '5',
            },
        };

        const newState = gameReducer(initialState, action);
        expect(newState.overrides.away[1]).toBe('5');
    });

    test('PITCHER_UPDATE action updates current pitcher', () => {
        const action = {
            type: ActionTypes.PITCHER_UPDATE,
            payload: {
                team: 'home', // defense team
                pitcher: '42',
            },
        };

        const newState = gameReducer(initialState, action);
        expect(newState.pitchers.home).toBe('42');
    });
});