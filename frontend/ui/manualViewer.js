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

import { manualSections } from '../manualContent.js';
import { createElement } from '../utils.js';

/**
 * Manages the User Manual view, including navigation, rendering, and search.
 */
export class ManualViewer {
    constructor() {
        this.sections = manualSections;
        this.tocContainer = document.getElementById('manual-toc');
        this.contentContainer = document.getElementById('manual-content');
        this.searchInput = document.getElementById('manual-search');

        this.activeSectionId = null;

        this.bindEvents();
        this.renderTOC(this.sections);
        // Default to first section if available
        if (this.sections.length > 0) {
            this.loadSection(this.sections[0].id);
        }
    }

    bindEvents() {
        if (this.searchInput) {
            this.searchInput.addEventListener('input', (e) => this.handleSearch(e.target.value));
        }
    }

    /**
     * Renders the Table of Contents sidebar.
     * @param {Array} sections - The list of sections to display.
     */
    renderTOC(sections) {
        if (!this.tocContainer) {
            return;
        }
        this.tocContainer.innerHTML = '';

        if (sections.length === 0) {
            this.tocContainer.appendChild(createElement('div', {
                className: 'p-3 text-xs text-gray-500',
                text: 'No results found.',
            }));
            return;
        }

        sections.forEach(s => {
            const btn = document.createElement('button');
            // Base classes
            let cls = 'w-full text-left px-3 py-2 rounded text-sm font-medium transition-colors mb-1 ';
            // Active state
            if (s.id === this.activeSectionId) {
                cls += 'bg-blue-100 text-blue-700 font-bold shadow-sm';
            } else {
                cls += 'text-gray-700 hover:bg-white hover:shadow-sm';
            }
            btn.className = cls;
            btn.textContent = s.title;
            btn.onclick = () => this.loadSection(s.id);
            this.tocContainer.appendChild(btn);
        });
    }

    /**
     * Filters the TOC based on search input.
     * @param {string} term
     */
    handleSearch(term) {
        const lower = term.toLowerCase();
        const filtered = this.sections.filter(s =>
            s.title.toLowerCase().includes(lower) ||
            (s.tags && s.tags.some(t => t.includes(lower))) ||
            (s.content && s.content.toLowerCase().includes(lower)),
        );
        this.renderTOC(filtered);
    }

    /**
     * Loads a specific section into the main content area.
     * @param {string} id
     */
    loadSection(id) {
        const section = this.sections.find(s => s.id === id);
        if (!section) {
            return;
        }

        this.activeSectionId = id;

        // Re-render TOC to update active state highlighting
        // (Optimally we'd just update classes, but this is fast enough)
        // We pass the *current filtered list* if we want to persist search results?
        // For simplicity, let's reset search or just re-render full list?
        // Let's re-render based on current search input value.
        const currentSearch = this.searchInput ? this.searchInput.value : '';
        this.handleSearch(currentSearch);

        if (this.contentContainer) {
            this.contentContainer.innerHTML = '';
            const parser = new DOMParser();
            const doc = parser.parseFromString(section.content, 'text/html');
            while (doc.body.firstChild) {
                this.contentContainer.appendChild(doc.body.firstChild);
            }
            this.contentContainer.scrollTop = 0;
        }
    }
}
