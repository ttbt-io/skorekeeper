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

import { HistoryManager } from './historyManager.js';
import {
    PitchCodeCalled,
    PitchCodeSwinging,
    PitchCodeDropped,
} from '../constants.js';

const TEMPLATES = {
    '1B': {
        'line': ['rips a single', 'laces a base hit', 'lines a sharp single', 'drills a hit'],
        'ground': ['finds the hole for a single', 'hits a worm-burner single', 'ground ball base hit', 'squeaks one through'],
        'fly': ['bloops a single', 'drops a hit', 'flares one in', 'soft single'],
        'default': ['singles', 'base hit'],
    },
    '2B': {
        'line': ['rips a double', 'laces a two-bagger', 'lines a gapper for two', 'drills a double'],
        'fly': ['hits it off the wall for a double', 'skies a double', 'deep fly ball double'],
        'ground': ['hits a ground rule double', 'bounces a double'],
        'default': ['doubles', 'hits a double'],
    },
    '3B': {
        'default': ['races for a triple', 'legs out a three-bagger', 'triples deep into the gap'],
    },
    'HR': {
        'default': ['crushes a massive home run!', 'goes yard!', 'clears the fence!', 'It is high! It is far! It is gone!', 'launches a moonshot!'],
    },
    'K': {
        'default': ['swings through it for strike three.', 'goes down swinging.', 'is set down on strikes.', 'whiffs for the out.'],
    },
    'ꓘ': {
        'default': ['looks at strike three.', 'is frozen by the pitch.', 'caught looking.', 'takes a called third strike.'],
    },
};

/**
 * NarrativeEngine translates the linear game history into a structured view-model
 * for the narrative feed.
 */
export class NarrativeEngine {
    constructor() {
        this.zoneNames = {
            1: 'Pitcher',
            2: 'Catcher',
            3: 'First Base',
            4: 'Second Base',
            5: 'Third Base',
            6: 'Shortstop',
            7: 'Left Field',
            8: 'Center Field',
            9: 'Right Field',
        };
    }

    getIcon(type) {
        const map = {
            '1B': '⚾', '2B': '⚾⚾', '3B': '⚾⚾⚾', 'HR': '🎆',
            'BB': '🚶', 'IBB': '🤌', 'HBP': '🤕',
            'K': '❌', 'ꓘ': '❌', 'Out': '🔴',
            'SB': '🏃', 'CS': '🛑', 'E': '⚠️', 'BK': '⚠️',
            'Run': '💎',
        };
        return map[type] || '⚾';
    }

    getLocationName(zones) {
        if (!zones) {
            return '';
        }
        const zonesArray = Array.isArray(zones) ? zones : String(zones).replace(/[^0-9]/g, '').split('').filter(s => s !== '');
        if (zonesArray.length > 1) {
            const parts = zonesArray.join('-');
            const first = String(zonesArray[0]).charAt(0);
            return `${parts} (${this.zoneNames[first] || 'the field'})`;
        }
        return this.zoneNames[String(zonesArray[0])] || '';
    }

    formatSequence(zones) {
        if (!zones) {
            return '';
        }
        const zonesArray = Array.isArray(zones) ? zones : String(zones).replace(/[^0-9]/g, '').split('').filter(s => s !== '');
        return zonesArray.join('-');
    }

    formatPlayer(p) {
        if (!p) {
            return 'Unknown';
        }
        return `${p.name} (#${p.number})`;
    }

