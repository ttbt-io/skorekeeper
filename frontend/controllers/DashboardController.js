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
import { StreamMerger } from '../services/streamMerger.js';

export class DashboardController {
    constructor(app) {
        this.app = app;
        this.merger = null;
        this.localMap = new Map();
        this.localRevisions = new Map();
        this.isLoading = false;
        this.query = '';
        this.scrollBound = false;
        this.batchSize = 20;
    }

    /**
     * Loads the dashboard view.
     */
    async loadDashboard() {
        this.app.state.games = [];
        this.app.state.view = 'dashboard';
        this.isLoading = true;

        // 1. Prepare Local Data
        const localGames = await this.app.db.getAllGames();
        this.localRevisions = await this.app.db.getLocalRevisions();
        this.localMap = new Map(localGames.map(g => [g.id, g]));

        // Filter and Sort Local Data (Date Descending)
        let filteredLocal = localGames;
        if (this.query) {
            const q = this.query.toLowerCase();
            filteredLocal = localGames.filter(g =>
                (g.event || '').toLowerCase().includes(q) ||
                (g.location || '').toLowerCase().includes(q) ||
                (g.away || '').toLowerCase().includes(q) ||
                (g.home || '').toLowerCase().includes(q),
            );
        }
        filteredLocal.sort((a, b) => new Date(b.date) - new Date(a.date));

        // 2. Initialize StreamMerger
        this.merger = new StreamMerger(
            filteredLocal,
            async(offset) => {
                if (!this.app.auth.getUser()) {
                    return { data: [], meta: { total: 0 } };
                }
                return this.app.sync.fetchGameList({
                    limit: 50,
                    offset: offset,
                    sortBy: 'date',
                    order: 'desc',
                    query: this.query,
                });
            },
            (a, b) => new Date(b.date) - new Date(a.date), // Comparator: Date Desc
            'id',
        );

        // 3. Bind Scroll Event
        this.bindScrollEvent();

        // 4. Initial Auto-Fill
        await this.autoFill();
        this.isLoading = false;
    }

    bindScrollEvent() {
        const container = document.getElementById('game-list-container');
        if (container && !this.scrollBound) {
            container.addEventListener('scroll', () => {
                this.handleScroll(container);
            });
            this.scrollBound = true;
        }
    }

    async handleScroll(container) {
        if (this.isLoading || !this.merger || !this.merger.hasMore()) {
            return;
        }

        const { scrollTop, scrollHeight, clientHeight } = container;
        if (scrollTop + clientHeight >= scrollHeight - 200) {
            await this.loadNextBatch();
        }
    }

    async autoFill() {
        const container = document.getElementById('game-list-container');
        await this.loadNextBatch();

        if (container && container.clientHeight > 0) {
            let safety = 0;
            while (
                container.scrollHeight <= container.clientHeight * 2 &&
                this.merger.hasMore() &&
                safety < 10
            ) {
                await this.loadNextBatch();
                safety++;
            }
        }
    }

    async loadNextBatch() {
        if (!this.merger) {
            return;
        }
        this.isLoading = true;

        try {
            const rawBatch = await this.merger.fetchNextBatch(this.batchSize);
            const processedBatch = this._processBatch(rawBatch);

            this.app.state.games.push(...processedBatch);
            this.app.render();

            this.hasMore = this.merger.hasMore();
        } finally {
            this.isLoading = false;
        }
        this.renderWithPagination();
    }

    _processBatch(batch) {
        return batch.map(item => {
            const remoteItem = item._remote;
            const localItem = (item.source === 'local') ? item : this.localMap.get(item.id);
            const base = localItem || item;

            let status = SyncStatusSynced;
            const localRev = localItem ? (this.localRevisions.get(localItem.id) || '') : '';
            const remoteRev = remoteItem ? remoteItem.revision : (item.source === 'remote' ? item.revision : '');

            if (!localItem) {
                status = SyncStatusRemoteOnly;
            } else if (!remoteItem && item.source === 'local') {
                status = SyncStatusLocalOnly;
            } else if (localRev !== remoteRev) {
                status = SyncStatusUnsynced;
            }

            return {
                ...base,
                source: localItem ? 'local' : 'remote',
                localRevision: localRev,
                remoteRevision: remoteRev,
                syncStatus: status,
            };
        });
    }

    async search(query) {
        this.query = query;
        await this.loadDashboard();
    }

    renderWithPagination() {
        this.app.render();

        const main = document.querySelector('main');
        if (!main || this.app.state.games.length === 0) {
            return;
        }

        if (this.hasMore || this.isLoading) {
            const sentinel = document.createElement('div');
            sentinel.className = 'py-4 text-center text-gray-500 text-sm font-medium';
            sentinel.textContent = this.isLoading ? 'Loading more games...' : 'Scroll for more';
            main.appendChild(sentinel);
        } else {
            const endMsg = document.createElement('div');
            endMsg.className = 'py-8 text-center text-gray-400 text-xs italic';
            endMsg.textContent = 'All games loaded.';
            main.appendChild(endMsg);
        }
    }

    // Legacy support for manual Load More click if needed
    async loadMore() {
        if (!this.hasMore || this.isLoading) {
            return;
        }
        await this.loadNextBatch();
    }
}