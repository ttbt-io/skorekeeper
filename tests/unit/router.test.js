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

import { Router } from '../../frontend/router.js';

describe('Router', () => {
    let router;

    beforeEach(() => {
        router = new Router();
    });

    test('should parse empty hash as dashboard', () => {
        expect(router.parseHash('')).toEqual({ view: 'dashboard', params: {} });
        expect(router.parseHash('#')).toEqual({ view: 'dashboard', params: {} });
    });

    test('should parse #teams', () => {
        expect(router.parseHash('#teams')).toEqual({ view: 'teams', params: {} });
    });

    test('should parse #stats', () => {
        expect(router.parseHash('#stats')).toEqual({ view: 'stats', params: {} });
    });

    test('should parse #broadcast/id', () => {
        expect(router.parseHash('#broadcast/game-123')).toEqual({
            view: 'broadcast',
            params: { gameId: 'game-123' },
        });
    });

    test('should parse #feed/id', () => {
        expect(router.parseHash('#feed/game-123')).toEqual({
            view: 'scoresheet',
            params: { gameId: 'game-123', subView: 'feed' },
        });
    });

    test('should parse #game/id', () => {
        expect(router.parseHash('#game/game-123')).toEqual({
            view: 'scoresheet',
            params: { gameId: 'game-123', subView: 'grid' },
        });
    });

    test('should parse unknown hash as dashboard', () => {
        expect(router.parseHash('#unknown')).toEqual({ view: 'dashboard', params: {} });
    });
});
