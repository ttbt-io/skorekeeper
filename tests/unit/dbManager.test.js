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

describe('DBManager', () => {
    let dbManager;
    let mockIndexedDB;
    let mockDb;
    let mockTx;
    let mockStore;

    beforeEach(() => {
        mockStore = {
            put: jest.fn(),
            get: jest.fn(),
            getAll: jest.fn(),
            delete: jest.fn(),
        };

        mockTx = {
            objectStore: jest.fn(() => mockStore),
            oncomplete: null,
            onerror: null,
        };

        mockDb = {
            objectStoreNames: {
                contains: jest.fn(() => false),
            },
            createObjectStore: jest.fn(),
            transaction: jest.fn(() => mockTx),
        };

        mockIndexedDB = {
            open: jest.fn(),
        };

        global.indexedDB = mockIndexedDB;
        dbManager = new DBManager('TestDB', 1);
    });

    afterEach(() => {
        jest.clearAllMocks();
    });

    test('open() should resolve with db instance', async() => {
        const mockRequest = {};
        mockIndexedDB.open.mockReturnValue(mockRequest);

        const openPromise = dbManager.open();

        // Simulate success
        mockRequest.onsuccess({ target: { result: mockDb } });

        const db = await openPromise;
        expect(db).toBe(mockDb);
        expect(dbManager.db).toBe(mockDb);
    });

    test('open() should handle onupgradeneeded', async() => {
        const mockRequest = {};
        mockIndexedDB.open.mockReturnValue(mockRequest);

        const openPromise = dbManager.open();

        // Simulate upgrade needed
        mockRequest.onupgradeneeded({ target: { result: mockDb } });

        expect(mockDb.createObjectStore).toHaveBeenCalledWith('games', { keyPath: 'id' });
        expect(mockDb.createObjectStore).toHaveBeenCalledWith('teams', { keyPath: 'id' });
        expect(mockDb.createObjectStore).toHaveBeenCalledWith('game_stats', { keyPath: 'id' });

        // Simulate success to resolve promise
        mockRequest.onsuccess({ target: { result: mockDb } });
        await openPromise;
    });

    test('open() should reject on error', async() => {
        const mockRequest = {};
        mockIndexedDB.open.mockReturnValue(mockRequest);

        const openPromise = dbManager.open();

        // Simulate error
        const error = new Error('Open failed');
        mockRequest.onerror(error);

        await expect(openPromise).rejects.toThrow(error);
    });

    test('saveGame() should open db if not open and put data', async() => {
        // Mock open
        const mockRequest = {};
        mockIndexedDB.open.mockReturnValue(mockRequest);

        const savePromise = dbManager.saveGame({ id: 'g1', data: 'test' });

        // Resolve open
        mockRequest.onsuccess({ target: { result: mockDb } });

        // Wait for save logic to run (it awaits open())
        // We need to trigger tx.oncomplete manually
        await new Promise(resolve => setTimeout(resolve, 0));

        expect(mockDb.transaction).toHaveBeenCalledWith(['games'], 'readwrite');
        expect(mockStore.put).toHaveBeenCalledWith(expect.objectContaining({
            schemaVersion: 3,
            id: 'g1',
            data: 'test',
            actionLog: [],
        }));

        // Resolve transaction
        mockTx.oncomplete();
        await savePromise;
    });

    test('loadGame() should return game data', async() => {
        dbManager.db = mockDb; // Simulate already open

        const mockGetReq = {};
        mockStore.get.mockReturnValue(mockGetReq);

        const loadPromise = dbManager.loadGame('g1');

        expect(mockDb.transaction).toHaveBeenCalledWith(['games'], 'readonly');
        expect(mockStore.get).toHaveBeenCalledWith('g1');

        mockGetReq.onsuccess({ target: { result: { id: 'g1', data: 'loaded' } } });

        const result = await loadPromise;
        expect(result).toEqual({ id: 'g1', data: 'loaded' });
    });

    test('getAllGames() should return summaries', async() => {
        dbManager.db = mockDb;

        const mockGetAllReq = {};
        mockStore.getAll.mockReturnValue(mockGetAllReq);

        const promise = dbManager.getAllGames();

        const rawData = [
            { id: 'g1', date: '2025-01-01', away: 'A', home: 'H', extra: 'ignored' },
            { id: 'g2', date: '2025-01-02', away: 'B', home: 'C' },
        ];
        mockGetAllReq.onsuccess({ target: { result: rawData } });

        const result = await promise;
        expect(result).toHaveLength(2);
        expect(result[0]).not.toHaveProperty('extra');
        expect(result[0].away).toBe('A');
    });

    test('saveGameStats() should put data', async() => {
        dbManager.db = mockDb;
        const promise = dbManager.saveGameStats('g1', { stat: 1 });

        expect(mockDb.transaction).toHaveBeenCalledWith(['game_stats'], 'readwrite');
        expect(mockStore.put).toHaveBeenCalledWith({ id: 'g1', stats: { stat: 1 } });

        mockTx.oncomplete();
        await promise;
    });

    test('getGameStats() should return stats property', async() => {
        dbManager.db = mockDb;
        const mockGetReq = {};
        mockStore.get.mockReturnValue(mockGetReq);

        const promise = dbManager.getGameStats('g1');

        mockGetReq.onsuccess({ target: { result: { id: 'g1', stats: { H: 10 } } } });

        const result = await promise;
        expect(result).toEqual({ H: 10 });
    });

    test('deleteTeam() should delete from store', async() => {
        dbManager.db = mockDb;
        const promise = dbManager.deleteTeam('t1');

        expect(mockDb.transaction).toHaveBeenCalledWith(['teams'], 'readwrite');
        expect(mockStore.delete).toHaveBeenCalledWith('t1');

        mockTx.oncomplete();
        await promise;
    });

    test('getAllGameStats() should return array', async() => {
        dbManager.db = mockDb;
        const mockReq = {};
        mockStore.getAll.mockReturnValue(mockReq);
        const promise = dbManager.getAllGameStats();
        mockReq.onsuccess({ target: { result: [1, 2] } });
        await expect(promise).resolves.toEqual([1, 2]);
    });

    test('deleteGameStats() should delete', async() => {
        dbManager.db = mockDb;
        const promise = dbManager.deleteGameStats('g1');
        expect(mockDb.transaction).toHaveBeenCalledWith(['game_stats'], 'readwrite');
        expect(mockStore.delete).toHaveBeenCalledWith('g1');
        mockTx.oncomplete();
        await promise;
    });

    test('getAllFullGames() should return raw array', async() => {
        dbManager.db = mockDb;
        const mockReq = {};
        mockStore.getAll.mockReturnValue(mockReq);
        const promise = dbManager.getAllFullGames();
        mockReq.onsuccess({ target: { result: [{ id: 1 }] } });
        await expect(promise).resolves.toEqual([{ id: 1 }]);
    });

    test('getLocalRevisions() should map id to last action id', async() => {
        dbManager.db = mockDb;
        const mockReq = {};
        mockStore.getAll.mockReturnValue(mockReq);
        const promise = dbManager.getLocalRevisions();

        const games = [
            { id: 'g1', actionLog: [{ id: 'a1' }, { id: 'a2' }] },
            { id: 'g2', actionLog: [] },
            { id: 'g3' }, // no log
        ];

        mockReq.onsuccess({ target: { result: games } });
        const map = await promise;

        expect(map.get('g1')).toBe('a2');
        expect(map.get('g2')).toBe('');
        expect(map.get('g3')).toBe('');
    });

    test('saveTeam() should put schema version', async() => {
        dbManager.db = mockDb;
        const promise = dbManager.saveTeam({ id: 't1' });

        expect(mockDb.transaction).toHaveBeenCalledWith(['teams'], 'readwrite');
        expect(mockStore.put).toHaveBeenCalledWith(expect.objectContaining({
            schemaVersion: 3,
            id: 't1',
        }));

        mockTx.oncomplete();
        await promise;
    });

    test('getAllTeams() should return array', async() => {
        dbManager.db = mockDb;
        const mockReq = {};
        mockStore.getAll.mockReturnValue(mockReq);
        const promise = dbManager.getAllTeams();
        mockReq.onsuccess({ target: { result: ['t1'] } });
        await expect(promise).resolves.toEqual(['t1']);
    });

    test('deleteGame() should delete', async() => {
        dbManager.db = mockDb;
        const promise = dbManager.deleteGame('g1');
        expect(mockDb.transaction).toHaveBeenCalledWith(['games'], 'readwrite');
        expect(mockStore.delete).toHaveBeenCalledWith('g1');
        mockTx.oncomplete();
        await promise;
    });

    test('saveGame() should persist dirty flag', async() => {
        dbManager.db = mockDb;
        const promise = dbManager.saveGame({ id: 'g1' }, false); // Explicit dirty=false

        expect(mockStore.put).toHaveBeenCalledWith(expect.objectContaining({
            id: 'g1',
            _dirty: false,
        }));

        mockTx.oncomplete();
        await promise;
    });

    test('saveGame() should default dirty flag to true', async() => {
        dbManager.db = mockDb;
        const promise = dbManager.saveGame({ id: 'g1' });

        expect(mockStore.put).toHaveBeenCalledWith(expect.objectContaining({
            id: 'g1',
            _dirty: true,
        }));

        mockTx.oncomplete();
        await promise;
    });

    test('markClean() should update dirty flag to false', async() => {
        dbManager.db = mockDb;
        const mockGetReq = {};
        mockStore.get.mockReturnValue(mockGetReq);

        const promise = dbManager.markClean('g1', 'games');

        expect(mockDb.transaction).toHaveBeenCalledWith(['games'], 'readwrite');
        expect(mockStore.get).toHaveBeenCalledWith('g1');

        // Simulate existing record
        const record = { id: 'g1', _dirty: true };
        mockGetReq.onsuccess({ target: { result: record } });

        expect(record._dirty).toBe(false);
        expect(mockStore.put).toHaveBeenCalledWith(record);

        mockTx.oncomplete();
        await promise;
    });
});
