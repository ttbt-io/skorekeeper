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

import { DashboardController } from '../../frontend/controllers/DashboardController.js';

describe('DashboardController', () => {
    let controller;
    let mockApp;

    beforeEach(() => {
        mockApp = {
            db: {
                getAllGames: jest.fn().mockResolvedValue([]),
                getLocalRevisions: jest.fn().mockResolvedValue(new Map()),
                deleteGame: jest.fn(),
                getAllTeams: jest.fn().mockResolvedValue([]),
            },
            auth: {
                getUser: jest.fn(() => ({ email: 'user@example.com' })),
                accessDeniedMessage: 'Denied',
            },
            sync: {
                fetchGameList: jest.fn().mockResolvedValue([]),
            },
            state: {
                games: [],
                view: '',
            },
            render: jest.fn(),
            modalConfirmFn: jest.fn(),
            hasReadAccess: jest.fn(() => true),
        };

        controller = new DashboardController(mockApp);
    });

    describe('loadDashboard', () => {
        test('should load local and remote games', async() => {
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1', name: 'Local Game' }]);
            mockApp.sync.fetchGameList.mockResolvedValue([{ id: 'g1', name: 'Remote Game', revision: 'rev1' }]);
            mockApp.db.getLocalRevisions.mockResolvedValue(new Map([['g1', 'rev1']]));

            await controller.loadDashboard();

            expect(mockApp.state.games.length).toBe(1);
            expect(mockApp.state.games[0]).toMatchObject({
                id: 'g1',
                syncStatus: 'synced',
                source: 'local',
            });
            expect(mockApp.render).toHaveBeenCalled();
        });

        test('should handle deleted remote games', async() => {
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1' }]);
            mockApp.sync.fetchGameList.mockResolvedValue([{ id: 'g1', status: 'deleted' }]);

            await controller.loadDashboard();

            expect(mockApp.db.deleteGame).toHaveBeenCalledWith('g1');
            expect(mockApp.state.games.length).toBe(0);
        });

        test('should handle unsynced games', async() => {
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1' }]);
            mockApp.sync.fetchGameList.mockResolvedValue([{ id: 'g1', revision: 'rev2' }]);
            mockApp.db.getLocalRevisions.mockResolvedValue(new Map([['g1', 'rev1']]));

            await controller.loadDashboard();

            expect(mockApp.state.games[0].syncStatus).toBe('unsynced');
        });

        test('should handle local only games', async() => {
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1' }]);
            mockApp.sync.fetchGameList.mockResolvedValue([]);

            await controller.loadDashboard();

            expect(mockApp.state.games[0].syncStatus).toBe('local_only');
        });

        test('should handle remote only games', async() => {
            mockApp.db.getAllGames.mockResolvedValue([]);
            mockApp.sync.fetchGameList.mockResolvedValue([{ id: 'g1', revision: 'rev1' }]);

            await controller.loadDashboard();

            expect(mockApp.state.games[0].syncStatus).toBe('remote_only');
        });

        test('should filter inaccessible games', async() => {
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1' }, { id: 'g2' }]);
            mockApp.hasReadAccess.mockImplementation((game) => game.id === 'g1');

            await controller.loadDashboard();

            expect(mockApp.state.games.length).toBe(1);
            expect(mockApp.state.games[0].id).toBe('g1');
        });
    });
});
