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
import { SyncStatusSynced, SyncStatusUnsynced } from '../../frontend/constants.js';

describe('DashboardController', () => {
    let controller;
    let mockApp;

    beforeEach(() => {
        mockApp = {
            db: {
                getAllGames: jest.fn().mockResolvedValue([]),
                getLocalRevisions: jest.fn().mockResolvedValue(new Map()),
                deleteGame: jest.fn().mockResolvedValue(true),
                getAllTeams: jest.fn().mockResolvedValue([]),
            },
            auth: {
                getUser: jest.fn(() => ({ email: 'test@example.com' })),
            },
            sync: {
                fetchGameList: jest.fn().mockResolvedValue({ data: [] }),
                checkGameDeletions: jest.fn().mockResolvedValue([]),
                isServerUnreachable: false,
            },
            dashboardRenderer: {
                render: jest.fn(),
            },
            state: {
                games: [],
                view: '',
            },
            render: jest.fn(),
            modalConfirmFn: jest.fn(),
            hasReadAccess: jest.fn(() => true),
        };

        // Mock DOM elements
        const container = document.createElement('div');
        container.id = 'game-list-container';
        Object.defineProperty(container, 'clientHeight', { value: 500, writable: true });
        Object.defineProperty(container, 'scrollHeight', { value: 500, writable: true });
        Object.defineProperty(container, 'scrollTop', { value: 0, writable: true });
        document.body.appendChild(container);

        controller = new DashboardController(mockApp);
    });

    afterEach(() => {
        document.body.innerHTML = '';
        jest.clearAllMocks();
    });

    /**
     * Helper to flush pending promises since loadDashboard is non-blocking.
     */
    const flushPromises = () => new Promise(resolve => setTimeout(resolve, 0));

    describe('loadDashboard', () => {
        test('should render immediately and then update with local games', async() => {
            mockApp.db.getAllGames.mockResolvedValue([
                { id: 'g1', event: 'Local Game', date: '2025-01-01' },
            ]);
            mockApp.db.getLocalRevisions.mockResolvedValue(new Map([['g1', 'rev1']]));

            // Act
            await controller.loadDashboard();

            // Initial render (empty)
            expect(mockApp.render).toHaveBeenCalled();

            // Wait for local load
            await flushPromises();

            // Verify Local Load
            expect(mockApp.db.getAllGames).toHaveBeenCalled();
            expect(mockApp.state.games.length).toBe(1);
            expect(mockApp.state.games[0]).toMatchObject({
                id: 'g1',
                source: 'local',
            });
        });

        test('should fetch remote games in background and merge', async() => {
            // Setup Local
            mockApp.db.getAllGames.mockResolvedValue([
                { id: 'g1', event: 'Local Game', date: '2025-01-01' },
            ]);
            mockApp.db.getLocalRevisions.mockResolvedValue(new Map([['g1', 'rev1']]));

            // Setup Remote (g1 updated, g2 new)
            mockApp.sync.fetchGameList.mockResolvedValue({
                data: [
                    { id: 'g1', event: 'Remote Update', date: '2025-01-01', revision: 'rev1' },
                    { id: 'g2', event: 'Remote New', date: '2025-01-02', revision: 'rev1' },
                ],
            });

            await controller.loadDashboard();
            await flushPromises();

            // Should have merged
            expect(mockApp.state.games.length).toBe(2);

            // Sort order: Date Descending -> g2 (Jan 2), g1 (Jan 1)
            expect(mockApp.state.games[0].id).toBe('g2');
            expect(mockApp.state.games[0].source).toBe('remote'); // g2 is remote only

            expect(mockApp.state.games[1].id).toBe('g1');
            expect(mockApp.state.games[1].event).toBe('Remote Update'); // Merged remote data
            expect(mockApp.state.games[1].syncStatus).toBe(SyncStatusSynced);
        });

        test('should handle offline mode gracefully', async() => {
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1', date: '2025-01-01' }]);
            mockApp.sync.fetchGameList.mockRejectedValue(new Error('Offline'));

            await controller.loadDashboard();
            await flushPromises();

            expect(mockApp.state.games.length).toBe(1);
            expect(mockApp.state.games[0].id).toBe('g1');
            // Should stay 'local' source if remote fetch fails
            expect(mockApp.state.games[0].source).toBe('local');
        });

        test('should detect unsynced state', async() => {
            // Local has _dirty=true
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1', date: '2025-01-01', _dirty: true }]);
            mockApp.db.getLocalRevisions.mockResolvedValue(new Map([['g1', 'rev1']]));

            mockApp.sync.fetchGameList.mockResolvedValue({
                data: [{ id: 'g1', date: '2025-01-01', revision: 'rev2' }],
            });

            await controller.loadDashboard();
            await flushPromises();

            const game = mockApp.state.games.find(g => g.id === 'g1');
            expect(game.syncStatus).toBe(SyncStatusUnsynced);
        });

        test('should handle remote deletion via checkDeletions', async() => {
            // Local has g1, but server says it's deleted
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1', date: '2025-01-01' }]);
            mockApp.sync.checkGameDeletions.mockResolvedValue(['g1']);

            await controller.loadDashboard();
            await flushPromises();

            expect(mockApp.db.deleteGame).toHaveBeenCalledWith('g1');
            expect(mockApp.state.games.length).toBe(0);
        });
    });

    describe('handleScroll', () => {
        test('should increase displayLimit and render', async() => {
            // Setup buffer with 100 items
            const manyGames = Array.from({ length: 100 }, (_, i) => ({
                id: `g${i}`,
                date: '2025-01-01',
            }));
            mockApp.db.getAllGames.mockResolvedValue(manyGames);

            await controller.loadDashboard();
            await flushPromises();

            // Initial Limit is 50
            expect(mockApp.state.games.length).toBe(50);

            // Mock Scroll to bottom
            const container = document.getElementById('game-list-container');
            // scrollTop + clientHeight >= scrollHeight - 200
            // 0 + 500 >= 500 - 200 (True, 500 >= 300)
            // But let's make it explicit
            Object.defineProperty(container, 'scrollTop', { value: 1000 });
            Object.defineProperty(container, 'scrollHeight', { value: 1500 });
            Object.defineProperty(container, 'clientHeight', { value: 500 }); // 1500 >= 1300

            controller.handleScroll(container);

            // Should show 20 more
            expect(mockApp.state.games.length).toBe(70);
        });

        test('should trigger remote fetch if running low', async() => {
            // Setup buffer with 60 items
            const games = Array.from({ length: 60 }, (_, i) => ({
                id: `g${i}`, date: '2025-01-01',
            }));
            mockApp.db.getAllGames.mockResolvedValue(games);

            await controller.loadDashboard();
            await flushPromises();

            // Initial state
            controller.displayLimit = 50;
            controller.remoteHasMore = true;
            controller.isFetchingRemote = false;

            // Scroll
            const container = document.getElementById('game-list-container');
            Object.defineProperty(container, 'scrollTop', { value: 1000 });
            Object.defineProperty(container, 'scrollHeight', { value: 1500 });
            Object.defineProperty(container, 'clientHeight', { value: 500 });

            controller.handleScroll(container);

            // Limit becomes 70. Total loaded 60. 70 > 60-20 (40). Should fetch.
            expect(mockApp.sync.fetchGameList).toHaveBeenCalled();
        });
    });

    describe('search', () => {
        test('should reset state and reload', async() => {
            const spy = jest.spyOn(controller, 'loadDashboard');
            await controller.search('query');
            expect(controller.query).toBe('query');
            expect(spy).toHaveBeenCalled();
        });
    });
});