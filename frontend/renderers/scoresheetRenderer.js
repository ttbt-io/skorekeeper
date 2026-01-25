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
import {
    BiPResultFly,
    BiPResultLine,
    BiPResultPop,
    BiPResultOut,
    BiPResultIFF,
} from '../constants.js';

/**
 * Handles rendering of the active game Scoresheet (Grid, Feed, Scoreboard).
 */
export class ScoresheetRenderer {
    /**
     * @param {object} options
     * @param {HTMLElement} options.gridContainer - The element for the PA grid.
     * @param {HTMLElement} options.scoreboardContainer - The element for the scoreboard.
     * @param {HTMLElement} options.feedContainer - The element for the narrative feed.
     * @param {object} options.callbacks - Callbacks for user interactions.
     */
    constructor({ gridContainer, scoreboardContainer, feedContainer, callbacks }) {
        this.gridContainer = gridContainer;
        this.scoreboardContainer = scoreboardContainer;
        this.feedContainer = feedContainer;
        this.callbacks = callbacks;

        this.gridPathCoords = [
            { x1: 30, y1: 55, x2: 44, y2: 42 }, // Home to 1st
            { x1: 44, y1: 42, x2: 30, y2: 28 }, // 1st to 2nd
            { x1: 30, y1: 28, x2: 16, y2: 42 }, // 2nd to 3rd
            { x1: 16, y1: 42, x2: 30, y2: 55 }, // 3rd to Home
        ];
    }

    /**
     * Renders the grid view.
     */
    renderGrid(game, team, stats, activeCtx, options = {}) {
        if (!this.gridContainer || !game) {
            return;
        }

        const isPrint = options.isPrint || false;
        const roster = game.roster[team];
        const visibleColumns = game.columns.filter(c => !c.team || c.team === team);

        // Use fixed widths for print to ensure they fit the page
        const colWidth = isPrint ? '70px' : '75px';
        const statsWidth = isPrint ? '35px' : '40px';
        const newCols = `120px repeat(${visibleColumns.length}, ${colWidth}) repeat(4, ${statsWidth})`;

        const currentColCount = parseInt(this.gridContainer.dataset.colCount || '0');
        const currentRosterLen = parseInt(this.gridContainer.dataset.rosterLen || '0');
        const currentTeam = this.gridContainer.dataset.team || '';
        const currentPrintMode = this.gridContainer.dataset.isPrint === 'true';

        const needsRebuild = (
            this.gridContainer.innerHTML === '' ||
            currentColCount !== visibleColumns.length ||
            currentRosterLen !== roster.length ||
            currentTeam !== team ||
            currentPrintMode !== isPrint
        );

        if (needsRebuild) {
            this.gridContainer.innerHTML = '';
            this.gridContainer.style.gridTemplateColumns = newCols;
            this.gridContainer.dataset.colCount = visibleColumns.length;
            this.gridContainer.dataset.rosterLen = roster.length;
            this.gridContainer.dataset.team = team;
            this.gridContainer.dataset.isPrint = isPrint;
            this.renderGridStructure(game, team, visibleColumns, roster, stats, activeCtx, isPrint);
        } else {
            this.updateGridContent(game, team, visibleColumns, roster, stats, activeCtx, isPrint);
        }
    }

