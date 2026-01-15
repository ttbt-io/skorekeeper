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

// frontend/modalPrompt.js

/**
 * Displays a custom modal prompt to the user and returns a Promise that resolves with the input value.
 *
 * @param {string} message - The message to display in the prompt.
 * @param {string} defaultValue - The default value for the input field.
 * @returns {Promise<string|null>} A Promise that resolves with the user's input string, or null if canceled.
 */
export function modalPrompt(message, defaultValue = '', options = null) {
    return new Promise((resolve) => {
        const modalId = 'custom-prompt-modal';
        let modal = document.getElementById(modalId);

        if (modal) {
            modal.remove(); // Remove any existing modal to prevent duplicates
        }

        // Create modal container
        modal = document.createElement('div');
        modal.id = modalId;
        modal.className = 'fixed inset-0 z-[70] flex justify-center items-center';

        const backdrop = document.createElement('div');
        backdrop.className = 'absolute inset-0 modal-backdrop-blur';
        modal.appendChild(backdrop);

        // Create modal content
        const modalContent = document.createElement('div');
        modalContent.className = 'relative z-10 p-5 border w-96 shadow-lg rounded-md bg-white max-h-[80vh] flex flex-col';

        // Message
        const messageEl = document.createElement('p');
        messageEl.className = 'text-gray-800 text-lg font-semibold mb-4';
        messageEl.textContent = message;

        modalContent.appendChild(messageEl);

        let inputEl = null;

        if (options && Array.isArray(options) && options.length > 0) {
            // Render Options List
            const optionsContainer = document.createElement('div');
            optionsContainer.id = 'custom-prompt-options';
            optionsContainer.className = 'flex flex-col gap-2 overflow-y-auto mb-4 flex-grow';

            options.forEach(opt => {
                const btn = document.createElement('button');
                btn.className = 'text-left px-4 py-3 bg-gray-100 hover:bg-gray-200 rounded text-gray-800 font-medium border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500';
                btn.textContent = opt.label;
                btn.onclick = () => {
                    modal.remove();
                    resolve(opt.value);
                };
                optionsContainer.appendChild(btn);
            });
            modalContent.appendChild(optionsContainer);
        } else {
            // Render Text Input
            inputEl = document.createElement('input');
            inputEl.type = 'text';
            inputEl.className = 'w-full p-2 border border-gray-300 rounded-md mb-4 focus:outline-none focus:ring-2 focus:ring-blue-500';
            inputEl.value = defaultValue;
            inputEl.placeholder = 'Enter value...';
            inputEl.setAttribute('data-test', 'custom-prompt-input'); // For testing

            // Focus on enter key
            inputEl.onkeyup = (e) => {
                if (e.key === 'Enter') {
                    const resultValue = inputEl.value;
                    modal.remove();
                    resolve(resultValue);
                }
            };
            modalContent.appendChild(inputEl);
        }

        // Button container
        const buttonContainer = document.createElement('div');
        buttonContainer.className = 'flex justify-end space-x-2 mt-auto';

        // Cancel button
        const cancelButton = document.createElement('button');
        cancelButton.id = 'btn-prompt-cancel';
        cancelButton.className = 'px-4 py-2 bg-gray-300 text-gray-800 text-base font-medium rounded-md shadow-sm hover:bg-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-400';
        cancelButton.textContent = 'Cancel';
        cancelButton.setAttribute('data-test', 'custom-prompt-cancel-btn'); // For testing
        cancelButton.onclick = () => {
            modal.remove();
            resolve(null);
        };
        buttonContainer.appendChild(cancelButton);

        // OK button (Only show for input mode, options act as immediate selection)
        if (!options) {
            const okButton = document.createElement('button');
            okButton.id = 'btn-prompt-ok';
            okButton.className = 'px-4 py-2 bg-blue-500 text-white text-base font-medium rounded-md shadow-sm hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500';
            okButton.textContent = 'OK';
            okButton.setAttribute('data-test', 'custom-prompt-ok-btn'); // For testing
            okButton.onclick = () => {
                const resultValue = inputEl ? inputEl.value : null;
                modal.remove();
                resolve(resultValue);
            };
            buttonContainer.appendChild(okButton);
        }

        modalContent.appendChild(buttonContainer);
        modal.appendChild(modalContent);

        document.body.appendChild(modal);
        if (inputEl) {
            inputEl.focus();
        }
    });
}

/**
 * Displays a custom modal confirmation to the user and returns a Promise that resolves with a boolean.
 *
 * @param {string} message - The message to display in the confirmation.
 * @returns {Promise<boolean>} A Promise that resolves with true if confirmed, or false if canceled.
 */
export function modalConfirm(message, options = {}) {
    return new Promise((resolve) => {
        const modalId = 'custom-confirm-modal';
        let modal = document.getElementById(modalId);

        if (modal) {
            modal.remove(); // Remove any existing modal to prevent duplicates
        }

        // Create modal container
        modal = document.createElement('div');
        modal.id = modalId;
        modal.className = 'fixed inset-0 z-[70] flex justify-center items-center';

        const backdrop = document.createElement('div');
        backdrop.className = 'absolute inset-0 modal-backdrop-blur';
        modal.appendChild(backdrop);

        // Create modal content
        const modalContent = document.createElement('div');
        modalContent.className = 'relative z-10 p-5 border w-96 shadow-lg rounded-md bg-white';

        // Message
        const messageEl = document.createElement('p');
        messageEl.className = 'text-gray-800 text-lg font-semibold mb-6';
        if (options.isError) {
            messageEl.className += ' text-red-600';
        }
        messageEl.textContent = message;

        // Button container
        const buttonContainer = document.createElement('div');
        buttonContainer.className = 'flex justify-end space-x-2';

        // OK button
        const okButton = document.createElement('button');
        okButton.id = 'btn-confirm-yes';
        okButton.className = 'px-4 py-2 bg-blue-500 text-white text-base font-medium rounded-md shadow-sm hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500';
        okButton.textContent = options.okText || 'OK';
        okButton.setAttribute('data-test', 'custom-confirm-ok-btn'); // For testing
        okButton.onclick = () => {
            modal.remove();
            resolve(true);
        };

        // Cancel button
        const cancelButton = document.createElement('button');
        cancelButton.id = 'btn-confirm-no';
        cancelButton.className = 'px-4 py-2 bg-gray-300 text-gray-800 text-base font-medium rounded-md shadow-sm hover:bg-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-400';
        cancelButton.textContent = options.cancelText || 'Cancel';
        cancelButton.setAttribute('data-test', 'custom-confirm-cancel-btn'); // For testing
        cancelButton.onclick = () => {
            modal.remove();
            resolve(false);
        };

        buttonContainer.appendChild(cancelButton);
        buttonContainer.appendChild(okButton);

        modalContent.appendChild(messageEl);
        modalContent.appendChild(buttonContainer);
        modal.appendChild(modalContent);

        document.body.appendChild(modal);
        okButton.focus(); // Focus the OK button by default
    });
}
