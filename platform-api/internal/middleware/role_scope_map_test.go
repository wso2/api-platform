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

import (
	"strings"
	"testing"
)

// A role may grant scopes belonging to a sibling component (the Developer
// Portal's "dp:*"), which this server mints but never enforces and therefore
// cannot check for existence. Its own "ap:" namespace is still validated against
// the spec, and a scope with no namespace at all is rejected in either case.
func TestValidateRoleScopeMap_NamespaceScoping(t *testing.T) {
	registry, err := LoadScopeRegistryFromBytes([]byte(`
openapi: 3.0.0
paths:
  /apis:
    get:
      security:
        - oauth2:
            - ap:rest_api:read
`))
	if err != nil {
		t.Fatalf("building the scope registry: %v", err)
	}

	tests := []struct {
		name    string
		scopes  []string
		wantErr string
	}{
		{
			name:   "declared platform scope",
			scopes: []string{"ap:rest_api:read"},
		},
		{
			name:   "foreign-namespace scope passes without being declared here",
			scopes: []string{"dp:org_manage", "dp:api_key_revoke"},
		},
		{
			name:    "undeclared platform scope is rejected",
			scopes:  []string{"ap:rest_api:reed"},
			wantErr: "unknown scope",
		},
		{
			// The one error still detectable in a namespace this server can't
			// validate: a scope that isn't namespaced at all.
			name:    "scope with no namespace is rejected",
			scopes:  []string{"dporg_manage"},
			wantErr: "malformed scope",
		},
		{
			name:    "uppercase scope is rejected as malformed",
			scopes:  []string{"DP:org_manage"},
			wantErr: "malformed scope",
		},
		{
			// This server mints these for the AI Workspace BFF to enforce, so they
			// appear in no spec it can check. A role must still be able to name
			// them — it is a file-mode user's only grant.
			name:   "minted platform scope passes without being declared here",
			scopes: []string{"ap:devportal:manage", "ap:git:read"},
		},
		{
			// The allowlist is exact, not a prefix: a typo inside it still fails.
			name:    "typo in a minted scope is still rejected",
			scopes:  []string{"ap:devportal:mange"},
			wantErr: "unknown scope",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoleScopeMap(map[string][]string{"ap_admin": tt.scopes}, registry)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected an error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
