import nearley from '../vendor/nearley.js';
import { grammar } from './grammar.js';

import { gameReducer, ActionTypes } from '../reducer.js';

/**
 * Parses a natural language string into an array of Scorekeeper Action objects.
 * @param {string} text The string to parse (e.g., "ball", "single, runner to second").
 * @param {Object} gameState The current game state.
 * @returns {Array<Object>} Array of Action objects.
 */
export function parseEvent(text, gameState = {}) {
    if (!text) {
        return [];
    }

    const parser = new nearley.Parser(nearley.Grammar.fromCompiled(grammar));

    try {
        const cleanText = text.toLowerCase().trim();
        parser.feed(cleanText);

        if (parser.results.length > 0) {
            const intents = parser.results[0];
            return resolveIntents(intents, gameState);
        } else {
            throw new Error(`Incomplete input: "${text}"`);
        }
    } catch (err) {
        throw new Error(`Syntax error: could not parse "${text}". ${err.message}`);
    }
}

/**
 * Resolves high-level intents into concrete Scorekeeper actions.
 * @param {Array<Object>} intents
 * @param {Object} gameState
 */
function resolveIntents(intents, gameState) {
    const actions = [];
    let currentState = { ...gameState };

    // Ensure state has context for resolution (Phase 6 robustness)
    if (!currentState.activeCtx || !currentState.activeTeam) {
        currentState.activeCtx = { b: 0, i: 1, col: '' };
        currentState.activeTeam = 'away';
        if (currentState.roster && currentState.roster.away && currentState.roster.away[0]) {
            currentState.activeBatterId = currentState.roster.away[0].current.id;
        }
    }

    for (const intent of intents) {
        const resolved = resolveSingleIntent(intent, currentState);
        const resolvedActions = Array.isArray(resolved) ? resolved : [resolved];

        for (const action of resolvedActions) {
            actions.push(action);
            // Update virtual state for the next intent in the sequence
            currentState = gameReducer(currentState, action);
        }
    }

    return actions;
}

function resolveSingleIntent(intent, state) {
    let actionType = intent.type;
    let payload = { ...intent };

    if (intent.type === 'BIP' || intent.type === 'OUT') {
        actionType = ActionTypes.PLAY_RESULT;
        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            batterId: state.activeBatterId,
            bipState: {
                res: intent.type === 'BIP' ? 'Safe' : 'Out',
                base: intent.result === 'strikeout' ? 'Home' : (intent.result || '1B'),
                type: intent.result || '1B',
            },
        };
        // Handle strikeout specifically if it's considered an OUT in grammar
        if (intent.result === 'strikeout') {
            const key = `${state.activeTeam}-${state.activeCtx.b}-${state.activeCtx.col}`;
            const currentStrikes = state.events && state.events[key] ? state.events[key].strikes || 0 : 0;
            const strikesNeeded = Math.max(1, 3 - currentStrikes);

            const strikeActions = [];
            for (let i = 0; i < strikesNeeded; i++) {
                strikeActions.push({
                    type: ActionTypes.PITCH,
                    payload: {
                        activeCtx: state.activeCtx,
                        activeTeam: state.activeTeam,
                        batterId: state.activeBatterId,
                        type: 'strike',
                        code: 'C', // Called strike
                    },
                });
            }
            return strikeActions;
        }
    } else if (intent.type === 'PITCH') {
        actionType = ActionTypes.PITCH;
        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            batterId: state.activeBatterId,
            type: intent.outcome === 'ball' ? 'ball' : 'strike',
            code: intent.outcome === 'foul' ? 'F' : 'C',
        };
    } else if (intent.type === 'RUNNER_ADVANCE') {
        actionType = ActionTypes.RUNNER_ADVANCE;
        const resolved = resolveRunnerAction(intent, state);
        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            runners: [{
                key: resolved.runnerKey,
                id: resolved.runnerId,
                base: resolved.fromBase,
                outcome: resolved.base === 'home' ? 'Score' : `To ${resolved.base}`,
            }],
        };
    } else if (intent.type === 'SUBSTITUTION' || intent.type === 'PITCHER_UPDATE') {
        actionType = intent.type === 'SUBSTITUTION' ? ActionTypes.SUBSTITUTION : ActionTypes.PITCHER_UPDATE;

        let resolvedPlayer = intent.player;
        if (intent.player && !intent.player.id && intent.player.jersey) {
            resolvedPlayer = resolvePlayer(intent.player.jersey, state);
        }

        if (!resolvedPlayer || (!resolvedPlayer.id && !resolvedPlayer.name)) {
            throw new Error(`Could not resolve player reference: ${JSON.stringify(intent.player)}`);
        }

        if (actionType === ActionTypes.SUBSTITUTION) {
            let resolvedReplaced = intent.replaced;
            if (intent.replaced && !intent.replaced.id && intent.replaced.jersey) {
                resolvedReplaced = resolvePlayer(intent.replaced.jersey, state);
            }

            if (!resolvedReplaced || !resolvedReplaced.id) {
                throw new Error(`Could not resolve player to be replaced: ${JSON.stringify(intent.replaced)}`);
            }

            payload = {
                team: state.activeTeam,
                subParams: resolvedPlayer,
            };
            if (resolvedReplaced && resolvedReplaced.id) {
                const roster = state.roster && state.roster[state.activeTeam || 'away'];
                if (roster) {
                    const idx = roster.findIndex(slot => slot.current && slot.current.id === resolvedReplaced.id);
                    if (idx !== -1) {
                        payload.rosterIndex = idx;
                    }
                }
            }
        } else {
            payload = {
                team: state.activeTeam,
                pitcher: resolvedPlayer.name || resolvedPlayer.jersey,
            };
        }
    }

    return { type: actionType, payload };
}

