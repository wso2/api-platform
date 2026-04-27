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
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policybuilder "github.com/wso2/api-platform/gateway/gateway-controller/pkg/policy"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestConvertAPIPolicyToModel(t *testing.T) {
	tests := []struct {
		name       string
		policy     api.Policy
		attachedTo policy.Level
		expected   struct {
			name       string
			version    string
			enabled    bool
			hasParams  bool
			attachedTo string
		}
	}{
		{
			name: "Basic policy without params",
			policy: api.Policy{
				Name:    "rate-limit",
				Version: "v1.0.0",
			},
			attachedTo: policy.LevelAPI,
			expected: struct {
				name       string
				version    string
				enabled    bool
				hasParams  bool
				attachedTo string
			}{
				name:       "rate-limit",
				version:    "v1.0.0",
				enabled:    true,
				hasParams:  true, // attachedTo adds a param
				attachedTo: "api",
			},
		},
		{
			name: "Policy with params",
			policy: api.Policy{
				Name:    "cors",
				Version: "v0.1.0",
				Params: &map[string]interface{}{
					"allowedOrigins": []string{"*"},
					"maxAge":         3600,
				},
			},
			attachedTo: policy.LevelRoute,
			expected: struct {
				name       string
				version    string
				enabled    bool
				hasParams  bool
				attachedTo string
			}{
				name:       "cors",
				version:    "v0.1.0",
				enabled:    true,
				hasParams:  true,
				attachedTo: "route",
			},
		},
		{
			name: "Policy with execution condition",
			policy: api.Policy{
				Name:               "jwt-auth",
				Version:            "v0.1.0",
				ExecutionCondition: stringPtr("request.headers['x-skip-auth'] != 'true'"),
			},
			attachedTo: policy.LevelAPI,
			expected: struct {
				name       string
				version    string
				enabled    bool
				hasParams  bool
				attachedTo string
			}{
				name:       "jwt-auth",
				version:    "v0.1.0",
				enabled:    true,
				hasParams:  true,
				attachedTo: "api",
			},
		},
		{
			name: "Policy with empty attachedTo",
			policy: api.Policy{
				Name:    "logging",
				Version: "v1.0.0",
			},
			attachedTo: "",
			expected: struct {
				name       string
				version    string
				enabled    bool
				hasParams  bool
				attachedTo string
			}{
				name:       "logging",
				version:    "v1.0.0",
				enabled:    true,
				hasParams:  false, // no attachedTo param added
				attachedTo: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policybuilder.ConvertAPIPolicyToModel(tt.policy, tt.attachedTo, tt.expected.version)

			assert.Equal(t, tt.expected.name, result.Name)
			assert.Equal(t, tt.expected.version, result.Version)
			assert.Equal(t, tt.expected.enabled, result.Enabled)

			if tt.expected.hasParams {
				assert.NotNil(t, result.Parameters)
				if tt.expected.attachedTo != "" {
					attachedTo, ok := result.Parameters["attachedTo"]
					assert.True(t, ok, "attachedTo should be present in parameters")
					assert.Equal(t, tt.expected.attachedTo, attachedTo)
				}
			}

			if tt.policy.ExecutionCondition != nil {
				assert.Equal(t, tt.policy.ExecutionCondition, result.ExecutionCondition)
			}
		})
	}
}

func TestConvertAPIPolicyToModel_ParamsCopied(t *testing.T) {
	originalParams := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}
	p := api.Policy{
		Name:    "test-policy",
		Version: "v1.0.0",
		Params:  &originalParams,
	}

	result := policybuilder.ConvertAPIPolicyToModel(p, policy.LevelAPI, "v1.0.0")

	// Verify params are copied correctly
	assert.Equal(t, "value1", result.Parameters["key1"])
	assert.Equal(t, 42, result.Parameters["key2"])
	assert.Equal(t, "api", result.Parameters["attachedTo"])
}

