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

package modelgateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func openAIEndpoint(entries ...api.ModelEndpointTransformer) Endpoint {
	return Endpoint{
		Handle:          "wso2",
		InboundTemplate: "openai",
		InboundGroup:    "wso2-openai",
		Transformers:    entries,
		DispatchPaths:   []string{"/chat/completions"},
	}
}

func templateEntry(tmpl, policy string, params map[string]interface{}, paths ...string) api.ModelEndpointTransformer {
	e := api.ModelEndpointTransformer{Template: ptr(tmpl), Type: policy, Version: ptr("v0")}
	if params != nil {
		e.Params = &params
	}
	if len(paths) > 0 {
		e.UpstreamPaths = &paths
	}
	return e
}

// resolved resolves a one-target Model on a provider.
func resolved(t *testing.T, p *Provider, upstreamModel string) *Model {
	t.Helper()
	l := newLookup()
	l.providers[p.Config.Metadata.Name] = p
	m, err := Resolve(model([]api.ModelTarget{target("a", p.Config.Metadata.Name, upstreamModel)}, nil), l,
		Options{DefaultVhost: "*"})
	require.NoError(t, err)
	return m
}

func TestBind_ProviderOnTheInboundFormatPassesThrough(t *testing.T) {
	m := resolved(t, provider("openai-prod", "openai", unexposed), "gpt-4o")
	b, err := Bind(m, openAIEndpoint())
	require.NoError(t, err)
	tr, present := b.Transformers["openai-prod"]
	assert.True(t, present)
	assert.Nil(t, tr, "an OpenAI provider behind an OpenAI endpoint needs no transformer")

	// The same on an Anthropic endpoint for an Anthropic provider.
	m = resolved(t, provider("claude", "anthropic", unexposed), "claude-sonnet")
	b, err = Bind(m, Endpoint{Handle: "anthropic-native", InboundTemplate: "anthropic", InboundGroup: "wso2-anthropic",
		DispatchPaths: []string{"/v1/messages"}})
	require.NoError(t, err)
	assert.Nil(t, b.Transformers["claude"])
}

func TestBind_NoEntryLeavesTheModelOut(t *testing.T) {
	m := resolved(t, provider("claude", "anthropic", unexposed), "claude-sonnet")
	_, err := Bind(m, openAIEndpoint())
	requireErrorMentioning(t, err, "endpoint 'wso2' (inbound format 'openai')", "no transformer for provider 'claude'",
		"template 'anthropic'", "spec.transformers", "reads 'openai' requests", "remove the endpoint from the Model")

	// An OpenAI provider behind an Anthropic endpoint needs a transformer the
	// customer writes; with none, the Model is left out.
	m = resolved(t, provider("openai-prod", "openai", unexposed), "gpt-4o")
	_, err = Bind(m, Endpoint{Handle: "anthropic-native", InboundTemplate: "anthropic", DispatchPaths: []string{"/v1/messages"}})
	requireErrorMentioning(t, err, "endpoint 'anthropic-native' (inbound format 'anthropic')",
		"no transformer for provider 'openai-prod'", "reads 'anthropic' requests")
}

func TestBind_ProviderEntryWinsOverTemplateEntry(t *testing.T) {
	m := resolved(t, provider("claude", "anthropic", unexposed), "claude-sonnet")
	b, err := Bind(m, openAIEndpoint(
		templateEntry("anthropic", "openai-to-anthropic-transformer", nil),
		api.ModelEndpointTransformer{Provider: ptr("claude"), Type: "custom-claude-transformer", Version: ptr("v2")},
	))
	require.NoError(t, err)
	require.NotNil(t, b.Transformers["claude"])
	assert.Equal(t, "custom-claude-transformer", b.Transformers["claude"].PolicyName)
	assert.Equal(t, "v2", b.Transformers["claude"].Version)
}

