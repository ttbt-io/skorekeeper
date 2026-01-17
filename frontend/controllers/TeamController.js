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

import { Team } from '../models/Team.js';
import { generateUUID } from '../utils.js';
import {
    SyncStatusSynced,
    SyncStatusLocalOnly,
    SyncStatusSyncing,
    SyncStatusError,
} from '../constants.js';

export class TeamController {
    constructor(app) {
        this.app = app;
        this.teamState = null;
        this.page = 0;
        this.limit = 50;
        this.hasMore = false;
        this.isLoading = false;
        this.query = '';
    }

    /**
     * Loads the teams view by fetching all saved teams.
     */
    async loadTeamsView() {
        this.page = 0;
        this.hasMore = false;
        this.app.state.teams = [];
        this.query = ''; // Reset query when navigating to teams via menu
        await this.loadTeams();

        // Trigger auto-sync for local-only teams
        if (this.app.auth.getUser()) {
            this.app.state.teams.forEach((t) => {
                if (t.syncStatus === SyncStatusLocalOnly) {
                    this.syncTeam(t.id);
                }
            });
        }

        this.app.state.view = 'teams';
        window.location.hash = 'teams';
        this.renderWithPagination();
    }

    async loadTeams() {
        if (this.isLoading) {
            return;
        }
        this.isLoading = true;

        const localTeamsRaw = await this.app.db.getAllTeams();
        const localTeams = localTeamsRaw.filter(t => this.app.hasTeamReadAccess(t));
        const localMap = new Map(localTeams.map(t => [t.id, t]));

        let remoteTeams = [];
        let total = 0;
        let isOffline = false;

        if (this.app.auth.getUser()) {
            try {
                const result = await this.app.teamSync.fetchTeamList({
                    limit: this.limit,
                    offset: this.page * this.limit,
                    sortBy: 'name',
                    order: 'asc',
                    query: this.query,
                });
                remoteTeams = result.data;
                total = result.meta.total;
            } catch (e) {
                console.warn('TeamController: Failed to fetch remote teams', e);
                isOffline = true;
            }
        } else {
            isOffline = true;
        }

        if (isOffline) {
            // Offline Mode: Filter, Sort, Paginate local teams
            let filtered = localTeams;
            if (this.query) {
                const q = this.query.toLowerCase();
                filtered = localTeams.filter(t => (t.name || '').toLowerCase().includes(q));
            }
            filtered.sort((a, b) => (a.name || '').localeCompare(b.name || ''));

            const start = this.page * this.limit;
            const end = start + this.limit;
            const sliced = filtered.slice(start, end);

            if (this.page === 0) {
                this.app.state.teams = sliced;
            } else {
                this.app.state.teams.push(...sliced);
            }
            this.hasMore = end < filtered.length;
            this.isLoading = false;
            return;
        }

        // Online Mode: Merge
        // Handle deletions first
        for (const t of remoteTeams) {
            if (t.status === 'deleted') {
                await this.app.db.deleteTeam(t.id);
                // Remove from local list to prevent re-adding
                const idx = localTeams.findIndex(lt => lt.id === t.id);
                if (idx !== -1) {
                    localTeams.splice(idx, 1);
                }
            }
        }

        const activeRemote = remoteTeams.filter(t => t.status !== 'deleted');
        const processedRemote = activeRemote.map(t => {
            const localT = localMap.get(t.id);
            return {
                ...(localT || t),
                syncStatus: SyncStatusSynced,
            };
        });

        if (this.page === 0) {
            const remoteIds = new Set(processedRemote.map(t => t.id));
            const localOnly = localTeams.filter(t => !remoteIds.has(t.id));
            const processedLocal = localOnly.map(t => ({
                ...t,
                syncStatus: SyncStatusLocalOnly,
            }));

            const all = [...processedRemote, ...processedLocal];
            all.sort((a, b) => (a.name || '').localeCompare(b.name || ''));
            this.app.state.teams = all;
        } else {
            const existingIds = new Set(this.app.state.teams.map(t => t.id));
            const newUnique = processedRemote.filter(t => !existingIds.has(t.id));
            this.app.state.teams.push(...newUnique);
            this.app.state.teams.sort((a, b) => (a.name || '').localeCompare(b.name || ''));
        }

        this.hasMore = (this.page + 1) * this.limit < total;
        this.isLoading = false;

        // Persist non-deleted remote teams locally
        for (const t of activeRemote) {
            await this.app.db.saveTeam(t);
        }
    }

