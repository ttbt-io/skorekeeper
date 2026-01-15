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

import { CurrentAppVersion, CurrentSchemaVersion } from '../constants.js';

/**
 * Manages Backup and Restore operations for Skorekeeper.
 * Handles JSONL stream generation and parsing.
 */
export class BackupManager {
    /**
     * @param {import('./dbManager.js').DBManager} dbManager
     * @param {import('./syncManager.js').SyncManager} syncManager
     * @param {import('./teamSyncManager.js').TeamSyncManager} teamSyncManager
     */
    constructor(dbManager, syncManager, teamSyncManager) {
        this.db = dbManager;
        this.sync = syncManager;
        this.teamSync = teamSyncManager;
    }

    /**
     * Generates a backup stream.
     * @param {object} options
     * @param {boolean} options.games - Include games.
     * @param {boolean} options.teams - Include teams.
     * @param {boolean} options.remote - Fetch remote data.
     * @param {function(string):void} [onProgress] - Callback for status updates.
     * @returns {ReadableStream} A stream of JSONL text.
     */
    async getBackupStream(options, onProgress) {
        const self = this;
        const encoder = new TextEncoder();

        return new ReadableStream({
            async start(controller) {
                const push = (str) => controller.enqueue(encoder.encode(str + '\n'));

                try {
                    // Header
                    push(JSON.stringify({
                        type: 'header',
                        version: 1,
                        appVersion: CurrentAppVersion,
                        schemaVersion: CurrentSchemaVersion,
                        timestamp: Date.now(),
                        source: 'Skorekeeper',
                        userAgent: navigator.userAgent,
                    }));

                    // 1. Teams
                    if (options.teams) {
                        if (onProgress) {
                            onProgress('Gathering teams...');
                        }
                        const teams = await self._getAllTeams(options.remote);
                        let count = 0;
                        for (const team of teams) {
                            if (options.teamFilter && !options.teamFilter(team)) {
                                continue;
                            }
                            count++;
                            if (onProgress && count % 5 === 0) {
                                onProgress(`Processing team ${count}`);
                            }

                            const record = {
                                type: 'team',
                                id: team.id,
                                data: team,
                            };
                            push(JSON.stringify(record));
                        }
                    }

                    // 2. Games
                    if (options.games) {
                        if (onProgress) {
                            onProgress('Gathering games...');
                        }
                        const games = await self._getAllGames(options.remote);
                        let count = 0;
                        for (const gameMeta of games) {
                            if (options.gameFilter && !options.gameFilter(gameMeta)) {
                                continue;
                            }
                            count++;
                            if (onProgress) {
                                onProgress(`Processing game ${count}`);
                            }

                            try {
                                const fullGame = await self._fetchFullGame(gameMeta.id);
                                if (fullGame) {
                                    const record = {
                                        type: 'game',
                                        id: fullGame.id,
                                        summary: {
                                            date: fullGame.date,
                                            away: fullGame.away,
                                            home: fullGame.home,
                                            event: fullGame.event,
                                            status: fullGame.status,
                                        },
                                        data: fullGame,
                                    };
                                    push(JSON.stringify(record));
                                }
                            } catch (e) {
                                console.error(`Failed to backup game ${gameMeta.id}`, e);
                            }
                        }
                    }

                    if (onProgress) {
                        onProgress('Backup complete.');
                    }
                    controller.close();
                } catch (e) {
                    console.error('Backup generation failed', e);
                    controller.error(e);
                }
            },
        });
    }

    /**
     * Scans a backup file and returns a manifest of items.
     * @param {File} file
     * @returns {Promise<Array<object>>} List of items {type, id, summary/name}.
     */
    async scanBackupFile(file) {
        const items = [];
        const reader = this._getJsonLineReader(file);

        let result;
        while ((result = await reader.next())) {
            if (result.done) {
                break;
            }
            if (result.error) {
                console.warn('Backup Scan: Skipping corrupted line', result.error);
                continue;
            }
            const json = result.value;

            if (json.type === 'game') {
                items.push({
                    type: 'game',
                    id: json.id,
                    summary: json.summary || { away: '?', home: '?', date: 0 },
                });
            } else if (json.type === 'team') {
                items.push({
                    type: 'team',
                    id: json.id,
                    name: json.data.name || 'Unknown Team',
                    shortName: json.data.shortName,
                });
            }
        }
        return items;
    }

