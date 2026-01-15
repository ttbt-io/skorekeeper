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
 * Structured content for the User Manual.
 * Used by ManualViewer to render the documentation offline.
 */
export const manualSections = [
    {
        id: 'intro',
        title: 'Introduction',
        tags: ['overview', 'about'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Welcome to Skorekeeper</h2>
            <p class="mb-4">Skorekeeper is a progressive web application designed for tracking baseball and softball games. It works offline, syncs with the cloud when online, and provides detailed stats and scoring capabilities.</p>
            <p class="mb-4">Key features:</p>
            <ul class="list-disc pl-5 mb-4 space-y-1">
                <li>Offline-first architecture with authoritative server-side sync.</li>
                <li>Detailed pitch-by-pitch tracking.</li>
                <li>Visual hit location and trajectory charts.</li>
                <li>Multi-user collaboration and team-based sharing.</li>
            </ul>
            <p>Use the sidebar menu to navigate between the Dashboard (your game list), Team Management, Statistics, and the active Game Scoresheet.</p>
            <img src="assets/manual/sidebar.png" alt="App Sidebar" class="my-4 border rounded shadow-sm w-full max-w-xs mx-auto">
        `,
    },
    {
        id: 'basics',
        title: 'Scorekeeping Basics',
        tags: ['how to', 'scoring', 'grid', 'cell'],
        content: `
            <h2 class="text-2xl font-bold mb-4">How to Keep Score</h2>
            <p class="mb-4">Welcome to the art of scorekeeping! Skorekeeper is designed to bridge the gap between traditional paper scorecards and modern digital tracking. This guide will introduce you to the basics of scorekeeping and show you how they appear in the app.</p>

            <h3 class="text-xl font-bold mb-2">The Scorecard Grid</h3>
            <p class="mb-4">The game is laid out in a grid where <strong>Rows</strong> represent the batters in the lineup and <strong>Columns</strong> represent the innings. The intersection is a <strong>Plate Appearance (PA) Cell</strong>.</p>

            <h3 class="text-xl font-bold mb-2">The Cell Layout</h3>
            <p class="mb-4">Each cell represents the baseball diamond and tracks the status of the at-bat:</p>
            <img src="assets/manual/cell-empty.png" alt="Empty Cell" class="my-4 border rounded shadow-sm w-full max-w-[100px] mx-auto">
            <ol class="list-decimal pl-5 mb-4 space-y-2">
                <li><strong>The Diamond:</strong> The central square represents the bases. Bottom is Home, Right is 1st, Top is 2nd, Left is 3rd.</li>
                <li><strong>Balls & Strikes (Top Left):</strong> Dots track the count (4 Balls top, 3 Strikes bottom).</li>
                <li><strong>Outs (Top Right):</strong> If the batter is out, a circle indicates their out position in the inning (1, 2, or 3).</li>
                <li><strong>Outcome (Bottom Right):</strong> Text describes the play result (e.g., "1B", "F8", "K").</li>
            </ol>

            <h3 class="text-xl font-bold mb-2">Recording Common Plays</h3>

            <div class="space-y-6">
                <div>
                    <h4 class="font-bold">1. Strikeouts (K) & Walks (BB)</h4>
                    <p class="text-sm mb-2">A strikeout happens at 3 strikes. <strong>K</strong> is swinging, <strong>ꓘ</strong> is called. A walk (Base on Balls) happens at 4 balls, advancing the batter to 1st.</p>
                    <div class="flex gap-4 justify-center">
                        <img src="assets/manual/cell-strikeout.png" alt="Strikeout" class="border rounded shadow-sm w-[100px]">
                        <img src="assets/manual/cell-walk.png" alt="Walk" class="border rounded shadow-sm w-[100px]">
                    </div>
                </div>

                <div>
                    <h4 class="font-bold">2. Hits (1B, 2B, 3B, HR)</h4>
                    <p class="text-sm mb-2">When a batter reaches base safely, the path is filled. For a Home Run, the entire diamond is shaded.</p>
                    <div class="flex gap-4 justify-center">
                        <img src="assets/manual/cell-single.png" alt="Single" class="border rounded shadow-sm w-[100px]">
                        <img src="assets/manual/cell-double.png" alt="Double" class="border rounded shadow-sm w-[100px]">
                        <img src="assets/manual/cell-homerun.png" alt="Home Run" class="border rounded shadow-sm w-[100px]">
                    </div>
                </div>

                <div>
                    <h4 class="font-bold">3. Outs on Balls in Play</h4>
                    <p class="text-sm mb-2">We use position numbers (1-9) to record outs. "F8" is a fly out to Center Field. "6-3" is a ground out from Shortstop to 1st Base.</p>
                    <div class="flex gap-4 justify-center">
                        <img src="assets/manual/cell-flyout.png" alt="Fly Out" class="border rounded shadow-sm w-[100px]">
                        <img src="assets/manual/cell-groundout.png" alt="Ground Out" class="border rounded shadow-sm w-[100px]">
                    </div>
                </div>

                <div>
                    <h4 class="font-bold">4. Advancing Runners</h4>
                    <p class="text-sm mb-2">As subsequent batters play, runners advance. Below, Batter 1 hit a single, then Batter 2 hit a double, moving Batter 1 to 3rd base.</p>
                    <img src="assets/manual/cell-advance.png" alt="Runner Advance" class="my-2 border rounded shadow-sm w-[100px] mx-auto">
                </div>

                <div>
                    <h4 class="font-bold">5. Ball Trajectories</h4>
                    <p class="text-sm mb-2">When a ball is put in play, specify its trajectory to provide context. These are visually distinguished in the scoresheet grid:</p>
                    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs mb-4">
                        <div class="border p-3 rounded-lg bg-slate-50 border-slate-200 shadow-sm">
                            <strong class="text-slate-900 block mb-1 uppercase tracking-wider">Grounder</strong>
                            <p class="text-slate-600">Hit along the ground. Represented by a <span class="font-bold">dashed line</span> in the grid.</p>
                        </div>
                        <div class="border p-3 rounded-lg bg-slate-50 border-slate-200 shadow-sm">
                            <strong class="text-slate-900 block mb-1 uppercase tracking-wider">Line Drive</strong>
                            <p class="text-slate-600">Hit hard and flat. Represented by a <span class="font-bold">solid straight line</span> in the grid.</p>
                        </div>
                        <div class="border p-3 rounded-lg bg-slate-50 border-slate-200 shadow-sm">
                            <strong class="text-slate-900 block mb-1 uppercase tracking-wider">Fly Ball</strong>
                            <p class="text-slate-600">Hit high and deep. Represented by a <span class="font-bold">curved line</span> in the grid.</p>
                        </div>
                        <div class="border p-3 rounded-lg bg-slate-50 border-slate-200 shadow-sm">
                            <strong class="text-slate-900 block mb-1 uppercase tracking-wider">Pop Fly</strong>
                            <p class="text-slate-600">Hit very high but shallow. Also represented by a <span class="font-bold">curved line</span>.</p>
                        </div>
                    </div>
                    <div class="flex flex-wrap gap-4 justify-center items-end">
                        <div class="text-center">
                            <p class="text-[10px] text-gray-500 font-bold mb-1 uppercase">Grounder</p>
                            <img src="assets/manual/cell-grounder.png" alt="Grounder Example" class="border rounded shadow-md w-[90px] mx-auto bg-white">
                        </div>
                        <div class="text-center">
                            <p class="text-[10px] text-gray-500 font-bold mb-1 uppercase">Line Drive</p>
                            <img src="assets/manual/cell-linedrive.png" alt="Line Drive Example" class="border rounded shadow-md w-[90px] mx-auto bg-white">
                        </div>
                        <div class="text-center">
                            <p class="text-[10px] text-gray-500 font-bold mb-1 uppercase">Fly Ball</p>
                            <img src="assets/manual/cell-flyball.png" alt="Fly Ball Example" class="border rounded shadow-md w-[90px] mx-auto bg-white">
                        </div>
                        <div class="text-center">
                            <p class="text-[10px] text-gray-500 font-bold mb-1 uppercase">Pop Fly</p>
                            <img src="assets/manual/cell-popfly.png" alt="Pop Fly Example" class="border rounded shadow-md w-[90px] mx-auto bg-white">
                        </div>
                    </div>
                </div>
            </div>
        `,
    },
    {
        id: 'dashboard',
        title: 'Dashboard & Game Management',
        tags: ['create', 'new game', 'sync', 'list', 'edit', 'finalize', 'delete'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Dashboard</h2>
            <p class="mb-4">The Dashboard is your home screen. Games are organized into two sections: <strong>Ongoing Games</strong> and <strong>Finalized Games</strong>.</p>
            <img src="assets/manual/dashboard.png" alt="Dashboard View" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">

            <h3 class="text-xl font-bold mb-2">Creating a New Game</h3>
            <p class="mb-4">Click the large <strong class="text-blue-600">New Game</strong> button in the sidebar (or the '+' icon on mobile) to start.</p>
            <img src="assets/manual/new-game.png" alt="New Game Modal" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
            <ul class="list-disc pl-5 mb-4 space-y-2">
                <li><strong>Event/Location:</strong> Optional metadata describing the game context.</li>
                <li><strong>Date:</strong> Defaults to the current date and time.</li>
                <li><strong>Teams:</strong> Enter names or select from your saved <strong>Teams</strong> list.</li>
            </ul>

            <h3 class="text-xl font-bold mb-2">Game Operations</h3>
            <ul class="list-disc pl-5 mb-4 space-y-2">
                <li><strong>Finalizing:</strong> When a game is over, open the sidebar and click <strong class="text-red-700">End Game</strong>. This marks the scoresheet as read-only and moves it to the "Finalized" section.</li>
                <li><strong>Editing:</strong> Right-click (or long-press) any game card to edit its metadata.</li>
                <li><strong>Deleting:</strong> Right-click any game card and select <strong>Delete Game</strong> to permanently remove it from both your local database and the server (if owned by you).</li>
            </ul>

            <h3 class="text-xl font-bold mb-2">Sync Status</h3>
            <p class="mb-4">Icons on the game card indicate synchronization status with the server:</p>
            <ul class="list-none pl-0 mb-4 space-y-2">
                <li>✅ <strong>Synced:</strong> All changes are saved to the cloud.</li>
                <li>☁️⬆️ <strong>Local Ahead:</strong> You have unsynced changes. Click to push them.</li>
                <li>☁️⬇️ <strong>Remote Ahead:</strong> A newer version is on the server. Click to pull it.</li>
                <li>⚠️ <strong>Conflict:</strong> Divergent history detected. Click to resolve.</li>
            </ul>
        `,
    },
    {
        id: 'teams',
        title: 'Teams & Collaboration',
        tags: ['team', 'members', 'roles', 'sharing', 'admin', 'roster'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Team Management</h2>
            <p class="mb-4">Skorekeeper uses a team-centric collaboration model. Access the <strong>Teams</strong> view from the sidebar.</p>

            <h3 class="text-xl font-bold mb-2">Rosters</h3>
            <p class="mb-4">Save your team rosters once and reuse them across games. Each player is tracked with a stable ID for statistical history.</p>

            <h3 class="text-xl font-bold mb-2">Members & Roles</h3>
            <p class="mb-4">Manage access to your teams via the <strong>Members</strong> tab in the Team Modal. You can invite users by email and assign one of three roles:</p>
            <img src="assets/manual/team-members.png" alt="Team Members Tab" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
            <ul class="list-disc pl-5 mb-4 space-y-2">
                <li><strong>Admin:</strong> Full control over the team and all associated games (including editing members and deleting).</li>
                <li><strong>Scorekeeper:</strong> Can create and edit games for the team.</li>
                <li><strong>Spectator:</strong> Read-only access to all the team's games.</li>
            </ul>
            <p class="mb-4 text-sm text-gray-600"><em>Note: Access to games is automatically inherited based on team membership.</em></p>
        `,
    },
    {
        id: 'statistics',
        title: 'Statistics',
        tags: ['stats', 'leaderboard', 'spray chart'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Statistics</h2>
            <p class="mb-4">Skorekeeper automatically aggregates data across all games to provide comprehensive leaderboards.</p>
            <img src="assets/manual/statistics.png" alt="Statistics View" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
            <p class="mb-4">Click any player to view their detailed profile, including seasonal summaries and visual spray charts of their hitting performance.</p>
        `,
    },
    {
        id: 'scoresheet',
        title: 'The Scoresheet View',
        tags: ['grid', 'scoreboard', 'innings', 'correction'],
        content: `
            <h2 class="text-2xl font-bold mb-4">The Scoresheet</h2>
            <p class="mb-4">The scoresheet displays the batting order and the play-by-play diamond grid.</p>
            <img src="assets/manual/scoresheet.png" alt="Scoresheet View" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">

            <h3 class="text-xl font-bold mb-2">Grid Interactions</h3>
            <ul class="list-disc pl-5 mb-4 space-y-2">
                <li><strong>Tap a cell</strong> to open the Contextual Scoring Overlay (CSO).</li>
                <li><strong>Inning Headers:</strong> Right-click an inning number to add or remove columns for the active team.</li>
                <li><strong>Inning Scores:</strong> Right-click any inning score in the scoreboard to manually override it.</li>
                <li><strong>Correct Player:</strong> Right-click the batter's name in the left column or the CSO header and select <strong>Correct Player in Slot</strong> to fix errors without a formal substitution log entry.</li>
            </ul>
            <img src="assets/manual/correct-batter.png" alt="Correct Batter Context Menu" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
        `,
    },
    {
        id: 'pitching',
        title: 'Recording Pitches & Basics',
        tags: ['cso', 'pitch', 'ball', 'strike', 'out', 'scoring'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Contextual Scoring Overlay (CSO)</h2>
            <p class="mb-4">The CSO is your primary tool for recording every action in a plate appearance.</p>
            <img src="assets/manual/cso-pitch.png" alt="CSO Pitch View" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">

            <h3 class="text-xl font-bold mb-2">Recording Pitches</h3>
            <p class="mb-4">Use the large buttons to record the pitch sequence.</p>
            <ul class="list-disc pl-5 mb-4">
                <li><strong>Ball / Strike / Foul:</strong> Updates the count.</li>
                <li><strong>Strikeout:</strong> Recorded after 3 strikes. Long-press 'Strike' for 'Called Strike' (ꓘ) or 'Dropped' for a Dropped 3rd Strike.</li>
                <li><strong>Out:</strong> Immediate out. Long-press for Interference, Stepped Out, or Penalty Outs.</li>
            </ul>
            <p class="mb-2 text-sm text-gray-600"><em>Tip: Click the <strong>Pitcher (P:#)</strong> button in the CSO header to track mid-inning pitching changes.</em></p>
        `,
    },
    {
        id: 'bip',
        title: 'Recording Hits & Outs',
        tags: ['hit', 'out', 'fielding', 'location', 'trajectory'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Ball in Play (BiP)</h2>
            <p class="mb-4">Click <strong>BALL IN PLAY</strong> to record fielding actions.</p>

            <h3 class="text-xl font-bold mb-2">Outcome Selection</h3>
            <p class="mb-4">Use the cycle buttons to select the Result (Safe/Out/Ground/Fly/Line/IFF), Base, and Hit Type (HIT/ERR/FC).</p>
            <p class="mb-4"><strong>DP/TP:</strong> Double and Triple plays are automatically detected if multiple outs are recorded in a single play. You do not need to select them manually.</p>
            <img src="assets/manual/cycle-options.png" alt="Cycle Button Context Menu" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
            <p class="mb-4 text-sm text-gray-600"><em>Tip: Right-click any cycle button to pick an option directly from a list.</em></p>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                <img src="assets/manual/play-hit-single.png" alt="Recording a Single" class="border rounded shadow-sm">
                <img src="assets/manual/play-homerun.png" alt="Recording a Home Run" class="border rounded shadow-sm">
            </div>

            <h3 class="text-xl font-bold mb-2">Recording Outs</h3>
            <p class="mb-4">Select the result (e.g., Ground) and Type (OUT). For Ground outs, forced runners are automatically suggested to advance by one base. For Fly/Line outs, runners stay by default.</p>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                <div>
                    <p class="font-bold text-center text-sm mb-1">Ground Out (6-3)</p>
                    <img src="assets/manual/play-ground-out.png" alt="Ground Out" class="border rounded shadow-sm">
                </div>
                <div>
                    <p class="font-bold text-center text-sm mb-1">Fly Out (F8)</p>
                    <img src="assets/manual/play-fly-out.png" alt="Fly Out" class="border rounded shadow-sm">
                </div>
            </div>

            <h3 class="text-xl font-bold mb-2">Hit Location & Trajectory</h3>
            <p class="mb-4">Click the <strong>📍</strong> icon to enable location mode. Tap the field to mark the hit. Trajectory controls (Ground/Line/Fly/Pop) will automatically appear.</p>

            <h3 class="text-xl font-bold mb-2">Dropped 3rd Strike</h3>
            <p class="mb-4">When a 3rd strike is dropped, the BiP view allows you to record if the batter reached first base (D3) or was thrown out (K).</p>
            <img src="assets/manual/play-dropped-3rd.png" alt="Dropped 3rd Strike View" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
        `,
    },
    {
        id: 'baserunning',
        title: 'Base Running',
        tags: ['steal', 'advance', 'cs', 'po'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Base Running</h2>

            <h3 class="text-xl font-bold mb-2">Steals & Pickoffs</h3>
            <p class="mb-4">Click <strong>RUNNER ACTIONS</strong> in the CSO to record events before the pitch (like Steals or Pickoffs).</p>
            <img src="assets/manual/play-steal.png" alt="Runner Actions Menu" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">

            <h3 class="text-xl font-bold mb-2">Advancing Runners</h3>
            <p class="mb-4">After a hit or out, if there are runners on base, you will be prompted to update their positions. The app predicts advancements based on the batter's reached base (e.g., a Double advances runners by 2 bases).</p>
            <img src="assets/manual/play-runner-advance.png" alt="Runner Advance Screen" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
        `,
    },
    {
        id: 'lineup',
        title: 'Lineups & Substitutions',
        tags: ['roster', 'sub', 'player', 'position', 'drag', 'drop', 'reorder'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Lineups & Substitutions</h2>

            <h3 class="text-xl font-bold mb-2">Editing the Lineup</h3>
            <p class="mb-4">Right-click (or long-press) the team tab ("AWAY" or "HOME") in the header to open the <strong>Edit Lineup</strong> modal.</p>
            <img src="assets/manual/edit-lineup.png" alt="Edit Lineup Modal" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">

            <h3 class="text-xl font-bold mb-2">Drag & Drop Reordering</h3>
            <p class="mb-4">Use the <strong>⋮⋮</strong> handle on the left of any player row to move them.</p>
            <ul class="list-disc pl-5 mb-4 space-y-2">
                <li><strong>Reorder Batting Order:</strong> Drag players up or down within the "Starters" list.</li>
                <li><strong>Manage the Bench:</strong> Drag players between "Starters" and "Substitutes" to move them into or out of the active game.</li>
                <li><strong>Flexible Size:</strong> Add or remove rows to accommodate any lineup size.</li>
            </ul>

            <h3 class="text-xl font-bold mb-2">Substitutions</h3>
            <p class="mb-4">To record a formal mid-game substitution, right-click a player's name in the scoresheet's left column and select <strong>Substitute Player</strong>.</p>
        `,
    },
    {
        id: 'unusual',
        title: 'Rare & Unusual Plays',
        tags: ['dp', 'tp', 'iff', 'ci', 'boo'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Rare & Unusual Plays</h2>

            <h3 class="text-xl font-bold mb-2">Double Plays</h3>
            <p class="mb-4">To record a Double Play, simply record the out for the batter (e.g., <strong>Ground</strong> -> <strong>OUT</strong>) and enter the fielding sequence. Then, on the next screen, mark the relevant runner(s) as <strong>Out</strong>. The system will automatically prefix the play with DP.</p>
            <img src="assets/manual/play-dp.png" alt="Double Play" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">

            <h3 class="text-xl font-bold mb-2">Context Menu Actions</h3>
            <p class="mb-4">Right-click grid cells for <strong>Move Play To...</strong> corrections.</p>
            <img src="assets/manual/play-context-move.png" alt="Move Play Context Menu" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
            <p class="mb-4">Long-press or right-click the <strong>OUT</strong> button in the CSO for <strong>Batting Out of Order</strong> penalties.</p>
            <img src="assets/manual/play-out-options.png" alt="Out Options Menu" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
        `,
    },
    {
        id: 'sync',
        title: 'Sync & Troubleshooting',
        tags: ['offline', 'conflict', 'fork', 'overwrite', 'cache', 'update'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Synchronization</h2>
            <p class="mb-4">Skorekeeper maintains an authoritative server-side log. If you work offline, your changes are synced when you reconnect.</p>

            <h3 class="text-xl font-bold mb-2">Handling Conflicts</h3>
            <p class="mb-4">If history diverges (e.g., two people edited simultaneously), you have three options:</p>
            <img src="assets/manual/conflict-resolution.png" alt="Conflict Resolution Modal" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
            <ul class="list-disc pl-5 mb-4 space-y-2">
                <li><strong>Overwrite Server:</strong> (Authoritative) Discards server changes and pushes your local version.</li>
                <li><strong>Overwrite Local:</strong> (Safe) Discards your local unsynced changes and fetches the server state.</li>
                <li><strong>Fork:</strong> Saves your version as a completely new game copy.</li>
            </ul>

            <h3 class="text-xl font-bold mb-2">Updates & Reloading</h3>
            <p class="mb-4">Use <strong>Clear Cache & Reload</strong> in the sidebar if you encounter sync issues or want to force a fresh update from the server.</p>
        `,
    },
    {
        id: 'broadcast',
        title: 'Broadcasting & Feed',
        tags: ['obs', 'overlay', 'link', 'stream', 'feed', 'pbp'],
        content: `
            <h2 class="text-2xl font-bold mb-4">Broadcasting & Live Feed</h2>
            <p class="mb-4">Skorekeeper provides dedicated views for consuming game data live, optimized for both viewers and streamers.</p>

            <h3 class="text-xl font-bold mb-2">Narrative Feed</h3>
            <p class="mb-4">Switch to the <strong>Feed</strong> view via the sidebar to see a human-readable play-by-play transcript with inline pitch sequences.</p>
            <ul class="list-disc pl-5 mb-4 space-y-2">
                <li><strong>Chronological:</strong> Events are grouped by Inning and Plate Appearance.</li>
                <li><strong>Detailed:</strong> See every pitch, runner move, and scoring play in order.</li>
                <li><strong>State-Driven:</strong> If a previous play is corrected, the feed automatically updates to reflect the new truth.</li>
            </ul>

            <h3 class="text-xl font-bold mb-2">Broadcast Overlay (OBS)</h3>
            <p class="mb-4">The Broadcast Overlay is a minimal "Scorebug" designed to be embedded in live video streams (like OBS).</p>
            <img src="assets/manual/broadcast-overlay.png" alt="Broadcast Scorebug" class="my-4 border rounded shadow-sm w-full max-w-md mx-auto">
            <ul class="list-disc pl-5 mb-4 space-y-2">
                <li><strong>Minimal UI:</strong> Only essential data (Score, Inning, Outs, Count, Bases) is shown.</li>
                <li><strong>Transparent Background:</strong> Designed to overlay cleanly on top of video.</li>
                <li><strong>Real-Time:</strong> Updates instantly as the skorekeeper records actions.</li>
            </ul>
            <p class="mb-4">To use the overlay, open the <strong>Share Game</strong> modal and click <strong>Copy Broadcast Overlay Link</strong>. Add this link as a "Browser Source" in your streaming software.</p>
        `,
    },
];
