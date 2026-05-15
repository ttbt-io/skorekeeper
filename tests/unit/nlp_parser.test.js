import { parseEvent } from '../../frontend/utils/parser.js';
import { ActionTypes } from '../../frontend/reducer.js';

describe('NLP Parser: End-to-End Tests', () => {
    const defaultState = {
        activeTeam: 'away',
        activeCtx: { i: 1, b: 2, col: 'col-1-0' }, // Set active batter to index 2
        activeBatterId: 'p3',
        roster: {
            away: [
                { current: { id: 'p1', name: 'Player 1', number: '1' } },
                { current: { id: 'p2', name: 'Player 2', number: '2' } },
                { current: { id: 'p3', name: 'Player 3', number: '3' } },
            ],
            home: [],
        },
        pitchers: { away: '', home: '' },
        overrides: { away: {}, home: {} },
        events: {},
        columns: [{ inning: 1, id: 'col-1-0' }],
        actionLog: [],
    };

    test('should parse simple pitches', () => {
        const res = parseEvent('ball', defaultState);
        expect(res[0].type).toBe(ActionTypes.PITCH);
        expect(res[0].payload.type).toBe('ball');
    });

    test('should parse basic hits (BIP)', () => {
        const res = parseEvent('single', defaultState);
        expect(res[0].type).toBe(ActionTypes.PLAY_RESULT);
        expect(res[0].payload.bipState.base).toBe('1B');
    });

    test('should resolve jersey numbers in substitutions', () => {
        const state = {
            ...defaultState,
            roster: {
                away: [
                    { current: { id: 'p1', name: 'Jones', number: '5' } },
                    { current: { id: 'p2', name: 'Smith', number: '11' } },
                ],
                home: [],
            },
        };
        const res = parseEvent('pinch runner 5 for 11', state);
        expect(res[0].type).toBe(ActionTypes.SUBSTITUTION);
        expect(res[0].payload.subParams.id).toBe('p1');
        expect(res[0].payload.rosterIndex).toBe(1);
    });

    test('should handle sequential simulation: single, runner to third', () => {
        const res = parseEvent('single, runner to third', defaultState);

        expect(res.length).toBe(2);
        expect(res[0].type).toBe(ActionTypes.PLAY_RESULT);
        expect(res[1].type).toBe(ActionTypes.RUNNER_ADVANCE);

        expect(res[1].payload.runners[0].id).toBe('p3');
        expect(res[1].payload.runners[0].outcome).toBe('To 3B');
    });

    test('should throw error on invalid physical state', () => {
        expect(() => parseEvent('runner to second', defaultState)).toThrow('No runners on base');
    });

    test('should resolve lead runner logically when multiple are on base', () => {
        const state = {
            ...defaultState,
            events: {
                'away-0-col-1-0': { pId: 'p1', paths: [1, 0, 0, 0] }, // Runner on 1st
                'away-1-col-1-0': { pId: 'p2', paths: [1, 1, 0, 0] },  // Runner on 2nd
            },
        };
        // "runner to third" should resolve to p2 (on 2nd)
        const res3 = parseEvent('runner to third', state);
        expect(res3[0].payload.runners[0].id).toBe('p2');

        // "runner scores" should resolve to p2 (on 2nd) since they are lead
        const resH = parseEvent('runner scores', state);
        expect(resH[0].payload.runners[0].id).toBe('p2');
    });

    test('should resolve trailing runner if target is unoccupied', () => {
        const state = {
            ...defaultState,
            events: {
                'away-0-col-1-0': { pId: 'p1', paths: [1, 0, 0, 0] }, // Runner on 1st
                'away-1-col-1-0': { pId: 'p2', paths: [1, 1, 1, 0] },  // Runner on 3rd
            },
        };
        // "runner to second" should resolve to p1 (on 1st) because p2 is on 3rd
        const res2 = parseEvent('runner to second', state);
        expect(res2[0].payload.runners[0].id).toBe('p1');
    });
});
