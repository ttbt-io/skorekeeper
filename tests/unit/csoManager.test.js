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

import { CSOManager } from '../../frontend/game/csoManager.js';
import { ActionTypes } from '../../frontend/reducer.js';

describe('CSOManager', () => {
    let csoManager;
    let mockDispatch;
    let mockGetBatterId;

    beforeEach(() => {
        mockDispatch = jest.fn();
        mockGetBatterId = jest.fn(() => 'batter-1');
        csoManager = new CSOManager({
            dispatch: mockDispatch,
            getBatterId: mockGetBatterId,
        });
    });

    describe('recordPitch', () => {
        test('should dispatch PITCH action', async() => {
            const state = { activeCtx: 'ctx', activeTeam: 'home' };
            await csoManager.recordPitch(state, 'ball', 'B');

            expect(mockDispatch).toHaveBeenCalledWith({
                type: ActionTypes.PITCH,
                payload: {
                    activeCtx: 'ctx',
                    type: 'ball',
                    code: 'B',
                    activeTeam: 'home',
                    batterId: 'batter-1',
                },
            });
        });

        test('should not dispatch if read-only', async() => {
            const state = { isReadOnly: true };
            await csoManager.recordPitch(state, 'ball', 'B');
            expect(mockDispatch).not.toHaveBeenCalled();
        });
    });

    describe('getCanonicalPosition', () => {
        test('should return coordinates for valid positions', () => {
            expect(csoManager.getCanonicalPosition(1)).toEqual({ x: 0.5, y: 0.625 }); // 100/200, 125/200
            expect(csoManager.getCanonicalPosition(6)).toEqual({ x: 0.35, y: 0.35 }); // 70/200, 70/200
        });

        test('should return null for invalid positions', () => {
            expect(csoManager.getCanonicalPosition(10)).toBeNull();
        });
    });

    describe('Sequence operations', () => {
        test('addToSequence should append', () => {
            const state = { seq: ['1'] };
            csoManager.addToSequence(state, '2');
            expect(state.seq).toEqual(['1', '2']);
        });

        test('backspaceSequence should pop', () => {
            const state = { seq: ['1', '2'] };
            csoManager.backspaceSequence(state);
            expect(state.seq).toEqual(['1']);
        });
    });

    describe('getSmartDefaultTrajectory', () => {
        test('should return Fly for Fly/IFF', () => {
            expect(csoManager.getSmartDefaultTrajectory({ res: 'Fly' })).toBe('Fly');
            expect(csoManager.getSmartDefaultTrajectory({ res: 'IFF' })).toBe('Fly');
        });

        test('should return Line for Line', () => {
            expect(csoManager.getSmartDefaultTrajectory({ res: 'Line' })).toBe('Line');
        });

        test('should return Ground for Ground/Safe', () => {
            expect(csoManager.getSmartDefaultTrajectory({ res: 'Ground' })).toBe('Ground');
            expect(csoManager.getSmartDefaultTrajectory({ res: 'Safe' })).toBe('Ground');
        });
    });

    describe('applySmartDefaults', () => {
        test('should apply location and trajectory based on first fielder', () => {
            const state = { res: 'Ground', seq: ['6-3'], hitData: {} };
            csoManager.applySmartDefaults(state);

            expect(state.hitData.location).toEqual({ x: 0.35, y: 0.35 }); // Position 6
            expect(state.hitData.trajectory).toBe('Ground');
        });

        test('should not overwrite existing location', () => {
            const existingLoc = { x: 0.1, y: 0.1 };
            const state = { res: 'Ground', seq: ['6-3'], hitData: { location: existingLoc } };
            csoManager.applySmartDefaults(state);

            expect(state.hitData.location).toBe(existingLoc);
        });

        test('should init hitData if missing', () => {
            const state = { res: 'Ground', seq: ['6'], hitData: null };
            csoManager.applySmartDefaults(state);
            expect(state.hitData).toBeDefined();
            expect(state.hitData.location).toEqual({ x: 0.35, y: 0.35 });
        });
    });

    describe('cycleBiP', () => {
        test('should cycle res options', () => {
            const state = { res: 'Safe', type: 'HIT' };
            csoManager.cycleBiP(state, 'res');
            // Options: ['Safe', 'Out', 'Ground', 'Fly', 'Line', 'IFF']
            // Next after Safe is Out
            expect(state.res).toBe('Out');
            // type should reset based on new res
            // Out -> ['OUT', 'SH', 'SF'] -> 'OUT'
            expect(state.type).toBe('OUT');
        });

        test('should cycle type options', () => {
            const state = { res: 'Safe', type: 'HIT' };
            // Options for Safe: ['HIT', 'ERR', 'FC']
            csoManager.cycleBiP(state, 'type');
            expect(state.type).toBe('ERR');
        });

        test('should handle dropped mode', () => {
            const state = { res: 'Safe', type: 'D3' };
            // Dropped res options: ['Safe', 'Out']
            csoManager.cycleBiP(state, 'res', 'dropped');
            expect(state.res).toBe('Out');

            // Dropped type options for Out: ['K'] (Wait, check code)
            // Code: if key='type', if currentRes='Safe' -> ['D3', 'FC'], else -> ['K']
            // We just cycled res to Out. But cycleBiP calls getBipOptions('type', 'Out', 'dropped')? No.
            // In cycleBiP:
            /*
            if (key === 'res') {
                if (mode !== 'dropped') {
                    // update type
                }
                // ...
            }
            */
            // So if mode IS dropped, type is NOT auto-updated when res cycles?
            // Let's verify expectations.
            // If I change res from Safe to Out in dropped mode, state.type remains 'D3'?
            expect(state.type).toBe('D3'); // Based on reading code logic
        });
    });

    describe('getBipOptions', () => {
        test('should return correct options for normal mode', () => {
            expect(csoManager.getBipOptions('res')).toContain('Safe');
            expect(csoManager.getBipOptions('base')).toContain('1B');
            expect(csoManager.getBipOptions('type', 'Safe')).toContain('HIT');
        });

        test('should return correct options for dropped mode', () => {
            expect(csoManager.getBipOptions('res', 'Safe', 'dropped')).toEqual(['Safe', 'Out']);
            expect(csoManager.getBipOptions('type', 'Safe', 'dropped')).toEqual(['D3', 'FC']);
            expect(csoManager.getBipOptions('type', 'Out', 'dropped')).toEqual(['K']);
        });
    });

    describe('calculateHitLocation', () => {
        test('should return null if svg missing', () => {
            expect(csoManager.calculateHitLocation({}, null)).toBeNull();
        });

        test('should use getBoundingClientRect fallback', () => {
            const mockSvg = {
                createSVGPoint: () => {
                    throw new Error('Not implemented in JSDOM');
                },
                getBoundingClientRect: () => ({ left: 0, top: 0, width: 200, height: 200 }),
            };
            const event = { clientX: 100, clientY: 100 };

            const loc = csoManager.calculateHitLocation(event, mockSvg);
            expect(loc).toEqual({ x: 0.5, y: 0.5 });
        });

        test('should clamp values', () => {
            const mockSvg = {
                createSVGPoint: () => {
                    throw new Error('Not implemented in JSDOM');
                },
                getBoundingClientRect: () => ({ left: 0, top: 0, width: 200, height: 200 }),
            };
            const event = { clientX: 300, clientY: -50 };

            const loc = csoManager.calculateHitLocation(event, mockSvg);
            expect(loc).toEqual({ x: 1, y: 0 });
        });
    });
});
