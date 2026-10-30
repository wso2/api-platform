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

package migration

import (
	"strings"
	"testing"
)

// The table row of a resource is its Verify report; whatever the migrate pass
// warned about (a renamed API key, say) must reach that row, in front of the
// parity note, and lift a PASS to WARN. A clean parity check never hides a
// FAIL from the migrate pass, and a migrate pass with nothing to say leaves
// the Verify report untouched.
func TestMergeMigrateReport(t *testing.T) {
	cases := []struct {
		name       string
		migrated   *ResourceReport
		verified   ResourceReport
		wantStatus Status
		wantFirst  string
		wantCount  int
	}{
		{"verify-only run (no migrate pass)", nil, ResourceReport{Status: StatusPass}, StatusPass, "", 0},
		{"quiet migrate pass", &ResourceReport{Status: StatusPass}, ResourceReport{Status: StatusPass, Messages: []string{"parity ok"}}, StatusPass, "parity ok", 1},
		{"migrate WARN lifts a PASS", &ResourceReport{Status: StatusWarn, Messages: []string{"api_key k1 renamed"}}, ResourceReport{Status: StatusPass}, StatusWarn, "api_key k1 renamed", 1},
		{"migrate WARN in a dry-run (verify skipped)", &ResourceReport{Status: StatusWarn, Messages: []string{"api_key k1 renamed"}}, ResourceReport{Status: StatusSkip, Messages: []string{"no target"}}, StatusWarn, "api_key k1 renamed", 2},
		{"migrate WARN never hides a verify FAIL", &ResourceReport{Status: StatusWarn, Messages: []string{"w"}}, ResourceReport{Status: StatusFail, Messages: []string{"parity 3 != 2"}}, StatusFail, "w", 2},
		{"migrate FAIL survives a clean parity check", &ResourceReport{Status: StatusFail, Messages: []string{"bad row"}}, ResourceReport{Status: StatusPass}, StatusFail, "bad row", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.verified
			got := mergeMigrateReport(tc.migrated, &v)
			if got.Status != tc.wantStatus {
				t.Fatalf("status = %s, want %s (messages %v)", got.Status, tc.wantStatus, got.Messages)
			}
			if len(got.Messages) != tc.wantCount {
				t.Fatalf("messages = %v, want %d", got.Messages, tc.wantCount)
			}
			if tc.wantFirst != "" && !strings.HasPrefix(got.Messages[0], tc.wantFirst) {
				t.Fatalf("first message = %q, want %q first", got.Messages[0], tc.wantFirst)
			}
		})
	}
}
