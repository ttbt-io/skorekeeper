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

import { ProfileRenderer } from '../renderers/profileRenderer.js';

export class ProfileController {
    /**
     * @param {object} app - The main AppController instance.
     */
    constructor(app) {
        this.app = app;
        this.renderer = new ProfileRenderer();
    }

    async loadProfile() {
        this.app.state.view = 'profile';
        this.app.render(); // Ensure view container is visible

        // 1. Get Auth Status
        const user = this.app.auth.getUser();

        // 2. Get Stats
        const stats = await this.app.data.getStorageStats();

        // 3. Render
        this.renderer.render(user, stats);
        this.renderer.bindEvents({
            onDeleteLocal: () => this.handleDeleteLocal(),
            onDeleteRemote: () => this.handleDeleteRemote(),
            onBack: () => this.app.toggleSidebar(true),
        });
    }

    async handleDeleteLocal() {
        const confirmed = await this.app.modalConfirmFn(
            'Are you sure you want to clear all data from this device? This action cannot be undone and any unsynced changes will be lost.',
            {
                okText: 'Clear Device Data',
                cancelText: 'Cancel',
                isDanger: true,
            },
        );

        if (confirmed) {
            try {
                await this.app.data.deleteAllLocalData();
                this.app.reload();
            } catch (e) {
                console.error('Failed to clear local data:', e);
                await this.app.modalConfirmFn('Failed to clear data: ' + e.message, { isError: true });
            }
        }
    }

    async handleDeleteRemote() {
        const user = this.app.auth.getUser();
        if (!user) {
            return;
        }

        const confirmed = await this.app.modalConfirmFn(
            `DANGER: You are about to permanently delete ALL games and teams from your account (${user.email}). This will destroy data on the server and cannot be undone.`,
            {
                okText: 'Delete Account Data',
                cancelText: 'Cancel',
                isDanger: true,
            },
        );

        if (confirmed) {
            // Double confirm
            const doubleConfirmed = await this.app.modalConfirmFn(
                'Please confirm one last time: This deletes everything on the server. Are you absolutely sure?',
                {
                    okText: 'YES, DELETE EVERYTHING',
                    cancelText: 'Stop',
                    isDanger: true,
                },
            );

            if (doubleConfirmed) {
                try {
                    await this.app.data.deleteAllRemoteData();
                    await this.app.modalConfirmFn('All server data has been deleted. You will now be logged out.', { autoClose: false });
                    await this.app.auth.logout();
                } catch (e) {
                    console.error('Failed to delete remote data:', e);
                    await this.app.modalConfirmFn('Failed to delete data: ' + e.message, { isError: true });
                }
            }
        }
    }
}
