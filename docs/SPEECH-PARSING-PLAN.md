# Speech and Natural Language Parsing Plan

This document outlines the incremental implementation of a natural language and speech-to-intent system for the Scorekeeper PWA. The goal is to allow users to record complex baseball plays via voice or text with high fidelity and low error rates.

## Core Philosophy: Deterministic Intent
To ensure the high fidelity required for a scoring application, we will avoid non-deterministic black-box LLMs. Instead, we will build a **Domain-Specific Language (DSL)** parser that uses the current game state to resolve ambiguity.

---

## Phase 1: Foundation - The DSL Grammar
**Goal:** Create a text-based parser that handles unambiguous single-event commands.

### Implementation Steps
1.  **Select Parser Library:** Use `nearley.js` for its ability to handle context-free grammars (CFG) and provide clear syntax errors.
2.  **Build Pipeline Integration:** Add `nearley` and `nearleyc` to `package.json`. Create a build script (e.g., `npm run build:parser`) to compile `.ne` grammar files into JavaScript utility files during the standard build process.
3.  **Define Tokenizer:** Create a lexicon of baseball terms (Ball, Strike, Foul, Out, Single, Double, Triple, Home Run, Grounder, Flyball).
4.  **Draft Initial Grammar:**
    *   `PITCH -> "ball" | "strike" | "foul" | "swinging strike"`
    *   `BIP_RESULT -> "single" | "double" | "triple" | "homerun"`
    *   `OUT -> "fly out" | "ground out" | "strikeout"`
5.  **Action Mapper:** Write a utility that maps parser output to Scorekeeper `Action` objects (e.g., `{ type: 'PITCH', outcome: 'ball' }`).

### Testing Plan
*   **Unit Tests:** Create `tests/unit/parser.test.js`.
*   **Test Cases:**
    *   Input: "ball" -> Result: `PITCH` action.
    *   Input: "strike" -> Result: `PITCH` action.
    *   Input: "single" -> Result: `BIP` action with `result: '1B'`.
    *   Verify that invalid strings (e.g., "touchdown") throw a syntax error.

---

## Phase 2: Sequential Parsing, Event Chaining, & Substitutions
**Goal:** Support multi-event plays separated by delimiters, as well as roster substitutions via name or jersey number.

### Implementation Steps
1.  **Delimiter Handling:** Modify the parser to split input strings by `","`, `"and"`, or `"then"`.
2.  **Recursive Grammar:** Update the `nearley` grammar to support a `PLAY` being a list of `EVENT`s.
3.  **Intermediate State Simulation:** Leverage the existing `reducer.js`. When parsing a sequence, apply the action from Fragment A to a clone of the current state, and pass this *new* virtual state to parse Fragment B. This ensures 100% parity with actual game logic.
4.  **Syntax Expansion (Gameplay):**
    *   `RUNNER_ACTION -> "runner to" BASE`
    *   `BASE -> "second" | "third" | "home"`
    *   `ERROR -> "overthrow" | "error"`
5.  **Syntax Expansion (Substitutions & Jersey Numbers):**
    *   `PLAYER_REF -> NAME | "number" NUMBER | NUMBER`
    *   `SUBSTITUTION -> "pinch runner" PLAYER_REF "for" PLAYER_REF`
    *   `PITCHING_CHANGE -> "pitching change" PLAYER_REF "to pitch"`

### Testing Plan
*   **Sequential Tests:**
    *   Input: "ball, runner steals second" -> Result: Two actions.
    *   Input: "single and runner to third" -> Result: `BIP` action + `RUNNER_ADVANCE` action.
*   **Substitution Tests:**
    *   Input: "pinch runner 5 for 11" -> Result: `SUBSTITUTION` action resolving jersey numbers to player IDs.
    *   Input: "number 15 is now pitching" -> Result: `PITCHING_CHANGE` action.
*   **Integrity Tests:** Ensure that if one part of a sequence fails, the entire sequence is rejected to prevent partial state corruption.

---

## Phase 3: Context-Aware Entity Resolution
**Goal:** Resolve ambiguous terms like "runner", "scores", and map spoken numbers to specific player IDs using the current game state.

### Implementation Steps
1.  **State Injection:** The `parse(text, gameState)` function accepts the current state (from `reducer.js`).
2.  **Runner/Player Resolution Logic:**
    *   If input is "runner scores" and only one runner is on base, identify that runner's ID.
    *   If multiple runners are on, and the input doesn't specify, flag as "Ambiguous" and prompt for clarification.
    *   If input references a player by jersey number (e.g., "number 5"), scan the `gameState` active roster to resolve the exact player ID.
3.  **Validation Checks:**
    *   Reject "runner to second" if no runners are on base.
    *   Reject a substitution if the referenced jersey number is not on the roster.

### Testing Plan
*   **State-Dependent Tests:**
    *   Mock state with runner on 1st. Input: "runner to second". Verify correct runner ID is used.
    *   Mock state with bases empty. Input: "runner to second". Verify error: "No runners on base".
