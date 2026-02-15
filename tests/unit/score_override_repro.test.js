
import { gameReducer, ActionTypes } from '../../frontend/reducer.js';

describe('Score Override Clearing Repro', () => {
    test('should REMOVE the override entry when score is cleared (empty string)', () => {
        const initialState = {
            overrides: {
                away: { 1: '2' },
            },
        };

        const action = {
            type: ActionTypes.SCORE_OVERRIDE,
            payload: { team: 'away', inning: 1, score: '' },
        };

        const newState = gameReducer(initialState, action);

        // If it just sets it to "", this might not be what we want if we want to revert to calculated score.
        // Most UI logic checks for existence of the key.
        expect(newState.overrides.away[1]).toBeUndefined();
    });
});