    async loadMore() {
        if (!this.hasMore || this.isLoading) {
            return;
        }
        this.page++;
        await this.loadTeams();
        this.renderWithPagination();
    }

    renderWithPagination() {
        this.app.render();
        if (this.hasMore) {
            const main = document.querySelector('#teams-view main');
            if (main) {
                const btn = document.createElement('button');
                btn.className = 'w-full py-3 bg-gray-200 text-gray-700 font-bold rounded mt-4 hover:bg-gray-300';
                btn.textContent = this.isLoading ? 'Loading...' : 'Load More Teams';
                btn.onclick = () => this.loadMore();
                main.appendChild(btn);
            }
        }
    }

    /**
     * Synchronizes a specific team with the server.
     * @param {string} teamId
     */
    async syncTeam(teamId) {
        const team = this.app.state.teams.find(t => t.id === teamId);
        if (!team || team.syncStatus === SyncStatusSyncing) {
            return;
        }

        const user = this.app.auth.getUser();
        if (!user) {
            console.warn('App: Cannot sync team without authentication');
            return;
        }

        // Optimistic status update
        team.syncStatus = SyncStatusSyncing;
        this.app.render();

        try {
            const success = await this.app.teamSync.saveTeam(team);
            if (success) {
                team.syncStatus = SyncStatusSynced;
            } else {
                team.syncStatus = SyncStatusError;
                if (this.app.auth.isStale) {
                    console.warn('App: Team sync failed due to stale session.');
                }
            }
        } catch (e) {
            console.error('App: Team sync error', e);
            team.syncStatus = SyncStatusError;
        }
        this.app.render();
    }

