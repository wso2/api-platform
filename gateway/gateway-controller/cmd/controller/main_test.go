/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

// Tests for generateAuthConfig function

func TestGenerateAuthConfig(t *testing.T) {
	t.Run("No authentication enabled", func(t *testing.T) {
		cfg := &config.Config{
			Controller: config.Controller{
				Auth: config.AuthConfig{
					Basic: config.BasicAuth{
						Enabled: false,
					},
					IDP: config.IDPConfig{
						Enabled: false,
					},
				},
			},
		}

		authConfig := generateAuthConfig(cfg)

		assert.False(t, authConfig.BasicAuth.Enabled)
		assert.False(t, authConfig.JWTConfig.Enabled)
		assert.NotNil(t, authConfig.ResourceRoles)
	})

	t.Run("Basic auth enabled with users", func(t *testing.T) {
		cfg := &config.Config{
			Controller: config.Controller{
				Auth: config.AuthConfig{
					Basic: config.BasicAuth{
						Enabled: true,
						Users: []config.AuthUser{
							{
								Username:       "admin",
								Password:       "admin123",
								PasswordHashed: false,
								Roles:          []string{"admin"},
							},
							{
								Username:       "developer",
								Password:       "dev123",
								PasswordHashed: true,
								Roles:          []string{"developer"},
							},
						},
					},
					IDP: config.IDPConfig{
						Enabled: false,
					},
				},
			},
		}

		authConfig := generateAuthConfig(cfg)

		assert.True(t, authConfig.BasicAuth.Enabled)
		assert.Len(t, authConfig.BasicAuth.Users, 2)
		assert.Equal(t, "admin", authConfig.BasicAuth.Users[0].Username)
		assert.Equal(t, "admin123", authConfig.BasicAuth.Users[0].Password)
		assert.False(t, authConfig.BasicAuth.Users[0].PasswordHashed)
		assert.Equal(t, []string{"admin"}, authConfig.BasicAuth.Users[0].Roles)
		assert.Equal(t, "developer", authConfig.BasicAuth.Users[1].Username)
		assert.True(t, authConfig.BasicAuth.Users[1].PasswordHashed)
	})

	t.Run("IDP auth enabled", func(t *testing.T) {
		roleMapping := map[string][]string{
			"admin":     {"gateway-admin"},
			"developer": {"gateway-dev"},
		}
		cfg := &config.Config{
			Controller: config.Controller{
				Auth: config.AuthConfig{
					Basic: config.BasicAuth{
						Enabled: false,
					},
					IDP: config.IDPConfig{
						Enabled:     true,
						Issuer:      "https://idp.example.com",
						JWKSURL:     "https://idp.example.com/.well-known/jwks.json",
						RolesClaim:  "roles",
						RoleMapping: roleMapping,
					},
				},
			},
		}

		authConfig := generateAuthConfig(cfg)

		assert.False(t, authConfig.BasicAuth.Enabled)
		assert.True(t, authConfig.JWTConfig.Enabled)
		assert.Equal(t, "https://idp.example.com", authConfig.JWTConfig.IssuerURL)
		assert.Equal(t, "https://idp.example.com/.well-known/jwks.json", authConfig.JWTConfig.JWKSUrl)
		assert.Equal(t, "roles", authConfig.JWTConfig.ScopeClaim)
		assert.NotNil(t, authConfig.JWTConfig.PermissionMapping)
	})

	t.Run("Both basic and IDP auth enabled", func(t *testing.T) {
		cfg := &config.Config{
			Controller: config.Controller{
				Auth: config.AuthConfig{
					Basic: config.BasicAuth{
						Enabled: true,
						Users: []config.AuthUser{
							{Username: "admin", Password: "admin123", Roles: []string{"admin"}},
						},
					},
					IDP: config.IDPConfig{
						Enabled:    true,
						Issuer:     "https://idp.example.com",
						JWKSURL:    "https://idp.example.com/.well-known/jwks.json",
						RolesClaim: "roles",
					},
				},
			},
		}

		authConfig := generateAuthConfig(cfg)

		assert.True(t, authConfig.BasicAuth.Enabled)
		assert.True(t, authConfig.JWTConfig.Enabled)
	})

	t.Run("Resource roles are populated correctly", func(t *testing.T) {
		cfg := &config.Config{
			Controller: config.Controller{
				Auth: config.AuthConfig{
					Basic: config.BasicAuth{Enabled: false},
					IDP:   config.IDPConfig{Enabled: false},
				},
			},
		}

		authConfig := generateAuthConfig(cfg)

		// Check some expected resource roles
		assert.Contains(t, authConfig.ResourceRoles, "POST /rest-apis")
		assert.Contains(t, authConfig.ResourceRoles, "GET /rest-apis")
		assert.Contains(t, authConfig.ResourceRoles, "GET /policies")
		assert.NotContains(t, authConfig.ResourceRoles, "GET /config_dump")
		assert.NotContains(t, authConfig.ResourceRoles, "GET /xds_sync_status")

		// Check role assignments
		assert.Contains(t, authConfig.ResourceRoles["POST /rest-apis"], "admin")
		assert.Contains(t, authConfig.ResourceRoles["POST /rest-apis"], "developer")
	})
}
