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

// Schema Versions
export const SchemaVersionV2 = 2;
export const SchemaVersionV3 = 3;

// Pitch Types
export const PitchTypeBall = 'ball';
export const PitchTypeStrike = 'strike';
export const PitchTypeFoul = 'foul';
export const PitchTypeInPlay = 'in_play';
// PitchTypeOutLegacy is deprecated but kept for legacy support/migration
export const PitchTypeOutLegacy = 'out';

// Pitch Codes
export const PitchCodeCalled = 'Called';
export const PitchCodeSwinging = 'Swinging';
export const PitchCodeFoul = 'Foul';
export const PitchCodeInPlay = 'InPlay';
export const PitchCodeHitByPitch = 'HitByPitch';
export const PitchCodeIntentionalBall = 'IntentionalBall';
export const PitchCodeDropped = 'Dropped';

// Pitch Outcomes (Resulting from sequences)
export const PitchOutcomeWalk = 'BB';
export const PitchOutcomeStrikeoutSwinging = 'K';
export const PitchOutcomeStrikeoutLooking = 'ꓘ';
export const PitchOutcomeIntentionalWalk = 'IBB';
export const PitchOutcomeHitByPitch = 'HBP';
export const PitchOutcomeCatcherInterference = 'CI';
export const PitchOutcomeDropped3rd = 'D3';

// Game Status
export const GameStatusOngoing = 'ongoing';
export const GameStatusFinal = 'final';

// Team Sides
export const TeamAway = 'away';
export const TeamHome = 'home';

// Scoresheet Views
export const ScoresheetViewGrid = 'grid';
export const ScoresheetViewFeed = 'feed';

// Sync Statuses
export const SyncStatusSynced = 'synced';
export const SyncStatusUnsynced = 'unsynced';
export const SyncStatusLocalOnly = 'local_only';
export const SyncStatusRemoteOnly = 'remote_only';
export const SyncStatusSyncing = 'syncing';
export const SyncStatusError = 'error';
export const SyncStatusConflict = 'conflict';

// Ball in Play (BiP) Results
export const BiPResultSafe = 'Safe';
export const BiPResultSingle = '1B';
export const BiPResultDouble = '2B';
export const BiPResultTriple = '3B';
export const BiPResultHR = 'HR';
export const BiPResultFly = 'Fly';
export const BiPResultLine = 'Line';
export const BiPResultGround = 'Ground';
export const BiPResultPop = 'Pop';
export const BiPResultBunt = 'Bunt';
export const BiPResultSac = 'Sac';
export const BiPResultFC = 'FC';
export const BiPResultError = 'Error';
export const BiPResultOut = 'Out';
export const BiPResultIFF = 'IFF';

// BiP Type Codes
export const BiPTypeCodeHit = 'HIT';
export const BiPTypeCodeErr = 'ERR';
export const BiPTypeCodeOut = 'OUT';
export const BiPTypeCodeDP = 'DP';
export const BiPTypeCodeTP = 'TP';
export const BiPTypeCodeSH = 'SH';
export const BiPTypeCodeSF = 'SF';

// BiP Modes
export const BiPModeNormal = 'normal';
export const BiPModeDropped = 'dropped';

// Runner Actions
export const RunnerActionSB = 'SB';
export const RunnerActionCS = 'CS';
export const RunnerActionPO = 'PO';
export const RunnerActionAdv = 'Adv';
export const RunnerActionScore = 'Score';
export const RunnerActionStay = 'Stay';
export const RunnerActionOut = 'Out';
export const RunnerActionCourtesy = 'CR';

// Bases
export const Base1B = '1B';
export const Base2B = '2B';
export const Base3B = '3B';
export const BaseHome = 'Home';

// Roles
export const RoleAdmins = 'admins';
export const RoleScorekeepers = 'scorekeepers';
export const RoleSpectators = 'spectators';

// Access Levels
export const AccessRead = 'read';
export const AccessWrite = 'write';

// App Versions
export const CurrentSchemaVersion = SchemaVersionV3;
export const CurrentProtocolVersion = 1;
export const CurrentAppVersion = '0.2.13';
