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
    SyncStatusRemoteOnly,
    SyncStatusLocalOnly,
} from '../constants.js';

export class DashboardController {
    constructor(app) {
        this.app = app;
        this.page = 0;
        this.limit = 50;
        this.hasMore = false;
        this.isLoading = false;
        this.query = '';
    }

    /**
     * Loads the dashboard view.
     */
    async loadDashboard() {
        this.page = 0;
        this.hasMore = false;
        this.app.state.games = [];
        this.app.state.view = 'dashboard';

        await this.loadGames();
        this.renderWithPagination();
    }

    async loadGames() {
        if (this.isLoading) {
            return;
        }
        this.isLoading = true;

        const localGames = await this.app.db.getAllGames();
        const localRevisions = await this.app.db.getLocalRevisions();
        const localMap = new Map(localGames.map(g => [g.id, g]));

        let remoteGames = [];
        let total = 0;
        let isOffline = false;

        if (this.app.auth.getUser()) {
            try {
                const result = await this.app.sync.fetchGameList({
                    limit: this.limit,
                    offset: this.page * this.limit,
                    sortBy: 'date',
                    order: 'desc',
                    query: this.query,
                });
                remoteGames = result.data;
                total = result.meta.total;
            } catch (e) {
                console.warn('Dashboard: Failed to fetch remote games', e);
                if (e.message && e.message.includes('status: 403')) {
                    this.app.modalConfirmFn(this.app.auth.accessDeniedMessage || 'Access Denied', { isError: true, autoClose: false });
                    this.isLoading = false;
                    return;
                }
                isOffline = true;
            }
        } else {
            isOffline = true;
        }

        if (isOffline) {
            // Offline Mode: Filter, Sort, and Paginate local games
            let filtered = localGames;
            if (this.query) {
                const q = this.query.toLowerCase();
                filtered = localGames.filter(g =>
                    (g.event || '').toLowerCase().includes(q) ||
                    (g.location || '').toLowerCase().includes(q) ||
                    (g.away || '').toLowerCase().includes(q) ||
                    (g.home || '').toLowerCase().includes(q),
                );
            }

            // Sort (Date Descending to match backend default)
            filtered.sort((a, b) => new Date(b.date) - new Date(a.date));

            // Paginate
            const start = this.page * this.limit;
            const end = start + this.limit;
            const sliced = filtered.slice(start, end);

            if (this.page === 0) {
                this.app.state.games = sliced;
            } else {
                this.app.state.games.push(...sliced);
            }

            this.hasMore = end < filtered.length;
            this.isLoading = false;
            return;
        }

        // Online Mode: Merge Remote Page with Local Data
        const processedRemote = remoteGames.map(g => {
            const localG = localMap.get(g.id);
            const localRev = localRevisions.get(g.id) || '';
            const remoteRev = g.revision;

            let status = SyncStatusSynced;
            if (!localG) {
                status = SyncStatusRemoteOnly;
            } else if (localRev !== remoteRev) {
                status = SyncStatusUnsynced;
            }

            // Prefer local data if available (for unsynced changes), else remote data
            const base = localG || g;

            return {
                ...base,
                source: localG ? 'local' : 'remote',
                localRevision: localRev,
                remoteRevision: remoteRev,
                syncStatus: status,
            };
        });

        if (this.page === 0) {
            // MERGE Remote Page 0 with ALL Local Games (filtered) to ensure local-only games are visible
            const remoteIds = new Set(processedRemote.map(g => g.id));

            // Identify Local-Only (not in this remote page)
            let localOnly = localGames.filter(g => !remoteIds.has(g.id));

            // Filter Local by Query
            if (this.query) {
                const q = this.query.toLowerCase();
                localOnly = localOnly.filter(g =>
                    (g.event || '').toLowerCase().includes(q) ||
                    (g.location || '').toLowerCase().includes(q) ||
                    (g.away || '').toLowerCase().includes(q) ||
                    (g.home || '').toLowerCase().includes(q),
                );
            }

            const processedLocal = localOnly.map(g => ({
                ...g,
                source: 'local',
                localRevision: localRevisions.get(g.id) || '',
                remoteRevision: '',
                syncStatus: SyncStatusLocalOnly,
            }));

            // Combine
            const all = [...processedRemote, ...processedLocal];

            // Sort
            all.sort((a, b) => new Date(b.date) - new Date(a.date));

            this.app.state.games = all;
        } else {
            // Append Remote Page N
            const existingIds = new Set(this.app.state.games.map(g => g.id));
            const newUnique = processedRemote.filter(g => !existingIds.has(g.id));

            this.app.state.games.push(...newUnique);
            this.app.state.games.sort((a, b) => new Date(b.date) - new Date(a.date));
        }

        this.hasMore = (this.page + 1) * this.limit < total;
        this.isLoading = false;
    }

    async loadMore() {
        if (!this.hasMore || this.isLoading) {
            return;
        }
        this.page++;
        await this.loadGames();
        this.renderWithPagination();
    }

    /**
     * Search for games by query string.
     * @param {string} query
     */
    async search(query) {
        this.query = query;
        this.page = 0;
        this.app.state.games = [];
        this.hasMore = false;
        await this.loadGames();
        this.renderWithPagination();
    }

    renderWithPagination() {
        this.app.render();

        // Inject Load More Button
        if (this.hasMore) {
            // Actually DashboardRenderer takes 'container' as option.
            // We can find the container by checking app.router?
            // Or simpler: The app.render() renders into main content.
            // We can append to the bottom of the main content.
            const main = document.querySelector('main');
            if (main) {
                const btn = document.createElement('button');
                btn.className = 'w-full py-3 bg-gray-200 text-gray-700 font-bold rounded mt-4 hover:bg-gray-300';
                btn.textContent = this.isLoading ? 'Loading...' : 'Load More';
                btn.onclick = () => this.loadMore();
                main.appendChild(btn);
            }
        }
    }
}
