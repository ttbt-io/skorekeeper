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
    }

    /**
     * Loads the teams view by fetching all saved teams.
     */
    async loadTeamsView() {
        const localTeamsRaw = await this.app.db.getAllTeams();
        const localTeams = localTeamsRaw.filter(t => this.app.hasTeamReadAccess(t));
        let remoteTeams = [];

        if (this.app.auth.getUser()) {
            const localIds = localTeams.map(t => t.id);
            remoteTeams = await this.app.teamSync.fetchTeamList(localIds);
        }

        const teamMap = new Map();

        // Handle deletions and index remote teams
        for (const t of remoteTeams) {
            if (t.status === 'deleted') {
                console.log(`[Teams] Remote deletion detected for ${t.id}. Deleting local copy.`);
                await this.app.db.deleteTeam(t.id);
                // Also remove from localTeams list in memory so we don't re-add it below
                const idx = localTeams.findIndex(lt => lt.id === t.id);
                if (idx !== -1) {
                    localTeams.splice(idx, 1);
                }
                continue;
            }

            teamMap.set(t.id, {
                ...t,
                syncStatus: SyncStatusSynced,
            });
            this.app.db.saveTeam(t); // Persist remote teams locally
        }

        localTeams.forEach(t => {
            if (!teamMap.has(t.id)) {
                teamMap.set(t.id, {
                    ...t,
                    syncStatus: SyncStatusLocalOnly,
                });
            } else {
                // For now, remote wins if already on server.
                // In future, could compare updatedAt.
            }
        });

        this.app.state.teams = Array.from(teamMap.values());

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
        this.app.render();
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
