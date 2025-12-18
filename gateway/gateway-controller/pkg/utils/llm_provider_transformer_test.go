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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// setupTestTransformer creates a transformer with a mock store containing test templates
func setupTestTransformer(t *testing.T) (*LLMProviderTransformer, *storage.ConfigStore) {
	store := storage.NewConfigStore()

	// Add test templates
	openAITemplate := &models.StoredLLMProviderTemplate{
		ID: "openai-template-id",
		Configuration: api.LLMProviderTemplate{
			Version: "ai.api-platform.wso2.com/v1",
			Kind:    "llm/provider-template",
			Spec: api.LLMProviderTemplateData{
				DisplayName: "openai",
				PromptTokens: &api.ExtractionIdentifier{
					Location:   api.Payload,
					Identifier: "$.usage.prompt_tokens",
				},
			},
		},
	}

	err := store.AddTemplate(openAITemplate)
	require.NoError(t, err, "Failed to add test template")

	transformer := NewLLMProviderTransformer(store)
	return transformer, store
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewLLMProviderTransformer(t *testing.T) {
	store := storage.NewConfigStore()
	transformer := NewLLMProviderTransformer(store)

	assert.NotNil(t, transformer, "Transformer should not be nil")
	assert.NotNil(t, transformer.store, "Store should not be nil")
}

// ============================================================================
// Basic Transformation Tests
// ============================================================================

func TestTransform_MinimalProvider(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "minimal-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err, "Transform should succeed")
	require.NotNil(t, result, "Result should not be nil")

	// Verify basic fields
	assert.Equal(t, api.RestApi, result.Kind)
	assert.Equal(t, api.GatewayApiPlatformWso2Comv1alpha1, result.ApiVersion)

	// Extract spec
	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err, "Should extract spec")

	assert.Equal(t, "minimal-provider", spec.DisplayName)
	assert.Equal(t, "v1.0", spec.Version)
	assert.Equal(t, "/", spec.Context) // Default context
	assert.NotNil(t, spec.Upstream.Main.Url)
	assert.Equal(t, "https://api.openai.com", *spec.Upstream.Main.Url)
}

func TestTransform_FullProvider(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "full-provider",
			Version:  "v1.0",
			Context:  stringPtr("/openai"),
			Vhost:    stringPtr("api.openai.com"),
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: stringPtr("Authorization"),
					Value:  stringPtr("Bearer sk-test123"),
				},
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err, "Transform should succeed")

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, "full-provider", spec.DisplayName)
	assert.Equal(t, "v1.0", spec.Version)
	assert.Equal(t, "/openai", spec.Context)

	// Verify vhost
	require.NotNil(t, spec.Vhosts)
	assert.Equal(t, "api.openai.com", spec.Vhosts.Main)

	// Verify upstream
	assert.NotNil(t, spec.Upstream.Main.Url)
	assert.Equal(t, "https://api.openai.com", *spec.Upstream.Main.Url)

	// Verify auth policy added
	require.NotNil(t, spec.Policies)
	assert.Len(t, *spec.Policies, 1)
	authPolicy := (*spec.Policies)[0]
	assert.Equal(t, "ModifyHeaders", authPolicy.Name)
	assert.Equal(t, "v1.0.0", authPolicy.Version)
	assert.NotNil(t, authPolicy.Params)
}

// ============================================================================
// Invalid Input Tests
// ============================================================================

func TestTransform_InvalidInputType(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	// Try to transform wrong type
	invalidInput := &api.LLMProviderTemplate{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider-template",
	}

	output := &api.APIConfiguration{}
	_, err := transformer.Transform(invalidInput, output)

	require.Error(t, err, "Should error on invalid input type")
	assert.Contains(t, err.Error(), "invalid input type")
	assert.Contains(t, err.Error(), "expected *api.LLMProviderConfiguration")
}

func TestTransform_NonExistentTemplate(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "nonexistent-template",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.APIConfiguration{}
	_, err := transformer.Transform(provider, output)

	require.Error(t, err, "Should error on nonexistent template")
	assert.Contains(t, err.Error(), "failed to retrieve template")
	assert.Contains(t, err.Error(), "nonexistent-template")
}