    renderGridStructure(game, team, visibleColumns, roster, stats, activeCtx, isPrint) {
        // Header
        const corner = document.createElement('div');
        corner.className = 'grid-header corner';
        corner.textContent = 'BATTER';
        this.gridContainer.appendChild(corner);

        // Group visible columns by inning
        const inningGroups = [];
        visibleColumns.forEach(col => {
            const lastGroup = inningGroups[inningGroups.length - 1];
            if (lastGroup && lastGroup[0].inning === col.inning) {
                lastGroup.push(col);
            } else {
                inningGroups.push([col]);
            }
        });

        inningGroups.forEach((group) => {
            const h = document.createElement('div');
            h.className = 'grid-header';
            if (group.length > 1) {
                h.style.gridColumn = `span ${group.length}`;
            }

            // Use the last column in the group for interactions (e.g. Remove Column targets the last one)
            const targetCol = group[group.length - 1];

            if (!isPrint) {
                h.className += ' cursor-pointer hover:bg-gray-200';
                h.onclick = e => this.callbacks.onColumnContextMenu(e, targetCol.id, targetCol.inning);
                h.oncontextmenu = (e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    h.onclick(e);
                };
            }
            h.textContent = targetCol.inning;
            this.gridContainer.appendChild(h);
        });

        ['AB', 'R', 'H', 'RBI'].forEach(t => {
            const h = document.createElement('div');
            h.className = 'grid-header text-xs';
            h.textContent = t;
            this.gridContainer.appendChild(h);
        });

        // Rows
        roster.forEach((p, i) => {
            const lc = document.createElement('div');
            lc.className = 'lineup-cell flex flex-col justify-center p-1 border-b border-r border-gray-300 bg-white h-[65px] z-10 sticky left-0 relative';
            if (!isPrint) {
                lc.className += ' cursor-pointer';
                lc.oncontextmenu = (e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    this.callbacks.onPlayerSubstitution(lc, e, team, i);
                };
            }
            lc.dataset.playerIdx = i;
            lc.dataset.team = team;

            this.renderLineupCellContent(lc, p, i);
            this.gridContainer.appendChild(lc);

            visibleColumns.forEach((col) => {
                const key = `${team}-${i}-${col.id}`;
                const cell = document.createElement('div');
                cell.className = 'grid-cell';
                if (!isPrint) {
                    cell.className += ' cursor-pointer';
                    cell.onclick = () => this.callbacks.onCellClick(i, col.inning, col.id);
                    cell.oncontextmenu = (e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        this.callbacks.onCellContextMenu(e, i, col.id, col.inning);
                    };
                }
                cell.dataset.key = key;
                cell.dataset.playerIdx = i;
                cell.dataset.colId = col.id;

                this.gridContainer.appendChild(cell);
            });

            // Stats Cells placeholders
            ['ab', 'r', 'h', 'rbi'].forEach(k => {
                const sCell = document.createElement('div');
                sCell.className = 'grid-cell stats-cell flex items-center justify-center font-mono text-sm bg-gray-50';
                sCell.dataset.statsKey = `${p.starter.id}-${k}`;
                this.gridContainer.appendChild(sCell);
            });
        });

        // Add footer row for totals
        const footerLabel = document.createElement('div');
        footerLabel.className = 'lineup-cell flex items-center justify-center font-bold text-xs bg-gray-200 border-t border-gray-400 sticky left-0 z-10';
        footerLabel.textContent = 'TOTALS';
        this.gridContainer.appendChild(footerLabel);

        visibleColumns.forEach((c) => {
            const cell = document.createElement('div');
            cell.className = 'grid-cell flex flex-col justify-center items-center text-[10px] bg-gray-200 leading-tight border-t border-gray-400';
            cell.dataset.footerCol = c.id;
            this.gridContainer.appendChild(cell);
        });

        for (let j = 0; j < 4; j++) {
            const cell = document.createElement('div');
            cell.className = 'grid-cell bg-gray-300 border-t border-gray-400';
            this.gridContainer.appendChild(cell);
        }


        // Populate content
        this.updateGridContent(game, team, visibleColumns, roster, stats, activeCtx, isPrint);
    }

    updateGridContent(game, team, visibleColumns, roster, stats, activeCtx, isPrint) {
        roster.forEach((p, i) => {
            // Update Lineup Cell (for substitutions)
            const lc = this.gridContainer.querySelector(`.lineup-cell[data-player-idx="${i}"][data-team="${team}"]`);
            if (lc) {
                this.renderLineupCellContent(lc, p, i);
            }

            let lastPId = p.starter.id || null;
            visibleColumns.forEach(col => {
                const key = `${team}-${i}-${col.id}`;
                const d = game.events[key] || { balls: 0, strikes: 0, outcome: '', paths: [0, 0, 0, 0], pathInfo: ['', '', '', ''] };
                const cell = this.gridContainer.querySelector(`.grid-cell[data-key="${key}"]`);

                if (cell) {
                    let cellClasses = 'grid-cell';
                    if (!isPrint) {
                        cellClasses += ' cursor-pointer';
                    }
                    if (d.pId) {
                        if (lastPId && d.pId !== lastPId) {
                            cellClasses += ' border-l-4 border-gray-900';
                        }
                        lastPId = d.pId;
                    }
                    if (d.outcome) {
                        cellClasses += ' bg-yellow-50';
                    }
                    const isActive = !isPrint && activeCtx && activeCtx.b === i && activeCtx.col === col.id;
                    if (isActive) {
                        cellClasses += ' active-pa';
                    }
                    cell.className = cellClasses;

                    if (col.leadRow && col.leadRow[team] === i) {
                        cell.style.borderTop = '4px double black';
                    } else {
                        cell.style.borderTop = '';
                    }

                    this.renderCellContent(cell, d);
                }
            });

            // Collect all unique players for this slot (Starter + Subs)
            const playerList = [];
            const seenIds = new Set();

            const candidates = [...(p.history || []), p.current];
            candidates.forEach(c => {
                if (c && c.id && !seenIds.has(c.id)) {
                    playerList.push(c);
                    seenIds.add(c.id);
                }
            });

            ['ab', 'r', 'h', 'rbi'].forEach(k => {
                const sCell = this.gridContainer.querySelector(`div[data-stats-key="${p.starter.id}-${k}"]`);
                if (sCell) {
                    sCell.innerHTML = '';
                    // Use text-sm for single player (original), text-[10px] for multiple to fit
                    const fontSize = playerList.length > 1 ? 'text-[10px]' : 'text-sm';
                    sCell.classList.remove('items-center', 'text-sm', 'text-xs');
                    sCell.classList.add('flex-col', 'justify-center', 'items-center', fontSize, 'leading-tight');

                    playerList.forEach((player, idx) => {
                        const s = stats.playerStats[player.id];
                        const val = s ? (s[k] !== undefined ? s[k] : 0) : 0;

                        const div = document.createElement('div');
                        div.textContent = val;
                        if (idx === 0) {
                            div.className = 'font-bold text-gray-900';
                        } else {
                            div.className = 'text-indigo-700 font-medium';
                        }
                        sCell.appendChild(div);
                    });
                }
            });
        });

        visibleColumns.forEach((c) => {
            const is = stats.inningStats[`${team}-${c.id}`] || { r: 0, h: 0, e: 0, lob: 0 };
            // For errors, we want to show the errors committed by the defense (the other team) during this half-inning.
            const otherTeam = team === 'away' ? 'home' : 'away';
            const defenseStats = stats.inningStats[`${otherTeam}-${c.id}`] || { e: 0 };

            const cell = this.gridContainer.querySelector(`div[data-footer-col="${c.id}"]`);
            if (cell) {
                cell.innerHTML = '';
                const runs = document.createElement('div');
                runs.textContent = `R:${is.r} H:${is.h}`;
                const extras = document.createElement('div');
                extras.textContent = `E:${defenseStats.e} L:${is.lob}`;
                cell.appendChild(runs);
                cell.appendChild(extras);
            }
        });
    }

