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

import { ActionTypes } from '../reducer.js';
import { createElement, generateUUID } from '../utils.js';

/**
 * Manages player substitutions and player-related context menus.
 */
export class SubstitutionManager {
    /**
     * @param {object} options
     * @param {Function} options.dispatch - App dispatch function.
     */
    constructor({ dispatch }) {
        this.dispatch = dispatch;
    }

    /**
     * Finds all players available for substitution (bench).
     */
    getAllAvailablePlayers(game, team) {
        if (!game || !game.roster || !game.roster[team]) {
            return [];
        }
        const roster = game.roster[team];
        const subs = game.subs[team] || [];

        const playerMap = new Map();

        const addPlayer = (p) => {
            const id = p.id || `${p.number}-${p.name}`;
            if (id && !playerMap.has(id)) {
                playerMap.set(id, {
                    name: p.name || '',
                    number: p.number || '',
                    pos: p.pos || '',
                    id: p.id || '',
                });
            }
        };

        // 1. Add Bench Players
        subs.forEach(addPlayer);

        // 2. Add Starters
        roster.forEach(slot => addPlayer(slot.starter));

        // 3. Add Current Players (who might have been subbed in)
        roster.forEach(slot => addPlayer(slot.current));

        return Array.from(playerMap.values());
    }

    /**
     * Renders options for the substitution datalist.
     * @param {HTMLElement} container - The datalist element.
     * @param {Array} players - List of available players.
     */
    renderSubstitutionOptions(container, players) {
        container.innerHTML = '';
        const seen = new Set();

        players.forEach(s => {
            if (!s.number && !s.name) {
                return;
            }

            const val = `${s.number} - ${s.name}`;
            if (!seen.has(val)) {
                const opt = createElement('option', {
                    dataset: {
                        u: s.number || '',
                        n: s.name || '',
                        p: s.pos || '',
                    },
                });
                opt.value = val;
                container.appendChild(opt);
                seen.add(val);
            }
        });
    }

    async handleSubstitution(team, rosterIndex, subParams, activeCtx = null) {
        const id = generateUUID();
        const action = {
            id,
            type: ActionTypes.SUBSTITUTION,
            payload: {
                team,
                rosterIndex,
                subParams,
                activeCtx,
                actionId: id,
            },
        };

        await this.dispatch(action);
        return action;
    }
}