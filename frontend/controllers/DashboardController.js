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

import {
    SyncStatusSynced,
    SyncStatusUnsynced,
    SyncStatusLocalOnly,
    SyncStatusRemoteOnly,
} from '../constants.js';

export class DashboardController {
    constructor(app) {
        this.app = app;
    }

    /**
     * Loads the dashboard view by fetching and merging local and remote games.
     */
    async loadDashboard() {
        const localGames = await this.app.db.getAllGames();
        let remoteGames = [];
        const localRevisions = await this.app.db.getLocalRevisions();

        // Attempt to fetch remote games if user seems authenticated
        if (this.app.auth.getUser()) {
            try {
                const localIds = localGames.map(g => g.id);
                remoteGames = await this.app.sync.fetchGameList(localIds);
            } catch (e) {
                console.warn('Dashboard: Failed to fetch remote games', e);
                if (e.message && e.message.includes('status: 403')) {
                    this.app.modalConfirmFn(this.app.auth.accessDeniedMessage || 'Access Denied', { isError: true, autoClose: false });
                    return; // Stop loading dashboard
                }
            }
        }

        // Merge lists: Map by ID to deduplicate.
        const gameMap = new Map();

        // 1. Process remote games (including deletions)
        for (const g of remoteGames) {
            if (g.status === 'deleted') {
                console.log(`[Dashboard] Remote deletion detected for game ${g.id}. Deleting local copy.`);
                await this.app.db.deleteGame(g.id);
                // Also remove from localGames list so we don't re-add it below
                const idx = localGames.findIndex(lg => lg.id === g.id);
                if (idx !== -1) {
                    localGames.splice(idx, 1);
                }
                continue;
            }

            gameMap.set(g.id, {
                ...g,
                source: 'remote',
                remoteRevision: g.revision,
                localRevision: '',
                syncStatus: SyncStatusRemoteOnly,
            });
        }

        // 2. Overwrite/Add local games
        localGames.forEach(g => {
            const remoteG = gameMap.get(g.id);
            const localRev = localRevisions.get(g.id) || '';
            const remoteRev = remoteG ? remoteG.remoteRevision : '';

            let status = SyncStatusSynced;
            if (!remoteG) {
                status = SyncStatusLocalOnly;
            } else if (localRev === remoteRev) {
                status = SyncStatusSynced;
            } else {
                status = SyncStatusUnsynced;
            }

            gameMap.set(g.id, {
                ...g,
                source: 'local',
                localRevision: localRev,
                remoteRevision: remoteRev,
                syncStatus: status,
            });
        });

        // Filter out inaccessible games
        const allTeams = await this.app.db.getAllTeams();
        const allGames = Array.from(gameMap.values());
        const accessibleGames = allGames.filter(g => this.app.hasReadAccess(g, allTeams));

        this.app.state.games = accessibleGames;
        this.app.state.view = 'dashboard';
        this.app.render();

        // Trigger auto-sync for divergent games
        // We only auto-sync if we have a valid auth token
        if (this.app.auth.getUser()) {
            this.app.state.games.forEach(g => {
                if (g.syncStatus === SyncStatusUnsynced || g.syncStatus === SyncStatusLocalOnly) {
                    // this.app.syncGame(g.id); // Uncomment to enable true auto-sync
                }
            });
        }
    }
}
