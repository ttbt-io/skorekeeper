import Fuse from '../vendor/fuse.js';
import nlp from '../vendor/compromise.js';

const BASEBALL_LEXICON = [
    { term: 'ball' }, { term: 'strike' }, { term: 'foul' },
    { term: 'single' }, { term: 'double' }, { term: 'triple' }, { term: 'homerun' },
    { term: 'strikeout' }, { term: 'fly' }, { term: 'ground' }, { term: 'out' },
    { term: 'runner' }, { term: 'to' }, { term: 'first' }, { term: 'second' }, { term: 'third' }, { term: 'home' },
    { term: 'scores' }, { term: 'pinch' }, { term: 'pitching' }, { term: 'change' },
    { term: 'for' }, { term: 'number' }, { term: 'shortstop' }, { term: 'short' }, { term: 'stop' },
    { term: 'safe' }, { term: 'hit' },
];

const fuse = new Fuse(BASEBALL_LEXICON, {
    keys: ['term'],
    threshold: 0.3,
    distance: 100,
    includeScore: true,
});

let lastRosterRef = null;
let activeFuse = fuse;

export function fuzzyCorrect(text, gameState = {}) {
    if (!text) {
        return '';
    }

    // Dynamic Roster Matching (Phase 4)
    // Cache the Fuse instance based on the roster reference to avoid re-instantiation
    const roster = gameState.roster;

    if (roster && roster !== lastRosterRef) {
        const dynamicLexicon = [...BASEBALL_LEXICON];
        ['away', 'home'].forEach(team => {
            if (roster[team]) {
                roster[team].forEach(slot => {
                    if (slot.current && slot.current.name) {
                        // Add full name and split parts
                        const name = slot.current.name.toLowerCase();
                        dynamicLexicon.push({ term: name, type: 'player' });
                        name.split(/\s+/).forEach(part => {
                            if (part.length > 2) {
                                dynamicLexicon.push({ term: part, type: 'player' });
                            }
                        });
                    }
                });
            }
        });
        activeFuse = new Fuse(dynamicLexicon, { keys: ['term'], threshold: 0.4 });
        lastRosterRef = roster;
    } else if (!roster) {
        activeFuse = fuse;
        lastRosterRef = null;
    }

    // Split by words but keep punctuation as separate tokens or attached
    // We want to correct the "word" part but keep the comma
    const tokens = text.toLowerCase().split(/(\s+|[,.!?])/);
    const correctedTokens = tokens.map(token => {
        if (!token || /^\s+$/.test(token) || /[,.!?]/.test(token)) {
            return token;
        }

        const word = token;
        const numberMap = {
            'won': '1', 'one': '1', 'too': '2', 'two': '2',
            'three': '3', 'tree': '3', 'four': '4', 'five': '5',
        };
        if (numberMap[word]) {
            return numberMap[word];
        }
        if (/^\d+$/.test(word)) {
            return word;
        }

        if (BASEBALL_LEXICON.some(l => l.term === word)) {
            return word;
        }

        const results = activeFuse.search(word);
        if (results.length > 0 && (results[0].score || 0) < 0.3) {
            return results[0].item.term;
        }
        return word;
    });

    return correctedTokens.join('');
}

export function cleanTranscript(transcript, gameState = {}) {
    // 1. Use compromise to strip filler words and normalize
    let doc = nlp(transcript.toLowerCase());

    // Normalize: "the batter hit a single" -> "single"
    // "she hit a double" -> "double"
    // "that's a ball" -> "ball"
    doc.match('(he|she|they|it|that|the batter) (is|was|are|hit|thrown)').delete();
    // Handle contractions
    doc.match('(he\'s|she\'s|they\'re|it\'s|that\'s)').delete();
    doc.match('(a|the|was)').delete();

    // Reconstruct text
    let cleaned = doc.text().replace(/[.!]/g, ' '); // Keep commas for event separation

    // 2. Fuzzy correct common terms and player names
    cleaned = fuzzyCorrect(cleaned, gameState);

    // 3. Phrasing expansion (Phase 6)
    cleaned = cleaned.replace(/safe\s+at\s+/g, 'runner to ')
        .replace(/got\s+him\s+at\s+/g, 'out at ')
        .replace(/out\s+at\s+first/g, 'ground out')
        .replace(/short\s+stop/g, 'shortstop');

    cleaned = cleaned.replace(/\s+and\s+then\s+/g, ', ')
        .replace(/\s+and\s+/g, ', ')
        .replace(/\s+then\s+/g, ', ');

    // Final cleanup of extra spaces and multi-commas
    return cleaned.replace(/,\s*,/g, ',')
        .replace(/\s+/g, ' ')
        .trim()
        .replace(/,$/, '');
}