func TestBind_TemplateEntryMatchesHandleOrGroup(t *testing.T) {
	entry := templateEntry("anthropic", "openai-to-anthropic-transformer", nil)

	// A customised copy of the out-of-box template: another handle, the same group.
	l := newLookup()
	l.templates["my-anthropic"] = template("my-anthropic", "wso2-anthropic")
	l.providers["claude"] = provider("claude", "my-anthropic", unexposed)
	m, err := Resolve(model([]api.ModelTarget{target("a", "claude", "c")}, nil), l, Options{})
	require.NoError(t, err)
	b, err := Bind(m, openAIEndpoint(entry))
	require.NoError(t, err)
	require.NotNil(t, b.Transformers["claude"])

	// A custom template is matched by its handle.
	m = resolved(t, provider("acme", "custom", unexposed), "a")
	b, err = Bind(m, openAIEndpoint(templateEntry("custom", "openai-to-acme", nil)))
	require.NoError(t, err)
	assert.Equal(t, "openai-to-acme", b.Transformers["acme"].PolicyName)
}

func TestBind_TypeNonePassesThrough(t *testing.T) {
	l := newLookup()
	l.templates["xai"] = template("xai", "")
	l.providers["grok"] = provider("grok", "xai", unexposed)
	m, err := Resolve(model([]api.ModelTarget{target("a", "grok", "grok-3")}, nil), l, Options{})
	require.NoError(t, err)

	_, err = Bind(m, openAIEndpoint())
	require.Error(t, err, "an OpenAI-compatible vendor still needs an explicit entry")

	b, err := Bind(m, openAIEndpoint(api.ModelEndpointTransformer{Template: ptr("xai"), Type: "none"}))
	require.NoError(t, err)
	assert.Nil(t, b.Transformers["grok"])
}

func TestBind_Placeholders(t *testing.T) {
	azureEntry := templateEntry("azure-openai", "openai-to-azure-openai-transformer",
		map[string]interface{}{"apiVersion": "{apiVersion}", "pathSuffix": "/chat/completions"})

	// {apiVersion} with no default needs the provider to state one.
	m := resolved(t, provider("az", "azure-openai", unexposed), "my-deployment")
	_, err := Bind(m, openAIEndpoint(azureEntry))
	requireErrorMentioning(t, err, "endpoint 'wso2', provider 'az'", "spec.upstream.apiVersion")

	m = resolved(t, provider("az", "azure-openai", unexposed, func(p *api.LLMProviderConfiguration) {
		p.Spec.Upstream.ApiVersion = ptr("2024-10-21")
	}), "my-deployment")
	b, err := Bind(m, openAIEndpoint(azureEntry))
	require.NoError(t, err)
	assert.Equal(t, "2024-10-21", b.Transformers["az"].Params["apiVersion"])
	assert.Equal(t, "/chat/completions", b.Transformers["az"].Params["pathSuffix"])

	// {apiVersion:<default>} falls back to the default, and is filled in nested values.
	geminiEntry := templateEntry("gemini", "openai-to-gemini-transformer", map[string]interface{}{
		"apiVersion": "{apiVersion:v1beta}",
		"nested":     map[string]interface{}{"list": []interface{}{"x-{apiVersion:v1beta}"}},
	})
	m = resolved(t, provider("g", "gemini", unexposed), "gemini-2.0-flash")
	b, err = Bind(m, openAIEndpoint(geminiEntry))
	require.NoError(t, err)
	assert.Equal(t, "v1beta", b.Transformers["g"].Params["apiVersion"])
	assert.Equal(t, []interface{}{"x-v1beta"}, b.Transformers["g"].Params["nested"].(map[string]interface{})["list"])

	m = resolved(t, provider("g", "gemini", unexposed, func(p *api.LLMProviderConfiguration) {
		p.Spec.Upstream.ApiVersion = ptr("v1")
	}), "gemini-2.0-flash")
	b, err = Bind(m, openAIEndpoint(geminiEntry))
	require.NoError(t, err)
	assert.Equal(t, "v1", b.Transformers["g"].Params["apiVersion"])
}

