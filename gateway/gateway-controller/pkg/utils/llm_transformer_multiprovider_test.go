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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestLLMProviderTransformer_TransformProxy_AdditionalProviderAuthIsConditional(t *testing.T) {
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-db-template-id-0000-000000000002",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: "openai"},
			Spec:       api.LLMProviderTemplateData{DisplayName: "openai"},
		},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	saveProvider := func(name, context string) {
		providerSourceConfig := api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata:   api.Metadata{Name: name},
			Spec: api.LLMProviderConfigData{
				DisplayName:   name,
				Version:       "v1.0",
				Context:       stringPtr(context),
				Template:      "openai",
				Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		}
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:                name + "-uuid",
			Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:              name,
			DisplayName:         name,
			Version:             "v1.0",
			SourceConfiguration: providerSourceConfig,
			DesiredState:        models.StateDeployed,
		}))
	}
	saveProvider("openai-provider", "/openai-provider")
	saveProvider("anthropic-provider", "/anthropic-provider")

	transformer := NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "openai-multi"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "openai-multi",
			Version:     "v1.0",
			Provider: api.LLMProxyProvider{
				Id: "openai-provider",
				Auth: &api.LLMUpstreamAuth{
					Type:   api.LLMUpstreamAuthTypeApiKey,
					Header: stringPtr("Authorization"),
					Value:  stringPtr("Bearer primary"),
				},
			},
			AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{
				Id: "anthropic-provider",
				Auth: &api.LLMUpstreamAuth{
					Type:   api.LLMUpstreamAuthTypeApiKey,
					Header: stringPtr("X-Provider-Key"),
					Value:  stringPtr("anthropic-loopback"),
				},
			}},
			Policies: &[]api.LLMPolicy{{
				Name:    "openai-header-router",
				Version: "v1",
				Paths: []api.LLMPolicyPath{{
					Path:    "/chat/completions",
					Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{
						"defaultProvider": "openai-provider",
					},
				}},
			}},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)
	require.NotNil(t, result.Spec.UpstreamDefinitions)
	require.Len(t, *result.Spec.UpstreamDefinitions, 1)
	assert.Equal(t, "anthropic-provider", (*result.Spec.UpstreamDefinitions)[0].Name)

	var chatOp *api.Operation
	for i := range result.Spec.Operations {
		if result.Spec.Operations[i].Path != nil && *result.Spec.Operations[i].Path == "/chat/completions" &&
			result.Spec.Operations[i].Method != nil && *result.Spec.Operations[i].Method == api.OperationMethod("POST") {
			chatOp = &result.Spec.Operations[i]
			break
		}
	}
	require.NotNil(t, chatOp)
	require.NotNil(t, chatOp.Policies)

	var authPolicies []api.Policy
	for _, pol := range *chatOp.Policies {
		if pol.Name == constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME {
			authPolicies = append(authPolicies, pol)
		}
	}
	require.Len(t, authPolicies, 2)
	require.NotNil(t, authPolicies[0].ExecutionCondition)
	require.NotNil(t, authPolicies[1].ExecutionCondition)
	assert.Contains(t, *authPolicies[0].ExecutionCondition, "openai-provider")
	assert.Contains(t, *authPolicies[1].ExecutionCondition, "anthropic-provider")
	assert.Equal(t, "Bearer primary", firstRequestHeaderValue(t, authPolicies[0].Params))
	assert.Equal(t, "anthropic-loopback", firstRequestHeaderValue(t, authPolicies[1].Params))
}

