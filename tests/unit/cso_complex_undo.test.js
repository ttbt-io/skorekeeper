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

describe('CSO Complex Undo Scenarios', () => {
    let app;
    let mockDB;
    let mockModalPrompt;
    let mockModalConfirm;

    beforeEach(() => {
        document.body.innerHTML = `
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
            <div class="cso-zoom-container"><svg></svg></div>
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
            <div id="new-game-modal" class="hidden"></div>
            <div id="edit-game-modal" class="hidden"></div>
            <div id="edit-lineup-modal" class="hidden"></div>
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

        // Use Real Reducer Logic via ActiveGameController
        jest.spyOn(app, 'saveState').mockResolvedValue(true);
        jest.spyOn(app, 'renderCSO').mockImplementation(() => {
        });
        jest.spyOn(app, 'renderGrid').mockImplementation(() => {
        });

        // Mock substitutionManager to return action with ID but call dispatch
        app.substitutionManager.handleSubstitution = jest.fn().mockImplementation(async(team, idx, subParams, activeCtx) => {
            const id = 'sub-action-' + Math.random().toString(36).substr(2, 9);
            const action = {
                id,
                type: ActionTypes.SUBSTITUTION,
                payload: { team, rosterIndex: idx, subParams, activeCtx, actionId: id },
            };
            await app.dispatch(action);
            return action;
        });

        // Initialize State with Real Logic
        app.state.activeTeam = 'away';
        app.state.activeCtx = { b: 0, i: 1, col: 'col-1-0' };
        app.state.activeGame = {
            id: 'test-game',
            away: 'Away',
            home: 'Home',
            status: 'ongoing',
            pitchers: { away: 'P1', home: 'P2' },
            roster: {
                'away': [{ slot: 1, starter: { id: 'p1', name: 'Starter' }, current: { id: 'p1', name: 'Starter' }, history: [] }],
                'home': [],
            },
            subs: { 'away': [{ id: 'sub1', name: 'Sub', number: '99' }], 'home': [] },
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
                            away: [{ id: 'p1', name: 'Starter', number: '1' }],
                            home: [],
                        },
                        initialSubs: { away: [{ id: 'sub1', name: 'Sub', number: '99' }] },
                    },
                },
            ],
        };

        app.state.activeData = {
            outcome: '',
            balls: 0, strikes: 0, outNum: 0, paths: [0,0,0,0], pathInfo: ['','','',''],
            pitchSequence: [],
            pId: 'p1',
        };

        app.activeGameController.app = app;
    });

    const scenarios = [
        {
            name: 'Ball, Sub, Undo -> Ball',
            actions: ['ball', 'sub', 'undo'],
            expectedSeq: ['ball'],
            expectedPlayer: 'Starter',
        },
        {
            name: 'Ball, Sub, Strike, Undo -> Ball, Sub',
            actions: ['ball', 'sub', 'strike', 'undo'],
            expectedSeq: ['ball', 'substitution'],
            expectedPlayer: 'Sub',
        },
        {
            name: 'Sub, Undo -> Empty',
            actions: ['sub', 'undo'],
            expectedSeq: [],
            expectedPlayer: 'Starter',
        },
        {
            name: 'Sub, Strike, Undo -> Sub',
            actions: ['sub', 'strike', 'undo'],
            expectedSeq: ['substitution'],
            expectedPlayer: 'Sub',
        },
        {
            name: 'Ball, Ball, Ball, Sub, Undo -> Ball, Ball, Ball',
            actions: ['ball', 'ball', 'ball', 'sub', 'undo'],
            expectedSeq: ['ball', 'ball', 'ball'],
            expectedPlayer: 'Starter',
        },
        {
            name: 'Strike, Strike, Sub, Undo -> Strike, Strike',
            actions: ['strike', 'strike', 'sub', 'undo'],
            expectedSeq: ['strike', 'strike'],
            expectedPlayer: 'Starter',
        },
        {
            name: 'Foul, Sub, Foul, Undo -> Foul, Sub',
            actions: ['foul', 'sub', 'foul', 'undo'],
            expectedSeq: ['foul', 'substitution'],
            expectedPlayer: 'Sub',
        },
        {
            name: 'Sub, Sub (Correction), Undo -> Sub',
            actions: ['sub', 'sub', 'undo'],
            expectedSeq: ['substitution'],
            expectedPlayer: 'Sub',
        },
        {
            name: 'Ball, Sub, Strike, Undo, Undo -> Ball',
            actions: ['ball', 'sub', 'strike', 'undo', 'undo'],
            expectedSeq: ['ball'],
            expectedPlayer: 'Starter',
        },
        {
            name: 'Ball, Sub, Strike, Undo, Ball -> Ball, Sub, Ball',
            actions: ['ball', 'sub', 'strike', 'undo', 'ball'],
            expectedSeq: ['ball', 'substitution', 'ball'],
            expectedPlayer: 'Sub',
        },
        {
            name: 'Strike, Sub, Ball, Undo, Undo -> Strike',
            actions: ['strike', 'sub', 'ball', 'undo', 'undo'],
            expectedSeq: ['strike'],
            expectedPlayer: 'Starter',
        },
        {
            name: 'Sub, Undo, Strike -> Strike',
            actions: ['sub', 'undo', 'strike'],
            expectedSeq: ['strike'],
            expectedPlayer: 'Starter',
        },
        {
            name: 'Ball, Sub, Undo, Sub (Same), Strike -> Ball, Sub, Strike',
            actions: ['ball', 'sub', 'undo', 'sub', 'strike'],
            expectedSeq: ['ball', 'substitution', 'strike'],
            expectedPlayer: 'Sub',
        },
        {
            name: 'Foul, Foul, Sub, Undo -> Foul, Foul',
            actions: ['foul', 'foul', 'sub', 'undo'],
            expectedSeq: ['foul', 'foul'],
            expectedPlayer: 'Starter',
        },
        {
            name: 'Ball, Ball, Ball, Sub, Undo, Ball -> Ball...Ball (Walk)',
            actions: ['ball', 'ball', 'ball', 'sub', 'undo', 'ball'],
            expectedSeq: ['ball', 'ball', 'ball', 'ball'],
            expectedPlayer: 'Starter',
            expectedOutcome: 'BB',
        },
        {
            name: 'Strike, Strike, Sub, Undo, Strike -> Strike...Strike (K)',
            actions: ['strike', 'strike', 'sub', 'undo', 'strike'],
            expectedSeq: ['strike', 'strike', 'strike'],
            expectedPlayer: 'Starter',
            expectedOutcome: 'K',
        },
        {
            name: 'Sub, Undo, Undo -> Empty',
            actions: ['sub', 'undo', 'undo'], // Second undo does nothing if log is empty? Or undoes prev?
            // If log has [GAME_START], undoing Sub makes log effective [GAME_START].
            // Another undo tries to undo GAME_START? We probably shouldn't undo game start here or app prevents it.
            // Let's assume undo on empty sequence is safe/noop for this test scope.
            expectedSeq: [],
            expectedPlayer: 'Starter',
        },
        {
            name: 'Complex: Ball, Sub, Strike, Undo, Ball, Sub, Strike',
            actions: ['ball', 'sub', 'strike', 'undo', 'ball', 'sub', 'strike'],
            expectedSeq: ['ball', 'substitution', 'ball', 'substitution', 'strike'],
            expectedPlayer: 'Sub', // The last sub was applied
        },
        {
            name: 'Ball, Sub, Undo, Strike, Sub -> Ball, Strike, Sub',
            actions: ['ball', 'sub', 'undo', 'strike', 'sub'],
            expectedSeq: ['ball', 'strike', 'substitution'],
            expectedPlayer: 'Sub',
        },
        {
            name: 'Multiple Subs: Sub, Sub, Undo, Undo -> Starter',
            actions: ['sub', 'sub', 'undo', 'undo'],
            expectedSeq: [],
            expectedPlayer: 'Starter',
        },
    ];

    scenarios.forEach(scenario => {
        test(scenario.name, async() => {
            app.openCSO(0, 1, 'col-1-0');

            for (const action of scenario.actions) {
                if (action === 'ball' || action === 'strike' || action === 'foul') {
                    await app.recordPitch(action);
                } else if (action === 'sub') {
                    app.openSubstitutionModal('away', 0);
                    // Cycle mock names if needed for multiple subs, but here we just reuse 'Sub'
                    // For complex cases involving distinct subs, we might need more logic,
                    // but for checking structure, 'Sub' is fine.
                    // If we want to verify distinct subs (scenario 15), we need dynamic naming.
                    const subName = app.state.activeGame.roster['away'][0].current.name === 'Sub' ? 'Sub2' : 'Sub';
                    document.getElementById('sub-incoming-num').value = '99';
                    document.getElementById('sub-incoming-name').value = subName;
                    await app.handleSubstitution();
                } else if (action === 'undo') {
                    await app.undoPitch();
                }
            }

            const seq = app.state.activeData.pitchSequence;
            expect(seq.map(s => s.type)).toEqual(scenario.expectedSeq);

            // Check Player
            // If expectedPlayer is 'Starter', verify name is 'Starter'
            // If 'Sub', verify name starts with 'Sub'
            const currentPlayer = app.state.activeGame.roster['away'][0].current.name;
            if (scenario.expectedPlayer === 'Starter') {
                expect(currentPlayer).toBe('Starter');
            } else {
                expect(currentPlayer).toMatch(/Sub/);
            }

            if (scenario.expectedOutcome) {
                expect(app.state.activeData.outcome).toBe(scenario.expectedOutcome);
            }
        });
    });
});
