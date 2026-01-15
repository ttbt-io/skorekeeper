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

import { TeamsRenderer } from '../../frontend/renderers/teamsRenderer.js';

describe('TeamsRenderer', () => {
    let listContainer;
    let membersContainer;
    let renderer;
    let mockCallbacks;

    beforeEach(() => {
        listContainer = document.createElement('div');
        membersContainer = document.createElement('div');

        // Mock DOM elements that might be queried
        document.body.innerHTML = `
            <button id="btn-create-team"></button>
        `;

        mockCallbacks = {
            onEdit: jest.fn(),
            onDelete: jest.fn(),
            onSync: jest.fn(),
            onRemoveMember: jest.fn(),
            canManage: jest.fn(() => true),
        };

        renderer = new TeamsRenderer({
            listContainer,
            membersContainer,
            callbacks: mockCallbacks,
        });
    });

    afterEach(() => {
        document.body.innerHTML = '';
        jest.clearAllMocks();
    });

    test('renderTeamsList() shows empty message', () => {
        renderer.renderTeamsList([], null);
        expect(listContainer.textContent).toContain('No teams found');
    });

    test('renderTeamsList() renders teams', () => {
        const teams = [
            { id: 't1', name: 'Tigers', roster: [1,2], color: 'red' },
            { id: 't2', name: 'Bears', roster: [], color: 'blue' },
        ];
        renderer.renderTeamsList(teams, null);

        expect(listContainer.querySelectorAll('.bg-white').length).toBe(2); // 2 cards
        expect(listContainer.textContent).toContain('Tigers');
        expect(listContainer.textContent).toContain('Bears');
    });

    test('renderTeamsList() handles quota', () => {
        const teams = [{ id: 't1', ownerId: 'me' }];
        const user = { email: 'me', maxTeams: 1 };

        renderer.renderTeamsList(teams, user);
        const btn = document.getElementById('btn-create-team');
        expect(btn.disabled).toBe(true);
        expect(btn.title).toContain('Quota Reached');
    });

    test('renderTeamsList() handles admin actions', () => {
        const teams = [{ id: 't1', ownerId: 'me', name: 'My Team' }];
        const user = { email: 'me' };

        renderer.renderTeamsList(teams, user);

        const deleteBtn = listContainer.querySelector('button[title="Delete Team"]');
        expect(deleteBtn).not.toBeNull();

        deleteBtn.click();
        expect(mockCallbacks.onDelete).toHaveBeenCalledWith('t1');
    });

    test('renderTeamsList() handles sync status', () => {
        const teams = [
            { id: 't1', syncStatus: 'synced' },
            { id: 't2', syncStatus: 'local_only' },
            { id: 't3', syncStatus: 'syncing' },
            { id: 't4', syncStatus: 'error' },
        ];
        const user = { email: 'me' };

        renderer.renderTeamsList(teams, user);

        // Sync button click - query within container
        const btn = listContainer.querySelector('#team-sync-btn-t2');
        btn.click();
        expect(mockCallbacks.onSync).toHaveBeenCalledWith('t2');

        // Syncing state
        const syncingBtn = listContainer.querySelector('#team-sync-btn-t3');
        expect(syncingBtn.disabled).toBe(true);
        expect(syncingBtn.className).toContain('animate-pulse');
    });

    test('renderTeamMembers() renders roles', () => {
        const team = {
            id: 't1',
            ownerId: 'owner@example.com',
            roles: {
                admins: ['admin@example.com'],
                scorekeepers: ['sk@example.com'],
                spectators: ['spec@example.com'],
            },
        };
        const user = { email: 'owner@example.com' };

        renderer.renderTeamMembers(team, user);

        expect(membersContainer.textContent).toContain('Admins');
        expect(membersContainer.textContent).toContain('Scorekeepers');
        expect(membersContainer.textContent).toContain('Spectators');

        expect(membersContainer.textContent).toContain('owner@example.com');
        expect(membersContainer.textContent).toContain('admin@example.com');
        expect(membersContainer.textContent).toContain('sk@example.com');
        expect(membersContainer.textContent).toContain('spec@example.com');
    });

    test('renderTeamMembers() handles remove member', () => {
        const team = {
            id: 't1',
            ownerId: 'owner@example.com',
            roles: {
                admins: ['admin@example.com'],
            },
        };
        const user = { email: 'owner@example.com' }; // Owner can remove others

        renderer.renderTeamMembers(team, user);

        // owner shouldn't have remove button next to them (logic: email !== team.ownerId)
        // admin@example.com should have one.

        // Find row for admin
        const adminRow = Array.from(membersContainer.querySelectorAll('div.flex')).find(div => div.textContent.includes('admin@example.com'));
        const removeBtn = adminRow.querySelector('button');
        removeBtn.click();

        expect(mockCallbacks.onRemoveMember).toHaveBeenCalledWith('admin@example.com', 'admins');
    });

    test('renderTeamRow() creates inputs', () => {
        const container = document.createElement('div');
        const player = { id: 'p1', number: '42', name: 'Jackie', pos: '2B' };

        renderer.renderTeamRow(container, player);

        const inputs = container.querySelectorAll('input');
        expect(inputs[0].value).toBe('42');
        expect(inputs[1].value).toBe('Jackie');
        expect(inputs[2].value).toBe('2B');

        // Remove button
        const removeBtn = container.querySelector('button');
        removeBtn.click();
        expect(container.children.length).toBe(0); // Row removed
    });

    test('renderTeamRow() handles empty player', () => {
        const container = document.createElement('div');
        renderer.renderTeamRow(container);
        const inputs = container.querySelectorAll('input');
        expect(inputs[0].value).toBe('');
    });
});
