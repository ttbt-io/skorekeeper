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

let currentPolicy = {
    defaultPolicy: 'allow',
    defaultMaxTeams: 0,
    defaultMaxGames: 0,
    defaultDenyMessage: '',
    admins: [],
    users: {}
};

async function loadPolicy() {
    try {
        const res = await fetch('/api/admin/policy');
        if (!res.ok) throw new Error(res.statusText);
        currentPolicy = await res.json();
        render();
        document.getElementById('loading').style.display = 'none';
        document.getElementById('policyForm').style.display = 'block';
    } catch (e) {
        showStatus('Failed to load policy: ' + e.message, true);
    }
}

function render() {
    // Globals
    document.getElementById('defaultPolicy').value = currentPolicy.defaultPolicy;
    document.getElementById('defaultMaxGames').value = currentPolicy.defaultMaxGames;
    document.getElementById('defaultMaxTeams').value = currentPolicy.defaultMaxTeams;
    document.getElementById('defaultDenyMessage').value = currentPolicy.defaultDenyMessage;

    // Admins
    const adminsContainer = document.getElementById('adminsList');
    adminsContainer.innerHTML = '';
    (currentPolicy.admins || []).forEach((email, idx) => {
        const div = document.createElement('div');
        div.className = 'list-item';
        div.innerHTML = `<span>${escapeHtml(email)}</span> <button type="button" class="btn btn-danger" data-action="remove-admin" data-idx="${idx}">Remove</button>`;
        adminsContainer.appendChild(div);
    });

    // Users
    const usersContainer = document.getElementById('usersList');
    usersContainer.innerHTML = '';
    Object.keys(currentPolicy.users || {}).forEach(email => {
        const u = currentPolicy.users[email];
        const div = document.createElement('div');
        div.className = 'user-row';
        div.innerHTML = `
            <div>${escapeHtml(email)}</div>
            <div>${u.access}</div>
            <div>${u.maxGames}</div>
            <div>${u.maxTeams}</div>
            <div>
                <button type="button" class="btn btn-add" style="background:#f59e0b; margin-right:5px;" data-action="edit-user" data-email="${escapeHtml(email)}">Edit</button>
                <button type="button" class="btn btn-danger" data-action="remove-user" data-email="${escapeHtml(email)}">Remove</button>
            </div>
        `;
        usersContainer.appendChild(div);
    });
}

function escapeHtml(text) {
    if (!text) return text;
    return text.replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

async function savePolicy(e) {
    e.preventDefault();
    
    // Update model from form inputs
    currentPolicy.defaultPolicy = document.getElementById('defaultPolicy').value;
    currentPolicy.defaultMaxGames = parseInt(document.getElementById('defaultMaxGames').value) || 0;
    currentPolicy.defaultMaxTeams = parseInt(document.getElementById('defaultMaxTeams').value) || 0;
    currentPolicy.defaultDenyMessage = document.getElementById('defaultDenyMessage').value;

    try {
        const res = await fetch('/api/admin/policy', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(currentPolicy)
        });
        if (!res.ok) throw new Error(res.statusText);
        showStatus('Policy saved successfully!', false);
    } catch (e) {
        showStatus('Failed to save: ' + e.message, true);
    }
}

function showStatus(msg, isError) {
    const el = document.getElementById('status');
    el.textContent = msg;
    el.className = isError ? 'error' : 'success';
    el.style.display = 'block';
    setTimeout(() => { el.style.display = 'none'; }, 3000);
}

// Actions
const removeAdmin = (idx) => {
    currentPolicy.admins.splice(idx, 1);
    render();
};

const removeUser = (email) => {
    delete currentPolicy.users[email];
    render();
};

const editUser = (email) => {
    const u = currentPolicy.users[email];
    if (u) {
        document.getElementById('newUserEmail').value = email;
        document.getElementById('newUserAccess').value = u.access;
        document.getElementById('newUserGames').value = u.maxGames;
        document.getElementById('newUserTeams').value = u.maxTeams;
        // Scroll to form
        document.getElementById('newUserEmail').scrollIntoView({behavior: "smooth"});
        // Highlight the Add button to indicate "Update"
        const btn = document.getElementById('btnAddUser');
        btn.textContent = "Update";
        btn.style.background = "#f59e0b";
    }
};

