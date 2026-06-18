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

package utils

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestNewLLMProviderTransformer_Basic(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
		HTTPSEnabled: false,
	}

	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())
	assert.NotNil(t, transformer)
	assert.Equal(t, store, transformer.store)
	assert.Equal(t, routerConfig, transformer.routerConfig)
}

func TestLLMProviderTransformer_Transform_InvalidInput(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	t.Run("Invalid input type returns error", func(t *testing.T) {
		output := &api.RestAPI{}
		_, err := transformer.Transform("invalid-type", output)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid input type")
	})

	t.Run("Nil input returns error", func(t *testing.T) {
		output := &api.RestAPI{}
		_, err := transformer.Transform(nil, output)
		assert.Error(t, err)
	})
}

func TestLLMProviderTransformer_TransformProvider_ReadsTemplateFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-db-template-id-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: "openai"},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "openai",
			},
		},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	provider := &api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "db-backed-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "db-backed-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}

	result, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)
	assert.Equal(t, "db-backed-provider", result.Spec.DisplayName)
}

func TestLLMProviderTransformer_TransformProxy_ReadsProviderAndTemplateFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-db-template-id-0000-000000000001",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: "openai"},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "openai",
			},
		},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	now := time.Now()
	providerSourceConfig := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "db-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "db-provider",
			Version:     "v1.0",
			Context:     stringPtr("/db-provider"),
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}
	providerRuntimeConfig := api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.RestAPIKindRestApi,
		Metadata:   api.Metadata{Name: "db-provider"},
		Spec: api.APIConfigData{
			DisplayName: "db-provider",
			Version:     "v1.0",
			Context:     "/db-provider",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{Url: stringPtr("https://api.openai.com")},
			},
		},
	}
	provider := &models.StoredConfig{
		UUID:                "0000-db-provider-id-0000-000000000000",
		Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:              "db-provider",
		DisplayName:         "db-provider",
		Version:             "v1.0",
		Configuration:       providerRuntimeConfig,
		SourceConfiguration: providerSourceConfig,
		DesiredState:        models.StateDeployed,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	require.NoError(t, db.SaveConfig(provider))

	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "db-proxy"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "db-proxy",
			Version:     "v1.0",
			Provider: api.LLMProxyProvider{
				Id: "db-provider",
			},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)
	require.NotNil(t, result.Spec.Upstream.Main.Url)
	assert.Equal(t, "http://127.0.0.1:8080/db-provider", *result.Spec.Upstream.Main.Url)
}

func TestGetUpstreamAuthApikeyPolicyParams_Extended(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		params, err := GetUpstreamAuthApikeyPolicyParams("Authorization", "Bearer token123")
		assert.NoError(t, err)
		assert.NotNil(t, params)
		assert.Contains(t, params, "request")
	})

	t.Run("Empty header name", func(t *testing.T) {
		params, err := GetUpstreamAuthApikeyPolicyParams("", "value")
		assert.NoError(t, err)
		assert.NotNil(t, params)
	})
}

func TestGetHostAdditionPolicyParams(t *testing.T) {
	t.Run("Valid host value", func(t *testing.T) {
		params, err := GetHostAdditionPolicyParams("api.example.com")
		assert.NoError(t, err)
		assert.NotNil(t, params)
		assert.Contains(t, params, "host")
		assert.Equal(t, "api.example.com", params["host"])
	})

	t.Run("Empty host value", func(t *testing.T) {
		params, err := GetHostAdditionPolicyParams("")
		assert.NoError(t, err)
		assert.NotNil(t, params)
	})
}

func TestBuildTemplateParams(t *testing.T) {
	t.Run("Nil template returns error", func(t *testing.T) {
		params, err := buildTemplateParams(nil, "/*")
		assert.Error(t, err)
		assert.Nil(t, params)
	})

	t.Run("Empty template returns empty params", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{},
			},
		}
		params, err := buildTemplateParams(template, "/*")
		assert.NoError(t, err)
		assert.NotNil(t, params)
		assert.Empty(t, params)
	})

	t.Run("Template with requestModel", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{
					RequestModel: &api.ExtractionIdentifier{
						Location:   "body",
						Identifier: "model",
					},
				},
			},
		}
		params, err := buildTemplateParams(template, "/*")
		assert.NoError(t, err)
		assert.Contains(t, params, "requestModel")

		rm := params["requestModel"].(map[string]interface{})
		assert.Equal(t, api.ExtractionIdentifierLocation("body"), rm["location"])
		assert.Equal(t, "model", rm["identifier"])
	})

	t.Run("Template with responseModel", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{
					ResponseModel: &api.ExtractionIdentifier{
						Location:   "body",
						Identifier: "model",
					},
				},
			},
		}
		params, err := buildTemplateParams(template, "/*")
		assert.NoError(t, err)
		assert.Contains(t, params, "responseModel")
	})

	t.Run("Template with promptTokens", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{
					PromptTokens: &api.ExtractionIdentifier{
						Location:   "body",
						Identifier: "usage.prompt_tokens",
					},
				},
			},
		}
		params, err := buildTemplateParams(template, "/*")
		assert.NoError(t, err)
		assert.Contains(t, params, "promptTokens")
	})

	t.Run("Template with completionTokens", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{
					CompletionTokens: &api.ExtractionIdentifier{
						Location:   "body",
						Identifier: "usage.completion_tokens",
					},
				},
			},
		}
		params, err := buildTemplateParams(template, "/*")
		assert.NoError(t, err)
		assert.Contains(t, params, "completionTokens")
	})

	t.Run("Template with totalTokens", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{
					TotalTokens: &api.ExtractionIdentifier{
						Location:   "body",
						Identifier: "usage.total_tokens",
					},
				},
			},
		}
		params, err := buildTemplateParams(template, "/*")
		assert.NoError(t, err)
		assert.Contains(t, params, "totalTokens")
	})

	t.Run("Template with remainingTokens", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{
					RemainingTokens: &api.ExtractionIdentifier{
						Location:   "header",
						Identifier: "x-remaining-tokens",
					},
				},
			},
		}
		params, err := buildTemplateParams(template, "/*")
		assert.NoError(t, err)
		assert.Contains(t, params, "remainingTokens")
	})

	t.Run("Template with all token identifiers", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{
					RequestModel:     &api.ExtractionIdentifier{Location: "body", Identifier: "model"},
					ResponseModel:    &api.ExtractionIdentifier{Location: "body", Identifier: "model"},
					PromptTokens:     &api.ExtractionIdentifier{Location: "body", Identifier: "usage.prompt_tokens"},
					CompletionTokens: &api.ExtractionIdentifier{Location: "body", Identifier: "usage.completion_tokens"},
					TotalTokens:      &api.ExtractionIdentifier{Location: "body", Identifier: "usage.total_tokens"},
					RemainingTokens:  &api.ExtractionIdentifier{Location: "header", Identifier: "x-remaining"},
				},
			},
		}
		params, err := buildTemplateParams(template, "/*")
		assert.NoError(t, err)
		assert.Len(t, params, 6)
	})

	t.Run("Resource mapping override is selected", func(t *testing.T) {
		defaultModel := "$.model"
		responsesModel := "$.response.model"
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{
					RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: defaultModel},
					ResourceMappings: &api.LLMProviderTemplateResourceMappings{
						Resources: &[]api.LLMProviderTemplateResourceMapping{
							{
								Resource:     "/responses",
								RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: responsesModel},
							},
						},
					},
				},
			},
		}

		responsesParams, err := buildTemplateParams(template, "/responses")
		assert.NoError(t, err)
		assert.Equal(t, responsesModel, responsesParams["requestModel"].(map[string]interface{})["identifier"])

		defaultParams, err := buildTemplateParams(template, "/chat/completions")
		assert.NoError(t, err)
		assert.Equal(t, defaultModel, defaultParams["requestModel"].(map[string]interface{})["identifier"])
	})
}

