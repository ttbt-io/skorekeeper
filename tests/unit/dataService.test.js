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

import { DataService } from '../../frontend/services/DataService.js';
import { ActionTypes } from '../../frontend/reducer.js';

describe('DataService', () => {
    let service;
    let mockDB;
    let mockAuth;
    let mockTeamSync;

    beforeEach(() => {
        mockDB = {
            loadGame: jest.fn(),
            saveGame: jest.fn(),
            getAllFullGames: jest.fn().mockResolvedValue([]),
            getAllTeams: jest.fn().mockResolvedValue([]),
            saveTeam: jest.fn(),
        };
        mockAuth = {
            getLocalId: jest.fn(() => 'local-123'),
        };
        mockTeamSync = {
            saveTeam: jest.fn().mockResolvedValue(true),
        };

        service = new DataService({ db: mockDB, auth: mockAuth, teamSync: mockTeamSync });
        window.fetch = jest.fn();
    });

    describe('ensureDemoGame', () => {
        test('should seed demo game if not exists', async() => {
            mockDB.loadGame.mockResolvedValue(null);
            window.fetch.mockResolvedValue({
                ok: true,
                json: async() => ({ id: 'demo-game-001', data: 'demo' }),
            });

            await service.ensureDemoGame();

            expect(mockDB.saveGame).toHaveBeenCalledWith(expect.objectContaining({
                id: 'demo-game-001',
                ownerId: 'local-123',
            }));
        });

        test('should not seed if exists', async() => {
            mockDB.loadGame.mockResolvedValue({ id: 'demo-game-001' });
            await service.ensureDemoGame();
            expect(window.fetch).not.toHaveBeenCalled();
        });
    });

    describe('reconcileLocalData', () => {
        const user = { email: 'user@example.com' };

        test('should update owner of local games', async() => {
            const localGame = {
                id: 'g1',
                ownerId: 'local-123',
                actionLog: [
                    { type: ActionTypes.GAME_START, payload: { id: 'g1', ownerId: 'local-123' } },
                ],
            };
            mockDB.getAllFullGames.mockResolvedValue([localGame]);

            await service.reconcileLocalData(user);

            expect(mockDB.saveGame).toHaveBeenCalledWith(expect.objectContaining({
                ownerId: 'user@example.com',
            }));

            // Verify actionLog update
            const savedGame = mockDB.saveGame.mock.calls[0][0];
            expect(savedGame.actionLog[0].payload.ownerId).toBe('user@example.com');
        });

        test('should update owner of local teams', async() => {
            const localTeam = {
                id: 't1',
                ownerId: 'local-123',
                name: 'Local Team',
            };
            mockDB.getAllTeams.mockResolvedValue([localTeam]);

            await service.reconcileLocalData(user);

            expect(mockDB.saveTeam).toHaveBeenCalledWith(expect.objectContaining({
                ownerId: 'user@example.com',
            }));
            expect(mockTeamSync.saveTeam).toHaveBeenCalled();
        });
    });
});
