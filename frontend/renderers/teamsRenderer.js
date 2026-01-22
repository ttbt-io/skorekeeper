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

import { createElement } from '../utils.js';

/**
 * Handles rendering of the Teams management view.
 */
export class TeamsRenderer {
    /**
     * @param {object} options
     * @param {HTMLElement} options.listContainer - Container for the team list.
     * @param {HTMLElement} options.membersContainer - Container for the team members list.
     * @param {object} options.callbacks - Callbacks for user interactions.
     */
    constructor({ listContainer, membersContainer, callbacks }) {
        this.listContainer = listContainer;
        this.membersContainer = membersContainer;
        this.callbacks = callbacks;
    }

    /**
     * Renders the list of teams.
     * @param {Array<object>} teams - List of teams.
     * @param {object|null} currentUser - Current authenticated user.
     */
    renderTeamsList(teams, currentUser) {
        if (!this.listContainer) {
            return;
        }
        this.listContainer.innerHTML = '';

        // Handle Quota UI
        const btnNewTeam = document.getElementById('btn-create-team');
        if (btnNewTeam && currentUser) {
            const ownedCount = teams.filter(t => t.ownerId === currentUser.email).length;
            if (currentUser.maxTeams !== 0 && ownedCount >= currentUser.maxTeams) {
                btnNewTeam.disabled = true;
                btnNewTeam.classList.add('opacity-50', 'cursor-not-allowed');
                btnNewTeam.title = `Quota Reached (${ownedCount}/${currentUser.maxTeams} teams)`;
            } else {
                btnNewTeam.disabled = false;
                btnNewTeam.classList.remove('opacity-50', 'cursor-not-allowed');
                btnNewTeam.title = '';
            }
        }

        if (teams.length === 0) {
            this.listContainer.appendChild(createElement('div', {
                className: 'text-center text-gray-500 mt-10',
                text: 'No teams found. Click "New Team" to get started.',
            }));
            return;
        }

        teams.forEach(team => {
            const card = document.createElement('div');
            card.className = 'bg-white p-4 rounded-xl shadow-sm border border-gray-200 hover:shadow-md transition-shadow cursor-pointer relative';

            const flex = document.createElement('div');
            flex.className = 'flex justify-between items-start';

            const info = document.createElement('div');
            const name = document.createElement('h3');
            name.className = 'text-lg font-bold text-gray-900';
            name.textContent = team.name;

            const meta = document.createElement('p');
            meta.className = 'text-sm text-gray-500';
            meta.textContent = `${team.shortName || '---'} • ${team.roster ? team.roster.length : 0} Players`;

            info.appendChild(name);
            info.appendChild(meta);

            const rightSide = document.createElement('div');
            rightSide.className = 'flex flex-col items-end gap-2';

            const colorBadge = document.createElement('div');
            colorBadge.className = 'w-6 h-6 rounded-full border border-gray-200 shadow-sm';
            colorBadge.style.backgroundColor = team.color || '#2563eb';
            rightSide.appendChild(colorBadge);

            // Sync Status Indicator
            if (currentUser) {
                const syncBtn = document.createElement('button');
                syncBtn.id = `team-sync-btn-${team.id}`;
                syncBtn.className = 'p-1 rounded-full text-lg hover:bg-gray-100 focus:outline-none transition-colors';

                let icon = '';
                let titleAttr = '';
                switch (team.syncStatus) {
                    case 'synced': icon = '✅'; titleAttr = 'Synced'; break;
                    case 'local_only': icon = '☁️⬆️'; titleAttr = 'Local Only (Click to Sync)'; break;
                    case 'syncing': icon = '⏳'; titleAttr = 'Syncing...'; break;
                    case 'error': icon = '❌'; titleAttr = 'Sync Error (Click to retry)'; break;
                    default: icon = '';
                }
                syncBtn.textContent = icon;
                syncBtn.title = titleAttr;

                if (team.syncStatus === 'syncing') {
                    syncBtn.disabled = true;
                    syncBtn.classList.add('animate-pulse');
                }

                syncBtn.onclick = (e) => {
                    e.stopPropagation();
                    this.callbacks.onSync(team.id);
                };

                if (icon) {
                    rightSide.appendChild(syncBtn);
                }
            }

            flex.appendChild(info);
            flex.appendChild(rightSide);
            card.appendChild(flex);

            // Admin Actions
            if (currentUser && (team.ownerId === currentUser.email || (team.roles && team.roles.admins && team.roles.admins.includes(currentUser.email)))) {
                // We don't show buttons here anymore, just click to view details
                // Or maybe we keep Delete? The prompt says "Edit Team" button is in the Team Screen.
                // Let's keep Delete here for convenience, or remove it.
                // Standard UI usually allows delete from list or detail.
                // Let's keep it minimal as per instruction "clicking on a team opens the Edit Team panel. Instead...".
                // I'll leave the Delete button but remove the Edit button from the card actions if any.
            }

            // Entire card clicks to team detail
            card.onclick = () => {
                window.location.hash = `#team/${team.id}`;
            };
            this.listContainer.appendChild(card);
        });
    }