func TestMergeParams(t *testing.T) {
	t.Run("Merge empty maps", func(t *testing.T) {
		base := map[string]interface{}{}
		extra := map[string]interface{}{}
		result := mergeParams(base, extra)
		assert.NotNil(t, result)
		assert.Empty(t, *result)
	})

	t.Run("Merge with base only", func(t *testing.T) {
		base := map[string]interface{}{"0000-key1-0000-000000000000": "value1"}
		extra := map[string]interface{}{}
		result := mergeParams(base, extra)
		assert.Equal(t, "value1", (*result)["0000-key1-0000-000000000000"])
	})

	t.Run("Merge with extra only", func(t *testing.T) {
		base := map[string]interface{}{}
		extra := map[string]interface{}{"0000-key2-0000-000000000000": "value2"}
		result := mergeParams(base, extra)
		assert.Equal(t, "value2", (*result)["0000-key2-0000-000000000000"])
	})

	t.Run("Merge with overlapping keys", func(t *testing.T) {
		base := map[string]interface{}{"key": "base-value"}
		extra := map[string]interface{}{"key": "extra-value"}
		result := mergeParams(base, extra)
		// Extra should override base
		assert.Equal(t, "extra-value", (*result)["key"])
	})

	t.Run("Merge with different keys", func(t *testing.T) {
		base := map[string]interface{}{"0000-key1-0000-000000000000": "value1"}
		extra := map[string]interface{}{"0000-key2-0000-000000000000": "value2"}
		result := mergeParams(base, extra)
		assert.Equal(t, "value1", (*result)["0000-key1-0000-000000000000"])
		assert.Equal(t, "value2", (*result)["0000-key2-0000-000000000000"])
	})
}

func TestExpandPolicyTargetPaths(t *testing.T) {
	t.Run("Wildcard operation expands to mapped resources", func(t *testing.T) {
		templateSpec := &api.LLMProviderTemplateData{
			ResourceMappings: &api.LLMProviderTemplateResourceMappings{
				Resources: &[]api.LLMProviderTemplateResourceMapping{
					{Resource: "/responses"},
					{Resource: "/chat/*"},
				},
			},
		}

		targets := expandPolicyTargetPaths("/*", templateSpec)
		assert.ElementsMatch(t, []string{"/responses", "/chat/*", "/*"}, targets)
	})

	t.Run("Wildcard operation falls back when mappings are missing", func(t *testing.T) {
		targets := expandPolicyTargetPaths("/*", &api.LLMProviderTemplateData{})
		assert.Equal(t, []string{"/*"}, targets)
	})

	t.Run("Explicit operation path is not expanded", func(t *testing.T) {
		targets := expandPolicyTargetPaths("/responses", &api.LLMProviderTemplateData{})
		assert.Equal(t, []string{"/responses"}, targets)
	})
}

func TestIsDeniedByException(t *testing.T) {
	deniedPathMethods := map[pathMethodKey]bool{
		{path: "/admin", method: "GET"}:      true,
		{path: "/admin", method: "POST"}:     true,
		{path: "/internal/*", method: "GET"}: true,
	}

	t.Run("Exact match is denied", func(t *testing.T) {
		result := isDeniedByException("/admin", "GET", deniedPathMethods)
		assert.True(t, result)
	})

	t.Run("Different method not denied", func(t *testing.T) {
		result := isDeniedByException("/admin", "DELETE", deniedPathMethods)
		assert.False(t, result)
	})

	t.Run("Different path not denied", func(t *testing.T) {
		result := isDeniedByException("/public", "GET", deniedPathMethods)
		assert.False(t, result)
	})

	t.Run("Wildcard path covers sub-paths", func(t *testing.T) {
		result := isDeniedByException("/internal/data", "GET", deniedPathMethods)
		assert.True(t, result)
	})

	t.Run("Wildcard path different method not denied", func(t *testing.T) {
		result := isDeniedByException("/internal/data", "POST", deniedPathMethods)
		assert.False(t, result)
	})
}

func TestHasDenyPolicy(t *testing.T) {
	t.Run("Nil policies returns false", func(t *testing.T) {
		op := &api.Operation{Path: "/test", Method: "GET", Policies: nil}
		result := hasDenyPolicy(op, testRespondVersion)
		assert.False(t, result)
	})

	t.Run("Empty policies returns false", func(t *testing.T) {
		policies := []api.Policy{}
		op := &api.Operation{Path: "/test", Method: "GET", Policies: &policies}
		result := hasDenyPolicy(op, testRespondVersion)
		assert.False(t, result)
	})

	t.Run("Has deny policy returns true", func(t *testing.T) {
		policies := []api.Policy{
			{Name: constants.ACCESS_CONTROL_DENY_POLICY_NAME, Version: testRespondVersion},
		}
		op := &api.Operation{Path: "/test", Method: "GET", Policies: &policies}
		result := hasDenyPolicy(op, testRespondVersion)
		assert.True(t, result)
	})

	t.Run("Different policy returns false", func(t *testing.T) {
		policies := []api.Policy{
			{Name: "other-policy", Version: "v1.0.0"},
		}
		op := &api.Operation{Path: "/test", Method: "GET", Policies: &policies}
		result := hasDenyPolicy(op, testRespondVersion)
		assert.False(t, result)
	})

	t.Run("Mixed policies with deny returns true", func(t *testing.T) {
		policies := []api.Policy{
			{Name: "other-policy", Version: "v1.0.0"},
			{Name: constants.ACCESS_CONTROL_DENY_POLICY_NAME, Version: testRespondVersion},
		}
		op := &api.Operation{Path: "/test", Method: "GET", Policies: &policies}
		result := hasDenyPolicy(op, testRespondVersion)
		assert.True(t, result)
	})
}

func TestDenyAppliesToTarget(t *testing.T) {
	t.Run("Exact deny policy matches target path", func(t *testing.T) {
		denyPolicies := []api.Policy{
			{Name: constants.ACCESS_CONTROL_DENY_POLICY_NAME, Version: testRespondVersion},
		}
		registry := map[pathMethodKey]*api.Operation{
			{path: "/chat/completions", method: "GET"}: {
				Path:     "/chat/completions",
				Method:   "GET",
				Policies: &denyPolicies,
			},
		}

		result := denyAppliesToTarget("/chat/completions", "GET", testRespondVersion, registry)
		assert.True(t, result)
	})

	t.Run("Wildcard deny policy matches concrete target path", func(t *testing.T) {
		denyPolicies := []api.Policy{
			{Name: constants.ACCESS_CONTROL_DENY_POLICY_NAME, Version: testRespondVersion},
		}
		registry := map[pathMethodKey]*api.Operation{
			{path: "/chat/*", method: "GET"}: {
				Path:     "/chat/*",
				Method:   "GET",
				Policies: &denyPolicies,
			},
		}

		result := denyAppliesToTarget("/chat/completions", "GET", testRespondVersion, registry)
		assert.True(t, result)
	})

	t.Run("Different method does not match wildcard deny policy", func(t *testing.T) {
		denyPolicies := []api.Policy{
			{Name: constants.ACCESS_CONTROL_DENY_POLICY_NAME, Version: testRespondVersion},
		}
		registry := map[pathMethodKey]*api.Operation{
			{path: "/chat/*", method: "GET"}: {
				Path:     "/chat/*",
				Method:   "GET",
				Policies: &denyPolicies,
			},
		}

		result := denyAppliesToTarget("/chat/completions", "POST", testRespondVersion, registry)
		assert.False(t, result)
	})
}

func TestIsAllowedByAccessControl(t *testing.T) {
	normalizedExceptions := map[pathMethodKey]bool{
		{path: "/api/v1/users", method: "GET"}:  true,
		{path: "/api/v1/users", method: "POST"}: true,
		{path: "/api/v1/data/*", method: "GET"}: true,
	}

	t.Run("Exact match is allowed", func(t *testing.T) {
		result := isAllowedByAccessControl("/api/v1/users", "GET", normalizedExceptions)
		assert.True(t, result)
	})

	t.Run("Different method not allowed", func(t *testing.T) {
		result := isAllowedByAccessControl("/api/v1/users", "DELETE", normalizedExceptions)
		assert.False(t, result)
	})

	t.Run("Different path not allowed", func(t *testing.T) {
		result := isAllowedByAccessControl("/api/v2/users", "GET", normalizedExceptions)
		assert.False(t, result)
	})

	t.Run("Wildcard path covers sub-paths", func(t *testing.T) {
		result := isAllowedByAccessControl("/api/v1/data/items", "GET", normalizedExceptions)
		assert.True(t, result)
	})

	t.Run("Wildcard path different method not allowed", func(t *testing.T) {
		result := isAllowedByAccessControl("/api/v1/data/items", "POST", normalizedExceptions)
		assert.False(t, result)
	})
}