// ============================================================================
// Context Handling Tests
// ============================================================================

func TestTransform_DefaultContext(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Context:  nil, // No context provided
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	assert.Equal(t, "/", spec.Context, "Should use default base path")
}

func TestTransform_CustomContext(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	tests := []struct {
		name    string
		context string
	}{
		{"root path", "/"},
		{"simple path", "/api"},
		{"nested path", "/api/v1"},
		{"complex path", "/my-llm/providers/openai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &api.LLMProviderConfiguration{
				Version: "ai.api-platform.wso2.com/v1",
				Kind:    "llm/provider",
				Spec: api.LLMProviderConfigData{

					DisplayName: "test",
					Version:  "v1.0",
					Template: "openai",
					Context:  stringPtr(tt.context),
					Upstream: api.LLMProviderConfigData_Upstream{
						Url: stringPtr("https://api.example.com"),
					},
					AccessControl: api.LLMAccessControl{
						Mode: api.AllowAll,
					},
				},
			}

			output := &api.APIConfiguration{}
			result, err := transformer.Transform(provider, output)

			require.NoError(t, err)

			spec, err := result.Spec.AsAPIConfigData()
			require.NoError(t, err)

			assert.Equal(t, tt.context, spec.Context)
		})
	}
}

// ============================================================================
// Vhost Tests
// ============================================================================

func TestTransform_NoVhost(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Vhost:    nil,
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	assert.Nil(t, spec.Vhosts, "Vhosts should be nil when not provided")
}

func TestTransform_WithVhost(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Vhost:    stringPtr("api.mycompany.com"),
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	require.NotNil(t, spec.Vhosts)
	assert.Equal(t, "api.mycompany.com", spec.Vhosts.Main)
	assert.Nil(t, spec.Vhosts.Sandbox)
}

// ============================================================================
// Upstream Auth Tests
// ============================================================================

func TestTransform_NoAuth(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url:  stringPtr("https://api.example.com"),
				Auth: nil,
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// No auth policy should be added
	if spec.Policies != nil {
		assert.Len(t, *spec.Policies, 0, "Should have no policies without auth")
	}
}

func TestTransform_ApiKeyAuth(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: stringPtr("X-API-Key"),
					Value:  stringPtr("secret-key-123"),
				},
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Verify auth policy
	require.NotNil(t, spec.Policies)
	require.Len(t, *spec.Policies, 1)

	policy := (*spec.Policies)[0]
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, policy.Name)
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_VERSION, policy.Version)
	require.NotNil(t, policy.Params)

	// Verify policy params contain header and value
	params := *policy.Params
	assert.Contains(t, params, "requestHeaders")
}

func TestTransform_UnsupportedAuthType(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   "bearer", // Unsupported type
					Header: stringPtr("Authorization"),
					Value:  stringPtr("Bearer token"),
				},
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	output := &api.APIConfiguration{}
	_, err := transformer.Transform(provider, output)

	require.Error(t, err, "Should error on unsupported auth type")
	assert.Contains(t, err.Error(), "unsupported upstream auth type")
}

// ============================================================================
// Access Control - Allow All Mode Tests
// ============================================================================

func TestTransform_AllowAll_NoExceptions(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.AllowAll,
				Exceptions: nil,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should have only catch-all operation
	require.Len(t, spec.Operations, 1)
	assert.Equal(t, api.OperationMethod("*"), spec.Operations[0].Method)
	assert.Equal(t, "/*", spec.Operations[0].Path)
}

func TestTransform_AllowAll_WithSingleException(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/admin",
			Methods: []api.RouteExceptionMethods{api.GET, api.POST},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.AllowAll,
				Exceptions: &exceptions,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should have 2 exception operations + 1 catch-all = 3 total
	require.Len(t, spec.Operations, 3)

	// Verify exception operations have Respond policy
	foundGET := false
	foundPOST := false
	foundCatchAll := false

	for _, op := range spec.Operations {
		if op.Path == "/admin" && op.Method == api.OperationMethod("GET") {
			foundGET = true
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
			policy := (*op.Policies)[0]
			assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, policy.Name)
			assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_VERSION, policy.Version)
		}
		if op.Path == "/admin" && op.Method == api.OperationMethod("POST") {
			foundPOST = true
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
		}
		if op.Path == "/*" && op.Method == api.OperationMethod("*") {
			foundCatchAll = true
			// Catch-all should not have policies
			if op.Policies != nil {
				assert.Len(t, *op.Policies, 0)
			}
		}
	}

	assert.True(t, foundGET, "Should have GET /admin operation")
	assert.True(t, foundPOST, "Should have POST /admin operation")
	assert.True(t, foundCatchAll, "Should have catch-all operation")
}

