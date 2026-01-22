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

import { formatDate, createElement } from '../utils.js';

/**
 * Handles rendering of the Statistics view and Player Profiles.
 */
export class StatsRenderer {
    /**
     * @param {object} options
     * @param {HTMLElement} options.container - The container for statistics content.
     * @param {object} options.callbacks - Callbacks for user interactions.
     * @param {Function} options.callbacks.onOpenPlayerProfile - Called when a player row is clicked.
     * @param {Function} options.callbacks.getDerivedHittingStats - Wrapper for StatsEngine logic.
     * @param {Function} options.callbacks.getDerivedPitchingStats - Wrapper for StatsEngine logic.
     */
    constructor({ container, callbacks }) {
        this.container = container;
        this.callbacks = callbacks;
        this.columnConfig = this.loadColumnConfig();
    }

    loadColumnConfig() {
        const hittingAll = ['G', 'PA', 'AB', 'H', 'R', 'RBI', 'BB', 'K', 'HBP', 'ROE', 'Fly', 'Line', 'Gnd', 'Other', 'AVG', 'OBP', 'SLG', 'OPS', 'HR'];
        const pitchingAll = ['G', 'IP', 'ERA', 'WHIP', 'K', 'K%', 'BB', 'BB%', 'H', 'BF', 'S', 'B', 'PC', 'Str%'];

        const defaults = {
            hitting: ['Player', ...hittingAll],
            pitching: ['Player', ...pitchingAll],
        };
        try {
            const saved = localStorage.getItem('sk_stats_cols');
            return saved ? JSON.parse(saved) : defaults;
        } catch {
            return defaults;
        }
    }

    saveColumnConfig(config) {
        this.columnConfig = config;
        localStorage.setItem('sk_stats_cols', JSON.stringify(config));
    }

