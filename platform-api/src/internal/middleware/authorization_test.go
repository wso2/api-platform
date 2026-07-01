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

package middleware

import "testing"

func TestScopeSatisfies(t *testing.T) {
	tests := []struct {
		name     string
		have     string
		required string
		want     bool
	}{
		// Exact matches.
		{"exact action", "ap:gateway:create", "ap:gateway:create", true},
		{"exact manage", "ap:gateway:manage", "ap:gateway:manage", true},
		{"exact sub-resource", "ap:gateway:token:create", "ap:gateway:token:create", true},
		{"mismatch action", "ap:gateway:create", "ap:gateway:read", false},
		{"mismatch resource", "ap:gateway:create", "ap:rest_api:create", false},

		// Top-level wildcard "ap:*" — every action on root-level resources.
		{"root wildcard covers resource action", "ap:*", "ap:gateway:create", true},
		{"root wildcard covers resource read", "ap:*", "ap:rest_api:read", true},
		{"root wildcard covers resource manage", "ap:*", "ap:project:manage", true},
		{"root wildcard rejects sub-resource", "ap:*", "ap:gateway:token:create", false},
		{"root wildcard rejects deep sub-resource", "ap:*", "ap:application:api_key:read", false},

		// Resource wildcard "ap:<resource>:*" — all actions directly on the resource.
		{"resource wildcard covers action", "ap:gateway:*", "ap:gateway:create", true},
		{"resource wildcard covers read", "ap:gateway:*", "ap:gateway:read", true},
		{"resource wildcard covers manage", "ap:gateway:*", "ap:gateway:manage", true},
		{"resource wildcard rejects sub-resource", "ap:gateway:*", "ap:gateway:token:create", false},
		{"resource wildcard rejects other resource", "ap:gateway:*", "ap:rest_api:create", false},
		{"resource wildcard rejects prefix sibling", "ap:gateway:*", "ap:gateway_custom_policy:read", false},

		// Sub-resource wildcard "ap:<resource>:<sub>:*" — all actions on that sub-resource.
		{"sub-resource wildcard covers action", "ap:gateway:token:*", "ap:gateway:token:create", true},
		{"sub-resource wildcard covers read", "ap:gateway:token:*", "ap:gateway:token:read", true},
		{"sub-resource wildcard rejects parent action", "ap:gateway:token:*", "ap:gateway:create", false},
		{"sub-resource wildcard rejects deeper", "ap:gateway:token:*", "ap:gateway:token:scope:read", false},

		// Wildcards never match a prefix or transitively.
		{"resource wildcard not a prefix match", "ap:gateway:*", "ap:gateway", false},
		{"root wildcard not transitive", "ap:*", "ap:gateway:token:read", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := scopeSatisfies(tc.have, tc.required); got != tc.want {
				t.Errorf("scopeSatisfies(%q, %q) = %v, want %v", tc.have, tc.required, got, tc.want)
			}
		})
	}
}
