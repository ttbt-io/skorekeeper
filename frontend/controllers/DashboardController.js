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
import { parseQuery } from '../utils/searchParser.js';
import { PullToRefresh } from '../ui/pullToRefresh.js';

export class DashboardController {
    constructor(app) {
        this.app = app;
        this.localBuffer = [];
        this.remoteBuffer = [];
        this.localRevisions = new Map();
        this.localMap = new Map();

        this.isLoading = false;
        this.isFetchingRemote = false;

        this.query = '';
        this.scrollBound = false;

        // Pagination state
        this.displayLimit = 50; // Initial limit (~2x screen height)
        this.remoteOffset = 0;
        this.remoteHasMore = true;
    }

    /**
     * Loads the dashboard view.
     * Starts async loading of local and remote data.
     * Returns immediately after initial setup.
     * @param {boolean} [force=false] - If true, clears local state and forces a fresh load.
     */
    async loadDashboard(force = false) {
        this.app.state.games = [];
        this.app.state.view = 'dashboard';

        // Reset state
        this.localBuffer = [];
        this.remoteBuffer = [];
        this.localRevisions = new Map();
        this.localMap = new Map();
        this.displayLimit = 50;
        this.remoteOffset = 0;
        this.remoteHasMore = true;

        this.app.render(); // Render empty state immediately
        this.bindScrollEvent();

        // Start independent async streams
        const localLoadPromise = this.loadAllLocalGames();

        // Only fetch remote if logged in and not filtering for local-only
        const parsedQ = parseQuery(this.query);
        const isLocalOnly = parsedQ.filters.some(f => f.key === 'is' && f.value === 'local');
        if (this.app.auth.getUser() && !isLocalOnly) {
            // Reset remote offset if forcing refresh
            if (force) {
                this.remoteOffset = 0;
                this.remoteHasMore = true;
            }
            this.fetchNextRemoteBatch();
            localLoadPromise.then(() => this.checkDeletions());
        }
    }

    bindScrollEvent() {
        const container = document.getElementById('game-list-container');
        if (!container) {
            return;
        }

        if (!this.scrollBound) {
            container.addEventListener('scroll', () => {
                this.handleScroll(container);
            });
            this.scrollBound = true;
        }

        // Initialize PullToRefresh
        if (!container.dataset.ptrInitialized) {
            new PullToRefresh(container, async() => {
                await this.loadDashboard(true);
            });
            container.dataset.ptrInitialized = 'true';
        }
    }

    handleScroll(container) {
        const { scrollTop, scrollHeight, clientHeight } = container;
        // Trigger when within 200px of bottom
        if (scrollTop + clientHeight >= scrollHeight - 200) {
            this.displayLimit += 20; // Show more

            // Check if we need more remote data
            // Simple heuristic: if we are displaying nearly everything we have, ask for more
            const totalLoaded = this.localBuffer.length + this.remoteBuffer.length;
            if (this.remoteHasMore && !this.isFetchingRemote && this.displayLimit > totalLoaded - 20) {
                this.fetchNextRemoteBatch();
            } else {
                this.mergeAndRender();
            }
        }
    }

    async loadAllLocalGames() {
        this.isLoading = true;
        try {
            // Fetch revisions first for sync status
            this.localRevisions = await this.app.db.getLocalRevisions();

            let localGames = await this.app.db.getAllGames();

            // Pre-process local games for search filtering
            const parsedQ = parseQuery(this.query);
            const isRemoteOnly = parsedQ.filters.some(f => f.key === 'is' && f.value === 'remote');

            if (isRemoteOnly) {
                this.localBuffer = [];
            } else if (this.query) {
                this.localBuffer = localGames.filter(g => this._matchesGame(g, parsedQ));
            } else {
                this.localBuffer = localGames;
            }

            // Populate local map for merging logic
            this.localMap = new Map(this.localBuffer.map(g => [g.id, g]));
        } finally {
            this.isLoading = false;
            this.mergeAndRender(); // Ensure loading indicator is cleared
        }
    }

