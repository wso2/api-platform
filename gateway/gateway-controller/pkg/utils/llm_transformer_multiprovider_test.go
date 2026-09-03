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
				Name:    "llm-header-router",
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
	require.NotNil(t, (*result.Spec.UpstreamDefinitions)[0].BasePath)
	assert.Equal(t, "/anthropic-provider", *(*result.Spec.UpstreamDefinitions)[0].BasePath)
	require.Len(t, (*result.Spec.UpstreamDefinitions)[0].Upstreams, 1)
	assert.Equal(t, "http://127.0.0.1:8080", (*result.Spec.UpstreamDefinitions)[0].Upstreams[0].Url)

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
		// The unconditional internal loopback marker is also a set-headers policy; exclude it.
		if pol.Name == constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME && !hasInternalLoopbackMarkerPolicy([]api.Policy{pol}) {
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

func TestLLMProviderTransformer_TransformProxy_RejectsInvalidAdditionalProviderSourceConfiguration(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-db-template-id-0000-000000000004",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: "openai"},
			Spec:       api.LLMProviderTemplateData{DisplayName: "openai"},
		},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	primaryProvider := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName:   "openai-provider",
			Version:       "v1.0",
			Context:       stringPtr("/openai-provider"),
			Template:      "openai",
			Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}
	require.NoError(t, db.SaveConfig(&models.StoredConfig{
		UUID:                "openai-provider-uuid",
		Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:              "openai-provider",
		DisplayName:         "openai-provider",
		Version:             "v1.0",
		SourceConfiguration: primaryProvider,
		DesiredState:        models.StateDeployed,
	}))

	require.NoError(t, db.SaveConfig(&models.StoredConfig{
		UUID:        "invalid-provider-uuid",
		Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:      "invalid-provider",
		DisplayName: "invalid-provider",
		Version:     "v1.0",
		SourceConfiguration: api.RestAPI{
			ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.RestAPIKindRestApi,
			Metadata:   api.Metadata{Name: "invalid-provider"},
			Spec: api.APIConfigData{
				DisplayName: "invalid-provider",
				Version:     "v1.0",
				Context:     "/invalid-provider",
			},
		},
		DesiredState: models.StateDeployed,
	}))

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
				Id: "invalid-provider",
			}},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "additional provider 'invalid-provider' source configuration is not LLMProviderConfiguration", err.Error())
}

// TestLLMProviderTransformer_TransformProxy_AdditionalProviderOAuth2AuthIsIsolated
// covers a proxy's primary provider and an additionalProviders entry, each with
// independent oauth2 credentials, emitting two separate oauth2 Policy
// attachments rather than one shared one.
func TestLLMProviderTransformer_TransformProxy_AdditionalProviderOAuth2AuthIsIsolated(t *testing.T) {
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-db-template-id-0000-000000000004",
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
	saveProvider("provider-a", "/provider-a")
	saveProvider("provider-b", "/provider-b")

	transformer := NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	// provider-b differs from provider-a in clientId, tokenEndpoint AND
	// clientSecret, not just name, to lock in isolation on every field the
	// cache key discriminates by.
	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "oauth2-multi"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "oauth2-multi",
			Version:     "v1.0",
			Provider: api.LLMProxyProvider{
				Id: "provider-a",
				Auth: &api.LLMUpstreamAuth{
					Type: api.LLMUpstreamAuthTypeOauth2,
					PolicyParams: &map[string]interface{}{
						"tokenEndpoint": "https://idp-a.example.com/token",
						"clientId":      "client-a",
						"clientSecret":  "secret-a",
					},
				},
			},
			AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{
				Id: "provider-b",
				Auth: &api.LLMUpstreamAuth{
					Type: api.LLMUpstreamAuthTypeOauth2,
					PolicyParams: &map[string]interface{}{
						"tokenEndpoint": "https://idp-b.example.com/token",
						"clientId":      "client-b",
						"clientSecret":  "secret-b",
					},
				},
			}},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)

	// No operationPolicies attached, so the transformer only generates
	// wildcard catch-all routes - any POST operation carries both oauth2
	// attachments.
	var postOp *api.Operation
	for i := range result.Spec.Operations {
		if result.Spec.Operations[i].Method != nil && *result.Spec.Operations[i].Method == api.OperationMethod("POST") {
			postOp = &result.Spec.Operations[i]
			break
		}
	}
	require.NotNil(t, postOp)
	require.NotNil(t, postOp.Policies)

	var oauth2Policies []api.Policy
	for _, pol := range *postOp.Policies {
		if pol.Name == constants.UPSTREAM_AUTH_OAUTH2_POLICY_NAME {
			oauth2Policies = append(oauth2Policies, pol)
		}
	}
	// Two separate oauth2 attachments on the same operation - the shape that
	// collided under the old API-identity-keyed cache.
	require.Len(t, oauth2Policies, 2)
	require.NotNil(t, oauth2Policies[0].ExecutionCondition)
	require.NotNil(t, oauth2Policies[1].ExecutionCondition)
	assert.Contains(t, *oauth2Policies[0].ExecutionCondition, "provider-a")
	assert.Contains(t, *oauth2Policies[1].ExecutionCondition, "provider-b")

	require.NotNil(t, oauth2Policies[0].Params)
	require.NotNil(t, oauth2Policies[1].Params)
	paramsA := *oauth2Policies[0].Params
	paramsB := *oauth2Policies[1].Params

	// Every field oauth2ConfigDiscriminator keys on must actually differ, or
	// the two would collide on the same Redis key regardless.
	assert.NotEqual(t, paramsA["clientId"], paramsB["clientId"])
	assert.NotEqual(t, paramsA["tokenEndpoint"], paramsB["tokenEndpoint"])
	assert.NotEqual(t, paramsA["clientSecret"], paramsB["clientSecret"])
	assert.Equal(t, "client-a", paramsA["clientId"])
	assert.Equal(t, "client-b", paramsB["clientId"])
	assert.Equal(t, "https://idp-a.example.com/token", paramsA["tokenEndpoint"])
	assert.Equal(t, "https://idp-b.example.com/token", paramsB["tokenEndpoint"])
	assert.Equal(t, "secret-a", paramsA["clientSecret"])
	assert.Equal(t, "secret-b", paramsB["clientSecret"])
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
