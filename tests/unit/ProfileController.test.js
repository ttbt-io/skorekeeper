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

import { ProfileController } from '../../frontend/controllers/ProfileController.js';

describe('ProfileController', () => {
    let controller;
    let mockApp;

    beforeEach(() => {
        mockApp = {
            auth: {
                getUser: jest.fn(),
                logout: jest.fn(),
            },
            data: {
                getStorageStats: jest.fn(),
                deleteAllLocalData: jest.fn(),
                deleteAllRemoteData: jest.fn(),
            },
            modalConfirmFn: jest.fn(),
            toggleSidebar: jest.fn(),
            reload: jest.fn(),
            render: jest.fn(),
            state: { view: '' },
        };

        // Mock window.location.reload

        controller = new ProfileController(mockApp);
        // Mock the renderer instance attached to controller
        controller.renderer = {
            render: jest.fn(),
            bindEvents: jest.fn(),
        };
    });

    afterEach(() => {
        jest.clearAllMocks();
    });

    describe('loadProfile', () => {
        test('should fetch data and render', async() => {
            const user = { email: 'test@example.com' };
            const stats = { local: { games: 1 }, remote: { games: 2 } };

            mockApp.auth.getUser.mockReturnValue(user);
            mockApp.data.getStorageStats.mockResolvedValue(stats);

            await controller.loadProfile();

            expect(mockApp.auth.getUser).toHaveBeenCalled();
            expect(mockApp.data.getStorageStats).toHaveBeenCalled();
            expect(controller.renderer.render).toHaveBeenCalledWith(user, stats);
            expect(controller.renderer.bindEvents).toHaveBeenCalled();
        });
    });

    describe('handleDeleteLocal', () => {
        test('should delete local data and reload on confirm', async() => {
            mockApp.modalConfirmFn.mockResolvedValue(true);
            mockApp.data.deleteAllLocalData.mockResolvedValue();

            await controller.handleDeleteLocal();

            expect(mockApp.modalConfirmFn).toHaveBeenCalled();
            expect(mockApp.data.deleteAllLocalData).toHaveBeenCalled();
            expect(mockApp.reload).toHaveBeenCalled();
        });

        test('should do nothing on cancel', async() => {
            mockApp.modalConfirmFn.mockResolvedValue(false);

            await controller.handleDeleteLocal();

            expect(mockApp.data.deleteAllLocalData).not.toHaveBeenCalled();
            expect(mockApp.reload).not.toHaveBeenCalled();
        });

        test('should show error on failure', async() => {
            mockApp.modalConfirmFn.mockResolvedValue(true);
            mockApp.data.deleteAllLocalData.mockRejectedValue(new Error('Failed'));

            await controller.handleDeleteLocal();

            // First call was confirmation, second should be error alert
            expect(mockApp.modalConfirmFn).toHaveBeenCalledTimes(2);
            expect(mockApp.modalConfirmFn).toHaveBeenLastCalledWith(expect.stringContaining('Failed'), expect.objectContaining({ isError: true }));
        });
    });

    describe('handleDeleteRemote', () => {
        test('should require double confirmation and delete remote data', async() => {
            mockApp.auth.getUser.mockReturnValue({ email: 'test@example.com' });
            // Mock sequence of confirmations: True, True
            mockApp.modalConfirmFn
                .mockResolvedValueOnce(true) // First confirm
                .mockResolvedValueOnce(true); // Second confirm

            mockApp.data.deleteAllRemoteData.mockResolvedValue();

            // Mock loadProfile to ensure it's called after delete
            jest.spyOn(controller, 'loadProfile').mockResolvedValue();

            await controller.handleDeleteRemote();

            expect(mockApp.modalConfirmFn).toHaveBeenCalledTimes(3); // 2 confirms + 1 success alert
            expect(mockApp.data.deleteAllRemoteData).toHaveBeenCalled();
            expect(mockApp.auth.logout).toHaveBeenCalled();
        });

        test('should stop if first confirm cancelled', async() => {
            mockApp.auth.getUser.mockReturnValue({ email: 'test@example.com' });
            mockApp.modalConfirmFn.mockResolvedValueOnce(false);

            await controller.handleDeleteRemote();

            expect(mockApp.data.deleteAllRemoteData).not.toHaveBeenCalled();
        });

        test('should stop if second confirm cancelled', async() => {
            mockApp.auth.getUser.mockReturnValue({ email: 'test@example.com' });
            mockApp.modalConfirmFn
                .mockResolvedValueOnce(true)
                .mockResolvedValueOnce(false);

            await controller.handleDeleteRemote();

            expect(mockApp.data.deleteAllRemoteData).not.toHaveBeenCalled();
        });
    });
});