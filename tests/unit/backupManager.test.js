

import { BackupManager } from '../../frontend/services/backupManager.js';
import { ReadableStream, TextDecoderStream } from 'stream/web';
import { TextEncoder, TextDecoder } from 'util';

// Polyfill globals for Node/Jest environment
global.ReadableStream = ReadableStream;
global.TextDecoderStream = TextDecoderStream;
global.TextEncoder = TextEncoder;
global.TextDecoder = TextDecoder;

describe('BackupManager', () => {
    let backupManager;
    let mockDb;
    let mockSync;
    let mockTeamSync;

    beforeEach(() => {
        mockDb = {
            getAllTeams: jest.fn().mockResolvedValue([]),
            getAllGames: jest.fn().mockResolvedValue([]),
            loadGame: jest.fn(),
            saveGame: jest.fn(),
            saveTeam: jest.fn(),
        };
        mockSync = {
            fetchGameList: jest.fn().mockResolvedValue([]),
        };
        mockTeamSync = {
            fetchTeamList: jest.fn().mockResolvedValue([]),
        };
        backupManager = new BackupManager(mockDb, mockSync, mockTeamSync);

        // Mock TextEncoder for environment
        if (typeof TextEncoder === 'undefined') {
            global.TextEncoder = class {
                encode(s) {
                    return Buffer.from(s);
                }
            };
        }
    });

    test('getBackupStream should generate a valid JSONL stream', async() => {
        const teamId = '11111111-1111-1111-1111-111111111111';
        const gameId = '22222222-2222-2222-2222-222222222222';
        const team = { id: teamId, name: 'Team 1' };
        const gameMeta = { id: gameId, date: '2023-01-01' };
        const fullGame = { id: gameId, date: '2023-01-01', away: 'A', home: 'B' };

        mockDb.getAllTeams.mockResolvedValue([team]);
        mockDb.getAllGames.mockResolvedValue([gameMeta]);
        mockDb.loadGame.mockResolvedValue(fullGame);

        const stream = await backupManager.getBackupStream({ games: true, teams: true, remote: false });
        const reader = stream.getReader();
        let result = '';

        while (true) {
            const { done, value } = await reader.read();
            if (done) {
                break;
            }
            result += new TextDecoder().decode(value);
        }

        const lines = result.trim().split('\n');
        expect(lines.length).toBe(3); // Header, Team, Game

        const header = JSON.parse(lines[0]);
        expect(header.type).toBe('header');

        const teamLine = JSON.parse(lines[1]);
        expect(teamLine.type).toBe('team');
        expect(teamLine.id).toBe(teamId);

        const gameLine = JSON.parse(lines[2]);
        expect(gameLine.type).toBe('game');
        expect(gameLine.id).toBe(gameId);
    });

    test('getBackupStream should include remote-only games', async() => {
        const remoteId = '33333333-3333-3333-3333-333333333333';
        // Local is empty
        mockDb.getAllGames.mockResolvedValue([]);
        mockDb.loadGame.mockResolvedValue(null);

        // Remote has one game
        mockSync.fetchGameList.mockResolvedValue([{ id: remoteId }]);

        // Mock fetch for the full game
        const remoteFullGame = { id: remoteId, away: 'Remote', home: 'Team', actionLog: [] };
        global.fetch = jest.fn().mockResolvedValue({
            ok: true,
            json: async() => remoteFullGame,
        });

        const stream = await backupManager.getBackupStream({ games: true, teams: false, remote: true });
        const reader = stream.getReader();
        let result = '';
        while (true) {
            const { done, value } = await reader.read();
            if (done) {
                break;
            }
            result += new TextDecoder().decode(value);
        }

        const lines = result.trim().split('\n');
        expect(lines.length).toBe(2); // Header, Game
        const gameLine = JSON.parse(lines[1]);
        expect(gameLine.id).toBe(remoteId);
        expect(gameLine.data.away).toBe('Remote');
    });

    test('restoreBackup should handle empty selection and empty DB', async() => {
        const gameId = '44444444-4444-4444-4444-444444444444';
        const jsonlData = [
            JSON.stringify({ type: 'header', version: 1 }),
            JSON.stringify({ type: 'game', id: gameId, data: { id: gameId, away: 'A', actionLog: [] } }),
        ].join('\n');

        const file = new Blob([jsonlData], { type: 'application/jsonl' });
        file.stream = () => new ReadableStream({
            start(controller) {
                controller.enqueue(new TextEncoder().encode(jsonlData));
                controller.close();
            },
        });

        // 1. Selection is empty
        const result1 = await backupManager.restoreBackup(file, new Set());
        expect(result1.success).toBe(0);
        expect(mockDb.saveGame).not.toHaveBeenCalled();

        // 2. Selection matches but DB is empty (should just save)
        const result2 = await backupManager.restoreBackup(file, new Set([gameId]));
        expect(result2.success).toBe(1);
        expect(mockDb.saveGame).toHaveBeenCalledWith({ id: gameId, away: 'A', actionLog: [] });
    });

    test('restoreBackup should save selected items', async() => {
        const teamId1 = '55555555-5555-5555-5555-555555555555';
        const teamId2 = '66666666-6666-6666-6666-666666666666';
        const gameId = '77777777-7777-7777-7777-777777777777';
        const jsonlData = [
            JSON.stringify({ type: 'header', version: 1 }),
            JSON.stringify({ type: 'team', id: teamId1, data: { id: teamId1, name: 'Team 1' } }),
            JSON.stringify({ type: 'game', id: gameId, data: { id: gameId, away: 'A', home: 'B', actionLog: [] } }),
            JSON.stringify({ type: 'team', id: teamId2, data: { id: teamId2, name: 'Team 2' } }),
        ].join('\n');

        const file = new Blob([jsonlData], { type: 'application/jsonl' });

        // Mock stream for Blob/File in Node/Jest
        file.stream = () => {
            const stream = new ReadableStream({
                start(controller) {
                    controller.enqueue(new TextEncoder().encode(jsonlData));
                    controller.close();
                },
            });
            return stream;
        };

        const selectedIds = new Set([teamId1, gameId]);
        const result = await backupManager.restoreBackup(file, selectedIds);

        expect(result.success).toBe(2);
        expect(mockDb.saveTeam).toHaveBeenCalledTimes(1);
        expect(mockDb.saveTeam).toHaveBeenCalledWith({ id: teamId1, name: 'Team 1' });
        expect(mockDb.saveGame).toHaveBeenCalledTimes(1);
        expect(mockDb.saveGame).toHaveBeenCalledWith({ id: gameId, away: 'A', home: 'B', actionLog: [] });

        // t2 was not selected
        expect(mockDb.saveTeam).not.toHaveBeenCalledWith({ id: teamId2, name: 'Team 2' });
    });
});
