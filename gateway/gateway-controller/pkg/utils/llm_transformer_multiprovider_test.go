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
	"encoding/json"
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
			Provider: &api.LLMProxyProvider{
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
	// Every attached provider is addressable by name, primary first, so a policy
	// that selects one can route to it.
	require.NotNil(t, result.Spec.UpstreamDefinitions)
	require.Len(t, *result.Spec.UpstreamDefinitions, 2)
	assert.Equal(t, "openai-provider", (*result.Spec.UpstreamDefinitions)[0].Name,
		"the primary is defined first")

	additional := (*result.Spec.UpstreamDefinitions)[1]
	assert.Equal(t, "anthropic-provider", additional.Name)
	require.NotNil(t, additional.BasePath)
	assert.Equal(t, "/anthropic-provider", *additional.BasePath)
	require.Len(t, additional.Upstreams, 1)
	assert.Equal(t, "http://127.0.0.1:8080", additional.Upstreams[0].Url)

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
			Provider:    &api.LLMProxyProvider{Id: "openai-provider"},
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
			Provider:    &api.LLMProxyProvider{Id: "openai-provider"},
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
			Provider: &api.LLMProxyProvider{
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

// TestLLMProviderTransformer_ShapeEquivalence is the property the canonical
// shape rests on: the same proxy expressed as `providers[]` or as the legacy
// `provider` plus `additionalProviders` must transform to identical output —
// same upstreams, same execution conditions, same transformers, same
// credentials. A difference means the two shapes are being carried separately
// rather than normalised.
func TestLLMProviderTransformer_ShapeEquivalence(t *testing.T) {
	transformerPolicy := func() *api.LLMProxyTransformer {
		return &api.LLMProxyTransformer{
			Type:    "openai-to-anthropic-transformer",
			Version: "v0",
			Params:  &map[string]interface{}{"model": "claude-sonnet-4-20250514"},
		}
	}
	primaryAuth := func() *api.LLMUpstreamAuth {
		return &api.LLMUpstreamAuth{
			Type: api.LLMUpstreamAuthTypeApiKey, Header: stringPtr("Authorization"), Value: stringPtr("Bearer primary"),
		}
	}
	additionalAuth := func() *api.LLMUpstreamAuth {
		return &api.LLMUpstreamAuth{
			Type: api.LLMUpstreamAuthTypeApiKey, Header: stringPtr("X-Provider-Key"), Value: stringPtr("anthropic-loopback"),
		}
	}

	legacy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "shape-equivalence"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "shape-equivalence",
			Version:     "v1.0",
			Context:     stringPtr("/shape-equivalence"),
			Provider:    &api.LLMProxyProvider{Id: "openai-provider", Auth: primaryAuth()},
			AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{
				Id: "anthropic-provider", As: stringPtr("claude"),
				Auth: additionalAuth(), Transformer: transformerPolicy(),
			}},
		},
	}

	// The canonical list deliberately declares the primary LAST: normalisation
	// must order the primary first regardless, or the two shapes would differ
	// purely by declaration order.
	canonical := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "shape-equivalence"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "shape-equivalence",
			Version:     "v1.0",
			Context:     stringPtr("/shape-equivalence"),
			Providers: &[]api.LLMProxyProviderEntry{
				{
					Id: "anthropic-provider", Alias: stringPtr("claude"),
					Auth: additionalAuth(), Transformer: transformerPolicy(),
				},
				{Id: "openai-provider", IsPrimary: true, Auth: primaryAuth()},
			},
		},
	}

	render := func(proxy *api.LLMProxyConfiguration) string {
		transformer, _ := newCompatEnvironment(t)
		result, err := transformer.Transform(proxy, &api.RestAPI{})
		require.NoError(t, err)
		encoded, err := json.MarshalIndent(result, "", "  ")
		require.NoError(t, err)
		return string(encoded)
	}

	assert.Equal(t, render(legacy), render(canonical),
		"the same proxy in either shape must transform identically")
}

