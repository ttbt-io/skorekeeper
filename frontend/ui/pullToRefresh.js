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

const PULL_RESISTANCE_FACTOR = 0.8;
const INDICATOR_HEIGHT = 40;
const INDICATOR_MAX_TOP_POSITION = 10;
const REFRESH_LOCK_POSITION = 50;
const INDICATOR_REFRESH_TOP_POSITION = 15;
const INDICATOR_HIDDEN_TOP_POSITION = -40;
const SNAP_BACK_DURATION_MS = 300;
const REFRESH_COMPLETION_DELAY_MS = 500;
const SLOP_THRESHOLD_PX = 10;

/**
 * Handles "Pull to Refresh" functionality for scrollable containers.
 */
export class PullToRefresh {
    /**
     * @param {HTMLElement} container - The scrollable container element.
     * @param {Function} onRefresh - Async function to call when refresh is triggered.
     * @param {object} options - Optional configuration.
     * @param {number} [options.threshold=70] - Drag distance in pixels to trigger refresh.
     * @param {string} [options.contentSelector] - Selector for the content element to translate. Defaults to first child.
     */
    constructor(container, onRefresh, options = {}) {
        this.container = container;
        this.onRefresh = onRefresh;
        this.threshold = options.threshold || 70;
        this.contentSelector = options.contentSelector;

        this.startY = 0;
        this.startX = 0;
        this.currentY = 0;
        this.isDragging = false;
        this.isRefreshing = false;

        this.content = null;
        this.indicator = null;

        this.init();
    }

    init() {
        // Ensure container is relative for absolute positioning of indicator
        if (window.getComputedStyle(this.container).position === 'static') {
            this.container.style.position = 'relative';
        }

        // Find the content wrapper.
        if (this.contentSelector) {
            this.content = this.container.querySelector(this.contentSelector);
        } else {
            this.content = this.container.firstElementChild;
        }

        if (!this.content) {
            console.warn('PullToRefresh: Container has no content child.');
            return;
        }

        // Create Refresh Indicator
        this.indicator = document.createElement('div');
        this.indicator.className = 'refresh-indicator';
        this.indicator.innerHTML = `
            <svg class="w-6 h-6 text-blue-600 transform transition-transform duration-200" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3"></path>
            </svg>
            <div class="refresh-spinner hidden animate-spin rounded-full h-5 w-5 border-2 border-blue-600 border-t-transparent"></div>
        `;
        this.container.prepend(this.indicator);

        // Bind Events
        this.container.addEventListener('touchstart', this.handleTouchStart.bind(this), { passive: true });
        this.container.addEventListener('touchmove', this.handleTouchMove.bind(this), { passive: false });
        this.container.addEventListener('touchend', this.handleTouchEnd.bind(this));
    }

    handleTouchStart(e) {
        if (this.container.scrollTop === 0 && !this.isRefreshing) {
            this.startY = e.touches[0].clientY;
            this.startX = e.touches[0].clientX;
            this.currentY = this.startY; // Initialize currentY to avoid jumps if no move occurs
            this.isDragging = true;
        } else {
            this.isDragging = false;
        }
    }

    handleTouchMove(e) {
        if (!this.isDragging || this.isRefreshing) {
            return;
        }

        this.currentY = e.touches[0].clientY;
        const currentX = e.touches[0].clientX;
        const diffY = this.currentY - this.startY;
        const diffX = Math.abs(currentX - this.startX);

        // 1. Horizontal Scroll Check: If moving more horizontally than vertically, abort pull.
        if (diffX > Math.abs(diffY)) {
            this.isDragging = false;
            return;
        }

        // 2. Only handle pull down from top
        if (diffY > 0 && this.container.scrollTop === 0) {
            // 3. Slop Check: Ignore tiny movements to preserve click events.
            // If we preventDefault on < 10px moves, simple taps can be killed.
            if (diffY < SLOP_THRESHOLD_PX) {
                return;
            }

            // Prevent default scroll behavior (overscroll)
            if (e.cancelable) {
                e.preventDefault();
            }

            // Apply resistance
            const pullDistance = Math.pow(diffY, PULL_RESISTANCE_FACTOR);

            // Translate content
            this.content.style.transform = `translateY(${pullDistance}px)`;

            // Move indicator into view
            this.indicator.style.top = `${Math.min(pullDistance - INDICATOR_HEIGHT, INDICATOR_MAX_TOP_POSITION)}px`;
            this.indicator.style.opacity = Math.min(1, pullDistance / this.threshold);

            // Rotate arrow based on distance
            const arrow = this.indicator.querySelector('svg');
            if (pullDistance > this.threshold) {
                arrow.style.transform = 'rotate(180deg)';
            } else {
                arrow.style.transform = 'rotate(0deg)';
            }
        } else {
            // Scrolled up or not at top
            this.isDragging = false;
        }
    }

    async handleTouchEnd() {
        if (!this.isDragging || this.isRefreshing) {
            return;
        }

        const diff = this.currentY - this.startY;

        // If movement was minimal (below slop), treat as no-op/click
        if (diff < SLOP_THRESHOLD_PX) {
            this.isDragging = false;
            // Ensure no lingering transforms from micro-moves
            this.content.style.transform = '';
            return;
        }

        const pullDistance = Math.pow(diff, PULL_RESISTANCE_FACTOR);
        this.isDragging = false;

        if (pullDistance > this.threshold) {
            // Trigger Refresh
            this.isRefreshing = true;
            this.content.style.transition = `transform ${SNAP_BACK_DURATION_MS/1000}s ease-out`;
            this.content.style.transform = `translateY(${REFRESH_LOCK_POSITION}px)`; // Lock at loading position

            this.indicator.style.transition = `top ${SNAP_BACK_DURATION_MS/1000}s ease-out`;
            this.indicator.style.top = `${INDICATOR_REFRESH_TOP_POSITION}px`;

            const arrow = this.indicator.querySelector('svg');
            const spinner = this.indicator.querySelector('.refresh-spinner');
            arrow.classList.add('hidden');
            spinner.classList.remove('hidden');

            try {
                await this.onRefresh();
            } catch (err) {
                console.error('Refresh failed', err);
            } finally {
                this.complete();
            }
        } else {
            // Snap back
            this.snapBack();
        }
    }

    snapBack() {
        this.content.style.transition = `transform ${SNAP_BACK_DURATION_MS/1000}s ease-out`;
        this.content.style.transform = 'translateY(0)';
        this.indicator.style.transition = `top ${SNAP_BACK_DURATION_MS/1000}s ease-out, opacity ${SNAP_BACK_DURATION_MS/1000}s`;
        this.indicator.style.top = `${INDICATOR_HIDDEN_TOP_POSITION}px`;
        this.indicator.style.opacity = '0';

        setTimeout(() => {
            this.content.style.transition = '';
            this.indicator.style.transition = '';
            this.content.style.transform = '';
        }, SNAP_BACK_DURATION_MS);
    }

    complete() {
        // Wait a small moment to show completion
        setTimeout(() => {
            const arrow = this.indicator.querySelector('svg');
            const spinner = this.indicator.querySelector('.refresh-spinner');
            arrow.classList.remove('hidden');
            arrow.style.transform = 'rotate(0deg)';
            spinner.classList.add('hidden');

            this.isRefreshing = false;
            this.snapBack();
        }, REFRESH_COMPLETION_DELAY_MS);
    }
}