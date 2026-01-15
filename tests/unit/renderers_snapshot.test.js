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

import { CSORenderer } from '../../frontend/renderers/csoRenderer.js';
import { DashboardRenderer } from '../../frontend/renderers/dashboardRenderer.js';
import { ScoresheetRenderer } from '../../frontend/renderers/scoresheetRenderer.js';
import { StatsRenderer } from '../../frontend/renderers/statsRenderer.js';
import { TeamsRenderer } from '../../frontend/renderers/teamsRenderer.js';

// Mock utils.js to prevent timezone issues with formatDate
jest.mock('../../frontend/utils.js', () => {
    const originalUtils = jest.requireActual('../../frontend/utils.js');
    return {
        ...originalUtils,
        formatDate: () => '12/31/2024',
    };
});

// Mock Utils if necessary, but we are using JSDOM so element creation is fine.
// We rely on createTextNode, createElement, etc.

describe('Renderers Snapshots', () => {
    let container;
    let mockCallbacks;

    beforeEach(() => {
        container = document.createElement('div');
        container.id = 'test-container';
        document.body.appendChild(container);

        mockCallbacks = {
            onCycleOuts: jest.fn(),
            resolvePlayerNumber: jest.fn(_id => '99'),
            onApplyRunnerAction: jest.fn(),
            onRBIEdit: jest.fn(),
            onGameSelect: jest.fn(),
            onDeleteGame: jest.fn(),
            onEditGame: jest.fn(),
            onBackup: jest.fn(),
            onRestore: jest.fn(),
            onClearCache: jest.fn(),
            onOpenTeam: jest.fn(),
            onNavigate: jest.fn(),
        };
    });

    afterEach(() => {
        document.body.innerHTML = '';
    });

    const mockGame = {
        id: 'game-1',
        date: '2025-01-01',
        away: 'Away Team',
        home: 'Home Team',
        status: 'in_progress',
        innings: [
            { away: { runs: 1, hits: 2, errors: 0 }, home: { runs: 0, hits: 0, errors: 0 } },
        ],
        teams: {
            away: { name: 'Away Team' },
            home: { name: 'Home Team' },
        },
        roster: {
            away: [{ slot: 1, starter: { id: 'p1', name: 'Player 1', number: '10' }, current: { id: 'p1', name: 'Player 1', number: '10' }, history: [] }],
            home: [{ slot: 1, starter: { id: 'p2', name: 'Player 2', number: '20' }, current: { id: 'p2', name: 'Player 2', number: '20' }, history: [] }],
        },
        columns: [
            { id: '1', label: '1', team: null },
            { id: '2', label: '2', team: null },
        ],
        lineups: {
            away: ['p1'],
            home: ['p2'],
        },
        events: {},
        batter: 'p1',
        activeTeam: 'away',
        outs: 1,
        balls: 2,
        strikes: 1,
        runners: { '1B': 'p3' },
    };

    test('DashboardRenderer Snapshot', () => {
        const renderer = new DashboardRenderer({ container, callbacks: mockCallbacks });
        const games = [mockGame];
        const currentUser = { email: 'user@example.com' };

        renderer.render(games, currentUser);
        expect(container.innerHTML).toMatchSnapshot();
    });

    test('ScoresheetRenderer Snapshot', () => {
        const scoreboardContainer = document.createElement('div');
        const feedContainer = document.createElement('div');
        document.body.appendChild(scoreboardContainer);
        document.body.appendChild(feedContainer);

        const renderer = new ScoresheetRenderer({
            gridContainer: container,
            scoreboardContainer: scoreboardContainer,
            feedContainer: feedContainer,
            callbacks: mockCallbacks,
        });

        const mockStats = {
            playerStats: {
                'p1': { PA: 10, AB: 8 },
                'p2': { IP: 5 },
            },
            inningStats: {},
        };
        // renderGrid(game, team, stats, activeCtx, options)
        renderer.renderGrid(mockGame, 'away', mockStats, { b: 0, s: 0, o: 0 }, { isPrint: false });
        expect(container.innerHTML).toMatchSnapshot();
    });

    test('CSORenderer Snapshot', () => {
        // Setup specific DOM elements expected by CSORenderer
        const zoomContainer = document.createElement('div');
        zoomContainer.id = 'zoom-view';
        const bipFieldSvg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        bipFieldSvg.id = 'bip-field-svg';

        document.body.appendChild(zoomContainer);
        document.body.appendChild(bipFieldSvg);

        // Additional UI elements updated by render
        const outcomeEl = document.createElement('div'); outcomeEl.id = 'zoom-outcome-text';
        const pitchSeq = document.createElement('div'); pitchSeq.id = 'pitch-sequence-container';
        const actionAreaPitch = document.createElement('div'); actionAreaPitch.id = 'action-area-pitch';
        const actionAreaRecorded = document.createElement('div'); actionAreaRecorded.id = 'action-area-recorded';
        const sideControls = document.createElement('div'); sideControls.id = 'side-controls';
        const btnRunnerActions = document.createElement('button'); btnRunnerActions.id = 'btn-runner-actions';

        document.body.append(outcomeEl, pitchSeq, actionAreaPitch, actionAreaRecorded, sideControls, btnRunnerActions);

        const renderer = new CSORenderer({ zoomContainer, bipFieldSvg, callbacks: mockCallbacks });
        const state = {
            activeGame: mockGame,
            activeData: {
                outcome: 'Ball',
                pitchSequence: [{ type: 'ball' }, { type: 'strike', code: 'Called' }],
                balls: 2,
                strikes: 1,
                outNum: 1,
                paths: [1, 0, 0, 0], // runner on 1st
                pathInfo: ['1B', '', '', ''],
                hitData: null,
            },
            activeCtx: { b: 2, s: 1, o: 1 },
            bipState: { hitData: null },
            isEditing: false,
            isReadOnly: false,
            isLocationMode: false,
            runnersOnBase: [{ idx: 0, base: 0, name: 'Runner 1' }], // base 0 = 1st
        };

        renderer.render(state, mockGame);

        expect(zoomContainer.innerHTML).toMatchSnapshot('Zoom Container');
        // We can also check side effects on other elements if we want, but snapshotting the zoom container is key.
    });

    test('TeamsRenderer Snapshot', () => {
        const listContainer = document.createElement('div');
        const membersContainer = document.createElement('div');
        document.body.appendChild(listContainer);
        document.body.appendChild(membersContainer);

        const renderer = new TeamsRenderer({ listContainer, membersContainer, callbacks: mockCallbacks });
        const teams = [
            { id: 't1', name: 'Tigers', color: 'orange', roster: [] },
            { id: 't2', name: 'Bears', color: 'brown', roster: [] },
        ];

        // renderTeamsList(teams, currentTeamId)
        renderer.renderTeamsList(teams, 't1');
        expect(listContainer.innerHTML).toMatchSnapshot();
    });

    test('StatsRenderer Snapshot', () => {
        const renderer = new StatsRenderer({ container, callbacks: mockCallbacks });
        const state = {
            activeGame: mockGame,
            stats: {
                'p1': { PA: 10, AB: 8, H: 3, HR: 1, AVG: 0.375 },
                'p2': { IP: 5, ER: 2, K: 4, ERA: 3.60 },
            },
        };

        // Mock filters
        const filterContainer = document.createElement('div');
        filterContainer.id = 'stats-filters';
        // Mock expected inputs inside filter container if render uses them
        // StatsRenderer usually reads checked state of radios/checkboxes
        // If they don't exist, it might use defaults.
        document.body.appendChild(filterContainer);

        renderer.render(state);
        expect(container.innerHTML).toMatchSnapshot();
    });
});