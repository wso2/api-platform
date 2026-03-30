/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// makeLLMStoredConfigWithResolvedRestAPI builds a StoredConfig that simulates the state
// after hydration and secret resolution: Configuration contains a RestAPI with resolved
// secrets, SourceConfiguration contains the original LLM provider config with $secret{} templates.
func makeLLMStoredConfigWithResolvedRestAPI(resolvedAuthValue string) *models.StoredConfig {
	setHeadersParams := map[string]interface{}{
		"request": map[string]interface{}{
			"headers": []interface{}{
				map[string]interface{}{
					"name":  "Authorization",
					"value": resolvedAuthValue,
				},
			},
		},
	}

	policies := []api.Policy{
		{
			Name:    "set-headers",
			Version: "v1",
			Params:  &setHeadersParams,
		},
	}

	restAPI := api.RestAPI{
		Kind:     api.RestApi,
		Metadata: api.Metadata{Name: "test-llm-provider"},
		Spec: api.APIConfigData{
			DisplayName: "Test LLM Provider",
			Context:     "/llm-test",
			Version:     "v1.0",
			Operations: []api.Operation{
				{Method: "POST", Path: "/*", Policies: &policies},
			},
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{Url: ptrStr("http://backend:8080")},
			},
		},
	}

	// SourceConfiguration is the original LLM provider config with unresolved secret
	providerConfig := api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "test-llm-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "Test LLM Provider",
			Version:     "v1.0",
			Template:    "openai",
			Vhost:       ptrStr("api.test.local"),
			Upstream: api.LLMProviderConfigData_Upstream{
				Auth: &struct {
					Header *string                               `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                               `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   api.LLMProviderConfigDataUpstreamAuthTypeApiKey,
					Header: ptrStr("Authorization"),
					Value:  ptrStr("Bearer $secret{test-secret}"),
				},
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	return &models.StoredConfig{
		UUID:                "test-llm-provider",
		Kind:                string(api.LlmProvider),
		Handle:              "test-llm-provider",
		DisplayName:         "Test LLM Provider",
		Version:             "v1.0",
		Configuration:       restAPI,
		SourceConfiguration: providerConfig,
	}
}

// TestLLMTransformer_UsesExistingConfiguration verifies that when cfg.Configuration
// is already a hydrated RestAPI (with resolved secrets), the transformer uses it
// directly instead of regenerating from SourceConfiguration.
func TestLLMTransformer_UsesExistingConfiguration(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"set-headers|v1.0.0": {Name: "set-headers", Version: "v1.0.0"},
	}

	restTransformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	llmTransformer := &LLMTransformer{
		restTransformer: restTransformer,
		// llmTransformer and store are not needed since Configuration is already set
	}

	resolvedValue := "Bearer sk-actual-secret-value-12345"
	cfg := makeLLMStoredConfigWithResolvedRestAPI(resolvedValue)

	rdc, err := llmTransformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	// Verify the resolved secret value appears in the policy chain, not the $secret{} template
	found := false
	for _, chain := range rdc.PolicyChains {
		for _, p := range chain.Policies {
			if p.Name == "set-headers" {
				found = true
				request, ok := p.Params["request"].(map[string]interface{})
				require.True(t, ok, "expected request param in set-headers policy")
				headers, ok := request["headers"].([]interface{})
				require.True(t, ok, "expected headers array in request param")
				require.Len(t, headers, 1)
				header, ok := headers[0].(map[string]interface{})
				require.True(t, ok, "expected header map")
				assert.Equal(t, resolvedValue, header["value"],
					"expected resolved secret value in set-headers policy, not $secret{} template")
			}
		}
	}
	assert.True(t, found, "expected set-headers policy in at least one policy chain")

	// Verify LLM metadata is still enriched from SourceConfiguration
	assert.Equal(t, string(api.LlmProvider), rdc.Metadata.Kind)
	require.NotNil(t, rdc.Metadata.LLM)
	assert.Equal(t, "openai", rdc.Metadata.LLM.TemplateHandle)
	assert.Equal(t, "test-llm-provider", rdc.Metadata.LLM.ProviderName)
}

// TestLLMTransformer_SecretTemplateNotLeaked verifies that when the Configuration
// contains a resolved secret, the original $secret{} template from SourceConfiguration
// does not leak into the RuntimeDeployConfig.
func TestLLMTransformer_SecretTemplateNotLeaked(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"set-headers|v1.0.0": {Name: "set-headers", Version: "v1.0.0"},
	}

	restTransformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	llmTransformer := &LLMTransformer{
		restTransformer: restTransformer,
	}

	resolvedValue := "Bearer sk-resolved-key"
	cfg := makeLLMStoredConfigWithResolvedRestAPI(resolvedValue)

	rdc, err := llmTransformer.Transform(cfg)
	require.NoError(t, err)

	// Verify no policy chain contains the unresolved $secret{} template
	for routeKey, chain := range rdc.PolicyChains {
		for _, p := range chain.Policies {
			if p.Name == "set-headers" {
				request, ok := p.Params["request"].(map[string]interface{})
				require.True(t, ok)
				headers, ok := request["headers"].([]interface{})
				require.True(t, ok)
				for _, h := range headers {
					header := h.(map[string]interface{})
					value, _ := header["value"].(string)
					assert.NotContains(t, value, "$secret{",
						"route %s: set-headers policy should not contain unresolved $secret{} template", routeKey)
				}
			}
		}
	}
}

