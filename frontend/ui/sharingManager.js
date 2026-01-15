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

import { ActionTypes } from '../reducer.js';

/**
 * Manages the Sharing Modal and collaboration settings.
 */
export class SharingManager {
    /**
     * @param {object} options
     * @param {Function} options.dispatch - App dispatch function.
     * @param {Function} options.renderCollaborators - Legacy render for backward compat or just logic.
     */
    constructor({ dispatch }) {
        this.dispatch = dispatch;
    }

    /**
     * Logic for opening the share modal.
     * The App will still handle the DOM visibility for now, but this holds the initialization logic.
     */
    getInitialShareState(game) {
        const isPublic = game.permissions && game.permissions.public === 'read';
        const baseUrl = window.location.origin + window.location.pathname;

        return {
            isPublic,
            shareUrl: `${baseUrl}#game/${game.id}`,
            broadcastUrl: `${baseUrl}#broadcast/${game.id}`,
        };
    }

    async addCollaborator(game, email) {
        const permissions = JSON.parse(JSON.stringify(game.permissions || { public: 'none', users: {} }));
        if (!permissions.users) {
            permissions.users = {};
        }
        permissions.users[email.toLowerCase()] = 'write';

        await this.dispatch({
            type: ActionTypes.GAME_METADATA_UPDATE,
            payload: {
                id: game.id,
                permissions: permissions,
            },
        });
    }

    async removeCollaborator(game, email) {
        const permissions = JSON.parse(JSON.stringify(game.permissions || { public: 'none', users: {} }));
        if (permissions.users) {
            delete permissions.users[email];
            await this.dispatch({
                type: ActionTypes.GAME_METADATA_UPDATE,
                payload: {
                    id: game.id,
                    permissions: permissions,
                },
            });
        }
    }

    async togglePublicSharing(game, isPublic) {
        const permissions = JSON.parse(JSON.stringify(game.permissions || { public: 'none', users: {} }));
        permissions.public = isPublic ? 'read' : 'none';

        await this.dispatch({
            type: ActionTypes.GAME_METADATA_UPDATE,
            payload: {
                id: game.id,
                permissions: permissions,
            },
        });
    }
}