func TestLLMProviderTransformer_TransformProxy_AdditionalProviderTransformerIsConditional(t *testing.T) {
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-db-template-id-0000-000000000003",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: "openai"},
			Spec:       api.LLMProviderTemplateData{DisplayName: "openai"},
		},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	saveProvider := func(name, context string) {
		providerSourceConfig := api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata:   api.Metadata{Name: name},
			Spec: api.LLMProviderConfigData{
				DisplayName:   name,
				Version:       "v1.0",
				Context:       stringPtr(context),
				Template:      "openai",
				Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		}
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:                name + "-uuid",
			Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:              name,
			DisplayName:         name,
			Version:             "v1.0",
			SourceConfiguration: providerSourceConfig,
			DesiredState:        models.StateDeployed,
		}))
	}
	saveProvider("openai-provider", "/openai-provider")
	saveProvider("anthropic-provider", "/anthropic-provider")

	transformer := NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "openai-multi"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "openai-multi",
			Version:     "v1.0",
			Provider:    api.LLMProxyProvider{Id: "openai-provider"},
			AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{
				Id: "anthropic-provider",
				Transformer: &api.LLMProxyTransformer{
					Type:    "openai-to-anthropic",
					Version: "v1",
					Params: &map[string]interface{}{
						"model": "claude-sonnet-4-5-20250929",
					},
				},
			}},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)

	// The translator is attached conditionally to every operation, so locate it
	// wherever it lands rather than assuming a specific route.
	var transformerPolicy *api.Policy
	for i := range result.Spec.Operations {
		op := result.Spec.Operations[i]
		if op.Policies == nil {
			continue
		}
		for j := range *op.Policies {
			if (*op.Policies)[j].Name == "openai-to-anthropic" {
				transformerPolicy = &(*op.Policies)[j]
				break
			}
		}
		if transformerPolicy != nil {
			break
		}
	}
	require.NotNil(t, transformerPolicy)
	assert.Equal(t, "v1", transformerPolicy.Version)
	require.NotNil(t, transformerPolicy.ExecutionCondition)
	assert.Contains(t, *transformerPolicy.ExecutionCondition, "anthropic-provider")
	require.NotNil(t, transformerPolicy.Params)
	assert.Equal(t, "anthropic-provider", (*transformerPolicy.Params)["providerId"])
	assert.Equal(t, "claude-sonnet-4-5-20250929", (*transformerPolicy.Params)["model"])
}

// Test that a proxy that loops back into a provider with a downstream api-key-auth policy
func TestLLMProviderTransformer_TransformProxy_LoopbackAuthCarriesProviderValuePrefix(t *testing.T) {
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-db-template-id-0000-000000000003",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: "mistralai"},
			Spec:       api.LLMProviderTemplateData{DisplayName: "mistralai"},
		},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	// Provider whose downstream api-key-auth requires a "Bearer" prefix, carried as a
	// global policy exactly as platform-api deploys it.
	providerSourceConfig := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "mistral-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName:   "mistral-provider",
			Version:       "v1.0",
			Context:       stringPtr("/mistral-provider"),
			Template:      "mistralai",
			Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			GlobalPolicies: &[]api.Policy{{
				Name: constants.API_KEY_AUTH_POLICY_NAME,
				Params: &map[string]interface{}{
					"in":          "header",
					"key":         "X-API-Key",
					"valuePrefix": "Bearer",
				},
			}},
		},
	}
	require.NoError(t, db.SaveConfig(&models.StoredConfig{
		UUID:                "mistral-provider-uuid",
		Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:              "mistral-provider",
		DisplayName:         "mistral-provider",
		Version:             "v1.0",
		SourceConfiguration: providerSourceConfig,
		DesiredState:        models.StateDeployed,
	}))

	transformer := NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "proxy-from-mistral"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "proxy-from-mistral",
			Version:     "v1.0",
			Provider: api.LLMProxyProvider{
				Id: "mistral-provider",
				Auth: &api.LLMUpstreamAuth{
					Type:   api.LLMUpstreamAuthTypeApiKey,
					Header: stringPtr("X-API-Key"),
					Value:  stringPtr(`{{ secret "sec-1" }}`),
				},
			},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)

	var authPolicy *api.Policy
	for i := range result.Spec.Operations {
		if result.Spec.Operations[i].Policies == nil {
			continue
		}
		for _, pol := range *result.Spec.Operations[i].Policies {
			if pol.Name == constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME {
				p := pol
				authPolicy = &p
				break
			}
		}
		if authPolicy != nil {
			break
		}
	}
	require.NotNil(t, authPolicy, "expected an upstream auth (set-headers) policy on the proxy")
	assert.Equal(t, `Bearer {{ secret "sec-1" }}`, firstRequestHeaderValue(t, authPolicy.Params))
}

func firstRequestHeaderValue(t *testing.T, params *map[string]interface{}) string {
	t.Helper()
	require.NotNil(t, params)
	request, ok := (*params)["request"].(map[string]interface{})
	require.True(t, ok)
	headers, ok := request["headers"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, headers)
	header, ok := headers[0].(map[string]interface{})
	require.True(t, ok)
	value, ok := header["value"].(string)
	require.True(t, ok)
	return value
}
