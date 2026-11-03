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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func validModelEndpoint() api.ModelEndpointConfiguration {
	return api.ModelEndpointConfiguration{
		ApiVersion: api.ModelEndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.ModelEndpointConfigurationKindModelEndpoint,
		Metadata:   api.Metadata{Name: "partners"},
		Spec: api.ModelEndpointConfigData{
			DisplayName: "Partner Model Endpoint",
			Version:     "v1.0",
			Context:     stringPtr("/partners/v1"),
			Policies:    &[]api.Policy{{Name: "api-key-auth", Version: "v1"}},
		},
	}
}

func transformerEntry(mutate func(*api.ModelEndpointTransformer)) api.ModelEndpointTransformer {
	e := api.ModelEndpointTransformer{
		Template: stringPtr("anthropic"),
		Type:     "openai-to-anthropic-transformer",
		Version:  stringPtr("v0"),
		Params:   &map[string]interface{}{"anthropicVersion": "{apiVersion:2023-06-01}"},
		UpstreamPaths: &[]string{
			"/v1/messages",
		},
	}
	if mutate != nil {
		mutate(&e)
	}
	return e
}

func TestModelEndpointValidator_Valid(t *testing.T) {
	e := validModelEndpoint()
	e.Spec.Transformers = &[]api.ModelEndpointTransformer{
		transformerEntry(nil),
		{Template: stringPtr("xai"), Type: "none"},
		{Provider: stringPtr("acme-prod"), Type: "openai-to-acme", Version: stringPtr("v1"),
			UpstreamPaths: &[]string{"/v2/models/{upstreamModel}:generate", "/{apiVersion:v1}/x/{apiVersion}"}},
	}
	assert.Empty(t, NewModelEndpointValidator().Validate(&e))
}

func TestModelEndpointValidator_DefaultHandleNeedsNoContext(t *testing.T) {
	// The provisioned endpoint's context is configuration.
	e := validModelEndpoint()
	e.Metadata.Name = models.DefaultModelEndpointHandle
	e.Spec.Context = nil
	assert.Empty(t, NewModelEndpointValidator().Validate(&e))

	e.Metadata.Name = "partners"
	assert.Contains(t, fieldsOf(NewModelEndpointValidator().Validate(&e)), "spec.context")
}

func TestModelEndpointValidator_DefaultHandleMatchesModels(t *testing.T) {
	assert.Equal(t, models.DefaultModelEndpointHandle, ModelEndpointDefaultHandle)
}

func TestModelEndpointValidator_Rules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*api.ModelEndpointConfiguration)
		field  string
	}{
		{"kind", func(e *api.ModelEndpointConfiguration) { e.Kind = "LlmProxy" }, "kind"},
		{"api version", func(e *api.ModelEndpointConfiguration) { e.ApiVersion = "x/v2" }, "apiVersion"},
		{"handle", func(e *api.ModelEndpointConfiguration) { e.Metadata.Name = "Bad_Handle" }, "metadata.name"},
		{"display name", func(e *api.ModelEndpointConfiguration) { e.Spec.DisplayName = "" }, "spec.displayName"},
		{"display name pattern", func(e *api.ModelEndpointConfiguration) { e.Spec.DisplayName = "a/b" }, "spec.displayName"},
		{"version", func(e *api.ModelEndpointConfiguration) { e.Spec.Version = "1.0" }, "spec.version"},
		{"root context", func(e *api.ModelEndpointConfiguration) { e.Spec.Context = stringPtr("/") }, "spec.context"},
		{"trailing slash", func(e *api.ModelEndpointConfiguration) { e.Spec.Context = stringPtr("/a/") }, "spec.context"},
		{"dot segment", func(e *api.ModelEndpointConfiguration) { e.Spec.Context = stringPtr("/a/../b") }, "spec.context"},
		{"health namespace", func(e *api.ModelEndpointConfiguration) {
			e.Spec.Context = stringPtr("/_gateway-health/models")
		}, "spec.context"},
		{"vhost", func(e *api.ModelEndpointConfiguration) { e.Spec.Vhost = stringPtr("not a host") }, "spec.vhost"},
		{"inbound template", func(e *api.ModelEndpointConfiguration) {
			e.Spec.InboundTemplate = stringPtr("awsbedrock")
		}, "spec.inboundTemplate"},
		{"policy version", func(e *api.ModelEndpointConfiguration) {
			e.Spec.Policies = &[]api.Policy{{Name: "api-key-auth", Version: "v1.0.0"}}
		}, "spec.policies[0].version"},
		{"deployment state", func(e *api.ModelEndpointConfiguration) {
			bad := api.ModelEndpointConfigDataDeploymentState("paused")
			e.Spec.DeploymentState = &bad
		}, "spec.deploymentState"},
		{"resilience", func(e *api.ModelEndpointConfiguration) {
			e.Spec.Resilience = &api.Resilience{Timeout: stringPtr("forever")}
		}, "spec.resilience.timeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := validModelEndpoint()
			tc.mutate(&e)
			errs := NewModelEndpointValidator().Validate(&e)
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tc.field)
		})
	}
}

