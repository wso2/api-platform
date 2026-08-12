/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package config

import "testing"

// parseSkipJWTValidation backs the ldflags-stamped build var. It is true only for
// the literal "true" (case-insensitive, surrounding space trimmed) — the empty
// value every normal build carries, and any other value, keep strict validation.
func TestParseSkipJWTValidation(t *testing.T) {
	cases := []struct {
		name string
		val  string
		want bool
	}{
		{"empty default is strict", "", false},
		{"explicit false", "false", false},
		{"arbitrary value is strict", "1", false},
		{"true enables bypass", "true", true},
		{"uppercase TRUE", "TRUE", true},
		{"padded true", "  true  ", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSkipJWTValidation(tc.val); got != tc.want {
				t.Errorf("parseSkipJWTValidation(%q) = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}
