import { cleanTranscript } from '../../frontend/utils/fuzzy.js';

describe('Phase 4: Fuzzy Matching and Cleaning', () => {
    test('should correct common misheard baseball terms', () => {
        expect(cleanTranscript('Boll')).toBe('ball');
        expect(cleanTranscript('Strike won')).toBe('strike 1');
        expect(cleanTranscript('Short stop')).toBe('shortstop');
    });

    test('should handle punctuation and case', () => {
        expect(cleanTranscript('Ball, and runner scores!')).toBe('ball, runner scores');
    });

    test('should map spoken numbers to digits', () => {
        expect(cleanTranscript('pinch runner 5 for 11')).toBe('pinch runner 5 for 11');
        expect(cleanTranscript('pinch runner five for too')).toBe('pinch runner 5 for 2');
    });

    test('should handle natural phrasing (Phase 6)', () => {
        expect(cleanTranscript('The batter hit a single')).toBe('single');
        expect(cleanTranscript('She hit a double')).toBe('double');
        expect(cleanTranscript('They hit a triple')).toBe('triple');
        expect(cleanTranscript('That was a ball')).toBe('ball');
        expect(cleanTranscript('They\'re safe at first')).toBe('runner to first');
    });
});