// TestLLMProviderTransformer_PrimaryIdentityAndDefaultAlias covers the two
// rules that make the canonical entry uniform: the proxy's provider identity
// comes from the entry marked primary, and an entry with no alias resolves to
// its id.
func TestLLMProviderTransformer_PrimaryIdentityAndDefaultAlias(t *testing.T) {
	spec := api.LLMProxyConfigData{
		DisplayName: "identity",
		Version:     "v1.0",
		Providers: &[]api.LLMProxyProviderEntry{
			{Id: "anthropic-provider"},
			{Id: "openai-provider", IsPrimary: true},
			{Id: "gemini-provider", Alias: stringPtr("flash")},
		},
	}

	primary, err := models.PrimaryLLMProxyAttachment(spec)
	require.NoError(t, err)
	assert.Equal(t, "openai-provider", primary.Id,
		"provider identity must come from the entry marked primary")

	attachments, err := models.NormaliseLLMProxyAttachments(spec)
	require.NoError(t, err)
	require.Len(t, attachments, 3)
	assert.True(t, attachments[0].IsPrimary, "the primary must be normalised first")
	assert.Equal(t, "anthropic-provider", attachments[1].EffectiveName(),
		"an entry with no alias must resolve to its id")
	assert.Equal(t, "flash", attachments[2].EffectiveName(),
		"an entry with an alias must resolve to it")
}

// TestLLMProviderTransformer_PrimaryTransformerIsConditional is User Story 1:
// a primary provider may carry a transformer, and it is attached under the
// primary's own execution condition — true when no provider was selected or
// when the primary was selected by name, false when another provider was
// conditions.
func TestLLMProviderTransformer_PrimaryTransformerIsConditional(t *testing.T) {
	transformerTransformer, _ := newCompatEnvironment(t)

	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "primary-transformer"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "primary-transformer",
			Version:     "v1.0",
			Context:     stringPtr("/primary-transformer"),
			Provider: &api.LLMProxyProvider{
				Id: "anthropic-provider",
				Transformer: &api.LLMProxyTransformer{
					Type: "openai-to-anthropic-transformer", Version: "v0",
				},
			},
			AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{
				Id: "gemini-provider",
				Transformer: &api.LLMProxyTransformer{
					Type: "openai-to-anthropic-transformer", Version: "v0",
				},
			}},
		},
	}

	result, err := transformerTransformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)

	conditions := map[string]string{}
	for _, op := range result.Spec.Operations {
		if op.Policies == nil {
			continue
		}
		for _, pol := range *op.Policies {
			if pol.Name != "openai-to-anthropic-transformer" || pol.Params == nil {
				continue
			}
			providerID, _ := (*pol.Params)["providerId"].(string)
			require.NotNil(t, pol.ExecutionCondition, "a transformer must always be conditional")
			conditions[providerID] = *pol.ExecutionCondition
		}
	}

	primaryCondition, attached := conditions["anthropic-provider"]
	require.True(t, attached, "the primary provider's transformer must be attached")

	// Runs when nothing was selected, and when the primary was selected by name.
	assert.Contains(t, primaryCondition, "!('selected_provider' in request.Metadata)",
		"the primary's transformer must run when no provider was selected")
	assert.Contains(t, primaryCondition, "request.Metadata['selected_provider'] == 'anthropic-provider'",
		"the primary's transformer must run when the primary was selected by name")
	assert.Equal(t, selectedProviderExecutionCondition("anthropic-provider", true), primaryCondition,
		"the primary's transformer must reuse the condition its credential already uses")

	// An additional provider's transformer keeps the stricter condition, so
	// selecting it means the primary's transformer does not run.
	additionalCondition, attached := conditions["gemini-provider"]
	require.True(t, attached, "the additional provider's transformer must be attached")
	assert.NotContains(t, additionalCondition, "!('selected_provider' in request.Metadata)",
		"an additional provider's transformer must not run by default")
	assert.Equal(t, selectedProviderExecutionCondition("gemini-provider", false), additionalCondition)
}

// newInboundTemplateEnvironment builds two templates whose extraction fields
// differ, plus a provider on each, so a test can tell which template the
// params came from.
func newInboundTemplateEnvironment(t *testing.T) *LLMProviderTransformer {
	t.Helper()
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)

	saveTemplate := func(handle, requestModel string) {
		require.NoError(t, db.SaveLLMProviderTemplate(&models.StoredLLMProviderTemplate{
			UUID: "0000-inbound-" + handle,
			Configuration: api.LLMProviderTemplate{
				ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
				Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
				Metadata:   api.Metadata{Name: handle},
				Spec: api.LLMProviderTemplateData{
					DisplayName: handle,
					RequestModel: &api.ExtractionIdentifier{
						Location:   api.Payload,
						Identifier: requestModel,
					},
				},
			},
		}))
	}
	saveTemplate("openai", "$.body.model")
	saveTemplate("anthropic", "$.body.anthropic_model")

	saveProvider := func(name, template string) {
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID: name + "-inbound-uuid", Kind: string(api.LLMProviderConfigurationKindLlmProvider),
			Handle: name, DisplayName: name, Version: "v1.0",
			SourceConfiguration: api.LLMProviderConfiguration{
				ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
				Kind:       api.LLMProviderConfigurationKindLlmProvider,
				Metadata:   api.Metadata{Name: name},
				Spec: api.LLMProviderConfigData{
					DisplayName: name, Version: "v1.0", Context: stringPtr("/" + name),
					Template:      template,
					Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
					AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
				},
			},
			DesiredState: models.StateDeployed,
		}))
	}
	saveProvider("openai-provider", "openai")
	saveProvider("anthropic-provider", "anthropic")

	return NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080},
		NewStaticPolicyVersionResolver(map[string]string{
			constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME: testSetHeadersVersion,
			constants.ACCESS_CONTROL_DENY_POLICY_NAME:  testRespondVersion,
			constants.UPSTREAM_AUTH_OAUTH2_POLICY_NAME: testOAuth2AuthenticationVersion,
			"llm-cost": "v1.0.0",
		}))
}

