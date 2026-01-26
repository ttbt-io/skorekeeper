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

import { DBManager } from '../../frontend/services/dbManager.js';
import { AppController } from '../../frontend/controllers/AppController.js';
import { ScoresheetRenderer } from '../../frontend/renderers/scoresheetRenderer.js';

import * as ReducerModule from '../../frontend/reducer.js';

// Mock global browser APIs
const mockConfirm = jest.fn();
const mockDateNow = jest.fn();
const mockConsoleError = jest.fn();
const mockConsoleWarn = jest.fn();

// Mock DOM elements and APIs
window.structuredClone = (val) => JSON.parse(JSON.stringify(val));

const mockHtml = `
  <div id="dashboard-view" class="hidden">
    <button id="btn-new-game"></button>
    <div id="game-list"></div>
  </div>
  <div id="sidebar-backdrop"></div>
  <div id="app-sidebar">
      <div id="sidebar-auth"></div>
      <button id="sidebar-btn-dashboard"></button>
      <button id="sidebar-btn-teams"></button>
      <div id="sidebar-game-actions" class="hidden">
        <button id="sidebar-btn-view-grid"></button>
        <button id="sidebar-btn-view-feed"></button>
        <button id="sidebar-btn-add-inning"></button>
        <button id="sidebar-btn-end-game"></button>
      </div>
      <div id="sidebar-export-actions" class="hidden">
        <button id="sidebar-btn-export-pdf"></button>
      </div>
  </div>
  <button id="btn-menu-dashboard"></button>
  <button id="btn-menu-scoresheet"></button>

  <div id="scoresheet-view" class="hidden">
    <div id="game-status-indicator" class="hidden"></div>
    <div id="sb-name-away"></div>
    <div id="sb-name-home"></div>
    <div id="tab-away"></div>
    <div id="tab-home"></div>
    <div id="sb-r-away"></div>
    <div id="sb-h-away"></div>
    <div id="sb-e-away"></div>
    <div id="sb-r-home"></div>
    <div id="sb-h-home"></div>
    <div id="sb-e-home"></div>
    <div id="sb-innings-away"></div>
    <div id="sb-innings-home"></div>
    <div id="sb-header-innings"></div>
    <div id="scoresheet-grid"></div>
    <button id="btn-undo"></button>
    <button id="btn-redo"></button>
    <button id="btn-add-inning"></button>
    <button id="btn-back-dashboard"></button>
  </div>
  <div id="manual-view" class="hidden"></div>
  <div id="teams-view" class="hidden"></div>
  <div id="new-game-modal" class="hidden">
    <input id="game-event-input" value="">
    <input id="game-location-input" value="">
    <input id="game-date-input" value="">
    <select id="team-away-select"></select>
    <input id="team-away-input" value="Away Team">
    <input id="pitcher-away-input" value="P1">
    <select id="team-home-select"></select>
    <input id="team-home-input" value="Home Team">
    <input id="pitcher-home-input" value="P2">
    <button id="btn-start-new-game"></button>
    <form id="new-game-form"></form>
  </div>
  <div id="edit-game-modal" class="hidden">
    <form id="edit-game-form"></form>
  </div>
  <div id="cso-modal" class="hidden">
    <div id="cso-title"></div>
    <div id="cso-subtitle"></div>
    <div id="cso-pitcher-num"></div>
    <div id="control-balls"></div>
    <div id="control-strikes"></div>
    <div id="control-outs"></div>
    <div id="pitch-sequence-container"></div>
    <div id="action-area-pitch">
        <button id="btn-runner-actions"></button>
    </div>
    <div id="action-area-recorded"></div>
    <div id="zoom-outcome-text"></div>
    <div id="cso-bip-view"></div>
    <div id="cso-runner-advance-view"></div>
    <div id="runner-menu-options"></div>
    <div id="cso-runner-menu"></div>
    <div id="cso-long-press-submenu">
        <div id="submenu-content"></div>
    </div>
    <div class="cso-zoom-container">
        <svg class="batter-cell-svg" viewBox="0 0 60 60">
            <line id="zoom-fg-1"></line> <rect id="zoom-base-1b"></rect> <text id="zoom-x-1"></text> <text id="zoom-txt-1"></text> <div id="zoom-out-1"></div>
            <line id="zoom-fg-2"></line> <rect id="zoom-base-2b"></rect> <text id="zoom-x-2"></text> <text id="zoom-txt-2"></text> <div id="zoom-out-2"></div>
            <line id="zoom-fg-3"></line> <rect id="zoom-base-3b"></rect> <text id="zoom-x-3"></text> <text id="zoom-txt-3"></text> <div id="zoom-out-3"></div>
            <line id="zoom-fg-4"></line> <rect id="zoom-base-home"></rect> <text id="zoom-x-4"></text> <text id="zoom-txt-4"></text> <div id="zoom-out-4"></div>
        </svg>
    </div>
  </div>
  <div id="player-context-menu" class="hidden"></div>
  <div id="substitution-modal" class="hidden">
    <span id="sub-outgoing-name"></span>
    <span id="sub-outgoing-num"></span>
    <input id="sub-incoming-num">
    <input id="sub-incoming-name">
    <input id="sub-incoming-pos">
    <form id="sub-form"></form>
  </div>
  <div id="conflict-resolution-modal" class="hidden">
    <button id="btn-conflict-overwrite"></button>
    <button id="btn-conflict-fork"></button>
  </div>
  <div id="team-context-menu" class="hidden"></div>
  <div id="column-context-menu" class="hidden"></div>
  <div id="runner-advance-list"></div>
  <div data-app-ready="false"></div>
`;

// In-memory mock for IndexedDB data
const mockDBStoreData = {};

// Mock IndexedDB
const mockIDBObjectStore = {
    get: jest.fn(() => {
        const request = new MockIDBRequest();
        // Trigger success manually in test
        return request;
    }),
    put: jest.fn((game) => {
        mockDBStoreData[game.id] = game;

        const request = new MockIDBRequest();
        return request;
    }),
    getAll: jest.fn(() => {
        const request = new MockIDBRequest();
        return request;
    }),
};
const mockIDBTransaction = {
    objectStore: jest.fn(() => mockIDBObjectStore),
    oncomplete: jest.fn(),
    onerror: jest.fn(),
};

class MockIDBRequest {
    constructor(result) {
        this._result = result;
        this.onsuccess = null;
        this.onerror = null;
        this.onupgradeneeded = null;
        this.target = { result: this._result };
    }

    _triggerSuccess(eventData = { target: { result: this._result } }) {
        if (this.onsuccess) {
            this.onsuccess(eventData);
        }
    }

    _triggerError(error) {
        if (this.onerror) {
            setTimeout(() => this.onerror(error), 0);
        }
    }

    _triggerUpgradeneeded(event) {
        if (this.onupgradeneeded) {
            return new Promise(resolve => setTimeout(() => {
                this.onupgradeneeded(event);
                resolve();
            }, 0));
        }
        return Promise.resolve();
    }

    get result() {
        return this._result;
    }
}

const mockIDBDatabase = {
    transaction: jest.fn(() => mockIDBTransaction),
    objectStoreNames: {
        contains: jest.fn(name => name === 'games' ? false : true), // Default to true, or handle specifically
    },
    createObjectStore: jest.fn(),
};