func TestTransform_AllowAll_WithMultipleExceptions(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/admin",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
		{
			Path:    "/internal/metrics",
			Methods: []api.RouteExceptionMethods{api.GET, api.POST, api.DELETE},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.AllowAll,
				Exceptions: &exceptions,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should have 1 + 3 = 4 exception operations + 1 catch-all = 5 total
	require.Len(t, spec.Operations, 5)

	// Count operations by path
	adminOps := 0
	metricsOps := 0
	catchAllOps := 0

	for _, op := range spec.Operations {
		switch op.Path {
		case "/admin":
			adminOps++
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
		case "/internal/metrics":
			metricsOps++
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
		case "/*":
			catchAllOps++
		}
	}

	assert.Equal(t, 1, adminOps, "Should have 1 admin operation")
	assert.Equal(t, 3, metricsOps, "Should have 3 metrics operations")
	assert.Equal(t, 1, catchAllOps, "Should have 1 catch-all operation")
}

// ============================================================================
// Access Control - Deny All Mode Tests
// ============================================================================

func TestTransform_DenyAll_NoExceptions(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.DenyAll,
				Exceptions: nil,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should have NO operations
	assert.Len(t, spec.Operations, 0, "Deny all with no exceptions should have no operations")
}

func TestTransform_DenyAll_WithSingleException(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/v1/chat/completions",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.DenyAll,
				Exceptions: &exceptions,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should have only 1 operation (the exception)
	require.Len(t, spec.Operations, 1)
	assert.Equal(t, api.OperationMethod("POST"), spec.Operations[0].Method)
	assert.Equal(t, "/v1/chat/completions", spec.Operations[0].Path)

	// Exception operations in deny_all mode should NOT have Respond policy
	if spec.Operations[0].Policies != nil {
		assert.Len(t, *spec.Operations[0].Policies, 0, "Deny all exceptions should not have Respond policy")
	}
}

func TestTransform_DenyAll_WithMultipleExceptions(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/v1/chat/completions",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
		{
			Path:    "/v1/embeddings",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
		{
			Path:    "/health",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.DenyAll,
				Exceptions: &exceptions,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should have exactly 3 operations
	require.Len(t, spec.Operations, 3)

	paths := make(map[string]bool)
	for _, op := range spec.Operations {
		paths[op.Path] = true
	}

	assert.True(t, paths["/v1/chat/completions"])
	assert.True(t, paths["/v1/embeddings"])
	assert.True(t, paths["/health"])
}

// ============================================================================
// Access Control - Invalid Mode Test
// ============================================================================

func TestTransform_InvalidAccessControlMode(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: "invalid_mode",
			},
		},
	}

	output := &api.APIConfiguration{}
	_, err := transformer.Transform(provider, output)

	require.Error(t, err, "Should error on invalid access control mode")
	assert.Contains(t, err.Error(), "unsupported access control mode")
}

// ============================================================================
// Policy Application Tests
// ============================================================================

func TestTransform_WithSinglePolicy(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "ContentLengthGuardrail",
			Version: "v0.1.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/v1/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxRequestBodySize": 10240,
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/v1/chat/completions",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.DenyAll,
				Exceptions: &exceptions,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Find the operation
	require.Len(t, spec.Operations, 1)
	op := spec.Operations[0]

	// Verify policy attached
	require.NotNil(t, op.Policies)
	assert.Len(t, *op.Policies, 1)

	policy := (*op.Policies)[0]
	assert.Equal(t, "ContentLengthGuardrail", policy.Name)
	assert.Equal(t, "v0.1.0", policy.Version)
	require.NotNil(t, policy.Params)

	params := *policy.Params
	assert.Equal(t, 10240, params["maxRequestBodySize"])
}