// requestModelParam reports the requestModel extraction field the transformer
// merged into the proxy's policies — the value that tells us which template
// the params were built from.
func requestModelParam(t *testing.T, result *api.RestAPI) string {
	t.Helper()
	for _, op := range result.Spec.Operations {
		if op.Policies == nil {
			continue
		}
		for _, pol := range *op.Policies {
			if pol.Params == nil {
				continue
			}
			if requestModel, ok := (*pol.Params)["requestModel"].(map[string]interface{}); ok {
				identifier, _ := requestModel["identifier"].(string)
				return identifier
			}
		}
	}
	return ""
}

// TestLLMProviderTransformer_ExtractionFollowsInboundInterface is User Story 2:
// template params derive from the proxy's declared inbound interface, do not
// move when the primary provider changes, and fall back to the primary's own
// template when no inbound interface is declared.
func TestLLMProviderTransformer_ExtractionFollowsInboundInterface(t *testing.T) {
	proxyWith := func(primaryID string, inbound *string) *api.LLMProxyConfiguration {
		return &api.LLMProxyConfiguration{
			ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProxyConfigurationKindLlmProxy,
			Metadata:   api.Metadata{Name: "inbound-interface"},
			Spec: api.LLMProxyConfigData{
				DisplayName:     "inbound-interface",
				Version:         "v1.0",
				Context:         stringPtr("/inbound-interface"),
				InboundTemplate: inbound,
				Provider:        &api.LLMProxyProvider{Id: primaryID},
				Policies: &[]api.LLMPolicy{{
					Name: "llm-cost", Version: "v1",
					Paths: []api.LLMPolicyPath{{
						Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"},
					}},
				}},
			},
		}
	}

	t.Run("params derive from the declared inbound interface", func(t *testing.T) {
		result, err := newInboundTemplateEnvironment(t).
			Transform(proxyWith("anthropic-provider", stringPtr("openai")), &api.RestAPI{})
		require.NoError(t, err)
		assert.Equal(t, "$.body.model", requestModelParam(t, result),
			"params must come from the inbound interface template, not the primary provider's")
	})

	t.Run("params do not move when the primary changes", func(t *testing.T) {
		before, err := newInboundTemplateEnvironment(t).
			Transform(proxyWith("openai-provider", stringPtr("openai")), &api.RestAPI{})
		require.NoError(t, err)
		after, err := newInboundTemplateEnvironment(t).
			Transform(proxyWith("anthropic-provider", stringPtr("openai")), &api.RestAPI{})
		require.NoError(t, err)
		assert.Equal(t, requestModelParam(t, before), requestModelParam(t, after),
			"promoting a different provider must not rewire extraction")
	})

	t.Run("falls back to the primary provider's template", func(t *testing.T) {
		result, err := newInboundTemplateEnvironment(t).
			Transform(proxyWith("anthropic-provider", nil), &api.RestAPI{})
		require.NoError(t, err)
		assert.Equal(t, "$.body.anthropic_model", requestModelParam(t, result),
			"with no inbound interface declared, today's behaviour must be preserved")
	})

	t.Run("an unresolvable inbound interface fails deployment naming it", func(t *testing.T) {
		_, err := newInboundTemplateEnvironment(t).
			Transform(proxyWith("openai-provider", stringPtr("no-such-template")), &api.RestAPI{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no-such-template",
			"the error must name the template that could not be resolved")
		assert.Contains(t, err.Error(), "inbound interface template")
	})
}

// TestLLMProxy_InboundTemplateRoundTrips is the guard that a field which
// is accepted on the way in and dropped on the way out would pass every other
// test in this file. The deployment artefact is JSON and YAML, and normalisation
// must not lose the declaration either.
func TestLLMProxy_InboundTemplateRoundTrips(t *testing.T) {
	original := api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "round-trip"},
		Spec: api.LLMProxyConfigData{
			DisplayName:     "round-trip",
			Version:         "v1.0",
			InboundTemplate: stringPtr("openai"),
			Providers: &[]api.LLMProxyProviderEntry{
				{Id: "anthropic-provider", IsPrimary: true, Alias: stringPtr("claude"),
					Transformer: &api.LLMProxyTransformer{Type: "openai-to-anthropic-transformer", Version: "v0"}},
				{Id: "gemini-provider"},
			},
		},
	}

	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	var decoded api.LLMProxyConfiguration
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	require.NotNil(t, decoded.Spec.InboundTemplate, "inboundTemplate must survive the artefact round trip")
	assert.Equal(t, "openai", *decoded.Spec.InboundTemplate)

	// Re-encoding must reproduce the same artefact, so a proxy read back and
	// redeployed keeps the same inbound interface.
	reencoded, err := json.Marshal(decoded)
	require.NoError(t, err)
	assert.JSONEq(t, string(encoded), string(reencoded))

	// Normalisation must not drop the per-entry fields either.
	attachments, err := models.NormaliseLLMProxyAttachments(decoded.Spec)
	require.NoError(t, err)
	require.Len(t, attachments, 2)
	assert.Equal(t, "claude", attachments[0].EffectiveName())
	assert.NotNil(t, attachments[0].Transformer, "the primary's transformer must survive normalisation")
}

