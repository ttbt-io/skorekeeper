import { cleanTranscript } from '../../frontend/utils/fuzzy.js';

describe('NLP: Roster-Aware Fuzzy Matching', () => {
    const gameState = {
        roster: {
            away: [
                { current: { id: 'p1', name: 'Jones', number: '5' } },
                { current: { id: 'p2', name: 'Smith-Peterson', number: '11' } },
            ],
            home: [
                { current: { id: 'p3', name: 'O\'Malley', number: '22' } },
            ],
        },
    };

    test('should correct misheard roster names', () => {
        // "Jonas" -> "Jones"
        expect(cleanTranscript('pinch runner Jonas for Smith', gameState)).toBe('pinch runner jones for smith-peterson');

        // "Omaly" -> "o'malley"
        expect(cleanTranscript('pitching change for Omaly', gameState)).toBe('pitching change for o\'malley');
    });

    test('should handle partial name matches', () => {
        expect(cleanTranscript('pinch runner peterson', gameState)).toBe('pinch runner smith-peterson');
    });

    test('should still correct baseball terms while resolving names', () => {
        expect(cleanTranscript('Boll, runner Jonas to second', gameState)).toBe('ball, runner jones to second');
    });
});