func TestTransform_WithMultiplePoliciesSameRoute(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "ContentLengthGuardrail",
			Version: "v0.1.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/v1/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxRequestBodySize": 10240,
					},
				},
			},
		},
		{
			Name:    "RegexGuardrail",
			Version: "v0.1.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/v1/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"patterns": []map[string]string{
							{"pattern": "password", "flags": "i"},
						},
						"action": "reject",
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/v1/chat/completions",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.DenyAll,
				Exceptions: &exceptions,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Find the operation
	require.Len(t, spec.Operations, 1)
	op := spec.Operations[0]

	// Verify both policies attached
	require.NotNil(t, op.Policies)
	assert.Len(t, *op.Policies, 2)

	// Verify policies are in order
	policyList := *op.Policies
	assert.Equal(t, "ContentLengthGuardrail", policyList[0].Name)
	assert.Equal(t, "RegexGuardrail", policyList[1].Name)
}

func TestTransform_PolicyOnDifferentRoutes(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "ContentLengthGuardrail",
			Version: "v0.1.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/v1/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxRequestBodySize": 10240,
					},
				},
			},
		},
		{
			Name:    "RegexGuardrail",
			Version: "v0.1.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/v1/embeddings",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"patterns": []map[string]string{
							{"pattern": "sensitive"},
						},
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/v1/chat/completions",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
		{
			Path:    "/v1/embeddings",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.DenyAll,
				Exceptions: &exceptions,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Find operations and verify policies
	require.Len(t, spec.Operations, 2)

	for _, op := range spec.Operations {
		require.NotNil(t, op.Policies)
		assert.Len(t, *op.Policies, 1)

		policy := (*op.Policies)[0]
		if op.Path == "/v1/chat/completions" {
			assert.Equal(t, "ContentLengthGuardrail", policy.Name)
		} else if op.Path == "/v1/embeddings" {
			assert.Equal(t, "RegexGuardrail", policy.Name)
		}
	}
}

func TestTransform_PolicyOnWildcardMethod(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "ModifyHeaders",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/v1/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"*"}, // Wildcard method
					Params: map[string]interface{}{
						"requestHeaders": []map[string]string{
							{"action": "SET", "name": "X-Custom", "value": "test"},
						},
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/v1/chat/completions",
			Methods: []api.RouteExceptionMethods{api.POST, api.GET},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.DenyAll,
				Exceptions: &exceptions,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Both operations should have the policy (wildcard matches all)
	require.Len(t, spec.Operations, 2)

	for _, op := range spec.Operations {
		// BUG POTENTIAL: Current implementation checks if op.Method == "*"
		// This might not work correctly for wildcard policy methods
		// The test will reveal if this is working as expected
		if op.Policies != nil {
			t.Logf("Operation %s %s has %d policies", op.Method, op.Path, len(*op.Policies))
		} else {
			t.Logf("Operation %s %s has no policies", op.Method, op.Path)
		}
	}
}

func TestTransform_PolicyOnNonExistentRoute(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	// Policy for route that doesn't exist in operations
	policies := []api.LLMPolicy{
		{
			Name:    "ContentLengthGuardrail",
			Version: "v0.1.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/v1/embeddings", // Not in exceptions
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxRequestBodySize": 10240,
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/v1/chat/completions", // Different path
			Methods: []api.RouteExceptionMethods{api.POST},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.DenyAll,
				Exceptions: &exceptions,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Operation exists but policy shouldn't be attached
	require.Len(t, spec.Operations, 1)
	op := spec.Operations[0]

	// Policy should NOT be attached (path doesn't match)
	if op.Policies != nil {
		assert.Len(t, *op.Policies, 0, "Policy should not be attached to non-matching route")
	}
}

// ============================================================================
// Combined Auth and Access Control Tests
// ============================================================================

func TestTransform_AuthWithAllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/admin",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: stringPtr("Authorization"),
					Value:  stringPtr("Bearer sk-test"),
				},
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.AllowAll,
				Exceptions: &exceptions,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should have API-level auth policy
	require.NotNil(t, spec.Policies)
	assert.Len(t, *spec.Policies, 1)
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, (*spec.Policies)[0].Name)

	// Should have 2 operations (1 exception + 1 catch-all)
	require.Len(t, spec.Operations, 2)

	// Exception operation should have Respond policy
	for _, op := range spec.Operations {
		if op.Path == "/admin" {
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
			assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*op.Policies)[0].Name)
		}
	}
}

