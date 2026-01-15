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
     * @param {Array<string>} [knownIds=null] - Optional list of local team IDs.
     * @returns {Promise<Array<object>>} A promise that resolves to an array of team objects.
     */
    async fetchTeamList(knownIds = null) {
        try {
            let response;
            if (knownIds && knownIds.length > 0) {
                response = await fetch('/api/list-teams', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ knownIds }),
                });
            } else {
                response = await fetch('/api/list-teams', {
                    method: 'GET',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                });
            }

            if (!response.ok) {
                if (response.status === 403 || response.status === 401) {
                    console.warn('TeamSyncManager: Not authenticated to fetch teams.');
                    return [];
                }
                throw new Error(`Server returned ${response.status}`);
            }

            const teams = await response.json();
            return teams || [];
        } catch (error) {
            console.error('TeamSyncManager: Error fetching remote team list:', error);
            // Return empty list on error to allow app to continue with local data
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
