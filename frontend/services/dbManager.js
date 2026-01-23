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

/**
 * Manages IndexedDB operations for the Skorekeeper application,
 * handling game data persistence.
 * @class
 */
export class DBManager {
    /** @private @type {string} The name of the IndexedDB database. */
    dbName;
    /** @private @type {number} The version of the IndexedDB database schema. */
    dbVersion;
    /** @private @type {IDBDatabase|null} The IndexedDB database instance. */
    db;

    /**
     * Creates an instance of DBManager.
     * @param {string} [dbName='SkorekeeperDB'] - The name of the database.
     * @param {number} [dbVersion=4] - The version of the database schema.
     */
    constructor(dbName = 'SkorekeeperDB', dbVersion = 4) {
        this.dbName = dbName;
        this.dbVersion = dbVersion;
        this.db = null;
    }

    /**
     * Opens the IndexedDB database, creating object stores if necessary.
     * @async
     * @returns {Promise<IDBDatabase>} A promise that resolves with the database instance once opened.
     */
    async open() {
        return new Promise((resolve, reject) => {
            const request = indexedDB.open(this.dbName, this.dbVersion);

            request.onupgradeneeded = (event) => {
                const db = event.target.result;
                // Create 'games' object store if it doesn't exist
                if (!db.objectStoreNames.contains('games')) {
                    db.createObjectStore('games', { keyPath: 'id' });
                }
                // Create 'teams' object store if it doesn't exist
                if (!db.objectStoreNames.contains('teams')) {
                    db.createObjectStore('teams', { keyPath: 'id' });
                }
                // Create 'game_stats' object store if it doesn't exist (added in v4)
                if (!db.objectStoreNames.contains('game_stats')) {
                    db.createObjectStore('game_stats', { keyPath: 'id' });
                }
            };

            request.onsuccess = ((e) => {
                this.db = e.target.result;
                resolve(this.db);
            }).bind(this);

            request.onerror = e => reject(e);
        });
    }

    /**
     * Closes the IndexedDB database connection.
     */
    close() {
        if (this.db) {
            this.db.close();
            this.db = null;
        }
    }

    /**
     * Deletes the entire IndexedDB database.
     * @async
     * @returns {Promise<void>}
     */
    async deleteDatabase() {
        this.close();
        return new Promise((resolve, reject) => {
            const req = indexedDB.deleteDatabase(this.dbName);
            req.onsuccess = () => resolve();
            req.onerror = () => reject(new Error('Failed to delete database'));
            req.onblocked = () => {
                console.warn('Delete database blocked. Please close other tabs.');
                reject(new Error('Database deletion blocked by other open tabs'));
            };
        });
    }