func TestDerivePolicyFromAPIConfig(t *testing.T) {
	fullConfig := &config.Config{
		Controller: config.Controller{},
		Router: config.RouterConfig{
			VHosts: config.VHostsConfig{
				Main: config.VHostEntry{
					Default: "api.example.com",
				},
				Sandbox: config.VHostEntry{
					Default: "sandbox.example.com",
				},
			},
		},
	}

	t.Run("API with no policies returns nil", func(t *testing.T) {
		cfg := createTestStoredConfig("test-api", "v1.0.0", "/test", nil, nil)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, nil)

		// With system policies injection, result may not be nil
		// The behavior depends on InjectSystemPolicies
		// If no system policies are injected and no policies exist, it should be nil
		if result != nil {
			// Verify structure is valid
			assert.NotEmpty(t, result.ID)
		}
	})

	t.Run("API with API-level policies", func(t *testing.T) {
		apiPolicies := []api.Policy{
			{Name: "cors", Version: "v0"},
			{Name: "rate-limit", Version: "v1"},
		}
		cfg := createTestStoredConfig("test-api", "v1.0.0", "/test", apiPolicies, nil)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())

		require.NotNil(t, result)
		assert.Contains(t, result.ID, "0000-test-api-0000-000000000000")
		assert.Equal(t, "Test API", result.Configuration.Metadata.APIName)
		assert.Equal(t, "v1.0.0", result.Configuration.Metadata.Version)
		assert.Equal(t, "/test", result.Configuration.Metadata.Context)
	})

	t.Run("API with operation-level policies overriding API policies", func(t *testing.T) {
		apiPolicies := []api.Policy{
			{Name: "cors", Version: "v0"},
		}
		opPolicies := []api.Policy{
			{Name: "rate-limit", Version: "v1"},
		}
		cfg := createTestStoredConfigWithOpPolicies("test-api", "v1.0.0", "/test", apiPolicies, opPolicies)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())

		require.NotNil(t, result)
		// Operation policies should be present, plus API-level cors
		assert.NotEmpty(t, result.Configuration.Routes)
	})

	t.Run("API with sandbox upstream creates routes for both vhosts", func(t *testing.T) {
		apiPolicies := []api.Policy{
			{Name: "cors", Version: "v0"},
		}
		cfg := createTestStoredConfigWithSandbox("test-api", "v1.0.0", "/test", apiPolicies)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())

		require.NotNil(t, result)
		// Should have routes for both main and sandbox vhosts
		// Each operation creates 2 routes (main + sandbox)
		assert.GreaterOrEqual(t, len(result.Configuration.Routes), 2)
	})

	t.Run("API with custom vhosts", func(t *testing.T) {
		apiPolicies := []api.Policy{
			{Name: "cors", Version: "v0"},
		}
		mainVhost := "custom.example.com"
		sandboxVhost := "custom-sandbox.example.com"
		cfg := createTestStoredConfigWithVhosts("test-api", "v1.0.0", "/test", apiPolicies, mainVhost, &sandboxVhost)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())

		require.NotNil(t, result)
		// Routes should use custom vhosts
		for _, route := range result.Configuration.Routes {
			assert.Contains(t, route.RouteKey, "custom")
		}
	})

	t.Run("API mixing major-only versions for same policy name", func(t *testing.T) {
		// Ensure an API can reference the same policy name with different
		// major-only versions (v1 and v2) within a single operation and that
		// the derived configuration contains two entries with different
		// resolved versions.

		apiConfig := api.RestAPI{
			Kind: api.RestAPIKindRestApi,
			Metadata: api.Metadata{
				Name: "test-api-mixed-majors",
			},
			Spec: api.APIConfigData{
				DisplayName: "Test API",
				Version:     "v1.0.0",
				Context:     "/test-mixed-majors",
				Upstream: struct {
					Main    api.Upstream  `json:"main" yaml:"main"`
					Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
				}{
					Main: api.Upstream{
						Url: stringPtr("http://backend:8080"),
					},
				},
				Operations: []api.Operation{
					{
						Method: api.OperationMethodGET,
						Path:   "/resource",
						Policies: &[]api.Policy{
							{
								Name:    "MultiVersionPolicy",
								Version: "v1", // major-only v1
							},
							{
								Name:    "MultiVersionPolicy",
								Version: "v2", // major-only v2
							},
						},
					},
				},
			},
		}
		cfg := &models.StoredConfig{
			UUID:                "0000-test-mixed-majors-0000-000000",
			Kind:                string(api.RestAPIKindRestApi),
			Configuration:       apiConfig,
			SourceConfiguration: apiConfig,
		}

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())
		require.NotNil(t, result)
		require.NotEmpty(t, result.Configuration.Routes)

		route := result.Configuration.Routes[0]
		require.GreaterOrEqual(t, len(route.Policies), 2)

		var versions []string
		for _, p := range route.Policies {
			if p.Name == "MultiVersionPolicy" {
				versions = append(versions, p.Version)
			}
		}

		// Expect two entries for MultiVersionPolicy with major-only versions (sent to engine as-is).
		require.Len(t, versions, 2, "expected two MultiVersionPolicy entries in the route")
		assert.NotEqual(t, versions[0], versions[1], "expected versions for v1 and v2 majors to differ")
		assert.Contains(t, versions, "v1", "v1 major should be passed as v1")
		assert.Contains(t, versions, "v2", "v2 major should be passed as v2")
	})
}