    renderCellContent(cell, d) {
        cell.innerHTML = ''; // Clear only content
        const SVG_NS = 'http://www.w3.org/2000/svg';
        const svg = document.createElementNS(SVG_NS, 'svg');
        svg.classList.add('batter-cell-svg');
        svg.setAttribute('viewBox', '0 0 60 60');

        const xElementsToAppend = [];

        if (d.paths && d.paths[3] === 1) { // Run Scored
            const fieldPoly = document.createElementNS(SVG_NS, 'polygon');
            fieldPoly.setAttribute('points', '30,28 44,42 30,55 16,42');
            fieldPoly.setAttribute('class', 'run-scored-diamond');
            svg.appendChild(fieldPoly);
        }

        // Foul Lines
        const foulLines = [
            { x1: 30, y1: 55, x2: 60, y2: 25 },
            { x1: 30, y1: 55, x2: 0, y2: 25 },
        ];
        foulLines.forEach(l => {
            const line = document.createElementNS(SVG_NS, 'line');
            line.setAttribute('x1', l.x1); line.setAttribute('y1', l.y1);
            line.setAttribute('x2', l.x2); line.setAttribute('y2', l.y2);
            line.setAttribute('stroke', '#cbd5e1');
            line.setAttribute('stroke-width', '1');
            svg.appendChild(line);
        });

        // Outfield Fence Arc
        const fenceArc = document.createElementNS(SVG_NS, 'path');
        fenceArc.setAttribute('d', 'M 0,25 A 34.2,34.2 0 0 1 60,25');
        fenceArc.setAttribute('stroke', '#cbd5e1');
        fenceArc.setAttribute('stroke-width', '1');
        fenceArc.setAttribute('fill', 'none');
        svg.appendChild(fenceArc);

        // Paths
        for (let idx = 0; idx < 4; idx++) {
            const line = document.createElementNS(SVG_NS, 'line');
            let pathSegmentClasses = ['base-path-segment'];

            let shouldFill = false;
            const isOccupiedOrOut = d.paths && (d.paths[idx] === 1 || d.paths[idx] === 2);
            const isPlace = d.pathInfo && d.pathInfo[idx] && d.pathInfo[idx].includes('Place');

            if (isOccupiedOrOut && !isPlace) {
                if (idx === 0) {
                    shouldFill = !d.pathInfo[idx] || !d.pathInfo[idx].includes('Place');
                } else {
                    shouldFill = d.paths[idx - 1] === 1;
                }
            }

            if (shouldFill) {
                pathSegmentClasses.push('filled-black');
            }
            line.classList.add(...pathSegmentClasses);
            line.setAttribute('x1', this.gridPathCoords[idx].x1);
            line.setAttribute('y1', this.gridPathCoords[idx].y1);
            line.setAttribute('x2', this.gridPathCoords[idx].x2);
            line.setAttribute('y2', this.gridPathCoords[idx].y2);

            if (d.paths && d.paths[idx] === 2) {
                const t = d.outPos ? d.outPos[idx] : 0.5;
                const lineX2 = this.gridPathCoords[idx].x1 + t * (this.gridPathCoords[idx].x2 - this.gridPathCoords[idx].x1);
                const lineY2 = this.gridPathCoords[idx].y1 + t * (this.gridPathCoords[idx].y2 - this.gridPathCoords[idx].y1);
                line.setAttribute('x2', lineX2);
                line.setAttribute('y2', lineY2);

                const textX = document.createElementNS(SVG_NS, 'text');
                textX.classList.add('path-x-grid');
                textX.setAttribute('x', lineX2);
                textX.setAttribute('y', lineY2);
                textX.textContent = 'X';
                xElementsToAppend.push(textX);
            }
            svg.appendChild(line);
        }

        // Bases
        const baseCoords = [{ x: 41, y: 39 }, { x: 27, y: 25 }, { x: 13, y: 39 }, { x: 27, y: 52 }];
        baseCoords.forEach((pt, idx) => {
            if (idx === 3) { // Home plate
                const diamondCenterX = pt.x + 3;
                const diamondCenterY = pt.y + 3;
                const points = `${diamondCenterX - 3},${diamondCenterY - 3} ${diamondCenterX + 3},${diamondCenterY - 3} ${diamondCenterX + 3},${diamondCenterY} ${diamondCenterX},${diamondCenterY + 3} ${diamondCenterX - 3},${diamondCenterY}`;
                const polygon = document.createElementNS(SVG_NS, 'polygon');
                polygon.setAttribute('points', points);
                polygon.classList.add('base-rect');
                if (d.paths && d.paths[idx] === 1) {
                    polygon.classList.add('base-filled');
                }
                svg.appendChild(polygon);
            } else {
                const rect = document.createElementNS(SVG_NS, 'rect');
                rect.setAttribute('x', pt.x); rect.setAttribute('y', pt.y);
                rect.setAttribute('width', '6'); rect.setAttribute('height', '6');
                rect.setAttribute('transform', `rotate(45 ${pt.x + 3} ${pt.y + 3})`);
                rect.classList.add('base-rect');
                if (d.paths && d.paths[idx] === 1) {
                    rect.classList.add('base-filled');
                }
                svg.appendChild(rect);
            }
        });

        xElementsToAppend.forEach(el => svg.appendChild(el));

        // Path Text Info
        const txtCoords = [{ x: 45, y: 50 }, { x: 45, y: 25 }, { x: 15, y: 25 }, { x: 15, y: 50 }];
        if (d.pathInfo) {
            d.pathInfo.forEach((txt, idx) => {
                if (txt && d.paths && d.paths[idx]) {
                    const textElement = document.createElementNS(SVG_NS, 'text');
                    textElement.classList.add('path-text-grid');
                    textElement.setAttribute('x', txtCoords[idx].x);
                    textElement.setAttribute('y', txtCoords[idx].y);
                    textElement.textContent = txt;
                    svg.appendChild(textElement);
                }
            });
        }

        // RBI Attribution
        if (d.paths && d.paths[3] === 1 && d.scoreInfo && d.scoreInfo.rbiCreditedTo) {
            const creditorId = d.scoreInfo.rbiCreditedTo;
            const creditorNum = this.callbacks.resolvePlayerNumber(creditorId) || '?';
            const rbiText = document.createElementNS(SVG_NS, 'text');
            rbiText.classList.add('rbi-grid-text');
            rbiText.setAttribute('x', '2');
            rbiText.setAttribute('y', '58');
            rbiText.textContent = '#' + creditorNum;
            svg.appendChild(rbiText);
        }

        // Hit Path
        if (d.hitData && d.hitData.location) {
            const homeX = 30, homeY = 55;
            const hitX = (d.hitData.location.x - 0.5) * 60 + 30;
            const hitY = (d.hitData.location.y - 0.85) * 60 + 55;

            let hitPath;
            if (d.hitData.trajectory === BiPResultFly || d.hitData.trajectory === BiPResultPop) {
                const midX = (homeX + hitX) / 2;
                const midY = (homeY + hitY) / 2;
                let controlX = midX;
                // Add curve for straight-away hits
                if (Math.abs(hitX - homeX) < 5) {
                    controlX += 10;
                }
                const controlY = midY - 30;
                hitPath = document.createElementNS(SVG_NS, 'path');
                hitPath.setAttribute('d', `M ${homeX} ${homeY} Q ${controlX} ${controlY} ${hitX} ${hitY}`);
            } else {
                hitPath = document.createElementNS(SVG_NS, 'line');
                hitPath.setAttribute('x1', homeX); hitPath.setAttribute('y1', homeY);
                hitPath.setAttribute('x2', hitX); hitPath.setAttribute('y2', hitY);
            }
            hitPath.classList.add('hit-path', d.hitData.trajectory.toLowerCase());
            svg.appendChild(hitPath);

            const out = d.outcome || '';
            let isAirOut = false;
            if (d.bipState) {
                const { res, type } = d.bipState;
                isAirOut = (
                    res === BiPResultFly || res === BiPResultLine || res === BiPResultPop || res === BiPResultIFF ||
                    (res === BiPResultOut && type === 'SF') ||
                    ((res === 'DP' || res === 'TP') && (type === BiPResultFly || type === BiPResultLine || type === BiPResultPop))
                );
            } else {
                isAirOut = out.startsWith('F') || out.startsWith('L') || out.startsWith('P') || out.startsWith(BiPResultIFF) || out.startsWith('SF');
            }

            if (d.paths && d.paths[0] !== 1 && isAirOut) {
                const textX = document.createElementNS(SVG_NS, 'text');
                textX.classList.add('path-x-grid');
                textX.setAttribute('x', hitX); textX.setAttribute('y', hitY);
                textX.textContent = 'X';
                svg.appendChild(textX);
            }

            const marker = document.createElementNS(SVG_NS, 'circle');
            marker.setAttribute('cx', hitX); marker.setAttribute('cy', hitY);
            marker.setAttribute('r', '2');
            marker.setAttribute('fill', 'black');
            svg.appendChild(marker);
        }

        cell.appendChild(svg);

        // Add content layer for counts/outs
        const contentLayer = document.createElement('div');
        contentLayer.className = 'cell-content-layer';

        const outDisplay = document.createElement('div');
        outDisplay.className = 'out-display';
        if (d.outNum > 0) {
            const outCircle = document.createElement('div');
            outCircle.className = 'out-number-circle';
            outCircle.textContent = d.outNum;
            outDisplay.appendChild(outCircle);
        }

        const countDisplay = document.createElement('div');
        countDisplay.className = 'count-display';

        const bDots = document.createElement('div');
        bDots.className = 'count-dots';
        for (let n = 0; n < 4; n++) {
            const dot = document.createElement('div');
            dot.className = `dot ${n < d.balls ? 'filled-black' : ''}`;
            bDots.appendChild(dot);
        }

        const sDots = document.createElement('div');
        sDots.className = 'count-dots';
        for (let n = 0; n < 3; n++) {
            const dot = document.createElement('div');
            dot.className = `dot ${n < d.strikes ? 'filled-black' : ''}`;
            sDots.appendChild(dot);
        }

        countDisplay.appendChild(bDots);
        countDisplay.appendChild(sDots);

        const outcomeText = document.createElement('div');
        outcomeText.className = 'outcome-text';
        outcomeText.textContent = d.outcome;

        contentLayer.appendChild(outDisplay);
        contentLayer.appendChild(countDisplay);
        contentLayer.appendChild(outcomeText);
        cell.appendChild(contentLayer);
    }

