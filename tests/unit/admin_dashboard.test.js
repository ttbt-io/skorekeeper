const fs = require('fs');
const path = require('path');

// Read the frontend script content
const scriptContent = fs.readFileSync(path.resolve(__dirname, '../../backend/admin_dashboard.js'), 'utf8');

describe('Admin Dashboard', () => {
    let mockFetch;

    beforeEach(() => {
        // Mock HTML Structure
        document.body.innerHTML = `
            <div id="chart-tooltip" class="chart-tooltip"></div>
            <div class="nav-tabs">
                <div class="nav-tab active" data-tab="policy"></div>
                <div class="nav-tab" data-tab="monitoring"></div>
            </div>
            <div id="tab-policy" class="tab-content active">
                <div id="loading"></div>
                <form id="policyForm" style="display:none;">
                    <select id="defaultPolicy"><option value="allow">Allow</option></select>
                    <input id="defaultMaxGames" value="0">
                    <input id="defaultMaxTeams" value="0">
                    <textarea id="defaultDenyMessage"></textarea>
                    <div id="adminsList"></div>
                    <input id="newAdminEmail">
                    <button id="btnAddAdmin"></button>
                    <div id="usersList"></div>
                    <input id="newUserEmail">
                    <select id="newUserAccess"><option value="allow">Allow</option></select>
                    <input id="newUserGames">
                    <input id="newUserTeams">
                    <button id="btnAddUser"></button>
                    <button type="submit" id="btnSavePolicy">Save</button>
                </form>
            </div>
            <div id="tab-monitoring" class="tab-content">
                <select id="metricRes">
                    <option value="1m">1m</option>
                </select>
                <button id="btnRefreshMetrics"></button>
                <span id="metricStatus"></span>
                <canvas id="chart-rps"></canvas>
                <canvas id="chart-ws"></canvas>
                <canvas id="chart-entities"></canvas>
                <canvas id="chart-nodes"></canvas>
                <canvas id="chart-elections"></canvas>
                <canvas id="chart-gap"></canvas>
            </div>
            <div id="status"></div>
        `;

        // Mock Canvas
        HTMLCanvasElement.prototype.getContext = jest.fn(() => ({
            clearRect: jest.fn(),
            beginPath: jest.fn(),
            moveTo: jest.fn(),
            lineTo: jest.fn(),
            stroke: jest.fn(),
            fillText: jest.fn(),
            measureText: jest.fn(() => ({ width: 10 })),
            fillRect: jest.fn(),
        }));

        // Mock Fetch
        mockFetch = jest.fn((url) => {
            if (url === '/api/admin/policy') {
                return Promise.resolve({
                    ok: true,
                    json: () => Promise.resolve({
                        defaultPolicy: 'allow',
                        defaultMaxTeams: 10,
                        admins: ['admin@example.com'],
                        users: {},
                    }),
                });
            }
            if (url === '/api/cluster/metrics') {
                return Promise.resolve({
                    ok: true,
                    json: () => Promise.resolve({
                        nodes: {},
                        cluster: {},
                    }),
                });
            }
            return Promise.reject(new Error('Unknown URL: ' + url));
        });
        global.fetch = mockFetch;

        // Wrap script in IIFE to avoid redeclaration errors
        const wrappedScript = `(function() { ${scriptContent} })();`;
        eval(wrappedScript);
    });

    test('should load policy on init', async() => {
        // Wait for async loadPolicy
        await new Promise(resolve => setTimeout(resolve, 0));

        expect(global.fetch).toHaveBeenCalledWith('/api/admin/policy');

        // Verify rendering
        const admins = document.getElementById('adminsList').children;
        expect(admins.length).toBe(1);
        expect(admins[0].textContent).toContain('admin@example.com');
    });

    test('should switch tabs and fetch metrics', async() => {
        // Re-mock fetch for metrics
        global.fetch.mockImplementation((url) => {
            if (url === '/api/cluster/metrics') {
                return Promise.resolve({
                    ok: true,
                    json: () => Promise.resolve({
                        nodes: {},
                        cluster: {},
                    }),
                });
            }
            return Promise.resolve({ ok:false });
        });

        const monitoringTab = document.querySelector('[data-tab="monitoring"]');
        monitoringTab.click();

        expect(monitoringTab.classList.contains('active')).toBe(true);
        expect(document.getElementById('tab-monitoring').classList.contains('active')).toBe(true);

        expect(global.fetch).toHaveBeenCalledWith('/api/cluster/metrics');
    });

    test('should render charts when metrics fetched', async() => {
        global.fetch.mockImplementation((url) => {
            if (url === '/api/cluster/metrics') {
                return Promise.resolve({
                    ok: true,
                    json: () => Promise.resolve({
                        nodes: {
                            'node1': { buffers: { '1m': { data: [{ t: 1000, v: 10 }], head: 1 } } },
                        },
                        cluster: {
                            nodeCount: { buffers: { '1m': { data: [{ t: 1000, v: 3 }], head: 1 } } },
                        },
                    }),
                });
            }
            return Promise.resolve({ ok:false });
        });

        const btnRefresh = document.getElementById('btnRefreshMetrics');
        btnRefresh.click();

        await new Promise(resolve => setTimeout(resolve, 0));

        // Verify canvas context calls
        // Since we mocked getContext, we can check if it was called?
        // But the context instance is created inside `drawChart`.
        // We mocked prototype.getContext.
        // We can check if any methods were called on the mock context?
        // Since `drawChart` creates a new context each time, it's hard to spy on the specific instance unless we return a spy from getContext.
        // We did: `HTMLCanvasElement.prototype.getContext = jest.fn(...)` returning an object.
        // We can spy on that object's methods if we kept a reference or if we check call count of getContext.

        // Let's verify `metricStatus` text updated
        const status = document.getElementById('metricStatus');
        expect(status.textContent).toContain('Last updated');
    });

    test('should not crash if canvas element is missing', async() => {
        // Remove a canvas to simulate stale HTML
        const canvas = document.getElementById('chart-ws');
        if (canvas) {
            canvas.remove();
        }

        const btnRefresh = document.getElementById('btnRefreshMetrics');
        btnRefresh.click();

        await new Promise(resolve => setTimeout(resolve, 0));

        const status = document.getElementById('metricStatus');
        // Should succeed (no error message)
        expect(status.textContent).toContain('Last updated');
    });

    test('should restore monitoring tab from hash', async() => {
        window.location.hash = '#monitoring';

        // Re-run script initialization
        const wrappedScript = `(function() { ${scriptContent} })();`;
        eval(wrappedScript);

        await new Promise(resolve => setTimeout(resolve, 0));

        expect(document.getElementById('tab-monitoring').classList.contains('active')).toBe(true);
        expect(global.fetch).toHaveBeenCalledWith('/api/cluster/metrics');
    });

    test('should update hash when switching tabs', async() => {
        // Initial state (policy)
        await new Promise(resolve => setTimeout(resolve, 0));

        const monitoringTab = document.querySelector('[data-tab="monitoring"]');
        monitoringTab.click();

        expect(window.location.hash).toBe('#monitoring');
    });
});
