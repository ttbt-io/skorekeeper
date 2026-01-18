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
 * Parses a search query string into structured filters and free text tokens.
 * Supports:
 * - key:value
 * - key:"value with spaces"
 * - flags: is:local
 * - date operators: date:>=2025
 * - date range: date:2025-01..2025-02
 *
 * @param {string} queryString
 * @returns {object} { tokens: [], filters: [{ key, value, operator, maxValue }] }
 */
export function parseQuery(queryString) {
    if (!queryString) {
        return { tokens: [], filters: [] };
    }

    const tokens = tokenize(queryString);
    const result = {
        tokens: [],
        filters: [],
    };

    for (const token of tokens) {
        const parts = token.split(':');
        if (parts.length >= 2 && parts[0]) {
            const key = parts.shift().toLowerCase();
            const rawVal = parts.join(':'); // Rejoin rest in case value has colons (unlikely for our simple syntax but safe)

            if (!key || !rawVal) {
                result.tokens.push(removeQuotes(token));
                continue;
            }

            // Parse Value/Operator
            let value = rawVal;
            let operator = '=';
            let maxValue = '';

            if (value.includes('..')) {
                const rangeParts = value.split('..');
                operator = '..';
                value = rangeParts[0];
                maxValue = rangeParts[1] || '';
            } else if (value.startsWith('>=')) {
                operator = '>=';
                value = value.substring(2);
            } else if (value.startsWith('<=')) {
                operator = '<=';
                value = value.substring(2);
            } else if (value.startsWith('>')) {
                operator = '>';
                value = value.substring(1);
            } else if (value.startsWith('<')) {
                operator = '<';
                value = value.substring(1);
            }

            result.filters.push({
                key,
                value: removeQuotes(value),
                operator,
                maxValue: removeQuotes(maxValue),
            });
        } else {
            result.tokens.push(removeQuotes(token));
        }
    }

    return result;
}

/**
 * Reconstructs a query string from the parsed object.
 * Useful for UI binding (Advanced Panel -> Search Box).
 */
export function buildQuery(parsedObj) {
    const parts = [];

    // Filters
    for (const f of parsedObj.filters) {
        let valStr = '';
        if (f.operator === '..') {
            valStr = `${quoteIfNeed(f.value)}..${quoteIfNeed(f.maxValue)}`;
        } else if (f.operator === '=') {
            valStr = quoteIfNeed(f.value);
        } else {
            valStr = `${f.operator}${quoteIfNeed(f.value)}`;
        }
        parts.push(`${f.key}:${valStr}`);
    }

    // Free Text
    for (const t of parsedObj.tokens) {
        parts.push(quoteIfNeed(t));
    }

    return parts.join(' ');
}

function tokenize(input) {
    const tokens = [];
    let currentToken = '';
    let inQuote = false;
    let quoteChar = '';

    for (let i = 0; i < input.length; i++) {
        const char = input[i];

        if (inQuote) {
            if (char === quoteChar) {
                inQuote = false;
                currentToken += char;
            } else {
                currentToken += char;
            }
        } else {
            if (char === ' ') {
                if (currentToken.length > 0) {
                    tokens.push(currentToken);
                    currentToken = '';
                }
            } else if (char === '"' || char === '\'') {
                inQuote = true;
                quoteChar = char;
                currentToken += char;
            } else {
                currentToken += char;
            }
        }
    }
    if (currentToken.length > 0) {
        tokens.push(currentToken);
    }
    return tokens;
}

function removeQuotes(s) {
    if (s.length >= 2) {
        const first = s[0];
        const last = s[s.length - 1];
        if ((first === '"' && last === '"') || (first === '\'' && last === '\'')) {
            return s.substring(1, s.length - 1);
        }
    }
    return s;
}

function quoteIfNeed(s) {
    if (!s) {
        return '';
    }
    if (s.includes(' ')) {
        return `"${s}"`;
    }
    return s;
}
