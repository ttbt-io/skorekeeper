import Fuse from '../vendor/fuse.js';
import nlp from '../vendor/compromise.js';

const BASEBALL_LEXICON = [
    { term: 'ball' }, { term: 'strike' }, { term: 'foul' },
    { term: 'single' }, { term: 'double' }, { term: 'triple' }, { term: 'homerun' },
    { term: 'strikeout' }, { term: 'fly' }, { term: 'ground' }, { term: 'out' },
    { term: 'runner' }, { term: 'runners' }, { term: 'to' }, { term: 'first' }, { term: 'second' }, { term: 'third' }, { term: 'home' },
    { term: 'scores' }, { term: 'pinch' }, { term: 'pitching' }, { term: 'change' },
    { term: 'for' }, { term: 'number' }, { term: 'shortstop' }, { term: 'short' }, { term: 'stop' },
    { term: 'safe' }, { term: 'hit' }, { term: 'walk' }, { term: 'bb' }, { term: 'hbp' }, { term: 'reached' },
    { term: 'error' }, { term: 'choice' }, { term: 'steals' }, { term: 'steal' },
    { term: 'caught' }, { term: 'stealing' }, { term: 'wild' }, { term: 'passed' },
    { term: 'fc' }, { term: 'bases' }, { term: 'loaded' }, { term: 'empty' },
    { term: 'on' }, { term: 'pitch' }, { term: 'at' }, { term: 'by' },
    { term: 'actually' }, { term: 'previous' }, { term: 'last' }, { term: 'was' },
    { term: 'play' }, { term: 'with' }, { term: 'correction' },
    { term: 'line' }, { term: 'pop' },
    { term: 'pitcher' }, { term: 'catcher' }, { term: 'baseman' }, { term: 'fielder' }, { term: 'fielder\'s' }, { term: 'field' },
    { term: 'left' }, { term: 'center' }, { term: 'right' }, { term: 'base' }, { term: 'balls' },
];

const fuse = new Fuse(BASEBALL_LEXICON, {
    keys: ['term'],
    threshold: 0.3,
    distance: 100,
    includeScore: true,
});

// NOTE: lastRosterRef / activeFuse are module-level singletons cached for
// performance (avoids re-instantiating Fuse on every call when the roster
// hasn't changed). This makes the module non-reentrant across different roster
// contexts within the same JS module scope (e.g. parallel test runs that share
// a module instance). Tests should reset between suite runs if roster isolation
// is needed.
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
        activeFuse = new Fuse(dynamicLexicon, {
            keys: ['term'],
            threshold: 0.5,
            distance: 100,
            includeScore: true,
        });
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
        if (results.length > 0) {
            const best = results[0];
            const maxScore = best.item.type === 'player' ? 0.5 : 0.3;
            if ((best.score || 0) < maxScore) {
                return best.item.term;
            }
        }
        return word;
    });

    return correctedTokens.join('');
}

