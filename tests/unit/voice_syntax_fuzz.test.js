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

import { parseEvent } from '../../frontend/utils/parser.js';
import { cleanTranscript } from '../../frontend/utils/fuzzy.js';

describe('Voice Scoring: Exhaustive Syntax Fuzzing', () => {
    const gameState = {
        activeTeam: 'away',
        activeCtx: { b: 1, i: 1, col: 'col-1-0' },
        activeBatterId: 'a2',
        columns: [{ inning: 1, id: 'col-1-0' }],
        events: {
            'away-0-col-1-0': {
                outcome: '1B',
                paths: [1, 0, 0, 0],
                pId: 'a1',
            },
        },
        roster: {
            away: [
                { current: { id: 'a1', name: 'Jones', number: '5' } },
                { current: { id: 'a2', name: 'Smith-Peterson', number: '11' } },
            ],
            home: [
                { current: { id: 'h1', name: 'O\'Malley', number: '22' } },
            ],
        },
        pitchers: {
            home: 'O\'Malley',
        },
    };

    const positions = [
        'pitcher', 'catcher', 'first baseman', 'second baseman', 'third baseman',
        'shortstop', 'left fielder', 'center fielder', 'right fielder',
        'left field', 'center field', 'right field', 'left', 'center', 'right',
        '1', '2', '3', '4', '5', '6', '7', '8', '9',
    ];

    const hits = ['single', 'double', 'triple', 'homerun'];
    const outs = ['fly out', 'ground out', 'line out', 'pop out'];
    const assertions = [
        'bases loaded', 'bases empty', 'runner on first', 'runner on second', 'runner on third',
        'runners on first and second', 'runners on first and third', 'runners on second and third',
        'runners on first, second', 'runners on first, third', 'runners on second, third',
    ];

    const generatedCommands = [];

    // 1. Hit + Position variations
    for (const hit of hits) {
        for (const pos of positions) {
            generatedCommands.push(`${hit} to ${pos}`);
        }
    }

    // 2. Out + Position variations
    for (const out of outs) {
        for (const pos of positions) {
            generatedCommands.push(`${out} to ${pos}`);
        }
    }

    // 3. Hits + Assertions (132 commands)
    for (const hit of ['single', 'double', 'triple']) {
        for (const pos of ['left', 'center', 'right']) {
            for (const assertion of assertions) {
                generatedCommands.push(`${hit} to ${pos}, ${assertion}`);
            }
        }
    }

    // 4. Standard Pitch Scenarios (11 commands)
    const pitches = [
        'ball 1', 'ball 2', 'ball 3', 'ball 4',
        'strike 1', 'strike 2', 'strike 3',
        'foul ball', 'ball', 'strike', 'foul',
    ];
    generatedCommands.push(...pitches);

    // 5. Strikeouts (3 commands)
    generatedCommands.push('strikeout', 'strikeout looking', 'strikeout swinging');

    // 6. Walks & HBP (5 commands)
    generatedCommands.push('walk', 'base on balls', 'bb', 'hit by pitch', 'hbp');

    // 7. Errors & Fielder's Choice (8 commands)
    generatedCommands.push(
        'safe on error', 'reached on error', 'reached on error to shortstop', 'safe on error to 3',
        'fielder\'s choice', 'reached on fielder\'s choice', 'fc', 'fielder\'s choice to second',
    );

    // 8. Runner actions (6 commands)
    generatedCommands.push(
        'runner steals second', 'stolen base third', 'runner caught stealing second',
        'caught stealing third', 'wild pitch', 'passed ball',
    );

    // 9. Corrections (6 commands)
    generatedCommands.push(
        'actually the last pitch was a ball', 'correction last pitch was a strike', 'actually last pitch was a foul',
        'correction the last play was actually a single', 'actually the previous play was a double to left field',
        'correction the last play was reached on error to shortstop',
    );

    // 10. Substitutions & pitching changes (4 commands)
    generatedCommands.push(
        'pinch runner 5', 'pinch runner 11 for 5', 'pitching change 22', 'pitching change 22 for 5',
    );

    // Total commands generated: ~367
    //
    // Success categories:
    //   - Parsed cleanly (no error)               → grammar success
    //   - AmbiguityError                           → grammar success, semantic disambiguation required
    //   - Semantic error (e.g. "No runners on base") → grammar parsed but game-state constraint violated;
    //     acceptable for a fuzz test with a minimal game state
    //
    // Failure category:
    //   - Syntax error from the grammar (unexpected token, incomplete input, etc.) → test fails
    test(`should successfully parse all ${generatedCommands.length} generated plausible commands without throwing syntax errors`, () => {
        const syntaxErrors = [];
        const semanticErrors = [];
        const ambiguities = [];
        let parseSuccesses = 0;

        for (const cmd of generatedCommands) {
            try {
                const cleaned = cleanTranscript(cmd, gameState);
                parseEvent(cleaned, gameState);
                parseSuccesses++;
            } catch (err) {
                if (err.name === 'AmbiguityError') {
                    // Grammar parsed successfully; needs disambiguation UI
                    ambiguities.push(cmd);
                } else {
                    // Distinguish syntax errors (grammar failure) from semantic errors (game state constraint)
                    const isSyntaxError = err.message.includes('Syntax error') ||
                                          err.message.includes('Unexpected') ||
                                          err.message.includes('expecting') ||
                                          err.message.includes('Incomplete input');
                    if (isSyntaxError) {
                        syntaxErrors.push({ command: cmd, error: err.message });
                    } else {
                        // Semantic error: grammar parsed it, but game state constraint violated.
                        // Acceptable in a fuzz test with minimal game state.
                        semanticErrors.push({ command: cmd, error: err.message });
                    }
                }
            }
        }

        const total = parseSuccesses + ambiguities.length + semanticErrors.length + syntaxErrors.length;

        if (syntaxErrors.length > 0) {
            console.error('SYNTAX FAILURES (grammar could not parse):', syntaxErrors);
        }
        if (semanticErrors.length > 0) {
            console.info(`Semantic errors (game-state constraints, not grammar failures): ${semanticErrors.length}`);
        }

        // Only syntax errors are a true test failure
        expect(syntaxErrors.length).toBe(0);
        expect(total).toBe(generatedCommands.length);
    });
});
