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

import { StatsEngine } from '../../frontend/game/statsEngine.js';

describe('StatsEngine - Enhanced Statistics', () => {
    const game = {
        away: 'AwayTeam',
        home: 'HomeTeam',
        columns: [{ id: 'c1', inning: 1 }],
        roster: {
            away: [{ starter: { id: 'p1', name: 'Batter 1' }, current: { id: 'p1', name: 'Batter 1' } }],
            home: [{ starter: { id: 'pitch1', name: 'Pitcher 1' }, current: { id: 'pitch1', name: 'Pitcher 1' } }],
        },
        events: {
            'away-0-c1': {
                pId: 'p1',
                outcome: 'F8',
                balls: 0, strikes: 0,
                outNum: 1,
                paths: [0, 0, 0, 0],
                pitchSequence: [
                    { type: 'strike', pitcher: 'pitch1' },
                    { type: 'ball', pitcher: 'pitch1' },
                    { type: 'strike', pitcher: 'pitch1' },
                ],
            },
        },
    };

    test('categorizes Flyout correctly', () => {
        const stats = StatsEngine.calculateGameStats(game);
        const ps = stats.playerStats['p1'];
        expect(ps.flyouts).toBe(1);
        expect(ps.lineouts).toBe(0);
        expect(ps.ab).toBe(1);
    });

    test('counts Ball-in-Play as a strike for pitcher', () => {
        const stats = StatsEngine.calculateGameStats(game);
        const ps = stats.pitcherStats['pitch1'];
        // 2 strikes from sequence + 1 from BiP (F8)
        expect(ps.strikes).toBe(3);
        expect(ps.balls).toBe(1);
        expect(ps.pitches).toBe(4);
    });

    test('categorizes Groundout correctly', () => {
        const g2 = JSON.parse(JSON.stringify(game));
        g2.events['away-0-c1'].outcome = '6-3';
        const stats = StatsEngine.calculateGameStats(g2);
        const ps = stats.playerStats['p1'];
        expect(ps.groundouts).toBe(1);
        expect(ps.flyouts).toBe(0);
    });

    test('categorizes Lineout correctly', () => {
        const g2 = JSON.parse(JSON.stringify(game));
        g2.events['away-0-c1'].outcome = 'L4';
        const stats = StatsEngine.calculateGameStats(g2);
        const ps = stats.playerStats['p1'];
        expect(ps.lineouts).toBe(1);
        expect(ps.flyouts).toBe(0);
    });

    test('calculates defensiveOuts correctly', () => {
        const stats = StatsEngine.calculateGameStats(game);
        const ps = stats.pitcherStats['pitch1'];
        expect(ps.ipOuts).toBe(1);
        expect(ps.defensiveOuts).toBe(1);
    });

    test('strikeout is NOT a defensive out', () => {
        const g2 = JSON.parse(JSON.stringify(game));
        g2.events['away-0-c1'].outcome = 'K';
        const stats = StatsEngine.calculateGameStats(g2);
        const ps = stats.pitcherStats['pitch1'];
        expect(ps.ipOuts).toBe(1);
        expect(ps.defensiveOuts).toBe(0);
        expect(ps.k).toBe(1);
    });

    test('called strikes from pitch sequence are tracked for batter', () => {
        const stats = StatsEngine.calculateGameStats(game);
        const ps = stats.playerStats['p1'];
        // 2 strikes in sequence were type 'strike' (called)
        expect(ps.calledStrikes).toBe(2);
    });

    test('ROE is tracked correctly', () => {
        const g2 = JSON.parse(JSON.stringify(game));
        g2.events['away-0-c1'].outcome = 'E5';
        g2.events['away-0-c1'].outNum = 0;
        const stats = StatsEngine.calculateGameStats(g2);
        const ps = stats.playerStats['p1'];
        expect(ps.roe).toBe(1);
        expect(ps.ab).toBe(1);
        expect(ps.h).toBe(0);
    });
});
