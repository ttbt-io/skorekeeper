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
 * Service for handling URL hash routing.
 * Parses the current hash and returns the appropriate view and parameters.
 */
export class Router {
    /**
     * Parses the current URL hash.
     * @param {string} hash - The current window.location.hash.
     * @returns {object} { view, params }
     */
    parseHash(hash) {
        if (!hash || hash === '#') {
            return { view: 'dashboard', params: {} };
        }

        if (hash === '#teams') {
            return { view: 'teams', params: {} };
        }

        if (hash === '#stats') {
            return { view: 'stats', params: {} };
        }

        if (hash.startsWith('#broadcast/')) {
            const gameId = hash.substring(11);
            return { view: 'broadcast', params: { gameId } };
        }

        if (hash.startsWith('#feed/')) {
            const gameId = hash.substring(6);
            return { view: 'scoresheet', params: { gameId, subView: 'feed' } };
        }

        if (hash.startsWith('#game/')) {
            const gameId = hash.substring(6);
            return { view: 'scoresheet', params: { gameId, subView: 'grid' } };
        }

        if (hash.startsWith('#team/')) {
            const teamId = hash.substring(6);
            return { view: 'team', params: { teamId } };
        }

        return { view: 'dashboard', params: {} };
    }
}