    async fetchNextRemoteBatch() {
        if (this.isFetchingRemote || !this.remoteHasMore) {
            return;
        }
        this.isFetchingRemote = true;
        this.mergeAndRender(); // Show loading spinner

        try {
            const result = await this.app.sync.fetchGameList({
                limit: 50, // Fetch a chunk
                offset: this.remoteOffset,
                sortBy: 'date',
                order: 'desc',
                query: this.query,
            });

            if (result && result.data) {
                this.remoteBuffer.push(...result.data);
                this.remoteOffset += result.data.length;
                if (result.data.length < 50) {
                    this.remoteHasMore = false;
                }
            } else {
                this.remoteHasMore = false;
            }
            this.mergeAndRender();
        } catch (e) {
            console.error('Failed to fetch remote games', e);
            this.remoteHasMore = false; // Stop trying on error
        } finally {
            this.isFetchingRemote = false;
            this.mergeAndRender();
        }
    }

    async checkDeletions() {
        try {
            // Get local IDs to check against server
            const allLocal = await this.app.db.getAllGames();
            const localIds = allLocal.map(g => g.id);
            if (localIds.length === 0) {
                return;
            }

            const deletedIds = await this.app.sync.checkGameDeletions(localIds);
            if (deletedIds.length > 0) {
                console.log('Dashboard: Deleting stale local games', deletedIds);
                const deletedSet = new Set(deletedIds);

                // 1. Delete from DB
                await Promise.all(deletedIds.map(id =>
                    this.app.db.deleteGame(id).catch(err => console.warn(`Dashboard: Failed to delete stale game ${id}`, err)),
                ));

                // 2. Remove from Local Buffer
                this.localBuffer = this.localBuffer.filter(g => !deletedSet.has(g.id));
                this.localMap = new Map(this.localBuffer.map(g => [g.id, g])); // Rebuild map

                // 3. Remove from Remote Buffer (if present, though unlikely if server says deleted)
                this.remoteBuffer = this.remoteBuffer.filter(g => !deletedSet.has(g.id));

                this.mergeAndRender();
            }
        } catch (e) {
            console.warn('Check deletions failed', e);
        }
    }

    mergeAndRender() {
        // 1. Deduplicate & Merge
        // Strategy: Use a Map. Remote items overwrite Local items (latest metadata).
        // But we must preserve the 'source' logic.
        const merged = new Map();

        // Add Local First
        this.localBuffer.forEach(g => {
            merged.set(g.id, { ...g, _source: 'local' });
        });

        // Add/Update Remote
        this.remoteBuffer.forEach(g => {
            const existing = merged.get(g.id);
            if (existing) {
                // Update existing local item with remote data, but mark as having both
                merged.set(g.id, { ...g, _source: 'both', _localGame: existing });
            } else {
                // New remote item
                merged.set(g.id, { ...g, _source: 'remote' });
            }
        });

        // 2. Convert to Array & Process Status
        const processed = Array.from(merged.values()).map(item => {
            let status = SyncStatusSynced;
            const localItem = item._source === 'local' ? item : item._localGame;
            const remoteItem = item._source === 'remote' || item._source === 'both' ? item : null;

            // Re-resolve revisions
            // Note: If it came from local buffer, it has revision. If from remote, it has revision.
            const localRev = localItem ? (this.localRevisions.get(localItem.id) || '') : '';
            const remoteRev = remoteItem ? remoteItem.revision : '';
            const isDirty = localItem && localItem._dirty;

            if (item._source === 'remote') {
                status = SyncStatusRemoteOnly;
            } else if (isDirty) {
                if (item._source === 'local') {
                    status = SyncStatusLocalOnly;
                } else {
                    status = SyncStatusUnsynced;
                }
            } else if (item._source === 'local') {
                // Local only, not dirty.
                // If offline, treat as cached (Synced).
                // If online, it might be a new local item that was marked clean? (Unlikely)
                // Or a zombie.
                if (!navigator.onLine) {
                    status = SyncStatusSynced;
                } else {
                    status = SyncStatusLocalOnly;
                }
            } else {
                // Both exist and !dirty
                status = SyncStatusSynced;
            }

            // Prefer local item if unsynced (show pending changes), otherwise prefer remote (latest source of truth)
            const base = (status === SyncStatusUnsynced || status === SyncStatusLocalOnly) ? localItem : (remoteItem || localItem);

            return {
                ...base,
                source: localItem ? 'local' : 'remote',
                localRevision: localRev,
                remoteRevision: remoteRev,
                syncStatus: status,
            };
        });

        // 3. Sort
        // Date Descending
        processed.sort((a, b) => new Date(b.date) - new Date(a.date));

        // 4. Slice
        this.app.state.games = processed.slice(0, this.displayLimit);

        // 5. Render
        this.renderWithPagination();
    }

