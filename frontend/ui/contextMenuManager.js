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
import {
    PitchCodeCalled,
    PitchCodeSwinging,
    PitchCodeDropped,
} from '../constants.js';

/**
 * Manages the lifecycle, positioning, and interaction of context menus.
 */
export class ContextMenuManager {
    /**
     * @param {object} options
     * @param {number} [options.autoHideDelay=3000] - Delay in ms before auto-hiding.
     */
    constructor({ autoHideDelay = 3000 } = {}) {
        this.activeContextMenu = null;
        this.contextMenuTimer = null;
        this.contextMenuTarget = null;
        this.autoHideDelay = autoHideDelay;
        this.currentHideDelay = autoHideDelay;
    }

    /**
     * Sets the auto-hide delay based on the event type (touch vs mouse).
     * @param {Event} e
     */
    setDelayFromEvent(e) {
        const isTouch = (e && (
            e.pointerType === 'touch' ||
            e.pointerType === 'pen' ||
            (e.sourceCapabilities && e.sourceCapabilities.firesTouchEvents)
        ));
        this.currentHideDelay = isTouch ? 10000 : this.autoHideDelay;
    }

    /**
     * Shows a generic options context menu.
     * @param {MouseEvent} e - The mouse event for positioning.
     * @param {Array<string>} options - List of labels/values.
     * @param {Function} onSelect - Callback when an option is selected.
     */
    showOptions(e, options, onSelect) {
        this.setDelayFromEvent(e);
        this.hide();
        this.activeContextMenu = document.getElementById('options-context-menu');
        const content = document.getElementById('options-menu-content');
        if (!this.activeContextMenu || !content) {
            return;
        }

        content.innerHTML = '';
        options.forEach(opt => {
            const btn = createElement('button', {
                className: 'block w-full text-left px-4 py-2 hover:bg-slate-700 text-sm font-bold rounded transition-colors',
                text: opt,
                onClick: () => {
                    onSelect(opt);
                    this.hide();
                },
            });
            content.appendChild(btn);
        });

        this.position(e);
        this.resumeTimer();
    }

    /**
     * Shows the CSO long-press menu.
     * @param {MouseEvent} e - The mouse event.
     * @param {string} type - The type ('ball', 'strike', 'out').
     * @param {Function} onSelect - Callback(type, code).
     */
    showLongPressMenu(e, type, onSelect) {
        this.setDelayFromEvent(e);
        this.hide();
        this.activeContextMenu = document.getElementById('cso-long-press-submenu');
        const content = document.getElementById('submenu-content');
        if (!this.activeContextMenu || !content) {
            return;
        }

        let opts = [];
        if (type === 'ball') {
            opts = [{ l: 'Wild Pitch', c: 'WP' }, { l: 'Passed Ball', c: 'PB' }];
        } else if (type === 'strike') {
            opts = [{ l: PitchCodeSwinging, c: PitchCodeSwinging }, { l: PitchCodeCalled, c: PitchCodeCalled }, { l: PitchCodeDropped, c: PitchCodeDropped }];
        } else {
            opts = [{ l: 'Interference', c: 'Int' }, { l: 'Stepped Out', c: 'SO' }, { l: 'Batting Out of Order', c: 'BOO' }];
        }

        content.innerHTML = '';
        opts.forEach(o => {
            const btn = createElement('button', {
                className: 'sub-opt-btn',
                text: o.l,
                dataset: { t: type, c: o.c },
                onClick: () => {
                    onSelect(type, o.c);
                    this.hide();
                },
            });
            content.appendChild(btn);
        });

        // Use custom positioning logic for CSO (centered gravitating)
        this.positionCSO(e);
        this.resumeTimer();
    }

    /**
     * Shows the column context menu.
     * @param {MouseEvent} e
     * @param {object} options - { onAdd, onRemove, canRemove }
     */
    showColumnContextMenu(e, { onAdd, onRemove, canRemove }) {
        this.setDelayFromEvent(e);
        this.hide();
        this.activeContextMenu = document.getElementById('column-context-menu');
        if (!this.activeContextMenu) {
            return;
        }
        this.activeContextMenu.innerHTML = '';

        const addBtn = createElement('button', {
            className: 'block w-full text-left p-2 hover:bg-gray-700',
            text: 'Add Column',
            onClick: () => {
                onAdd();
                this.hide();
            },
        });
        this.activeContextMenu.appendChild(addBtn);

        if (canRemove) {
            const removeBtn = createElement('button', {
                className: 'block w-full text-left p-2 hover:bg-gray-700 text-red-400',
                text: 'Remove Column',
                onClick: () => {
                    onRemove();
                    this.hide();
                },
            });
            this.activeContextMenu.appendChild(removeBtn);
        }

        this.position(e);
        this.resumeTimer();
    }

