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

import { TeamController } from '../../frontend/controllers/TeamController.js';

describe('TeamController', () => {
    let controller;
    let mockApp;

    beforeEach(() => {
        mockApp = {
            db: {
                getAllTeams: jest.fn().mockResolvedValue([]),
                saveTeam: jest.fn(),
                deleteTeam: jest.fn(),
            },
            auth: {
                getUser: jest.fn(() => ({ email: 'user@example.com' })),
                getLocalId: jest.fn(() => 'local-123'),
                isStale: false,
            },
            teamSync: {
                fetchTeamList: jest.fn().mockResolvedValue([]),
                saveTeam: jest.fn().mockResolvedValue(true),
                deleteTeam: jest.fn(),
            },
            teamsRenderer: {
                renderTeamsList: jest.fn(),
                renderTeamMembers: jest.fn(),
                renderTeamRow: jest.fn(),
            },
            state: {
                teams: [],
                currentUser: { email: 'user@example.com' },
            },
            render: jest.fn(),
            modalConfirmFn: jest.fn().mockResolvedValue(true),
            validate: jest.fn(() => true),
            hasTeamReadAccess: jest.fn(() => true),
            pendingSaves: 0,
            updateSaveStatus: jest.fn(),
        };

        controller = new TeamController(mockApp);
    });

    describe('loadTeamsView', () => {
        test('should load local and remote teams', async() => {
            mockApp.db.getAllTeams.mockResolvedValue([{ id: 't1', name: 'Local Team' }]);
            mockApp.teamSync.fetchTeamList.mockResolvedValue({
                data: [{ id: 't1', name: 'Remote Team' }],
                meta: { total: 1 },
            });

            await controller.loadTeamsView();

            expect(mockApp.db.saveTeam).toHaveBeenCalledWith(expect.objectContaining({
                id: 't1',
                name: 'Remote Team',
            }));
            expect(mockApp.state.teams.length).toBe(1);
            expect(mockApp.render).toHaveBeenCalled();
        });

        test('should handle deleted remote teams', async() => {
            mockApp.db.getAllTeams.mockResolvedValue([{ id: 't1' }]);
            mockApp.teamSync.fetchTeamList.mockResolvedValue({
                data: [{ id: 't1', status: 'deleted' }],
                meta: { total: 1 },
            });

            await controller.loadTeamsView();

            expect(mockApp.db.deleteTeam).toHaveBeenCalledWith('t1');
            expect(mockApp.state.teams.length).toBe(0);
        });

        test('should handle offline mode', async() => {
            mockApp.db.getAllTeams.mockResolvedValue([{ id: 't1', name: 'Local' }]);
            mockApp.teamSync.fetchTeamList.mockRejectedValue(new Error('Offline'));

            await controller.loadTeamsView();

            expect(mockApp.state.teams.length).toBe(1);
            expect(controller.hasMore).toBe(false);
        });
    });

    describe('loadMore', () => {
        test('should load next page', async() => {
            controller.page = 0;
            controller.limit = 10;
            controller.hasMore = true;
            mockApp.state.teams = [{ id: 't1' }];

            mockApp.db.getAllTeams.mockResolvedValue([]);
            mockApp.teamSync.fetchTeamList.mockResolvedValue({
                data: [{ id: 't2' }],
                meta: { total: 20 },
            });

            await controller.loadMore();

            expect(controller.page).toBe(1);
            expect(mockApp.state.teams.length).toBe(2);
            expect(mockApp.teamSync.fetchTeamList).toHaveBeenCalledWith(expect.objectContaining({
                offset: 10,
            }));
        });
    });

    describe('syncTeam', () => {
        test('should sync team and update status', async() => {
            mockApp.state.teams = [{ id: 't1', syncStatus: 'local_only' }];
            await controller.syncTeam('t1');

            expect(mockApp.teamSync.saveTeam).toHaveBeenCalled();
            expect(mockApp.state.teams[0].syncStatus).toBe('synced');
        });
    });

    describe('deleteTeam', () => {
        test('should delete team locally and remotely', async() => {
            await controller.deleteTeam('t1');

            expect(mockApp.db.deleteTeam).toHaveBeenCalledWith('t1');
            expect(mockApp.teamSync.deleteTeam).toHaveBeenCalledWith('t1');
        });
    });
});
