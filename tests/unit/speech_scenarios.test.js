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

import { parseEvent, AmbiguityError } from '../../frontend/utils/parser.js';
import { cleanTranscript } from '../../frontend/utils/fuzzy.js';
import { ActionTypes, computeStateFromLog, gameReducer } from '../../frontend/reducer.js';

// Helper to initialize a clean softball game state with 9 players on each roster
function createNewGame() {
    const startAction = {
        type: ActionTypes.GAME_START,
        payload: {
            id: 'test-game-123',
            schemaVersion: 1,
            date: '2026-05-22',
            away: 'Away Team',
            home: 'Home Team',
            awayTeamId: 't-away',
            homeTeamId: 't-home',
            initialRosters: {
                away: [
                    { id: 'a1', name: 'Alice', number: '10' },
                    { id: 'a2', name: 'Becky', number: '2' },
                    { id: 'a3', name: 'Charlie', number: '14' },
                    { id: 'a4', name: 'Deb', number: '16' },
                    { id: 'a5', name: 'Elsa', number: '18' },
                    { id: 'a6', name: 'Fiona', number: '20' },
                    { id: 'a7', name: 'Grace', number: '22' },
                    { id: 'a8', name: 'Heidi', number: '24' },
                    { id: 'a9', name: 'Ivy', number: '26' },
                ],
                home: [
                    { id: 'h1', name: 'Hannah', number: '1' },
                    { id: 'h2', name: 'Hope', number: '3' },
                    { id: 'h3', name: 'Heather', number: '5' },
                    { id: 'h4', name: 'Holly', number: '7' },
                    { id: 'h5', name: 'Hayley', number: '9' },
                    { id: 'h6', name: 'Harper', number: '11' },
                    { id: 'h7', name: 'Hazel', number: '13' },
                    { id: 'h8', name: 'Harmony', number: '15' },
                    { id: 'h9', name: 'Haven', number: '17' },
                ],
            },
        },
    };
    let state = computeStateFromLog([startAction]);
    state.activeTeam = 'away';
    state.activeCtx = { b: 0, i: 1, col: 'col-1-0' };
    state.activeBatterId = 'a1';
    return state;
}

function getNextBatterIndexForTeam(state, team) {
    const roster = state.roster && state.roster[team];
    const rosterLen = roster ? roster.length : 9;
    if (!state.events) {
        return 0;
    }

    const teamKeys = Object.keys(state.events).filter(k => k.startsWith(`${team}-`));
    if (teamKeys.length === 0) {
        return 0;
    }

    teamKeys.sort((a, b) => {
        const partsA = a.split('-');
        const partsB = b.split('-');

        const bIdxA = parseInt(partsA[1]);
        const bIdxB = parseInt(partsB[1]);

        const colA = partsA.slice(2).join('-');
        const colB = partsB.slice(2).join('-');

        const colPartsA = colA.split('-');
        const colPartsB = colB.split('-');
        const innA = parseInt(colPartsA[1] || '0');
        const innB = parseInt(colPartsB[1] || '0');
        const suffA = parseInt(colPartsA[2] || '0');
        const suffB = parseInt(colPartsB[2] || '0');

        if (innA !== innB) {
            return innA - innB;
        }
        if (suffA !== suffB) {
            return suffA - suffB;
        }
        return bIdxA - bIdxB;
    });

    const lastKey = teamKeys[teamKeys.length - 1];
    const lastParts = lastKey.split('-');
    const lastBIdx = parseInt(lastParts[1]);

    const event = state.events[lastKey];
    const isDone = event && (
        event.outcome ||
        event.bipState ||
        (event.paths && event.paths[0] > 0) ||
        event.outNum > 0
    );

    return isDone ? (lastBIdx + 1) % rosterLen : lastBIdx;
}