// TestLLMProxy_AttributionFollowsTheEffectiveModel: the model a request is
// attributed to for cost, rate limiting and analytics must be the model that
// actually served it.
//
// The gateway does not resolve models itself — it merges extraction locations
// from the inbound interface template into every attached policy, and those
// locations read the wire. Two paths matter:
//
//   - responseModel reads the translated response, and every transformer reports
//     the model that served the request there. Attribution therefore follows the
//     effective model by construction.
//   - requestModel reads the client's payload. Because the transformers resolve
//     the model payload-first, the client's model IS the effective model whenever
//     the client names one, so the two agree. When the client names none, the
//     transformer falls back to its configured model and requestModel extracts
//     nothing — an absent value, never a wrong one.
//
// A configured model can no longer override a client-named one, so the
// misattribution this test was written against is not reachable and no
// gateway-side mechanism is needed.
func TestLLMProxy_AttributionFollowsTheEffectiveModel(t *testing.T) {
	transformer := newInboundTemplateEnvironment(t)
	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "attribution"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "attribution", Version: "v1.0", Context: stringPtr("/attribution"),
			InboundTemplate: stringPtr("openai"),
			Provider:        &api.LLMProxyProvider{Id: "anthropic-provider"},
			Policies: &[]api.LLMPolicy{{
				Name: "llm-cost", Version: "v1",
				Paths: []api.LLMPolicyPath{{
					Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"},
				}},
			}},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)

	// The extraction locations a cost policy reads come from the inbound
	// interface, not from the primary provider whose format differs.
	assert.Equal(t, "$.body.model", requestModelParam(t, result),
		"attribution must read the interface the client actually spoke")
}

// TestLLMProviderTransformer_PrimaryTransformerIsRoutable is the regression
// guard for a 503 seen in the gateway: the primary's transformer asks to be
// routed to by name (the policy sets UpstreamName from its providerId param),
// but named upstream definitions were only ever built for additional
// providers, because the primary is the default upstream. The runtime then
// resolved a cluster that did not exist and returned 503 cluster_not_found.
//
// A provider that a policy can be redirected to must have a definition to be
// redirected to, primary or not.
func TestLLMProviderTransformer_PrimaryTransformerIsRoutable(t *testing.T) {
	transformer, _ := newCompatEnvironment(t)

	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "primary-routable"},
		Spec: api.LLMProxyConfigData{
			DisplayName:     "primary-routable",
			Version:         "v1.0",
			Context:         stringPtr("/primary-routable"),
			InboundTemplate: stringPtr("openai"),
			Provider: &api.LLMProxyProvider{
				Id: "anthropic-provider",
				Transformer: &api.LLMProxyTransformer{
					Type: "openai-to-anthropic-transformer", Version: "v0",
				},
			},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)

	require.NotNil(t, result.Spec.UpstreamDefinitions,
		"a primary carrying a transformer must be addressable by name")
	var names []string
	var primaryDef *api.UpstreamDefinition
	for i, def := range *result.Spec.UpstreamDefinitions {
		names = append(names, def.Name)
		if def.Name == "anthropic-provider" {
			primaryDef = &(*result.Spec.UpstreamDefinitions)[i]
		}
	}
	require.NotNil(t, primaryDef,
		"no upstream definition for the primary — the policy redirects to %q, which resolves to no cluster (got %v)",
		"anthropic-provider", names)

	require.NotNil(t, primaryDef.BasePath, "the primary's definition needs the provider's loopback base path")
	assert.Equal(t, "/anthropic-provider", *primaryDef.BasePath)
	require.Len(t, primaryDef.Upstreams, 1)
	assert.Equal(t, "http://127.0.0.1:8080", primaryDef.Upstreams[0].Url)
}

