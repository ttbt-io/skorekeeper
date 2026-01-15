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

package backend

// Schema Versions
const (
	SchemaVersionV2 = 2
	SchemaVersionV3 = 3
)

// Pitch Types
const (
	PitchTypeBall   = "ball"
	PitchTypeStrike = "strike"
	PitchTypeFoul   = "foul"
	PitchTypeInPlay = "in_play"
	// PitchTypeOut is deprecated but kept for legacy support/migration
	PitchTypeOutLegacy = "out"
)

// Pitch Codes
const (
	PitchCodeCalled          = "Called"
	PitchCodeSwinging        = "Swinging"
	PitchCodeFoul            = "Foul"
	PitchCodeInPlay          = "InPlay"
	PitchCodeHitByPitch      = "HitByPitch"
	PitchCodeIntentionalBall = "IntentionalBall"
)

// Ball in Play (BiP) Results
const (
	BiPResultSingle = "1B"
	BiPResultDouble = "2B"
	BiPResultTriple = "3B"
	BiPResultHR     = "HR"
	BiPResultFly    = "Fly"
	BiPResultLine   = "Line"
	BiPResultGround = "Ground"
	BiPResultPop    = "Pop"
	BiPResultBunt   = "Bunt"
	BiPResultSac    = "Sac"
	BiPResultFC     = "FC"
	BiPResultError  = "Error"
)

// Runner Actions
const (
	RunnerActionSB    = "SB"
	RunnerActionCS    = "CS"
	RunnerActionPO    = "PO"
	RunnerActionAdv   = "Adv"
	RunnerActionScore = "Score"
	RunnerActionStay  = "Stay"
	RunnerActionOut   = "Out"
)
