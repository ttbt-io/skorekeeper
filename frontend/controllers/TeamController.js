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
import { StreamMerger } from '../services/streamMerger.js';

export class TeamController {
    constructor(app) {
        this.app = app;
        this.teamState = null;
        this.merger = null;
        this.localMap = new Map();
        this.isLoading = false;
        this.query = '';
        this.scrollBound = false;
        this.batchSize = 20;
        this.hasMore = false;
    }

    /**
     * Loads the teams view by fetching all saved teams.
     */
    async loadTeamsView() {
        this.app.state.teams = [];
        this.query = '';
        this.app.state.view = 'teams';
        window.location.hash = 'teams';
        this.isLoading = true;

        // 1. Prepare Local Data
        const localTeamsRaw = await this.app.db.getAllTeams();
        const localTeams = localTeamsRaw.filter(t => this.app.hasTeamReadAccess(t));
        this.localMap = new Map(localTeams.map(t => [t.id, t]));

        // Filter and Sort Local Data (Name Ascending)
        let filteredLocal = localTeams;
        filteredLocal.sort((a, b) => (a.name || '').localeCompare(b.name || ''));

        // 2. Initialize StreamMerger
        this.merger = new StreamMerger(
            filteredLocal,
            async(offset) => {
                if (!this.app.auth.getUser()) {
                    return { data: [], meta: { total: 0 } };
                }
                return this.app.teamSync.fetchTeamList({
                    limit: 50,
                    offset: offset,
                    sortBy: 'name',
                    order: 'asc',
                    query: this.query,
                });
            },
            (a, b) => (a.name || '').localeCompare(b.name || ''), // Comparator: Name Asc
            'id',
        );

        // 3. Bind Scroll
        this.bindScrollEvent();

        // 4. Initial Auto-Fill
        await this.autoFill();
        this.isLoading = false;

        this.triggerVisibleAutoSync();
    }

    bindScrollEvent() {
        const container = document.getElementById('teams-list-container');
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
        const container = document.getElementById('teams-list-container');
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

        const rawBatch = await this.merger.fetchNextBatch(this.batchSize);
        const processedBatch = await this._processBatch(rawBatch);

        this.app.state.teams.push(...processedBatch);
        this.app.render();

        this.triggerVisibleAutoSync();

        this.hasMore = this.merger.hasMore();
        this.renderWithPagination();

        this.isLoading = false;
    }

    async _processBatch(batch) {
        const results = [];
        const dbOperations = [];

        for (const item of batch) {
            const remoteItem = item._remote;
            const localItem = (item.source === 'local') ? item : this.localMap.get(item.id);

            // Handle Persistence of Remote Data
            if (remoteItem) {
                if (remoteItem.status === 'deleted') {
                    dbOperations.push(this.app.db.deleteTeam(remoteItem.id));
                    continue;
                } else {
                    dbOperations.push(this.app.db.saveTeam(remoteItem));
                }
            } else if (item.source === 'remote' && item.status !== 'deleted') {
                dbOperations.push(this.app.db.saveTeam(item));
            }

            // Sync Status Logic
            let status = SyncStatusSynced;
            if (!remoteItem && item.source === 'local') {
                status = SyncStatusLocalOnly;
            }

            const base = remoteItem || localItem || item;

            results.push({
                ...base,
                syncStatus: status,
            });
        }

        // Parallel await for performance
        if (dbOperations.length > 0) {
            await Promise.all(dbOperations);
        }

        return results;
    }

    triggerVisibleAutoSync() {
        if (this.app.auth.getUser()) {
            this.app.state.teams.forEach((t) => {
                if (t.syncStatus === SyncStatusLocalOnly) {
                    this.syncTeam(t.id);
                }
            });
        }
    }