    /**
     * Renders the team detail view.
     * @param {HTMLElement} container
     * @param {object} team
     */
    renderTeamDetail(container, team) {
        if (!container) {
            return;
        }

        // 1. Update Header
        const nameEl = document.getElementById('team-detail-name');
        if (nameEl) {
            nameEl.textContent = team.name;
        }

        // 2. Render Roster
        const rosterContainer = document.getElementById('team-detail-roster-view');
        if (rosterContainer) {
            rosterContainer.innerHTML = '';
            if (!team.roster || team.roster.length === 0) {
                rosterContainer.appendChild(createElement('div', {
                    className: 'text-center text-gray-500 py-8',
                    text: 'No players in roster.',
                }));
            } else {
                const list = document.createElement('div');
                list.className = 'bg-white rounded-xl shadow-sm border border-gray-200 divide-y divide-gray-100';

                team.roster.forEach(p => {
                    const row = document.createElement('div');
                    row.className = 'p-4 flex items-center justify-between hover:bg-gray-50 cursor-pointer transition-colors';
                    row.onclick = () => {
                        if (this.callbacks.onOpenPlayerProfile) {
                            this.callbacks.onOpenPlayerProfile(p.id);
                        }
                    };

                    const left = document.createElement('div');
                    left.className = 'flex items-center gap-4';

                    const num = document.createElement('span');
                    num.className = 'font-mono text-gray-400 font-bold w-6 text-right';
                    num.textContent = p.number || '--';

                    const name = document.createElement('span');
                    name.className = 'font-bold text-gray-900';
                    name.textContent = p.name;

                    left.appendChild(num);
                    left.appendChild(name);

                    const pos = document.createElement('span');
                    pos.className = 'text-xs font-bold text-gray-500 bg-gray-100 px-2 py-1 rounded';
                    pos.textContent = p.pos || 'BENCH';

                    row.appendChild(left);
                    row.appendChild(pos);
                    list.appendChild(row);
                });
                rosterContainer.appendChild(list);
            }
        }

        // 3. Render Members
        const membersContainer = document.getElementById('team-detail-members-view');
        if (membersContainer) {
            // We reuse renderTeamMembers but we need to target a specific container inside.
            // Actually renderTeamMembers uses `this.membersContainer` which is bound to the modal.
            // We should make a reusable internal render function or temporarily bind the container.
            // Better: Manual rendering here for read-only list, as logic differs (no delete buttons in read-only view?).
            // The prompt implies we can see members.

            membersContainer.innerHTML = '';
            const list = document.createElement('div');
            list.className = 'space-y-4';

            const renderGroup = (title, emails) => {
                if (!emails || emails.length === 0) {
                    return;
                }
                const grp = document.createElement('div');
                grp.appendChild(createElement('h3', { className: 'text-xs font-bold text-gray-400 uppercase tracking-widest mb-2', text: title }));

                emails.forEach(email => {
                    const row = document.createElement('div');
                    row.className = 'bg-white p-3 rounded border border-gray-200 text-sm font-medium text-gray-700 flex items-center gap-2';
                    // Avatar placeholder
                    const avatar = document.createElement('div');
                    avatar.className = 'w-6 h-6 rounded-full bg-blue-100 text-blue-600 flex items-center justify-center text-xs font-bold';
                    avatar.textContent = email.substring(0, 2).toUpperCase();
                    row.appendChild(avatar);
                    row.appendChild(document.createTextNode(email));
                    grp.appendChild(row);
                });
                list.appendChild(grp);
            };

            const roles = team.roles || {};
            const admins = roles.admins || [];
            const owner = team.ownerId ? [team.ownerId] : [];
            // Merge owner into admins for display if not present
            const allAdmins = Array.from(new Set([...owner, ...admins]));

            renderGroup('Admins', allAdmins);
            renderGroup('Scorekeepers', roles.scorekeepers);
            renderGroup('Spectators', roles.spectators);

            membersContainer.appendChild(list);
        }
    }

