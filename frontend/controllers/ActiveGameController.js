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
import { ActionTypes, computeStateFromLog, gameReducer } from '../reducer.js';
import {
    ScoresheetViewGrid,
    ScoresheetViewFeed,
    TeamAway,
    GameStatusFinal,
} from '../constants.js';

export class ActiveGameController {
    constructor(app) {
        this.app = app;
    }

    /**
     * Adds an action to the game's action log.
     * @param {object} action - The action to dispatch.
     * @param {boolean} fromRemote - Whether this action came from the server.
     */
    async dispatch(action, fromRemote = false) {
        if (!this.app.state.activeGame) {
            return;
        }

        // Wrap in Action model to ensure schemaVersion and common fields (ID, timestamp)
        const user = this.app.state.currentUser;
        const localId = this.app.auth.getLocalId();
        const actionData = new Action({
            ...action,
            userId: fromRemote ? action.userId : (user ? user.email : localId),
        }).toJSON();

        // Determine if this is a duplicate action (already in log) to avoid re-applying
        if (this.app.state.activeGame.actionLog && this.app.state.activeGame.actionLog.some(a => a.id === actionData.id)) {
            // Already processed
            return;
        }

        // Append to Log (Source of Truth)
        if (!this.app.state.activeGame.actionLog) {
            this.app.state.activeGame.actionLog = [];
        }
        this.app.state.activeGame.actionLog.push(actionData);

        let nextGameState;

        if (actionData.type === ActionTypes.UNDO) {
            // Must recompute entire state to handle tombstones
            nextGameState = computeStateFromLog(this.app.state.activeGame.actionLog);
        } else {
            // Optimization: Apply incrementally
            nextGameState = gameReducer(this.app.state.activeGame, actionData);
            // Ensure log is preserved in new state object
            nextGameState.actionLog = this.app.state.activeGame.actionLog;
        }

        // Update activeGame
        this.app.state.activeGame = nextGameState;

        // Specific Feed Update
        if (this.app.currentView === ScoresheetViewFeed) {
            this.app.renderFeed();
        }

        this.app.render();

        // Persist
        await this.app.saveState();

        // Sync if local
        if (!fromRemote) {
            this.app.sync.sendAction(actionData);
        }
    }

    /**
     * Renders the game view (scoresheet, grid, scoreboard).
     */
    renderGame() {
        const gridCont = document.getElementById('grid-container');
        const feedCont = document.getElementById('feed-container');
        const scoreboard = document.getElementById('scoresheet-scoreboard');
        const teamSwitcher = document.querySelector('.team-switcher');

        if (this.app.state.scoresheetView === ScoresheetViewGrid) {
            if (gridCont) {
                gridCont.classList.remove('hidden');
            }
            if (feedCont) {
                feedCont.classList.add('hidden');
            }
            if (scoreboard) {
                scoreboard.classList.remove('hidden');
            }
            if (teamSwitcher) {
                teamSwitcher.classList.remove('hidden');
            }
            this.app.renderScoreboard();
            this.app.renderGrid();
        } else {
            if (gridCont) {
                gridCont.classList.add('hidden');
            }
            if (feedCont) {
                feedCont.classList.remove('hidden');
            }
            if (scoreboard) {
                scoreboard.classList.add('hidden');
            }
            if (teamSwitcher) {
                teamSwitcher.classList.add('hidden');
            }
            this.app.renderFeed();
        }

        // Handle Read-Only and Finalized State UI
        const isFinal = this.app.state.activeGame && this.app.state.activeGame.status === GameStatusFinal;
        const isReadOnly = this.app.state.isReadOnly || isFinal;
        const statusInd = document.getElementById('game-status-indicator');
        const scoresheetGrid = document.getElementById('scoresheet-grid');

        if (isReadOnly) {
            if (statusInd && isFinal) {
                statusInd.classList.remove('hidden');
            }
            if (scoresheetGrid) {
                scoresheetGrid.classList.add('pointer-events-none');
            }
        } else {
            if (statusInd) {
                statusInd.classList.add('hidden');
            }
            if (scoresheetGrid) {
                scoresheetGrid.classList.remove('pointer-events-none');
            }
        }
    }

    /**
     * Loads a game by ID and prepares it for the specified view.
     * @param {string} gameId
     * @param {string} viewType
     * @param {string} [initialScoresheetView='grid']
     */
    async loadGameForView(gameId, viewType, initialScoresheetView = ScoresheetViewGrid) {
        // Validate gameId to prevent path traversal
        if (!/^[0-9a-fA-F-]{36}$/.test(gameId) && !gameId.startsWith('demo-')) {
            console.error('App: Invalid game ID format', gameId);
            window.location.hash = '';
            await this.app.loadDashboard();
            return;
        }

        let gameData = await this.app.db.loadGame(gameId);

        if (!gameData) {
            // Try fetching from server (public/shared link)
            try {
                const response = await fetch(`/api/load/${encodeURIComponent(gameId)}`);
                if (response.ok) {
                    gameData = await response.json();
                    await this.app.db.saveGame(new Game(gameData).toJSON());
                }
            } catch (e) {
                console.error('Failed to fetch remote game:', e);
            }
        }

        if (gameData) {
            let game;
            if (!gameData.actionLog || gameData.actionLog.length === 0) {
                const importAction = new Action({
                    type: ActionTypes.GAME_IMPORT,
                    payload: JSON.parse(JSON.stringify(gameData)),
                    timestamp: Date.now(),
                }).toJSON();
                gameData.actionLog = [importAction];
                game = new Game(gameData).toJSON();
                await this.app.db.saveGame(game);
            } else {
                game = computeStateFromLog(gameData.actionLog);
                // Preserve local metadata that isn't derived from the log
                if (gameData.syncStatus) {
                    game.syncStatus = gameData.syncStatus;
                }
                if (gameData.ownerId) {
                    game.ownerId = gameData.ownerId;
                }
                if (gameData.id) {
                    game.id = gameData.id;
                }
            }

            this.app.state.activeGame = game;
            this.app.state.view = viewType;
            if (viewType === 'scoresheet') {
                this.app.state.scoresheetView = initialScoresheetView;
                this.app.state.activeTeam = TeamAway;
                this.app.state.activeCtx = { b: 0, i: 1, col: 'col-1-0' };
            }

            await this.app.checkPermissions();

            // Redirect if no access and not public
            const allTeams = await this.app.db.getAllTeams();
            if (!this.app.hasReadAccess(game, allTeams)) {
                console.warn('No access to game. Redirecting to dashboard.');
                window.location.hash = '';
                await this.app.loadDashboard();
                return;
            }

            let lastActionId = '';
            if (game.actionLog && game.actionLog.length > 0) {
                lastActionId = game.actionLog[game.actionLog.length - 1].id;
            }

            // Ensure sync manager is connected to this game
            // We use bind on app for the conflict handler as it might rely on app context for now
            this.app.sync.onConflict = this.app.handleSyncConflict.bind(this.app);
            this.app.sync.connect(gameId, lastActionId);
            this.app.renderSyncStatusUI();

            this.app.render();
        } else {
            console.warn(`Game with ID ${gameId} not found. Loading dashboard.`);
            window.location.hash = '';
            await this.app.loadDashboard();
        }
    }
}
