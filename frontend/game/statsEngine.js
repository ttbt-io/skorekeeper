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

/**
 * StatsEngine utility for aggregating and calculating baseball/softball statistics.
 */
export class StatsEngine {
    /**
     * Calculates statistics for a single game.
     * @param {object} game - The game state object.
     * @returns {object} Calculated statistics.
     */
    static calculateGameStats(game) {
        if (!game) {
            return {
                playerStats: {},
                inningStats: {},
                pitcherStats: {},
                score: { away: { R: 0, H: 0, E: 0 }, home: { R: 0, H: 0, E: 0 } },
                currentPA: null,
                innings: { away: {}, home: {} },
                hasAB: { away: {}, home: {} },
            };
        }

        const playerStats = {};
        const inningStats = {}; // Keys: 'team-colId'
        const pitcherStats = {};

        const columns = game.columns || [];
        const events = game.events || {};

        // Initialize inning stats for both teams
        columns.forEach((c) => {
            inningStats[`away-${c.id}`] = { r: 0, h: 0, e: 0, lob: 0, outs: 0, pa: 0 };
            inningStats[`home-${c.id}`] = { r: 0, h: 0, e: 0, lob: 0, outs: 0, pa: 0 };
        });

        const getPStats = (pId, name = '', teamName = '') => {
            if (!pId) {
                return null;
            }
            if (!playerStats[pId]) {
                playerStats[pId] = {
                    ab: 0, r: 0, h: 0, rbi: 0, bb: 0, k: 0, hbp: 0, sf: 0, sh: 0,
                    singles: 0, doubles: 0, triples: 0, hr: 0, sb: 0, pa: 0,
                    flyouts: 0, lineouts: 0, groundouts: 0, otherOuts: 0, roe: 0, calledStrikes: 0,
                    name: name,
                    team: teamName,
                };
            } else {
                if (name && !playerStats[pId].name) {
                    playerStats[pId].name = name;
                }
                if (teamName && !playerStats[pId].team) {
                    playerStats[pId].team = teamName;
                }
            }
            return playerStats[pId];
        };

        const getPitcherStats = (pId, teamName = '') => {
            if (!pId) {
                return null;
            }
            if (!pitcherStats[pId]) {
                pitcherStats[pId] = {
                    ipOuts: 0, h: 0, bb: 0, er: 0, k: 0, hbp: 0,
                    pitches: 0, strikes: 0, balls: 0, bf: 0,
                    defensiveOuts: 0, errors: 0,
                    team: teamName,
                };
            } else if (teamName && !pitcherStats[pId].team) {
                pitcherStats[pId].team = teamName;
            }
            return pitcherStats[pId];
        };

        Object.keys(events).forEach((key) => {
            const evt = events[key];
            const pId = evt.pId;
            let pName = '';
            const parts = key.split('-');
            const team = parts[0];
            const slot = parseInt(parts[1]);
            const teamName = team === 'away' ? (game.away || '') : (game.home || '');
            const otherTeamName = team === 'away' ? (game.home || '') : (game.away || '');

            if (game.roster && game.roster[team] && game.roster[team][slot]) {
                const cur = game.roster[team][slot].current;
                pName = cur.name || '';
            }

            const stats = getPStats(pId, pName, teamName);
            const colId = parts.slice(2).join('-');
            const iStats = inningStats[`${team}-${colId}`];
            const otherTeam = team === 'away' ? 'home' : 'away';
            const iStatsDefense = inningStats[`${otherTeam}-${colId}`];

            if (stats) {
                stats.pa++;
                if (iStats) {
                    iStats.pa++;
                    if (evt.outNum) {
                        iStats.outs = Math.max(iStats.outs, evt.outNum);
                    }
                }
                if (evt.outcome) {
                    const out = evt.outcome;
                    const isSac = out.includes('SH') || out.includes('SF');
                    const isWalk = out.startsWith('BB') || out.startsWith('IBB');
                    const isHBP = out === 'HBP';
                    const isInt = out.includes('CI') || out.includes('INT');

                    if (!isWalk && !isHBP && !isSac && !isInt) {
                        stats.ab++;
                    }
                    if (out.includes('SH')) {
                        stats.sh++;
                    }
                    if (out.includes('SF')) {
                        stats.sf++;
                    }
                    if (isHBP) {
                        stats.hbp++;
                    }

                    if (out === '1B') {
                        stats.h++; stats.singles++;
                        if (iStats) {
                            iStats.h++;
                        }
                    }
                    else if (out === '2B') {
                        stats.h++; stats.doubles++;
                        if (iStats) {
                            iStats.h++;
                        }
                    }
                    else if (out === '3B') {
                        stats.h++; stats.triples++;
                        if (iStats) {
                            iStats.h++;
                        }
                    }
                    else if (out === 'HR') {
                        stats.h++; stats.hr++;
                        if (iStats) {
                            iStats.h++;
                        }
                    }

                    // Detailed Categorization
                    if (out.startsWith('E')) {
                        stats.roe++;
                        if (iStatsDefense) {
                            iStatsDefense.e++;
                        }
                    } else if (/^[FP]/.test(out)) {
                        stats.flyouts++;
                    } else if (out.startsWith('L')) {
                        stats.lineouts++;
                    } else if (out.includes('-')) {
                        stats.groundouts++;
                    } else if (out !== '1B' && out !== '2B' && out !== '3B' && out !== 'HR' && !isWalk && !isHBP && !isInt) {
                        if (!out.includes('K') && out !== 'ꓘ') {
                            stats.otherOuts++;
                        }
                    }

                    if (out.includes('K') || out === 'ꓘ') {
                        stats.k++;
                        if (out === 'ꓘ') {
                            stats.calledStrikes++;
                        }
                    }
                    if (isWalk) {
                        stats.bb++;
                    }
                }

                // Called strikes from pitch sequence
                if (evt.pitchSequence && Array.isArray(evt.pitchSequence)) {
                    evt.pitchSequence.forEach(p => {
                        if (p.type === 'strike') {
                            stats.calledStrikes++;
                        }
                    });
                }

                // Error counting from paths (e.g. runner advances on error)
                if (evt.pathInfo) {
                    evt.pathInfo.forEach(info => {
                        if (info && typeof info === 'string' && info.startsWith('E')) {
                            if (iStatsDefense) {
                                iStatsDefense.e++;
                            }
                        }
                    });
                }

                if (evt.paths[3] === 1) {
                    stats.r++;
                    if (iStats) {
                        iStats.r++;
                    }
                    if (evt.scoreInfo && evt.scoreInfo.rbiCreditedTo) {
                        const creditor = getPStats(evt.scoreInfo.rbiCreditedTo, '', teamName);
                        if (creditor) {
                            creditor.rbi++;
                        }
                    }
                }

                if (iStats) {
                    const outs = iStats.outs || 0;
                    const runs = iStats.r || 0;
                    const pa = iStats.pa || 0;
                    iStats.lob = Math.max(0, pa - outs - runs);
                }
            }

            if (evt.pitchSequence) {
                evt.pitchSequence.forEach(p => {
                    const ps = getPitcherStats(p.pitcher, otherTeamName);
                    if (ps) {
                        ps.pitches++;
                        if (['strike', 'foul', 'out'].includes(p.type)) {
                            ps.strikes++;
                        } else if (p.type === 'ball') {
                            ps.balls++;
                        }
                    }
                });
            }

            const lastPitch = evt.pitchSequence && evt.pitchSequence.length > 0 ? evt.pitchSequence[evt.pitchSequence.length - 1] : null;
            const finalPitcher = lastPitch ? lastPitch.pitcher : null;
            const ps = getPitcherStats(finalPitcher, otherTeamName);
            if (ps) {
                ps.bf++;
                // Ball-in-Play counts as a strike for stats
                if (evt.outcome && !evt.outcome.includes('K') && evt.outcome !== 'ꓘ' && !['BB', 'IBB', 'HBP', 'CI'].some(prefix => evt.outcome.startsWith(prefix))) {
                    ps.strikes++;
                    ps.pitches++;
                }

                const out = evt.outcome;
                if (out.startsWith('BB') || out.startsWith('IBB')) {
                    ps.bb++;
                }
                if (out === 'HBP') {
                    ps.hbp++;
                }
                if (['1B', '2B', '3B', 'HR'].includes(out)) {
                    ps.h++;
                }
                if (out.includes('K') || out === 'ꓘ') {
                    ps.k++;
                }
                if (out.startsWith('E')) {
                    ps.errors++;
                }
                if (evt.paths[3] === 1) {
                    ps.er++;
                }
            }
        });

        // IP Calculation
        const teamInningEvents = {};
        Object.keys(events).forEach(k => {
            const parts = k.split('-');
            const ck = `${parts[0]}-${parts.slice(2).join('-')}`;
            if (!teamInningEvents[ck]) {
                teamInningEvents[ck] = [];
            }
            teamInningEvents[ck].push({ slot: parseInt(parts[1]), evt: events[k] });
        });

        Object.keys(teamInningEvents).forEach(ck => {
            const sorted = teamInningEvents[ck].sort((a, b) => a.slot - b.slot);
            let lastOuts = 0;
            sorted.forEach(e => {
                const currentOuts = e.evt.outNum || 0;
                const outsInPA = Math.max(0, currentOuts - lastOuts);
                if (outsInPA > 0) {
                    const lastPitch = e.evt.pitchSequence && e.evt.pitchSequence.length > 0 ? e.evt.pitchSequence[e.evt.pitchSequence.length - 1] : null;
                    const ps = getPitcherStats(lastPitch ? lastPitch.pitcher : null);
                    if (ps) {
                        ps.ipOuts += outsInPA;
                        const ksInPA = (e.evt.outcome && (e.evt.outcome.includes('K') || e.evt.outcome === 'ꓘ')) ? 1 : 0;
                        ps.defensiveOuts += Math.max(0, outsInPA - ksInPA);
                    }
                }
                lastOuts = currentOuts;
            });
        });

        const score = {
            away: { R: this.calculateTotalRunsInternal(game, 'away', inningStats), H: 0, E: 0 },
            home: { R: this.calculateTotalRunsInternal(game, 'home', inningStats), H: 0, E: 0 },
        };

        const innings = { away: {}, home: {} };
        const hasAB = { away: {}, home: {} };

        ['away', 'home'].forEach(team => {
            columns.forEach(col => {
                const is = inningStats[`${team}-${col.id}`];
                if (!is) {
                    return;
                }

                // Errors committed by this team (regardless of batting side)
                score[team].E += is.e;

                if (!col.team || col.team === team) {
                    const inn = col.inning;
                    innings[team][inn] = (innings[team][inn] || 0) + is.r;
                    score[team].H += is.h;
                }
            });

            // Calculate hasAB
            Object.keys(events).forEach(k => {
                if (k.startsWith(team + '-')) {
                    const parts = k.split('-');
                    const colId = parts.slice(2).join('-');
                    const col = columns.find(c => c.id === colId);
                    if (col) {
                        const evt = events[k];
                        if (evt.outcome || (evt.pitchSequence && evt.pitchSequence.length > 0)) {
                            hasAB[team][col.inning] = true;
                        }
                    }
                }
            });
        });

        const inningOuts = {};
        Object.keys(events).forEach(k => {
            const parts = k.split('-');
            const colId = parts.slice(2).join('-');
            const col = (game.columns || []).find(c => c.id === colId);
            if (col) {
                const ik = `${parts[0]}-${col.inning}`;
                inningOuts[ik] = Math.max(inningOuts[ik] || 0, events[k].outNum || 0);
            }
        });

        let currentPA = null;
        const sortedInnings = [...new Set((game.columns || []).map(c => c.inning))].sort((a, b) => a - b);
        for (let i = sortedInnings.length - 1; i >= 0; i--) {
            const inn = sortedInnings[i];
            const teams = ['home', 'away']; // Check home first (bottom of inning)
            for (const team of teams) {
                if (inningOuts[`${team}-${inn}`] < 3) {
                    const teamEvents = Object.keys(events).filter(k => k.startsWith(team + '-'));
                    let lastSlot = -1;
                    let lastColId = '';
                    teamEvents.forEach(k => {
                        const parts = k.split('-');
                        const colId = parts.slice(2).join('-');
                        const col = columns.find(c => c.id === colId);
                        if (col && col.inning === inn) {
                            const slot = parseInt(parts[1]);
                            if (slot > lastSlot) {
                                lastSlot = slot; lastColId = colId;
                            }
                        }
                    });
                    if (lastSlot !== -1) {
                        const e = events[`${team}-${lastSlot}-${lastColId}`];
                        currentPA = { inning: inn, team, balls: e.balls, strikes: e.strikes, outs: e.outNum || 0, paths: e.paths };
                        break;
                    }
                }
            }
            if (currentPA) {
                break;
            }
        }
        if (!currentPA) {
            currentPA = { inning: 1, team: 'away', balls: 0, strikes: 0, outs: 0, paths: [0, 0, 0, 0] };
        }

        return { playerStats, inningStats, pitcherStats, score, currentPA, innings, hasAB };
    }

