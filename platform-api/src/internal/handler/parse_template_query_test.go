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

func TestParseTemplateQuery(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantGroupID string
		wantVersion string
		wantFound   bool
	}{
		{"empty", "", "", "", false},
		{"groupId only", "groupId:openai", "openai", "", true},
		{"slug groupId", "groupId:deep-seek", "deep-seek", "", true},
		{"groupId and version", "groupId:openai&version:v1.0", "openai", "v1.0", true},
		{"order independent", "version:v2.0&groupId:openai", "openai", "v2.0", true},
		{"unknown key ignored", "displayName:ab&groupId:openai", "openai", "", true},
		{"whitespace trimmed", " groupId : openai & version : v1.0 ", "openai", "v1.0", true},
		{"malformed token skipped", "garbage&groupId:openai&version:v3.0", "openai", "v3.0", true},
		{"version without groupId is not found", "version:v2.0", "", "v2.0", false},
		{"blank groupId value", "groupId:", "", "", true},
		{"blank groupId whitespace", "groupId:   ", "", "", true},
		{"blank groupId with version", "version:v2.0&groupId:", "", "v2.0", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, found := parseTemplateQuery(tc.raw)
			if q.GroupID != tc.wantGroupID || q.Version != tc.wantVersion || found != tc.wantFound {
				t.Fatalf("parseTemplateQuery(%q) = (%+v, %t), want (groupId=%q version=%q, %t)",
					tc.raw, q, found, tc.wantGroupID, tc.wantVersion, tc.wantFound)
			}
		})
	}
}
