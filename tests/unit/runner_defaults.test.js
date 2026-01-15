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

import { RunnerManager } from '../../frontend/game/runnerManager.js';

describe('RunnerManager - calculateDefaultAdvances', () => {
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

    // Helper to check expected outcomes
    const checkOutcomes = (runners, expected) => {
        runners.forEach((r, i) => {
            expect(r.outcome).toBe(expected[i]);
        });
    };

    test('Advancement (+1) for Safe Hits (Single)', () => {
        const runners = createRunners([0, 1]); // Runners on 1st, 2nd
        const bip = { res: 'Safe', base: '1B', type: 'HIT' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // Batter safe at 1st -> forces R1 to 2nd, R2 to 3rd
        checkOutcomes(result, ['To 2nd', 'To 3rd']);
    });

    test('Advancement (+1) for Ground Outs (Forced)', () => {
        const runners = createRunners([0]); // Runner on 1st
        const bip = { res: 'Ground', base: '', type: 'Ground' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // Ground ball -> R1 forced to 2nd (unless DP, but default is advance)
        checkOutcomes(result, ['To 2nd']);
    });

    test('Advancement (+1) for Dropped 3rd Strike (Out)', () => {
        const runners = createRunners([0]); // Runner on 1st (Assuming 2 outs, otherwise occupied 1st is auto-out for batter)
        const bip = { res: 'Out', base: '', type: 'K' }; // K on dropped
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // Dropped 3rd strike -> Batter becomes runner -> R1 forced
        checkOutcomes(result, ['To 2nd']);
    });

    test('NO Advancement for Air Outs (Fly)', () => {
        const runners = createRunners([0, 2]); // 1st and 3rd
        const bip = { res: 'Fly', base: '', type: 'Fly' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // Fly out -> Runners stay (tag up logic is manual)
        checkOutcomes(result, ['Stay', 'Stay']);
    });

    test('NO Advancement for Air Outs (Line)', () => {
        const runners = createRunners([1]); // 2nd
        const bip = { res: 'Line', base: '', type: 'Line' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        checkOutcomes(result, ['Stay']);
    });

    test('Identify Forced Runners Correctly', () => {
        // Scenario: Runners on 2nd and 3rd. Ground ball.
        // Batter to 1st (if safe) or out.
        // R2 (on 2nd) and R3 (on 3rd) are NOT forced.

        const runners = createRunners([1, 2]); // 2nd, 3rd
        const bip = { res: 'Ground', base: '', type: 'Ground' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // Expect Stay because force is broken at 1st base
        checkOutcomes(result, ['Stay', 'Stay']);
    });

    test('Identify Forced Runners Chain', () => {
        // Scenario: Bases Loaded. Ground ball.
        const runners = createRunners([0, 1, 2]); // 1st, 2nd, 3rd
        const bip = { res: 'Ground', base: '', type: 'Ground' };
        const events = {};

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events);

        // All forced
        checkOutcomes(result, ['To 2nd', 'To 3rd', 'Score']);
    });

    test('3rd Out on Batter -> Runners Stay (Fly)', () => {
        const runners = createRunners([0, 1]); // 1st, 2nd
        const bip = { res: 'Fly', base: '', type: 'Fly' };
        const events = {};
        const currentOuts = 2; // +1 out = 3

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events, currentOuts);

        checkOutcomes(result, ['Stay', 'Stay']);
    });

    test('3rd Out on Batter -> Runners Stay (Ground Out)', () => {
        const runners = createRunners([0, 1, 2]); // Bases loaded
        const bip = { res: 'Out', base: '', type: 'Ground' };
        const events = {};
        const currentOuts = 2;

        const result = runnerManager.calculateDefaultAdvances(runners, bip, events, currentOuts);

        // Even though forced, it's the 3rd out, so defaults to Stay.
        checkOutcomes(result, ['Stay', 'Stay', 'Stay']);
    });
});
