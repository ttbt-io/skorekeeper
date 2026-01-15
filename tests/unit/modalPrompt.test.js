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

import '@testing-library/jest-dom';
import { modalPrompt } from '../../frontend/ui/modalPrompt.js';

describe('modalPrompt', () => {
    let originalBody;

    beforeEach(() => {
    // Store the original document body
        originalBody = document.body.cloneNode(true);
        // Clean up the DOM before each test
        document.body.innerHTML = '';
    });

    afterEach(() => {
    // Restore the original document body
        document.body = originalBody;
        // Ensure any remaining modals are removed
        const modal = document.getElementById('custom-prompt-modal');
        if (modal) {
            modal.remove();
        }
    });

    test('should display a modal prompt with the given message and default value', async() => {
        const message = 'Enter your name:';
        const defaultValue = 'John Doe';

        // Call modalPrompt but don't await it yet, as it's a Promise
        const promptPromise = modalPrompt(message, defaultValue);

        // Allow a short delay for the DOM to update
        await Promise.resolve();

        const modal = document.getElementById('custom-prompt-modal');
        expect(modal).toBeInTheDocument();
        expect(modal.querySelector('p').textContent).toBe(message);

        const input = modal.querySelector('input[type="text"]');
        expect(input).toBeInTheDocument();
        expect(input.value).toBe(defaultValue);
        expect(input).toHaveFocus();

        // Resolve the promise by simulating a click (the afterEach will clean up)
        modal.querySelector('[data-test="custom-prompt-cancel-btn"]').click();
        await promptPromise; // Await the resolution
    });

    test('should resolve the promise with the input value when OK is clicked', async() => {
        const message = 'Enter your age:';
        const defaultValue = '30';
        const inputValue = '42';

        const promptPromise = modalPrompt(message, defaultValue);

        await Promise.resolve(); // Allow DOM update

        const modal = document.getElementById('custom-prompt-modal');
        const input = modal.querySelector('input[type="text"]');
        const okButton = modal.querySelector('[data-test="custom-prompt-ok-btn"]');

        // Simulate user typing
        input.value = inputValue;
        okButton.click();

        const result = await promptPromise;
        expect(result).toBe(inputValue);
        expect(modal).not.toBeInTheDocument();
    });

    test('should resolve the promise with null when Cancel is clicked', async() => {
        const message = 'Are you sure?';
        const defaultValue = 'No';

        const promptPromise = modalPrompt(message, defaultValue);

        await Promise.resolve(); // Allow DOM update

        const modal = document.getElementById('custom-prompt-modal');
        const cancelButton = modal.querySelector('[data-test="custom-prompt-cancel-btn"]');

        cancelButton.click();

        const result = await promptPromise;
        expect(result).toBeNull();
        expect(modal).not.toBeInTheDocument();
    });

    test('should replace an existing modal if called again', async() => {
        modalPrompt('First prompt');
        await Promise.resolve();
        const firstModal = document.getElementById('custom-prompt-modal');
        expect(firstModal).toBeInTheDocument();

        // Call modalPrompt again
        const secondPromptPromise = modalPrompt('Second prompt');
        await Promise.resolve();
        const secondModal = document.getElementById('custom-prompt-modal');

        expect(firstModal).not.toBeInTheDocument(); // First modal should be removed
        expect(secondModal).toBeInTheDocument();
        expect(secondModal.querySelector('p').textContent).toBe('Second prompt');

        // Clean up the second modal
        secondModal.querySelector('[data-test="custom-prompt-cancel-btn"]').click();
        await secondPromptPromise;
    });
});
