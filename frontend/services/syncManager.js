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

import { CurrentAppVersion, CurrentProtocolVersion, CurrentSchemaVersion } from '../constants.js';

/**
 * Manages WebSocket synchronization for the active game.
 */
export class SyncManager {
    constructor(app, onRemoteAction, onConflict, onError, onStatusChange) {
        this.app = app;
        this.onRemoteAction = onRemoteAction;
        this.onConflict = onConflict;
        this.onError = onError;
        this.onStatusChange = onStatusChange; // New callback for status updates
        this.socket = null;
        this.gameId = null;
        this.isConnected = false;
        this.queue = []; // Queue for actions while connecting
        this.lastRevision = ''; // ID of the last known synced action
        this.currentStatus = 'disconnected'; // Initial status

        // Keep-alive timers
        this.pingInterval = null;
        this.pongTimeout = null;
        this.PING_INTERVAL_MS = 25000; // Send a ping every 25 seconds
        this.PONG_TIMEOUT_MS = 5000;   // Expect a pong within 5 seconds

        // Reconnection logic
        this.reconnectTimer = null;
        this.reconnectAttempts = 0;
        this.MAX_RECONNECT_ATTEMPTS = 10;
        this.BASE_RECONNECT_DELAY_MS = 1000;
        this.shouldReconnect = false; // Flag to distinguish intentional disconnects

        // Optimistic UI tracking
        this.pendingActionIds = new Set();

        // HTTP Action Queue
        this.httpQueue = [];
        this.isHttpDraining = false;
        this.httpRetryCount = 0;
        this.isPaused = false;
        this.isSyncingHistory = false;
        this.queueGeneration = 0; // Generation counter to discard stale results
        this.isServerUnreachable = false;

        // Listen for online event to resume HTTP queue
        window.addEventListener('online', () => {
            console.log('[SyncManager] Online event detected');
            this.isServerUnreachable = false;
            if (this.shouldReconnect && !this.isConnected) {
                console.log('[SyncManager] Triggering immediate reconnect');
                this.connect(this.gameId, null);
            }
            this.processHttpQueue();
        });
    }

    /**
     * Pauses the outgoing HTTP action queue.
     */
    pause() {
        this.isPaused = true;
    }

    /**
     * Resumes the outgoing HTTP action queue and triggers processing.
     */
    resume() {
        this.isPaused = false;
        this.processHttpQueue();
    }

    /**
     * Updates the current status and notifies the listener.
     * @param {string} newStatus
     */
    setStatus(newStatus) {
        if (this.currentStatus !== newStatus) {
            this.currentStatus = newStatus;
            if (this.onStatusChange) {
                this.onStatusChange(newStatus);
            }
        }
    }

    /**
     * Helper to classify and handle fetch errors.
     * Updates sync status to 'disconnected' if the error indicates a network or server issue.
     * @param {Error} error
     */
    handleFetchError(error) {
        const isNetworkError = error instanceof TypeError;
        const isServerError = error.isServerError || (error.message && (error.message.includes('HTTP 5') || error.message.includes('Server returned 5')));

        if (isNetworkError || isServerError) {
            this.isServerUnreachable = true;
            this.setStatus('disconnected');
        }
    }

