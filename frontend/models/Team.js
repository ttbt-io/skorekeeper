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
import { Player } from './Player.js';

/**
 * Represents a Team in the Skorekeeper system.
 */
export class Team {
    /**
     * @param {object} data - Initial team data.
     */
    constructor(data = {}) {
        this.id = data.id || generateUUID();
        this.schemaVersion = data.schemaVersion || CurrentSchemaVersion;
        this.name = data.name || '';
        this.shortName = data.shortName || '';
        this.color = data.color || '#2563eb';
        this.ownerId = data.ownerId || '';
        this.roles = data.roles || { admins: [], scorekeepers: [], spectators: [] };
        this.updatedAt = data.updatedAt || Date.now();

        // Ensure roster contains Player instances
        this.roster = (data.roster || []).map(p => p instanceof Player ? p : new Player(p));
    }

    /**
     * Returns a plain object representation for JSON serialization.
     */
    toJSON() {
        return {
            id: this.id,
            schemaVersion: this.schemaVersion,
            name: this.name,
            shortName: this.shortName,
            color: this.color,
            ownerId: this.ownerId,
            roles: this.roles,
            updatedAt: this.updatedAt,
            roster: this.roster.map(p => p.toJSON()),
        };
    }
}