    /**
     * Renders the scoreboard.
     */
    renderScoreboard(game, stats) {
        if (!this.scoreboardContainer || !game) {
            return;
        }

        const titleEl = document.getElementById('header-game-title');
        if (titleEl) {
            let titleText = '';
            if (game.event) {
                titleText += game.event;
            }
            if (game.location) {
                if (titleText) {
                    titleText += ' @ ';
                }
                titleText += game.location;
            }
            if (game.date) {
                const dateStr = formatDate(game.date);
                if (titleText) {
                    titleText += ' - ';
                }
                titleText += dateStr;
            }
            if (!titleText) {
                titleText = `${game.away} vs ${game.home}`;
            }
            titleEl.textContent = titleText;
        }

        document.getElementById('sb-name-away').textContent = game.away;
        document.getElementById('sb-name-home').textContent = game.home;
        document.getElementById('tab-away').textContent = game.away;
        document.getElementById('tab-home').textContent = game.home;

        const uniqueInnings = [...new Set(game.columns.map(c => c.inning))].sort((a, b) => a - b);

        const renderInnings = (id, teamKey) => {
            const row = document.getElementById(id);
            if (!row) {
                return;
            }
            row.innerHTML = '';

            uniqueInnings.forEach(inning => {
                let val = '';
                let cls = 'sb-cell cursor-pointer hover:bg-gray-700 select-none';

                if (game.overrides && game.overrides[teamKey] && game.overrides[teamKey][inning] !== undefined) {
                    val = game.overrides[teamKey][inning];
                    cls += ' text-yellow-400 font-bold underline decoration-dotted';
                } else if (stats.hasAB[teamKey][inning]) {
                    val = stats.innings[teamKey][inning] || 0;
                }

                const cell = document.createElement('div');
                cell.className = cls;
                cell.textContent = val;
                cell.oncontextmenu = (e) => {
                    e.preventDefault();
                    this.callbacks.onScoreOverride(teamKey, inning);
                };
                row.appendChild(cell);
            });
        };

        renderInnings('sb-innings-away', 'away');
        renderInnings('sb-innings-home', 'home');

        ['r', 'h', 'e'].forEach(key => {
            document.getElementById(`sb-${key}-away`).textContent = stats.score.away[key.toUpperCase()] || 0;
            document.getElementById(`sb-${key}-home`).textContent = stats.score.home[key.toUpperCase()] || 0;
        });

        const hDiv = document.getElementById('sb-header-innings');
        if (hDiv) {
            hDiv.innerHTML = '';
            uniqueInnings.forEach(i => {
                const cell = document.createElement('div');
                cell.className = 'sb-cell';
                cell.textContent = i;
                hDiv.appendChild(cell);
            });
        }
    }