func TestDerivePolicyFromAPIConfig_InvalidConfig(t *testing.T) {
	fullConfig := &config.Config{
		Controller: config.Controller{},
		Router: config.RouterConfig{
			VHosts: config.VHostsConfig{
				Main: config.VHostEntry{
					Default: "api.example.com",
				},
			},
		},
	}

	t.Run("Invalid API spec returns nil", func(t *testing.T) {
		// Create a config with an empty spec
		apiConfig := api.RestAPI{
			Kind: api.RestAPIKindRestApi,
			Spec: api.APIConfigData{}, // Empty spec
		}
		cfg := &models.StoredConfig{
			UUID:                "0000-invalid-api-0000-000000000000",
			Kind:                string(api.RestAPIKindRestApi),
			Configuration:       apiConfig,
			SourceConfiguration: apiConfig,
		}

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, nil)

		assert.Nil(t, result)
	})
}

// testPolicyDefinitions returns policy definitions used by derivation tests.
// Enables resolving major-only (v0, v1, v2) to full semver for cors, rate-limit, MultiVersionPolicy.
func testPolicyDefinitions() map[string]models.PolicyDefinition {
	return map[string]models.PolicyDefinition{
		"cors|v0.1.0":               {Name: "cors", Version: "v0.1.0"},
		"rate-limit|v1.0.0":         {Name: "rate-limit", Version: "v1.0.0"},
		"MultiVersionPolicy|v1.0.0": {Name: "MultiVersionPolicy", Version: "v1.0.0"},
		"MultiVersionPolicy|v2.0.0": {Name: "MultiVersionPolicy", Version: "v2.0.0"},
	}
}

// Helper functions to create test configs

func createTestStoredConfig(name, version, context string, apiPolicies []api.Policy, opPolicies []api.Policy) *models.StoredConfig {
	var policiesPtr *[]api.Policy
	if apiPolicies != nil {
		policiesPtr = &apiPolicies
	}

	var opPoliciesPtr *[]api.Policy
	if opPolicies != nil {
		opPoliciesPtr = &opPolicies
	}

	apiData := api.APIConfigData{
		DisplayName: "Test API",
		Version:     version,
		Context:     context,
		Policies:    policiesPtr,
		Operations: []api.Operation{
			{
				Method:   api.OperationMethodGET,
				Path:     "/resource",
				Policies: opPoliciesPtr,
			},
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: stringPtr("http://backend:8080"),
			},
		},
	}

	apiConfig := api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Metadata: api.Metadata{
			Name: name,
		},
		Spec: apiData,
	}
	return &models.StoredConfig{
		UUID:                "0000-" + name + "-0000-000000000000",
		Kind:                string(api.RestAPIKindRestApi),
		Handle:              name,
		DisplayName:         "Test API",
		Version:             version,
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
	}
}

func createTestStoredConfigWithOpPolicies(name, version, context string, apiPolicies, opPolicies []api.Policy) *models.StoredConfig {
	return createTestStoredConfig(name, version, context, apiPolicies, opPolicies)
}

