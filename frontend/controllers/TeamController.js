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
    SyncStatusRemoteOnly,
} from '../constants.js';
import { parseQuery } from '../utils/searchParser.js';
import { PullToRefresh } from '../ui/pullToRefresh.js';

export class TeamController {
    constructor(app) {
        this.app = app;
        this.teamState = null;

        this.localBuffer = [];
        this.remoteBuffer = [];
        this.localMap = new Map();
        this.syncStatuses = new Map();

        this.isLoading = false;
        this.isFetchingRemote = false;

        this.query = '';
        this.scrollBound = false;

        // Pagination state
        this.displayLimit = 50;
        this.remoteOffset = 0;
        this.remoteHasMore = true;
    }

    /**
     * Loads the teams view.
     * Starts async loading of local and remote data.
     * Returns immediately after initial setup.
     * @param {boolean} [force=false] - If true, clears state and forces fresh load.
     */
    async loadTeamsView() {
        this.app.state.teams = [];
        this.app.state.view = 'teams';
        window.location.hash = 'teams';

        // Reset state
        this.localBuffer = [];
        this.remoteBuffer = [];
        this.localMap = new Map();
        this.syncStatuses = new Map();
        this.displayLimit = 50;
        this.remoteOffset = 0;
        this.remoteHasMore = true;

        this.app.render(); // Render empty state immediately
        this.bindScrollEvent();

        // Start independent async streams
        const localLoadPromise = this.loadAllLocalTeams();

        // Only fetch remote if logged in
        if (this.app.auth.getUser()) {
            this.fetchNextRemoteBatch();
            localLoadPromise.then(() => this.checkDeletions());
        }
    }

    /**
     * Loads the details for a specific team.
     * @param {string} teamId
     */
    async loadTeamDetail(teamId) {
        this.app.state.view = 'team';

        // Ensure we have the team data.
        let team = this.app.state.teams.find(t => t.id === teamId);
        if (!team) {
            // Try fetching from DB
            const localTeams = await this.app.db.getAllTeams();
            team = localTeams.find(t => t.id === teamId);

            if (!team && this.app.auth.getUser()) {
                // Try fetching from server
                try {
                    const response = await fetch(`/api/load-team/${encodeURIComponent(teamId)}`);
                    if (response.ok) {
                        team = await response.json();
                        // We don't auto-save remote teams to local DB just by viewing them
                        // unless we are syncing. For now, just use it for display.
                    }
                } catch (e) {
                    console.error('Failed to fetch team details', e);
                }
            }
        }

        this.teamState = team ? { ...team } : null; // Use teamState to hold the current detail team

        if (!this.teamState) {
            this.app.modalConfirmFn('Team not found.', { isError: true });
            window.location.hash = 'teams';
            return;
        }

        this.app.render();
        this.bindDetailEvents();
    }

    renderTeamDetail() {
        if (!this.teamState) {
            return;
        }

        const container = document.getElementById('team-view');
        this.app.teamsRenderer.renderTeamDetail(container, this.teamState, this.app.auth.getUser());
    }

    bindDetailEvents() {
        const tabRoster = document.getElementById('tab-team-detail-roster');
        const tabMembers = document.getElementById('tab-team-detail-members');
        const viewRoster = document.getElementById('team-detail-roster-view');
        const viewMembers = document.getElementById('team-detail-members-view');

        if (tabRoster && tabMembers) {
            tabRoster.onclick = () => {
                tabRoster.classList.add('border-blue-600', 'text-blue-600');
                tabRoster.classList.remove('border-transparent', 'text-gray-500');
                tabMembers.classList.remove('border-blue-600', 'text-blue-600');
                tabMembers.classList.add('border-transparent', 'text-gray-500');

                viewRoster.classList.remove('hidden');
                viewMembers.classList.add('hidden');
            };

            tabMembers.onclick = () => {
                tabMembers.classList.add('border-blue-600', 'text-blue-600');
                tabMembers.classList.remove('border-transparent', 'text-gray-500');
                tabRoster.classList.remove('border-blue-600', 'text-blue-600');
                tabRoster.classList.add('border-transparent', 'text-gray-500');

                viewRoster.classList.add('hidden');
                viewMembers.classList.remove('hidden');
            };
        }

        const btnBack = document.getElementById('btn-team-detail-back');
        if (btnBack) {
            btnBack.onclick = () => {
                window.location.hash = 'teams';
            };
        }

        const btnEdit = document.getElementById('btn-team-detail-edit');
        const btnDelete = document.getElementById('btn-team-detail-delete');

        if (btnEdit || btnDelete) {
            const user = this.app.auth.getUser();
            const canEdit = this.app.canWriteTeam(user ? user.email : null, this.teamState);

            if (btnEdit) {
                if (canEdit) {
                    btnEdit.classList.remove('hidden');
                    btnEdit.onclick = () => {
                        this.openEditTeamModal(this.teamState);
                    };
                } else {
                    btnEdit.classList.add('hidden');
                }
            }

            if (btnDelete) {
                // Only owner or admin can delete? canWriteTeam covers this usually.
                // Or maybe strictly owner? The original renderer checked:
                // team.ownerId === currentUser.email || (team.roles && team.roles.admins && team.roles.admins.includes(currentUser.email))
                // canWriteTeam is likely similar.
                if (canEdit) {
                    btnDelete.classList.remove('hidden');
                    btnDelete.onclick = () => {
                        this.deleteTeam(this.teamState.id);
                    };
                } else {
                    btnDelete.classList.add('hidden');
                }
            }
        }

        const btnStats = document.getElementById('btn-team-detail-stats');
        if (btnStats) {
            btnStats.onclick = async() => {
                // Pre-set the filter so loadStatisticsView can pick it up
                this.app.state.pendingStatsFilter = { teamId: this.teamState.id };
                window.location.hash = 'stats';
            };
        }
    }

