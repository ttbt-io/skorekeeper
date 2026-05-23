import nearley from '../vendor/nearley.js';
import { grammar } from './grammar.js';

import { gameReducer, ActionTypes, computeStateFromLog } from '../reducer.js';
import { buildTimeline } from '../game/timeline.js';

export class AmbiguityError extends Error {
    constructor(message, options) {
        super(message);
        this.name = 'AmbiguityError';
        this.options = options;
    }
}


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

    const cleanText = text.toLowerCase().trim();

    // Check for pitch correction
    if (cleanText.startsWith('__correction_pitch__')) {
        const newPitchType = cleanText.replace('__correction_pitch__', '').trim();
        const log = gameState.actionLog || [];
        const effectiveLog = buildTimeline(log);
        let targetPitchAction = null;

        if (gameState.activeCtx) {
            for (let i = effectiveLog.length - 1; i >= 0; i--) {
                const action = effectiveLog[i];
                if (action.type === ActionTypes.PITCH) {
                    const pCtx = action.payload.activeCtx;
                    const pTeam = action.payload.activeTeam;
                    if (pCtx && pCtx.i === gameState.activeCtx.i && pCtx.b === gameState.activeCtx.b && pCtx.col === gameState.activeCtx.col && pTeam === gameState.activeTeam) {
                        targetPitchAction = action;
                        break;
                    }
                }
            }
        }

        if (!targetPitchAction) {
            for (let i = effectiveLog.length - 1; i >= 0; i--) {
                const action = effectiveLog[i];
                if (action.type === ActionTypes.PITCH) {
                    targetPitchAction = action;
                    break;
                }
            }
        }

        if (!targetPitchAction) {
            throw new Error('No recent pitch found to correct.');
        }

        const targetIdx = log.findIndex(a => a.id === targetPitchAction.id);
        const preLog = log.slice(0, targetIdx);
        const preState = computeStateFromLog(preLog);
        if (targetPitchAction.payload) {
            preState.activeCtx = targetPitchAction.payload.activeCtx;
            preState.activeTeam = targetPitchAction.payload.activeTeam;
            preState.activeBatterId = targetPitchAction.payload.batterId;
        }

        const newActions = parseEvent(newPitchType, preState);

        return [
            { type: ActionTypes.UNDO, payload: { refId: targetPitchAction.id } },
            ...newActions,
        ];
    }

    // Check for play correction
    if (cleanText.startsWith('__correction_play__')) {
        const correctedPlayText = cleanText.replace('__correction_play__', '').trim();
        const log = gameState.actionLog || [];
        const effectiveLog = buildTimeline(log);

        let lastPlayResultAction = null;
        for (let i = effectiveLog.length - 1; i >= 0; i--) {
            const action = effectiveLog[i];
            if (action.type === ActionTypes.PLAY_RESULT) {
                lastPlayResultAction = action;
                break;
            }
        }

        if (!lastPlayResultAction) {
            throw new Error('No recent play found to correct.');
        }

        const idxInEffective = effectiveLog.findIndex(a => a.id === lastPlayResultAction.id);
        const actionsToUndo = [];
        for (let i = idxInEffective; i < effectiveLog.length; i++) {
            const action = effectiveLog[i];
            if (action.type === ActionTypes.PLAY_RESULT ||
                action.type === ActionTypes.RUNNER_ADVANCE ||
                action.type === ActionTypes.RUNNER_BATCH_UPDATE ||
                action.type === ActionTypes.SUBSTITUTION ||
                action.type === ActionTypes.PITCHER_UPDATE) {
                actionsToUndo.push(action);
            }
        }

        const undoActions = [...actionsToUndo].reverse().map(action => ({
            type: ActionTypes.UNDO,
            payload: { refId: action.id },
        }));

        const rawIdx = log.findIndex(a => a.id === lastPlayResultAction.id);
        if (rawIdx === -1) {
            throw new Error('Could not find last play in action log.');
        }
        const preLog = log.slice(0, rawIdx);
        const preState = computeStateFromLog(preLog);
        if (lastPlayResultAction.payload) {
            preState.activeCtx = lastPlayResultAction.payload.activeCtx;
            preState.activeTeam = lastPlayResultAction.payload.activeTeam;
            preState.activeBatterId = lastPlayResultAction.payload.batterId;
        }

        const newActions = parseEvent(correctedPlayText, preState);

        return [
            ...undoActions,
            ...newActions,
        ];
    }

    const parser = new nearley.Parser(nearley.Grammar.fromCompiled(grammar));

    let parseResult;
    try {
        parser.feed(cleanText);
        if (parser.results.length > 0) {
            parseResult = parser.results[0];
        } else {
            throw new Error(`Incomplete input: "${text}"`);
        }
    } catch (err) {
        // Only re-wrap genuine grammar/parse failures (from parser.feed).
        // AmbiguityError and semantic errors thrown by resolveIntents propagate as-is.
        if (err instanceof AmbiguityError || err.name === 'AmbiguityError') {
            throw err;
        }
        throw new Error(`Syntax error: could not parse "${text}". ${err.message}`);
    }

    // resolveIntents may throw AmbiguityError or semantic errors — let them propagate unmodified.
    return resolveIntents(parseResult, gameState);
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

    const assertionIntent = intents.find(i => i.type === 'STATE_ASSERTION');
    const normalIntents = intents.filter(i => i.type !== 'STATE_ASSERTION');

    for (const intent of normalIntents) {
        const resolved = resolveSingleIntent(intent, currentState);
        const resolvedActions = Array.isArray(resolved) ? resolved : [resolved];

        for (const action of resolvedActions) {
            actions.push(action);
            // Update virtual state for the next intent in the sequence
            currentState = gameReducer(currentState, action);
        }
    }

    if (assertionIntent) {
        const extraActions = resolveStateAssertion(assertionIntent.bases, actions, gameState);
        for (const action of extraActions) {
            actions.push(action);
        }
    } else {
        checkAmbiguity(actions, gameState);
    }

    return actions;
}