    /**
     * Opens the Team Modal for creating or editing a team.
     * @param {string|object|null} teamOrId - The team object or ID to edit, or null for a new team.
     */
    async openEditTeamModal(teamOrId = null) {
        const modal = document.getElementById('team-modal');
        const title = document.getElementById('team-modal-title');
        const idInput = document.getElementById('team-id');
        const nameInput = document.getElementById('team-name');
        const shortNameInput = document.getElementById('team-short-name');
        const colorInput = document.getElementById('team-color');
        const rosterContainer = document.getElementById('team-roster-container');

        if (rosterContainer) {
            rosterContainer.innerHTML = '';
        }
        this.switchTeamModalTab('roster'); // Default tab

        if (teamOrId) {
            let team = typeof teamOrId === 'object' ? teamOrId : null;
            const teamId = typeof teamOrId === 'string' ? teamOrId : (team ? team.id : null);

            if (!team && teamId) {
                // Validate teamId
                if (!/^[0-9a-fA-F-]{36}$/.test(teamId)) {
                    console.error('App: Invalid team ID format', teamId);
                    return;
                }

                const allTeamsData = await this.app.db.getAllTeams();
                const teamData = allTeamsData.find(t => t.id === teamId);

                if (!teamData) {
                    // Try fetching from server
                    try {
                        const response = await fetch(`/api/load-team/${encodeURIComponent(teamId)}`);
                        if (response.ok) {
                            const remoteTeamData = await response.json();
                            team = new Team(remoteTeamData);
                            await this.app.db.saveTeam(team.toJSON());
                        }
                    } catch (e) {
                        console.error('Failed to fetch remote team:', e);
                    }
                } else {
                    team = new Team(teamData);
                }
            } else if (team) {
                team = new Team(team);
            }

            if (!team) {
                console.error('Team not found:', teamId);
                return;
            }

            if (title) {
                title.textContent = 'Edit Team';
            }
            if (idInput) {
                idInput.value = team.id;
            }
            if (nameInput) {
                nameInput.value = team.name;
            }
            if (shortNameInput) {
                shortNameInput.value = team.shortName || '';
            }
            if (colorInput) {
                colorInput.value = team.color || '#2563eb';
            }
            if (team.roster && rosterContainer) {
                team.roster.forEach(p => this.app.teamsRenderer.renderTeamRow(rosterContainer, p));
            }

            // Initialize Team State for Role management
            this.teamState = {
                id: team.id,
                ownerId: team.ownerId || (this.app.state.currentUser ? this.app.state.currentUser.email : ''),
                roles: team.roles || { admins: [], scorekeepers: [], spectators: [] },
            };
        } else {
            if (title) {
                title.textContent = 'New Team';
            }
            if (idInput) {
                idInput.value = '';
            }
            if (nameInput) {
                nameInput.value = '';
            }
            if (shortNameInput) {
                shortNameInput.value = '';
            }
            if (colorInput) {
                colorInput.value = '#2563eb';
            }
            // Start with 9 empty rows for a new team
            if (rosterContainer) {
                for (let i = 0; i < 9; i++) {
                    this.app.teamsRenderer.renderTeamRow(rosterContainer);
                }
            }

            // Default State
            const email = this.app.state.currentUser ? this.app.state.currentUser.email : '';
            this.teamState = {
                id: '',
                ownerId: email,
                roles: { admins: [email], scorekeepers: [], spectators: [] },
            };
        }

        this.renderTeamMembers();
        if (modal) {
            modal.classList.remove('hidden');
        }
    }

    /**
     * Updates the UI tab for team members in the team modal.
     * @param {string} tab - The tab to switch to ('roster' or 'members').
     */
    switchTeamModalTab(tab) {
        const rosterView = document.getElementById('team-roster-view');
        const membersView = document.getElementById('team-members-view');
        const rosterTab = document.getElementById('tab-team-roster');
        const membersTab = document.getElementById('tab-team-members');

        if (!rosterView || !membersView || !rosterTab || !membersTab) {
            return;
        }

        if (tab === 'roster') {
            rosterView.classList.remove('hidden');
            membersView.classList.add('hidden');
            rosterTab.className = 'px-4 py-2 font-bold border-b-2 border-blue-600 text-blue-600';
            membersTab.className = 'px-4 py-2 font-bold border-b-2 border-transparent text-gray-500 hover:text-blue-600';
        } else {
            rosterView.classList.add('hidden');
            membersView.classList.remove('hidden');
            rosterTab.className = 'px-4 py-2 font-bold border-b-2 border-transparent text-gray-500 hover:text-blue-600';
            membersTab.className = 'px-4 py-2 font-bold border-b-2 border-blue-600 text-blue-600';
            this.renderTeamMembers();
        }
    }

    /**
     * Renders the members list for the currently editing team.
     */
    renderTeamMembers() {
        if (!this.teamState) {
            return;
        }

        if (!this.app.teamsRenderer.membersContainer) {
            this.app.teamsRenderer.membersContainer = document.getElementById('team-members-container');
        }
        this.app.teamsRenderer.renderTeamMembers(this.teamState, this.app.auth.getUser());
    }

