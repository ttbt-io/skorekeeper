import { parseQuery, buildQuery } from '../../frontend/utils/searchParser.js';

describe('Search Parser', () => {
    test('parses simple free text', () => {
        const res = parseQuery('hello world');
        expect(res.tokens).toEqual(['hello', 'world']);
        expect(res.filters).toEqual([]);
    });

    test('parses key:value', () => {
        const res = parseQuery('event:Finals');
        expect(res.filters).toEqual([{ key: 'event', value: 'Finals', operator: '=', maxValue: '' }]);
    });

    test('parses quoted strings', () => {
        const res = parseQuery('event:"World Series" "New York"');
        expect(res.filters).toEqual([{ key: 'event', value: 'World Series', operator: '=', maxValue: '' }]);
        expect(res.tokens).toEqual(['New York']);
    });

    test('parses date operators', () => {
        const res = parseQuery('date:>=2025');
        expect(res.filters).toEqual([{ key: 'date', value: '2025', operator: '>=', maxValue: '' }]);
    });

    test('parses date range', () => {
        const res = parseQuery('date:2025-01..2025-03');
        expect(res.filters).toEqual([{ key: 'date', value: '2025-01', operator: '..', maxValue: '2025-03' }]);
    });

    test('parses mixed query', () => {
        const res = parseQuery('is:local stadium');
        expect(res.filters).toEqual([{ key: 'is', value: 'local', operator: '=', maxValue: '' }]);
        expect(res.tokens).toEqual(['stadium']);
    });

    test('buildQuery reconstructs string', () => {
        const obj = {
            tokens: ['stadium'],
            filters: [
                { key: 'event', value: 'World Series', operator: '=', maxValue: '' },
                { key: 'date', value: '2025', operator: '>=', maxValue: '' },
            ],
        };
        const str = buildQuery(obj);
        // Expect order: filters then tokens
        expect(str).toContain('event:"World Series"');
        expect(str).toContain('date:>=2025');
        expect(str).toContain('stadium');
    });
});
