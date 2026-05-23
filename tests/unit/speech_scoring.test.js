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

jest.mock('../../frontend/utils/SpeechManager.js', () => {
    return {
        SpeechManager: jest.fn().mockImplementation(() => {
            return {
                start: jest.fn(),
                stop: jest.fn(),
                isListening: false,
            };
        }),
    };
});

const mockHtml = `
<div id="game-list"></div>
<div id="teams-list"></div>
<div id="team-members-container"></div>
<div id="scoresheet-grid"></div>
<div id="scoresheet-scoreboard"></div>
<div id="narrative-feed"></div>
<div class="cso-zoom-container"></div>
<div id="cso-bip-view"><div class="field-svg-keyboard"><svg></svg></div></div>
<div id="sync-status-container"></div>
<span id="app-version"></span>
<div id="app-sidebar" class="-translate-x-full"></div>
<div id="sidebar-backdrop" class="hidden"></div>
<button id="sidebar-btn-dashboard"></button>
<button id="sidebar-btn-teams"></button>
<button id="sidebar-btn-add-inning"></button>
<button id="sidebar-btn-end-game"></button>
<div id="sidebar-game-actions" class="hidden"></div>
<div id="sidebar-export-actions" class="hidden"></div>
<button id="sidebar-btn-view-grid"></button>
<button id="sidebar-btn-view-feed"></button>
<button id="btn-speech-toggle"></button>
<button id="btn-cso-speech-toggle"></button>
<div id="speech-preview-bar" class="hidden"></div>
<div id="speech-action-chips"></div>
<button id="btn-speech-confirm"></button>
<button id="btn-speech-cancel"></button>
<div id="disambiguation-modal" class="hidden"></div>
<div id="disambiguation-modal-desc"></div>
<div id="disambiguation-options-container"></div>
<button id="btn-disambiguation-cancel"></button>
<div id="cso-modal" class="hidden"></div>
<div id="conflict-resolution-modal" class="hidden"></div>
`;

describe('Speech Scoring & Continuous Listening', () => {
    let app;
    let mockDB;

    beforeEach(() => {
        document.body.textContent = '';
        const parser = new DOMParser();
        const doc = parser.parseFromString(mockHtml, 'text/html');
        while (doc.body.firstChild) {
            document.body.appendChild(doc.body.firstChild);
        }
        localStorage.clear();

        mockDB = {
            open: jest.fn().mockResolvedValue(true),
            saveGame: jest.fn().mockResolvedValue(true),
            loadGame: jest.fn().mockResolvedValue(null),
            getAllGames: jest.fn().mockResolvedValue([]),
            getAllTeams: jest.fn().mockResolvedValue([]),
            getLocalRevisions: jest.fn().mockResolvedValue(new Map()),
        };

        jest.spyOn(AppController.prototype, 'init').mockResolvedValue();

        app = new AppController(mockDB);
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('should initialize speech continuous mode from settings', () => {
        localStorage.setItem('voiceScoringMode', 'continuous');
        expect(localStorage.getItem('voiceScoringMode')).toBe('continuous');
    });

    test('toggleSpeech: start/stop listener in ptt mode', () => {
        localStorage.setItem('voiceScoringMode', 'ptt');
        app.speechManager.isListening = false;

        app.toggleSpeech();
        expect(app.speechListeningIntentional).toBe(true);
        expect(app.speechManager.start).toHaveBeenCalled();

        app.speechManager.isListening = true;
        app.toggleSpeech();
        expect(app.speechListeningIntentional).toBe(false);
        expect(app.speechManager.stop).toHaveBeenCalled();
    });

    test('toggleSpeech: start continuous listening mode loop', () => {
        localStorage.setItem('voiceScoringMode', 'continuous');
        app.speechManager.isListening = false;

        app.toggleSpeech();
        expect(app.speechListeningIntentional).toBe(true);
        expect(app.speechManager.start).toHaveBeenCalled();
    });

    test('handleSpeechError: catch AmbiguityError, pause listening, and open modal', () => {
        const error = {
            name: 'AmbiguityError',
            message: 'Which runner stole?',
            options: [
                { text: 'Player A stole', actions: [{ type: 'STEAL' }] },
                { text: 'Player B stole', actions: [{ type: 'STEAL' }] },
            ],
        };

        app.speechListeningIntentional = true;
        app.speechManager.isListening = true;

        app.handleSpeechError(error);

        expect(app.speechListeningIntentional).toBe(false);
        expect(app.speechManager.stop).toHaveBeenCalled();

        const modal = document.getElementById('disambiguation-modal');
        expect(modal.classList.contains('hidden')).toBe(false);

        const desc = document.getElementById('disambiguation-modal-desc');
        expect(desc.textContent).toBe('Which runner stole?');

        const container = document.getElementById('disambiguation-options-container');
        expect(container.children.length).toBe(2);
    });

    test('disambiguation options click: dispatch actions and resume continuous listening', async() => {
        const options = [
            { text: 'Player A stole', actions: [{ type: 'STEAL_A' }] },
        ];
        jest.spyOn(app, 'dispatch').mockResolvedValue();
        jest.spyOn(app, 'startListeningContinuous').mockImplementation();

        localStorage.setItem('voiceScoringMode', 'continuous');
        app.showDisambiguationModal('Choose option', options, true);

        const container = document.getElementById('disambiguation-options-container');
        const btn = container.querySelector('button');
        expect(btn).not.toBeNull();

        await btn.onclick();

        expect(app.dispatch).toHaveBeenCalledWith({ type: 'STEAL_A' });
        expect(app.startListeningContinuous).toHaveBeenCalled();
        expect(document.getElementById('disambiguation-modal').classList.contains('hidden')).toBe(true);
    });

    test('disambiguation cancel click: close modal and resume listening', () => {
        localStorage.setItem('voiceScoringMode', 'ptt');
        jest.spyOn(app, 'toggleSpeech').mockImplementation();

        app.showDisambiguationModal('Choose option', [], true);
        const cancelBtn = document.getElementById('btn-disambiguation-cancel');

        cancelBtn.onclick();

        expect(app.toggleSpeech).toHaveBeenCalled();
        expect(document.getElementById('disambiguation-modal').classList.contains('hidden')).toBe(true);
    });
});