    /**
     * Renders the statistics view.
     * @param {object} stats - The stats object (aggregated or single-game).
     * @param {object} [game] - Optional single game object.
     * @param {object} [options] - Rendering options.
     */
    render(stats, game, options = {}) {
        const container = options.container || this.container;
        if (!container) {
            return;
        }

        if (!stats) {
            container.appendChild(createElement('div', {
                className: 'text-center text-gray-500 mt-10',
                text: 'Loading statistics...',
            }));
            return;
        }

        const isPrint = options.isPrint || false;
        container.innerHTML = '';

        if (isPrint && game) {
            this.renderBoxScore(container, stats, game);
            return;
        }

        const exportCont = createElement('div', { className: 'flex justify-end gap-2 mb-4 no-print' });
        const btnCSV = createElement('button', {
            className: 'bg-white hover:bg-gray-50 text-gray-700 px-3 py-1 rounded text-xs font-bold border border-gray-300 flex items-center gap-1',
            text: 'Export CSV',
        });
        btnCSV.onclick = () => this.callbacks.onExportCSV(stats);
        exportCont.appendChild(btnCSV);

        const btnPDF = createElement('button', {
            className: 'bg-white hover:bg-gray-50 text-gray-700 px-3 py-1 rounded text-xs font-bold border border-gray-300 flex items-center gap-1',
            text: 'Export PDF',
        });
        btnPDF.onclick = () => this.callbacks.onExportPDF();
        exportCont.appendChild(btnPDF);

        container.appendChild(exportCont);

        // Support both single-game stats structure and aggregated stats structure
        const players = stats.players || stats.playerStats || {};
        const pitchers = stats.pitchers || stats.pitcherStats || {};
        const teams = stats.teams || {};

        // --- HITTING LEADERBOARD ---
        const hittingSection = document.createElement('section');
        hittingSection.className = 'mb-10 break-after-page';
        hittingSection.appendChild(createElement('h2', {
            className: 'text-xl font-bold mb-4 border-b pb-2 text-gray-800 uppercase tracking-wide',
            text: 'Hitting Leaderboard',
        }));
        const hitTableCont = document.createElement('div');
        hitTableCont.className = 'overflow-x-auto bg-white rounded-xl shadow-sm border border-gray-200';

        const hitList = Object.keys(players)
            .map(id => {
                const p = players[id];
                return { id, ...p, ...this.callbacks.getDerivedHittingStats(p) };
            })
            .filter(p => p.pa > 0)
            .sort((a, b) => b.ops - a.ops || b.avg - a.avg);

        if (hitList.length === 0) {
            hittingSection.appendChild(createElement('p', {
                className: 'text-gray-500 text-sm italic',
                text: 'Not enough data for leaderboard.',
            }));
        } else {
            const table = createElement('table', { className: 'w-full text-left text-sm' });
            const thead = createElement('thead', { className: 'bg-gray-50 text-gray-600 font-bold border-b border-gray-200' });
            const trHead = document.createElement('tr');

            const hittingCols = this.columnConfig.hitting;
            hittingCols.forEach(h => {
                let cls = 'p-3 whitespace-nowrap';
                if (h === 'Player') {
                    cls += ' sticky left-0 bg-gray-50 z-10 border-r border-gray-200';
                }
                trHead.appendChild(createElement('th', { className: cls, text: h }));
            });
            thead.appendChild(trHead);
            table.appendChild(thead);

            const tbody = createElement('tbody', { className: 'divide-y divide-gray-100' });
            hitList.forEach(p => {
                const tr = document.createElement('tr');
                tr.className = 'hover:bg-gray-50 cursor-pointer transition-colors';
                tr.onclick = () => this.callbacks.onOpenPlayerProfile(p.id);

                hittingCols.forEach(col => {
                    let val = '';
                    let cls = 'p-3 text-gray-600';
                    if (col === 'Player') {
                        val = p.name || 'Unknown';
                        cls = 'p-3 font-bold text-gray-900 sticky left-0 bg-white z-10 border-r border-gray-200';
                    } else {
                        switch(col) {
                            case 'G': val = p.games || 1; break;
                            case 'PA': val = p.pa; break;
                            case 'AB': val = p.ab; break;
                            case 'H': val = p.h; break;
                            case 'R': val = p.r; break;
                            case 'RBI': val = p.rbi; break;
                            case 'BB': val = p.bb; break;
                            case 'K': val = p.k; break;
                            case 'HBP': val = p.hbp; break;
                            case 'ROE': val = p.roe; break;
                            case 'Fly': val = p.flyouts; break;
                            case 'Line': val = p.lineouts; break;
                            case 'Gnd': val = p.groundouts; break;
                            case 'Other': val = p.otherOuts; break;
                            case 'AVG': val = p.avg; cls = 'p-3 font-mono text-gray-700'; break;
                            case 'OBP': val = p.obp; cls = 'p-3 font-mono text-gray-700'; break;
                            case 'SLG': val = p.slg; cls = 'p-3 font-mono text-gray-700'; break;
                            case 'OPS': val = p.ops; cls = 'p-3 font-mono font-bold text-blue-600'; break;
                            case 'HR': val = p.hr; break;
                        }
                    }
                    tr.appendChild(createElement('td', { className: cls, text: String(val) }));
                });

                tbody.appendChild(tr);
            });
            table.appendChild(tbody);
            hitTableCont.appendChild(table);
            hittingSection.appendChild(hitTableCont);
        }
        container.appendChild(hittingSection);

        // --- PITCHING LEADERBOARD ---

        const pitchingSection = document.createElement('section');

        pitchingSection.className = 'mb-10 break-after-page';


        pitchingSection.appendChild(createElement('h2', {
            className: 'text-xl font-bold mb-4 border-b pb-2 text-gray-800 uppercase tracking-wide',
            text: 'Pitching Leaderboard',
        }));
        const pitchTableCont = document.createElement('div');
        pitchTableCont.className = 'overflow-x-auto bg-white rounded-xl shadow-sm border border-gray-200';

        const pitchList = Object.keys(pitchers)
            .map(id => {
                const p = pitchers[id];
                return { id, ...p, ...this.callbacks.getDerivedPitchingStats(p) };
            })
            .filter(p => p.bf > 0)
            .sort((a, b) => a.era - b.era);

        if (pitchList.length === 0) {
            pitchingSection.appendChild(createElement('p', {
                className: 'text-gray-500 text-sm italic',
                text: 'Not enough data for leaderboard.',
            }));
        } else {
            const table = createElement('table', { className: 'w-full text-left text-sm' });
            const thead = createElement('thead', { className: 'bg-gray-50 text-gray-600 font-bold border-b border-gray-200' });
            const trHead = document.createElement('tr');

            const pitchingCols = this.columnConfig.pitching;
            pitchingCols.forEach(h => {
                let cls = 'p-3 whitespace-nowrap';
                if (h === 'Player') {
                    cls += ' sticky left-0 bg-gray-50 z-10 border-r border-gray-200';
                }
                trHead.appendChild(createElement('th', { className: cls, text: h }));
            });
            thead.appendChild(trHead);
            table.appendChild(thead);

            const tbody = createElement('tbody', { className: 'divide-y divide-gray-100' });
            pitchList.forEach(p => {
                const tr = document.createElement('tr');
                tr.className = 'hover:bg-gray-50 cursor-pointer transition-colors';
                tr.onclick = () => this.callbacks.onOpenPlayerProfile(p.id);

                pitchingCols.forEach(col => {
                    let val = '';
                    let cls = 'p-3 text-gray-600';
                    if (col === 'Player') {
                        val = p.name || 'Unknown';
                        cls = 'p-3 font-bold text-gray-900 sticky left-0 bg-white z-10 border-r border-gray-200';
                    } else {
                        switch(col) {
                            case 'G': val = p.games || 1; break;
                            case 'IP': val = p.ip; cls = 'p-3 font-mono text-gray-700'; break;
                            case 'ERA': val = p.era; cls = 'p-3 font-mono font-bold text-red-600'; break;
                            case 'WHIP': val = p.whip; cls = 'p-3 font-mono text-gray-700'; break;
                            case 'K': val = p.k; break;
                            case 'K%': val = p.kPct; cls = 'p-3 font-mono'; break;
                            case 'BB': val = p.bb; break;
                            case 'BB%': val = p.walkPct; cls = 'p-3 font-mono'; break;
                            case 'H': val = p.h; break;
                            case 'BF': val = p.bf; break;
                            case 'S': val = p.strikes; break;
                            case 'B': val = p.balls; break;
                            case 'PC': val = (p.pitches || 0) + (p.balls || 0); break;
                            case 'Str%': val = p.strikePct; cls = 'p-3 font-mono'; break;
                        }
                    }
                    tr.appendChild(createElement('td', { className: cls, text: String(val) }));
                });

                tbody.appendChild(tr);
            });
            table.appendChild(tbody);
            pitchTableCont.appendChild(table);
            pitchingSection.appendChild(pitchTableCont);
        }
        container.appendChild(pitchingSection);

        // --- TEAM STANDINGS ---

        const teamSection = document.createElement('section');

        teamSection.className = 'mb-10 break-after-page';


        teamSection.appendChild(createElement('h2', {
            className: 'text-xl font-bold mb-4 border-b pb-2 text-gray-800 uppercase tracking-wide',
            text: 'Team Performance',
        }));
        const teamTableCont = document.createElement('div');
        teamTableCont.className = 'overflow-x-auto bg-white rounded-xl shadow-sm border border-gray-200';

        const teamList = Object.values(teams).sort((a, b) => (b.w / (b.games || 1)) - (a.w / (a.games || 1)));

        if (teamList.length === 0) {
            teamSection.appendChild(createElement('p', {
                className: 'text-gray-500 text-sm italic',
                text: 'No team data found.',
            }));
        } else {
            const table = createElement('table', { className: 'w-full text-left text-sm' });
            const thead = createElement('thead', { className: 'bg-gray-50 text-gray-600 font-bold border-b border-gray-200' });
            const trHead = document.createElement('tr');
            ['Team', 'G', 'W-L-T', 'RS', 'RA', 'DIFF'].forEach((h, idx) => {
                const align = (idx > 0) ? 'text-center' : '';
                trHead.appendChild(createElement('th', { className: `p-3 ${align}`, text: h }));
            });
            thead.appendChild(trHead);
            table.appendChild(thead);

            const tbody = createElement('tbody', { className: 'divide-y divide-gray-100' });
            teamList.forEach(t => {
                const tr = document.createElement('tr');
                const diff = t.rs - t.ra;

                tr.appendChild(createElement('td', { className: 'p-3 font-bold text-gray-900', text: t.name }));
                tr.appendChild(createElement('td', { className: 'p-3 text-center text-gray-600', text: String(t.games) }));
                tr.appendChild(createElement('td', { className: 'p-3 text-center font-mono text-gray-700', text: `${t.w}-${t.l}-${t.t}` }));
                tr.appendChild(createElement('td', { className: 'p-3 text-center text-gray-600', text: String(t.rs) }));
                tr.appendChild(createElement('td', { className: 'p-3 text-center text-gray-600', text: String(t.ra) }));

                const diffCls = `p-3 text-center font-bold ${diff >= 0 ? 'text-green-600' : 'text-red-600'}`;
                tr.appendChild(createElement('td', { className: diffCls, text: `${diff > 0 ? '+' : ''}${diff}` }));

                tbody.appendChild(tr);
            });
            table.appendChild(tbody);
            teamTableCont.appendChild(table);
            teamSection.appendChild(teamTableCont);
        }
        container.appendChild(teamSection);

    }

