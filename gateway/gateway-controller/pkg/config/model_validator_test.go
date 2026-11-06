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

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func validModel() api.ModelConfiguration {
	return api.ModelConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "Model",
		Metadata:   api.Metadata{Name: "gpt-4o"},
		Spec: api.ModelConfigData{
			DisplayName: "GPT-4o",
			Version:     "v1.0",
			ModelName:   "gpt-4o",
			Endpoints:   []string{"wso2"},
			Targets: []api.ModelTarget{{
				Name: "gpt-4o", Provider: "openai-provider", Alias: stringPtr("openai"),
			}},
		},
	}
}

func TestModelValidator_Valid(t *testing.T) {
	m := validModel()
	assert.Empty(t, NewModelValidator().Validate(&m))
}

func TestModelValidator_TargetAliases(t *testing.T) {
	routing := &api.ModelRouting{Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1"}}
	cases := map[string][]api.ModelTarget{
		"no alias":                {{Name: "gpt-4o", Provider: "openai-provider"}},
		"a provider's model name": {{Name: "anthropic.claude-3-5-sonnet-20240620-v1:0", Provider: "bedrock"}},
		"same model on two providers, one aliased": {
			{Name: "gpt-4o", Provider: "openai-provider"},
			{Name: "gpt-4o", Provider: "azure-provider", Alias: stringPtr("azure")},
		},
		"alias equal to its own name": {{Name: "gpt-4o", Provider: "openai-provider", Alias: stringPtr("gpt-4o")}},
	}
	for name, targets := range cases {
		t.Run(name, func(t *testing.T) {
			m := validModel()
			m.Spec.Targets = targets
			if len(targets) > 1 {
				m.Spec.Routing = routing
			}
			assert.Empty(t, NewModelValidator().Validate(&m))
		})
	}
}

func TestModelValidator_Rules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*api.ModelConfiguration)
		field  string
	}{
		{"kind", func(m *api.ModelConfiguration) { m.Kind = "LlmProxy" }, "kind"},
		{"api version", func(m *api.ModelConfiguration) { m.ApiVersion = "x/v2" }, "apiVersion"},
		{"handle", func(m *api.ModelConfiguration) { m.Metadata.Name = "Bad_Handle" }, "metadata.name"},
		{"display name", func(m *api.ModelConfiguration) { m.Spec.DisplayName = "" }, "spec.displayName"},
		{"version", func(m *api.ModelConfiguration) { m.Spec.Version = "1.0" }, "spec.version"},
		{"model name required", func(m *api.ModelConfiguration) { m.Spec.ModelName = "" }, "spec.modelName"},
		{"model name length", func(m *api.ModelConfiguration) {
			m.Spec.ModelName = strings.Repeat("a", MaxModelNameBytes+1)
		}, "spec.modelName"},
		{"model name whitespace", func(m *api.ModelConfiguration) { m.Spec.ModelName = " gpt" }, "spec.modelName"},
		{"model name control character", func(m *api.ModelConfiguration) { m.Spec.ModelName = "gpt\n4o" }, "spec.modelName"},
		{"no targets", func(m *api.ModelConfiguration) { m.Spec.Targets = nil }, "spec.targets"},
		{"no endpoints", func(m *api.ModelConfiguration) { m.Spec.Endpoints = nil }, "spec.endpoints"},
		{"empty endpoints", func(m *api.ModelConfiguration) { m.Spec.Endpoints = []string{} }, "spec.endpoints"},
		{"endpoint handle", func(m *api.ModelConfiguration) { m.Spec.Endpoints = []string{"Bad Handle"} }, "spec.endpoints[0]"},
		{"duplicate endpoint", func(m *api.ModelConfiguration) { m.Spec.Endpoints = []string{"wso2", "wso2"} }, "spec.endpoints[1]"},
		{"target name required", func(m *api.ModelConfiguration) { m.Spec.Targets[0].Name = "" }, "spec.targets[0].name"},
		{"target name length", func(m *api.ModelConfiguration) {
			m.Spec.Targets[0].Name = strings.Repeat("a", MaxModelNameBytes+1)
		}, "spec.targets[0].name"},
		{"target name whitespace", func(m *api.ModelConfiguration) { m.Spec.Targets[0].Name = " gpt-4o" }, "spec.targets[0].name"},
		{"target provider", func(m *api.ModelConfiguration) { m.Spec.Targets[0].Provider = "" }, "spec.targets[0].provider"},
		{"target provider handle", func(m *api.ModelConfiguration) { m.Spec.Targets[0].Provider = "Bad Handle" }, "spec.targets[0].provider"},
		{"target alias empty", func(m *api.ModelConfiguration) { m.Spec.Targets[0].Alias = stringPtr("") }, "spec.targets[0].alias"},
		{"target alias pattern", func(m *api.ModelConfiguration) { m.Spec.Targets[0].Alias = stringPtr("a b") }, "spec.targets[0].alias"},
		{"duplicate target alias", func(m *api.ModelConfiguration) {
			second := m.Spec.Targets[0]
			second.Name = "gpt-4o-mini"
			m.Spec.Targets = append(m.Spec.Targets, second)
			m.Spec.Routing = &api.ModelRouting{Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1"}}
		}, "spec.targets[1].alias"},
		{"same model on two providers without an alias", func(m *api.ModelConfiguration) {
			m.Spec.Targets = []api.ModelTarget{
				{Name: "gpt-4o", Provider: "openai-provider"},
				{Name: "gpt-4o", Provider: "azure-provider"},
			}
			m.Spec.Routing = &api.ModelRouting{Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1"}}
		}, "spec.targets[1].alias"},
		{"alias naming another target", func(m *api.ModelConfiguration) {
			m.Spec.Targets = append(m.Spec.Targets, api.ModelTarget{Name: "gpt-4o-mini", Provider: "openai-provider", Alias: stringPtr("gpt-4o")},
				api.ModelTarget{Name: "gpt-4o", Provider: "azure-provider"})
			m.Spec.Routing = &api.ModelRouting{Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1"}}
		}, "spec.targets[2].alias"},
		{"duplicate provider and model", func(m *api.ModelConfiguration) {
			second := m.Spec.Targets[0]
			second.Alias = stringPtr("other")
			m.Spec.Targets = append(m.Spec.Targets, second)
			m.Spec.Routing = &api.ModelRouting{Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1"}}
		}, "spec.targets[1]"},
		{"two targets need routing", func(m *api.ModelConfiguration) {
			m.Spec.Targets = append(m.Spec.Targets, api.ModelTarget{
				Name: "gpt-4o-mini", Provider: "openai-provider", Alias: stringPtr("b")})
		}, "spec.routing"},
		{"routing version", func(m *api.ModelConfiguration) {
			m.Spec.Routing = &api.ModelRouting{Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1.0.0"}}
		}, "spec.routing.policy.version"},
		{"target auth", func(m *api.ModelConfiguration) {
			m.Spec.Targets[0].Auth = &api.LLMUpstreamAuth{Type: api.LLMUpstreamAuthTypeApiKey}
		}, "spec.targets[0].auth.header"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validModel()
			tc.mutate(&m)
			errs := NewModelValidator().Validate(&m)
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tc.field)
		})
	}
}

func validExposedProvider() api.LLMProviderConfiguration {
	p := validProviderWithResilience(nil)
	return p
}

func TestLLMValidator_ProviderExposure(t *testing.T) {
	v := NewLLMValidator()

	t.Run("route needs accessControl", func(t *testing.T) {
		p := validExposedProvider()
		p.Spec.AccessControl = nil
		assert.Contains(t, fieldsOf(v.Validate(&p)), "spec.accessControl")
	})

	t.Run("none needs no accessControl", func(t *testing.T) {
		p := validExposedProvider()
		none := api.LLMProviderConfigDataExposure(models.LLMProviderExposureNone)
		p.Spec.Exposure = &none
		p.Spec.AccessControl = nil
		assert.Empty(t, v.Validate(&p))
	})

	t.Run("none refuses route-only fields", func(t *testing.T) {
		p := validExposedProvider()
		none := api.LLMProviderConfigDataExposure(models.LLMProviderExposureNone)
		manual := api.LLMProviderConfigDataUpstreamHostRewriteManual
		p.Spec.Exposure = &none
		p.Spec.Context = stringPtr("/openai")
		p.Spec.Vhost = stringPtr("api.example.com")
		p.Spec.GlobalPolicies = &[]api.Policy{{Name: "api-key-auth", Version: "v1"}}
		p.Spec.Upstream.HostRewrite = &manual
		fields := fieldsOf(v.Validate(&p))
		for _, f := range []string{"spec.context", "spec.vhost", "spec.accessControl", "spec.globalPolicies", "spec.upstream.hostRewrite"} {
			assert.Contains(t, fields, f)
		}
	})

	t.Run("unknown exposure", func(t *testing.T) {
		p := validExposedProvider()
		bad := api.LLMProviderConfigDataExposure("public")
		p.Spec.Exposure = &bad
		assert.Contains(t, fieldsOf(v.Validate(&p)), "spec.exposure")
	})

	t.Run("api version", func(t *testing.T) {
		p := validExposedProvider()
		p.Spec.Upstream.ApiVersion = stringPtr("2024-10-21")
		assert.Empty(t, v.Validate(&p))
		p.Spec.Upstream.ApiVersion = stringPtr("2024 10 21")
		assert.Contains(t, fieldsOf(v.Validate(&p)), "spec.upstream.apiVersion")
	})
}

func TestModelValidator_EndpointsNotCheckedAgainstGateway(t *testing.T) {
	// A Model may name endpoints the gateway does not have: it is served on the ones
	// it has, and naming the right ones is the configurator's job.
	m := validModel()
	m.Spec.Endpoints = []string{"wso2", "partners", "not-on-this-gateway"}
	assert.Empty(t, NewModelValidator().Validate(&m))
}

func TestConfig_Validate_ModelsConfig(t *testing.T) {
	cases := []struct {
		name    string
		models  ModelsConfig
		wantErr string
		check   func(*testing.T, ModelsConfig)
	}{
		{name: "defaults", models: ModelsConfig{}, check: func(t *testing.T, m ModelsConfig) {
			assert.Equal(t, DefaultModelsContext, m.Endpoint.Context)
			assert.Equal(t, int64(1048576), m.Endpoint.MaxRequestBodyBytes)
			assert.False(t, m.ProvisionsDefaultEndpoint(), "nothing is provisioned while the feature is off")
		}},
		{name: "provisions when enabled", models: ModelsConfig{Enabled: true, Endpoint: ModelEndpointConfig{Provision: true}},
			check: func(t *testing.T, m ModelsConfig) { assert.True(t, m.ProvisionsDefaultEndpoint()) }},
		{name: "provision off", models: ModelsConfig{Enabled: true},
			check: func(t *testing.T, m ModelsConfig) { assert.False(t, m.ProvisionsDefaultEndpoint()) }},
		{name: "root context", models: ModelsConfig{Endpoint: ModelEndpointConfig{Context: "/"}}, wantErr: "must not be '/'"},
		{name: "relative context", models: ModelsConfig{Endpoint: ModelEndpointConfig{Context: "wso2"}}, wantErr: "absolute path"},
		{name: "trailing slash", models: ModelsConfig{Endpoint: ModelEndpointConfig{Context: "/wso2/"}}, wantErr: "absolute path"},
		{name: "dot segment", models: ModelsConfig{Endpoint: ModelEndpointConfig{Context: "/wso2/../x"}}, wantErr: "absolute path"},
		{name: "health namespace", models: ModelsConfig{Endpoint: ModelEndpointConfig{Context: "/_gateway-health/x"}}, wantErr: "health-check"},
		{name: "negative body", models: ModelsConfig{Endpoint: ModelEndpointConfig{MaxRequestBodyBytes: -1}}, wantErr: "negative"},
		{name: "body above listener", models: ModelsConfig{Endpoint: ModelEndpointConfig{MaxRequestBodyBytes: 1 << 30}}, wantErr: "raise both together"},
		{name: "custom", models: ModelsConfig{Enabled: true, Endpoint: ModelEndpointConfig{Context: "/ai/v1", MaxRequestBodyBytes: 4096}},
			check: func(t *testing.T, m ModelsConfig) {
				assert.Equal(t, "/ai/v1", m.Endpoint.Context)
				assert.Equal(t, int64(4096), m.Endpoint.MaxRequestBodyBytes)
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Controller.Models = tc.models
			err := cfg.Validate()
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			if tc.check != nil {
				tc.check(t, cfg.Controller.Models)
			}
		})
	}
}
