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

import { TextEncoder, TextDecoder } from 'util';
import { ReadableStream, TextDecoderStream } from 'stream/web';

// Polyfill globals for Node/Jest environment
global.TextEncoder = TextEncoder;
global.TextDecoder = TextDecoder;
global.ReadableStream = ReadableStream;
global.TextDecoderStream = TextDecoderStream;

// Mock fetch globally
global.fetch = jest.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: async() => ({ data: [], meta: { total: 0 } }),
    text: async() => '',
    headers: { get: () => null },
});

// Mock WebSocket globally
global.WebSocket = class MockWebSocket {
    constructor() {
        this.readyState = 3;
    } // CLOSED
    close() {
    }
    send() {
    }
};

// Mock LocalStorage
const localStorageMock = (function() {
    let store = {};
    return {
        getItem: function(key) {
            return store[key] || null;
        },
        setItem: function(key, value) {
            store[key] = value.toString();
        },
        removeItem: function(key) {
            delete store[key];
        },
        clear: function() {
            store = {};
        },
    };
})();
Object.defineProperty(window, 'localStorage', { value: localStorageMock });
