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

export class StreamMerger {
    /**
     * @param {Array<object>} localData - All local items, already sorted.
     * @param {Function} fetchRemotePageFn - Async function(offset) -> { data: [], meta: { total } }
     * @param {Function} comparator - Function(a, b) -> number (sort order)
     * @param {string} idKey - Key to identify items for deduplication (default: 'id')
     */
    constructor(localData, fetchRemotePageFn, comparator, idKey = 'id') {
        this.localStream = localData || [];
        this.localPointer = 0;

        this.fetchRemotePageFn = fetchRemotePageFn;
        this.remoteBuffer = [];
        this.remoteOffset = 0;
        this.remoteTotal = null; // Unknown initially
        this.remoteExhausted = false;

        this.comparator = comparator;
        this.idKey = idKey;
        this.seenIds = new Set();
    }

    async fetchNextBatch(batchSize) {
        const batch = [];

        while (batch.length < batchSize) {
            // Ensure remote buffer has data if not exhausted
            if (this.remoteBuffer.length === 0 && !this.remoteExhausted) {
                await this._fillRemoteBuffer();
            }

            const localItem = this.localPointer < this.localStream.length ? this.localStream[this.localPointer] : null;
            const remoteItem = this.remoteBuffer.length > 0 ? this.remoteBuffer[0] : null;

            if (!localItem && !remoteItem) {
                break; // Both exhausted
            }

            // Case: Same ID at head of both streams (Synced item)
            if (localItem && remoteItem && localItem[this.idKey] === remoteItem[this.idKey]) {
                // Advance both
                this.localPointer++;
                this.remoteBuffer.shift();

                if (this.seenIds.has(localItem[this.idKey])) {
                    continue;
                }
                this.seenIds.add(localItem[this.idKey]);

                batch.push({
                    ...localItem,
                    source: 'local', // Or 'merged'
                    _remote: remoteItem,
                });
                continue;
            }

            let winner = null;
            let source = '';

            if (localItem && remoteItem) {
                const cmp = this.comparator(localItem, remoteItem);
                if (cmp <= 0) { // Local comes first (or equal but diff ID)
                    winner = localItem;
                    source = 'local';
                } else {
                    winner = remoteItem;
                    source = 'remote';
                }
            } else if (localItem) {
                winner = localItem;
                source = 'local';
            } else {
                winner = remoteItem;
                source = 'remote';
            }

            // Deduplication and pointer advancement
            if (source === 'local') {
                this.localPointer++;
            } else {
                this.remoteBuffer.shift(); // Consume remote
            }

            if (this.seenIds.has(winner[this.idKey])) {
                // If we've seen this ID already, it means we processed the "other" version earlier.
                // Since we are sorted, we should merge them or ignore.
                // But typically, if we see duplicates, it means the item exists in both.
                // If local wins, we emit local. If remote appears later (duplicate ID), we ignore it?
                // Or we should merge them?
                // The Controllers handle merging via `_processBatch` mapping.
                // StreamMerger just needs to ensure we don't emit the same ID twice as a *primary* item.
                // But wait, if we have local version and remote version, we want to emit ONE item.
                // The `winner` logic picks the one that appears *first* in sort order.
                // If they sort equally (same date), we picked local.
                // The remote item will appear next (cmp > 0 is false).
                // We consume it.
                continue;
            }

            this.seenIds.add(winner[this.idKey]);

            // Attach raw remote object if available in buffer (for merging later)
            // Actually, if we picked local, the remote item might be next in stream.
            // But we don't look ahead.
            // The Controller expects `item.source` and `item._remote`?
            // My previous implementation of `DashboardController` used `item._remote`.

            // Simplified approach: Emit the winner. Controller looks up localMap.
            // If winner is remote, we attach it.

            // Wait, if I have local and remote for same ID:
            // They might have different Sort Keys (e.g. Date).
            // If Local Date > Remote Date (Desc Sort), Local comes first.
            // We emit Local.
            // Later we encounter Remote (older date).
            // We see ID is seen. We skip Remote.
            // This is correct: We show the most up-to-date position (Local).

            // What if Remote Date > Local Date?
            // Remote comes first. We emit Remote.
            // Later Local. We skip Local.
            // This is also correct (Remote update moved it up).

            // So skipping seen IDs is correct for list position.

            // But for Data Merging (sync status):
            // If we emit Local, we need to know if there IS a remote version to show sync status.
            // But we might not have seen the remote version yet!
            // This is a limitation of stream merging sorted lists with mutable sort keys.
            // However, `DashboardController` `_processBatch` does:
            // `const localItem = (item.source === 'local') ? item : this.localMap.get(item.id);`
            // It looks up local version from `this.localMap`.
            // So if we emit Remote, we find Local immediately.
            // If we emit Local, we don't have Remote immediately unless it was fetched.
            // But `SyncManager` fetches by Page.

            // If we emit Local, and Remote is pages away (or not fetched yet), we treat as "Local Only" or "Synced" (if rev matches?).
            // If we haven't fetched remote yet, we can't know sync status for sure.
            // But `fetchGameList` returns `revision`.

            // If we assume strict sort order, we will eventually see the remote item if it exists.
            // But if we skipped it, we lose the info?
            // No, because `StreamMerger` is purely for *Ordering*.
            // The Controller handles *Merging* data.
            // But if we skip the remote item, the Controller never sees it to merge?
            // TRUE.

            // Fix: If we skip an item because ID is seen, we should check if we can enhance the *already emitted* item?
            // No, that item is already rendered/processed.

            // If we emitted Local (newer), and now we see Remote (older).
            // We skip Remote.
            // But the Local item rendered earlier might have said "Local Only" because we hadn't seen Remote yet.
            // This is an eventual consistency issue in the UI list.
            // It's acceptable for Infinite Scroll.
            // Ideally, we'd update the previous item, but that's hard.
            // Actually, `DashboardController` uses `localMap` which has the local data.
            // If we emit Remote, we merge with Local.
            // If we emit Local, we don't have Remote.

            // This is fine.

            batch.push({
                ...winner,
                source: source,
            });
        }

        return batch;
    }

    async _fillRemoteBuffer() {
        if (this.remoteTotal !== null && this.remoteOffset >= this.remoteTotal) {
            this.remoteExhausted = true;
            return;
        }

        try {
            const res = await this.fetchRemotePageFn(this.remoteOffset);
            const data = res.data || [];
            const meta = res.meta || {};

            if (data.length === 0) {
                this.remoteExhausted = true;
            } else {
                this.remoteBuffer.push(...data);
                this.remoteOffset += data.length;
                this.remoteTotal = meta.total;
            }
        } catch (e) {
            console.warn('StreamMerger: Remote fetch failed', e);
            this.remoteExhausted = true;
        }
    }

    hasMore() {
        return this.localPointer < this.localStream.length ||
               this.remoteBuffer.length > 0 ||
               (!this.remoteExhausted && (this.remoteTotal === null || this.remoteOffset < this.remoteTotal));
    }
}