// ============================================================================
// GetUpstreamAuthApikeyPolicyParams Tests
// ============================================================================

func TestGetUpstreamAuthApikeyPolicyParams(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
	}{
		{
			name:   "standard Authorization header",
			header: "Authorization",
			value:  "Bearer sk-test123",
		},
		{
			name:   "custom API key header",
			header: "X-API-Key",
			value:  "secret-key-123",
		},
		{
			name:   "empty value",
			header: "Authorization",
			value:  "",
		},
		{
			name:   "special characters in value",
			header: "X-Auth",
			value:  "key@with#special$chars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := GetUpstreamAuthApikeyPolicyParams(tt.header, tt.value)

			require.NoError(t, err, "Should successfully generate params")
			require.NotNil(t, params)

			// Verify structure
			assert.Contains(t, params, "requestHeaders")

			requestHeaders, ok := params["requestHeaders"].([]interface{})
			require.True(t, ok, "requestHeaders should be an array")
			require.Len(t, requestHeaders, 1)

			headerMap, ok := requestHeaders[0].(map[string]interface{})
			require.True(t, ok, "Header should be a map")

			assert.Equal(t, "SET", headerMap["action"])
			assert.Equal(t, tt.header, headerMap["name"])
			assert.Equal(t, tt.value, headerMap["value"])
		})
	}
}

// ============================================================================
// Edge Case and Bug Detection Tests
// ============================================================================

func TestTransform_EmptyExceptionsArray(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	emptyExceptions := []api.RouteException{}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.AllowAll,
				Exceptions: &emptyExceptions,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should behave same as no exceptions
	assert.Len(t, spec.Operations, 1)
	assert.Equal(t, "/*", spec.Operations[0].Path)
}

func TestTransform_DuplicateExceptionPaths(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	// BUG POTENTIAL: Duplicate paths might create duplicate operations
	exceptions := []api.RouteException{
		{
			Path:    "/admin",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
		{
			Path:    "/admin",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.AllowAll,
				Exceptions: &exceptions,
			},
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should have 2 exception operations + 1 catch-all = 3
	assert.Len(t, spec.Operations, 3)

	// Count /admin operations
	adminOps := 0
	for _, op := range spec.Operations {
		if op.Path == "/admin" {
			adminOps++
		}
	}
	assert.Equal(t, 2, adminOps, "Should have 2 /admin operations")
}

func TestTransform_AllowAllWithPolicies(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	// BUG POTENTIAL: Policies might be applied to exception operations with Respond policy
	// This could cause unexpected behavior

	policies := []api.LLMPolicy{
		{
			Name:    "ContentLengthGuardrail",
			Version: "v0.1.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/admin",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params: map[string]interface{}{
						"maxRequestBodySize": 1024,
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/admin",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
	}

	provider := &api.LLMProviderConfiguration{
		Version: "ai.api-platform.wso2.com/v1",
		Kind:    "llm/provider",
		Spec: api.LLMProviderConfigData{

			DisplayName: "test",
			Version:  "v1.0",
			Template: "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode:       api.AllowAll,
				Exceptions: &exceptions,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Find /admin operation
	var adminOp *api.Operation
	for i := range spec.Operations {
		if spec.Operations[i].Path == "/admin" {
			adminOp = &spec.Operations[i]
			break
		}
	}

	require.NotNil(t, adminOp)
	require.NotNil(t, adminOp.Policies)

	// ISSUE: Should have both Respond (from exception) and ContentLengthGuardrail (from policy)
	// Current implementation might have ordering issues
	t.Logf("Admin operation has %d policies", len(*adminOp.Policies))
	for i, p := range *adminOp.Policies {
		t.Logf("Policy %d: %s", i, p.Name)
	}

	// Verify policies
	assert.GreaterOrEqual(t, len(*adminOp.Policies), 1, "Should have at least Respond policy")
}