    /**
     * Renders the team members list in the modal.
     * @param {object} team - The team object.
     * @param {object|null} currentUser - Current authenticated user.
     */
    renderTeamMembers(team, currentUser) {
        console.log('[TeamsRenderer] renderTeamMembers', team.id, team.roles);
        if (!this.membersContainer) {
            return;
        }
        this.membersContainer.innerHTML = '';

        const renderRoleGroup = (title, emails, roleKey) => {
            console.log('[TeamsRenderer] renderRoleGroup', title, emails);
            if (!emails || emails.length === 0) {
                return;
            }

            const group = document.createElement('div');
            group.className = 'mb-4';
            const h4 = document.createElement('h4');
            h4.className = 'text-xs font-bold text-gray-400 uppercase tracking-widest mb-2';
            h4.textContent = title;
            group.appendChild(h4);

            emails.forEach(email => {
                const row = document.createElement('div');
                row.className = 'flex justify-between items-center bg-white p-2 rounded border border-gray-100 mb-1 text-sm';

                const emailDiv = document.createElement('div');
                emailDiv.className = 'font-medium text-gray-700';
                emailDiv.textContent = email;
                row.appendChild(emailDiv);

                if (this.callbacks.canManage(currentUser, team) && email !== team.ownerId) {
                    const removeBtn = document.createElement('button');
                    removeBtn.className = 'text-red-500 hover:text-red-700 font-bold px-2';
                    removeBtn.textContent = '×';
                    removeBtn.onclick = () => this.callbacks.onRemoveMember(email, roleKey);
                    row.appendChild(removeBtn);
                }
                group.appendChild(row);
            });
            this.membersContainer.appendChild(group);
        };

        const roles = team.roles || { admins: [], scorekeepers: [], spectators: [] };

        // Owner is always an admin but shown separately or first
        const admins = roles.admins || [];
        const owner = team.ownerId;
        const adminList = owner ? [owner, ...admins.filter(a => a !== owner)] : admins;

        renderRoleGroup('Admins', adminList, 'admins');
        renderRoleGroup('Scorekeepers', roles.scorekeepers, 'scorekeepers');
        renderRoleGroup('Spectators', roles.spectators, 'spectators');
    }

    /**
     * Renders a single player row in the team roster editor.
     */
    renderTeamRow(container, player = null) {
        const row = document.createElement('div');
        row.className = 'grid grid-cols-[60px_1fr_60px_40px] gap-2 items-center bg-gray-50 p-2 rounded border border-gray-200';
        if (player && player.id) {
            row.dataset.pid = player.id;
        }

        const numInput = document.createElement('input');
        numInput.type = 'text';
        numInput.name = 'number';
        numInput.className = 'w-full border p-2 rounded text-center font-mono';
        numInput.value = player ? player.number : '';
        numInput.placeholder = '#';

        const nameInput = document.createElement('input');
        nameInput.type = 'text';
        nameInput.name = 'name';
        nameInput.className = 'w-full border p-2 rounded';
        nameInput.value = player ? player.name : '';
        nameInput.placeholder = 'Player Name';

        const posInput = document.createElement('input');
        posInput.type = 'text';
        posInput.name = 'pos';
        posInput.className = 'w-full border p-2 rounded text-center uppercase text-xs';
        posInput.value = player && player.pos ? player.pos : '';
        posInput.placeholder = 'Pos';

        const removeBtn = document.createElement('button');
        removeBtn.className = 'text-red-500 hover:text-red-700 font-bold text-xl';
        removeBtn.textContent = '×';
        removeBtn.onclick = () => row.remove();

        row.appendChild(numInput);
        row.appendChild(nameInput);
        row.appendChild(posInput);
        row.appendChild(removeBtn);
        container.appendChild(row);
    }
}
