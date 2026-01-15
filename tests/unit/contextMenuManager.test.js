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

import { ContextMenuManager } from '../../frontend/ui/contextMenuManager.js';

describe('ContextMenuManager', () => {
    let manager;
    let mockEvent;

    // DOM elements
    let optionsMenu, optionsContent;
    let csoMenu, csoContent;
    let columnMenu;
    let playerMenu;

    beforeEach(() => {
        jest.useFakeTimers();

        // Setup DOM
        document.body.innerHTML = `
            <div id="options-context-menu" class="hidden"></div>
            <div id="options-menu-content"></div>
            <div id="cso-long-press-submenu" class="hidden"></div>
            <div id="submenu-content"></div>
            <div id="column-context-menu" class="hidden"></div>
            <div id="player-context-menu" class="hidden"></div>
        `;

        optionsMenu = document.getElementById('options-context-menu');
        optionsContent = document.getElementById('options-menu-content');
        csoMenu = document.getElementById('cso-long-press-submenu');
        csoContent = document.getElementById('submenu-content');
        columnMenu = document.getElementById('column-context-menu');
        playerMenu = document.getElementById('player-context-menu');

        // Mock getBoundingClientRect
        const mockRect = { width: 100, height: 100, top: 0, left: 0, bottom: 0, right: 0 };
        optionsMenu.getBoundingClientRect = () => mockRect;
        csoMenu.getBoundingClientRect = () => mockRect;
        columnMenu.getBoundingClientRect = () => mockRect;
        playerMenu.getBoundingClientRect = () => mockRect;

        // Mock window dimensions
        window.innerWidth = 1000;
        window.innerHeight = 800;

        mockEvent = {
            clientX: 100,
            clientY: 100,
            pointerType: 'mouse',
            preventDefault: jest.fn(),
            stopPropagation: jest.fn(),
        };

        manager = new ContextMenuManager();
    });

    afterEach(() => {
        jest.useRealTimers();
        document.body.innerHTML = '';
    });

    test('constructor sets default options', () => {
        expect(manager.autoHideDelay).toBe(3000);
        expect(manager.currentHideDelay).toBe(3000);
    });

    test('setDelayFromEvent detects touch', () => {
        const touchEvent = { pointerType: 'touch' };
        manager.setDelayFromEvent(touchEvent);
        expect(manager.currentHideDelay).toBe(10000);

        const mouseEvent = { pointerType: 'mouse' };
        manager.setDelayFromEvent(mouseEvent);
        expect(manager.currentHideDelay).toBe(3000);
    });

    test('showOptions populates and shows menu', () => {
        const onSelect = jest.fn();
        const options = ['Opt 1', 'Opt 2'];

        manager.showOptions(mockEvent, options, onSelect);

        expect(manager.activeContextMenu).toBe(optionsMenu);
        expect(optionsMenu.classList.contains('hidden')).toBe(false);
        expect(optionsContent.children.length).toBe(2);
        expect(optionsContent.children[0].textContent).toBe('Opt 1');

        // Test interaction
        optionsContent.children[0].click();
        expect(onSelect).toHaveBeenCalledWith('Opt 1');
        expect(optionsMenu.classList.contains('hidden')).toBe(true);
    });

    test('showLongPressMenu populates and shows menu (ball)', () => {
        const onSelect = jest.fn();

        manager.showLongPressMenu(mockEvent, 'ball', onSelect);

        expect(manager.activeContextMenu).toBe(csoMenu);
        expect(csoMenu.classList.contains('hidden')).toBe(false);
        expect(csoContent.children.length).toBe(2); // WP, PB
        expect(csoContent.children[0].textContent).toBe('Wild Pitch');

        // Test interaction
        csoContent.children[0].click();
        expect(onSelect).toHaveBeenCalledWith('ball', 'WP');
    });

    test('showLongPressMenu populates and shows menu (strike)', () => {
        const onSelect = jest.fn();
        manager.showLongPressMenu(mockEvent, 'strike', onSelect);
        expect(csoContent.children.length).toBe(3); // Swinging, Called, Dropped
    });

    test('showLongPressMenu populates and shows menu (other/out)', () => {
        const onSelect = jest.fn();
        manager.showLongPressMenu(mockEvent, 'out', onSelect); // or any other
        expect(csoContent.children.length).toBe(3); // Interference, SO, BOO
    });

    test('showColumnContextMenu populates and shows menu', () => {
        const onAdd = jest.fn();
        const onRemove = jest.fn();

        manager.showColumnContextMenu(mockEvent, { onAdd, onRemove, canRemove: true });

        expect(manager.activeContextMenu).toBe(columnMenu);
        expect(columnMenu.classList.contains('hidden')).toBe(false);
        expect(columnMenu.children.length).toBe(2); // Add, Remove

        // Click Add
        columnMenu.children[0].click();
        expect(onAdd).toHaveBeenCalled();
        expect(columnMenu.classList.contains('hidden')).toBe(true);

        // Show again to test Remove
        manager.showColumnContextMenu(mockEvent, { onAdd, onRemove, canRemove: true });
        columnMenu.children[1].click();
        expect(onRemove).toHaveBeenCalled();
    });

    test('showColumnContextMenu handles canRemove=false', () => {
        manager.showColumnContextMenu(mockEvent, { onAdd: jest.fn(), onRemove: jest.fn(), canRemove: false });
        expect(columnMenu.children.length).toBe(1); // Only Add
    });

    test('showSubstitutionMenu populates and shows menu', () => {
        const onSubstitute = jest.fn();

        manager.showSubstitutionMenu(mockEvent, onSubstitute);

        expect(manager.activeContextMenu).toBe(playerMenu);
        expect(playerMenu.classList.contains('hidden')).toBe(false);
        expect(playerMenu.children.length).toBe(1);
        expect(playerMenu.children[0].textContent).toBe('Substitute Player');

        playerMenu.children[0].click();
        expect(onSubstitute).toHaveBeenCalled();
    });

    test('showPlayerMenu populates and shows menu', () => {
        const onCorrect = jest.fn();

        manager.showPlayerMenu(mockEvent, onCorrect);

        expect(manager.activeContextMenu).toBe(playerMenu);
        expect(playerMenu.classList.contains('hidden')).toBe(false);
        expect(playerMenu.children.length).toBe(1);
        expect(playerMenu.children[0].textContent).toBe('Correct Player in Slot');

        playerMenu.children[0].click();
        expect(onCorrect).toHaveBeenCalled();
    });

    test('showCellMenu populates and shows menu', () => {
        const onToggleLead = jest.fn();
        const onMovePlay = jest.fn();

        // Lead: false, canMove: true
        manager.showCellMenu(mockEvent, { isLead: false, onToggleLead, canMove: true, onMovePlay });

        expect(manager.activeContextMenu).toBe(columnMenu); // It reuses column-context-menu ID in code
        expect(columnMenu.children.length).toBe(2);
        expect(columnMenu.children[0].textContent).toBe('Set Lead');
        expect(columnMenu.children[1].textContent).toBe('Move Play To...');

        columnMenu.children[0].click();
        expect(onToggleLead).toHaveBeenCalled();
    });

    test('showCellMenu handles isLead: true and canMove: false', () => {
        const onToggleLead = jest.fn();
        manager.showCellMenu(mockEvent, { isLead: true, onToggleLead, canMove: false });

        expect(columnMenu.children.length).toBe(1);
        expect(columnMenu.children[0].textContent).toBe('Unset Lead');
    });

    test('auto-hide timer works', () => {
        const onSelect = jest.fn();
        manager.showOptions(mockEvent, ['Opt'], onSelect);

        expect(manager.activeContextMenu).not.toBeNull();

        jest.runAllTimers();

        expect(manager.activeContextMenu).toBeNull();
        expect(optionsMenu.classList.contains('hidden')).toBe(true);
    });

    test('pauseTimer and resumeTimer work', () => {
        manager.showOptions(mockEvent, ['Opt'], () => {
        });

        manager.pauseTimer();
        // Advance time past delay
        jest.advanceTimersByTime(4000);
        // Should still be open
        expect(manager.activeContextMenu).not.toBeNull();

        manager.resumeTimer();
        jest.advanceTimersByTime(4000);
        // Should be closed now
        expect(manager.activeContextMenu).toBeNull();
    });

    test('position logic clamps to viewport (right/bottom edge)', () => {
        // Mock window size
        window.innerWidth = 500;
        window.innerHeight = 500;

        // Click near bottom right
        mockEvent.clientX = 480;
        mockEvent.clientY = 480;

        manager.showOptions(mockEvent, ['Opt'], () => {
        });

        // Menu is 100x100 (mocked)
        // Logic: if x + width > viewportW -> x = clientX - width - 10
        // 480 + 100 > 500 -> x = 480 - 100 - 10 = 370
        // 480 + 100 > 500 -> y = 480 - 100 - 10 = 370

        expect(optionsMenu.style.left).toBe('370px');
        expect(optionsMenu.style.top).toBe('370px');
    });

    test('position logic clamps to viewport (top/left edge)', () => {
        // Click near top left, but logic handles < 10 case
        // Logic: if x < 10 -> x = 10
        mockEvent.clientX = -50;
        mockEvent.clientY = -50;

        manager.showOptions(mockEvent, ['Opt'], () => {
        });

        expect(optionsMenu.style.left).toBe('10px');
        expect(optionsMenu.style.top).toBe('10px');
    });

    test('positionCSO logic (bottom half)', () => {
        window.innerWidth = 1000;
        window.innerHeight = 1000;

        // Click in bottom half
        mockEvent.clientY = 700; // > 600
        mockEvent.clientX = 400; // < 500

        manager.showLongPressMenu(mockEvent, 'ball', () => {
        });

        // Bottom logic: bottom = screenH - clientY + 10
        // 1000 - 700 + 10 = 310
        expect(csoMenu.style.bottom).toBe('310px');
        expect(csoMenu.style.left).toBe('410px'); // 400 + 10
        expect(csoMenu.style.top).toBe('auto');
    });

    test('positionCSO logic (top right)', () => {
        window.innerWidth = 1000;
        window.innerHeight = 1000;

        // Click in top right
        mockEvent.clientY = 200; // < 600
        mockEvent.clientX = 800; // > 500

        manager.showLongPressMenu(mockEvent, 'ball', () => {
        });

        expect(csoMenu.style.top).toBe('210px'); // 200 + 10
        expect(csoMenu.style.right).toBe('210px'); // 1000 - 800 + 10 = 210
        expect(csoMenu.style.left).toBe('auto');
    });

    test('hide handles null active menu', () => {
        manager.activeContextMenu = null;
        expect(() => manager.hide()).not.toThrow();
    });

    test('show methods return early if elements missing', () => {
        document.body.innerHTML = '';
        manager = new ContextMenuManager();

        expect(() => manager.showOptions(mockEvent, [], () => {
        })).not.toThrow();
        expect(() => manager.showLongPressMenu(mockEvent, 'ball', () => {
        })).not.toThrow();
        expect(() => manager.showColumnContextMenu(mockEvent, {})).not.toThrow();
        expect(() => manager.showSubstitutionMenu(mockEvent, () => {
        })).not.toThrow();
        expect(() => manager.showPlayerMenu(mockEvent, () => {
        })).not.toThrow();
        expect(() => manager.showCellMenu(mockEvent, {})).not.toThrow();
    });
});
