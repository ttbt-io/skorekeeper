const fs = require('fs');
const path = require('path');

// Read the frontend script content
const scriptContent = fs.readFileSync(path.resolve(__dirname, '../../backend/cluster_dashboard.js'), 'utf8');

describe('Cluster Dashboard', () => {
    let mockFetch;
    let mockAlert;
    let mockConfirm;

    beforeEach(() => {
        // Mock HTML Structure
        document.body.innerHTML = `
            <div id="login-section" class="card">
                <div id="login-error" class="error hidden"></div>
                <input type="password" id="secret" value="">
                <button type="button" id="btn-access-dashboard"></button>
            </div>
            <div id="dashboard-section" class="hidden">
                <div id="cluster-info"></div>
                <button type="button" id="refresh-btn"></button>
                <button type="button" id="show-add-node-btn"></button>
                <table><tbody id="nodes-table-body"></tbody></table>
                <div id="add-node-form-container" class="hidden">
                    <div id="add-error" class="hidden"></div>
                    <form id="add-node-form">
                        <input id="http-addr" value="">
                        <input id="pub-key" value="">
                        <input type="checkbox" id="non-voter">
                        <button type="button" id="btn-join-node"></button>
                        <button type="button" id="cancel-add-btn"></button>
                    </form>
                </div>
            </div>
        `;

        // Mock Browser APIs
        mockAlert = jest.fn();
        mockConfirm = jest.fn();
        window.alert = mockAlert;
        window.confirm = mockConfirm;

        // Mock Fetch
        mockFetch = jest.fn();
        global.fetch = mockFetch;

        // Default mock implementation for fetch
        mockFetch.mockImplementation((url, options) => {
            if (url === '/api/cluster/status') {
                const secret = options.headers['X-Raft-Secret'];
                if (secret === 'correct-secret') {
                    return Promise.resolve({
                        ok: true,
                        json: () => Promise.resolve({
                            nodeId: 'node1',
                            state: 'Leader',
                            leaderId: 'node1',
                            leaderAddr: '127.0.0.1:9090',
                            nodes: [
                                { id: 'node1', raftAddr: '127.0.0.1:8081', httpAddr: '127.0.0.1:9090', suffrage: 'Voter' },
                                { id: 'node2', raftAddr: '127.0.0.1:8082', httpAddr: '127.0.0.1:9091', suffrage: 'Voter' },
                            ],
                        }),
                    });
                } else {
                    return Promise.resolve({ status: 403, ok: false });
                }
            }
            if (url === '/api/cluster/join') {
                return Promise.resolve({ ok: true });
            }
            if (url === '/api/cluster/remove') {
                return Promise.resolve({ ok: true });
            }
            return Promise.reject(new Error('Unknown URL: ' + url));
        });

        // Run the script
        const wrappedScript = `(function() { ${scriptContent} })();`;
        eval(wrappedScript);
    });

    test('should show login error on invalid secret', async() => {
        document.getElementById('secret').value = 'wrong-secret';
        document.getElementById('btn-access-dashboard').click();

        await new Promise(resolve => setTimeout(resolve, 0));

        const loginError = document.getElementById('login-error');
        expect(loginError.classList.contains('hidden')).toBe(false);
        expect(loginError.textContent).toContain('Invalid Secret');
        expect(document.getElementById('dashboard-section').classList.contains('hidden')).toBe(true);
    });

    test('should load dashboard on valid secret', async() => {
        document.getElementById('secret').value = 'correct-secret';
        document.getElementById('btn-access-dashboard').click();

        await new Promise(resolve => setTimeout(resolve, 0));

        expect(document.getElementById('login-section').classList.contains('hidden')).toBe(true);
        expect(document.getElementById('dashboard-section').classList.contains('hidden')).toBe(false);

        // Verify cluster info
        const info = document.getElementById('cluster-info');
        expect(info.textContent).toContain('My Node ID: node1');
        expect(info.textContent).toContain('Leader: node1');

        // Verify nodes table
        const rows = document.getElementById('nodes-table-body').children;
        expect(rows.length).toBe(2);
        expect(rows[0].textContent).toContain('node1');
        expect(rows[1].textContent).toContain('node2');
    });

    test('should show add node form', () => {
        // Need to login first to see dashboard elements usually, but logic attaches listener anyway
        // Wait, element visibility is managed by CSS classes.

        document.getElementById('show-add-node-btn').click();
        const formContainer = document.getElementById('add-node-form-container');
        expect(formContainer.classList.contains('hidden')).toBe(false);

        document.getElementById('cancel-add-btn').click();
        expect(formContainer.classList.contains('hidden')).toBe(true);
    });

    test('should send join request', async() => {
        // Setup authenticated state (internal variable `raftSecret` needs to be set)
        // We do this by triggering login flow
        document.getElementById('secret').value = 'correct-secret';
        document.getElementById('btn-access-dashboard').click();
        await new Promise(resolve => setTimeout(resolve, 0));

        // Fill form
        document.getElementById('http-addr').value = 'https://node3:9090';
        document.getElementById('pub-key').value = 'abc123key';
        document.getElementById('non-voter').checked = true;

        document.getElementById('btn-join-node').click();
        await new Promise(resolve => setTimeout(resolve, 0));

        expect(mockFetch).toHaveBeenCalledWith('/api/cluster/join', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({
                httpAddr: 'https://node3:9090',
                pubKey: 'abc123key',
                nonVoter: true,
            }),
            headers: expect.objectContaining({
                'X-Raft-Secret': 'correct-secret',
            }),
        }));

        expect(mockAlert).toHaveBeenCalledWith(expect.stringContaining('successfully'));
        expect(document.getElementById('add-node-form-container').classList.contains('hidden')).toBe(true);
    });

    test('should handle join errors', async() => {
        document.getElementById('secret').value = 'correct-secret';
        document.getElementById('btn-access-dashboard').click();
        await new Promise(resolve => setTimeout(resolve, 0));

        mockFetch.mockImplementationOnce((url) => {
            if (url === '/api/cluster/join') {
                return Promise.resolve({
                    ok: false,
                    text: () => Promise.resolve('Join Failed'),
                });
            }
        });

        document.getElementById('http-addr').value = 'bad-addr';
        document.getElementById('pub-key').value = 'key';

        document.getElementById('btn-join-node').click();
        await new Promise(resolve => setTimeout(resolve, 0));

        const err = document.getElementById('add-error');
        expect(err.classList.contains('hidden')).toBe(false);
        expect(err.textContent).toContain('Join Failed');
    });

    test('should remove node on confirmation', async() => {
        // Login and load
        document.getElementById('secret').value = 'correct-secret';
        document.getElementById('btn-access-dashboard').click();
        await new Promise(resolve => setTimeout(resolve, 0));

        // Mock confirm to say YES
        mockConfirm.mockReturnValue(true);

        // Call removeNode (attached to window)
        await window.removeNode('node2');

        expect(mockConfirm).toHaveBeenCalled();
        expect(mockFetch).toHaveBeenCalledWith('/api/cluster/remove', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ nodeId: 'node2' }),
        }));
        expect(mockAlert).toHaveBeenCalledWith(expect.stringContaining('removed'));
    });
});
