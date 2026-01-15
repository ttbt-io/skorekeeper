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

import { LineupManager } from '../../frontend/game/lineupManager.js';
import { ActionTypes } from '../../frontend/reducer.js';

describe('LineupManager', () => {
    let lineupManager;
    let mockDispatch;
    let mockDB;

    beforeEach(() => {
        mockDispatch = jest.fn();
        mockDB = {
            getAllTeams: jest.fn().mockResolvedValue([]),
        };
        lineupManager = new LineupManager({ dispatch: mockDispatch, db: mockDB, validate: jest.fn() });
    });

    describe('movePlayer', () => {
        beforeEach(() => {
            lineupManager.lineupState = {
                starters: [{ id: 's1' }, { id: 's2' }, { id: 's3' }],
                subs: [{ id: 'b1' }, { id: 'b2' }],
            };
        });

        test('should move player within starters', () => {
            lineupManager.movePlayer(0, 'starter', 2, 'starter');
            expect(lineupManager.lineupState.starters.map(p => p.id)).toEqual(['s2', 's3', 's1']);
        });

        test('should move player within subs', () => {
            lineupManager.movePlayer(1, 'sub', 0, 'sub');
            expect(lineupManager.lineupState.subs.map(p => p.id)).toEqual(['b2', 'b1']);
        });

        test('should move player from sub to starter', () => {
            lineupManager.movePlayer(0, 'sub', 1, 'starter');
            expect(lineupManager.lineupState.subs.map(p => p.id)).toEqual(['b2']);
            expect(lineupManager.lineupState.starters.map(p => p.id)).toEqual(['s1', 'b1', 's2', 's3']);
        });
    });

    describe('add/remove player', () => {
        beforeEach(() => {
            lineupManager.lineupState = { starters: [], subs: [] };
        });

        test('should add player to group', () => {
            lineupManager.addPlayerToGroup('starter');
            expect(lineupManager.lineupState.starters.length).toBe(1);
            expect(lineupManager.lineupState.starters[0].id).toBeDefined();
        });

        test('should remove player from group', () => {
            lineupManager.lineupState.subs = [{ id: 'b1' }];
            lineupManager.removePlayerFromGroup(0, 'sub');
            expect(lineupManager.lineupState.subs.length).toBe(0);
        });
    });

    describe('getInitialLineupState', () => {
        test('should initialize from game roster', async() => {
            const game = {
                roster: {
                    home: [{ starter: { id: 'p1', name: 'P1' } }],
                },
                subs: { home: [] },
                homeTeamId: 't1',
            };

            // Mock DB to return team data
            mockDB.getAllTeams.mockResolvedValue([
                {
                    id: 't1',
                    roster: [{ id: 'p1', name: 'P1' }, { id: 'p2', name: 'P2' }], // p2 is missing in game
                },
            ]);

            const state = await lineupManager.getInitialLineupState(game, 'home');

            expect(state.starters[0].name).toBe('P1');
            // Check if missing team player (p2) was added to subs
            expect(state.subs.find(s => s.name === 'P2')).toBeDefined();
        });
    });

    describe('save', () => {
        test('should dispatch LINEUP_UPDATE', async() => {
            const game = {
                roster: { home: [] },
            };
            lineupManager.lineupState = {
                starters: [{ id: 'p1', name: 'P1', number: '1' }],
                subs: [{ id: 'b1', name: 'B1', number: '2' }],
            };

            await lineupManager.save(game, 'home', 'New Name');

            expect(mockDispatch).toHaveBeenCalledWith({
                type: ActionTypes.LINEUP_UPDATE,
                payload: expect.objectContaining({
                    team: 'home',
                    teamName: 'New Name',
                    roster: expect.arrayContaining([
                        expect.objectContaining({ starter: expect.objectContaining({ name: 'P1' }) }),
                    ]),
                    subs: expect.arrayContaining([
                        expect.objectContaining({ name: 'B1' }),
                    ]),
                }),
            });
        });
    });

    describe('UI Rendering & Interaction', () => {
        let container;
        beforeEach(() => {
            container = document.createElement('div');
            document.body.appendChild(container);
        });

        afterEach(() => {
            document.body.innerHTML = '';
        });

        test('renderRow() creates DOM elements', () => {
            const row = lineupManager.renderRow('starter', 0, { name: 'Player', number: '1', pos: 'P' }, {});
            expect(row.querySelector('input[name="name"]').value).toBe('Player');
            expect(row.querySelector('input[name="pos"]')).not.toBeNull();
        });

        test('renderRow() creates sub row (no pos)', () => {
            const row = lineupManager.renderRow('sub', 0, { name: 'Sub', number: '99' }, {});
            expect(row.querySelector('input[name="pos"]')).toBeNull();
        });

        test('handleDragStart() sets draggedItem', () => {
            const row = document.createElement('div');
            row.dataset.idx = '0';
            row.dataset.type = 'starter';

            const event = {
                target: row,
                dataTransfer: { effectAllowed: '' },
                preventDefault: jest.fn(),
            };
            // Need closest to work
            const inner = document.createElement('div');
            row.appendChild(inner);
            event.target = inner;

            lineupManager.handleDragStart(event);
            expect(lineupManager.draggedItem).toEqual(expect.objectContaining({ index: 0, type: 'starter' }));
            expect(row.classList.contains('dragging')).toBe(true);
        });

        test('handleDrop() moves player', () => {
            lineupManager.lineupState = {
                starters: [{ id: 's1' }, { id: 's2' }],
                subs: [],
            };
            lineupManager.draggedItem = { index: 0, type: 'starter' }; // Drag s1

            const targetRow = document.createElement('div');
            targetRow.dataset.idx = '1';
            targetRow.dataset.type = 'starter'; // Drop on s2

            const event = {
                target: targetRow,
                dataTransfer: {},
                preventDefault: jest.fn(),
            };
            // Mock closest
            const inner = document.createElement('div');
            targetRow.appendChild(inner);
            event.target = inner;

            const onUpdate = jest.fn();
            lineupManager.handleDrop(event, onUpdate);

            expect(lineupManager.lineupState.starters.map(s => s.id)).toEqual(['s2', 's1']);
            expect(onUpdate).toHaveBeenCalled();
        });

        test('scrape() updates state from DOM', () => {
            const startersDiv = document.createElement('div');
            const row = document.createElement('div');
            row.innerHTML = `
                <input name="number" value="10">
                <input name="name" value="New Name">
                <input name="pos" value="C">
            `;
            row.dataset.pid = 'p1';
            startersDiv.appendChild(row);

            const subsDiv = document.createElement('div');

            lineupManager.scrape({ starters: startersDiv, subs: subsDiv });

            expect(lineupManager.lineupState.starters[0]).toEqual({
                number: '10', name: 'New Name', pos: 'C', id: 'p1',
            });
        });
    });
});