func TestModelEndpointValidator_InboundTemplates(t *testing.T) {
	for _, tmpl := range []string{"openai", "anthropic", "gemini"} {
		e := validModelEndpoint()
		e.Spec.InboundTemplate = stringPtr(tmpl)
		assert.Empty(t, NewModelEndpointValidator().Validate(&e), tmpl)
	}
}

func TestValidateModelEndpointTransformers(t *testing.T) {
	cases := []struct {
		name    string
		entries []api.ModelEndpointTransformer
		field   string
	}{
		{"neither provider nor template", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.Template = nil
		})}, "t[0]"},
		{"both provider and template", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.Provider = stringPtr("p")
		})}, "t[0]"},
		{"duplicate template", []api.ModelEndpointTransformer{transformerEntry(nil), transformerEntry(nil)}, "t[1].template"},
		{"duplicate provider", []api.ModelEndpointTransformer{
			{Provider: stringPtr("p"), Type: "none"}, {Provider: stringPtr("p"), Type: "none"},
		}, "t[1].provider"},
		{"no type", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.Type = ""
		})}, "t[0].type"},
		{"no version", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.Version = nil
		})}, "t[0].version"},
		{"full version", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.Version = stringPtr("v1.2.0")
		})}, "t[0].version"},
		{"none with version", []api.ModelEndpointTransformer{{Template: stringPtr("xai"), Type: "none", Version: stringPtr("v1")}}, "t[0].version"},
		{"none with params", []api.ModelEndpointTransformer{{Template: stringPtr("xai"), Type: "none",
			Params: &map[string]interface{}{"a": "b"}}}, "t[0].params"},
		{"providerId set", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.Params = &map[string]interface{}{"providerId": "x"}
		})}, "t[0].params.providerId"},
		{"empty placeholder default", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.Params = &map[string]interface{}{"v": "{apiVersion:}"}
		})}, "t[0].params.v"},
		{"relative path", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.UpstreamPaths = &[]string{"v1/messages"}
		})}, "t[0].upstreamPaths[0]"},
		{"path with query", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.UpstreamPaths = &[]string{"/v1/messages?beta=true"}
		})}, "t[0].upstreamPaths[0]"},
		{"unknown placeholder", []api.ModelEndpointTransformer{transformerEntry(func(e *api.ModelEndpointTransformer) {
			e.UpstreamPaths = &[]string{"/v1/{region}/messages"}
		})}, "t[0].upstreamPaths[0]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateModelEndpointTransformers("t", &tc.entries)
			require.NotEmpty(t, errs)
			assert.Contains(t, fieldsOf(errs), tc.field)
		})
	}
}

func TestLLMProxyTransformerSharesTheEntryRules(t *testing.T) {
	// One rule for a proxy's transformer and an endpoint's entries.
	errs := NewLLMValidator().validateLLMProxyTransformer("p", &api.LLMProxyTransformer{Type: "t", Version: "v1.2"})
	assert.Contains(t, fieldsOf(errs), "p.version")
	errs = ValidateModelEndpointTransformers("t", &[]api.ModelEndpointTransformer{{Template: stringPtr("a"), Type: "t", Version: stringPtr("v1.2")}})
	assert.Contains(t, fieldsOf(errs), "t[0].version")
}

func TestModelEndpointWarnings(t *testing.T) {
	quota := func(keys ...string) map[string]interface{} {
		var extraction []interface{}
		for _, k := range keys {
			extraction = append(extraction, map[string]interface{}{"type": k})
		}
		q := map[string]interface{}{"name": "q"}
		if extraction != nil {
			q["keyExtraction"] = extraction
		}
		return q
	}
	spec := func(quotas ...map[string]interface{}) api.ModelEndpointConfigData {
		list := make([]interface{}, 0, len(quotas))
		for _, q := range quotas {
			list = append(list, q)
		}
		return api.ModelEndpointConfigData{Policies: &[]api.Policy{{
			Name: "advanced-ratelimit", Version: "v1", Params: &map[string]interface{}{"quotas": list},
		}}}
	}

	// Keyed on the route — the default — counts each Model apart.
	assert.Len(t, ModelEndpointWarnings(spec(quota())), 1)
	assert.Len(t, ModelEndpointWarnings(spec(quota("authproperty"))), 1)
	assert.Len(t, ModelEndpointWarnings(spec(quota("apiname", "routename"))), 1)
	// Keyed on the API is shared across the endpoint's Models.
	assert.Empty(t, ModelEndpointWarnings(spec(quota("apiname"))))
	assert.Empty(t, ModelEndpointWarnings(spec(quota("apiname", "authproperty"))))
	// Other policies are not inspected.
	assert.Empty(t, ModelEndpointWarnings(api.ModelEndpointConfigData{Policies: &[]api.Policy{{Name: "basic-ratelimit", Version: "v1"}}}))
}
