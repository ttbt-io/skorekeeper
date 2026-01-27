
import { NarrativeEngine } from '../../frontend/game/narrativeEngine.js';
import { HistoryManager } from '../../frontend/game/historyManager.js';
import { ActionTypes } from '../../frontend/reducer.js';

describe('NarrativeEngine & HistoryManager Integration (Reproduction)', () => {
    let engine;
    let historyManager;

    beforeEach(() => {
        engine = new NarrativeEngine();
        historyManager = new HistoryManager({ dispatch: () => {
        } });
    });

    test('Mid-PA Substitution should not split narrative block or cause NaN summary', () => {
        const game = {
            id: 'test-game',
            away: 'Away',
            home: 'Home',
            status: 'ongoing',
            rules: { innings: 9 },
            roster: {
                away: [
                    { slot: 1, starter: { id: 'p1', name: 'Starter', number: '1' }, current: { id: 'p1', name: 'Starter', number: '1' }, history: [] },
                ],
                home: [],
            },
            subs: { away: [{ id: 'sub1', name: 'Sub', number: '99', pos: 'PH' }] },
            events: {},
            columns: [{ id: 'col-1-0', inning: 1 }],
            actionLog: [
                {
                    id: 'start',
                    type: ActionTypes.GAME_START,
                    payload: {
                        id: 'test-game',
                        away: 'Away',
                        home: 'Home',
                        initialRosters: {
                            away: [{ id: 'p1', name: 'Starter', number: '1' }],
                            home: [],
                        },
                    },
                },
                {
                    id: 'p1',
                    type: ActionTypes.PITCH,
                    payload: { activeTeam: 'away', activeCtx: { b: 0, i: 1, col: 'col-1-0' }, type: 'ball' },
                },
                {
                    id: 'p2',
                    type: ActionTypes.PITCH,
                    payload: { activeTeam: 'away', activeCtx: { b: 0, i: 1, col: 'col-1-0' }, type: 'ball' },
                },
                // Substitution with activeCtx but MISSING activeTeam (mimicking actual payload)
                {
                    id: 'sub1',
                    type: ActionTypes.SUBSTITUTION,
                    payload: {
                        // activeTeam: 'away', // REMOVED
                        team: 'away',
                        rosterIndex: 0,
                        subParams: { id: 'sub1', name: 'Sub', number: '99', pos: 'PH' },
                        activeCtx: { b: 0, i: 1, col: 'col-1-0' },
                    },
                },
                {
                    id: 'p3',
                    type: ActionTypes.PITCH,
                    payload: { activeTeam: 'away', activeCtx: { b: 0, i: 1, col: 'col-1-0' }, type: 'strike' },
                },
                {
                    id: 'p4',
                    type: ActionTypes.PITCH,
                    payload: { activeTeam: 'away', activeCtx: { b: 0, i: 1, col: 'col-1-0' }, type: 'strike' },
                },
            ],
        };

        // const linearHistory = historyManager.generateLinearHistory(game); // Unused
        const feed = engine.generateNarrative(game, historyManager);

        // Assertion 1: Should be 1 Inning Block (Top 1)
        expect(feed).toHaveLength(1);

        // Assertion 3: Grouping
        const playItems = feed[0].items.filter(i => i.type === 'PLAY');

        // If it splits, we'll see 2 items.
        // Item 1: Starter with Ball, Ball
        // Item 2: Sub with Sub, Strike, Strike (or just Sub?)

        expect(playItems).toHaveLength(1); // We want ONE block for the PA
    });
});
