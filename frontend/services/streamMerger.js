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
                await (this._fetching || this._fillRemoteBuffer());
                // If still empty after fill, and exhausted, break early
                if (this.remoteBuffer.length === 0 && this.remoteExhausted) {
                    if (this.localPointer >= this.localStream.length) {
                        break;
                    }
                }
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
                continue;
            }

            this.seenIds.add(winner[this.idKey]);

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

        if (this._fetching) {
            return this._fetching;
        }

        this._fetching = (async() => {
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
            } finally {
                this._fetching = null;
            }
        })();

        return this._fetching;
    }

    hasMore() {
        return this.localPointer < this.localStream.length ||
               this.remoteBuffer.length > 0 ||
               (!this.remoteExhausted && (this.remoteTotal === null || this.remoteOffset < this.remoteTotal));
    }
}