func TestIsMoreSpecificPath(t *testing.T) {
	// isMoreSpecificPath is only ever invoked (via moreSpecificPolicyAttachmentCovers) on two
	// paths that both already match the same target, so "concrete beats wildcard, then longer
	// beats shorter" is a sound specificity ordering for the path forms in use (exact paths and
	// trailing "/*" wildcards). These cases pin that behaviour.
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{"concrete beats wildcard", "/chat/completions", "/chat/*", true},
		{"wildcard loses to concrete", "/chat/*", "/chat/completions", false},
		{"concrete beats root wildcard", "/chat/completions", "/*", true},
		{"root wildcard loses to concrete", "/*", "/chat/completions", false},
		{"longer wildcard prefix beats shorter", "/chat/*", "/*", true},
		{"shorter wildcard prefix loses", "/*", "/chat/*", false},
		{"deeper nested wildcard beats shallower", "/chat/completions/*", "/chat/*", true},
		{"shallower nested wildcard loses", "/chat/*", "/chat/completions/*", false},
		{"three-level nesting beats one-level", "/a/b/c/*", "/a/*", true},
		{"identical concrete is not strictly more specific", "/chat/completions", "/chat/completions", false},
		{"identical wildcard is not strictly more specific", "/chat/*", "/chat/*", false},
		{"identical root wildcard", "/*", "/*", false},
		{"longer concrete wins on length", "/chat/completions", "/chat", true},
		{"shorter concrete loses on length", "/chat", "/chat/completions", false},
		{"equal-length concrete paths tie", "/aaa/bbb", "/bbb/aaa", false},
		{"equal-length wildcards tie", "/ab/*", "/cd/*", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isMoreSpecificPath(tc.a, tc.b))
		})
	}

	// Properties: a path is never strictly more specific than itself (irreflexive), and two
	// distinct paths cannot each be strictly more specific than the other (antisymmetric).
	paths := []string{"/chat/completions", "/chat/*", "/*", "/chat/completions/*", "/a"}
	for _, p := range paths {
		assert.Falsef(t, isMoreSpecificPath(p, p), "%q should not be more specific than itself", p)
	}
	for _, x := range paths {
		for _, y := range paths {
			if x != y && isMoreSpecificPath(x, y) {
				assert.Falsef(t, isMoreSpecificPath(y, x),
					"%q>%q and %q>%q cannot both hold", x, y, y, x)
			}
		}
	}
}

func TestMoreSpecificPolicyAttachmentCovers(t *testing.T) {
	// The function is block-scoped by signature: it only inspects current.policy.Paths, so
	// these cases construct the block (and the current entry within it) directly. It returns
	// true iff some OTHER entry in the same block both covers (targetPath, method) and is
	// strictly more specific than the current entry.
	mk := func(path string, methods ...string) api.OperationPolicyPath {
		return api.OperationPolicyPath{Path: path, Methods: methods}
	}
	ccAll := mk("/chat/completions", "*")
	ccGet := mk("/chat/completions", "GET")
	ccPost := mk("/chat/completions", "POST")
	ccGetPost := mk("/chat/completions", "GET", "POST")
	chatWild := mk("/chat/*", "*")
	chatWildPost := mk("/chat/*", "POST")
	root := mk("/*", "*")

	tests := []struct {
		name       string
		block      []api.OperationPolicyPath
		current    api.OperationPolicyPath
		targetPath string
		method     string
		want       bool
	}{
		// --- nothing more specific present ---
		{"only the current entry in the block", []api.OperationPolicyPath{root}, root, "/chat/completions", "POST", false},
		{"current is the most specific entry", []api.OperationPolicyPath{ccAll, chatWild, root}, ccAll, "/chat/completions", "POST", false},
		{"only a less specific sibling", []api.OperationPolicyPath{ccAll, root}, ccAll, "/chat/completions", "POST", false},

		// --- more specific by PATH ---
		{"more specific concrete sibling covers target", []api.OperationPolicyPath{ccAll, root}, root, "/chat/completions", "POST", true},
		{"more specific nested-wildcard sibling covers target", []api.OperationPolicyPath{chatWild, root}, root, "/chat/foo", "POST", true},
		{"more specific sibling does not cover target", []api.OperationPolicyPath{ccAll, root}, root, "/models", "POST", false},

		// --- method gating of the more specific sibling ---
		{"more specific sibling does not apply to method", []api.OperationPolicyPath{ccGet, root}, root, "/chat/completions", "POST", false},
		{"more specific sibling applies to method", []api.OperationPolicyPath{ccGet, root}, root, "/chat/completions", "GET", true},

		// --- method specificity on the SAME path ---
		{"concrete method beats wildcard method", []api.OperationPolicyPath{ccAll, ccGet}, ccAll, "/chat/completions", "GET", true},
		{"wildcard method not suppressed for uncovered method", []api.OperationPolicyPath{ccAll, ccGet}, ccAll, "/chat/completions", "POST", false},
		{"concrete-method current not suppressed by wildcard-method sibling", []api.OperationPolicyPath{ccAll, ccGet}, ccGet, "/chat/completions", "GET", false},
		{"narrower method set beats broader", []api.OperationPolicyPath{ccGetPost, ccPost}, ccGetPost, "/chat/completions", "POST", true},
		{"broader method set does not suppress narrower", []api.OperationPolicyPath{ccGetPost, ccPost}, ccPost, "/chat/completions", "POST", false},

		// --- ties ---
		{"equal-specificity duplicate is a tie", []api.OperationPolicyPath{ccPost, mk("/chat/completions", "POST")}, ccPost, "/chat/completions", "POST", false},

		// --- path dominates method ---
		{"specific path beats method-specific wildcard path", []api.OperationPolicyPath{chatWildPost, ccAll}, chatWildPost, "/chat/completions", "POST", true},
		{"method-specific wildcard path does not beat specific path", []api.OperationPolicyPath{chatWildPost, ccAll}, ccAll, "/chat/completions", "POST", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			att := llmPolicyAttachment{
				policy:    api.OperationPolicy{Name: "advanced-ratelimit", Version: "v1", Paths: tc.block},
				pathEntry: tc.current,
			}
			assert.Equal(t, tc.want, moreSpecificPolicyAttachmentCovers(tc.targetPath, tc.method, att))
		})
	}
}

func TestMethodSet(t *testing.T) {
	mm := func(methods ...string) []string { return methods }

	t.Run("single concrete method", func(t *testing.T) {
		assert.Equal(t, map[string]bool{"GET": true}, methodSet(mm("GET")))
	})
	t.Run("multiple concrete methods", func(t *testing.T) {
		assert.Equal(t, map[string]bool{"GET": true, "POST": true}, methodSet(mm("GET", "POST")))
	})
	t.Run("duplicate methods are de-duplicated", func(t *testing.T) {
		assert.Equal(t, map[string]bool{"GET": true}, methodSet(mm("GET", "GET")))
	})
	t.Run("empty methods yield empty set", func(t *testing.T) {
		assert.Empty(t, methodSet(mm()))
	})
	t.Run("lone wildcard expands to all supported methods", func(t *testing.T) {
		got := methodSet(mm("*"))
		require.Len(t, got, len(constants.WILDCARD_HTTP_METHODS))
		for _, m := range constants.WILDCARD_HTTP_METHODS {
			assert.Truef(t, got[m], "expected %s in expanded wildcard set", m)
		}
		assert.False(t, got["*"], "the literal '*' must not be a member of the expanded set")
	})
	t.Run("wildcard only expands when it is the sole element", func(t *testing.T) {
		// expandLLMPolicyMethods only treats a single-element ["*"] as the wildcard; mixed with
		// other methods the '*' stays literal.
		assert.Equal(t, map[string]bool{"GET": true, "*": true}, methodSet(mm("GET", "*")))
	})
}

func TestIsStrictMethodSubset(t *testing.T) {
	mm := func(methods ...string) []string { return methods }
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{"strict subset", mm("POST"), mm("GET", "POST"), true},
		{"multi-element strict subset", mm("POST", "PUT"), mm("GET", "POST", "PUT"), true},
		{"superset is not a subset", mm("GET", "POST"), mm("POST"), false},
		{"equal single set is not strict", mm("POST"), mm("POST"), false},
		{"equal multi set is not strict", mm("GET", "POST"), mm("GET", "POST"), false},
		{"concrete is strict subset of wildcard", mm("POST"), mm("*"), true},
		{"two concrete are strict subset of wildcard", mm("GET", "POST"), mm("*"), true},
		{"wildcard is not subset of concrete", mm("*"), mm("POST"), false},
		{"wildcard equals wildcard is not strict", mm("*"), mm("*"), false},
		{"disjoint equal length", mm("GET"), mm("POST"), false},
		{"smaller but not a member subset", mm("GET"), mm("POST", "PUT"), false},
		{"empty is not a strict subset", mm(), mm("POST"), false},
		{"both empty", mm(), mm(), false},
		{"duplicates collapse before comparison", mm("POST", "POST"), mm("POST"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isStrictMethodSubset(tc.a, tc.b))
		})
	}
}

