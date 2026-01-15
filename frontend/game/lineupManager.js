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

import { generateUUID } from '../utils.js';
import { ActionTypes } from '../reducer.js';

/**
 * Manages the Lineup Editor (Starters and Substitutes).
 * Encapsulates Drag-and-Drop and Touch interaction logic.
 */
export class LineupManager {
    /**
     * @param {object} options
     * @param {Function} options.dispatch - App dispatch function.
     * @param {object} options.db - DBManager instance.
     * @param {Function} options.validate - Validation helper.
     */
    constructor({ dispatch, db, validate }) {
        this.dispatch = dispatch;
        this.db = db;
        this.validate = validate;
        this.lineupState = { starters: [], subs: [] };
        this.draggedItem = null;
    }

    /**
     * Initializes the lineup state for a given team.
     */
    async getInitialLineupState(game, team) {
        const roster = game.roster[team];
        let subs = game.subs[team] || [];
        const teamId = game[team === 'away' ? 'awayTeamId' : 'homeTeamId'];

        if (teamId) {
            const allTeams = await this.db.getAllTeams();
            const teamData = allTeams.find(t => t.id === teamId);
            if (teamData) {
                const usedIds = new Set();
                roster.forEach(r => {
                    if (r.starter.id) {
                        usedIds.add(r.starter.id);
                    }
                });
                subs.forEach(s => {
                    if (s.id) {
                        usedIds.add(s.id);
                    }
                });

                const missing = teamData.roster.filter(p => !usedIds.has(p.id));
                const newSubs = missing.map(p => ({ name: p.name, number: p.number, id: p.id, pos: p.pos || '' }));
                subs = [...subs, ...newSubs];
            }
        }

        const starters = [];
        const starterCount = Math.max(roster.length, 9);
        for (let i = 0; i < starterCount; i++) {
            starters.push(roster[i] ? roster[i].starter : { name: '', number: '', pos: '', id: generateUUID() });
        }

        const subsList = [];
        if (subs.length === 0) {
            subsList.push({ name: '', number: '', id: generateUUID() });
        } else {
            subs.forEach(s => subsList.push({ ...s, id: s.id || generateUUID() }));
        }

        this.lineupState = { starters, subs: subsList };
        return this.lineupState;
    }

    /**
     * Scrapes current input values from the modal.
     */
    scrape(containers) {
        const { starters: sContainer, subs: subContainer } = containers;

        const sRows = sContainer.children;
        this.lineupState.starters = Array.from(sRows).map(row => ({
            number: row.querySelector('input[name="number"]').value,
            name: row.querySelector('input[name="name"]').value,
            pos: row.querySelector('input[name="pos"]').value,
            id: row.dataset.pid || '',
        }));

        const subRows = subContainer.children;
        this.lineupState.subs = Array.from(subRows).map(row => ({
            number: row.querySelector('input[name="number"]').value,
            name: row.querySelector('input[name="name"]').value,
            id: row.dataset.pid || '',
        }));
    }

    // --- Interaction Handlers ---

    handleDragStart(e) {
        const row = e.target.closest('[data-idx]');
        this.draggedItem = {
            index: parseInt(row.dataset.idx),
            type: row.dataset.type,
            element: row,
        };
        e.dataTransfer.effectAllowed = 'move';
        row.classList.add('dragging');
    }

    handleDragOver(e) {
        e.preventDefault();
        const row = e.target.closest('[data-idx]');
        if (!row || row === this.draggedItem?.element) {
            return;
        }
        e.dataTransfer.dropEffect = 'move';
        row.classList.add('drag-over');
    }

    handleDragLeave(e) {
        const row = e.target.closest('[data-idx]');
        if (row) {
            row.classList.remove('drag-over');
        }
    }

    handleDrop(e, onUpdate) {
        e.preventDefault();
        const row = e.target.closest('[data-idx]');
        if (!row || !this.draggedItem) {
            return;
        }
        row.classList.remove('drag-over');

        const targetIndex = parseInt(row.dataset.idx);
        const targetType = row.dataset.type;

        this.movePlayer(this.draggedItem.index, this.draggedItem.type, targetIndex, targetType);
        if (onUpdate) {
            onUpdate();
        }
    }

    handleDragEnd() {
        if (this.draggedItem?.element) {
            this.draggedItem.element.classList.remove('dragging');
        }
        document.querySelectorAll('.drag-over').forEach(el => el.classList.remove('drag-over'));
        this.draggedItem = null;
    }

    handleTouchStart(e) {
        const handle = e.target.closest('.drag-handle');
        if (!handle) {
            return;
        }

        const row = handle.closest('[data-idx]');
        this.draggedItem = {
            index: parseInt(row.dataset.idx),
            type: row.dataset.type,
            element: row,
            startY: e.touches[0].clientY,
        };
        row.classList.add('dragging');
    }

    handleTouchMove(e) {
        if (!this.draggedItem) {
            return;
        }
        e.preventDefault();

        const touch = e.touches[0];
        const target = document.elementFromPoint(touch.clientX, touch.clientY);
        const row = target ? target.closest('[data-idx]') : null;

        document.querySelectorAll('.drag-over').forEach(el => el.classList.remove('drag-over'));
        if (row && row !== this.draggedItem.element) {
            row.classList.add('drag-over');
        }
    }

