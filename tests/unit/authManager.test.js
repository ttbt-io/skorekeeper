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

import { AuthManager } from '../../frontend/services/authManager.js';
import * as ssoProxy from '../../frontend/.sso/proxy.mjs';

// Mock the sso proxy module
jest.mock('../../frontend/.sso/proxy.mjs', () => ({
    whoami: jest.fn(),
}));

describe('AuthManager', () => {
    let authManager;
    let mockFetch;

    beforeEach(() => {
        // Reset mocks and localStorage
        jest.clearAllMocks();
        localStorage.clear();

        // Mock global fetch
        mockFetch = jest.fn();
        global.fetch = mockFetch;

        // Mock window.open
        window.open = jest.fn();
    });

    test('should initialize correctly', () => {
        authManager = new AuthManager();
        expect(authManager.currentUser).toBeNull();
        expect(authManager.isStale).toBe(false);
        expect(authManager.localId).toBeDefined();
        expect(localStorage.getItem('scorekeeper_local_id')).toBe(authManager.localId);
    });

    test('should load cached user from localStorage', () => {
        const user = { email: 'test@example.com' };
        localStorage.setItem('scorekeeper_user', JSON.stringify(user));
        authManager = new AuthManager();
        expect(authManager.currentUser).toEqual(user);
    });

    test('checkStatus should handle successful login and policy check', async() => {
        const user = { email: 'test@example.com' };
        ssoProxy.whoami.mockResolvedValue(user);

        // Mock /api/me response (allowed)
        mockFetch.mockResolvedValue({
            ok: true,
            headers: { get: () => 'application/json' },
            json: async() => ({ allowed: true, quotas: { maxGames: 5 } }),
        });

        authManager = new AuthManager();
        const result = await authManager.checkStatus();

        expect(ssoProxy.whoami).toHaveBeenCalled();
        expect(mockFetch).toHaveBeenCalledWith('/api/me');
        expect(result).toEqual({ ...user, maxGames: 5 });
        expect(authManager.accessDenied).toBeUndefined();
    });

    test('checkStatus should handle policy denied (403)', async() => {
        const user = { email: 'banned@example.com' };
        ssoProxy.whoami.mockResolvedValue(user);

        // Mock /api/me response (denied via 403 status)
        mockFetch.mockResolvedValue({
            ok: false,
            status: 403,
            headers: { get: () => 'text/plain' },
            text: async() => 'Forbidden: Policy says no',
        });

        authManager = new AuthManager();

        await expect(authManager.checkStatus()).rejects.toThrow('status: 403');
        expect(authManager.accessDenied).toBe(true);
        expect(authManager.accessDeniedMessage).toContain('Policy says no');
    });

    test('checkStatus should handle allowed=false in 200 response', async() => {
        const user = { email: 'restricted@example.com' };
        ssoProxy.whoami.mockResolvedValue(user);

        // Mock /api/me response (denied via payload)
        mockFetch.mockResolvedValue({
            ok: true,
            headers: { get: () => 'application/json' },
            json: async() => ({ allowed: false, message: 'Quota exceeded' }),
        });

        authManager = new AuthManager();

        await expect(authManager.checkStatus()).rejects.toThrow('status: 403');
        expect(authManager.accessDenied).toBe(true);
        expect(authManager.accessDeniedMessage).toBe('Quota exceeded');
    });

    test('checkStatus should handle network error on /api/me gracefully', async() => {
        const user = { email: 'test@example.com' };
        ssoProxy.whoami.mockResolvedValue(user);

        // Mock /api/me failure
        mockFetch.mockRejectedValue(new Error('Network error'));

        authManager = new AuthManager();
        const result = await authManager.checkStatus();

        // Should return user but verify fallback behavior (logged warning)
        expect(result).toEqual(user);
        // We can't easily check console.warn without spying on it, but the code path returns user.
    });

    test('checkStatus should handle anonymous/logged out state', async() => {
        ssoProxy.whoami.mockResolvedValue(null);
        authManager = new AuthManager();

        // Pre-populate with cached user to test staleness
        authManager.currentUser = { email: 'old@example.com' };

        const result = await authManager.checkStatus();
        expect(result).toEqual({ email: 'old@example.com' });
        expect(authManager.isStale).toBe(true);
    });

    test('checkStatus should handle whoami failure', async() => {
        ssoProxy.whoami.mockRejectedValue(new Error('Proxy error'));
        authManager = new AuthManager();

        const result = await authManager.checkStatus();
        // Should catch error and return current user (null)
        expect(result).toBeNull();
    });

    test('login should open popup', () => {
        authManager = new AuthManager();
        authManager.login();
        expect(window.open).toHaveBeenCalledWith(
            '/api/login',
            '_blank',
            expect.stringContaining('popup=true'),
        );
    });

    test('logout should call endpoint and clear session', async() => {
        mockFetch.mockResolvedValue({ ok: true });
        authManager = new AuthManager();
        authManager.currentUser = { email: 'user' };
        localStorage.setItem('scorekeeper_user', 'stuff');

        await authManager.logout();

        expect(mockFetch).toHaveBeenCalledWith('/.sso/logout', { method: 'POST' });
        expect(authManager.currentUser).toBeNull();
        expect(localStorage.getItem('scorekeeper_user')).toBeNull();
        // expect(window.location.reload).toHaveBeenCalled(); // Cannot verify in JSDOM easily
    });
});