func TestIsMoreSpecificAttachment(t *testing.T) {
	mk := func(path string, methods ...string) api.OperationPolicyPath {
		return api.OperationPolicyPath{Path: path, Methods: methods}
	}
	tests := []struct {
		name string
		a, b api.OperationPolicyPath
		want bool
	}{
		// Path specificity dominates, regardless of methods.
		{"more specific path wins over broader-path narrower-method", mk("/chat/completions", "*"), mk("/chat/*", "POST"), true},
		{"less specific path loses even with narrower method", mk("/chat/*", "GET"), mk("/chat/completions", "*"), false},
		{"concrete path beats root wildcard", mk("/chat/completions", "POST"), mk("/*", "*"), true},
		{"nested wildcard beats root wildcard", mk("/chat/*", "*"), mk("/*", "*"), true},
		// Same path -> method specificity decides.
		{"same path, narrower method set wins", mk("/chat/completions", "POST"), mk("/chat/completions", "GET", "POST"), true},
		{"same path, broader method set loses", mk("/chat/completions", "GET", "POST"), mk("/chat/completions", "POST"), false},
		{"same path, concrete method beats wildcard method", mk("/chat/completions", "GET"), mk("/chat/completions", "*"), true},
		{"same path, wildcard method loses to concrete", mk("/chat/completions", "*"), mk("/chat/completions", "GET"), false},
		{"same path, equal methods are not more specific", mk("/chat/completions", "POST"), mk("/chat/completions", "POST"), false},
		// Equal path-specificity (different, non-overlapping paths) falls through to method
		// comparison; unreachable in practice but pins the documented ordering.
		{"equal path specificity falls through to method subset", mk("/ab/*", "POST"), mk("/cd/*", "GET", "POST"), true},
		{"equal path specificity, method not a subset", mk("/ab/*", "GET"), mk("/cd/*", "POST"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isMoreSpecificAttachment(tc.a, tc.b))
		})
	}
}

func TestPathsMatch(t *testing.T) {
	tests := []struct {
		name       string
		opPath     string
		policyPath string
		expected   bool
	}{
		{
			name:       "Root wildcard matches everything",
			opPath:     "/any/path",
			policyPath: "/*",
			expected:   true,
		},
		{
			name:       "Exact match",
			opPath:     "/users",
			policyPath: "/users",
			expected:   true,
		},
		{
			name:       "Same wildcard match",
			opPath:     "/users/*",
			policyPath: "/users/*",
			expected:   true,
		},
		{
			name:       "Wildcard policy covers specific path",
			opPath:     "/users/123",
			policyPath: "/users/*",
			expected:   true,
		},
		{
			name:       "Wildcard policy covers nested path",
			opPath:     "/users/123/orders",
			policyPath: "/users/*",
			expected:   true,
		},
		{
			name:       "Different paths don't match",
			opPath:     "/orders",
			policyPath: "/users",
			expected:   false,
		},
		{
			name:       "Specific policy doesn't match wildcard operation",
			opPath:     "/users/*",
			policyPath: "/users/123",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pathsMatch(tt.opPath, tt.policyPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortOperationsBySpecificity(t *testing.T) {
	t.Run("Empty list returns empty", func(t *testing.T) {
		ops := []api.Operation{}
		result := sortOperationsBySpecificity(ops)
		assert.Empty(t, result)
	})

	t.Run("Single operation unchanged", func(t *testing.T) {
		ops := []api.Operation{
			{Path: "/users", Method: "GET"},
		}
		result := sortOperationsBySpecificity(ops)
		assert.Len(t, result, 1)
		assert.Equal(t, "/users", result[0].Path)
	})

	t.Run("Non-wildcard before wildcard", func(t *testing.T) {
		ops := []api.Operation{
			{Path: "/*", Method: "GET"},
			{Path: "/users", Method: "GET"},
		}
		result := sortOperationsBySpecificity(ops)
		assert.Equal(t, "/users", result[0].Path)
		assert.Equal(t, "/*", result[1].Path)
	})

	t.Run("Longer paths before shorter", func(t *testing.T) {
		ops := []api.Operation{
			{Path: "/a", Method: "GET"},
			{Path: "/ab", Method: "GET"},
			{Path: "/abc", Method: "GET"},
		}
		result := sortOperationsBySpecificity(ops)
		assert.Equal(t, "/abc", result[0].Path)
		assert.Equal(t, "/ab", result[1].Path)
		assert.Equal(t, "/a", result[2].Path)
	})

	t.Run("Complex sorting", func(t *testing.T) {
		ops := []api.Operation{
			{Path: "/*", Method: "GET"},
			{Path: "/users/*", Method: "GET"},
			{Path: "/users/123", Method: "GET"},
			{Path: "/users", Method: "GET"},
		}
		result := sortOperationsBySpecificity(ops)

		// Non-wildcard paths should come first
		assert.False(t, containsWildcard(result[0].Path) && containsWildcard(result[1].Path))
	})
}

func containsWildcard(path string) bool {
	for _, c := range path {
		if c == '*' {
			return true
		}
	}
	return false
}

func TestShouldSwap(t *testing.T) {
	t.Run("Wildcard should come after non-wildcard", func(t *testing.T) {
		op1 := api.Operation{Path: "/*", Method: "GET"}
		op2 := api.Operation{Path: "/users", Method: "GET"}
		result := shouldSwap(op1, op2)
		assert.True(t, result)
	})

	t.Run("Non-wildcard should not swap with wildcard", func(t *testing.T) {
		op1 := api.Operation{Path: "/users", Method: "GET"}
		op2 := api.Operation{Path: "/*", Method: "GET"}
		result := shouldSwap(op1, op2)
		assert.False(t, result)
	})

	t.Run("Shorter path should come after longer", func(t *testing.T) {
		op1 := api.Operation{Path: "/a", Method: "GET"}
		op2 := api.Operation{Path: "/ab", Method: "GET"}
		result := shouldSwap(op1, op2)
		assert.True(t, result)
	})

	t.Run("Same length paths compared lexicographically", func(t *testing.T) {
		op1 := api.Operation{Path: "/bbb", Method: "GET"}
		op2 := api.Operation{Path: "/aaa", Method: "GET"}
		result := shouldSwap(op1, op2)
		assert.True(t, result)
	})

	t.Run("Same path different methods compared alphabetically", func(t *testing.T) {
		op1 := api.Operation{Path: "/users", Method: "POST"}
		op2 := api.Operation{Path: "/users", Method: "GET"}
		result := shouldSwap(op1, op2)
		assert.True(t, result)
	})
}

func TestTransformProvider_MissingTemplate(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "test-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "Test Provider",
			Version:     "1.0.0",
			Template:    "non-existent-template",
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.RestAPI{}
	_, err := transformer.Transform(provider, output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve template")
}

func TestTransformProvider_AllowAllMode(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	// Add a template to the store
	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
	db.SaveLLMProviderTemplate(template)
	err := store.AddTemplate(template)
	require.NoError(t, err)

	upstreamURL := "https://api.openai.com"
	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "OpenAI Provider",
			Version:     "1.0.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: &upstreamURL,
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.RestAPI{}
	result, err := transformer.Transform(provider, output)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, api.RestAPIKindRestApi, result.Kind)
}

func TestTransformProvider_DenyAllMode(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	// Add a template to the store
	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
	db.SaveLLMProviderTemplate(template)
	err := store.AddTemplate(template)
	require.NoError(t, err)

	upstreamURL := "https://api.openai.com"
	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "OpenAI Provider",
			Version:     "1.0.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: &upstreamURL,
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.DenyAll,
				Exceptions: &[]api.RouteException{
					{Path: "/v1/chat/completions", Methods: []api.RouteExceptionMethods{"POST"}},
				},
			},
		},
	}

	output := &api.RestAPI{}
	result, err := transformer.Transform(provider, output)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, api.RestAPIKindRestApi, result.Kind)
}

