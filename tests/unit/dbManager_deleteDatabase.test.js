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

describe('DBManager.deleteDatabase', () => {
    let dbManager;
    let mockIndexedDB;
    let mockDb;

    beforeEach(() => {
        mockDb = {
            close: jest.fn(),
        };

        mockIndexedDB = {
            deleteDatabase: jest.fn(),
        };

        global.indexedDB = mockIndexedDB;
        dbManager = new DBManager('TestDB', 1);
        dbManager.db = mockDb;
    });

    test('should close existing connection and delete database', async() => {
        const mockRequest = {};
        mockIndexedDB.deleteDatabase.mockReturnValue(mockRequest);

        const promise = dbManager.deleteDatabase();

        expect(mockDb.close).toHaveBeenCalled();
        expect(dbManager.db).toBeNull();
        expect(mockIndexedDB.deleteDatabase).toHaveBeenCalledWith('TestDB');

        // Simulate success
        mockRequest.onsuccess();
        await expect(promise).resolves.toBeUndefined();
    });

    test('should reject on error', async() => {
        const mockRequest = {};
        mockIndexedDB.deleteDatabase.mockReturnValue(mockRequest);

        const promise = dbManager.deleteDatabase();

        mockRequest.onerror();
        await expect(promise).rejects.toThrow('Failed to delete database');
    });

    test('should reject on blocked', async() => {
        const mockRequest = {};
        mockIndexedDB.deleteDatabase.mockReturnValue(mockRequest);

        const promise = dbManager.deleteDatabase();

        mockRequest.onblocked();
        await expect(promise).rejects.toThrow('Database deletion blocked');
    });
});