    /**
     * Fetches the list of games from the remote server.
     * @param {object} options - Pagination and filter options.
     * @param {number} [options.limit=50]
     * @param {number} [options.offset=0]
     * @param {string} [options.sortBy='date']
     * @param {string} [options.order='desc']
     * @param {string} [options.query='']
     * @returns {Promise<object>} A promise that resolves to { data: [], meta: {} }.
     */
    async fetchGameList({ limit = 50, offset = 0, sortBy = 'date', order = 'desc', query = '' } = {}) {
        if (!this.authCallback) {
            // No auth callback available to check login status
        }

        try {
            const params = new URLSearchParams({
                limit,
                offset,
                sortBy,
                order,
                q: query,
            });

            const response = await fetch(`/api/list-games?${params.toString()}`, {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json',
                },
            });

            if (!response.ok) {
                if (response.status === 403 || response.status === 401) {
                    const msg = await response.text().catch(() => '');
                    const error = new Error('status: 403');
                    error.message = 'status: 403, message: ' + (msg || 'Access denied');
                    throw error;
                }
                const error = new Error(`Server returned ${response.status}`);
                if (response.status >= 500) {
                    error.isServerError = true;
                }
                throw error;
            }

            const result = await response.json();
            // Fallback for old API (array)
            if (Array.isArray(result)) {
                return { data: result, meta: { total: result.length } };
            }
            return result;
        } catch (error) {
            console.warn('SyncManager: Error fetching remote game list:', error);
            this.handleFetchError(error);
            return { data: [], meta: { total: 0 } };
        }
    }

    /**
     * Checks if any of the provided game IDs have been deleted on the server.
     * @param {Array<string>} gameIds
     * @returns {Promise<Array<string>>} The list of deleted game IDs.
     */
    async checkGameDeletions(gameIds) {
        try {
            const response = await fetch('/api/check-deletions', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ gameIds: gameIds }),
            });
            if (!response.ok) {
                const error = new Error(`Server returned ${response.status}`);
                if (response.status >= 500) {
                    error.isServerError = true;
                }
                throw error;
            }
            const result = await response.json();
            return result.deletedGameIds || [];
        } catch (error) {
            console.warn('SyncManager: Error checking game deletions:', error);
            this.handleFetchError(error);
            return [];
        }
    }

    /**
     * Connects to the WebSocket for a specific game.
     * @param {string} gameId
     * @param {string} initialRevision - The ID of the last action in the local log.
     */
    connect(gameId, initialRevision = '') {
        if (this.app.state.activeGame &&
            this.app.state.activeGame.id === gameId &&
            gameId.startsWith('demo-')) {
            console.log('SyncManager: Skipping connection for demo game');
            this.setStatus('offline'); // Or 'local'
            return;
        }

        if (this.socket && this.gameId === gameId && this.isConnected) {
            return; // Already connected
        }

        // Clean up any existing connection or retry timers
        this.disconnect(false); // false = don't stop reconnecting, we are starting a new attempt

        this.shouldReconnect = true; // We want to be connected
        this.gameId = gameId;
        // Only update lastRevision if it's provided (not on auto-reconnect calls that might pass null)
        if (initialRevision !== null) {
            this.lastRevision = initialRevision;
        }

        this.setStatus('connecting');

        // Construct WS URL. Use window.location.host to match current origin.
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = `${protocol}//${window.location.host}/api/ws?gameId=${gameId}`;

        this.socket = new WebSocket(url);

        this.socket.onopen = () => {
            console.log('WS Connected');
            this.isConnected = true;
            this.setStatus('connected');
            this.reconnectAttempts = 0;// Reset counters
            this.isSyncingHistory = true; // Wait for JOIN ACK/SYNC_UPDATE

            this.flushQueue();

            // Calculate effective lastRevision for JOIN
            // If we have pending actions, we should join with the revision BEFORE those actions
            // to avoid false conflict detection on the server (which might not have received them yet).
            let effectiveLastRevision = this.lastRevision;
            if (this.httpQueue.length > 0 && this.httpQueue[0].baseRevision) {
                effectiveLastRevision = this.httpQueue[0].baseRevision;
                console.log('SyncManager: Joining with effective revision (pending queue)', effectiveLastRevision);
            }

            this.processHttpQueue();
            // Send JOIN message with our last known revision
            this.send({
                type: 'JOIN',
                gameId: this.gameId,
                lastRevision: effectiveLastRevision,
                appVersion: CurrentAppVersion,
                protocolVersion: CurrentProtocolVersion,
                schemaVersion: CurrentSchemaVersion,
            });
            this.startPinging(); // Start keep-alive pings
        };

        this.socket.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                this.handleMessage(msg);
            } catch (e) {
                console.error('WS Message Parse Error:', e);
            }
        };

        this.socket.onclose = () => {
            console.log('WS Disconnected');
            this.isConnected = false;
            this.socket = null;
            this.setStatus('disconnected');
            this.stopPinging();

            if (this.shouldReconnect) {
                this.scheduleReconnect();
            }
        };

        this.socket.onerror = (event) => {
            console.error('WS Connection Error:', event);
            this.setStatus('error');
            // onerror is usually followed by onclose, so we handle retry there.
            // But we ensure pinging stops.
            this.stopPinging();
        };

    }

    disconnect(stopRetrying = true) {
        if (stopRetrying) {
            this.shouldReconnect = false;
            if (this.reconnectTimer) {
                clearTimeout(this.reconnectTimer);
                this.reconnectTimer = null;
            }
        }

        this.isConnected = false;
        if (this.socket) {
            // Remove listeners to prevent "onclose" triggering more reconnects if we are intentionally closing
            this.socket.onclose = null;
            this.socket.onerror = null;
            this.socket.close();
            this.socket = null;
        }
        this.stopPinging(); // Ensure timers are cleared
    }

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.MAX_RECONNECT_ATTEMPTS) {
            console.error('WS: Max reconnect attempts reached.');
            this.setStatus('error'); // Or 'offline'
            if (this.onError) {
                this.onError('Connection lost. Please reload to try again.');
            }
            return;
        }

        // Adaptive reconnection jitter
        // algorithm: base * 1.5^attempts + random(min(10000, attempts * 1000))
        const baseDelay = this.BASE_RECONNECT_DELAY_MS * Math.pow(1.5, this.reconnectAttempts);
        const jitterWindow = Math.min(10000, this.reconnectAttempts * 1000);
        const delay = baseDelay + (Math.random() * jitterWindow);

        console.log(`WS: Reconnecting in ${Math.round(delay)}ms (Attempt ${this.reconnectAttempts + 1})`);

        this.reconnectTimer = setTimeout(() => {
            this.reconnectAttempts++;
            this.connect(this.gameId, null); // Keep existing lastRevision
        }, delay);
    }

    startPinging() {
        this.stopPinging(); // Clear any existing timers
        this.pingInterval = setInterval(() => {
            if (this.isConnected) {
                this.send({ type: 'PING' });
                this.pongTimeout = setTimeout(() => {
                    console.warn('WS Pong timeout, disconnecting...');
                    if (this.socket) {
                        this.socket.close(); // This will trigger onclose -> reconnect
                    }
                }, this.PONG_TIMEOUT_MS);
            }
        }, this.PING_INTERVAL_MS);
    }

    stopPinging() {
        if (this.pingInterval) {
            clearInterval(this.pingInterval);
            this.pingInterval = null;
        }
        if (this.pongTimeout) {
            clearTimeout(this.pongTimeout);
            this.pongTimeout = null;
        }
    }

    resetPongTimeout() {
        if (this.pongTimeout) {
            clearTimeout(this.pongTimeout);
            this.pongTimeout = null;
        }
    }

    sendAction(action) {
        // We use our last known revision as the base for this new action.
        // Optimistically, we assume we are up to date.
        const msg = {
            type: 'ACTION',
            action: action,
            baseRevision: this.lastRevision,
            schemaVersion: CurrentSchemaVersion,
        };

        // Track this action as pending to prevent "reverting" our lastRevision
        // when we receive the broadcast back from the server.
        if (action.id) {
            this.pendingActionIds.add(action.id);
            // Optimistically update our last revision to this action's ID
            this.lastRevision = action.id;
        }

        this.enqueueHttp(msg);
    }

    /**
     * Resolves a fast-forward conflict by re-sending actions that the server missed.
     * @param {string} serverHeadRev - The revision ID the server is currently at.
     * @param {Array<object>} actions - The list of actions to replay.
     */
    resolveFastForward(serverHeadRev, actions) {
        console.log(`[SyncManager] resolveFastForward: replaying ${actions.length} actions from serverHeadRev=${serverHeadRev}`);
        this.httpQueue = []; // Clear pending queue
        this.queueGeneration++; // Invalidate any in-flight requests
        let currentBase = serverHeadRev;

        actions.forEach(action => {
            const msg = {
                type: 'ACTION',
                action: action,
                baseRevision: currentBase,
            };

            if (action.id) {
                this.pendingActionIds.add(action.id);
                // Update base for next action in chain
                currentBase = action.id;
            }

            this.enqueueHttp(msg);
        });

        // Update local head to the end of the replayed chain
        this.lastRevision = currentBase;
    }

    enqueueHttp(msg) {
        this.httpQueue.push(msg);
        this.processHttpQueue();
    }

    async processHttpQueue() {
        if (this.app.debug) {
            console.log(`[SyncManager] processHttpQueue: isOnline=${this.app.isOnline}, isSyncingHistory=${this.isSyncingHistory}, isHttpDraining=${this.isHttpDraining}, isConnected=${this.isConnected}, queue=${this.httpQueue.length}`);
        }
        if (!this.app.isOnline || this.isSyncingHistory || this.isHttpDraining || !this.isConnected) {
            return;
        }

        const currentGeneration = this.queueGeneration;
        this.isHttpDraining = true;
        let continueDraining = false;

        // Ensure we have a gameId
        if (!this.gameId && this.app.state && this.app.state.activeGame) {
            this.gameId = this.app.state.activeGame.id;
        }
        if (!this.gameId) {
            console.warn('[SyncManager] Cannot process queue: missing gameId');
            this.isHttpDraining = false;
            return;
        }

        // Action Batching
        // Drain the entire queue into a single batch request
        // Limit batch size to 100 to match server constraints
        const batchSize = Math.min(this.httpQueue.length, 100);
        const batchMessages = this.httpQueue.splice(0, batchSize);
        if (batchMessages.length === 0) {
            this.isHttpDraining = false;
            return;
        }

        let msg;
        if (batchMessages.length === 1) {
            msg = batchMessages[0];
            msg.gameId = this.gameId;
        } else {
            // Note: baseRevision is taken from the first message in the batch.
            // For sequential actions (A->B->C), checking the first base against the server's head
            // is sufficient to validate the entire chain.
            msg = {
                type: 'ACTION',
                gameId: this.gameId,
                actions: batchMessages.map(m => m.action),
                baseRevision: batchMessages[0].baseRevision,
                schemaVersion: CurrentSchemaVersion,
            };
        }

        try {
            const response = await fetch('/api/action', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(msg),
            });

            if (this.queueGeneration !== currentGeneration) {
                console.log('[SyncManager] Discarding stale HTTP response');
                return;
            }

            if (response.ok) {
                const respMsg = await response.json();
                this.handleMessage(respMsg);
                this.httpRetryCount = 0; // Reset on success
                continueDraining = true;
            } else if (response.status === 429) {
                const retryAfter = response.headers.get('Retry-After');
                const seconds = parseInt(retryAfter || '5', 10);
                console.warn(`Server busy (429), retrying after ${seconds}s`);

                // Put the batch back at the front of the queue
                this.httpQueue.unshift(...batchMessages);
                this.isHttpDraining = false;
                setTimeout(() => this.processHttpQueue(), seconds * 1000);
                return; // Exit without setting continueDraining
            } else if (response.status === 409) {
                const errData = await response.json();
                this.handleMessage(errData);
                this.httpRetryCount = 0; // Reset on application error
                continueDraining = true;
            } else if (response.status === 403 || response.status === 401) {
                this.handleMessage({ type: 'ERROR', error: 'Unauthenticated' });
                this.disconnect(true); // Stop all retries
                this.httpRetryCount = 0;
                continueDraining = true;
            } else {
                throw new Error('HTTP ' + response.status);
            }
        } catch (e) {
            if (this.queueGeneration !== currentGeneration) {
                console.log('[SyncManager] Discarding stale HTTP error');
                return;
            }
            console.error('HTTP Action Failed', e);
            this.handleFetchError(e);

            // Put the batch back at the front of the queue
            this.httpQueue.unshift(...batchMessages);

            this.isHttpDraining = false;
            this.httpRetryCount++;
            const retryDelay = Math.min(30000, 2000 * Math.pow(1.5, this.httpRetryCount - 1));
            setTimeout(() => this.processHttpQueue(), retryDelay);
            return; // Exit without setting continueDraining
        } finally {
            if (this.queueGeneration === currentGeneration) {
                this.isHttpDraining = false;
                if (continueDraining && this.app.isOnline && this.httpQueue.length > 0) {
                    this.processHttpQueue();
                }
            }
        }
    }

    send(msg) {
        if (this.isConnected && this.socket && this.socket.readyState === WebSocket.OPEN) {
            this.socket.send(JSON.stringify(msg));
        } else {
            this.queue.push(msg);
        }
    }

    flushQueue() {
        while (this.queue.length > 0 && this.isConnected && this.socket && this.socket.readyState === WebSocket.OPEN) {
            const msg = this.queue.shift();
            // Note: msg.baseRevision was set when sendAction was called.
            // This is correct because it reflects the state *at that time*.
            this.send(msg);
        }
    }

    handleMessage(msg) {
        // Reset pong timeout on any message, indicating connection is active
        this.resetPongTimeout();

        switch (msg.type) {
            case 'ACK':
                console.log('WS ACK received');
                this.isSyncingHistory = false;
                // Only mark clean if we have no more pending actions for this game
                if (this.pendingActionIds.size === 0) {
                    this.setStatus('synced');
                    this.app.db.markClean(this.gameId, 'games');
                }
                this.processHttpQueue();
                break;

            case 'PONG': // Handle explicit PONG response to our PING
                console.log('WS PONG received');
                // Timeout already reset by resetPongTimeout() above
                break;

            case 'ACTION':
                // Remote action broadcasted from server
                if (msg.action && this.onRemoteAction) {
                    const actionId = msg.action.id;

                    // Check if this is an echo of our own action
                    if (actionId && this.pendingActionIds.has(actionId)) {
                        console.log('WS: Received echo of local action', actionId);
                        this.pendingActionIds.delete(actionId);
                        // Only update lastRevision if we are now fully synced to avoid reverting optimistic head
                        if (this.pendingActionIds.size === 0) {
                            this.lastRevision = actionId;
                        }
                    } else {
                        // Truly remote action
                        // Only update lastRevision if we are NOT waiting for pending actions.
                        // If we are pending, our lastRevision is ahead (optimistic).
                        if (actionId && this.pendingActionIds.size === 0) {
                            this.lastRevision = actionId;
                        }
                        this.onRemoteAction(msg.action);
                    }

                    // If we have no more pending actions, we are fully synced
                    if (this.pendingActionIds.size === 0) {
                        this.setStatus('synced');
                        this.app.db.markClean(this.gameId, 'games');
                    }
                }
                break;

            case 'SYNC_UPDATE':
                // Server sent missing actions (catch-up)
                if (msg.actions && this.onRemoteAction) {
                    msg.actions.forEach(a => {
                        if (a.id) {
                            // If catch-up includes our own actions (rare but possible), clear them
                            if (this.pendingActionIds.has(a.id)) {
                                this.pendingActionIds.delete(a.id);
                            }
                            // Only update lastRevision if we are not waiting for pending actions
                            if (this.pendingActionIds.size === 0) {
                                this.lastRevision = a.id;
                            }
                        }
                        this.onRemoteAction(a);
                    });
                    this.isSyncingHistory = false;
                    this.setStatus('synced');
                    this.processHttpQueue();
                }
                break;

            case 'CONFLICT':
                console.warn('WS Sync Conflict:', msg);
                this.isSyncingHistory = false;
                this.setStatus('conflict');
                if (this.onConflict) {
                    this.onConflict(msg);
                }
                break;

            case 'ERROR':
                console.error('WS Error:', msg.error);
                this.isSyncingHistory = false;
                if (msg.error.includes('Unauthenticated') || msg.error.includes('Login required')) {
                    this.setStatus('error'); // Pause sync
                    // Trigger an auth check
                    if (this.app && this.app.auth) {
                        this.app.auth.checkStatus().then(user => {
                            if (user && !this.app.auth.isStale) {
                                console.log('SyncManager: Auth recovered, retrying connection...');
                                this.disconnect(false);
                                this.connect(this.gameId, this.lastRevision);
                            } else {
                                console.warn('SyncManager: Auth required. Waiting for login...');
                                let attempt = 0;
                                const poll = () => {
                                    const delay = Math.min(30000, 2000 * Math.pow(1.5, attempt++));
                                    setTimeout(() => {
                                        if (!this.gameId) {
                                            return;
                                        }
                                        this.app.auth.checkStatus().then(u => {
                                            if (u && !this.app.auth.isStale) {
                                                console.log('SyncManager: Login detected. Resuming...');
                                                this.disconnect(false);
                                                this.connect(this.gameId, this.lastRevision);
                                            } else {
                                                poll();
                                            }
                                        });
                                    }, delay);
                                };
                                poll();
                            }
                        });
                    }
                } else if (msg.error.includes('throttled')) {
                    // Back off and check status
                    this.setStatus('connecting');
                    setTimeout(() => {
                        this.send({
                            type: 'JOIN',
                            gameId: this.gameId,
                            lastRevision: this.lastRevision,
                        });
                    }, 1000);
                } else {
                    this.setStatus('error');
                    if (this.onError) {
                        this.onError(msg.error);
                    }
                    // For generic errors (e.g. Validation, Server Error), our local state might be invalid/rejected.
                    // We disconnect and reconnect to force a sync check (JOIN) which will trigger CONFLICT/Catch-up if needed.
                    console.warn('SyncManager: Action rejected/failed. Reconnecting to ensure sync.');
                    this.disconnect(false);
                    // Add delay to prevent tight loops
                    setTimeout(() => {
                        this.connect(this.gameId, this.lastRevision);
                    }, 1000);
                }
                break;

            default:
                console.warn('Unknown WS message', msg);
        }
    }
}