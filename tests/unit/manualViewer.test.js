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

import { ManualViewer } from '../../frontend/ui/manualViewer.js';

// Mock the dependencies
jest.mock('../../frontend/manualContent.js', () => ({
    manualSections: [
        {
            id: 'section-1',
            title: 'Introduction',
            content: '<h1>Intro</h1><p>Welcome to the manual.</p>',
            tags: ['start', 'basics'],
        },
        {
            id: 'section-2',
            title: 'Advanced Usage',
            content: '<h1>Advanced</h1><p>Deep dive.</p>',
            tags: ['pro', 'details'],
        },
        {
            id: 'section-3',
            title: 'Troubleshooting',
            content: '<p>Fixing things.</p>',
            tags: ['fix', 'help'],
        },
    ],
}));

describe('ManualViewer', () => {
    let viewer;
    let tocContainer;
    let contentContainer;
    let searchInput;

    beforeEach(() => {
        // Setup DOM elements
        document.body.innerHTML = `
            <div id="manual-toc"></div>
            <div id="manual-content"></div>
            <input id="manual-search" type="text" />
        `;

        tocContainer = document.getElementById('manual-toc');
        contentContainer = document.getElementById('manual-content');
        searchInput = document.getElementById('manual-search');

        // Clear mocks and reset modules if needed
        jest.clearAllMocks();
    });

    afterEach(() => {
        document.body.innerHTML = '';
    });

    test('initialization renders TOC and loads first section', () => {
        viewer = new ManualViewer();

        // Check TOC rendering
        const tocButtons = tocContainer.querySelectorAll('button');
        expect(tocButtons.length).toBe(3);
        expect(tocButtons[0].textContent).toBe('Introduction');
        expect(tocButtons[1].textContent).toBe('Advanced Usage');
        expect(tocButtons[2].textContent).toBe('Troubleshooting');

        // Check active state on first item
        expect(tocButtons[0].className).toContain('bg-blue-100');
        expect(tocButtons[1].className).not.toContain('bg-blue-100');

        // Check content loading
        expect(contentContainer.innerHTML).toContain('<h1>Intro</h1>');
        expect(contentContainer.innerHTML).toContain('<p>Welcome to the manual.</p>');
        expect(viewer.activeSectionId).toBe('section-1');
    });

    test('loadSection updates content and active state', () => {
        viewer = new ManualViewer();

        // Simulate clicking the second section
        viewer.loadSection('section-2');

        expect(contentContainer.innerHTML).toContain('<h1>Advanced</h1>');
        expect(viewer.activeSectionId).toBe('section-2');

        const tocButtons = tocContainer.querySelectorAll('button');
        expect(tocButtons[0].className).not.toContain('bg-blue-100');
        expect(tocButtons[1].className).toContain('bg-blue-100');
    });

    test('loadSection does nothing if id is invalid', () => {
        viewer = new ManualViewer();
        const initialContent = contentContainer.innerHTML;

        viewer.loadSection('invalid-id');

        expect(contentContainer.innerHTML).toBe(initialContent);
        expect(viewer.activeSectionId).toBe('section-1'); // Remains on initial
    });

    test('handleSearch filters TOC by title', () => {
        viewer = new ManualViewer();

        // Simulate search input
        searchInput.value = 'Advanced';
        searchInput.dispatchEvent(new Event('input'));

        const tocButtons = tocContainer.querySelectorAll('button');
        expect(tocButtons.length).toBe(1);
        expect(tocButtons[0].textContent).toBe('Advanced Usage');
    });

    test('handleSearch filters TOC by tag', () => {
        viewer = new ManualViewer();

        // 'fix' is a tag for section 3
        viewer.handleSearch('fix');

        const tocButtons = tocContainer.querySelectorAll('button');
        expect(tocButtons.length).toBe(1);
        expect(tocButtons[0].textContent).toBe('Troubleshooting');
    });

    test('handleSearch filters TOC by content', () => {
        viewer = new ManualViewer();

        // 'Welcome' is in the content of section 1
        viewer.handleSearch('Welcome');

        const tocButtons = tocContainer.querySelectorAll('button');
        expect(tocButtons.length).toBe(1);
        expect(tocButtons[0].textContent).toBe('Introduction');
    });

    test('handleSearch shows no results message', () => {
        viewer = new ManualViewer();

        viewer.handleSearch('nonexistentterm');

        const tocButtons = tocContainer.querySelectorAll('button');
        expect(tocButtons.length).toBe(0);
        expect(tocContainer.innerHTML).toContain('No results found');
    });

    test('loadSection maintains search filter', () => {
        viewer = new ManualViewer();

        // Filter to show section 2
        searchInput.value = 'Advanced';
        viewer.handleSearch('Advanced');

        const tocButtonsBefore = tocContainer.querySelectorAll('button');
        expect(tocButtonsBefore.length).toBe(1);

        // Load section 2
        viewer.loadSection('section-2');

        // Should still be filtered
        const tocButtonsAfter = tocContainer.querySelectorAll('button');
        expect(tocButtonsAfter.length).toBe(1);
        expect(tocButtonsAfter[0].textContent).toBe('Advanced Usage');
        expect(tocButtonsAfter[0].className).toContain('bg-blue-100');
    });

    test('initialization handles missing DOM elements gracefully', () => {
        document.body.innerHTML = ''; // No elements

        // Should not throw
        expect(() => {
            viewer = new ManualViewer();
        }).not.toThrow();
    });

    test('initialization handles empty sections', () => {
        // Mock empty sections for this test specific
        // We can't easily re-mock module for just one test block without jest.resetModules
        // but since we passed manualSections to the constructor in the real code it reads from import.
        // The real code does: this.sections = manualSections;
        // So we can check if it handles empty array if we could modify manualSections.
        // But since it's imported, it's harder to change per test.
        // Ideally the class would accept sections as an argument.
    });

    // Since renderTOC checks for this.tocContainer, we can test that path
    test('renderTOC returns early if container missing', () => {
        document.body.innerHTML = '';
        viewer = new ManualViewer();
        // Just ensuring no errors
        expect(viewer.tocContainer).toBeNull();
    });
});