*   **Conflict Tests:**
    *   Mock state with runners on 1st and 2nd. Input: "runner to third". Verify error: "Multiple runners on base, please specify".

---

## Phase 4: Speech-to-Text Bridge & Fuzzy Matching
**Goal:** Connect the browser's microphone and handle the inherent inaccuracies of speech recognition.

### Implementation Steps
1.  **Web Speech API Integration:** Create a `SpeechManager.js` service to handle `window.SpeechRecognition`.
2.  **Fuzzy Correction Layer:** 
    *   Integrate `Fuse.js`.
    *   Define a "Baseball Lexicon" for fuzzy matching (e.g., "Boll" -> "Ball", "Short stop" -> "Shortstop").
    *   Map spoken numbers ("one", "first") to numerical/ordinal constants. This is crucial for base numbers AND jersey numbers (e.g., correcting "number to" to "number two").
3.  **Roster Matching:** Dynamically add current player names and jersey numbers to the fuzzy matcher's dictionary.
4.  **Cleaning Pipeline:** Pre-process transcripts (lowercase, remove punctuation, fuzzy match keywords) before passing to the Phase 3 parser.

### Testing Plan
*   **Fuzzy Logic Tests:**
    *   Input: "Strike won" -> Result: "Strike one" (corrected).
    *   Input: "Grounder to short stop" -> Result: "Grounder to shortstop" (corrected).
    *   Input: "Pinch runner for number too" -> Result: "Pinch runner for number 2" (corrected).
*   **Integration Logging:** Add a "Voice Debug" mode that logs: `[Raw Transcript] -> [Cleaned Text] -> [Parsed Actions]`.

---

## Phase 5: Staging UI & Verification Loop
**Goal:** Provide a safety barrier where users must confirm parsed actions before they are committed to the `actionLog`.

### Implementation Steps
1.  **Pending Action Store:** Create a temporary state in `AppController.js` to hold "Staged" actions.
2.  **Staging UI Component:**
    *   Design a "Speech Preview" bar at the bottom of the screen.
    *   Display parsed actions as human-readable chips (e.g., `[Pitch: Ball]`, `[BIP: Single]`, `[Sub: #5 for #11]`).
3.  **Voice Feedback (Speech Synthesis):**
    *   Integrate `window.speechSynthesis` to read back the *parsed results*.
    *   Example: "Staged: Ball, runner to second. Say 'Confirm' or tap to save."
    *   Add a toggle in Settings to enable/disable "Voice Readback."
4.  **Commit/Cancel Logic:**
    *   "Confirm" button (or voice command "Confirm"): Appends all staged actions to the store and clears the staging area.
    *   "Cancel" button (or voice command "Cancel"): Clears the staging area.
5.  **Haptic/Visual Feedback:** Provide a subtle vibration or flash when a phrase is successfully parsed.

### Testing Plan
*   **Speech Synthesis Test:** Verify that the correct action labels are read aloud after a phrase is parsed.
*   **E2E Workflow:**
    1. Activate voice input.
    2. Speak a phrase.
    3. Verify preview bar appears AND voice feedback is heard.
    4. Click "Cancel" -> Verify game state is unchanged.
    5. Speak phrase again -> Click "Confirm" -> Verify game state updates.
*   **Conflict Resolution:** Verify that if a manual action is taken while a voice action is staged, the staged action is either cleared or validated against the new state.

---

## Phase 6: Natural Language Refinement
**Goal:** Support more varied phrasing and "filler" words to make the system feel "natural" while maintaining strict logic.

### Implementation Steps
1.  **Compromise.js Integration:** Use `compromise` to identify and strip non-essential parts of speech (conjunctions, articles, pronouns).
2.  **Intent Expansion:** Support phrases like:
    *   "That's a ball" -> "ball"
    *   "He's safe at first" -> "runner to first"
    *   "Got him at second" -> "out at second"
    *   "Number 15 is now pitching" -> "pitching change 15 to pitch"
3.  **Contextual "Out" Mapping:** 
    *   If a BIP is recorded and user says "Out at first", resolve the runner.
4.  **Performance Optimization:** Move the parser and fuzzy matcher to a **Web Worker** to ensure the main UI thread stays at 60fps during speech processing.

### Testing Plan
*   **Natural Language Tests:**
    *   "He hit a double" -> `BIP: 2B`.
    *   "That was low for a ball" -> `PITCH: ball`.
*   **Performance Benchmarking:** Measure parsing latency on low-end mobile devices; target <100ms for "Clean -> Parse -> Stage".

---

## Future Considerations
*   **Offline Support:** Investigate `Vosk` or `TensorFlow.js` for on-device speech-to-text to eliminate reliance on the browser's native API (which often requires a connection).
*   **Multi-Language Support:** Allow the lexicon and grammar to be swapped for Spanish, Japanese, or Korean.
