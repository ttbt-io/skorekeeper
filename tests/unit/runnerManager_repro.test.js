
import { RunnerManager } from '../../frontend/game/runnerManager.js';

describe('RunnerManager getRunnersOnBase Repro', () => {
    let runnerManager;

    beforeEach(() => {
        runnerManager = new RunnerManager({ dispatch: jest.fn(), renderCSO: jest.fn(), getBatterId: () => 'p1' });
    });

    test('should NOT include subsequent batters in the same inning as runners on base', () => {
        const game = {
            roster: {
                away: [
                    { current: { name: 'P1', id: 'p1' } },
                    { current: { name: 'P2', id: 'p2' } },
                    { current: { name: 'P3', id: 'p3' } },
                    { current: { name: 'P4', id: 'p4' } },
                    { current: { name: 'P5', id: 'p5' } },
                    { current: { name: 'P6', id: 'p6' } },
                    { current: { name: 'P7', id: 'p7' } },
                    { current: { name: 'P8', id: 'p8' } },
                    { current: { name: 'P9', id: 'p9' } },
                ],
            },
            columns: [
                { inning: 1, id: 'col-1-0', leadRow: { away: 0 } },
            ],
            events: {
                // Batter 0 (P1) hits 1B
                'away-0-col-1-0': { paths: [1, 0, 0, 0], outcome: '1B' },
                // Batter 1 (P2) hits 1B, P1 stays at 1st? No, P1 moves to 2nd.
                'away-1-col-1-0': { paths: [1, 0, 0, 0], outcome: '1B' },
            },
        };

        // If we are at Batter 0 (editing), Batter 1 should NOT be considered a runner on base for Batter 0.
        const ctx = { i: 1, b: 0, col: 'col-1-0' };
        const runners = runnerManager.getRunnersOnBase(game, 'away', ctx);

        // Expected: Empty (P1 is the batter, nobody was on before him)
        expect(runners).toEqual([]);
    });

    test('should correctly identify runners who were on BEFORE the current batter', () => {
        const game = {
            roster: {
                away: [
                    { current: { name: 'P1', id: 'p1' } },
                    { current: { name: 'P2', id: 'p2' } },
                ],
            },
            columns: [
                { inning: 1, id: 'col-1-0', leadRow: { away: 0 } },
            ],
            events: {
                'away-0-col-1-0': { paths: [1, 0, 0, 0], outcome: '1B' },
            },
        };

        // Batter 1 is up. Batter 0 should be on 1st.
        const ctx = { i: 1, b: 1, col: 'col-1-0' };
        const runners = runnerManager.getRunnersOnBase(game, 'away', ctx);

        expect(runners).toHaveLength(1);
        expect(runners[0].idx).toBe(0);
        expect(runners[0].base).toBe(0);
    });
});