func TestBind_RoutedTargetNeedsItsPathsAllowed(t *testing.T) {
	geminiEntry := templateEntry("gemini", "openai-to-gemini-transformer",
		map[string]interface{}{"apiVersion": "{apiVersion:v1beta}"},
		"/{apiVersion:v1beta}/models/{upstreamModel}:generateContent",
		"/{apiVersion:v1beta}/models/{upstreamModel}:streamGenerateContent")

	routed := provider("g", "gemini", func(p *api.LLMProviderConfiguration) {
		// Only the non-streaming path is allowed; a streamed request would 404.
		p.Spec.AccessControl = &api.LLMAccessControl{Mode: api.DenyAll, Exceptions: &[]api.RouteException{{
			Path: "/v1beta/models/{model}:generateContent", Methods: []api.RouteExceptionMethods{"POST"},
		}}}
	})
	m := resolved(t, routed, "gemini-1.5-pro")
	_, err := Bind(m, openAIEndpoint(geminiEntry))
	requireErrorMentioning(t, err, ":streamGenerateContent", "accessControl", "endpoint 'wso2'")

	m.Targets[0].Provider.Config.Spec.AccessControl.Exceptions = &[]api.RouteException{{
		Path: "/v1beta/models/*", Methods: []api.RouteExceptionMethods{"POST"},
	}}
	_, err = Bind(m, openAIEndpoint(geminiEntry))
	require.NoError(t, err)

	// A direct target has no route, so nothing is checked.
	m = resolved(t, provider("g", "gemini", unexposed), "gemini-1.5-pro")
	_, err = Bind(m, openAIEndpoint(geminiEntry))
	require.NoError(t, err)
}

func TestBind_RoutedTargetWithUndeclaredPathsIsAWarning(t *testing.T) {
	m := resolved(t, provider("claude", "anthropic"), "claude-sonnet")
	b, err := Bind(m, openAIEndpoint(templateEntry("anthropic", "openai-to-anthropic-transformer", nil)))
	require.NoError(t, err, "a transformer that declares no paths is accepted, as an LLM proxy's is")
	require.Len(t, b.Warnings, 1)
	assert.Contains(t, b.Warnings[0], "declares no upstreamPaths")
}

func TestBind_PassThroughRoutedTargetIsCheckedOnTheEndpointPaths(t *testing.T) {
	denyChat := func(p *api.LLMProviderConfiguration) {
		p.Spec.AccessControl = &api.LLMAccessControl{Mode: api.DenyAll}
	}
	m := resolved(t, provider("oa", "openai", denyChat), "gpt-4o")
	_, err := Bind(m, openAIEndpoint())
	requireErrorMentioning(t, err, "POST /chat/completions")

	// A Gemini endpoint's model segment is filled with the target's upstream model.
	allowFlash := func(p *api.LLMProviderConfiguration) {
		p.Spec.AccessControl = &api.LLMAccessControl{Mode: api.DenyAll, Exceptions: &[]api.RouteException{{
			Path: "/v1beta/models/gemini-flash:generateContent", Methods: []api.RouteExceptionMethods{"POST"},
		}}}
	}
	m = resolved(t, provider("g", "gemini", allowFlash), "gemini-flash")
	_, err = Bind(m, Endpoint{Handle: "gem", InboundTemplate: "gemini", InboundGroup: "wso2-gemini",
		DispatchPaths: []string{"/v1beta/models/{model}:generateContent"}})
	require.NoError(t, err)
}

func TestTemplateMatches(t *testing.T) {
	assert.True(t, TemplateMatches(template("anthropic", "wso2-anthropic"), "anthropic"))
	assert.True(t, TemplateMatches(template("my-anthropic", "wso2-anthropic"), "anthropic"))
	assert.True(t, TemplateMatches(template("my-anthropic", "wso2-anthropic"), "wso2-anthropic"))
	assert.True(t, TemplateMatches(template("custom", ""), "custom"))
	assert.False(t, TemplateMatches(template("custom", ""), "anthropic"))
	assert.False(t, TemplateMatches(nil, "anthropic"))
	assert.False(t, TemplateMatches(template("custom", ""), ""))
}
