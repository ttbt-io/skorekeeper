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

import (
	"encoding/json"
	"testing"
)

// FuzzValidateAction tests ValidateAction with arbitrary byte slices to ensure no panics.
func FuzzValidateAction(f *testing.F) {
	f.Add([]byte(`{"id": "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", "type": "PITCH", "payload": {"type": "ball"}}`))
	f.Add([]byte(`invalid json`))
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = ValidateAction(json.RawMessage(data))
	})
}

// FuzzValidateGameData tests ValidateGameData with arbitrary byte slices to ensure no panics.
func FuzzValidateGameData(f *testing.F) {
	f.Add([]byte(`{"id": "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", "actionLog": []}`))
	f.Add([]byte(`invalid json`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = ValidateGameData(data)
	})
}
