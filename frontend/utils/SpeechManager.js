export class SpeechManager {
    constructor(gameStateCallback) {
        this.gameStateCallback = gameStateCallback;
        this.recognition = null;
        this.isListening = false;
        this.onResultCallback = null;
        this.onErrorCallback = null;
        this.onEndCallback = null;
        this.worker = null;

        // Initialize Web Worker for NLP (Phase 6)
        if (typeof window !== 'undefined' && window.Worker) {
            try {
                // Use eval to hide import.meta from the static analyzer (Jest/Node)
                let metaUrl = null;
                try {
                    metaUrl = eval('import.meta.url');
                } catch {
                    // Node/Jest will throw here, but that's fine
                }

                const workerUrl = metaUrl
                    ? new URL('../workers/nlpWorker.js', metaUrl)
                    : './workers/nlpWorker.js';

                this.worker = new Worker(workerUrl, { type: 'module' });
                this.worker.onmessage = (e) => {
                    if (e.data.type === 'result') {
                        if (this.onResultCallback) {
                            this.onResultCallback({
                                transcript: e.data.transcript,
                                cleaned: e.data.cleaned,
                                actions: e.data.actions,
                            });
                        }
                    } else if (e.data.type === 'error') {
                        console.error('NLP Error from Worker:', e.data.message);
                        if (this.onErrorCallback) {
                            this.onErrorCallback(e.data.message);
                        }
                    }
                };
                this.worker.onerror = (err) => {
                    console.error('Worker Script Error:', err.message, 'at', err.filename, ':', err.lineno);
                    if (this.onErrorCallback) {
                        this.onErrorCallback('NLP processing failed: ' + (err.message || 'load error'));
                    }
                };
            } catch {
                console.warn('Web Worker could not be initialized');
            }
        }
        if (typeof window !== 'undefined' && (window.SpeechRecognition || window.webkitSpeechRecognition)) {
            const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
            try {
                this.recognition = new SpeechRecognition();
                this.recognition.continuous = false;
                this.recognition.interimResults = false;
                this.recognition.lang = 'en-US';

                this.recognition.onresult = (event) => {
                    const transcript = event.results[0][0].transcript;
                    this.handleTranscript(transcript);
                };

                this.recognition.onerror = (event) => {
                    console.error('Speech recognition error:', event.error);
                    if (this.onErrorCallback) {
                        this.onErrorCallback(event.error);
                    }
                    this.isListening = false;
                };

                this.recognition.onend = () => {
                    this.isListening = false;
                    if (this.onEndCallback) {
                        this.onEndCallback();
                    }
                };
            } catch (e) {
                console.warn('Speech recognition could not be initialized:', e);
                this.recognition = null;
            }
        }
    }

    start(onResult, onError, onEnd) {
        if (!this.recognition) {
            if (onError) {
                onError('Speech recognition not supported');
            }
            return;
        }
        if (this.isListening) {
            return;
        }

        this.onResultCallback = onResult;
        this.onErrorCallback = onError;
        this.onEndCallback = onEnd;

        try {
            this.recognition.start();
            this.isListening = true;
        } catch (err) {
            if (onError) {
                onError(err.message);
            }
        }
    }

    stop() {
        if (this.recognition && this.isListening) {
            this.recognition.stop();
            this.isListening = false;
        }
    }

    handleTranscript(transcript) {
        let state = this.gameStateCallback();

        // Ensure state has essential fields (Phase 6 robustness)
        if (!state) {
            import('../reducer.js').then(({ getInitialState }) => {
                state = getInitialState();
                this._sendToWorker(transcript, state);
            });
            return;
        }

        this._sendToWorker(transcript, state);
    }

    _sendToWorker(transcript, state) {
        if (this.worker) {
            // Offload to Web Worker (Phase 6)
            this.worker.postMessage({
                transcript,
                gameState: state,
            });
        } else {
            // Fallback for environments without workers (should be rare)
            console.warn('Web Workers not supported. NLP will run on main thread.');
            import('./fuzzy.js').then(({ cleanTranscript }) => {
                import('./parser.js').then(({ parseEvent }) => {
                    try {
                        const cleaned = cleanTranscript(transcript, state);
                        const actions = parseEvent(cleaned, state);
                        if (this.onResultCallback) {
                            this.onResultCallback({ transcript, cleaned, actions });
                        }
                    } catch (err) {
                        if (this.onErrorCallback) {
                            this.onErrorCallback(err.message);
                        }
                    }
                });
            });
        }
    }
}