    bindScrollEvent() {
        const container = document.getElementById('teams-list-container');
        if (container && !container.dataset.scrollBound) {
            container.addEventListener('scroll', () => {
                this.handleScroll(container);
            });
            container.dataset.scrollBound = 'true';
            this.scrollBound = true;
        }

        if (container) {
            // Initialize PullToRefresh
            new PullToRefresh(container, async() => {
                await this.refreshTeamsData();
            });
        }
    }

    /**
     * Refreshes the teams data without resetting the view state.
     */
    async refreshTeamsData() {
        // Reset state
        this.localBuffer = [];
        this.remoteBuffer = [];
        this.localMap = new Map();
        this.syncStatuses = new Map();
        this.displayLimit = 50;
        this.remoteOffset = 0;
        this.remoteHasMore = true;

        // Start independent async streams
        const localLoadPromise = this.loadAllLocalTeams();

        // Only fetch remote if logged in
        if (this.app.auth.getUser()) {
            this.fetchNextRemoteBatch();
            localLoadPromise.then(() => this.checkDeletions());
        }
    }

    handleScroll(container) {
        const { scrollTop, scrollHeight, clientHeight } = container;
        // Trigger when within 200px of bottom
        if (scrollTop + clientHeight >= scrollHeight - 200) {
            this.displayLimit += 20; // Show more

            const totalLoaded = this.localBuffer.length + this.remoteBuffer.length;
            if (this.remoteHasMore && !this.isFetchingRemote && this.displayLimit > totalLoaded - 20) {
                this.fetchNextRemoteBatch();
            } else {
                this.mergeAndRender();
            }
        }
    }

    async loadAllLocalTeams() {
        this.isLoading = true;
        try {
            let localTeamsRaw = await this.app.db.getAllTeams();
            const localTeams = localTeamsRaw.filter(t => this.app.hasTeamReadAccess(t));

            // Pre-process local teams for search filtering
            const parsedQ = parseQuery(this.query);
            const isRemoteOnly = parsedQ.filters.some(f => f.key === 'is' && f.value === 'remote');

            if (isRemoteOnly) {
                this.localBuffer = [];
            } else if (this.query) {
                this.localBuffer = localTeams.filter(t => this._matchesTeam(t, parsedQ));
            } else {
                this.localBuffer = localTeams;
            }

            // Populate local map for merging logic
            this.localMap = new Map(this.localBuffer.map(t => [t.id, t]));
        } finally {
            this.isLoading = false;
            this.mergeAndRender();
            this.triggerVisibleAutoSync();
        }
    }