    /**
     * Custom positioning for CSO menus (avoids off-screen).
     */
    positionCSO(e) {
        const menu = this.activeContextMenu;
        menu.style.top = 'auto'; menu.style.bottom = 'auto';
        menu.style.left = 'auto'; menu.style.right = 'auto';
        menu.style.position = 'fixed';
        menu.classList.remove('hidden');

        const screenH = window.innerHeight;
        const screenW = window.innerWidth;

        if (e.clientY > screenH * 0.6) {
            menu.style.bottom = `${screenH - e.clientY + 10}px`;
        } else {
            menu.style.top = `${e.clientY + 10}px`;
        }

        if (e.clientX > screenW / 2) {
            menu.style.right = `${screenW - e.clientX + 10}px`;
        } else {
            menu.style.left = `${e.clientX + 10}px`;
        }
    }

    /**
     * Shows the substitution menu.
     * @param {MouseEvent} e
     * @param {Function} onSubstitute
     */
    showSubstitutionMenu(e, onSubstitute) {
        this.setDelayFromEvent(e);
        this.hide();
        this.activeContextMenu = document.getElementById('player-context-menu');
        if (!this.activeContextMenu) {
            return;
        }
        this.activeContextMenu.innerHTML = '';

        const subBtn = createElement('button', {
            className: 'block w-full text-left p-2 hover:bg-gray-700',
            text: 'Substitute Player',
            id: 'btn-open-sub',
            onClick: () => {
                onSubstitute();
                this.hide();
            },
        });
        this.activeContextMenu.appendChild(subBtn);

        this.position(e);
        this.resumeTimer();
    }

    /**
     * Shows the player context menu.
     * @param {MouseEvent} e
     * @param {Function} onCorrect
     */
    showPlayerMenu(e, onCorrect) {
        this.setDelayFromEvent(e);
        this.hide();
        this.activeContextMenu = document.getElementById('player-context-menu');
        if (!this.activeContextMenu) {
            return;
        }
        this.activeContextMenu.innerHTML = '';

        const correctBtn = createElement('button', {
            className: 'block w-full text-left p-2 hover:bg-gray-700',
            text: 'Correct Player in Slot',
            onClick: () => {
                onCorrect();
                this.hide();
            },
        });
        this.activeContextMenu.appendChild(correctBtn);

        this.position(e);
        this.resumeTimer();
    }

    /**
     * Shows the cell context menu.
     * @param {MouseEvent} e
     * @param {object} options
     * @param {boolean} options.isLead
     * @param {Function} options.onToggleLead
     * @param {boolean} options.canMove
     * @param {Function} options.onMovePlay
     */
    showCellMenu(e, { isLead, onToggleLead, canMove, onMovePlay }) {
        this.setDelayFromEvent(e);
        this.hide();
        this.activeContextMenu = document.getElementById('column-context-menu');
        if (!this.activeContextMenu) {
            return;
        }
        this.activeContextMenu.innerHTML = '';

        const leadBtn = createElement('button', {
            className: 'block w-full text-left p-2 hover:bg-gray-700',
            text: isLead ? 'Unset Lead' : 'Set Lead',
            onClick: () => {
                onToggleLead();
                this.hide();
            },
        });
        this.activeContextMenu.appendChild(leadBtn);

        if (canMove) {
            const moveBtn = createElement('button', {
                className: 'block w-full text-left p-2 hover:bg-gray-700',
                text: 'Move Play To...',
                onClick: () => {
                    onMovePlay();
                    this.hide();
                },
            });
            this.activeContextMenu.appendChild(moveBtn);
        }

        this.position(e);
        this.resumeTimer();
    }

    /**
     * Hides the currently active context menu.
     */
    hide() {
        if (this.activeContextMenu) {
            this.activeContextMenu.classList.add('hidden');
            this.activeContextMenu = null;
        }
        this.clearTimer();
    }

    /**
     * Positions the menu relative to the viewport, ensuring it doesn't overflow.
     * @param {MouseEvent} e
     */
    position(e) {
        const menu = this.activeContextMenu;
        if (!menu) {
            return;
        }

        menu.style.top = ''; menu.style.left = '';
        menu.style.bottom = ''; menu.style.right = '';
        menu.style.position = 'fixed';
        menu.classList.remove('hidden');

        const rect = menu.getBoundingClientRect();
        const viewportW = window.innerWidth;
        const viewportH = window.innerHeight;

        let x = e.clientX + 10;
        let y = e.clientY + 10;

        if (x + rect.width > viewportW) {
            x = e.clientX - rect.width - 10;
        }
        if (y + rect.height > viewportH) {
            y = e.clientY - rect.height - 10;
        }

        if (x < 10) {
            x = 10;
        }
        if (y < 10) {
            y = 10;
        }

        menu.style.left = `${x}px`;
        menu.style.top = `${y}px`;
    }

    pauseTimer() {
        this.clearTimer();
    }

    resumeTimer() {
        if (!this.contextMenuTimer) {
            this.contextMenuTimer = setTimeout(() => this.hide(), this.currentHideDelay);
        }
    }

    clearTimer() {
        if (this.contextMenuTimer) {
            clearTimeout(this.contextMenuTimer);
            this.contextMenuTimer = null;
        }
    }
}