    handleTouchEnd(e, onUpdate) {
        if (!this.draggedItem) {
            return;
        }

        const touch = e.changedTouches[0];
        const target = document.elementFromPoint(touch.clientX, touch.clientY);
        const row = target ? target.closest('[data-idx]') : null;

        if (row) {
            const targetIndex = parseInt(row.dataset.idx);
            const targetType = row.dataset.type;
            this.movePlayer(this.draggedItem.index, this.draggedItem.type, targetIndex, targetType);
            if (onUpdate) {
                onUpdate();
            }
        }

        this.handleDragEnd();
    }

    movePlayer(fromIdx, fromType, toIdx, toType) {
        const sourceArr = fromType === 'starter' ? this.lineupState.starters : this.lineupState.subs;
        const targetArr = toType === 'starter' ? this.lineupState.starters : this.lineupState.subs;

        const [item] = sourceArr.splice(fromIdx, 1);
        targetArr.splice(toIdx, 0, item);
    }

    addPlayerToGroup(type) {
        const arr = type === 'starter' ? this.lineupState.starters : this.lineupState.subs;
        arr.push({ name: '', number: '', pos: '', id: generateUUID() });
    }

    removePlayerFromGroup(index, type) {
        const arr = type === 'starter' ? this.lineupState.starters : this.lineupState.subs;
        arr.splice(index, 1);
    }

    async save(game, team, newName) {
        const newTeamName = newName.trim() || null;

        // Rebuild roster slots
        const roster = this.lineupState.starters
            .filter(p => p.name.trim() !== '')
            .map((p, i) => {
                const existing = game.roster[team].find(s => s.starter.id === p.id);
                if (existing) {
                    return {
                        ...existing,
                        slot: i,
                        starter: { ...p },
                        // Update current name/number if it was the same as starter
                        current: existing.current.id === existing.starter.id ? { ...p } : existing.current,
                    };
                }
                return {
                    slot: i,
                    starter: { ...p },
                    current: { ...p },
                    history: [],
                };
            });

        const subs = this.lineupState.subs.filter(p => p.name.trim() !== '' || p.number.trim() !== '');

        await this.dispatch({
            type: ActionTypes.LINEUP_UPDATE,
            payload: {
                team,
                teamName: newTeamName,
                roster,
                subs,
            },
        });
    }
    /**
     * Renders a single row in the lineup editor.
     */
    renderRow(type, index, data, callbacks) {
        const div = document.createElement('div');
        div.className = type === 'starter' ? 'grid grid-cols-[30px_45px_1fr_55px_44px] gap-1 items-center bg-white p-1 rounded border border-gray-100 shadow-sm mb-1' : 'grid grid-cols-[30px_45px_1fr_44px] gap-1 items-center bg-white p-1 rounded border border-gray-100 shadow-sm mb-1';
        div.dataset.idx = index;
        div.dataset.type = type;
        div.draggable = true;

        if (data.id) {
            div.dataset.pid = data.id;
        }

        // DnD Listeners
        div.ondragstart = (e) => this.handleDragStart(e);
        div.ondragover = (e) => this.handleDragOver(e);
        div.ondragleave = (e) => this.handleDragLeave(e);
        div.ondrop = (e) => this.handleDrop(e, callbacks.onUpdate);
        div.ondragend = () => this.handleDragEnd();

        // Drag Handle
        const handle = document.createElement('div');
        handle.className = 'drag-handle text-gray-300 p-2 flex items-center justify-center font-bold text-xl cursor-move select-none hover:text-gray-500';
        handle.textContent = '⋮⋮';
        handle.ontouchstart = (e) => this.handleTouchStart(e);
        handle.ontouchmove = (e) => this.handleTouchMove(e);
        handle.ontouchend = (e) => this.handleTouchEnd(e, callbacks.onUpdate);
        div.appendChild(handle);

        const numInput = document.createElement('input');
        numInput.type = 'text'; numInput.name = 'number';
        numInput.className = 'w-full border p-1 rounded text-center font-mono text-sm focus:border-blue-500 outline-none';
        numInput.placeholder = '#'; numInput.value = data.number || '';
        div.appendChild(numInput);

        const nameInput = document.createElement('input');
        nameInput.type = 'text'; nameInput.name = 'name';
        nameInput.className = 'w-full border p-1 rounded text-sm focus:border-blue-500 outline-none';
        nameInput.placeholder = 'Name'; nameInput.value = data.name || '';
        div.appendChild(nameInput);

        if (type === 'starter') {
            const posInput = document.createElement('input');
            posInput.type = 'text'; posInput.name = 'pos';
            posInput.className = 'w-full border p-1 rounded text-center text-sm focus:border-blue-500 outline-none';
            posInput.placeholder = 'Pos'; posInput.value = data.pos || '';
            div.appendChild(posInput);
        }

        const actionsDiv = document.createElement('div');
        actionsDiv.className = 'flex gap-1 justify-end';
        if (type !== 'starter') {
            const delBtn = document.createElement('button');
            delBtn.className = 'text-red-400 hover:text-red-600 p-2 text-xl font-bold transition-colors';
            delBtn.textContent = '×';
            delBtn.onclick = () => callbacks.onRemove(index, type);
            actionsDiv.appendChild(delBtn);
        }
        div.appendChild(actionsDiv);

        return div;
    }
}
