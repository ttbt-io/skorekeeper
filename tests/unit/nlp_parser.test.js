import { parseEvent, AmbiguityError } from '../../frontend/utils/parser.js';
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
        expect(res[1].payload.runners[0].outcome).toBe('To 3rd');
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

    test('should parse walks (BB) and generate missing called balls', () => {
        // No balls in event yet
        const res = parseEvent('walk', defaultState);
        expect(res.length).toBe(4);
        expect(res[0].type).toBe(ActionTypes.PITCH);
        expect(res[0].payload.type).toBe('ball');
        expect(res[0].payload.code).toBe('C');

        // With 2 balls already in the event
        const stateWithBalls = {
            ...defaultState,
            events: {
                'away-2-col-1-0': { pId: 'p3', balls: 2, strikes: 1, paths: [0, 0, 0, 0], pitchSequence: [{ type: 'ball', code: 'C' }, { type: 'ball', code: 'C' }, { type: 'strike', code: 'C' }] },
            },
        };
        const res2 = parseEvent('walk', stateWithBalls);
        expect(res2.length).toBe(2);
    });

    test('should parse HBP', () => {
        const res = parseEvent('hit by pitch', defaultState);
        expect(res[0].type).toBe(ActionTypes.PLAY_RESULT);
        expect(res[0].payload.bipState.type).toBe('HBP');
        expect(res[0].payload.bipState.res).toBe('Safe');
    });

    test('should parse errors and fielders choice', () => {
        const resErr = parseEvent('safe on error to 5', defaultState);
        expect(resErr[0].type).toBe(ActionTypes.PLAY_RESULT);
        expect(resErr[0].payload.bipState.type).toBe('ERR');
        expect(resErr[0].payload.bipState.seq).toBe('5');

        const resFc = parseEvent('fielder\'s choice to shortstop', defaultState);
        expect(resFc[0].type).toBe(ActionTypes.PLAY_RESULT);
        expect(resFc[0].payload.bipState.type).toBe('FC');
        expect(resFc[0].payload.bipState.seq).toBe('6');
    });

    test('should parse stolen bases and wild pitches', () => {
        const state = {
            ...defaultState,
            events: {
                'away-0-col-1-0': { pId: 'p1', paths: [1, 0, 0, 0] }, // Runner on 1B
            },
        };
        const resSteal = parseEvent('runner steals second', state);
        expect(resSteal[0].type).toBe(ActionTypes.RUNNER_BATCH_UPDATE);
        expect(resSteal[0].payload.updates[0].action).toBe('SB');
        expect(resSteal[0].payload.updates[0].base).toBe(0); // 1B is 0-indexed in updates

        const resWp = parseEvent('wild pitch', state);
        expect(resWp[0].type).toBe(ActionTypes.RUNNER_BATCH_UPDATE);
        expect(resWp[0].payload.updates[0].action).toBe('Adv');
    });

    test('should resolve state assertions: single, bases loaded', () => {
        const state = {
            ...defaultState,
            events: {
                'away-0-col-1-0': { pId: 'p1', paths: [1, 0, 0, 0] }, // Runner on 1st
                'away-1-col-1-0': { pId: 'p2', paths: [1, 1, 0, 0] },  // Runner on 2nd
            },
        };
        // "single, bases loaded" should advance p2 (on 2nd) to 3B, p1 (on 1B) to 2B, and batter to 1B.
        const res = parseEvent('single, bases loaded', state);
        // Expecting: PLAY_RESULT (single) + RUNNER_ADVANCE (p2 to 3B) + RUNNER_ADVANCE (p1 to 2B)
        expect(res.some(a => a.type === ActionTypes.PLAY_RESULT)).toBe(true);
        const advances = res.filter(a => a.type === ActionTypes.RUNNER_ADVANCE);
        expect(advances.length).toBe(2);

        // p2 is lead runner, should advance to 3B
        const advP2 = advances.find(a => a.payload.runners[0].id === 'p2');
        expect(advP2.payload.runners[0].outcome).toBe('To 3rd');

        // p1 is trailing, should advance to 2B
        const advP1 = advances.find(a => a.payload.runners[0].id === 'p1');
        expect(advP1.payload.runners[0].outcome).toBe('To 2nd');
    });

    test('should throw AmbiguityError when runner on 2B and single hit (no assertion)', () => {
        const state = {
            ...defaultState,
            events: {
                'away-0-col-1-0': { pId: 'p1', paths: [1, 1, 0, 0] }, // Runner on 2nd
            },
        };

        expect(() => parseEvent('single', state)).toThrow(AmbiguityError);

        try {
            parseEvent('single', state);
        } catch (err) {
            expect(err).toBeInstanceOf(AmbiguityError);
            expect(err.options.length).toBe(2);
            expect(err.options[0].text).toBe('Runner on 2nd advances to 3rd');
            expect(err.options[1].text).toBe('Runner on 2nd scores');
        }
    });
});

