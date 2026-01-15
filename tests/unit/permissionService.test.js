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

import { PermissionService } from '../../frontend/services/PermissionService.js';

describe('PermissionService', () => {
    let service;
    const localId = 'local-user-123';
    const userEmail = 'user@example.com';
    const user = { email: userEmail };

    beforeEach(() => {
        service = new PermissionService();
    });

    describe('canWriteTeam', () => {
        test('should allow owner (localId)', () => {
            const team = { ownerId: localId };
            expect(service.canWriteTeam(null, localId, team)).toBe(true);
        });

        test('should allow owner (userId)', () => {
            const team = { ownerId: userEmail };
            expect(service.canWriteTeam(userEmail, localId, team)).toBe(true);
        });

        test('should allow admins', () => {
            const team = { ownerId: 'someone-else', roles: { admins: [userEmail] } };
            expect(service.canWriteTeam(userEmail, localId, team)).toBe(true);
        });

        test('should allow scorekeepers', () => {
            const team = { ownerId: 'someone-else', roles: { scorekeepers: [userEmail] } };
            expect(service.canWriteTeam(userEmail, localId, team)).toBe(true);
        });

        test('should deny spectators', () => {
            const team = { ownerId: 'someone-else', roles: { spectators: [userEmail] } };
            expect(service.canWriteTeam(userEmail, localId, team)).toBe(false);
        });

        test('should deny unauthenticated users if not local owner', () => {
            const team = { ownerId: 'someone-else' };
            expect(service.canWriteTeam(null, localId, team)).toBe(false);
        });
    });

    describe('hasTeamReadAccess', () => {
        test('should allow local owner', () => {
            const team = { ownerId: localId };
            expect(service.hasTeamReadAccess(null, localId, team)).toBe(true);
        });

        test('should allow authenticated owner', () => {
            const team = { ownerId: userEmail };
            expect(service.hasTeamReadAccess(user, localId, team)).toBe(true);
        });

        test('should allow spectators', () => {
            const team = { ownerId: 'someone-else', roles: { spectators: [userEmail] } };
            expect(service.hasTeamReadAccess(user, localId, team)).toBe(true);
        });

        test('should deny others', () => {
            const team = { ownerId: 'someone-else', roles: { spectators: [] } };
            expect(service.hasTeamReadAccess(user, localId, team)).toBe(false);
        });
    });

    describe('hasReadAccess', () => {
        test('should allow owner', () => {
            const game = { ownerId: userEmail };
            expect(service.hasReadAccess(game, [], user, localId)).toBe(true);
        });

        test('should allow local owner', () => {
            const game = { ownerId: localId };
            expect(service.hasReadAccess(game, [], null, localId)).toBe(true);
        });

        test('should allow demo games', () => {
            const game = { id: 'demo-123', ownerId: 'someone-else' };
            expect(service.hasReadAccess(game, [], null, localId)).toBe(true);
        });

        test('should allow public games', () => {
            const game = { ownerId: 'someone-else', permissions: { public: 'read' } };
            expect(service.hasReadAccess(game, [], null, localId)).toBe(true);
        });

        test('should allow direct user access', () => {
            const game = { ownerId: 'someone-else', permissions: { users: { [userEmail]: 'read' } } };
            expect(service.hasReadAccess(game, [], user, localId)).toBe(true);
        });

        test('should allow via team inheritance', () => {
            const game = { ownerId: 'someone-else', awayTeamId: 't1' };
            const teams = [{ id: 't1', ownerId: userEmail }];
            expect(service.hasReadAccess(game, teams, user, localId)).toBe(true);
        });
    });

    describe('isReadOnly', () => {
        test('should be false for owner', () => {
            const game = { ownerId: userEmail };
            expect(service.isReadOnly(game, user, [], localId)).toBe(false);
        });

        test('should be false for direct write access', () => {
            const game = { ownerId: 'someone-else', permissions: { users: { [userEmail]: 'write' } } };
            expect(service.isReadOnly(game, user, [], localId)).toBe(false);
        });

        test('should be true for public read games', () => {
            const game = { ownerId: 'someone-else', permissions: { public: 'read' } };
            expect(service.isReadOnly(game, user, [], localId)).toBe(true);
        });

        test('should be false for demo games', () => {
            const game = { id: 'demo-123', ownerId: 'someone-else' };
            expect(service.isReadOnly(game, null, [], localId)).toBe(false);
        });
    });
});