function cleanNormalTranscript(text, gameState = {}) {
    // 1. Use compromise to strip filler words and normalize
    let doc = nlp(text.toLowerCase());

    // Normalize: "the batter hit a single" -> "single"
    // "she hit a double" -> "double"
    // "that's a ball" -> "ball"
    doc.match('(he|she|they|it|that|the batter) (is|was|are|hit|thrown)').delete();
    // Handle contractions
    doc.match('(he\'s|she\'s|they\'re|it\'s|that\'s)').delete();
    doc.match('(a|the|was)').delete();

    // Reconstruct text
    let cleaned = doc.text().replace(/[.!]/g, ' '); // Keep commas for event separation

    // Normalize fly shorthand (e.g. f8 -> fly out to 8)
    cleaned = cleaned.replace(/\bf([1-9])\b/g, 'fly out to $1');

    // 2. Verb/Phrasing normalization before fuzzy correction
    cleaned = cleaned.replace(/\bsingled\b/g, 'single')
        .replace(/\bdoubled\b/g, 'double')
        .replace(/\btripled\b/g, 'triple')
        .replace(/\bwalked\b/g, 'walk')
        .replace(/\bstole\b/g, 'steals')
        .replace(/\bscoring\b/g, 'scores')
        .replace(/\bfielder choice\b/g, 'fielder\'s choice')
        .replace(/\breached on an error\b/g, 'reached on error')
        .replace(/\bsafe on an error\b/g, 'safe on error')
        .replace(/with\s+advance\s+to\s+(first|second|third|home)(?:\s+base)?/g, ', runner to $1')
        .replace(/advance\s+to\s+(first|second|third|home)(?:\s+base)?/g, ', runner to $1')
        .replace(/(?<!reached\s+|safe\s+)on\s+error\s+by\s+(\w+(?:\s+baseman|\s+fielder)?)/g, '')
        .replace(/\breached on error by (\w+(?:\s+baseman|\s+fielder)?)/g, 'reached on error to $1')
        .replace(/\bsafe on error by (\w+(?:\s+baseman|\s+fielder)?)/g, 'safe on error to $1');

    // 2.5 Normalizing separators before fuzzy correction to avoid corrupting them into roster names
    cleaned = cleaned.replace(/\s+and\s+then\s+/g, ', ')
        .replace(/\s+and\s+/g, ', ')
        .replace(/\s+then\s+/g, ', ');

    // 3. Fuzzy correct common terms and player names
    cleaned = fuzzyCorrect(cleaned, gameState);

    // 3.2 Post-fuzzy normalization for grammar safety
    cleaned = cleaned.replace(/\bfielder\s+choice\b/g, 'fielder\'s choice')
        .replace(/\b(first|second|third)\s*,\s*(second|third)\b/g, '$1 and $2');

    // 3.5 Replace player names/parts with "runner"
    const playerNames = [];
    if (gameState.roster) {
        const baseballTerms = new Set(BASEBALL_LEXICON.map(l => l.term));
        ['away', 'home'].forEach(team => {
            if (gameState.roster[team]) {
                gameState.roster[team].forEach(slot => {
                    if (slot.current && slot.current.name) {
                        const fullName = slot.current.name.toLowerCase().trim();
                        if (fullName && !baseballTerms.has(fullName)) {
                            playerNames.push(fullName);
                        }
                        fullName.split(/\s+/).forEach(part => {
                            const trimmedPart = part.trim();
                            if (trimmedPart.length > 2 && !baseballTerms.has(trimmedPart)) {
                                playerNames.push(trimmedPart);
                            }
                        });
                    }
                });
            }
        });
    }

    // Sort by length descending to replace longer names first
    const uniquePlayerNames = Array.from(new Set(playerNames)).sort((a, b) => b.length - a.length);
    uniquePlayerNames.forEach(name => {
        const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        // Only replace a player name when it is followed by 'to [base]' or 'scores',
        // i.e. when it is clearly acting as a runner reference. This is intentional:
        // bare mentions (e.g. "Smith-Peterson steals second" without a leading "runner")
        // are not substituted and must be handled by the grammar or a future NER pass.
        const regex = new RegExp('(?:\\brunner\\s+)?\\b' + escaped + '\\b(?=\\s+(?:to\\s+(?:first|second|third|home|1b|2b|3b)|scores))', 'gi');
        cleaned = cleaned.replace(regex, 'runner');
    });

    // 4. Phrasing expansion (Phase 6)
    cleaned = cleaned.replace(/safe\s+at\s+/g, 'runner to ')
        .replace(/got\s+him\s+at\s+/g, 'out at ')
        .replace(/out\s+at\s+first/g, 'ground out')
        .replace(/short\s+stop/g, 'shortstop');

    // Final cleanup of extra spaces and multi-commas
    return cleaned.replace(/,\s*,/g, ',')
        .replace(/\s+/g, ' ')
        .trim()
        .replace(/,$/, '');
}

export function cleanTranscript(transcript, gameState = {}) {
    if (!transcript) {
        return '';
    }
    const normalized = transcript.toLowerCase().trim();

    // Check pitch correction first
    const pitchMatch = normalized.match(/^(?:correction|actually)(?:[,:]|\s)+(?:the\s+last\s+pitch\s+was\s+|last\s+pitch\s+was\s+|that\s+was\s+a\s+|it\s+was\s+a\s+)?(?:a\s+)?(ball|strike|foul)$/i);
    if (pitchMatch) {
        return `__correction_pitch__ ${pitchMatch[1].toLowerCase()}`;
    }

    // Check play correction next
    const playMatch = normalized.match(/^(?:the\s+previous\s+play\s+was\s+actually|correction\s+the\s+last\s+play\s+was|correction\s+the\s+previous\s+play\s+was|actually\s+the\s+last\s+play\s+was|actually\s+the\s+previous\s+play\s+was|correction\s+last\s+play|actually\s+she|actually\s+he|actually\s+they|actually|correction)(?:[,:]|\s)+(.+)$/i);
    if (playMatch) {
        const cleanedRemainder = cleanNormalTranscript(playMatch[1], gameState);
        return `__correction_play__ ${cleanedRemainder}`;
    }

    return cleanNormalTranscript(transcript, gameState);
}