    renderColumnSelector() {
        const modal = document.getElementById('stats-columns-modal');
        if (!modal) {
            return;
        }

        const hittingAll = ['G', 'PA', 'AB', 'H', 'R', 'RBI', 'BB', 'K', 'HBP', 'ROE', 'Fly', 'Line', 'Gnd', 'AVG', 'OBP', 'SLG', 'OPS', 'HR'];
        const pitchingAll = ['G', 'IP', 'ERA', 'WHIP', 'K', 'K%', 'BB', 'BB%', 'H', 'BF', 'S', 'B', 'PC', 'Str%'];

        const renderSection = (containerId, columns, current) => {
            const cont = document.getElementById(containerId);
            if (!cont) {
                return;
            }
            cont.innerHTML = '';
            columns.forEach(col => {
                const label = createElement('label', { className: 'flex items-center gap-2 p-2 hover:bg-gray-50 rounded cursor-pointer text-sm' });
                const cb = createElement('input', {
                    type: 'checkbox',
                    className: 'w-4 h-4 rounded text-blue-600',
                    value: col,
                    checked: current.includes(col),
                });
                label.appendChild(cb);
                label.appendChild(document.createTextNode(col));
                cont.appendChild(label);
            });
        };

        renderSection('stats-cols-hitting', hittingAll, this.columnConfig.hitting);
        renderSection('stats-cols-pitching', pitchingAll, this.columnConfig.pitching);

        modal.classList.remove('hidden');
    }

