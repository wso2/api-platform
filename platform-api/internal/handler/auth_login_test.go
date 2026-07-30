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

package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/platform-api/config"
)

// A file-mode user's scope claim is exactly what its role grants — the role is
// the whole grant, so the mapping file is the only place the user's privileges
// are defined. The mapping may name scopes in any component's namespace (the
// Developer Portal's "dp:*", for example), which is what makes a per-user scope
// list unnecessary.
func TestEffectiveScopes(t *testing.T) {
	h := NewAuthLoginHandler(&config.Server{}, map[string][]string{
		"ap_admin":  {"ap:organization:manage", "ap:rest_api:manage", "dp:org_manage"},
		"ap_viewer": {"ap:organization:read"},
		"ap_dupes":  {"ap:rest_api:manage", "ap:organization:read", "ap:rest_api:manage"},
	})

	tests := []struct {
		name string
		user config.FileBasedUser
		want string
	}{
		{
			name: "role expands to its scopes, in order",
			user: config.FileBasedUser{Role: "ap_viewer"},
			want: "ap:organization:read",
		},
		{
			// The mapping carries foreign-namespace scopes too, so nothing has to
			// be granted outside it.
			name: "role spanning multiple namespaces",
			user: config.FileBasedUser{Role: "ap_admin"},
			want: "ap:organization:manage ap:rest_api:manage dp:org_manage",
		},
		{
			// A role listing the same scope twice must not repeat it in the claim.
			name: "duplicate scope in a role is deduped",
			user: config.FileBasedUser{Role: "ap_dupes"},
			want: "ap:rest_api:manage ap:organization:read",
		},
		{
			// validateFileUserRoles rejects this at startup; if it ever reached
			// here the token must carry no scopes rather than a guessed grant.
			name: "unknown role grants nothing",
			user: config.FileBasedUser{Role: "no-such-role"},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, h.effectiveScopes(&tt.user))
		})
	}
}
