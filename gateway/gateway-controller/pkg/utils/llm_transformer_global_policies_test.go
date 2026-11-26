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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// payloadModel is the requestModel param that setExtractionParam builds from the template.
var payloadModel = map[string]interface{}{"location": api.ExtractionIdentifierLocation("payload"), "identifier": "$.model"}

func templateWithRequestModel(name string) *models.StoredLLMProviderTemplate {
	return &models.StoredLLMProviderTemplate{
		UUID: "0000-global-template-" + name,
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: name},
			Spec: api.LLMProviderTemplateData{
				DisplayName:  name,
				RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: "$.model"},
				PromptTokens: &api.ExtractionIdentifier{Location: "payload", Identifier: "$.usage.prompt_tokens"},
			},
		},
	}
}

func modelRouterPolicy() api.Policy {
	return api.Policy{
		Name:    "model-round-robin",
		Version: "v1",
		Params:  &map[string]interface{}{"models": []interface{}{map[string]interface{}{"model": "gpt-4o-mini"}}},
	}
}

// findPolicy returns the named policy from an API's api-level policies.
func findPolicy(t *testing.T, policies *[]api.Policy, name string) api.Policy {
	t.Helper()
	require.NotNil(t, policies, "the API has no api-level policies")
	for _, pol := range *policies {
		if pol.Name == name {
			return pol
		}
	}
	t.Fatalf("api-level policy %q not found", name)
	return api.Policy{}
}

func TestWithGlobalTemplateParams_AddsBaseSpecExtractionParams(t *testing.T) {
	policies := []api.Policy{modelRouterPolicy(), {Name: "no-params", Version: "v1"}}

	out := withGlobalTemplateParams(policies, templateWithRequestModel("openai"))

	require.Len(t, out, 2)
	for _, pol := range out {
		require.NotNil(t, pol.Params, "policy %s", pol.Name)
		assert.Equal(t, payloadModel, (*pol.Params)["requestModel"], "policy %s", pol.Name)
		assert.Contains(t, *pol.Params, "promptTokens", "policy %s", pol.Name)
	}
	assert.Contains(t, *out[0].Params, "models", "the user's params are kept")
	assert.NotContains(t, *policies[0].Params, "requestModel", "the input policies are not modified")
	assert.Nil(t, policies[1].Params, "the input policies are not modified")
}

func TestWithGlobalTemplateParams_TemplateWithoutExtractionFields(t *testing.T) {
	template := templateWithRequestModel("bare")
	template.Configuration.Spec.RequestModel = nil
	template.Configuration.Spec.PromptTokens = nil

	out := withGlobalTemplateParams([]api.Policy{{Name: "no-params", Version: "v1"}}, template)

	require.Len(t, out, 1)
	assert.Nil(t, out[0].Params)
}

func TestTransformProvider_GlobalPoliciesReceiveTemplateParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)
	require.NoError(t, db.SaveLLMProviderTemplate(templateWithRequestModel("openai")))
	transformer := NewLLMProviderTransformer(storage.NewConfigStore(), db,
		&config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	provider := &api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "global-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName:    "global-provider",
			Version:        "v1.0",
			Template:       "openai",
			Upstream:       api.LLMProviderConfigData_Upstream{Url: stringPtr("https://api.openai.com")},
			AccessControl:  api.LLMAccessControl{Mode: api.AllowAll},
			GlobalPolicies: &[]api.Policy{modelRouterPolicy()},
		},
	}

	result, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)

	pol := findPolicy(t, result.Spec.Policies, "model-round-robin")
	require.NotNil(t, pol.Params)
	assert.Equal(t, payloadModel, (*pol.Params)["requestModel"])
	assert.Contains(t, *pol.Params, "models")
}

func TestTransformProxy_GlobalPoliciesReceiveTemplateParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)
	require.NoError(t, db.SaveLLMProviderTemplate(templateWithRequestModel("openai")))

	now := time.Now()
	require.NoError(t, db.SaveConfig(&models.StoredConfig{
		UUID:        "0000-global-provider-0000-000000000000",
		Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:      "global-provider",
		DisplayName: "global-provider",
		Version:     "v1.0",
		Configuration: api.RestAPI{
			ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.RestAPIKindRestApi,
			Metadata:   api.Metadata{Name: "global-provider"},
			Spec: api.APIConfigData{
				DisplayName: "global-provider",
				Version:     "v1.0",
				Context:     "/global-provider",
				Upstream: struct {
					Main    api.Upstream  `json:"main" yaml:"main"`
					Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
				}{Main: api.Upstream{Url: stringPtr("https://api.openai.com")}},
			},
		},
		SourceConfiguration: api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata:   api.Metadata{Name: "global-provider"},
			Spec: api.LLMProviderConfigData{
				DisplayName:   "global-provider",
				Version:       "v1.0",
				Context:       stringPtr("/global-provider"),
				Template:      "openai",
				Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://api.openai.com")},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
		DesiredState: models.StateDeployed,
		CreatedAt:    now,
		UpdatedAt:    now,
	}))
	transformer := NewLLMProviderTransformer(storage.NewConfigStore(), db,
		&config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "global-proxy"},
		Spec: api.LLMProxyConfigData{
			DisplayName:    "global-proxy",
			Version:        "v1.0",
			Provider:       &api.LLMProxyProvider{Id: "global-provider"},
			GlobalPolicies: &[]api.Policy{modelRouterPolicy()},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)

	pol := findPolicy(t, result.Spec.Policies, "model-round-robin")
	require.NotNil(t, pol.Params)
	assert.Equal(t, payloadModel, (*pol.Params)["requestModel"])
	assert.Contains(t, *pol.Params, "models")
}
