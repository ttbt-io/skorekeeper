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

        const loginSection = document.getElementById('login-section');
        const dashboardSection = document.getElementById('dashboard-section');
        const btnAccessDashboard = document.getElementById('btn-access-dashboard');
        const loginError = document.getElementById('login-error');
        const nodesTableBody = document.getElementById('nodes-table-body');
        const refreshBtn = document.getElementById('refresh-btn');
        const showAddNodeBtn = document.getElementById('show-add-node-btn');
        const addNodeFormContainer = document.getElementById('add-node-form-container');
        const btnJoinNode = document.getElementById('btn-join-node');
        const cancelAddBtn = document.getElementById('cancel-add-btn');
        const addError = document.getElementById('add-error');
        const clusterInfo = document.getElementById('cluster-info');

        let raftSecret = '';

        btnAccessDashboard.addEventListener('click', () => {
            raftSecret = document.getElementById('secret').value;
            loadStatus();
        });

        refreshBtn.addEventListener('click', loadStatus);

        showAddNodeBtn.addEventListener('click', () => {
            addNodeFormContainer.classList.remove('hidden');
            addError.classList.add('hidden');
        });

        cancelAddBtn.addEventListener('click', () => {
            addNodeFormContainer.classList.add('hidden');
        });

        btnJoinNode.addEventListener('click', async () => {
            addError.classList.add('hidden');
            const httpAddr = document.getElementById('http-addr').value;
            const pubKey = document.getElementById('pub-key').value;
            const nonVoter = document.getElementById('non-voter').checked;

            if (!httpAddr || !pubKey) {
                addError.textContent = "HTTP Address and Public Key are required.";
                addError.classList.remove('hidden');
                return;
            }

            try {
                const res = await fetch('/api/cluster/join', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Raft-Secret': raftSecret
                    },
                    body: JSON.stringify({ httpAddr, pubKey, nonVoter })
                });

                if (!res.ok) throw new Error(await res.text());
                
                alert(`Node join request sent successfully. Auto-discovery initiated.`);
                document.getElementById('add-node-form').reset();
                addNodeFormContainer.classList.add('hidden');
                loadStatus();
            } catch (err) {
                addError.textContent = err.message;
                addError.classList.remove('hidden');
            }
        });

        async function loadStatus() {
            try {
                const res = await fetch('/api/cluster/status', {
                    headers: { 'X-Raft-Secret': raftSecret }
                });

                if (res.status === 403) {
                    showLogin("Invalid Secret");
                    return;
                }
                if (!res.ok) throw new Error(await res.text());

                const data = await res.json();
                showDashboard(data);
            } catch (err) {
                console.error(err);
                if (!raftSecret) {
                     showLogin();
                } else {
                    alert('Failed to load status: ' + err.message);
                }
            }
        }

        function showLogin(msg) {
            loginSection.classList.remove('hidden');
            dashboardSection.classList.add('hidden');
            if (msg) {
                loginError.textContent = msg;
                loginError.classList.remove('hidden');
            }
        }

        function showDashboard(data) {
            loginSection.classList.add('hidden');
            dashboardSection.classList.remove('hidden');

            clusterInfo.innerHTML = '';
            
            const pNodeId = document.createElement('p');
            const bNodeId = document.createElement('strong');
            bNodeId.textContent = 'My Node ID: ';
            pNodeId.appendChild(bNodeId);
            pNodeId.appendChild(document.createTextNode(data.nodeId));
            clusterInfo.appendChild(pNodeId);

            const pState = document.createElement('p');
            const bState = document.createElement('strong');
            bState.textContent = 'State: ';
            pState.appendChild(bState);
            pState.appendChild(document.createTextNode(data.state));
            clusterInfo.appendChild(pState);

            const pLeader = document.createElement('p');
            const bLeader = document.createElement('strong');
            bLeader.textContent = 'Leader: ';
            pLeader.appendChild(bLeader);
            pLeader.appendChild(document.createTextNode(`${data.leaderId} (${data.leaderAddr})`));
            clusterInfo.appendChild(pLeader);

            nodesTableBody.innerHTML = '';
            
            // Handle if 'nodes' is missing (e.g. single node bootstrap or old version)
            const nodes = data.nodes || [];
            
            nodes.forEach(node => {
                const tr = document.createElement('tr');
                const isLeader = node.id === data.leaderId;
                
                const tdId = document.createElement('td');
                tdId.textContent = node.id;
                tr.appendChild(tdId);

                const tdAddr = document.createElement('td');
                const spanRaft = document.createElement('span');
                spanRaft.textContent = node.raftAddr;
                tdAddr.appendChild(spanRaft);
                tdAddr.appendChild(document.createElement('br'));
                const spanHttp = document.createElement('span');
                spanHttp.textContent = node.httpAddr || '-';
                tdAddr.appendChild(spanHttp);
                tr.appendChild(tdAddr);

                const tdRole = document.createElement('td');
                const role = isLeader ? 'Leader' : 'Follower';
                const suffrage = node.suffrage ? ` (${node.suffrage})` : '';
                tdRole.textContent = role + suffrage;
                tr.appendChild(tdRole);

                const tdVer = document.createElement('td');
                const appVer = node.appVersion || '-';
                const protoSchema = (node.protocolVersion || '-') + ' / ' + (node.schemaVersion || '-');
                const spanApp = document.createElement('span');
                spanApp.textContent = appVer;
                tdVer.appendChild(spanApp);
                tdVer.appendChild(document.createElement('br'));
                const spanProto = document.createElement('span');
                spanProto.textContent = protoSchema;
                tdVer.appendChild(spanProto);
                tr.appendChild(tdVer);

                const tdActions = document.createElement('td');
                const btn = document.createElement('button');
                btn.className = 'danger btn-sm';
                btn.textContent = 'Remove';
                btn.onclick = () => window.removeNode(node.id);
                tdActions.appendChild(btn);
                tr.appendChild(tdActions);

                nodesTableBody.appendChild(tr);
            });
        }

        window.removeNode = async (nodeId) => {
            if (!confirm(`Are you sure you want to remove node ${nodeId}? This action is destructive.`)) return;
            
            try {
                const res = await fetch('/api/cluster/remove', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Raft-Secret': raftSecret
                    },
                    body: JSON.stringify({ nodeId })
                });

                if (!res.ok) throw new Error(await res.text());
                
                alert(`Node ${nodeId} removed.`);
                loadStatus();
            } catch (err) {
                alert('Failed to remove node: ' + err.message);
            }
        };
