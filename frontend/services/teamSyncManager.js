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
 * Manages Team data synchronization with the remote server.
 */
export class TeamSyncManager {
    constructor() {
    }

    /**
     * Fetches the list of teams from the remote server.
     * @param {object} options - Pagination and filter options.
     * @param {number} [options.limit=50]
     * @param {number} [options.offset=0]
     * @param {string} [options.sortBy='name']
     * @param {string} [options.order='asc']
     * @param {string} [options.query='']
     * @returns {Promise<object>} A promise that resolves to { data: [], meta: {} }.
     */
    async fetchTeamList({ limit = 50, offset = 0, sortBy = 'name', order = 'asc', query = '' } = {}) {
        try {
            const params = new URLSearchParams({
                limit,
                offset,
                sortBy,
                order,
                q: query,
            });

            const response = await fetch(`/api/list-teams?${params.toString()}`, {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json',
                },
            });

            if (!response.ok) {
                if (response.status === 403 || response.status === 401) {
                    console.warn('TeamSyncManager: Not authenticated to fetch teams.');
                    return { data: [], meta: { total: 0 } };
                }
                throw new Error(`Server returned ${response.status}`);
            }

            const result = await response.json();
            if (Array.isArray(result)) {
                return { data: result, meta: { total: result.length } };
            }
            return result;
        } catch (error) {
            console.error('TeamSyncManager: Error fetching remote team list:', error);
            return { data: [], meta: { total: 0 } };
        }
    }

    /**
     * Checks if any of the provided team IDs have been deleted on the server.
     * @param {Array<string>} teamIds
     * @returns {Promise<Array<string>>} The list of deleted team IDs.
     */
    async checkTeamDeletions(teamIds) {
        try {
            const response = await fetch('/api/check-deletions', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ teamIds: teamIds }),
            });
            if (!response.ok) {
                return [];
            }
            const result = await response.json();
            return result.deletedTeamIds || [];
        } catch (error) {
            console.warn('TeamSyncManager: Error checking team deletions:', error);
            return [];
        }
    }

    /**
     * Saves a team to the remote server.
     * @param {object} team
     * @returns {Promise<boolean>} True if successful, false otherwise.
     */
    async saveTeam(team) {
        try {
            const response = await fetch('/api/save-team', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(team),
            });

            if (!response.ok) {
                throw new Error(`Server returned ${response.status}`);
            }
            return true;
        } catch (error) {
            console.error('TeamSyncManager: Error saving team:', error);
            return false;
        }
    }

    /**
     * Deletes a team from the remote server.
     * @param {string} teamId
     * @returns {Promise<boolean>} True if successful, false otherwise.
     */
    async deleteTeam(teamId) {
        try {
            const response = await fetch('/api/delete-team', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ id: teamId }),
            });

            if (!response.ok) {
                throw new Error(`Server returned ${response.status}`);
            }
            return true;
        } catch (error) {
            console.error('TeamSyncManager: Error deleting team:', error);
            return false;
        }
    }
}