    static calculateTotalRunsInternal(game, team, inningStats) {
        let total = 0;
        const columns = game.columns || [];
        const uniqueInnings = [...new Set(columns.map(c => c.inning))];
        uniqueInnings.forEach(inn => {
            if (game.overrides && game.overrides[team] && game.overrides[team][inn] !== undefined) {
                total += (parseInt(game.overrides[team][inn]) || 0);
            } else {
                columns.filter(c => c.inning === inn && (!c.team || c.team === team)).forEach(c => {
                    const is = inningStats[`${team}-${c.id}`];
                    if (is) {
                        total += is.r;
                    }
                });
            }
        });
        return total;
    }

    /**
     * Aggregates statistics across multiple games.
     * @param {Array<object>} games - Array of game objects.
     * @returns {object} Aggregated statistics.
     */
    static aggregateStats(games) {
        const aggregated = { players: {}, pitchers: {}, teams: {} };
        games.forEach(game => {
            const stats = this.calculateGameStats(game);
            Object.keys(stats.playerStats).forEach(pId => {
                if (!aggregated.players[pId]) {
                    aggregated.players[pId] = {
                        ab: 0, r: 0, h: 0, rbi: 0, bb: 0, k: 0, hbp: 0, sf: 0, sh: 0,
                        singles: 0, doubles: 0, triples: 0, hr: 0, sb: 0, pa: 0,
                        flyouts: 0, lineouts: 0, groundouts: 0, otherOuts: 0, roe: 0, calledStrikes: 0,
                        games: 0, name: stats.playerStats[pId].name,
                    };
                }
                const p = aggregated.players[pId];
                const s = stats.playerStats[pId];
                Object.keys(s).forEach(k => {
                    if (typeof s[k] === 'number') {
                        p[k] += s[k];
                    }
                });
                p.games++;
            });
            Object.keys(stats.pitcherStats).forEach(pId => {
                if (!aggregated.pitchers[pId]) {
                    aggregated.pitchers[pId] = {
                        ipOuts: 0, h: 0, bb: 0, er: 0, k: 0, hbp: 0,
                        pitches: 0, strikes: 0, balls: 0, bf: 0,
                        defensiveOuts: 0, errors: 0, games: 0,
                    };
                }
                const p = aggregated.pitchers[pId];
                const s = stats.pitcherStats[pId];
                Object.keys(s).forEach(k => {
                    if (p[k] !== undefined && typeof s[k] === 'number') {
                        p[k] += s[k];
                    }
                });
                p.games++;
            });
            ['away', 'home'].forEach(t => {
                const teamId = game[t + 'TeamId'];
                const teamName = game[t];
                if (teamId) {
                    if (!aggregated.teams[teamId]) {
                        aggregated.teams[teamId] = { w: 0, l: 0, t: 0, rs: 0, ra: 0, games: 0, name: teamName };
                    }
                    const ts = aggregated.teams[teamId];
                    if (teamName && !ts.name) {
                        ts.name = teamName;
                    }
                    ts.rs += stats.score[t].R;
                    ts.ra += stats.score[t === 'away' ? 'home' : 'away'].R;
                    ts.games++;
                    if (game.status === 'final') {
                        if (stats.score[t].R > stats.score[t === 'away' ? 'home' : 'away'].R) {
                            ts.w++;
                        } else if (stats.score[t].R < stats.score[t === 'away' ? 'home' : 'away'].R) {
                            ts.l++;
                        } else {
                            ts.t++;
                        }
                    }
                }
            });
        });
        return aggregated;
    }

