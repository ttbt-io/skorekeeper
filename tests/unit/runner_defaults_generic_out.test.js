
import { RunnerManager } from '../../frontend/game/runnerManager.js';
import { BiPResultOut, BiPResultGround } from '../../frontend/constants.js';

describe('RunnerManager - Repro Default Advances for OUT', () => {
    let runnerManager;
    let mockDispatch;
    let mockGetBatterId;
    let mockRenderCSO;

    beforeEach(() => {
        mockDispatch = jest.fn();
        mockRenderCSO = jest.fn();
        mockGetBatterId = jest.fn();
        runnerManager = new RunnerManager({
            dispatch: mockDispatch,
            renderCSO: mockRenderCSO,
            getBatterId: mockGetBatterId,
        });
    });

    const createRunners = (bases) => bases.map(b => ({ key: `runner-${b}`, base: b, outcome: 'Stay' }));

    const checkOutcomes = (runners, expected) => {
        runners.forEach((r, i) => {
            expect(r.outcome).toBe(expected[i]);
        });
    };

    test('Runners should STAY on Generic OUT (BiPResultOut)', () => {
        const runners = createRunners([0]); // Runner on 1st
        const bip = { res: BiPResultOut, base: '', type: 'Out' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // BEHAVIOR: 'Stay' (Generic Out should NOT advance)
        checkOutcomes(result, ['Stay']);
    });

    test('Runners should STAY on Interference (BiPResultOut)', () => {
        const runners = createRunners([0]); // Runner on 1st
        const bip = { res: BiPResultOut, base: '', type: 'Int' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // BEHAVIOR: 'Stay'
        checkOutcomes(result, ['Stay']);
    });

    test('Runners should STAY on BOO (BiPResultOut)', () => {
        const runners = createRunners([0]); // Runner on 1st
        const bip = { res: BiPResultOut, base: '', type: 'BOO' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // BEHAVIOR: 'Stay'
        checkOutcomes(result, ['Stay']);
    });

    test('Runners should ADVANCE on Ground Out (BiPResultGround)', () => {
        const runners = createRunners([0]); // Runner on 1st
        const bip = { res: BiPResultGround, base: '', type: 'Ground' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // BEHAVIOR: 'To 2nd' (Ground Out advances forced runners)
        checkOutcomes(result, ['To 2nd']);
    });

    test('Runners should ADVANCE on Dropped 3rd Strike (BiPResultOut w/ K)', () => {
        const runners = createRunners([0]); // Runner on 1st
        const bip = { res: BiPResultOut, base: '', type: 'K' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // BEHAVIOR: 'To 2nd' (Dropped 3rd is like a grounder/force)
        checkOutcomes(result, ['To 2nd']);
    });
});
