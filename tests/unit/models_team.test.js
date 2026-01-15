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

import { Team } from '../../frontend/models/Team.js';
import { Player } from '../../frontend/models/Player.js';
import { CurrentSchemaVersion } from '../../frontend/constants.js';

describe('Team Model', () => {
    test('should initialize with default values', () => {
        const team = new Team();
        expect(team.id).toBeDefined();
        expect(team.schemaVersion).toBe(CurrentSchemaVersion);
        expect(team.name).toBe('');
        expect(team.shortName).toBe('');
        expect(team.color).toBe('#2563eb');
        expect(team.ownerId).toBe('');
        expect(team.roles).toEqual({ admins: [], scorekeepers: [], spectators: [] });
        expect(team.roster).toEqual([]);
        expect(team.updatedAt).toBeLessThanOrEqual(Date.now());
    });

    test('should initialize with provided data', () => {
        const now = Date.now();
        const data = {
            id: 'test-team-id',
            name: 'Tigers',
            shortName: 'TIG',
            color: '#000000',
            ownerId: 'owner@example.com',
            updatedAt: now,
            roster: [{ id: 'p1', name: 'Player 1' }],
        };
        const team = new Team(data);

        expect(team.id).toBe('test-team-id');
        expect(team.name).toBe('Tigers');
        expect(team.shortName).toBe('TIG');
        expect(team.color).toBe('#000000');
        expect(team.ownerId).toBe('owner@example.com');
        expect(team.updatedAt).toBe(now);
        expect(team.roster.length).toBe(1);
        expect(team.roster[0]).toBeInstanceOf(Player);
        expect(team.roster[0].name).toBe('Player 1');
    });

    test('toJSON should return a plain object', () => {
        const team = new Team({
            name: 'Bears',
            roster: [new Player({ name: 'Bob' })],
        });

        const json = team.toJSON();
        expect(json.name).toBe('Bears');
        expect(json.roster).toHaveLength(1);
        expect(json.roster[0].name).toBe('Bob');
        // Ensure it's not a class instance
        expect(json.roster[0] instanceof Player).toBe(false);
    });
});
