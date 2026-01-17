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

        // Mock container to prevent autoFill loop
        const container = document.createElement('div');
        container.id = 'game-list-container';
        Object.defineProperty(container, 'clientHeight', { value: 500 });
        Object.defineProperty(container, 'scrollHeight', { value: 2000 });
        document.body.appendChild(container);

        controller = new DashboardController(mockApp);
    });

    afterEach(() => {
        document.body.innerHTML = '';
    });

    describe('loadDashboard', () => {
        test('should load page 0 and merge with local games', async() => {
            controller.batchSize = 1; // Prevent exhausting stream with duplicates
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1', name: 'Local Game', date: '2025-01-01' }]);
            mockApp.sync.fetchGameList.mockResolvedValue({
                data: [{ id: 'g1', name: 'Remote Game', revision: 'rev1', date: '2025-01-01' }],
                meta: { total: 100 },
            });
            mockApp.db.getLocalRevisions.mockResolvedValue(new Map([['g1', 'rev1']]));

            await controller.loadDashboard();

            expect(mockApp.state.games.length).toBe(1);
            expect(mockApp.state.games[0]).toMatchObject({
                id: 'g1',
                syncStatus: 'synced',
                source: 'local',
            });
            expect(controller.hasMore).toBe(true);
            expect(mockApp.render).toHaveBeenCalled();
        });

        test('should handle offline mode (fetch failure)', async() => {
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1' }]);
            mockApp.sync.fetchGameList.mockRejectedValue(new Error('Offline'));

            await controller.loadDashboard();

            // Should fall back to local games
            expect(mockApp.state.games.length).toBe(1);
            expect(controller.hasMore).toBe(false);
        });

        test('should filter and sort local games in offline mode', async() => {
            mockApp.sync.fetchGameList.mockRejectedValue(new Error('Offline'));
            mockApp.db.getAllGames.mockResolvedValue([
                { id: 'g1', event: 'Match A', date: '2025-01-01' },
                { id: 'g2', event: 'Match B', date: '2025-01-02' },
                { id: 'g3', event: 'Other', date: '2025-01-03' },
            ]);

            controller.query = 'match';
            await controller.loadDashboard();

            // Should return g2 (newest match) then g1. g3 filtered out.
            expect(mockApp.state.games.length).toBe(2);
            expect(mockApp.state.games[0].id).toBe('g2');
            expect(mockApp.state.games[1].id).toBe('g1');
        });

        test('should handle unsynced games in list', async() => {
            mockApp.db.getAllGames.mockResolvedValue([{ id: 'g1' }]);
            mockApp.sync.fetchGameList.mockResolvedValue({
                data: [{ id: 'g1', revision: 'rev2' }],
                meta: { total: 1 },
            });
            mockApp.db.getLocalRevisions.mockResolvedValue(new Map([['g1', 'rev1']]));

            await controller.loadDashboard();

            expect(mockApp.state.games[0].syncStatus).toBe('unsynced');
        });

        test('should handle remote only games in list', async() => {
            mockApp.db.getAllGames.mockResolvedValue([]);
            mockApp.sync.fetchGameList.mockResolvedValue({
                data: [{ id: 'g1', revision: 'rev1' }],
                meta: { total: 1 },
            });

            await controller.loadDashboard();

            expect(mockApp.state.games[0].syncStatus).toBe('remote_only');
        });
    });

    describe('loadMore', () => {
        test('should fetch next page and append', async() => {
            controller.hasMore = true;
            controller.merger = {
                fetchNextBatch: jest.fn().mockResolvedValue([{ id: 'g2', source: 'remote' }]),
                hasMore: jest.fn().mockReturnValue(true),
            };
            mockApp.state.games = [{ id: 'g1' }];

            await controller.loadMore();

            expect(mockApp.state.games.length).toBe(2);
            expect(mockApp.state.games[1].id).toBe('g2');
            expect(controller.merger.fetchNextBatch).toHaveBeenCalled();
        });

        test('should do nothing if hasMore is false', async() => {
            controller.hasMore = false;
            controller.merger = { fetchNextBatch: jest.fn() };
            await controller.loadMore();
            expect(controller.merger.fetchNextBatch).not.toHaveBeenCalled();
        });
    });

    describe('search', () => {
        test('should reset page and fetch with query', async() => {
            mockApp.db.getAllGames.mockResolvedValue([]);
            mockApp.sync.fetchGameList.mockResolvedValue({
                data: [{ id: 'g1' }],
                meta: { total: 1 },
            });
            mockApp.db.getLocalRevisions.mockResolvedValue(new Map());

            await controller.search('yankees');

            expect(controller.query).toBe('yankees');
            // Check that fetchGameList was called with query
            expect(mockApp.sync.fetchGameList).toHaveBeenCalledWith(expect.objectContaining({
                query: 'yankees',
                offset: 0,
            }));
            expect(mockApp.render).toHaveBeenCalled();
        });

        test('should not show sentinel if no results', async() => {
            mockApp.db.getAllGames.mockResolvedValue([]);
            mockApp.sync.fetchGameList.mockResolvedValue({
                data: [],
                meta: { total: 0 },
            });
            
            // Set up DOM
            const main = document.createElement('main');
            main.id = 'game-list-container';
            document.body.appendChild(main);
            
            await controller.search('nothing');
            
            expect(main.textContent).not.toContain('Scroll for more');
            expect(main.textContent).not.toContain('All games loaded');
        });
    });
});