    /**
     * Restores selected items from the backup file.
     * @param {File} file
     * @param {Set<string>} selectedIds
     * @param {function(string):void} [onProgress]
     * @returns {Promise<{success: number, errors: number}>}
     */
    async restoreBackup(file, selectedIds, onProgress) {
        const reader = this._getJsonLineReader(file);
        let success = 0;
        let errors = 0;
        let totalProcessed = 0;

        let result;
        while ((result = await reader.next())) {
            if (result.done) {
                break;
            }
            if (result.error) {
                console.error('Backup Restore: Corrupted record skipped', result.error);
                errors++;
                continue;
            }
            const json = result.value;

            if (json.type === 'header') {
                continue;
            }

            if (selectedIds.has(json.id)) {
                totalProcessed++;
                if (onProgress) {
                    onProgress(`Restoring item ${totalProcessed}...`);
                }

                try {
                    if (!json.data || !json.data.id) {
                        throw new Error('Invalid data structure: missing id or data block');
                    }

                    if (json.type === 'game') {
                        // Basic Game Validation
                        if (!Array.isArray(json.data.actionLog)) {
                            throw new Error('Invalid game data: missing actionLog');
                        }
                        await this.db.saveGame(json.data);
                        // Trigger remote save if possible/online?
                        // We leave that to the user accessing it or auto-sync logic later.
                    } else if (json.type === 'team') {
                        // Basic Team Validation
                        if (!json.data.name) {
                            throw new Error('Invalid team data: missing name');
                        }
                        await this.db.saveTeam(json.data);
                    }
                    success++;
                } catch (e) {
                    console.error(`Failed to restore ${json.type} ${json.id}`, e);
                    errors++;
                }
            }
        }
        return { success, errors };
    }

    // --- Helpers ---

    async _getAllTeams(includeRemote) {
        const localTeams = await this.db.getAllTeams();
        const teamMap = new Map();
        localTeams.forEach(t => teamMap.set(t.id, t));

        if (includeRemote) {
            try {
                const remoteTeams = await this.teamSync.fetchTeamList();
                remoteTeams.forEach(t => {
                    // Prefer remote or local?
                    // For backup, we probably want the latest.
                    // If we have a local copy, check timestamps if available?
                    // Simple logic: Remote overwrites local if we assume server is authority.
                    // But for "Backup", maybe we want to backup what we see on screen?
                    // Let's assume merge: Remote fills gaps. Local changes might be pending.
                    if (!teamMap.has(t.id)) {
                        teamMap.set(t.id, t);
                    }
                });
            } catch (e) {
                console.warn('Backup: Failed to fetch remote teams', e);
            }
        }
        return Array.from(teamMap.values());
    }

    async _getAllGames(includeRemote) {
        const localGames = await this.db.getAllGames();
        const gameMap = new Map();
        localGames.forEach(g => gameMap.set(g.id, g));

        if (includeRemote) {
            try {
                const remoteGames = await this.sync.fetchGameList();
                remoteGames.forEach(g => {
                    if (!gameMap.has(g.id)) {
                        gameMap.set(g.id, g);
                    }
                });
            } catch (e) {
                console.warn('Backup: Failed to fetch remote games', e);
            }
        }
        return Array.from(gameMap.values());
    }

    async _fetchFullGame(id) {
        // 1. Try Local
        let game = await this.db.loadGame(id);

        // 2. If missing, fetch Remote
        if (!game) {
            // Validate id
            if (!/^[0-9a-fA-F-]{36}$/.test(id) && !id.startsWith('demo-')) {
                console.warn(`Backup: Invalid game ID format ${id}`);
                return null;
            }

            try {
                const response = await fetch(`/api/load/${encodeURIComponent(id)}`);
                if (response.ok) {
                    game = await response.json();
                }
            } catch (e) {
                console.warn(`Backup: Could not fetch game ${id}`, e);
            }
        }
        return game;
    }

    /**
     * Creates an async generator to read JSON lines from a File.
     * @param {File} file
     * @returns {{next: () => Promise<{value: any, done: boolean}>}}
     */
    _getJsonLineReader(file) {
        const textStream = file.stream().pipeThrough(new TextDecoderStream());
        const reader = textStream.getReader();
        let buffer = '';

        return {
            next: async() => {
                while (true) {
                    const newlineIndex = buffer.indexOf('\n');
                    if (newlineIndex !== -1) {
                        const line = buffer.slice(0, newlineIndex);
                        buffer = buffer.slice(newlineIndex + 1);
                        if (!line.trim()) {
                            continue;
                        }
                        try {
                            return { value: JSON.parse(line), done: false };
                        } catch (e) {
                            return { error: e, raw: line, done: false };
                        }
                    }

                    const { value, done } = await reader.read();
                    if (done) {
                        if (buffer.trim()) {
                            const line = buffer;
                            buffer = '';
                            try {
                                return { value: JSON.parse(line), done: false };
                            } catch (e) {
                                return { error: e, raw: line, done: false };
                            }
                        }
                        return { done: true };
                    }
                    buffer += value;
                }
            },
        };
    }
}