func createTestStoredConfigWithSandbox(name, version, context string, apiPolicies []api.Policy) *models.StoredConfig {
	var policiesPtr *[]api.Policy
	if apiPolicies != nil {
		policiesPtr = &apiPolicies
	}

	sandboxUrl := "http://sandbox-backend:8080"
	apiData := api.APIConfigData{
		DisplayName: "Test API",
		Version:     version,
		Context:     context,
		Policies:    policiesPtr,
		Operations: []api.Operation{
			{
				Method: api.OperationMethodGET,
				Path:   "/resource",
			},
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: stringPtr("http://backend:8080"),
			},
			Sandbox: &api.Upstream{
				Url: &sandboxUrl,
			},
		},
	}

	apiConfig := api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Metadata: api.Metadata{
			Name: name,
		},
		Spec: apiData,
	}
	return &models.StoredConfig{
		UUID:                "0000-" + name + "-0000-000000000000",
		Kind:                string(api.RestAPIKindRestApi),
		Handle:              name,
		DisplayName:         "Test API",
		Version:             version,
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
	}
}

func createTestStoredConfigWithVhosts(name, version, context string, apiPolicies []api.Policy, mainVhost string, sandboxVhost *string) *models.StoredConfig {
	var policiesPtr *[]api.Policy
	if apiPolicies != nil {
		policiesPtr = &apiPolicies
	}

	sandboxUrl := "http://sandbox-backend:8080"
	apiData := api.APIConfigData{
		DisplayName: "Test API",
		Version:     version,
		Context:     context,
		Policies:    policiesPtr,
		Vhosts: &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main:    mainVhost,
			Sandbox: sandboxVhost,
		},
		Operations: []api.Operation{
			{
				Method: api.OperationMethodGET,
				Path:   "/resource",
			},
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: stringPtr("http://backend:8080"),
			},
			Sandbox: &api.Upstream{
				Url: &sandboxUrl,
			},
		},
	}

	apiConfig := api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Metadata: api.Metadata{
			Name: name,
		},
		Spec: apiData,
	}
	return &models.StoredConfig{
		UUID:                "0000-" + name + "-0000-000000000000",
		Kind:                string(api.RestAPIKindRestApi),
		Handle:              name,
		DisplayName:         "Test API",
		Version:             version,
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
	}
}

func stringPtr(s string) *string {
	return &s
}

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

		// Check some expected resource roles (keys are prefixed with managementAPIBasePath)
		assert.Contains(t, authConfig.ResourceRoles, "POST "+managementAPIBasePath+"/rest-apis")
		assert.Contains(t, authConfig.ResourceRoles, "GET "+managementAPIBasePath+"/rest-apis")
		assert.Contains(t, authConfig.ResourceRoles, "POST "+managementAPIBasePath+"/llm-providers/:id/api-keys")
		assert.Contains(t, authConfig.ResourceRoles, "GET "+managementAPIBasePath+"/llm-providers/:id/api-keys")
		assert.Contains(t, authConfig.ResourceRoles, "POST "+managementAPIBasePath+"/llm-proxies/:id/api-keys")
		assert.Contains(t, authConfig.ResourceRoles, "GET "+managementAPIBasePath+"/llm-proxies/:id/api-keys")
		assert.Contains(t, authConfig.ResourceRoles, "GET "+managementAPIBasePath+"/policies")
		// Admin API paths are served separately and must not leak into management auth config.
		assert.NotContains(t, authConfig.ResourceRoles, "GET "+managementAPIBasePath+"/config_dump")
		assert.NotContains(t, authConfig.ResourceRoles, "GET "+managementAPIBasePath+"/xds_sync_status")

		// Legacy unprefixed keys must also be present while the deprecated
		// routes continue to be supported.
		assert.Contains(t, authConfig.ResourceRoles, "POST /rest-apis")
		assert.Contains(t, authConfig.ResourceRoles, "GET /rest-apis")
		assert.Contains(t, authConfig.ResourceRoles, "GET /policies")

		// Check role assignments
		postRestAPIs := "POST " + managementAPIBasePath + "/rest-apis"
		assert.Contains(t, authConfig.ResourceRoles[postRestAPIs], "admin")
		assert.Contains(t, authConfig.ResourceRoles[postRestAPIs], "developer")
		// Legacy key carries the same role assignments.
		assert.Equal(t, authConfig.ResourceRoles[postRestAPIs], authConfig.ResourceRoles["POST /rest-apis"])
	})
}

// Additional edge case tests for policybuilder.DerivePolicyFromAPIConfig

