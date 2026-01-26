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

import { AppController } from '../../frontend/controllers/AppController.js';
import { ActionTypes } from '../../frontend/reducer.js';

describe('CSO Substitution Flow', () => {
    let app;
    let mockDB;
    let mockModalPrompt;
    let mockModalConfirm;

    beforeEach(() => {
        // Setup DOM
        document.body.innerHTML = `
            <div id="dashboard-view"></div>
            <div id="teams-view"></div>
            <div id="team-view"></div>
            <div id="profile-view"></div>
            <div id="statistics-view"></div>
            <div id="manual-view"></div>
            <div id="scoresheet-view" class="hidden">
                <div id="game-status-indicator" class="hidden"></div>
                <div id="sb-name-away"></div>
                <div id="sb-name-home"></div>
                <div id="tab-away"></div>
                <div id="tab-home"></div>
                <div id="sb-r-away"></div>
                <div id="sb-h-away"></div>
                <div id="sb-e-away"></div>
                <div id="sb-r-home"></div>
                <div id="sb-h-home"></div>
                <div id="sb-e-home"></div>
                <div id="sb-innings-away"></div>
                <div id="sb-innings-home"></div>
                <div id="sb-header-innings"></div>
                <div id="scoresheet-grid"></div>
                <button id="btn-undo"></button>
                <button id="btn-redo"></button>
                <button id="btn-add-inning"></button>
                <button id="btn-share-game"></button>
            </div>
            <div id="new-game-modal" class="hidden">
                <form id="new-game-form"></form>
                <input id="game-event-input">
                <input id="game-location-input">
                <input id="game-date-input">
                <select id="team-away-select"></select>
                <input id="team-away-input">
                <select id="team-home-select"></select>
                <input id="team-home-input">
                <button id="btn-start-new-game"></button>
                <button id="btn-cancel-new-game"></button>
            </div>
            <div id="edit-game-modal" class="hidden">
                <form id="edit-game-form"></form>
                <button id="btn-cancel-edit-game"></button>
                <button id="btn-save-edit-game"></button>
            </div>
            <div id="edit-lineup-modal" class="hidden">
                <button id="btn-cancel-lineup"></button>
                <button id="btn-save-lineup"></button>
                <button id="btn-add-starter-row"></button>
                <button id="btn-add-sub-row"></button>
                <input id="lineup-team-name">
            </div>
            <div id="cso-modal" class="hidden">
                <div id="cso-header">
                    <div id="cso-title"></div>
                    <div id="cso-subtitle"></div>
                    <div id="cso-pitcher-num"></div>
                </div>
                <div id="cso-main-view"></div>
                <div id="cso-ball-in-play-view"></div>
                <div id="pitch-sequence-container"></div>
                <div id="action-area-pitch">
                    <button id="btn-ball"></button>
                    <button id="btn-strike"></button>
                    <button id="btn-foul"></button>
                    <button id="btn-out"></button>
                    <button id="btn-clear-all"></button>
                </div>
                <div id="action-area-recorded" class="hidden">
                    <button id="btn-toggle-action"></button>
                </div>
                <div id="side-controls">
                    <button id="btn-runner-actions"></button>
                    <button id="btn-show-bip"></button>
                </div>
                <button id="btn-undo-pitch"></button>
                <button id="btn-close-cso"></button>
                <button id="btn-change-pitcher"></button>
                <div id="cso-bip-view" class="hidden">
                    <div class="field-svg-keyboard">
                        <svg></svg>
                    </div>
                    <button id="btn-loc"></button>
                    <button id="btn-clear-loc"></button>
                    <button id="btn-res"></button>
                    <button id="btn-base"></button>
                    <button id="btn-type"></button>
                    <button id="btn-cancel-bip"></button>
                    <button id="btn-save-bip"></button>
                    <div id="div-traj-controls">
                        <button id="btn-traj"></button>
                    </div>
                </div>
                <div id="cso-runner-action-view" class="hidden">
                    <button id="btn-close-runner-action"></button>
                    <button id="btn-save-runner-actions"></button>
                </div>
                <div id="cso-runner-advance-view" class="hidden">
                    <button id="btn-finish-turn"></button>
                    <button id="btn-runner-advance-close"></button>
                </div>
            </div>
            <div id="substitution-modal" class="hidden">
                <span id="sub-outgoing-name"></span>
                <span id="sub-outgoing-num"></span>
                <input id="sub-incoming-num">
                <input id="sub-incoming-name">
                <input id="sub-incoming-pos">
                <datalist id="sub-options"></datalist>
                <button id="btn-cancel-sub"></button>
                <button id="btn-confirm-sub"></button>
                <form id="sub-form"></form>
            </div>
            <div id="conflict-resolution-modal" class="hidden">
                <button id="btn-conflict-force-save"></button>
                <button id="btn-conflict-overwrite"></button>
                <button id="btn-conflict-fork"></button>
            </div>
            <div id="backup-modal" class="hidden">
                <button id="btn-cancel-backup"></button>
                <button id="btn-start-backup"></button>
                <button id="btn-open-restore"></button>
            </div>
            <div id="restore-modal" class="hidden">
                <button id="btn-cancel-restore"></button>
                <button id="btn-select-restore-file"></button>
                <input id="restore-file-input" type="file">
                <button id="btn-restore-select-all"></button>
                <button id="btn-restore-select-none"></button>
                <button id="btn-confirm-restore"></button>
            </div>
            <div id="app-sidebar" class="hidden">
                <button id="sidebar-btn-dashboard"></button>
                <button id="sidebar-btn-teams"></button>
                <button id="sidebar-btn-view-grid"></button>
                <button id="sidebar-btn-view-feed"></button>
                <button id="sidebar-btn-add-inning"></button>
                <button id="sidebar-btn-end-game"></button>
                <div id="sidebar-game-actions"></div>
                <div id="sidebar-export-actions"></div>
            </div>
            <div id="grid-container"></div>
            <div id="scoresheet-scoreboard"></div>
            <div class="cso-zoom-container">
                <svg></svg>
            </div>
        `;

        mockDB = {
            open: jest.fn().mockResolvedValue(true),
            saveGame: jest.fn().mockResolvedValue(true),
            saveGameStats: jest.fn().mockResolvedValue(true),
            loadGame: jest.fn().mockResolvedValue(null),
            getAllGames: jest.fn().mockResolvedValue([]),
            getAllTeams: jest.fn().mockResolvedValue([]),
            getLocalRevisions: jest.fn().mockResolvedValue(new Map()),
        };
        mockModalPrompt = jest.fn();
        mockModalConfirm = jest.fn();

        app = new AppController(mockDB, mockModalPrompt, mockModalConfirm);

        // Mock substitutionManager.handleSubstitution
        app.substitutionManager.handleSubstitution = jest.fn().mockImplementation(async(team, idx, subParams, activeCtx) => {
            const id = 'test-sub-id';
            const action = {
                id,
                type: ActionTypes.SUBSTITUTION,
                payload: { team, rosterIndex: idx, subParams, activeCtx, actionId: id },
            };
            await app.dispatch(action);
            return Promise.resolve(action);
        });

        // Ensure ActiveGameController uses our app instance
        app.activeGameController.app = app;

        // Setup a mock game state
        app.state.activeGame = {
            id: 'test-game',
            away: 'Away',
            home: 'Home',
            status: 'ongoing',
            pitchers: { away: 'P1', home: 'P2' },
            roster: {
                'away': [
                    { slot: 1, starter: { id: 'p1', name: 'Player 1', number: '1' }, current: { id: 'p1', name: 'Player 1', number: '1' }, history: [] },
                    { slot: 2, starter: { id: 'p2', name: 'Player 2', number: '2' }, current: { id: 'p2', name: 'Player 2', number: '2' }, history: [] },
                ],
                'home': [
                    { slot: 1, starter: { id: 'h1', name: 'H Player 1', number: '1' }, current: { id: 'h1', name: 'H Player 1', number: '1' }, history: [] },
                    { slot: 2, starter: { id: 'h2', name: 'H Player 2', number: '2' }, current: { id: 'h2', name: 'H Player 2', number: '2' }, history: [] },
                ],
            },
            subs: { 'away': [{ id: 'sub1', name: 'Sub 1', number: '99', pos: 'SS' }], 'home': [] },
            events: {},
            columns: [{ id: 'col-1-0', inning: 1 }],
            actionLog: [
                {
                    id: 'game-start',
                    type: ActionTypes.GAME_START,
                    payload: {
                        id: 'test-game',
                        away: 'Away',
                        home: 'Home',
                        initialRosters: {
                            away: [{ id: 'p1', name: 'Player 1', number: '1' }, { id: 'p2', name: 'Player 2', number: '2' }],
                            home: [{ id: 'h1', name: 'H Player 1', number: '1' }, { id: 'h2', name: 'H Player 2', number: '2' }],
                        },
                        initialSubs: { away: [{ id: 'sub1', name: 'Sub 1', number: '99', pos: 'SS' }] },
                    },
                },
            ],
        };
        app.state.activeTeam = 'away';
        app.state.activeCtx = { b: 0, i: 1, col: 'col-1-0' };
        app.state.activeData = {
            outcome: '',
            balls: 0,
            strikes: 0,
            outNum: 0,
            paths: [0, 0, 0, 0],
            pathInfo: ['', '', '', ''],
            pitchSequence: [],
            pId: 'p1',
        };
    });

    test('clicking CSO title should open substitution modal', () => {
        app.openCSO(0, 1, 'col-1-0');
        const titleEl = document.getElementById('cso-title');

        expect(titleEl.textContent).toContain('Player 1');

        // Simulate click
        titleEl.click();

        expect(document.getElementById('substitution-modal').classList.contains('hidden')).toBe(false);
        expect(document.getElementById('sub-outgoing-name').textContent).toBe('Player 1');
    });

    test('completing substitution in CSO should update pitch sequence and pId', async() => {
        app.openCSO(0, 1, 'col-1-0');
        app.openSubstitutionModal('away', 0);

        document.getElementById('sub-incoming-num').value = '99';
        document.getElementById('sub-incoming-name').value = 'Sub 1';
        document.getElementById('sub-incoming-pos').value = 'SS';

        jest.spyOn(app, 'saveState').mockResolvedValue(true);

        await app.handleSubstitution();

        // Verify activeData state
        expect(app.state.activeData.pId).toBe('sub1'); // Based on mock implementation above returning 'test-sub-id' as refId, but logic sets pId to newId
        // Wait, logic: const newId = existing && existing.id ? existing.id : generateUUID();
        // Here we provide inputs for 'Sub 1'. It matches subs['away'][0].
        // The mock sub has id 'sub1'.
        // So newId should be 'sub1'.

        expect(app.state.activeData.pitchSequence).toHaveLength(1);
        expect(app.state.activeData.pitchSequence[0].type).toBe('substitution');
        expect(app.state.activeData.pitchSequence[0].refId).toBe('test-sub-id');

        // Verify CSO title updated
        expect(document.getElementById('cso-title').textContent).toContain('Sub 1');
    });

    test('undoPitch should restore previous player when last action was substitution', async() => {
        app.openCSO(0, 1, 'col-1-0');

        // 1. Perform substitution
        app.openSubstitutionModal('away', 0);
        document.getElementById('sub-incoming-num').value = '99';
        document.getElementById('sub-incoming-name').value = 'Sub 1';
        document.getElementById('sub-incoming-pos').value = 'SS';

        // Mock dispatch to spy on calls
        const dispatchSpy = jest.spyOn(app, 'dispatch');
        jest.spyOn(app, 'saveState').mockResolvedValue(true);

        await app.handleSubstitution();

        const subActionId = app.state.activeData.pitchSequence[0].refId;

        // 3. Trigger Undo
        await app.undoPitch();

        expect(dispatchSpy).toHaveBeenCalledWith(expect.objectContaining({
            type: ActionTypes.UNDO,
            payload: { refId: subActionId },
        }));

        expect(app.state.activeData.pitchSequence).toHaveLength(0);
        expect(app.state.activeData.pId).toBe('p1');
    });
});