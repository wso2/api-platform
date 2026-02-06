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

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// loadDummyConfig creates a dummy router configuration
func loadDummyConfig() config.RouterConfig {
	return config.RouterConfig{
		EventGateway: config.EventGatewayConfig{
			Enabled:       true,
			WebSubHubURL:  "http://host.docker.internal",
			WebSubHubPort: 9098,
		},
		AccessLogs: config.AccessLogsConfig{
			Enabled: true,
			Format:  "json",
			JSONFields: map[string]string{
				"start_time":            "%START_TIME%",
				"method":                "%REQ(:METHOD)%",
				"path":                  "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%",
				"protocol":              "%PROTOCOL%",
				"response_code":         "%RESPONSE_CODE%",
				"response_flags":        "%RESPONSE_FLAGS%",
				"response_flags_long":   "%RESPONSE_FLAGS_LONG%",
				"bytes_received":        "%BYTES_RECEIVED%",
				"bytes_sent":            "%BYTES_SENT%",
				"duration":              "%DURATION%",
				"upstream_service_time": "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%",
				"x_forwarded_for":       "%REQ(X-FORWARDED-FOR)%",
				"user_agent":            "%REQ(USER-AGENT)%",
				"request_id":            "%REQ(X-REQUEST-ID)%",
				"authority":             "%REQ(:AUTHORITY)%",
				"upstream_host":         "%UPSTREAM_HOST%",
				"upstream_cluster":      "%UPSTREAM_CLUSTER%",
			},
			TextFormat: "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" " +
				"%RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% %BYTES_SENT% %DURATION% " +
				"\"%REQ(X-FORWARDED-FOR)%\" \"%REQ(USER-AGENT)%\" \"%REQ(X-REQUEST-ID)%\" " +
				"\"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\"\n",
		},
		ListenerPort: 8080,
		HTTPSEnabled: false,
		HTTPSPort:    8443,
		DownstreamTLS: config.DownstreamTLS{
			CertPath:               "./listener-certs/server.crt",
			KeyPath:                "./listener-certs/server.key",
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_3",
			Ciphers:                "ECDHE-ECDSA-AES128-GCM-SHA256,ECDHE-RSA-AES128-GCM-SHA256,ECDHE-ECDSA-AES128-SHA,ECDHE-RSA-AES128-SHA,AES128-GCM-SHA256,AES128-SHA,ECDHE-ECDSA-AES256-GCM-SHA384,ECDHE-RSA-AES256-GCM-SHA384,ECDHE-ECDSA-AES256-SHA,ECDHE-RSA-AES256-SHA,AES256-GCM-SHA384,AES256-SHA",
		},
		GatewayHost: "localhost",
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "*"},
			Sandbox: config.VHostEntry{Default: "sandbox-*"},
		},
	}
}

// setupTestTransformer creates a transformer with a mock store containing test templates
func setupTestTransformer(t *testing.T) (*LLMProviderTransformer, *storage.ConfigStore) {
	store := storage.NewConfigStore()

	// Add test templates
	openAITemplate := &models.StoredLLMProviderTemplate{
		ID: "openai-template-id",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
			Kind:       "LlmProviderTemplate",
			Metadata:   api.Metadata{Name: "openai"},
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
	cfg := loadDummyConfig()
	transformer := NewLLMProviderTransformer(store, &cfg, newTestPolicyVersionResolver())
	return transformer, store
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewLLMProviderTransformer(t *testing.T) {
	store := storage.NewConfigStore()
	cfg := loadDummyConfig()
	transformer := NewLLMProviderTransformer(store, &cfg, newTestPolicyVersionResolver())

	assert.NotNil(t, transformer, "Transformer should not be nil")
	assert.NotNil(t, transformer.store, "Store should not be nil")
}

// ============================================================================
// Basic Transformation Tests
// ============================================================================

func TestTransform_MinimalProvider(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
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
	assert.Equal(t, api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1, result.ApiVersion)

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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "full-provider",
			Version:     "v1.0",
			Context:     stringPtr("/openai"),
			Vhost:       stringPtr("api.openai.com"),
			Template:    "openai",
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
	assert.Equal(t, "modify-headers", authPolicy.Name)
	assert.Equal(t, testModifyHeadersVersion, authPolicy.Version)
	assert.NotNil(t, authPolicy.Params)
}

// ============================================================================
// Invalid Input Tests
// ============================================================================

func TestTransform_NonExistentTemplate(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "nonexistent-template",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Context:     nil, // No context provided
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
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       "LlmProvider",
				Metadata:   api.Metadata{Name: "openai-provider"},
				Spec: api.LLMProviderConfigData{
					DisplayName: "test",
					Version:     "v1.0",
					Template:    "openai",
					Context:     stringPtr(tt.context),
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Vhost:       nil,
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Vhost:       stringPtr("api.mycompany.com"),
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
	assert.Equal(t, testModifyHeadersVersion, policy.Version)
	require.NotNil(t, policy.Params)

	// Verify policy params contain header and value
	params := *policy.Params
	assert.Contains(t, params, "requestHeaders")
}

func TestTransform_UnsupportedAuthType(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
	require.Len(t, spec.Operations, len(constants.WILDCARD_HTTP_METHODS))
	for i, op := range spec.Operations {
		assert.Contains(t, constants.WILDCARD_HTTP_METHODS, string(op.Method),
			"Operation %d method should be in WILDCARD_HTTP_METHODS", i)
		assert.Equal(t, "/*", op.Path,
			"Operation %d should have wildcard path", i)
	}
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
	require.Len(t, spec.Operations, 2+len(constants.WILDCARD_HTTP_METHODS))

	// Verify exception operations have respond policy
	foundGET := false
	foundPOST := false
	catchAllCount := 0

	for i, op := range spec.Operations {
		if op.Path == "/admin" && op.Method == api.OperationMethod("GET") {
			foundGET = true
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
			policy := (*op.Policies)[0]
			assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, policy.Name)
			assert.Equal(t, testRespondVersion, policy.Version)
		}
		if op.Path == "/admin" && op.Method == api.OperationMethod("POST") {
			foundPOST = true
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
			policy := (*op.Policies)[0]
			assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, policy.Name)
			assert.Equal(t, testRespondVersion, policy.Version)
		}
		if op.Path == "/*" {
			assert.Contains(t, constants.WILDCARD_HTTP_METHODS, string(op.Method),
				"Operation %d method should be in WILDCARD_HTTP_METHODS", i)
			assert.Equal(t, "/*", op.Path,
				"Operation %d should have wildcard path", i)
			catchAllCount++

			// Catch-all should not have policies
			if op.Policies != nil {
				assert.Len(t, *op.Policies, 0)
			}
		}

	}

	assert.True(t, foundGET, "Should have GET /admin operation")
	assert.True(t, foundPOST, "Should have POST /admin operation")
	assert.Equal(t, catchAllCount, len(constants.WILDCARD_HTTP_METHODS), "Should have catch-all operation")
}

func TestTransform_AllowAll_WithSingleExceptionWithWildCardMethod(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/admin",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Should have 12
	require.Len(t, spec.Operations, 2*len(constants.WILDCARD_HTTP_METHODS))

	// Verify exception operations have respond policy
	foundWildCardCount := 0
	foundCatchCount := 0

	for _, op := range spec.Operations {
		if op.Path == "/admin" {
			foundWildCardCount++
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
			policy := (*op.Policies)[0]
			assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, policy.Name)
			assert.Equal(t, testRespondVersion, policy.Version)
		}
		if op.Path == "/*" {
			foundCatchCount++
			// Catch-all should not have policies
			if op.Policies != nil {
				assert.Len(t, *op.Policies, 0)
			}
		}
	}

	assert.Equal(t, foundWildCardCount, len(constants.WILDCARD_HTTP_METHODS), "Should have * /admin operation")
	assert.Equal(t, foundCatchCount, len(constants.WILDCARD_HTTP_METHODS), "Should have catch-all operation")
}