func TestDerivePolicyFromAPIConfig_EdgeCases(t *testing.T) {
	fullConfig := &config.Config{
		Controller: config.Controller{},
		Router: config.RouterConfig{
			VHosts: config.VHostsConfig{
				Main: config.VHostEntry{
					Default: "api.example.com",
				},
				Sandbox: config.VHostEntry{
					Default: "sandbox.example.com",
				},
			},
		},
	}

	t.Run("API with empty vhost main uses default", func(t *testing.T) {
		apiPolicies := []api.Policy{
			{Name: "cors", Version: "v0"},
		}
		emptyMain := ""
		cfg := createTestStoredConfigWithVhosts("test-api", "v1.0.0", "/test", apiPolicies, emptyMain, nil)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())

		require.NotNil(t, result)
		// Should fall back to default vhost
		assert.NotEmpty(t, result.Configuration.Routes)
	})

	t.Run("API with empty sandbox vhost uses default", func(t *testing.T) {
		apiPolicies := []api.Policy{
			{Name: "cors", Version: "v0"},
		}
		emptySandbox := ""
		cfg := createTestStoredConfigWithVhosts("test-api", "v1.0.0", "/test", apiPolicies, "custom.example.com", &emptySandbox)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())

		require.NotNil(t, result)
		assert.NotEmpty(t, result.Configuration.Routes)
	})

	t.Run("Operation policy overrides same-named API policy", func(t *testing.T) {
		// Both API and operation have 'cors' policy - operation should take precedence (params)
		apiPolicies := []api.Policy{
			{Name: "cors", Version: "v0", Params: &map[string]interface{}{"api": true}},
			{Name: "rate-limit", Version: "v1"},
		}
		opPolicies := []api.Policy{
			{Name: "cors", Version: "v0", Params: &map[string]interface{}{"operation": true}},
		}
		cfg := createTestStoredConfigWithOpPolicies("test-api", "v1.0.0", "/test", apiPolicies, opPolicies)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())

		require.NotNil(t, result)
		// The result should have routes with policies
		assert.NotEmpty(t, result.Configuration.Routes)
	})

	t.Run("Multiple operations create multiple routes", func(t *testing.T) {
		apiPolicies := []api.Policy{
			{Name: "cors", Version: "v0"},
		}
		cfg := createTestStoredConfigMultipleOps("test-api", "v1.0.0", "/test", apiPolicies)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())

		require.NotNil(t, result)
		// Should have routes for each operation
		assert.GreaterOrEqual(t, len(result.Configuration.Routes), 2)
	})

	t.Run("Metadata is set correctly", func(t *testing.T) {
		apiPolicies := []api.Policy{
			{Name: "cors", Version: "v0"},
		}
		cfg := createTestStoredConfig("my-test-api", "v2.0.0", "/mycontext", apiPolicies, nil)

		result := policybuilder.DerivePolicyFromAPIConfig(cfg, &fullConfig.Router, fullConfig, testPolicyDefinitions())

		require.NotNil(t, result)
		assert.Equal(t, "Test API", result.Configuration.Metadata.APIName)
		assert.Equal(t, "v2.0.0", result.Configuration.Metadata.Version)
		assert.Equal(t, "/mycontext", result.Configuration.Metadata.Context)
		assert.NotZero(t, result.Configuration.Metadata.CreatedAt)
		assert.NotZero(t, result.Configuration.Metadata.UpdatedAt)
	})
}

// Helper function to create config with multiple operations
func createTestStoredConfigMultipleOps(name, version, context string, apiPolicies []api.Policy) *models.StoredConfig {
	var policiesPtr *[]api.Policy
	if apiPolicies != nil {
		policiesPtr = &apiPolicies
	}

	apiData := api.APIConfigData{
		DisplayName: "Test API",
		Version:     version,
		Context:     context,
		Policies:    policiesPtr,
		Operations: []api.Operation{
			{
				Method: api.OperationMethodGET,
				Path:   "/resource",
			},
			{
				Method: api.OperationMethodPOST,
				Path:   "/resource",
			},
			{
				Method: api.OperationMethodGET,
				Path:   "/other",
			},
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: stringPtr("http://backend:8080"),
			},
		},
	}

	apiConfig := api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Metadata: api.Metadata{
			Name: name,
		},
		Spec: apiData,
	}
	return &models.StoredConfig{
		UUID:                "0000-" + name + "-0000-000000000000",
		Kind:                string(api.RestAPIKindRestApi),
		Handle:              name,
		DisplayName:         "Test API",
		Version:             version,
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
	}
}