    getContextString(state, game) {
        if (!state) {
            return '';
        }
        const { outs, runners, score } = state;
        const onBase = [];
        if (runners[2]) {
            onBase.push(`${runners[2].name} on 3rd`);
        }
        if (runners[1]) {
            onBase.push(`${runners[1].name} on 2nd`);
        }
        if (runners[0]) {
            onBase.push(`${runners[0].name} on 1st`);
        }

        let text = `${outs} Out${outs !== 1 ? 's' : ''}.`;
        if (onBase.length > 0) {
            if (onBase.length === 3) {
                text += ` Bases loaded (${runners[0].name}, ${runners[1].name}, ${runners[2].name}).`;
            } else {
                text += ` ${onBase.reverse().join(', ')}.`;
            }
        } else {
            text += ' Bases empty.';
        }

        if (score && game) {
            text = `${game.away} ${score.away}, ${game.home} ${score.home}. ` + text;
        }

        return text;
    }

    getDeterministicTemplate(key, subKey, seed) {
        const category = TEMPLATES[key];
        if (!category) {
            return null;
        }
        const list = category[subKey] || category['default'];
        if (!list || list.length === 0) {
            return null;
        }

        if (!seed) {
            return list[0];
        }

        // Simple stable hash
        let hash = 0;
        for (let i = 0; i < seed.length; i++) {
            hash = ((hash << 5) - hash) + seed.charCodeAt(i);
            hash |= 0;
        }
        const index = Math.abs(hash) % list.length;
        return list[index];
    }