    /**
     * Renders a static, print-friendly version of the scoreboard.
     */
    renderPrintScoreboard(container, game, stats) {
        const uniqueInnings = [...new Set(game.columns.map(c => c.inning))].sort((a, b) => a - b);

        const table = createElement('table', { className: 'w-full border-collapse border border-slate-400 text-sm' });
        const thead = createElement('thead', { className: 'bg-slate-50' });
        const trHead = document.createElement('tr');

        trHead.appendChild(createElement('th', { className: 'border border-slate-400 p-2 text-left', text: 'Team' }));
        uniqueInnings.forEach(i => {
            trHead.appendChild(createElement('th', { className: 'border border-slate-400 p-2 text-center w-8', text: String(i) }));
        });
        ['R', 'H', 'E'].forEach(h => {
            trHead.appendChild(createElement('th', { className: 'border border-slate-400 p-2 text-center w-10 font-black', text: h }));
        });
        thead.appendChild(trHead);
        table.appendChild(thead);

        const tbody = document.createElement('tbody');
        ['away', 'home'].forEach(teamKey => {
            const tr = document.createElement('tr');
            tr.appendChild(createElement('td', { className: 'border border-slate-400 p-2 font-bold', text: game[teamKey] }));

            uniqueInnings.forEach(inning => {
                let val = '0';
                if (game.overrides && game.overrides[teamKey] && game.overrides[teamKey][inning] !== undefined) {
                    val = game.overrides[teamKey][inning];
                } else if (stats.hasAB[teamKey][inning]) {
                    val = String(stats.innings[teamKey][inning] || 0);
                } else {
                    val = '-';
                }
                tr.appendChild(createElement('td', { className: 'border border-slate-400 p-2 text-center', text: val }));
            });

            tr.appendChild(createElement('td', { className: 'border border-slate-400 p-2 text-center font-black bg-slate-50', text: String(stats.score[teamKey].R) }));
            tr.appendChild(createElement('td', { className: 'border border-slate-400 p-2 text-center font-bold', text: String(stats.score[teamKey].H) }));
            tr.appendChild(createElement('td', { className: 'border border-slate-400 p-2 text-center', text: String(stats.score[teamKey].E) }));
            tbody.appendChild(tr);
        });
        table.appendChild(tbody);
        container.appendChild(table);
    }

