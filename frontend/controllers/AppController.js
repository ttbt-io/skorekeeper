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

import { DBManager } from '../services/dbManager.js';
import { AuthManager } from '../services/authManager.js';
import { SyncManager } from '../services/syncManager.js';
import { TeamSyncManager } from '../services/teamSyncManager.js';
import { BackupManager } from '../services/backupManager.js';
import { modalPrompt, modalConfirm } from '../ui/modalPrompt.js';
import { generateUUID, createElement } from '../utils.js';
import { ActionTypes, computeStateFromLog, getInitialState } from '../reducer.js';
import {
    PitchTypeBall,
    PitchTypeStrike,
    PitchTypeFoul,
    PitchTypeOutLegacy,
    PitchCodeCalled,
    PitchCodeDropped,
    GameStatusFinal,
    TeamAway,
    TeamHome,
    ScoresheetViewGrid,
    ScoresheetViewFeed,
    BiPResultSingle,
    BiPModeNormal,
    BiPModeDropped,
    CurrentAppVersion,
} from '../constants.js';

import { Game } from '../models/Game.js';
import { Action } from '../models/Action.js';
import { DashboardRenderer } from '../renderers/dashboardRenderer.js';
import { TeamsRenderer } from '../renderers/teamsRenderer.js';
import { ScoresheetRenderer } from '../renderers/scoresheetRenderer.js';
import { CSORenderer } from '../renderers/csoRenderer.js';
import { StatsRenderer } from '../renderers/statsRenderer.js';
import { LineupManager } from '../game/lineupManager.js';
import { SharingManager } from '../ui/sharingManager.js';
import { ContextMenuManager } from '../ui/contextMenuManager.js';
import { RunnerManager } from '../game/runnerManager.js';
import { CSOManager } from '../game/csoManager.js';
import { SubstitutionManager } from '../game/substitutionManager.js';
import { HistoryManager } from '../game/historyManager.js';
import { ManualViewer } from '../ui/manualViewer.js';
import { StatsEngine } from '../game/statsEngine.js';
import { NarrativeEngine } from '../game/narrativeEngine.js';
import qrcode from '../vendor/qrcode.mjs';
import { PermissionService } from '../services/PermissionService.js';
import { Router } from '../router.js';
import { DataService } from '../services/DataService.js';
import { TeamController } from './TeamController.js';
import { DashboardController } from './DashboardController.js';
import { ActiveGameController } from './ActiveGameController.js';
import { parseQuery, buildQuery } from '../utils/searchParser.js';
import { PullToRefresh } from '../ui/pullToRefresh.js';

/**
 * The main application controller for the Skorekeeper PWA.
 * Manages application state, UI rendering, event handling, and data persistence.
 * @class
 */
export class AppController {
    /** @type {DBManager} Instance of the DBManager for IndexedDB operations. */
    db;
    /** @type {AuthManager} Instance of the AuthManager for authentication. */
    auth;
    /** @type {SyncManager} Instance of the SyncManager for WebSocket synchronization. */
    sync;
    /** @type {TeamSyncManager} Instance of the TeamSyncManager for team synchronization. */
    teamSync;

    /** @type {PermissionService} Instance of the PermissionService for access control. */
    permissions;
    /** @type {Router} Instance of the Router for hash handling. */
    router;
    /** @type {DataService} Instance of the DataService for data operations. */
    data;
    /** @type {TeamController} Instance of the TeamController. */
    teamController;
    /** @type {DashboardController} Instance of the DashboardController. */
    dashboardController;
    /** @type {ActiveGameController} Instance of the ActiveGameController. */
    activeGameController;

    /**
     * The application's main state object.
     * @type {object}
     * @property {string} view - Current view ('dashboard' or 'scoresheet').
     * @property {string} activeTeam - The currently active team ('away' or 'home').
     * @property {Array<object>} games - List of all saved games.
     * @property {object|null} activeGame - The currently loaded game.
     * @property {object} activeCtx - Context for the active cell (batter index, inning, column ID).
     * @property {number} activeBaseIdx - Index of the currently active base for runner actions.
     * @property {object} bipState - State for Ball-in-Play actions.
     * @property {string} bipMode - Mode for Ball-in-Play ('normal' or 'dropped').
     * @property {Array<object>} pendingRunnerState - Temporary state for runner advance logic.
     * @property {object} activeData - Data for the currently active plate appearance, including its outNum (1, 2, or 3 for inning's total outs after this play).
     * @property {boolean} isEditing - Indicates if an item is currently being edited.
     */
    state;
    /** @type {object|null} Target player/team for substitution. */
    subTarget;
    /** @type {Array<object>} Coordinates for rendering paths in the CSO zoom view. */
    pathCoords;
    /** @type {Array<object>} Coordinates for rendering paths in the grid cells. */
    gridPathCoords;

    /**
     * Creates an instance of AppController.
     * @param {DBManager} [dbManager=null] - Optional, pre-initialized DBManager instance for testing.
     * @param {Function} [modalPromptFn=modalPrompt] - Optional, injected modalPrompt function for testing.
     * @param {Function} [modalConfirmFn=modalConfirm] - Optional, injected modalConfirm function for testing.
     */
    constructor(dbManager = null, modalPromptFn = modalPrompt, modalConfirmFn = modalConfirm) {
        this.db = dbManager || new DBManager();
        this.auth = new AuthManager();
        this.sync = new SyncManager(
            this,
            this.handleRemoteAction.bind(this),
            this.handleSyncConflict.bind(this),
            this.handleSyncError.bind(this),
            this.renderSyncStatusUI.bind(this), // Pass onStatusChange callback
        );
        this.teamSync = new TeamSyncManager();
        this.backupManager = new BackupManager(this.db, this.sync, this.teamSync);
        this.modalPromptFn = modalPromptFn;
        this.modalConfirmFn = modalConfirmFn;

        this.permissions = new PermissionService();
        this.router = new Router();
        this.data = new DataService({
            db: this.db,
            auth: this.auth,
            teamSync: this.teamSync,
        });
        this.teamController = new TeamController(this);
        this.dashboardController = new DashboardController(this);
        this.activeGameController = new ActiveGameController(this);
        this.stats = new StatsEngine();
        this.narrative = new NarrativeEngine();

        this.state = {
            view: 'dashboard',
            scoresheetView: ScoresheetViewGrid, // 'grid' or 'feed'
            activeTeam: TeamAway,
            games: [],
            teams: [],
            activeGame: null,
            allGames: [], // Full game objects for stats
            aggregatedStats: null, // Aggregated stats from StatsEngine

            activeCtx: { b: 0, i: 1, col: '' },
            activeBaseIdx: -1,
            bipState: { res: 'Safe', base: BiPResultSingle, type: 'HIT', seq: [] },
            bipMode: BiPModeNormal,
            pendingBipState: null,
            pendingRunnerState: [],

            activeData: {
                outcome: '',
                balls: 0,
                strikes: 0,
                outNum: 0,
                paths: [0, 0, 0, 0],
                pathInfo: ['', '', '', ''],
                pitchSequence: [],
            },
            isEditing: false,
            isLocationMode: false,
            isReadOnly: false,
            currentUser: null,
        };

        this.contextMenuTimer = null;
        this.contextMenuTarget = null;
        this.subTarget = null;
        this.manualViewer = null;

        // Queue for sequential execution of save operations
        this.saveQueue = Promise.resolve();
        // Pending saves counter for UI feedback and unload protection
        this.pendingSaves = 0;
        this.saveIndicatorTimer = null;
        this.isResolvingConflict = false;

        // SVG coordinates for base paths (CSO zoom view)
        this.pathCoords = [
            { x1: 100, y1: 180, x2: 180, y2: 100 }, // Home to 1st
            { x1: 180, y1: 100, x2: 100, y2: 20 }, // 1st to 2nd
            { x1: 100, y1: 20, x2: 20, y2: 100 }, // 2nd to 3rd
            { x1: 20, y1: 100, x2: 100, y2: 180 }, // 3rd to Home
        ];
        // SVG coordinates for base paths (grid cells)
        // New Coords (Scaled/Shifted): Home(30,55), 1B(44,42), 2B(30,28), 3B(16,42)
        this.gridPathCoords = [
            { x1: 30, y1: 55, x2: 44, y2: 42 }, // Home to 1st
            { x1: 44, y1: 42, x2: 30, y2: 28 }, // 1st to 2nd
            { x1: 30, y1: 28, x2: 16, y2: 42 }, // 2nd to 3rd
            { x1: 16, y1: 42, x2: 30, y2: 55 }, // 3rd to Home
        ];

        this.dashboardRenderer = new DashboardRenderer({
            container: document.getElementById('game-list'),
            callbacks: {
                onSync: (id) => this.syncGame(id),
                onOpen: async(g) => {
                    if (g.syncStatus === 'remote_only') {
                        await this.syncGame(g.id);
                    }
                    window.location.hash = 'game/' + g.id;
                },
                onContextMenu: (e, id) => this.showGameContextMenu(e, id),
            },
        });

        this.teamsRenderer = new TeamsRenderer({
            listContainer: document.getElementById('teams-list'),
            membersContainer: document.getElementById('team-members-container'),
            callbacks: {
                onEdit: (team) => this.openEditTeamModal(team),
                onDelete: (id) => this.deleteTeam(id),
                onSync: (id) => this.syncTeam(id),
                canManage: (user, team) => this.canWriteTeam(user ? user.email : null, team),
                onRemoveMember: (email, roleKey) => this.removeTeamMember(email, roleKey),
                onOpenPlayerProfile: (id) => this.openPlayerProfile(id),
            },
        });

        this.scoresheetRenderer = new ScoresheetRenderer({
            gridContainer: document.getElementById('scoresheet-grid'),
            scoreboardContainer: document.getElementById('scoresheet-scoreboard'),
            feedContainer: document.getElementById('narrative-feed'),
            callbacks: {
                onColumnContextMenu: (e, colId, inning) => this.showColumnContextMenu(e, colId, inning),
                onPlayerContextMenu: (e, player, type, idx) => this.showPlayerContextMenu(e, player, type, idx),
                onPlayerSubstitution: (el, e, team, idx) => this.showSubstitutionMenu(el, e, team, idx),
                onCellClick: (b, i, col) => this.openCSO(b, i, col),
                onCellContextMenu: (e, b, col, i) => this.showCellContextMenu(e, b, col, i),
                onScoreOverride: (team, inn) => this.editScore(team, inn),
                resolvePlayerNumber: (id) => this.resolvePlayerNumber(id),
            },
        });

        this.csoRenderer = new CSORenderer({
            zoomContainer: document.querySelector('.cso-zoom-container'),
            bipFieldSvg: document.querySelector('#cso-bip-view .field-svg-keyboard svg'),
            callbacks: {
                onCycleOuts: () => this.cycleOutNum(),
                resolvePlayerNumber: (id) => this.resolvePlayerNumber(id),
                onRBIEdit: () => this.openRBIEditModal(),
                onApplyRunnerAction: (idx, code) => this.applyRunnerAction(idx, code),
            },
        });

        this.statsRenderer = new StatsRenderer({
            container: document.getElementById('stats-content'),
            callbacks: {
                onOpenPlayerProfile: (id) => this.openPlayerProfile(id),
                onExportCSV: (s) => this.onExportCSV(s),
                onExportPDF: () => this.onExportPDF(),
                getDerivedHittingStats: (s) => StatsEngine.getDerivedHittingStats(s),
                getDerivedPitchingStats: (s) => StatsEngine.getDerivedPitchingStats(s),
                calculateGameStats: (g) => StatsEngine.calculateGameStats(g),
            },
        });

        this.lineupManager = new LineupManager({
            dispatch: (a) => this.dispatch(a),
            db: this.db,
            validate: (v, m, l) => this.validate(v, m, l),
        });

        this.sharingManager = new SharingManager({
            dispatch: (a) => this.dispatch(a),
        });

        this.contextMenuManager = new ContextMenuManager();

        this.runnerManager = new RunnerManager({
            dispatch: (a) => this.dispatch(a),
            renderCSO: () => this.renderCSO(),
            getBatterId: () => this.getBatterId(),
        });

        this.csoManager = new CSOManager({
            dispatch: (a) => this.dispatch(a),
            getBatterId: () => this.getBatterId(),
        });

        this.substitutionManager = new SubstitutionManager({
            dispatch: (a) => this.dispatch(a),
        });

        this.historyManager = new HistoryManager({
            dispatch: (a) => this.dispatch(a),
        });

        this.bindEvents();
        this.init().then(() => {
            document.body.dataset.appReady = 'true';
        });
    }

    /**
     * Handles a sync conflict by showing the conflict resolution modal.
     * @param {object} msg - The conflict message from the server.
     */
    handleSyncConflict(msg) {
        const baseRevision = msg.baseRevision || '';
        const log = this.state.activeGame ? this.state.activeGame.actionLog : [];
        console.log(`App: Received Sync Conflict. baseRev="${baseRevision}", logLen=${log.length}, hasActiveGame=${!!this.state.activeGame}`);

        if (this.sync.blockConflicts || this.pendingConflictMsg || this.isResolvingConflict) {
            console.log('App: Conflict resolution in progress or blocked');
            return;
        }

        let baseIndex = -1;
        if (baseRevision === '') {
            baseIndex = -1;
        } else {
            baseIndex = log.findIndex(a => a.id === baseRevision);
        }
        console.log(`App: baseIndex=${baseIndex}`);

        if (baseIndex !== -1 || (baseRevision === '' && log.length > 0)) {
            const actionsToReplay = log.slice(baseIndex + 1);
            console.log(`App: Resolving conflict via Fast-Forward (replaying ${actionsToReplay.length} actions)`);
            if (actionsToReplay.length > 0) {
                this.isResolvingConflict = true;
                this.sync.pause();
                this.sync.resolveFastForward(baseRevision, actionsToReplay);

                // Immediately update local revision to match the end of replayed chain
                if (this.state.activeGame) {
                    const lastAction = actionsToReplay[actionsToReplay.length - 1];
                    this.state.activeGame.revision = lastAction.id;
                }

                // Small delay to let the state settle before allowing new actions to be sent
                setTimeout(() => {
                    this.isResolvingConflict = false;
                    this.sync.resume();
                }, 100);
                return;
            } else {
                this.sync.lastRevision = baseRevision;
                if (this.state.activeGame) {
                    this.state.activeGame.revision = baseRevision;
                }
                this.sync.setStatus('synced');
                return;
            }
        }

        console.log('App: Showing Conflict Resolution Modal');
        // Hide CSO properly
        this.closeCSO();
        const cso = document.getElementById('cso-modal');
        if (cso) {
            cso.classList.add('hidden');
        }

        const conflictModal = document.getElementById('conflict-resolution-modal');
        if (conflictModal) {
            this.pendingConflictMsg = msg;
            conflictModal.classList.remove('hidden');
        }
    }

    /**
     * Resolves a conflict by overwriting local changes with the server state.
     */
    async resolveConflictOverwrite() {
        if (!this.pendingConflictMsg || !this.state.activeGame) {
            return;
        }

        const gameId = this.state.activeGame.id;

        // 0. Disconnect and clear modal immediately to prevent race conditions
        this.sync.disconnect(false);
        this.sync.queue = []; // Clear any pending actions that caused the conflict
        this.sync.blockConflicts = true; // Block new conflicts while fetching state
        document.getElementById('conflict-resolution-modal').classList.add('hidden');
        this.pendingConflictMsg = null;

        try {
            // 1. Fetch full game state from server
            const response = await fetch(`/api/load/${encodeURIComponent(gameId)}`);

            if (!response.ok) {
                throw new Error('Failed to fetch authoritative game state');
            }

            const remoteGame = await response.json();

            // 2. Overwrite local activeGame
            // Recompute state from log to ensure consistency with client-side logic
            // (and ensure actionLog is present and valid)
            if (!remoteGame.actionLog) {
                remoteGame.actionLog = [];
            }
            const newState = computeStateFromLog(remoteGame.actionLog);

            this.state.activeGame = newState;

            // 3. Save to IndexedDB (Persist overwrite)
            await this.db.saveGame(newState, false);

            // 4. Update SyncManager revision before reconnecting
            const log = newState.actionLog;
            const lastId = log.length > 0 ? log[log.length - 1].id : '';
            this.sync.lastRevision = lastId;
            // Clear pending actions as we have overwritten/discarded them
            this.sync.pendingActionIds.clear();

            // 5. Update UI
            this.render();

            // 6. Reconnect with new revision
            this.sync.blockConflicts = false; // Unblock
            this.sync.connect(gameId, lastId);


        } catch (e) {
            console.error('Failed to resolve conflict:', e);
            this.sync.blockConflicts = false;
            // Re-connect even on error to attempt recovery
            this.sync.connect(gameId, this.sync.lastRevision);
        }
    }

