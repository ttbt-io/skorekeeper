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
 * Represents a Game state in the Skorekeeper system.
 */
export class Game {
    /**
     * @param {object} data - Initial game data.
     */
    constructor(data = {}) {
        this.id = data.id || generateUUID();
        this.schemaVersion = data.schemaVersion || CurrentSchemaVersion;
        this.date = data.date || new Date().toISOString();
        this.location = data.location || '';
        this.event = data.event || '';
        this.away = data.away || '';
        this.home = data.home || '';
        this.status = data.status || 'ongoing';
        this.ownerId = data.ownerId || '';
        this.awayTeamId = data.awayTeamId || '';
        this.homeTeamId = data.homeTeamId || '';

        this.pitchers = data.pitchers || { away: '', home: '' };
        this.overrides = data.overrides || { away: {}, home: {} };
        this.events = data.events || {};
        this.columns = data.columns || [];
        this.pitchLog = data.pitchLog || [];
        this.actionLog = data.actionLog || [];
        this.permissions = data.permissions || { public: 'none', users: {} };

        // Normalize Roster
        this.roster = {
            away: (data.roster?.away || []).map(s => this.normalizeSlot(s)),
            home: (data.roster?.home || []).map(s => this.normalizeSlot(s)),
        };

        // Normalize Subs
        this.subs = {
            away: (data.subs?.away || []).map(p => p instanceof Player ? p : new Player(p)),
            home: (data.subs?.home || []).map(p => p instanceof Player ? p : new Player(p)),
        };
    }

    /**
     * Normalizes a roster slot to ensure it contains Player instances.
     * @private
     */
    normalizeSlot(slot) {
        return {
            slot: slot.slot ?? 0,
            starter: slot.starter instanceof Player ? slot.starter : new Player(slot.starter),
            current: slot.current instanceof Player ? slot.current : new Player(slot.current),
            history: (slot.history || []).map(p => p instanceof Player ? p : new Player(p)),
        };
    }

    /**
     * Returns a plain object representation for JSON serialization.
     * optimized to exclude derived state that can be recomputed from actionLog.
     */
    toJSON() {
        const json = {
            id: this.id,
            schemaVersion: this.schemaVersion,
            date: this.date,
            location: this.location,
            event: this.event,
            away: this.away,
            home: this.home,
            status: this.status,
            ownerId: this.ownerId,
            awayTeamId: this.awayTeamId,
            homeTeamId: this.homeTeamId,
            permissions: this.permissions,
            actionLog: this.actionLog,
        };
        return json;
    }
}