    /**
     * Renders the narrative feed using the structured view-model.
     */
    renderFeed(innings) {
        if (!this.feedContainer) {
            return;
        }

        const wasEmpty = this.feedContainer.innerHTML === '';
        const threshold = 150;
        const wasNearBottom = this.feedContainer.scrollHeight - this.feedContainer.scrollTop - this.feedContainer.clientHeight < threshold;

        // Initial setup if empty
        if (wasEmpty) {
            this.feedContainer.setAttribute('role', 'log');
            this.feedContainer.setAttribute('aria-live', 'polite');
        }

        // Track which IDs we still have to avoid removing them
        const seenIds = new Set();

        // 1. Process Inning Blocks
        innings.forEach(inn => {
            let innDiv = this.feedContainer.querySelector(`div[data-id="${inn.id}"]`);
            if (!innDiv) {
                innDiv = document.createElement('div');
                innDiv.dataset.id = inn.id;
                innDiv.className = 'border-l-2 border-slate-700 pl-4 py-2 mb-4';

                const title = document.createElement('div');
                title.className = 'text-gray-500 text-xs font-bold uppercase mb-4 tracking-widest';
                title.textContent = `${inn.side} ${inn.inning} - ${inn.team}`;
                innDiv.appendChild(title);
                this.feedContainer.appendChild(innDiv);
            }
            seenIds.add(inn.id);

            // 2. Process Items within Inning (PAs, Subs, etc.)
            inn.items.forEach(pa => {
                let paWrapper = innDiv.querySelector(`div[data-id="${pa.id}"]`);
                if (!paWrapper) {
                    paWrapper = document.createElement('div');
                    paWrapper.dataset.id = pa.id;
                    // Find correct insertion point
                    const itemIndex = inn.items.indexOf(pa);
                    if (itemIndex === 0) {
                        innDiv.insertBefore(paWrapper, innDiv.firstChild.nextSibling); // After title
                    } else {
                        const prevId = inn.items[itemIndex - 1].id;
                        const prevEl = innDiv.querySelector(`div[data-id="${prevId}"]`);
                        if (prevEl) {
                            innDiv.insertBefore(paWrapper, prevEl.nextSibling);
                        }
                        else {
                            innDiv.appendChild(paWrapper);
                        }
                    }
                }
                seenIds.add(pa.id);

                // Update Style (Striking) - Reactive Style Update
                const targetClasses = `mb-6 bg-slate-800 rounded p-3 relative shadow-sm border border-slate-700 transition-all duration-300 ${pa.isStricken ? 'line-through opacity-50 grayscale-[0.5]' : ''}`;
                if (paWrapper.className !== targetClasses) {
                    paWrapper.className = targetClasses;
                }

                // Update Content (Header) - Reactive Context Update
                let header = paWrapper.querySelector('.pa-header');
                if (!header) {
                    header = document.createElement('div');
                    header.className = 'pa-header flex justify-between items-start mb-2 border-b border-slate-600 pb-2';
                    paWrapper.appendChild(header);
                }

                let bInfo = header.querySelector('.batter-info');
                if (!bInfo) {
                    bInfo = document.createElement('div');
                    bInfo.className = 'batter-info font-bold text-yellow-400 text-base';
                    header.appendChild(bInfo);
                }
                if (bInfo.textContent !== pa.batterText) {
                    bInfo.textContent = pa.batterText;
                }

                let cInfo = header.querySelector('.context-info');
                if (pa.context) {
                    if (!cInfo) {
                        cInfo = document.createElement('div');
                        cInfo.className = 'context-info text-xs text-gray-400 mt-1 italic';
                        header.appendChild(cInfo);
                    }
                    if (cInfo.textContent !== pa.context) {
                        cInfo.textContent = pa.context;
                    }
                } else if (cInfo) {
                    cInfo.remove();
                }

                // Update Content (Events) - Minimized Visual Churn
                let list = paWrapper.querySelector('.pa-event-list');
                if (!list) {
                    list = document.createElement('div');
                    list.className = 'pa-event-list space-y-1 mb-2 pl-2 text-sm';
                    paWrapper.appendChild(list);
                }

                // Only rebuild events if length changed or it's a correction
                const existingCount = list.querySelectorAll('.event-row').length;
                if (existingCount !== pa.events.length || pa.isCorrection) {
                    list.innerHTML = '';
                    pa.events.forEach(ev => {
                        const row = this.createEventRow(ev);
                        row.classList.add('event-row');
                        list.appendChild(row);
                    });
                }
            });
        });

        // 3. Cleanup removed items
        this.feedContainer.querySelectorAll('div[data-id]').forEach(el => {
            if (!seenIds.has(el.dataset.id)) {
                el.remove();
            }
        });

        // Autoscroll using RAF to ensure layout is updated
        requestAnimationFrame(() => {
            if (wasNearBottom || wasEmpty) {
                this.feedContainer.scrollTop = this.feedContainer.scrollHeight;
            }
        });
    }

