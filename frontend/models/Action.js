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

import { generateUUID } from '../utils.js';
import { CurrentSchemaVersion } from '../constants.js';

/**
 * Represents a discrete event in the game's action log.
 */
export class Action {
    /**
     * @param {object} data - Initial action data.
     */
    constructor(data = {}) {
        this.id = data.id || generateUUID();
        this.type = data.type || '';
        this.payload = data.payload || {};
        this.schemaVersion = data.schemaVersion || CurrentSchemaVersion;
        this.timestamp = data.timestamp || Date.now();
        this.userId = data.userId || '';
    }

    /**
     * Returns a plain object representation for JSON serialization.
     */
    toJSON() {
        return {
            id: this.id,
            type: this.type,
            payload: this.payload,
            schemaVersion: this.schemaVersion,
            timestamp: this.timestamp,
            userId: this.userId,
        };
    }
}