function resolvePlayer(jersey, state) {
    const teams = state.activeTeam ? [state.activeTeam] : ['away', 'home'];
    for (const team of teams) {
        const roster = state.roster && state.roster[team];
        if (!Array.isArray(roster)) {
            continue;
        }

        for (const slot of roster) {
            if (slot.current && parseInt(slot.current.number) === jersey) {
                return { id: slot.current.id, name: slot.current.name, number: slot.current.number };
            }
        }
    }
    return { jersey };
}

function resolveRunnerAction(intent, state) {
    const runnersOnBase = getRunnersOnBase(state);

    // Also consider the current batter as a candidate if they've already reached base (Phase 4 sequential)
    const currentBatterKey = `${state.activeTeam}-${state.activeCtx.b}-${state.activeCtx.col}`;
    const currentBatterEvent = state.events ? state.events[currentBatterKey] : null;
    if (currentBatterEvent && currentBatterEvent.paths) {
        const event = currentBatterEvent;
        let base = 0;
        if (event.paths[3] === 0 && event.paths.indexOf(2) === -1) {
            if (event.paths[2] === 1) {
                base = 3;
            }
            else if (event.paths[1] === 1 && event.paths[2] === 0) {
                base = 2;
            }
            else if (event.paths[0] === 1 && event.paths[1] === 0) {
                base = 1;
            }
        }
        if (base > 0) {
            runnersOnBase.push({ id: event.pId || state.activeBatterId, base, key: currentBatterKey });
        }
    }

    if (runnersOnBase.length === 0) {
        throw new Error('No runners on base to advance.');
    }

    // Convert target base to numeric index for logic
    const baseMap = { '1B': 1, '2B': 2, '3B': 3, 'home': 4 };
    const targetBase = baseMap[intent.base];

    // Find the most logical runner to move to this base
    // We look for the runner closest to the target base who hasn't passed it
    let selectedRunner = null;

    // Sort runners by base descending (3rd, then 2nd, then 1st)
    const sortedRunners = [...runnersOnBase].sort((a, b) => b.base - a.base);

    for (const runner of sortedRunners) {
        if (runner.base < targetBase) {
            selectedRunner = runner;
            break; // Found the lead runner who can advance to this base
        }
    }

    if (selectedRunner) {
        return {
            ...intent,
            runnerId: selectedRunner.id,
            runnerKey: selectedRunner.key,
            fromBase: selectedRunner.base,
        };
    }

    throw new Error(`No runner is in a position to move to ${intent.base}.`);
}

function getRunnersOnBase(state) {
    if (!state.activeTeam || !state.activeCtx || !state.events) {
        return [];
    }

    const currentInning = state.activeCtx.i;
    const currentBatterIndex = state.activeCtx.b;
    const inningCols = state.columns ? state.columns.filter(c => c.inning === currentInning).map(c => c.id) : [state.activeCtx.col];

    const runners = [];

    Object.keys(state.events).forEach(key => {
        const parts = key.split('-');
        if (parts[0] !== state.activeTeam) {
            return;
        }

        // Exclude current batter (to match GEMINI.md mandate)
        const batterIndex = parseInt(parts[1]);
        if (batterIndex === currentBatterIndex) {
            return;
        }

        const colId = parts.slice(2).join('-');
        if (!inningCols.includes(colId)) {
            return;
        }

        const event = state.events[key];

        let base = 0;
        if (event.paths && event.paths[3] === 0 && event.paths.indexOf(2) === -1) {
            // Not scored (paths[3] !== 1) and not out (no 2s in paths)
            if (event.paths[2] === 1) {
                base = 3;
            }
            else if (event.paths[1] === 1 && event.paths[2] === 0) {
                base = 2;
            }
            else if (event.paths[0] === 1 && event.paths[1] === 0) {
                base = 1;
            }
        }

        if (base > 0) {
            runners.push({ id: event.pId, base, key });
        }
    });

    return runners;
}
