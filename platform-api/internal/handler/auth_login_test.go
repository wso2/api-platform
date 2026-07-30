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

// A file-mode user's scope claim is the union of the scopes its role grants and
// the scopes granted directly. The role covers the platform scopes (validated
// against the OpenAPI spec at startup); the direct list carries scopes this
// server does not declare, such as the Developer Portal's "dp:*" scopes.
func TestEffectiveScopes(t *testing.T) {
	h := NewAuthLoginHandler(&config.Server{}, map[string][]string{
		"ap_admin":  {"ap:organization:manage", "ap:rest_api:manage"},
		"ap_viewer": {"ap:organization:read"},
	})

	tests := []struct {
		name string
		user config.FileBasedUser
		want string
	}{
		{
			name: "role only",
			user: config.FileBasedUser{Role: "ap_viewer"},
			want: "ap:organization:read",
		},
		{
			name: "scopes only",
			user: config.FileBasedUser{Scopes: "dp:org_manage ap:devportal:manage"},
			want: "dp:org_manage ap:devportal:manage",
		},
		{
			// Role scopes first, then the extras the mapping cannot name.
			name: "role unioned with direct scopes",
			user: config.FileBasedUser{Role: "ap_admin", Scopes: "dp:org_manage"},
			want: "ap:organization:manage ap:rest_api:manage dp:org_manage",
		},
		{
			// A direct scope the role already grants must not appear twice.
			name: "overlapping scope is deduped",
			user: config.FileBasedUser{Role: "ap_admin", Scopes: "ap:rest_api:manage dp:org_manage"},
			want: "ap:organization:manage ap:rest_api:manage dp:org_manage",
		},
		{
			// validateFileUserRoles rejects this at startup; if it ever reaches
			// here the direct scopes must still be honored rather than dropped.
			name: "unknown role falls back to direct scopes",
			user: config.FileBasedUser{Role: "no-such-role", Scopes: "dp:org_manage"},
			want: "dp:org_manage",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, h.effectiveScopes(&tt.user))
		})
	}
}
