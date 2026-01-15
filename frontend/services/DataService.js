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

import { Action } from '../models/Action.js';
import { Game } from '../models/Game.js';
import { Team } from '../models/Team.js';
import { ActionTypes } from '../reducer.js';

/**
 * Service for high-level data operations and reconciliation.
 */
export class DataService {
    /**
     * @param {object} options
     * @param {object} options.db - DBManager instance.
     * @param {object} options.auth - AuthManager instance.
     * @param {object} options.teamSync - TeamSyncManager instance.
     */
    constructor({ db, auth, teamSync }) {
        this.db = db;
        this.auth = auth;
        this.teamSync = teamSync;
    }

    /**
     * Seeds the local database with the demo game if it doesn't exist.
     */
    async ensureDemoGame() {
        const demoId = 'demo-game-001';
        try {
            const exists = await this.db.loadGame(demoId);
            if (!exists) {
                console.log('Seeding demo game...');
                const response = await fetch('assets/demo-game.json');
                if (response.ok) {
                    const gameData = await response.json();
                    gameData.ownerId = this.auth.getLocalId();
                    await this.db.saveGame(gameData);
                    console.log('Demo game seeded successfully.');
                } else {
                    console.warn('Failed to fetch demo game asset:', response.status);
                }
            }
        } catch (e) {
            console.warn('Failed to seed demo game:', e);
        }
    }

    /**
     * Adopts locally created games/teams (owned by localId) to the currently authenticated user.
     * @param {object|null} currentUser
     */
    async reconcileLocalData(currentUser) {
        if (!currentUser) {
            return;
        }
        const email = currentUser.email;
        const localId = this.auth.getLocalId();

        // 1. Reconcile Games
        const games = await this.db.getAllFullGames();
        for (const gameData of games) {
            if (gameData.ownerId === localId) {
                const game = new Game(gameData);
                game.ownerId = email;

                // Update GAME_START action if present
                if (game.actionLog && game.actionLog.length > 0) {
                    const startData = game.actionLog[0];
                    if (startData.type === ActionTypes.GAME_START && startData.payload) {
                        const startAction = new Action(startData);
                        startAction.payload.ownerId = email;
                        game.actionLog[0] = startAction.toJSON();
                    }
                }

                await this.db.saveGame(game.toJSON());
            }
        }
        // 2. Reconcile Teams
        const teams = await this.db.getAllTeams();
        for (const teamData of teams) {
            if (teamData.ownerId === localId) {
                const team = new Team(teamData);
                team.ownerId = email;
                const finalTeamData = team.toJSON();
                await this.db.saveTeam(finalTeamData);
                // Push to server
                await this.teamSync.saveTeam(finalTeamData);
            }
        }
    }
}
