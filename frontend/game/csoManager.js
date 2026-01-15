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

import { ActionTypes } from '../reducer.js';
import {
    BiPResultFly,
    BiPResultLine,
    BiPResultPop,
    BiPResultGround,
    BiPResultOut,
    BiPResultSingle,
    BiPResultDouble,
    BiPResultTriple,
    BiPResultFC,
    BiPResultIFF,
    BiPResultSafe,
    BiPTypeCodeHit,
    BiPTypeCodeErr,
    BiPTypeCodeOut,
    BiPTypeCodeDP,
    BiPTypeCodeTP,
    BiPTypeCodeSH,
    BiPTypeCodeSF,
    PitchOutcomeDropped3rd,
    PitchOutcomeStrikeoutSwinging,
    BiPModeNormal,
    BiPModeDropped,
    BaseHome,
} from '../constants.js';

/**
 * Manages the logic and state for the Contextual Scoring Overlay (CSO).
 * Handles pitch sequences, Ball-in-Play (BIP) transitions, and hit location.
 */
export class CSOManager {
    /**
     * @param {object} options
     * @param {Function} options.dispatch - App dispatch function.
     * @param {Function} options.getBatterId - Helper to get current batter ID.
     */
    constructor({ dispatch, getBatterId }) {
        this.dispatch = dispatch;
        this.getBatterId = getBatterId;
    }

    /**
     * Records a pitch or an out.
     */
    async recordPitch(state, type, code) {
        if (state.isReadOnly) {
            return;
        }

        const action = {
            type: ActionTypes.PITCH,
            payload: {
                activeCtx: state.activeCtx,
                type,
                code,
                activeTeam: state.activeTeam,
                batterId: this.getBatterId(),
            },
        };

        await this.dispatch(action);
    }

    /**
     * Returns the canonical SVG coordinates (0-1) for a given field position (1-9).
     */
    getCanonicalPosition(pos) {
        const positions = {
            1: { x: 100 / 200, y: 125 / 200 }, // P
            2: { x: 100 / 200, y: 185 / 200 }, // C
            3: { x: 155 / 200, y: 115 / 200 }, // 1B
            4: { x: 130 / 200, y: 70 / 200 },  // 2B
            5: { x: 45 / 200, y: 115 / 200 },  // 3B
            6: { x: 70 / 200, y: 70 / 200 },   // SS
            7: { x: 35 / 200, y: 40 / 200 },   // LF
            8: { x: 100 / 200, y: 25 / 200 },  // CF
            9: { x: 165 / 200, y: 40 / 200 },  // RF
        };
        return positions[pos] || null;
    }

    /**
     * Adds a position to the current sequence (e.g. '6-3').
     */
    addToSequence(bipState, pos) {
        bipState.seq.push(pos);
    }

    /**
     * Removes the last item from the sequence.
     */
    backspaceSequence(bipState) {
        bipState.seq.pop();
    }

    /**
     * Provides smart default trajectories based on outcome.
     */
    getSmartDefaultTrajectory(bipState) {
        const { res, type } = bipState;
        if ([BiPResultFly, BiPResultIFF].includes(res)) {
            return BiPResultFly;
        }
        if (res === BiPResultLine) {
            return BiPResultLine;
        }
        if ([BiPTypeCodeDP, BiPTypeCodeTP].includes(type) && (res === BiPResultFly || res === BiPResultLine)) {
            return res;
        }
        return BiPResultGround;
    }

    /**
     * Applies smart default hit location and trajectory if not manually set.
     */
    applySmartDefaults(bipState) {
        const { res, seq, hitData } = bipState;
        if ([BiPResultGround, BiPResultFly, BiPResultLine].includes(res) && seq.length > 0 && (!hitData || !hitData.location)) {
            const firstFielder = parseInt(seq[0]);
            if (firstFielder >= 1 && firstFielder <= 9) {
                const pos = this.getCanonicalPosition(firstFielder);
                if (pos) {
                    if (!bipState.hitData) {
                        bipState.hitData = {};
                    }
                    bipState.hitData.location = pos;
                    // If trajectory isn't set, match the result
                    if (!bipState.hitData.trajectory) {
                        bipState.hitData.trajectory = res;
                    }
                }
            }
        }
    }

