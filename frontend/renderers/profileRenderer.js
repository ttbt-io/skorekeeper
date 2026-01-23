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

/**
 * Renderer for the User Profile view.
 */
export class ProfileRenderer {
    constructor() {
        this.container = document.getElementById('profile-view');
        this.elements = {
            avatar: document.getElementById('profile-avatar-initial'),
            email: document.getElementById('profile-email'),
            status: document.getElementById('profile-status'),

            localGames: document.getElementById('profile-local-games'),
            localTeams: document.getElementById('profile-local-teams'),

            remoteStatsContent: document.getElementById('profile-remote-stats-content'),
            remoteStatsLocked: document.getElementById('profile-remote-stats-locked'),
            remoteGames: document.getElementById('profile-remote-games'),
            remoteTeams: document.getElementById('profile-remote-teams'),

            btnDeleteLocal: document.getElementById('btn-profile-delete-local'),
            btnDeleteRemote: document.getElementById('btn-profile-delete-remote'),
            remoteDeleteContainer: document.getElementById('profile-remote-delete-container'),
        };
    }

    /**
     * Renders the profile view with current user data and stats.
     * @param {object|null} user - The current user object.
     * @param {object} stats - Storage stats { local: {games, teams}, remote: {games, teams} }.
     */
    render(user, stats) {
        // Identity
        if (user) {
            this.elements.email.textContent = user.email;
            this.elements.status.textContent = 'Account Type: ' + (user.allowed ? 'Authorized' : 'Restricted');
            this.elements.avatar.textContent = user.email.charAt(0).toUpperCase();
            this.elements.avatar.className = 'w-16 h-16 bg-blue-100 rounded-full flex items-center justify-center text-2xl font-bold text-blue-600';
        } else {
            this.elements.email.textContent = 'Guest User';
            this.elements.status.textContent = 'Offline / Local Mode';
            this.elements.avatar.textContent = '?';
            this.elements.avatar.className = 'w-16 h-16 bg-slate-200 rounded-full flex items-center justify-center text-2xl font-bold text-slate-500';
        }

        // Local Stats
        this.elements.localGames.textContent = stats.local.games;
        this.elements.localTeams.textContent = stats.local.teams;

        // Remote Stats
        if (user) {
            this.elements.remoteStatsContent.classList.remove('hidden');
            this.elements.remoteStatsLocked.classList.add('hidden');
            this.elements.remoteGames.textContent = stats.remote.games !== null ? stats.remote.games : '--';
            this.elements.remoteTeams.textContent = stats.remote.teams !== null ? stats.remote.teams : '--';

            // Enable Remote Delete
            this.elements.remoteDeleteContainer.classList.remove('opacity-50');
            this.elements.btnDeleteRemote.disabled = false;
        } else {
            this.elements.remoteStatsContent.classList.add('hidden');
            this.elements.remoteStatsLocked.classList.remove('hidden');

            // Disable Remote Delete
            this.elements.remoteDeleteContainer.classList.add('opacity-50');
            this.elements.btnDeleteRemote.disabled = true;
        }
    }

    bindEvents(callbacks) {
        if (this.elements.btnDeleteLocal) {
            this.elements.btnDeleteLocal.onclick = callbacks.onDeleteLocal;
        }
        if (this.elements.btnDeleteRemote) {
            this.elements.btnDeleteRemote.onclick = callbacks.onDeleteRemote;
        }
        const backBtn = document.getElementById('btn-menu-profile');
        if (backBtn) {
            backBtn.onclick = callbacks.onBack;
        }
    }
}
