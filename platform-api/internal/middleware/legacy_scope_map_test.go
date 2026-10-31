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

// legacy_scope_map_test.go — unit tests for LegacyScopeMap: loading the YAML,
// the startup validation (a legacy name must not still be declared, every target
// must be declared, shapes and namespaces are checked) and Expand's behaviour of
// returning the held scopes plus what each legacy name grants, never fewer.

package middleware

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// legacyTestRegistry declares only tree permissions, the shape of the spec
// after the permission-tree cutover.
func legacyTestRegistry(t *testing.T) *ScopeRegistry {
	t.Helper()
	registry, err := LoadScopeRegistryFromBytes([]byte(`
openapi: 3.0.0
paths:
  /rest-apis:
    get:
      security:
        - oauth2:
            - ap:api:rest:read
            - ap:api:rest:manage
            - ap:api:read
            - ap:api:manage
    post:
      security:
        - oauth2:
            - ap:api:rest:manage
            - ap:api:manage
  /me/api-keys:
    get:
      security:
        - oauth2:
            - ap:credential:api_key:read
            - ap:credential:api_key:all:manage
`))
	if err != nil {
		t.Fatalf("building the scope registry: %v", err)
	}
	return registry
}

func writeLegacyMap(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy-scope-mapping.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func TestLoadLegacyScopeMap(t *testing.T) {
	path := writeLegacyMap(t, `
scopes:
  ap:rest_api:read: [ap:api:rest:read]
  ap:rest_api:manage:
    - ap:api:rest:manage
`)
	m, err := LoadLegacyScopeMap(path)
	if err != nil {
		t.Fatalf("LoadLegacyScopeMap: %v", err)
	}
	want := LegacyScopeMap{
		"ap:rest_api:read":   {"ap:api:rest:read"},
		"ap:rest_api:manage": {"ap:api:rest:manage"},
	}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("LoadLegacyScopeMap = %v, want %v", m, want)
	}
}

// A mapping file that exists but maps nothing is a misconfiguration, not a
// way to disable legacy names — that is an empty legacy_scope_mapping_path.
func TestLoadLegacyScopeMap_RejectsEmptyFile(t *testing.T) {
	path := writeLegacyMap(t, "scopes: {}\n")
	if _, err := LoadLegacyScopeMap(path); err == nil || !strings.Contains(err.Error(), "declares no scopes") {
		t.Fatalf("LoadLegacyScopeMap(empty) error = %v, want 'declares no scopes'", err)
	}
	if _, err := LoadLegacyScopeMap(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("LoadLegacyScopeMap(missing file) succeeded, want error")
	}
}

func TestValidateLegacyScopeMap(t *testing.T) {
	registry := legacyTestRegistry(t)
	tests := []struct {
		name    string
		m       LegacyScopeMap
		wantErr string
	}{
		{
			name: "legacy names onto declared permissions",
			m: LegacyScopeMap{
				"ap:rest_api:read":      {"ap:api:rest:read"},
				"ap:rest_api:create":    {"ap:api:rest:manage"},
				"ap:api_key:all:manage": {"ap:credential:api_key:all:manage"},
			},
		},
		{
			name:    "legacy name still declared by an operation",
			m:       LegacyScopeMap{"ap:api:rest:read": {"ap:api:rest:manage"}},
			wantErr: "still declared",
		},
		{
			name:    "maps to an undeclared scope",
			m:       LegacyScopeMap{"ap:rest_api:read": {"ap:api:rest:reed"}},
			wantErr: "no operation in the OpenAPI spec declares",
		},
		{
			name:    "maps to nothing",
			m:       LegacyScopeMap{"ap:rest_api:read": nil},
			wantErr: "maps to no scope",
		},
		{
			name:    "legacy name outside this server's namespace",
			m:       LegacyScopeMap{"dp:api:read": {"ap:api:rest:read"}},
			wantErr: "malformed legacy scope",
		},
		{
			name:    "wildcard is not a scope",
			m:       LegacyScopeMap{"ap:rest_api:*": {"ap:api:rest:manage"}},
			wantErr: "malformed legacy scope",
		},
		{
			name:    "target outside this server's namespace",
			m:       LegacyScopeMap{"ap:rest_api:read": {"dp:api:read"}},
			wantErr: "malformed scope",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLegacyScopeMap(tc.m, registry)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("ValidateLegacyScopeMap: unexpected error %v", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("ValidateLegacyScopeMap error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestLegacyScopeMap_Expand(t *testing.T) {
	m := LegacyScopeMap{
		"ap:rest_api:read":   {"ap:api:rest:read"},
		"ap:rest_api:manage": {"ap:api:rest:manage"},
		"ap:api_key:read":    {"ap:credential:api_key:read"},
	}
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "legacy names gain their permission, originals kept first",
			in:   []string{"ap:rest_api:read", "ap:organization:read"},
			want: []string{"ap:rest_api:read", "ap:organization:read", "ap:api:rest:read"},
		},
		{
			name: "tree permissions pass through unchanged",
			in:   []string{"ap:api:rest:manage", "ap:api:manage"},
			want: []string{"ap:api:rest:manage", "ap:api:manage"},
		},
		{
			name: "no duplicates when a token carries both names",
			in:   []string{"ap:rest_api:manage", "ap:api:rest:manage", "ap:rest_api:manage"},
			want: []string{"ap:rest_api:manage", "ap:api:rest:manage"},
		},
		{
			name: "empty stays empty",
			in:   nil,
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.Expand(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Expand(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}

	// A nil map is "legacy names disabled": the input comes back untouched.
	in := []string{"ap:rest_api:read"}
	if got := LegacyScopeMap(nil).Expand(in); !reflect.DeepEqual(got, in) {
		t.Errorf("nil map Expand(%v) = %v, want unchanged", in, got)
	}
}

// A role may grant a legacy name — an operator's existing roles file keeps
// loading — but only one the mapping knows; a typo is still rejected.
func TestValidateRoleScopeMap_AcceptsMappedLegacyNames(t *testing.T) {
	registry := legacyTestRegistry(t)
	legacy := LegacyScopeMap{"ap:rest_api:manage": {"ap:api:rest:manage"}}

	if err := ValidateRoleScopeMap(map[string][]string{"ap_publisher": {"ap:rest_api:manage"}}, registry, legacy); err != nil {
		t.Errorf("legacy name the mapping knows was rejected: %v", err)
	}
	if err := ValidateRoleScopeMap(map[string][]string{"ap_publisher": {"ap:rest_api:manage"}}, registry, nil); err == nil {
		t.Error("legacy name was accepted with legacy names disabled")
	}
	if err := ValidateRoleScopeMap(map[string][]string{"ap_publisher": {"ap:rest_api:mange"}}, registry, legacy); err == nil || !strings.Contains(err.Error(), "unknown scope") {
		t.Errorf("misspelt scope error = %v, want 'unknown scope'", err)
	}
}