const mockIndexedDB = {
    open: jest.fn(() => {
        const request = new MockIDBRequest(mockIDBDatabase);
        return request;
    }),
};

// Setup JSDOM environment before each test
beforeEach(() => {
    document.body.innerHTML = mockHtml;
    jest.clearAllMocks();

    window.MockIDBRequest = MockIDBRequest; // Add this line

    window.confirm = mockConfirm;
    Date.now = mockDateNow;
    console.error = mockConsoleError;
    console.warn = mockConsoleWarn;
    window.indexedDB = mockIndexedDB;

    // Reset internal mock states for IndexedDB
    Object.keys(mockDBStoreData).forEach(key => delete mockDBStoreData[key]);
    mockIDBDatabase.objectStoreNames.contains.mockReturnValue(true);

    // Reset hash
    window.location.hash = '';
});

afterAll(() => {
    jest.restoreAllMocks();
});



// --- DBManager Tests ---
describe('DBManager', () => {
    let dbm;
    beforeEach(() => {
        dbm = new DBManager('TestDB', 1);
    });
    afterEach(() => {
    });

    test('open should open IndexedDB and set this.db', async() => {
        const openPromise = dbm.open();
        // Get the request object returned by the mock call inside dbm.open()
        const request = mockIndexedDB.open.mock.results[0].value;
        request._triggerSuccess();
        await openPromise;
        expect(mockIndexedDB.open).toHaveBeenCalledWith('TestDB', 1);
        expect(dbm.db).toBe(mockIDBDatabase);
    });

    test('open should handle upgradeneeded when objectStore does not exist', async() => {
        mockIDBDatabase.objectStoreNames.contains.mockReturnValue(false);
        const openPromise = dbm.open();
        const request = mockIndexedDB.open.mock.results[0].value;
        await request._triggerUpgradeneeded({ target: { result: mockIDBDatabase } }); // Await here
        request._triggerSuccess();
        await openPromise;
        expect(mockIDBDatabase.objectStoreNames.contains).toHaveBeenCalledWith('games');
        expect(mockIDBDatabase.createObjectStore).toHaveBeenCalledWith('games', { keyPath: 'id' });
        expect(dbm.db).toBe(mockIDBDatabase);
    });

    test('saveGame should put game into object store', async() => {
        const game = { id: 'game1', name: 'Test Game', actionLog: [] };
        // Ensure DB is open
        const openPromise = dbm.open();
        const openRequest = mockIndexedDB.open.mock.results[0].value;
        openRequest._triggerSuccess();
        await openPromise;

        const putPromise = dbm.saveGame(game);
        mockIDBTransaction.oncomplete({ target: { result: undefined } });
        await putPromise;

        expect(mockIDBDatabase.transaction).toHaveBeenCalledWith(['games'], 'readwrite');
        // Verify new schema structure
        expect(mockIDBObjectStore.put).toHaveBeenCalledWith({
            id: 'game1',
            name: 'Test Game',
            actionLog: [],
            schemaVersion: 3,
            _dirty: true,
        });
    });

    test('loadGame should get game from object store', async() => {
        const gameId = 'game1';
        const mockGame = { id: gameId, name: 'Loaded Game' };
        mockDBStoreData[gameId] = mockGame;

        const openPromise = dbm.open();
        const openRequest = mockIndexedDB.open.mock.results[0].value;
        openRequest._triggerSuccess();
        await openPromise;

        const loadPromise = dbm.loadGame(gameId);
        const request = mockIDBObjectStore.get.mock.results[0].value;
        request._triggerSuccess({ target: { result: mockGame } });
        const loadedGame = await loadPromise;

        expect(mockIDBDatabase.transaction).toHaveBeenCalledWith(['games'], 'readonly');
        expect(mockIDBObjectStore.get).toHaveBeenCalledWith(gameId);
        expect(loadedGame).toEqual(mockGame);
    });

    test('getAllGames should get all games from object store', async() => {
        const mockGames = [{ id: 'game1' }, { id: 'game2' }];
        mockDBStoreData.game1 = mockGames[0];
        mockDBStoreData.game2 = mockGames[1];

        const openPromise = dbm.open();
        const openRequest = mockIndexedDB.open.mock.results[0].value;
        openRequest._triggerSuccess();
        await openPromise;

        const getAllPromise = dbm.getAllGames();
        const request = mockIDBObjectStore.getAll.mock.results[0].value;
        request._triggerSuccess({ target: { result: mockGames } });
        const allGames = await getAllPromise;

        expect(mockIDBDatabase.transaction).toHaveBeenCalledWith(['games'], 'readonly');
        expect(mockIDBObjectStore.getAll).toHaveBeenCalled();
        expect(allGames).toEqual(mockGames);
    });
});