    /**
     * Resolves a conflict by forcing the current local state to the server.
     */
    async resolveConflictForceSave() {
        if (!this.pendingConflictMsg || !this.state.activeGame) {
            return;
        }


        // 0. Disconnect to prevent further conflicts during state push
        this.sync.disconnect(false);

        try {
            // 1. Force push current local game state to server
            // Using /api/save?force=true because it overwrites the entire log without revision checks.
            const response = await fetch('/api/save?force=true', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(this.state.activeGame),
            });

            if (!response.ok) {
                throw new Error('Failed to force save local state to server');
            }

            // 2. Update SyncManager revision to match the head we just pushed
            const log = this.state.activeGame.actionLog || [];
            const lastId = log.length > 0 ? log[log.length - 1].id : '';
            this.sync.lastRevision = lastId;
            // Clear pending actions as we have forcefully synced them
            this.sync.pendingActionIds.clear();

            // 3. Clear conflict state
            document.getElementById('conflict-resolution-modal').classList.add('hidden');
            this.pendingConflictMsg = null;

            // 4. Reconnect with new revision
            this.sync.connect(this.state.activeGame.id, lastId);


        } catch (e) {
            console.error('Failed to force resolve conflict:', e);
            // Re-connect to attempt recovery
            this.sync.connect(this.state.activeGame.id, this.sync.lastRevision);
        }
    }

    /**
     * Resolves a conflict by saving current local state as a new game, then reverting original.
     */
    async resolveConflictFork() {
        if (this.pendingConflictMsg === null) {
            return;
        }

        // 1. Clone current game
        const forkedGame = JSON.parse(JSON.stringify(this.state.activeGame));
        const newId = generateUUID();
        forkedGame.id = newId;
        forkedGame.event = (forkedGame.event || '') + ' (Conflicted Copy)';

        // 2. Clean up and Re-ID the ActionLog for the forked game.
        // All actions in the fork must have NEW unique IDs to be saved properly
        // and the GAME_START must match the new game ID.
        if (forkedGame.actionLog) {
            forkedGame.actionLog = forkedGame.actionLog.map((action, index) => {
                const newAction = JSON.parse(JSON.stringify(action));
                newAction.id = generateUUID(); // New ID for every action in the fork
                if (index === 0 && newAction.type === 'GAME_START') {
                    newAction.payload.id = newId;
                }
                return newAction;
            });
        }

        // 3. Save Fork to local DB
        await this.db.saveGame(forkedGame);

        // 4. Prevent further conflicts from interrupting navigation
        this.sync.disconnect(true);
        this.sync.lastRevision = '';
        this.sync.onConflict = null;

        // 5. Clear conflict modal state locally
        document.getElementById('conflict-resolution-modal').classList.add('hidden');
        this.pendingConflictMsg = null;

        // 6. Navigate to Fork immediately
        window.location.hash = `game/${forkedGame.id}`;
    }

    /**
     * Handles a synchronization error reported by the server.
     * @param {string} error - The error message.
     */
    handleSyncError(error) {
        console.error('Sync Error:', error);
        this.renderSyncStatusUI(); // Update UI to show error status

    }

    /**
     * Handles an action received from the remote server.
     * @param {object} action - The action to apply.
     */
    async handleRemoteAction(action) {
        await this.dispatch(action, true);

        // Sync activeData with the new state from the reducer
        // This ensures that if we are currently viewing/editing this cell, our local buffer is updated.
        if (this.state.activeCtx && this.state.activeGame) {
            const k = `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`;
            if (this.state.activeGame.events[k]) {
                this.state.activeData = JSON.parse(JSON.stringify(this.state.activeGame.events[k]));
            }
        }

        this.render(); // Ensure UI updates (Scoreboard, Grid)
        this.renderSyncStatusUI(); // Update UI after remote action

        // If CSO is open, re-render it to show new data (e.g. pitch count)
        if (!document.getElementById('cso-modal').classList.contains('hidden')) {
            this.renderCSO();
        }
    }

    /**
     * Renders the synchronization status in the Scoresheet header.
     */
    renderSyncStatusUI() {
        const container = document.getElementById('sync-status-container');
        if (!container) {
            return;
        }

        const status = this.sync.currentStatus;
        let path = '';
        let text = '';
        let color = '';
        let animate = false;

        switch (status) {
            case 'connecting':
                // Wifi/Signal icon
                path = 'M8.288 15.038a5.25 5.25 0 017.424 0M5.106 11.856c3.807-3.808 9.98-3.808 13.788 0M1.924 8.674c5.565-5.565 14.587-5.565 20.152 0M12.53 18.22l-.53.53-.53-.53a.75.75 0 011.06 0z';
                text = 'Connecting...';
                color = 'text-gray-400';
                animate = true;
                break;
            case 'connected':
                // Link icon
                path = 'M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m13.35-.622l1.757-1.757a4.5 4.5 0 00-6.364-6.364l-4.5 4.5a4.5 4.5 0 001.242 7.244';
                text = 'Connected';
                color = 'text-blue-400';
                break;
            case 'synced':
                // Check circle
                path = 'M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z';
                text = 'Synced';
                color = 'text-green-400';
                break;
            case 'conflict':
                // Alert triangle
                path = 'M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z';
                text = 'Conflict!';
                color = 'text-yellow-400';
                break;
            case 'error':
                // Exclamation circle (more distinct for "Error")
                path = 'M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z';
                text = 'Error';
                color = 'text-red-400';
                break;
            case 'disconnected':
            default:
                // Cloud slash / Offline
                path = 'M3.98 8.223A4.474 4.474 0 003 10.25a4.5 4.5 0 004.5 4.5h.75m-.75-5.633c.118-.317.285-.616.493-.883m1.71-3.048a4.504 4.504 0 017.144 1.2c.123.014.245.034.366.062m5.012 3.039A4.474 4.474 0 0121 10.25a4.5 4.5 0 01-4.5 4.5H16.5m-13.5-12l18 18';
                text = 'Offline';
                color = 'text-gray-500';
                break;
        }

        container.innerHTML = '';
        const wrapper = createElement('span', {
            className: 'cursor-help',
            title: text,
        });

        const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        svg.setAttribute('viewBox', '0 0 24 24');
        svg.setAttribute('fill', 'none');
        svg.setAttribute('stroke', 'currentColor');
        svg.setAttribute('stroke-width', '2');
        svg.setAttribute('class', `w-6 h-6 ${color} ${animate ? 'animate-pulse' : ''}`);

        const p = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        p.setAttribute('d', path);
        p.setAttribute('stroke-linecap', 'round');
        p.setAttribute('stroke-linejoin', 'round');
        svg.appendChild(p);

        wrapper.appendChild(svg);
        container.appendChild(wrapper);
    }

    /**
     * Helper to check online status.
     * @returns {boolean} True if the browser is online.
     */
    get isOnline() {
        return navigator.onLine;
    }

    /**
     * Adds an action to the game's action log.
     * @param {object} action - The action to dispatch.
     * @param {boolean} fromRemote - Whether this action came from the server.
     */
    async dispatch(action, fromRemote = false) {
        await this.activeGameController.dispatch(action, fromRemote);
    }

    /**
     * Initializes the application by opening the database and handling initial routing.
     * @async
     */
    async init() {
        try {
            await this.db.open();
            const user = await this.auth.checkStatus();
            this.state.currentUser = user;

            // Reconcile any locally-owned data with the authenticated user
            await this.data.reconcileLocalData(user);

            this.renderAuthUI();
            this.renderSyncStatusUI(); // Render initial sync status

            // Set version string in UI
            const verEl = document.getElementById('app-version');
            if (verEl) {
                verEl.textContent = CurrentAppVersion;
            }

            // Subscribe to global errors
            if (window.SkErrorLogger) {
                window.SkErrorLogger.onError = (err) => this.handleGlobalError(err);
            }

            window.onhashchange = () => {
                this.handleHashChange();
            };
            window.onbeforeunload = (e) => {
                if (this.sync.isHttpDraining) {
                    e.preventDefault();
                    e.returnValue = 'Data is still saving. Are you sure you want to leave?';
                    return e.returnValue;
                }
            };
            await this.handleHashChange(); // Initial load based on hash
        }
        catch (e) {
            this.state.currentUser = this.auth.getUser();
            this.renderAuthUI();
            this.render(); // Ensure sidebar renders with user info
            if (this.auth.accessDenied) {
                await this.modalConfirmFn(this.auth.accessDeniedMessage, { isError: true, autoClose: false });
                this.auth.logout();
            }
            console.error('App: Init failed', e);
        }
    }

    async handleGlobalError(err) {
        const msg = `An unexpected error occurred:\n${err.message}\n\nType: ${err.type}`;
        const silence = await this.modalConfirmFn(msg, {
            isError: true,
            okText: 'Silence Future Errors',
            cancelText: 'Dismiss',
        });

        if (silence) {
            window.SkErrorLogger.dismissAll = true;
        }
    }

    /**
     * Toggles the visibility of the sidebar menu.
     * @param {boolean} show - Whether to show or hide the sidebar.
     */
    toggleSidebar(show) {
        const sidebar = document.getElementById('app-sidebar');
        const backdrop = document.getElementById('sidebar-backdrop');
        if (show) {
            sidebar.classList.remove('-translate-x-full');
            backdrop.classList.remove('hidden');
        } else {
            sidebar.classList.add('-translate-x-full');
            backdrop.classList.add('hidden');
        }
    }

    /**
     * Updates the sidebar menu items based on the current view.
     */
    updateSidebar() {
        const btnDashboard = document.getElementById('sidebar-btn-dashboard');
        const btnTeams = document.getElementById('sidebar-btn-teams');
        const btnAddInning = document.getElementById('sidebar-btn-add-inning');
        const btnEndGame = document.getElementById('sidebar-btn-end-game');
        const gameActionsContainer = document.getElementById('sidebar-game-actions');
        const exportActionsContainer = document.getElementById('sidebar-export-actions');
        const btnViewGrid = document.getElementById('sidebar-btn-view-grid');
        const btnViewFeed = document.getElementById('sidebar-btn-view-feed');

        // Dashboard button: show unless already on dashboard
        if (this.state.view === 'dashboard') {
            btnDashboard.classList.add('hidden');
        } else {
            btnDashboard.classList.remove('hidden');
        }

        // Teams button: show unless already on teams
        if (this.state.view === 'teams') {
            btnTeams.classList.add('hidden');
        } else {
            btnTeams.classList.remove('hidden');
        }

        // Game actions: only if a game is active and we are in the scoresheet view
        if (this.state.activeGame && this.state.view === 'scoresheet') {
            if (gameActionsContainer) {
                gameActionsContainer.classList.remove('hidden');
            }
            if (exportActionsContainer) {
                exportActionsContainer.classList.remove('hidden');
            }

            // Highlight current view mode
            if (this.state.view === 'scoresheet') {
                if (btnViewGrid) {
                    btnViewGrid.classList.add('bg-blue-600', 'text-white');
                    btnViewGrid.classList.remove('hover:bg-slate-700');
                }
                if (btnViewFeed) {
                    btnViewFeed.classList.remove('bg-blue-600', 'text-white');
                    btnViewFeed.classList.add('hover:bg-slate-700');
                }
            } else {
                if (btnViewFeed) {
                    btnViewFeed.classList.add('bg-blue-600', 'text-white');
                    btnViewFeed.classList.remove('hover:bg-slate-700');
                }
                if (btnViewGrid) {
                    btnViewGrid.classList.remove('bg-blue-600', 'text-white');
                    btnViewGrid.classList.add('hover:bg-slate-700');
                }
            }

            const isFinal = this.state.activeGame && this.state.activeGame.status === GameStatusFinal;
            const isReadOnly = this.state.isReadOnly || isFinal;
            if (isReadOnly) {
                if (btnAddInning) {
                    btnAddInning.classList.add('hidden');
                }
                if (btnEndGame) {
                    btnEndGame.classList.add('hidden');
                }
            }
            else {
                if (btnAddInning) {
                    btnAddInning.classList.remove('hidden');
                }
                if (btnEndGame) {
                    btnEndGame.classList.remove('hidden');
                }
            }
        } else {
            if (gameActionsContainer) {
                gameActionsContainer.classList.add('hidden');
            }
            if (exportActionsContainer) {
                exportActionsContainer.classList.add('hidden');
            }
        }
    }

    /**
     * Calculates the total runs for each team, respecting overrides.
     * @returns {object} { away: number, home: number }
     */
    calculateTotals() {
        if (!this.state.activeGame) {
            return { away: { R: 0, H: 0, E: 0 }, home: { R: 0, H: 0, E: 0 } };
        }
        return this.calculateStats().score;
    }

    /**
     * Ends the game, making it read-only.
     */
    async endGame() {
        if (!this.state.activeGame || this.state.activeGame.status === GameStatusFinal) {
            return;
        }

        if (await this.modalConfirmFn('Are you sure you want to end the game? It will become read-only.')) {
            const totals = this.calculateTotals();
            const stats = this.calculateStats();

            // Dispatch FINAL action
            await this.dispatch({
                type: ActionTypes.GAME_FINALIZE,
                payload: {
                    finalScore: {
                        away: totals.away.R,
                        home: totals.home.R,
                    },
                    stats: stats,
                    timestamp: Date.now(),
                },
            });

            this.render();
        }
    }

    /**
     * Renders the authentication UI in the sidebar header.
     */
    renderAuthUI() {
        const container = document.getElementById('sidebar-auth');
        if (!container) {
            return;
        }

        const user = this.auth.getUser();
        container.innerHTML = '';

        if (user) {
            const div = document.createElement('div');
            div.className = 'flex flex-col gap-2 w-full';

            const email = document.createElement('span');
            email.className = 'font-mono text-sm text-gray-300 truncate';
            email.textContent = user.email;

            const logoutBtn = document.createElement('button');
            logoutBtn.id = 'btn-logout';
            logoutBtn.className = 'bg-slate-700 hover:bg-slate-600 px-3 py-2 rounded text-white border border-slate-500 text-sm w-full';
            logoutBtn.textContent = 'Logout';
            logoutBtn.onclick = () => this.auth.logout();

            if (this.auth.isStale) {
                const staleBtn = document.createElement('button');
                staleBtn.id = 'btn-session-expired';
                staleBtn.className = 'bg-yellow-600 hover:bg-yellow-500 text-black px-2 py-1 rounded text-[10px] font-bold uppercase tracking-wider';
                staleBtn.textContent = 'Session Expired - Re-login';
                staleBtn.onclick = () => this.auth.login();
                div.appendChild(staleBtn);
            }

            div.appendChild(email);
            div.appendChild(logoutBtn);
            container.appendChild(div);
        } else {
            const loginBtn = document.createElement('button');
            loginBtn.id = 'btn-login';
            loginBtn.className = 'bg-blue-600 hover:bg-blue-500 px-4 py-2 rounded text-white font-bold shadow w-full';
            loginBtn.textContent = 'Login';
            loginBtn.onclick = () => this.auth.login();
            container.appendChild(loginBtn);
        }
    }

    /**
     * Handles changes in the URL hash, routing the application to the appropriate view or game.
     * @async
     */
    async handleHashChange() {
        const { view, params } = this.router.parseHash(window.location.hash);
        let gameId = params.gameId;

        if (gameId === 'demo') {
            gameId = 'demo-game-001';
            await this.data.ensureDemoGame();
        }

        switch (view) {
            case 'teams':
                await this.loadTeamsView();
                break;

            case 'team':
                await this.teamController.loadTeamDetail(params.teamId);
                break;

            case 'stats':
                await this.loadStatisticsView();
                break;

            case 'broadcast':
                await this.loadGameForView(gameId, 'broadcast');
                break;

            case 'scoresheet':
                if (this.state.activeGame && this.state.activeGame.id === gameId && this.state.view === 'scoresheet') {
                    this.state.scoresheetView = params.subView;
                    this.render();
                } else {
                    await this.loadGameForView(gameId, 'scoresheet', params.subView);
                }
                break;

            case 'dashboard':
            default:
                this.sync.disconnect(); // Disconnect if leaving game view
                this.renderSyncStatusUI(); // Update status after disconnecting
                await this.loadDashboard();
                break;
        }
    }

    async loadGameForView(gameId, viewType, initialScoresheetView = ScoresheetViewGrid) {
        await this.activeGameController.loadGameForView(gameId, viewType, initialScoresheetView);
    }

    /**
     * Determines if the current user has write access to the active game.
     * Sets `this.state.isReadOnly` accordingly.
     * @async
     */
    async checkPermissions() {
        const game = this.state.activeGame;
        if (!game) {
            this.state.isReadOnly = false;
            return;
        }
        const user = this.state.currentUser;
        const localId = this.auth.getLocalId();
        const allTeams = await this.db.getAllTeams();

        this.state.isReadOnly = this.permissions.isReadOnly(game, user, allTeams, localId);
    }

    hasReadAccess(game, allTeams) {
        return this.permissions.hasReadAccess(
            game,
            allTeams,
            this.state.currentUser,
            this.auth.getLocalId(),
        );
    }

    /**
     * Checks if the current user has read access to a team.
     * @param {object} team
     * @returns {boolean}
     */
    hasTeamReadAccess(team) {
        return this.permissions.hasTeamReadAccess(
            this.state.currentUser,
            this.auth.getLocalId(),
            team,
        );
    }

    /**
     * Resolves a player number from their ID by searching rosters and subs.
     */
    resolvePlayerNumber(playerId) {
        if (!this.state.activeGame) {
            return null;
        }
        let num = null;
        [TeamAway, TeamHome].forEach((t) => {
            const r = this.state.activeGame.roster[t];
            r.forEach((slot) => {
                if (slot.current.id === playerId) {
                    num = slot.current.number;
                }
                if (slot.starter.id === playerId) {
                    num = slot.starter.number;
                }
                slot.history.forEach((h) => {
                    if (h.id === playerId) {
                        num = h.number;
                    }
                });
            });
            (this.state.activeGame.subs[t] || []).forEach((s) => {
                if (s.id === playerId) {
                    num = s.number;
                }
            });
        });
        return num;
    }

    /**
     * Opens the Share Game modal.
     */
    openShareModal() {
        const game = this.state.activeGame;
        if (!game) {
            return;
        }
        const info = this.sharingManager.getInitialShareState(game);
        document.getElementById('share-public-toggle').checked = info.isPublic;
        document.getElementById('public-link-container').classList.toggle('hidden', !info.isPublic);
        document.getElementById('public-share-url').value = info.shareUrl;
        document.getElementById('broadcast-share-url').value = info.broadcastUrl;

        // Generate QR Code
        try {
            const qr = qrcode(0, 'L');
            qr.addData(info.shareUrl);
            qr.make();
            document.getElementById('share-qrcode').innerHTML = qr.createImgTag(4);
        } catch (e) {
            console.error('Failed to generate QR code:', e);
            document.getElementById('share-qrcode').innerHTML = '';
        }

        this.renderCollaborators();
        document.getElementById('share-modal').classList.remove('hidden');
    }

    renderCollaborators() {
        const container = document.getElementById('game-collaborators-list');
        if (!container || !this.state.activeGame?.permissions?.users) {
            return;
        }
        container.innerHTML = '';
        Object.keys(this.state.activeGame.permissions.users).forEach(email => {
            if (this.state.activeGame.permissions.users[email] !== 'write') {
                return;
            }
            const row = document.createElement('div');
            row.className = 'flex justify-between items-center bg-slate-50 p-2 rounded border border-slate-200';
            row.appendChild(createElement('span', {
                className: 'text-sm font-medium text-slate-700',
                text: email,
            }));
            const btn = document.createElement('button');
            btn.className = 'text-red-500 hover:text-red-700 text-xs font-bold';
            btn.textContent = 'Remove';
            btn.onclick = () => this.removeCollaborator(email);
            row.appendChild(btn);
            container.appendChild(row);
        });
    }

    async addCollaborator() {
        const email = document.getElementById('share-invite-email').value.trim();
        if (!email.includes('@')) {
            return;
        }
        await this.sharingManager.addCollaborator(this.state.activeGame, email);
        document.getElementById('share-invite-email').value = '';
        this.renderCollaborators();
    }

    async removeCollaborator(email) {
        await this.sharingManager.removeCollaborator(this.state.activeGame, email);
        this.renderCollaborators();
    }

    async togglePublicSharing() {
        const checked = document.getElementById('share-public-toggle').checked;
        await this.sharingManager.togglePublicSharing(this.state.activeGame, checked);
        document.getElementById('public-link-container').classList.toggle('hidden', !checked);
    }

    /**
     * Copies the public share URL to the clipboard.
     */
    copyShareUrl() {
        const urlInput = document.getElementById('public-share-url');
        urlInput.select();
        document.execCommand('copy');

        const btn = document.getElementById('btn-copy-share-url');
        const oldText = btn.textContent;
        btn.textContent = 'Copied!';
        btn.classList.replace('bg-blue-600', 'bg-green-600');
        setTimeout(() => {
            btn.textContent = oldText;
            btn.classList.replace('bg-green-600', 'bg-blue-600');
        }, 2000);
    }

    copyBroadcastUrl() {
        const urlInput = document.getElementById('broadcast-share-url');
        urlInput.select();
        document.execCommand('copy');

        const btn = document.getElementById('btn-copy-broadcast-url');
        const oldText = btn.textContent;
        btn.textContent = 'Copied!';
        btn.classList.replace('bg-slate-700', 'bg-green-600');
        setTimeout(() => {
            btn.textContent = oldText;
            btn.classList.replace('bg-green-600', 'bg-slate-700');
        }, 2000);
    }

    /**
     * Checks if the current user has write access to a team.
     * @param {string} userId - The user's email ID.
     * @param {object} team - The team object.
     * @returns {boolean}
     */
    canWriteTeam(userId, team) {
        return this.permissions.canWriteTeam(
            userId,
            this.auth.getLocalId(),
            team,
        );
    }

    /**
     * Loads the teams view by fetching all saved teams.
     */
    async loadTeamsView() {
        this.teamController.query = '';
        const searchInput = document.getElementById('teams-search');
        if (searchInput) {
            searchInput.value = '';
        }
        await this.teamController.loadTeamsView();
    }

    /**
     * Synchronizes a specific team with the server.
     * @param {string} teamId
     */
    async syncTeam(teamId) {
        await this.teamController.syncTeam(teamId);
    }

    /**
     * Renders the list of teams.
     */
    renderTeamsList() {
        if (!this.teamsRenderer.listContainer) {
            this.teamsRenderer.listContainer = document.getElementById('teams-list');
        }
        this.teamsRenderer.renderTeamsList(this.state.teams, this.auth.getUser());
    }

    /**
     * Opens the Team Modal for creating or editing a team.
     * @param {string|object|null} teamOrId - The team object or ID to edit, or null for a new team.
     */
    async openEditTeamModal(teamOrId = null) {
        await this.teamController.openEditTeamModal(teamOrId);
    }

    /**
     * Updates the UI tab for team members in the team modal.
     * @param {string} tab - The tab to switch to ('roster' or 'members').
     */
    switchTeamModalTab(tab) {
        this.teamController.switchTeamModalTab(tab);
    }

    /**
     * Renders the members list for the currently editing team.
     */
    renderTeamMembers() {
        this.teamController.renderTeamMembers();
    }

    /**
     * Adds a new member to the teamState and re-renders the list.
     */
    addTeamMember() {
        this.teamController.addTeamMember();
    }

    /**
     * Removes a member from teamState.
     */
    removeTeamMember(email, roleKey) {
        this.teamController.removeTeamMember(email, roleKey);
    }

    /**
     * Adds an empty player row to the team roster container.
     */
    addTeamPlayerRow() {
        this.teamController.addTeamPlayerRow();
    }

    /**
     * Closes the Team Modal.
     */
    closeTeamModal() {
        this.teamController.closeTeamModal();
    }

    /**
     * Saves the currently editing team to DB and Server.
     */
    async saveTeam() {
        await this.teamController.saveTeam();
    }

    /**
     * Deletes a team.
     * @param {string} teamId
     */
    async deleteTeam(teamId) {
        await this.teamController.deleteTeam(teamId);
    }

    /**
     * Confirms and deletes a game.
     * @param {string} gameId
     */
    async deleteGame(gameId) {
        if (await this.modalConfirmFn('Are you sure you want to delete this game?')) {
            this.pendingSaves++;
            this.updateSaveStatus();
            await this.db.deleteGame(gameId);
            if (this.auth.getUser()) {
                try {
                    await fetch('/api/delete-game', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                        },
                        body: JSON.stringify({ id: gameId }),
                    });
                } catch (e) {
                    console.error('Failed to delete game from server:', e);
                }
            }
            this.pendingSaves--;
            this.updateSaveStatus();
            await this.loadDashboard();
        }
    }

    /**
     * Loads the dashboard view by fetching and merging local and remote games.
     */
    async loadDashboard() {
        await this.dashboardController.loadDashboard();
    }

    /**
     * Generates a high-fidelity PDF by rendering a hidden print-only view and triggering window.print().
     */
    async generatePDF() {
        if (!this.state.activeGame) {
            return;
        }

        // 1. Create a print container
        const existing = document.getElementById('print-view-container');
        if (existing) {
            existing.remove();
        }

        const printContainer = document.createElement('div');
        printContainer.id = 'print-view-container';
        printContainer.className = 'print-only';
        document.body.appendChild(printContainer);

        try {
            // 2. Prepare Data
            const game = this.state.activeGame;
            const stats = StatsEngine.calculateGameStats(game);

            // 3. Add Close/Print controls (Screen only)
            const controls = document.createElement('div');
            controls.className = 'no-print p-4 bg-slate-100 border-b flex justify-between items-center sticky top-0 z-[10001]';

            const backBtn = document.createElement('button');
            backBtn.className = 'bg-slate-800 text-white px-4 py-2 rounded font-bold';
            backBtn.textContent = '← Back to App';
            backBtn.onclick = () => {
                document.body.classList.remove('print-preview-active');
                if (printContainer.parentNode) {
                    document.body.removeChild(printContainer);
                }
            };

            const printBtn = document.createElement('button');
            printBtn.className = 'bg-blue-600 text-white px-6 py-2 rounded font-bold shadow-lg';
            printBtn.textContent = 'Print Report';
            printBtn.onclick = () => window.print();

            controls.appendChild(backBtn);
            controls.appendChild(printBtn);
            printContainer.appendChild(controls);

            // 4. Render Header (Light theme)
            const header = document.createElement('div');
            header.className = 'p-8 border-b-4 border-slate-900 bg-white text-slate-900 mb-8';

            const headerTop = document.createElement('div');
            headerTop.className = 'flex justify-between items-start gap-8';

            const headerText = document.createElement('div');
            const title = document.createElement('h1');
            title.className = 'text-4xl font-black mb-2 uppercase tracking-tight';
            title.textContent = `${game.away} vs ${game.home}`;

            const meta = document.createElement('p');
            meta.className = 'text-base text-slate-500 font-bold uppercase tracking-widest';
            meta.textContent = `${game.event || 'No Event'} @ ${game.location || 'No Location'} - ${new Date(game.date).toLocaleDateString()}`;

            headerText.appendChild(title);
            headerText.appendChild(meta);
            headerTop.appendChild(headerText);

            // Reusable QR generator helper
            const getQRCodeTag = (size = 3) => {
                try {
                    const shareInfo = this.sharingManager.getInitialShareState(game);
                    const qr = qrcode(0, 'L');
                    qr.addData(shareInfo.shareUrl);
                    qr.make();
                    return qr.createImgTag(size);
                } catch (e) {
                    console.error('Failed to generate print QR code:', e);
                    return '';
                }
            };

            // QR Code for First Page Header
            const qrHeader = document.createElement('div');
            qrHeader.innerHTML = getQRCodeTag(2);
            qrHeader.className = 'flex-shrink-0';
            headerTop.appendChild(qrHeader);

            header.appendChild(headerTop);
            printContainer.appendChild(header);

            // 5. Render Scoreboard Table
            const sbContainer = document.createElement('div');
            sbContainer.className = 'px-8 mb-12 clear-both'; // Clear both to ensure sb doesn't wrap if header is short
            printContainer.appendChild(sbContainer);
            this.scoresheetRenderer.renderPrintScoreboard(sbContainer, game, stats);

            const addPageBreak = () => {
                const pb = document.createElement('div');
                pb.className = 'break-before-page';
                printContainer.appendChild(pb);
            };

            const createMiniHeader = (subtitle) => {
                const mini = document.createElement('div');
                mini.className = 'p-4 border-b-2 border-slate-900 bg-white text-slate-900 mb-6 flex justify-between items-end';

                const left = document.createElement('div');
                const t = document.createElement('h3');
                t.className = 'text-xl font-black uppercase tracking-tight';
                t.textContent = `${game.away} vs ${game.home}`;
                const m = document.createElement('p');
                m.className = 'text-[10px] text-slate-500 font-bold uppercase tracking-widest';
                m.textContent = `${game.event || 'No Event'} @ ${game.location || 'No Location'} - ${new Date(game.date).toLocaleDateString()}`;
                left.appendChild(t);
                left.appendChild(m);

                const right = document.createElement('div');
                right.className = 'text-lg font-black text-slate-400 uppercase';
                right.textContent = subtitle;

                mini.appendChild(left);
                mini.appendChild(right);
                return mini;
            };

            const addBottomQR = () => {
                const qrCont = document.createElement('div');
                qrCont.innerHTML = getQRCodeTag(2);
                qrCont.className = 'flex justify-end px-8 mt-8 mb-4';
                printContainer.appendChild(qrCont);
            };

            const renderTeamSection = (teamKey) => {
                const teamName = teamKey === TeamAway ? game.away : game.home;

                // Stats Page
                addPageBreak();
                printContainer.appendChild(createMiniHeader(`${teamName} Statistics`));

                const statsArea = document.createElement('div');
                statsArea.className = 'px-8';
                printContainer.appendChild(statsArea);
                this.statsRenderer.renderTeamBoxScore(statsArea, stats, game, teamKey);
                addBottomQR();

                // Scoresheet Page
                addPageBreak();
                printContainer.appendChild(createMiniHeader(`${teamName} Scoresheet`));

                const gridWrapper = document.createElement('div');
                gridWrapper.className = 'px-8 overflow-visible print-grid-container';
                const grid = document.createElement('div');
                grid.className = 'grid border-t border-l border-gray-300 w-fit mx-auto scale-90 origin-top';
                gridWrapper.appendChild(grid);
                printContainer.appendChild(gridWrapper);

                const printRenderer = new ScoresheetRenderer({
                    gridContainer: grid,
                    callbacks: this.scoresheetRenderer.callbacks,
                });
                printRenderer.renderGrid(game, teamKey, stats, null, { isPrint: true });
                addBottomQR();
            };

            // Order: Home then Away as requested
            renderTeamSection(TeamHome);
            renderTeamSection(TeamAway);

            // 6. Enter Preview Mode
            document.body.classList.add('print-preview-active');

            // 7. Trigger Print
            // Using a robust waiting mechanism for mobile browsers
            await new Promise(resolve => {
                requestAnimationFrame(() => {
                    requestAnimationFrame(() => {
                        setTimeout(resolve, 1000); // 1s delay to ensure everything is painted
                    });
                });
            });
            window.print();

        } catch (e) {
            console.error('PDF Generation failed:', e);
            document.body.classList.remove('print-preview-active');
            this.modalConfirmFn('Failed to generate PDF. Check console for details.', { isError: true });
            if (printContainer.parentNode) {
                document.body.removeChild(printContainer);
            }
        }
    }

    /**
     * Synchronizes a specific game with the server (Push or Pull).
     * @param {string} gameId - The ID of the game to sync.
     */
    async syncGame(gameId) {
        const game = this.state.games.find(g => g.id === gameId);
        if (!game) {
            return;
        }

        // Visual feedback
        const btn = document.getElementById(`sync-btn-${gameId}`);
        if (btn) {
            btn.textContent = '⏳'; // Spinner
            btn.disabled = true;
        }

        try {
            if (game.syncStatus === 'remote_only' || (game.syncStatus === 'unsynced' && !game.localRevision)) {
                // Validate gameId
                if (!/^[0-9a-fA-F-]{36}$/.test(gameId) && !gameId.startsWith('demo-')) {
                    throw new Error('Invalid game ID format');
                }

                // PULL: Load from server
                const response = await fetch(`/api/load/${encodeURIComponent(gameId)}`);
                if (!response.ok) {
                    throw new Error('Load failed');
                }
                const remoteData = await response.json();

                // Sanitize via Game model
                const sanitizedGame = new Game(remoteData).toJSON();

                // Save locally (clean, as it matches remote)
                await this.db.saveGame(sanitizedGame, false);
            } else {
                // PUSH: Save to server
                // We load the FULL game from DB to push it
                const fullGameData = await this.db.loadGame(gameId);
                if (fullGameData) {
                    // Sanitize via Game model
                    const fullGame = new Game(fullGameData).toJSON();
                    const response = await fetch('/api/save', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(fullGame),
                    });
                    if (response.status === 409) {
                        game.syncStatus = 'conflict';
                        // We mark it dirty to ensure it stays "Unsynced" or "Conflict" locally?
                        // Actually, setting syncStatus property directly on the object in state works for the current session render.
                        // But DashboardController re-calculates status on load.
                        // Ideally we should persist this "Conflict" state?
                        // Or just let the user see it now.
                        console.warn(`Sync Conflict for game ${gameId}`);
                        // Don't throw error, handled.
                    } else if (!response.ok) {
                        throw new Error('Save failed');
                    } else {
                        await this.db.markClean(gameId, 'games');
                    }
                }
            }
            // Refresh dashboard to show new status
            await this.loadDashboard();
        } catch (e) {
            console.error(`Sync failed for ${gameId}:`, e);
            if (btn) {
                btn.textContent = '❌';
                btn.title = 'Sync Failed';
                btn.disabled = false;
            }
        }
    }

    /**
     * Renders the appropriate view based on the current application state (`this.state.view`).
     * Also updates the undo/redo button UI.
     */
    render() {
        const view = this.state.view;
        this.showView(view + '-view');

        if (view === 'dashboard') {
            this.renderGameList();
        } else if (view === 'teams') {
            this.renderTeamsList();
        } else if (view === 'team') {
            this.teamController.renderTeamDetail();
        } else if (view === 'statistics') {
            this.renderStatistics();
        } else if (view === 'broadcast') {
            this.renderBroadcast();
        } else if (view === 'manual') {
            // Manual viewer handles its own internal rendering when show() is called
        } else if (view === 'scoresheet') {
            this.activeGameController.renderGame();
        }
        this.updateSidebar();
        this.updateUndoRedoUI();
    }

    /**
     * Shows the specified view container and hides all others.
     * @param {string} viewId - The ID of the view container to show.
     */
    showView(viewId) {
        const views = ['dashboard-view', 'scoresheet-view', 'teams-view', 'team-view', 'statistics-view', 'manual-view', 'broadcast-view'];
        views.forEach(id => {
            const el = document.getElementById(id);
            if (el) {
                if (id === viewId) {
                    el.classList.remove('hidden');
                } else {
                    el.classList.add('hidden');
                }
            }
        });
    }

    /**
     * Renders the list of games on the dashboard, with optional filtering.
     * @param {string} [filter=''] - Optional filter string to search game details.
     */
    renderGameList(filter = '') {
        // Update container reference just in case (e.g. initial load vs subsequent renders)
        if (!this.dashboardRenderer.container) {
            this.dashboardRenderer.container = document.getElementById('game-list');
        }
        this.dashboardRenderer.render(this.state.games, this.auth.getUser(), filter);
    }

    /**
     * Renders the scoreboard for the active game, displaying team names, runs, hits, errors, and inning scores.
     */
    renderScoreboard() {
        if (!this.state.activeGame) {
            return;
        }
        const stats = this.calculateStats();

        // Tab highlight
        const tA = document.getElementById('tab-away');
        const tH = document.getElementById('tab-home');
        if (this.state.activeTeam === TeamAway) {
            tA.className = 'team-tab active';
            tH.className = 'team-tab inactive';
        } else {
            tA.className = 'team-tab inactive';
            tH.className = 'team-tab active';
        }

        this.scoresheetRenderer.renderScoreboard(this.state.activeGame, stats);
    }

    /**
     * Switches the scoresheet view between Grid and Feed.
     * @param {string} view - The view to switch to ('grid' or 'feed').
     */
    switchScoresheetView(view) {
        this.state.scoresheetView = view;
        if (this.state.activeGame) {
            const prefix = view === ScoresheetViewFeed ? 'feed' : 'game';
            const newHash = `#${prefix}/${this.state.activeGame.id}`;
            if (window.location.hash !== newHash) {
                window.location.hash = newHash;
                // Updating hash triggers handleHashChange -> loadGameForView.
                // We optimize handleHashChange to not reload if ID matches.
                return;
            }
        }
        this.render();
    }

    /**
     * Renders the play-by-play narrative feed for the active game.
     */
    renderFeed() {
        if (!this.state.activeGame) {
            return;
        }
        const innings = this.narrative.generateNarrative(this.state.activeGame, this.historyManager);
        this.scoresheetRenderer.renderFeed(innings);
    }

    /**
     * Renders the broadcast-optimized view.
     */
    renderBroadcast() {
        if (!this.state.activeGame) {
            return;
        }
        const stats = this.calculateStats();
        this.scoresheetRenderer.renderBroadcast(this.state.activeGame, stats);
    }

    /**
     * Renders the main scoresheet grid, including player lineup and individual plate appearance cells.
     */
    renderGrid() {
        if (!this.state.activeGame) {
            return;
        }
        const stats = this.calculateStats();

        // Update container reference
        if (!this.scoresheetRenderer.gridContainer) {
            this.scoresheetRenderer.gridContainer = document.getElementById('scoresheet-grid');
        }

        this.scoresheetRenderer.renderGrid(
            this.state.activeGame,
            this.state.activeTeam,
            stats,
            this.state.activeCtx,
        );
    }
    syncAdvancedPanelFromQuery(query) {
        const parsed = parseQuery(query);

        // Reset
        ['adv-search-event', 'adv-search-location', 'adv-search-team', 'adv-search-date-start', 'adv-search-date-end'].forEach(id => {
            const el = document.getElementById(id);
            if (el) {
                el.value = '';
            }
        });
        const cbLocal = document.getElementById('adv-search-local');
        const cbRemote = document.getElementById('adv-search-remote');
        if (cbLocal) {
            cbLocal.checked = false;
        }
        if (cbRemote) {
            cbRemote.checked = false;
        }

        for (const f of parsed.filters) {
            if (f.key === 'event') {
                document.getElementById('adv-search-event').value = f.value;
            }
            if (f.key === 'location') {
                document.getElementById('adv-search-location').value = f.value;
            }
            if (f.key === 'away' || f.key === 'home') {
                document.getElementById('adv-search-team').value = f.value;
            } // Simplified binding

            if (f.key === 'date') {
                if (f.operator === '>=') {
                    document.getElementById('adv-search-date-start').value = f.value;
                }
                if (f.operator === '<=') {
                    document.getElementById('adv-search-date-end').value = f.value;
                }
                // Range
                if (f.operator === '..') {
                    document.getElementById('adv-search-date-start').value = f.value;
                    document.getElementById('adv-search-date-end').value = f.maxValue;
                }
            }

            if (f.key === 'is') {
                if (f.value === 'local') {
                    cbLocal.checked = true;
                }
                if (f.value === 'remote') {
                    cbRemote.checked = true;
                }
            }
        }
    }

    syncTeamsAdvancedPanelFromQuery(query) {
        const parsed = parseQuery(query);

        const nameInput = document.getElementById('teams-adv-search-name');
        if (nameInput) {
            nameInput.value = '';
        }
        const cbLocal = document.getElementById('teams-adv-search-local');
        const cbRemote = document.getElementById('teams-adv-search-remote');
        if (cbLocal) {
            cbLocal.checked = false;
        }
        if (cbRemote) {
            cbRemote.checked = false;
        }

        for (const f of parsed.filters) {
            if (f.key === 'name') {
                if (nameInput) {
                    nameInput.value = f.value;
                }
            }
            if (f.key === 'is') {
                if (f.value === 'local') {
                    cbLocal.checked = true;
                }
                if (f.value === 'remote') {
                    cbRemote.checked = true;
                }
            }
        }
        // Also map free text to name for UI convenience if filter not set?
        // Current parser treats free text separate from filters.
        // TeamController _matchesTeam checks FreeText against name.
        // So we can put free text into name input?
        // But buildTeamsAdvancedQuery creates name:filter.
        // Let's stick to explicit filters for the panel.
    }

    buildAdvancedQuery() {
        const event = document.getElementById('adv-search-event').value.trim();
        const location = document.getElementById('adv-search-location').value.trim();
        const team = document.getElementById('adv-search-team').value.trim();
        const dateStart = document.getElementById('adv-search-date-start').value;
        const dateEnd = document.getElementById('adv-search-date-end').value;
        const isLocal = document.getElementById('adv-search-local').checked;
        const isRemote = document.getElementById('adv-search-remote').checked;

        const filters = [];
        if (event) {
            filters.push({ key: 'event', value: event, operator: '=' });
        }
        if (location) {
            filters.push({ key: 'location', value: location, operator: '=' });
        }

        const tokens = [];
        if (team) {
            tokens.push(team);
        }

        if (dateStart && dateEnd) {
            filters.push({ key: 'date', value: dateStart, operator: '..', maxValue: dateEnd });
        } else if (dateStart) {
            filters.push({ key: 'date', value: dateStart, operator: '>=' });
        } else if (dateEnd) {
            filters.push({ key: 'date', value: dateEnd, operator: '<=' });
        }

        if (isLocal) {
            filters.push({ key: 'is', value: 'local', operator: '=' });
        }
        if (isRemote) {
            filters.push({ key: 'is', value: 'remote', operator: '=' });
        }

        return buildQuery({ filters, tokens });
    }

    buildTeamsAdvancedQuery() {
        const name = document.getElementById('teams-adv-search-name').value.trim();
        const isLocal = document.getElementById('teams-adv-search-local').checked;
        const isRemote = document.getElementById('teams-adv-search-remote').checked;

        const filters = [];
        // Use name filter instead of free text for precision in panel
        if (name) {
            filters.push({ key: 'name', value: name, operator: '=' });
        }
        if (isLocal) {
            filters.push({ key: 'is', value: 'local', operator: '=' });
        }
        if (isRemote) {
            filters.push({ key: 'is', value: 'remote', operator: '=' });
        }
        return buildQuery({ filters, tokens: [] });
    }

    syncStatsAdvancedPanelFromQuery(query) {
        const parsed = parseQuery(query);

        const eventInput = document.getElementById('stats-adv-search-event');
        const teamSelect = document.getElementById('stats-adv-search-team');
        const dateStart = document.getElementById('stats-adv-search-date-start');
        const dateEnd = document.getElementById('stats-adv-search-date-end');

        if (eventInput) {
            eventInput.value = '';
        }
        if (teamSelect) {
            teamSelect.value = '';
        }
        if (dateStart) {
            dateStart.value = '';
        }
        if (dateEnd) {
            dateEnd.value = '';
        }

        for (const f of parsed.filters) {
            if (f.key === 'event') {
                if (eventInput) {
                    eventInput.value = f.value;
                }
            }
            if (f.key === 'team') {
                if (teamSelect) {
                    teamSelect.value = f.value;
                }
            }
            if (f.key === 'date') {
                if (f.operator === '>=') {
                    if (dateStart) {
                        dateStart.value = f.value;
                    }
                }
                if (f.operator === '<=') {
                    if (dateEnd) {
                        dateEnd.value = f.value;
                    }
                }
                if (f.operator === '..') {
                    if (dateStart) {
                        dateStart.value = f.value;
                    }
                    if (dateEnd) {
                        dateEnd.value = f.maxValue;
                    }
                }
            }
        }
    }

    buildStatsAdvancedQuery() {
        const event = document.getElementById('stats-adv-search-event').value.trim();
        const team = document.getElementById('stats-adv-search-team').value;
        const dateStart = document.getElementById('stats-adv-search-date-start').value;
        const dateEnd = document.getElementById('stats-adv-search-date-end').value;

        const filters = [];
        if (event) {
            filters.push({ key: 'event', value: event, operator: '=' });
        }
        if (team) {
            filters.push({ key: 'team', value: team, operator: '=' });
        }
        if (dateStart && dateEnd) {
            filters.push({ key: 'date', value: dateStart, operator: '..', maxValue: dateEnd });
        } else if (dateStart) {
            filters.push({ key: 'date', value: dateStart, operator: '>=' });
        } else if (dateEnd) {
            filters.push({ key: 'date', value: dateEnd, operator: '<=' });
        }

        return buildQuery({ filters, tokens: [] });
    }


    bindEvents() {
        /**
         * Helper to get an element by its ID.
         * @param {string} id - The ID of the element.
         * @returns {HTMLElement|null} The HTMLElement or null if not found.
         */
        const byId = id => document.getElementById(id);
        /**
         * Helper to bind click and contextmenu events to an element.
         * @param {string} id - The ID of the element.
         * @param {Function} fn - The event handler function.
         */
        const click = (id, fn) => {
            const el = byId(id);
            if (el) {
                if (typeof fn !== 'function') {
                    console.error(`App: Binding failed for ID '${id}'. Function is ${typeof fn}`);
                }
                else {
                    el.onclick = fn.bind(this);
                    if (!el.oncontextmenu) {
                        el.oncontextmenu = (e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            fn.call(this, e);
                        };
                    }
                }
            }
            // else { console.warn... } // Optional: suppress warning for optional elements
        };

        // Synchronize scoreboard scrolling
        const sbContainers = ['sb-header-innings', 'sb-innings-away', 'sb-innings-home'].map(id => byId(id));
        let isSyncing = false;
        sbContainers.forEach(el => {
            if (el) {
                el.addEventListener('scroll', (e) => {
                    if (!isSyncing) {
                        isSyncing = true;
                        const left = e.target.scrollLeft;
                        sbContainers.forEach(target => {
                            if (target && target !== e.target && target.scrollLeft !== left) {
                                target.scrollLeft = left;
                            }
                        });
                        requestAnimationFrame(() => {
                            isSyncing = false;
                        });
                    }
                });
            }
        });

        // Sidebar events
        click('btn-menu-dashboard', () => this.toggleSidebar(true));
        click('btn-menu-teams', () => this.toggleSidebar(true));
        click('btn-menu-stats', () => this.toggleSidebar(true));
        click('btn-menu-scoresheet', () => this.toggleSidebar(true));
        click('sidebar-backdrop', () => this.toggleSidebar(false));

        click('sidebar-btn-dashboard', () => {
            window.location.hash = '';
            this.toggleSidebar(false);
        });
        click('btn-new-game', () => {
            this.openNewGameModal();
        });
        click('sidebar-btn-add-inning', () => {
            this.addInning();
            this.toggleSidebar(false);
        });
        click('sidebar-btn-end-game', () => {
            this.endGame();
            this.toggleSidebar(false);
        });
        click('sidebar-btn-export-pdf', () => {
            this.toggleSidebar(false);
            this.generatePDF();
        });
        click('sidebar-btn-teams', () => {
            this.loadTeamsView();
            this.toggleSidebar(false);
        });
        click('sidebar-btn-stats', () => {
            this.loadStatisticsView();
            this.toggleSidebar(false);
        });
        click('sidebar-btn-manual', () => this.openManual());
        click('sidebar-btn-report-bug', () => {
            this.toggleSidebar(false);
            this.openBugReportModal();
        });
        click('sidebar-btn-backup', () => {
            this.toggleSidebar(false);
            this.openBackupModal();
        });
        click('sidebar-btn-clear-cache', () => this.handleClearCache());
        click('btn-manual-back', () => this.closeManual());

        // Dashboard Search
        const dashboardSearch = byId('dashboard-search');
        if (dashboardSearch) {
            let debounceTimer;
            dashboardSearch.oninput = () => {
                clearTimeout(debounceTimer);
                debounceTimer = setTimeout(() => {
                    this.dashboardController.search(dashboardSearch.value);
                }, 300);
            };
        }

        // Teams Search
        const teamsSearch = byId('teams-search');
        if (teamsSearch) {
            let debounceTimer;
            teamsSearch.oninput = () => {
                clearTimeout(debounceTimer);
                debounceTimer = setTimeout(() => {
                    this.teamController.search(teamsSearch.value);
                }, 300);
            };
        }

        click('btn-toggle-teams-advanced-search', () => {
            const panel = byId('teams-advanced-search-panel');
            if (panel) {
                const isHidden = panel.classList.contains('hidden');
                if (isHidden) {
                    panel.classList.remove('hidden');
                    this.syncTeamsAdvancedPanelFromQuery(teamsSearch.value);
                } else {
                    panel.classList.add('hidden');
                }
            }
        });

        click('btn-teams-adv-apply', () => {
            const query = this.buildTeamsAdvancedQuery();
            if (teamsSearch) {
                teamsSearch.value = query;
            }
            this.teamController.search(query);
        });

        click('btn-teams-adv-clear', () => {
            const nameInput = byId('teams-adv-search-name');
            if (nameInput) {
                nameInput.value = '';
            }
            const cbLocal = byId('teams-adv-search-local');
            if (cbLocal) {
                cbLocal.checked = false;
            }
            const cbRemote = byId('teams-adv-search-remote');
            if (cbRemote) {
                cbRemote.checked = false;
            }

            if (teamsSearch) {
                teamsSearch.value = '';
            }
            this.teamController.search('');
        });

        // Advanced Search
        click('btn-toggle-advanced-search', () => {
            const panel = byId('advanced-search-panel');
            if (panel) {
                const isHidden = panel.classList.contains('hidden');
                if (isHidden) {
                    panel.classList.remove('hidden');
                    this.syncAdvancedPanelFromQuery(dashboardSearch.value);
                } else {
                    panel.classList.add('hidden');
                }
            }
        });

        click('btn-adv-apply', () => {
            const query = this.buildAdvancedQuery();
            if (dashboardSearch) {
                dashboardSearch.value = query;
            }
            this.dashboardController.search(query);
        });

        click('btn-adv-clear', () => {
            ['adv-search-event', 'adv-search-location', 'adv-search-team', 'adv-search-date-start', 'adv-search-date-end'].forEach(id => {
                const el = byId(id);
                if (el) {
                    el.value = '';
                }
            });
            const cbLocal = byId('adv-search-local');
            const cbRemote = byId('adv-search-remote');
            if (cbLocal) {
                cbLocal.checked = false;
            }
            if (cbRemote) {
                cbRemote.checked = false;
            }

            if (dashboardSearch) {
                dashboardSearch.value = '';
            }
            this.dashboardController.search('');
        });

        // Stats Search
        const statsSearch = byId('stats-search');
        if (statsSearch) {
            let debounceTimer;
            statsSearch.oninput = () => {
                clearTimeout(debounceTimer);
                debounceTimer = setTimeout(() => {
                    this.loadStatisticsView();
                }, 300);
            };
        }

        click('btn-toggle-stats-advanced-search', () => {
            const panel = byId('stats-advanced-search-panel');
            if (panel) {
                const isHidden = panel.classList.contains('hidden');
                if (isHidden) {
                    panel.classList.remove('hidden');
                    this.syncStatsAdvancedPanelFromQuery(statsSearch.value);
                } else {
                    panel.classList.add('hidden');
                }
            }
        });

        click('btn-stats-adv-apply', () => {
            const query = this.buildStatsAdvancedQuery();
            if (statsSearch) {
                statsSearch.value = query;
            }
            this.loadStatisticsView();
        });

        click('btn-stats-adv-clear', () => {
            const eventInput = byId('stats-adv-search-event');
            if (eventInput) {
                eventInput.value = '';
            }
            const teamSelect = byId('stats-adv-search-team');
            if (teamSelect) {
                teamSelect.value = '';
            }
            const dateStart = byId('stats-adv-search-date-start');
            if (dateStart) {
                dateStart.value = '';
            }
            const dateEnd = byId('stats-adv-search-date-end');
            if (dateEnd) {
                dateEnd.value = '';
            }

            if (statsSearch) {
                statsSearch.value = '';
            }
            this.loadStatisticsView();
        });

        click('btn-stats-columns', () => this.statsRenderer.renderColumnSelector());
        click('btn-cancel-stats-cols', () => byId('stats-columns-modal').classList.add('hidden'));
        click('btn-save-stats-cols', () => {
            const getChecked = (containerId) => {
                return Array.from(document.getElementById(containerId).querySelectorAll('input:checked')).map(cb => cb.value);
            };
            const config = {
                hitting: ['Player', ...getChecked('stats-cols-hitting')],
                pitching: ['Player', ...getChecked('stats-cols-pitching')],
            };
            this.statsRenderer.saveColumnConfig(config);
            byId('stats-columns-modal').classList.add('hidden');
            this.render();
        });

        // Team Management Events
        click('btn-create-team', () => this.openEditTeamModal());
        click('btn-cancel-team', () => this.closeTeamModal());
        click('btn-save-team', () => this.saveTeam());
        click('btn-cancel-bug-report', () => this.closeBugReportModal());
        click('btn-download-bug-report', () => this.downloadBugReport());
        click('btn-add-team-player', () => this.addTeamPlayerRow());
        click('tab-team-roster', () => this.switchTeamModalTab('roster'));
        click('tab-team-members', () => this.switchTeamModalTab('members'));
        click('btn-add-member', () => this.addTeamMember());
        const memberEmailInput = byId('member-invite-email');
        if (memberEmailInput) {
            memberEmailInput.onkeyup = (e) => {
                if (e.key === 'Enter') {
                    this.addTeamMember();
                }
            };
        }

        // Stats
        click('btn-rebuild-stats', () => this.rebuildStatistics());

        // Modals
        click('btn-cancel-new-game', () => this.closeNewGameModal());

        click('btn-start-new-game', this.createGame);
        const ngForm = byId('new-game-form');
        if (ngForm) {
            /* New game form onsubmit removed */
        }
        else {
            console.error('new-game-form not found');
        }

        click('btn-undo', this.undo);
        click('btn-redo', this.redo);
        click('btn-close-profile', () => document.getElementById('player-profile-modal').classList.add('hidden'));
        click('btn-share-game', this.openShareModal);
        click('btn-close-share', () => document.getElementById('share-modal').classList.add('hidden'));
        click('btn-share-add-user', () => this.addCollaborator());
        click('btn-copy-share-url', () => this.copyShareUrl());
        click('btn-copy-broadcast-url', () => this.copyBroadcastUrl());
        const shareEmailInput = byId('share-invite-email');
        if (shareEmailInput) {
            shareEmailInput.onkeyup = (e) => {
                if (e.key === 'Enter') {
                    this.addCollaborator();
                }
            };
        }
        const publicToggle = byId('share-public-toggle');
        if (publicToggle) {
            publicToggle.onchange = () => this.togglePublicSharing();
        }

        click('tab-away', () => this.switchTeam(TeamAway));
        click('tab-home', () => this.switchTeam(TeamHome));

        const tA = byId('tab-away'); if (tA) {
            tA.oncontextmenu = (e) => {
                e.preventDefault();
                e.stopPropagation();
                this.openEditLineupModal(TeamAway);
            };
        }
        const tH = byId('tab-home'); if (tH) {
            tH.oncontextmenu = (e) => {
                e.preventDefault();
                e.stopPropagation();
                this.openEditLineupModal(TeamHome);
            };
        }

        click('btn-close-cso', this.closeCSO);
        click('btn-change-pitcher', this.changePitcher);
        click('btn-undo-pitch', this.undoPitch);
        click('btn-show-bip', this.showBallInPlay);
        click('btn-clear-all', this.clearAllData);
        click('btn-toggle-action', () => {
            this.state.isEditing = true; this.renderCSO();
        });

        [PitchTypeBall, PitchTypeStrike, PitchTypeOutLegacy, PitchTypeFoul].forEach((t) => {
            const btn = document.getElementById(`btn-${t}`);
            if (btn) {
                btn.onclick = () => this.recordPitch(t);
                if (t === PitchTypeFoul) {
                    btn.oncontextmenu = (e) => {
                        e.preventDefault();
                        this.recordPitch(t);
                    };
                } else {
                    btn.oncontextmenu = (e) => {
                        e.preventDefault();
                        this.showLongPressMenu(t, e);
                    };
                }
            }
        });

        document.querySelectorAll('[data-auto-advance]').forEach((b) => {
            b.onclick = () => this.recordAutoAdvance(b.dataset.autoStart || b.innerText || b.textContent);
        });

        click('btn-cancel-bip', this.hideBallInPlay);
        click('btn-runner-advance-close', () => this.closeRunnerAdvanceView());
        click('btn-save-bip', this.commitBiP);
        click('btn-backspace', this.backspaceSequence);

        // BiP Buttons with Context Menu
        ['res', 'base', 'type'].forEach(k => {
            const btn = byId(`btn-${k}`);
            if (btn) {
                btn.onclick = () => this.cycleBiP(k);
                btn.oncontextmenu = (e) => this.showOptionsContextMenu(e, this.getCycleOptions(k), (opt) => {
                    this.state.bipState[k] = opt;
                    // Side effects
                    if (k === 'res') {
                        if (this.state.bipMode !== BiPModeDropped) {
                            const newTypes = this.getCycleOptions('type');
                            this.state.bipState.type = newTypes.length > 0 ? newTypes[0] : '';
                        }
                        if (this.state.bipState.hitData?.location) {
                            this.state.bipState.hitData.trajectory = this.getSmartDefaultTrajectory();
                        }
                    } else if (k === 'type') {
                        if (this.state.bipState.hitData?.location) {
                            this.state.bipState.hitData.trajectory = this.getSmartDefaultTrajectory();
                        }
                    }
                    this.updateBiPButtons();
                });
            }
        });

        click('btn-loc', this.toggleLocationMode); // New: Hit Location toggle

        const btnTraj = byId('btn-traj');
        if (btnTraj) {
            btnTraj.onclick = () => this.cycleTrajectory();
            btnTraj.oncontextmenu = (e) => this.showOptionsContextMenu(e, this.getCycleOptions('trajectory'), (opt) => {
                if (this.state.bipState.hitData) {
                    this.state.bipState.hitData.trajectory = opt;
                    this.renderCSO();
                }
            });
        }

        click('btn-clear-loc', this.clearHitLocation); // New: Clear hit location

        const fieldSvgKeyboard = byId('cso-bip-view').querySelector('.field-svg-keyboard');
        if (fieldSvgKeyboard) {
            fieldSvgKeyboard.onclick = (e) => this.handleFieldSvgClick(e);
        }
        document.querySelectorAll('.pos-key').forEach((k) => {
            k.onclick = (e) => this.handlePosKeyClick(e, k.dataset.pos);
        });


        click('btn-finish-turn', this.finishTurn);
        click('btn-close-runner-menu', this.closeRunnerMenu);
        click('btn-close-long-press-menu', this.closeSubmenu);

        click('btn-menu-add-col', this.addColFromMenu);
        click('btn-menu-remove-col', this.removeColFromMenu);

        // Context Menu Timers
        ['column-context-menu', 'player-context-menu', 'cso-long-press-submenu', 'game-context-menu'].forEach(id => {
            const menu = document.getElementById(id);
            if (menu) {
                menu.addEventListener('mouseenter', () => this.pauseContextMenuTimer());
                menu.addEventListener('mouseleave', () => this.resumeContextMenuTimer());
            }
        });

        click('btn-runner-actions', this.openRunnerActionView);
        click('btn-close-runner-action', this.closeRunnerActionView);
        click('btn-save-runner-actions', this.saveRunnerActions);

        click('btn-cancel-sub', this.closeSubstitutionModal);
        click('btn-confirm-sub', this.handleSubstitution);
        const sForm = byId('sub-form');
        if (sForm) {
            /* sub-form onsubmit removed */
        }
        else {
            console.error('sub-form not found');
        }

        click('btn-conflict-force-save', () => this.resolveConflictForceSave());
        click('btn-conflict-overwrite', () => this.resolveConflictOverwrite());
        click('btn-conflict-fork', () => this.resolveConflictFork());

        // Edit Game Modal Events
        click('btn-cancel-edit-game', this.closeEditGameModal);
        click('btn-save-edit-game', this.saveEditedGame);

        // Lineup Modal Events
        click('btn-cancel-lineup', this.closeEditLineupModal);

        // Backup / Restore Modal Events
        click('btn-cancel-backup', () => document.getElementById('backup-modal').classList.add('hidden'));
        click('btn-start-backup', () => this.startBackup());
        click('btn-open-restore', () => {
            document.getElementById('backup-modal').classList.add('hidden');
            this.openRestoreModal();
        });
        click('btn-cancel-restore', () => document.getElementById('restore-modal').classList.add('hidden'));
        click('btn-select-restore-file', () => document.getElementById('restore-file-input').click());

        const fileInput = byId('restore-file-input');
        if (fileInput) {
            fileInput.onchange = (e) => this.handleRestoreFileSelect(e);
        }

        click('btn-restore-select-all', () => this.toggleRestoreSelection(true));
        click('btn-restore-select-none', () => this.toggleRestoreSelection(false));
        click('btn-confirm-restore', () => this.startRestore());

        click('sidebar-btn-view-grid', () => {
            this.switchScoresheetView(ScoresheetViewGrid);
            this.toggleSidebar(false);
        });
        click('sidebar-btn-view-feed', () => {
            this.switchScoresheetView(ScoresheetViewFeed);
            this.toggleSidebar(false);
        });

        click('btn-save-lineup', this.saveLineup);
        click('btn-add-starter-row', () => this.addPlayerToGroup('starter'));
        click('btn-add-sub-row', () => this.addPlayerToGroup('sub'));

        const subNumInput = byId('sub-incoming-num');
        if (subNumInput) {
            subNumInput.addEventListener('input', this.handleSubInput.bind(this));
        }

        document.addEventListener('click', e => this.handleGlobalClick(e));

        const zoomCont = document.querySelector('.cso-zoom-container');
        if (zoomCont) {
            zoomCont.onclick = e => this.handleZoomClick(e);
        }

        // Contextual Help Links
        document.querySelectorAll('[data-help-section]').forEach(btn => {
            btn.onclick = (e) => {
                e.stopPropagation();
                const section = btn.dataset.helpSection;
                this.openManual(section);
            };
        });

        // Signal that the app is fully initialized and ready
        document.body.setAttribute('data-app-ready', 'true');
    }

    /**
     * Handles global click events to close context menus and modals when clicking outside.
     * @param {MouseEvent} e - The click event.
     */
    handleGlobalClick(e) {
        if (this.contextMenuManager.activeContextMenu && !this.contextMenuManager.activeContextMenu.contains(e.target)) {
            this.hideContextMenu();
        }
        const optMenu = document.getElementById('options-context-menu');
        if (optMenu && !optMenu.classList.contains('hidden') && !optMenu.contains(e.target) && this.contextMenuManager.activeContextMenu !== optMenu) {
            optMenu.classList.add('hidden');
        }

        ['cso-modal', 'substitution-modal', 'new-game-modal', 'edit-game-modal', 'edit-lineup-modal'].forEach((id) => {
            const m = document.getElementById(id);
            if (!m) {
                return;
            }
            const isBackdropClick = (e.target === m) || (e.target.classList.contains('modal-backdrop-blur') && e.target.closest('#' + id) === m);
            if (isBackdropClick) {
                if (id === 'cso-modal') {
                    this.closeCSO();
                }
                else if (id === 'substitution-modal') {
                    this.closeSubstitutionModal();
                }
                else if (id === 'new-game-modal') {
                    this.closeNewGameModal();
                }
                else if (id === 'edit-game-modal') {
                    this.closeEditGameModal();
                }
                else if (id === 'edit-lineup-modal') {
                    this.closeEditLineupModal();
                }
            }
        });
    }

    /**
     * Handles click events within the CSO zoom container, specifically for base paths.
     * @param {MouseEvent} e - The click event.
     */
    handleZoomClick(e) {
        const t = e.target;
        let idx = -1;
        if (t.id.includes('zoom-bg-') || t.id.includes('zoom-base-')) {
            if (t.id.includes('zoom-bg-1') || t.id.includes('zoom-base-1b')) {
                idx = 0;
            }
            else if (t.id.includes('zoom-bg-2') || t.id.includes('zoom-base-2b')) {
                idx = 1;
            }
            else if (t.id.includes('zoom-bg-3') || t.id.includes('zoom-base-3b')) {
                idx = 2;
            }
            else if (t.id.includes('zoom-bg-4') || t.id.includes('zoom-base-home')) {
                idx = 3;
            }
        }
        if (idx !== -1) {
            this.handleBaseClick(e, idx);
        }
    }

    /**
     * Returns the available options for a given cycle button type.
     * @param {string} key - The type of cycle button ('res', 'base', 'type', 'trajectory', 'runnerAction', 'runnerAdvance').
     * @param {any} context - Additional context (e.g., runner index).
     * @returns {string[]} Array of option strings.
     */
    getCycleOptions(key, context = null) {
        if (key === 'runnerAction') {
            return ['Stay', 'SB', 'CS', 'PO', 'Adv', 'Left Early', 'Look Back', 'INT'];
        }
        if (key === 'runnerAdvance') {
            const r = this.state.pendingRunnerState[context];
            if (!r) {
                return [];
            }
            const order = ['Stay'];
            if (r.base === 0) {
                order.push('To 2nd');
            }
            if (r.base <= 1) {
                order.push('To 3rd');
            }
            order.push('Score');
            order.push('Out');
            return order;
        }

        // Delegate BIP options to CSOManager
        return this.csoManager.getBipOptions(key, this.state.bipState.res, this.state.bipMode);
    }

    /**
     * Shows a context menu with options for a cycle button.
     * @param {MouseEvent} e - The event triggering the menu.
     * @param {string[]} options - The list of options to display.
     * @param {Function} onSelect - Callback function when an option is selected.
     */
    showOptionsContextMenu(e, options, onSelect) {
        if (this.state.isReadOnly) {
            return;
        }
        e.preventDefault(); e.stopPropagation();
        this.contextMenuManager.showOptions(e, options, onSelect);
    }

    /**
     * Opens the runner action modal, displaying available actions for runners on base.
     */
    openRunnerActionView() {
        const runners = this.getRunnersOnBase();
        if (runners.length === 0) {
            return;
        }
        this.state.pendingRunnerState = runners.map(r => ({ ...r, action: 'Stay' }));
        this.renderRunnerActionList();
        document.getElementById('cso-main-view').classList.add('hidden');
        document.getElementById('cso-runner-action-view').classList.remove('hidden');
    }

    /**
     * Renders the list of runners and their pending actions.
     */
    renderRunnerActionList() {
        this.csoRenderer.renderRunnerActionList(
            this.state.pendingRunnerState,
            this.state.activeGame.events,
            (i) => this.cycleRunnerAction(i),
            (e, i) => this.showOptionsContextMenu(e, this.getCycleOptions('runnerAction'), (opt) => {
                if (this.state.pendingRunnerState[i]) {
                    this.state.pendingRunnerState[i].action = opt;
                    this.renderRunnerActionList();
                }
            }),
        );
    }

    /**
     * Cycles the action for a specific runner in the pending state.
     * @param {number} idx - Index in pendingRunnerState.
     */
    cycleRunnerAction(idx) {
        const r = this.state.pendingRunnerState[idx];
        if (!r) {
            return;
        }
        const options = this.getCycleOptions('runnerAction');
        this.runnerManager.cycleAction(idx, this.state.pendingRunnerState, options);
        this.renderRunnerActionList();
    }

    /**
     * Saves the batch of runner actions.
     */
    async saveRunnerActions() {
        const updates = this.state.pendingRunnerState
            .filter(r => r.key && r.action && r.action !== 'Stay')
            .map(r => ({
                key: r.key,
                base: r.base,
                action: r.action,
            }));

        if (updates.length > 0) {
            await this.dispatch({
                type: ActionTypes.RUNNER_BATCH_UPDATE,
                payload: {
                    updates,
                    activeCtx: this.state.activeCtx,
                    activeTeam: this.state.activeTeam,
                },
            });
            this.renderCSO(); // Refresh CSO to show updated paths/outs
        }

        this.closeRunnerActionView();

        // Check for 3rd out (Standalone Runner Action -> Current Batter leads next inning)
        const stats = this.getInningStats(this.state.activeTeam, this.state.activeCtx.i);
        if (stats.outs >= 3) {
            await this.setNextInningLead(this.state.activeCtx.b);
        }
    }

    /**
     * Closes the runner action modal and returns to the main CSO view.
     */
    closeRunnerActionView() {
        const cav = document.getElementById('cso-runner-action-view');
        if (cav) {
            cav.classList.add('hidden');
        }
        const cmv = document.getElementById('cso-main-view');
        if (cmv) {
            cmv.classList.remove('hidden');
        }
    }

    /**
     * Closes the runner advance modal and returns to the main CSO view.
     */
    closeRunnerAdvanceView() {
        const cav = document.getElementById('cso-runner-advance-view');
        if (cav) {
            cav.classList.add('hidden');
        }
        const cmv = document.getElementById('cso-main-view');
        if (cmv) {
            cmv.classList.remove('hidden');
        }
    }

    /**
     * Clears all caches and unregisters service workers, then reloads the page.
     * Use this to force a full refresh of the application logic.
     */
    async handleClearCache() {
        const confirmed = await modalConfirm('This will clear the app cache and reload to get the latest version. Continue?');
        if (!confirmed) {
            return;
        }

        try {
            // Unregister service workers
            if ('serviceWorker' in navigator) {
                const registrations = await navigator.serviceWorker.getRegistrations();
                for (const registration of registrations) {
                    await registration.unregister();
                }
            }

            // Clear caches
            if ('caches' in window) {
                const keys = await caches.keys();
                await Promise.all(keys.map(key => caches.delete(key)));
            }

            // Reload page ignoring cache
            window.location.reload(true);
        } catch (error) {
            console.error('Failed to clear cache:', error);
            await modalConfirm('Failed to clear cache. Please try manually in browser settings.', { isError: true });
        }
    }

    /**
     * Sanitizes a string for safe HTML display by escaping HTML entities.
     * @param {string} str - The string to sanitize.
     * @returns {string} The sanitized string.
     */
    /**
     * Validates that a string value does not exceed a maximum length.
     * Displays an alert if validation fails.
     * @param {string} val - The value to validate.
     * @param {number} max - The maximum allowed length.
     * @param {string} name - The display name of the field for the error message.
     * @returns {boolean} True if valid, false otherwise.
     */
    validate(val, max, name) {
        if (val && val.length > max) {
            this.modalConfirmFn(`${name} must be ${max} characters or less.`, { isError: true });
            return false;
        }
        return true;
    }

    /**
     * Creates a new game based on input from the new game modal, saves it to the database,
     * and navigates to its scoresheet.
     * @async
     */
    async createGame() {
        const getSanitizedId = (id) => {
            if (!id || id.startsWith('--')) {
                return null;
            }
            return id;
        };

        const awayId = getSanitizedId(document.getElementById('team-away-select').value);
        const homeId = getSanitizedId(document.getElementById('team-home-select').value);
        let away = document.getElementById('team-away-input').value;
        let home = document.getElementById('team-home-input').value;

        // Fetch team data if selected
        const localTeams = await this.db.getAllTeams();
        const awayTeam = awayId ? localTeams.find(t => t.id === awayId) : null;
        const homeTeam = homeId ? localTeams.find(t => t.id === homeId) : null;

        if (awayTeam) {
            away = awayTeam.name;
        }
        if (homeTeam) {
            home = homeTeam.name;
        }

        const eventName = document.getElementById('game-event-input').value || '';
        const location = document.getElementById('game-location-input').value || 'Park';
        const dateInput = document.getElementById('game-date-input').value;
        const date = dateInput ? new Date(dateInput).toISOString() : new Date().toISOString();

        if (!this.validate(away, 50, 'Away Team') ||
            !this.validate(home, 50, 'Home Team') ||
            !this.validate(eventName, 100, 'Event Name') ||
            !this.validate(location, 100, 'Location')) {
            return;
        }

        // Prepare Initial Rosters and Subs
        const initialRosters = { away: [], home: [] };
        const initialSubs = { away: [], home: [] };
        [TeamAway, TeamHome].forEach(side => {
            const team = side === TeamAway ? awayTeam : homeTeam;
            if (team && team.roster && team.roster.length > 0) {
                // Use team roster
                const starters = team.roster.slice(0, 9);
                const bench = team.roster.slice(9);

                starters.forEach((p, _i) => {
                    initialRosters[side].push({
                        id: p.id,
                        name: p.name,
                        number: p.number,
                        pos: p.pos || '',
                    });
                });
                // Fill up to 9 starters if roster is short
                for (let i = starters.length; i < 9; i++) {
                    initialRosters[side].push({
                        id: generateUUID(),
                        name: `${side === TeamAway ? 'Player' : 'H Player'} ${i + 1}`,
                        number: `${i + 1}`,
                    });
                }

                bench.forEach(p => {
                    initialSubs[side].push({
                        id: p.id,
                        name: p.name,
                        number: p.number,
                        pos: p.pos || '',
                    });
                });
            } else {
                // Generate new roster
                for (let i = 0; i < 9; i++) {
                    initialRosters[side].push({
                        id: generateUUID(),
                        name: `${side === TeamAway ? 'Player' : 'H Player'} ${i + 1}`,
                        number: `${i + 1}`,
                    });
                }
            }
        });

        const newGameId = generateUUID();

        // Initialize activeGame shell so dispatch has something to work with
        this.state.activeGame = getInitialState();

        const startAction = new Action({
            type: ActionTypes.GAME_START,
            payload: {
                id: newGameId,
                date,
                location,
                event: eventName,
                away,
                home,
                initialRosters,
                initialSubs,
                awayTeamId: awayId || null,
                homeTeamId: homeId || null,
                ownerId: this.state.currentUser ? this.state.currentUser.email : this.auth.getLocalId(),
                permissions: { public: 'none', users: {} },
            },
        }).toJSON();

        await this.dispatch(startAction);

        this.closeNewGameModal();
        window.location.hash = `game/${newGameId}`;
    }

    /**
     * Sets the view to 'scoresheet' for the active game and initializes history.
     */
    navigateToScoreSheet() {
        this.state.view = 'scoresheet';
        this.state.activeTeam = TeamAway;
        this.render();
    }

    /**
     * Switches the currently active team and re-renders the scoreboard and grid.
     * @param {string} team - The team to switch to ('away' or 'home').
     */
    switchTeam(team) {
        this.state.activeTeam = team;
        this.renderScoreboard();
        this.renderGrid();
    }

    /**

         * Updates the save indicator UI based on pending saves.

         * Shows "Saving..." after 100ms delay if saves are pending.

         * Hides immediately if no saves are pending.

         */

    updateSaveStatus() {
        const indicator = document.getElementById('save-indicator');

        if (!indicator) {
            return;
        }

        if (this.pendingSaves > 0) {
            if (!this.saveIndicatorTimer && indicator.classList.contains('hidden')) {
                this.saveIndicatorTimer = setTimeout(() => {
                    if (this.pendingSaves > 0) {
                        indicator.classList.remove('hidden');

                    }

                    this.saveIndicatorTimer = null;

                }, 100);

            }
        }
        else {
            if (this.saveIndicatorTimer) {
                clearTimeout(this.saveIndicatorTimer);

                this.saveIndicatorTimer = null;

            }

            indicator.classList.add('hidden');

        }
    }





    /**

         * Saves the current active game state to history and to IndexedDB.

         * Uses a promise queue to ensure sequential writes.

         * @async

         */

    async saveState() {
        if (!this.state.activeGame) {
            return;
        }

        // 1. Synchronously capture and sanitize the state NOW using the Game model.
        const currentStateSnapshot = new Game(this.state.activeGame).toJSON();

        this.updateUndoRedoUI();
        this.pendingSaves++;
        this.updateSaveStatus();

        // 2. Queue the async DB operation
        this.saveQueue = this.saveQueue.then(async() => {
            // Optimize Storage: Do NOT persist the undo/redo stacks to IndexedDB.
            // This keeps the DB record small (~10KB vs ~500KB) and efficient for syncing.
            // Undo history becomes session-only (lost on refresh).

            // DBManager.saveGame now handles converting the state snapshot into the
            // Event Sourcing schema (Actions + Metadata), so we just pass the full snapshot.
            await this.db.saveGame(currentStateSnapshot);

            // Incremental Stats Update:
            // Calculate stats for this game and save to game_stats store.
            // This ensures we always have up-to-date stats for this game without full rehydration later.
            const stats = StatsEngine.calculateGameStats(this.state.activeGame);
            await this.db.saveGameStats(this.state.activeGame.id, stats);

            this.pendingSaves--;
            this.updateSaveStatus();
        }).catch((err) => {
            console.error('App: Error during saveState queue execution', err);
            this.pendingSaves--;
            this.updateSaveStatus();
            this.modalConfirmFn('CRITICAL ERROR: Data could not be saved! Check console for details. Please do not close this tab until resolved.', { autoClose: false, isError: true });
        });
        return this.saveQueue;
    }

    /**
     * Helper to build a map of ID -> Action for O(1) lookups.
     * @param {Array} log - The action log.
     * @returns {Map} Map of action IDs to action objects.
     */
    _buildActionMap(log) {
        const map = new Map();
        log.forEach(a => map.set(a.id, a));
        return map;
    }

    /**
     * Determines the target action ID for the next Undo operation.
     * Logic: Find the latest effective Generative or Redo action. Skip effective Primary Undos.
     * @param {Array} log - The action log.
     * @returns {string|null} The ID of the action to undo, or null if none.
     */
    _getUndoTargetId(log) {
        const effectivelyUndone = this.getEffectivelyUndoneSet(log);
        const actionMap = this._buildActionMap(log);

        for (let i = log.length - 1; i >= 0; i--) {
            const action = log[i];
            if (effectivelyUndone.has(action.id)) {
                continue;
            }

            if (action.type === ActionTypes.UNDO) {
                // Check what this undo targets
                const targetId = action.payload.refId;
                const target = actionMap.get(targetId);
                if (target && target.type === ActionTypes.UNDO) {
                    // This is a Redo (Undo of Undo). It effectively restored state.
                    // We can Undo this Redo.
                    return action.id;
                }
                // Else: Primary Undo (Undo of Generative).
                // We are currently "behind" this action in state. Keep scanning backwards.
                continue;
            } else {
                // Generative Action. It is effective. We can Undo it.
                return action.id;
            }
        }
        return null;
    }

    /**
     * Determines the target action ID for the next Redo operation.
     * Logic: Find the latest Effective Primary Undo. Stop if blocked by Generative or Redo actions.
     * @param {Array} log - The action log.
     * @returns {string|null} The ID of the action to redo (which is the ID of the Undo action to be undone), or null.
     */
    _getRedoTargetId(log) {
        const effectivelyUndone = this.getEffectivelyUndoneSet(log);
        const actionMap = this._buildActionMap(log);

        for (let i = log.length - 1; i >= 0; i--) {
            const action = log[i];
            if (effectivelyUndone.has(action.id)) {
                continue;
            }

            if (action.type === ActionTypes.UNDO) {
                const targetId = action.payload.refId;
                const target = actionMap.get(targetId);
                if (target && target.type !== ActionTypes.UNDO) {
                    // This is a Primary Undo (targets Generative).
                    // It is the barrier preventing us from seeing the future.
                    // We can Redo it.
                    return action.id;
                }
                // Else: It is a Redo (targets Undo).
                // A Redo restores state (Generative-like).
                // Unlike standard Generative, it doesn't block "older" Primary Undos in the stack model,
                // but for our linear scan, we skip it to find pending Primary Undos?
                // Example: [A, B, Undo(B), Undo(A), Redo(A)] -> State is A. We want to find Undo(B).
                // Redo(A) is skipped. Undo(A) is ineffective. Undo(B) is found.
                continue;
            } else {
                // Generative Action.
                // This blocks any further Redo history. The timeline has diverged or is fresh.
                return null;
            }
        }
        return null;
    }

    /**
     * Performs an undo operation by dispatching an UNDO action targeting the last effective action.
     */
    async undo() {
        if (!this.state.activeGame) {
            return;
        }
        await this.historyManager.undo(this.state.activeGame.actionLog, this.state.isReadOnly);
        this.render();
    }

    async redo() {
        if (!this.state.activeGame) {
            return;
        }
        await this.historyManager.redo(this.state.activeGame.actionLog, this.state.isReadOnly);
        this.render();
    }

    updateUndoRedoUI() {
        const u = document.getElementById('btn-undo');
        const r = document.getElementById('btn-redo');
        if (!u || !r || !this.state.activeGame) {
            return;
        }

        const log = this.state.activeGame.actionLog || [];
        const hasUndo = !!this.historyManager.getUndoTargetId(log);
        const hasRedo = !!this.historyManager.getRedoTargetId(log);

        u.disabled = !hasUndo || this.state.isReadOnly;
        r.disabled = !hasRedo || this.state.isReadOnly;
        u.classList.toggle('opacity-50', u.disabled);
        r.classList.toggle('opacity-50', r.disabled);
    }

    /**
     * Hides the currently active context menu.
     */
    hideContextMenu() {
        this.contextMenuManager.hide();
        // Legacy: column-context-menu might not be the 'active' one tracked by manager yet
        const colMenu = document.getElementById('column-context-menu');
        if (colMenu) {
            colMenu.classList.add('hidden');
        }
    }

    pauseContextMenuTimer() {
        this.contextMenuManager.pauseTimer();
    }

    resumeContextMenuTimer() {
        this.contextMenuManager.resumeTimer();
    }

    positionContextMenu(e) {
        this.contextMenuManager.position(e);
    }
    /**
     * Displays the substitution menu for a player.
     * @param {HTMLElement} el - The element that triggered the menu.
     * @param {MouseEvent} e - The mouse event.
     * @param {string} team - The team ('away' or 'home').
     * @param {number} idx - The index of the player in the roster.
     */
    showSubstitutionMenu(el, e, team, idx) {
        if (this.state.isReadOnly) {
            return;
        }
        this.contextMenuManager.showSubstitutionMenu(e, () => this.openSubstitutionModal(team, idx));
    }

    /**
     * Displays a generic player context menu with options like 'Correct Player'.
     * @param {MouseEvent} e - The mouse event.
     * @param {object} player - The player object for whom the menu is being shown.
     * @param {string} targetType - Identifier for the context (e.g., 'cso-batter').
     */
    showPlayerContextMenu(e, _player, _targetType) {
        if (this.state.isReadOnly) {
            return;
        }
        this.contextMenuManager.showPlayerMenu(e, () => {
            this.correctPlayer();
            this.hideContextMenu();
        });
    }

    /**
     * Displays a context menu for a grid cell, with options relevant to the cell's content.
     * @param {MouseEvent} e - The mouse event.
     * @param {number} batterIdx - The batter index of the cell.
     * @param {string} colId - The column ID of the cell.
     * @param {number} inning - The inning number of the cell.
     */
    showCellContextMenu(e, batterIdx, colId, inning) {
        if (this.state.isReadOnly) {
            return;
        }

        // Store the context of the cell being acted upon
        this.contextMenuTarget = { batterIdx, colId, inning };

        // Option: Set/Unset Inning Lead
        const col = this.state.activeGame.columns.find(c => c.id === colId);
        const team = this.state.activeTeam;
        const isLead = col && col.leadRow && col.leadRow[team] === batterIdx;

        // Get the event data for the current cell
        const currentEventKey = `${team}-${batterIdx}-${colId}`;
        const currentEvent = this.state.activeGame.events[currentEventKey];
        const canMove = currentEvent && currentEvent.outcome;

        this.contextMenuManager.showCellMenu(e, {
            isLead,
            onToggleLead: async() => {
                await this.dispatch({
                    type: ActionTypes.SET_INNING_LEAD,
                    payload: {
                        team,
                        colId,
                        rowId: isLead ? null : batterIdx,
                    },
                });
                this.renderGrid();
            },
            canMove,
            onMovePlay: async() => {
                const team = this.state.activeTeam;
                const roster = this.state.activeGame.roster[team];

                // Prepare options for the target batter modal prompt
                const options = roster.map((slot, idx) => ({
                    value: String(idx), // Store index as string
                    label: `${slot.current.number} - ${slot.current.name}`,
                }));

                const targetBatterIdxStr = await this.modalPromptFn(
                    'Move play to which batter?',
                    String(batterIdx), // Default to current batter
                    options,
                );

                if (targetBatterIdxStr !== null && targetBatterIdxStr !== '') {
                    const targetBatterIdx = parseInt(targetBatterIdxStr, 10);
                    if (!isNaN(targetBatterIdx)) {
                        await this.movePlay(batterIdx, colId, targetBatterIdx, colId);
                    }
                }
            },
        });
    }

    /**
     * Retrieves a combined list of all players (starters and subs) for a team.
     * @param {string} team - The team ('away' or 'home').
     * @returns {Array} Array of player objects {n, u, p?}.
     */
    getAllAvailablePlayers(team) {
        return this.substitutionManager.getAllAvailablePlayers(this.state.activeGame, team);
    }

    /**
     * Opens the substitution modal for a given player.
     * @param {string} team - The team ('away' or 'home') of the player.
     * @param {number} idx - The index of the player in the roster.
     */
    handleSubInput(e) {
        const val = e.target.value;
        const team = this.subTarget ? this.subTarget.team : null;
        if (!team) {
            return;
        }

        const allPlayers = this.getAllAvailablePlayers(team);
        const match = allPlayers.find(s => `${s.number} - ${s.name}` === val);

        if (match) {
            document.getElementById('sub-incoming-num').value = match.number || '';
            document.getElementById('sub-incoming-name').value = match.name || '';
            document.getElementById('sub-incoming-pos').value = match.pos || '';
        }
    }

    openSubstitutionModal(team, idx) {
        this.hideContextMenu();
        this.subTarget = { team, idx };
        const p = this.state.activeGame.roster[team][idx].current;
        document.getElementById('sub-outgoing-name').textContent = p.name;
        document.getElementById('sub-outgoing-num').textContent = p.number;

        const numInput = document.getElementById('sub-incoming-num');
        numInput.value = '';
        document.getElementById('sub-incoming-name').value = '';
        document.getElementById('sub-incoming-pos').value = '';

        // Populate datalist with all available players (subs + starters)
        const datalist = document.getElementById('sub-options');
        const allPlayers = this.getAllAvailablePlayers(team);

        this.substitutionManager.renderSubstitutionOptions(datalist, allPlayers);

        document.getElementById('substitution-modal').classList.remove('hidden');
        numInput.focus();
    }

    /**
     * Closes the substitution modal.
     */
    closeSubstitutionModal() {
        document.getElementById('substitution-modal').classList.add('hidden'); this.subTarget = null;
    }

    async handleSubstitution() {
        if (this.state.isReadOnly || !this.subTarget) {
            return;
        }
        const { team, idx } = this.subTarget;
        const newN = document.getElementById('sub-incoming-name').value.trim();
        const newU = document.getElementById('sub-incoming-num').value.trim();
        const newP = document.getElementById('sub-incoming-pos').value.trim();

        if (!this.validate(newN, 50, 'Player Name') || !this.validate(newU, 10, 'Player Number') || !this.validate(newP, 10, 'Position')) {
            return;
        }

        const allPlayers = this.getAllAvailablePlayers(team);
        const existing = allPlayers.find(p => p.number == newU && p.name == newN);
        const newId = existing && existing.id ? existing.id : generateUUID();

        await this.substitutionManager.handleSubstitution(team, idx, {
            name: newN, number: newU, pos: newP, id: newId,
        });

        this.renderGrid();
        this.closeSubstitutionModal();
    }

    async openEditLineupModal(team) {
        if (this.state.isReadOnly || this.state.activeGame?.status === GameStatusFinal) {
            return;
        }
        this.hideContextMenu();
        await this.lineupManager.getInitialLineupState(this.state.activeGame, team);
        document.getElementById('lineup-team-name').value = this.state.activeGame[team];
        document.getElementById('lineup-team-name').dataset.team = team;
        this.renderLineupLists();
        document.getElementById('edit-lineup-modal').classList.remove('hidden');
    }

    scrapeLineupState() {
        this.lineupManager.scrape({
            starters: document.getElementById('lineup-starters-container'),
            subs: document.getElementById('lineup-subs-container'),
        });
    }

    renderLineupLists() {
        const sCont = document.getElementById('lineup-starters-container');
        const subCont = document.getElementById('lineup-subs-container');
        if (!sCont || !subCont) {
            return;
        }
        sCont.innerHTML = '';
        subCont.innerHTML = '';
        this.lineupManager.lineupState.starters.forEach((p, i) => {
            sCont.appendChild(this.lineupManager.renderRow('starter', i, p, {
                onUpdate: () => this.renderLineupLists(),
                onRemove: (idx, type) => {
                    this.scrapeLineupState();
                    this.lineupManager.removePlayerFromGroup(idx, type);
                    this.renderLineupLists();
                },
            }));
        });
        this.lineupManager.lineupState.subs.forEach((p, i) => {
            subCont.appendChild(this.lineupManager.renderRow('sub', i, p, {
                onUpdate: () => this.renderLineupLists(),
                onRemove: (idx, type) => {
                    this.scrapeLineupState();
                    this.lineupManager.removePlayerFromGroup(idx, type);
                    this.renderLineupLists();
                },
            }));
        });
    }

    addPlayerToGroup(type) {
        this.scrapeLineupState();
        this.lineupManager.addPlayerToGroup(type);
        this.renderLineupLists();
    }

    closeEditLineupModal() {
        document.getElementById('edit-lineup-modal').classList.add('hidden');
    }

    async saveLineup() {
        const team = document.getElementById('lineup-team-name').dataset.team;
        const newName = document.getElementById('lineup-team-name').value;
        if (!this.validate(newName, 50, 'Team Name')) {
            return;
        }
        this.scrapeLineupState();
        await this.lineupManager.save(this.state.activeGame, team, newName);
        this.closeEditLineupModal();
        this.render();
    }

    /**
     * Displays the context menu for a specific column in the grid.
     * @param {MouseEvent} e - The mouse event.
     * @param {string} colId - The ID of the column.
     * @param {number} inning - The inning number of the column.
     */
    showColumnContextMenu(e, colId, inning) {
        if (this.state.isReadOnly) {
            return;
        }

        const team = this.state.activeTeam;
        const hasRecordedData = Object.keys(this.state.activeGame.events).some((key) => {
            const parts = key.split('-');
            if (parts[0] !== team) {
                return false;
            }
            const eventColId = parts.slice(2).join('-');
            if (eventColId !== colId) {
                return false;
            }
            const event = this.state.activeGame.events[key];
            return event && (event.outcome || (event.pitchSequence && event.pitchSequence.length > 0));
        });

        const canRemove = !hasRecordedData;

        this.contextMenuManager.showColumnContextMenu(e, {
            onAdd: () => this.addColFromMenu(),
            onRemove: () => this.removeColFromMenu(),
            canRemove,
        });

        // Store target for use by menu action handlers
        this.contextMenuTarget = { colId, inning };
    }
    /**
     * Adds a new column (sub-inning) to the active game after the currently targeted column.
     */
    async addColFromMenu() {
        if (this.state.isReadOnly) {
            return;
        }
        if (!this.state.activeGame || !this.contextMenuTarget) {
            return;
        }

        const currentInning = this.contextMenuTarget.inning;
        const team = this.state.activeTeam;

        await this.dispatch({
            type: ActionTypes.ADD_COLUMN,
            payload: { targetInning: currentInning, team },
        });

        this.renderGrid();
        this.renderScoreboard();
        this.hideContextMenu();
    }

    /**
     * Removes a column from the active game, along with any associated events.
     * Prevents removal of the first column of an inning or columns with recorded data.
     */
    async removeColFromMenu() {
        if (this.state.isReadOnly) {
            return;
        }
        if (!this.state.activeGame || !this.contextMenuTarget) {
            return;
        }

        const colIdToRemove = this.contextMenuTarget.colId;
        const currentInning = this.contextMenuTarget.inning;
        const team = this.state.activeTeam;

        // Check if there is data for THIS team
        const hasRecordedData = Object.keys(this.state.activeGame.events).some((key) => {
            const parts = key.split('-');
            if (parts[0] !== team) {
                return false;
            }
            const eventColId = parts.slice(2).join('-');
            if (eventColId !== colIdToRemove) {
                return false;
            }
            const event = this.state.activeGame.events[key];
            // Consider occupied if there is an outcome OR any pitches recorded
            return event && (event.outcome || (event.pitchSequence && event.pitchSequence.length > 0));
        });

        if (hasRecordedData) {
            console.warn(`Cannot remove column ${colIdToRemove} as it contains recorded data for ${team}.`);
            this.hideContextMenu();
            return;
        }

        // Check if this is the last visible column for this inning for this team
        const inningColumns = this.state.activeGame.columns.filter(col =>
            col.inning === currentInning && (!col.team || col.team === team),
        );

        if (inningColumns.length <= 1 && inningColumns.some(c => c.id === colIdToRemove)) {
            console.warn(`Cannot remove the last column of inning ${currentInning} for ${team}.`);
            this.hideContextMenu();
            return;
        }

        await this.dispatch({
            type: ActionTypes.REMOVE_COLUMN,
            payload: { colId: colIdToRemove, team },
        });

        this.renderGrid();
        this.renderScoreboard();
        this.hideContextMenu();
    }



    /**
     * Opens the Count-Strike-Out (CSO) modal for a specific plate appearance.
     * @param {number} b - The batter's index in the roster.
     * @param {number} i - The inning number.
     * @param {string} col - The column ID for the plate appearance.
     */
    openCSO(b, i, col) {
        const team = this.state.activeTeam;
        const player = this.state.activeGame.roster[team][b].current;

        this.state.activeCtx = { b, i, col };
        this.syncActiveData();

        const titleEl = document.getElementById('cso-title');
        if (titleEl) {
            titleEl.textContent = `${player.name} (#${player.number})`;
        }
        titleEl.oncontextmenu = (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.showPlayerContextMenu(e, player, 'cso-batter');
        };
        document.getElementById('cso-subtitle').textContent = `Inning ${i}`;

        const defense = team === TeamAway ? TeamHome : TeamAway;

        // Auto-detect pitcher if not set
        if (!this.state.activeGame.pitchers[defense]) {
            const defRoster = this.state.activeGame.roster[defense];
            const pitcher = defRoster.find(p => p.current.p === 'P');
            if (pitcher && pitcher.current) {
                this.state.activeGame.pitchers[defense] = pitcher.current.number || pitcher.current.name;
            }
        }

        document.getElementById('cso-pitcher-num').textContent = this.state.activeGame.pitchers[defense];

        // Reset Runner UI directly (avoid dispatch loop)
        const runnerMenu = document.getElementById('cso-runner-menu');
        if (runnerMenu) {
            runnerMenu.classList.add('hidden');
        }
        const runnerActionView = document.getElementById('cso-runner-action-view');
        if (runnerActionView) {
            runnerActionView.classList.add('hidden');
        }

        this.hideBallInPlay(); // Ensure correct view is shown initially

        // Ensure all CSO sub-views are reset
        const views = ['cso-main-view', 'cso-ball-in-play-view', 'cso-runner-advance-view', 'cso-runner-action-view'];
        views.forEach(id => {
            const el = document.getElementById(id);
            if (el) {
                el.classList.add('hidden');
            }
        });
        document.getElementById('cso-main-view').classList.remove('hidden');

        this.renderCSO(); // Render CSO based on the new activeData

        const cso = document.getElementById('cso-modal');
        if (cso) {
            cso.style.display = ''; // Clear any inline styles that might block visibility
            cso.classList.remove('hidden');
        }
    }

    /**
     * Closes the CSO modal and re-renders the grid and scoreboard.
     */
    closeCSO() {
        document.getElementById('cso-modal').classList.add('hidden');
        this.renderGrid();
        this.renderScoreboard();
    }

    /**
     * Prompts for and changes the active pitcher for the defensive team.
     */
    async changePitcher() {
        const defense = this.state.activeTeam === TeamAway ? TeamHome : TeamAway;
        const current = this.state.activeGame.pitchers[defense];
        const newNum = await this.modalPromptFn('Enter New Pitcher #:', current);
        if (newNum !== null && newNum.trim() !== '') {
            const val = newNum.trim();
            if (!this.validate(val, 50, 'Pitcher Name/Number')) {
                return;
            }
            const action = {
                type: ActionTypes.PITCHER_UPDATE,
                payload: { team: defense, pitcher: val },
            };
            await this.dispatch(action);
            document.getElementById('cso-pitcher-num').textContent = val;
        }
    }

    /**
     * Moves a recorded plate appearance event from one grid cell to another.
     * Used primarily for correcting "Batting Out of Order" scenarios where a play was recorded in the wrong slot.
     * @param {number} sourceBatterIdx - The batter index of the source cell.
     * @param {string} sourceColId - The column ID of the source cell.
     * @param {number} targetBatterIdx - The batter index of the target cell.
     * @param {string} targetColId - The column ID of the target cell.
     */
    async movePlay(sourceBatterIdx, sourceColId, targetBatterIdx, targetColId) {
        const team = this.state.activeTeam;
        const sourceKey = `${team}-${sourceBatterIdx}-${sourceColId}`;
        let finalTargetColId = targetColId;
        let newColumn = null;

        if (!this.state.activeGame.events[sourceKey]) {
            console.warn('Cannot move play: Source cell is empty.');
            return;
        }

        // Determine the sequence of columns for this inning
        const currentInning = parseInt(sourceColId.split('-')[1]);
        const inningColumns = this.state.activeGame.columns.filter(col => col.inning === currentInning);

        // Find the index of the target column in the inning array to start searching from
        let startIndex = inningColumns.findIndex(c => c.id === targetColId);
        if (startIndex === -1) {
            startIndex = 0;
        }

        let foundEmpty = false;

        // Search forward from the target column to find the first empty slot for the target batter
        for (let i = startIndex; i < inningColumns.length; i++) {
            const col = inningColumns[i];
            const key = `${team}-${targetBatterIdx}-${col.id}`;
            const ev = this.state.activeGame.events[key];
            if (!ev || !ev.outcome) {
                finalTargetColId = col.id;
                foundEmpty = true;
                break;
            }
        }

        if (!foundEmpty) {
            // No empty slot found in existing columns, need to create a new one
            let maxSubCol = 0;
            inningColumns.forEach((col) => {
                const parts = col.id.split('-');
                if (parts.length === 3) {
                    maxSubCol = Math.max(maxSubCol, parseInt(parts[2]));
                }
            });
            const newSubColIndex = maxSubCol + 1;
            finalTargetColId = `col-${currentInning}-${newSubColIndex}`;

            newColumn = {
                inning: currentInning,
                id: finalTargetColId,
            };
        }

        const finalTargetKey = `${team}-${targetBatterIdx}-${finalTargetColId}`;

        // Create a copy of the event to move
        const eventToMove = JSON.parse(JSON.stringify(this.state.activeGame.events[sourceKey]));

        // Update the player ID in the event to match the target batter
        const targetPlayer = this.state.activeGame.roster[team][targetBatterIdx].current;
        eventToMove.pId = targetPlayer.id;

        // Dispatch a MOVE_PLAY action
        await this.dispatch({
            type: ActionTypes.MOVE_PLAY,
            payload: {
                sourceKey,
                targetKey: finalTargetKey,
                eventData: eventToMove,
                newActiveBatterIdx: targetBatterIdx,
                newActiveColId: finalTargetColId,
                newColumn: newColumn, // Pass newColumn only if created
            },
        });

        this.renderGrid();
        this.renderScoreboard();
        this.hideContextMenu();
    }

    /**
     * Corrects the player in the currently active plate appearance slot.
     * Used for fixing "Batting Out of Order" errors or unannounced substitutions retroactively.
     */
    async correctPlayer() {
        const team = this.state.activeTeam;
        const currentSlotIdx = this.state.activeCtx.b;
        const currentSlot = this.state.activeGame.roster[team][currentSlotIdx];
        const currentPlayer = currentSlot.current;

        // Get all available players (starters + subs) for the current team
        const allAvailablePlayers = this.getAllAvailablePlayers(team);

        // Prepare options for the modal prompt
        const options = allAvailablePlayers.map(p => ({
            value: p.id,
            label: `${p.number} - ${p.name} ${p.pos ? '(' + p.pos + ')' : ''}`,
        }));

        const selectedPlayerId = await this.modalPromptFn('Select player to put in this slot:', currentPlayer.id, options);

        if (selectedPlayerId && selectedPlayerId !== currentPlayer.id) {
            const selectedPlayer = allAvailablePlayers.find(p => p.id === selectedPlayerId);

            if (selectedPlayer) {
                // Directly update the current player in the roster slot
                // This is a correction, not a formal substitution with history.
                currentSlot.current = { ...selectedPlayer };

                // Update activeData.pId if the current activeData belongs to this slot
                if (this.state.activeData && this.state.activeData.pId === currentPlayer.id) {
                    this.state.activeData.pId = selectedPlayer.id;
                }

                // Update the CSO title immediately
                document.getElementById('cso-title').textContent = `${selectedPlayer.name} (#${selectedPlayer.number})`;

                // Re-render relevant parts of the UI
                this.renderGrid();
                this.renderCSO(); // Re-render CSO to update any player-specific visuals if needed
                await this.saveState(); // Persist the direct state change
            }
        }
    }

    /**
     * Saves the `activeData` (current plate appearance details) to the active game's events.
     */
    async saveActiveData() {
        const k = `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`;
        this.state.activeGame.events[k] = JSON.parse(JSON.stringify(this.state.activeData));
        await this.saveState();
        this.render();
    }

    /**
     * Updates the ball/strike counts in activeData based on the pitch sequence.
     * Synchronous helper for optimistic UI updates.
     */
    updateActiveDataCounts() {
        if (!this.state.activeData || !this.state.activeData.pitchSequence) {
            return;
        }
        let b = 0, s = 0;
        this.state.activeData.pitchSequence.forEach((p) => {
            if (p.type === 'ball') {
                if (b < 4) {
                    b++;
                }
            } else if (p.type === 'strike') {
                if (s < 3) {
                    s++;
                }
            } else if (p.type === 'foul') {
                if (s < 2) {
                    s++;
                }
            }
        });
        this.state.activeData.balls = b;
        this.state.activeData.strikes = s;
    }

    /**
     * Records a pitch (ball, strike, foul) or an immediate out.
     * @param {string} type - The type of pitch ('ball', 'strike', 'out', 'foul').
     * @param {string} [code] - Optional code for the pitch (e.g., 'Called', 'Swinging').
     */
    async recordPitch(type, code) {
        if (this.state.isReadOnly) {
            return;
        }

        if (type === 'out') {
            this.state.pendingBipState = {
                res: 'Out',
                base: '',
                type: code || 'Out',
                seq: [],
            };

            const runners = this.getRunnersOnBase();
            // Penalty outs (BOO) typically don't involve runner advancements.
            // Also, for quick outs, we may want to skip the update screen if intended as a procedural out.
            if (runners.length > 0 && code !== 'BOO') {
                this.showRunnerAdvance(runners, this.state.pendingBipState);
                this.hideBallInPlay();
            }
            else {
                await this.finalizePlay();
            }
            return;
        }

        const defense = this.state.activeTeam === TeamAway ? TeamHome : TeamAway;
        this.state.activeData.pitchSequence.push({ type, code, pitcher: this.state.activeGame.pitchers[defense] });
        this.updateActiveDataCounts();
        this.renderCSO();

        await this.csoManager.recordPitch(this.state, type, code);
        this.syncActiveData();

        if (this.state.activeData.outcome === 'BB') {
            await this.handleForcedAdvance();
        } else if (['K', 'ꓘ'].includes(this.state.activeData.outcome)) {
            // If it's a dropped 3rd strike, don't close CSO yet - wait for play result
            const last = this.state.activeData.pitchSequence[this.state.activeData.pitchSequence.length - 1];
            if (last && last.code === PitchCodeDropped) {
                this.state.bipMode = BiPModeDropped;
                this.showBallInPlay();
                return;
            }
            this.closeCSO();
            const stats = this.getInningStats(this.state.activeTeam, this.state.activeCtx.i);
            if (stats.outs >= 3) {
                const nextBatterIdx = (this.state.activeCtx.b + 1) % this.state.activeGame.roster[this.state.activeTeam].length;
                await this.setNextInningLead(nextBatterIdx);
            }
        }
        this.renderCSO();
    }

    /**
     * Recalculates the ball/strike count and checks for walk or strikeout outcomes.
     */
    async recalcCount() {
        let b = 0, s = 0;
        this.state.activeData.pitchSequence.forEach((p) => {
            if (p.type === PitchTypeBall) {
                if (b < 4) {
                    b++;
                }
            }
            else if (p.type === PitchTypeStrike) {
                if (s < 3) {
                    s++;
                }
            }
            else if (p.type === PitchTypeFoul) {
                if (s < 2) {
                    s++;
                }
            }
        });
        this.state.activeData.balls = b; this.state.activeData.strikes = s;
        if (b >= 4) {
            this.state.activeData.outcome = 'BB';
            this.state.activeData.paths[0] = 1;
            await this.handleForcedAdvance();
        }
        else if (s >= 3) {
            const last = this.state.activeData.pitchSequence[this.state.activeData.pitchSequence.length - 1];
            const isCalled = last && last.type === PitchTypeStrike && last.code === PitchCodeCalled;
            this.state.activeData.outcome = isCalled ? 'ꓘ' : 'K';
            const currentStats = this.getInningStats(this.state.activeTeam, this.state.activeCtx.i, `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`);
            this.state.activeData.outNum = currentStats.outs + 1;
            if (this.state.activeData.outNum > 3) {
                this.state.activeData.outNum = 3;
            }
            this.closeCSO();
            await this.saveActiveData();

            let totalOutsThisInning = 0;
            const inningCols = this.state.activeGame.columns.filter(c => c.inning === this.state.activeCtx.i).map(c => c.id);
            Object.keys(this.state.activeGame.events).forEach((k) => {
                const parts = k.split('-');
                if (parts[0] === this.state.activeTeam) {
                    const colId = parts.slice(2).join('-');
                    if (inningCols.includes(colId)) {
                        const d = this.state.activeGame.events[k];
                        if (d.outNum) {
                            totalOutsThisInning = Math.max(totalOutsThisInning, d.outNum);
                        } // Update to use max
                    }
                }
            });

            if (totalOutsThisInning >= 3) {
                // This condition for closing CSO is now redundant here if we close after every plate appearance end.
                // However, we still need this logic to eventually advance innings.
                // For now, let's unconditionally close CSO after a strikeout, the test expects it.
                const nextBatterIdx = (this.state.activeCtx.b + 1) % this.state.activeGame.roster[this.state.activeTeam].length;
                await this.setNextInningLead(nextBatterIdx);
            }
            // this.closeCSO(); // Removed redundant call
        }
    }

    /**
     * Renders the contents of the CSO modal, including pitch sequence, base paths, and outcome.
     * Dynamically creates SVG elements for bases, paths, and runners.
     */
    renderCSO() {
        if (!this.csoRenderer.zoomContainer) {
            this.csoRenderer.zoomContainer = document.querySelector('.cso-zoom-container');
            this.csoRenderer.bipFieldSvg = document.querySelector('#cso-bip-view .field-svg-keyboard svg');
        }
        this.csoRenderer.render({
            ...this.state,
            runnersOnBase: this.getRunnersOnBase(),
        }, this.state.activeGame);
    }
    /**
     * Shows a long-press context menu with additional options for pitch types or outs.
     * @param {string} type - The type of action ('ball', 'strike', 'out').
     * @param {MouseEvent} e - The mouse event (for positioning).
     */
    showLongPressMenu(type, e) {
        this.contextMenuManager.showLongPressMenu(e, type, (t, c) => this.selectMenuOpt(t, c));
    }

    /**
     * Selects an option from a long-press menu and applies the corresponding action.
     * @param {string} t - The type of action ('ball', 'strike', 'out').
     * @param {string} c - The code for the selected option.
     */
    async selectMenuOpt(t, c) {
        if (c === 'Dropped') {
            this.state.bipMode = BiPModeDropped;
            this.showBallInPlay();
        } else {
            this.recordPitch(t, c);
        }
        this.hideContextMenu();
    }

    /**
     * Closes the currently active submenu.
     */
    closeSubmenu() {
        this.contextMenuManager.hide();
    }

    /**
     * Displays the Ball-in-Play (BIP) view within the CSO modal.
     */
    showBallInPlay() {
        document.getElementById('cso-bip-view').classList.remove('hidden');
        document.getElementById('cso-bip-view').classList.add('flex');

        let bipState = { res: 'Safe', base: '1B', type: 'HIT', seq: [], hitData: null };

        if (this.state.activeData && this.state.activeData.bipState) {
            bipState = JSON.parse(JSON.stringify(this.state.activeData.bipState));
        } else if (this.state.activeData && this.state.activeData.hitData) {
            // Fallback: If only hitData is available (legacy), preserve it
            bipState.hitData = JSON.parse(JSON.stringify(this.state.activeData.hitData));
        }

        this.state.bipState = bipState;

        if (this.state.bipMode === BiPModeDropped) {
            this.state.bipState.type = 'D3';
        }
        // Ensure location mode is off initially
        this.state.isLocationMode = false;
        document.getElementById('btn-loc').classList.remove('active');
        document.querySelector('.field-svg-keyboard').classList.remove('location-mode-active');

        this.updateBiPButtons();
        this.updateSequenceDisplay();
        this.renderCSO(); // Call renderCSO to ensure hitData marker and controls visibility are updated
    }

    /**
     * Hides the Ball-in-Play (BIP) view.
     */
    hideBallInPlay() {
        document.getElementById('cso-bip-view').classList.add('hidden');
        document.getElementById('cso-bip-view').classList.remove('flex');
        this.state.bipMode = 'normal';
    }

    /**
     * Cycles through options for a given Ball-in-Play state property (res, base, type).
     * @param {string} k - The key of the bipState property to cycle.
     */
    cycleBiP(k) {
        this.csoManager.cycleBiP(this.state.bipState, k, this.state.bipMode);
        this.updateBiPButtons();
    }

    /**
     * Updates the text and styling of the BIP action buttons based on `bipState`.
     */
    updateBiPButtons() {
        const s = this.state.bipState;
        const isSafe = s.res === 'Safe';
        const resBtn = document.getElementById('btn-res');

        const resBtnText = document.getElementById('btn-res-text');
        if (resBtnText) {
            resBtnText.textContent = s.res;
        } else {
            resBtn.textContent = s.res;
        }

        // Dynamic coloring
        let cls = 'cycle-btn ';
        if (isSafe || s.type === 'SF' || s.type === 'SH') {
            cls += 'bg-green-600';
        } else if (['DP', 'TP'].includes(s.res)) {
            cls += 'bg-purple-600'; // distinct color for multi-out plays
        } else {
            cls += 'bg-red-600';
        }

        resBtn.className = cls;

        const baseBtnText = document.getElementById('btn-base-text');
        if (baseBtnText) {
            baseBtnText.textContent = s.base;
        } else {
            document.getElementById('btn-base').textContent = s.base;
        }

        const typeBtnText = document.getElementById('btn-type-text');
        if (typeBtnText) {
            typeBtnText.textContent = s.type;
        } else {
            document.getElementById('btn-type').textContent = s.type;
        }
    }

    /**
     * Adds a position key to the BIP sequence.
     * @param {string} p - The position key to add.
     */
    addToSequence(p) {
        this.csoManager.addToSequence(this.state.bipState, p);
        this.updateSequenceDisplay();
    }

    backspaceSequence() {
        this.csoManager.backspaceSequence(this.state.bipState);
        this.updateSequenceDisplay();
    }
    updateSequenceDisplay() {
        document.getElementById('sequence-display').textContent = this.state.bipState.seq.join('-') || '_';
    }

    getBatterId() {
        if (!this.state.activeGame || !this.state.activeTeam) {
            return null;
        }
        const roster = this.state.activeGame.roster[this.state.activeTeam];
        return roster[this.state.activeCtx.b].current.id;
    }

    /**
     * Saves the result of the Ball-in-Play interaction to the active game state.
     * Calculates outcomes (e.g., "1B", "F8"), updates paths, and handles RBI logic.
     * If runners are on base, transitions to the runner advancement screen.
     */
    async commitBiP() {
        if (this.state.isReadOnly) {
            return;
        }
        if (this.state.bipMode === BiPModeDropped) {
            await this.recordPitch(PitchTypeStrike, PitchCodeDropped);
        }

        if (!this.state.bipState) {
            return;
        }

        this.csoManager.applySmartDefaults(this.state.bipState);

        // Store BIP state for final commit (merging with runner advancements)
        this.state.pendingBipState = {
            ...this.state.bipState,
            seq: [...this.state.bipState.seq],
        };

        const runners = this.getRunnersOnBase(); // Check for runners on base

        if (runners.length > 0) {
            this.showRunnerAdvance(runners, this.state.pendingBipState); // Show runner advance screen
            this.hideBallInPlay(); // Hide the Ball-in-Play view
        }
        else {
            // No runners on base, commit immediately
            await this.finalizePlay();
        }
    }

    /**
     * Finalizes the current play by dispatching a merged action to the log.
     * @param {Array<object>} [runnerAdvancements=[]] - List of runner advancements.
     */
    async finalizePlay(runnerAdvancements = []) {
        if (this.state.isReadOnly) {
            return;
        }
        if (!this.state.pendingBipState) {
            return;
        }

        await this.dispatch({
            type: ActionTypes.PLAY_RESULT,
            payload: {
                activeCtx: this.state.activeCtx,
                activeTeam: this.state.activeTeam,
                bipState: this.state.pendingBipState,
                batterId: this.getBatterId(),
                bipMode: this.state.bipMode,
                hitData: this.state.pendingBipState.hitData,
                runnerAdvancements,
            },
        });

        // Cleanup
        this.state.pendingBipState = null;
        this.state.isEditing = false;

        this.syncActiveData();

        this.hideBallInPlay();
        this.closeCSO();
        this.render();

        // Check for 3rd out to set next inning lead
        const stats = this.getInningStats(this.state.activeTeam, this.state.activeCtx.i);
        if (stats.outs >= 3) {
            const nextBatterIdx = (this.state.activeCtx.b + 1) % this.state.activeGame.roster[this.state.activeTeam].length;
            await this.setNextInningLead(nextBatterIdx);
        }
    }

    /**
     * Handles clicks on SVG bases within the CSO modal to toggle path states and open runner menus.
     * @param {MouseEvent} e - The click event.
     * @param {number} idx - The index of the base (0-3 for 1st, 2nd, 3rd, Home).
     */
    handleBaseClick(e, idx) {
        const d = this.state.activeData;
        d.paths[idx] = (d.paths[idx] + 1) % 3;

        if (d.paths[idx] === 2) {
            const svg = e.currentTarget.querySelector('svg');
            if (!svg) {
                console.error('SVG element not found within cso-zoom-container'); return;
            }
            const CTM = svg.getScreenCTM();
            const svgPoint = svg.createSVGPoint();
            svgPoint.x = e.clientX;
            svgPoint.y = e.clientY;
            const cursorPoint = svgPoint.matrixTransform(CTM.inverse());

            const pC = this.pathCoords[idx];
            const x1 = pC.x1, y1 = pC.y1;
            const x2 = pC.x2, y2 = pC.y2;
            const cx = cursorPoint.x, cy = cursorPoint.y;

            const lengthSq = (x2 - x1) ** 2 + (y2 - y1) ** 2;
            let t = 0.5;
            if (lengthSq !== 0) {
                t = ((cx - x1) * (x2 - x1) + (cy - y1) * (y2 - y1)) / lengthSq;
                t = Math.max(0, Math.min(1, t));
            }
            if (!d.outPos) {
                d.outPos = [0.5, 0.5, 0.5, 0.5];
            }
            d.outPos[idx] = t;
        }

        // Sync to activeGame.events to ensure state is consistent before save/render
        // This fixes the issue where getRunnersOnBase sees stale data
        const k = `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`;
        this.state.activeGame.events[k] = JSON.parse(JSON.stringify(d));

        if (d.paths[idx] === 1) {
            this.openRunnerMenu(idx, 'advance');
        }
        else if (d.paths[idx] === 2) {
            this.openRunnerMenu(idx, 'out');
        }
        else {
            document.getElementById('cso-runner-menu').classList.add('hidden');
            // If we reset to 0, we must record it in the log
            let pId = '';
            const roster = this.state.activeGame?.roster?.[this.state.activeTeam];
            if (roster && roster[this.state.activeCtx.b]) {
                pId = roster[this.state.activeCtx.b].current.id;
            }

            this.dispatch({
                type: ActionTypes.MANUAL_PATH_OVERRIDE,
                payload: {
                    key: k,
                    activeCtx: this.state.activeCtx,
                    activeTeam: this.state.activeTeam,
                    data: {
                        paths: d.paths,
                        pathInfo: d.pathInfo || ['', '', '', ''],
                        outPos: d.outPos || [0.5, 0.5, 0.5, 0.5],
                        pId: pId,
                    },
                },
            });
            this.syncActiveData();
        }
        this.renderCSO();
        // this.saveState(); // Removed: handled by dispatch or final action
    }

    /**
     * Opens the runner context menu for a specific base with a given action type.
     * @param {number} idx - The index of the base.
     * @param {string} type - The type of runner action ('advance' or 'out').
     */
    openRunnerMenu(idx, type) {
        this.state.activeBaseIdx = idx;
        this.csoRenderer.openRunnerMenu(idx, type);
    }

    /**
     * Applies a selected runner action to the active plate appearance data.
     * @param {number} idx - The index of the base the action applies to.
     * @param {string} code - The code representing the runner action (e.g., 'SB', 'CR').
     */
    async applyRunnerAction(idx, code) {
        const success = await this.runnerManager.applyRunnerAction(
            this.state,
            idx,
            code,
            (msg, def) => this.modalPromptFn(msg, def),
        );

        if (success) {
            this.syncActiveData();
            document.getElementById('cso-runner-menu').classList.add('hidden');
            this.render(); // Ensure grid/scoreboard update
            this.renderCSO();
        }
    }

    /**
     * Closes the runner context menu and records the manual state change in the action log.
     */
    async closeRunnerMenu() {
        const menu = document.getElementById('cso-runner-menu');
        if (menu) {
            menu.classList.add('hidden');
        }

        // Record the manual change (paths) even without a specific reason
        const k = `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`;
        let pId = '';
        const roster = this.state.activeGame?.roster?.[this.state.activeTeam];
        if (roster && roster[this.state.activeCtx.b]) {
            pId = roster[this.state.activeCtx.b].current.id;
        }

        await this.dispatch({
            type: ActionTypes.MANUAL_PATH_OVERRIDE,
            payload: {
                key: k,
                activeCtx: this.state.activeCtx,
                activeTeam: this.state.activeTeam,
                data: {
                    paths: this.state.activeData.paths,
                    pathInfo: this.state.activeData.pathInfo || ['', '', '', ''],
                    outPos: this.state.activeData.outPos || [0.5, 0.5, 0.5, 0.5],
                    pId: pId,
                },
            },
        });
        this.renderCSO();
    }

    /**
     * Retrieves a list of all runners currently on base.
     * @returns {Array<object>} An array of runner objects, each with idx, name, base, and key.
     */
    getRunnersOnBase() {
        return this.runnerManager.getRunnersOnBase(this.state.activeGame, this.state.activeTeam, this.state.activeCtx);
    }
    /**
     * Renders the list of runners and their potential advance outcomes in the runner advance view.
     */
    showRunnerAdvance(runners, bip) {
        const stats = this.getInningStats(this.state.activeTeam, this.state.activeCtx.i);
        const currentOuts = stats.outs;
        this.state.pendingRunnerState = this.runnerManager.calculateDefaultAdvances(runners, bip, this.state.activeGame.events, currentOuts);
        if (document.getElementById('runner-advance-list')) {
            this.renderRunnerAdvanceList();
            document.getElementById('cso-main-view').classList.add('hidden');
            document.getElementById('cso-runner-advance-view').classList.remove('hidden');
        }
    }

    renderRunnerAdvanceList() {
        this.csoRenderer.renderRunnerAdvanceList(
            this.state.pendingRunnerState,
            (i) => this.cycleRunnerAdvance(i),
            (e, i) => this.showOptionsContextMenu(e, this.getCycleOptions('runnerAdvance', i), (opt) => {
                if (this.state.pendingRunnerState[i]) {
                    this.state.pendingRunnerState[i].outcome = opt;
                    this.renderRunnerAdvanceList();
                }
            }),
        );
    }

    /**
     * Directly sets the outcome for a specific runner and re-renders the list.
     * Used for testing purposes to quickly set runner outcomes.
     * @param {number} i - The index of the runner in `pendingRunnerState`.
     * @param {string} outcome - The desired outcome (e.g., 'Stay', 'Score', 'Out').
     */
    setRunnerOutcome(i, outcome) {
        if (i < 0 || i >= this.state.pendingRunnerState.length) {
            console.error('Invalid runner index:', i);
            return;
        }
        const r = this.state.pendingRunnerState[i];
        r.outcome = outcome;
        this.renderRunnerAdvanceList();
    }

    /**
     * Cycles through the possible advance outcomes for a specific runner.
     * @param {number} i - The index of the runner in `pendingRunnerState`.
     */
    cycleRunnerAdvance(i) {
        const options = this.getCycleOptions('runnerAdvance', i);
        this.runnerManager.cycleAdvance(i, this.state.pendingRunnerState, options);
        this.renderRunnerAdvanceList();
    }

    /**
     * Sets the lead batter for the next inning based on the provided batter index.
     * @param {number} nextBatterIdx - The index of the batter who will lead off the next inning.
     */
    async setNextInningLead(nextBatterIdx) {
        if (!this.state.activeGame) {
            return;
        }

        const currentInning = this.state.activeCtx.i;
        const nextInning = currentInning + 1;

        // Find the first column of the next inning (usually ends in -0)
        let nextInningCol = this.state.activeGame.columns.find(c => c.inning === nextInning && c.id.endsWith('-0'));

        // If not found (e.g., inning hasn't been added yet), we can't set the marker on the grid.
        // Usually, users add innings manually or we could auto-add.
        // For now, only set if column exists.
        if (!nextInningCol) {
            // Try to find ANY column for next inning
            nextInningCol = this.state.activeGame.columns.find(c => c.inning === nextInning);
        }

        if (nextInningCol) {
            await this.dispatch({
                type: ActionTypes.SET_INNING_LEAD,
                payload: {
                    team: this.state.activeTeam,
                    colId: nextInningCol.id,
                    rowId: nextBatterIdx,
                },
            });
            this.renderGrid();
        }
    }

    /**
     * Finalizes the runner movements based on `pendingRunnerState`, updates the game state, and closes the CSO modal.
     */
    async finishTurn() {
        if (this.state.isReadOnly) {
            return;
        }
        const batterId = this.getBatterId();
        const runners = this.state.pendingRunnerState
            .filter(r => r.key)
            .map(r => ({
                key: r.key,
                base: r.base,
                outcome: r.outcome || 'Stay',
            }));

        // Merged Play Result logic
        if (this.state.pendingBipState) {
            await this.finalizePlay(runners);
            return;
        }

        const outcome = this.state.activeData.outcome;
        const eligible = batterId && !outcome.includes('E') && !outcome.includes('DP');

        let outSequencing = 'BatterFirst';
        if (this.state.bipState &&
           ['DP', 'TP'].includes(this.state.bipState.res) &&
           this.state.bipState.type === 'Ground') {
            outSequencing = 'RunnersFirst';
        }

        await this.dispatch({
            type: ActionTypes.RUNNER_ADVANCE,
            payload: {
                runners,
                batterId,
                rbiEligible: eligible,
                outSequencing,
                activeCtx: this.state.activeCtx,
                activeTeam: this.state.activeTeam,
            },
        });

        document.getElementById('cso-runner-advance-view').classList.add('hidden');
        this.closeCSO();

        // Check for 3rd out to set next inning lead (Ball in Play / Post-BIP Runner Action -> Next Batter)
        const stats = this.getInningStats(this.state.activeTeam, this.state.activeCtx.i);
        if (stats.outs >= 3) {
            const nextBatterIdx = (this.state.activeCtx.b + 1) % this.state.activeGame.roster[this.state.activeTeam].length;
            await this.setNextInningLead(nextBatterIdx);
        }
    }

    /**
     * Adds a new inning column to the game.
     */
    /**
     * Adds a new inning (column) to the active game.
     */
    async addInning() {
        if (this.state.isReadOnly) {
            return;
        }
        if (!this.state.activeGame) {
            return;
        }
        await this.dispatch({ type: ActionTypes.ADD_INNING });
        this.render();
    }

    /**
     * Prompts the user to override an inning's score.
     * @param {string} team - The team ('away' or 'home') to edit the score for.
     * @param {number} inning - The inning number.
     */
    async editScore(team, inning) {
        if (!this.state.activeGame || this.state.activeGame.status === GameStatusFinal) {
            return;
        }
        const currentVal = (this.state.activeGame.overrides[team] && this.state.activeGame.overrides[team][inning] !== undefined) ? this.state.activeGame.overrides[team][inning] : '';
        const promptText = `Edit ${team.toUpperCase()} Inning ${inning} Score:`;
        const val = await this.modalPromptFn(promptText, currentVal);

        if (val !== null) {
            if (!this.validate(val, 5, 'Score')) {
                return;
            }

            const action = {
                type: ActionTypes.SCORE_OVERRIDE,
                payload: { team, inning, score: val },
            };

            await this.dispatch(action);
            this.renderScoreboard();
        }
    }

    /**
         * Undoes the last recorded pitch in the active plate appearance.
         */
    async undoPitch() {
        if (!this.state.activeGame || !this.state.activeCtx) {
            return;
        }

        const log = this.state.activeGame.actionLog;
        const ctx = this.state.activeCtx;

        // Use HistoryManager logic to find the last effective PITCH action for this PA
        // We can't access getEffectiveLog directly if it's not exposed, but we can replicate the filter
        // or assume the HistoryManager instance has it (it does).
        const effectiveLog = this.historyManager.getEffectiveLog(log);

        let targetId = null;
        for (let i = effectiveLog.length - 1; i >= 0; i--) {
            const action = effectiveLog[i];
            if (action.type === ActionTypes.PITCH) {
                const pCtx = action.payload.activeCtx;
                if (pCtx && pCtx.i === ctx.i && pCtx.b === ctx.b && pCtx.col === ctx.col && action.payload.activeTeam === this.state.activeTeam) {
                    targetId = action.id;
                    break;
                }
            }
        }

        if (targetId) {
            const dispatchPromise = this.dispatch({ type: ActionTypes.UNDO, payload: { refId: targetId } });
            this.syncActiveData();
            this.renderCSO();
            await dispatchPromise;
        }
    }    /**
     * Clears all recorded data for the current plate appearance.
     */
    async clearAllData() {
        if (await this.modalConfirmFn('Clear?')) {
            // Dispatch the CLEAR_DATA action
            await this.dispatch({
                type: ActionTypes.CLEAR_DATA,
                payload: {
                    activeCtx: this.state.activeCtx,
                    activeTeam: this.state.activeTeam,
                    batterId: this.getBatterId(),
                },
            });

            // Sync activeData with the new state
            const key = `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`;
            if (this.state.activeGame.events[key]) {
                this.state.activeData = JSON.parse(JSON.stringify(this.state.activeGame.events[key]));
            }

            // Close CSO and save state (although dispatch already saves, explicit close triggers re-render)
            this.closeCSO();
        }
    }

    /**
     * Cycles the number of outs recorded for the current plate appearance (0, 1, 2, 3, then back to 0).
     */
    async cycleOutNum() {
        if (!this.state.activeData) {
            return;
        }
        const newOutNum = (this.state.activeData.outNum + 1) % 4;
        const k = `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`;
        await this.dispatch({
            type: ActionTypes.OUT_NUM_UPDATE,
            payload: {
                key: k,
                outNum: newOutNum,
            },
        });
        this.syncActiveData();
        this.renderCSO();
    }

    /**
     * Synchronizes the local activeData buffer with the current game state from the reducer.
     */
    syncActiveData() {
        if (!this.state.activeGame || !this.state.activeCtx) {
            return;
        }
        const k = `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`;
        if (this.state.activeGame.events[k]) {
            this.state.activeData = JSON.parse(JSON.stringify(this.state.activeGame.events[k]));
        } else {
            const team = this.state.activeTeam;
            const b = this.state.activeCtx.b;
            const player = this.state.activeGame.roster[team][b].current;
            this.state.activeData = {
                outcome: '',
                balls: 0,
                strikes: 0,
                outNum: 0,
                paths: [0, 0, 0, 0],
                pathInfo: ['', '', '', ''],
                pitchSequence: [],
                pId: player.id,
            };
        }
    }

    /**
     * Calculates and returns the total outs and runs for a given team in a specific inning.
     * @param {string} team - The team ('away' or 'home').
     * @param {number} inning - The inning number.
     * @param {string} [excludeKey] - Optional event key to exclude from calculations (e.g., for optimistic UI updates).
     * @returns {{outs: number, runs: number}} An object containing the total outs and runs.
     */
    getInningStats(team, inning, excludeKey) {
        let outs = 0, runs = 0;
        const inningCols = this.state.activeGame.columns.filter(col => col.inning === inning).map(col => col.id);

        Object.keys(this.state.activeGame.events).forEach((k) => {
            if (k === excludeKey) {
                return;
            }
            const parts = k.split('-');
            if (parts[0] === team) {
                const colId = parts.slice(2).join('-');
                if (inningCols.includes(colId)) {
                    const d = this.state.activeGame.events[k];
                    // New logic: outNum represents the cumulative out of the inning, so find the max
                    if (d.outNum && d.outNum > outs) {
                        outs = d.outNum;
                    }
                    if (d.paths[3] === 1) {
                        runs++;
                    }
                }
            }
        });
        return { outs, runs };
    }

    /**
     * Aggregates player and inning statistics for the active game.
     * @returns {object} An object containing `playerStats` and `inningStats`.
     */
    calculateStats() {
        if (!this.state.activeGame) {
            return {
                playerStats: {},
                inningStats: {},
                pitcherStats: {},
                score: { away: { R: 0, H: 0, E: 0 }, home: { R: 0, H: 0, E: 0 } },
                currentPA: null,
                innings: { away: {}, home: {} },
                hasAB: { away: {}, home: {} },
            };
        }
        return StatsEngine.calculateGameStats(this.state.activeGame);
    }

    /**
     * Marks the current plate appearance as resulting in one out.
     */
    markCurrentPAAsOut() {
        const currentStats = this.getInningStats(this.state.activeTeam, this.state.activeCtx.i, `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`);
        this.state.activeData.outNum = currentStats.outs + 1;
        if (this.state.activeData.outNum > 3) {
            this.state.activeData.outNum = 3;
        }
    }

    /**
     * Opens the backup modal.
     */
    openBackupModal() {
        document.getElementById('backup-modal').classList.remove('hidden');
    }

    /**
     * Starts the backup process (download).
     */
    async startBackup() {
        const includeGames = document.getElementById('backup-include-games').checked;
        const includeTeams = document.getElementById('backup-include-teams').checked;
        const includeRemote = document.getElementById('backup-remote-data').checked;
        const progressEl = document.getElementById('backup-progress');
        const btnStart = document.getElementById('btn-start-backup');

        if (!includeGames && !includeTeams) {
            this.modalConfirmFn('Please select at least one data type to backup.', { isError: true });
            return;
        }

        progressEl.classList.remove('hidden');
        progressEl.textContent = 'Starting backup...';
        btnStart.disabled = true;

        try {
            const allTeams = await this.db.getAllTeams();
            const stream = await this.backupManager.getBackupStream({
                games: includeGames,
                teams: includeTeams,
                remote: includeRemote,
                gameFilter: (g) => this.hasReadAccess(g, allTeams),
                teamFilter: (t) => this.hasTeamReadAccess(t),
            }, (msg) => {
                progressEl.textContent = msg;
            });

            const date = new Date();
            const dateStr = date.getFullYear() + '-' + String(date.getMonth() + 1).padStart(2, '0') + '-' + String(date.getDate()).padStart(2, '0');
            const filename = `skorekeeper-backup-${dateStr}.jsonl`;

            if (window.showSaveFilePicker) {
                try {
                    const handle = await window.showSaveFilePicker({
                        suggestedName: filename,
                        types: [{ description: 'JSONL File', accept: { 'application/jsonl': ['.jsonl'] } }],
                    });
                    const writable = await handle.createWritable();
                    await stream.pipeTo(writable);
                } catch (err) {
                    if (err.name !== 'AbortError') {
                        throw err; // Re-throw real errors to be caught below
                    }
                    // User aborted, do nothing (or reset UI)
                    progressEl.classList.add('hidden');
                    btnStart.disabled = false;
                    return;
                }
            } else {
                // Fallback for browsers without File System Access API
                // Create a Response to read the stream as a Blob
                const response = new Response(stream);
                const blob = await response.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = filename;
                document.body.appendChild(a);
                a.click();
                window.URL.revokeObjectURL(url);
                document.body.removeChild(a);
            }

            progressEl.textContent = 'Backup Complete!';
            setTimeout(() => {
                document.getElementById('backup-modal').classList.add('hidden');
                progressEl.classList.add('hidden');
                btnStart.disabled = false;
            }, 1000);

        } catch (e) {
            console.error('Backup failed:', e);
            progressEl.textContent = 'Error: ' + e.message;
            btnStart.disabled = false;
        }
    }

    /**
     * Opens the restore modal.
     */
    openRestoreModal() {
        document.getElementById('restore-modal').classList.remove('hidden');
        // Reset state
        document.getElementById('restore-step-1').classList.remove('hidden');
        document.getElementById('restore-step-2').classList.add('hidden');
        document.getElementById('btn-confirm-restore').classList.add('hidden');
        document.getElementById('restore-file-input').value = '';
        document.getElementById('restore-list-container').innerHTML = '';
        this.restoreFile = null;
        this.restoreItems = [];
        this.selectedRestoreIds = new Set();
    }

    /**
     * Handles file selection for restore.
     * @param {Event} e
     */
    async handleRestoreFileSelect(e) {
        const file = e.target.files[0];
        if (!file) {
            return;
        }

        this.restoreFile = file;
        const btn = document.getElementById('btn-select-restore-file');
        const originalText = btn.textContent;
        btn.textContent = 'Scanning File...';
        btn.disabled = true;

        try {
            this.restoreItems = await this.backupManager.scanBackupFile(file);
            this.renderRestoreList();

            document.getElementById('restore-step-1').classList.add('hidden');
            document.getElementById('restore-step-2').classList.remove('hidden');
            document.getElementById('btn-confirm-restore').classList.remove('hidden');
        } catch (error) {
            console.error('Scan failed:', error);
            this.modalConfirmFn('Failed to read backup file. Please ensure it is a valid .jsonl file generated by Skorekeeper.', { isError: true });
            // Reset file input so change event fires again if user selects same file
            document.getElementById('restore-file-input').value = '';
        } finally {
            btn.textContent = originalText;
            btn.disabled = false;
        }
    }

    /**
     * Renders the list of items found in the backup file.
     */
    renderRestoreList() {
        const container = document.getElementById('restore-list-container');
        container.innerHTML = '';

        if (this.restoreItems.length === 0) {
            container.innerHTML = '<div class="p-4 text-center text-gray-500">No restorable items found.</div>';
            return;
        }

        const fragment = document.createDocumentFragment();

        this.restoreItems.forEach(item => {
            const div = document.createElement('div');
            div.className = 'flex items-center gap-3 p-2 bg-white border rounded hover:bg-slate-50';

            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.className = 'w-5 h-5';
            checkbox.dataset.id = item.id;
            checkbox.onchange = (e) => {
                if (e.target.checked) {
                    this.selectedRestoreIds.add(item.id);
                }
                else {
                    this.selectedRestoreIds.delete(item.id);
                }
                this.updateRestoreButton();
            };

            const info = document.createElement('div');
            info.className = 'flex-1';

            if (item.type === 'game') {
                const dateStr = item.summary.date ? new Date(item.summary.date).toLocaleDateString() : 'Unknown Date';
                const line1 = document.createElement('div');
                line1.className = 'font-bold text-sm text-slate-800';
                line1.textContent = `[GAME] ${item.summary.away} vs ${item.summary.home}`;
                const line2 = document.createElement('div');
                line2.className = 'text-xs text-gray-500';
                line2.textContent = `${item.summary.event || 'No Event'} • ${dateStr}`;
                info.appendChild(line1);
                info.appendChild(line2);
            } else if (item.type === 'team') {
                const line1 = document.createElement('div');
                line1.className = 'font-bold text-sm text-blue-800';
                line1.textContent = `[TEAM] ${item.name}`;
                const line2 = document.createElement('div');
                line2.className = 'text-xs text-gray-500';
                line2.textContent = item.shortName || '';
                info.appendChild(line1);
                info.appendChild(line2);
            }

            div.appendChild(checkbox);
            div.appendChild(info);
            fragment.appendChild(div);
        });

        container.appendChild(fragment);
    }

    toggleRestoreSelection(selectAll) {
        const checkboxes = document.getElementById('restore-list-container').querySelectorAll('input[type="checkbox"]');
        checkboxes.forEach(cb => {
            cb.checked = selectAll;
            if (selectAll) {
                this.selectedRestoreIds.add(cb.dataset.id);
            }
            else {
                this.selectedRestoreIds.delete(cb.dataset.id);
            }
        });
        this.updateRestoreButton();
    }

    updateRestoreButton() {
        const btn = document.getElementById('btn-confirm-restore');
        btn.textContent = `Restore Selected (${this.selectedRestoreIds.size})`;
        btn.disabled = this.selectedRestoreIds.size === 0;
        if (btn.disabled) {
            btn.classList.add('opacity-50');
        }
        else {
            btn.classList.remove('opacity-50');
        }
    }

    /**
     * Starts the restore process.
     */
    async startRestore() {
        if (!this.restoreFile || this.selectedRestoreIds.size === 0) {
            return;
        }

        const progressEl = document.getElementById('restore-progress');
        progressEl.classList.remove('hidden');
        const btn = document.getElementById('btn-confirm-restore');
        btn.disabled = true;

        try {
            const result = await this.backupManager.restoreBackup(
                this.restoreFile,
                this.selectedRestoreIds,
                (msg) => progressEl.textContent = msg,
            );

            await this.modalConfirmFn(`Restore Complete!\nSuccess: ${result.success}\nErrors: ${result.errors}`);
            document.getElementById('restore-modal').classList.add('hidden');
            // Refresh views
            this.loadDashboard();
        } catch (e) {
            console.error('Restore failed:', e);
            this.modalConfirmFn('Restore failed: ' + e.message, { isError: true });
        } finally {
            progressEl.classList.add('hidden');
            btn.disabled = false;
        }
    }

    /**
     * Opens the new game creation modal.
     */
    /**
     * Opens the new game creation modal.
     */
    async openNewGameModal() {
        const teamsRaw = await this.db.getAllTeams();
        const teams = teamsRaw.filter(t => this.hasTeamReadAccess(t));
        const awaySelect = document.getElementById('team-away-select');
        const homeSelect = document.getElementById('team-home-select');

        if (awaySelect && homeSelect) {
            const populate = (select) => {
                select.innerHTML = '';
                select.appendChild(createElement('option', { value: '', text: '-- Select Team (Optional) --' }));
                teams.forEach(t => {
                    const opt = document.createElement('option');
                    opt.value = t.id;
                    opt.textContent = t.name;
                    select.appendChild(opt);
                });
            };
            populate(awaySelect);
            populate(homeSelect);
        }

        document.getElementById('new-game-modal').classList.remove('hidden');
    }

    /**
     * Closes the new game creation modal.
     */
    closeNewGameModal() {
        document.getElementById('new-game-modal').classList.add('hidden');
    }

    /**
     * Records an auto-advance outcome (e.g., HBP, IBB).
     * @param {string} code - The code for the auto-advance.
     */
    /**
     * Records an auto-advance outcome (HBP, IBB, CI) by opening the runner advance view.
     * @param {string} code - The outcome code.
     */
    async recordAutoAdvance(code) {
        if (this.state.isReadOnly) {
            return;
        }

        // Standardize outcome codes if needed (e.g. from UI text)
        let type = code;
        if (code.includes('Hit')) {
            type = 'HBP';
        }
        if (code.includes('Walk')) {
            type = 'IBB';
        }
        if (code.includes('Interference')) {
            type = 'CI';
        }

        this.state.pendingBipState = {
            res: 'Safe',
            base: '1B',
            type: type,
            seq: [],
        };

        const runners = this.getRunnersOnBase();
        if (runners.length > 0) {
            this.showRunnerAdvance(runners, this.state.pendingBipState);
        }
        else {
            await this.finalizePlay();
        }
    }

    /**
     * Handles logic for a forced advance (e.g., Walk, HBP).
     * Calculates forced moves and opens the runner advance view locally.
     * @param {string} [type='BB'] - The type of forced advance.
     */
    async handleForcedAdvance(type = 'BB') {
        const runners = this.getRunnersOnBase();
        if (runners.length === 0) {
            this.closeCSO();
            return;
        }

        // Create local BIP state for calculation, but do NOT set pendingBipState
        // This ensures finishTurn uses RUNNER_ADVANCE (legacy flow) instead of PLAY_RESULT
        // preventing Walks from being recorded as Hits in the reducer.
        const bip = {
            res: 'Safe',
            base: '1B',
            type: type,
            seq: [],
        };

        this.showRunnerAdvance(runners, bip);
    }

    /**
     * Toggles the CSO modal into editing mode.
     */
    toggleActionView() {
        this.state.isEditing = true; this.renderCSO();
    }

    /**
     * Opens a modal to edit the RBI attribution for the current play.
     */
    async openRBIEditModal() {
        if (this.state.isReadOnly) {
            return;
        }
        const menuContainer = document.getElementById('runner-menu-options');
        menuContainer.innerHTML = '';

        const header = document.createElement('div');
        header.className = 'text-white font-bold mb-2 text-center';
        header.textContent = 'Credit RBI To:';
        menuContainer.appendChild(header);

        const scrollablePlayerList = document.createElement('div');
        scrollablePlayerList.className = 'max-h-48 overflow-y-auto pr-2'; // Tailwind classes for max height and vertical scroll
        menuContainer.appendChild(scrollablePlayerList);

        const createBtn = (text, id) => {
            const btn = document.createElement('button');
            btn.className = 'text-xs bg-gray-700 p-2 rounded text-white font-bold mb-1 w-full text-left';
            btn.textContent = text;
            btn.onclick = async() => {
                const k = `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`;
                this.dispatch({
                    type: ActionTypes.RBI_EDIT,
                    payload: {
                        key: k,
                        rbiCreditedTo: id,
                    },
                });
                this.syncActiveData();
                document.getElementById('cso-runner-menu').classList.add('hidden');
                this.renderCSO();
            };
            scrollablePlayerList.appendChild(btn); // Append to scrollable list
        };

        // Option: None - should be outside scrollable list
        const noneBtn = document.createElement('button');

        noneBtn.className = 'text-xs bg-red-700 p-2 rounded text-white font-bold mb-1 w-full text-left';
        noneBtn.textContent = 'No RBI';
        noneBtn.onclick = async() => {
            const k = `${this.state.activeTeam}-${this.state.activeCtx.b}-${this.state.activeCtx.col}`;
            this.dispatch({
                type: ActionTypes.RBI_EDIT,
                payload: {
                    key: k,
                    rbiCreditedTo: null,
                },
            });
            this.syncActiveData();
            document.getElementById('cso-runner-menu').classList.add('hidden');
            this.renderCSO();
        };
        menuContainer.appendChild(noneBtn); // Keep outside scrollable list

        // List Lineup
        const team = this.state.activeTeam;
        this.state.activeGame.roster[team].forEach((slot) => {
            const p = slot.current;
            let label = `#${p.number} ${p.name}`;
            if (slot.slot === this.state.activeCtx.b) {
                label += ' (Batter)';
            }
            createBtn(label, p.id);
        });

        document.getElementById('cso-runner-menu').classList.remove('hidden');
    }

    /**
     * Toggles the hit location input mode.
     * @param {MouseEvent} [e] - The click event.
     */
    toggleLocationMode(e) {
        if (e && e.stopPropagation) {
            e.stopPropagation();
        }
        this.state.isLocationMode = !this.state.isLocationMode;
        const btnLoc = document.getElementById('btn-loc');
        const fieldSvgKeyboard = document.querySelector('.field-svg-keyboard');

        if (this.state.isLocationMode) {
            btnLoc.classList.add('active');
            fieldSvgKeyboard.classList.add('location-mode-active');
        } else {
            btnLoc.classList.remove('active');
            fieldSvgKeyboard.classList.remove('location-mode-active');
        }
        // Re-render CSO to update marker and controls visibility
        this.renderCSO();
    }

    /**
     * Handles clicks on the main field SVG area within the BIP view.
     * Delegates to specific handlers based on current mode.
     * @param {MouseEvent} e - The click event.
     */
    handleFieldSvgClick(e) {
        if (this.state.isLocationMode) {
            this.handleFieldClickForLocation(e);
        } else {
            // Clicks directly on the SVG (not on a pos-key) are ignored in fielder mode.
            // pos-key clicks are handled by handlePosKeyClick
        }
    }

    /**
     * Handles clicks on position keys (e.g., '1', '2', '3') on the field SVG.
     * @param {MouseEvent} e - The click event.
     * @param {string} pos - The position number clicked.
     */
    handlePosKeyClick(e, pos) {
        // Stop propagation to prevent handleFieldSvgClick from firing if this was a pos-key click.
        e.stopPropagation();
        if (!this.state.isLocationMode) {
            this.addToSequence(pos);
        }
        // If in location mode, pos-key clicks are ignored, only general field clicks (handleFieldClickForLocation) matter.
    }

    /**
     * Captures click coordinates on the field SVG and records them as hit location.
     * @param {MouseEvent} e - The click event.
     */
    handleFieldClickForLocation(e) {
        const fieldSvgKeyboard = e.currentTarget;
        const svg = fieldSvgKeyboard.querySelector('svg');
        if (!svg) {
            return;
        }

        const location = this.csoManager.calculateHitLocation(e, svg);
        if (!location) {
            return;
        }

        this.state.bipState.hitData = {
            location,
            trajectory: this.state.bipState.hitData?.trajectory || this.getSmartDefaultTrajectory(),
        };

        this.toggleLocationMode();
        this.renderCSO();
    }
    cycleTrajectory() {
        const trajectories = this.getCycleOptions('trajectory');
        let currentTraj = this.state.bipState.hitData?.trajectory || 'Ground';
        const nextIdx = (trajectories.indexOf(currentTraj) + 1) % trajectories.length;
        this.state.bipState.hitData.trajectory = trajectories[nextIdx];
        this.renderCSO(); // Re-render to update the button text
    }

    /**
     * Clears the hit location data and hides related controls.
     */
    clearHitLocation() {
        this.state.bipState.hitData = null;
        this.renderCSO(); // Re-render to hide marker and controls
    }

    getSmartDefaultTrajectory() {
        return this.csoManager.getSmartDefaultTrajectory(this.state.bipState);
    }
    /**
     * Displays the context menu for a game card.
     * @param {MouseEvent} e - The mouse event.
     * @param {string} gameId - The ID of the game.
     */
    showGameContextMenu(e, gameId) {
        this.contextMenuManager.activeContextMenu = document.getElementById('game-context-menu');

        // Store game ID on the menu or a temp property
        this.contextMenuTarget = { gameId };

        const btnEdit = document.getElementById('btn-menu-edit-game');
        btnEdit.onclick = () => {
            this.hideContextMenu();
            this.openEditGameModal(gameId);
        };

        const btnDelete = document.getElementById('btn-menu-delete-game');
        if (btnDelete) {
            btnDelete.onclick = () => {
                this.hideContextMenu();
                this.deleteGame(gameId);
            };
        }

        this.positionContextMenu(e);
        this.resumeContextMenuTimer();
    }

    /**
     * Opens the modal to edit game details.
     * @param {string} gameId - The ID of the game to edit.
     */
    openEditGameModal(gameId) {
        const game = this.state.games.find(g => g.id === gameId);
        if (!game) {
            return;
        }

        document.getElementById('edit-game-id').value = game.id;
        document.getElementById('edit-game-event').value = game.event || '';
        document.getElementById('edit-game-location').value = game.location || '';

        // Date input needs YYYY-MM-DDTHH:mm format
        if (game.date) {
            const d = new Date(game.date);
            // Adjust to local time string for input
            const localIso = new Date(d.getTime() - (d.getTimezoneOffset() * 60000)).toISOString().slice(0, 16);
            document.getElementById('edit-game-date').value = localIso;
        } else {
            document.getElementById('edit-game-date').value = '';
        }

        document.getElementById('edit-team-away').value = game.away || '';
        document.getElementById('edit-team-home').value = game.home || '';

        document.getElementById('edit-game-modal').classList.remove('hidden');
    }

    /**
     * Closes the edit game modal.
     */
    closeEditGameModal() {
        document.getElementById('edit-game-modal').classList.add('hidden');
    }

    /**
     * Saves the changes made in the edit game modal.
     */
    async saveEditedGame() {
        const gameId = document.getElementById('edit-game-id').value;
        const event = document.getElementById('edit-game-event').value;
        const location = document.getElementById('edit-game-location').value;
        const dateVal = document.getElementById('edit-game-date').value;
        const away = document.getElementById('edit-team-away').value;
        const home = document.getElementById('edit-team-home').value;

        const date = dateVal ? new Date(dateVal).toISOString() : new Date().toISOString();

        if (!this.validate(away, 50, 'Away Team') ||
            !this.validate(home, 50, 'Home Team') ||
            !this.validate(event, 100, 'Event Name') ||
            !this.validate(location, 100, 'Location')) {
            return;
        }

        await this.dispatch({
            type: ActionTypes.GAME_METADATA_UPDATE,
            payload: {
                id: gameId,
                event,
                location,
                date,
                away,
                home,
            },
        });

        this.closeEditGameModal();
        this.loadDashboard(); // Refresh list to show changes
    }

    /**
     * Opens the User Manual view.
     * @param {string} [sectionId] - The specific section to scroll to.
     */
    openManual(sectionId) {
        // Initialize viewer on first access
        if (!this.manualViewer) {
            this.manualViewer = new ManualViewer();
        }

        this.state.view = 'manual';
        this.render();

        if (sectionId) {
            this.manualViewer.loadSection(sectionId);
        }

        this.toggleSidebar(false); // Close sidebar if opened via menu
    }

    /**
     * Opens the Player Profile modal for a specific player.
     * @param {string} playerId - The ID of the player to view.
     */
    async openPlayerProfile(playerId) {
        // 1. Find which games this player is in using GameStats
        const allStats = await this.db.getAllGameStats();
        const gameIds = [];
        allStats.forEach(item => {
            if (item.stats && item.stats.playerStats && item.stats.playerStats[playerId]) {
                gameIds.push(item.id);
            }
        });

        // 2. Load and Hydrate those games
        const fullGames = [];
        for (const id of gameIds) {
            const g = await this.db.loadGame(id);
            if (g && g.actionLog) {
                // We need events for spray chart, so we MUST rehydrate
                const hydrated = computeStateFromLog(g.actionLog);
                // Ensure metadata from the shell is preserved (id, date, teams)
                // computeStateFromLog's applyGameStart should handle most, but we merge to be safe.
                fullGames.push({ ...hydrated, ...g, events: hydrated.events });
            }
        }

        // Look up player info fallback
        let playerInfo = null;
        // Check current team context first (most likely source of click)
        if (this.teamController && this.teamController.teamState) {
            const found = this.teamController.teamState.roster?.find(p => p.id === playerId);
            if (found) {
                playerInfo = found;
            }
        }
        // Fallback to searching all teams if not found or no team context
        if (!playerInfo && this.state.teams) {
            for (const t of this.state.teams) {
                const found = t.roster?.find(p => p.id === playerId);
                if (found) {
                    playerInfo = found;
                    break;
                }
            }
        }

        // 3. Render
        if (this.statsRenderer) {
            this.statsRenderer.renderPlayerProfile(playerId, this.state.aggregatedStats, fullGames, playerInfo);
        }
    }

    /**
     * Loads the statistics view by aggregating pre-calculated stats.
     * @async
     */
    async loadStatisticsView() {
        this.state.view = 'statistics';
        window.location.hash = 'stats';

        // Handle pending filter from other views
        if (this.state.pendingStatsFilter && this.state.pendingStatsFilter.teamId) {
            const teamId = this.state.pendingStatsFilter.teamId;
            const searchInput = document.getElementById('stats-search');
            if (searchInput) {
                // Using "team:" prefix to target the team filter specifically
                searchInput.value = `team:${teamId}`;
            }
            // Clear it
            this.state.pendingStatsFilter = null;
        }

        this.render(); // Show loading state or clear previous content

        // Initialize Pull-to-Refresh
        const container = document.getElementById('stats-list-container');
        if (container && !container.dataset.ptrInitialized) {
            new PullToRefresh(container, async() => {
                await this.refreshStatisticsData();
            });
            container.dataset.ptrInitialized = 'true';
        }

        await this.refreshStatisticsData();
    }

    /**
     * Refreshes the statistics data without resetting the view.
     */
    async refreshStatisticsData() {
        try {
            // Always fetch full team list for stats filter to avoid pagination issues
            // and ensure we have the complete list for lookup
            const allTeams = await this.db.getAllTeams();

            // 1. Get all game data
            const games = await this.db.getAllFullGames();
            const accessibleGames = games.filter(g => this.hasReadAccess(g, allTeams));
            this.state.allGames = accessibleGames;

            // 2. Populate Team Filter (in Advanced Panel)
            const teamFilter = document.getElementById('stats-adv-search-team');
            if (teamFilter) {
                const currentVal = teamFilter.value;
                teamFilter.innerHTML = '<option value="">All Teams</option>';

                const accessibleTeams = allTeams.filter(t => this.hasTeamReadAccess(t));
                const options = [];
                const dbTeamNames = new Set();

                // Add DB Teams
                accessibleTeams.forEach(t => {
                    options.push({ value: t.id, label: t.name });
                    dbTeamNames.add(t.name.toLowerCase());
                });

                // Add Ad-hoc Teams
                const adHocNames = new Set();
                accessibleGames.forEach(g => {
                    ['away', 'home'].forEach(side => {
                        const name = g[side];
                        const id = g[side + 'TeamId'];
                        if (name && !id) {
                            adHocNames.add(name);
                        }
                    });
                });

                Array.from(adHocNames).sort().forEach(name => {
                    let label = name;
                    if (dbTeamNames.has(name.toLowerCase())) {
                        label = `${name} (Ad-hoc)`;
                    }
                    options.push({ value: name, label: label });
                });

                options.sort((a, b) => a.label.localeCompare(b.label));

                options.forEach(opt => {
                    const el = document.createElement('option');
                    el.value = opt.value;
                    el.textContent = opt.label;
                    teamFilter.appendChild(el);
                });
                teamFilter.value = currentVal;
            }

            // 3. Get all pre-calculated stats
            const allStats = await this.db.getAllGameStats();
            const statsMap = new Map();
            const accessibleIds = new Set(accessibleGames.map(g => g.id));
            allStats.forEach(s => {
                if (accessibleIds.has(s.id)) {
                    statsMap.set(s.id, s);
                }
            });

            // 4. Identify missing stats (Migration / Repair)
            const missingIds = [];
            accessibleGames.forEach(g => {
                if (!statsMap.has(g.id)) {
                    missingIds.push(g.id);
                }
            });

            // 5. Heal missing stats
            if (missingIds.length > 0) {
                console.log(`Stats: Healing ${missingIds.length} missing game stats...`);
                for (const id of missingIds) {
                    const game = await this.db.loadGame(id);
                    if (game && game.actionLog) {
                        const fullGame = computeStateFromLog(game.actionLog);
                        // Ensure basic metadata is present for the aggregator
                        Object.assign(fullGame, game);

                        const s = StatsEngine.calculateGameStats(fullGame);
                        await this.db.saveGameStats(id, s);
                        statsMap.set(id, { id, stats: s });
                    }
                }
            }

            // 6. Apply Filters via Search Box
            const searchInput = document.getElementById('stats-search');
            const query = searchInput ? searchInput.value : '';
            const parsedQ = parseQuery(query);

            // DashboardController._matchesGame handles event, location, away, home, date.
            // Stats view specifically cares about "Participating Team" (Away OR Home).
            // Dashboard's _matchesGame checks:
            // if (f.key === 'away' && !away.includes(val)) return false;
            // The Stats UI has a "Teams" dropdown which implies "Is this team playing?".
            // If I map the dropdown to "team:Name", my parser might need to handle it.
            // My parser maps keys to specific filters.
            // If I use "team:Name", DashboardController._matchesGame doesn't check "team". It checks "away" and "home".
            // I should implement a specific matcher for stats or reuse Dashboard's if I map "team" to "away" OR "home".
            // But parsedQ is ANDed.
            // If I want to match "Yankees" (either away or home), I can use Free Text.
            // If I use `team:Yankees` in the query, _matchesGame won't find `f.key === 'team'`.
            // So I should implement a local matcher here.

            const matchesStats = (g, q) => {
                // 1. Free Text
                for (const token of q.tokens) {
                    const t = token.toLowerCase();
                    const match = (g.event || '').toLowerCase().includes(t) ||
                                  (g.location || '').toLowerCase().includes(t) ||
                                  (g.away || '').toLowerCase().includes(t) ||
                                  (g.home || '').toLowerCase().includes(t);
                    if (!match) {
                        return false;
                    }
                }
                // 2. Filters
                for (const f of q.filters) {
                    const val = f.value.toLowerCase();
                    if (f.key === 'event' && !(g.event || '').toLowerCase().includes(val)) {
                        return false;
                    }
                    if (f.key === 'location' && !(g.location || '').toLowerCase().includes(val)) {
                        return false;
                    }
                    // "team" key matches either name or ID
                    if (f.key === 'team') {
                        const matchesName = (g.away || '').toLowerCase().includes(val) || (g.home || '').toLowerCase().includes(val);
                        // val is lowercased above. IDs should be compared case-insensitively or assuming lower.
                        // We check direct match for ID.
                        const matchesId = (g.awayTeamId === f.value) || (g.homeTeamId === f.value) || (g.awayTeamId === val) || (g.homeTeamId === val);
                        if (!matchesName && !matchesId) {
                            return false;
                        }
                    }

                    if (f.key === 'away' && !(g.away || '').toLowerCase().includes(val)) {
                        return false;
                    }
                    if (f.key === 'home' && !(g.home || '').toLowerCase().includes(val)) {
                        return false;
                    }

                    if (f.key === 'date') {
                        const d = g.date || '';
                        if (f.operator === '=') {
                            if (!d.startsWith(f.value)) {
                                return false;
                            }
                        }
                        else if (f.operator === '>=') {
                            if (!(d >= f.value)) {
                                return false;
                            }
                        }
                        else if (f.operator === '<=') {
                            if (!(d <= f.value)) {
                                return false;
                            }
                        }
                        else if (f.operator === '>') {
                            if (!(d > f.value)) {
                                return false;
                            }
                        }
                        else if (f.operator === '<') {
                            if (!(d < f.value)) {
                                return false;
                            }
                        }
                        else if (f.operator === '..') {
                            const maxVal = f.maxValue + '~';
                            if (!(d >= f.value && d <= maxVal)) {
                                return false;
                            }
                        }
                    }
                }
                return true;
            };

            const finalFiltered = accessibleGames.filter(g => matchesStats(g, parsedQ));

            const filteredIds = new Set(finalFiltered.map(g => g.id));
            const statsList = Array.from(statsMap.values()).filter(s => filteredIds.has(s.id));

            // Extract team filter for aggregation (if specific team selected)
            // We look for 'team' filter in parsedQ
            const teamFilterF = parsedQ.filters.find(f => f.key === 'team');
            const teamFilterVal = teamFilterF ? teamFilterF.value : '';

            // 7. Aggregate
            this.state.aggregatedStats = StatsEngine.aggregatePrecalculatedStats(statsList, accessibleGames, teamFilterVal);
            this.render();
        } catch (e) {
            console.error('Failed to load statistics:', e);
            await this.modalConfirmFn('Failed to load statistics data.', { isError: true });
        }
    }

    async onExportCSV(stats) {
        const players = stats.players || stats.playerStats || {};
        const pitchers = stats.pitchers || stats.pitcherStats || {};

        let csv = 'Type,Name,G,PA,AB,H,R,RBI,BB,K,HBP,ROE,Fly,Line,Gnd,AVG,OBP,SLG,OPS,IP,ERA,WHIP,Str%,PC\n';

        Object.keys(players).forEach(id => {
            const p = players[id];
            const d = StatsEngine.getDerivedHittingStats(p);
            csv += `Hitter,"${p.name}",${p.games || 1},${p.pa},${p.ab},${p.h},${p.r},${p.rbi},${p.bb},${p.k},${p.hbp},${p.roe},${p.flyouts},${p.lineouts},${p.groundouts},${d.avg},${d.obp},${d.slg},${d.ops},,,,,\n`;
        });

        Object.keys(pitchers).forEach(id => {
            const p = pitchers[id];
            if (p.bf === 0) {
                return;
            }
            const d = StatsEngine.getDerivedPitchingStats(p);
            csv += `Pitcher,"${p.name || 'Unknown'}",${p.games || 1},,,,,,,,,,,,,,,${d.ip},${d.era},${d.whip},${d.strikePct},${(p.pitches || 0) + (p.balls || 0)}\n`;
        });

        const blob = new Blob([csv], { type: 'text/csv' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `skorekeeper-stats-${new Date().toISOString().split('T')[0]}.csv`;
        a.click();
        URL.revokeObjectURL(url);
    }

    async onExportPDF() {
        const stats = this.state.aggregatedStats;
        if (!stats) {
            return;
        }

        // 1. Create a print container
        const existing = document.getElementById('print-view-container');
        if (existing) {
            existing.remove();
        }

        const printContainer = document.createElement('div');
        printContainer.id = 'print-view-container';
        printContainer.className = 'print-only';
        document.body.appendChild(printContainer);

        try {
            // 2. Add Close/Print controls (Screen only)
            const controls = document.createElement('div');
            controls.className = 'no-print p-4 bg-slate-100 border-b flex justify-between items-center sticky top-0 z-[10001]';

            const backBtn = document.createElement('button');
            backBtn.className = 'bg-slate-800 text-white px-4 py-2 rounded font-bold';
            backBtn.textContent = '← Back to App';
            backBtn.onclick = () => {
                document.body.classList.remove('print-preview-active');
                if (printContainer.parentNode) {
                    document.body.removeChild(printContainer);
                }
            };

            const printBtn = document.createElement('button');
            printBtn.className = 'bg-blue-600 text-white px-6 py-2 rounded font-bold shadow-lg';
            printBtn.textContent = 'Print Report';
            printBtn.onclick = () => window.print();

            controls.appendChild(backBtn);
            controls.appendChild(printBtn);
            printContainer.appendChild(controls);

            // 3. Render Header
            const header = document.createElement('div');
            header.className = 'p-8 border-b-4 border-slate-900 bg-white text-slate-900 mb-8';
            header.innerHTML = `
                <h1 class="text-4xl font-black uppercase tracking-tighter">Statistics Report</h1>
                <div class="text-gray-500 font-mono text-sm mt-2">Generated on ${new Date().toLocaleString()}</div>
            `;
            printContainer.appendChild(header);

            // 4. Render Stats using StatsRenderer
            const statsWrapper = document.createElement('div');
            statsWrapper.className = 'p-8 bg-white';
            this.statsRenderer.render(stats, null, { container: statsWrapper, isPrint: false });
            printContainer.appendChild(statsWrapper);

            // 5. Activate Print Preview Mode
            document.body.classList.add('print-preview-active');

            // 6. Trigger print dialog
            setTimeout(() => window.print(), 500);
        } catch (e) {
            console.error('Failed to generate Stats PDF:', e);
            this.modalConfirmFn('Failed to generate printable report.', { isError: true });
        }
    }

    /**
     * Manually rebuilds all statistics from raw action logs.
     */
    async rebuildStatistics() {
        if (!await this.modalConfirmFn('This will re-calculate statistics for ALL games from the raw action logs. This may take a while. Continue?')) {
            return;
        }

        const btn = document.getElementById('btn-rebuild-stats');
        if (btn) {
            btn.textContent = 'Rebuilding...';
            btn.disabled = true;
        }

        try {
            const allGames = await this.db.getAllGames();
            const allTeams = await this.db.getAllTeams();
            const games = allGames.filter(g => this.hasReadAccess(g, allTeams));

            let count = 0;
            for (const g of games) {
                const game = await this.db.loadGame(g.id);
                if (game && game.actionLog) {
                    const fullGame = computeStateFromLog(game.actionLog);
                    Object.assign(fullGame, game);

                    const s = StatsEngine.calculateGameStats(fullGame);
                    await this.db.saveGameStats(g.id, s);
                }
                count++;
                if (count % 10 === 0) {
                    console.log(`Rebuilt ${count}/${games.length}`);
                }
            }
            await this.loadStatisticsView(); // Refresh
            console.log('Statistics rebuild complete.');
        } catch (e) {
            console.error('Rebuild failed:', e);
            await this.modalConfirmFn('Rebuild failed. Check console.', { isError: true });
        } finally {
            if (btn) {
                btn.textContent = 'Rebuild Stats';
                btn.disabled = false;
            }
        }
    }

    /**
     * Renders the statistics view content.
     */
    renderStatistics() {
        if (!this.statsRenderer.container) {
            this.statsRenderer.container = document.getElementById('stats-content');
        }
        this.statsRenderer.render(this.state.aggregatedStats);
    }

    /**
     * Closes the manual and returns to the appropriate view.
     */
    closeManual() {
        if (this.state.activeGame) {
            this.state.view = 'scoresheet';
        } else {
            this.state.view = 'dashboard';
        }
        this.render();
    }

    /**
     * Opens the bug report modal.
     */
    openBugReportModal() {
        document.getElementById('bug-report-description').value = '';
        document.getElementById('bug-report-modal').classList.remove('hidden');
    }

    /**
     * Closes the bug report modal.
     */
    closeBugReportModal() {
        document.getElementById('bug-report-modal').classList.add('hidden');
    }

    /**
     * Gathers app state and errors, compresses them, and triggers a download.
     */
    async downloadBugReport() {
        const description = document.getElementById('bug-report-description').value;
        if (!description.trim()) {
            await this.modalConfirmFn('Please provide a brief description before downloading the report.');
            return;
        }

        const reportData = {
            timestamp: new Date().toISOString(),
            userAgent: navigator.userAgent,
            url: window.location.href,
            description: description,
            errors: window.sk_errors || [],
            appState: {
                view: this.state.view,
                activeTeam: this.state.activeTeam,
                activeCtx: this.state.activeCtx,
                bipMode: this.state.bipMode,
                isReadOnly: this.state.isReadOnly,
                activeGameSummary: this.state.activeGame ? {
                    id: this.state.activeGame.id,
                    status: this.state.activeGame.status,
                    event: this.state.activeGame.event,
                    location: this.state.activeGame.location,
                    away: this.state.activeGame.away,
                    home: this.state.activeGame.home,
                } : null,
                // We include the full active game state if available
                activeGame: this.state.activeGame,
            },
        };

        const jsonString = JSON.stringify(reportData, null, 2);

        try {
            // Compress using browser's native CompressionStream (GZIP)
            const stream = new Blob([jsonString], { type: 'application/json' }).stream();
            const compressedStream = stream.pipeThrough(new CompressionStream('gzip'));
            const response = new Response(compressedStream);
            const blob = await response.blob();

            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `skorekeeper_bug_report_${new Date().getTime()}.json.gz`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            setTimeout(() => URL.revokeObjectURL(url), 100);

            this.closeBugReportModal();
        } catch (error) {
            console.error('Failed to generate bug report:', error);
            await this.modalConfirmFn('An error occurred while generating the report. Please try again.');
        }
    }
}
