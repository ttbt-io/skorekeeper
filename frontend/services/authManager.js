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

import { whoami } from '/.sso/proxy.mjs';

/**
 * Manages user authentication state and interactions with the SSO endpoints.
 */
export class AuthManager {
    constructor() {
        this.currentUser = null;
        this.isStale = false;
        this.localId = localStorage.getItem('scorekeeper_local_id');
        if (!this.localId) {
            if (typeof crypto !== 'undefined' && crypto.randomUUID) {
                this.localId = crypto.randomUUID();
            } else {
                this.localId = 'local-' + Date.now() + '-' + Math.floor(Math.random() * 1000000);
            }
            localStorage.setItem('scorekeeper_local_id', this.localId);
        }

        try {
            const cached = localStorage.getItem('scorekeeper_user');
            if (cached) {
                this.currentUser = JSON.parse(cached);
            }
        } catch (e) {
            console.warn('Failed to load cached user', e);
        }
    }

    /**
     * Checks the current authentication status with the backend.
     * @returns {Promise<object|null>} The user object if logged in, or null.
     */
    async checkStatus() {
        console.log('AuthManager: Checking status...');
        try {
            const info = await whoami();
            if (info && info.email) {
                // Identity known, set immediately so UI can show it even if denied by policy later
                this.currentUser = info;
                this.isStale = false;
                localStorage.setItem('scorekeeper_user', JSON.stringify(info));

                // Secondary check: Are we allowed by policy?
                // We hit /api/me which returns user info and quota/allowed status.
                try {
                    const meRes = await fetch('/api/me');
                    let meData = {};
                    const contentType = meRes.headers.get('content-type');
                    if (contentType && contentType.includes('application/json')) {
                        meData = await meRes.json().catch(() => ({}));
                    } else if (meRes.status === 403) {
                        // Fallback for plain text error from backend
                        const text = await meRes.text().catch(() => '');
                        meData = { message: text.replace('Forbidden: ', '') };
                    }

                    if (!meRes.ok) {
                        if (meRes.status === 403) {
                            this.accessDenied = true;
                            this.accessDeniedMessage = meData.message || 'Access to this service is restricted by policy.';
                            const error = new Error('status: 403');
                            error.message = 'status: 403, message: ' + this.accessDeniedMessage;
                            throw error;
                        }
                        throw new Error(`Server returned ${meRes.status}`);
                    }

                    // Check explicit allowed flag from API (now returns 200 OK)
                    if (meData.allowed !== true) {
                        this.accessDenied = true;
                        this.accessDeniedMessage = meData.message || 'Access to this service is restricted by policy.';
                        const error = new Error('Access Denied');
                        // Preserve the "status: 403" string for downstream handlers
                        error.message = 'status: 403, message: ' + this.accessDeniedMessage;
                        throw error;
                    }

                    // Policy allows access
                    this.quotaInfo = meData.quotas;
                    this.currentUser = { ...this.currentUser, ...meData.quotas };
                    localStorage.setItem('scorekeeper_user', JSON.stringify(this.currentUser));
                } catch (policyErr) {
                    if (policyErr.message && policyErr.message.includes('status: 403')) {
                        throw policyErr;
                    }
                    console.warn('Policy check failed (network):', policyErr);
                    // On network error during /api/me, we maintain current session but it might fail later
                }
            } else {

                // Server says "Anonymous" (or explicit null).
                // If we have a cached user, we are now stale.
                if (this.currentUser) {
                    this.isStale = true;
                }
                console.warn('AuthManager: Session expired or invalid, but keeping local identity.');
            }
            return this.currentUser;
        } catch (error) {
            if (error.message.includes('status: 403')) {
                this.accessDenied = true;
                this.accessDeniedMessage = 'Access to this service is restricted by policy.';
                if (error.message.includes('message:')) {
                    this.accessDeniedMessage = error.message.split('message: ')[1];
                }
                throw error;
            }
            console.error('Auth check failed (Network or Server Error):', error);
            // On network error, we don't know if we are stale or just offline.
            // We assume we are NOT stale (maintain current state) but effectively offline.
            return this.currentUser;
        }
    }

    /**
     * Initiates the login flow.
     */
    login() {
        const width = Math.floor(window.innerWidth * 0.7);
        const height = Math.floor(window.innerHeight * 0.7);
        const left = Math.floor(window.innerWidth * 0.15);
        const top = Math.floor(window.innerHeight * 0.15);
        window.open('/api/login', '_blank', 'popup=true,menubar=false,location=false,toolbar=false,status=false,width='+width+',height='+height+',left='+left+',top='+top+'');
    }

    /**
     * Logs the user out.
     */
    async logout() {
        try {
            await fetch('/.sso/logout', { method: 'POST' });
        } catch (error) {
            console.error('Logout failed:', error);
        } finally {
            this.currentUser = null;
            localStorage.removeItem('scorekeeper_user');
            // Refresh to clear state/UI
            window.location.reload();
        }
    }

    /**
     * Returns the currently cached user object.
     * @returns {object|null}
     */
    getUser() {
        return this.currentUser;
    }

    getLocalId() {
        return this.localId;
    }
}