// Event Listeners for Delegation
document.getElementById('adminsList').addEventListener('click', (e) => {
    if (e.target.dataset.action === 'remove-admin') {
        removeAdmin(parseInt(e.target.dataset.idx));
    }
});

document.getElementById('usersList').addEventListener('click', (e) => {
    const action = e.target.dataset.action;
    const email = e.target.dataset.email;
    if (action === 'remove-user') {
        removeUser(email);
    } else if (action === 'edit-user') {
        editUser(email);
    }
});

document.getElementById('btnAddAdmin').onclick = () => {
    const email = document.getElementById('newAdminEmail').value.trim();
    if (email) {
        if (!currentPolicy.admins) currentPolicy.admins = [];
        if (!currentPolicy.admins.includes(email)) {
            currentPolicy.admins.push(email);
            document.getElementById('newAdminEmail').value = '';
            render();
        }
    }
};

document.getElementById('btnAddUser').onclick = () => {
    const email = document.getElementById('newUserEmail').value.trim();
    if (email) {
        if (!currentPolicy.users) currentPolicy.users = {};
        currentPolicy.users[email] = {
            access: document.getElementById('newUserAccess').value,
            maxGames: parseInt(document.getElementById('newUserGames').value) || 0,
            maxTeams: parseInt(document.getElementById('newUserTeams').value) || 0
        };
        document.getElementById('newUserEmail').value = '';
        render();

        // Reset button state
        const btn = document.getElementById('btnAddUser');
        btn.textContent = "Add";
        btn.style.background = "";
    }
};

document.getElementById('policyForm').onsubmit = savePolicy;

// Init
const hash = window.location.hash.replace('#', '');
if (hash === 'monitoring') {
    switchTab('monitoring');
} else {
    loadPolicy(); // Default
}

// --- UI Logic ---

function switchTab(tab, target) {
    document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.nav-tab').forEach(el => el.classList.remove('active'));
    document.getElementById('tab-' + tab).classList.add('active');
    
    // Highlight nav tab
    if (target) {
        target.classList.add('active');
    } else {
        const tabEl = document.querySelector(`.nav-tab[data-tab="${tab}"]`);
        if (tabEl) tabEl.classList.add('active');
    }

    // Update Hash
    window.location.hash = tab;

    if (tab === 'monitoring') fetchMetrics();
    if (tab === 'policy') loadPolicy();
}

document.querySelectorAll('.nav-tab').forEach(el => {
    el.addEventListener('click', (e) => switchTab(e.target.dataset.tab, e.target));
});

// --- Monitoring Logic ---

let metricData = null;
const colors = ['#2563eb', '#dc2626', '#16a34a', '#d97706', '#9333ea', '#0891b2', '#db2777'];

async function fetchMetrics() {
    const statusEl = document.getElementById('metricStatus');
    statusEl.textContent = 'Loading...';
    try {
        const res = await fetch('/api/cluster/metrics');
        if (!res.ok) throw new Error(res.statusText);
        metricData = await res.json();
        renderCharts();
        statusEl.textContent = 'Last updated: ' + new Date().toLocaleTimeString();
    } catch (e) {
        statusEl.textContent = 'Error: ' + e.message;
    }
}

function calculatePercentile(hist, p) {
    if (!hist || hist.c === 0) return 0;
    const target = hist.c * (p / 100);
    let count = 0;
    for (let i = 0; i < hist.b.length; i++) {
        count += hist.b[i];
        if (count >= target) {
            // Bucket i covers [(i * 125), (i+1) * 125]
            return (i * 125) + 62.5;
        }
    }
    return 5000;
}