function resolveSingleIntent(intent, state) {
    let actionType = intent.type;
    let payload = { ...intent };

    if (intent.type === 'BIP' || intent.type === 'OUT') {
        actionType = ActionTypes.PLAY_RESULT;

        // Strikeout: model as a PITCH sequence, not as a PLAY_RESULT.
        // Must be detected early, before the bipState payload is constructed,
        // so that the PLAY_RESULT branch is never partially built then discarded.
        if (intent.type === 'OUT' && (
            intent.result === 'strikeout' ||
            intent.result === 'strikeout looking' ||
            intent.result === 'strikeout swinging'
        )) {
            const key = `${state.activeTeam}-${state.activeCtx.b}-${state.activeCtx.col}`;
            const currentStrikes = state.events && state.events[key] ? state.events[key].strikes || 0 : 0;
            const strikesNeeded = Math.max(1, 3 - currentStrikes);
            const pitchCode = intent.result === 'strikeout looking' ? 'Called' : 'Swinging';
            return Array.from({ length: strikesNeeded }, (_, i) => ({
                type: ActionTypes.PITCH,
                payload: {
                    activeCtx: state.activeCtx,
                    activeTeam: state.activeTeam,
                    batterId: state.activeBatterId,
                    type: 'strike',
                    code: i === strikesNeeded - 1 ? pitchCode : 'Swinging',
                },
            }));
        }

        let runnerAdvancements = [];
        if (intent.result === 'HR') {
            const runners = getRunnersOnBase(state);
            runnerAdvancements = runners.map(r => ({
                key: r.key,
                id: r.id,
                base: r.base - 1, // 0-indexed
                outcome: 'Score',
            }));
        }

        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            batterId: state.activeBatterId,
            bipState: {
                res: intent.type === 'BIP' ? 'Safe' : 'Out',
                base: intent.type === 'OUT' ? '1B' : ((intent.result === 'HR') ? 'Home' : (intent.result || '1B')),
                type: intent.result || '1B',
                pos: intent.pos || '',
                seq: intent.sequence || intent.pos || '',
            },
            runnerAdvancements,
        };
    } else if (intent.type === 'WALK') {
        const key = `${state.activeTeam}-${state.activeCtx.b}-${state.activeCtx.col}`;
        const currentBalls = state.events && state.events[key] ? state.events[key].balls || 0 : 0;
        const ballsNeeded = Math.max(1, 4 - currentBalls);

        const ballActions = [];
        for (let i = 0; i < ballsNeeded; i++) {
            ballActions.push({
                type: ActionTypes.PITCH,
                payload: {
                    activeCtx: state.activeCtx,
                    activeTeam: state.activeTeam,
                    batterId: state.activeBatterId,
                    type: 'ball',
                    code: 'C', // Called ball
                },
            });
        }

        // Generate forced runner advancements
        const forced = getWalkRunnerAdvancements(state);
        forced.forEach(adv => {
            ballActions.push({
                type: ActionTypes.RUNNER_ADVANCE,
                payload: {
                    activeCtx: state.activeCtx,
                    activeTeam: state.activeTeam,
                    runners: [{
                        key: adv.key,
                        id: adv.id,
                        base: adv.base - 1, // 0-indexed
                        outcome: adv.outcome,
                    }],
                },
            });
        });
        return ballActions;
    } else if (intent.type === 'HBP') {
        actionType = ActionTypes.PLAY_RESULT;
        const forced = getWalkRunnerAdvancements(state);
        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            batterId: state.activeBatterId,
            bipState: {
                res: 'Safe',
                base: '1B',
                type: 'HBP',
            },
            runnerAdvancements: forced.map(adv => ({
                key: adv.key,
                id: adv.id,
                base: adv.base - 1, // 0-indexed
                outcome: adv.outcome,
            })),
        };
    } else if (intent.type === 'ERROR') {
        actionType = ActionTypes.PLAY_RESULT;
        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            batterId: state.activeBatterId,
            bipState: {
                res: 'Safe',
                base: '1B',
                type: 'ERR',
                seq: intent.pos || '',
            },
        };
    } else if (intent.type === 'FIELDERS_CHOICE') {
        actionType = ActionTypes.PLAY_RESULT;
        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            batterId: state.activeBatterId,
            bipState: {
                res: 'Safe',
                base: '1B',
                type: 'FC',
                seq: intent.pos || '',
            },
        };
    } else if (intent.type === 'STEAL' || intent.type === 'CAUGHT_STEALING') {
        const runners = getRunnersOnBase(state);
        if (runners.length === 0) {
            throw new Error(`No runners on base to ${intent.type === 'STEAL' ? 'steal' : 'be caught stealing'}.`);
        }
        let targetRunner = null;
        if (intent.base) {
            const baseMap = { '1B': 1, '2B': 2, '3B': 3, 'home': 4 };
            const targetBaseNum = baseMap[intent.base];
            // runners.base is 1-indexed (1=1st, 2=2nd, 3=3rd).
            // A runner stealing 2nd must currently be on 1st (targetBaseNum - 1).
            targetRunner = runners.find(r => r.base === targetBaseNum - 1);
            if (!targetRunner) {
                throw new Error(`No runner on base in position to steal ${intent.base}.`);
            }
        } else {
            if (runners.length === 1) {
                targetRunner = runners[0];
            } else {
                const choices = runners.map(r => {
                    const currentBaseStr = r.base === 1 ? '1st' : r.base === 2 ? '2nd' : '3rd';
                    const nextBaseStr = r.base === 1 ? '2nd' : r.base === 2 ? '3rd' : 'home';
                    const actionCode = intent.type === 'STEAL' ? 'SB' : 'CS';
                    const actionText = intent.type === 'STEAL' ? 'steals' : 'caught stealing';
                    return {
                        text: `Runner on ${currentBaseStr} ${actionText} ${nextBaseStr}`,
                        actions: [{
                            type: ActionTypes.RUNNER_BATCH_UPDATE,
                            payload: {
                                activeCtx: state.activeCtx,
                                activeTeam: state.activeTeam,
                                updates: [{
                                    key: r.key,
                                    base: r.base - 1,
                                    action: actionCode,
                                }],
                            },
                        }],
                    };
                });
                throw new AmbiguityError(`Which runner ${intent.type === 'STEAL' ? 'stole' : 'was caught stealing'}?`, choices);
            }
        }

        const actionCode = intent.type === 'STEAL' ? 'SB' : 'CS';
        actionType = ActionTypes.RUNNER_BATCH_UPDATE;
        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            updates: [{
                key: targetRunner.key,
                base: targetRunner.base - 1,
                action: actionCode,
            }],
        };
    } else if (intent.type === 'WILD_PITCH' || intent.type === 'PASSED_BALL') {
        const runners = getRunnersOnBase(state);
        if (runners.length === 0) {
            throw new Error('No runners on base to advance.');
        }
        actionType = ActionTypes.RUNNER_BATCH_UPDATE;
        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            updates: runners.map(r => ({
                key: r.key,
                base: r.base - 1,
                action: 'Adv',
            })),
        };
    } else if (intent.type === 'PITCH') {
        actionType = ActionTypes.PITCH;
        payload = {
            activeCtx: state.activeCtx,
            activeTeam: state.activeTeam,
            batterId: state.activeBatterId,
            type: intent.outcome === 'ball' ? 'ball' : (intent.outcome === 'foul' ? 'foul' : 'strike'),
            code: intent.outcome === 'foul' ? 'Foul' : 'Called',
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
                base: resolved.fromBase - 1, // 0-indexed
                // Use canonical outcome strings matching the reducer's RunnerActionScore / 'To Xrd' vocabulary.
                outcome: resolved.base === 'home' ? 'Score' : (resolved.base === '3B' ? 'To 3rd' : 'To 2nd'),
            }],
        };
    } else if (intent.type === 'SUBSTITUTION' || intent.type === 'PITCHER_UPDATE') {
        actionType = intent.type === 'SUBSTITUTION' ? ActionTypes.SUBSTITUTION : ActionTypes.PITCHER_UPDATE;

        const targetTeam = actionType === ActionTypes.PITCHER_UPDATE
            ? (state.activeTeam === 'away' ? 'home' : 'away')
            : state.activeTeam;

        let resolvedPlayer = intent.player;
        if (intent.player && !intent.player.id && intent.player.jersey) {
            resolvedPlayer = resolvePlayer(intent.player.jersey, state, targetTeam);
        }

        if (actionType === ActionTypes.SUBSTITUTION) {
            if (resolvedPlayer && !resolvedPlayer.id && resolvedPlayer.jersey) {
                resolvedPlayer = {
                    id: `new-${resolvedPlayer.jersey}-${Math.random().toString(36).substr(2, 9)}`,
                    name: `Player #${resolvedPlayer.jersey}`,
                    number: String(resolvedPlayer.jersey),
                };
            }
            if (!resolvedPlayer || !resolvedPlayer.id || !resolvedPlayer.name) {
                throw new Error(`Could not resolve player reference: ${JSON.stringify(intent.player)}`);
            }

            let resolvedReplaced = intent.replaced;
            if (intent.replaced && !intent.replaced.id && intent.replaced.jersey) {
                resolvedReplaced = resolvePlayer(intent.replaced.jersey, state, targetTeam);
            }

            // Only throw if a replaced player was explicitly provided but couldn't be resolved.
            // intent.replaced is null for "pinch runner <n>" (no explicit replaced player), which is valid.
            if (intent.replaced && (!resolvedReplaced || !resolvedReplaced.id)) {
                throw new Error(`Could not resolve player to be replaced: ${JSON.stringify(intent.replaced)}`);
            }

            payload = {
                team: targetTeam,
                subParams: resolvedPlayer,
            };
            if (resolvedReplaced && resolvedReplaced.id) {
                const roster = state.roster && state.roster[targetTeam || 'away'];
                if (roster) {
                    const idx = roster.findIndex(slot => slot.current && slot.current.id === resolvedReplaced.id);
                    if (idx !== -1) {
                        payload.rosterIndex = idx;
                    }
                }
            }
        } else {
            if (!resolvedPlayer || (!resolvedPlayer.id && !resolvedPlayer.name && !resolvedPlayer.jersey)) {
                throw new Error(`Could not resolve player reference: ${JSON.stringify(intent.player)}`);
            }
            payload = {
                team: targetTeam,
                pitcher: resolvedPlayer.name || String(resolvedPlayer.jersey),
            };
        }
    }

    return { type: actionType, payload };
}

