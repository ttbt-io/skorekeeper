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

import { AppController } from './controllers/AppController.js';
import { modalConfirm } from './ui/modalPrompt.js';

let updateConfirmed = false;

let bc;
if ('BroadcastChannel' in window) {
    bc = new BroadcastChannel('skorekeeper');
    bc.onmessage = e => {
        if (e.data === 'reload') {
            window.location.reload();
        }
    };
}

if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
        navigator.serviceWorker.register('./sw.js')
            .then((registration) => {
                console.log('ServiceWorker registration successful with scope: ', registration.scope);

                // Try to update the service worker on every page load
                registration.update()
                    .then(() => handleServiceWorkerUpdates(registration))
                    .catch(err => {
                    // Swallow the error to prevent the global error handler from showing a modal.
                    // This often happens in dev environments or when offline.
                        console.warn('ServiceWorker update failed:', err);
                    });
            })
            .catch((err) => {
                console.log('ServiceWorker registration failed: ', err);
            });
    });

    // Handle controller change (reload the page)
    let refreshing = false;
    navigator.serviceWorker.addEventListener('controllerchange', () => {
        if (updateConfirmed && !refreshing) {
            if (bc) {
                bc.postMessage('reload');
            }
            refreshing = true;
            window.location.reload();
        }
    });
}

function handleServiceWorkerUpdates(registration) {
    // Check if there is already a waiting service worker
    if (registration.waiting) {
        promptForUpdate(registration.waiting);
    }

    registration.addEventListener('updatefound', () => {
        const newWorker = registration.installing;
        if (newWorker) {
            newWorker.addEventListener('statechange', () => {
                if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
                    // New update available
                    promptForUpdate(newWorker);
                }
            });
        }
    });
}

function promptForUpdate(worker) {
    modalConfirm('A new version of Scorekeeper is available. Update now?', {
        okText: 'Update',
        cancelText: 'Later',
    }).then((confirmed) => {
        if (confirmed) {
            updateConfirmed = true;
            worker.postMessage({ type: 'SKIP_WAITING' });
        }
    });
}

document.addEventListener('DOMContentLoaded', () => {
    window.app = new AppController();
});