function renderCharts() {
    if (!metricData) return;
    const res = document.getElementById('metricRes').value; // '1m', '5m', etc.

    // 1. Prepare Node Data
    const finalRpsSeries = [];
    const finalWsSeries = [];
    
    // Map NodeID to Color Index
    const nodeColors = {};
    let colorIdx = 0;
    
    if (metricData.nodes) {
        for (const key of Object.keys(metricData.nodes)) {
            const realID = key.replace(':ws', '');
            if (nodeColors[realID] === undefined) {
                nodeColors[realID] = colorIdx++;
            }
        }
        
        for (const [key, series] of Object.entries(metricData.nodes)) {
            const buf = series.buffers[res];
            if (!buf || !buf.data) continue;
            
            const realID = key.replace(':ws', '');
            const color = colors[nodeColors[realID] % colors.length];
            
            if (key.endsWith(':ws')) {
                finalWsSeries.push({ label: realID, data: reconstructRingBuffer(buf), color: color });
            } else {
                finalRpsSeries.push({ label: realID, data: reconstructRingBuffer(buf), color: color });
            }
        }
    }

    drawChart('chart-rps', finalRpsSeries, { title: 'Requests / Second', yMin: 0 });
    drawChart('chart-ws', finalWsSeries, { title: 'Active Connections', yMin: 0 });

    // 1.5 Prepare Latency Data
    const latencySeries = [];
    if (metricData.latencies) {
        // Aggregate histograms across nodes for cluster-wide percentiles
        const clusterHistMap = {}; // ts -> consolidated histogram
        
        for (const series of Object.values(metricData.latencies)) {
            const buf = series.buffers[res];
            if (!buf || !buf.data) continue;
            
            buf.data.forEach(pt => {
                if (pt.t > 0) {
                    if (!clusterHistMap[pt.t]) {
                        clusterHistMap[pt.t] = { b: new Array(41).fill(0), c: 0, s: 0 };
                    }
                    const h = clusterHistMap[pt.t];
                    pt.v.b.forEach((count, idx) => h.b[idx] += count);
                    h.c += pt.v.c;
                    h.s += pt.v.s;
                }
            });
        }
        
        const sortedTs = Object.keys(clusterHistMap).map(Number).sort((a, b) => a - b);
        const p50 = [], p90 = [], p95 = [], p99 = [];
        
        sortedTs.forEach(ts => {
            const h = clusterHistMap[ts];
            p50.push({ x: ts * 1000, y: calculatePercentile(h, 50) });
            p90.push({ x: ts * 1000, y: calculatePercentile(h, 90) });
            p95.push({ x: ts * 1000, y: calculatePercentile(h, 95) });
            p99.push({ x: ts * 1000, y: calculatePercentile(h, 99) });
        });
        
        if (sortedTs.length > 0) {
            latencySeries.push({ label: 'P50', data: p50, color: '#10b981' });
            latencySeries.push({ label: 'P90', data: p90, color: '#f59e0b' });
            latencySeries.push({ label: 'P95', data: p95, color: '#ef4444' });
            latencySeries.push({ label: 'P99', data: p99, color: '#7c3aed' });
        }
    }
    drawChart('chart-latency', latencySeries, { title: 'Request Latency (ms)', yMin: 0 });

    // 2. Prepare Cluster Data
    const entitySeries = [];
    const nodeSeries = [];
    const electionSeries = [];
    const gapSeries = [];

    if (metricData.cluster) {
        // Total Games & Teams
        if (metricData.cluster.totalGames && metricData.cluster.totalGames.buffers[res]) {
            entitySeries.push({
                label: 'Games',
                data: reconstructRingBuffer(metricData.cluster.totalGames.buffers[res]),
                color: '#16a34a'
            });
        }
        if (metricData.cluster.totalTeams && metricData.cluster.totalTeams.buffers[res]) {
            entitySeries.push({
                label: 'Teams',
                data: reconstructRingBuffer(metricData.cluster.totalTeams.buffers[res]),
                color: '#d97706'
            });
        }

        // Node Count
        if (metricData.cluster.nodeCount && metricData.cluster.nodeCount.buffers[res]) {
            nodeSeries.push({
                label: 'Nodes',
                data: reconstructRingBuffer(metricData.cluster.nodeCount.buffers[res]),
                color: '#2563eb'
            });
        }

        // Elections
        if (metricData.cluster.elections && metricData.cluster.elections.buffers[res]) {
            electionSeries.push({
                label: 'Elections',
                data: reconstructRingBuffer(metricData.cluster.elections.buffers[res]),
                color: '#dc2626'
            });
        }

        // Leader Gap
        if (metricData.cluster.leaderGapMs && metricData.cluster.leaderGapMs.buffers[res]) {
            gapSeries.push({
                label: 'Leader Gap (ms)',
                data: reconstructRingBuffer(metricData.cluster.leaderGapMs.buffers[res]),
                color: '#9333ea'
            });
        }
    }

    drawChart('chart-entities', entitySeries, { title: 'Games & Teams', yMin: 0 });
    drawChart('chart-nodes', nodeSeries, { title: 'Cluster Size', yMin: 0 });
    drawChart('chart-elections', electionSeries, { title: 'Elections', yMin: 0 });
    drawChart('chart-gap', gapSeries, { title: 'Leader Downtime (ms)', yMin: 0 });
}

