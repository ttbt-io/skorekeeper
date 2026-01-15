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

// Minimal mock setup
const mockHtml = `
<div id="game-list"></div>
<div id="teams-list"></div>
<div id="team-members-container"></div>
<div id="scoresheet-grid"></div>
<div id="scoresheet-scoreboard"></div>
<div id="narrative-feed"></div>
<div id="stats-content"></div>
<div id="dashboard-view"></div>
<div id="scoresheet-view"></div>
<div id="manual-view"></div>
<div id="broadcast-view"></div>
<div id="teams-view" class="hidden"></div>
<div id="statistics-view"></div>
<div id="cso-bip-view"><div class="field-svg-keyboard"></div></div>
<div id="cso-modal"></div>
<div id="new-game-modal"></div>
<div id="edit-game-modal"></div>
<div id="substitution-modal"></div>
<div id="player-context-menu"></div>
<div id="column-context-menu"></div>
<div id="runner-menu-options"></div>
<div id="cso-runner-menu"></div>
<div id="runner-action-list"></div>
<div id="cso-runner-advance-view"></div>
<div id="runner-advance-list"></div>
<div id="sidebar-backdrop"></div>
<div id="app-sidebar">
  <div id="sidebar-auth"></div>
</div>
<!-- Buttons required for bindEvents -->
<button id="btn-menu-dashboard"></button>
<button id="btn-menu-teams"></button>
<button id="btn-menu-stats"></button>
<button id="btn-menu-scoresheet"></button>
<button id="sidebar-btn-dashboard"></button>
<button id="sidebar-btn-teams"></button>
<button id="sidebar-btn-stats"></button>
<button id="sidebar-btn-manual"></button>
<button id="sidebar-btn-add-inning"></button>
<button id="sidebar-btn-end-game"></button>
<div id="sidebar-game-actions" class="hidden"></div>
<div id="sidebar-export-actions" class="hidden">
  <button id="sidebar-btn-export-pdf"></button>
</div>
<button id="sidebar-btn-view-grid"></button>
<button id="sidebar-btn-view-feed"></button>
<button id="sidebar-btn-clear-cache"></button>
<button id="btn-new-game"></button>
<button id="btn-cancel-new-game"></button>
<button id="btn-start-new-game"></button>
<button id="btn-back-dashboard"></button>
<button id="btn-undo"></button>
<button id="btn-redo"></button>
<button id="btn-add-inning"></button>
<button id="btn-close-cso"></button>
<button id="btn-change-pitcher"></button>
<button id="btn-undo-pitch"></button>
<button id="btn-show-bip"></button>
<button id="btn-clear-all"></button>
<button id="btn-toggle-action"></button>
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
<button id="btn-runner-actions"></button>
<button id="btn-runner-advance-close"></button>
<button id="btn-close-runner-action"></button>
<button id="btn-cancel-sub"></button>
<button id="btn-confirm-sub"></button>
<button id="btn-conflict-overwrite"></button>
<button id="btn-conflict-force-save"></button>
<button id="btn-conflict-fork"></button>
<button id="btn-cancel-lineup"></button>
<button id="btn-save-lineup"></button>
<button id="btn-add-starter-row"></button>
<button id="btn-add-sub-row"></button>
<button id="btn-hbp" data-auto-advance="HBP"></button>
<button id="btn-ci" data-auto-advance="CI"></button>
<button id="btn-ibb" data-auto-advance="IBB"></button>
<button id="btn-menu-add-col"></button>
<button id="btn-menu-remove-col"></button>
<button id="btn-loc"></button>
<button id="btn-clear-loc"></button>
<button id="btn-save-runner-actions"></button>
<button id="btn-cancel-edit-game"></button>
<button id="btn-save-edit-game"></button>
<button id="btn-manual-back"></button>
<button id="btn-create-team"></button>
<button id="btn-cancel-team"></button>
<button id="btn-save-team"></button>
<button id="btn-add-team-player"></button>
<div id="tab-team-roster"></div>
<div id="tab-team-members"></div>
<button id="btn-add-member"></button>
<button id="btn-close-profile"></button>
<button id="btn-share-game"></button>
<button id="btn-close-share"></button>
<button id="btn-share-add-user"></button>
<button id="btn-copy-share-url"></button>
<button id="btn-copy-broadcast-url"></button>
<div id="tab-away"></div>
<div id="tab-home"></div>
<input id="team-away-input">
<input id="team-home-input">
<form id="new-game-form"></form>
<form id="edit-game-form"></form>
`;

const mockDB = {
    open: jest.fn().mockResolvedValue(true),
    saveGame: jest.fn().mockResolvedValue(true),
    loadGame: jest.fn().mockResolvedValue({ id: 'test-game-id', actionLog: [], ownerId: 'test@example.com' }), // Add ownerId for permissions
    getAllFullGames: jest.fn().mockResolvedValue([]),
    getAllGames: jest.fn().mockResolvedValue([]), // Used in loadDashboard
    getAllTeams: jest.fn().mockResolvedValue([]),
};

describe('Routing Switch', () => {
    let app;

    beforeEach(() => {
        document.body.innerHTML = mockHtml;
        app = new AppController(mockDB);

        // Mock render to avoid DOM errors
        jest.spyOn(app, 'render').mockImplementation(() => {
        });
        // Mock loadDashboard
        jest.spyOn(app, 'loadDashboard').mockImplementation(() => {
        });
        // Mock Auth local ID
        jest.spyOn(app.auth, 'getLocalId').mockReturnValue('test@example.com');

        // Mock active game
        app.state.activeGame = { id: 'test-game-id' };
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('switchScoresheetView("feed") should update hash to #feed/', () => {
        window.location.hash = '#game/test-game-id';
        app.switchScoresheetView('feed');
        expect(window.location.hash).toBe('#feed/test-game-id');
        expect(app.state.scoresheetView).toBe('feed');
    });

    test('switchScoresheetView("grid") should update hash to #game/', () => {
        window.location.hash = '#feed/test-game-id';
        app.switchScoresheetView('grid');
        expect(window.location.hash).toBe('#game/test-game-id');
        expect(app.state.scoresheetView).toBe('grid');
    });
});