    /**
     * Adds a new member to the teamState and re-renders the list.
     */
    addTeamMember() {
        const emailInput = document.getElementById('member-invite-email');
        const roleSelect = document.getElementById('member-invite-role');
        const email = emailInput.value.trim().toLowerCase();
        const role = roleSelect.value;

        if (!email || !this.app.validate(email, 100, 'Email')) {
            return;
        }
        if (!email.includes('@')) {
            this.app.modalConfirmFn('Please enter a valid email address.', { isError: true });
            return;
        }

        // Check for duplicates
        const allMembers = [
            ...this.teamState.roles.admins,
            ...this.teamState.roles.scorekeepers,
            ...this.teamState.roles.spectators,
        ];

        if (allMembers.includes(email)) {
            this.app.modalConfirmFn('User is already a member of this team.', { isError: true });
            return;
        }

        // Add to state
        if (!this.teamState.roles[role + 's']) {
            this.teamState.roles[role + 's'] = [];
        }
        this.teamState.roles[role + 's'].push(email);

        // Clear input and re-render
        emailInput.value = '';
        this.renderTeamMembers();
    }

    /**
     * Removes a member from teamState.
     */
    removeTeamMember(email, roleKey) {
        const list = this.teamState.roles[roleKey];
        if (!list) {
            return;
        }
        this.teamState.roles[roleKey] = list.filter(e => e !== email);
        this.renderTeamMembers();
    }

    /**
     * Adds an empty player row to the team roster container.
     */
    addTeamPlayerRow() {
        const rosterContainer = document.getElementById('team-roster-container');
        if (rosterContainer) {
            this.app.teamsRenderer.renderTeamRow(rosterContainer);
        }
    }

    /**
     * Closes the Team Modal.
     */
    closeTeamModal() {
        const modal = document.getElementById('team-modal');
        if (modal) {
            modal.classList.add('hidden');
        }
    }

    /**
     * Saves the currently editing team to DB and Server.
     */
    async saveTeam() {
        const id = document.getElementById('team-id').value || generateUUID();
        const name = document.getElementById('team-name').value;
        const shortName = document.getElementById('team-short-name').value;
        const color = document.getElementById('team-color').value;

        if (!name) {
            await this.app.modalConfirmFn('Team name is required.', { isError: true });
            return;
        }

        const roster = [];
        const rows = document.getElementById('team-roster-container').children;
        for (const row of rows) {
            const number = row.querySelector('input[name="number"]').value;
            const playerName = row.querySelector('input[name="name"]').value;
            const pos = row.querySelector('input[name="pos"]').value;
            if (playerName) {
                roster.push({
                    id: row.dataset.pid || generateUUID(),
                    name: playerName,
                    number: number,
                    pos: pos,
                });
            }
        }

        const user = this.app.state.currentUser;
        const ownerId = this.teamState.ownerId || (user ? user.email : this.app.auth.getLocalId());

        const team = new Team({
            id,
            name,
            shortName,
            color,
            roster,
            ownerId: ownerId,
            roles: this.teamState.roles,
            updatedAt: Date.now(),
        });

        const teamData = team.toJSON();

        // Save locally
        this.app.pendingSaves++;
        this.app.updateSaveStatus();
        await this.app.db.saveTeam(teamData);

        // Sync with server
        if (this.app.auth.getUser()) {
            const success = await this.app.teamSync.saveTeam(teamData);
            if (!success) {
                console.warn('App: Team saved locally but failed to sync with server. It will sync automatically when you visit the Teams view while online.');
            }
        }

        this.app.pendingSaves--;
        this.app.updateSaveStatus();

        this.closeTeamModal();
        await this.loadTeamsView();
    }

    /**
     * Deletes a team.
     * @param {string} teamId
     */
    async deleteTeam(teamId) {
        if (await this.app.modalConfirmFn('Are you sure you want to delete this team?')) {
            this.app.pendingSaves++;
            this.app.updateSaveStatus();
            await this.app.db.deleteTeam(teamId);
            if (this.app.auth.getUser()) {
                await this.app.teamSync.deleteTeam(teamId);
            }
            this.app.pendingSaves--;
            this.app.updateSaveStatus();
            await this.loadTeamsView();
        }
    }
}