document.getElementById('metricRes').addEventListener('change', renderCharts);
document.getElementById('btnRefreshMetrics').addEventListener('click', fetchMetrics);

function reconstructRingBuffer(buf) {
    // buf has .data (array) and .head (int)
    // The oldest point is at head, newest at head-1.
    // We want chronological order.
    const points = [];
    const len = buf.data.length;
    for (let i = 0; i < len; i++) {
        const idx = (buf.head + i) % len;
        const pt = buf.data[idx];
        if (pt && pt.t > 0) {
            points.push({ x: pt.t * 1000, y: pt.v });
        }
    }
    return points;
}

const chartMeta = {};

function drawChart(canvasId, seriesList, options) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) {
        console.warn(`Canvas not found: ${canvasId}`);
        return;
    }
    const ctx = canvas.getContext('2d');
    
    // Resize handling (naive)
    canvas.width = canvas.parentElement.clientWidth;
    canvas.height = 300;
    const w = canvas.width;
    const h = canvas.height;
    const pad = { top: 20, right: 20, bottom: 40, left: 50 };
    const chartW = w - pad.left - pad.right;
    const chartH = h - pad.top - pad.bottom;

    ctx.clearRect(0, 0, w, h);

    if (seriesList.length === 0) {
        ctx.fillStyle = '#666';
        ctx.fillText("No data available", w/2, h/2);
        return;
    }

    // Determine Ranges
    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
    seriesList.forEach(s => {
        s.data.forEach(p => {
            if (p.x < minX) minX = p.x;
            if (p.x > maxX) maxX = p.x;
            if (p.y < minY) minY = p.y;
            if (p.y > maxY) maxY = p.y;
        });
    });

    if (options.yMin !== undefined) minY = Math.min(minY, options.yMin);
    if (maxY === -Infinity) maxY = 10; // Default range
    if (minY === Infinity) minY = 0;
    if (maxY === minY) maxY = minY + 10;
    
    // Scale Helpers
    const scaleX = (val) => pad.left + ((val - minX) / (maxX - minX)) * chartW;
    const scaleY = (val) => pad.top + chartH - ((val - minY) / (maxY - minY)) * chartH;

    // Save Meta for Tooltips
    chartMeta[canvasId] = { seriesList, scaleX, minX, maxX, chartW, pad, w, h, minY, maxY, chartH };

    // Draw Axes
    ctx.strokeStyle = '#e2e8f0';
    ctx.lineWidth = 1;
    ctx.beginPath();
    // Y-Axis
    ctx.moveTo(pad.left, pad.top);
    ctx.lineTo(pad.left, pad.top + chartH);
    // X-Axis
    ctx.lineTo(pad.left + chartW, pad.top + chartH);
    ctx.stroke();

    // Draw Ticks & Labels (Simplified)
    ctx.fillStyle = '#64748b';
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'right';
    
    // Y-Ticks (5 steps)
    for (let i = 0; i <= 5; i++) {
        const val = minY + (maxY - minY) * (i / 5);
        const y = scaleY(val);
        ctx.fillText(val.toFixed(1), pad.left - 5, y + 3);
        ctx.beginPath();
        ctx.moveTo(pad.left, y);
        ctx.lineTo(pad.left + chartW, y); // Grid line
        ctx.strokeStyle = '#f1f5f9';
        ctx.stroke();
    }

    // X-Ticks (Time)
    ctx.textAlign = 'center';
    if (minX !== Infinity) {
        const timeSteps = 5;
        for (let i = 0; i <= timeSteps; i++) {
            const timeVal = minX + (maxX - minX) * (i / timeSteps);
            const x = scaleX(timeVal);
            const date = new Date(timeVal);
            let timeStr = date.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
            if (maxX - minX > 86400000) { // > 1 day
                 timeStr = date.toLocaleDateString() + ' ' + timeStr;
            }
            ctx.fillText(timeStr, x, pad.top + chartH + 15);
        }
    }

    // Draw Series
    seriesList.forEach(s => {
        if (s.data.length === 0) return;
        ctx.beginPath();
        ctx.strokeStyle = s.color || '#000';
        ctx.lineWidth = 2;
        
        let first = true;
        s.data.forEach(p => {
            if (first) {
                ctx.moveTo(scaleX(p.x), scaleY(p.y));
                first = false;
            } else {
                ctx.lineTo(scaleX(p.x), scaleY(p.y));
            }
        });
        ctx.stroke();
    });

    // Legend
    let legendX = pad.left;
    seriesList.forEach(s => {
        ctx.fillStyle = s.color;
        ctx.fillRect(legendX, 5, 10, 10);
        ctx.fillStyle = '#333';
        ctx.textAlign = 'left';
        ctx.fillText(s.label, legendX + 15, 14);
        legendX += ctx.measureText(s.label).width + 30;
    });

    // Setup Interaction
    if (!canvas.dataset.hasListener) {
        canvas.addEventListener('mousemove', handleChartMove);
        canvas.addEventListener('mouseleave', handleChartLeave);
        canvas.dataset.hasListener = "true";
    }
}