// TestLLMTransformer_MultipleHeadersWithResolvedSecrets verifies that when multiple
// headers contain resolved secrets, all are preserved through the transform.
func TestLLMTransformer_MultipleHeadersWithResolvedSecrets(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"set-headers|v1.0.0": {Name: "set-headers", Version: "v1.0.0"},
	}

	restTransformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	llmTransformer := &LLMTransformer{
		restTransformer: restTransformer,
	}

	// Build config with multiple resolved headers
	setHeadersParams := map[string]interface{}{
		"request": map[string]interface{}{
			"headers": []interface{}{
				map[string]interface{}{
					"name":  "Authorization",
					"value": "Bearer sk-resolved-auth-key",
				},
				map[string]interface{}{
					"name":  "X-Custom-Key",
					"value": "resolved-custom-value",
				},
			},
		},
	}

	policies := []api.Policy{{Name: "set-headers", Version: "v1", Params: &setHeadersParams}}
	restAPI := api.RestAPI{
		Kind:     api.RestApi,
		Metadata: api.Metadata{Name: "multi-header-provider"},
		Spec: api.APIConfigData{
			DisplayName: "Multi Header Provider",
			Context:     "/multi-test",
			Version:     "v1.0",
			Operations: []api.Operation{
				{Method: "POST", Path: "/*", Policies: &policies},
			},
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{Url: ptrStr("http://backend:8080")},
			},
		},
	}

	providerConfig := api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "multi-header-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "Multi Header Provider",
			Version:     "v1.0",
			Template:    "openai",
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	cfg := &models.StoredConfig{
		UUID:                "multi-header-provider",
		Kind:                string(api.LlmProvider),
		Handle:              "multi-header-provider",
		Configuration:       restAPI,
		SourceConfiguration: providerConfig,
	}

	rdc, err := llmTransformer.Transform(cfg)
	require.NoError(t, err)
	require.NotNil(t, rdc)

	// Find the set-headers policy and verify both headers are present
	found := false
	for _, chain := range rdc.PolicyChains {
		for _, p := range chain.Policies {
			if p.Name == "set-headers" {
				found = true
				request := p.Params["request"].(map[string]interface{})
				headers := request["headers"].([]interface{})
				assert.Len(t, headers, 2, "both resolved headers should be preserved")

				h0 := headers[0].(map[string]interface{})
				assert.Equal(t, "Bearer sk-resolved-auth-key", h0["value"])
				h1 := headers[1].(map[string]interface{})
				assert.Equal(t, "resolved-custom-value", h1["value"])
				break
			}
		}
		if found {
			break
		}
	}
	assert.True(t, found, "set-headers policy should be in policy chain")
}

// TestLLMTransformer_NonRestAPIConfiguration verifies that when cfg.Configuration
// is not a RestAPI (e.g., a string), the transformer enters the else branch
// and attempts to regenerate from SourceConfiguration.
func TestLLMTransformer_NonRestAPIConfiguration(t *testing.T) {
	defs := map[string]models.PolicyDefinition{}

	restTransformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	llmTransformer := &LLMTransformer{
		restTransformer: restTransformer,
	}

	// Configuration is a string (not a RestAPI) and SourceConfiguration is
	// not a recognized LLM type — should return an "unsupported" error.
	cfg := &models.StoredConfig{
		UUID:                "test",
		Kind:                string(api.LlmProvider),
		Configuration:       "not-a-rest-api",
		SourceConfiguration: "also-not-llm",
	}

	_, err := llmTransformer.Transform(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported LLM source configuration type")
}

// TestLLMTransformer_NilConfiguration verifies the fallback path when
// cfg.Configuration is nil — the else branch is entered.
func TestLLMTransformer_NilConfiguration(t *testing.T) {
	defs := map[string]models.PolicyDefinition{}

	restTransformer := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	llmTransformer := &LLMTransformer{
		restTransformer: restTransformer,
	}

	cfg := &models.StoredConfig{
		UUID:                "test",
		Kind:                string(api.LlmProvider),
		Configuration:       nil,
		SourceConfiguration: "not-a-recognized-type",
	}

	_, err := llmTransformer.Transform(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported LLM source configuration type")
}