    /**
     * Processes the authoritative linear history to generate the narrative view-model.
     * @param {object} game - The full game object.
     * @param {HistoryManager} historyManager - Optional instance of HistoryManager.
     */
    generateNarrative(game, historyManager) {
        const hm = historyManager || new HistoryManager({ dispatch: () => {
        } });
        const linearHistory = hm.generateLinearHistory(game);
        const feed = [];

        let currentInningBlock = null;

        linearHistory.forEach((item) => {
            if (item.type === 'INNING_HEADER') {
                if (currentInningBlock) {
                    this.appendInningSummary(currentInningBlock, linearHistory);
                }

                currentInningBlock = {
                    id: item.id,
                    inning: item.inning,
                    team: item.team,
                    side: item.side,
                    items: [],
                };
                feed.push(currentInningBlock);
                return;
            }

            if (item.type === 'SUMMARY') {
                if (currentInningBlock) {
                    this.appendInningSummary(currentInningBlock, linearHistory);
                }
                const summaryText = this.generateGameSummary(game, item.stateBefore.score);
                feed.push({
                    id: item.id,
                    inning: 'FINAL',
                    team: 'Game Summary',
                    side: '',
                    items: [{
                        id: `${item.id}-content`,
                        type: 'SUMMARY',
                        batterText: 'Match Report',
                        context: '',
                        isStricken: false,
                        events: [{ type: 'SUMMARY', description: summaryText }],
                    }],
                });
                return;
            }

            if (!currentInningBlock) {
                return;
            }

            // Skip items that only contain CLEAR_DATA, as the previous item will display the cleared message
            if (item.events.every(e => e.type === 'CLEAR_DATA')) {
                return;
            }

            const parts = item.ctxKey.split('-');
            const batter = item.batter;

            // Clutch Check using authoritative stateBefore
            const score = item.stateBefore.score;
            const scoreDiff = Math.abs(score.home - score.away);
            const totalInnings = (game.rules && game.rules.innings) ? game.rules.innings : 7;
            const isLate = item.inning >= (totalInnings - 1);
            const isRISP = item.stateBefore.runners[1] || item.stateBefore.runners[2];
            let isClutch = false;

            if (isLate && scoreDiff <= 3 && (isRISP || item.stateBefore.runners[0])) {
                isClutch = true;
            }

            // Detect "Passive Placement" (ITB/Manual setup)
            // Item has NO generative events (PITCH, PLAY_RESULT) and IS NOT stricken
            // but contains a MANUAL_PATH_OVERRIDE that changes state.
            let isPassivePlacement = false;
            if (!item.isStricken && item.events.length > 0) {
                const hasGenerativeEvents = item.events.some(e => ['PITCH', 'PLAY_RESULT'].includes(e.type));
                const hasManualOverride = item.events.some(e => e.type === 'MANUAL_PATH_OVERRIDE');
                if (!hasGenerativeEvents && hasManualOverride) {
                    isPassivePlacement = true;
                }
            }

            const paBlock = {
                id: item.id,
                ctxKey: item.ctxKey,
                type: 'PLAY',
                isStricken: item.isStricken,
                isCorrection: item.isCorrection,
                batter: batter,
                batterText: isPassivePlacement
                    ? '' // Will be populated by events
                    : (batter ? `${this.formatPlayer(batter)} now batting` : ''),
                context: isPassivePlacement ? '' : this.getContextString(item.stateBefore, game),
                events: [],
            };

            if (isClutch) {
                paBlock.events.push({
                    type: 'CLUTCH_ALERT',
                    description: '🔥 CRITICAL SITUATION',
                });
            }

            let currentBalls = 0;
            let currentStrikes = 0;
            let currentOuts = item.stateBefore.outs;
            let pitchesInAtBat = 0;

            const hasExplicitRunnerUpdate = item.events.some(e => e.type === 'RUNNER_ADVANCE' || e.type === 'RUNNER_BATCH_UPDATE');

            item.events.forEach((action, actionIdx) => {
                const payload = action.payload || {};
                const eventSeed = `${item.id}-${actionIdx}`;

                switch (action.type) {
                    case 'PITCH': {
                        pitchesInAtBat++;
                        let desc = 'Pitch';
                        const isTwoStrikes = currentStrikes === 2;

                        if (payload.type === 'ball') {
                            desc = 'Ball';
                            currentBalls++;
                        } else if (payload.type === 'strike') {
                            desc = payload.code === PitchCodeSwinging ? 'Strike (Swinging)' : 'Strike';
                            currentStrikes++;
                        } else if (payload.type === 'foul') {
                            desc = isTwoStrikes ? 'Fouls it off, staying alive.' : 'Foul';
                            if (currentStrikes < 2) {
                                currentStrikes++;
                            }
                        } else if (payload.type === 'bip') {
                            desc = 'In Play';
                        }

                        if (pitchesInAtBat > 6) {
                            desc = `Pitch #${pitchesInAtBat}: ${desc}`;
                        }

                        paBlock.events.push({
                            type: 'PITCH',
                            description: desc,
                            count: `${currentBalls}-${currentStrikes}`,
                        });

                        if (currentBalls >= 4) {
                            paBlock.events.push({
                                type: 'PLAY',
                                description: `${this.getIcon('BB')} ${batter?.name} walks.`,
                                outcome: 'BB',
                            });
                        } else if (currentStrikes >= 3 && payload.type !== 'bip') {
                            const outcomeCode = payload.code === PitchCodeCalled ? 'ꓘ' : 'K';
                            currentOuts++;
                            const kVerb = this.getDeterministicTemplate(outcomeCode, 'default', eventSeed) || 'strikes out';
                            paBlock.events.push({
                                type: 'PLAY',
                                description: `${this.getIcon(outcomeCode)} ${batter?.name} ${kVerb}`,
                                outcome: payload.code === PitchCodeDropped ? 'D3' : outcomeCode,
                            });
                            paBlock.events.push({ type: 'OUT_COUNT', description: `${currentOuts} Out${currentOuts !== 1 ? 's' : ''}` });
                        }
                        break;
                    }
                    case 'STRIKEOUT': {
                        const outcomeCode = payload.code === PitchCodeCalled ? 'ꓘ' : 'K';
                        currentOuts++;
                        const kVerb = this.getDeterministicTemplate(outcomeCode, 'default', eventSeed) || 'strikes out';
                        paBlock.events.push({
                            type: 'PLAY',
                            description: `${this.getIcon(outcomeCode)} ${batter?.name} ${kVerb}`,
                            outcome: payload.code === PitchCodeDropped ? 'D3' : outcomeCode,
                        });
                        paBlock.events.push({ type: 'OUT_COUNT', description: `${currentOuts} Out${currentOuts !== 1 ? 's' : ''}` });
                        break;
                    }
                    case 'PLAY_RESULT': {
                        const { res, base, type, seq } = payload.bipState;
                        const resultText = this.getPlayDescription(payload.bipState, batter, payload.hitData, eventSeed);
                        let playOuts = (res !== 'Safe') ? 1 : 0;

                        let outcome = type || (res === 'Safe' ? base : 'Out');
                        if (outcome === 'HIT') {
                            outcome = (base === 'Home' || base === 'Home Run') ? 'HR' : base;
                        } else if (outcome === 'ERR') {
                            outcome = 'E';
                        } else if (['HBP', 'IBB', 'CI'].includes(type)) {
                            outcome = type;
                        } else if (outcome === 'OUT' || (res !== 'Safe' && !type)) {
                            const traj = payload.hitData?.trajectory?.toLowerCase();
                            let prefix = '';
                            if (traj === 'fly') {
                                prefix = 'F';
                            } else if (traj === 'line') {
                                prefix = 'L';
                            } else if (traj === 'pop') {
                                outcome = 'IFF';
                                prefix = '';
                            } else if (traj === 'ground') {
                                prefix = '';
                            }

                            if (outcome !== 'IFF') {
                                outcome = prefix + this.formatSequence(seq);
                            }

                            // DP/TP check: scan subsequent events in this block for outs
                            let outsInThisPlay = (res !== 'Safe' ? 1 : 0);
                            item.events.slice(actionIdx + 1).forEach(e => {
                                if (e.type === 'RUNNER_ADVANCE' || e.type === 'RUNNER_BATCH_UPDATE') {
                                    const upds = e.type === 'RUNNER_BATCH_UPDATE' ? e.payload.updates : e.payload.runners;
                                    upds.forEach(u => {
                                        const outActions = ['CS', 'Out', 'PO', 'Tag', 'Force'];
                                        if (outActions.includes(u.action || u.outcome)) {
                                            outsInThisPlay++;
                                        }
                                    });
                                }
                            });

                            if (outsInThisPlay > 1) {
                                outcome = (outsInThisPlay > 2 ? 'TP ' : 'DP ') + outcome;
                            }
                        } else if (outcome === 'SH') {
                            outcome = 'SH' + this.formatSequence(seq);
                        }

                        if (!outcome || outcome === 'Out') {
                            outcome = this.formatSequence(seq) || 'Out';
                        }
                        if (outcome === 'Home') {
                            outcome = 'HR';
                        }
                        if (type === 'D3' && res === 'Safe') {
                            outcome = 'D3';
                        }

                        paBlock.events.push({
                            type: 'PLAY',
                            description: `${this.getIcon(['HBP', 'IBB', 'CI'].includes(type) ? type : (res === 'Safe' ? base : 'Out'))} ${resultText}`,
                            outcome: outcome,
                        });

                        if (!hasExplicitRunnerUpdate && payload.runnerAdvancements) {
                            payload.runnerAdvancements.forEach(adv => {
                                if (adv.outcome === 'Out') {
                                    playOuts++;
                                }
                                const name = adv.resolvedName || 'Runner';

                                if (adv.outcome === 'Score') {
                                    paBlock.events.push({ type: 'RUNNER', description: `${this.getIcon('Run')} ${name} scores!` });
                                } else if (adv.outcome === 'Out') {
                                    paBlock.events.push({ type: 'RUNNER', description: `${this.getIcon('Out')} ${name} thrown out.` });
                                } else if (adv.outcome.startsWith('To')) {
                                    paBlock.events.push({ type: 'RUNNER', description: `${name} advances to ${adv.outcome.split(' ')[1]}.` });
                                }
                            });
                        }

                        if (playOuts > 0) {
                            currentOuts += playOuts;
                            paBlock.events.push({ type: 'OUT_COUNT', description: `${currentOuts} Out${currentOuts !== 1 ? 's' : ''}` });
                        }
                        break;
                    }
                    case 'MOVE_PLAY': {
                        paBlock.events.push({
                            type: 'SUMMARY',
                            description: 'Mistaken record was moved here from another slot.',
                        });
                        break;
                    }
                    case 'MANUAL_PATH_OVERRIDE': {
                        if (isPassivePlacement) {
                            // Generic placement logic (e.g. ITB)
                            // Determine where they ended up by checking stateAfter vs stateBefore?
                            // Or just check payload paths.
                            const paths = payload.data?.paths;
                            if (paths) {
                                const baseNames = ['1st', '2nd', '3rd', 'Home'];
                                let placedBase = -1;
                                for (let i = 0; i < 3; i++) {
                                    if (paths[i] === 1) {
                                        placedBase = i;
                                        // Assume single placement per override for ITB
                                        break;
                                    }
                                }
                                if (placedBase !== -1) {
                                    const desc = `${batter?.name || 'Runner'} placed on ${baseNames[placedBase]}.`;
                                    if (isPassivePlacement) {
                                        if (paBlock.batterText) {
                                            paBlock.batterText += ' ';
                                        }
                                        paBlock.batterText += desc;
                                    } else {
                                        paBlock.events.push({
                                            type: 'RUNNER',
                                            description: desc,
                                        });
                                    }
                                }
                            }
                        }
                        break;
                    }
                    case 'RUNNER_ADVANCE':
                    case 'RUNNER_BATCH_UPDATE': {
                        const updates = action.type === 'RUNNER_BATCH_UPDATE' ? payload.updates : payload.runners;
                        updates.forEach(u => {
                            const name = u.resolvedName || 'Runner';
                            const outcome = u.action || u.outcome;

                            if (outcome === 'Stay') {
                                return;
                            }

                            // Skip redundant Out/Safe outcomes if they just confirm the main play result
                            // This is a heuristic: if we just processed a PLAY_RESULT, we might want to skip basic outcomes here
                            // unless they are specific runner actions like SB, CS, PO.
                            // But since we are iterating, we don't easily know if "this" event is the "main" one or a side effect without looking back.
                            // However, the "Extra Out" lines in the diff are "Out".
                            // The golden file has "Red Rookie is out." (from PLAY_RESULT presumably) and then we print "Out" again?
                            // No, the golden has "Rookie is out." (outcome: Out) and our code prints "Out" as a separate line.
                            // Wait, the "Out" in the diff is a *description* or an *outcome*?
                            // In the unit test, actualLines pushes `e.description` and `e.outcome`.
                            // The diff shows:
                            //   🔴 Rookie is out.
                            // + Out
                            // This means `e.description` was "🔴 Rookie is out." and `e.outcome` was "Out".
                            // AND THEN we got another event with "Out"?
                            // No, `actualLines` pushes description, then outcome.
                            // So "🔴 Rookie is out." is description. "Out" is outcome.
                            // The GOLDEN file does NOT have the outcome code "Out" printed on a new line for this play?
                            // Let's look at the golden:
                            //   Rookie (#12) now batting
                            //   2 Outs. Bases empty.
                            //   ⚾ Rookie base hit.
                            //   1B
                            //   Speedy (#1) now batting
                            //   2 Outs. Rookie on 1st.
                            //   ➜ 🔴 Rookie put out.
                            //   3 Outs
                            //
                            // Our generated text:
                            //   ➜ 🔴 Rookie put out.
                            //   Out
                            //   3 Outs
                            //
                            // Ah! The `RUNNER_...` block pushes a `paBlock.events` entry.
                            // My code sets `description` to "➜ 🔴 Rookie put out."
                            // But I am NOT setting an `outcome` property for RUNNER events in the `RUNNER_...` case.
                            // Wait, if I don't set `outcome`, then `e.outcome` is undefined, and `actualLines` won't print it.
                            // So where is "+ Out" coming from?
                            // It must be that `e.outcome` IS being set, or I am pushing a separate event that has "Out" as description?
                            //
                            // Let's look at the `PLAY_RESULT` case. I set `outcome = ...`.
                            // If `outcome` is 'Out', it prints "Out".
                            // The golden file for "Rookie put out" (which is a Runner event, PO/CS?)
                            // In Scenario 2, Top 6:
                            // runStep("Top 6: #12 PO", ... HandleRunnerAction(ctx, "Rookie", "PO") ...)
                            // This creates a RUNNER_ADVANCE (or BATCH) event.
                            // My code for RUNNER_...:
                            // `paBlock.events.push({ type: 'RUNNER', description: ... })`
                            // It does NOT set `outcome`.
                            //
                            // So why does the test output show "+ Out"?
                            // Maybe the `PLAY_RESULT` logic is leaking?
                            //
                            // Wait, looking at the diff for Scenario 2 Top 5:
                            //   🔴 Stretch is out.
                            // + Out
                            //   1 Out
                            // This looks like a PLAY_RESULT "Stretch is out." (description) followed by "Out" (outcome).
                            // The GOLDEN file for Scenario 2 Top 5:
                            //   Stretch (#7) now batting
                            //   0 Outs. Bases empty.
                            //   🔴 Stretch is out.
                            //   1 Out
                            //
                            // The golden file DOES NOT have the outcome code "Out" or "1B" or anything for this play?
                            // Ah, for "Stretch is out.", the outcome code might be empty or specific?
                            // If it's a generic out, maybe we shouldn't print the outcome code line if it's just "Out"?
                            // The golden file usually has codes like "K", "1B", "F8".
                            // If the code is just "Out", maybe it is suppressed?
                            //
                            // Let's check `tests/unit/narrativeEngine.test.js` again.
                            // `if (e.outcome) actualLines.push(e.outcome.trim());`
                            // So if I set `outcome` to 'Out', it gets printed.
                            // The golden file does NOT have "Out" lines.
                            // So I should NOT set `outcome` to 'Out' if it's just 'Out'.
                            // Or I should filter it in the test?
                            // No, the test should match the golden.
                            // So `generateNarrative` should return an empty outcome or null if it's just generic "Out".

                            if (outcome === 'SB' || outcome === 'Adv' || outcome.startsWith('E') || outcome.startsWith('To') || outcome === 'Place' || outcome === 'Score' || ['WP', 'PB', 'BK'].includes(outcome)) {
                                let destIdx = -1;
                                if (outcome.startsWith('To')) {
                                    destIdx = ['1st', '2nd', '3rd', 'Home'].indexOf(outcome.split(' ')[1]);
                                } else if (outcome === 'Score') {
                                    destIdx = 3;
                                } else {
                                    const originBase = typeof u.base === 'number' ? u.base : (u.baseIdx !== undefined ? u.baseIdx : -1);
                                    destIdx = originBase + 1;
                                }

                                if ((destIdx === 3 || outcome === 'Score') && !(outcome === 'SB' && destIdx === 3)) {
                                    paBlock.events.push({ type: 'RUNNER', description: `${this.getIcon('Run')} ${name} scores!` });
                                }

                                if (outcome === 'SB') {
                                    paBlock.events.push({ type: 'RUNNER', description: `${this.getIcon('SB')} ${name} steals ${['1st', '2nd', '3rd', 'Home'][destIdx]}!` });
                                } else if (outcome.startsWith('E')) {
                                    let desc = `${this.getIcon('E')} ${name} advances to ${['1st', '2nd', '3rd', 'Home'][destIdx]} on an error`;
                                    const pos = outcome.substring(1);
                                    if (pos) {
                                        const locName = this.getLocationName(pos);
                                        if (locName) {
                                            desc += ` by ${locName}`;
                                        }
                                    }
                                    desc += '.';
                                    paBlock.events.push({ type: 'RUNNER', description: desc });
                                } else if (outcome.startsWith('To')) {
                                    paBlock.events.push({ type: 'RUNNER', description: `${name} advances to ${outcome.split(' ')[1]}.` });
                                } else if (outcome === 'Adv') {
                                    paBlock.events.push({ type: 'RUNNER', description: `${name} advances on the throw.` });
                                } else if (['WP', 'PB', 'BK'].includes(outcome)) {
                                    const method = outcome === 'WP' ? 'wild pitch' : (outcome === 'PB' ? 'passed ball' : 'balk');
                                    paBlock.events.push({ type: 'RUNNER', description: `${name} advances on a ${method}.` });
                                }
                            } else if (['CS', 'Out', 'PO', 'Tag', 'Force', 'INT', 'LE', 'Left Early', 'Interference'].includes(outcome)) {
                                currentOuts++;
                                let reason = 'put out';
                                if (outcome === 'INT' || outcome === 'Interference') {
                                    reason = 'out due to interference';
                                }
                                if (outcome === 'LE' || outcome === 'Left Early') {
                                    reason = 'out for leaving early';
                                }
                                paBlock.events.push({ type: 'RUNNER', description: `${this.getIcon('Out')} ${name} ${reason}.` });
                                paBlock.events.push({ type: 'OUT_COUNT', description: `${currentOuts} Out${currentOuts !== 1 ? 's' : ''}` });
                            }
                        });
                        break;
                    }
                    case 'SUBSTITUTION': {
                        const { subParams, outgoingName } = payload;
                        let desc = `🔄 Substitution: ${subParams.name}`;
                        if (outgoingName) {
                            desc += ` replaces ${outgoingName}`;
                        }
                        paBlock.events.push({ type: 'SUB', description: desc });
                        break;
                    }
                }
                // Track LOB after every discrete action
            });

            if (item.wasCleared) {
                paBlock.events.push({
                    type: 'SUMMARY',
                    description: 'Mistaken record for ' + (batter?.name || 'batter') + ' was cleared.',
                });
            }

            // Score announcement based on authoritative snapshots
            const team = parts[1];
            if (item.stateAfter && item.stateBefore && item.stateAfter.score && item.stateBefore.score && item.stateAfter.score[team] > item.stateBefore.score[team]) {
                const diff = item.stateAfter.score.home - item.stateAfter.score.away;
                const prevDiff = item.stateBefore.score.home - item.stateBefore.score.away;
                let scoreText = '';
                if (diff === 0) {
                    scoreText = '**We are tied!**';
                }
                else if (Math.sign(diff) !== Math.sign(prevDiff) && prevDiff !== 0) {
                    scoreText = `**${diff > 0 ? game.home : game.away} takes the lead!**`;
                } else if (prevDiff === 0 && diff !== 0) {
                    scoreText = `**${diff > 0 ? game.home : game.away} takes the lead!**`;
                }
                if (scoreText) {
                    const lastRun = [...paBlock.events].reverse().find(e => e.type === 'RUNNER' && e.description.includes('scores'));
                    if (lastRun) {
                        lastRun.description += ` ${scoreText}`;
                    }
                    else {
                        paBlock.events.push({ type: 'SUMMARY', description: scoreText });
                    }
                }
            }

            currentInningBlock.items.push(paBlock);
        });

        if (currentInningBlock) {
            this.appendInningSummary(currentInningBlock, linearHistory);
        }

        return feed;
    }