    /**
     * Saves game stats to the 'game_stats' object store.
     * @async
     * @param {string} id - Game ID.
     * @param {object} stats - The calculated stats object.
     */
    async saveGameStats(id, stats) {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve, reject) => {
            const tx = this.db.transaction(['game_stats'], 'readwrite');
            tx.objectStore('game_stats').put({ id, stats });
            tx.oncomplete = () => resolve();
            tx.onerror = e => reject(e);
        });
    }

    /**
     * Loads game stats by ID.
     * @async
     * @param {string} id
     * @returns {Promise<object|undefined>}
     */
    async getGameStats(id) {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve) => {
            const req = this.db.transaction(['game_stats'], 'readonly').objectStore('game_stats').get(id);
            req.onsuccess = (e) => resolve(e.target.result ? e.target.result.stats : undefined);
        });
    }

    /**
     * Retrieves all game stats.
     * @async
     * @returns {Promise<object[]>}
     */
    async getAllGameStats() {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve) => {
            const req = this.db.transaction(['game_stats'], 'readonly').objectStore('game_stats').getAll();
            req.onsuccess = (e) => {
                resolve((e.target.result || []));
            };
        });
    }

    /**
     * Deletes game stats by ID.
     * @async
     * @param {string} id
     */
    async deleteGameStats(id) {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve, reject) => {
            const tx = this.db.transaction(['game_stats'], 'readwrite');
            tx.objectStore('game_stats').delete(id);
            tx.oncomplete = () => resolve();
            tx.onerror = e => reject(e);
        });
    }

    /**
     * Saves a game object to the 'games' object store.
     * If the database is not open, it will attempt to open it first.
     * @async
     * @param {object} game - The game state object to save (will be converted to storage format).
     * @param {boolean} [dirty=true] - Whether the game has local changes not yet synced.
     * @returns {Promise<void>} A promise that resolves when the game is successfully saved.
     */
    async saveGame(game, dirty = true) {
        if (!this.db) {
            await this.open();
        }

        // Convert App State (Snapshot) to Storage Format.
        // Schema Version 3: Flat structure (matches Backend and Redux State).
        const storageObject = {
            schemaVersion: 3,
            ...game, // Spread all top-level props (id, date, away, home, roster, etc.)
            // Ensure actionLog is present (legacy states might miss it)
            actionLog: game.actionLog || [],
            _dirty: dirty,
        };

        return new Promise((resolve, reject) => {
            const tx = this.db.transaction(['games'], 'readwrite');
            tx.objectStore('games').put(storageObject);
            tx.oncomplete = () => resolve();
            tx.onerror = e => reject(e);
        });
    }

    /**
     * Loads a game object by its ID from the 'games' object store.
     * If the database is not open, it will attempt to open it first.
     * @async
     * @param {string} id - The ID of the game to load.
     * @returns {Promise<object|undefined>} A promise that resolves with the game object if found, otherwise undefined.
     */
    async loadGame(id) {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve) => {
            const req = this.db.transaction(['games'], 'readonly').objectStore('games').get(id);
            req.onsuccess = (e) => {
                const record = e.target.result;
                // Return as-is (flat structure)
                resolve(record);
            };
        });
    }

    /**
     * Retrieves all game objects from the 'games' object store.
     * If the database is not open, it will attempt to open it first.
     * @async
     * @returns {Promise<object[]>} A promise that resolves with an array of game summaries (metadata).
     */
    async getAllGames() {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve) => {
            const req = this.db.transaction(['games'], 'readonly').objectStore('games').getAll();
            req.onsuccess = (e) => {
                const rawRecords = e.target.result;
                const summaries = rawRecords.map(record => {
                    return {
                        id: record.id,
                        date: record.date,
                        location: record.location,
                        event: record.event,
                        away: record.away,
                        home: record.home,
                        awayTeamId: record.awayTeamId,
                        homeTeamId: record.homeTeamId,
                        status: record.status,
                        ownerId: record.ownerId,
                        _dirty: record._dirty, // Expose dirty flag
                    };
                });
                resolve(summaries);
            };
        });
    }

    /**
     * Retrieves all full game objects from the 'games' object store.
     * @async
     * @returns {Promise<object[]>}
     */
    async getAllFullGames() {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve) => {
            const req = this.db.transaction(['games'], 'readonly').objectStore('games').getAll();
            req.onsuccess = (e) => {
                resolve(e.target.result || []);
            };
        });
    }

    /**
     * Retrieves the last revision ID for all local games.
     * @async
     * @returns {Promise<Map<string, string>>} A promise that resolves to a map of GameID -> RevisionID.
     */
    async getLocalRevisions() {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve) => {
            const req = this.db.transaction(['games'], 'readonly').objectStore('games').getAll();
            req.onsuccess = (e) => {
                const rawRecords = e.target.result;
                const revisionMap = new Map();
                rawRecords.forEach(record => {
                    let rev = '';
                    if (record.actionLog && record.actionLog.length > 0) {
                        rev = record.actionLog[record.actionLog.length - 1].id;
                    }
                    revisionMap.set(record.id, rev);
                });
                resolve(revisionMap);
            };
        });
    }

    /**
     * Saves a team object to the 'teams' object store.
     * @async
     * @param {object} team - The team object to save.
     * @param {boolean} [dirty=true] - Whether the team has local changes not yet synced.
     * @returns {Promise<void>}
     */
    async saveTeam(team, dirty = true) {
        if (!this.db) {
            await this.open();
        }

        const storageObject = {
            schemaVersion: 3,
            ...team,
            _dirty: dirty,
        };

        return new Promise((resolve, reject) => {
            const tx = this.db.transaction(['teams'], 'readwrite');
            tx.objectStore('teams').put(storageObject);
            tx.oncomplete = () => resolve();
            tx.onerror = e => reject(e);
        });
    }

    /**
     * Retrieves all team objects from the 'teams' object store.
     * @async
     * @returns {Promise<object[]>}
     */
    async getAllTeams() {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve) => {
            const req = this.db.transaction(['teams'], 'readonly').objectStore('teams').getAll();
            req.onsuccess = (e) => {
                resolve(e.target.result || []);
            };
        });
    }

    /**
     * Marks an item as clean (synced) in the specified object store.
     * @async
     * @param {string} id - The ID of the item.
     * @param {string} storeName - The name of the object store ('games' or 'teams').
     * @returns {Promise<void>}
     */
    async markClean(id, storeName) {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve, reject) => {
            const tx = this.db.transaction([storeName], 'readwrite');
            const store = tx.objectStore(storeName);
            const req = store.get(id);

            req.onsuccess = (e) => {
                const record = e.target.result;
                if (record) {
                    record._dirty = false;
                    store.put(record);
                }
            };

            tx.oncomplete = () => resolve();
            tx.onerror = e => reject(e);
        });
    }

    /**
     * Deletes a team object by its ID from the 'teams' object store.
     * @async
     * @param {string} id - The ID of the team to delete.
     * @returns {Promise<void>}
     */
    async deleteTeam(id) {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve, reject) => {
            const tx = this.db.transaction(['teams'], 'readwrite');
            tx.objectStore('teams').delete(id);
            tx.oncomplete = () => resolve();
            tx.onerror = e => reject(e);
        });
    }

    /**
     * Deletes a game object by its ID from the 'games' object store.
     * @async
     * @param {string} id - The ID of the game to delete.
     * @returns {Promise<void>}
     */
    async deleteGame(id) {
        if (!this.db) {
            await this.open();
        }
        return new Promise((resolve, reject) => {
            const tx = this.db.transaction(['games'], 'readwrite');
            tx.objectStore('games').delete(id);
            tx.oncomplete = () => resolve();
            tx.onerror = e => reject(e);
        });
    }
}