    async search(query) {
        this.query = query;
        this.app.state.teams = [];
        this.isLoading = true;
        this.app.render();

        const localTeamsRaw = Array.from(this.localMap.values());
        let filteredLocal = localTeamsRaw;
        if (this.query) {
            const q = this.query.toLowerCase();
            filteredLocal = localTeamsRaw.filter(t => (t.name || '').toLowerCase().includes(q));
        }
        filteredLocal.sort((a, b) => (a.name || '').localeCompare(b.name || ''));

        this.merger = new StreamMerger(
            filteredLocal,
            async(offset) => {
                if (!this.app.auth.getUser()) {
                    return { data: [], meta: { total: 0 } };
                }
                return this.app.teamSync.fetchTeamList({
                    limit: 50,
                    offset: offset,
                    sortBy: 'name',
                    order: 'asc',
                    query: this.query,
                });
            },
            (a, b) => (a.name || '').localeCompare(b.name || ''),
            'id',
        );

        await this.autoFill();
        this.isLoading = false;
    }

    renderWithPagination() {
        this.app.render();
        if (this.hasMore) {
            const main = document.querySelector('#teams-view main');
            if (main) {
                const sentinel = document.createElement('div');
                sentinel.className = 'py-4 text-center text-gray-500 text-sm font-medium';
                sentinel.textContent = this.isLoading ? 'Loading more teams...' : 'Scroll for more';
                main.appendChild(sentinel);
            }
        }
    }

    async loadMore() {
        if (!this.hasMore || this.isLoading) {
            return;
        }
        await this.loadNextBatch();
    }

    // ... (Keep existing methods: syncTeam, openEditTeamModal, switchTeamModalTab, renderTeamMembers, addTeamMember, removeTeamMember, addTeamPlayerRow, closeTeamModal, saveTeam, deleteTeam) ...
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
        this.switchTeamModalTab('roster');

        if (teamOrId) {
            let team = typeof teamOrId === 'object' ? teamOrId : null;
            const teamId = typeof teamOrId === 'string' ? teamOrId : (team ? team.id : null);

            if (!team && teamId) {
                if (!/^[0-9a-fA-F-]{36}$/.test(teamId)) {
                    console.error('App: Invalid team ID format', teamId);
                    return;
                }

                const allTeamsData = await this.app.db.getAllTeams();
                const teamData = allTeamsData.find(t => t.id === teamId);

                if (!teamData) {
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
            if (rosterContainer) {
                for (let i = 0; i < 9; i++) {
                    this.app.teamsRenderer.renderTeamRow(rosterContainer);
                }
            }

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

    renderTeamMembers() {
        if (!this.teamState) {
            return;
        }

        if (!this.app.teamsRenderer.membersContainer) {
            this.app.teamsRenderer.membersContainer = document.getElementById('team-members-container');
        }
        this.app.teamsRenderer.renderTeamMembers(this.teamState, this.app.auth.getUser());
    }

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

        const allMembers = [
            ...this.teamState.roles.admins,
            ...this.teamState.roles.scorekeepers,
            ...this.teamState.roles.spectators,
        ];

        if (allMembers.includes(email)) {
            this.app.modalConfirmFn('User is already a member of this team.', { isError: true });
            return;
        }

        if (!this.teamState.roles[role + 's']) {
            this.teamState.roles[role + 's'] = [];
        }
        this.teamState.roles[role + 's'].push(email);

        emailInput.value = '';
        this.renderTeamMembers();
    }

    removeTeamMember(email, roleKey) {
        const list = this.teamState.roles[roleKey];
        if (!list) {
            return;
        }
        this.teamState.roles[roleKey] = list.filter(e => e !== email);
        this.renderTeamMembers();
    }

    addTeamPlayerRow() {
        const rosterContainer = document.getElementById('team-roster-container');
        if (rosterContainer) {
            this.app.teamsRenderer.renderTeamRow(rosterContainer);
        }
    }

    closeTeamModal() {
        const modal = document.getElementById('team-modal');
        if (modal) {
            modal.classList.add('hidden');
        }
    }

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

        this.app.pendingSaves++;
        this.app.updateSaveStatus();
        await this.app.db.saveTeam(teamData);

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