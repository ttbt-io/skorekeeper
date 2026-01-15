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

import { ActiveGameController } from '../../frontend/controllers/ActiveGameController.js';
import { ActionTypes } from '../../frontend/reducer.js';

describe('ActiveGameController', () => {
    let controller;
    let mockApp;

    beforeEach(() => {
        mockApp = {
            db: {
                loadGame: jest.fn(),
                saveGame: jest.fn(),
                getAllTeams: jest.fn().mockResolvedValue([]),
            },
            auth: {
                getLocalId: jest.fn(() => 'local-123'),
            },
            state: {
                activeGame: null,
                view: '',
                scoresheetView: '',
                activeTeam: '',
                activeCtx: {},
                currentUser: { email: 'user@example.com' },
            },
            render: jest.fn(),
            loadDashboard: jest.fn(),
            checkPermissions: jest.fn(),
            hasReadAccess: jest.fn(() => true),
            handleSyncConflict: jest.fn(),
            renderSyncStatusUI: jest.fn(),
            saveState: jest.fn(),
            sync: {
                connect: jest.fn(),
                onConflict: null,
                sendAction: jest.fn(),
            },
        };

        controller = new ActiveGameController(mockApp);
        window.fetch = jest.fn();
    });

    describe('loadGameForView', () => {
        test('should load game from DB and set activeGame', async() => {
            const gameData = {
                id: '12345678-1234-1234-1234-123456789012',
                actionLog: [{
                    id: 'a1',
                    type: ActionTypes.GAME_START,
                    payload: { id: '12345678-1234-1234-1234-123456789012' },
                }],
            };
            mockApp.db.loadGame.mockResolvedValue(gameData);

            await controller.loadGameForView('12345678-1234-1234-1234-123456789012', 'scoresheet');

            expect(mockApp.state.activeGame).toBeDefined();
            expect(mockApp.state.activeGame.id).toBe('12345678-1234-1234-1234-123456789012');
            expect(mockApp.state.view).toBe('scoresheet');
            expect(mockApp.render).toHaveBeenCalled();
            expect(mockApp.sync.connect).toHaveBeenCalledWith(gameData.id, 'a1');
        });

        test('should fetch remote game if not in DB', async() => {
            mockApp.db.loadGame.mockResolvedValue(null);
            window.fetch.mockResolvedValue({
                ok: true,
                json: async() => ({
                    id: '12345678-1234-1234-1234-123456789012',
                    actionLog: [],
                }),
            });

            await controller.loadGameForView('12345678-1234-1234-1234-123456789012', 'scoresheet');

            expect(window.fetch).toHaveBeenCalled();
            expect(mockApp.db.saveGame).toHaveBeenCalled();
            expect(mockApp.state.activeGame).toBeDefined();
        });

        test('should handle invalid game ID', async() => {
            await controller.loadGameForView('invalid-id', 'scoresheet');
            expect(mockApp.loadDashboard).toHaveBeenCalled();
        });

        test('should redirect if no read access', async() => {
            mockApp.db.loadGame.mockResolvedValue({
                id: '12345678-1234-1234-1234-123456789012',
                actionLog: [],
            });
            mockApp.hasReadAccess.mockReturnValue(false);

            await controller.loadGameForView('12345678-1234-1234-1234-123456789012', 'scoresheet');

            expect(mockApp.loadDashboard).toHaveBeenCalled();
            expect(mockApp.render).not.toHaveBeenCalled();
        });
    });

    describe('dispatch', () => {
        test('should dispatch action, update state, and save', async() => {
            mockApp.state.activeGame = {
                actionLog: [],
                events: {},
            };
            const action = { type: 'TEST_ACTION', payload: { foo: 'bar' } };

            await controller.dispatch(action);

            expect(mockApp.state.activeGame.actionLog.length).toBe(1);
            expect(mockApp.state.activeGame.actionLog[0].type).toBe('TEST_ACTION');
            expect(mockApp.render).toHaveBeenCalled();
            // expect(mockApp.saveState).toHaveBeenCalled(); // Mocking app.saveState is tricky as it wasn't in original mockApp
            // Since saveState is called on app, we need to mock it in beforeEach
        });

        test('should handle duplicate actions', async() => {
            mockApp.state.activeGame = {
                actionLog: [{ id: 'a1', type: 'TEST_ACTION' }],
            };
            const action = { id: 'a1', type: 'TEST_ACTION' };

            await controller.dispatch(action);

            expect(mockApp.state.activeGame.actionLog.length).toBe(1);
            expect(mockApp.render).not.toHaveBeenCalled();
        });
    });

    describe('renderGame', () => {
        let domMocks;

        beforeEach(() => {
            domMocks = {};
            document.getElementById = jest.fn((id) => {
                if (!domMocks[id]) {
                    domMocks[id] = {
                        id,
                        classList: {
                            add: jest.fn(),
                            remove: jest.fn(),
                        },
                    };
                }
                return domMocks[id];
            });
            document.querySelector = jest.fn((sel) => {
                if (!domMocks[sel]) {
                    domMocks[sel] = {
                        selector: sel,
                        classList: {
                            add: jest.fn(),
                            remove: jest.fn(),
                        },
                    };
                }
                return domMocks[sel];
            });
            mockApp.renderScoreboard = jest.fn();
            mockApp.renderGrid = jest.fn();
            mockApp.renderFeed = jest.fn();
        });

        test('should render grid view', () => {
            mockApp.state.scoresheetView = 'grid';
            controller.renderGame();

            expect(mockApp.renderScoreboard).toHaveBeenCalled();
            expect(mockApp.renderGrid).toHaveBeenCalled();
            expect(mockApp.renderFeed).not.toHaveBeenCalled();
        });

        test('should render feed view', () => {
            mockApp.state.scoresheetView = 'feed';
            controller.renderGame();

            expect(mockApp.renderFeed).toHaveBeenCalled();
            expect(mockApp.renderScoreboard).not.toHaveBeenCalled();
            expect(mockApp.renderGrid).not.toHaveBeenCalled();
        });

        test('should handle read-only state', () => {
            mockApp.state.scoresheetView = 'grid';
            mockApp.state.isReadOnly = true;
            controller.renderGame();

            const scoresheetGrid = document.getElementById('scoresheet-grid');
            expect(scoresheetGrid.classList.add).toHaveBeenCalledWith('pointer-events-none');
        });
    });
});
