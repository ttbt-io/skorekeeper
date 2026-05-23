import { cleanTranscript } from '../utils/fuzzy.js';
import { parseEvent } from '../utils/parser.js';

self.onmessage = async(e) => {
    const { transcript, gameState } = e.data;

    try {
        const cleaned = cleanTranscript(transcript, gameState);
        const actions = parseEvent(cleaned, gameState);

        self.postMessage({
            type: 'result',
            transcript,
            cleaned,
            actions,
        });
    } catch (err) {
        console.error('NLP Worker Error:', err);
        self.postMessage({
            type: 'error',
            name: err.name,
            message: err.message,
            options: err.options || null,
            transcript,
        });
    }
};
