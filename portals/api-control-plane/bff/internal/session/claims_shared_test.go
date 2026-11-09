/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// sharedCasesPath is the claim-mapping cases the console app runs too
// (src/contexts/auth/userFromClaims.test.ts). Behind a gateway or auth proxy
// the console resolves the user from claims itself, so both implementations
// must give the same user for the same claims.
var sharedCasesPath = filepath.Join("..", "..", "..", "testdata", "claim-mapping-cases.json")

type sharedClaimCase struct {
	Name     string            `json:"name"`
	Mappings map[string]string `json:"mappings"`
	Claims   map[string]any    `json:"claims"`
	Expected map[string]any    `json:"expected"`
}

// mappingFromShared overlays a case's claim names on the defaults, with the
// same keys as [api_control_plane.auth.claim_mappings].
func mappingFromShared(t *testing.T, overrides map[string]string) ClaimMapping {
	t.Helper()
	m := DefaultClaimMapping()
	fields := map[string]*string{
		"username":     &m.Username,
		"email":        &m.Email,
		"roles":        &m.Roles,
		"scope":        &m.Scope,
		"organization": &m.OrgID,
		"org_name":     &m.OrgName,
		"org_handle":   &m.OrgHandle,
		"picture":      &m.Picture,
	}
	for key, value := range overrides {
		field, ok := fields[key]
		if !ok {
			t.Fatalf("unknown claim mapping key %q in shared cases", key)
		}
		*field = value
	}
	return m
}

func TestUserFromClaims_SharedCases(t *testing.T) {
	raw, err := os.ReadFile(sharedCasesPath)
	if err != nil {
		t.Fatalf("read shared cases: %v", err)
	}
	var file struct {
		Cases []sharedClaimCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse shared cases: %v", err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("no shared cases found")
	}

	for _, tc := range file.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			user := UserFromClaims(tc.Claims, nil, mappingFromShared(t, tc.Mappings))

			// Compare as /api/session serializes the user, so omitempty fields
			// match the case's omitted keys.
			encoded, err := json.Marshal(user)
			if err != nil {
				t.Fatalf("marshal user: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("unmarshal user: %v", err)
			}
			if !reflect.DeepEqual(got, tc.Expected) {
				t.Errorf("user mismatch\n got:  %s\n want: %s", encoded, mustJSON(t, tc.Expected))
			}
		})
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