func TestTransformProvider_ExpandsWildcardPolicyPathWithTemplateMappings(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	defaultModel := "$.model"
	responsesModel := "$.response.model"
	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec: api.LLMProviderTemplateData{
				RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: defaultModel},
				ResourceMappings: &api.LLMProviderTemplateResourceMappings{
					Resources: &[]api.LLMProviderTemplateResourceMapping{
						{
							Resource:     "/responses",
							RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: responsesModel},
						},
					},
				},
			},
		},
	}
	db.SaveLLMProviderTemplate(template)
	err := store.AddTemplate(template)
	require.NoError(t, err)

	upstreamURL := "https://api.openai.com"
	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "OpenAI Provider",
			Version:     "1.0.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: &upstreamURL,
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			Policies: &[]api.LLMPolicy{
				{
					Name:    "request-transformer",
					Version: "v1.0.0",
					Paths: []api.LLMPolicyPath{
						{
							Path:    "/*",
							Methods: []api.LLMPolicyPathMethods{"POST"},
							Params:  map[string]interface{}{"userParam": "value"},
						},
					},
				},
			},
		},
	}

	output := &api.RestAPI{}
	result, err := transformer.Transform(provider, output)
	require.NoError(t, err)
	require.NotNil(t, result)

	var responsesOp *api.Operation
	var wildcardPostOp *api.Operation
	for i := range result.Spec.Operations {
		op := &result.Spec.Operations[i]
		if op.Method == "POST" && op.Path == "/responses" {
			responsesOp = op
		}
		if op.Method == "POST" && op.Path == "/*" {
			wildcardPostOp = op
		}
	}

	require.NotNil(t, responsesOp)
	require.NotNil(t, responsesOp.Policies)

	var responsesPolicy *api.Policy
	for i := range *responsesOp.Policies {
		pol := &(*responsesOp.Policies)[i]
		if pol.Name == "request-transformer" {
			responsesPolicy = pol
			break
		}
	}
	require.NotNil(t, responsesPolicy)
	require.NotNil(t, responsesPolicy.Params)

	reqModel, ok := (*responsesPolicy.Params)["requestModel"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, responsesModel, reqModel["identifier"])
	assert.Equal(t, "value", (*responsesPolicy.Params)["userParam"])

	require.NotNil(t, wildcardPostOp)
	require.NotNil(t, wildcardPostOp.Policies)

	var wildcardPolicy *api.Policy
	for i := range *wildcardPostOp.Policies {
		pol := &(*wildcardPostOp.Policies)[i]
		if pol.Name == "request-transformer" {
			wildcardPolicy = pol
			break
		}
	}
	require.NotNil(t, wildcardPolicy)
	require.NotNil(t, wildcardPolicy.Params)

	wildcardReqModel, ok := (*wildcardPolicy.Params)["requestModel"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, defaultModel, wildcardReqModel["identifier"])
	assert.Equal(t, "value", (*wildcardPolicy.Params)["userParam"])
}

func TestTransformProvider_PolicyOrderDoesNotAffectWildcardCoverage(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000001",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
	db.SaveLLMProviderTemplate(template)
	err := store.AddTemplate(template)
	require.NoError(t, err)

	upstreamURL := "https://api.openai.com"
	assertPolicies := func(t *testing.T, result *api.RestAPI) {
		chatOp := findOperation(result.Spec.Operations, "/chat/completions", "POST")
		require.NotNil(t, chatOp)
		require.NotNil(t, chatOp.Policies)
		require.Len(t, *chatOp.Policies, 2)
		assert.Equal(t, "set-headers-all", (*chatOp.Policies)[0].Name, "/chat/completions should apply the wildcard policy before the specific policy")
		assert.Equal(t, "set-headers", (*chatOp.Policies)[1].Name, "/chat/completions should keep its specific policy after the wildcard policy")

		wildcardOp := findOperation(result.Spec.Operations, "/*", "POST")
		require.NotNil(t, wildcardOp)
		require.NotNil(t, wildcardOp.Policies)
		assert.Len(t, *wildcardOp.Policies, 1)
		assert.Equal(t, "set-headers-all", (*wildcardOp.Policies)[0].Name)
	}

	newProvider := func(policies []api.LLMPolicy) *api.LLMProviderConfiguration {
		return &api.LLMProviderConfiguration{
			Metadata: api.Metadata{Name: "openai-provider"},
			Spec: api.LLMProviderConfigData{
				DisplayName: "OpenAI Provider",
				Version:     "1.0.0",
				Template:    "openai",
				Upstream: api.LLMProviderConfigData_Upstream{
					Url: &upstreamURL,
				},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
				Policies:      &policies,
			},
		}
	}

	wildcardPolicy := api.LLMPolicy{
		Name:    "set-headers-all",
		Version: "v1",
		Paths: []api.LLMPolicyPath{{
			Path:    "/*",
			Methods: []api.LLMPolicyPathMethods{"POST"},
			Params:  map[string]interface{}{"scope": "all"},
		}},
	}
	specificPolicy := api.LLMPolicy{
		Name:    "set-headers",
		Version: "v1",
		Paths: []api.LLMPolicyPath{{
			Path:    "/chat/completions",
			Methods: []api.LLMPolicyPathMethods{"POST"},
			Params:  map[string]interface{}{"scope": "specific"},
		}},
	}

	t.Run("wildcard then specific", func(t *testing.T) {
		provider := newProvider([]api.LLMPolicy{wildcardPolicy, specificPolicy})

		result, err := transformer.Transform(provider, &api.RestAPI{})
		require.NoError(t, err)
		require.NotNil(t, result)

		assertPolicies(t, result)
	})

	t.Run("specific then wildcard", func(t *testing.T) {
		provider := newProvider([]api.LLMPolicy{specificPolicy, wildcardPolicy})

		result, err := transformer.Transform(provider, &api.RestAPI{})
		require.NoError(t, err)
		require.NotNil(t, result)

		assertPolicies(t, result)
	})
}

func TestTransformProvider_MostSpecificPathWinsForSamePolicy(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000002",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))
	require.NoError(t, store.AddTemplate(template))

	rateLimitParams := func(quotaName string, limit int) map[string]interface{} {
		return map[string]interface{}{
			"quotas": []interface{}{
				map[string]interface{}{
					"name": quotaName,
					"limits": []interface{}{
						map[string]interface{}{"limit": limit, "duration": "1h"},
					},
				},
			},
		}
	}

	upstreamURL := "https://api.openai.com"
	// The same policy (advanced-ratelimit) is attached to a specific path (4/h) and to the
	// wildcard catch-all (1/h). The most specific path must win, so /chat/completions keeps
	// only its own quota and is not also limited by the wildcard quota.
	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "OpenAI Provider",
			Version:     "1.0.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: &upstreamURL,
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			Policies: &[]api.LLMPolicy{
				{
					Name:    "advanced-ratelimit",
					Version: "v1",
					Paths: []api.LLMPolicyPath{
						{
							Path:    "/chat/completions",
							Methods: []api.LLMPolicyPathMethods{"POST"},
							Params:  rateLimitParams("chat-quota", 4),
						},
						{
							Path:    "/*",
							Methods: []api.LLMPolicyPathMethods{"*"},
							Params:  rateLimitParams("wildcard-quota", 1),
						},
					},
				},
			},
		},
	}

	result, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)
	require.NotNil(t, result)

	firstQuotaName := func(params *map[string]interface{}) string {
		require.NotNil(t, params)
		quotas, ok := (*params)["quotas"].([]interface{})
		require.True(t, ok, "params should contain a quotas slice")
		require.NotEmpty(t, quotas)
		first, ok := quotas[0].(map[string]interface{})
		require.True(t, ok)
		name, ok := first["name"].(string)
		require.True(t, ok)
		return name
	}

	// POST /chat/completions must carry ONLY the specific advanced-ratelimit (chat-quota),
	// not the wildcard one.
	chatOp := findOperation(result.Spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, chatOp)
	require.NotNil(t, chatOp.Policies)
	require.Len(t, *chatOp.Policies, 1, "/chat/completions should not stack the wildcard policy on top of its specific one")
	assert.Equal(t, "advanced-ratelimit", (*chatOp.Policies)[0].Name)
	assert.Equal(t, "chat-quota", firstQuotaName((*chatOp.Policies)[0].Params))

	// POST /* keeps the wildcard advanced-ratelimit (wildcard-quota) so other paths remain governed by it.
	wildcardOp := findOperation(result.Spec.Operations, "/*", "POST")
	require.NotNil(t, wildcardOp)
	require.NotNil(t, wildcardOp.Policies)
	require.Len(t, *wildcardOp.Policies, 1)
	assert.Equal(t, "advanced-ratelimit", (*wildcardOp.Policies)[0].Name)
	assert.Equal(t, "wildcard-quota", firstQuotaName((*wildcardOp.Policies)[0].Params))
}