function resolveStateAssertion(assertedBases, actions, gameState) {
    let currentState = { ...gameState };
    for (const action of actions) {
        currentState = gameReducer(currentState, action);
    }

    const runners = getRunnersOnBase(currentState);
    const currentBatterKey = `${currentState.activeTeam}-${currentState.activeCtx.b}-${currentState.activeCtx.col}`;
    const currentBatterEvent = currentState.events ? currentState.events[currentBatterKey] : null;
    let batterBase = 0;
    if (currentBatterEvent && currentBatterEvent.paths && currentBatterEvent.paths[3] === 0 && currentBatterEvent.paths.indexOf(2) === -1) {
        if (currentBatterEvent.paths[2] === 1) {
            batterBase = 3;
        } else if (currentBatterEvent.paths[1] === 1 && currentBatterEvent.paths[2] === 0) {
            batterBase = 2;
        } else if (currentBatterEvent.paths[0] === 1 && currentBatterEvent.paths[1] === 0) {
            batterBase = 1;
        }
    }

    const allRunners = [];
    if (batterBase > 0) {
        allRunners.push({
            id: currentState.activeBatterId,
            base: batterBase,
            key: currentBatterKey,
            isBatter: true,
        });
    }
    runners.forEach(r => {
        allRunners.push({
            id: r.id,
            base: r.base,
            key: r.key,
            isBatter: false,
        });
    });

    // Sort runners by base descending (lead runner first), but always put the batter last
    allRunners.sort((a, b) => {
        if (a.isBatter && !b.isBatter) {
            return 1;
        }
        if (!a.isBatter && b.isBatter) {
            return -1;
        }
        if (b.base !== a.base) {
            return b.base - a.base;
        }
        const idxA = parseInt(a.key.split('-')[1]);
        const idxB = parseInt(b.key.split('-')[1]);
        return idxA - idxB;
    });

    const sortedAsserted = [...assertedBases].sort((a, b) => b - a);
    const extraActions = [];

    for (const runner of allRunners) {
        const idx = sortedAsserted.findIndex(b => b >= runner.base);
        if (idx !== -1) {
            const targetBase = sortedAsserted[idx];
            sortedAsserted.splice(idx, 1);
            if (targetBase > runner.base) {
                extraActions.push({
                    type: ActionTypes.RUNNER_ADVANCE,
                    payload: {
                        activeCtx: currentState.activeCtx,
                        activeTeam: currentState.activeTeam,
                        runners: [{
                            key: runner.key,
                            id: runner.id,
                            base: runner.base - 1, // 0-indexed
                            outcome: targetBase === 4 ? 'Score' : (targetBase === 3 ? 'To 3rd' : 'To 2nd'),
                        }],
                    },
                });
            }
        } else {
            // Runner must have scored
            extraActions.push({
                type: ActionTypes.RUNNER_ADVANCE,
                payload: {
                    activeCtx: currentState.activeCtx,
                    activeTeam: currentState.activeTeam,
                    runners: [{
                        key: runner.key,
                        id: runner.id,
                        base: runner.base - 1, // 0-indexed
                        outcome: 'Score',
                    }],
                },
            });
        }
    }

    return extraActions;
}