    appendInningSummary(block, history) {
        if (!block || block.items.length === 0) {
            return;
        }
        const lastItem = history.find(h => h.id === block.items[block.items.length - 1].id);
        const firstItem = history.find(h => h.id === block.items[0].id);
        if (!lastItem || !firstItem) {
            return;
        }

        const side = firstItem.ctxKey.split('-')[1];
        const r = lastItem.stateAfter.score[side] - firstItem.stateBefore.score[side];
        const h = lastItem.stateAfter.hits[side] - firstItem.stateBefore.hits[side];
        const lob = lastItem.stateAfter.runners.filter(r => r !== null).length;

        const summary = `Inning Summary: ${r} Run${r !== 1 ? 's' : ''}, ${h} Hit${h !== 1 ? 's' : ''}, ${lob} LOB.`;
        block.items.push({
            id: `${block.id}-summary`,
            type: 'INNING_SUMMARY',
            batterText: '',
            context: '',
            isStricken: false,
            events: [{ type: 'SUMMARY', description: summary }],
        });
    }

    getOrdinal(n) {

        const s = ['th', 'st', 'nd', 'rd'];
        const v = n % 100;
        return n + (s[(v - 20) % 10] || s[v] || s[0]);
    }

    generateGameSummary(game, score) {
        const awayName = game.away || 'Away';
        const homeName = game.home || 'Home';
        let text = `Final Score: ${awayName} ${score.away}, ${homeName} ${score.home}. `;
        if (score.away > score.home) {
            text += `${awayName} wins!`;
        }
        else if (score.home > score.away) {
            text += `${homeName} wins!`;
        }
        else {
            text += 'The game ends in a tie.';
        }
        return text;
    }