// --- AppController Tests ---
describe('AppController', () => {
    let app;
    let mockDB;
    let mockModalPrompt;
    let mockModalConfirm;

    beforeEach(async() => {
        jest.clearAllMocks(); // Clear all mocks before each test

        // Declare and reset mocks for modalPrompt and modalConfirm within beforeEach

        // This ensures fresh mocks for each test run.

        mockModalPrompt = jest.fn();

        mockModalConfirm = jest.fn();

        // Set default resolved values for modalPrompt and modalConfirm

        // These can be overridden by individual tests using .mockResolvedValueOnce()

        mockModalPrompt.mockResolvedValue(''); // Default to empty string for prompts

        mockModalConfirm.mockResolvedValue(true); // Default to true for confirms

        // Mock dependencies

        mockDB = {

            open: jest.fn().mockResolvedValue(true),

            saveGame: jest.fn().mockResolvedValue(true),

            loadGame: jest.fn().mockResolvedValue(null),

            getAllGames: jest.fn().mockResolvedValue([]),

            getAllTeams: jest.fn().mockResolvedValue([]),

            getLocalRevisions: jest.fn().mockResolvedValue(new Map()),

        };

        app = new AppController(mockDB, mockModalPrompt, mockModalConfirm);

    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('constructor should use provided dependencies', () => {
        expect(app.db).toBe(mockDB);
    });

    test('init should open DB and load dashboard', async() => {
        jest.spyOn(app, 'render');
        jest.spyOn(app, 'renderGameList').mockImplementation(() => {
        });

        await app.init();
        // Wait for async handleHashChange -> loadDashboard to complete
        await new Promise(resolve => setTimeout(resolve, 0));

        expect(mockDB.open).toHaveBeenCalled();
        expect(mockDB.getAllGames).toHaveBeenCalled();
        expect(app.state.view).toBe('dashboard');
        expect(app.render).toHaveBeenCalled();
        expect(app.renderGameList).toHaveBeenCalled();
    });

    test('createGame should create a new game, save it, and navigate to scoresheet', async() => {
        mockDateNow.mockReturnValue(12345);

        document.getElementById('team-away-input').value = 'Test Away';
        document.getElementById('team-home-input').value = 'Test Home';
        document.getElementById('game-event-input').value = 'Test Event';
        document.getElementById('game-location-input').value = 'Test Location';
        // Date input left empty to test default

        jest.spyOn(app, 'closeNewGameModal').mockImplementation(() => {
        });
        jest.spyOn(app, 'navigateToScoreSheet').mockImplementation(() => {
        });

        await app.createGame();

        expect(mockDB.saveGame).toHaveBeenCalledTimes(1);
        const savedGame = mockDB.saveGame.mock.calls[0][0];
        expect(typeof savedGame.id).toBe('string'); // Assert ID is a string
        expect(savedGame.away).toBe('Test Away');
        expect(savedGame.home).toBe('Test Home');
        expect(savedGame.event).toBe('Test Event');
        expect(savedGame.location).toBe('Test Location');
        expect(savedGame.date).toBeDefined();

        // Verify actionLog is initialized
        expect(savedGame.actionLog).toHaveLength(1);
        expect(savedGame.actionLog[0].type).toBe('GAME_START');

        expect(app.closeNewGameModal).toHaveBeenCalled();
        expect(window.location.hash).toBe(`#game/${savedGame.id}`);
        expect(app.navigateToScoreSheet).not.toHaveBeenCalled();
        expect(app.state.activeGame.id).toBe(savedGame.id);
    });

    test('saveState should call modalConfirmFn with error message if db.saveGame fails', async() => {
        const testGameInitialState = { id: 'test', data: 'some', actionLog: [] };
        app.state.activeGame = { ...testGameInitialState };

        jest.spyOn(app, 'updateUndoRedoUI').mockImplementation(() => {
        });
        jest.spyOn(app, 'updateSaveStatus').mockImplementation(() => {
        });

        const mockError = new Error('DB write failed');
        mockDB.saveGame.mockRejectedValueOnce(mockError); // Simulate DB save failure

        await app.saveState();

        expect(mockDB.saveGame).toHaveBeenCalledTimes(1);
        expect(mockModalConfirm).toHaveBeenCalledWith(
            'CRITICAL ERROR: Data could not be saved! Check console for details. Please do not close this tab until resolved.',
            { autoClose: false, isError: true },
        );
        expect(app.updateUndoRedoUI).toHaveBeenCalled();
        expect(app.updateSaveStatus).toHaveBeenCalled();
    });

    test('undo should append UNDO action targeting the last effective action', async() => {
        const action1 = { type: 'TEST_ACTION_1', id: 'a1', timestamp: 1 };
        const action2 = { type: 'TEST_ACTION_2', id: 'a2', timestamp: 2 };

        // Setup state with two actions
        app.state.activeGame = {
            id: 'game1',
            step: 2,
            actionLog: [action1, action2],
        };

        // Spy on computeStateFromLog to verify it's called
        // We need to return a valid state object
        const computedState = { id: 'game1', step: 1, actionLog: [] };
        // Note: dispatch calls saveState which calls db.saveGame(activeGame)
        // If we mock return value of computeStateFromLog, ensure it has actionLog if needed by saveState
        // But AppController.dispatch ensures log is preserved? No, dispatch sets nextGameState = computedState.
        // So computedState MUST have actionLog attached if we want it preserved.
        // Actually, dispatch implementation:
        // if (action.type === UNDO) nextGameState = computeStateFromLog(log);
        // ... this.state.activeGame = nextGameState;
        // So computeStateFromLog is responsible for returning state WITH log.
        // Let's ensure our mock returns it.
        computedState.actionLog = [action1, action2, { type: 'UNDO', payload: { refId: 'a2' } }]; // Simplified expectation for the mock return

        // We can't easily predict the exact ID/Timestamp of the new UNDO action inside the mock return unless we capture arguments.
        // Better: Spy on the implementation but let it do nothing or return a fixed state,
        // AND check that dispatch was called with the correct UNDO action.

        jest.spyOn(ReducerModule, 'computeStateFromLog').mockReturnValue(computedState);
        jest.spyOn(app, 'render').mockImplementation(() => {
        });
        jest.spyOn(app, 'updateUndoRedoUI').mockImplementation(() => {
        });
        jest.spyOn(app, 'saveState').mockResolvedValue(true);

        await app.undo();

        // Verify UNDO action was appended to the log locally before dispatch passed it?
        // App.undo calls dispatch(undoAction).
        // dispatch pushes to log, then calls computeStateFromLog(log).

        // Let's verify the log passed to computeStateFromLog contains the new UNDO action
        expect(ReducerModule.computeStateFromLog).toHaveBeenCalled();
        const logPassed = ReducerModule.computeStateFromLog.mock.calls[0][0];
        const lastAction = logPassed[logPassed.length - 1];

        expect(lastAction.type).toBe('UNDO');
        expect(lastAction.payload.refId).toBe('a2');

        expect(app.render).toHaveBeenCalled();
        expect(app.saveState).toHaveBeenCalled();
    });

    test('redo should append UNDO action targeting the last effective UNDO action', async() => {
        const action1 = { type: 'TEST_ACTION_1', id: 'a1' };
        const undoAction = { type: 'UNDO', id: 'u1', payload: { refId: 'a1' } };

        app.state.activeGame = {
            id: 'game1',
            actionLog: [action1, undoAction],
        };

        jest.spyOn(ReducerModule, 'computeStateFromLog').mockReturnValue({
            id: 'game1',
            actionLog: [action1, undoAction, { type: 'UNDO', payload: { refId: 'u1' } }],
        });
        jest.spyOn(app, 'render').mockImplementation(() => {
        });
        jest.spyOn(app, 'updateUndoRedoUI').mockImplementation(() => {
        });
        jest.spyOn(app, 'saveState').mockResolvedValue(true);

        await app.redo();

        // Verify computeStateFromLog called with new action
        expect(ReducerModule.computeStateFromLog).toHaveBeenCalled();
        const logPassed = ReducerModule.computeStateFromLog.mock.calls[0][0];
        const lastAction = logPassed[logPassed.length - 1];

        expect(lastAction.type).toBe('UNDO');
        expect(lastAction.payload.refId).toBe('u1'); // Targets the previous Undo
    });

    // Helper for testing if needed, or just mock reducer return values?
    // Actually unit tests for reducer cover logic. Here we test wiring.


    test('editScore should prompt for score, update override, and save state', async() => {
        app.state.activeGame = { overrides: { away: {}, home: {} } };
        jest.spyOn(app, 'saveState').mockResolvedValue(true);
        jest.spyOn(app, 'renderScoreboard').mockImplementation(() => {
        });

        mockModalPrompt.mockResolvedValueOnce('5'); // Simulate entering '5'

        await app.editScore('away', 1);

        expect(mockModalPrompt).toHaveBeenCalledWith('Edit AWAY Inning 1 Score:', '');
        expect(app.state.activeGame.overrides.away[1]).toBe('5');
        expect(app.renderScoreboard).toHaveBeenCalledTimes(1);
        expect(app.saveState).toHaveBeenCalledTimes(1);
    });

    test('handleSubstitution should update player roster and save state', async() => {
        app.state.activeGame = {
            roster: {
                away: [{ slot: 0, starter: { name: 'P1', number: '1' }, current: { name: 'P1', number: '1' }, history: [] }],
            },
            subs: { away: [], home: [] },
        };
        app.subTarget = { team: 'away', idx: 0 };
        document.getElementById('sub-incoming-num').value = '99';
        document.getElementById('sub-incoming-name').value = 'Sub Player';
        document.getElementById('sub-incoming-pos').value = 'SS';

        // Ensure isLiveCSO is false by setting activeCtx to a different slot
        app.state.activeCtx = { b: 1, i: 1, col: 'col-1-0' };

        jest.spyOn(app, 'saveState').mockResolvedValue(true);
        jest.spyOn(app, 'renderGrid').mockImplementation(() => {
        });
        jest.spyOn(app, 'closeSubstitutionModal').mockImplementation(() => {
        });

        await app.handleSubstitution();

        const updatedPlayer = app.state.activeGame.roster.away[0];
        expect(updatedPlayer.current.name).toBe('Sub Player');
        expect(updatedPlayer.current.number).toBe('99');
        expect(app.saveState).toHaveBeenCalledTimes(1);
    });

    test('changePitcher should prompt for new pitcher and save state', async() => {
        app.state.activeGame = { pitchers: { away: 'P1', home: 'P2' } };
        app.state.activeTeam = 'away';

        jest.spyOn(app, 'saveState').mockResolvedValue(true);

        mockModalPrompt.mockResolvedValueOnce('P3'); // Simulate entering 'P3'

        await app.changePitcher();

        expect(mockModalPrompt).toHaveBeenCalledWith('Enter New Pitcher #:', 'P2');
        expect(app.state.activeGame.pitchers.home).toBe('P3');
        expect(app.saveState).toHaveBeenCalledTimes(1);
    });

    test('saveRunnerActions should update runner state (SB) and save', async() => {
        // Setup: R1 on 1st base
        const runnerKey = 'away-0-col-1-0';
        app.state.activeGame = {
            roster: { away: [{ current: { name: 'R1' } }, { current: { name: 'B2' } }] },
            events: {
                [runnerKey]: { paths: [1, 0, 0, 0], pathInfo: ['', '', '', ''], outNum: 0 },
            },
            columns: [{ id: 'col-1-0', inning: 1 }],
        };
        app.state.activeTeam = 'away';
        app.state.activeCtx = { b: 1, i: 1, col: 'col-1-0' };

        // Mock UI state
        app.state.pendingRunnerState = [
            { idx: 0, name: 'R1', base: 0, key: runnerKey, action: 'SB' },
        ];

        jest.spyOn(app, 'saveState').mockResolvedValue(true);
        jest.spyOn(app, 'renderCSO').mockImplementation(() => {
        });
        jest.spyOn(app, 'closeRunnerActionView').mockImplementation(() => {
        });

        // Execute
        await app.saveRunnerActions();

        const evt = app.state.activeGame.events[runnerKey];
        expect(evt.paths[1]).toBe(1); // Safe
        expect(evt.pathInfo[1]).toBe('SB');
        expect(app.saveState).toHaveBeenCalled();
    });

    test('saveRunnerActions should update runner state (CS) and increment outs', async() => {
        const runnerKey = 'away-0-col-1-0';
        app.state.activeGame = {
            roster: { away: [{ current: { name: 'R1' } }, { current: { name: 'B2' } }] },
            events: {
                [runnerKey]: { paths: [1, 0, 0, 0], pathInfo: ['', '', '', ''], outNum: 0 },
            },
            columns: [{ id: 'col-1-0', inning: 1 }],
        };
        app.state.activeTeam = 'away';
        app.state.activeCtx = { b: 1, i: 1, col: 'col-1-0' };

        // Mock UI state
        app.state.pendingRunnerState = [
            { idx: 0, name: 'R1', base: 0, key: runnerKey, action: 'CS' },
        ];

        jest.spyOn(app, 'saveState').mockResolvedValue(true);
        jest.spyOn(app, 'renderCSO').mockImplementation(() => {
        });
        jest.spyOn(app, 'closeRunnerActionView').mockImplementation(() => {
        });

        // Execute
        await app.saveRunnerActions();

        const evt = app.state.activeGame.events[runnerKey];
        expect(evt.paths[1]).toBe(2); // Out
        expect(evt.pathInfo[1]).toBe('CS');
        expect(evt.outNum).toBe(1);
        expect(app.saveState).toHaveBeenCalled();
    });

    test('getRunnersOnBase should correctly identify runners in P1(1B)-P2(Out)-P3(AB) scenario', () => {
    // Scenario:
    // P1 (idx 0): Single (Safe at 1B)
    // P2 (idx 1): Strikeout (Out)
    // P3 (idx 2): At Bat
    // Expected: P1 is returned as a runner on base 0.

        const p1Key = 'away-0-col-1-0';
        const p2Key = 'away-1-col-1-0';

        app.state.activeGame = {
            roster: { away: [
                { current: { name: 'P1' } },
                { current: { name: 'P2' } },
                { current: { name: 'P3' } },
            ] },
            events: {
                [p1Key]: { outcome: '1B', paths: [1, 0, 0, 0], pathInfo: ['', '', '', ''], outNum: 0 },
                [p2Key]: { outcome: 'K', paths: [0, 0, 0, 0], pathInfo: ['', '', '', ''], outNum: 1 },
            },
            columns: [{ id: 'col-1-0', inning: 1 }],
        };
        app.state.activeTeam = 'away';
        app.state.activeCtx = { b: 2, i: 1, col: 'col-1-0' }; // P3 is at bat

        const runners = app.getRunnersOnBase();

        expect(runners.length).toBe(1);
        expect(runners[0].name).toBe('P1');
        expect(runners[0].base).toBe(0); // 1B
        expect(runners[0].key).toBe(p1Key);
    });

    test('switchTeam should update activeTeam and render', () => {
        app.state.activeTeam = 'away';
        jest.spyOn(app, 'renderScoreboard').mockImplementation(() => {
        });
        jest.spyOn(app, 'renderGrid').mockImplementation(() => {
        });

        app.switchTeam('home');

        expect(app.state.activeTeam).toBe('home');
        expect(app.renderScoreboard).toHaveBeenCalled();
        expect(app.renderGrid).toHaveBeenCalled();
    });

    test('renderGameList should display games grouped by date', () => {
        app.state.games = [
            { id: 'g1', date: '2023-10-01T10:00', away: 'A1', home: 'H1', location: 'Loc1' },
            { id: 'g2', date: '2023-10-01T12:00', away: 'A2', home: 'H2', location: 'Loc1' },
            { id: 'g3', date: '2023-10-02T10:00', away: 'A3', home: 'H3', location: 'Loc2' },
        ];
        app.renderGameList();

        const list = document.getElementById('game-list');
        // Should have 2 date headers and 3 game cards
        const headers = list.querySelectorAll('.text-gray-500.font-bold');
        expect(headers.length).toBe(2);
        expect(headers[0].textContent).toContain('2023');

        const cards = list.querySelectorAll('[data-game-id]');
        expect(cards.length).toBe(3);
        // Descending sort: G3 (10/2) -> G2 (10/1 12:00) -> G1 (10/1 10:00)
        expect(cards[0].textContent).toContain('A3 vs H3');
    });



    test('cycleOutNum should cycle outs 0->1->2->3->0', async() => {
        app.state.activeTeam = 'away';
        app.state.activeCtx = { b: 0, col: 'col-1-0', i: 1 };
        const key = 'away-0-col-1-0';
        app.state.activeGame = {
            events: {
                [key]: { outNum: 0, paths: [0, 0, 0, 0], pathInfo: ['', '', '', ''], pitchSequence: [] },
            },
            actionLog: [],
        };
        app.syncActiveData();

        jest.spyOn(app, 'renderCSO').mockImplementation(() => {
        });

        await app.cycleOutNum();
        expect(app.state.activeData.outNum).toBe(1);

        await app.cycleOutNum();
        expect(app.state.activeData.outNum).toBe(2);

        await app.cycleOutNum();
        expect(app.state.activeData.outNum).toBe(3);

        await app.cycleOutNum();
        expect(app.state.activeData.outNum).toBe(0);
    });

    test('clearAllData should reset activeData', async() => {
        app.state.activeCtx = { b: 0, col: 'col-1-0', i: 1 };
        app.state.activeTeam = 'away';

        // Mock activeGame structure required by clearAllData
        app.state.activeGame = {
            events: {
                'away-0-col-1-0': { outcome: '1B', balls: 2, strikes: 1, pId: 'p1' },
            },
            roster: { away: [{ current: { id: 'p1' } }] }, // Mock roster for getBatterId
        };

        app.state.activeData = app.state.activeGame.events['away-0-col-1-0'];

        jest.spyOn(app, 'dispatch').mockImplementation(async(action) => {
            // Simulate reducer update
            if (action.type === 'CLEAR_DATA') {
                app.state.activeGame.events['away-0-col-1-0'] = {
                    outcome: '',
                    balls: 0,
                    strikes: 0,
                    outNum: 0,
                    paths: [0, 0, 0, 0],
                    pathInfo: ['', '', '', ''],
                    pitchSequence: [],
                    pId: 'p1',
                };
            }
        });
        jest.spyOn(app, 'closeCSO').mockImplementation(() => {
        });

        mockModalConfirm.mockResolvedValueOnce(true); // Simulate confirming clearing

        await app.clearAllData();

        expect(mockModalConfirm).toHaveBeenCalledWith('Clear?');
        expect(app.dispatch).toHaveBeenCalledWith(expect.objectContaining({
            type: 'CLEAR_DATA',
            payload: expect.objectContaining({
                activeCtx: app.state.activeCtx,
                activeTeam: 'away',
                batterId: 'p1',
            }),
        }));

        // activeData should be synced from the (mocked) updated state
        expect(app.state.activeData.outcome).toBe('');
        expect(app.state.activeData.balls).toBe(0);
        expect(app.closeCSO).toHaveBeenCalled();
    });

    test('recordAutoAdvance should setup pending state and show runners', async() => {
        app.state.activeGame = {
            columns: [{ id: 'col-1-0', inning: 1 }],
            events: {},
            roster: { away: [], home: [] },
        };
        app.state.activeCtx = { b: 0, i: 1, col: 'col-1-0' };
        app.state.activeTeam = 'away';

        jest.spyOn(app, 'getRunnersOnBase').mockReturnValue([]);
        jest.spyOn(app, 'finalizePlay').mockImplementation(() => {
        });

        await app.recordAutoAdvance('HBP');

        expect(app.state.pendingBipState.type).toBe('HBP');
        expect(app.finalizePlay).toHaveBeenCalled();
    });
    test('explicit out button should dispatch PLAY_RESULT with res: Out', async() => {
        app.state.activeGame = {
            events: {},
            pitchers: { home: '10', away: '20' },
            columns: [{ inning: 1, id: 'col-1-0' }],
            roster: {
                away: [{
                    starter: { id: 'p1', name: 'P1', number: '1' },
                    current: { id: 'p1', name: 'P1', number: '1' },
                    history: [],
                }],
                home: [],
            },
            actionLog: [], // needed for dispatch
        };
        app.state.activeCtx = { b: 0, i: 1, col: 'col-1-0' };
        app.state.activeTeam = 'away'; // Ensure activeTeam is set
        app.state.activeData = { outcome: '', pitchSequence: [], paths: [0, 0, 0, 0] };

        jest.spyOn(app, 'dispatch').mockImplementation(async() => {
        });
        jest.spyOn(app, 'getRunnersOnBase').mockReturnValue([]);

        await app.recordPitch('out');

        expect(app.dispatch).toHaveBeenCalledWith(expect.objectContaining({
            type: 'PLAY_RESULT',
            payload: expect.objectContaining({
                bipState: expect.objectContaining({
                    res: 'Out',
                }),
            }),
        }));
    });

    test('addColFromMenu should add column', async() => {
        app.state.activeGame = { columns: [{ id: 'col-1-0', inning: 1 }] };
        app.contextMenuTarget = { colId: 'col-1-0', inning: 1 };
        jest.spyOn(app, 'saveState').mockImplementation(() => {
        });
        jest.spyOn(app, 'renderGrid').mockImplementation(() => {
        });
        jest.spyOn(app, 'renderScoreboard').mockImplementation(() => {
        });
        jest.spyOn(app, 'hideContextMenu').mockImplementation(() => {
        });

        await app.addColFromMenu();
        expect(app.state.activeGame.columns.length).toBe(2);
        expect(app.state.activeGame.columns[1].id).toBe('col-1-1');
    });

    test('removeColFromMenu should remove empty column (by converting shared to other team)', async() => {
        app.state.activeGame = {
            columns: [{ id: 'col-1-0', inning: 1 }, { id: 'col-1-1', inning: 1 }],
            events: {},
        };
        app.state.activeTeam = 'away';
        app.contextMenuTarget = { colId: 'col-1-1', inning: 1 };
        jest.spyOn(app, 'saveState').mockImplementation(() => {
        });
        jest.spyOn(app, 'renderGrid').mockImplementation(() => {
        });
        jest.spyOn(app, 'renderScoreboard').mockImplementation(() => {
        });
        jest.spyOn(app, 'hideContextMenu').mockImplementation(() => {
        });

        await app.removeColFromMenu();

        // Shared column is not removed from array, but converted to 'home'
        expect(app.state.activeGame.columns.length).toBe(2);
        const col = app.state.activeGame.columns.find(c => c.id === 'col-1-1');
        expect(col.team).toBe('home');
    });

    test('applyRunnerAction with CR should prompt for runner number and save state', async() => {
        // Mock CSO views required by openCSO
        const csoMainView = document.createElement('div');
        csoMainView.id = 'cso-main-view';
        document.body.appendChild(csoMainView);
        const csoRunnerAdvView = document.createElement('div');
        csoRunnerAdvView.id = 'cso-runner-advance-view';
        document.body.appendChild(csoRunnerAdvView);

        const runnerKey = 'away-0-col-1-0';
        app.state.activeGame = {
            roster: { away: [{ current: { name: 'R1' } }] },
            events: {
                [runnerKey]: { paths: [1, 0, 0, 0], pathInfo: ['', '', '', ''], outNum: 0 },
            },
            columns: [{ id: 'col-1-0', inning: 1 }],
            pitchers: { away: 'P1', home: 'P2' }, // Added pitchers
        };
        app.state.activeTeam = 'away';
        app.state.activeCtx = { b: 0, i: 1, col: 'col-1-0' }; // Corrected batter index to 0

        jest.spyOn(app, 'saveState').mockResolvedValue(true);
        jest.spyOn(app, 'renderCSO').mockImplementation(() => {
        });
        jest.spyOn(app, 'closeRunnerMenu').mockImplementation(() => {
        });

        mockModalPrompt.mockResolvedValueOnce('99'); // Simulate entering '99'

        // Manually open CSO to initialize activeData correctly
        app.openCSO(app.state.activeCtx.b, app.state.activeCtx.i, app.state.activeCtx.col);

        await app.applyRunnerAction(0, 'CR');

        expect(mockModalPrompt).toHaveBeenCalledWith('CR #:', '');
        expect(app.state.activeGame.events[runnerKey].pathInfo[0]).toBe('CR 99');
        expect(app.saveState).toHaveBeenCalled();
        expect(app.renderCSO).toHaveBeenCalled();
    });

    test('handleBaseClick logic should cycle paths', () => {
        app.state.activeData = { paths: [0, 0, 0, 0], outPos: [0.5, 0.5, 0.5, 0.5] };
        app.state.activeGame = { events: {} }; // Initialize events
        jest.spyOn(app, 'openRunnerMenu').mockImplementation(() => {
        });
        jest.spyOn(app, 'renderCSO').mockImplementation(() => {
        });
        jest.spyOn(app, 'saveState').mockImplementation(() => {
        });

        const e = { preventDefault: jest.fn(), stopPropagation: jest.fn(), currentTarget: { querySelector: jest.fn() } };

        // 0 -> 1
        app.handleBaseClick(e, 0);
        expect(app.state.activeData.paths[0]).toBe(1);

        // 1 -> 2
        app.handleBaseClick(e, 0);
        expect(app.state.activeData.paths[0]).toBe(2);

        // 2 -> 0
        app.handleBaseClick(e, 0);
        expect(app.state.activeData.paths[0]).toBe(0);
    });

    test('commitBiP should set outcome for Fly Out', async() => {
        app.state.bipState = { res: 'Fly', base: '1B', type: 'F', seq: ['8'] };
        app.state.activeData = { outcome: '', paths: [0, 0, 0, 0] };
        app.state.activeGame = {
            columns: [{ inning: 1, id: 'c' }],
            events: {},
            roster: {
                away: Array(9).fill(0).map((_, i) => ({ current: { id: `p${i}` } })),
                home: Array(9).fill(0).map((_, i) => ({ current: { id: `h${i}` } })),
            },
        };
        app.state.activeCtx = { i: 1, b: 0, col: 'c' };
        app.state.activeTeam = 'away'; // Ensure activeTeam is set

        // Pre-populate event so sync works
        const key = 'away-0-c';
        app.state.activeGame.events[key] = app.state.activeData;

        jest.spyOn(app, 'saveActiveData').mockImplementation(() => {
        });
        jest.spyOn(app, 'getRunnersOnBase').mockReturnValue([]);
        jest.spyOn(app, 'closeCSO').mockImplementation(() => {
        });
        jest.spyOn(app, 'hideBallInPlay').mockImplementation(() => {
        });
        jest.spyOn(app, 'render').mockImplementation(() => {
        }); // Mock render

        await app.commitBiP();

        expect(app.state.activeData.outcome).toBe('F8');
        // outNum logic depends on reducer calculating inning outs.
        // With 0 existing outs, it should be 1.
        expect(app.state.activeData.outNum).toBe(1);
    });

    test('handleForcedAdvance should set pendingRunnerState for forced advance from 1st to 2nd', async() => {
        const key = 'away-0-col-1-0';
        app.state.activeGame = {
            roster: { away: [{ current: { name: 'R1' } }, { current: { name: 'B2' } }] },
            events: {
                [key]: { paths: [1, 0, 0, 0], pathInfo: ['', '', '', ''] },
            },
            columns: [{ id: 'col-1-0', inning: 1 }],
        };
        app.state.activeTeam = 'away';
        app.state.activeCtx = { i: 1, b: 1 }; // B2 is up

        jest.spyOn(app, 'saveActiveData').mockResolvedValue();
        jest.spyOn(app, 'renderRunnerAdvanceList').mockImplementation(() => {
        });
        jest.spyOn(app, 'closeCSO').mockImplementation(() => {
        });

        // Mock getRunnersOnBase to return a runner on 1st
        const runner = { idx: 0, name: 'Runner 1', base: 0, key: 'away-0-col-1-0' };
        jest.spyOn(app, 'getRunnersOnBase').mockReturnValue([runner]);
        app.state.activeGame.events = {
            'away-0-col-1-0': { paths: [1, 0, 0, 0], pitchSequence: [] },
        };

        // Mock document.getElementById for 'cso-runner-advance-view'
        const mockClassList = { remove: jest.fn(), add: jest.fn() };
        // We need to preserve original getElementById for other calls if any, but for this test it's sufficient
        // to mock it for the specific ID.
        const originalGetElementById = document.getElementById.bind(document);
        document.getElementById = jest.fn((id) => {
            if (id === 'cso-runner-advance-view' || id === 'cso-main-view') {
                return { classList: mockClassList };
            }
            return originalGetElementById(id);
        });

        await app.handleForcedAdvance();

        expect(app.state.pendingRunnerState.length).toBe(1);
        expect(app.state.pendingRunnerState[0].outcome).toBe('To 2nd');
        expect(app.renderRunnerAdvanceList).toHaveBeenCalled();
        expect(mockClassList.remove).toHaveBeenCalledWith('hidden');

        document.getElementById = originalGetElementById; // Restore
    });

    test('calculateStats should correctly aggregate player and inning statistics', () => {
        app.state.activeGame = {
            columns: [{ id: 'col-1', inning: 1 }],
            events: {
                'away-0-col-1': { pId: 'p1', outcome: '1B', paths: [1, 1, 1, 1], scoreInfo: { rbiCreditedTo: 'p3' } }, // P1 Single, Scored (driven in by P3)
                'away-1-col-1': { pId: 'p2', outcome: 'K', paths: [0, 0, 0, 0], outNum: 1 }, // P2 Strikeout
                'away-2-col-1': { pId: 'p3', outcome: 'HR', paths: [1, 1, 1, 1], scoreInfo: { rbiCreditedTo: 'p3' }, outNum: 1 }, // P3 Homerun (still 1 out)
                'away-3-col-1': { pId: 'p4', outcome: 'BB', paths: [1, 0, 0, 0], outNum: 1 }, // P4 Walk (LOB)
            },
        };

        const stats = app.calculateStats();

        // P1 Stats: 1B, 1 Run
        expect(stats.playerStats.p1).toEqual({
            ab: 1, r: 1, h: 1, rbi: 0, bb: 0, k: 0, pa: 1, hbp: 0, sf: 0, sh: 0, singles: 1, doubles: 0, triples: 0, hr: 0, sb: 0,
            flyouts: 0, lineouts: 0, groundouts: 0, otherOuts: 0, roe: 0, calledStrikes: 0, name: '', team: '',
        });

        // P2 Stats: K
        expect(stats.playerStats.p2).toEqual({
            ab: 1, r: 0, h: 0, rbi: 0, bb: 0, k: 1, pa: 1, hbp: 0, sf: 0, sh: 0, singles: 0, doubles: 0, triples: 0, hr: 0, sb: 0,
            flyouts: 0, lineouts: 0, groundouts: 0, otherOuts: 0, roe: 0, calledStrikes: 0, name: '', team: '',
        });

        // P3 Stats: HR, 1 Run, 2 RBIs (Self + P1)
        expect(stats.playerStats.p3).toEqual({
            ab: 1, r: 1, h: 1, rbi: 2, bb: 0, k: 0, pa: 1, hbp: 0, sf: 0, sh: 0, singles: 0, doubles: 0, triples: 0, hr: 1, sb: 0,
            flyouts: 0, lineouts: 0, groundouts: 0, otherOuts: 0, roe: 0, calledStrikes: 0, name: '', team: '',
        });

        // P4 Stats: BB, 0 AB, LOB
        expect(stats.playerStats.p4).toEqual({
            ab: 0, r: 0, h: 0, rbi: 0, bb: 1, k: 0, pa: 1, hbp: 0, sf: 0, sh: 0, singles: 0, doubles: 0, triples: 0, hr: 0, sb: 0,
            flyouts: 0, lineouts: 0, groundouts: 0, otherOuts: 0, roe: 0, calledStrikes: 0, name: '', team: '',
        });

        // Inning Stats
        // 2 Runs (P1, P3)
        // 2 Hits (P1, P3)
        // 0 Errors
        // 1 LOB (P4)
        expect(stats.inningStats['away-col-1']).toEqual({ r: 2, h: 2, e: 0, lob: 1, outs: 1, pa: 4 });
    });

    describe('Hit Location Feature', () => {
        beforeEach(() => {
            // Setup minimal activeGame for bipState usage
            app.state.activeGame = {
                columns: [{ inning: 1, id: 'col-1-0' }],
                events: {},
                roster: { away: [{ current: { id: 'p1', name: 'P1', number: '1' } }] },
            };
            app.state.activeCtx = { b: 0, i: 1, col: 'col-1-0' };
            app.state.activeTeam = 'away';
            // Mock the DOM elements for hit location and BIP controls
            document.body.innerHTML += `
                <div id="cso-bip-view">
                    <h3 class="font-bold text-yellow-400 text-center">Result of Play</h3>
                    <div class="flex flex-col flex-1 gap-1">
                        <div class="flex justify-center items-center bg-black rounded p-2 text-yellow-400 font-mono text-xl tracking-widest mb-2 h-12">
                            <span id="sequence-display">_</span>
                            <button class="ml-auto text-red-400 text-sm font-bold px-2" id="btn-backspace">⌫</button>
                        </div>
                        <div class="field-svg-keyboard flex-1 border-2 border-gray-600 rounded relative bg-green-800">
                            <svg viewBox="0 0 200 200" class="absolute inset-0 w-full h-full"></svg>
                            <button id="btn-loc"></button>
                        </div>
                    </div>
                    <div id="div-traj-controls" class="hidden flex gap-2">
                        <button class="cycle-btn flex-1" id="btn-traj"></button>
                        <button class="bg-red-600 hover:bg-red-500 text-white font-bold py-3 px-4 rounded shadow-lg" id="btn-clear-loc"></button>
                    </div>
                    <div class="flex flex-col gap-2">
                        <div class="flex gap-2">
                            <button class="cycle-btn" id="btn-res">SAFE</button>
                            <button class="cycle-btn" id="btn-base">1B</button>
                            <button class="cycle-btn" id="btn-type">HIT</button>
                        </div>
                        <div class="flex gap-2">
                            <button class="bg-gray-600 flex-1 py-3 rounded text-white" id="btn-cancel-bip">Cancel</button>
                            <button class="bg-blue-600 flex-1 py-3 rounded text-white font-bold" id="btn-save-bip">SAVE</button>
                        </div>
                    </div>
                </div>
            `;
            // Re-query elements after innerHTML update
            document.getElementById('btn-loc');
            document.querySelector('.field-svg-keyboard');
            document.getElementById('div-traj-controls');
            document.getElementById('btn-traj');
            document.getElementById('btn-clear-loc');
            document.getElementById('btn-res');
            document.getElementById('btn-base');
            document.getElementById('btn-type');
            document.getElementById('sequence-display');
            document.getElementById('btn-backspace');
            document.getElementById('btn-cancel-bip');
            document.getElementById('btn-save-bip');
        });

        test('toggleLocationMode should update state and UI classes', () => {
            const btnLoc = document.getElementById('btn-loc');
            const fieldSvgKeyboard = document.querySelector('.field-svg-keyboard');

            expect(app.state.isLocationMode).toBe(false);
            expect(btnLoc.classList.contains('active')).toBe(false);
            expect(fieldSvgKeyboard.classList.contains('location-mode-active')).toBe(false);

            app.toggleLocationMode();

            expect(app.state.isLocationMode).toBe(true);
            expect(btnLoc.classList.contains('active')).toBe(true);
            expect(fieldSvgKeyboard.classList.contains('location-mode-active')).toBe(true);

            app.toggleLocationMode();

            expect(app.state.isLocationMode).toBe(false);
            expect(btnLoc.classList.contains('active')).toBe(false);
            expect(fieldSvgKeyboard.classList.contains('location-mode-active')).toBe(false);
        });

        test('handleFieldClickForLocation should record coordinates and toggle mode off', () => {
            app.state.isLocationMode = true; // Manually activate for test
            app.state.bipState = { res: 'Safe', base: '1B', type: 'HIT', seq: [] };
            jest.spyOn(app, 'toggleLocationMode');
            jest.spyOn(app, 'renderCSO');

            const mockSvg = document.querySelector('.field-svg-keyboard svg');
            mockSvg.getBoundingClientRect = () => ({
                left: 0, top: 0, width: 200, height: 200,
                x: 0, y: 0, right: 200, bottom: 200,
            });

            // Mock the necessary SVG methods for JSDOM
            mockSvg.createSVGPoint = jest.fn(() => ({
                x: 0,
                y: 0,
                matrixTransform: jest.fn(_matrix => ({ x: 50, y: 100 })), // Simulate transformed point
            }));
            mockSvg.getScreenCTM = jest.fn(() => ({
                inverse: jest.fn(() => ({})), // No need for actual matrix values for this test
            }));

            // Simulate click at 50, 100
            const mockEvent = {
                clientX: 50,
                clientY: 100,
                currentTarget: document.querySelector('.field-svg-keyboard'),
            };

            app.handleFieldClickForLocation(mockEvent);

            expect(app.state.bipState.hitData).toEqual({
                location: { x: 0.25, y: 0.5 }, // 50/200, 100/200
                trajectory: 'Ground', // Default trajectory
            });
            expect(app.toggleLocationMode).toHaveBeenCalled(); // Should toggle off
            expect(app.renderCSO).toHaveBeenCalled();
        });

        test('cycleTrajectory should cycle through trajectories', () => {
            app.state.bipState = { hitData: { location: { x: 0.5, y: 0.5 }, trajectory: 'Ground' } };
            jest.spyOn(app, 'renderCSO');

            expect(app.state.bipState.hitData.trajectory).toBe('Ground');
            app.cycleTrajectory();
            expect(app.state.bipState.hitData.trajectory).toBe('Line');
            app.cycleTrajectory();
            expect(app.state.bipState.hitData.trajectory).toBe('Fly');
            app.cycleTrajectory();
            expect(app.state.bipState.hitData.trajectory).toBe('Pop');
            app.cycleTrajectory();
            expect(app.state.bipState.hitData.trajectory).toBe('Ground'); // Cycles back
            expect(app.renderCSO).toHaveBeenCalledTimes(4);
        });

        test('clearHitLocation should set hitData to null and re-render', () => {
            app.state.bipState = { hitData: { location: { x: 0.5, y: 0.5 }, trajectory: 'Fly' } };
            jest.spyOn(app, 'renderCSO');

            app.clearHitLocation();

            expect(app.state.bipState.hitData).toBeNull();
            expect(app.renderCSO).toHaveBeenCalled();
        });

        test('getSmartDefaultTrajectory should return correct default based on bipState', () => {
            // Initial state: Ground
            app.state.bipState = { res: 'Safe', type: 'HIT' };
            expect(app.getSmartDefaultTrajectory()).toBe('Ground');

            // Fly -> Fly
            app.state.bipState = { res: 'Fly', type: 'F' };
            expect(app.getSmartDefaultTrajectory()).toBe('Fly');

            // Out -> Ground
            app.state.bipState = { res: 'Out', type: 'OUT' };
            expect(app.getSmartDefaultTrajectory()).toBe('Ground');

            // Line -> Line
            app.state.bipState = { res: 'Line', type: 'OUT' };
            expect(app.getSmartDefaultTrajectory()).toBe('Line');

            // IFF -> Fly
            app.state.bipState = { res: 'IFF', type: 'OUT' };
            expect(app.getSmartDefaultTrajectory()).toBe('Fly');

            // DP/TP Fly -> Fly
            app.state.bipState = { res: 'Fly', type: 'DP' };
            expect(app.getSmartDefaultTrajectory()).toBe('Fly');
            app.state.bipState = { res: 'Fly', type: 'TP' };
            expect(app.getSmartDefaultTrajectory()).toBe('Fly');

            // DP/TP Line -> Line
            app.state.bipState = { res: 'Line', type: 'DP' };
            expect(app.getSmartDefaultTrajectory()).toBe('Line');
            app.state.bipState = { res: 'Line', type: 'TP' };
            expect(app.getSmartDefaultTrajectory()).toBe('Line');
        });

        test('showBallInPlay should initialize bipState.hitData to null and reset location mode', () => {
            app.state.bipState = { res: 'Safe', base: '1B', type: 'HIT', seq: [], hitData: { location: { x: 0.1, y: 0.1 }, trajectory: 'Line' } };
            app.state.isLocationMode = true;
            document.getElementById('btn-loc').classList.add('active');
            document.querySelector('.field-svg-keyboard').classList.add('location-mode-active');
            jest.spyOn(app, 'updateBiPButtons');
            jest.spyOn(app, 'updateSequenceDisplay');
            jest.spyOn(app, 'renderCSO');

            app.showBallInPlay();

            expect(app.state.bipState.hitData).toBeNull();
            expect(app.state.isLocationMode).toBe(false);
            expect(document.getElementById('btn-loc').classList.contains('active')).toBe(false);
            expect(document.querySelector('.field-svg-keyboard').classList.contains('location-mode-active')).toBe(false);
            expect(app.updateBiPButtons).toHaveBeenCalled();
            expect(app.updateSequenceDisplay).toHaveBeenCalled();
            expect(app.renderCSO).toHaveBeenCalled();
        });

        test('commitBiP should dispatch PLAY_RESULT with hitData', async() => {
            app.state.bipState = {
                res: 'Safe',
                base: '1B',
                type: 'HIT',
                seq: ['8'],
                hitData: { location: { x: 0.2, y: 0.3 }, trajectory: 'Ground' },
            };
            app.state.activeCtx = { i: 1, b: 0, col: 'c' };
            app.state.activeTeam = 'away';
            app.state.activeGame = {
                columns: [{ inning: 1, id: 'c' }],
                events: {},
                roster: { away: [{ current: { id: 'p1' } }] },
            };
            const key = 'away-0-c';
            app.state.activeGame.events[key] = { paths: [0, 0, 0, 0] }; // Pre-populate event for state sync

            jest.spyOn(app, 'dispatch');
            jest.spyOn(app, 'getRunnersOnBase').mockReturnValue([]); // No runners for simplicity
            jest.spyOn(app, 'hideBallInPlay').mockImplementation(() => {
            });
            jest.spyOn(app, 'closeCSO').mockImplementation(() => {
            });
            jest.spyOn(app, 'render').mockImplementation(() => {
            });

            await app.commitBiP();

            expect(app.dispatch).toHaveBeenCalledWith(expect.objectContaining({
                type: 'PLAY_RESULT',
                payload: expect.objectContaining({
                    bipState: expect.objectContaining({
                        res: 'Safe',
                        seq: ['8'],
                    }),
                    hitData: { location: { x: 0.2, y: 0.3 }, trajectory: 'Ground' },
                }),
            }));
        });

        test('commitBiP should apply smart defaults for Ground out to SS', async() => {
            app.state.bipState = {
                res: 'Ground',
                base: '1B',
                type: 'OUT',
                seq: ['6', '3'],
                hitData: null,
            };
            app.state.activeCtx = { i: 1, b: 0, col: 'col-1' };
            app.state.activeTeam = 'away';

            jest.spyOn(app, 'dispatch');
            jest.spyOn(app, 'getRunnersOnBase').mockReturnValue([]);
            jest.spyOn(app, 'hideBallInPlay').mockImplementation(() => {
            });
            jest.spyOn(app, 'closeCSO').mockImplementation(() => {
            });
            jest.spyOn(app, 'render').mockImplementation(() => {
            });

            await app.commitBiP();

            expect(app.dispatch).toHaveBeenCalledWith(expect.objectContaining({
                type: 'PLAY_RESULT',
                payload: expect.objectContaining({
                    hitData: {
                        location: { x: 70 / 200, y: 70 / 200 }, // SS canonical position
                        trajectory: 'Ground',
                    },
                }),
            }));
        });
    });

    test('generatePDF should render print view and call window.print', async() => {
        // Setup: Active Game with minimal data
        app.state.activeGame = {
            id: 'game1',
            away: 'Team A',
            home: 'Team B',
            date: '2023-10-01T10:00:00Z',
            columns: [{ id: 'col-1', inning: 1 }],
            events: {},
            roster: {
                away: [{ slot: 0, starter: { id: 'p1', name: 'P1', number: '1' }, current: { id: 'p1', name: 'P1', number: '1' }, history: [] }],
                home: [{ slot: 0, starter: { id: 'h1', name: 'H1', number: '1' }, current: { id: 'h1', name: 'H1', number: '1' }, history: [] }],
            },
        };

        const originalPrint = window.print;
        window.print = jest.fn();

        // Spy on prototype since generatePDF creates new instances
        const gridSpy = jest.spyOn(ScoresheetRenderer.prototype, 'renderGrid');

        await app.generatePDF();

        expect(window.print).toHaveBeenCalledTimes(1);
        expect(gridSpy).toHaveBeenCalledTimes(2); // Once for Away, once for Home
        expect(gridSpy).toHaveBeenCalledWith(expect.any(Object), 'away', expect.any(Object), null, { isPrint: true });
        expect(gridSpy).toHaveBeenCalledWith(expect.any(Object), 'home', expect.any(Object), null, { isPrint: true });

        // Verify preview mode
        expect(document.body.classList.contains('print-preview-active')).toBe(true);
        const container = document.getElementById('print-view-container');
        expect(container).not.toBeNull();

        // Simulate closing the preview
        const backBtn = Array.from(container.querySelectorAll('button')).find(b => b.textContent.includes('Back to App'));
        expect(backBtn).toBeDefined();
        backBtn.click();

        // Verify cleanup
        expect(document.body.classList.contains('print-preview-active')).toBe(false);
        expect(document.getElementById('print-view-container')).toBeNull();

        gridSpy.mockRestore();
        window.print = originalPrint; // Restore
    });
});
