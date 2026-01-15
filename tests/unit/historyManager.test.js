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

import { HistoryManager } from '../../frontend/game/historyManager.js';

describe('HistoryManager.generateLinearHistory', () => {
    let historyManager;

    beforeEach(() => {
        historyManager = new HistoryManager({ dispatch: jest.fn() });
    });

    const createPitch = (id, count, type = 'strike', ctx = { i: 1, b: 0, col: 'c1' }, team = 'away') => ({
        id,
        type: 'PITCH',
        payload: { type, count, activeCtx: ctx, activeTeam: team },
    });

    const createResult = (id, res, ctx = { i: 1, b: 0, col: 'c1' }, team = 'away', advs = []) => ({
        id,
        type: 'PLAY_RESULT',
        payload: {
            bipState: { res, base: '1B', type: 'Ground', seq: [6, 3] },
            activeCtx: ctx,
            activeTeam: team,
            runnerAdvancements: advs,
        },
    });

    test('should build a simple linear history with headers', () => {
        const game = {
            actionLog: [
                createPitch('a1', '0-0'),
                createPitch('a2', '0-1'),
                createResult('a3', 'Out'),
            ],
        };

        const history = historyManager.generateLinearHistory(game);

        // Expected: [Header, PLAY]
        expect(history).toHaveLength(2);
        expect(history[0].type).toBe('INNING_HEADER');
        expect(history[1].type).toBe('PLAY');
        expect(history[1].ctxKey).toBe('1-away-0-c1');
        expect(history[1].events).toHaveLength(3);
        expect(history[1].stateBefore.outs).toBe(0);
        expect(history[1].stateAfter.outs).toBe(1);
    });

    test('should resolve player names in runner snapshots', () => {
        const ctx1 = { i: 1, b: 0, col: 'c1' };
        const ctx2 = { i: 1, b: 1, col: 'c1' };
        const game = {
            roster: {
                away: [
                    { current: { name: 'Smith', number: '1' } },
                    { current: { name: 'Davis', number: '2' } },
                ],
            },
            actionLog: [
                createResult('a1', 'Safe', ctx1), // Smith on 1B
                createResult('a2', 'Out', ctx2),   // Davis is up
            ],
        };

        const history = historyManager.generateLinearHistory(game);

        // Header, Smith PLAY, Davis PLAY
        expect(history).toHaveLength(3);

        // Davis PA (history[2]) should see Smith on 1st
        const davisPA = history[2];
        expect(davisPA.stateBefore.runners[0]).toBeDefined();
        expect(davisPA.stateBefore.runners[0].name).toBe('Smith');
    });

    test('should reduce undos from the history', () => {
        const game = {
            actionLog: [
                createPitch('a1', '0-0'),
                { type: 'UNDO', payload: { refId: 'a1' } },
                createPitch('a2', '0-0'),
            ],
        };

        const history = historyManager.generateLinearHistory(game);

        // Expected: [Header, PLAY]
        expect(history).toHaveLength(2);
        expect(history[1].events).toHaveLength(1);
        expect(history[1].events[0].id).toBe('a2');
    });

    test('should handle historical play edits by appending corrections chronologically', () => {
        const ctx1 = { i: 1, b: 0, col: 'c1' };
        const ctx2 = { i: 1, b: 1, col: 'c1' };

        const game = {
            actionLog: [
                createResult('a1', 'Out', ctx1),
                createResult('a2', 'Out', ctx2),
                // Edit a1: Change Out to Safe
                createResult('a3', 'Safe', ctx1),
            ],
        };

        const history = historyManager.generateLinearHistory(game);

        // Expected: [Header, a1 (stricken), a2, a3 (correction)]
        expect(history).toHaveLength(4);
        expect(history[1].events[0].id).toBe('a1');
        expect(history[1].isStricken).toBe(true);

        expect(history[2].events[0].id).toBe('a2');
        expect(history[2].isStricken).toBe(false);

        expect(history[3].events[0].id).toBe('a3');
        expect(history[3].isCorrection).toBe(true);
        expect(history[3].isStricken).toBe(false);
    });

    test('should propagate state changes through subsequent plays after a correction', () => {
        const ctx1 = { i: 1, b: 0, col: 'c1' };
        const ctx2 = { i: 1, b: 1, col: 'c1' };

        const game = {
            actionLog: [
                createResult('a1', 'Out', ctx1), // 1 Out
                createResult('a2', 'Out', ctx2), // 2 Outs
                // Edit a1: Change Out to Safe
                createResult('a3', 'Safe', ctx1), // 0 Outs (stricken a1)
            ],
        };

        const history = historyManager.generateLinearHistory(game);

        // Header, a1 (Stricken), a2, a3 (Correction)
        // a1 (Stricken)
        expect(history[1].stateBefore.outs).toBe(0);

        // a2
        expect(history[2].stateBefore.outs).toBe(0); // Because a1 is stricken
        expect(history[2].stateAfter.outs).toBe(1);

        // a3 (Correction)
        expect(history[3].stateBefore.outs).toBe(1);
        expect(history[3].stateAfter.outs).toBe(1); // Safe hit doesn't add out
    });

    test('should handle multiple corrections of the same play', () => {
        const ctx1 = { i: 1, b: 0, col: 'c1' };
        const game = {
            actionLog: [
                createResult('a1', 'Out', ctx1),
                createResult('a2', 'Safe', ctx1), // first correction
                createResult('a3', 'Out', ctx1),   // second correction
            ],
        };

        const history = historyManager.generateLinearHistory(game);

        // Expected: [Header, a1 (stricken), a2 (stricken, correction), a3 (correction)]
        expect(history).toHaveLength(4);
        expect(history[1].isStricken).toBe(true);
        expect(history[2].isStricken).toBe(true);
        expect(history[2].isCorrection).toBe(true);
        expect(history[3].isStricken).toBe(false);
        expect(history[3].isCorrection).toBe(true);
    });

    test('should reset state on inning transitions', () => {
        const ctx1 = { i: 1, b: 0, col: 'c1' }; // Top 1
        const ctx2 = { i: 1, b: 0, col: 'c1' }; // Bottom 1 (team changed)

        const game = {
            actionLog: [
                createResult('a1', 'Out', ctx1, 'away'),
                createResult('a2', 'Out', ctx2, 'home'),
            ],
        };

        const history = historyManager.generateLinearHistory(game);

        // [Header Top 1, a1, Header Bottom 1, a2]
        expect(history).toHaveLength(4);
        expect(history[1].stateBefore.outs).toBe(0);
        expect(history[1].stateAfter.outs).toBe(1);

        expect(history[3].stateBefore.outs).toBe(0); // Reset for Bottom 1
        expect(history[3].stateAfter.outs).toBe(1);
    });

    test('should correctly track runners and LOB in complex scenario', () => {
        // P1 hits single. P2 grounds out (P1 advances). P3 hits double (P1 scores). P4 strikes out. P5 hits a fly out.
        const ctx1 = { i: 1, b: 0, col: 'c1' };
        const ctx2 = { i: 1, b: 1, col: 'c1' };
        const ctx3 = { i: 1, b: 2, col: 'c1' };
        const ctx4 = { i: 1, b: 3, col: 'c1' };
        const ctx5 = { i: 1, b: 4, col: 'c1' };

        const game = {
            roster: {
                away: [
                    { starter: { id: 'p1', name: 'P1' }, current: { id: 'p1', name: 'P1' } },
                    { starter: { id: 'p2', name: 'P2' }, current: { id: 'p2', name: 'P2' } },
                    { starter: { id: 'p3', name: 'P3' }, current: { id: 'p3', name: 'P3' } },
                    { starter: { id: 'p4', name: 'P4' }, current: { id: 'p4', name: 'P4' } },
                    { starter: { id: 'p5', name: 'P5' }, current: { id: 'p5', name: 'P5' } },
                ],
            },
            actionLog: [
                // P1 Single
                {
                    id: 'a1', type: 'PLAY_RESULT',
                    payload: { activeCtx: ctx1, activeTeam: 'away', bipState: { res: 'Safe', base: '1B' } },
                },
                // P2 Ground Out, P1 advances to 2nd
                {
                    id: 'a2', type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: ctx2, activeTeam: 'away', bipState: { res: 'Out', base: '1B' },
                        runnerAdvancements: [{ base: 0, outcome: 'To 2nd', key: 'away-0-c1' }],
                    },
                },
                // P3 Double, P1 scores
                {
                    id: 'a3', type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: ctx3, activeTeam: 'away', bipState: { res: 'Safe', base: '2B' },
                        runnerAdvancements: [{ base: 1, outcome: 'Score', key: 'away-0-c1' }],
                    },
                },
                // P4 K
                {
                    id: 'a4', type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: ctx4, activeTeam: 'away',
                        bipState: { res: 'Out', base: '', type: 'K', seq: [] },
                    },
                },
                // P5 F1
                {
                    id: 'a5', type: 'PLAY_RESULT',
                    payload: { activeCtx: ctx5, activeTeam: 'away', bipState: { res: 'Out', base: '1B' } },
                },
            ],
        };

        const history = historyManager.generateLinearHistory(game);

        // Header + 5 Plays
        expect(history).toHaveLength(6);

        // Verify state after P3 Double: P3 should be on 2nd, P1 scored
        const afterP3 = history[3].stateAfter;
        expect(afterP3.score.away).toBe(1);
        expect(afterP3.runners[1]).toBeDefined();
        expect(afterP3.runners[1].name).toBe('P3');

        // Verify state before P4 K: Should see P3 on 2nd
        const beforeP4 = history[4].stateBefore;
        expect(beforeP4.runners[1]).toBeDefined();
        expect(beforeP4.runners[1].name).toBe('P3');

        // Verify state before P5 F1: Should still see P3 on 2nd
        const beforeP5 = history[5].stateBefore;
        expect(beforeP5.outs).toBe(2);
        expect(beforeP5.runners[1]).toBeDefined();
        expect(beforeP5.runners[1].name).toBe('P3');

        // Final state after 3 outs: Runners cleared in stateAfter for inning reset
        const final = history[5].stateAfter;
        expect(final.outs).toBe(0);
        expect(final.runners[1]).toBeDefined();
        expect(final.runners[1].name).toBe('P3');
    });

    test('should correctly handle RUNNER_ADVANCE action', () => {
        const game = {
            roster: {
                away: [
                    { starter: { id: 'p1', name: 'P1' }, current: { id: 'p1', name: 'P1' } },
                ],
            },
            actionLog: [
                // P1 Single
                {
                    id: 'a1', type: 'PLAY_RESULT',
                    payload: { activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Safe', base: '1B' } },
                },
                // Manual Advance P1 from 1st to 2nd (e.g. on Wild Pitch)
                {
                    id: 'a2', type: 'RUNNER_ADVANCE',
                    payload: {
                        activeCtx: { i: 1, b: 1, col: 'c1' }, activeTeam: 'away',
                        runners: [{ base: 0, outcome: 'To 2nd', key: 'away-0-c1' }],
                    },
                },
            ],
        };

        const history = historyManager.generateLinearHistory(game);
        expect(history).toHaveLength(3); // Header, P1 PLAY, Advance PLAY

        // Verify P1 moved to 2nd base
        const afterAdv = history[2].stateAfter;
        expect(afterAdv.runners[0]).toBeNull();
        expect(afterAdv.runners[1]).toBeDefined();
        expect(afterAdv.runners[1].name).toBe('P1');
    });

    test('should preserve runners with outcome Stay in PLAY_RESULT', () => {
        const game = {
            roster: {
                away: [
                    { starter: { id: 'p1', name: 'P1' }, current: { id: 'p1', name: 'P1' } },
                    { starter: { id: 'p2', name: 'P2' }, current: { id: 'p2', name: 'P2' } },
                ],
            },
            actionLog: [
                // P1 Single
                {
                    id: 'a1', type: 'PLAY_RESULT',
                    payload: { activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Safe', base: '1B' } },
                },
                // P2 Fly Out, P1 Stays on 1st
                {
                    id: 'a2', type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: { i: 1, b: 1, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Out', base: '1B' },
                        runnerAdvancements: [{ base: 0, outcome: 'Stay', key: 'away-0-c1' }],
                    },
                },
            ],
        };

        const history = historyManager.generateLinearHistory(game);
        expect(history[2].stateAfter.runners[0]).toBeDefined();
        expect(history[2].stateAfter.runners[0].name).toBe('P1');
    });

    test('should handle walk followed by manual runner advance without stomping', () => {
        const game = {
            roster: {
                away: [
                    { starter: { id: 'p1', name: 'Alpha' }, current: { id: 'p1', name: 'Alpha' } },
                    { starter: { id: 'p2', name: 'Beta' }, current: { id: 'p2', name: 'Beta' } },
                ],
            },
            actionLog: [
                { id: 'a1', type: 'PLAY_RESULT', payload: { activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Safe', base: '1B' } } },
                // Simulating Walk via Play Result for test stability
                { id: 'a2', type: 'PLAY_RESULT', payload: { activeCtx: { i: 1, b: 1, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Safe', base: '1B' } } },
                { id: 'a3', type: 'RUNNER_ADVANCE', payload: { activeCtx: { i: 1, b: 1, col: 'c1' }, activeTeam: 'away', runners: [{ base: 0, outcome: 'To 2nd', key: 'away-0-c1' }] } },
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        const final = history[2].stateAfter;
        // Verify both runners are accounted for
        expect(final.runners[0]).toBeDefined();
        expect(final.runners[0].name).toBe('Beta'); // Batter
        expect(final.runners[1]).toBeDefined();
        expect(final.runners[1].name).toBe('Alpha'); // Runner
    });
});

describe('HistoryManager.getUndoTargetId', () => {
    let historyManager;

    beforeEach(() => {
        historyManager = new HistoryManager({ dispatch: jest.fn() });
    });

    test('should return the last action if no undos', () => {
        const log = [{ id: 'a1' }, { id: 'a2' }];
        expect(historyManager.getUndoTargetId(log)).toBe('a2');
    });

    test('should skip undone events', () => {
        const log = [
            { id: 'a1' },
            { id: 'a2' },
            { type: 'UNDO', payload: { refId: 'a2' } },
        ];
        expect(historyManager.getUndoTargetId(log)).toBe('a1');
    });

    test('should return null if all events undone', () => {
        const log = [
            { id: 'a1' },
            { type: 'UNDO', payload: { refId: 'a1' } },
        ];
        expect(historyManager.getUndoTargetId(log)).toBeNull();
    });
});

describe('HistoryManager.getRedoTargetId', () => {
    let historyManager;

    beforeEach(() => {
        historyManager = new HistoryManager({ dispatch: jest.fn() });
    });

    test('should return the target of the last UNDO', () => {
        const log = [
            { id: 'a1', type: 'PITCH' },
            { id: 'ua1', type: 'UNDO', payload: { refId: 'a1' } },
        ];
        expect(historyManager.getRedoTargetId(log)).toBe('ua1');
    });

    test('should return null if the last action was not an UNDO', () => {
        const log = [
            { id: 'a1' },
            { type: 'UNDO', payload: { refId: 'a1' } },
            { id: 'a2' },
        ];
        expect(historyManager.getRedoTargetId(log)).toBeNull();
    });
});

describe('HistoryManager Name Resolution', () => {
    let historyManager;
    const roster = {
        away: [
            { starter: { id: 'p1', name: 'Alice', number: '1' }, current: { id: 'p1', name: 'Alice', number: '1' } },
            { starter: { id: 'p2', name: 'Bob', number: '2' }, current: { id: 'p2', name: 'Bob', number: '2' } },
            { starter: { id: 'p3', name: 'Charlie', number: '3' }, current: { id: 'p3', name: 'Charlie', number: '3' } },
            { starter: { id: 'p4', name: 'Dave', number: '4' }, current: { id: 'p4', name: 'Dave', number: '4' } },
        ],
        home: [],
    };

    beforeEach(() => {
        historyManager = new HistoryManager({ dispatch: jest.fn() });
    });

    const createPlay = (id, res, base, batterIdx, advs = []) => ({
        id,
        type: 'PLAY_RESULT',
        payload: {
            activeCtx: { i: 1, b: batterIdx, col: 'c1' },
            activeTeam: 'away',
            bipState: { res, base, type: 'HIT' },
            runnerAdvancements: advs,
        },
    });

    test('should identify batter name correctly', () => {
        const game = {
            roster,
            actionLog: [createPlay('a1', 'Safe', '1B', 0)],
        };
        const history = historyManager.generateLinearHistory(game);
        expect(history[1].batter.name).toBe('Alice');
    });

    test('should identify runner name after reaching base', () => {
        const game = {
            roster,
            actionLog: [
                createPlay('a1', 'Safe', '1B', 0), // Alice 1B
                createPlay('a2', 'Out', '', 1),    // Bob Out
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        const state = history[2].stateBefore; // Bob's PA
        expect(state.runners[0].name).toBe('Alice');
    });

    test('should identify runner name after multi-base hit', () => {
        const game = {
            roster,
            actionLog: [
                createPlay('a1', 'Safe', '3B', 0), // Alice 3B
                createPlay('a2', 'Out', '', 1),    // Bob Out
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        const state = history[2].stateBefore; // Bob's PA
        expect(state.runners[2].name).toBe('Alice');
    });

    test('should maintain runner identity through advancements', () => {
        const game = {
            roster,
            actionLog: [
                createPlay('a1', 'Safe', '1B', 0), // Alice 1B
                // Bob hits, Alice to 3rd
                createPlay('a2', 'Safe', '1B', 1, [{ base: 0, outcome: 'To 3rd', key: 'away-0-c1' }]),
                createPlay('a3', 'Out', '', 2), // Charlie Out
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        const state = history[3].stateBefore; // Charlie's PA
        expect(state.runners[0].name).toBe('Bob');   // 1st
        expect(state.runners[2].name).toBe('Alice'); // 3rd
    });

    test('should handle substitutions and track new player name', () => {
        const game = {
            roster: JSON.parse(JSON.stringify(roster)),
            actionLog: [
                createPlay('a1', 'Out', '', 0), // Alice Out
                {
                    id: 'sub1',
                    type: 'SUBSTITUTION',
                    payload: {
                        team: 'away',
                        rosterIndex: 1, // Replace Bob
                        subParams: { id: 'p99', name: 'Zack', number: '99' },
                    },
                },
                createPlay('a2', 'Safe', '1B', 1), // Zack 1B
                createPlay('a3', 'Out', '', 2),    // Charlie Out
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        const state = history[3].stateBefore; // Charlie's PA
        expect(state.runners[0].name).toBe('Zack');
    });

    test('should correctly identify batter who reaches base then advances in same play', () => {
        const game = {
            roster,
            actionLog: [
                // Alice hits single, advances to 2nd on throw
                createPlay('a1', 'Safe', '1B', 0),
                {
                    id: 'adv1',
                    type: 'RUNNER_ADVANCE',
                    payload: {
                        activeCtx: { i: 1, b: 0, col: 'c1' },
                        activeTeam: 'away',
                        runners: [{ base: 0, outcome: 'To 2nd', key: 'away-0-c1' }],
                    },
                },
                createPlay('a2', 'Out', '', 1), // Bob Out
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        const state = history[2].stateBefore; // Bob's PA (Index 2)
        expect(state.runners[1].name).toBe('Alice');
    });

    test('should resolve all runner names in a Triple Play', () => {
        const game = {
            roster,
            actionLog: [
                createPlay('a1', 'Safe', '1B', 0), // Alice 1B
                createPlay('a2', 'Safe', '1B', 1, [{ base: 0, outcome: 'To 2nd', key: 'away-0-c1' }]), // Bob 1B, Alice 2B
                createPlay('a3', 'Safe', '1B', 2, [{ base: 0, outcome: 'To 2nd', key: 'away-1-c1' }, { base: 1, outcome: 'To 3rd', key: 'away-0-c1' }]), // Charlie 1B, Bob 2B, Alice 3B
                // Dave hits TP
                {
                    id: 'tp',
                    type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: { i: 1, b: 3, col: 'c1' },
                        activeTeam: 'away',
                        bipState: { res: 'Line', type: 'TP', base: '', seq: [6] },
                        runnerAdvancements: [
                            { base: 2, outcome: 'Out', key: 'away-0-c1' }, // Alice Out
                            { base: 1, outcome: 'Out', key: 'away-1-c1' }, // Bob Out
                            { base: 0, outcome: 'Out', key: 'away-2-c1' }, // Charlie Out? Or Batter Out?
                        ],
                    },
                },
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        const tpItem = history[history.length - 1]; // The TP play

        // Check resolved names in the event payload
        const advs = tpItem.events[0].payload.runnerAdvancements;
        expect(advs[0].resolvedName).toBe('Alice');
        expect(advs[1].resolvedName).toBe('Bob');
    });

    test('should resolve runners correctly after a Walk (BB) and then a Grand Slam', () => {
        const game = {
            roster,
            actionLog: [
                // 1. Alice Single -> 1B
                createPlay('a1', 'Safe', '1B', 0),
                // 2. Bob Single -> 1B, Alice -> 2B
                createPlay('a2', 'Safe', '1B', 1, [{ base: 0, outcome: 'To 2nd', key: 'away-0-c1' }]),
                // 3. Charlie Walks (BB) -> 1B, Bob -> 2B, Alice -> 3B
                {
                    id: 'p1', type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: { i: 1, b: 2, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Safe', base: '1B' },
                        runnerAdvancements: [
                            { base: 1, outcome: 'To 3rd', key: 'away-0-c1' }, // Alice 2B -> 3B
                            { base: 0, outcome: 'To 2nd', key: 'away-1-c1' }, // Bob 1B -> 2B
                        ],
                    },
                },
                // 4. Dave Grand Slam -> Home
                {
                    id: 'hr',
                    type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: { i: 1, b: 3, col: 'c1' },
                        activeTeam: 'away',
                        bipState: { res: 'Safe', base: 'Home', type: 'HIT' },
                        runnerAdvancements: [
                            { base: 2, outcome: 'Score', key: 'away-0-c1' }, // Alice
                            { base: 1, outcome: 'Score', key: 'away-1-c1' }, // Bob
                            { base: 0, outcome: 'Score', key: 'away-2-c1' }, // Charlie
                        ],
                    },
                },
            ],
        };
        const history = historyManager.generateLinearHistory(game);

        // Check Charlie (batter index 2) made it to 1st base after the walk
        // history[0]=Header, [1]=a1, [2]=a2, [3]=p1, [4]=hr
        const hrState = history[4].stateBefore; // State before Dave's HR
        expect(hrState.runners[0]).toBeDefined();
        expect(hrState.runners[0].name).toBe('Charlie');

        // Check resolved names in HR advancements
        const hrAdvs = history[4].events[0].payload.runnerAdvancements;
        expect(hrAdvs[2].resolvedName).toBe('Charlie');
    });

    test('should resolve missing runner on 1st using previous batter heuristic', () => {
        const game = {
            roster,
            actionLog: [
                // 1. Alice (index 0) Walks (but PITCH logic fails to place her on 1st for some reason)
                // We simulate this by having NO event that places her.
                { id: 'p1', type: 'PITCH', payload: { activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away', type: 'ball', count: '0-0' } },
                // 2. Bob (index 1) hits Double, Alice scores from 1st
                {
                    id: 'd1',
                    type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: { i: 1, b: 1, col: 'c1' },
                        activeTeam: 'away',
                        bipState: { res: 'Safe', base: '2B', type: 'HIT' },
                        runnerAdvancements: [
                            { base: 0, outcome: 'Score', key: 'away-0-c1' }, // Advancement for Alice (who is missing from state)
                        ],
                    },
                },
            ],
        };
        const history = historyManager.generateLinearHistory(game);

        // Bob's PA (Index 2)
        const advs = history[2].events[0].payload.runnerAdvancements;
        expect(advs[0].resolvedName).toBe('Alice'); // Should resolve to previous batter (Alice)
    });

    test('should resolve runners correctly after Catcher Interference (CI)', () => {
        const game = {
            roster,
            actionLog: [
                // 1. Alice Walks (BB) -> 1B
                { id: 'p1', type: 'PLAY_RESULT', payload: { activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Safe', base: '1B' } } },
                // 2. Bob reaches on CI -> 1B, Alice -> 2B
                {
                    id: 'ci', type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: { i: 1, b: 1, col: 'c1' }, activeTeam: 'away',
                        batterId: 'p2',
                        bipState: { res: 'Safe', base: '1B', type: 'CI', seq: [] },
                        runnerAdvancements: [
                            { key: 'away-0-c1', base: 0, outcome: 'To 2nd' },
                        ],
                    },
                },
                // 3. Charlie hits FC, Bob out at 2nd?
                {
                    id: 'fc', type: 'PLAY_RESULT',
                    payload: {
                        activeCtx: { i: 1, b: 2, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Safe', base: '1B', type: 'FC' },
                        runnerAdvancements: [
                            { key: 'away-1-c1', base: 0, outcome: 'Out' }, // Bob
                        ],
                    },
                },
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        const fcItem = history[3]; // Header, p1, ci, fc
        expect(fcItem.events[0].payload.runnerAdvancements[0].resolvedName).toBe('Bob');
    });

    test('should append supplemental MANUAL_PATH_OVERRIDE to existing play', () => {
        const game = {
            actionLog: [
                { id: 'a1', type: 'PLAY_RESULT', payload: { activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Safe', base: '1B' } } },
                {
                    id: 'm1', type: 'MANUAL_PATH_OVERRIDE',
                    payload: {
                        key: 'away-0-c1',
                        data: { outcome: 'Adv', paths: [0, 1, 0, 0] }, // Move to 2nd
                        activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away',
                    },
                },
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        expect(history).toHaveLength(2); // Header + 1 Play
        const playItem = history[1];
        expect(playItem.events).toHaveLength(2);
        expect(playItem.events[0].id).toBe('a1');
        expect(playItem.events[1].id).toBe('m1');
        expect(playItem.isStricken).toBe(false);
    });

    test('should split items after CLEAR_DATA', () => {
        const game = {
            actionLog: [
                { id: 'a1', type: 'PLAY_RESULT', payload: { activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away', bipState: { res: 'Safe', base: '1B' } } },
                { id: 'c1', type: 'CLEAR_DATA', payload: { activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away' } },
                { id: 'a2', type: 'PITCH', payload: { activeCtx: { i: 1, b: 0, col: 'c1' }, activeTeam: 'away', type: 'strike', count: '0-1' } },
            ],
        };
        const history = historyManager.generateLinearHistory(game);
        // Header, a1 (stricken), c1, a2
        expect(history).toHaveLength(4);
        expect(history[1].events[0].id).toBe('a1');
        expect(history[1].isStricken).toBe(true);
        expect(history[2].events[0].id).toBe('c1');
        expect(history[2].isStricken).toBe(false);
        expect(history[3].events[0].id).toBe('a2');
        expect(history[3].isStricken).toBe(false);
    });
});
