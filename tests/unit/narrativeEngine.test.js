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

import { NarrativeEngine } from '../../frontend/game/narrativeEngine.js';

describe('NarrativeEngine', () => {
    let engine;

    beforeEach(() => {
        engine = new NarrativeEngine();
    });

    test('getLocationName maps zones correctly', () => {
        expect(engine.getLocationName(8)).toBe('Center Field');
        expect(engine.getLocationName('643')).toBe('6-4-3 (Shortstop)');
        expect(engine.getLocationName(null)).toBe('');
    });

    test('getPlayDescription handles basic hits', () => {
        const batter = { name: 'Doe, J.' };
        const desc1B = engine.getPlayDescription({ res: 'Safe', base: '1B', type: 'HIT' }, batter, { trajectory: 'Line' }, 'seed1');
        expect(desc1B).toContain('Doe, J.');
        expect(desc1B).toMatch(/(single|base hit|hit)/);
    });

    test('getPlayDescription handles Home Run', () => {
        const batter = { name: 'Doe, J.' };
        const descHR = engine.getPlayDescription({ res: 'Safe', base: 'Home', type: 'HIT' }, batter, null, 'seed2');
        expect(descHR).toContain('Doe, J.');
        expect(descHR).toMatch(/(home run|goes yard|clears the fence|gone|moonshot)/);
    });

    test('getPlayDescription handles outs', () => {
        const batter = { name: 'Doe, J.' };
        const descSF = engine.getPlayDescription({ res: 'Out', type: 'SF' }, batter, null, 'seed3');
        expect(descSF).toContain('Doe, J.');
        expect(descSF).toMatch(/(flies out|fly ball|pops out|cans of corn|out)/);

        const descSH = engine.getPlayDescription({ res: 'Out', type: 'SH' }, batter, null, 'seed4');
        expect(descSH).toContain('Doe, J.');
        expect(descSH).toMatch(/(grounds out|bounces out|taps out|out)/);
    });

    test('getPlayDescription describes bip with location', () => {
        const batter = { name: 'Smith' };
        const bip = { res: 'Safe', base: '1B', type: 'HIT', seq: [7] };
        const desc = engine.getPlayDescription(bip, batter, null, 'seed5');
        expect(desc).toContain('Smith');
        expect(desc).toContain('Left Field');
    });

    test('generateNarrative groups by inning and team from linear history', () => {
        const game = {
            away: 'AwayTeam', home: 'HomeTeam', rules: { innings: 7 },
            columns: [{ inning: 1, id: 'col-1-0' }],
            roster: { away: [{ current: { name: 'Player 1', number: '1', id: 'p1' }, history: [] }], home: [] },
            events: { 'away-0-col-1-0': { outcome: '1B' } },
        };

        const mockLinearHistory = [
            { id: 'header-1', type: 'INNING_HEADER', inning: 1, team: 'away', side: 'Top', ctxKey: 'Header-1', stateBefore: { score: { away: 0, home: 0 }, hits: { away: 0, home: 0 } } },
            {
                id: 'play-1', ctxKey: '1-away-0-col-1-0', type: 'PLAY', isStricken: false, isCorrection: false,
                batter: { id: 'p1', name: 'Player 1', number: '1' },
                stateBefore: { outs: 0, runners: [null, null, null], score: { away: 0, home: 0 }, hits: { away: 0, home: 0 } },
                stateAfter: { outs: 0, runners: [{ id: 'p1', name: 'Player 1' }, null, null], score: { away: 0, home: 0 }, hits: { away: 1, home: 0 } },
                events: [{ type: 'PLAY_RESULT', payload: { bipState: { res: 'Safe', base: '1B', type: 'HIT', seq: [] }, hitData: { trajectory: 'Line' } } }],
            },
        ];

        const mockHistoryManager = { generateLinearHistory: () => mockLinearHistory };
        const feed = engine.generateNarrative(game, mockHistoryManager);

        expect(feed).toHaveLength(1);
        expect(feed[0].inning).toBe(1);
        expect(feed[0].items).toHaveLength(2);

        const pa = feed[0].items[0];
        expect(pa.batterText).toContain('Player 1');
        expect(pa.events).toHaveLength(1);
        expect(pa.events[0].description).toMatch(/(single|base hit|hit)/);
    });

    test('Inning Summary generation', () => {
        const game = { rules: { innings: 7 }, roster: { away: [], home: [] }, events: {} };
        const mockLinearHistory = [
            { id: 'h1', type: 'INNING_HEADER', inning: 1, side: 'Top', ctxKey: 'h1', stateBefore: { score: { away: 0, home: 0 }, hits: { away: 0, home: 0 } } },
            {
                id: 'p1', ctxKey: '1-away-0', type: 'PLAY', batter: { name: 'B1' },
                stateBefore: { outs: 2, runners: [null, null, null], score: { away: 0, home: 0 }, hits: { away: 0, home: 0 } },
                stateAfter: { outs: 3, runners: [null, null, null], score: { away: 1, home: 0 }, hits: { away: 1, home: 0 } },
                events: [{ type: 'PLAY_RESULT', payload: { bipState: { res: 'Safe', base: '1B' }, runnerAdvancements: [{ outcome: 'Score', name: 'R1' }] } }],
            },
            { id: 'h2', type: 'INNING_HEADER', inning: 1, side: 'Bottom', ctxKey: 'h2', stateBefore: { score: { away: 1, home: 0 }, hits: { away: 1, home: 0 } } },
        ];

        const mockHM = { generateLinearHistory: () => mockLinearHistory };
        const feed = engine.generateNarrative(game, mockHM);

        expect(feed).toHaveLength(2);
        const top1 = feed[0];
        const lastItem = top1.items[top1.items.length - 1];
        expect(lastItem.type).toBe('INNING_SUMMARY');
        expect(lastItem.events[0].description).toContain('1 Run');
        expect(lastItem.events[0].description).toContain('1 Hit');
        expect(lastItem.events[0].description).toContain('0 LOB');
    });

    test('getDeterministicTemplate is stable', () => {
        const key = '1B';
        const subKey = 'line';
        const seed = 'game1-event1';
        const result1 = engine.getDeterministicTemplate(key, subKey, seed);
        const result2 = engine.getDeterministicTemplate(key, subKey, seed);
        expect(result1).toBe(result2);
        expect(typeof result1).toBe('string');
    });
});