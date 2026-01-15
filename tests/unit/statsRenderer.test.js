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

import { StatsRenderer } from '../../frontend/renderers/statsRenderer.js';

describe('StatsRenderer', () => {
    let container;
    let renderer;
    let mockCallbacks;

    beforeEach(() => {
        container = document.createElement('div');
        document.body.appendChild(container);

        mockCallbacks = {
            onOpenPlayerProfile: jest.fn(),
            onExportCSV: jest.fn(),
            onExportPDF: jest.fn(),
            getDerivedHittingStats: jest.fn(_p => ({
                avg: '.300', ops: '.900', obp: '.400', slg: '.500',
            })),
            getDerivedPitchingStats: jest.fn(_p => ({
                ip: '1.0', era: '0.00', whip: '1.00', kPct: '20%', walkPct: '5%', strikePct: '60%',
            })),
            calculateGameStats: jest.fn(() => ({
                playerStats: {
                    'p1': { ab: 3, h: 1 },
                },
            })),
        };

        renderer = new StatsRenderer({ container, callbacks: mockCallbacks });
    });

    afterEach(() => {
        document.body.innerHTML = '';
        localStorage.clear();
        jest.clearAllMocks();
    });

    test('render() handles empty stats', () => {
        renderer.render(null);
        expect(container.textContent).toContain('Loading statistics...');
    });

    test('render() renders export buttons', () => {
        renderer.render({});
        expect(container.textContent).toContain('Export CSV');
        expect(container.textContent).toContain('Export PDF');
    });

    test('render() calls callbacks on export', () => {
        renderer.render({});
        const btns = container.querySelectorAll('button');
        btns[0].click();
        expect(mockCallbacks.onExportCSV).toHaveBeenCalled();
        btns[1].click();
        expect(mockCallbacks.onExportPDF).toHaveBeenCalled();
    });

    test('render() renders hitting leaderboard', () => {
        const stats = {
            players: {
                'p1': { name: 'Slugger', pa: 10, ab: 8, h: 4, r: 2, rbi: 3, bb: 2, k: 1, hr: 1, games: 2 },
            },
        };
        renderer.render(stats);
        expect(container.innerHTML).toContain('Hitting Leaderboard');
        expect(container.innerHTML).toContain('Slugger');
        expect(container.innerHTML).toContain('.300'); // From mock
    });

    test('render() renders pitching leaderboard', () => {
        const stats = {
            pitchers: {
                'p2': { name: 'Ace', bf: 10, k: 3, bb: 1, h: 2, r: 0, er: 0, ip: 3, games: 1 },
            },
        };
        renderer.render(stats);
        expect(container.innerHTML).toContain('Pitching Leaderboard');
        expect(container.innerHTML).toContain('Ace');
        expect(container.innerHTML).toContain('0.00'); // From mock
    });

    test('render() renders team standings', () => {
        const stats = {
            teams: {
                't1': { name: 'Tigers', games: 2, w: 1, l: 1, t: 0, rs: 10, ra: 8 },
            },
        };
        renderer.render(stats);
        expect(container.innerHTML).toContain('Team Performance');
        expect(container.innerHTML).toContain('Tigers');
        expect(container.innerHTML).toContain('1-1-0');
        expect(container.innerHTML).toContain('+2');
    });

    test('render() triggers profile on row click', () => {
        const stats = {
            players: { 'p1': { name: 'Slugger', pa: 10 } },
        };
        renderer.render(stats);
        const row = container.querySelector('tbody tr');
        row.click();
        expect(mockCallbacks.onOpenPlayerProfile).toHaveBeenCalledWith('p1');
    });

    test('renderBoxScore() renders print view', () => {
        const stats = {
            playerStats: { 'p1': { name: 'P1', pa: 4 } },
            pitcherStats: { 'p2': { name: 'P2', bf: 15 } },
        };
        const game = {
            away: 'Away', home: 'Home',
            roster: {
                away: [{ starter: { id: 'p1', name: 'P1' } }],
                home: [{ starter: { id: 'p2', name: 'P2' } }], // P2 is pitcher and player
            },
        };

        renderer.render(stats, game, { isPrint: true });
        expect(container.innerHTML).toContain('Away Hitting');
        // Check for table structure
        expect(container.querySelectorAll('table').length).toBeGreaterThan(0);
    });

    test('renderPlayerProfile() populates modal', () => {
        // Setup modal in DOM
        document.body.innerHTML += `
            <div id="player-profile-modal" class="hidden">
                <div id="profile-name"></div>
                <div id="profile-subtitle"></div>
                <div id="profile-stats-card"></div>
                <div id="spray-markers"></div>
                <table><tbody id="profile-game-log"></tbody></table>
            </div>
        `;

        const stats = {
            players: { 'p1': { name: 'Hero', pa: 100, h: 30, hr: 5, games: 25 } },
            pitchers: { 'p1': { bf: 50, k: 10 } }, // Also pitched
        };
        const games = [
            { date: '2025-01-01', away: 'A', home: 'H', events: { 'e1': { pId: 'p1', hitData: { location: { x: 0.5, y: 0.5 }, trajectory: 'Line' }, outcome: '1B' } } },
        ];

        renderer.renderPlayerProfile('p1', stats, games);

        const modal = document.getElementById('player-profile-modal');
        expect(modal.classList.contains('hidden')).toBe(false);
        expect(document.getElementById('profile-name').textContent).toBe('Hero');
        expect(document.getElementById('profile-stats-card').textContent).toContain('30'); // Hits

        // Pitching breakdown check
        expect(modal.textContent).toContain('Pitching Performance');

        // Spray chart check
        expect(document.getElementById('spray-markers').children.length).toBeGreaterThan(0);
    });

    test('loadColumnConfig loads defaults', () => {
        const config = renderer.loadColumnConfig();
        expect(config.hitting).toContain('AVG');
        expect(config.pitching).toContain('ERA');
    });

    test('loadColumnConfig loads saved config', () => {
        localStorage.setItem('sk_stats_cols', JSON.stringify({ hitting: ['H'], pitching: ['K'] }));
        const config = renderer.loadColumnConfig();
        expect(config.hitting).toEqual(['H']);
    });

    test('saveColumnConfig saves to localStorage', () => {
        const config = { hitting: ['HR'], pitching: ['BB'] };
        renderer.saveColumnConfig(config);
        expect(localStorage.getItem('sk_stats_cols')).toContain('HR');
    });

    test('renderColumnSelector populates modal', () => {
        document.body.innerHTML += `
            <div id="stats-columns-modal" class="hidden">
                <div id="stats-cols-hitting"></div>
                <div id="stats-cols-pitching"></div>
            </div>
        `;
        renderer.renderColumnSelector();
        const modal = document.getElementById('stats-columns-modal');
        expect(modal.classList.contains('hidden')).toBe(false);
        expect(document.getElementById('stats-cols-hitting').children.length).toBeGreaterThan(0);
    });
});