    /**
     * Helper to create a single event row.
     */
    createEventRow(ev) {
        const row = document.createElement('div');
        if (ev.type === 'PITCH') {
            row.className = 'text-gray-400 font-mono text-xs';
            row.textContent = `• ${ev.description}`;
        } else if (ev.type === 'RUNNER') {
            row.className = 'text-blue-300 italic';
            row.textContent = `➜ ${ev.description}`;
        } else if (ev.type === 'OUT_COUNT') {
            row.className = 'text-red-400 text-xs font-bold italic pt-1';
            row.textContent = ev.description;
        } else if (ev.type === 'SUMMARY') {
            row.className = 'text-green-400 font-bold mt-2 whitespace-pre-line';
            row.textContent = ev.description;
        } else if (ev.type === 'SUB') {
            row.className = 'text-indigo-400 font-bold text-xs italic mt-1 pl-2 border-l-2 border-indigo-600';
            row.textContent = ev.description;
        } else if (ev.type === 'PLAY') {
            const d = document.createElement('div');
            d.className = 'mt-2 pt-2 border-t border-slate-700 flex justify-between items-start';

            const textCont = document.createElement('div');
            const res = document.createElement('div');
            res.className = 'text-white font-bold text-lg';
            res.textContent = ev.description;
            textCont.appendChild(res);

            if (ev.outcome) {
                const badge = document.createElement('div');
                badge.className = 'bg-yellow-400 text-black text-xs font-black px-2 py-1 rounded shadow-sm ml-4 mt-1 shrink-0';
                badge.textContent = ev.outcome;
                d.appendChild(badge);
            }
            d.prepend(textCont);
            row.appendChild(d);
        }
        return row;
    }


