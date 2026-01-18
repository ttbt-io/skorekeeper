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

import { TeamSyncManager } from '../../frontend/services/teamSyncManager.js';

describe('TeamSyncManager', () => {
    let teamSyncManager;
    let mockFetch;

    beforeEach(() => {
        jest.clearAllMocks();
        mockFetch = jest.fn();
        global.fetch = mockFetch;
        teamSyncManager = new TeamSyncManager();
    });

    describe('fetchTeamList', () => {
        test('should fetch team list with pagination params', async() => {
            const mockTeams = { data: [{ id: 't1', name: 'Team 1' }], meta: { total: 1 } };
            mockFetch.mockResolvedValue({
                ok: true,
                json: async() => mockTeams,
            });

            const result = await teamSyncManager.fetchTeamList({ limit: 10, offset: 0 });

            expect(mockFetch).toHaveBeenCalledWith(
                expect.stringMatching(/\/api\/list-teams\?.*limit=10/),
                expect.objectContaining({
                    method: 'GET',
                }),
            );
            expect(result).toEqual(mockTeams);
        });

        test('should handle legacy array response', async() => {
            const mockTeams = [{ id: 't1' }];
            mockFetch.mockResolvedValue({
                ok: true,
                json: async() => mockTeams,
            });

            const result = await teamSyncManager.fetchTeamList();
            expect(result).toEqual({ data: mockTeams, meta: { total: 1 } });
        });

        test('should return empty list structure on 401/403', async() => {
            mockFetch.mockResolvedValue({
                ok: false,
                status: 403,
            });

            const result = await teamSyncManager.fetchTeamList();
            expect(result).toEqual({ data: [], meta: { total: 0 } });
        });

        test('should handle network errors gracefully', async() => {
            mockFetch.mockRejectedValue(new Error('Network error'));

            const result = await teamSyncManager.fetchTeamList();
            expect(result).toEqual({ data: [], meta: { total: 0 } });
        });
    });

    describe('saveTeam', () => {
        test('should save team successfully', async() => {
            const team = { id: 't1', name: 'Team 1' };
            mockFetch.mockResolvedValue({ ok: true });

            const result = await teamSyncManager.saveTeam(team);

            expect(mockFetch).toHaveBeenCalledWith('/api/save-team', expect.objectContaining({
                method: 'POST',
                body: JSON.stringify(team),
            }));
            expect(result).toBe(true);
        });

        test('should handle save error', async() => {
            mockFetch.mockResolvedValue({ ok: false, status: 500 });
            const result = await teamSyncManager.saveTeam({});
            expect(result).toBe(false);
        });
    });

    describe('deleteTeam', () => {
        test('should delete team successfully', async() => {
            mockFetch.mockResolvedValue({ ok: true });

            const result = await teamSyncManager.deleteTeam('t1');

            expect(mockFetch).toHaveBeenCalledWith('/api/delete-team', expect.objectContaining({
                method: 'POST',
                body: JSON.stringify({ id: 't1' }),
            }));
            expect(result).toBe(true);
        });

        test('should handle delete error', async() => {
            mockFetch.mockResolvedValue({ ok: false, status: 500 });
            const result = await teamSyncManager.deleteTeam('t1');
            expect(result).toBe(false);
        });
    });
});