    /**
     * Aggregates statistics from a list of pre-calculated game stats.
     * @param {Array<object>} statsList - Array of { id, stats } objects.
     * @param {Array<object>} games - Array of game metadata objects (for team names/ids).
     * @param {string} [teamFilter] - Optional team name to filter stats by.
     * @returns {object} Aggregated statistics.
     */
    static aggregatePrecalculatedStats(statsList, games, teamFilter) {
        const aggregated = { players: {}, pitchers: {}, teams: {} };
        const gameMap = new Map();
        if (games) {
            games.forEach(g => gameMap.set(g.id, g));
        }

        statsList.forEach(item => {
            const stats = item.stats;
            const gameId = item.id;
            const game = gameMap.get(gameId);

            if (!stats || !game) {
                return;
            }

            // Determine which side(s) to aggregate if teamFilter is set
            let sides = ['away', 'home'];
            if (teamFilter) {
                sides = [];
                if (game.away === teamFilter) {
                    sides.push('away');
                }
                if (game.home === teamFilter) {
                    sides.push('home');
                }
            }

            if (sides.length === 0) {
                return;
            }

            Object.keys(stats.playerStats).forEach(pId => {
                const s = stats.playerStats[pId];
                // If we are filtering by team, only include the player if they played for that team in THIS game
                if (teamFilter && s.team !== teamFilter) {
                    return;
                }

                if (!aggregated.players[pId]) {
                    aggregated.players[pId] = {
                        ab: 0, r: 0, h: 0, rbi: 0, bb: 0, k: 0, hbp: 0, sf: 0, sh: 0,
                        singles: 0, doubles: 0, triples: 0, hr: 0, sb: 0, pa: 0,
                        flyouts: 0, lineouts: 0, groundouts: 0, otherOuts: 0, roe: 0, calledStrikes: 0,
                        games: 0, name: s.name,
                    };
                }
                const p = aggregated.players[pId];
                Object.keys(s).forEach(k => {
                    if (typeof s[k] === 'number') {
                        p[k] += s[k];
                    }
                });
                p.games++;
            });

            Object.keys(stats.pitcherStats).forEach(pId => {
                const s = stats.pitcherStats[pId];
                // If we are filtering by team, only include the pitcher if they pitched for that team in THIS game
                if (teamFilter && s.team !== teamFilter) {
                    return;
                }

                if (!aggregated.pitchers[pId]) {
                    aggregated.pitchers[pId] = {
                        ipOuts: 0, h: 0, bb: 0, er: 0, k: 0, hbp: 0,
                        pitches: 0, strikes: 0, balls: 0, bf: 0,
                        defensiveOuts: 0, errors: 0, games: 0,
                    };
                }
                const p = aggregated.pitchers[pId];
                Object.keys(s).forEach(k => {
                    if (p[k] !== undefined && typeof s[k] === 'number') {
                        p[k] += s[k];
                    }
                });
                p.games++;
            });

            // Aggregate Team Stats
            ['away', 'home'].forEach(t => {
                const teamName = game[t];
                if (teamFilter && teamName !== teamFilter) {
                    return;
                }

                const teamId = game[t + 'TeamId'] || teamName;
                if (teamId) {
                    if (!aggregated.teams[teamId]) {
                        aggregated.teams[teamId] = { w: 0, l: 0, t: 0, rs: 0, ra: 0, games: 0, name: teamName };
                    }
                    const ts = aggregated.teams[teamId];
                    if (teamName && !ts.name) {
                        ts.name = teamName;
                    }
                    // Use pre-calculated score from stats
                    if (stats.score && stats.score[t]) {
                        ts.rs += stats.score[t].R;
                        ts.ra += stats.score[t === 'away' ? 'home' : 'away'].R;
                    }
                    ts.games++;
                    if (game.status === 'final') {
                        if (stats.score[t].R > stats.score[t === 'away' ? 'home' : 'away'].R) {
                            ts.w++;
                        } else if (stats.score[t].R < stats.score[t === 'away' ? 'home' : 'away'].R) {
                            ts.l++;
                        } else {
                            ts.t++;
                        }
                    }
                }
            });
        });
        return aggregated;
    }

