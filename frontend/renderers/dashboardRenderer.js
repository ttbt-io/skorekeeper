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

import { formatDate, createElement } from '../utils.js';

/**
 * Handles rendering of the Dashboard view.
 */
export class DashboardRenderer {
    /**
     * @param {object} options
     * @param {HTMLElement} options.container - The element where the game list should be rendered.
     * @param {object} options.callbacks - Callbacks for user interactions.
     * @param {Function} options.callbacks.onSync - Called when sync is requested for a game.
     * @param {Function} options.callbacks.onOpen - Called when a game is clicked to open.
     * @param {Function} options.callbacks.onContextMenu - Called when a game is right-clicked.
     */
    constructor({ container, callbacks }) {
        this.container = container;
        this.callbacks = callbacks;
    }

    /**
     * Renders the list of games.
     * @param {Array<object>} games - The list of games to render.
     * @param {object|null} currentUser - The current authenticated user.
     * @param {string} filter - Filter term for searching.
     */
    render(games, currentUser, filter = '') {
        if (!this.container) {
            return;
        }
        this.container.innerHTML = '';

        // Handle Quota UI
        const btnNewGame = document.getElementById('btn-new-game');
        if (btnNewGame && currentUser) {
            const ownedCount = games.filter(g => g.ownerId === currentUser.email).length;
            if (currentUser.maxGames !== 0 && ownedCount >= currentUser.maxGames) {
                btnNewGame.disabled = true;
                btnNewGame.classList.add('opacity-50', 'cursor-not-allowed');
                btnNewGame.title = `Quota Reached (${ownedCount}/${currentUser.maxGames} games)`;
            } else {
                btnNewGame.disabled = false;
                btnNewGame.classList.remove('opacity-50', 'cursor-not-allowed');
                btnNewGame.title = '';
            }
        }

        const term = filter.toLowerCase();
        const filteredGames = games.filter((g) => {
            return (g.away && g.away.toLowerCase().includes(term))
                || (g.home && g.home.toLowerCase().includes(term))
                || (g.event && g.event.toLowerCase().includes(term))
                || (g.location && g.location.toLowerCase().includes(term))
                || (formatDate(g.date).toLowerCase().includes(term));
        });

        // Sort by date descending
        filteredGames.sort((a, b) => new Date(b.date) - new Date(a.date));
        const ongoing = filteredGames.filter(g => g.status !== 'final');
        const finalized = filteredGames.filter(g => g.status === 'final');

        this.renderGroup('Ongoing Games', ongoing, false, currentUser);
        this.renderGroup('Finalized Games', finalized, true, currentUser);

        if (filteredGames.length === 0) {
            this.container.appendChild(createElement('div', {
                className: 'text-center text-gray-500 mt-10',
                text: 'No games found.',
            }));
        }
    }

    /**
     * @private
     */
    renderGroup(title, games, isFinal, currentUser) {
        if (games.length === 0) {
            return;
        }

        const header = document.createElement('h2');
        header.className = 'text-xl font-bold text-gray-800 mt-6 mb-4 border-b border-gray-300 pb-2 uppercase tracking-wide';
        header.textContent = title;
        this.container.appendChild(header);

        let lastDate = '';
        games.forEach((g) => {
            const dateStr = formatDate(g.date);
            if (dateStr !== lastDate) {
                const dateHeader = document.createElement('div');
                dateHeader.className = 'text-gray-500 font-bold mt-4 mb-2 text-sm';
                dateHeader.textContent = dateStr;
                this.container.appendChild(dateHeader);
                lastDate = dateStr;
            }

            const gameCardDiv = document.createElement('div');
            let cardClass = 'bg-white p-3 rounded shadow mb-2 cursor-pointer hover:bg-gray-50 relative';
            if (isFinal) {
                cardClass = 'bg-gray-100 p-3 rounded shadow-sm mb-2 cursor-pointer hover:bg-gray-50 relative opacity-80';
            }
            gameCardDiv.className = cardClass;
            gameCardDiv.dataset.gameId = g.id;

            const flexDiv = document.createElement('div');
            flexDiv.className = 'flex justify-between items-center mb-2';

            const h3 = document.createElement('h3');
            h3.className = 'font-bold text-lg';
            h3.textContent = `${g.away} vs ${g.home}`;

            const dateSpan = document.createElement('span');
            dateSpan.className = 'text-xs text-gray-500';
            dateSpan.textContent = dateStr;

            flexDiv.appendChild(h3);
            flexDiv.appendChild(dateSpan);

            // Shared indicator
            if (currentUser && g.ownerId && g.ownerId !== currentUser.email) {
                const sharedBadge = document.createElement('div');
                sharedBadge.className = 'text-[10px] bg-blue-50 text-blue-600 px-1.5 py-0.5 rounded font-bold uppercase w-fit mt-1';
                sharedBadge.textContent = `Shared by ${g.ownerId}`;
                gameCardDiv.appendChild(sharedBadge);
            }

            const pElement = document.createElement('p');
            pElement.className = 'text-sm text-gray-700';
            pElement.textContent = `${g.event || 'No Event'} at ${g.location || 'No Location'}`;

            gameCardDiv.appendChild(flexDiv);
            gameCardDiv.appendChild(pElement);

            // Sync Status Indicator / Button
            if (currentUser) {
                const syncBtn = document.createElement('button');
                syncBtn.id = `sync-btn-${g.id}`;
                syncBtn.className = 'absolute bottom-3 right-3 p-1 rounded-full text-lg hover:bg-gray-200 focus:outline-none';

                let icon = '';
                let titleAttr = '';
                let hideBtn = false;

                if (g.id.startsWith('demo-')) {
                    hideBtn = true;
                }

                switch (g.syncStatus) {
                    case 'synced': icon = '✅'; titleAttr = 'Synced'; break;
                    case 'local_only': icon = '☁️⬆️'; titleAttr = 'Upload to Cloud'; break;
                    case 'remote_only': icon = '☁️⬇️'; titleAttr = 'Download from Cloud'; break;
                    case 'unsynced': icon = '⚠️'; titleAttr = 'Unsynced (Click to Sync)'; break;
                    default: icon = '❓';
                }

                if (!hideBtn) {
                    syncBtn.textContent = icon;
                    syncBtn.title = titleAttr;

                    syncBtn.onclick = (e) => {
                        e.stopPropagation();
                        this.callbacks.onSync(g.id);
                    };

                    gameCardDiv.appendChild(syncBtn);
                }
            }

            gameCardDiv.onclick = () => this.callbacks.onOpen(g);
            gameCardDiv.oncontextmenu = (e) => {
                e.preventDefault();
                this.callbacks.onContextMenu(e, g.id);
            };
            this.container.appendChild(gameCardDiv);
        });
    }
}