    async fetchNextRemoteBatch() {
        if (this.isFetchingRemote || !this.remoteHasMore) {
            return;
        }
        this.isFetchingRemote = true;
        this.mergeAndRender();

        try {
            const result = await this.app.teamSync.fetchTeamList({
                limit: 50,
                offset: this.remoteOffset,
                sortBy: 'name',
                order: 'asc',
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
            console.error('Failed to fetch remote teams', e);
            this.remoteHasMore = false;
        } finally {
            this.isFetchingRemote = false;
            this.mergeAndRender();
        }
    }

    async checkDeletions() {
        try {
            const allLocal = await this.app.db.getAllTeams();
            const localIds = allLocal.map(t => t.id);
            if (localIds.length === 0) {
                return;
            }

            const deletedIds = await this.app.teamSync.checkTeamDeletions(localIds);
            if (deletedIds.length > 0) {
                console.log('TeamController: Deleting stale local teams', deletedIds);
                const deletedSet = new Set(deletedIds);

                // 1. Delete from DB
                await Promise.all(deletedIds.map(id =>
                    this.app.db.deleteTeam(id).catch(err => console.warn(`TeamController: Failed to delete stale team ${id}`, err)),
                ));

                // 2. Remove from Local Buffer
                this.localBuffer = this.localBuffer.filter(t => !deletedSet.has(t.id));
                this.localMap = new Map(this.localBuffer.map(t => [t.id, t]));

                // 3. Remove from Remote Buffer
                this.remoteBuffer = this.remoteBuffer.filter(t => !deletedSet.has(t.id));

                this.mergeAndRender();
            }
        } catch (e) {
            console.warn('TeamController: Check deletions failed', e);
        }
    }

    mergeAndRender() {
        const merged = new Map();

        // Add Local First
        this.localBuffer.forEach(t => {
            merged.set(t.id, { ...t, _source: 'local' });
        });

        // Add/Update Remote
        this.remoteBuffer.forEach(t => {
            const existing = merged.get(t.id);
            if (existing) {
                merged.set(t.id, { ...t, _source: 'both', _localTeam: existing });
            } else {
                merged.set(t.id, { ...t, _source: 'remote' });
            }
        });

        const processed = Array.from(merged.values()).map(item => {
            let status = SyncStatusSynced;
            const localItem = item._source === 'local' ? item : item._localTeam;
            const remoteItem = item._source === 'remote' || item._source === 'both' ? item : null;
            const isDirty = localItem && localItem._dirty;

            if (item._source === 'remote') {
                status = SyncStatusRemoteOnly;
            } else if (isDirty) {
                status = SyncStatusLocalOnly;
            } else if (item._source === 'local') {
                if (!navigator.onLine || this.app.sync.isServerUnreachable) {
                    status = SyncStatusSynced;
                } else {
                    status = SyncStatusLocalOnly;
                }
            } else if (localItem && remoteItem) {
                status = SyncStatusSynced;
            }

            // Apply transient status from sync operations
            if (this.syncStatuses.has(item.id)) {
                status = this.syncStatuses.get(item.id);
            }

            // Prefer local for editing, remote for latest data?
            // Usually local is the source of truth for offline edits.
            const base = localItem || remoteItem;
            return {
                ...base,
                source: localItem ? 'local' : 'remote',
                syncStatus: status,
            };
        });

        // Sort by Name Ascending
        processed.sort((a, b) => (a.name || '').localeCompare(b.name || ''));

        this.app.state.teams = processed.slice(0, this.displayLimit);
        this.renderWithPagination();
    }

    renderWithPagination() {
        this.app.render();

        const main = document.querySelector('#teams-view main');
        if (!main) {
            return;
        }

        // Clean up existing sentinels
        const existing = main.querySelectorAll('.pagination-sentinel');
        existing.forEach(el => el.remove());

        const totalAvailable = this.localBuffer.length + this.remoteBuffer.length;
        const showingAll = this.app.state.teams.length >= totalAvailable && !this.remoteHasMore;

        let sentinelText = '';
        if (this.isLoading || this.isFetchingRemote) {
            sentinelText = 'Loading more teams...';
        } else if (!showingAll) {
            sentinelText = 'Scroll for more';
        } else if (this.app.state.teams.length > 0) {
            sentinelText = 'All teams loaded.';
        }

        if (sentinelText) {
            const sentinel = document.createElement('div');
            sentinel.className = 'pagination-sentinel py-4 text-center text-gray-500 text-sm font-medium';
            sentinel.textContent = sentinelText;
            main.appendChild(sentinel);
        }
    }

    /**
     * Triggers auto-sync for any visible local-only teams.
     */
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
        await this.loadTeamsView(); // Reloads with query
    }

    _matchesTeam(t, parsedQ) {
        // 1. Free Text (AND)
        for (const token of parsedQ.tokens) {
            if (!(t.name || '').toLowerCase().includes(token.toLowerCase())) {
                return false;
            }
        }
        // 2. Filters
        for (const f of parsedQ.filters) {
            if (f.key === 'is') {
                continue;
            }
            if (f.key === 'name' && !(t.name || '').toLowerCase().includes(f.value.toLowerCase())) {
                return false;
            }
        }
        return true;
    }

    async loadMore() {
        const container = document.getElementById('teams-list-container');
        if (container) {
            this.handleScroll(container);
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

        this.syncStatuses.set(teamId, SyncStatusSyncing);
        this.mergeAndRender();

        try {
            const success = await this.app.teamSync.saveTeam(team);
            if (success) {
                this.syncStatuses.set(teamId, SyncStatusSynced);
                await this.app.db.markClean(teamId, 'teams');
            } else {
                this.syncStatuses.set(teamId, SyncStatusError);
                if (this.app.auth.isStale) {
                    console.warn('App: Team sync failed due to stale session.');
                }
            }
        } catch (e) {
            console.error('App: Team sync error', e);
            this.syncStatuses.set(teamId, SyncStatusError);
        }
        this.mergeAndRender();
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
            name, shortName, color, roster,
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
            } else {
                await this.app.db.markClean(id, 'teams');
            }
        }

        this.app.pendingSaves--;
        this.app.updateSaveStatus();

        this.closeTeamModal();
        await this.loadTeamsView();
    }

    /**
     * Deletes a team locally and from the server.
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