func TestTransformProvider_MostSpecificPathWinsAcrossNestedWildcards(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000003",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))
	require.NoError(t, store.AddTemplate(template))

	rlParams := func(limit int) map[string]interface{} {
		return map[string]interface{}{
			"quotas": []interface{}{
				map[string]interface{}{
					"name": "request-limit",
					"limits": []interface{}{
						map[string]interface{}{"limit": limit, "duration": "1h"},
					},
				},
			},
		}
	}

	upstreamURL := "https://api.openai.com"
	// One policy attached to three overlapping paths of decreasing specificity. Each path's
	// limit differs so we can tell which one governs each operation. The most specific path
	// must win for every operation - this must work for nested wildcards (/chat/*), not just
	// the root catch-all (/*).
	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "OpenAI Provider",
			Version:     "1.0.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: &upstreamURL,
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			Policies: &[]api.LLMPolicy{
				{
					Name:    "advanced-ratelimit",
					Version: "v1",
					Paths: []api.LLMPolicyPath{
						{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlParams(4)},
						{Path: "/chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlParams(2)},
						{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlParams(1)},
					},
				},
			},
		},
	}

	result, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)
	require.NotNil(t, result)

	firstLimit := func(params *map[string]interface{}) int {
		require.NotNil(t, params)
		quotas, ok := (*params)["quotas"].([]interface{})
		require.True(t, ok)
		require.NotEmpty(t, quotas)
		quota, ok := quotas[0].(map[string]interface{})
		require.True(t, ok)
		limits, ok := quota["limits"].([]interface{})
		require.True(t, ok)
		require.NotEmpty(t, limits)
		limit, ok := limits[0].(map[string]interface{})
		require.True(t, ok)
		v, ok := limit["limit"].(int)
		require.True(t, ok)
		return v
	}

	assertSingleLimit := func(t *testing.T, path string, want int) {
		t.Helper()
		op := findOperation(result.Spec.Operations, path, "POST")
		require.NotNil(t, op, "expected a POST operation for %s", path)
		require.NotNil(t, op.Policies)
		require.Len(t, *op.Policies, 1, "%s should carry exactly one advanced-ratelimit (most specific path wins)", path)
		assert.Equal(t, "advanced-ratelimit", (*op.Policies)[0].Name)
		assert.Equal(t, want, firstLimit((*op.Policies)[0].Params), "%s should be governed by its own limit", path)
	}

	// /chat/completions (exact) -> 4, /chat/* (nested wildcard) -> 2, /* (root) -> 1.
	assertSingleLimit(t, "/chat/completions", 4)
	assertSingleLimit(t, "/chat/*", 2)
	assertSingleLimit(t, "/*", 1)
}

func TestTransformProvider_MostSpecificPathWinsAcrossMethodsAndOrder(t *testing.T) {
	rlParams := func(limit int) map[string]interface{} {
		return map[string]interface{}{
			"quotas": []interface{}{
				map[string]interface{}{
					"name": "request-limit",
					"limits": []interface{}{
						map[string]interface{}{"limit": limit, "duration": "1h"},
					},
				},
			},
		}
	}

	ccPost := api.LLMPolicyPath{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: rlParams(4)}
	ccGet := api.LLMPolicyPath{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: rlParams(10)}
	chatWild := api.LLMPolicyPath{Path: "/chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlParams(2)}
	rootWild := api.LLMPolicyPath{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlParams(1)}

	// The expected outcome must be identical regardless of declaration order, including a
	// same path with different methods carrying different limits (POST=4, GET=10).
	cases := []struct {
		name  string
		paths []api.LLMPolicyPath
	}{
		{"specific first", []api.LLMPolicyPath{ccPost, ccGet, chatWild, rootWild}},
		{"wildcards first", []api.LLMPolicyPath{rootWild, chatWild, ccGet, ccPost}},
		{"interleaved", []api.LLMPolicyPath{rootWild, ccPost, chatWild, ccGet}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := storage.NewConfigStore()
			db := newTestMockDB()
			routerConfig := &config.RouterConfig{ListenerPort: 8080}
			transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

			template := &models.StoredLLMProviderTemplate{
				UUID:          "0000-template-1-0000-000000000004",
				Configuration: api.LLMProviderTemplate{Metadata: api.Metadata{Name: "openai"}, Spec: api.LLMProviderTemplateData{}},
			}
			require.NoError(t, db.SaveLLMProviderTemplate(template))
			require.NoError(t, store.AddTemplate(template))

			upstreamURL := "https://api.openai.com"
			provider := &api.LLMProviderConfiguration{
				Metadata: api.Metadata{Name: "openai-provider"},
				Spec: api.LLMProviderConfigData{
					DisplayName:   "OpenAI Provider",
					Version:       "1.0.0",
					Template:      "openai",
					Upstream:      api.LLMProviderConfigData_Upstream{Url: &upstreamURL},
					AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
					Policies: &[]api.LLMPolicy{
						{Name: "advanced-ratelimit", Version: "v1", Paths: tc.paths},
					},
				},
			}

			result, err := transformer.Transform(provider, &api.RestAPI{})
			require.NoError(t, err)
			require.NotNil(t, result)

			firstLimit := func(params *map[string]interface{}) int {
				require.NotNil(t, params)
				quotas, ok := (*params)["quotas"].([]interface{})
				require.True(t, ok)
				require.NotEmpty(t, quotas)
				quota, ok := quotas[0].(map[string]interface{})
				require.True(t, ok)
				limits, ok := quota["limits"].([]interface{})
				require.True(t, ok)
				require.NotEmpty(t, limits)
				limit, ok := limits[0].(map[string]interface{})
				require.True(t, ok)
				v, ok := limit["limit"].(int)
				require.True(t, ok)
				return v
			}
			assertLimit := func(path, method string, want int) {
				op := findOperation(result.Spec.Operations, path, method)
				require.NotNil(t, op, "expected %s %s operation", method, path)
				require.NotNil(t, op.Policies)
				require.Len(t, *op.Policies, 1, "%s %s should carry exactly one advanced-ratelimit", method, path)
				assert.Equal(t, want, firstLimit((*op.Policies)[0].Params), "%s %s limit", method, path)
			}

			// Same path, method-specific limits win over the wildcards.
			assertLimit("/chat/completions", "POST", 4)
			assertLimit("/chat/completions", "GET", 10)
			// Nested wildcard wins over root wildcard, for every method.
			assertLimit("/chat/*", "POST", 2)
			assertLimit("/chat/*", "GET", 2)
			// Root wildcard governs everything else.
			assertLimit("/*", "POST", 1)
		})
	}
}

