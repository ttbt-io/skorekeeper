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

import { createElement } from '../utils.js';
import {
    PitchCodeCalled,
    BiPResultFly,
    BiPResultLine,
    BiPResultPop,
    BiPResultGround,
    BiPResultOut,
    RunnerActionOut,
} from '../constants.js';

/**
 * Handles rendering of the Contextual Scoring Overlay (CSO).
 */
export class CSORenderer {
    /**
     * @param {object} options
     * @param {HTMLElement} options.zoomContainer - The container for the zoomed diamond SVG.
     * @param {HTMLElement} options.bipFieldSvg - The field SVG in the BIP view.
     * @param {object} options.callbacks - Callbacks for user interactions.
     * @param {Function} options.callbacks.onCycleOuts - Called when the out circle is clicked.
     * @param {Function} options.callbacks.resolvePlayerNumber - Called to resolve a player ID to a number string.
     * @param {Function} options.callbacks.onApplyRunnerAction - Called when a runner action is selected.
     */
    constructor({ zoomContainer, bipFieldSvg, callbacks }) {
        this.zoomContainer = zoomContainer;
        this.bipFieldSvg = bipFieldSvg;
        this.callbacks = callbacks;

        this.gridPathCoords = [
            { x1: 30, y1: 55, x2: 44, y2: 42 }, // Home to 1st
            { x1: 44, y1: 42, x2: 30, y2: 28 }, // 1st to 2nd
            { x1: 30, y1: 28, x2: 16, y2: 42 }, // 2nd to 3rd
            { x1: 16, y1: 42, x2: 30, y2: 55 }, // 3rd to Home
        ];
    }

    /**
     * Opens the runner context menu for a specific base with a given action type.
     * @param {number} idx - The index of the base.
     * @param {string} type - The type of runner action ('advance' or 'out').
     */
    openRunnerMenu(idx, type) {
        const container = document.getElementById('runner-menu-options');
        if (!container) {
            return;
        }

        container.innerHTML = '';
        let opts = type === 'advance' ? ['SB', 'WP', 'PB', 'Adv', 'Err', 'Obs', 'Place', 'CR'] : ['CS', 'PO', 'Force', 'Tag'];
        opts.forEach((c) => {
            const btn = createElement('button', {
                className: 'text-xs bg-gray-700 p-2 rounded text-white font-bold',
                dataset: { c },
                text: c,
                onClick: () => this.callbacks.onApplyRunnerAction(idx, c),
            });
            container.appendChild(btn);
        });

        const menu = document.getElementById('cso-runner-menu');
        if (menu) {
            menu.classList.remove('hidden');
        }
    }

