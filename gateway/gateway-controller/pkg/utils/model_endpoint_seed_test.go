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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/modelgateway"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestModelEndpointSeedTransformers_AreValid(t *testing.T) {
	seed, err := ModelEndpointSeedTransformers()
	require.NoError(t, err)
	templates := map[string]string{}
	for _, e := range seed {
		require.NotNil(t, e.Template)
		assert.Nil(t, e.Version, "the provisioner writes the loaded version")
		require.NotNil(t, e.UpstreamPaths, "every seed entry declares its paths, so routed targets are checked")
		templates[*e.Template] = e.Type
	}
	assert.Equal(t, map[string]string{
		"azure-openai": "openai-to-azure-openai-transformer",
		"anthropic":    "openai-to-anthropic-transformer",
		"gemini":       "openai-to-gemini-transformer",
		"awsbedrock":   "openai-to-bedrock-transformer",
		"mistralai":    "openai-to-mistral-transformer",
	}, templates)

	// With versions written, the entries pass the endpoint's own validation.
	for i := range seed {
		seed[i].Version = ptrTo("v0")
	}
	assert.Empty(t, config.ValidateModelEndpointTransformers("spec.transformers", &seed))
}

// The seed entries give every template what the compiled-in table gave it: the
// same transformer, the same API version handling, and the same paths checked.
func TestModelEndpointSeedTransformers_MatchTheTemplates(t *testing.T) {
	seed, err := ModelEndpointSeedTransformers()
	require.NoError(t, err)
	for i := range seed {
		seed[i].Version = ptrTo("v0")
	}
	endpoint := modelgateway.Endpoint{Handle: "wso2", InboundTemplate: "openai", InboundGroup: "wso2-openai",
		Transformers: seed, DispatchPaths: []string{"/chat/completions"}}

	templateStore, err := NewLLMTemplateLoader(slog.New(slog.NewTextHandler(io.Discard, nil))).
		LoadTemplatesFromDirectory("../../default-llm-provider-templates")
	require.NoError(t, err)

	cases := []struct {
		template, apiVersion, wantPolicy, wantParam, wantValue, wantErr string
		upstreamModel, allowedPath                                      string
	}{
		{template: "openai", allowedPath: "/chat/completions"},
		{template: "anthropic", wantPolicy: "openai-to-anthropic-transformer", wantParam: "anthropicVersion",
			wantValue: "2023-06-01", allowedPath: "/v1/messages"},
		{template: "anthropic", apiVersion: "2024-01-01", wantPolicy: "openai-to-anthropic-transformer",
			wantParam: "anthropicVersion", wantValue: "2024-01-01", allowedPath: "/v1/messages"},
		{template: "gemini", wantPolicy: "openai-to-gemini-transformer", wantParam: "apiVersion", wantValue: "v1beta",
			upstreamModel: "gemini-2.0-flash", allowedPath: "/v1beta/models/*"},
		{template: "awsbedrock", wantPolicy: "openai-to-bedrock-transformer", upstreamModel: "amazon.nova-lite-v1:0",
			allowedPath: "/model/*"},
		{template: "mistralai", wantPolicy: "openai-to-mistral-transformer", allowedPath: "/v1/chat/completions"},
		{template: "azure-openai", wantErr: "apiVersion"},
		{template: "azure-openai", apiVersion: "2024-10-21", wantPolicy: "openai-to-azure-openai-transformer",
			wantParam: "apiVersion", wantValue: "2024-10-21", upstreamModel: "my-deployment",
			allowedPath: "/openai/deployments/my-deployment/chat/completions"},
		{template: "azureai-foundry", wantErr: "no transformer"},
	}
	for _, tc := range cases {
		t.Run(tc.template+"/"+tc.apiVersion, func(t *testing.T) {
			tmpl := templateStore[tc.template]
			require.NotNil(t, tmpl, "template %s", tc.template)
			upstreamModel := tc.upstreamModel
			if upstreamModel == "" {
				upstreamModel = "m"
			}
			provider := api.LLMProviderConfiguration{
				Metadata: api.Metadata{Name: "p"},
				Spec: api.LLMProviderConfigData{DisplayName: "p", Version: "v1.0", Template: tc.template, Context: ptrTo("/p"),
					AccessControl: &api.LLMAccessControl{Mode: api.DenyAll, Exceptions: &[]api.RouteException{{
						Path: tc.allowedPath, Methods: []api.RouteExceptionMethods{"POST"},
					}}}},
			}
			if tc.apiVersion != "" {
				provider.Spec.Upstream.ApiVersion = ptrTo(tc.apiVersion)
			}
			// Routed, so the paths are checked against the provider route.
			m := &modelgateway.Model{Handle: "m", ModelName: "m", Targets: []modelgateway.Target{{
				Name: "a", UpstreamModel: upstreamModel, ProviderHandle: "p", Mode: modelgateway.ModeRoute,
				Provider: &modelgateway.Provider{Config: provider}, Template: storedTemplate(tmpl),
			}}}
			b, err := modelgateway.Bind(m, endpoint)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			tr := b.Transformers["p"]
			if tc.wantPolicy == "" {
				assert.Nil(t, tr)
				return
			}
			require.NotNil(t, tr)
			assert.Equal(t, tc.wantPolicy, tr.PolicyName)
			if tc.wantParam != "" {
				assert.Equal(t, tc.wantValue, tr.Params[tc.wantParam])
			}
		})
	}
}