    /**
     * Renders the player profile modal.
     */
    renderPlayerProfile(playerId, stats, games, playerInfo = null) {
        const modal = document.getElementById('player-profile-modal');
        if (!modal) {
            return;
        }

        const p = (stats && stats.players && stats.players[playerId]) ? stats.players[playerId] : {
            ab: 0, r: 0, h: 0, rbi: 0, bb: 0, k: 0, hbp: 0, sf: 0, sh: 0,
            singles: 0, doubles: 0, triples: 0, hr: 0, sb: 0, pa: 0,
            flyouts: 0, lineouts: 0, groundouts: 0, otherOuts: 0, roe: 0, calledStrikes: 0,
            games: 0,
            name: playerInfo ? playerInfo.name : 'Unknown',
            number: playerInfo ? playerInfo.number : '',
        };
        const derived = this.callbacks.getDerivedHittingStats(p);

        document.getElementById('profile-name').textContent = p.name;
        document.getElementById('profile-subtitle').textContent = `${p.games} Games • ${p.pa} PA • ${p.h} Hits • ${p.hr} HR`;

        // Big Stats
        const card = document.getElementById('profile-stats-card');
        card.innerHTML = '';

        // Clear previous breakdowns if any
        const existingBreakdowns = card.parentElement.querySelectorAll('.profile-breakdown');
        existingBreakdowns.forEach(el => el.remove());

        [
            { l: 'AVG', v: derived.avg },
            { l: 'OPS', v: derived.ops },
            { l: 'Hits', v: p.h },
            { l: 'HR', v: p.hr },
            { l: 'RBI', v: p.rbi },
            { l: 'AB', v: p.ab },
        ].forEach(s => {
            const div = createElement('div', { className: 'text-center' });
            div.appendChild(createElement('div', { className: 'text-xs text-gray-500 uppercase', text: s.l }));
            div.appendChild(createElement('div', { className: 'text-xl font-bold', text: String(s.v) }));
            card.appendChild(div);
        });

        // Detailed Hitting Breakdown
        const hittingBreakdown = createElement('div', { className: 'mt-6 profile-breakdown' });
        hittingBreakdown.appendChild(createElement('h4', { className: 'text-sm font-bold text-gray-400 uppercase tracking-widest mb-2', text: 'Hitting Breakdown' }));
        const hGrid = createElement('div', { className: 'grid grid-cols-3 gap-2' });
        const hItems = [
            { l: 'K', v: p.k },
            { l: 'BB', v: p.bb },
            { l: 'HBP', v: p.hbp },
            { l: 'ROE', v: p.roe },
            { l: 'Fly', v: p.flyouts },
            { l: 'Line', v: p.lineouts },
            { l: 'Gnd', v: p.groundouts },
            { l: 'Other', v: p.otherOuts },
            { l: 'CS', v: p.calledStrikes, t: 'Called Strikes' },
            { l: 'PA', v: p.pa },
            { l: 'AB', v: p.ab },
            { l: 'R', v: p.r },
            { l: 'RBI', v: p.rbi },
            { l: 'HR', v: p.hr },
        ];
        hItems.filter(item => this.columnConfig.hitting.includes(item.l)).forEach(s => {
            const div = createElement('div', { className: 'bg-white p-2 rounded border border-gray-100 text-center', title: s.t || '' });
            div.appendChild(createElement('div', { className: 'text-[10px] text-gray-400 uppercase', text: s.l }));
            div.appendChild(createElement('div', { className: 'text-sm font-bold text-gray-700', text: String(s.v) }));
            hGrid.appendChild(div);
        });
        if (hGrid.children.length > 0) {
            hittingBreakdown.appendChild(hGrid);
            card.parentElement.appendChild(hittingBreakdown);
        }

        // Pitching Breakdown (if applicable)
        const pitchingData = stats ? (stats.pitchers || stats.pitcherStats || {}) : {};
        const pitchStats = pitchingData[playerId];
        if (pitchStats && pitchStats.bf > 0) {
            const dps = this.callbacks.getDerivedPitchingStats(pitchStats);
            const pitchingBreakdown = createElement('div', { className: 'mt-6 pt-6 border-t border-gray-200 profile-breakdown' });
            pitchingBreakdown.appendChild(createElement('h4', { className: 'text-sm font-bold text-gray-400 uppercase tracking-widest mb-2', text: 'Pitching Performance' }));

            const pGridBig = createElement('div', { className: 'grid grid-cols-3 gap-4 mb-4' });
            [
                { l: 'IP', v: dps.ip },
                { l: 'ERA', v: dps.era },
                { l: 'WHIP', v: dps.whip },
            ].filter(item => this.columnConfig.pitching.includes(item.l)).forEach(s => {
                const div = createElement('div', { className: 'text-center' });
                div.appendChild(createElement('div', { className: 'text-[10px] text-gray-500 uppercase', text: s.l }));
                div.appendChild(createElement('div', { className: 'text-lg font-bold text-gray-800', text: String(s.v) }));
                pGridBig.appendChild(div);
            });
            if (pGridBig.children.length > 0) {
                pitchingBreakdown.appendChild(pGridBig);
            }

            const pGridSmall = createElement('div', { className: 'grid grid-cols-4 gap-2' });
            const pItems = [
                { l: 'K', v: pitchStats.k },
                { l: 'BB', v: pitchStats.bb },
                { l: 'H', v: pitchStats.h },
                { l: 'HBP', v: pitchStats.hbp },
                { l: 'BF', v: pitchStats.bf },
                { l: 'DO', v: pitchStats.defensiveOuts, t: 'Defensive Outs' },
                { l: 'E', v: pitchStats.errors, t: 'Pitcher Errors' },
                { l: 'S', v: pitchStats.strikes },
                { l: 'B', v: pitchStats.balls },
                { l: 'PC', v: (pitchStats.pitches || 0) + (pitchStats.balls || 0), t: 'Pitch Count' },
                { l: 'Str%', v: dps.strikePct },
                { l: 'K%', v: dps.kPct },
                { l: 'BB%', v: dps.walkPct },
            ];
            pItems.filter(item => this.columnConfig.pitching.includes(item.l)).forEach(s => {
                const div = createElement('div', { className: 'bg-white p-2 rounded border border-gray-100 text-center', title: s.t || '' });
                div.appendChild(createElement('div', { className: 'text-[10px] text-gray-400 uppercase', text: s.l }));
                div.appendChild(createElement('div', { className: 'text-sm font-bold text-gray-700', text: String(s.v) }));
                pGridSmall.appendChild(div);
            });
            if (pGridSmall.children.length > 0) {
                pitchingBreakdown.appendChild(pGridSmall);
            }
            if (pitchingBreakdown.children.length > 1) {
                card.parentElement.appendChild(pitchingBreakdown);
            }
        }
        // Table
        const tbody = document.getElementById('profile-game-log');
        tbody.innerHTML = '';

        // Filter and sort games for this player
        const playerGames = games.filter(g => {
            const gameStats = this.callbacks.calculateGameStats(g);
            return gameStats.playerStats[playerId] !== undefined;
        }).sort((a, b) => new Date(b.date) - new Date(a.date));

        playerGames.forEach(g => {
            const gs = this.callbacks.calculateGameStats(g);
            const ps = gs.playerStats[playerId];
            const tr = document.createElement('tr');

            tr.appendChild(createElement('td', { className: 'p-2 text-gray-700 whitespace-nowrap', text: formatDate(g.date) }));
            tr.appendChild(createElement('td', { className: 'p-2 font-bold', text: `${g.away} @ ${g.home}` }));
            tr.appendChild(createElement('td', { className: 'p-2 text-center', text: `${ps.ab} AB, ${ps.h} H` }));

            tbody.appendChild(tr);
        });

        // Spray Chart logic
        const sprayGroup = document.getElementById('spray-markers');
        if (sprayGroup) {
            sprayGroup.innerHTML = '';
            const SVG_NS = 'http://www.w3.org/2000/svg';
            playerGames.forEach(g => {
                Object.keys(g.events).forEach(k => {
                    const evt = g.events[k];
                    if (evt.pId === playerId && evt.hitData && evt.hitData.location) {
                        const hitX = (evt.hitData.location.x - 0.5) * 200 + 100;
                        const hitY = (evt.hitData.location.y - 0.85) * 200 + 185;
                        const homeX = 100, homeY = 185;

                        let pathEl;
                        if (evt.hitData.trajectory === 'Fly' || evt.hitData.trajectory === 'Pop') {
                            pathEl = document.createElementNS(SVG_NS, 'path');
                            const midX = (homeX + hitX) / 2;
                            const midY = (homeY + hitY) / 2;
                            const controlX = midX;
                            const controlY = midY - 90;
                            pathEl.setAttribute('d', `M ${homeX} ${homeY} Q ${controlX} ${controlY} ${hitX} ${hitY}`);
                            pathEl.setAttribute('fill', 'none');
                        } else {
                            pathEl = document.createElementNS(SVG_NS, 'line');
                            pathEl.setAttribute('x1', homeX); pathEl.setAttribute('y1', homeY);
                            pathEl.setAttribute('x2', hitX); pathEl.setAttribute('y2', hitY);
                        }
                        pathEl.setAttribute('stroke', 'rgba(255,255,255,0.2)');
                        pathEl.setAttribute('stroke-width', '1');
                        sprayGroup.appendChild(pathEl);

                        const marker = document.createElementNS(SVG_NS, 'circle');
                        marker.setAttribute('cx', hitX);
                        marker.setAttribute('cy', hitY);
                        marker.setAttribute('r', '3');
                        marker.setAttribute('fill', evt.outcome === 'HR' ? '#fbbf24' : '#4ade80');
                        sprayGroup.appendChild(marker);
                    }
                });
            });
        }

        modal.classList.remove('hidden');
    }

