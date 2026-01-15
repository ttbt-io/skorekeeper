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
loadPolicy();