    renderWithPagination() {
        this.app.render();

        const main = document.getElementById('game-list-container');
        if (!main) {
            return;
        }

        // Clean up existing sentinels
        const existing = main.querySelectorAll('.pagination-sentinel');
        existing.forEach(el => el.remove());

        // Ensure we show loading or end-of-list state
        const totalAvailable = this.localBuffer.length + this.remoteBuffer.length; // Approximate
        const showingAll = this.app.state.games.length >= totalAvailable && !this.remoteHasMore;

        let sentinelText = '';
        if (this.isLoading || this.isFetchingRemote) {
            sentinelText = 'Loading more games...';
        } else if (!showingAll) {
            sentinelText = 'Scroll for more';
        } else if (this.app.state.games.length > 0) {
            sentinelText = 'All games loaded.';
        }

        if (sentinelText) {
            const sentinel = document.createElement('div');
            sentinel.className = 'pagination-sentinel py-4 text-center text-gray-500 text-sm font-medium';
            sentinel.textContent = sentinelText;
            main.appendChild(sentinel);
        }
    }

    async search(query) {
        this.query = query;
        await this.loadDashboard();
    }

    _matchesGame(g, parsedQ) {
        // 1. Free Text (AND)
        for (const token of parsedQ.tokens) {
            const t = token.toLowerCase();
            const match = (g.event || '').toLowerCase().includes(t) ||
                          (g.location || '').toLowerCase().includes(t) ||
                          (g.away || '').toLowerCase().includes(t) ||
                          (g.home || '').toLowerCase().includes(t);
            if (!match) {
                return false;
            }
        }
        // 2. Filters
        for (const f of parsedQ.filters) {
            if (f.key === 'is') {
                continue;
            }
            const val = f.value.toLowerCase();

            if (f.key === 'event' && !(g.event || '').toLowerCase().includes(val)) {
                return false;
            }
            if (f.key === 'location' && !(g.location || '').toLowerCase().includes(val)) {
                return false;
            }
            if (f.key === 'away' && !(g.away || '').toLowerCase().includes(val)) {
                return false;
            }
            if (f.key === 'home' && !(g.home || '').toLowerCase().includes(val)) {
                return false;
            }

            if (f.key === 'date') {
                const d = g.date || '';
                if (f.operator === '=') {
                    if (!d.startsWith(f.value)) {
                        return false;
                    }
                }
                else if (f.operator === '>=') {
                    if (!(d >= f.value)) {
                        return false;
                    }
                }
                else if (f.operator === '<=') {
                    if (!(d <= f.value)) {
                        return false;
                    }
                }
                else if (f.operator === '>') {
                    if (!(d > f.value)) {
                        return false;
                    }
                }
                else if (f.operator === '<') {
                    if (!(d < f.value)) {
                        return false;
                    }
                }
                else if (f.operator === '..') {
                    // Inclusive range: use ~ to make upper bound cover suffixes
                    const maxVal = f.maxValue + '~';
                    if (!(d >= f.value && d <= maxVal)) {
                        return false;
                    }
                }
            }
        }
        return true;
    }
}