    /**
     * Calculates the hit location from click coordinates.
     */
    calculateHitLocation(e, svgElement) {
        if (!svgElement) {
            return null;
        }

        let normalizedX = 0.5;
        let normalizedY = 0.5;

        try {
            const pt = svgElement.createSVGPoint();
            pt.x = e.clientX;
            pt.y = e.clientY;
            const svgP = pt.matrixTransform(svgElement.getScreenCTM().inverse());
            normalizedX = svgP.x / 200;
            normalizedY = svgP.y / 200;
        } catch (err) {
            console.error('Error calculating SVG coordinates:', err);
            const rect = svgElement.getBoundingClientRect();
            normalizedX = (e.clientX - rect.left) / rect.width;
            normalizedY = (e.clientY - rect.top) / rect.height;
        }

        return {
            x: Math.max(0, Math.min(1, normalizedX)),
            y: Math.max(0, Math.min(1, normalizedY)),
        };
    }

    /**
     * Cycles a Ball-in-Play state property.
     * @param {object} bipState - The mutable BIP state object.
     * @param {string} key - The property to cycle ('res', 'base', 'type').
     * @param {string} mode - The current BIP mode ('normal', 'dropped').
     */
    cycleBiP(bipState, key, mode = BiPModeNormal) {
        if (!bipState.res) {
            bipState.res = BiPResultSafe;
        }

        const options = this.getBipOptions(key, bipState.res, mode);
        const current = bipState[key];
        const nextIdx = (options.indexOf(current) + 1) % options.length;
        bipState[key] = options[nextIdx];

        if (key === 'res') {
            if (mode !== BiPModeDropped) {
                const newTypes = this.getBipOptions('type', bipState.res, mode);
                bipState.type = newTypes.length > 0 ? newTypes[0] : '';
            }
            if (bipState.hitData?.location) {
                bipState.hitData.trajectory = this.getSmartDefaultTrajectory(bipState);
            }
        } else if (key === 'type') {
            if (bipState.hitData?.location) {
                bipState.hitData.trajectory = this.getSmartDefaultTrajectory(bipState);
            }
        }
    }

    /**
     * Returns available options for BIP fields.
     */
    getBipOptions(key, currentRes = BiPResultSafe, mode = BiPModeNormal) {
        // Dropped mode special cases
        if (mode === BiPModeDropped) {
            if (key === 'res') {
                return [BiPResultSafe, BiPResultOut];
            }
            if (key === 'type') {
                if (currentRes === BiPResultSafe) {
                    return [PitchOutcomeDropped3rd, BiPResultFC];
                }
                return [PitchOutcomeStrikeoutSwinging];
            }
        }

        if (key === 'res') {
            return [BiPResultSafe, BiPResultOut, BiPResultGround, BiPResultFly, BiPResultLine, BiPResultIFF];
        }
        if (key === 'base') {
            return [BiPResultSingle, BiPResultDouble, BiPResultTriple, BaseHome];
        }
        if (key === 'type') {
            const types = {
                [BiPResultSafe]: [BiPTypeCodeHit, BiPTypeCodeErr, BiPResultFC],
                [BiPResultOut]: [BiPTypeCodeOut, BiPTypeCodeSH, BiPTypeCodeSF],
                [BiPResultGround]: [BiPTypeCodeOut, BiPTypeCodeSH],
                [BiPResultFly]: [BiPTypeCodeOut, BiPTypeCodeSF],
                [BiPResultLine]: [BiPTypeCodeOut],
                [BiPResultIFF]: [BiPTypeCodeOut],
            };
            return types[currentRes] || [];
        }
        if (key === 'trajectory') {
            return [BiPResultGround, BiPResultLine, BiPResultFly, BiPResultPop];
        }
        return [];
    }
}