function checkAmbiguity(actions, gameState) {
    const hasRunnerAdvance = actions.some(a => a.type === ActionTypes.RUNNER_ADVANCE || a.type === ActionTypes.RUNNER_BATCH_UPDATE);
    if (hasRunnerAdvance) {
        return; // Already has runner actions explicitly dictated
    }

    const playResultAction = actions.find(a => a.type === ActionTypes.PLAY_RESULT);
    if (!playResultAction || !playResultAction.payload.bipState) {
        return; // No BIP/play result
    }

    const { res, base } = playResultAction.payload.bipState;
    if (res !== 'Safe') {
        return; // Out, so no standard safe play ambiguity
    }

    const batterBase = base === '1B' ? 1 : base === '2B' ? 2 : base === '3B' ? 3 : 0;
    if (batterBase === 0) {
        return;
    }

    const runners = getRunnersOnBase(gameState);
    if (runners.length === 0) {
        return;
    }

    const has1B = runners.some(r => r.base === 1);
    const has2B = runners.some(r => r.base === 2);

    const sortedRunners = [...runners].sort((a, b) => b.base - a.base);
    let leadUnforced = null;

    for (const r of sortedRunners) {
        let isForced = false;
        if (r.base === 3) {
            isForced = has2B && has1B && batterBase === 1;
        } else if (r.base === 2) {
            isForced = has1B && batterBase === 1;
        } else if (r.base === 1) {
            isForced = batterBase === 1;
        }
        if (!isForced) {
            leadUnforced = r;
            break;
        }
    }

    if (leadUnforced) {
        const choices = [];
        if (leadUnforced.base === 3) {
            choices.push({
                text: 'Runner on 3rd stays on 3rd',
                actions: [...actions],
            });
            choices.push({
                text: 'Runner on 3rd scores',
                actions: [
                    ...actions,
                    {
                        type: ActionTypes.RUNNER_ADVANCE,
                        payload: {
                            activeCtx: gameState.activeCtx,
                            activeTeam: gameState.activeTeam,
                            runners: [{
                                key: leadUnforced.key,
                                id: leadUnforced.id,
                                base: 3,
                                outcome: 'Score',
                            }],
                        },
                    },
                ],
            });
        } else if (leadUnforced.base === 2) {
            const forceActions = [];
            if (has1B) {
                const runner1B = runners.find(r => r.base === 1);
                if (runner1B) {
                    forceActions.push({
                        type: ActionTypes.RUNNER_ADVANCE,
                        payload: {
                            activeCtx: gameState.activeCtx,
                            activeTeam: gameState.activeTeam,
                            runners: [{
                                key: runner1B.key,
                                id: runner1B.id,
                                base: 1,
                                outcome: 'To 2nd',
                            }],
                        },
                    });
                }
            }

            choices.push({
                text: 'Runner on 2nd advances to 3rd',
                actions: [
                    ...actions,
                    ...forceActions,
                    {
                        type: ActionTypes.RUNNER_ADVANCE,
                        payload: {
                            activeCtx: gameState.activeCtx,
                            activeTeam: gameState.activeTeam,
                            runners: [{
                                key: leadUnforced.key,
                                id: leadUnforced.id,
                                base: 2,
                                outcome: 'To 3rd',
                            }],
                        },
                    },
                ],
            });
            choices.push({
                text: 'Runner on 2nd scores',
                actions: [
                    ...actions,
                    ...forceActions,
                    {
                        type: ActionTypes.RUNNER_ADVANCE,
                        payload: {
                            activeCtx: gameState.activeCtx,
                            activeTeam: gameState.activeTeam,
                            runners: [{
                                key: leadUnforced.key,
                                id: leadUnforced.id,
                                base: 2,
                                outcome: 'Score',
                            }],
                        },
                    },
                ],
            });
        } else if (leadUnforced.base === 1) {
            choices.push({
                text: 'Runner on 1st advances to 3rd',
                actions: [
                    ...actions,
                    {
                        type: ActionTypes.RUNNER_ADVANCE,
                        payload: {
                            activeCtx: gameState.activeCtx,
                            activeTeam: gameState.activeTeam,
                            runners: [{
                                key: leadUnforced.key,
                                id: leadUnforced.id,
                                base: 1,
                                outcome: 'To 3rd',
                            }],
                        },
                    },
                ],
            });
            choices.push({
                text: 'Runner on 1st scores',
                actions: [
                    ...actions,
                    {
                        type: ActionTypes.RUNNER_ADVANCE,
                        payload: {
                            activeCtx: gameState.activeCtx,
                            activeTeam: gameState.activeTeam,
                            runners: [{
                                key: leadUnforced.key,
                                id: leadUnforced.id,
                                base: 1,
                                outcome: 'Score',
                            }],
                        },
                    },
                ],
            });
        }

        if (choices.length > 0) {
            throw new AmbiguityError('Choose runner advancement:', choices);
        }
    }
}