// Simulates app-level context updates following a plate appearance completion
function updateActiveContext(state) {
    if (!state.activeCtx || !state.activeTeam) {
        state.activeCtx = { b: 0, i: 1, col: 'col-1-0' };
        state.activeTeam = 'away';
    }

    // Ensure all columns exist for up to the current inning
    if (!state.columns) {
        state.columns = [];
    }

    let iterations = 0;
    const maxIterations = 50; // Safety cap

    while (iterations < maxIterations) {
        const team = state.activeTeam;
        const inning = state.activeCtx.i;
        const colId = state.activeCtx.col;
        const bIdx = state.activeCtx.b;
        const key = `${team}-${bIdx}-${colId}`;

        const maxInning = Math.max(7, inning);
        for (let i = 1; i <= maxInning; i++) {
            if (!state.columns.some(c => c.inning === i && c.id === `col-${i}-0`)) {
                state.columns.push({ inning: i, id: `col-${i}-0` });
            }
        }

        // Check if the current batter finished their turn
        const event = state.events && state.events[key];
        const isBatterDone = event && (
            event.outcome ||
            event.bipState ||
            (event.paths && event.paths[0] > 0) ||
            event.outNum > 0
        );

        if (!isBatterDone) {
            break;
        }

        // Count outs in the current inning to see if we transition to next half inning
        const inningCols = state.columns.filter(c => c.inning === inning).map(c => c.id);
        let maxOutNum = 0;
        if (state.events) {
            Object.keys(state.events).forEach(k => {
                const parts = k.split('-');
                if (parts[0] === team) {
                    const col = parts.slice(2).join('-');
                    if (inningCols.includes(col)) {
                        const ev = state.events[k];
                        if (ev.outNum) {
                            maxOutNum = Math.max(maxOutNum, ev.outNum);
                        }
                    }
                }
            });
        }

        if (maxOutNum >= 3) {
            // Half inning is over! Switch team.
            if (team === 'away') {
                state.activeTeam = 'home';
                const nextHomeB = getNextBatterIndexForTeam(state, 'home');
                state.activeCtx = { b: nextHomeB, i: inning, col: `col-${inning}-0` };
            } else {
                state.activeTeam = 'away';
                const nextInning = inning + 1;
                if (!state.columns.some(c => c.inning === nextInning)) {
                    state.columns.push({ inning: nextInning, id: `col-${nextInning}-0` });
                }
                const nextAwayB = getNextBatterIndexForTeam(state, 'away');
                state.activeCtx = { b: nextAwayB, i: nextInning, col: `col-${nextInning}-0` };
            }
        } else {
            // Inning not over, advance to next batter in lineup
            const roster = state.roster && state.roster[team];
            const rosterLen = roster ? roster.length : 9;
            const nextB = (bIdx + 1) % rosterLen;
            let nextCol = colId;
            if (nextB === 0) {
                // Batted around!
                const parts = colId.split('-');
                const suffix = parseInt(parts[2] || '0', 10) + 1;
                nextCol = `col-${inning}-${suffix}`;
                if (!state.columns.some(c => c.id === nextCol)) {
                    state.columns.push({ inning, id: nextCol });
                }
            }
            state.activeCtx = { b: nextB, i: inning, col: nextCol };
        }
        iterations++;
    }

    // Set activeBatterId
    const activeRoster = state.roster && state.roster[state.activeTeam];
    if (activeRoster && activeRoster[state.activeCtx.b]) {
        state.activeBatterId = activeRoster[state.activeCtx.b].current.id;
    }
}

// Applies a spoken speech command to the simulated game state
function applyCommand(state, text) {
    const cleaned = cleanTranscript(text, state);
    const actions = parseEvent(cleaned, state);

    let nextState = { ...state };
    if (!nextState.actionLog) {
        nextState.actionLog = [];
    }

    const hasUndo = actions.some(a => a.type === ActionTypes.UNDO);
    if (hasUndo) {
        const log = [...nextState.actionLog];
        for (const action of actions) {
            const actionData = {
                id: 'act-' + Math.random().toString(36).substr(2, 9),
                timestamp: Date.now(),
                ...action,
            };
            log.push(actionData);
        }
        nextState = computeStateFromLog(log);
    } else {
        const log = [...nextState.actionLog];
        for (const action of actions) {
            const actionData = {
                id: 'act-' + Math.random().toString(36).substr(2, 9),
                timestamp: Date.now(),
                ...action,
            };
            log.push(actionData);
            nextState = gameReducer(nextState, actionData);
        }
        nextState.actionLog = log;
    }

    updateActiveContext(nextState);
    return nextState;
}

