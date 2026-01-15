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
                const actions = document.createElement('div');
                actions.className = 'mt-4 flex gap-2 border-t pt-3';

                const editBtn = document.createElement('button');
                editBtn.className = 'text-xs font-bold text-blue-600 hover:text-blue-800 uppercase tracking-wider';
                editBtn.textContent = 'Edit Roster & Members';
                editBtn.onclick = (e) => {
                    e.stopPropagation();
                    this.callbacks.onEdit(team);
                };

                const deleteBtn = document.createElement('button');
                deleteBtn.className = 'text-xs font-bold text-red-600 hover:text-red-800 ml-auto p-1';
                deleteBtn.title = 'Delete Team';

                const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
                svg.setAttribute('class', 'w-5 h-5');
                svg.setAttribute('fill', 'none');
                svg.setAttribute('stroke', 'currentColor');
                svg.setAttribute('viewBox', '0 0 24 24');
                const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
                path.setAttribute('stroke-linecap', 'round');
                path.setAttribute('stroke-linejoin', 'round');
                path.setAttribute('stroke-width', '2');
                path.setAttribute('d', 'M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16');
                svg.appendChild(path);
                deleteBtn.appendChild(svg);

                deleteBtn.onclick = (e) => {
                    e.stopPropagation();
                    this.callbacks.onDelete(team.id);
                };

                actions.appendChild(editBtn);
                actions.appendChild(deleteBtn);
                card.appendChild(actions);
            }

            card.onclick = () => this.callbacks.onEdit(team);
            this.listContainer.appendChild(card);
        });
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