    getPlayDescription(bip, batter, hitData, seed) {
        let name = batter ? batter.name : 'Batter';

        const { res, base, type, seq } = bip;
        const explicitLocation = this.getLocationName(seq);
        let location = explicitLocation;

        if (!location && hitData && hitData.location) {
            const y = hitData.location.y;
            location = y > 0.75 ? 'Catcher' : (y > 0.4 ? 'Infield' : 'Outfield');
        }

        let text = `${name} `;

        if (res === 'Safe') {
            if (type === 'ERR') {
                text += `reaches on an error by ${explicitLocation || 'the defense'}.`;
            } else if (type === 'FC') {
                text += 'reaches on a fielder\'s choice.';
            } else if (['WP', 'PB', 'D3'].includes(type)) {
                text += 'reaches on a dropped 3rd strike.';
            } else if (type === 'HBP') {
                text += 'is hit by pitch.';
            } else if (type === 'IBB') {
                text += 'is intentionally walked.';
            } else if (type === 'CI') {
                text += 'reaches on catcher interference.';
            } else if (base === 'Home') {
                const verb = this.getDeterministicTemplate('HR', 'default', seed) || 'hits a home run!';
                text += `${verb}`;
            } else {
                const traj = hitData?.trajectory?.toLowerCase() || 'default';
                const verb = this.getDeterministicTemplate(base, traj, seed) || this.getDeterministicTemplate(base, 'default', seed) || 'hits a single';
                text += `${verb}${location ? ' to ' + location : ''}.`;
            }
        } else {
            if (type === 'Int') {
                text += 'is out due to interference.';
            } else if (type === 'SO') {
                text += 'stepped out of the batter\'s box.';
            } else if (type === 'BOO') {
                text += 'is out for batting out of order.';
            } else {
                let traj = hitData?.trajectory?.toLowerCase() || '';
                if (!traj) {
                    if (type === 'SF') {
                        traj = 'fly';
                    }
                    if (type === 'SH') {
                        traj = 'ground';
                    }
                }
                if (!traj) {
                    traj = 'default';
                }

                if (['fly', 'line', 'ground'].includes(traj)) {
                    const verb = this.getDeterministicTemplate('Out', traj, seed) || 'is out';
                    text += `${verb}${location ? ' to ' + location : ''}.`;
                } else if (location) {
                    text += `${seq && seq.length > 1 ? 'grounds' : 'flies'} out to ${location}.`;
                } else {
                    text += 'is out.';
                }
            }
        }
        return text;
    }
}