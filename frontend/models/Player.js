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

/**
 * Represents a player in the Skorekeeper system.
 */
export class Player {
    /**
     * @param {object} data - Initial player data.
     */
    constructor(data = {}) {
        this.id = data.id || generateUUID();
        this.name = data.name || '';
        this.number = data.number || '';
        this.pos = data.pos || '';
    }

    /**
     * Returns a plain object representation for JSON serialization.
     */
    toJSON() {
        return {
            id: this.id,
            name: this.name,
            number: this.number,
            pos: this.pos,
        };
    }
}