    static formatIP(outs) {
        return `${Math.floor(outs / 3)}.${outs % 3}`;
    }

    static getDerivedHittingStats(s) {
        const avg = s.ab > 0 ? s.h / s.ab : 0;
        const obpDenom = s.ab + s.bb + s.hbp + s.sf;
        const obp = obpDenom > 0 ? (s.h + s.bb + s.hbp) / obpDenom : 0;
        const slg = s.ab > 0 ? (s.singles + 2 * s.doubles + 3 * s.triples + 4 * s.hr) / s.ab : 0;
        return {
            avg: avg.toFixed(3),
            obp: obp.toFixed(3),
            slg: slg.toFixed(3),
            ops: (obp + slg).toFixed(3),
        };
    }

    static getDerivedPitchingStats(s, eraInnings = 7) {
        const ip = s.ipOuts / 3;
        const era = ip > 0 ? (s.er * eraInnings) / ip : 0;
        const whip = ip > 0 ? (s.bb + s.h) / ip : 0;
        const totalPitches = (s.pitches || 0) + (s.balls || 0);
        const strikePct = totalPitches > 0 ? (s.strikes / totalPitches) * 100 : 0;
        const walkPct = s.bf > 0 ? (s.bb / s.bf) * 100 : 0;
        const kPct = s.bf > 0 ? (s.k / s.bf) * 100 : 0;

        return {
            era: era.toFixed(2),
            whip: whip.toFixed(2),
            strikePct: strikePct.toFixed(1) + '%',
            walkPct: walkPct.toFixed(1) + '%',
            kPct: kPct.toFixed(1) + '%',
            ip: this.formatIP(s.ipOuts),
        };
    }
}