func TestTransform_AllowAll_WithSingleExceptionWithWildCardResource(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/admin/*",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Should have 12
	require.Len(t, spec.Operations, 2*len(constants.WILDCARD_HTTP_METHODS))

	// Verify exception operations have respond policy
	foundWildCardCount := 0
	foundCatchCount := 0

	for _, op := range spec.Operations {
		if op.Path == "/admin/*" {
			foundWildCardCount++
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
			policy := (*op.Policies)[0]
			assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, policy.Name)
			assert.Equal(t, testRespondVersion, policy.Version)
		}
		if op.Path == "/*" {
			foundCatchCount++
			// Catch-all should not have policies
			if op.Policies != nil {
				assert.Len(t, *op.Policies, 0)
			}
		}
	}

	assert.Equal(t, foundWildCardCount, len(constants.WILDCARD_HTTP_METHODS), "Should have * /admin operation")
	assert.Equal(t, foundCatchCount, len(constants.WILDCARD_HTTP_METHODS), "Should have catch-all operation")
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Should have 1 + 3 = 4 exception operations + 6 catch-all (one per HTTP method) = 10 total
	require.Len(t, spec.Operations, 10)

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
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllOps, "Should have catch-all operations for all HTTP methods")
}

// ============================================================================
// Access Control - Deny All Mode Tests
// ============================================================================

func TestTransform_DenyAll_NoExceptions(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Exception operations in deny_all mode should NOT have respond policy
	if spec.Operations[0].Policies != nil {
		assert.Len(t, *spec.Operations[0].Policies, 0, "Deny all exceptions should not have respond policy")
	}
}

func TestTransform_DenyAll_WithSingleExceptionWithWildCardMethod(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/v1/chat/completions",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Verify all operations have the correct path and methods match WILDCARD_HTTP_METHODS
	for i, op := range spec.Operations {
		assert.Equal(t, "/v1/chat/completions", op.Path, "Operation %d should have wildcard path", i)

		// Verify the method is one of the wildcard methods
		assert.Contains(t, constants.WILDCARD_HTTP_METHODS, string(op.Method),
			"Operation %d method %s should be in WILDCARD_HTTP_METHODS", i, op.Method)

		// Exception operations in deny_all mode should NOT have respond policy
		if op.Policies != nil {
			assert.Len(t, *op.Policies, 0, "Operation %d (%s) should not have respond policy", i, op.Method)
		}
	}

	// Additionally, verify all WILDCARD_HTTP_METHODS are present
	foundMethods := make(map[string]bool)
	for _, op := range spec.Operations {
		foundMethods[string(op.Method)] = true
	}

	for _, expectedMethod := range constants.WILDCARD_HTTP_METHODS {
		assert.True(t, foundMethods[expectedMethod], "Expected method %s should be present in operations", expectedMethod)
	}

	// Exception operations in deny_all mode should NOT have respond policy
	if spec.Operations[0].Policies != nil {
		assert.Len(t, *spec.Operations[0].Policies, 0, "Deny all exceptions should not have respond policy")
	}
}

func TestTransform_DenyAll_WithSingleExceptionWithWildCardResource(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/v1/chat/*",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Should have len(constants.WILDCARD_HTTP_METHODS) operations (one for each method with wildcard path)
	require.Len(t, spec.Operations, len(constants.WILDCARD_HTTP_METHODS))

	// Verify all operations have the correct path and methods match WILDCARD_HTTP_METHODS
	for i, op := range spec.Operations {
		assert.Equal(t, "/v1/chat/*", op.Path, "Operation %d should have wildcard path", i)

		// Verify the method is one of the wildcard methods
		assert.Contains(t, constants.WILDCARD_HTTP_METHODS, string(op.Method),
			"Operation %d method %s should be in WILDCARD_HTTP_METHODS", i, op.Method)

		// Exception operations in deny_all mode should NOT have respond policy
		if op.Policies != nil {
			assert.Len(t, *op.Policies, 0, "Operation %d (%s) should not have respond policy", i, op.Method)
		}
	}

	// Additionally, verify all WILDCARD_HTTP_METHODS are present
	foundMethods := make(map[string]bool)
	for _, op := range spec.Operations {
		foundMethods[string(op.Method)] = true
	}

	for _, expectedMethod := range constants.WILDCARD_HTTP_METHODS {
		assert.True(t, foundMethods[expectedMethod], "Expected method %s should be present in operations", expectedMethod)
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
// Policy Application Tests with Access Control Deny All
// ============================================================================

func TestTransform_WithSinglePolicy(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "content-length-guardrail",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
	assert.Equal(t, "content-length-guardrail", policy.Name)
	assert.Equal(t, "v0.1.0", policy.Version)
	require.NotNil(t, policy.Params)

	params := *policy.Params
	assert.Equal(t, 10240.0, params["maxRequestBodySize"])
}

func TestTransform_WithMultiplePoliciesSameRoute(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "content-length-guardrail",
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
			Name:    "regex-guardrail",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
	assert.Equal(t, "content-length-guardrail", policyList[0].Name)
	assert.Equal(t, "regex-guardrail", policyList[1].Name)
}

func TestTransform_PolicyOnDifferentRoutes(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "content-length-guardrail",
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
			Name:    "regex-guardrail",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
			assert.Equal(t, "content-length-guardrail", policy.Name)
		} else if op.Path == "/v1/embeddings" {
			assert.Equal(t, "regex-guardrail", policy.Name)
		}
	}
}

// This tests with specific exception methods and policy wildcard method
func TestTransform_PolicyOnWildcardMethod_1(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "modify-headers",
			Version: "v0.1.0",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
		// Each operation should have exactly one policy
		require.NotNil(t, op.Policies, "Operation %s %s should have policies", op.Method, op.Path)
		require.Len(t, *op.Policies, 1, "Operation %s %s should have exactly 1 policy", op.Method, op.Path)

		// The policy should be modify-headers
		policy := (*op.Policies)[0]
		assert.Equal(t, "modify-headers", policy.Name, "Operation %s %s should have modify-headers policy", op.Method, op.Path)
		assert.Equal(t, "v0.1.0", policy.Version, "Operation %s %s should have correct policy version", op.Method, op.Path)

		// Verify the policy params are set correctly
		require.NotNil(t, policy.Params, "Operation %s %s policy should have params", op.Method, op.Path)
		assert.Contains(t, *policy.Params, "requestHeaders", "Operation %s %s policy should have requestHeaders param", op.Method, op.Path)
	}
}

// This tests with wildcard exception method and policy wildcard method
func TestTransform_PolicyOnWildcardMethod_2(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "modify-headers",
			Version: "v0.1.0",
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
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Wildcard methods expand to all standard HTTP methods
	require.Len(t, spec.Operations, len(constants.WILDCARD_HTTP_METHODS))

	// Verify all operations have the policy attached
	for i, op := range spec.Operations {
		assert.Equal(t, "/v1/chat/completions", op.Path, "Operation %d should have correct path", i)

		// Verify the method is one of the wildcard methods
		assert.Contains(t, constants.WILDCARD_HTTP_METHODS, string(op.Method),
			"Operation %d method %s should be in WILDCARD_HTTP_METHODS", i, op.Method)

		// Each operation should have exactly one policy
		require.NotNil(t, op.Policies, "Operation %s %s should have policies", op.Method, op.Path)
		require.Len(t, *op.Policies, 1, "Operation %s %s should have exactly 1 policy", op.Method, op.Path)

		// The policy should be modify-headers
		policy := (*op.Policies)[0]
		assert.Equal(t, "modify-headers", policy.Name, "Operation %s %s should have modify-headers policy", op.Method, op.Path)
		assert.Equal(t, "v0.1.0", policy.Version, "Operation %s %s should have correct policy version", op.Method, op.Path)

		// Verify the policy params are set correctly
		require.NotNil(t, policy.Params, "Operation %s %s policy should have params", op.Method, op.Path)
		assert.Contains(t, *policy.Params, "requestHeaders", "Operation %s %s policy should have requestHeaders param", op.Method, op.Path)
	}
}

func TestTransform_PolicyOnNonExistentRoute(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	// Policy for route that doesn't exist in operations
	policies := []api.LLMPolicy{
		{
			Name:    "content-length-guardrail",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Should have 1 exception + 6 catch-all operations (one per HTTP method) = 7 total
	require.Len(t, spec.Operations, 7)

	// Exception operation should have respond policy
	foundException := false
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/admin" {
			foundException = true
			require.NotNil(t, op.Policies)
			assert.Len(t, *op.Policies, 1)
			assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*op.Policies)[0].Name)
		} else if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.True(t, foundException, "Should have exception operation")
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have catch-all operations for all HTTP methods")
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Should behave same as no exceptions - 6 catch-all operations (one per HTTP method)
	assert.Len(t, spec.Operations, len(constants.WILDCARD_HTTP_METHODS))
	for _, op := range spec.Operations {
		assert.Equal(t, "/*", op.Path)
	}
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Should have 2 exception operations + 6 catch-all (one per HTTP method) = 8 total
	assert.Len(t, spec.Operations, 8)

	// Count /admin operations
	adminOps := 0
	catchAllOps := 0
	for _, op := range spec.Operations {
		if op.Path == "/admin" {
			adminOps++
		} else if op.Path == "/*" {
			catchAllOps++
		}
	}
	assert.Equal(t, 2, adminOps, "Should have 2 /admin operations")
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllOps, "Should have catch-all operations for all HTTP methods")
}

func TestTransform_AllowAllWithPolicies(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	// BUG POTENTIAL: Policies might be applied to exception operations with respond policy
	// This could cause unexpected behavior

	policies := []api.LLMPolicy{
		{
			Name:    "content-length-guardrail",
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
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// ISSUE: Should have both respond (from exception) and content-length-guardrail (from policy)
	// Current implementation might have ordering issues
	t.Logf("Admin operation has %d policies", len(*adminOp.Policies))
	for i, p := range *adminOp.Policies {
		t.Logf("Policy %d: %s", i, p.Name)
	}

	// Verify policies
	assert.GreaterOrEqual(t, len(*adminOp.Policies), 1, "Should have at least respond policy")
}

// ============================================================================
// API-Level Policies (Root Wildcard /*) Tests - Section 10.3.1
// ============================================================================

func TestTransform_APILevelPolicy_AllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "GlobalRateLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"rps": 100,
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Verify no API-level policies exist
	assert.Nil(t, spec.Policies, "No API-level policies should exist")

	// Verify /* policy is attached to individual operations for all HTTP methods
	require.Len(t, spec.Operations, len(constants.WILDCARD_HTTP_METHODS), "Should have 6 catch-all operations (all HTTP methods)")

	for _, op := range spec.Operations {
		assert.Equal(t, "/*", op.Path)
		require.NotNil(t, op.Policies, "Catch-all operation should have policies")
		require.Len(t, *op.Policies, 1, "Should have exactly 1 policy attached")

		policy := (*op.Policies)[0]
		assert.Equal(t, "GlobalRateLimit", policy.Name)
		assert.Equal(t, "v1.0.0", policy.Version)
		require.NotNil(t, policy.Params)
		assert.Equal(t, 100.0, (*policy.Params)["rps"])
	}
}

func TestTransform_APILevelPolicy_DenyAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "GlobalAuth",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"required": true,
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/health",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Verify no API-level policies exist (/* policy now attaches to operations)
	assert.Nil(t, spec.Policies, "API-level policies should not exist")

	// Verify operation exists from access control exception
	require.Len(t, spec.Operations, 1, "Should have 1 operation from exception")

	op := spec.Operations[0]
	assert.Equal(t, "/health", op.Path)
	assert.Equal(t, api.OperationMethod("GET"), op.Method)

	// Verify GlobalAuth policy is attached to the operation
	require.NotNil(t, op.Policies, "Operation should have policies attached")
	require.Len(t, *op.Policies, 1, "Operation should have exactly 1 policy")

	attachedPolicy := (*op.Policies)[0]
	assert.Equal(t, "GlobalAuth", attachedPolicy.Name)
	assert.Equal(t, "v1.0.0", attachedPolicy.Version)
}

func TestTransform_MultipleAPILevelPolicies_AllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "GlobalAuth",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"required": true,
					},
				},
			},
		},
		{
			Name:    "GlobalRateLimit",
			Version: "v2.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"rps": 200,
					},
				},
			},
		},
		{
			Name:    "GlobalLogging",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"level": "info",
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Verify no API-level policies exist
	assert.Nil(t, spec.Policies, "No API-level policies should exist")

	// Verify 6 catch-all operations exist (one for each HTTP method)
	require.Len(t, spec.Operations, 6, "Should have 6 catch-all operations")

	// Verify all operations have all 3 policies attached
	for _, op := range spec.Operations {
		assert.Equal(t, "/*", op.Path)
		require.NotNil(t, op.Policies, "Operation should have policies")
		require.Len(t, *op.Policies, 3, "Each operation should have 3 policies")

		policyNames := make(map[string]bool)
		for _, policy := range *op.Policies {
			policyNames[policy.Name] = true
		}

		assert.True(t, policyNames["GlobalAuth"], "Should have GlobalAuth")
		assert.True(t, policyNames["GlobalRateLimit"], "Should have GlobalRateLimit")
		assert.True(t, policyNames["GlobalLogging"], "Should have GlobalLogging")
	}
}

func TestTransform_UpstreamAuth_Plus_APILevelPolicy_AllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "GlobalRateLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"rps": 100,
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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
				Mode: api.AllowAll,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Verify only upstream auth policy at API level
	require.NotNil(t, spec.Policies, "API-level policies should exist")
	require.Len(t, *spec.Policies, 1, "Should have only 1 API-level policy (auth)")
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, (*spec.Policies)[0].Name)

	// Verify 6 catch-all operations with GlobalRateLimit attached
	require.Len(t, spec.Operations, 6, "Should have 6 catch-all operations")

	for _, op := range spec.Operations {
		assert.Equal(t, "/*", op.Path)
		require.NotNil(t, op.Policies, "Operation should have policies")
		require.Len(t, *op.Policies, 1, "Each operation should have GlobalRateLimit attached")

		policy := (*op.Policies)[0]
		assert.Equal(t, "GlobalRateLimit", policy.Name)
		assert.Equal(t, "v1.0.0", policy.Version)
		require.NotNil(t, policy.Params)
		assert.Equal(t, 100.0, (*policy.Params)["rps"])
	}
}

func TestTransform_UpstreamAuth_Plus_APILevelPolicy_DenyAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "GlobalCORS",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"GET", "POST", "DELETE"},
					Params: map[string]interface{}{
						"allowOrigin": "*",
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/api/v1/*",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: stringPtr("X-API-Key"),
					Value:  stringPtr("secret123"),
				},
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

	// Verify BOTH auth policy and /* policy are in spec.Policies
	require.NotNil(t, spec.Policies, "API-level policies should exist")
	assert.Len(t, *spec.Policies, 1, "Should have upstream auth policy only")
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, (*spec.Policies)[0].Name)

	// Verify operations exist for all HTTP methods on /api/v1/*
	require.Len(t, spec.Operations, len(constants.WILDCARD_HTTP_METHODS),
		"Should have operations for all HTTP methods")

	methodsFound := make(map[string]bool)
	for _, op := range spec.Operations {
		assert.Equal(t, "/api/v1/*", op.Path, "All operations should be for /api/v1/*")
		methodsFound[string(op.Method)] = true

		// Check which methods have GlobalCORS policy attached
		if op.Method == "GET" || op.Method == "POST" || op.Method == "DELETE" {
			require.NotNil(t, op.Policies, "GET, POST, DELETE should have policies")
			require.Len(t, *op.Policies, 1, "These methods should have exactly 1 policy")
			assert.Equal(t, "GlobalCORS", (*op.Policies)[0].Name)
		} else {
			// PUT, PATCH, OPTIONS should NOT have GlobalCORS policy
			assert.Nil(t, op.Policies, "Other methods should not have policies attached")
		}
	}

	// Verify all HTTP methods are present
	expectedMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "OPTIONS": true}
	assert.Equal(t, expectedMethods, methodsFound, "All HTTP methods should be present")
}

func TestTransform_APILevel_Plus_OperationLevel_Policies_AllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "TokenLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxTokens": 1000,
					},
				},
			},
		},
		{
			Name:    "RateLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/embeddings",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"rps": 50,
					},
				},
			},
		},
		{
			Name:    "GlobalAuth",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"required": true,
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Verify no API-level policies exist
	assert.Nil(t, spec.Policies, "No API-level policies should exist")

	// Verify 8 total operations (6 catch-all + 2 specific)
	require.Len(t, spec.Operations, 8, "Should have 8 total operations")

	// Verify catch-all operations (/*) have GlobalAuth
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			require.NotNil(t, op.Policies, "Catch-all operation should have policies")
			require.Len(t, *op.Policies, 1, "Catch-all should have 1 policy")
			assert.Equal(t, "GlobalAuth", (*op.Policies)[0].Name)
		}
	}

	// Verify /chat/completions POST has GlobalAuth and TokenLimit
	chatOp := findOperation(spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, chatOp, "Should have /chat/completions POST operation")
	require.NotNil(t, chatOp.Policies, "Operation should have policies")
	require.Len(t, *chatOp.Policies, 2, "Should have 2 policies")

	chatPolicies := make(map[string]bool)
	for _, pol := range *chatOp.Policies {
		chatPolicies[pol.Name] = true
	}
	assert.True(t, chatPolicies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, chatPolicies["TokenLimit"], "Should have TokenLimit")

	// Verify /embeddings POST has GlobalAuth and RateLimit
	embeddingsOp := findOperation(spec.Operations, "/embeddings", "POST")
	require.NotNil(t, embeddingsOp, "Should have /embeddings POST operation")
	require.NotNil(t, embeddingsOp.Policies, "Operation should have policies")
	require.Len(t, *embeddingsOp.Policies, 2, "Should have 2 policies")

	embeddingsPolicies := make(map[string]bool)
	for _, pol := range *embeddingsOp.Policies {
		embeddingsPolicies[pol.Name] = true
	}
	assert.True(t, embeddingsPolicies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, embeddingsPolicies["RateLimit"], "Should have RateLimit")
}

func TestTransform_APILevel_Plus_OperationLevel_Policies_DenyAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "GlobalMetrics",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
		{
			Name:    "SpecificGuardrail",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"check": true,
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/chat/completions",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
		{
			Path:    "/health",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Verify no API-level policies exist
	assert.Nil(t, spec.Policies, "No API-level policies should exist")

	// Verify exactly 2 operations exist
	require.Len(t, spec.Operations, 2, "Should have exactly 2 operations")

	// Verify /chat/completions POST operation has both GlobalMetrics and SpecificGuardrail
	chatOp := findOperation(spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, chatOp, "/chat/completions POST operation should exist")
	require.NotNil(t, chatOp.Policies, "Operation should have policies")
	require.Len(t, *chatOp.Policies, 2, "Should have 2 policies attached")

	policyNames := make(map[string]bool)
	for _, pol := range *chatOp.Policies {
		policyNames[pol.Name] = true
	}
	assert.True(t, policyNames["GlobalMetrics"], "GlobalMetrics should be attached")
	assert.True(t, policyNames["SpecificGuardrail"], "SpecificGuardrail should be attached")

	// Verify /health GET operation has only GlobalMetrics
	healthOp := findOperation(spec.Operations, "/health", "GET")
	require.NotNil(t, healthOp, "/health GET operation should exist")
	require.NotNil(t, healthOp.Policies, "Operation should have policies")
	require.Len(t, *healthOp.Policies, 1, "Should have 1 policy attached")
	assert.Equal(t, "GlobalMetrics", (*healthOp.Policies)[0].Name)
}

func TestTransform_APILevelPolicy_WildcardMethods_AllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "GlobalPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"}, // Wildcard methods
					Params: map[string]interface{}{
						"setting": "value",
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Verify no API-level policies exist
	assert.Nil(t, spec.Policies, "No API-level policies should exist")

	// Verify 6 catch-all operations exist (one for each HTTP method)
	require.Len(t, spec.Operations, 6, "Should have 6 catch-all operations")

	// Verify all catch-all operations have GlobalPolicy attached
	for _, op := range spec.Operations {
		assert.Equal(t, "/*", op.Path)
		require.NotNil(t, op.Policies, "Operation should have policies")
		require.Len(t, *op.Policies, 1, "Should have 1 policy attached")

		policy := (*op.Policies)[0]
		assert.Equal(t, "GlobalPolicy", policy.Name)
		assert.Equal(t, "v1.0.0", policy.Version)
		require.NotNil(t, policy.Params)
		assert.Equal(t, "value", (*policy.Params)["setting"])
	}
}

func TestTransform_NoAPILevelPolicy_OperationLevelOnly_AllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "SpecificPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxTokens": 1000,
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)

	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Verify NO API-level policies (since no /* policy)
	if spec.Policies != nil {
		assert.Len(t, *spec.Policies, 0, "Should have no API-level policies")
	}

	// Verify operations count
	assert.Len(t, spec.Operations, 7, "Should have 2 operations (catch-all for all methods + POST)")

	// Verify operation-level policy exists
	chatOp := findOperation(spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, chatOp)
	require.NotNil(t, chatOp.Policies)
	assert.Len(t, *chatOp.Policies, 1)
	assert.Equal(t, "SpecificPolicy", (*chatOp.Policies)[0].Name)
}

// ============================================================================
// Exception Precedence Tests (AllowAll Mode) - P0 Critical
// ============================================================================

func TestTransform_ExceptionPrecedence_ExactMatch(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/admin/delete",
			Methods: []api.RouteExceptionMethods{api.DELETE},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "Logging",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/admin/delete",
					Methods: []api.LLMPolicyPathMethods{"DELETE"},
					Params: map[string]interface{}{
						"level": "info",
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Find the admin/delete DELETE operation
	op := findOperation(spec.Operations, "/admin/delete", "DELETE")
	require.NotNil(t, op, "admin/delete DELETE operation should exist")

	// Should have deny policy only, NOT user policy
	require.NotNil(t, op.Policies)
	assert.Len(t, *op.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*op.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/*" {
			catchAllCount++
			// Catch-all should not have policies
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 0, "Catch-all should not have policies")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have catch-all operations for all HTTP methods")
}

func TestTransform_ExceptionPrecedence_WildcardCoverage_InternalPath(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/internal/*",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "TokenLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/internal/health",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params: map[string]interface{}{
						"maxTokens": 100,
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Find internal/health GET operation
	op := findOperation(spec.Operations, "/internal/health", "GET")
	require.Nil(t, op, "internal/health GET operation should exist")

	// Verify internal/* wildcard operations also exist with deny policy
	wildcardOp := findOperation(spec.Operations, "/internal/*", "GET")
	require.NotNil(t, wildcardOp, "internal/* GET operation should exist")
	require.NotNil(t, wildcardOp.Policies)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*wildcardOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/*" {
			catchAllCount++
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 0, "Catch-all should not have policies")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have catch-all operations")
}

func TestTransform_ExceptionPrecedence_NestedWildcards(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/api/*",
			Methods: []api.RouteExceptionMethods{"*"},
		},
		{
			Path:    "/api/admin/*",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "Logging",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/api/users",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params: map[string]interface{}{
						"level": "info",
					},
				},
			},
		},
		{
			Name:    "Authorization",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/api/admin/delete",
					Methods: []api.LLMPolicyPathMethods{"DELETE"},
					Params: map[string]interface{}{
						"required": true,
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Both paths should be blocked by their respective wildcard exceptions
	usersOp := findOperation(spec.Operations, "/api/users", "GET")
	require.Nil(t, usersOp, "api/users GET operation should exist")

	adminOp := findOperation(spec.Operations, "/api/admin/delete", "DELETE")
	require.Nil(t, adminOp, "api/admin/delete DELETE operation should exist")

	// Both paths should be blocked by their respective wildcard exceptions
	apiWildOp := findOperation(spec.Operations, "/api/*", "GET")
	require.NotNil(t, apiWildOp, "api/* GET operation should exist")
	require.NotNil(t, apiWildOp.Policies)
	assert.Len(t, *apiWildOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*apiWildOp.Policies)[0].Name)

	apiAdminWildOp := findOperation(spec.Operations, "/api/admin/*", "DELETE")
	require.NotNil(t, apiAdminWildOp, "api/admin/* DELETE operation should exist")
	require.NotNil(t, apiAdminWildOp.Policies)
	assert.Len(t, *apiAdminWildOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*apiAdminWildOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/*" {
			catchAllCount++
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 0, "Catch-all should not have policies")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have catch-all operations")
}

func TestTransform_ExceptionPrecedence_PolicyAllowedWhenNotCovered(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/admin/*",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "RateLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"rps": 10,
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// chat/completions is NOT covered by admin/*, so policy should attach
	op := findOperation(spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, op, "chat/completions POST operation should exist")
	require.NotNil(t, op.Policies)
	require.Len(t, *op.Policies, 1, "Should have user policy")

	// Verify it's the user policy, not deny policy
	assert.Equal(t, "RateLimit", (*op.Policies)[0].Name)
	assert.Equal(t, "v1.0.0", (*op.Policies)[0].Version)

	// Verify exception operations exist
	adminExceptionCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/admin/*" {
			adminExceptionCount++
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 1, "Should have block policy")
				adminWildOp := operation
				require.NotNil(t, adminWildOp, "admin/* block operation should exist")
				require.NotNil(t, adminWildOp.Policies)
				assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminWildOp.Policies)[0].Name)
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), adminExceptionCount, "Should have all operations")

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/*" {
			catchAllCount++
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 0, "Catch-all should not have policies")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have catch-all operations")
}

func TestTransform_ExceptionPrecedence_MultipleOverlappingExceptions(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/internal/*",
			Methods: []api.RouteExceptionMethods{"GET", "POST"},
		},
		{
			Path:    "/internal/health",
			Methods: []api.RouteExceptionMethods{"GET"},
		},
		{
			Path:    "/internal/metrics",
			Methods: []api.RouteExceptionMethods{"POST"},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "Monitoring",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/internal/health",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params: map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
		{
			Name:    "Logging",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/internal/metrics",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"level": "debug",
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// All internal paths should be denied, policies should NOT attach
	healthOp := findOperation(spec.Operations, "/internal/health", "GET")
	require.NotNil(t, healthOp, "internal/health GET operation should exist")
	require.NotNil(t, healthOp.Policies)
	assert.Len(t, *healthOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*healthOp.Policies)[0].Name)

	metricsOp := findOperation(spec.Operations, "/internal/metrics", "POST")
	require.NotNil(t, metricsOp, "internal/metrics POST operation should exist")
	require.NotNil(t, metricsOp.Policies)
	assert.Len(t, *metricsOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*metricsOp.Policies)[0].Name)

	// Verify internal/* wildcard operations also exist with deny policy
	internalGetOp := findOperation(spec.Operations, "/internal/*", "GET")
	require.NotNil(t, internalGetOp, "internal/* GET operation should exist")
	require.NotNil(t, internalGetOp.Policies)
	assert.Len(t, *internalGetOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalGetOp.Policies)[0].Name)

	internalPostOp := findOperation(spec.Operations, "/internal/*", "POST")
	require.NotNil(t, internalPostOp, "internal/* POST operation should exist")
	require.NotNil(t, internalPostOp.Policies)
	assert.Len(t, *internalPostOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalPostOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/*" {
			catchAllCount++
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 0, "Catch-all should not have policies")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have catch-all operations")
}

func TestTransform_ExceptionPrecedence_WildcardMethodExpansion(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/admin/users",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "GlobalPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/admin/users",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Check all HTTP methods - all should be denied
	httpMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	for _, method := range httpMethods {
		op := findOperation(spec.Operations, "/admin/users", method)
		require.NotNil(t, op, "admin/users %s operation should exist", method)
		require.NotNil(t, op.Policies)
		assert.Len(t, *op.Policies, 1, "Should have only deny policy for %s", method)
		assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*op.Policies)[0].Name,
			"Should be deny policy for %s", method)
	}

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/*" {
			catchAllCount++
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 0, "Catch-all should not have policies")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have catch-all operations")
}

func TestTransform_ExceptionPrecedence_PartialMethodCoverage(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/admin/users",
			Methods: []api.RouteExceptionMethods{"DELETE", "POST"},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "Logging",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/admin/users",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"level": "info",
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// DELETE and POST should be denied (exception precedence)
	deleteOp := findOperation(spec.Operations, "/admin/users", "DELETE")
	require.NotNil(t, deleteOp)
	require.NotNil(t, deleteOp.Policies)
	assert.Len(t, *deleteOp.Policies, 1, "DELETE should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*deleteOp.Policies)[0].Name)

	postOp := findOperation(spec.Operations, "/admin/users", "POST")
	require.NotNil(t, postOp)
	require.NotNil(t, postOp.Policies)
	assert.Len(t, *postOp.Policies, 1, "POST should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*postOp.Policies)[0].Name)

	// GET, PUT, PATCH, OPTIONS should allow user policy
	allowedMethods := []string{"GET", "PUT", "PATCH", "OPTIONS"}
	for _, method := range allowedMethods {
		op := findOperation(spec.Operations, "/admin/users", method)
		require.NotNil(t, op, "admin/users %s operation should exist", method)
		require.NotNil(t, op.Policies)

		// Should have user policy
		foundUserPolicy := false
		for _, policy := range *op.Policies {
			if policy.Name == "Logging" {
				foundUserPolicy = true
				break
			}
		}
		assert.True(t, foundUserPolicy, "User policy should be attached to %s method", method)
	}

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/*" {
			catchAllCount++
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 0, "Catch-all should not have policies")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have catch-all operations")
}

func TestTransform_ExceptionPrecedence_DeepNestedPath(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "/api/*",
			Methods: []api.RouteExceptionMethods{"*"},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "DeepPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/api/v1/resources/items/details",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params: map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Deeply nested path should still be covered by api/* exception
	op := findOperation(spec.Operations, "/api/v1/resources/items/details", "GET")
	require.Nil(t, op, "Deeply nested operation should not exist")

	// Verify all blocked operations exist
	apiOpCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/api/*" {
			apiOpCount++
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 1, "Should have only deny policy")
				assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*operation.Policies)[0].Name)
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), apiOpCount, "Should have catch-all operations")

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/*" {
			catchAllCount++
			if operation.Policies != nil {
				assert.Len(t, *operation.Policies, 0, "Catch-all should not have policies")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have catch-all operations")
}

// ============================================================================
// Combined API-Level + Operation-Level + Auth Policies Tests - Section 10.3.3
// ============================================================================

func TestTransform_Auth_Plus_APILevel_Plus_OperationLevel_AllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "TokenLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxTokens": 1000,
					},
				},
			},
		},
		{
			Name:    "GlobalRateLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"rps": 100,
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/admin/delete",
			Methods: []api.RouteExceptionMethods{"DELETE"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: stringPtr("Authorization"),
					Value:  stringPtr("Bearer secret-token"),
				},
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

	// Verify only auth policy at API level
	require.NotNil(t, spec.Policies, "API-level policies should exist")
	require.Len(t, *spec.Policies, 1, "Should have only auth policy at API level")
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, (*spec.Policies)[0].Name)

	// Verify catch-all operations (6) have GlobalRateLimit
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			require.NotNil(t, op.Policies, "Catch-all operation should have policies")
			require.Len(t, *op.Policies, 1, "Should have GlobalRateLimit")
			assert.Equal(t, "GlobalRateLimit", (*op.Policies)[0].Name)
		}
	}

	// Verify /chat/completions POST has GlobalRateLimit and TokenLimit
	chatOp := findOperation(spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, chatOp, "/chat/completions POST should exist")
	require.NotNil(t, chatOp.Policies, "Operation should have policies")
	require.Len(t, *chatOp.Policies, 2, "Should have 2 policies")

	chatPolicies := make(map[string]bool)
	for _, pol := range *chatOp.Policies {
		chatPolicies[pol.Name] = true
	}
	assert.True(t, chatPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, chatPolicies["TokenLimit"], "Should have TokenLimit")

	// Verify exception operation /admin/delete DELETE has no policies
	adminOp := findOperation(spec.Operations, "/admin/delete", "DELETE")
	require.NotNil(t, adminOp, "/admin/delete DELETE should exist")
	if adminOp.Policies != nil {
		assert.Len(t, *adminOp.Policies, 1, "/admin/delete should have Deny policy only")
		assert.Equal(t, "respond", (*adminOp.Policies)[0].Name)
	}

	// Verify total operations count
	totalOps := 0
	for _, _ = range spec.Operations {
		totalOps++
	}
	// 6 catch-all + 1 /chat/completions POST + 1 /admin/delete DELETE = 8 operations
	assert.Equal(t, 8, totalOps, "Should have 8 total operations")
}

func TestTransform_Auth_Plus_APILevel_Plus_OperationLevel_DenyAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "GlobalAuth",
			Version: "v2.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"required": true,
					},
				},
			},
		},
		{
			Name:    "ContentGuard",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"GET", "POST"},
					Params: map[string]interface{}{
						"maxLength": 5000,
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/chat/completions",
			Methods: []api.RouteExceptionMethods{"POST"},
		},
		{
			Path:    "/health",
			Methods: []api.RouteExceptionMethods{"GET"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: stringPtr("X-API-Key"),
					Value:  stringPtr("secret-key"),
				},
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

	// Verify only upstream auth policy at API level
	require.NotNil(t, spec.Policies, "API-level policies should exist")
	require.Len(t, *spec.Policies, 1, "Should have only auth policy at API level")
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, (*spec.Policies)[0].Name)

	// Verify exactly 2 operations exist (from exceptions)
	require.Len(t, spec.Operations, 2, "Should have exactly 2 operations from exceptions")

	// Verify /chat/completions POST operation has GlobalAuth and ContentGuard
	chatOp := findOperation(spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, chatOp, "/chat/completions POST should exist")
	require.NotNil(t, chatOp.Policies, "Operation should have policies")
	require.Len(t, *chatOp.Policies, 2, "Should have 2 policies attached")

	chatPolicyNames := make(map[string]bool)
	for _, pol := range *chatOp.Policies {
		chatPolicyNames[pol.Name] = true
	}
	assert.True(t, chatPolicyNames["GlobalAuth"], "GlobalAuth should be attached")
	assert.Equal(t, "v2.0.0", (*chatOp.Policies)[0].Version)
	assert.True(t, chatPolicyNames["ContentGuard"], "ContentGuard should be attached")

	// Verify /health GET operation has only GlobalAuth
	healthOp := findOperation(spec.Operations, "/health", "GET")
	require.NotNil(t, healthOp, "/health GET should exist")
	require.NotNil(t, healthOp.Policies, "Operation should have policies")
	require.Len(t, *healthOp.Policies, 1, "Should have 1 policy attached")
	assert.Equal(t, "GlobalAuth", (*healthOp.Policies)[0].Name)
	assert.Equal(t, "v2.0.0", (*healthOp.Policies)[0].Version)
}

func TestTransform_MultipleAPILevelPolicies_Plus_Exceptions_Plus_OperationPolicies_AllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "TokenLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxTokens": 2000,
					},
				},
			},
		},
		{
			Name:    "Audit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/api/models",
					Methods: []api.LLMPolicyPathMethods{"GET", "POST"},
					Params: map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
		{
			Name:    "GlobalRateLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"rps": 100,
					},
				},
			},
		},
		{
			Name:    "GlobalLogging",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"level": "INFO",
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/internal/*",
			Methods: []api.RouteExceptionMethods{"*"},
		},
		{
			Path:    "/admin/users",
			Methods: []api.RouteExceptionMethods{"DELETE", "POST"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
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

	// Verify no API-level policies exist
	assert.Nil(t, spec.Policies, "No API-level policies should exist")

	// Verify catch-all operations (6) have GlobalRateLimit and GlobalLogging
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
			require.NotNil(t, op.Policies, "Catch-all operation should have policies")
			require.Len(t, *op.Policies, 2, "Should have 2 policies")

			policies := make(map[string]bool)
			for _, p := range *op.Policies {
				policies[p.Name] = true
			}
			assert.True(t, policies["GlobalRateLimit"], "Should have GlobalRateLimit")
			assert.True(t, policies["GlobalLogging"], "Should have GlobalLogging")
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have 6 catch-all operations")

	// Verify /internal/* wildcard exception operations exist with deny policy (respond)
	internalWildcardCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/internal/*" {
			internalWildcardCount++
			require.NotNil(t, op.Policies, "Operation should have policies")
			require.Len(t, *op.Policies, 1, "Should have 1 policy (deny policy)")
			assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*op.Policies)[0].Name)
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), internalWildcardCount, "Should have /internal/* for all 6 HTTP methods")

	// Verify /admin/users DELETE exception operation with deny policy (respond)
	adminDeleteOp := findOperation(spec.Operations, "/admin/users", "DELETE")
	require.NotNil(t, adminDeleteOp, "/admin/users DELETE should exist")
	require.NotNil(t, adminDeleteOp.Policies, "Operation should have policies")
	require.Len(t, *adminDeleteOp.Policies, 1, "Should have 1 policy (deny policy)")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminDeleteOp.Policies)[0].Name)

	// Verify /admin/users POST exception operation with deny policy (respond)
	adminPostOp := findOperation(spec.Operations, "/admin/users", "POST")
	require.NotNil(t, adminPostOp, "/admin/users POST should exist")
	require.NotNil(t, adminPostOp.Policies, "Operation should have policies")
	require.Len(t, *adminPostOp.Policies, 1, "Should have 1 policy (deny policy)")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminPostOp.Policies)[0].Name)

	// Verify /chat/completions POST has GlobalRateLimit, GlobalLogging, and TokenLimit
	chatOp := findOperation(spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, chatOp, "/chat/completions POST should exist")
	require.NotNil(t, chatOp.Policies, "Operation should have policies")
	require.Len(t, *chatOp.Policies, 3, "Should have 3 policies")

	chatPolicies := make(map[string]bool)
	for _, p := range *chatOp.Policies {
		chatPolicies[p.Name] = true
	}
	assert.True(t, chatPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, chatPolicies["GlobalLogging"], "Should have GlobalLogging")
	assert.True(t, chatPolicies["TokenLimit"], "Should have TokenLimit")

	// Verify /api/models GET has GlobalRateLimit, GlobalLogging, and Audit
	modelsGetOp := findOperation(spec.Operations, "/api/models", "GET")
	require.NotNil(t, modelsGetOp, "/api/models GET should exist")
	require.NotNil(t, modelsGetOp.Policies, "Operation should have policies")
	require.Len(t, *modelsGetOp.Policies, 3, "Should have 3 policies")

	modelsGetPolicies := make(map[string]bool)
	for _, p := range *modelsGetOp.Policies {
		modelsGetPolicies[p.Name] = true
	}
	assert.True(t, modelsGetPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, modelsGetPolicies["GlobalLogging"], "Should have GlobalLogging")
	assert.True(t, modelsGetPolicies["Audit"], "Should have Audit")

	// Verify /api/models POST has GlobalRateLimit, GlobalLogging, and Audit
	modelsPostOp := findOperation(spec.Operations, "/api/models", "POST")
	require.NotNil(t, modelsPostOp, "/api/models POST should exist")
	require.NotNil(t, modelsPostOp.Policies, "Operation should have policies")
	require.Len(t, *modelsPostOp.Policies, 3, "Should have 3 policies")

	modelsPostPolicies := make(map[string]bool)
	for _, p := range *modelsPostOp.Policies {
		modelsPostPolicies[p.Name] = true
	}
	assert.True(t, modelsPostPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, modelsPostPolicies["GlobalLogging"], "Should have GlobalLogging")
	assert.True(t, modelsPostPolicies["Audit"], "Should have Audit")

	// Verify total operations count
	// 6 catch-all + 6 /internal/* (exceptions) + 2 /admin/users (exceptions) + 1 /chat/completions POST + 1 /api/models GET + 1 /api/models POST = 17 operations
	assert.Len(t, spec.Operations, 17, "Should have 17 total operations")
}

func TestTransform_AllPolicyTypes_WildcardExceptions_WildcardOperations_AllowAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "SpecificLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"limit": 100,
					},
				},
			},
		},
		{
			Name:    "ChatGuardrail",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/*",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxTokens": 1500,
					},
				},
			},
		},
		{
			Name:    "APIMonitoring",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/api/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"track": true,
					},
				},
			},
		},
		{
			Name:    "GlobalSecurity",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"level": "high",
					},
				},
			},
		},
		{
			Name:    "GlobalLogging",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"POST", "PUT", "DELETE"},
					Params: map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/admin/*",
			Methods: []api.RouteExceptionMethods{"DELETE", "POST"},
		},
		{
			Path:    "/internal/health",
			Methods: []api.RouteExceptionMethods{"GET"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: stringPtr("Authorization"),
					Value:  stringPtr("Bearer token"),
				},
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

	// Verify only auth policy at API level
	require.NotNil(t, spec.Policies, "API-level policies should exist")
	require.Len(t, *spec.Policies, 1, "Should have only auth policy at API level")
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, (*spec.Policies)[0].Name)

	// Verify catch-all operations (6) have GlobalSecurity and GlobalLogging (for POST/PUT/DELETE)
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
			require.NotNil(t, op.Policies, "Catch-all operation should have policies")

			policies := make(map[string]bool)
			for _, p := range *op.Policies {
				policies[p.Name] = true
			}
			assert.True(t, policies["GlobalSecurity"], "Should have GlobalSecurity")

			// GlobalLogging only applies to POST, PUT, DELETE
			method := string(op.Method)
			if method == "POST" || method == "PUT" || method == "DELETE" {
				require.Len(t, *op.Policies, 2, "POST/PUT/DELETE should have both policies")
				assert.True(t, policies["GlobalLogging"], "POST/PUT/DELETE should have GlobalLogging")
			} else {
				require.Len(t, *op.Policies, 1, "GET/PATCH/OPTIONS should have only GlobalSecurity")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have 6 catch-all operations")

	// Verify /chat/* POST has GlobalSecurity, GlobalLogging (for POST), and ChatGuardrail
	chatWildOp := findOperation(spec.Operations, "/chat/*", "POST")
	require.NotNil(t, chatWildOp, "/chat/* POST should exist")
	require.NotNil(t, chatWildOp.Policies, "Operation should have policies")
	require.Len(t, *chatWildOp.Policies, 3, "Should have 3 policies")

	chatWildPolicies := make(map[string]bool)
	for _, p := range *chatWildOp.Policies {
		chatWildPolicies[p.Name] = true
	}
	assert.True(t, chatWildPolicies["GlobalSecurity"], "Should have GlobalSecurity")
	assert.True(t, chatWildPolicies["GlobalLogging"], "Should have GlobalLogging")
	assert.True(t, chatWildPolicies["ChatGuardrail"], "Should have ChatGuardrail")

	// Verify /chat/completions POST has GlobalSecurity, GlobalLogging, ChatGuardrail, and SpecificLimit
	chatCompletionOp := findOperation(spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, chatCompletionOp, "/chat/completions POST should exist")
	require.NotNil(t, chatCompletionOp.Policies, "Operation should have policies")
	require.Len(t, *chatCompletionOp.Policies, 4, "Should have 4 policies")

	chatCompletionPolicies := make(map[string]bool)
	for _, p := range *chatCompletionOp.Policies {
		chatCompletionPolicies[p.Name] = true
	}
	assert.True(t, chatCompletionPolicies["GlobalSecurity"], "Should have GlobalSecurity")
	assert.True(t, chatCompletionPolicies["GlobalLogging"], "Should have GlobalLogging")
	assert.True(t, chatCompletionPolicies["ChatGuardrail"], "Should have ChatGuardrail")
	assert.True(t, chatCompletionPolicies["SpecificLimit"], "Should have SpecificLimit")

	// Verify /api/* operations (6 HTTP methods) have GlobalSecurity, GlobalLogging (for POST/PUT/DELETE), and APIMonitoring
	apiWildcardCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/api/*" {
			apiWildcardCount++
			require.NotNil(t, op.Policies, "Operation should have policies")

			policies := make(map[string]bool)
			for _, p := range *op.Policies {
				policies[p.Name] = true
			}
			assert.True(t, policies["GlobalSecurity"], "Should have GlobalSecurity")
			assert.True(t, policies["APIMonitoring"], "Should have APIMonitoring")

			// GlobalLogging only applies to POST, PUT, DELETE
			method := string(op.Method)
			if method == "POST" || method == "PUT" || method == "DELETE" {
				require.Len(t, *op.Policies, 3, "POST/PUT/DELETE should have 3 policies")
				assert.True(t, policies["GlobalLogging"], "POST/PUT/DELETE should have GlobalLogging")
			} else {
				require.Len(t, *op.Policies, 2, "GET/PATCH/OPTIONS should have 2 policies")
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), apiWildcardCount, "Should have 6 /api/* operations")

	// Verify /admin/* exception operations (DELETE and POST only) have deny policy
	adminDeleteOp := findOperation(spec.Operations, "/admin/*", "DELETE")
	require.NotNil(t, adminDeleteOp, "/admin/* DELETE should exist")
	require.NotNil(t, adminDeleteOp.Policies, "Operation should have policies")
	require.Len(t, *adminDeleteOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminDeleteOp.Policies)[0].Name)

	adminPostOp := findOperation(spec.Operations, "/admin/*", "POST")
	require.NotNil(t, adminPostOp, "/admin/* POST should exist")
	require.NotNil(t, adminPostOp.Policies, "Operation should have policies")
	require.Len(t, *adminPostOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminPostOp.Policies)[0].Name)

	// Verify /internal/health GET exception has deny policy
	healthOp := findOperation(spec.Operations, "/internal/health", "GET")
	require.NotNil(t, healthOp, "/internal/health GET should exist")
	require.NotNil(t, healthOp.Policies, "Operation should have policies")
	require.Len(t, *healthOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*healthOp.Policies)[0].Name)

	// Verify total operations count
	// 6 catch-all + 6 /api/* + 1 /chat/* + 1 /chat/completions + 2 /admin/* (DELETE, POST) + 1 /internal/health = 17 operations
	assert.Len(t, spec.Operations, 17, "Should have 22 total operations")
}

func TestTransform_AllPolicyTypes_WildcardExceptions_WildcardOperations_DenyAll(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "ModelAudit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/api/models",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"enabled": true,
					},
				},
			},
		},
		{
			Name:    "CompletionLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"limit": 500,
					},
				},
			},
		},
		{
			Name:    "ChatPolicies",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/chat/*",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"maxTokens": 2000,
					},
				},
			},
		},
		{
			Name:    "GlobalAuth",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "/*",
					Methods: []api.LLMPolicyPathMethods{"*"},
					Params: map[string]interface{}{
						"required": true,
					},
				},
			},
		},
	}

	exceptions := []api.RouteException{
		{
			Path:    "/chat/*",
			Methods: []api.RouteExceptionMethods{"POST"},
		},
		{
			Path:    "/api/models",
			Methods: []api.RouteExceptionMethods{"GET", "POST"},
		},
		{
			Path:    "/health",
			Methods: []api.RouteExceptionMethods{"GET"},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.example.com"),
				Auth: &struct {
					Header *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: stringPtr("X-API-Key"),
					Value:  stringPtr("secret"),
				},
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

	// Verify only auth policy at API level
	require.NotNil(t, spec.Policies, "API-level policies should exist")
	require.Len(t, *spec.Policies, 1, "Should have only auth policy")
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, (*spec.Policies)[0].Name)

	// Verify exactly 5 operations exist
	require.Len(t, spec.Operations, 5, "Should have exactly 5 operations")

	// Verify /chat/* POST operation has GlobalAuth and ChatPolicies and CompletionLimit
	chatWildcardOp := findOperation(spec.Operations, "/chat/*", "POST")
	require.NotNil(t, chatWildcardOp, "/chat/* POST should exist")
	require.NotNil(t, chatWildcardOp.Policies, "Operation should have policies")
	require.Len(t, *chatWildcardOp.Policies, 2, "Should have 2 policies: GlobalAuth, ChatPolicies")

	chatWildcardPolicies := make(map[string]bool)
	for _, pol := range *chatWildcardOp.Policies {
		chatWildcardPolicies[pol.Name] = true
	}
	assert.True(t, chatWildcardPolicies["GlobalAuth"], "GlobalAuth should be attached")
	assert.True(t, chatWildcardPolicies["ChatPolicies"], "ChatPolicies should be attached")

	// Verify /api/models GET operation has GlobalAuth and ModelAudit
	modelsGetOp := findOperation(spec.Operations, "/api/models", "GET")
	require.NotNil(t, modelsGetOp, "/api/models GET should exist")
	require.NotNil(t, modelsGetOp.Policies, "Operation should have policies")
	require.Len(t, *modelsGetOp.Policies, 2, "Should have 2 policies: GlobalAuth, ModelAudit")

	modelsGetPolicies := make(map[string]bool)
	for _, pol := range *modelsGetOp.Policies {
		modelsGetPolicies[pol.Name] = true
	}
	assert.True(t, modelsGetPolicies["GlobalAuth"], "GlobalAuth should be attached")
	assert.True(t, modelsGetPolicies["ModelAudit"], "ModelAudit should be attached")

	// Verify /api/models POST operation has GlobalAuth and ModelAudit
	modelsPostOp := findOperation(spec.Operations, "/api/models", "POST")
	require.NotNil(t, modelsPostOp, "/api/models POST should exist")
	require.NotNil(t, modelsPostOp.Policies, "Operation should have policies")
	require.Len(t, *modelsPostOp.Policies, 2, "Should have 2 policies: GlobalAuth, ModelAudit")

	modelsPostPolicies := make(map[string]bool)
	for _, pol := range *modelsPostOp.Policies {
		modelsPostPolicies[pol.Name] = true
	}
	assert.True(t, modelsPostPolicies["GlobalAuth"], "GlobalAuth should be attached")
	assert.True(t, modelsPostPolicies["ModelAudit"], "ModelAudit should be attached")

	// Verify /health GET operation has only GlobalAuth
	healthOp := findOperation(spec.Operations, "/health", "GET")
	require.NotNil(t, healthOp, "/health GET should exist")
	require.NotNil(t, healthOp.Policies, "Operation should have policies")
	require.Len(t, *healthOp.Policies, 1, "Should have 1 policy: GlobalAuth")
	assert.Equal(t, "GlobalAuth", (*healthOp.Policies)[0].Name)

	// Verify /chat/completions POST operation has GlobalAuth and ChatPolicies and CompletionLimit
	chatCompletionOp := findOperation(spec.Operations, "/chat/completions", "POST")
	require.NotNil(t, chatCompletionOp, "/chat/completions POST should exist")
	require.NotNil(t, chatCompletionOp.Policies, "Operation should have policies")
	require.Len(t, *chatCompletionOp.Policies, 3, "Should have 3 policies: GlobalAuth, ChatPolicies, CompletionLimit")

	chatCompletionOpPolicies := make(map[string]bool)
	for _, pol := range *chatCompletionOp.Policies {
		chatCompletionOpPolicies[pol.Name] = true
	}
	assert.True(t, chatCompletionOpPolicies["GlobalAuth"], "GlobalAuth should be attached")
	assert.True(t, chatCompletionOpPolicies["ChatPolicies"], "ChatPolicies should be attached")
	assert.True(t, chatCompletionOpPolicies["CompletionLimit"], "CompletionLimit should be attached")

	// Verify NO catch-all operations in DenyAll mode
	catchAllCount := 0
	for _, operation := range spec.Operations {
		if operation.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, 0, catchAllCount, "Should NOT have catch-all operations in DenyAll")
}

// ============================================================================
// Path Matching Edge Cases Tests - Section 10.5.1
// ============================================================================

func TestTransform_PolicyMoreGeneral_AccessControlSpecific_DenyAll(t *testing.T) {
	// Access control allows specific path, policy targets wildcard
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "chat/completions/stream",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "WildcardGuardrail",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "chat/*",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params:  map[string]interface{}{"check": true},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Should have only 1 operation (the specific exception)
	require.NotNil(t, spec.Operations)
	assert.Len(t, spec.Operations, 1)

	// Verify operation has the wildcard policy attached
	// (specific path is covered by wildcard policy)
	op := findOperation(spec.Operations, "chat/completions/stream", "POST")
	require.NotNil(t, op, "chat/completions/stream POST should exist")
	require.NotNil(t, op.Policies)
	assert.Len(t, *op.Policies, 1)
	assert.Equal(t, "WildcardGuardrail", (*op.Policies)[0].Name)
}

func TestTransform_MultipleOverlappingExceptions_MultipleWildcardPolicies_DenyAll(t *testing.T) {
	// Multiple overlapping exceptions (chat/*, chat/completions/*) with multiple wildcard policies
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "chat/completions/stream",
			Methods: []api.RouteExceptionMethods{"POST"},
		},
		{
			Path:    "chat/completions/streams",
			Methods: []api.RouteExceptionMethods{"POST"},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatWildcardPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "chat/*",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params:  map[string]interface{}{"category": "chat", "level": 1},
				},
			},
		},
		{
			Name:    "ChatCompletionsWildcardPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "chat/completions/*",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params:  map[string]interface{}{"category": "completions", "level": 2},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Should have 2 operations: chat/completions/stream and chat/completions/streams
	require.NotNil(t, spec.Operations)
	assert.Len(t, spec.Operations, 2)

	// Verify chat/completions/stream operation has both policies
	chatStramOp := findOperation(spec.Operations, "chat/completions/stream", "POST")
	require.NotNil(t, chatStramOp, "chat/completions/stream POST should exist")
	require.NotNil(t, chatStramOp.Policies)
	assert.Len(t, *chatStramOp.Policies, 2)

	// Verify both policy names are present
	policyNames := make(map[string]bool)
	for _, p := range *chatStramOp.Policies {
		policyNames[p.Name] = true
	}
	assert.True(t, policyNames["ChatWildcardPolicy"], "chat/completions/stream should have ChatWildcardPolicy (covered by chat/*)")
	assert.True(t, policyNames["ChatCompletionsWildcardPolicy"], "chat/completions/stream should have ChatCompletionsWildcardPolicy (exact match)")

	// Verify chat/completions/stream operation has both policies
	chatStreamsOp := findOperation(spec.Operations, "chat/completions/streams", "POST")
	require.NotNil(t, chatStreamsOp, "chat/completions/streams POST should exist")
	require.NotNil(t, chatStreamsOp.Policies)
	assert.Len(t, *chatStreamsOp.Policies, 2)

	// Verify both policy names are present
	policyNames = make(map[string]bool)
	for _, p := range *chatStreamsOp.Policies {
		policyNames[p.Name] = true
	}
	assert.True(t, policyNames["ChatWildcardPolicy"], "chat/completions/streams should have ChatWildcardPolicy (covered by chat/*)")
	assert.True(t, policyNames["ChatCompletionsWildcardPolicy"], "chat/completions/streams should have ChatCompletionsWildcardPolicy (exact match)")
}

func TestTransform_PolicyMoreSpecific_AccessControlWildcard_DenyAll(t *testing.T) {
	// Access control allows wildcard, policy targets specific path
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "chat/*",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "SpecificTokenLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params:  map[string]interface{}{"maxTokens": 1000},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Should have 2 operations: chat/* (catch-all) and chat/completions (specific with policy)
	require.NotNil(t, spec.Operations)
	assert.Len(t, spec.Operations, 2)

	// Verify specific operation has policy
	specificOp := findOperation(spec.Operations, "chat/completions", "POST")
	require.NotNil(t, specificOp, "chat/completions POST should exist")
	require.NotNil(t, specificOp.Policies)
	assert.Len(t, *specificOp.Policies, 1)
	assert.Equal(t, "SpecificTokenLimit", (*specificOp.Policies)[0].Name)

	// Verify wildcard operation has NO policy
	wildcardOp := findOperation(spec.Operations, "chat/*", "POST")
	require.NotNil(t, wildcardOp, "chat/* POST should exist")
	if wildcardOp.Policies != nil {
		assert.Len(t, *wildcardOp.Policies, 0)
	}
}

func TestTransform_MultipleOverlappingWildcards_DenyAll(t *testing.T) {
	// Multiple overlapping wildcards: chat/*, chat/comp*
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "chat/*",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
		{
			Path:    "chat/comp*",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "chat/*",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params:  map[string]interface{}{"chatCheck": true},
				},
			},
		},
		{
			Name:    "CompPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "chat/comp*",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params:  map[string]interface{}{"compCheck": true},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Should have 2 operations: chat/* and chat/comp*
	require.NotNil(t, spec.Operations)
	assert.Len(t, spec.Operations, 2)

	// chat/comp* is more specific, should come first after sorting
	ops := spec.Operations
	compOp := findOperation(ops, "chat/comp*", "POST")
	chatOp := findOperation(ops, "chat/*", "POST")

	require.NotNil(t, compOp, "chat/comp* POST should exist")
	require.NotNil(t, chatOp, "chat/* POST should exist")

	// chat/comp* should have both policies (it's covered by both wildcards)
	require.NotNil(t, compOp.Policies)
	assert.Len(t, *compOp.Policies, 2)
	policyNames := []string{(*compOp.Policies)[0].Name, (*compOp.Policies)[1].Name}
	assert.Contains(t, policyNames, "ChatPolicy")
	assert.Contains(t, policyNames, "CompPolicy")

	// chat/* should have only ChatPolicy
	require.NotNil(t, chatOp.Policies)
	assert.Len(t, *chatOp.Policies, 1)
	assert.Equal(t, "ChatPolicy", (*chatOp.Policies)[0].Name)
}

func TestTransform_TripleNestedWildcards_DenyAll(t *testing.T) {
	// Triple nesting: a/*, a/b/*, a/b/c/*
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "api/*",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
		{
			Path:    "api/v1/*",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
		{
			Path:    "api/v1/models/*",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "Level1Policy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "api/*",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params:  map[string]interface{}{"level": 1},
				},
			},
		},
		{
			Name:    "Level2Policy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "api/v1/*",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params:  map[string]interface{}{"level": 2},
				},
			},
		},
		{
			Name:    "Level3Policy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "api/v1/models/*",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params:  map[string]interface{}{"level": 3},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Should have 3 operations
	require.NotNil(t, spec.Operations)
	assert.Len(t, spec.Operations, 3)

	// Most specific path should be first: api/v1/models/*
	level3Op := findOperation(spec.Operations, "api/v1/models/*", "GET")
	require.NotNil(t, level3Op, "api/v1/models/* GET should exist")
	require.NotNil(t, level3Op.Policies)
	// Should have all 3 policies (nested coverage)
	assert.Len(t, *level3Op.Policies, 3)
	policyNames := []string{
		(*level3Op.Policies)[0].Name,
		(*level3Op.Policies)[1].Name,
		(*level3Op.Policies)[2].Name,
	}
	assert.Contains(t, policyNames, "Level1Policy")
	assert.Contains(t, policyNames, "Level2Policy")
	assert.Contains(t, policyNames, "Level3Policy")

	// api/v1/* should have Level1Policy and Level2Policy
	level2Op := findOperation(spec.Operations, "api/v1/*", "GET")
	require.NotNil(t, level2Op, "api/v1/* GET should exist")
	require.NotNil(t, level2Op.Policies)
	assert.Len(t, *level2Op.Policies, 2)
	level2PolicyNames := []string{(*level2Op.Policies)[0].Name, (*level2Op.Policies)[1].Name}
	assert.Contains(t, level2PolicyNames, "Level1Policy")
	assert.Contains(t, level2PolicyNames, "Level2Policy")

	// api/* should have only Level1Policy
	level1Op := findOperation(spec.Operations, "api/*", "GET")
	require.NotNil(t, level1Op, "api/* GET should exist")
	require.NotNil(t, level1Op.Policies)
	assert.Len(t, *level1Op.Policies, 1)
	assert.Equal(t, "Level1Policy", (*level1Op.Policies)[0].Name)
}

func TestTransform_SiblingWildcards_DenyAll(t *testing.T) {
	// Sibling wildcards: chat/*, models/* - independent paths
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "chat/*",
			Methods: []api.RouteExceptionMethods{api.POST},
		},
		{
			Path:    "models/*",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "chat/*",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params:  map[string]interface{}{"chatLimit": 100},
				},
			},
		},
		{
			Name:    "ModelsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "models/*",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params:  map[string]interface{}{"modelCheck": true},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Should have 2 operations
	require.NotNil(t, spec.Operations)
	assert.Len(t, spec.Operations, 2)

	// Verify chat/* operation
	chatOp := findOperation(spec.Operations, "chat/*", "POST")
	require.NotNil(t, chatOp, "chat/* POST should exist")
	require.NotNil(t, chatOp.Policies)
	assert.Len(t, *chatOp.Policies, 1)
	assert.Equal(t, "ChatPolicy", (*chatOp.Policies)[0].Name)

	// Verify models/* operation
	modelsOp := findOperation(spec.Operations, "models/*", "GET")
	require.NotNil(t, modelsOp, "models/* GET should exist")
	require.NotNil(t, modelsOp.Policies)
	assert.Len(t, *modelsOp.Policies, 1)
	assert.Equal(t, "ModelsPolicy", (*modelsOp.Policies)[0].Name)

	// Verify policies are NOT cross-applied
	// chat/* should NOT have ModelsPolicy
	for _, policy := range *chatOp.Policies {
		assert.NotEqual(t, "ModelsPolicy", policy.Name)
	}
	// models/* should NOT have ChatPolicy
	for _, policy := range *modelsOp.Policies {
		assert.NotEqual(t, "ChatPolicy", policy.Name)
	}
}

func TestTransform_PathMatchingEdgeCases_AllowAll_PolicyMoreGeneral(t *testing.T) {
	// AllowAll: Exception on specific path, policy on wildcard
	// Policy should NOT attach due to exception precedence
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{
			Path:    "internal/metrics",
			Methods: []api.RouteExceptionMethods{api.GET},
		},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "InternalPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "internal/*",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params:  map[string]interface{}{"check": true},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Find exception operation
	exceptionOp := findOperation(spec.Operations, "internal/metrics", "GET")
	require.NotNil(t, exceptionOp, "internal/metrics GET should exist")

	// Should have deny policy only, NOT user policy (exception precedence)
	require.NotNil(t, exceptionOp.Policies)
	assert.Len(t, *exceptionOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*exceptionOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

func TestTransform_PathMatchingEdgeCases_AllowAll_NestedWildcardPolicies(t *testing.T) {
	// AllowAll: Multiple nested wildcard policies with no exceptions
	// All policies should attach to catch-all operations
	transformer, _ := setupTestTransformer(t)

	policies := []api.LLMPolicy{
		{
			Name:    "TopLevelPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "api/*",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params:  map[string]interface{}{"level": "top"},
				},
			},
		},
		{
			Name:    "NestedPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "api/v1/*",
					Methods: []api.LLMPolicyPathMethods{"GET"},
					Params:  map[string]interface{}{"level": "nested"},
				},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
			Policies: &policies,
		},
	}

	output := &api.APIConfiguration{}
	result, err := transformer.Transform(provider, output)

	require.NoError(t, err)
	spec, err := result.Spec.AsAPIConfigData()
	require.NoError(t, err)

	// Should have operations for both wildcard policies + catch-all
	require.NotNil(t, spec.Operations)

	// Find api/v1/* operation
	nestedOp := findOperation(spec.Operations, "api/v1/*", "GET")
	require.NotNil(t, nestedOp, "api/v1/* GET should exist")
	require.NotNil(t, nestedOp.Policies)
	assert.Len(t, *nestedOp.Policies, 1)
	assert.Equal(t, "NestedPolicy", (*nestedOp.Policies)[0].Name)

	// Find api/* operation
	topOp := findOperation(spec.Operations, "api/*", "GET")
	require.NotNil(t, topOp, "api/* GET should exist")
	require.NotNil(t, topOp.Policies)
	assert.Len(t, *topOp.Policies, 1)
	assert.Equal(t, "TopLevelPolicy", (*topOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

// ============================================================================
// Complex Combined Scenarios Tests - Section 10.5.2
// ============================================================================

func TestTransform_ComplexCombined_MultipleAPILevelPolicies_NestedWildcards_AllowAll(t *testing.T) {
	// Multiple API-level policies + nested wildcard exceptions + nested operation policies
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"DELETE", "POST"}},
		{Path: "admin/users/*", Methods: []api.RouteExceptionMethods{"DELETE"}},
		{Path: "internal/debug", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	policies := []api.LLMPolicy{
		// Specific operation policies
		{
			Name:    "ChatPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"streaming": true}},
			},
		},
		// Nested wildcard policies
		{
			Name:    "APIModelsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/models/*", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"cache": true}},
			},
		},
		{
			Name:    "APIPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"version": "v1"}},
			},
		},
		// Global policies
		{
			Name:    "GlobalAuth",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"enabled": true}},
			},
		},
		{
			Name:    "GlobalRateLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"rps": 1000}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// ===== API-Level Policies =====
	// Verify no API-level policies exist (/* policies go to operations)
	assert.Nil(t, spec.Policies, "No API-level policies should exist")

	// ===== Catch-All Operations =====
	// Verify 6 catch-all operations (/*) have both GlobalAuth and GlobalRateLimit
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
			require.NotNil(t, op.Policies, "Catch-all operation should have policies")
			require.Len(t, *op.Policies, 2, "Should have 2 policies (GlobalAuth, GlobalRateLimit)")

			catchAllPolicies := make(map[string]bool)
			for _, p := range *op.Policies {
				catchAllPolicies[p.Name] = true
			}
			assert.True(t, catchAllPolicies["GlobalAuth"], "Should have GlobalAuth")
			assert.True(t, catchAllPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have 6 catch-all operations")

	// ===== API Operations (not denied) =====
	// Verify api/* operations (6 HTTP methods) have GlobalAuth, GlobalRateLimit, and APIPolicy
	apiWildcardCount := 0
	for _, op := range spec.Operations {
		if op.Path == "api/*" {
			apiWildcardCount++
			require.NotNil(t, op.Policies, "api/* operation should have policies")
			require.Len(t, *op.Policies, 3, "Should have 3 policies (GlobalAuth, GlobalRateLimit, APIPolicy)")

			apiPolicies := make(map[string]bool)
			for _, p := range *op.Policies {
				apiPolicies[p.Name] = true
			}
			assert.True(t, apiPolicies["GlobalAuth"], "Should have GlobalAuth")
			assert.True(t, apiPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
			assert.True(t, apiPolicies["APIPolicy"], "Should have APIPolicy")
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), apiWildcardCount, "Should have 6 api/* operations (all HTTP methods)")

	// Verify api/models/* GET operation has GlobalAuth, GlobalRateLimit, APIPolicy, and APIModelsPolicy
	apiModelsGetOp := findOperation(spec.Operations, "api/models/*", "GET")
	require.NotNil(t, apiModelsGetOp, "api/models/* GET should exist")
	require.NotNil(t, apiModelsGetOp.Policies, "Operation should have policies")
	require.Len(t, *apiModelsGetOp.Policies, 4, "Should have 4 policies (GlobalAuth, GlobalRateLimit, APIPolicy, APIModelsPolicy)")

	apiModelsGetPolicies := make(map[string]bool)
	for _, p := range *apiModelsGetOp.Policies {
		apiModelsGetPolicies[p.Name] = true
	}
	assert.True(t, apiModelsGetPolicies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, apiModelsGetPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, apiModelsGetPolicies["APIPolicy"], "Should have APIPolicy (api/* coverage)")
	assert.True(t, apiModelsGetPolicies["APIModelsPolicy"], "Should have APIModelsPolicy (exact match)")

	// Verify api/models/* other methods (not GET) does not exist
	apiModelsPostOp := findOperation(spec.Operations, "api/models/*", "POST")
	require.Nil(t, apiModelsPostOp, "api/models/* POST should exist")

	// ===== Chat Operations =====
	// Verify chat/completions POST has GlobalAuth, GlobalRateLimit, and ChatPolicy
	chatCompletionsOp := findOperation(spec.Operations, "chat/completions", "POST")
	require.NotNil(t, chatCompletionsOp, "chat/completions POST should exist")
	require.NotNil(t, chatCompletionsOp.Policies, "Operation should have policies")
	require.Len(t, *chatCompletionsOp.Policies, 3, "Should have 3 policies")

	chatPolicies := make(map[string]bool)
	for _, p := range *chatCompletionsOp.Policies {
		chatPolicies[p.Name] = true
	}
	assert.True(t, chatPolicies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, chatPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, chatPolicies["ChatPolicy"], "Should have ChatPolicy")

	// ===== Exception Operations (Denied) =====
	// Verify admin/* DELETE exception has deny policy
	adminDeleteOp := findOperation(spec.Operations, "admin/*", "DELETE")
	require.NotNil(t, adminDeleteOp, "admin/* DELETE should exist")
	require.NotNil(t, adminDeleteOp.Policies, "Operation should have policies")
	require.Len(t, *adminDeleteOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminDeleteOp.Policies)[0].Name)

	// Verify admin/* POST exception has deny policy
	adminPostOp := findOperation(spec.Operations, "admin/*", "POST")
	require.NotNil(t, adminPostOp, "admin/* POST should exist")
	require.NotNil(t, adminPostOp.Policies, "Operation should have policies")
	require.Len(t, *adminPostOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminPostOp.Policies)[0].Name)

	// Verify admin/users/* DELETE exception (nested, more specific) has deny policy
	adminUsersDeleteOp := findOperation(spec.Operations, "admin/users/*", "DELETE")
	require.NotNil(t, adminUsersDeleteOp, "admin/users/* DELETE should exist")
	require.NotNil(t, adminUsersDeleteOp.Policies, "Operation should have policies")
	require.Len(t, *adminUsersDeleteOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminUsersDeleteOp.Policies)[0].Name)

	// Verify internal/debug GET exception has deny policy
	internalDebugOp := findOperation(spec.Operations, "internal/debug", "GET")
	require.NotNil(t, internalDebugOp, "internal/debug GET should exist")
	require.NotNil(t, internalDebugOp.Policies, "Operation should have policies")
	require.Len(t, *internalDebugOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalDebugOp.Policies)[0].Name)

	// ===== Total Operations Count =====
	// 6 catch-all + 6 api/* + 1 api/models/* GET + 1 chat/completions POST +
	// 1 admin/* DELETE + 1 admin/* POST + 1 admin/users/* DELETE + 1 internal/debug GET = 18 operations
	assert.Len(t, spec.Operations, 18, "Should have 19 total operations")
}

func TestTransform_ComplexCombined_MultipleAPILevelPolicies_NestedWildcards_DenyAll(t *testing.T) {
	// Same as AllowAll but in DenyAll mode - no catch-all operations
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "api/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "api/v1/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "api/v1/models", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "health", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	policies := []api.LLMPolicy{
		// Specific operation policy
		{
			Name:    "ModelsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/v1/models", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"cache": true}},
			},
		},
		{
			Name:    "APIV1Policy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/v1/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"v1_specific": true}},
			},
		},
		// Nested wildcard policies
		{
			Name:    "APIVersionPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"v1": true}},
			},
		},
		// API-level policies
		{
			Name:    "GlobalSecurity",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"strict": true}},
			},
		},
		{
			Name:    "GlobalAudit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"log": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify no API-level policies (GlobalSecurity and GlobalAudit apply to operations via /* matching)
	assert.Nil(t, spec.Policies, "Should have no API-level policies")

	// Verify exactly 14 operations (6 for api/*, 6 for api/v1/*, 1 for api/v1/models GET, 1 for health GET)
	require.Len(t, spec.Operations, 14, "Should have exactly 14 operations")

	// Verify api/* operations (6 HTTP methods)
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		op := findOperation(spec.Operations, "api/*", method)
		require.NotNil(t, op, "api/* operation should exist for method %s", method)
		require.NotNil(t, op.Policies, "api/* %s should have policies", method)
		require.Len(t, *op.Policies, 3, "api/* %s should have 3 policies", method)

		policyNames := make(map[string]bool)
		for _, p := range *op.Policies {
			policyNames[p.Name] = true
		}
		assert.True(t, policyNames["GlobalSecurity"], "api/* %s should have GlobalSecurity", method)
		assert.True(t, policyNames["GlobalAudit"], "api/* %s should have GlobalAudit", method)
		assert.True(t, policyNames["APIVersionPolicy"], "api/* %s should have APIVersionPolicy", method)
	}

	// Verify api/v1/* operations (6 HTTP methods)
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		op := findOperation(spec.Operations, "api/v1/*", method)
		require.NotNil(t, op, "api/v1/* operation should exist for method %s", method)
		require.NotNil(t, op.Policies, "api/v1/* %s should have policies", method)
		require.Len(t, *op.Policies, 4, "api/v1/* %s should have 4 policies", method)

		policyNames := make(map[string]bool)
		for _, p := range *op.Policies {
			policyNames[p.Name] = true
		}
		assert.True(t, policyNames["GlobalSecurity"], "api/v1/* %s should have GlobalSecurity", method)
		assert.True(t, policyNames["GlobalAudit"], "api/v1/* %s should have GlobalAudit", method)
		assert.True(t, policyNames["APIVersionPolicy"], "api/v1/* %s should have APIVersionPolicy", method)
		assert.True(t, policyNames["APIV1Policy"], "api/v1/* %s should have APIV1Policy", method)
	}

	// Verify api/v1/models GET operation
	modelsOp := findOperation(spec.Operations, "api/v1/models", "GET")
	require.NotNil(t, modelsOp, "api/v1/models GET should exist")
	require.NotNil(t, modelsOp.Policies, "api/v1/models GET should have policies")
	require.Len(t, *modelsOp.Policies, 5, "api/v1/models GET should have 5 policies")

	modelsPolicyNames := make(map[string]bool)
	for _, p := range *modelsOp.Policies {
		modelsPolicyNames[p.Name] = true
	}
	assert.True(t, modelsPolicyNames["GlobalSecurity"], "api/v1/models GET should have GlobalSecurity")
	assert.True(t, modelsPolicyNames["GlobalAudit"], "api/v1/models GET should have GlobalAudit")
	assert.True(t, modelsPolicyNames["APIVersionPolicy"], "api/v1/models GET should have APIVersionPolicy")
	assert.True(t, modelsPolicyNames["APIV1Policy"], "api/v1/models GET should have APIV1Policy")
	assert.True(t, modelsPolicyNames["ModelsPolicy"], "api/v1/models GET should have ModelsPolicy")

	// Verify health GET operation has only GlobalSecurity and GlobalAudit
	healthOp := findOperation(spec.Operations, "health", "GET")
	require.NotNil(t, healthOp, "health GET should exist")
	require.NotNil(t, healthOp.Policies, "health GET should have policies")
	require.Len(t, *healthOp.Policies, 2, "health GET should have exactly 2 policies")

	healthPolicyNames := make(map[string]bool)
	for _, p := range *healthOp.Policies {
		healthPolicyNames[p.Name] = true
	}
	assert.True(t, healthPolicyNames["GlobalSecurity"], "health GET should have GlobalSecurity")
	assert.True(t, healthPolicyNames["GlobalAudit"], "health GET should have GlobalAudit")

	// Verify NO catch-all operations in DenyAll mode
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, 0, catchAllCount, "DenyAll mode should have no catch-all operations")
}

func TestTransform_ComplexCombined_MaximumComplexity_AllowAll(t *testing.T) {
	// Maximum complexity test: auth + nested wildcards + exceptions + specific policies
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "internal/debug/*", Methods: []api.RouteExceptionMethods{"GET", "POST"}},
		{Path: "system/health", Methods: []api.RouteExceptionMethods{"DELETE"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "SpecificModelPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/models/gpt4", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"model": "gpt4"}},
			},
		},
		{
			Name:    "SpecificChatPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"specific": true}},
			},
		},
		// Policies that would be blocked by exceptions
		{
			Name:    "AdminPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "admin/users", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"blocked": true}},
			},
		},
		{
			Name:    "ChatCompletionsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions/*", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"streaming": "enabled"}},
			},
		},
		{
			Name:    "ChatPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"category": "chat"}},
			},
		},
		{
			Name:    "APIModelsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/models/*", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"cache": 3600}},
			},
		},
		// Nested wildcard policies
		{
			Name:    "APIPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/*", Methods: []api.LLMPolicyPathMethods{"GET", "POST"}, Params: map[string]interface{}{"version": "v1"}},
			},
		},
		// Global policies
		{
			Name:    "GlobalAuth",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"required": true}},
			},
		},
		{
			Name:    "GlobalRateLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"rps": 5000}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
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

	// ===== API-Level Policies =====
	// Verify only auth policy at API level
	require.NotNil(t, spec.Policies, "API-level policies should exist")
	require.Len(t, *spec.Policies, 1, "Should have only auth policy at API level")
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, (*spec.Policies)[0].Name)

	// ===== Catch-All Operations =====
	// Verify 6 catch-all operations (/*) have both GlobalAuth and GlobalRateLimit
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
			require.NotNil(t, op.Policies, "Catch-all operation should have policies")
			require.Len(t, *op.Policies, 2, "Should have 2 policies (GlobalAuth, GlobalRateLimit)")

			catchAllPolicies := make(map[string]bool)
			for _, p := range *op.Policies {
				catchAllPolicies[p.Name] = true
			}
			assert.True(t, catchAllPolicies["GlobalAuth"], "Should have GlobalAuth")
			assert.True(t, catchAllPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount, "Should have 6 catch-all operations")

	// ===== API Nested Wildcard Operations =====
	// Verify api/* GET operations (method-restricted) have GlobalAuth, GlobalRateLimit, and APIPolicy
	apiGetOp := findOperation(spec.Operations, "api/*", "GET")
	require.NotNil(t, apiGetOp, "api/* GET should exist")
	require.NotNil(t, apiGetOp.Policies, "Operation should have policies")
	require.Len(t, *apiGetOp.Policies, 3, "Should have 3 policies")

	apiGetPolicies := make(map[string]bool)
	for _, p := range *apiGetOp.Policies {
		apiGetPolicies[p.Name] = true
	}
	assert.True(t, apiGetPolicies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, apiGetPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, apiGetPolicies["APIPolicy"], "Should have APIPolicy (api/* GET match)")

	// Verify api/* POST has APIPolicy (method-restricted to GET, POST)
	apiPostOp := findOperation(spec.Operations, "api/*", "POST")
	require.NotNil(t, apiPostOp, "api/* POST should exist")
	require.NotNil(t, apiPostOp.Policies, "Operation should have policies")
	require.Len(t, *apiPostOp.Policies, 3, "Should have 3 policies")

	apiPostPolicies := make(map[string]bool)
	for _, p := range *apiPostOp.Policies {
		apiPostPolicies[p.Name] = true
	}
	assert.True(t, apiPostPolicies["APIPolicy"], "Should have APIPolicy (api/* POST match)")

	// Verify api/* DELETE does NOT exist (method restriction: only GET, POST)
	apiDeleteOp := findOperation(spec.Operations, "api/*", "DELETE")
	require.Nil(t, apiDeleteOp, "api/* DELETE should exist")

	// Verify api/models/* GET has GlobalAuth, GlobalRateLimit, APIPolicy, and APIModelsPolicy
	apiModelsGetOp := findOperation(spec.Operations, "api/models/*", "GET")
	require.NotNil(t, apiModelsGetOp, "api/models/* GET should exist")
	require.NotNil(t, apiModelsGetOp.Policies, "Operation should have policies")
	require.Len(t, *apiModelsGetOp.Policies, 4, "Should have 4 policies")

	apiModelsGetPolicies := make(map[string]bool)
	for _, p := range *apiModelsGetOp.Policies {
		apiModelsGetPolicies[p.Name] = true
	}
	assert.True(t, apiModelsGetPolicies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, apiModelsGetPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, apiModelsGetPolicies["APIPolicy"], "Should have APIPolicy (api/* coverage)")
	assert.True(t, apiModelsGetPolicies["APIModelsPolicy"], "Should have APIModelsPolicy (api/models/* GET match)")

	// Verify api/models/gpt4 GET has all 5 policies (most specific)
	apiModelsGPT4Op := findOperation(spec.Operations, "api/models/gpt4", "GET")
	require.NotNil(t, apiModelsGPT4Op, "api/models/gpt4 GET should exist")
	require.NotNil(t, apiModelsGPT4Op.Policies, "Operation should have policies")
	require.Len(t, *apiModelsGPT4Op.Policies, 5, "Should have 5 policies")

	gpt4Policies := make(map[string]bool)
	for _, p := range *apiModelsGPT4Op.Policies {
		gpt4Policies[p.Name] = true
	}
	assert.True(t, gpt4Policies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, gpt4Policies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, gpt4Policies["APIPolicy"], "Should have APIPolicy (api/* coverage)")
	assert.True(t, gpt4Policies["APIModelsPolicy"], "Should have APIModelsPolicy (api/models/* coverage)")
	assert.True(t, gpt4Policies["SpecificModelPolicy"], "Should have SpecificModelPolicy (exact match)")

	// ===== Chat Nested Wildcard Operations =====
	// Verify chat/* GET has GlobalAuth, GlobalRateLimit, and ChatPolicy
	chatGetOp := findOperation(spec.Operations, "chat/*", "GET")
	require.NotNil(t, chatGetOp, "chat/* GET should exist")
	require.NotNil(t, chatGetOp.Policies, "Operation should have policies")
	require.Len(t, *chatGetOp.Policies, 3, "Should have 3 policies")

	chatGetPolicies := make(map[string]bool)
	for _, p := range *chatGetOp.Policies {
		chatGetPolicies[p.Name] = true
	}
	assert.True(t, chatGetPolicies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, chatGetPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, chatGetPolicies["ChatPolicy"], "Should have ChatPolicy (chat/* match)")

	// Verify chat/* POST has ChatPolicy
	chatPostOp := findOperation(spec.Operations, "chat/*", "POST")
	require.NotNil(t, chatPostOp, "chat/* POST should exist")
	require.NotNil(t, chatPostOp.Policies, "Operation should have policies")
	require.Len(t, *chatPostOp.Policies, 3, "Should have 3 policies")

	chatPostPolicies := make(map[string]bool)
	for _, p := range *chatPostOp.Policies {
		chatPostPolicies[p.Name] = true
	}
	assert.True(t, chatPostPolicies["ChatPolicy"], "Should have ChatPolicy")

	// Verify chat/completions/* POST has GlobalAuth, GlobalRateLimit, ChatPolicy, and ChatCompletionsPolicy
	chatCompletionsWildcardOp := findOperation(spec.Operations, "chat/completions/*", "POST")
	require.NotNil(t, chatCompletionsWildcardOp, "chat/completions/* POST should exist")
	require.NotNil(t, chatCompletionsWildcardOp.Policies, "Operation should have policies")
	require.Len(t, *chatCompletionsWildcardOp.Policies, 4, "Should have 4 policies")

	chatCompWildPolicies := make(map[string]bool)
	for _, p := range *chatCompletionsWildcardOp.Policies {
		chatCompWildPolicies[p.Name] = true
	}
	assert.True(t, chatCompWildPolicies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, chatCompWildPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, chatCompWildPolicies["ChatPolicy"], "Should have ChatPolicy (chat/* coverage)")
	assert.True(t, chatCompWildPolicies["ChatCompletionsPolicy"], "Should have ChatCompletionsPolicy (chat/completions/* match)")

	// Verify chat/completions POST has all 4 policies (most specific)
	chatCompletionsOp := findOperation(spec.Operations, "chat/completions", "POST")
	require.NotNil(t, chatCompletionsOp, "chat/completions POST should exist")
	require.NotNil(t, chatCompletionsOp.Policies, "Operation should have policies")
	require.Len(t, *chatCompletionsOp.Policies, 4, "Should have 4 policies")

	chatCompPolicies := make(map[string]bool)
	for _, p := range *chatCompletionsOp.Policies {
		chatCompPolicies[p.Name] = true
	}
	assert.True(t, chatCompPolicies["GlobalAuth"], "Should have GlobalAuth")
	assert.True(t, chatCompPolicies["GlobalRateLimit"], "Should have GlobalRateLimit")
	assert.True(t, chatCompPolicies["ChatPolicy"], "Should have ChatPolicy (chat/* coverage)")
	assert.True(t, chatCompPolicies["SpecificChatPolicy"], "Should have SpecificChatPolicy (exact match)")

	// ===== Exception Operations (Denied) =====
	// Verify admin/* ALL methods (6) have ONLY deny policy
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		op := findOperation(spec.Operations, "admin/*", method)
		require.NotNil(t, op, "admin/* operation should exist for method %s", method)
		require.NotNil(t, op.Policies, "Operation should have policies")
		require.Len(t, *op.Policies, 1, "Should have only deny policy for admin/* %s", method)
		assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*op.Policies)[0].Name)
	}

	// Verify internal/debug/* GET has ONLY deny policy
	internalDebugGetOp := findOperation(spec.Operations, "internal/debug/*", "GET")
	require.NotNil(t, internalDebugGetOp, "internal/debug/* GET should exist")
	require.NotNil(t, internalDebugGetOp.Policies, "Operation should have policies")
	require.Len(t, *internalDebugGetOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalDebugGetOp.Policies)[0].Name)

	// Verify internal/debug/* POST has ONLY deny policy
	internalDebugPostOp := findOperation(spec.Operations, "internal/debug/*", "POST")
	require.NotNil(t, internalDebugPostOp, "internal/debug/* POST should exist")
	require.NotNil(t, internalDebugPostOp.Policies, "Operation should have policies")
	require.Len(t, *internalDebugPostOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalDebugPostOp.Policies)[0].Name)

	// Verify system/health DELETE has ONLY deny policy
	systemHealthDeleteOp := findOperation(spec.Operations, "system/health", "DELETE")
	require.NotNil(t, systemHealthDeleteOp, "system/health DELETE should exist")
	require.NotNil(t, systemHealthDeleteOp.Policies, "Operation should have policies")
	require.Len(t, *systemHealthDeleteOp.Policies, 1, "Should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*systemHealthDeleteOp.Policies)[0].Name)

	// ===== Exception Precedence Verification =====
	// Verify AdminPolicy is NOT attached to admin/* operations (exception takes precedence)
	adminGetOp := findOperation(spec.Operations, "admin/*", "GET")
	require.NotNil(t, adminGetOp, "admin/* GET should exist")
	for _, p := range *adminGetOp.Policies {
		assert.NotEqual(t, "AdminPolicy", p.Name, "AdminPolicy should NOT be attached to admin/* (exception blocks it)")
	}

	// ===== Total Operations Count =====
	// 6 catch-all + 2 api/* {"GET", "POST"} + 1 api/models/* GET + 1 api/models/gpt4 GET +
	// 6 chat/* + 1 chat/completions/* POST + 1 chat/completions POST +
	// 6 admin/* (all methods) + 2 internal/debug/* (GET, POST) + 1 system/health DELETE = 27 operations
	assert.Len(t, spec.Operations, 27, "Should have 27 total operations")
}

func TestTransform_ComplexCombined_MaximumComplexity_DenyAll(t *testing.T) {
	// Maximum complexity test in DenyAll mode
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "chat/completions/*", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "api/*", Methods: []api.RouteExceptionMethods{"GET", "POST"}},
		{Path: "api/v1/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "api/v1/models", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "health", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	policies := []api.LLMPolicy{
		// Specific operation policies
		{
			Name:    "ModelsSpecificPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/v1/models", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"cache": true}},
			},
		},
		// Nested wildcard policies
		{
			Name:    "ChatCompletionsWildcardPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions/*", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"streaming": true}},
			},
		},
		{
			Name:    "ChatRootPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"category": "chat"}},
			},
		},
		{
			Name:    "APIV1Policy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/v1/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"v1": true}},
			},
		},
		{
			Name:    "APIRootPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/*", Methods: []api.LLMPolicyPathMethods{"GET", "POST"}, Params: map[string]interface{}{"version": "latest"}},
			},
		},
		// API-level policies
		{
			Name:    "GlobalSecurity",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"enabled": true}},
			},
		},
		{
			Name:    "GlobalMonitoring",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"metrics": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
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

	// Verify only auth policy at API level
	require.NotNil(t, spec.Policies)
	require.Len(t, *spec.Policies, 1, "Should have only auth policy at API level")
	assert.Equal(t, constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME, (*spec.Policies)[0].Name)

	// Verify api/v1/models GET has all applicable policies
	modelsOp := findOperation(spec.Operations, "api/v1/models", "GET")
	require.NotNil(t, modelsOp, "api/v1/models GET should exist")
	require.NotNil(t, modelsOp.Policies, "Operation should have policies")

	modelsPolicies := make(map[string]bool)
	for _, p := range *modelsOp.Policies {
		modelsPolicies[p.Name] = true
	}
	assert.True(t, modelsPolicies["GlobalSecurity"], "Should have GlobalSecurity")
	assert.True(t, modelsPolicies["GlobalMonitoring"], "Should have GlobalMonitoring")
	assert.True(t, modelsPolicies["APIRootPolicy"], "Should have APIRootPolicy (api/* coverage)")
	assert.True(t, modelsPolicies["APIV1Policy"], "Should have APIV1Policy (api/v1/* coverage)")
	assert.True(t, modelsPolicies["ModelsSpecificPolicy"], "Should have ModelsSpecificPolicy (exact match)")

	// Verify chat/completions/* POST has nested policies
	chatCompletionsWildcardOp := findOperation(spec.Operations, "chat/completions/*", "POST")
	require.NotNil(t, chatCompletionsWildcardOp, "chat/completions/* POST should exist")
	require.NotNil(t, chatCompletionsWildcardOp.Policies, "Operation should have policies")

	chatCompletionsPolicies := make(map[string]bool)
	for _, p := range *chatCompletionsWildcardOp.Policies {
		chatCompletionsPolicies[p.Name] = true
	}
	assert.True(t, chatCompletionsPolicies["GlobalSecurity"], "Should have GlobalSecurity")
	assert.True(t, chatCompletionsPolicies["GlobalMonitoring"], "Should have GlobalMonitoring")
	assert.True(t, chatCompletionsPolicies["ChatRootPolicy"], "Should have ChatRootPolicy (chat/* coverage)")
	assert.True(t, chatCompletionsPolicies["ChatCompletionsWildcardPolicy"], "Should have ChatCompletionsWildcardPolicy (exact match)")

	// Verify chat/* operations exist for all methods with ChatRootPolicy and global policies
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		op := findOperation(spec.Operations, "chat/*", method)
		require.NotNil(t, op, "chat/* operation should exist for method %s", method)
		require.NotNil(t, op.Policies, "chat/* %s should have policies", method)

		chatPolicies := make(map[string]bool)
		for _, p := range *op.Policies {
			chatPolicies[p.Name] = true
		}
		assert.True(t, chatPolicies["GlobalSecurity"], "chat/* %s should have GlobalSecurity", method)
		assert.True(t, chatPolicies["GlobalMonitoring"], "chat/* %s should have GlobalMonitoring", method)
		assert.True(t, chatPolicies["ChatRootPolicy"], "chat/* %s should have ChatRootPolicy", method)
	}

	// Verify api/* GET and POST operations with APIRootPolicy and global policies
	for _, method := range []string{"GET", "POST"} {
		op := findOperation(spec.Operations, "api/*", method)
		require.NotNil(t, op, "api/* operation should exist for method %s", method)
		require.NotNil(t, op.Policies, "api/* %s should have policies", method)

		apiPolicies := make(map[string]bool)
		for _, p := range *op.Policies {
			apiPolicies[p.Name] = true
		}
		assert.True(t, apiPolicies["GlobalSecurity"], "api/* %s should have GlobalSecurity", method)
		assert.True(t, apiPolicies["GlobalMonitoring"], "api/* %s should have GlobalMonitoring", method)
		assert.True(t, apiPolicies["APIRootPolicy"], "api/* %s should have APIRootPolicy", method)
	}

	// Verify api/v1/* operations exist for all methods with APIV1Policy and APIRootPolicy (for GET/POST)
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		op := findOperation(spec.Operations, "api/v1/*", method)
		require.NotNil(t, op, "api/v1/* operation should exist for method %s", method)
		require.NotNil(t, op.Policies, "api/v1/* %s should have policies", method)

		v1Policies := make(map[string]bool)
		for _, p := range *op.Policies {
			v1Policies[p.Name] = true
		}
		assert.True(t, v1Policies["GlobalSecurity"], "api/v1/* %s should have GlobalSecurity", method)
		assert.True(t, v1Policies["GlobalMonitoring"], "api/v1/* %s should have GlobalMonitoring", method)
		assert.True(t, v1Policies["APIV1Policy"], "api/v1/* %s should have APIV1Policy", method)

		// For GET and POST, should also have APIRootPolicy
		if method == "GET" || method == "POST" {
			assert.True(t, v1Policies["APIRootPolicy"], "api/v1/* %s should have APIRootPolicy", method)
		}
	}

	// Verify health GET operation has only GlobalSecurity and GlobalMonitoring
	healthOp := findOperation(spec.Operations, "health", "GET")
	require.NotNil(t, healthOp, "health GET should exist")
	require.NotNil(t, healthOp.Policies, "health GET should have policies")
	require.Len(t, *healthOp.Policies, 2, "health GET should have exactly 2 policies")

	healthPolicies := make(map[string]bool)
	for _, p := range *healthOp.Policies {
		healthPolicies[p.Name] = true
	}
	assert.True(t, healthPolicies["GlobalSecurity"], "health GET should have GlobalSecurity")
	assert.True(t, healthPolicies["GlobalMonitoring"], "health GET should have GlobalMonitoring")

	// Verify NO catch-all operations in DenyAll mode
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, 0, catchAllCount, "DenyAll mode should have no catch-all operations")
}

// ============================================================================
// Policy Wildcard Attachment Tests - Section 10.4.1
// ============================================================================

func TestTransform_PolicyWildcard_AllowAll(t *testing.T) {
	// Policy with wildcard path (chat/*) in AllowAll mode
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"DELETE"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatWildcardPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/*", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"category": "chat"}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify admin/* exception operation exists with deny policy
	adminDeleteOp := findOperation(spec.Operations, "admin/*", "DELETE")
	require.NotNil(t, adminDeleteOp)
	require.NotNil(t, adminDeleteOp.Policies)
	require.Len(t, *adminDeleteOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminDeleteOp.Policies)[0].Name)

	// Verify chat/* wildcard operation exists with policy
	chatWildcardOp := findOperation(spec.Operations, "chat/*", "POST")
	require.NotNil(t, chatWildcardOp, "chat/* POST operation should exist")
	require.NotNil(t, chatWildcardOp.Policies)
	require.Len(t, *chatWildcardOp.Policies, 1)
	assert.Equal(t, "ChatWildcardPolicy", (*chatWildcardOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
			// Catch-all should have NO policies
			if op.Policies != nil {
				assert.Len(t, *op.Policies, 0)
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

func TestTransform_PolicyWildcard_DenyAll(t *testing.T) {
	// Policy with wildcard path (chat/*) in DenyAll mode
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "health", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatWildcardPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/*", Methods: []api.LLMPolicyPathMethods{"POST", "GET"}, Params: map[string]interface{}{"category": "chat"}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify chat/* operations exist for all HTTP methods (from exception)
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		op := findOperation(spec.Operations, "chat/*", method)
		require.NotNil(t, op, "chat/* operation should exist for method %s", method)

		// Only POST and GET should have the policy
		if method == "POST" || method == "GET" {
			require.NotNil(t, op.Policies)
			require.Len(t, *op.Policies, 1)
			assert.Equal(t, "ChatWildcardPolicy", (*op.Policies)[0].Name)
		} else {
			// Other methods should have NO policies
			if op.Policies != nil {
				assert.Len(t, *op.Policies, 0, "chat/* %s should have no policies", method)
			}
		}
	}

	// Verify health operation has NO policies
	healthOp := findOperation(spec.Operations, "health", "GET")
	require.NotNil(t, healthOp)
	if healthOp.Policies != nil {
		assert.Len(t, *healthOp.Policies, 0)
	}

	// Verify NO catch-all operations in DenyAll mode
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, 0, catchAllCount)
}

func TestTransform_PolicyWildcard_MatchingMultipleSpecificOperations_DenyAll(t *testing.T) {
	// Policy wildcard matching multiple specific operations
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/completions", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "chat/embeddings", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "chat/models", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "api/status", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatWildcardPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"applies": "to_all_chat"}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify all chat/* operations have the policy
	chatCompletionsOp := findOperation(spec.Operations, "chat/completions", "POST")
	require.NotNil(t, chatCompletionsOp)
	require.NotNil(t, chatCompletionsOp.Policies)
	require.Len(t, *chatCompletionsOp.Policies, 1)
	assert.Equal(t, "ChatWildcardPolicy", (*chatCompletionsOp.Policies)[0].Name)

	chatEmbeddingsOp := findOperation(spec.Operations, "chat/embeddings", "POST")
	require.NotNil(t, chatEmbeddingsOp)
	require.NotNil(t, chatEmbeddingsOp.Policies)
	require.Len(t, *chatEmbeddingsOp.Policies, 1)
	assert.Equal(t, "ChatWildcardPolicy", (*chatEmbeddingsOp.Policies)[0].Name)

	chatModelsOp := findOperation(spec.Operations, "chat/models", "GET")
	require.NotNil(t, chatModelsOp)
	require.NotNil(t, chatModelsOp.Policies)
	require.Len(t, *chatModelsOp.Policies, 1)
	assert.Equal(t, "ChatWildcardPolicy", (*chatModelsOp.Policies)[0].Name)

	// Verify api/status does NOT have the chat/* policy
	apiStatusOp := findOperation(spec.Operations, "api/status", "GET")
	require.NotNil(t, apiStatusOp)
	if apiStatusOp.Policies != nil {
		assert.Len(t, *apiStatusOp.Policies, 0, "api/status should not have chat/* policy")
	}
}

func TestTransform_PolicyWildcard_MatchingWildcardOperations_DenyAll(t *testing.T) {
	// Policy wildcard matching wildcard operations
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "chat/completions/*", Methods: []api.RouteExceptionMethods{"POST"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatRootPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/*", Methods: []api.LLMPolicyPathMethods{"POST", "GET"}, Params: map[string]interface{}{"level": "root"}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify chat/* operations have the policy for POST and GET
	chatPostOp := findOperation(spec.Operations, "chat/*", "POST")
	require.NotNil(t, chatPostOp)
	require.NotNil(t, chatPostOp.Policies)
	require.Len(t, *chatPostOp.Policies, 1)
	assert.Equal(t, "ChatRootPolicy", (*chatPostOp.Policies)[0].Name)

	chatGetOp := findOperation(spec.Operations, "chat/*", "GET")
	require.NotNil(t, chatGetOp)
	require.NotNil(t, chatGetOp.Policies)
	require.Len(t, *chatGetOp.Policies, 1)
	assert.Equal(t, "ChatRootPolicy", (*chatGetOp.Policies)[0].Name)

	// Verify chat/completions/* also has the policy (covered by chat/*)
	chatCompletionsWildcardOp := findOperation(spec.Operations, "chat/completions/*", "POST")
	require.NotNil(t, chatCompletionsWildcardOp)
	require.NotNil(t, chatCompletionsWildcardOp.Policies)
	require.Len(t, *chatCompletionsWildcardOp.Policies, 1)
	assert.Equal(t, "ChatRootPolicy", (*chatCompletionsWildcardOp.Policies)[0].Name)

	// Verify other methods on chat/* have NO policies
	chatPutOp := findOperation(spec.Operations, "chat/*", "PUT")
	require.NotNil(t, chatPutOp)
	if chatPutOp.Policies != nil {
		assert.Len(t, *chatPutOp.Policies, 0)
	}
}

func TestTransform_NestedPolicyWildcards_DenyAll(t *testing.T) {
	// Nested policy wildcards (chat/*, chat/completions/*)
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "chat/completions/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "chat/completions/stream", Methods: []api.RouteExceptionMethods{"POST"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatRootPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"level": 1}},
			},
		},
		{
			Name:    "ChatCompletionsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"level": 2}},
			},
		},
		{
			Name:    "StreamSpecificPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions/stream", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"level": 3}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify chat/* operations have only ChatRootPolicy
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		chatOp := findOperation(spec.Operations, "chat/*", method)
		require.NotNil(t, chatOp, "chat/* should exist for method %s", method)
		require.NotNil(t, chatOp.Policies)
		require.Len(t, *chatOp.Policies, 1)
		assert.Equal(t, "ChatRootPolicy", (*chatOp.Policies)[0].Name)
	}

	// Verify chat/completions/* has both ChatRootPolicy and ChatCompletionsPolicy
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		chatCompOp := findOperation(spec.Operations, "chat/completions/*", method)
		require.NotNil(t, chatCompOp, "chat/completions/* should exist for method %s", method)
		require.NotNil(t, chatCompOp.Policies)
		require.Len(t, *chatCompOp.Policies, 2)

		policyNames := make(map[string]bool)
		for _, p := range *chatCompOp.Policies {
			policyNames[p.Name] = true
		}
		assert.True(t, policyNames["ChatRootPolicy"], "Should have ChatRootPolicy (chat/* coverage)")
		assert.True(t, policyNames["ChatCompletionsPolicy"], "Should have ChatCompletionsPolicy (exact match)")
	}

	// Verify chat/completions/stream has all three policies
	streamOp := findOperation(spec.Operations, "chat/completions/stream", "POST")
	require.NotNil(t, streamOp)
	require.NotNil(t, streamOp.Policies)
	require.Len(t, *streamOp.Policies, 3)

	policyNames := make(map[string]bool)
	for _, p := range *streamOp.Policies {
		policyNames[p.Name] = true
	}
	assert.True(t, policyNames["ChatRootPolicy"], "Should have ChatRootPolicy (chat/* coverage)")
	assert.True(t, policyNames["ChatCompletionsPolicy"], "Should have ChatCompletionsPolicy (chat/completions/* coverage)")
	assert.True(t, policyNames["StreamSpecificPolicy"], "Should have StreamSpecificPolicy (exact match)")
}

func TestTransform_PolicyWildcard_OverlappingAccessControlWildcards_DenyAll(t *testing.T) {
	// Policy wildcard when access control has overlapping wildcards
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "api/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "api/v1/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "api/v1/chat/*", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "api/v1/models/*", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "APIRootPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/*", Methods: []api.LLMPolicyPathMethods{"GET", "POST"}, Params: map[string]interface{}{"api_level": "root"}},
			},
		},
		{
			Name:    "APIV1Policy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/v1/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"api_level": "v1"}},
			},
		},
		{
			Name:    "ChatSpecificPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/v1/chat/*", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"resource": "chat"}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify api/* GET and POST have only APIRootPolicy
	apiGetOp := findOperation(spec.Operations, "api/*", "GET")
	require.NotNil(t, apiGetOp)
	require.NotNil(t, apiGetOp.Policies)
	require.Len(t, *apiGetOp.Policies, 1)
	assert.Equal(t, "APIRootPolicy", (*apiGetOp.Policies)[0].Name)

	apiPostOp := findOperation(spec.Operations, "api/*", "POST")
	require.NotNil(t, apiPostOp)
	require.NotNil(t, apiPostOp.Policies)
	require.Len(t, *apiPostOp.Policies, 1)
	assert.Equal(t, "APIRootPolicy", (*apiPostOp.Policies)[0].Name)

	// Verify api/v1/* operations have nested policies
	// GET should have both APIRootPolicy and APIV1Policy
	apiV1GetOp := findOperation(spec.Operations, "api/v1/*", "GET")
	require.NotNil(t, apiV1GetOp)
	require.NotNil(t, apiV1GetOp.Policies)
	require.Len(t, *apiV1GetOp.Policies, 2)

	policyNames := make(map[string]bool)
	for _, p := range *apiV1GetOp.Policies {
		policyNames[p.Name] = true
	}
	assert.True(t, policyNames["APIRootPolicy"], "Should have APIRootPolicy (api/* coverage)")
	assert.True(t, policyNames["APIV1Policy"], "Should have APIV1Policy (exact match)")

	// POST should have both APIRootPolicy and APIV1Policy
	apiV1PostOp := findOperation(spec.Operations, "api/v1/*", "POST")
	require.NotNil(t, apiV1PostOp)
	require.NotNil(t, apiV1PostOp.Policies)
	require.Len(t, *apiV1PostOp.Policies, 2)

	policyNames = make(map[string]bool)
	for _, p := range *apiV1PostOp.Policies {
		policyNames[p.Name] = true
	}
	assert.True(t, policyNames["APIRootPolicy"], "Should have APIRootPolicy (api/* coverage)")
	assert.True(t, policyNames["APIV1Policy"], "Should have APIV1Policy (exact match)")

	// Verify api/v1/chat/* POST has all three nested policies
	apiV1ChatPostOp := findOperation(spec.Operations, "api/v1/chat/*", "POST")
	require.NotNil(t, apiV1ChatPostOp)
	require.NotNil(t, apiV1ChatPostOp.Policies)
	require.Len(t, *apiV1ChatPostOp.Policies, 3)

	policyNames = make(map[string]bool)
	for _, p := range *apiV1ChatPostOp.Policies {
		policyNames[p.Name] = true
	}
	assert.True(t, policyNames["APIRootPolicy"], "Should have APIRootPolicy (api/* coverage)")
	assert.True(t, policyNames["APIV1Policy"], "Should have APIV1Policy (api/v1/* coverage)")
	assert.True(t, policyNames["ChatSpecificPolicy"], "Should have ChatSpecificPolicy (exact match)")

	// Verify api/v1/models/* GET has APIRootPolicy and APIV1Policy (no ChatSpecificPolicy)
	apiV1ModelsGetOp := findOperation(spec.Operations, "api/v1/models/*", "GET")
	require.NotNil(t, apiV1ModelsGetOp)
	require.NotNil(t, apiV1ModelsGetOp.Policies)
	require.Len(t, *apiV1ModelsGetOp.Policies, 2)

	policyNames = make(map[string]bool)
	for _, p := range *apiV1ModelsGetOp.Policies {
		policyNames[p.Name] = true
	}
	assert.True(t, policyNames["APIRootPolicy"], "Should have APIRootPolicy (api/* coverage)")
	assert.True(t, policyNames["APIV1Policy"], "Should have APIV1Policy (api/v1/* coverage)")
	assert.False(t, policyNames["ChatSpecificPolicy"], "Should NOT have ChatSpecificPolicy (different path)")
}

func TestTransform_PolicyWildcard_AllowAll_WithExceptionPrecedence(t *testing.T) {
	// Policy wildcard in AllowAll mode with exception precedence
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "internal/debug/*", Methods: []api.RouteExceptionMethods{"GET", "POST"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "AdminWildcardPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "admin/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"blocked": true}},
			},
		},
		{
			Name:    "InternalPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "internal/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"internal": true}},
			},
		},
		{
			Name:    "ChatWildcardPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/*", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"allowed": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify admin/* exception operations have ONLY deny policy (exception precedence)
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		adminOp := findOperation(spec.Operations, "admin/*", method)
		require.NotNil(t, adminOp, "admin/* operation should exist for method %s", method)
		require.NotNil(t, adminOp.Policies)
		require.Len(t, *adminOp.Policies, 1, "admin/* should have only deny policy, not user policy")
		assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminOp.Policies)[0].Name)
	}

	// Verify internal/debug/* has ONLY deny policy for GET and POST (exception precedence)
	internalDebugGetOp := findOperation(spec.Operations, "internal/debug/*", "GET")
	require.NotNil(t, internalDebugGetOp)
	require.NotNil(t, internalDebugGetOp.Policies)
	require.Len(t, *internalDebugGetOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalDebugGetOp.Policies)[0].Name)

	internalDebugPostOp := findOperation(spec.Operations, "internal/debug/*", "POST")
	require.NotNil(t, internalDebugPostOp)
	require.NotNil(t, internalDebugPostOp.Policies)
	require.Len(t, *internalDebugPostOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalDebugPostOp.Policies)[0].Name)

	// Verify chat/* wildcard operation has user policy (not blocked by exception)
	chatWildcardOp := findOperation(spec.Operations, "chat/*", "POST")
	require.NotNil(t, chatWildcardOp)
	require.NotNil(t, chatWildcardOp.Policies)
	require.Len(t, *chatWildcardOp.Policies, 1)
	assert.Equal(t, "ChatWildcardPolicy", (*chatWildcardOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
			if op.Policies != nil {
				assert.Len(t, *op.Policies, 0)
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

// ============================================================================
// Dynamic Operation Creation Tests - Section 10.4.2
// ============================================================================

func TestTransform_DynamicOperationCreation_PolicyOnSpecific_AccessControlWildcard_DenyAll(t *testing.T) {
	// DenyAll: Policy on chat/completions when only chat/* in access control
	// Should create BOTH chat/* and chat/completions operations
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/*", Methods: []api.RouteExceptionMethods{"POST"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "CompletionsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"specific": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify both operations exist
	chatWildcardOp := findOperation(spec.Operations, "chat/*", "POST")
	require.NotNil(t, chatWildcardOp, "chat/* operation should exist from access control")

	chatCompletionsOp := findOperation(spec.Operations, "chat/completions", "POST")
	require.NotNil(t, chatCompletionsOp, "chat/completions operation should be created dynamically for policy")

	// Verify policy is ONLY on specific operation, not wildcard
	if chatWildcardOp.Policies != nil {
		assert.Len(t, *chatWildcardOp.Policies, 0, "chat/* should have NO policies")
	}

	require.NotNil(t, chatCompletionsOp.Policies)
	require.Len(t, *chatCompletionsOp.Policies, 1)
	assert.Equal(t, "CompletionsPolicy", (*chatCompletionsOp.Policies)[0].Name)

	// Verify correct count (2 operations total)
	assert.Equal(t, 2, len(spec.Operations))
}

func TestTransform_DynamicOperationCreation_MultiplePolicies_SameOperation_DenyAll(t *testing.T) {
	// DenyAll: Multiple policies creating same operation dynamically
	// Should create single operation with both policies attached
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "api/*", Methods: []api.RouteExceptionMethods{"*"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ModelRateLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/models", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"rps": 100}},
			},
		},
		{
			Name:    "ModelAuth",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/models", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"required": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify api/models GET operation exists (created dynamically)
	apiModelsOp := findOperation(spec.Operations, "api/models", "GET")
	require.NotNil(t, apiModelsOp, "api/models GET operation should be created dynamically")

	// Verify BOTH policies are attached
	require.NotNil(t, apiModelsOp.Policies)
	require.Len(t, *apiModelsOp.Policies, 2)

	policyNames := make(map[string]bool)
	for _, p := range *apiModelsOp.Policies {
		policyNames[p.Name] = true
	}
	assert.True(t, policyNames["ModelRateLimit"], "Should have ModelRateLimit policy")
	assert.True(t, policyNames["ModelAuth"], "Should have ModelAuth policy")

	// Verify api/* operations also exist
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		apiWildcardOp := findOperation(spec.Operations, "api/*", method)
		require.NotNil(t, apiWildcardOp, "api/* should exist for method %s", method)
	}

	// Count operations: 6 api/* operations + 1 api/models = 7 total
	assert.Equal(t, 7, len(spec.Operations))
}

func TestTransform_DynamicOperationCreation_AllowAll_PolicyCreatesOperation(t *testing.T) {
	// AllowAll: Policy creating new operation (not exception, not catch-all)
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"*"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "CompletionsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"maxTokens": 2000}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify chat/completions operation was created dynamically
	chatCompletionsOp := findOperation(spec.Operations, "chat/completions", "POST")
	require.NotNil(t, chatCompletionsOp, "chat/completions POST should be created dynamically by policy")

	// Verify policy is attached
	require.NotNil(t, chatCompletionsOp.Policies)
	require.Len(t, *chatCompletionsOp.Policies, 1)
	assert.Equal(t, "CompletionsPolicy", (*chatCompletionsOp.Policies)[0].Name)

	// Verify exception operations exist with deny policy
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		adminOp := findOperation(spec.Operations, "admin/*", method)
		require.NotNil(t, adminOp, "admin/* should exist for method %s", method)
		require.NotNil(t, adminOp.Policies)
		require.Len(t, *adminOp.Policies, 1)
		assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminOp.Policies)[0].Name)
	}

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)

	// Total: 1 chat/completions + 6 admin/* + 6 catch-all = 13 operations
	assert.Equal(t, 13, len(spec.Operations))
}

func TestTransform_DynamicOperationCreation_OperationRegistry_PreventsDuplicates_DenyAll(t *testing.T) {
	// Verify operation registry prevents duplicates when both access control and policy target same path
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/completions", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "chat/*", Methods: []api.RouteExceptionMethods{"POST"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "CompletionsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"specific": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Count chat/completions POST operations - should be exactly 1
	completionsCount := 0
	var completionsOp *api.Operation
	for i := range spec.Operations {
		if spec.Operations[i].Path == "chat/completions" && string(spec.Operations[i].Method) == "POST" {
			completionsCount++
			completionsOp = &spec.Operations[i]
		}
	}

	assert.Equal(t, 1, completionsCount, "Should have exactly ONE chat/completions POST operation (no duplicates)")
	require.NotNil(t, completionsOp)

	// Verify policy is attached to the single operation
	require.NotNil(t, completionsOp.Policies)
	require.Len(t, *completionsOp.Policies, 1)
	assert.Equal(t, "CompletionsPolicy", (*completionsOp.Policies)[0].Name)

	// Verify chat/* operation also exists
	chatWildcardOp := findOperation(spec.Operations, "chat/*", "POST")
	require.NotNil(t, chatWildcardOp, "chat/* POST should exist")

	// Total: 1 chat/completions + 1 chat/* = 2 operations
	assert.Equal(t, 2, len(spec.Operations))
}

func TestTransform_DynamicOperationCreation_NestedSpecificPaths_DenyAll(t *testing.T) {
	// Multiple policies on nested specific paths under same wildcard access control
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "api/*", Methods: []api.RouteExceptionMethods{"*"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "CompletionsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"level": 3}},
			},
		},
		{
			Name:    "ModelsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/models", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"level": 2}},
			},
		},
		{
			Name:    "EmbeddingsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/embeddings", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"level": 2}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify all specific operations were created dynamically
	chatCompletionsOp := findOperation(spec.Operations, "api/chat/completions", "POST")
	require.NotNil(t, chatCompletionsOp, "api/chat/completions POST should be created")
	require.NotNil(t, chatCompletionsOp.Policies)
	require.Len(t, *chatCompletionsOp.Policies, 1)
	assert.Equal(t, "CompletionsPolicy", (*chatCompletionsOp.Policies)[0].Name)

	modelsOp := findOperation(spec.Operations, "api/models", "GET")
	require.NotNil(t, modelsOp, "api/models GET should be created")
	require.NotNil(t, modelsOp.Policies)
	require.Len(t, *modelsOp.Policies, 1)
	assert.Equal(t, "ModelsPolicy", (*modelsOp.Policies)[0].Name)

	embeddingsOp := findOperation(spec.Operations, "api/embeddings", "POST")
	require.NotNil(t, embeddingsOp, "api/embeddings POST should be created")
	require.NotNil(t, embeddingsOp.Policies)
	require.Len(t, *embeddingsOp.Policies, 1)
	assert.Equal(t, "EmbeddingsPolicy", (*embeddingsOp.Policies)[0].Name)

	// Verify api/* operations exist
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		apiWildcardOp := findOperation(spec.Operations, "api/*", method)
		require.NotNil(t, apiWildcardOp, "api/* should exist for method %s", method)
	}

	// Total: 3 specific operations + 6 api/* wildcard = 9 operations
	assert.Equal(t, 9, len(spec.Operations))
}

func TestTransform_DynamicOperationCreation_AllowAll_MultipleSpecificPolicies(t *testing.T) {
	// AllowAll: Multiple policies creating different specific operations
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"DELETE"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "CompletionsGuard",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"guard": "content"}},
			},
		},
		{
			Name:    "EmbeddingsLimit",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "embeddings", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"limit": 1000}},
			},
		},
		{
			Name:    "ModelsCache",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "models", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"ttl": 300}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify all specific operations were created dynamically
	chatCompletionsOp := findOperation(spec.Operations, "chat/completions", "POST")
	require.NotNil(t, chatCompletionsOp, "chat/completions POST should be created")
	require.NotNil(t, chatCompletionsOp.Policies)
	require.Len(t, *chatCompletionsOp.Policies, 1)
	assert.Equal(t, "CompletionsGuard", (*chatCompletionsOp.Policies)[0].Name)

	embeddingsOp := findOperation(spec.Operations, "embeddings", "POST")
	require.NotNil(t, embeddingsOp, "embeddings POST should be created")
	require.NotNil(t, embeddingsOp.Policies)
	require.Len(t, *embeddingsOp.Policies, 1)
	assert.Equal(t, "EmbeddingsLimit", (*embeddingsOp.Policies)[0].Name)

	modelsOp := findOperation(spec.Operations, "models", "GET")
	require.NotNil(t, modelsOp, "models GET should be created")
	require.NotNil(t, modelsOp.Policies)
	require.Len(t, *modelsOp.Policies, 1)
	assert.Equal(t, "ModelsCache", (*modelsOp.Policies)[0].Name)

	// Verify exception operation exists
	adminDeleteOp := findOperation(spec.Operations, "admin/*", "DELETE")
	require.NotNil(t, adminDeleteOp)
	require.NotNil(t, adminDeleteOp.Policies)
	require.Len(t, *adminDeleteOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminDeleteOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)

	// Total: 3 specific + 1 exception + 6 catch-all = 10 operations
	assert.Equal(t, 10, len(spec.Operations))
}

// ============================================================================
// Operation Sorting Validation Tests - Section 10.4.3
// ============================================================================

func TestTransform_OperationSorting_NonWildcardBeforeWildcard_DenyAll(t *testing.T) {
	// Verify non-wildcard paths come before wildcard paths
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/*", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "chat/completions", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "models/*", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "models/list", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify operation count
	assert.Equal(t, 4, len(spec.Operations))

	// Verify ordering: specific paths before wildcards
	// Expected order: chat/completions, models/list, chat/*, models/*
	assert.Equal(t, "chat/completions", spec.Operations[0].Path, "First should be chat/completions (specific)")
	assert.Equal(t, "models/list", spec.Operations[1].Path, "Second should be models/list (specific)")
	assert.Equal(t, "models/*", spec.Operations[2].Path, "Third should be models/* (wildcard)")
	assert.Equal(t, "chat/*", spec.Operations[3].Path, "Fourth should be chat/* (wildcard)")

	// All should be GET method
	for _, op := range spec.Operations {
		assert.Equal(t, "GET", string(op.Method))
	}
}

func TestTransform_OperationSorting_LongerPathsFirst_DenyAll(t *testing.T) {
	// Verify longer (more specific) paths come before shorter paths
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "api/v1/chat/completions/stream", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "api/v1/chat/completions", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "api/v1/chat", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "api/v1", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "api", Methods: []api.RouteExceptionMethods{"POST"}},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify operation count
	assert.Equal(t, 5, len(spec.Operations))

	// Verify ordering: longest paths first (most specific)
	assert.Equal(t, "api/v1/chat/completions/stream", spec.Operations[0].Path, "First should be longest path")
	assert.Equal(t, "api/v1/chat/completions", spec.Operations[1].Path, "Second should be second longest")
	assert.Equal(t, "api/v1/chat", spec.Operations[2].Path, "Third should be third longest")
	assert.Equal(t, "api/v1", spec.Operations[3].Path, "Fourth should be fourth longest")
	assert.Equal(t, "api", spec.Operations[4].Path, "Fifth should be shortest")

	// Verify path lengths are strictly decreasing
	for i := 0; i < len(spec.Operations)-1; i++ {
		assert.Greater(t, len(spec.Operations[i].Path), len(spec.Operations[i+1].Path),
			"Path at index %d should be longer than path at index %d", i, i+1)
	}
}

func TestTransform_OperationSorting_CatchAllLast_AllowAll(t *testing.T) {
	// Verify catch-all operations (/*) are last in AllowAll mode
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"DELETE"}},
		{Path: "internal/debug", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"enabled": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Find where catch-all operations start
	catchAllStartIndex := -1
	for i, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllStartIndex = i
			break
		}
	}

	require.NotEqual(t, -1, catchAllStartIndex, "Catch-all operations should exist")

	// Verify all operations before catch-all are NOT catch-all
	for i := 0; i < catchAllStartIndex; i++ {
		assert.NotEqual(t, "/*", spec.Operations[i].Path,
			"Operation at index %d should not be catch-all", i)
	}

	// Verify all operations from catch-all start to end ARE catch-all
	for i := catchAllStartIndex; i < len(spec.Operations); i++ {
		assert.Equal(t, "/*", spec.Operations[i].Path,
			"Operation at index %d should be catch-all", i)
	}

	// Verify we have exactly 6 catch-all operations (one per HTTP method)
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

// @TODO
//func TestTransform_OperationSorting_StableSortingSameLength_DenyAll(t *testing.T) {
//	// Verify stable sorting when paths have same length (lexicographic order)
//	transformer, _ := setupTestTransformer(t)
//
//	exceptions := []api.RouteException{
//		{Path: "zebra", Methods: []api.RouteExceptionMethods{"GET"}},
//		{Path: "apple", Methods: []api.RouteExceptionMethods{"GET"}},
//		{Path: "mango", Methods: []api.RouteExceptionMethods{"GET"}},
//		{Path: "banana", Methods: []api.RouteExceptionMethods{"GET"}},
//	}
//
//	provider := &api.LLMProviderConfiguration{
//		ApiVersion:  "gateway.api-platform.wso2.com/v1alpha1",
//		Kind:     "LlmProvider",
//		Metadata: api.Metadata{Name: "openai-provider"},
//		Spec: api.LLMProviderConfigData{
//			DisplayName: "test-provider",
//			Version:     "v1.0",
//			Template:    "openai",
//			Upstream: api.LLMProviderConfigData_Upstream{
//				Url: stringPtr("https://api.openai.com"),
//			},
//			AccessControl: api.LLMAccessControl{
//				Mode:       api.DenyAll,
//				Exceptions: &exceptions,
//			},
//		},
//	}
//
//	output := &api.APIConfiguration{}
//	result, err := transformer.Transform(provider, output)
//	require.NoError(t, err)
//
//	spec, err := result.Spec.AsAPIConfigData()
//	require.NoError(t, err)
//
//	// Verify operation count
//	assert.Equal(t, 4, len(spec.Operations))
//
//	// All paths have same length (5 characters), should be sorted lexicographically
//	assert.Equal(t, "apple", spec.Operations[0].Path)
//	assert.Equal(t, "banana", spec.Operations[1].Path)
//	assert.Equal(t, "mango", spec.Operations[2].Path)
//	assert.Equal(t, "zebra", spec.Operations[3].Path)
//
//	// Verify lexicographic ordering
//	for i := 0; i < len(spec.Operations)-1; i++ {
//		assert.Less(t, spec.Operations[i].Path, spec.Operations[i+1].Path,
//			"Path at index %d should be lexicographically before path at index %d", i, i+1)
//	}
//}

func TestTransform_OperationSorting_ComplexMultipleWildcardLevels_DenyAll(t *testing.T) {
	// Complex sorting with multiple wildcard levels
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "api/*", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "api/v1/chat/completions/stream", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "api/v1/*", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "api/v1/chat", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "api/v1/chat/*", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "api/v1/chat/completions", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "models", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify operation count
	assert.Equal(t, 7, len(spec.Operations))

	// Expected order (most specific to least specific):
	// 1. api/v1/chat/completions/stream (longest, no wildcard)
	// 2. api/v1/chat/completions (second longest, no wildcard)
	// 3. api/v1/chat (third longest, no wildcard)
	// 4. models (no wildcard, shorter)
	// 5. api/v1/chat/* (wildcard, but more specific than api/v1/*)
	// 6. api/v1/* (wildcard, but more specific than api/*)
	// 7. api/* (least specific wildcard)

	assert.Equal(t, "api/v1/chat/completions/stream", spec.Operations[0].Path)
	assert.Equal(t, "api/v1/chat/completions", spec.Operations[1].Path)
	assert.Equal(t, "api/v1/chat", spec.Operations[2].Path)
	assert.Equal(t, "models", spec.Operations[3].Path)
	assert.Equal(t, "api/v1/chat/*", spec.Operations[4].Path)
	assert.Equal(t, "api/v1/*", spec.Operations[5].Path)
	assert.Equal(t, "api/*", spec.Operations[6].Path)

	// Verify non-wildcard operations come before wildcard operations
	firstWildcardIndex := -1
	for i, op := range spec.Operations {
		if len(op.Path) > 0 && op.Path[len(op.Path)-1] == '*' {
			firstWildcardIndex = i
			break
		}
	}
	assert.Equal(t, 4, firstWildcardIndex, "First wildcard should be at index 4")

	// All operations before index 4 should be non-wildcard
	for i := 0; i < firstWildcardIndex; i++ {
		assert.False(t, len(spec.Operations[i].Path) > 0 && spec.Operations[i].Path[len(spec.Operations[i].Path)-1] == '*',
			"Operation at index %d should not be wildcard", i)
	}

	// All operations from index 4 onwards should be wildcard
	for i := firstWildcardIndex; i < len(spec.Operations); i++ {
		assert.True(t, len(spec.Operations[i].Path) > 0 && spec.Operations[i].Path[len(spec.Operations[i].Path)-1] == '*',
			"Operation at index %d should be wildcard", i)
	}
}

func TestTransform_OperationSorting_MixedMethodsSamePath_DenyAll(t *testing.T) {
	// Verify operations with same path are sorted by method alphabetically
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/completions", Methods: []api.RouteExceptionMethods{"*"}},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Should have 6 operations (one per HTTP method)
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), len(spec.Operations))

	// All should have same path
	for _, op := range spec.Operations {
		assert.Equal(t, "chat/completions", op.Path)
	}

	// Verify methods are sorted alphabetically
	expectedMethods := []string{"DELETE", "GET", "OPTIONS", "PATCH", "POST", "PUT"}
	for i, op := range spec.Operations {
		assert.Equal(t, expectedMethods[i], string(op.Method),
			"Method at index %d should be %s", i, expectedMethods[i])
	}

	// Verify alphabetical ordering
	for i := 0; i < len(spec.Operations)-1; i++ {
		assert.Less(t, string(spec.Operations[i].Method), string(spec.Operations[i+1].Method),
			"Method at index %d should be alphabetically before method at index %d", i, i+1)
	}
}

func TestTransform_OperationSorting_AllowAll_ComplexMixedOperations(t *testing.T) {
	// Complex AllowAll sorting: exceptions, policies, catch-all
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "internal/debug/logs", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "internal/*", Methods: []api.RouteExceptionMethods{"DELETE"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "CompletionsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"enabled": true}},
			},
		},
		{
			Name:    "ModelsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "models", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"cache": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Identify operation types
	specificPaths := []string{}
	wildcardPaths := []string{}
	catchAllPaths := []string{}

	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllPaths = append(catchAllPaths, op.Path)
		} else if len(op.Path) > 0 && op.Path[len(op.Path)-1] == '*' {
			wildcardPaths = append(wildcardPaths, op.Path)
		} else {
			specificPaths = append(specificPaths, op.Path)
		}
	}

	// Verify structure: specific paths, then wildcards, then catch-all
	// Find indices
	firstWildcardIndex := -1
	firstCatchAllIndex := -1

	for i, op := range spec.Operations {
		if firstWildcardIndex == -1 && len(op.Path) > 0 && op.Path[len(op.Path)-1] == '*' && op.Path != "/*" {
			firstWildcardIndex = i
		}
		if firstCatchAllIndex == -1 && op.Path == "/*" {
			firstCatchAllIndex = i
			break
		}
	}

	// If we have wildcards, verify they come after specific paths
	if firstWildcardIndex != -1 {
		for i := 0; i < firstWildcardIndex; i++ {
			assert.False(t, len(spec.Operations[i].Path) > 0 && spec.Operations[i].Path[len(spec.Operations[i].Path)-1] == '*',
				"Operation at index %d should not be wildcard (before first wildcard at %d)", i, firstWildcardIndex)
		}
	}

	// Verify catch-all operations are last
	if firstCatchAllIndex != -1 {
		for i := firstCatchAllIndex; i < len(spec.Operations); i++ {
			assert.Equal(t, "/*", spec.Operations[i].Path,
				"Operation at index %d should be catch-all", i)
		}
	}

	// Verify we have catch-all operations
	assert.NotEmpty(t, catchAllPaths, "Should have catch-all operations in AllowAll mode")
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), len(catchAllPaths))
}

func TestTransform_OperationSorting_SpecificityPreservation_DenyAll(t *testing.T) {
	// Verify specificity is preserved: exact > longer > shorter > wildcard
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "a", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "a/b", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "a/b/c", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "a/b/c/d", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "a/*", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "a/b/*", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Expected order (most specific to least specific):
	// 1. a/b/c/d (longest, no wildcard)
	// 2. a/b/c (second longest, no wildcard)
	// 3. a/b (third longest, no wildcard)
	// 4. a (shortest, no wildcard)
	// 5. a/b/* (wildcard, longer)
	// 6. a/* (wildcard, shorter)

	assert.Equal(t, 6, len(spec.Operations))

	assert.Equal(t, "a/b/c/d", spec.Operations[0].Path)
	assert.Equal(t, "a/b/c", spec.Operations[1].Path)
	assert.Equal(t, "a/b", spec.Operations[2].Path)
	assert.Equal(t, "a", spec.Operations[3].Path)
	assert.Equal(t, "a/b/*", spec.Operations[4].Path)
	assert.Equal(t, "a/*", spec.Operations[5].Path)

	// Verify no wildcard in first 4 operations
	for i := 0; i < 4; i++ {
		assert.False(t, len(spec.Operations[i].Path) > 0 && spec.Operations[i].Path[len(spec.Operations[i].Path)-1] == '*',
			"Operation at index %d should not be wildcard", i)
	}

	// Verify wildcard in last 2 operations
	for i := 4; i < 6; i++ {
		assert.True(t, len(spec.Operations[i].Path) > 0 && spec.Operations[i].Path[len(spec.Operations[i].Path)-1] == '*',
			"Operation at index %d should be wildcard", i)
	}
}

// ============================================================================
// AllowAll Policy Application Flow Tests - Section 10.4.4
// ============================================================================

func TestTransform_AllowAll_UserPolicyOnCatchAll_NotDenied(t *testing.T) {
	// User policy attached to catch-all when not denied
	transformer, _ := setupTestTransformer(t)

	// No exceptions - everything allowed
	exceptions := []api.RouteException{}

	policies := []api.LLMPolicy{
		{
			Name:    "CatchAllLogger",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "monitor/*", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"log": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify monitor/* operation was created with policy
	monitorOp := findOperation(spec.Operations, "monitor/*", "GET")
	require.NotNil(t, monitorOp, "monitor/* GET operation should be created for policy")
	require.NotNil(t, monitorOp.Policies)
	require.Len(t, *monitorOp.Policies, 1)
	assert.Equal(t, "CatchAllLogger", (*monitorOp.Policies)[0].Name)

	// Verify catch-all operations exist (no exceptions to deny)
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
			// Catch-all should have NO policies
			if op.Policies != nil {
				assert.Len(t, *op.Policies, 0)
			}
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

func TestTransform_AllowAll_UserPolicyOnSpecificOperation_NotDenied(t *testing.T) {
	// User policy attached to specific operation when not denied
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"*"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "CompletionsGuard",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"guard": "content"}},
			},
		},
		{
			Name:    "ModelsCache",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "models", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"ttl": 300}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify chat/completions operation was created with policy (not denied)
	chatCompletionsOp := findOperation(spec.Operations, "chat/completions", "POST")
	require.NotNil(t, chatCompletionsOp, "chat/completions POST should be created")
	require.NotNil(t, chatCompletionsOp.Policies)
	require.Len(t, *chatCompletionsOp.Policies, 1)
	assert.Equal(t, "CompletionsGuard", (*chatCompletionsOp.Policies)[0].Name)

	// Verify models operation was created with policy (not denied)
	modelsOp := findOperation(spec.Operations, "models", "GET")
	require.NotNil(t, modelsOp, "models GET should be created")
	require.NotNil(t, modelsOp.Policies)
	require.Len(t, *modelsOp.Policies, 1)
	assert.Equal(t, "ModelsCache", (*modelsOp.Policies)[0].Name)

	// Verify admin/* exception operations have ONLY deny policy
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		adminOp := findOperation(spec.Operations, "admin/*", method)
		require.NotNil(t, adminOp, "admin/* should exist for method %s", method)
		require.NotNil(t, adminOp.Policies)
		require.Len(t, *adminOp.Policies, 1)
		assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminOp.Policies)[0].Name)
	}

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

func TestTransform_AllowAll_DenyPolicyPreventsUserPolicy(t *testing.T) {
	// hasDenyPolicy() correctly identifies and skips denied operations
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "internal/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "admin/users", Methods: []api.RouteExceptionMethods{"DELETE", "POST"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "InternalPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "internal/health", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"check": true}},
			},
		},
		{
			Name:    "UsersPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "admin/users", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"audit": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify internal/health does not exist (covered by internal/* exception)
	internalHealthOp := findOperation(spec.Operations, "internal/health", "GET")
	require.Nil(t, internalHealthOp, "internal/health GET should not exist")

	// Verify internal/* exception has ONLY deny policy
	internalExceptionOp := findOperation(spec.Operations, "internal/*", "GET")
	require.NotNil(t, internalExceptionOp, "internal/* should exist for GET method")
	require.NotNil(t, internalExceptionOp.Policies)
	require.Len(t, *internalExceptionOp.Policies, 1, "Should have only deny policy, not user policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalExceptionOp.Policies)[0].Name)

	// Verify internal/* wildcard operations have only deny policy
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		internalWildcardOp := findOperation(spec.Operations, "internal/*", method)
		require.NotNil(t, internalWildcardOp, "internal/* should exist for method %s", method)
		require.NotNil(t, internalWildcardOp.Policies)
		require.Len(t, *internalWildcardOp.Policies, 1)
		assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalWildcardOp.Policies)[0].Name)
	}

	// Verify admin/users DELETE and POST have ONLY deny policy (exception precedence)
	adminDeleteOp := findOperation(spec.Operations, "admin/users", "DELETE")
	require.NotNil(t, adminDeleteOp)
	require.NotNil(t, adminDeleteOp.Policies)
	require.Len(t, *adminDeleteOp.Policies, 1, "DELETE should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminDeleteOp.Policies)[0].Name)

	adminPostOp := findOperation(spec.Operations, "admin/users", "POST")
	require.NotNil(t, adminPostOp)
	require.NotNil(t, adminPostOp.Policies)
	require.Len(t, *adminPostOp.Policies, 1, "POST should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminPostOp.Policies)[0].Name)

	// Verify admin/users GET has user policy (not in exceptions)
	adminGetOp := findOperation(spec.Operations, "admin/users", "GET")
	require.NotNil(t, adminGetOp)
	require.NotNil(t, adminGetOp.Policies)
	require.Len(t, *adminGetOp.Policies, 1)
	assert.Equal(t, "UsersPolicy", (*adminGetOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

func TestTransform_AllowAll_PolicyOnAllowedPath_MixedExceptions(t *testing.T) {
	// Policy on allowed path with mixed exceptions (wildcard and specific)
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "admin/*", Methods: []api.RouteExceptionMethods{"DELETE"}},
		{Path: "internal/debug", Methods: []api.RouteExceptionMethods{"GET", "POST"}},
		{Path: "system/*", Methods: []api.RouteExceptionMethods{"*"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "ChatPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"enabled": true}},
			},
		},
		{
			Name:    "AdminPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "admin/users", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"allowed": true}},
			},
		},
		{
			Name:    "InternalPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "internal/health", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"check": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify chat/completions has user policy (not covered by any exception)
	chatCompletionsOp := findOperation(spec.Operations, "chat/completions", "POST")
	require.NotNil(t, chatCompletionsOp)
	require.NotNil(t, chatCompletionsOp.Policies)
	require.Len(t, *chatCompletionsOp.Policies, 1)
	assert.Equal(t, "ChatPolicy", (*chatCompletionsOp.Policies)[0].Name)

	// Verify admin/users GET has user policy (not in DELETE exception)
	adminUsersOp := findOperation(spec.Operations, "admin/users", "GET")
	require.NotNil(t, adminUsersOp)
	require.NotNil(t, adminUsersOp.Policies)
	require.Len(t, *adminUsersOp.Policies, 1)
	assert.Equal(t, "AdminPolicy", (*adminUsersOp.Policies)[0].Name)

	// Verify admin/* DELETE has only deny policy
	adminDeleteOp := findOperation(spec.Operations, "admin/*", "DELETE")
	require.NotNil(t, adminDeleteOp)
	require.NotNil(t, adminDeleteOp.Policies)
	require.Len(t, *adminDeleteOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminDeleteOp.Policies)[0].Name)

	// Verify internal/health GET has user policy (not in exception - exception is for internal/debug)
	internalHealthOp := findOperation(spec.Operations, "internal/health", "GET")
	require.NotNil(t, internalHealthOp)
	require.NotNil(t, internalHealthOp.Policies)
	require.Len(t, *internalHealthOp.Policies, 1)
	assert.Equal(t, "InternalPolicy", (*internalHealthOp.Policies)[0].Name)

	// Verify internal/debug GET and POST have only deny policy
	internalDebugGetOp := findOperation(spec.Operations, "internal/debug", "GET")
	require.NotNil(t, internalDebugGetOp)
	require.NotNil(t, internalDebugGetOp.Policies)
	require.Len(t, *internalDebugGetOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalDebugGetOp.Policies)[0].Name)

	internalDebugPostOp := findOperation(spec.Operations, "internal/debug", "POST")
	require.NotNil(t, internalDebugPostOp)
	require.NotNil(t, internalDebugPostOp.Policies)
	require.Len(t, *internalDebugPostOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalDebugPostOp.Policies)[0].Name)

	// Verify system/* has deny policy for all methods
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		systemOp := findOperation(spec.Operations, "system/*", method)
		require.NotNil(t, systemOp, "system/* should exist for method %s", method)
		require.NotNil(t, systemOp.Policies)
		require.Len(t, *systemOp.Policies, 1)
		assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*systemOp.Policies)[0].Name)
	}

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

func TestTransform_AllowAll_WildcardPolicyWithExceptions(t *testing.T) {
	// Wildcard policy application with multiple exceptions
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "api/admin/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "api/internal/*", Methods: []api.RouteExceptionMethods{"DELETE", "POST"}},
		{Path: "api/spec/*", Methods: []api.RouteExceptionMethods{"GET", "PUT"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "APIWildcardPolicy_4",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/admin/chat", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"track": true}},
			},
		},
		{
			Name:    "APIWildcardPolicy_3",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/admin/*", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"track": true}},
			},
		},
		{
			Name:    "APIWildcardPolicy_5",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/user", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"track": true}},
			},
		},
		{
			Name:    "APIWildcardPolicy_1",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"track": true}},
			},
		},
		{
			Name:    "APIWildcardPolicy_2",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/*", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"track": true}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// 1. Verify all exception routes have ONLY deny policy for relevant methods

	// api/admin/* should have deny policy for all methods (wildcard exception)
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		apiAdminOp := findOperation(spec.Operations, "api/admin/*", method)
		require.NotNil(t, apiAdminOp, "api/admin/* should exist for method %s", method)
		require.NotNil(t, apiAdminOp.Policies)
		require.Len(t, *apiAdminOp.Policies, 1, "api/admin/* %s should have only deny policy", method)
		assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*apiAdminOp.Policies)[0].Name)
	}

	// api/internal/* should have deny policy for DELETE and POST only
	apiInternalDeleteOp := findOperation(spec.Operations, "api/internal/*", "DELETE")
	require.NotNil(t, apiInternalDeleteOp, "api/internal/* DELETE should exist")
	require.NotNil(t, apiInternalDeleteOp.Policies)
	require.Len(t, *apiInternalDeleteOp.Policies, 1, "api/internal/* DELETE should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*apiInternalDeleteOp.Policies)[0].Name)

	apiInternalPostOp := findOperation(spec.Operations, "api/internal/*", "POST")
	require.NotNil(t, apiInternalPostOp, "api/internal/* POST should exist")
	require.NotNil(t, apiInternalPostOp.Policies)
	require.Len(t, *apiInternalPostOp.Policies, 1, "api/internal/* POST should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*apiInternalPostOp.Policies)[0].Name)

	// api/spec/* should have deny policy for GET and PUT only
	apiSpecGetOp := findOperation(spec.Operations, "api/spec/*", "GET")
	require.NotNil(t, apiSpecGetOp, "api/spec/* GET should exist")
	require.NotNil(t, apiSpecGetOp.Policies)
	require.Len(t, *apiSpecGetOp.Policies, 1, "api/spec/* GET should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*apiSpecGetOp.Policies)[0].Name)

	apiSpecPutOp := findOperation(spec.Operations, "api/spec/*", "PUT")
	require.NotNil(t, apiSpecPutOp, "api/spec/* PUT should exist")
	require.NotNil(t, apiSpecPutOp.Policies)
	require.Len(t, *apiSpecPutOp.Policies, 1, "api/spec/* PUT should have only deny policy")
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*apiSpecPutOp.Policies)[0].Name)

	// 2. Verify api/* POST has both APIWildcardPolicy_1 and APIWildcardPolicy_2
	apiWildcardPostOp := findOperation(spec.Operations, "api/*", "POST")
	require.NotNil(t, apiWildcardPostOp, "api/* POST should exist")
	require.NotNil(t, apiWildcardPostOp.Policies)
	require.Len(t, *apiWildcardPostOp.Policies, 2, "api/* POST should have both APIWildcardPolicy_1 and APIWildcardPolicy_2")

	postPolicyNames := make(map[string]bool)
	for _, p := range *apiWildcardPostOp.Policies {
		postPolicyNames[p.Name] = true
	}
	assert.True(t, postPolicyNames["APIWildcardPolicy_1"], "api/* POST should have APIWildcardPolicy_1")
	assert.True(t, postPolicyNames["APIWildcardPolicy_2"], "api/* POST should have APIWildcardPolicy_2")

	// Verify api/* other methods (GET, PUT, PATCH, DELETE, OPTIONS) have only APIWildcardPolicy_1
	otherApiMethods := []string{"GET", "PUT", "PATCH", "DELETE", "OPTIONS"}
	for _, method := range otherApiMethods {
		apiWildcardOp := findOperation(spec.Operations, "api/*", method)
		require.NotNil(t, apiWildcardOp, "api/* should exist for method %s", method)
		require.NotNil(t, apiWildcardOp.Policies)
		require.Len(t, *apiWildcardOp.Policies, 1, "api/* %s should have only APIWildcardPolicy_1", method)
		assert.Equal(t, "APIWildcardPolicy_1", (*apiWildcardOp.Policies)[0].Name)
	}

	// 3. Verify api/admin/chat POST should not exist (covered by api/admin/* exception)
	apiAdminChatPostOp := findOperation(spec.Operations, "api/admin/chat", "POST")
	assert.Nil(t, apiAdminChatPostOp, "api/admin/chat POST should not exist, covered by api/admin/* exception")

	// 4. Verify APIWildcardPolicy_3 is NOT applied to api/admin/* (should only have deny policy)
	// Already verified in step 1 - all api/admin/* methods have only deny policy, no user policies

	// 5. Verify api/user POST has APIWildcardPolicy_5, APIWildcardPolicy_1, and APIWildcardPolicy_2
	apiUserPostOp := findOperation(spec.Operations, "api/user", "POST")
	require.NotNil(t, apiUserPostOp, "api/user POST should exist")
	require.NotNil(t, apiUserPostOp.Policies)
	require.Len(t, *apiUserPostOp.Policies, 3, "api/user POST should have APIWildcardPolicy_5, APIWildcardPolicy_1, and APIWildcardPolicy_2")

	userPolicyNames := make(map[string]bool)
	for _, p := range *apiUserPostOp.Policies {
		userPolicyNames[p.Name] = true
	}
	assert.True(t, userPolicyNames["APIWildcardPolicy_5"], "api/user POST should have APIWildcardPolicy_5")
	assert.True(t, userPolicyNames["APIWildcardPolicy_1"], "api/user POST should have APIWildcardPolicy_1")
	assert.True(t, userPolicyNames["APIWildcardPolicy_2"], "api/user POST should have APIWildcardPolicy_2")

	// Verify api/internal/* has wildcard policy for non-exception methods (GET, PUT, PATCH, OPTIONS)
	otherInternalMethods := []string{"GET", "PUT", "PATCH", "OPTIONS"}
	for _, method := range otherInternalMethods {
		apiInternalOp := findOperation(spec.Operations, "api/internal/*", method)
		require.Nil(t, apiInternalOp, "api/internal/* should not exist for method %s", method)
	}

	// Verify api/spec/* has wildcard policy for non-exception methods (POST, DELETE, PATCH, OPTIONS)
	otherSpecMethods := []string{"POST", "DELETE", "PATCH", "OPTIONS"}
	for _, method := range otherSpecMethods {
		apiSpecOp := findOperation(spec.Operations, "api/spec/*", method)
		require.Nil(t, apiSpecOp, "api/spec/* should not exist for method %s", method)
	}

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

func TestTransform_AllowAll_MultiplePolicies_PartiallyDenied(t *testing.T) {
	// Multiple policies where some paths are denied
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "chat/completions", Methods: []api.RouteExceptionMethods{"DELETE"}},
		{Path: "models/*", Methods: []api.RouteExceptionMethods{"POST", "PUT"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "CompletionsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "chat/completions", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"guard": true}},
			},
		},
		{
			Name:    "ModelsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "models/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"cache": true}},
			},
		},
		{
			Name:    "EmbeddingsPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "embeddings", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"limit": 1000}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify chat/completions DELETE has only deny policy
	chatDeleteOp := findOperation(spec.Operations, "chat/completions", "DELETE")
	require.NotNil(t, chatDeleteOp)
	require.NotNil(t, chatDeleteOp.Policies)
	require.Len(t, *chatDeleteOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*chatDeleteOp.Policies)[0].Name)

	// Verify chat/completions other methods have user policy
	otherChatMethods := []string{"GET", "POST", "PUT", "PATCH", "OPTIONS"}
	for _, method := range otherChatMethods {
		chatOp := findOperation(spec.Operations, "chat/completions", method)
		require.NotNil(t, chatOp, "chat/completions should exist for method %s", method)
		require.NotNil(t, chatOp.Policies)
		require.Len(t, *chatOp.Policies, 1)
		assert.Equal(t, "CompletionsPolicy", (*chatOp.Policies)[0].Name)
	}

	// Verify models/* POST and PUT have only deny policy
	modelsPostOp := findOperation(spec.Operations, "models/*", "POST")
	require.NotNil(t, modelsPostOp)
	require.NotNil(t, modelsPostOp.Policies)
	require.Len(t, *modelsPostOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*modelsPostOp.Policies)[0].Name)

	modelsPutOp := findOperation(spec.Operations, "models/*", "PUT")
	require.NotNil(t, modelsPutOp)
	require.NotNil(t, modelsPutOp.Policies)
	require.Len(t, *modelsPutOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*modelsPutOp.Policies)[0].Name)

	// Verify models/* other methods have user policy
	otherModelsMethods := []string{"GET", "DELETE", "PATCH", "OPTIONS"}
	for _, method := range otherModelsMethods {
		modelsOp := findOperation(spec.Operations, "models/*", method)
		require.NotNil(t, modelsOp, "models/* should exist for method %s", method)
		require.NotNil(t, modelsOp.Policies)
		require.Len(t, *modelsOp.Policies, 1)
		assert.Equal(t, "ModelsPolicy", (*modelsOp.Policies)[0].Name)
	}

	// Verify embeddings POST has user policy (not denied)
	embeddingsOp := findOperation(spec.Operations, "embeddings", "POST")
	require.NotNil(t, embeddingsOp)
	require.NotNil(t, embeddingsOp.Policies)
	require.Len(t, *embeddingsOp.Policies, 1)
	assert.Equal(t, "EmbeddingsPolicy", (*embeddingsOp.Policies)[0].Name)

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

func TestTransform_AllowAll_NestedPolicyWithPartialExceptions(t *testing.T) {
	// Nested policy coverage with partial exceptions at different levels
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "api/v1/admin/*", Methods: []api.RouteExceptionMethods{"*"}},
		{Path: "api/v1/internal/debug", Methods: []api.RouteExceptionMethods{"GET"}},
	}

	policies := []api.LLMPolicy{
		{
			Name:    "InternalPolicy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/v1/internal/*", Methods: []api.LLMPolicyPathMethods{"GET"}, Params: map[string]interface{}{"internal": true}},
			},
		},
		{
			Name:    "APIV1Policy",
			Version: "v1.0.0",
			Paths: []api.LLMPolicyPath{
				{Path: "api/v1/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"version": "v1"}},
			},
		},
	}

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-provider",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://api.openai.com"),
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

	// Verify api/v1/admin/* has only deny policy for all methods (exception takes precedence)
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		adminOp := findOperation(spec.Operations, "api/v1/admin/*", method)
		require.NotNil(t, adminOp, "api/v1/admin/* should exist for method %s", method)
		require.NotNil(t, adminOp.Policies)
		require.Len(t, *adminOp.Policies, 1)
		assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*adminOp.Policies)[0].Name)
	}

	// Verify api/v1/internal/debug GET has only deny policy (specific exception)
	internalDebugOp := findOperation(spec.Operations, "api/v1/internal/debug", "GET")
	require.NotNil(t, internalDebugOp)
	require.NotNil(t, internalDebugOp.Policies)
	require.Len(t, *internalDebugOp.Policies, 1)
	assert.Equal(t, constants.ACCESS_CONTROL_DENY_POLICY_NAME, (*internalDebugOp.Policies)[0].Name)

	// Verify api/v1/internal/* GET has only InternalPolicy (more specific operation doesn't inherit from broader wildcard)
	internalWildcardOp := findOperation(spec.Operations, "api/v1/internal/*", "GET")
	require.NotNil(t, internalWildcardOp, "api/v1/internal/* GET should be created for InternalPolicy")
	require.NotNil(t, internalWildcardOp.Policies)
	require.Len(t, *internalWildcardOp.Policies, 2, "Should have InternalPolicy and APIV1Policy")

	policyNames := make(map[string]bool)
	for _, p := range *internalWildcardOp.Policies {
		policyNames[p.Name] = true
	}
	assert.True(t, policyNames["InternalPolicy"], "Should have InternalPolicy")
	assert.True(t, policyNames["APIV1Policy"], "Should have APIV1Policy")

	// Verify api/v1/* has APIV1Policy for all methods
	for _, method := range constants.WILDCARD_HTTP_METHODS {
		apiV1Op := findOperation(spec.Operations, "api/v1/*", method)
		require.NotNil(t, apiV1Op, "api/v1/* should exist for method %s", method)
		require.NotNil(t, apiV1Op.Policies)
		require.Len(t, *apiV1Op.Policies, 1)
		assert.Equal(t, "APIV1Policy", (*apiV1Op.Policies)[0].Name)
	}

	// Verify catch-all operations exist
	catchAllCount := 0
	for _, op := range spec.Operations {
		if op.Path == "/*" {
			catchAllCount++
		}
	}
	assert.Equal(t, len(constants.WILDCARD_HTTP_METHODS), catchAllCount)
}

// Helper function to find an operation by path and method
func findOperation(ops []api.Operation, path string, method string) *api.Operation {
	for i := range ops {
		if ops[i].Path == path && string(ops[i].Method) == method {
			return &ops[i]
		}
	}
	return nil
}