    renderBroadcast(game, stats) {
        const score = stats.score;
        const currentPA = stats.currentPA || { balls: 0, strikes: 0, outs: 0, paths: [0, 0, 0, 0] };

        // Teams & Score
        document.getElementById('bug-away-name').textContent = game.away.substring(0, 3);
        document.getElementById('bug-away-score').textContent = score.away.R;
        document.getElementById('bug-home-name').textContent = game.home.substring(0, 3);
        document.getElementById('bug-home-score').textContent = score.home.R;

        // Inning & Outs
        const side = currentPA.team === 'away' ? '▲' : '▼';
        document.getElementById('bug-inning-arrow').textContent = side;
        document.getElementById('bug-inning-num').textContent = currentPA.inning;

        const out1 = document.getElementById('bug-out-1');
        const out2 = document.getElementById('bug-out-2');
        if (out1) {
            out1.className = `w-2 h-2 rounded-full ${currentPA.outs >= 1 ? 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.8)]' : 'bg-slate-700'}`;
        }
        if (out2) {
            out2.className = `w-2 h-2 rounded-full ${currentPA.outs >= 2 ? 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.8)]' : 'bg-slate-700'}`;
        }

        // Count & Diamond
        document.getElementById('bug-count').textContent = `${currentPA.balls}-${currentPA.strikes}`;

        const setBase = (id, active) => {
            const el = document.getElementById(id);
            if (!el) {
                return;
            }
            el.className = `absolute w-4 h-4 border border-slate-600 ${active ? 'bg-yellow-400 shadow-[0_0_10px_rgba(250,204,21,0.6)]' : 'bg-slate-800'}`;
            if (id === 'bug-base-1') {
                el.classList.add('-top-1', '-right-1');
            }
            if (id === 'bug-base-2') {
                el.classList.add('-top-1', '-left-1');
            }
            if (id === 'bug-base-3') {
                el.classList.add('-bottom-1', '-left-1');
            }
        };

        setBase('bug-base-1', currentPA.paths[0] === 1);
        setBase('bug-base-2', currentPA.paths[1] === 1);
        setBase('bug-base-3', currentPA.paths[2] === 1);
    }

    renderLineupCellContent(lc, p, idx) {
        lc.innerHTML = '';
        const infoDiv = document.createElement('div');
        infoDiv.className = 'flex flex-col overflow-hidden pl-3'; // Added padding for index

        const idxBadge = document.createElement('div');
        idxBadge.className = 'absolute top-0 left-1 text-[10px] text-gray-400 font-mono';
        idxBadge.textContent = (idx !== undefined) ? idx + 1 : '';
        lc.appendChild(idxBadge);

        const nameDiv = document.createElement('div');
        nameDiv.className = 'font-bold text-gray-900 truncate';
        nameDiv.textContent = p.starter.name;
        infoDiv.appendChild(nameDiv);

        const detailsDiv = document.createElement('div');
        detailsDiv.className = 'text-xs text-gray-500 flex gap-1';

        const uSpan = document.createElement('span');
        uSpan.textContent = `#${p.starter.number}`;
        const pSpan = document.createElement('span');
        pSpan.textContent = p.starter.pos || 'Pos';

        detailsDiv.appendChild(uSpan);
        detailsDiv.appendChild(pSpan);
        infoDiv.appendChild(detailsDiv);

        if (p.history.length > 0) {
            const subs = [...p.history.slice(1), p.current];
            if (subs.length > 0) {
                const subTextDiv = document.createElement('div');
                subTextDiv.className = 'text-xs text-indigo-700 italic mt-1';
                subTextDiv.textContent = `Sub: ${subs.map(s => '#' + s.number).join(', ')}`;
                infoDiv.appendChild(subTextDiv);
            }
        }
        lc.appendChild(infoDiv);
    }
}
