/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package handler

import "testing"

func TestParseTemplateQueryGroupID(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantGroupID string
		wantFound   bool
	}{
		{"empty", "", "", false},
		{"groupId only", "groupId:openai", "openai", true},
		{"slug groupId", "groupId:deep-seek", "deep-seek", true},
		{"extra keys ignored", "groupId:openai&version:v2", "openai", true},
		{"order independent", "version:v2&groupId:openai", "openai", true},
		{"unknown key ignored", "displayName:ab&groupId:openai", "openai", true},
		{"whitespace trimmed", " groupId : openai ", "openai", true},
		{"malformed token skipped", "garbage&groupId:openai", "openai", true},
		{"no groupId key", "version:v2", "", false},
		{"blank groupId value", "groupId:", "", true},
		{"blank groupId whitespace", "groupId:   ", "", true},
		{"blank groupId with other keys", "version:v2&groupId:", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := parseTemplateQueryGroupID(tc.raw)
			if got != tc.wantGroupID || found != tc.wantFound {
				t.Fatalf("parseTemplateQueryGroupID(%q) = (%q, %t), want (%q, %t)",
					tc.raw, got, found, tc.wantGroupID, tc.wantFound)
			}
		})
	}
}
