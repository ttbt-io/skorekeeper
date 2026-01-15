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

/**
 * Service for managing user permissions and access control.
 * Centralizes logic for checking read/write access to games and teams.
 */
export class PermissionService {
    /**
     * Checks if the current user has write access to a team.
     * @param {string|null} userId - The user's email ID.
     * @param {string} localId - The local browser identity ID.
     * @param {object} team - The team object.
     * @returns {boolean}
     */
    canWriteTeam(userId, localId, team) {
        if (!team) {
            return false;
        }
        if (team.ownerId === localId) {
            return true;
        }
        if (!userId) {
            return false;
        }
        if (team.ownerId === userId) {
            return true;
        }
        if (team.roles) {
            if (team.roles.admins && team.roles.admins.includes(userId)) {
                return true;
            }
            if (team.roles.scorekeepers && team.roles.scorekeepers.includes(userId)) {
                return true;
            }
        }
        return false;
    }

    /**
     * Checks if the current user has read access to a team.
     * @param {object|null} user - The current user object.
     * @param {string} localId - The local browser identity ID.
     * @param {object} team - The team object.
     * @returns {boolean}
     */
    hasTeamReadAccess(user, localId, team) {
        if (!team) {
            return false;
        }
        const userId = user ? user.email : null;

        if (team.ownerId === localId) {
            return true;
        }
        if (userId && team.ownerId === userId) {
            return true;
        }

        if (!userId) {
            return false;
        }

        if (team.roles) {
            if (team.roles.admins && team.roles.admins.includes(userId)) {
                return true;
            }
            if (team.roles.scorekeepers && team.roles.scorekeepers.includes(userId)) {
                return true;
            }
            if (team.roles.spectators && team.roles.spectators.includes(userId)) {
                return true;
            }
        }
        return false;
    }

    /**
     * Determines if the current user has read access to a game.
     * @param {object} game - The game object.
     * @param {Array<object>} allTeams - All teams the user has access to.
     * @param {object|null} user - The current user object.
     * @param {string} localId - The local browser identity ID.
     * @returns {boolean}
     */
    hasReadAccess(game, allTeams, user, localId) {
        if (!game) {
            return false;
        }
        const userId = user ? user.email : null;

        // 1. Owner & Local & Demo
        if (userId && game.ownerId === userId) {
            return true;
        }
        if (game.ownerId === localId) {
            return true;
        }
        if (game.id && game.id.startsWith('demo-')) {
            return true;
        }

        // 2. Public
        if (game.permissions && game.permissions.public === 'read') {
            return true;
        }

        if (!userId) {
            return false;
        }

        // 3. Direct
        if (game.permissions && game.permissions.users && game.permissions.users[userId]) {
            return true;
        }

        // 4. Team Inheritance
        const checkTeam = (teamId) => {
            if (!teamId) {
                return false;
            }
            const team = allTeams.find(t => t.id === teamId);
            return this.hasTeamReadAccess(user, localId, team);
        };

        if (checkTeam(game.awayTeamId) || checkTeam(game.homeTeamId)) {
            return true;
        }

        return false;
    }

    /**
     * Determines if the game should be read-only for the current user.
     * @param {object} game - The game object.
     * @param {Array<object>} allTeams - All teams the user has access to.
     * @param {object|null} user - The current user object.
     * @param {string} localId - The local browser identity ID.
     * @returns {boolean}
     */
    isReadOnly(game, user, allTeams, localId) {
        if (!game) {
            return false;
        }
        const userId = user ? user.email : null;

        // 1. Owner has write access
        if (userId && game.ownerId === userId) {
            return false;
        }
        // 1b. Local Owner (Offline/Anonymous creation)
        if (game.ownerId === localId) {
            return false;
        }
        // 1c. Demo Games are always writeable by the local user
        if (game.id && game.id.startsWith('demo-')) {
            return false;
        }

        // 2. Direct permissions
        if (userId && game.permissions && game.permissions.users) {
            if (game.permissions.users[userId] === 'write') {
                return false;
            }
        }

        // 3. Team Inheritance
        const checkTeam = (teamId) => {
            if (!teamId) {
                return false;
            }
            const team = allTeams.find(t => t.id === teamId);
            return this.canWriteTeam(userId, localId, team);
        };

        if (checkTeam(game.awayTeamId) || checkTeam(game.homeTeamId)) {
            return false;
        }

        // Default to Read-Only if not explicitly granted write access
        return true;
    }
}