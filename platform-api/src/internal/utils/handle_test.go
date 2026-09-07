/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package utils

import (
	"errors"
	"strings"
	"testing"

	"platform-api/src/internal/constants"
)

// TestValidateHandle_LengthBoundary pins the handle length ceiling at 40:
// a 40-char handle is accepted, a 41-char handle is rejected with
// constants.ErrHandleTooLong. This matches Platform API v2's VARCHAR(40) limit.
func TestValidateHandle_LengthBoundary(t *testing.T) {
	tests := []struct {
		name    string
		handle  string
		wantErr error
	}{
		{"40 chars accepted", strings.Repeat("a", 40), nil},
		{"41 chars rejected", strings.Repeat("a", 41), constants.ErrHandleTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(tt.handle); (tt.wantErr == nil) != (got <= 40) {
				t.Fatalf("test setup wrong: handle len %d vs wantErr %v", got, tt.wantErr)
			}
			err := ValidateHandle(tt.handle)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateHandle(%d chars) = %v, want %v", len(tt.handle), err, tt.wantErr)
			}
		})
	}
}

// TestValidateHandle_ChoreoUUID confirms the Choreo APIM case — a 36-char
// lowercase UUID sent as the resource handle — still validates unchanged.
func TestValidateHandle_ChoreoUUID(t *testing.T) {
	uuid := "e331974d-368b-411b-b038-c6598d85731c" // 36 chars, canonical randomUUID shape
	if len(uuid) != 36 {
		t.Fatalf("test setup wrong: uuid len = %d, want 36", len(uuid))
	}
	if err := ValidateHandle(uuid); err != nil {
		t.Errorf("ValidateHandle(36-char UUID) = %v, want nil", err)
	}
}

// TestSanitizeToHandle_TruncatesTo40 verifies sanitizeToHandle never emits a
// handle longer than 40 chars and never leaves a trailing hyphen when the
// truncation boundary lands on one.
func TestSanitizeToHandle_TruncatesTo40(t *testing.T) {
	// 60 chars, all valid alphanumeric -> truncated straight to 40.
	if got := sanitizeToHandle(strings.Repeat("a", 60)); len(got) != 40 {
		t.Errorf("sanitizeToHandle(60 x 'a') len = %d, want 40", len(got))
	}

	// Boundary lands on a hyphen: "aaa…(39)a bbb…" -> the space becomes a
	// hyphen at position 40, which must be trimmed off after truncation.
	in := strings.Repeat("a", 39) + " " + strings.Repeat("b", 20)
	got := sanitizeToHandle(in)
	if len(got) > 40 {
		t.Errorf("sanitizeToHandle length = %d, want <= 40", len(got))
	}
	if strings.HasSuffix(got, "-") {
		t.Errorf("sanitizeToHandle = %q, must not end with '-'", got)
	}
}
