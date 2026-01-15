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
 * Utility functions for the Skorekeeper PWA.
 */

/**
 * Generates a unique UUID (v4).
 * Uses crypto.randomUUID if available, otherwise falls back to a math-based generator.
 * @returns {string} A random UUID string.
 */
export function generateUUID() {
    if (typeof crypto !== 'undefined' && crypto.randomUUID) {
        return crypto.randomUUID();
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

/**
 * Sanitizes a string for safe HTML rendering by creating a text node.
 * @param {string} str - The string to sanitize.
 * @returns {string} The sanitized HTML string.
 */
export function sanitizeHTML(str) {
    if (!str) {
        return '';
    }
    const div = document.createElement('div');
    div.appendChild(document.createTextNode(str));
    return div.innerHTML;
}

/**
 * Formats a date string or timestamp into a localized date string.
 * @param {string|number} dateVal - The date to format.
 * @returns {string} Localized date string.
 */
export function formatDate(dateVal) {
    if (!dateVal) {
        return '';
    }
    return new Date(dateVal).toLocaleDateString();
}

/**
 * Creates a DOM element with specified attributes and children.
 * @param {string} tag - HTML tag name.
 * @param {object} [options] - Element options.
 * @param {string} [options.className] - CSS classes.
 * @param {string} [options.text] - Text content.
 * @param {string} [options.id] - Element ID.
 * @param {object} [options.dataset] - Data attributes.
 * @param {Function} [options.onClick] - Click handler.
 * @param {Array<HTMLElement>} [options.children] - Child elements to append.
 * @param {string} [options.type] - Element type (e.g. for input).
 * @param {string} [options.value] - Element value.
 * @param {boolean} [options.checked] - Checked state for inputs.
 * @param {string} [options.name] - Name attribute.
 * @returns {HTMLElement} The created element.
 */
export function createElement(tag, options = {}) {
    const el = document.createElement(tag);
    if (options.className) {
        el.className = options.className;
    }
    if (options.text) {
        el.textContent = options.text;
    }
    if (options.id) {
        el.id = options.id;
    }
    if (options.title) {
        el.title = options.title;
    }
    if (options.type) {
        el.type = options.type;
    }
    if (options.value !== undefined) {
        el.value = String(options.value);
    }
    if (options.checked !== undefined) {
        el.checked = options.checked;
    }
    if (options.name) {
        el.name = options.name;
    }
    if (options.dataset) {
        Object.keys(options.dataset).forEach(key => {
            el.dataset[key] = options.dataset[key];
        });
    }
    if (options.onClick) {
        el.onclick = options.onClick;
    }
    if (options.children) {
        options.children.forEach(child => {
            if (child) {
                el.appendChild(child);
            }
        });
    }
    return el;
}
