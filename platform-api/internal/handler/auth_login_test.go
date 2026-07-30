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

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/platform-api/config"
)

// A file-mode user's scope claim is exactly what its roles grant — the roles are
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
			user: config.FileBasedUser{Roles: []string{"ap_viewer"}},
			want: "ap:organization:read",
		},
		{
			// The mapping carries foreign-namespace scopes too, so nothing has to
			// be granted outside it.
			name: "role spanning multiple namespaces",
			user: config.FileBasedUser{Roles: []string{"ap_admin"}},
			want: "ap:organization:manage ap:rest_api:manage dp:org_manage",
		},
		{
			// A role listing the same scope twice must not repeat it in the claim.
			name: "duplicate scope in a role is deduped",
			user: config.FileBasedUser{Roles: []string{"ap_dupes"}},
			want: "ap:rest_api:manage ap:organization:read",
		},
		{
			// Several roles union — most-permissive wins, matching how a token
			// carrying several roles is expanded in role authorization mode.
			name: "multiple roles union their scopes",
			user: config.FileBasedUser{Roles: []string{"ap_viewer", "ap_admin"}},
			want: "ap:organization:read ap:organization:manage ap:rest_api:manage dp:org_manage",
		},
		{
			// A scope two of the user's roles both grant appears once.
			name: "scope granted by two roles is deduped",
			user: config.FileBasedUser{Roles: []string{"ap_admin", "ap_dupes"}},
			want: "ap:organization:manage ap:rest_api:manage dp:org_manage ap:organization:read",
		},
		{
			// validateFileUserRoles rejects this at startup; if it ever reached
			// here the token must carry no scopes rather than a guessed grant.
			name: "unknown role grants nothing",
			user: config.FileBasedUser{Roles: []string{"no-such-role"}},
			want: "",
		},
		{
			// Config validation rejects this at startup; the token must carry no
			// scopes rather than a guessed grant if it ever reached here.
			name: "no roles grants nothing",
			user: config.FileBasedUser{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, h.effectiveScopes(&tt.user))
		})
	}
}

// A claim mapping may be a dot-separated path into a nested claim object, the
// shape Keycloak and similar IDPs use. setClaim is the write-side mirror of the
// middleware's resolveClaimPath, so a mapping configured for that layout must
// round-trip: what this endpoint signs is what the reader finds.
func TestSetClaimNestedPaths(t *testing.T) {
	t.Run("flat path writes a top-level claim", func(t *testing.T) {
		claims := jwt.MapClaims{}
		setClaim(claims, "roles", []string{"ap_admin"})
		assert.Equal(t, []string{"ap_admin"}, claims["roles"])
	})

	t.Run("dotted path writes a nested object", func(t *testing.T) {
		claims := jwt.MapClaims{}
		setClaim(claims, "realm_access.roles", []string{"ap_admin"})

		nested, ok := claims["realm_access"].(map[string]interface{})
		assert.True(t, ok, "realm_access should be a nested object, not a literal key")
		assert.Equal(t, []string{"ap_admin"}, nested["roles"])
		assert.Nil(t, claims["realm_access.roles"], "the dotted name must not survive as a flat key")
	})

	t.Run("mappings sharing a prefix both survive", func(t *testing.T) {
		claims := jwt.MapClaims{}
		setClaim(claims, "realm_access.roles", []string{"ap_admin"})
		setClaim(claims, "realm_access.org_id", "acme")

		nested := claims["realm_access"].(map[string]interface{})
		assert.Equal(t, []string{"ap_admin"}, nested["roles"])
		assert.Equal(t, "acme", nested["org_id"])
	})

	t.Run("deeper nesting is created for every intermediate level", func(t *testing.T) {
		claims := jwt.MapClaims{}
		setClaim(claims, "resource_access.platform-api.roles", []string{"ap_admin"})

		client := claims["resource_access"].(map[string]interface{})["platform-api"].(map[string]interface{})
		assert.Equal(t, []string{"ap_admin"}, client["roles"])
	})

	t.Run("empty path writes nothing", func(t *testing.T) {
		claims := jwt.MapClaims{}
		setClaim(claims, "", "value")
		assert.Empty(t, claims)
	})
}
