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

const mockHtml = `
<div id="dashboard-view"></div>
<div id="scoresheet-view"></div>
<div id="manual-view"></div>
<div id="teams-view" class="hidden"></div>
<div id="game-list"></div>
<div id="new-game-modal"><form id="new-game-form"></form></div>
<div id="edit-game-modal"><form id="edit-game-form"></form></div>
<div id="cso-modal"></div>
<div id="substitution-modal"></div>
<div id="player-context-menu"></div>
<div id="column-context-menu"></div>
<div id="cso-long-press-submenu"><div id="submenu-content"></div></div>
<div id="runner-menu-options"></div>
<div id="cso-runner-menu"></div>
<div id="cso-runner-action-view"></div>
<div id="runner-action-list"></div>
<div id="cso-bip-view"></div>
<div id="cso-runner-advance-view"></div>
<div id="runner-advance-list"></div>

<div id="sidebar-backdrop"></div>
<div id="app-sidebar">
  <div id="sidebar-auth"></div>
  <button id="sidebar-btn-dashboard"></button>
  <button id="sidebar-btn-teams"></button>
  <div id="sidebar-game-actions" class="hidden">
    <button id="sidebar-btn-view-grid"></button>
    <button id="sidebar-btn-view-feed"></button>
    <button id="sidebar-btn-add-inning"></button>
    <button id="sidebar-btn-end-game"></button>
  </div>
  <div id="sidebar-export-actions" class="hidden">
    <button id="sidebar-btn-export-pdf"></button>
  </div>
</div>
<button id="btn-menu-dashboard"></button>
<button id="btn-menu-scoresheet"></button>

<button id="btn-new-game"></button>
<button id="btn-cancel-new-game"></button>
<button id="btn-start-new-game"></button>
<button id="btn-back-dashboard"></button>
<button id="btn-undo"></button>
<button id="btn-redo"></button>
<button id="btn-add-inning"></button>
<div id="tab-away"></div>
<div id="tab-home"></div>
<button id="btn-close-cso"></button>
<button id="btn-change-pitcher"></button>
<button id="btn-undo-pitch"></button>
<button id="btn-show-bip"></button>
<button id="btn-clear-all"></button>
<button id="btn-toggle-action"></button>
<div id="game-status-indicator"></div>
        <button id="btn-ball"></button>
<button id="btn-strike"></button>
<button id="btn-out"></button>
<button id="btn-foul"></button>
<button id="btn-cancel-bip"></button>
<button id="btn-save-bip"></button>
<button id="btn-backspace"></button>
<button id="btn-res"></button>
<button id="btn-base"></button>
<button id="btn-type"></button>
<button id="btn-finish-turn"></button>
<button id="btn-close-runner-menu"></button>
<button id="btn-close-long-press-menu"></button>
<button id="btn-runner-actions"></button>
<button id="btn-close-runner-action"></button>
<button id="btn-cancel-sub"></button>
<button id="btn-confirm-sub"></button>

<button id="btn-conflict-overwrite"></button>
<button id="btn-conflict-fork"></button>
<button id="btn-cancel-lineup"></button>
<button id="btn-save-lineup"></button>
<button id="btn-add-starter-row"></button>
<button id="btn-add-sub-row"></button>

<button data-auto-advance="HBP" id="btn-hbp">HBP</button>
<button data-auto-advance="CI" id="btn-ci">CI</button>
<button data-auto-advance="IBB" id="btn-ibb">IBB</button>

<button id="btn-menu-add-col"></button>
<button id="btn-menu-remove-col"></button>

<input id="team-away-input">
<input id="team-home-input">
<input id="game-event-input">
<input id="game-location-input">
<input id="game-date-input">
<input id="sub-incoming-num">
<input id="sub-incoming-name">
<input id="sub-incoming-pos">
<span id="cso-title"></span>
<span id="cso-subtitle"></span>
<span id="cso-pitcher-num"></span>
<div id="control-balls"></div>
<div id="control-strikes"></div>
<div id="control-outs"></div>
<div id="pitch-sequence-container"></div>
<div id="action-area-pitch"></div>
<div id="action-area-recorded"></div>
<div id="zoom-outcome-text"></div>
<div class="cso-zoom-container"></div>
<div id="scoresheet-grid"></div>
<div id="sb-name-away"></div>
<div id="sb-name-home"></div>
<div id="sb-r-away"></div>
<div id="sb-h-away"></div>
<div id="sb-e-away"></div>
<div id="sb-r-home"></div>
<div id="sb-h-home"></div>
<div id="sb-e-home"></div>
<div id="sb-innings-away"></div>
<div id="sb-innings-home"></div>
<div id="sb-header-innings"></div>
<form id="sub-form"></form>
`;