describe('Softball Game Voice Scoring: End-to-End Scenarios', () => {

    describe('1. Pitch Counts & Sequential Calls', () => {
        test('should process various spoken pitches correctly', () => {
            let state = createNewGame();

            // Batter 1: Alice (a1)
            state = applyCommand(state, 'ball');
            state = applyCommand(state, 'that was a ball');
            state = applyCommand(state, 'strike 1');
            state = applyCommand(state, 'foul ball');

            const event = state.events['away-0-col-1-0'];
            expect(event.balls).toBe(2);
            expect(event.strikes).toBe(2);
            expect(event.pitchSequence.length).toBe(4);
            expect(state.activeBatterId).toBe('a1'); // Still active
        });
    });

    describe('2. Standard Strikeouts', () => {
        test('should process called and swinging strikeouts', () => {
            let state = createNewGame();

            // Batter 1: Alice strikes out swinging
            state = applyCommand(state, 'strike');
            state = applyCommand(state, 'strike');
            state = applyCommand(state, 'strikeout'); // swinging strikeout

            expect(state.events['away-0-col-1-0'].outcome).toBe('K');
            expect(state.events['away-0-col-1-0'].outNum).toBe(1);
            expect(state.activeCtx.b).toBe(1); // advanced to Becky (a2)

            // Batter 2: Becky strikes out looking
            state = applyCommand(state, 'strike');
            state = applyCommand(state, 'strike');
            state = applyCommand(state, 'strikeout looking');

            expect(state.events['away-1-col-1-0'].outcome).toBe('ꓘ');
            expect(state.events['away-1-col-1-0'].outNum).toBe(2);
            expect(state.activeCtx.b).toBe(2); // advanced to Charlie (a3)
        });
    });

    describe('3. Walks (Base on Balls)', () => {
        test('should process walk command and advance batter to first', () => {
            let state = createNewGame();

            // Batter 1: Alice walks
            state = applyCommand(state, 'walk');
            expect(state.events['away-0-col-1-0'].paths[0]).toBe(1); // Batter on 1B
            expect(state.activeCtx.b).toBe(1); // advanced to Becky

            // Batter 2: Becky walks via "base on balls"
            state = applyCommand(state, 'base on balls');
            expect(state.events['away-1-col-1-0'].paths[0]).toBe(1); // Batter on 1B
            expect(state.events['away-0-col-1-0'].paths[1]).toBe(1); // Alice forced to 2B
            expect(state.activeCtx.b).toBe(2); // advanced to Charlie
        });
    });

    describe('4. Hit By Pitch (HBP)', () => {
        test('should process hit by pitch / HBP', () => {
            let state = createNewGame();
            state = applyCommand(state, 'hit by pitch');
            expect(state.events['away-0-col-1-0'].bipState.type).toBe('HBP');
            expect(state.events['away-0-col-1-0'].paths[0]).toBe(1);
            expect(state.activeCtx.b).toBe(1);

            state = applyCommand(state, 'hbp');
            expect(state.events['away-1-col-1-0'].bipState.type).toBe('HBP');
            expect(state.activeCtx.b).toBe(2);
        });
    });

    describe('5. Clean Base Hits (BIP)', () => {
        test('should process singles, doubles, triples, homeruns with fields', () => {
            let state = createNewGame();

            // Batter 1: Alice hits a single to left
            state = applyCommand(state, 'single to left field');
            expect(state.events['away-0-col-1-0'].bipState.base).toBe('1B');
            expect(state.events['away-0-col-1-0'].bipState.type).toBe('1B');
            expect(state.events['away-0-col-1-0'].bipState.pos).toBe('7');
            expect(state.events['away-0-col-1-0'].paths[0]).toBe(1);
            expect(state.activeCtx.b).toBe(1);

            // Batter 2: Becky hits a double to center field
            // Since runner on 1B and batter doubles, this is physically ambiguous!
            // Let's assert state: "double, runners on second and third"
            state = applyCommand(state, 'double to center field, runners on second and third');
            expect(state.events['away-1-col-1-0'].bipState.base).toBe('2B');
            expect(state.events['away-1-col-1-0'].bipState.pos).toBe('8');
            expect(state.events['away-1-col-1-0'].paths[1]).toBe(1); // batter on 2B
            expect(state.events['away-0-col-1-0'].paths[2]).toBe(1); // runner on 3B
            expect(state.activeCtx.b).toBe(2);
        });

        test('should process triples and home runs with scoring', () => {
            let state = createNewGame();
            state = applyCommand(state, 'triple to right field');
            expect(state.events['away-0-col-1-0'].paths[2]).toBe(1); // 3B

            // Now Charlie hits a home run
            state = applyCommand(state, 'homerun to center field');
            expect(state.events['away-1-col-1-0'].paths[3]).toBe(1); // batter scored
            expect(state.events['away-0-col-1-0'].paths[3]).toBe(1); // runner scored
            expect(state.score.away).toBe(2);
        });
    });

    describe('6. Ground Outs', () => {
        test('should process ground outs to specific fielders', () => {
            let state = createNewGame();

            state = applyCommand(state, 'ground out to shortstop');
            expect(state.events['away-0-col-1-0'].outNum).toBe(1);
            expect(state.events['away-0-col-1-0'].paths[0]).toBe(2); // Out at 1B

            state = applyCommand(state, 'ground out 5-3');
            expect(state.events['away-1-col-1-0'].outNum).toBe(2);
            expect(state.events['away-1-col-1-0'].paths[0]).toBe(2);
        });
    });

    describe('7. Fly Outs', () => {
        test('should process fly outs to left/center/right fielders', () => {
            let state = createNewGame();

            state = applyCommand(state, 'fly out to left field');
            expect(state.events['away-0-col-1-0'].outNum).toBe(1);
            expect(state.events['away-0-col-1-0'].paths[0]).toBe(2);

            state = applyCommand(state, 'f8'); // Fuzzied to fly out to 8
            expect(state.events['away-1-col-1-0'].outNum).toBe(2);
        });
    });

    describe('8. Line Outs & Pop Outs', () => {
        test('should process line outs and pop outs to catcher/infielders', () => {
            let state = createNewGame();

            state = applyCommand(state, 'line out to shortstop');
            expect(state.events['away-0-col-1-0'].outNum).toBe(1);
            expect(state.events['away-0-col-1-0'].paths[0]).toBe(2);

            state = applyCommand(state, 'pop out to catcher');
            expect(state.events['away-1-col-1-0'].outNum).toBe(2);
            expect(state.events['away-1-col-1-0'].paths[0]).toBe(2);
        });
    });

    describe('9. Errors (Batter Safe)', () => {
        test('should process reached/safe on error', () => {
            let state = createNewGame();

            state = applyCommand(state, 'reached on error to shortstop');
            expect(state.events['away-0-col-1-0'].bipState.type).toBe('ERR');
            expect(state.events['away-0-col-1-0'].bipState.seq).toBe('6');
            expect(state.events['away-0-col-1-0'].paths[0]).toBe(1);

            state = applyCommand(state, 'safe on error to third');
            expect(state.events['away-1-col-1-0'].bipState.type).toBe('ERR');
            expect(state.events['away-1-col-1-0'].bipState.seq).toBe('5');
            expect(state.events['away-1-col-1-0'].paths[0]).toBe(1);
        });
    });

    describe('10. Fielder\'s Choice', () => {
        test('should process fielder\'s choice safe play', () => {
            let state = createNewGame();
            state = applyCommand(state, 'reached on fielder\'s choice to shortstop');
            expect(state.events['away-0-col-1-0'].bipState.type).toBe('FC');
            expect(state.events['away-0-col-1-0'].bipState.seq).toBe('6');
            expect(state.events['away-0-col-1-0'].paths[0]).toBe(1);
        });
    });

    describe('11. Runner Steals', () => {
        test('should process stolen bases on specific bases', () => {
            let state = createNewGame();
            state = applyCommand(state, 'single');

            // Alice is on 1B. She steals 2B.
            state = applyCommand(state, 'runner steals second');
            expect(state.events['away-0-col-1-0'].paths[1]).toBe(1); // Safe on 2B
            expect(state.events['away-0-col-1-0'].pathInfo[1]).toBe('SB');

            // Now Becky walks, and Alice steals 3B
            state = applyCommand(state, 'walk');
            state = applyCommand(state, 'runner steals third');
            expect(state.events['away-0-col-1-0'].paths[2]).toBe(1); // Safe on 3B
            expect(state.events['away-0-col-1-0'].pathInfo[2]).toBe('SB');
        });
    });

    describe('12. Caught Stealing', () => {
        test('should process caught stealing and increment out count', () => {
            let state = createNewGame();
            state = applyCommand(state, 'single');

            // Runner on 1B caught stealing second
            state = applyCommand(state, 'runner caught stealing second');
            expect(state.events['away-0-col-1-0'].paths[1]).toBe(2); // Out at 2B
            expect(state.events['away-0-col-1-0'].pathInfo[1]).toBe('CS');
            expect(state.events['away-0-col-1-0'].outNum).toBe(1); // First out of inning
        });
    });

    describe('13. Wild Pitch & Passed Ball', () => {
        test('should advance all active runners on base', () => {
            let state = createNewGame();
            state = applyCommand(state, 'single');
            state = applyCommand(state, 'walk'); // Alice on 2B, Becky on 1B

            state = applyCommand(state, 'wild pitch');
            expect(state.events['away-0-col-1-0'].paths[2]).toBe(1); // Alice to 3B
            expect(state.events['away-1-col-1-0'].paths[1]).toBe(1); // Becky to 2B

            state = applyCommand(state, 'passed ball');
            expect(state.events['away-0-col-1-0'].paths[3]).toBe(1); // Alice scores
            expect(state.events['away-1-col-1-0'].paths[2]).toBe(1); // Becky to 3B
            expect(state.score.away).toBe(1);
        });
    });

    describe('14. Substitutions (Pinch Runner)', () => {
        test('should replace a player in the roster and track history', () => {
            let state = createNewGame();

            // Becky is slot index 1. We replace her with player number 2 (Becky's jersey is 2, wait, let's use jersey 30).
            // Roster has Charlie jersey 14, Becky jersey 2. Let's sub pinch runner jersey 30 for 2.
            state = applyCommand(state, 'pinch runner 30 for 2');

            const slot = state.roster.away[1];
            expect(slot.current.number).toBe('30');
            expect(slot.current.id).toBeDefined();
            expect(slot.history.length).toBe(1);
            expect(slot.history[0].number).toBe('2');
        });
    });

    describe('15. Pitching Changes', () => {
        test('should update active pitcher for team', () => {
            let state = createNewGame();

            state = applyCommand(state, 'pitching change 15');
            expect(state.pitchers.home).toBe('Harmony');
        });
    });

    describe('16. Base Assertions & Auto-Advancement', () => {
        test('should resolve ambiguous single with runner on 1B and 2B to bases loaded', () => {
            let state = createNewGame();
            state = applyCommand(state, 'single');
            state = applyCommand(state, 'walk'); // runners on 1B & 2B

            // Next batter hits a single and we assert "bases loaded"
            state = applyCommand(state, 'single, bases loaded');

            expect(state.events['away-0-col-1-0'].paths[2]).toBe(1); // lead runner to 3B
            expect(state.events['away-1-col-1-0'].paths[1]).toBe(1); // trailing runner to 2B
            expect(state.events['away-2-col-1-0'].paths[0]).toBe(1); // batter to 1B
        });

        test('should resolve single with runner on 2B scoring via "runner scores" assertion', () => {
            let state = createNewGame();
            state = applyCommand(state, 'double'); // runner on 2B

            state = applyCommand(state, 'single, runner scores');
            expect(state.events['away-0-col-1-0'].paths[3]).toBe(1); // lead runner scores
            expect(state.events['away-1-col-1-0'].paths[0]).toBe(1); // batter to 1B
            expect(state.score.away).toBe(1);
        });

        test('should resolve double with runner on 1B to runner scores', () => {
            let state = createNewGame();
            state = applyCommand(state, 'single');
            state = applyCommand(state, 'double, runner scores');

            expect(state.events['away-0-col-1-0'].paths[3]).toBe(1); // runner scores
            expect(state.events['away-1-col-1-0'].paths[1]).toBe(1); // batter double
            expect(state.score.away).toBe(1);
        });
    });

    describe('17. Ambiguity Detection (No Assertion)', () => {
        test('should throw AmbiguityError when single occurs with runner on 2B', () => {
            let state = createNewGame();
            state = applyCommand(state, 'double'); // runner on 2B

            expect(() => applyCommand(state, 'single')).toThrow(AmbiguityError);

            try {
                applyCommand(state, 'single');
            } catch (err) {
                expect(err.options.length).toBe(2);
                expect(err.options[0].text).toContain('advances to 3rd');
                expect(err.options[1].text).toContain('scores');
            }
        });
    });

    describe('18. Pitch Corrections (Timeline Modification)', () => {
        test('should undo previous pitch and apply new pitch type', () => {
            let state = createNewGame();

            state = applyCommand(state, 'strike');
            state = applyCommand(state, 'ball');
            expect(state.events['away-0-col-1-0'].balls).toBe(1);
            expect(state.events['away-0-col-1-0'].strikes).toBe(1);

            // Correct the last pitch to be a strike
            state = applyCommand(state, 'correction the last pitch was a strike');
            expect(state.events['away-0-col-1-0'].balls).toBe(0);
            expect(state.events['away-0-col-1-0'].strikes).toBe(2);
            expect(state.events['away-0-col-1-0'].pitchSequence.length).toBe(2);
        });
    });

    describe('19. Play Corrections (Timeline Modification)', () => {
        test('should rewrite double to single with runners returning to correct base', () => {
            let state = createNewGame();

            state = applyCommand(state, 'single'); // Alice on 1B

            // Becky hits a double, and Alice scores
            state = applyCommand(state, 'double, runner scores');
            expect(state.events['away-0-col-1-0'].paths[3]).toBe(1); // Alice scores
            expect(state.events['away-1-col-1-0'].paths[1]).toBe(1); // Becky double

            // Correction: the previous play was actually a single and Alice advanced to second
            state = applyCommand(state, 'the previous play was actually a single, runner to second');

            expect(state.events['away-0-col-1-0'].paths[3]).toBe(0); // Alice score undone!
            expect(state.events['away-0-col-1-0'].paths[1]).toBe(1); // Alice on 2B
            expect(state.events['away-1-col-1-0'].paths[0]).toBe(1); // Becky is on 1B (single, not double)
            expect(state.score.away).toBe(0); // Score reset to 0
        });

        test('should undo intermediate substitutions and pitcher changes when correcting a play', () => {
            let state = createNewGame();

            state = applyCommand(state, 'single'); // Alice on 1B

            // Becky hits a double, and Alice scores
            state = applyCommand(state, 'double, runner scores');
            expect(state.events['away-0-col-1-0'].paths[3]).toBe(1); // Alice scores

            // Apply a substitution: pinch runner Ivy (number 26) for Alice (number 10)
            state = applyCommand(state, 'pinch runner 26 for 10');
            expect(state.roster.away[0].current.id).toBe('a9'); // Ivy is now in slot 0

            // Apply a pitcher change: pitching change Hope (number 3)
            state = applyCommand(state, 'pitching change 3');
            expect(state.pitchers.home).toBe('Hope');

            // Correction: the previous play was actually a single and Alice advanced to second
            state = applyCommand(state, 'the previous play was actually a single, runner to second');

            // Verify the play result and advancements are corrected
            expect(state.events['away-0-col-1-0'].paths[3]).toBe(0); // Alice score undone!
            expect(state.events['away-0-col-1-0'].paths[1]).toBe(1); // Alice on 2B
            expect(state.events['away-1-col-1-0'].paths[0]).toBe(1); // Becky is on 1B (single, not double)

            // Verify the intermediate substitution and pitcher change were also undone
            expect(state.roster.away[0].current.id).toBe('a1'); // Restored to Alice
            expect(state.pitchers.home).toBe(''); // Restored to no pitcher change (or default)
        });
    });

    describe('20. Play Correction with Error (Advanced Non-Linear)', () => {
        test('should parse corrected play with advance to base on error', () => {
            let state = createNewGame();
            state = applyCommand(state, 'single'); // Alice on 1B
            state = applyCommand(state, 'double, runner scores'); // Becky double, Alice scores

            // Scorer corrects: "the previous play was actually a single with advance to second base on error by shortstop"
            // Cleaned transcript should strip "on error by shortstop" and keep "single, runner to second"
            state = applyCommand(state, 'the previous play was actually a single with advance to second base on error by shortstop');

            expect(state.events['away-0-col-1-0'].paths[1]).toBe(1); // Alice on 2B
            expect(state.events['away-1-col-1-0'].paths[0]).toBe(1); // Becky on 1B (single)
            expect(state.score.away).toBe(0); // Score reset
        });
    });

    describe('21. Multi-pitch parsing in single command', () => {
        test('should process multiple pitches in one speech event', () => {
            let state = createNewGame();
            state = applyCommand(state, 'strike, ball, foul, strikeout');

            expect(state.events['away-0-col-1-0'].outcome).toBe('K');
            expect(state.activeCtx.b).toBe(1); // Next batter
        });
    });

    describe('22. Sequential Play separators', () => {
        test('should support then / and as separators', () => {
            let state = createNewGame();
            state = applyCommand(state, 'single and then runner to second');

            expect(state.events['away-0-col-1-0'].paths[0]).toBe(1); // batter to 1B
            expect(state.events['away-0-col-1-0'].paths[1]).toBe(1); // batter advanced to 2B
        });
    });

    describe('23. Spoken Number Normalization', () => {
        test('should normalize spoken numbers to jersey digits', () => {
            let state = createNewGame();

            // pinch runner five for too (pinch runner 5 for 2)
            state = applyCommand(state, 'pinch runner five for too');
            const slot = state.roster.away[1]; // Becky is index 1, jersey 2
            expect(slot.current.number).toBe('5');
            expect(slot.history[0].number).toBe('2');
        });
    });

    describe('24. Roster Wrap-around and Inning End', () => {
        test('should cycle roster and switch teams when 3 outs occur', () => {
            let state = createNewGame();

            // 3 quick groundouts to end the top of the 1st
            state = applyCommand(state, 'ground out'); // 1 out
            state = applyCommand(state, 'ground out'); // 2 outs
            state = applyCommand(state, 'ground out'); // 3 outs

            expect(state.activeTeam).toBe('home'); // Switched to home team batting
            expect(state.activeCtx.b).toBe(0); // Lead off home batter index 0 (Hannah)
            expect(state.activeCtx.i).toBe(1); // Still 1st inning (bottom half)

            // 3 quick fly outs to end bottom of the 1st
            state = applyCommand(state, 'fly out'); // 1 out
            state = applyCommand(state, 'fly out'); // 2 outs
            state = applyCommand(state, 'fly out'); // 3 outs

            expect(state.activeTeam).toBe('away'); // Switched back to away team batting
            expect(state.activeCtx.b).toBe(3); // Roster index 3 (Deb), who is 4th batter
            expect(state.activeCtx.i).toBe(2); // Top of the 2nd inning
        });
    });

    describe('25. Complex Half-Inning Simulation', () => {
        test('should score a complete inning with hits, runs, outs, and verify correct state', () => {
            let state = createNewGame();

            // Batter 1 (Alice): Single
            state = applyCommand(state, 'single');

            // Batter 2 (Becky): Double, Alice to third
            state = applyCommand(state, 'double, runner to third');
            expect(state.events['away-0-col-1-0'].paths[2]).toBe(1); // Alice on 3B
            expect(state.events['away-1-col-1-0'].paths[1]).toBe(1); // Becky on 2B

            // Batter 3 (Charlie): Walk -> Bases Loaded (Alice on 3B, Becky on 2B, Charlie on 1B)
            state = applyCommand(state, 'walk');
            expect(state.events['away-0-col-1-0'].paths[2]).toBe(1); // Alice on 3B
            expect(state.events['away-1-col-1-0'].paths[1]).toBe(1); // Becky on 2B
            expect(state.events['away-2-col-1-0'].paths[0]).toBe(1); // Charlie on 1B

            // Batter 4 (Deb): Strikeout swinging (1 out)
            state = applyCommand(state, 'strike, strike, strikeout');
            expect(state.events['away-3-col-1-0'].outcome).toBe('K');
            expect(state.events['away-3-col-1-0'].outNum).toBe(1);

            // Batter 5 (Elsa): Single, Alice scores, Becky scores, Charlie to second
            // State assertion: "single, runner scores, runner scores, runner to second"
            // Wait, we can just say "single, Charlie to second" and since it's Charlie,
            // the lead runners (Alice and Becky) must have scored!
            // Let's assert "single, Charlie to second" -> wait, Charlie is on 1B, she goes to 2B.
            // Under baseball rules, if Charlie goes to 2B, the runners ahead of her (Alice and Becky) must score or advance.
            // Let's explicitly assert: "single, Charlie to second, Becky scores, Alice scores"
            state = applyCommand(state, 'single, Charlie to second, Becky scores, Alice scores');

            expect(state.events['away-0-col-1-0'].paths[3]).toBe(1); // Alice scores
            expect(state.events['away-1-col-1-0'].paths[3]).toBe(1); // Becky scores
            expect(state.events['away-2-col-1-0'].paths[1]).toBe(1); // Charlie on 2B
            expect(state.events['away-4-col-1-0'].paths[0]).toBe(1); // Elsa on 1B
            expect(state.score.away).toBe(2);

            // Batter 6 (Fiona): Ground out to shortstop. Elsa out at second (fielder's choice out), Fiona safe at first (2 outs)
            // Wait, let's say: "fielder's choice to shortstop, Elsa out at second"
            // Let's check how fielder's choice out on runner is scored.
            // In nearley, we support "fielder's choice". Let's run a simple ground out and runner out:
            // "ground out, runner caught stealing" - wait, we can just record "ground out" to retire the batter,
            // and advance/out runners.
            // Let's say: "ground out, runner out at second" -> wait, "out at second" is not in the grammar directly as "runner out at second",
            // but we can do a ground out (1 out) and then a caught stealing or just a normal out.
            // Let's keep it simple: Fiona hits a fly out to center (2 outs).
            state = applyCommand(state, 'fly out to center field');
            expect(state.events['away-5-col-1-0'].outNum).toBe(2);

            // Batter 7 (Grace): Triple, Charlie scores, Elsa scores
            state = applyCommand(state, 'triple, Charlie scores, Elsa scores');
            expect(state.events['away-2-col-1-0'].paths[3]).toBe(1); // Charlie scores
            expect(state.events['away-4-col-1-0'].paths[3]).toBe(1); // Elsa scores
            expect(state.events['away-6-col-1-0'].paths[2]).toBe(1); // Grace on 3B
            expect(state.score.away).toBe(4);

            // Batter 8 (Heidi): Ground out to shortstop (3 outs)
            state = applyCommand(state, 'ground out to shortstop');
            expect(state.events['away-7-col-1-0'].outNum).toBe(3);

            // Verify half inning ends correctly
            expect(state.activeTeam).toBe('home'); // transitioned to home batting
            expect(state.activeCtx.i).toBe(1); // Still 1st inning bottom half
            expect(state.score.away).toBe(4); // 4 runs scored
        });
    });
});