    /**
     * Renders a concise box score for the print view.
     */
    renderBoxScore(container, stats, game) {
        ['away', 'home'].forEach(side => {
            this.renderTeamBoxScore(container, stats, game, side);
        });
    }

    /**
     * Renders box score for a specific team.
     */
    renderTeamBoxScore(container, stats, game, side) {
        const createTable = (title, headers, rows) => {
            const section = document.createElement('div');
            section.className = 'mb-8';
            section.appendChild(createElement('h3', { className: 'text-lg font-bold mb-2 uppercase border-b border-slate-300 pb-1', text: title }));

            const table = createElement('table', { className: 'w-full text-left text-xs border-collapse' });
            const thead = createElement('thead', { className: 'bg-slate-50 border-b border-slate-300' });
            const trHead = document.createElement('tr');
            headers.forEach(h => trHead.appendChild(createElement('th', { className: 'p-2 font-bold', text: h })));
            thead.appendChild(trHead);
            table.appendChild(thead);

            const tbody = createElement('tbody', { className: 'divide-y divide-slate-100' });
            rows.forEach(row => {
                const tr = document.createElement('tr');
                row.forEach((cell, idx) => {
                    tr.appendChild(createElement('td', {
                        className: `p-2 ${idx > 0 ? 'font-mono' : 'font-bold'}`,
                        text: String(cell),
                    }));
                });
                tbody.appendChild(tr);
            });
            table.appendChild(tbody);
            section.appendChild(table);
            return section;
        };

        const hittingHeaders = this.columnConfig.hitting;
        const pitchingHeaders = this.columnConfig.pitching;

        const teamName = side === 'away' ? game.away : game.home;

        const getHittingRowValue = (ps, derived, col) => {
            switch(col) {
                case 'Player': return ps.name || 'Unknown';
                case 'G': return 1;
                case 'PA': return ps.pa;
                case 'AB': return ps.ab;
                case 'H': return ps.h;
                case 'R': return ps.r;
                case 'RBI': return ps.rbi;
                case 'BB': return ps.bb;
                case 'K': return ps.k;
                case 'HBP': return ps.hbp;
                case 'ROE': return ps.roe;
                case 'Fly': return ps.flyouts;
                case 'Line': return ps.lineouts;
                case 'Gnd': return ps.groundouts;
                case 'Other': return ps.otherOuts;
                case 'AVG': return derived.avg;
                case 'OBP': return derived.obp;
                case 'SLG': return derived.slg;
                case 'OPS': return derived.ops;
                case 'HR': return ps.hr;
                default: return '';
            }
        };

        const getPitchingRowValue = (ps, dps, col) => {
            switch(col) {
                case 'Player': return ps.name || 'Unknown';
                case 'G': return 1;
                case 'IP': return dps.ip;
                case 'ERA': return dps.era;
                case 'WHIP': return dps.whip;
                case 'K': return ps.k;
                case 'K%': return dps.kPct;
                case 'BB': return ps.bb;
                case 'BB%': return dps.walkPct;
                case 'H': return ps.h;
                case 'BF': return ps.bf;
                case 'S': return ps.strikes;
                case 'B': return ps.balls;
                case 'PC': return (ps.pitches || 0) + (ps.balls || 0);
                case 'Str%': return dps.strikePct;
                default: return '';
            }
        };

        // Hitting
        const hittingRows = game.roster[side].map(slot => {
            const p = slot.starter;
            const ps = stats.playerStats[p.id] || {
                ab: 0, r: 0, h: 0, rbi: 0, bb: 0, k: 0, hbp: 0, roe: 0, pa: 0, singles: 0, doubles: 0, triples: 0, hr: 0, sf: 0, sh: 0,
                flyouts: 0, lineouts: 0, groundouts: 0, otherOuts: 0, calledStrikes: 0, name: p.name,
            };
            const derived = this.callbacks.getDerivedHittingStats(ps);
            return hittingHeaders.map(col => getHittingRowValue(ps, derived, col));
        });
        container.appendChild(createTable(`${teamName} Hitting`, hittingHeaders, hittingRows));

        // Pitching
        const defenseSide = side === 'away' ? 'home' : 'away';
        const pitcherIds = Object.keys(stats.pitcherStats).filter(id => {
            return stats.pitcherStats[id].bf > 0 &&
                   game.roster[defenseSide].some(slot => slot.starter.id === id || (slot.history && slot.history.some(h => h.id === id)));
        });

        const pitchingRows = pitcherIds.map(id => {
            const ps = stats.pitcherStats[id];
            const dps = this.callbacks.getDerivedPitchingStats(ps);
            // Find name
            let name = 'Unknown';
            const p = [...game.roster.away, ...game.roster.home].find(s => s.starter.id === id || (s.history && s.history.some(h => h.id === id)));
            if (p) {
                name = p.starter.name;
            }
            ps.name = name;

            return pitchingHeaders.map(col => getPitchingRowValue(ps, dps, col));
        });
        if (pitchingRows.length > 0) {
            container.appendChild(createTable(`${side === 'away' ? game.home : game.away} Pitching`, pitchingHeaders, pitchingRows));
        }
    }
}
