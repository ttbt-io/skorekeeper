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
                deleteTeam: jest.fn().mockResolvedValue(true),
            },
            auth: {
                getUser: jest.fn(() => ({ email: 'user@example.com' })),
                getLocalId: jest.fn(() => 'local-123'),
                isStale: false,
            },
            teamSync: {
                fetchTeamList: jest.fn().mockResolvedValue({ data: [] }),
                checkTeamDeletions: jest.fn().mockResolvedValue([]),
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

    afterEach(() => {
        document.body.innerHTML = '';
        jest.clearAllMocks();
    });

    const flushPromises = () => new Promise(resolve => setTimeout(resolve, 0));

    describe('loadTeamsView', () => {
        test('should render immediately and update with local teams', async() => {
            mockApp.db.getAllTeams.mockResolvedValue([{ id: 't1', name: 'Local Team' }]);

            await controller.loadTeamsView();
            expect(mockApp.render).toHaveBeenCalled();

            await flushPromises();
            expect(mockApp.state.teams.length).toBe(1);
            expect(mockApp.state.teams[0].id).toBe('t1');
            expect(mockApp.state.teams[0].source).toBe('local');
        });

        test('should fetch remote teams and merge', async() => {
            mockApp.db.getAllTeams.mockResolvedValue([{ id: 't1', name: 'Local' }]);
            mockApp.teamSync.fetchTeamList.mockResolvedValue({
                data: [{ id: 't1', name: 'Remote' }],
            });

            await controller.loadTeamsView();
            await flushPromises();

            expect(mockApp.state.teams.length).toBe(1);
            expect(mockApp.state.teams[0].id).toBe('t1');
            expect(mockApp.state.teams[0].name).toBe('Local'); // Prefers local for editing safety
        });

        test('should handle offline mode', async() => {
            mockApp.db.getAllTeams.mockResolvedValue([{ id: 't1', name: 'Local' }]);
            mockApp.teamSync.fetchTeamList.mockRejectedValue(new Error('Offline'));

            await controller.loadTeamsView();
            await flushPromises();

            expect(mockApp.state.teams.length).toBe(1);
            expect(controller.remoteHasMore).toBe(false);
        });

        test('should handle remote deletions', async() => {
            mockApp.db.getAllTeams.mockResolvedValue([{ id: 't1' }]);
            mockApp.teamSync.checkTeamDeletions.mockResolvedValue(['t1']);
            mockApp.db.deleteTeam.mockResolvedValue(true);

            await controller.loadTeamsView();
            await flushPromises();

            expect(mockApp.db.deleteTeam).toHaveBeenCalledWith('t1');
            expect(mockApp.state.teams.length).toBe(0);
        });
    });

    describe('handleScroll', () => {
        test('should increase displayLimit', async() => {
            // Buffer size = 60
            const teams = Array.from({ length: 60 }, (_, i) => ({ id: `t${i}`, name: `Team ${i}` }));
            mockApp.db.getAllTeams.mockResolvedValue(teams);

            await controller.loadTeamsView();
            await flushPromises();

            expect(mockApp.state.teams.length).toBe(50); // Initial limit

            const container = document.createElement('div');
            Object.defineProperty(container, 'scrollTop', { value: 1000 });
            Object.defineProperty(container, 'scrollHeight', { value: 1500 });
            Object.defineProperty(container, 'clientHeight', { value: 500 });

            controller.handleScroll(container);

            // Limited by buffer size of 60
            expect(mockApp.state.teams.length).toBe(60);
        });

        test('should trigger remote fetch if running low', async() => {
            const teams = Array.from({ length: 60 }, (_, i) => ({
                id: `t${i}`, name: `Team ${i}`,
            }));
            mockApp.db.getAllTeams.mockResolvedValue(teams);

            await controller.loadTeamsView();
            await flushPromises();

            controller.displayLimit = 50;
            controller.remoteHasMore = true;
            controller.isFetchingRemote = false;

            const container = document.createElement('div');
            Object.defineProperty(container, 'scrollTop', { value: 1000 });
            Object.defineProperty(container, 'scrollHeight', { value: 1500 });
            Object.defineProperty(container, 'clientHeight', { value: 500 });

            controller.handleScroll(container);

            expect(mockApp.teamSync.fetchTeamList).toHaveBeenCalled();
        });
    });

    describe('search', () => {
        test('should reset state and reload', async() => {
            const spy = jest.spyOn(controller, 'loadTeamsView');
            await controller.search('query');
            expect(controller.query).toBe('query');
            expect(spy).toHaveBeenCalled();
        });
    });

    describe('syncTeam', () => {
        test('should sync team and update status', async() => {
            const team = { id: 't1', name: 'Team', syncStatus: 'local_only' };
            controller.localBuffer = [team];
            controller.localMap = new Map([['t1', team]]);
            mockApp.state.teams = [team]; // Initial state (though mergeAndRender will overwrite)

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
