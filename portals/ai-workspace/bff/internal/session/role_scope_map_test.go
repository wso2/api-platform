/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package session

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeMapping writes a grant table to a temp file and returns its path.
func writeMapping(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "role-to-scope-mapping.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write mapping: %v", err)
	}
	return path
}

const sampleMapping = `
roles:
  - name: ap_admin
    scopes:
      - ap:organization:manage
      - ap:project:manage
      - ap:project:manage
  - name: ap_viewer
    scopes:
      - ap:project:read
`

func TestLoadRoleScopeMap(t *testing.T) {
	m, err := LoadRoleScopeMap(writeMapping(t, sampleMapping))
	if err != nil {
		t.Fatalf("LoadRoleScopeMap: %v", err)
	}
	// The duplicate ap:project:manage is collapsed, not carried twice.
	want := []string{"ap:organization:manage", "ap:project:manage"}
	if !slices.Equal(m["ap_admin"], want) {
		t.Errorf("ap_admin = %v, want %v", m["ap_admin"], want)
	}
	if !slices.Equal(m["ap_viewer"], []string{"ap:project:read"}) {
		t.Errorf("ap_viewer = %v", m["ap_viewer"])
	}
}

func TestLoadRoleScopeMap_Rejects(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		wantErr  string
	}{
		{
			// Last-wins would leave one entry silently inert, decided by file order.
			name:     "duplicate role",
			contents: "roles:\n  - name: ap_admin\n    scopes: [ap:project:read]\n  - name: ap_admin\n    scopes: [ap:project:manage]\n",
			wantErr:  "declared more than once",
		},
		{
			name:     "entry without name",
			contents: "roles:\n  - scopes: [ap:project:read]\n",
			wantErr:  "has no \"name\"",
		},
		{
			name:     "no roles list",
			contents: "something_else: true\n",
			wantErr:  "must contain a non-empty top-level",
		},
		{
			name:     "not yaml",
			contents: "roles: [unclosed\n",
			wantErr:  "is not valid YAML",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadRoleScopeMap(writeMapping(t, tc.contents))
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestLoadRoleScopeMap_RejectsBadPaths(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{"empty", "", "not a usable file path"},
		{"null byte", dir + "/map\x00.yaml", "not a usable file path"},
		// Checked on the raw input: filepath.Clean would collapse this to a path
		// containing no ".." at all, which a later check could not catch.
		{"traversal", dir + "/../../etc/passwd", "traversal sequences"},
		{"missing", dir + "/absent.yaml", "could not be read"},
		{"directory", dir, "is not a file"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadRoleScopeMap(tc.path)
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// The real grant table both services read must load and expand — a guard against the
// shipped file drifting into a shape this loader rejects.
func TestLoadRoleScopeMap_ShippedFile(t *testing.T) {
	// Resolved to an absolute path: the loader rejects any ".." in the configured
	// value, and a real deployment always mounts the table at an absolute path.
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "platform-api", "resources", "role-to-scope-mapping.yaml"))
	if err != nil {
		t.Fatalf("resolve shipped grant table path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("shipped grant table not present at %s", path)
	}
	m, err := LoadRoleScopeMap(path)
	if err != nil {
		t.Fatalf("LoadRoleScopeMap(shipped): %v", err)
	}
	if len(ExpandRoles([]string{"ap_admin"}, m)) == 0 {
		t.Error("ap_admin expanded to no scopes in the shipped grant table")
	}
}

func TestExpandRoles(t *testing.T) {
	m := map[string][]string{
		"ap_admin":  {"ap:organization:manage", "ap:project:manage"},
		"ap_viewer": {"ap:project:read", "ap:project:manage"},
	}
	tests := []struct {
		name  string
		roles []string
		want  []string
	}{
		{"single role", []string{"ap_admin"}, []string{"ap:organization:manage", "ap:project:manage"}},
		{
			// Union, first-seen order, no duplicate for the shared scope.
			name:  "several roles union",
			roles: []string{"ap_admin", "ap_viewer"},
			want:  []string{"ap:organization:manage", "ap:project:manage", "ap:project:read"},
		},
		// An unmapped role grants nothing — the same deny-by-default result the
		// Platform API reaches for the same token.
		{"unknown role", []string{"some_other_group"}, []string{}},
		{"no roles", nil, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExpandRoles(tc.roles, m); !slices.Equal(got, tc.want) {
				t.Errorf("ExpandRoles(%v) = %v, want %v", tc.roles, got, tc.want)
			}
		})
	}
}

func TestExpandRoles_NilMap(t *testing.T) {
	if got := ExpandRoles([]string{"ap_admin"}, nil); len(got) != 0 {
		t.Errorf("ExpandRoles with nil map = %v, want empty", got)
	}
}
