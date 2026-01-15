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

import { SyncManager } from '../../frontend/services/syncManager.js';

describe('SyncManager Rate Limiting & Batching', () => {
    let syncManager;
    let mockApp;
    let mockOnRemoteAction;
    let mockOnConflict;
    let mockOnError;
    let mockOnStatusChange;

    beforeEach(() => {
        mockApp = {
            isOnline: true,
            debug: true,
            state: { activeGame: { id: 'game-1' } },
            dbManager: {
                savePendingAction: jest.fn(),
                getPendingActions: jest.fn().mockResolvedValue([]),
                deletePendingAction: jest.fn(),
            },
            historyManager: {
                getHeadRevision: jest.fn().mockReturnValue('r1'),
            },
            handleSyncAction: jest.fn(),
        };

        mockOnRemoteAction = jest.fn();
        mockOnConflict = jest.fn();
        mockOnError = jest.fn();
        mockOnStatusChange = jest.fn();

        syncManager = new SyncManager(
            mockApp,
            mockOnRemoteAction,
            mockOnConflict,
            mockOnError,
            mockOnStatusChange,
        );
        syncManager.gameId = 'test-game';
        syncManager.isConnected = true;
        syncManager.isSyncingHistory = false;
        window.fetch = jest.fn();
    });

    test('processHttpQueue should batch multiple actions', async() => {
        syncManager.httpQueue.push({ type: 'ACTION', action: { id: 'a1' }, baseRevision: 'r0' });
        syncManager.httpQueue.push({ type: 'ACTION', action: { id: 'a2' }, baseRevision: 'r0' });

        window.fetch.mockResolvedValueOnce({
            ok: true,
            json: async() => ({ type: 'ACK' }),
        });

        await syncManager.processHttpQueue();

        expect(window.fetch).toHaveBeenCalledTimes(1);
        const body = JSON.parse(window.fetch.mock.calls[0][1].body);
        expect(body.actions).toBeDefined();
        expect(body.actions.length).toBe(2);
    });

    test('processHttpQueue should respect isSyncingHistory', async() => {
        syncManager.isSyncingHistory = true;
        syncManager.httpQueue.push({ type: 'ACTION', action: { id: 'a1' }, baseRevision: 'r0' });

        await syncManager.processHttpQueue();

        expect(window.fetch).not.toHaveBeenCalled();
    });

    test('handleMessage(ACK) should clear isSyncingHistory and trigger queue', () => {
        syncManager.isSyncingHistory = true;
        const spy = jest.spyOn(syncManager, 'processHttpQueue');

        syncManager.handleMessage({ type: 'ACK' });

        expect(syncManager.isSyncingHistory).toBe(false);
        expect(spy).toHaveBeenCalled();
    });

    test('processHttpQueue should respect HTTP 429 and Retry-After', async() => {
        jest.useFakeTimers();
        const setTimeoutSpy = jest.spyOn(window, 'setTimeout');
        syncManager.httpQueue.push({ type: 'ACTION', action: { id: 'a1' }, baseRevision: 'r0' });

        window.fetch.mockResolvedValueOnce({
            status: 429,
            headers: {
                get: (name) => (name === 'Retry-After' ? '2' : null),
            },
            ok: false,
        });

        await syncManager.processHttpQueue();

        expect(syncManager.httpQueue.length).toBe(1);
        expect(setTimeoutSpy).toHaveBeenCalledWith(expect.any(Function), 2000);
        setTimeoutSpy.mockRestore();
        jest.useRealTimers();
    });

    test('processHttpQueue should implement exponential backoff on network failure', async() => {
        jest.useFakeTimers();
        const setTimeoutSpy = jest.spyOn(window, 'setTimeout');
        syncManager.httpQueue.push({ type: 'ACTION', action: { id: 'a1' }, baseRevision: 'r0' });

        window.fetch.mockRejectedValueOnce(new Error('Network Error'));

        await syncManager.processHttpQueue();

        expect(syncManager.httpQueue.length).toBe(1);
        expect(syncManager.httpRetryCount).toBe(1);
        // base (1000) * 1.5^1 = 1500ms
        expect(setTimeoutSpy).toHaveBeenCalledWith(expect.any(Function), expect.any(Number));
        const delay = setTimeoutSpy.mock.calls[0][1];
        expect(delay).toBeGreaterThanOrEqual(1500);

        // Mock success on second attempt
        window.fetch.mockResolvedValueOnce({
            ok: true,
            json: async() => ({ type: 'ACK' }),
        });

        jest.runAllTimers();
        // Wait for promises
        await Promise.resolve();
        await Promise.resolve();

        expect(window.fetch).toHaveBeenCalledTimes(2);
        setTimeoutSpy.mockRestore();
        jest.useRealTimers();
    });

    test('processHttpQueue should cap batches at 100 actions and continue draining', async() => {
        // Fill queue with 150 actions
        for (let i = 0; i < 150; i++) {
            syncManager.httpQueue.push({ type: 'ACTION', action: { id: 'large-' + i }, baseRevision: 'r0' });
        }

        window.fetch.mockResolvedValue({
            ok: true,
            json: async() => ({ type: 'ACK' }),
        });

        await syncManager.processHttpQueue();

        expect(window.fetch).toHaveBeenCalledTimes(2);

        // First batch should be 100
        const body1 = JSON.parse(window.fetch.mock.calls[0][1].body);
        expect(body1.actions.length).toBe(100);

        // Second batch should be 50
        const body2 = JSON.parse(window.fetch.mock.calls[1][1].body);
        expect(body2.actions.length).toBe(50);

        expect(syncManager.httpQueue.length).toBe(0);
    });

    test('lastRevision should not revert when receiving stale remote action if pending actions exist', () => {
        syncManager.lastRevision = 'rev-initial';

        // 1. Send Action A (Optimistic)
        const actionA = { id: 'rev-A' };
        syncManager.sendAction(actionA);
        expect(syncManager.lastRevision).toBe('rev-A');
        expect(syncManager.pendingActionIds.has('rev-A')).toBe(true);

        // 2. Receive Remote Action 'rev-initial' (Simulate stale/late broadcast)
        // With fix: lastRevision should remain 'rev-A' because pending is not empty
        syncManager.handleMessage({
            type: 'ACTION',
            action: { id: 'rev-initial' },
        });

        expect(syncManager.lastRevision).toBe('rev-A');
    });

    test('lastRevision should track pending tip even if confirmed middle action arrives', () => {
        syncManager.lastRevision = 'rev-initial';

        // 1. Send A, B
        syncManager.sendAction({ id: 'rev-A' });
        syncManager.sendAction({ id: 'rev-B' });

        expect(syncManager.lastRevision).toBe('rev-B');

        // 2. Receive Echo A
        syncManager.handleMessage({
            type: 'ACTION',
            action: { id: 'rev-A' },
        });

        // lastRevision should NOT revert to A. It should stay B.
        expect(syncManager.lastRevision).toBe('rev-B');
        expect(syncManager.pendingActionIds.has('rev-A')).toBe(false);
        expect(syncManager.pendingActionIds.has('rev-B')).toBe(true);
    });

    describe('WebSocket & Connection', () => {
        let MockWebSocket;

        beforeEach(() => {
            MockWebSocket = class {
                constructor(url) {
                    this.url = url;
                    this.send = jest.fn();
                    this.readyState = 1; // OPEN
                    setTimeout(() => this.onopen && this.onopen(), 0);
                }
                close() {
                    if (this.onclose) {
                        this.onclose();
                    }
                }
            };
            global.WebSocket = MockWebSocket;
            global.WebSocket.OPEN = 1;
        });

        afterEach(() => {
            if (syncManager.pingInterval) {
                clearInterval(syncManager.pingInterval);
            }
            if (syncManager.pongTimeout) {
                clearTimeout(syncManager.pongTimeout);
            }
        });

        test('connect() should establish websocket connection', async() => {
            syncManager.gameId = 'game-1';
            await syncManager.connect('game-1');
            expect(syncManager.socket).toBeInstanceOf(MockWebSocket);
            expect(syncManager.socket.url).toContain('/api/ws?gameId=game-1');
        });

        test('sendAction() queues action', () => {
            syncManager.isConnected = false;
            syncManager.gameId = 'game-1';
            syncManager.sendAction({ id: 'a1' });
            expect(syncManager.httpQueue.length).toBe(1);
        });

        test('handleMessage(SYNC_UPDATE) should dispatch remote actions', () => {
            const actions = [{ id: 'h1' }];
            syncManager.handleMessage({ type: 'SYNC_UPDATE', actions });
            expect(mockOnRemoteAction).toHaveBeenCalledWith(actions[0]);
        });

        test('handleMessage(ERROR) should log error', () => {
            const spy = jest.spyOn(console, 'error').mockImplementation(() => {
            });
            syncManager.handleMessage({ type: 'ERROR', error: 'Test Error' });
            expect(spy).toHaveBeenCalledWith('WS Error:', 'Test Error');
            spy.mockRestore();
        });

        test('handleMessage(CONFLICT) should trigger conflict resolution', () => {
            const conflictData = { type: 'CONFLICT', conflictType: 'FORK', commonAncestorId: 'anc', serverBranch: [] };
            syncManager.handleMessage(conflictData);
            expect(mockOnConflict).toHaveBeenCalledWith(conflictData);
        });
    });
});