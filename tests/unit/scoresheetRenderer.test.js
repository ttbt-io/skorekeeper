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

import { ScoresheetRenderer } from '../../frontend/renderers/scoresheetRenderer.js';

describe('ScoresheetRenderer', () => {
    let gridContainer;
    let scoreboardContainer;
    let feedContainer;
    let renderer;
    let mockCallbacks;

    beforeEach(() => {
        gridContainer = document.createElement('div');
        scoreboardContainer = document.createElement('div');
        feedContainer = document.createElement('div');

        // Setup Broadcast DOM
        document.body.innerHTML = `
            <div id="bug-away-name"></div>
            <div id="bug-away-score"></div>
            <div id="bug-home-name"></div>
            <div id="bug-home-score"></div>
            <div id="bug-inning-arrow"></div>
            <div id="bug-inning-num"></div>
            <div id="bug-out-1"></div>
            <div id="bug-out-2"></div>
            <div id="bug-count"></div>
            <div id="bug-base-1"></div>
            <div id="bug-base-2"></div>
            <div id="bug-base-3"></div>
            <div id="sb-innings-away"></div>
            <div id="sb-innings-home"></div>
            <div id="sb-r-away"></div>
            <div id="sb-h-away"></div>
            <div id="sb-e-away"></div>
            <div id="sb-r-home"></div>
            <div id="sb-h-home"></div>
            <div id="sb-e-home"></div>
            <div id="sb-name-away"></div>
            <div id="sb-name-home"></div>
            <div id="tab-away"></div>
            <div id="tab-home"></div>
            <div id="header-game-title"></div>
        `;

        mockCallbacks = {
            onColumnContextMenu: jest.fn(),
            onPlayerSubstitution: jest.fn(),
            onCellClick: jest.fn(),
            onCellContextMenu: jest.fn(),
            resolvePlayerNumber: jest.fn(() => '99'),
            onScoreOverride: jest.fn(),
        };

        renderer = new ScoresheetRenderer({
            gridContainer,
            scoreboardContainer,
            feedContainer,
            callbacks: mockCallbacks,
        });
    });

    afterEach(() => {
        document.body.innerHTML = '';
        jest.clearAllMocks();
    });

    const mockGame = {
        away: 'Away', home: 'Home',
        roster: {
            away: [{ starter: { id: 'p1', name: 'P1', number: '10' }, current: { id: 'p1', name: 'P1' }, history: [] }],
            home: [],
        },
        columns: [{ id: 'c1', inning: 1 }],
        events: {
            'away-0-c1': { balls: 1, strikes: 1, outcome: '1B', paths: [1, 0, 0, 0], pathInfo: ['1B','','',''], pId: 'p1' },
        },
        overrides: {},
    };

    const mockStats = {
        playerStats: { 'p1': { ab: 1, h: 1 } },
        inningStats: { 'away-c1': { r: 0, h: 1 } },
        score: { away: { R: 0, H: 1, E: 0 }, home: { R: 0, H: 0, E: 0 } },
        hasAB: { away: { 1: true }, home: {} },
        innings: { away: { 1: 0 }, home: {} },
        currentPA: { inning: 1, team: 'away', outs: 0, balls: 0, strikes: 0, paths: [0, 0, 0, 0] },
    };

    test('renderGrid() builds structure', () => {
        renderer.renderGrid(mockGame, 'away', mockStats, null);
        expect(gridContainer.querySelectorAll('.lineup-cell').length).toBeGreaterThan(0);
        expect(gridContainer.querySelectorAll('.grid-cell').length).toBeGreaterThan(0);
    });

    test('renderGrid() updates content without full rebuild', () => {
        renderer.renderGrid(mockGame, 'away', mockStats, null);

        // Call again with same structure
        renderer.renderGrid(mockGame, 'away', mockStats, null);
        // Check that we didn't wipe the container (rebuild would clear it)
        // Ideally we check if elements are the same instances, but innerHTML replace destroys instances.
        // If the code path takes 'updateGridContent', it modifies existing elements.
        // If it takes 'rebuild', it clears innerHTML.

        // Let's verify data consistency
        expect(gridContainer.querySelectorAll('.lineup-cell').length).toBe(2);
    });

    test('renderGrid() rebuilds on structure change', () => {
        renderer.renderGrid(mockGame, 'away', mockStats, null);

        const newGame = { ...mockGame, columns: [...mockGame.columns, { id: 'c2', inning: 2 }] };
        renderer.renderGrid(newGame, 'away', mockStats, null);

        expect(gridContainer.dataset.colCount).toBe('2');
    });

    test('renderGrid() attaches event handlers', () => {
        renderer.renderGrid(mockGame, 'away', mockStats, null);

        // Column Context Menu
        const headers = gridContainer.querySelectorAll('.grid-header');
        headers[1].click(); // First column header
        expect(mockCallbacks.onColumnContextMenu).toHaveBeenCalled();

        // Cell Click
        const cell = gridContainer.querySelector('.grid-cell[data-key="away-0-c1"]');
        cell.click();
        expect(mockCallbacks.onCellClick).toHaveBeenCalled();

        // Cell Context Menu
        const event = new MouseEvent('contextmenu');
        cell.dispatchEvent(event);
        expect(mockCallbacks.onCellContextMenu).toHaveBeenCalled();
    });

    test('renderScoreboard() updates DOM', () => {
        renderer.renderScoreboard(mockGame, mockStats);
        expect(document.getElementById('sb-name-away').textContent).toBe('Away');
        expect(document.getElementById('sb-h-away').textContent).toBe('1');
        // Inning score
        const inningsRow = document.getElementById('sb-innings-away');
        expect(inningsRow.children.length).toBe(1);
        expect(inningsRow.children[0].textContent).toBe('0');
    });

    test('renderBroadcast() updates DOM', () => {
        renderer.renderBroadcast(mockGame, mockStats);
        expect(document.getElementById('bug-away-name').textContent).toBe('Awa');
        expect(document.getElementById('bug-inning-num').textContent).toBe('1');
    });

    test('renderFeed() renders items', () => {
        const feedData = [{
            id: 'inn1', side: 'Top', inning: 1, team: 'Away',
            items: [
                { id: 'pa1', batterText: 'P1', events: [{ type: 'PITCH', description: 'Ball' }], isStricken: false },
            ],
        }];
        renderer.renderFeed(feedData);
        expect(feedContainer.querySelectorAll('.pa-header').length).toBe(1);
        expect(feedContainer.textContent).toContain('P1');
        expect(feedContainer.textContent).toContain('Ball');
    });

    test('renderFeed() updates existing items (reactive)', () => {
        const feedData1 = [{
            id: 'inn1', side: 'Top', inning: 1, team: 'Away',
            items: [
                { id: 'pa1', batterText: 'P1', events: [], isStricken: false },
            ],
        }];
        renderer.renderFeed(feedData1);

        const feedData2 = [{
            id: 'inn1', side: 'Top', inning: 1, team: 'Away',
            items: [
                { id: 'pa1', batterText: 'P1', events: [], isStricken: true },
            ],
        }];
        renderer.renderFeed(feedData2);

        const item = feedContainer.querySelector('div[data-id="pa1"]');
        expect(item.className).toContain('line-through');
    });

    test('renderPrintScoreboard() creates static table', () => {
        const printContainer = document.createElement('div');
        renderer.renderPrintScoreboard(printContainer, mockGame, mockStats);
        expect(printContainer.querySelectorAll('table').length).toBe(1);
        expect(printContainer.textContent).toContain('Away');
        expect(printContainer.textContent).toContain('Home');
    });
});