function resolvePlayer(jersey, state, targetTeam) {
    const teams = targetTeam ? [targetTeam] : (state.activeTeam ? [state.activeTeam] : ['away', 'home']);
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
            runnersOnBase.push({ id: event.pId || state.activeBatterId, base, key: currentBatterKey, isBatter: true });
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

    // Sort runners by base descending, but always put the batter last
    const sortedRunners = [...runnersOnBase].sort((a, b) => {
        if (a.isBatter && !b.isBatter) {
            return 1;
        }
        if (!a.isBatter && b.isBatter) {
            return -1;
        }
        return b.base - a.base;
    });

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

function getWalkRunnerAdvancements(state) {
    const runners = getRunnersOnBase(state);
    const has1B = runners.some(r => r.base === 1);
    const has2B = runners.some(r => r.base === 2);
    const has3B = runners.some(r => r.base === 3);

    const advancements = [];
    if (has1B) {
        const r1 = runners.find(r => r.base === 1);
        advancements.push({
            key: r1.key,
            id: r1.id,
            base: 1,
            outcome: 'To 2nd',
        });
        if (has2B) {
            const r2 = runners.find(r => r.base === 2);
            advancements.push({
                key: r2.key,
                id: r2.id,
                base: 2,
                outcome: 'To 3rd',
            });
            if (has3B) {
                const r3 = runners.find(r => r.base === 3);
                advancements.push({
                    key: r3.key,
                    id: r3.id,
                    base: 3,
                    outcome: 'Score',
                });
            }
        }
    }
    return advancements;
}