// TestLLMProviderTransformer_EverySelectableProviderIsAddressable is the
// cross-repository seam this feature closes. The router (gateway-controllers)
// emits an upstream name taken from its own mappings and defaultProvider; the
// controller (api-platform) decides which names exist as upstream definitions.
// If those two disagree the request resolves to a cluster that does not exist
// and fails with 503 cluster_not_found — the failure mode that already bit a
// primary carrying a transformer.
//
// The invariant: every provider a router can select must be addressable,
// whether or not it carries a transformer, and including the primary when
// defaultProvider names it.
func TestLLMProviderTransformer_EverySelectableProviderIsAddressable(t *testing.T) {
	transformer, _ := newCompatEnvironment(t)

	// The router's view: what it can emit as an upstream name.
	const defaultProvider = "openai-provider"
	routerSelectable := []string{"openai-provider", "anthropic-provider", "gemini-provider"}

	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "selectable"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "selectable", Version: "v1.0", Context: stringPtr("/selectable"),
			Provider: &api.LLMProxyProvider{Id: "openai-provider"},
			AdditionalProviders: &[]api.LLMProxyAdditionalProvider{
				// carries a transformer — routed correctly even before this fix
				{Id: "anthropic-provider", Transformer: &api.LLMProxyTransformer{
					Type: "openai-to-anthropic-transformer", Version: "v0"}},
				// no transformer — the case that silently reached the primary
				{Id: "gemini-provider"},
			},
			Policies: &[]api.LLMPolicy{{
				Name: "llm-header-router", Version: "v1",
				Paths: []api.LLMPolicyPath{{
					Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"},
					Params: map[string]interface{}{"defaultProvider": defaultProvider},
				}},
			}},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)

	defined := map[string]bool{}
	require.NotNil(t, result.Spec.UpstreamDefinitions)
	for _, def := range *result.Spec.UpstreamDefinitions {
		defined[def.Name] = true
	}

	for _, name := range routerSelectable {
		assert.True(t, defined[name],
			"the router can route to %q but no upstream definition exists for it — "+
				"that resolves to a missing cluster and fails with 503; defined: %v", name, defined)
	}

	// And the default cluster is still the primary, so the fall-through path
	// (nothing selected) is unaffected by any of the above.
	require.NotNil(t, result.Spec.Upstream.Main.Url)
	assert.Equal(t, "http://127.0.0.1:8080/openai-provider", *result.Spec.Upstream.Main.Url)
}

// TestLLMProviderTransformer_AllTransformersNoRoutingChange guards that
// a proxy whose selectable providers all carry transformers routed correctly
// before this fix, and must route identically after it. The upstream names the
// router emits are unchanged; only the primary's definition is added.
func TestLLMProviderTransformer_AllTransformersNoRoutingChange(t *testing.T) {
	transformer, _ := newCompatEnvironment(t)
	tf := func() *api.LLMProxyTransformer {
		return &api.LLMProxyTransformer{Type: "openai-to-anthropic-transformer", Version: "v0"}
	}

	result, err := transformer.Transform(&api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "all-transformers"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "all-transformers", Version: "v1.0", Context: stringPtr("/all-transformers"),
			Provider: &api.LLMProxyProvider{Id: "openai-provider", Transformer: tf()},
			AdditionalProviders: &[]api.LLMProxyAdditionalProvider{
				{Id: "anthropic-provider", Transformer: tf()},
			},
		},
	}, &api.RestAPI{})
	require.NoError(t, err)

	var names []string
	for _, def := range *result.Spec.UpstreamDefinitions {
		names = append(names, def.Name)
	}
	assert.Equal(t, []string{"openai-provider", "anthropic-provider"}, names,
		"the addressable set is unchanged for a fully-transformed proxy")
}