const mockHistory = {
    push: jest.fn(),
    undo: jest.fn(),
    redo: jest.fn(),
    undoStack: [],
    redoStack: [],
};

describe('Interactive Components', () => {
    let app;
    let mockDB; // Defined here

    beforeEach(() => {
        document.body.innerHTML = mockHtml;

        mockDB = {
            open: jest.fn().mockResolvedValue(true),
            saveGame: jest.fn().mockResolvedValue(true),
            loadGame: jest.fn().mockResolvedValue(null),
            getAllGames: jest.fn().mockResolvedValue([]),
            getAllTeams: jest.fn().mockResolvedValue([]),
            getLocalRevisions: jest.fn().mockResolvedValue(new Map()),
        };

        // Spy on prototype methods to capture bound events
        jest.spyOn(AppController.prototype, 'closeNewGameModal').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'addInning').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'switchTeam').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'changePitcher').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'undoPitch').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'clearAllData').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'recordPitch').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'recordAutoAdvance').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'addColFromMenu').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'removeColFromMenu').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'openRunnerActionView').mockImplementation(() => {

        });
        jest.spyOn(AppController.prototype, 'openEditLineupModal').mockImplementation(() => {

        });


        app = new AppController(mockDB, mockHistory);

        app.state.activeGame = {
            columns: [],
            pitchers: { away: '', home: '' },
            roster: { away: [], home: [] },
            events: {},
            away: 'Away',
            home: 'Home',
        };

        console.log = jest.fn();
        console.warn = jest.fn();
        console.error = jest.fn();
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('btn-cancel-new-game should call closeNewGameModal', () => {
        document.getElementById('btn-cancel-new-game').click();
        expect(AppController.prototype.closeNewGameModal).toHaveBeenCalled();
    });

    test('sidebar-btn-dashboard should clear hash', () => {
        window.location.hash = 'game/123';
        document.getElementById('sidebar-btn-dashboard').click();
        expect(window.location.hash).toBe('');
    });

    test('sidebar-btn-add-inning should call addInning', () => {
        document.getElementById('sidebar-btn-add-inning').click();
        expect(AppController.prototype.addInning).toHaveBeenCalled();
    });

    test('tab-away should call switchTeam("away")', () => {
        document.getElementById('tab-away').click();
        expect(AppController.prototype.switchTeam).toHaveBeenCalledWith('away');
    });

    test('tab-away contextmenu should call openEditLineupModal', () => {
        const evt = new MouseEvent('contextmenu', { bubbles: true, cancelable: true });
        document.getElementById('tab-away').dispatchEvent(evt);
        expect(AppController.prototype.openEditLineupModal).toHaveBeenCalledWith('away');
    });

    test('btn-change-pitcher should call changePitcher', () => {
        document.getElementById('btn-change-pitcher').click();
        expect(AppController.prototype.changePitcher).toHaveBeenCalled();
    });

    test('btn-undo-pitch should call undoPitch', () => {
        document.getElementById('btn-undo-pitch').click();
        expect(AppController.prototype.undoPitch).toHaveBeenCalled();
    });

    test('btn-clear-all should call clearAllData', () => {
        document.getElementById('btn-clear-all').click();
        expect(AppController.prototype.clearAllData).toHaveBeenCalled();
    });

    test('btn-toggle-action should set isEditing and renderCSO', () => {
        jest.spyOn(app, 'renderCSO').mockImplementation(() => {
        });
        document.getElementById('btn-toggle-action').click();
        expect(app.state.isEditing).toBe(true);
        expect(app.renderCSO).toHaveBeenCalled();
    });

    test('btn-foul should call recordPitch("foul")', () => {
        document.getElementById('btn-foul').click();
        expect(AppController.prototype.recordPitch).toHaveBeenCalledWith('foul');
    });

    test('data-auto-advance buttons should call recordAutoAdvance', () => {
        document.getElementById('btn-hbp').click();
        expect(AppController.prototype.recordAutoAdvance).toHaveBeenCalledWith('HBP');
    });

    test('btn-menu-add-col should call addColFromMenu', () => {
        document.getElementById('btn-menu-add-col').click();
        expect(AppController.prototype.addColFromMenu).toHaveBeenCalled();
    });

    test('btn-menu-remove-col should call removeColFromMenu', () => {
        document.getElementById('btn-menu-remove-col').click();
        expect(AppController.prototype.removeColFromMenu).toHaveBeenCalled();
    });

    test('btn-runner-actions should call openRunnerActionView', () => {
        document.getElementById('btn-runner-actions').click();
        expect(AppController.prototype.openRunnerActionView).toHaveBeenCalled();
    });
});