func TestTransformProvider_MostSpecificMethodWinsOnSamePath(t *testing.T) {
	rlParams := func(limit int) map[string]interface{} {
		return map[string]interface{}{
			"quotas": []interface{}{
				map[string]interface{}{
					"name": "request-limit",
					"limits": []interface{}{
						map[string]interface{}{"limit": limit, "duration": "1h"},
					},
				},
			},
		}
	}

	// Same path /chat/completions with a wildcard-method entry (limit 4) and a GET-specific
	// entry (limit 10), plus nested and root wildcards. GET must win on /chat/completions;
	// every other method falls to the '*' entry.
	ccAll := api.LLMPolicyPath{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlParams(4)}
	ccGet := api.LLMPolicyPath{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: rlParams(10)}
	rootWild := api.LLMPolicyPath{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlParams(1)}
	chatWild := api.LLMPolicyPath{Path: "/chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlParams(2)}

	cases := []struct {
		name  string
		paths []api.LLMPolicyPath
	}{
		{"as declared", []api.LLMPolicyPath{ccAll, ccGet, rootWild, chatWild}},
		{"reversed", []api.LLMPolicyPath{chatWild, rootWild, ccGet, ccAll}},
		{"get before all", []api.LLMPolicyPath{ccGet, ccAll, chatWild, rootWild}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := storage.NewConfigStore()
			db := newTestMockDB()
			routerConfig := &config.RouterConfig{ListenerPort: 8080}
			transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

			template := &models.StoredLLMProviderTemplate{
				UUID:          "0000-template-1-0000-000000000005",
				Configuration: api.LLMProviderTemplate{Metadata: api.Metadata{Name: "openai"}, Spec: api.LLMProviderTemplateData{}},
			}
			require.NoError(t, db.SaveLLMProviderTemplate(template))
			require.NoError(t, store.AddTemplate(template))

			upstreamURL := "https://api.openai.com"
			provider := &api.LLMProviderConfiguration{
				Metadata: api.Metadata{Name: "openai-provider"},
				Spec: api.LLMProviderConfigData{
					DisplayName:   "OpenAI Provider",
					Version:       "1.0.0",
					Template:      "openai",
					Upstream:      api.LLMProviderConfigData_Upstream{Url: &upstreamURL},
					AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
					Policies:      &[]api.LLMPolicy{{Name: "advanced-ratelimit", Version: "v1", Paths: tc.paths}},
				},
			}

			result, err := transformer.Transform(provider, &api.RestAPI{})
			require.NoError(t, err)
			require.NotNil(t, result)

			firstLimit := func(params *map[string]interface{}) int {
				require.NotNil(t, params)
				quotas, ok := (*params)["quotas"].([]interface{})
				require.True(t, ok)
				require.NotEmpty(t, quotas)
				quota, ok := quotas[0].(map[string]interface{})
				require.True(t, ok)
				limits, ok := quota["limits"].([]interface{})
				require.True(t, ok)
				require.NotEmpty(t, limits)
				limit, ok := limits[0].(map[string]interface{})
				require.True(t, ok)
				v, ok := limit["limit"].(int)
				require.True(t, ok)
				return v
			}
			assertLimit := func(path, method string, want int) {
				op := findOperation(result.Spec.Operations, path, method)
				require.NotNil(t, op, "expected %s %s operation", method, path)
				require.NotNil(t, op.Policies)
				require.Len(t, *op.Policies, 1, "%s %s should carry exactly one advanced-ratelimit", method, path)
				assert.Equal(t, want, firstLimit((*op.Policies)[0].Params), "%s %s limit", method, path)
			}

			// GET is the most specific (concrete method on the most specific path) -> 10.
			assertLimit("/chat/completions", "GET", 10)
			// Every other method on /chat/completions falls to the '*' entry -> 4.
			assertLimit("/chat/completions", "POST", 4)
			assertLimit("/chat/completions", "PUT", 4)
			// Nested and root wildcards unchanged.
			assertLimit("/chat/*", "POST", 2)
			assertLimit("/*", "POST", 1)
		})
	}
}

// --- shared helpers for policy path/method specificity tests ---

func rlQuota(limit int) map[string]interface{} {
	return map[string]interface{}{
		"quotas": []interface{}{
			map[string]interface{}{
				"name": "request-limit",
				"limits": []interface{}{
					map[string]interface{}{"limit": limit, "duration": "1h"},
				},
			},
		},
	}
}

func quotaLimitOf(t *testing.T, p api.Policy) int {
	t.Helper()
	require.NotNil(t, p.Params)
	quotas, ok := (*p.Params)["quotas"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, quotas)
	q, ok := quotas[0].(map[string]interface{})
	require.True(t, ok)
	limits, ok := q["limits"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, limits)
	lim, ok := limits[0].(map[string]interface{})
	require.True(t, ok)
	v, ok := lim["limit"].(int)
	require.True(t, ok)
	return v
}

func transformProviderWithPolicies(t *testing.T, uuid string, tmplSpec api.LLMProviderTemplateData,
	ac api.LLMAccessControl, policies []api.LLMPolicy) *api.RestAPI {
	t.Helper()
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())
	template := &models.StoredLLMProviderTemplate{
		UUID:          uuid,
		Configuration: api.LLMProviderTemplate{Metadata: api.Metadata{Name: "openai"}, Spec: tmplSpec},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))
	require.NoError(t, store.AddTemplate(template))
	upstreamURL := "https://api.openai.com"
	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName:   "OpenAI Provider",
			Version:       "1.0.0",
			Template:      "openai",
			Upstream:      api.LLMProviderConfigData_Upstream{Url: &upstreamURL},
			AccessControl: ac,
			Policies:      &policies,
		},
	}
	result, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

// #1: a narrower HTTP method set is more specific than a broader one on the same path.
func TestTransformProvider_NarrowerMethodSetWins(t *testing.T) {
	result := transformProviderWithPolicies(t, "0000-template-1-0000-000000000006",
		api.LLMProviderTemplateData{}, api.LLMAccessControl{Mode: api.AllowAll},
		[]api.LLMPolicy{{Name: "advanced-ratelimit", Version: "v1", Paths: []api.LLMPolicyPath{
			{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"GET", "POST"}, Params: rlQuota(3)},
			{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: rlQuota(100)},
		}}})

	// GET is only covered by [GET, POST] -> 3.
	getOp := findOperation(result.Spec.Operations, "/chat/completions", "GET")
	require.NotNil(t, getOp)
	require.NotNil(t, getOp.Policies)
	require.Len(t, *getOp.Policies, 1)
	assert.Equal(t, 3, quotaLimitOf(t, (*getOp.Policies)[0]))

	// POST is covered by both; the narrower [POST] entry wins -> 100.
	postOp := findOperation(result.Spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, postOp)
	require.NotNil(t, postOp.Policies)
	require.Len(t, *postOp.Policies, 1, "POST should be governed only by the narrower [POST] entry")
	assert.Equal(t, 100, quotaLimitOf(t, (*postOp.Policies)[0]))
}

// #2: path specificity dominates method specificity (confirmed behavior).
func TestTransformProvider_PathDominatesMethodSpecificity(t *testing.T) {
	result := transformProviderWithPolicies(t, "0000-template-1-0000-000000000007",
		api.LLMProviderTemplateData{}, api.LLMAccessControl{Mode: api.AllowAll},
		[]api.LLMPolicy{{Name: "advanced-ratelimit", Version: "v1", Paths: []api.LLMPolicyPath{
			{Path: "/chat/*", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: rlQuota(100)},
			{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(2)},
		}}})

	// POST /chat/completions: the more specific PATH wins over a method-specific rule on a
	// less specific (wildcard) path -> 2.
	postOp := findOperation(result.Spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, postOp)
	require.NotNil(t, postOp.Policies)
	require.Len(t, *postOp.Policies, 1, "the most specific path must win over a method-specific rule on a wildcard path")
	assert.Equal(t, 2, quotaLimitOf(t, (*postOp.Policies)[0]))

	// POST /chat/<other> is still governed by /chat/* [POST] -> 100.
	wildOp := findOperation(result.Spec.Operations, "/chat/*", "POST")
	require.NotNil(t, wildOp)
	require.NotNil(t, wildOp.Policies)
	require.Len(t, *wildOp.Policies, 1)
	assert.Equal(t, 100, quotaLimitOf(t, (*wildOp.Policies)[0]))
}

// #3: different policy names resolve their specificity independently (no cross-policy suppression).
func TestTransformProvider_DifferentPoliciesResolveIndependently(t *testing.T) {
	hdr := func(scope string) map[string]interface{} {
		return map[string]interface{}{"scope": scope}
	}
	result := transformProviderWithPolicies(t, "0000-template-1-0000-000000000008",
		api.LLMProviderTemplateData{}, api.LLMAccessControl{Mode: api.AllowAll},
		[]api.LLMPolicy{
			{Name: "advanced-ratelimit", Version: "v1", Paths: []api.LLMPolicyPath{
				{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(4)},
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(1)},
			}},
			{Name: "set-headers", Version: "v1", Paths: []api.LLMPolicyPath{
				{Path: "/chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: hdr("chat")},
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: hdr("root")},
			}},
		})

	// /chat/completions POST: advanced-ratelimit from its own specific path (4) AND
	// set-headers from /chat/* (the most specific set-headers path covering it).
	op := findOperation(result.Spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, op)
	require.NotNil(t, op.Policies)
	require.Len(t, *op.Policies, 2, "two different policies must both apply (no cross-policy suppression)")
	byName := map[string]api.Policy{}
	for _, p := range *op.Policies {
		byName[p.Name] = p
	}
	require.Contains(t, byName, "advanced-ratelimit")
	require.Contains(t, byName, "set-headers")
	assert.Equal(t, 4, quotaLimitOf(t, byName["advanced-ratelimit"]))
	assert.Equal(t, "chat", (*byName["set-headers"].Params)["scope"])
}

