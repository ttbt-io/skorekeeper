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

const CACHE_NAME = 'skorekeeper-v0.1.122';
const CORE_ASSETS = [
    './',
    './.sso/proxy.mjs',
    './constants.js',
    './controllers/ActiveGameController.js',
    './controllers/AppController.js',
    './controllers/DashboardController.js',
    './controllers/ProfileController.js',
    './controllers/TeamController.js',
    './css/style.css',
    './game/csoManager.js',
    './game/historyManager.js',
    './game/lineupManager.js',
    './game/narrativeEngine.js',
    './game/runnerManager.js',
    './game/statsEngine.js',
    './game/substitutionManager.js',
    './icon-144x144.png',
    './icon-192x192.png',
    './icon-512x512.png',
    './icon-64x64.png',
    './icon.png',
    './index.html',
    './init.js',
    './login-success.js',
    './manifest.json',
    './manualContent.js',
    './models/Action.js',
    './models/Game.js',
    './models/Player.js',
    './models/Team.js',
    './reducer.js',
    './renderers/csoRenderer.js',
    './renderers/dashboardRenderer.js',
    './renderers/profileRenderer.js',
    './renderers/scoresheetRenderer.js',
    './renderers/statsRenderer.js',
    './renderers/teamsRenderer.js',
    './router.js',
    './services/DataService.js',
    './services/PermissionService.js',
    './services/authManager.js',
    './services/backupManager.js',
    './services/dbManager.js',
    './services/errorLogger.js',
    './services/syncManager.js',
    './services/teamSyncManager.js',
    './ui/contextMenuManager.js',
    './ui/manualViewer.js',
    './ui/modalPrompt.js',
    './ui/pullToRefresh.js',
    './ui/sharingManager.js',
    './utils.js',
    './utils/searchParser.js',
    './vendor/qrcode.mjs',
];

const OPTIONAL_ASSETS = [
    // Manual assets
    './assets/manual/broadcast-overlay.png',
    './assets/manual/cell-advance.png',
    './assets/manual/cell-double.png',
    './assets/manual/cell-empty.png',
    './assets/manual/cell-flyball.png',
    './assets/manual/cell-flyout.png',
    './assets/manual/cell-grounder.png',
    './assets/manual/cell-groundout.png',
    './assets/manual/cell-homerun.png',
    './assets/manual/cell-linedrive.png',
    './assets/manual/cell-popfly.png',
    './assets/manual/cell-single.png',
    './assets/manual/cell-strikeout.png',
    './assets/manual/cell-walk.png',
    './assets/manual/conflict-resolution.png',
    './assets/manual/correct-batter.png',
    './assets/manual/cso-pitch.png',
    './assets/manual/cycle-options.png',
    './assets/manual/dashboard.png',
    './assets/manual/edit-lineup.png',
    './assets/manual/new-game.png',
    './assets/manual/play-context-move.png',
    './assets/manual/play-dp.png',
    './assets/manual/play-dropped-3rd.png',
    './assets/manual/play-fly-out.png',
    './assets/manual/play-ground-out.png',
    './assets/manual/play-hit-single.png',
    './assets/manual/play-homerun.png',
    './assets/manual/play-out-options.png',
    './assets/manual/play-runner-advance.png',
    './assets/manual/play-steal.png',
    './assets/manual/play-strikeout-swinging.png',
    './assets/manual/scoresheet.png',
    './assets/manual/sidebar.png',
    './assets/manual/statistics.png',
    './assets/manual/team-members.png',
];

self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open(CACHE_NAME).then(async(cache) => {
            console.log('[ServiceWorker] Pre-caching offline page');

            // Cache Core Assets (Fail if missing)
            for (const url of CORE_ASSETS) {
                try {
                    await cache.add(url);
                } catch (error) {
                    console.error(`[ServiceWorker] Failed to cache core asset ${url}:`, error);
                    throw error; // Fail installation
                }
            }

            // Cache Optional Assets (Log if missing)
            for (const url of OPTIONAL_ASSETS) {
                try {
                    await cache.add(url);
                } catch (error) {
                    console.error(`[ServiceWorker] Failed to cache optional asset ${url}:`, error);
                }
            }
        }),
    );
});

self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches.keys().then((keyList) => {
            return Promise.all(keyList.map((key) => {
                if (key !== CACHE_NAME) {
                    console.log('[ServiceWorker] Removing old cache', key);
                    return caches.delete(key);
                }
            }));
        }),
    );
    self.clients.claim();
});

self.addEventListener('message', (event) => {
    if (event.data && event.data.type === 'SKIP_WAITING') {
        self.skipWaiting();
    }
});

self.addEventListener('fetch', (event) => {
    // Ignore non-GET requests
    if (event.request.method !== 'GET') {
        return;
    }

    // API requests: Network Only (or let the app handle it)
    if (event.request.url.includes('/api/')) {
        return;
    }

    // Static Assets: Cache-First, falling back to Network (lazy caching)
    event.respondWith(
        caches.match(event.request).then((cachedResponse) => {
            if (cachedResponse) {
                return cachedResponse;
            }

            return fetch(event.request).then((networkResponse) => {
                // Check if we received a valid response
                if (!networkResponse || networkResponse.status !== 200 || networkResponse.type !== 'basic') {
                    return networkResponse;
                }

                // Check Cache-Control header
                const cacheControl = networkResponse.headers.get('Cache-Control');
                if (cacheControl && (cacheControl.includes('no-cache') || cacheControl.includes('no-store'))) {
                    return networkResponse;
                }

                // Clone the response
                const responseToCache = networkResponse.clone();

                caches.open(CACHE_NAME).then((cache) => {
                    cache.put(event.request, responseToCache);
                });

                return networkResponse;
            });
        }),
    );
});