    /**
     * Renders the list of runners and their potential advance outcomes in the runner advance view.
     * @param {Array<object>} runnerState - The pending runner state.
     * @param {Function} onCycle - Callback to cycle the outcome.
     * @param {Function} [onContextMenu] - Callback for context menu (event, index).
     */
    renderRunnerAdvanceList(runnerState, onCycle, onContextMenu) {
        const c = document.getElementById('runner-advance-list');
        if (!c) {
            return;
        }
        c.innerHTML = '';

        runnerState.forEach((r, i) => {
            const bases = ['1st', '2nd', '3rd', 'Home'];
            let btnClass = 'bg-gray-700';
            if (r.outcome.startsWith('To') || r.outcome === 'Score') {
                btnClass = 'bg-green-600';
            }
            if (r.outcome === RunnerActionOut) {
                btnClass = 'bg-red-600';
            }

            const outerDiv = createElement('div', { className: 'bg-gray-900 p-3 rounded border border-gray-700' });

            const nameBaseDiv = createElement('div', {
                className: 'text-sm text-gray-400 mb-1',
                text: `${r.name} (on ${bases[r.base]})`,
            });
            outerDiv.appendChild(nameBaseDiv);

            const button = createElement('button', {
                className: `${btnClass} w-full py-3 rounded font-bold text-white text-lg flex items-center justify-center gap-2`,
                id: `btn-adv-${i}`,
                onClick: () => onCycle(i),
            });

            if (onContextMenu) {
                button.oncontextmenu = (e) => onContextMenu(e, i);
            }

            const spanText = createElement('span', { text: r.outcome });
            button.appendChild(spanText);
            const icon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
            icon.setAttribute('class', 'w-5 h-5 opacity-70');
            icon.setAttribute('fill', 'none');
            icon.setAttribute('stroke', 'currentColor');
            icon.setAttribute('viewBox', '0 0 24 24');
            const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
            path.setAttribute('stroke-linecap', 'round');
            path.setAttribute('stroke-linejoin', 'round');
            path.setAttribute('stroke-width', '2');
            path.setAttribute('d', 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15');
            icon.appendChild(path);
            button.appendChild(icon);

            outerDiv.appendChild(button);
            c.appendChild(outerDiv);
        });
    }

    /**
     * Renders the list of current runner actions (steals, etc.) in the Runner Action View.
     * @param {Array<object>} runners - The current runners on base.
     * @param {object} events - The event map to check current state.
     * @param {Function} onCycle - Callback to cycle the action.
     * @param {Function} [onContextMenu] - Callback for context menu (event, index).
     */
    renderRunnerActionList(runners, events, onCycle, onContextMenu) {
        const c = document.getElementById('runner-action-list');
        if (!c) {
            return;
        }
        c.innerHTML = '';

        if (runners.length === 0) {
            c.textContent = 'No runners on base.';
            return;
        }

        runners.forEach((r, i) => {
            const bases = ['1st', '2nd', '3rd', 'Home'];
            const action = r.action || 'Stay';
            let btnClass = 'bg-gray-700';
            if (['SB', 'Adv', 'Wild Pitch', 'Passed Ball'].includes(action)) {
                btnClass = 'bg-green-600';
            }
            if (['CS', 'PO', 'Left Early'].includes(action)) {
                btnClass = 'bg-red-600';
            }

            const outerDiv = createElement('div', { className: 'bg-gray-900 p-3 rounded border border-gray-700' });

            const nameBaseDiv = createElement('div', {
                className: 'text-sm text-gray-400 mb-1',
                text: `${r.name} (on ${bases[r.base]})`,
            });
            outerDiv.appendChild(nameBaseDiv);

            const button = createElement('button', {
                className: `${btnClass} w-full py-3 rounded font-bold text-white text-lg`,
                text: action,
                onClick: () => onCycle(i),
            });

            if (onContextMenu) {
                button.oncontextmenu = (e) => onContextMenu(e, i);
            }

            outerDiv.appendChild(button);
            c.appendChild(outerDiv);
        });
    }

    /**
     * Renders the entire CSO view based on the current state.
     */
    render(state, activeGame) {
        if (!this.zoomContainer || !this.bipFieldSvg) {
            return;
        }

        const { activeData, activeCtx, bipState, isEditing, isReadOnly, isLocationMode } = state;
        const isFinal = activeGame && activeGame.status === 'final';
        const isLocked = isReadOnly || isFinal;
        const isRecorded = (!!activeData.outcome && !isEditing) || isLocked;

        // 1. Basic Metadata
        const outcomeEl = document.getElementById('zoom-outcome-text');
        if (outcomeEl) {
            outcomeEl.textContent = activeData.outcome;
        }

        // 2. Pitch Sequence
        const pitchSequenceContainer = document.getElementById('pitch-sequence-container');
        if (pitchSequenceContainer) {
            pitchSequenceContainer.innerHTML = '';
            (activeData.pitchSequence || []).forEach((p) => {
                const pitchBadge = document.createElement('div');
                pitchBadge.className = `pitch-badge pitch-${p.type}`;
                let label = p.code || (p.type ? p.type[0].toUpperCase() : '?');
                if (p.type === 'strike') {
                    label = p.code === PitchCodeCalled ? 'ꓘ' : 'K';
                }
                pitchBadge.textContent = label;
                pitchSequenceContainer.appendChild(pitchBadge);
            });
        }

        // 3. View Visibility Toggles
        const actionAreaPitch = document.getElementById('action-area-pitch');
        if (actionAreaPitch) {
            actionAreaPitch.classList.toggle('hidden', isRecorded);
        }
        const actionAreaRecorded = document.getElementById('action-area-recorded');
        if (actionAreaRecorded) {
            actionAreaRecorded.classList.toggle('hidden', !isRecorded);
        }

        const sideControls = document.getElementById('side-controls');
        if (sideControls) {
            sideControls.classList.toggle('hidden', isRecorded || isLocked);
        }

        const runnersOnBase = state.runnersOnBase || [];
        const hasRunners = runnersOnBase.length > 0;
        const btnRunnerActions = document.getElementById('btn-runner-actions');
        if (btnRunnerActions) {
            btnRunnerActions.classList.toggle('hidden', !hasRunners || isLocked);
        }
        const btnClearAll = document.getElementById('btn-clear-all');
        if (btnClearAll) {
            btnClearAll.classList.toggle('hidden', isLocked);
        }
        const btnShowBip = document.getElementById('btn-show-bip');
        if (btnShowBip) {
            btnShowBip.classList.toggle('hidden', isLocked);
        }

        // 4. Hit Location Markers & Pos Keys
        const posKeys = this.bipFieldSvg.querySelectorAll('.pos-key');
        posKeys.forEach(el => el.style.display = isLocationMode ? 'none' : '');

        // Clear previous markers
        Array.from(this.bipFieldSvg.children).forEach(child => {
            if (child.classList.contains('hit-location-marker')) {
                this.bipFieldSvg.removeChild(child);
            }
        });

        const hitData = bipState.hitData;
        const divTrajControls = document.getElementById('div-traj-controls');
        const btnTraj = document.getElementById('btn-traj');

        if (hitData && hitData.location) {
            if (divTrajControls) {
                divTrajControls.classList.remove('hidden');
            }
            if (btnTraj) {
                btnTraj.textContent = hitData.trajectory.charAt(0).toUpperCase();
            }

            const SVG_NS = 'http://www.w3.org/2000/svg';
            const homeX = 100, homeY = 170;
            const hitX = hitData.location.x * 200;
            const hitY = hitData.location.y * 200;

            let pathEl;
            if (hitData.trajectory === BiPResultFly || hitData.trajectory === BiPResultPop) {
                pathEl = document.createElementNS(SVG_NS, 'path');
                const midX = (homeX + hitX) / 2;
                const midY = (homeY + hitY) / 2;
                let controlX = midX;
                // Add curve for straight-away hits
                if (Math.abs(hitX - homeX) < 10) {
                    controlX += 25;
                }
                const controlY = midY - 90;
                pathEl.setAttribute('d', `M ${homeX} ${homeY} Q ${controlX} ${controlY} ${hitX} ${hitY}`);
                pathEl.setAttribute('fill', 'none');
            } else {
                pathEl = document.createElementNS(SVG_NS, 'line');
                pathEl.setAttribute('x1', homeX); pathEl.setAttribute('y1', homeY);
                pathEl.setAttribute('x2', hitX); pathEl.setAttribute('y2', hitY);
            }

            pathEl.setAttribute('stroke', '#cbd5e1');
            pathEl.setAttribute('stroke-width', '4');
            pathEl.setAttribute('stroke-linecap', 'round');
            if (hitData.trajectory === BiPResultGround) {
                pathEl.setAttribute('stroke-dasharray', '8 4');
            }
            pathEl.classList.add('hit-location-marker');
            this.bipFieldSvg.appendChild(pathEl);

            const marker = document.createElementNS(SVG_NS, 'circle');
            marker.classList.add('hit-location-marker');
            marker.setAttribute('cx', hitX); marker.setAttribute('cy', hitY);
            marker.setAttribute('r', '8');
            marker.setAttribute('fill', 'red');
            this.bipFieldSvg.appendChild(marker);
        } else {
            if (divTrajControls) {
                divTrajControls.classList.add('hidden');
            }
        }

        // 5. Visual Layer (Zoomed Diamond)
        const SVG_NS = 'http://www.w3.org/2000/svg';

        // Cleanup old content layer
        const oldLayer = this.zoomContainer.querySelector('.cell-content-layer');
        if (oldLayer) {
            this.zoomContainer.removeChild(oldLayer);
        }

        let svg = this.zoomContainer.querySelector('svg.batter-cell-svg');
        if (!svg) {
            svg = document.createElementNS(SVG_NS, 'svg');
            svg.classList.add('batter-cell-svg');
            svg.setAttribute('viewBox', '0 0 60 60');
            this.zoomContainer.prepend(svg);
        } else {
            svg.innerHTML = '';
            svg.setAttribute('viewBox', '0 0 60 60');
        }

        // Diamond Graphics
        if (activeData.paths[3] === 1) {
            const fieldPoly = document.createElementNS(SVG_NS, 'polygon');
            fieldPoly.setAttribute('points', '30,28 44,42 30,55 16,42');
            fieldPoly.setAttribute('class', 'run-scored-diamond');
            svg.appendChild(fieldPoly);
        }

        // Foul Lines & Fence
        [{ x1: 30, y1: 55, x2: 60, y2: 25 }, { x1: 30, y1: 55, x2: 0, y2: 25 }].forEach(l => {
            const line = document.createElementNS(SVG_NS, 'line');
            line.setAttribute('x1', l.x1); line.setAttribute('y1', l.y1);
            line.setAttribute('x2', l.x2); line.setAttribute('y2', l.y2);
            line.setAttribute('stroke', '#cbd5e1'); line.setAttribute('stroke-width', '1');
            svg.appendChild(line);
        });
        const fenceArc = document.createElementNS(SVG_NS, 'path');
        fenceArc.setAttribute('d', 'M 0,25 A 34.2,34.2 0 0 1 60,25');
        fenceArc.setAttribute('stroke', '#cbd5e1'); fenceArc.setAttribute('stroke-width', '1');
        fenceArc.setAttribute('fill', 'none');
        svg.appendChild(fenceArc);

        const xMarkers = [];

        // Paths
        for (let idx = 0; idx < 4; idx++) {
            const line = document.createElementNS(SVG_NS, 'line');
            let classes = ['base-path-segment'];
            let filled = false;
            const status = activeData.paths[idx];

            if (status === 1 || status === 2) {
                if (idx === 0) {
                    filled = !activeData.pathInfo[idx] || !activeData.pathInfo[idx].includes('Place');
                }
                else {
                    filled = activeData.paths[idx-1] === 1;
                }
            }

            if (filled) {
                classes.push('filled-black');
            }
            line.classList.add(...classes);
            line.setAttribute('x1', this.gridPathCoords[idx].x1);
            line.setAttribute('y1', this.gridPathCoords[idx].y1);
            line.setAttribute('x2', this.gridPathCoords[idx].x2);
            line.setAttribute('y2', this.gridPathCoords[idx].y2);

            if (status === 2) {
                const t = activeData.outPos ? activeData.outPos[idx] : 0.5;
                const lx2 = this.gridPathCoords[idx].x1 + t * (this.gridPathCoords[idx].x2 - this.gridPathCoords[idx].x1);
                const ly2 = this.gridPathCoords[idx].y1 + t * (this.gridPathCoords[idx].y2 - this.gridPathCoords[idx].y1);
                line.setAttribute('x2', lx2); line.setAttribute('y2', ly2);

                const txt = document.createElementNS(SVG_NS, 'text');
                txt.style.fontSize = '6px'; txt.style.fontWeight = 'bold';
                txt.setAttribute('text-anchor', 'middle'); txt.setAttribute('dominant-baseline', 'middle');
                txt.setAttribute('x', lx2); txt.setAttribute('y', ly2);
                txt.textContent = 'X';

                const bg = document.createElementNS(SVG_NS, 'rect');
                bg.setAttribute('x', lx2 - 3); bg.setAttribute('y', ly2 - 3);
                bg.setAttribute('width', '6'); bg.setAttribute('height', '6');
                bg.setAttribute('fill', 'white'); bg.setAttribute('rx', '1');
                xMarkers.push(bg, txt);
            }
            svg.appendChild(line);
        }
        xMarkers.forEach(m => svg.appendChild(m));

        // Bases
        const baseCoords = [{ x: 41, y: 39, id: '1b' }, { x: 27, y: 25, id: '2b' }, { x: 13, y: 39, id: '3b' }, { x: 27, y: 52, id: 'home' }];

        // Identify which bases have runners (excluding current batter)
        const occupiedBases = new Set();
        (state.runnersOnBase || []).forEach(r => {
            if (r.idx !== activeCtx.b) {
                occupiedBases.add(r.base);
            }
        });

        baseCoords.forEach((pt, idx) => {
            const el = idx === 3 ? document.createElementNS(SVG_NS, 'polygon') : document.createElementNS(SVG_NS, 'rect');
            if (idx === 3) {
                const cx = pt.x + 3, cy = pt.y + 3;
                el.setAttribute('points', `${cx-3},${cy-3} ${cx+3},${cy-3} ${cx+3},${cy} ${cx},${cy+3} ${cx-3},${cy}`);
            } else {
                el.setAttribute('x', pt.x); el.setAttribute('y', pt.y);
                el.setAttribute('width', '6'); el.setAttribute('height', '6');
                el.setAttribute('transform', `rotate(45 ${pt.x+3} ${pt.y+3})`);
            }
            el.classList.add('base-rect');
            if (activeData.paths[idx] === 1) {
                el.classList.add('base-filled');
            }
            svg.appendChild(el);

            // Ghost Runner
            if (!isRecorded && occupiedBases.has(idx)) {
                const ghostRunner = document.createElementNS(SVG_NS, 'circle');
                ghostRunner.setAttribute('r', '2.5');
                ghostRunner.setAttribute('fill', 'white');
                ghostRunner.setAttribute('stroke', 'black');
                ghostRunner.setAttribute('stroke-width', '0.5');
                ghostRunner.classList.add('ghost-runner');
                ghostRunner.dataset.baseIdx = idx;

                const ghostOffsets = [
                    { x: -3, y: -3 }, // 1B -> NW
                    { x: -3, y: 3 },  // 2B -> SW
                    { x: 3, y: 3 },   // 3B -> SE
                    { x: 3, y: -3 },  // Home -> NE
                ];
                const currentBaseCenter = { x: pt.x + 3, y: pt.y + 3 };
                const offset = ghostOffsets[idx];
                ghostRunner.setAttribute('cx', currentBaseCenter.x + offset.x);
                ghostRunner.setAttribute('cy', currentBaseCenter.y + offset.y);
                svg.appendChild(ghostRunner);
            }
        });

        // Render Hit Path (on Diamond) based on activeData
        const diamondHitData = activeData.hitData;
        if (diamondHitData && diamondHitData.location) {
            const homePlateX = 30;
            const homePlateY = 55;

            // Map (0.5, 0.85) -> (30, 55). Scale 60.
            const hitX = (diamondHitData.location.x - 0.5) * 60 + 30;
            const hitY = (diamondHitData.location.y - 0.85) * 60 + 55;

            let hitPath;
            if (diamondHitData.trajectory === BiPResultFly || diamondHitData.trajectory === BiPResultPop) {
                const midX = (homePlateX + hitX) / 2;
                const midY = (homePlateY + hitY) / 2;
                let controlX = midX;
                // Add curve for straight-away hits
                if (Math.abs(hitX - homePlateX) < 5) {
                    controlX += 10;
                }
                const controlY = midY - 30;
                hitPath = document.createElementNS(SVG_NS, 'path');
                hitPath.setAttribute('d', `M ${homePlateX} ${homePlateY} Q ${controlX} ${controlY} ${hitX} ${hitY}`);
            } else {
                hitPath = document.createElementNS(SVG_NS, 'line');
                hitPath.setAttribute('x1', homePlateX);
                hitPath.setAttribute('y1', homePlateY);
                hitPath.setAttribute('x2', hitX);
                hitPath.setAttribute('y2', hitY);
            }
            hitPath.classList.add('hit-path', diamondHitData.trajectory.toLowerCase());
            hitPath.setAttribute('stroke-width', '0.5'); // Thinner stroke for zoomed view
            svg.appendChild(hitPath);

            // Draw X if Air Out
            const out = activeData.outcome || '';
            let isAirOut = false;
            if (activeData.bipState) {
                const { res, type } = activeData.bipState;
                isAirOut = (
                    res === BiPResultFly ||
                    res === BiPResultLine ||
                    res === BiPResultPop ||
                    res === 'IFF' ||
                    (res === BiPResultOut && type === 'SF') ||
                    ((res === 'DP' || res === 'TP') && (type === BiPResultFly || type === BiPResultLine || type === BiPResultPop))
                );
            } else {
                isAirOut = out.startsWith('F') || out.startsWith('L') || out.startsWith('P') || out.startsWith('IFF') || out.startsWith('SF');
            }

            if (activeData.paths[0] !== 1 && isAirOut) {
                const textX = document.createElementNS(SVG_NS, 'text');
                textX.setAttribute('x', hitX);
                textX.setAttribute('y', hitY);
                textX.setAttribute('text-anchor', 'middle');
                textX.setAttribute('dominant-baseline', 'middle');
                textX.style.fontSize = '6px';
                textX.style.fontWeight = 'bold';
                textX.textContent = 'X';
                svg.appendChild(textX);
            }
        }

        // Path Labels (PO, SB, etc)
        const txtCoords = [{ x: 45, y: 50 }, { x: 45, y: 25 }, { x: 15, y: 25 }, { x: 15, y: 50 }];
        (activeData.pathInfo || []).forEach((txt, idx) => {
            if (txt && activeData.paths[idx]) {
                const el = document.createElementNS(SVG_NS, 'text');
                el.classList.add('path-text-grid');
                el.setAttribute('x', txtCoords[idx].x); el.setAttribute('y', txtCoords[idx].y);
                el.setAttribute('text-anchor', 'middle'); el.setAttribute('dominant-baseline', 'middle');
                el.style.fontSize = '5px'; el.style.fontWeight = 'bold';
                el.textContent = txt;
                svg.appendChild(el);
            }
        });

        // 6. Interaction Layer
        const iGroup = document.createElementNS(SVG_NS, 'g');
        iGroup.setAttribute('id', 'cso-interaction-layer');
        iGroup.style.pointerEvents = 'all';

        for (let idx = 0; idx < 4; idx++) {
            const line = document.createElementNS(SVG_NS, 'line');
            line.setAttribute('id', `zoom-bg-${idx+1}`);
            line.setAttribute('x1', this.gridPathCoords[idx].x1); line.setAttribute('y1', this.gridPathCoords[idx].y1);
            line.setAttribute('x2', this.gridPathCoords[idx].x2); line.setAttribute('y2', this.gridPathCoords[idx].y2);
            line.setAttribute('stroke', 'red'); line.setAttribute('stroke-width', '8');
            line.setAttribute('stroke-opacity', '0');
            iGroup.appendChild(line);
        }

        baseCoords.forEach(pt => {
            const c = document.createElementNS(SVG_NS, 'circle');
            c.setAttribute('id', `zoom-base-${pt.id}`);
            c.setAttribute('cx', pt.x + 3); c.setAttribute('cy', pt.y + 3);
            c.setAttribute('r', '6'); c.setAttribute('fill', 'blue'); c.setAttribute('fill-opacity', '0');
            iGroup.appendChild(c);
        });
        svg.appendChild(iGroup);

        // 7. RBI Indicator
        if (activeData.paths[3] === 1) {
            const creditorId = activeData.scoreInfo?.rbiCreditedTo;
            const rbiText = creditorId ? ('#' + this.callbacks.resolvePlayerNumber(creditorId)) : 'RBI?';
            const rGroup = document.createElementNS(SVG_NS, 'g');
            rGroup.setAttribute('id', 'rbi-indicator-group');
            rGroup.style.cursor = 'pointer';
            rGroup.onclick = (e) => {
                e.stopPropagation(); this.callbacks.onRBIEdit();
            };

            const bg = document.createElementNS(SVG_NS, 'circle');
            bg.setAttribute('cx', '7'); bg.setAttribute('cy', '57'); bg.setAttribute('r', '6');
            bg.setAttribute('fill', creditorId ? '#2563eb' : '#ef4444');
            bg.setAttribute('stroke', 'white'); bg.setAttribute('stroke-width', '0.5');
            rGroup.appendChild(bg);

            const txt = document.createElementNS(SVG_NS, 'text');
            txt.setAttribute('x', '7'); txt.setAttribute('y', '57.5');
            txt.setAttribute('text-anchor', 'middle'); txt.setAttribute('dominant-baseline', 'middle');
            txt.style.fontSize = '4.5px'; txt.style.fontWeight = 'bold'; txt.setAttribute('fill', 'white');
            txt.textContent = rbiText;
            rGroup.appendChild(txt);
            svg.appendChild(rGroup);
        }

        // 8. HTML Content Layer (Overlay)
        const cLayer = document.createElement('div');
        cLayer.className = 'cell-content-layer';
        cLayer.style.position = 'absolute'; cLayer.style.inset = '0'; cLayer.style.pointerEvents = 'none';

        const oDisp = document.createElement('div');
        oDisp.className = 'out-display'; oDisp.style.cursor = 'pointer'; oDisp.style.pointerEvents = 'auto';
        oDisp.onclick = (e) => {
            e.stopPropagation(); this.callbacks.onCycleOuts();
        };
        const oCirc = document.createElement('div');
        oCirc.className = 'out-number-circle';
        if (activeData.outNum > 0) {
            oCirc.textContent = activeData.outNum;
        }
        else {
            oCirc.textContent = '\u00A0'; // non-breaking space
        }
        oDisp.appendChild(oCirc);

        const count = document.createElement('div');
        count.className = 'count-display'; count.style.gap = '2px';
        const renderDots = (n, max) => {
            const div = document.createElement('div');
            div.className = 'count-dots'; div.style.gap = '2px';
            for (let i = 0; i < max; i++) {
                const dot = document.createElement('div');
                dot.className = `dot ${i < n ? 'filled-black' : ''}`;
                dot.style.width = '6px'; dot.style.height = '6px';
                div.appendChild(dot);
            }
            return div;
        };
        count.appendChild(renderDots(activeData.balls, 4));
        count.appendChild(renderDots(activeData.strikes, 3));

        const resText = document.createElement('div');
        resText.className = 'outcome-text'; resText.textContent = activeData.outcome;

        cLayer.appendChild(oDisp); cLayer.appendChild(count); cLayer.appendChild(resText);
        this.zoomContainer.appendChild(cLayer);
    }
}