// #4: most specific wins under deny_all access control (over allowed exception paths).
func TestTransformProvider_DenyAllModeMostSpecificWins(t *testing.T) {
	exceptions := []api.RouteException{
		{Path: "/chat/completions", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "/chat/other", Methods: []api.RouteExceptionMethods{"*"}},
	}
	result := transformProviderWithPolicies(t, "0000-template-1-0000-000000000009",
		api.LLMProviderTemplateData{},
		api.LLMAccessControl{Mode: api.DenyAll, Exceptions: &exceptions},
		[]api.LLMPolicy{{Name: "advanced-ratelimit", Version: "v1", Paths: []api.LLMPolicyPath{
			{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(4)},
			{Path: "/chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(2)},
		}}})

	// /chat/completions keeps only its own quota (4), not the /chat/* one.
	ccOp := findOperation(result.Spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, ccOp)
	require.NotNil(t, ccOp.Policies)
	require.Len(t, *ccOp.Policies, 1, "/chat/completions should keep only its own quota in deny_all mode")
	assert.Equal(t, 4, quotaLimitOf(t, (*ccOp.Policies)[0]))

	// /chat/other (covered only by /chat/*) gets the /chat/* quota (2).
	otherOp := findOperation(result.Spec.Operations, "/chat/other", "POST")
	require.NotNil(t, otherOp)
	require.NotNil(t, otherOp.Policies)
	require.Len(t, *otherOp.Policies, 1)
	assert.Equal(t, 2, quotaLimitOf(t, (*otherOp.Policies)[0]))
}

// #6: a wildcard policy expanded onto template resource paths still loses to a same-name
// policy attached directly to that resource path.
func TestTransformProvider_TemplateExpansionDedupesSameNamePolicy(t *testing.T) {
	tmplSpec := api.LLMProviderTemplateData{
		ResourceMappings: &api.LLMProviderTemplateResourceMappings{
			Resources: &[]api.LLMProviderTemplateResourceMapping{
				{Resource: "/responses"},
			},
		},
	}
	result := transformProviderWithPolicies(t, "0000-template-1-0000-000000000010",
		tmplSpec, api.LLMAccessControl{Mode: api.AllowAll},
		[]api.LLMPolicy{{Name: "advanced-ratelimit", Version: "v1", Paths: []api.LLMPolicyPath{
			{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(1)},
			{Path: "/responses", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(5)},
		}}})

	// /responses (an expanded template resource) keeps ONLY its specific limit (5).
	respOp := findOperation(result.Spec.Operations, "/responses", "POST")
	require.NotNil(t, respOp)
	require.NotNil(t, respOp.Policies)
	require.Len(t, *respOp.Policies, 1, "/responses should not also carry the wildcard policy after template expansion")
	assert.Equal(t, 5, quotaLimitOf(t, (*respOp.Policies)[0]))

	// /* keeps the wildcard limit (1).
	wildOp := findOperation(result.Spec.Operations, "/*", "POST")
	require.NotNil(t, wildOp)
	require.NotNil(t, wildOp.Policies)
	require.Len(t, *wildOp.Policies, 1)
	assert.Equal(t, 1, quotaLimitOf(t, (*wildOp.Policies)[0]))
}

// Two separate policy blocks of the SAME name each resolve most-specific-within-block and
// both layer onto every route (the user's two-advanced-ratelimit scenario).
func TestTransformProvider_SeparatePoliciesOfSameNameBothApply(t *testing.T) {
	block1 := api.LLMPolicy{Name: "advanced-ratelimit", Version: "v1", Paths: []api.LLMPolicyPath{
		{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(4)},
		{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: rlQuota(10)},
		{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(1)},
		{Path: "/chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(2)},
	}}
	block2 := api.LLMPolicy{Name: "advanced-ratelimit", Version: "v1", Paths: []api.LLMPolicyPath{
		{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: rlQuota(100)},
	}}
	result := transformProviderWithPolicies(t, "0000-template-1-0000-000000000011",
		api.LLMProviderTemplateData{}, api.LLMAccessControl{Mode: api.AllowAll},
		[]api.LLMPolicy{block1, block2})

	limitsFor := func(path, method string) []int {
		op := findOperation(result.Spec.Operations, path, method)
		require.NotNil(t, op, "expected %s %s operation", method, path)
		require.NotNil(t, op.Policies)
		out := []int{}
		for _, p := range *op.Policies {
			assert.Equal(t, "advanced-ratelimit", p.Name)
			out = append(out, quotaLimitOf(t, p))
		}
		return out
	}

	// Each route carries block #1's most-specific match AND block #2's /* (100). Both apply.
	assert.ElementsMatch(t, []int{4, 100}, limitsFor("/chat/completions", "POST"))
	assert.ElementsMatch(t, []int{10, 100}, limitsFor("/chat/completions", "GET"))
	assert.ElementsMatch(t, []int{2, 100}, limitsFor("/chat/*", "POST"))
	assert.ElementsMatch(t, []int{1, 100}, limitsFor("/*", "POST"))
}

func TestTransformProvider_WithUpstreamAuth(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	// Add a template to the store
	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
	db.SaveLLMProviderTemplate(template)
	err := store.AddTemplate(template)
	require.NoError(t, err)

	upstreamURL := "https://api.openai.com"
	authHeader := "Authorization"
	authValue := "Bearer sk-xxx"
	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "OpenAI Provider",
			Version:     "1.0.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: &upstreamURL,
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: &authHeader,
					Value:  &authValue,
				},
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.RestAPI{}
	result, err := transformer.Transform(provider, output)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that policies were added
	require.NotEmpty(t, result.Spec.Operations)
	for _, op := range result.Spec.Operations {
		require.NotNil(t, op.Policies)
		found := false
		for _, p := range *op.Policies {
			if p.Name == constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME {
				found = true
				break
			}
		}
		assert.True(t, found, "operation %s %s should include upstream auth policy", op.Method, op.Path)
	}
}

func TestTransformProxy_WithUpstreamAuth(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	// Add a template to the store (required for proxy transform to resolve provider template params)
	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
	db.SaveLLMProviderTemplate(template)
	err := store.AddTemplate(template)
	require.NoError(t, err)

	// Create and store a provider config referenced by the proxy
	upstreamURL := "https://api.openai.com"
	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "OpenAI Provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: &upstreamURL,
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}

	providerOut := &api.RestAPI{}
	providerAPI, err := transformer.Transform(provider, providerOut)
	require.NoError(t, err)
	require.NotNil(t, providerAPI)

	storedProvider := &models.StoredConfig{
		UUID:                "0000-prov-cfg-1-0000-000000000000",
		Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:              "openai-provider",
		DisplayName:         "OpenAI Provider",
		Version:             "v1.0",
		Configuration:       *providerAPI,
		SourceConfiguration: *provider,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
	}
	db.SaveConfig(storedProvider)
	err = store.Add(storedProvider)
	require.NoError(t, err)

	// Create proxy config with upstreamAuth override
	authHeader := "Authorization"
	authValue := "Bearer proxy-secret"
	proxy := &api.LLMProxyConfiguration{
		Metadata: api.Metadata{Name: "openai-proxy"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "OpenAI Proxy",
			Version:     "v1.0",
			Provider: api.LLMProxyProvider{
				Id: "openai-provider",
				Auth: &api.LLMUpstreamAuth{
					Type:   api.LLMUpstreamAuthTypeApiKey,
					Header: &authHeader,
					Value:  &authValue,
				},
			},
		},
	}

	output := &api.RestAPI{}
	result, err := transformer.Transform(proxy, output)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.NotEmpty(t, result.Spec.Operations)

	// Ensure upstream-auth policy is present on all operations
	for _, op := range result.Spec.Operations {
		require.NotNil(t, op.Policies)
		found := false
		for _, p := range *op.Policies {
			if p.Name == constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME {
				found = true
				break
			}
		}
		assert.True(t, found, "operation %s %s should include upstream auth policy", op.Method, op.Path)
	}
}

func TestTransformProvider_UnsupportedMode(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	// Add a template to the store
	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
	db.SaveLLMProviderTemplate(template)
	err := store.AddTemplate(template)
	require.NoError(t, err)

	upstreamURL := "https://api.openai.com"
	provider := &api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "OpenAI Provider",
			Version:     "1.0.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: &upstreamURL,
			},
			AccessControl: api.LLMAccessControl{
				Mode: "unsupported-mode",
			},
		},
	}

	output := &api.RestAPI{}
	_, err = transformer.Transform(provider, output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported access control mode")
}