function handleChartLeave(e) {
    const el = document.getElementById('chart-tooltip');
    if (el) el.style.display = 'none';
}

function handleChartMove(e) {
    const canvas = e.target;
    const meta = chartMeta[canvas.id];
    if (!meta) return;

    const rect = canvas.getBoundingClientRect();
    const mouseX = e.clientX - rect.left;

    if (mouseX < meta.pad.left || mouseX > meta.w - meta.pad.right) {
        const el = document.getElementById('chart-tooltip');
        if (el) el.style.display = 'none';
        return;
    }

    const timeVal = meta.minX + ((mouseX - meta.pad.left) / meta.chartW) * (meta.maxX - meta.minX);
    
    let closestTime = -1;
    let minDiff = Infinity;
    
    // Find global closest time across all series
    meta.seriesList.forEach(s => {
        s.data.forEach(p => {
            const diff = Math.abs(p.x - timeVal);
            if (diff < minDiff) {
                minDiff = diff;
                closestTime = p.x;
            }
        });
    });

    if (closestTime === -1) return;

    const el = document.getElementById('chart-tooltip');
    if (!el) return;

    // Build Tooltip HTML
    const dateStr = new Date(closestTime).toLocaleString();
    let html = `<div class="tooltip-time">${dateStr}</div>`;
    
    meta.seriesList.forEach(s => {
        const pt = s.data.find(p => Math.abs(p.x - closestTime) < 1000); 
        if (pt) {
            html += `
                <div class="tooltip-item">
                    <div class="tooltip-color" style="background:${s.color}"></div>
                    <span>${s.label}: <strong>${pt.y.toFixed(2)}</strong></span>
                </div>
            `;
        }
    });

    el.innerHTML = html;
    el.style.display = 'block';
    
    const tipX = e.pageX + 15;
    const tipY = e.pageY + 15;
    el.style.left = tipX + 'px';
    el.style.top = tipY + 'px';
}
