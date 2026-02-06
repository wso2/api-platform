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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestNewLLMProviderTransformer_Basic(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
		HTTPSEnabled: false,
	}

	transformer := NewLLMProviderTransformer(store, routerConfig, newTestPolicyVersionResolver())
	assert.NotNil(t, transformer)
	assert.Equal(t, store, transformer.store)
	assert.Equal(t, routerConfig, transformer.routerConfig)
}

func TestLLMProviderTransformer_Transform_InvalidInput(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, routerConfig, newTestPolicyVersionResolver())

	t.Run("Invalid input type returns error", func(t *testing.T) {
		output := &api.APIConfiguration{}
		_, err := transformer.Transform("invalid-type", output)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid input type")
	})

	t.Run("Nil input returns error", func(t *testing.T) {
		output := &api.APIConfiguration{}
		_, err := transformer.Transform(nil, output)
		assert.Error(t, err)
	})
}

func TestGetUpstreamAuthApikeyPolicyParams_Extended(t *testing.T) {
	t.Run("Valid parameters", func(t *testing.T) {
		params, err := GetUpstreamAuthApikeyPolicyParams("Authorization", "Bearer token123")
		assert.NoError(t, err)
		assert.NotNil(t, params)
		assert.Contains(t, params, "requestHeaders")
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
		assert.Contains(t, params, "requestHeaders")
	})

	t.Run("Empty host value", func(t *testing.T) {
		params, err := GetHostAdditionPolicyParams("")
		assert.NoError(t, err)
		assert.NotNil(t, params)
	})
}

func TestBuildTemplateParams(t *testing.T) {
	t.Run("Nil template returns error", func(t *testing.T) {
		params, err := buildTemplateParams(nil)
		assert.Error(t, err)
		assert.Nil(t, params)
	})

	t.Run("Empty template returns empty params", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			Configuration: api.LLMProviderTemplate{
				Spec: api.LLMProviderTemplateData{},
			},
		}
		params, err := buildTemplateParams(template)
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
		params, err := buildTemplateParams(template)
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
		params, err := buildTemplateParams(template)
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
		params, err := buildTemplateParams(template)
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
		params, err := buildTemplateParams(template)
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
		params, err := buildTemplateParams(template)
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
		params, err := buildTemplateParams(template)
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
		params, err := buildTemplateParams(template)
		assert.NoError(t, err)
		assert.Len(t, params, 6)
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
		base := map[string]interface{}{"key1": "value1"}
		extra := map[string]interface{}{}
		result := mergeParams(base, extra)
		assert.Equal(t, "value1", (*result)["key1"])
	})

	t.Run("Merge with extra only", func(t *testing.T) {
		base := map[string]interface{}{}
		extra := map[string]interface{}{"key2": "value2"}
		result := mergeParams(base, extra)
		assert.Equal(t, "value2", (*result)["key2"])
	})

	t.Run("Merge with overlapping keys", func(t *testing.T) {
		base := map[string]interface{}{"key": "base-value"}
		extra := map[string]interface{}{"key": "extra-value"}
		result := mergeParams(base, extra)
		// Extra should override base
		assert.Equal(t, "extra-value", (*result)["key"])
	})

	t.Run("Merge with different keys", func(t *testing.T) {
		base := map[string]interface{}{"key1": "value1"}
		extra := map[string]interface{}{"key2": "value2"}
		result := mergeParams(base, extra)
		assert.Equal(t, "value1", (*result)["key1"])
		assert.Equal(t, "value2", (*result)["key2"])
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
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, routerConfig, newTestPolicyVersionResolver())

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

	output := &api.APIConfiguration{}
	_, err := transformer.Transform(provider, output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve template")
}

func TestTransformProvider_AllowAllMode(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, routerConfig, newTestPolicyVersionResolver())

	// Add a template to the store
	template := &models.StoredLLMProviderTemplate{
		ID: "template-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
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

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, api.RestApi, result.Kind)
}

func TestTransformProvider_DenyAllMode(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, routerConfig, newTestPolicyVersionResolver())

	// Add a template to the store
	template := &models.StoredLLMProviderTemplate{
		ID: "template-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
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

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, api.RestApi, result.Kind)
}

func TestTransformProvider_WithUpstreamAuth(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, routerConfig, newTestPolicyVersionResolver())

	// Add a template to the store
	template := &models.StoredLLMProviderTemplate{
		ID: "template-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
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

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that policies were added
	apiData, err := result.Spec.AsAPIConfigData()
	assert.NoError(t, err)
	assert.NotNil(t, apiData.Policies)
}

func TestTransformProvider_UnsupportedMode(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{
		ListenerPort: 8080,
	}
	transformer := NewLLMProviderTransformer(store, routerConfig, newTestPolicyVersionResolver())

	// Add a template to the store
	template := &models.StoredLLMProviderTemplate{
		ID: "template-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{},
		},
	}
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

	output := &api.APIConfiguration{}
	_, err = transformer.Transform(provider, output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported access control mode")
}