func storedTemplate(t *api.LLMProviderTemplate) *models.StoredLLMProviderTemplate {
	return &models.StoredLLMProviderTemplate{Configuration: *t}
}

func TestModelKeyScopes(t *testing.T) {
	store := storage.NewConfigStore()
	addEndpoint := func(handle, inbound string) {
		e := api.ModelEndpointConfiguration{Metadata: api.Metadata{Name: handle}}
		if inbound != "" {
			e.Spec.InboundTemplate = ptrTo(inbound)
		}
		require.NoError(t, store.Add(&models.StoredConfig{UUID: models.ModelEndpointUUID(handle),
			Kind: models.KindModelEndpoint, Handle: handle, DisplayName: handle, Version: "v1.0", SourceConfiguration: e}))
	}
	addEndpoint("wso2", "")
	addEndpoint("anthropic-native", "anthropic")
	m := api.ModelConfiguration{Metadata: api.Metadata{Name: "claude-haiku"},
		Spec: api.ModelConfigData{Endpoints: []string{"wso2", "anthropic-native", "not-here"}}}
	require.NoError(t, store.Add(&models.StoredConfig{UUID: "model-uuid", Kind: models.KindModel,
		Handle: "claude-haiku", DisplayName: "Claude Haiku", Version: "v1.0", SourceConfiguration: m}))
	require.NoError(t, store.Add(&models.StoredConfig{UUID: "rest-uuid", Kind: models.KindRestApi, Handle: "orders",
		DisplayName: "Orders", Version: "v1.0", SourceConfiguration: api.RestAPI{}}))

	resolve := ModelKeyScopes(store)

	scopes, ok := resolve("model-uuid")
	require.True(t, ok)
	assert.Equal(t, []apikeyxds.KeyScope{
		{APIId: models.ModelEndpointUUID("wso2"), Operations: []string{"claude-haiku", "GET /models"}, Owner: "claude-haiku"},
		{APIId: models.ModelEndpointUUID("anthropic-native"), Operations: []string{"claude-haiku", "GET /v1/models"},
			Owner: "claude-haiku"},
	}, scopes, "one scope per endpoint the gateway has; the missing one is skipped")

	_, ok = resolve("rest-uuid")
	assert.False(t, ok, "keys of other kinds are sent as they always were")
	_, ok = resolve("unknown")
	assert.False(t, ok)
}
