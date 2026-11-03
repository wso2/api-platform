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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

type fakeLookup struct {
	providers   map[string]*Provider
	templates   map[string]*models.StoredLLMProviderTemplate
	definitions map[string]*models.PolicyDefinition
}

func (l *fakeLookup) Provider(handle string) (*Provider, error) {
	if p, ok := l.providers[handle]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, handle)
}

func (l *fakeLookup) Template(handle string) (*models.StoredLLMProviderTemplate, error) {
	if t, ok := l.templates[handle]; ok {
		return t, nil
	}
	return nil, errors.New("not found")
}

func (l *fakeLookup) PolicyDefinition(name, _ string) (*models.PolicyDefinition, error) {
	if d, ok := l.definitions[name]; ok {
		return d, nil
	}
	return nil, errors.New("not loaded")
}

func ptr[T any](v T) *T { return &v }

func template(handle, group string) *models.StoredLLMProviderTemplate {
	t := &models.StoredLLMProviderTemplate{Configuration: api.LLMProviderTemplate{Metadata: api.Metadata{Name: handle}}}
	if group != "" {
		t.Configuration.Spec.GroupId = ptr(group)
	}
	return t
}

func provider(handle, tmpl string, mutate ...func(*api.LLMProviderConfiguration)) *Provider {
	p := api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: handle},
		Spec: api.LLMProviderConfigData{
			DisplayName: handle, Version: "v1.0", Template: tmpl,
			Context:       ptr("/" + handle),
			AccessControl: &api.LLMAccessControl{Mode: api.AllowAll},
		},
	}
	for _, m := range mutate {
		m(&p)
	}
	return &Provider{UUID: "uuid-" + handle, DesiredState: models.StateDeployed, Config: p}
}

func unexposed(p *api.LLMProviderConfiguration) {
	none := api.LLMProviderConfigDataExposure(models.LLMProviderExposureNone)
	p.Spec.Exposure = &none
	p.Spec.Context = nil
	p.Spec.AccessControl = nil
}

func withAPIKeyAuth(prefix string) func(*api.LLMProviderConfiguration) {
	return func(p *api.LLMProviderConfiguration) {
		params := map[string]interface{}{"key": "x-api-key", "in": "header"}
		if prefix != "" {
			params["valuePrefix"] = prefix
		}
		p.Spec.GlobalPolicies = &[]api.Policy{{Name: "api-key-auth", Version: "v1", Params: &params}}
	}
}

func newLookup() *fakeLookup {
	return &fakeLookup{
		providers: map[string]*Provider{},
		templates: map[string]*models.StoredLLMProviderTemplate{
			"openai":          template("openai", "wso2-openai"),
			"anthropic":       template("anthropic", "wso2-anthropic"),
			"azure-openai":    template("azure-openai", "wso2-azure-openai"),
			"gemini":          template("gemini", "wso2-gemini"),
			"awsbedrock":      template("awsbedrock", "wso2-awsbedrock"),
			"mistralai":       template("mistralai", "wso2-mistralai"),
			"azureai-foundry": template("azureai-foundry", "wso2-azureai-foundry"),
			"custom":          template("custom", "acme-custom"),
		},
		definitions: map[string]*models.PolicyDefinition{
			"model-round-robin":      {Name: "model-round-robin", Version: "v1.1.1"},
			"semantic-model-routing": {Name: "semantic-model-routing", Version: "v1.0.0"},
			"llm-header-router":      {Name: "llm-header-router", Version: "v1.0.0"},
			"set-headers":            {Name: "set-headers", Version: "v1.0.0"},
		},
	}
}

func model(targets []api.ModelTarget, routing *api.ModelRouting) *api.ModelConfiguration {
	return &api.ModelConfiguration{
		Metadata: api.Metadata{Name: "m"},
		Spec:     api.ModelConfigData{DisplayName: "M", Version: "v1.0", ModelName: "m", Targets: targets, Routing: routing},
	}
}

func target(name, providerID, upstreamModel string) api.ModelTarget {
	return api.ModelTarget{Name: name, Provider: api.ModelTargetProvider{Id: providerID}, UpstreamModel: upstreamModel}
}

func apiKeyAuth(value string) *api.LLMUpstreamAuth {
	return &api.LLMUpstreamAuth{Type: api.LLMUpstreamAuthTypeApiKey, Header: ptr("x-api-key"), Value: ptr(value)}
}

func requireErrorMentioning(t *testing.T, err error, fragments ...string) {
	t.Helper()
	require.Error(t, err)
	for _, f := range fragments {
		assert.Contains(t, err.Error(), f)
	}
}

func strict() Options { return Options{DefaultVhost: "*", Strict: true} }

func TestResolve_DirectTarget(t *testing.T) {
	l := newLookup()
	l.providers["openai"] = provider("openai", "openai", unexposed)

	m, err := Resolve(model([]api.ModelTarget{target("a", "openai", "gpt-4o")}, nil), l, strict())
	require.NoError(t, err)
	require.Len(t, m.Targets, 1)
	assert.Equal(t, ModeDirect, m.Targets[0].Mode)
	assert.Nil(t, m.Routing)
}

func TestResolve_DirectTargetMustNotCarryAuth(t *testing.T) {
	l := newLookup()
	l.providers["openai"] = provider("openai", "openai", unexposed)
	tg := target("a", "openai", "gpt-4o")
	tg.Auth = apiKeyAuth("k")

	_, err := Resolve(model([]api.ModelTarget{tg}, nil), l, strict())
	requireErrorMentioning(t, err, "spec.targets[0].auth", "exposure: none")

	// Compiled leniently, the moot credential is ignored and the Model is served.
	lenient := strict()
	lenient.Strict = false
	m, err := Resolve(model([]api.ModelTarget{tg}, nil), l, lenient)
	require.NoError(t, err)
	assert.Nil(t, m.Targets[0].RouteAuth)
}

func TestResolve_RoutedTargetPresentsTheRouteCredential(t *testing.T) {
	l := newLookup()
	l.providers["claude"] = provider("claude", "anthropic", withAPIKeyAuth("Key"), func(p *api.LLMProviderConfiguration) {
		p.Spec.AccessControl = &api.LLMAccessControl{Mode: api.DenyAll, Exceptions: &[]api.RouteException{{
			Path: "/v1/messages", Methods: []api.RouteExceptionMethods{"POST"},
		}}}
	})
	tg := target("a", "claude", "claude-sonnet")
	tg.Auth = apiKeyAuth("route-key")

	m, err := Resolve(model([]api.ModelTarget{tg}, nil), l, strict())
	require.NoError(t, err)
	got := m.Targets[0]
	assert.Equal(t, ModeRoute, got.Mode)
	assert.Equal(t, "Key", got.RouteValuePrefix)
	assert.Equal(t, "/claude", got.RouteContext)
	assert.Equal(t, tg.Auth, got.RouteAuth)
}

func TestResolve_RoutedTargetAuthRules(t *testing.T) {
	cases := []struct {
		name     string
		provider func(*api.LLMProviderConfiguration)
		auth     *api.LLMUpstreamAuth
		wantErr  []string
	}{
		{name: "api-key-auth needs a key", provider: withAPIKeyAuth(""), wantErr: []string{"must set auth", "api-key"}},
		{name: "api-key-auth refuses none", provider: withAPIKeyAuth(""),
			auth: &api.LLMUpstreamAuth{Type: api.LLMUpstreamAuthTypeNone}, wantErr: []string{"must set auth"}},
		{name: "api-key-auth refuses oauth2", provider: withAPIKeyAuth(""),
			auth:    &api.LLMUpstreamAuth{Type: api.LLMUpstreamAuthTypeOauth2, PolicyParams: &map[string]interface{}{"a": 1}},
			wantErr: []string{"auth.type", "must be 'api-key'"}},
		{name: "unauthenticated route needs nothing", provider: func(*api.LLMProviderConfiguration) {}},
		{name: "jwt-auth cannot be presented to", provider: func(p *api.LLMProviderConfiguration) {
			p.Spec.GlobalPolicies = &[]api.Policy{{Name: "jwt-auth", Version: "v1"}}
		}, auth: apiKeyAuth("k"), wantErr: []string{"jwt-auth"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLookup()
			l.providers["p"] = provider("p", "openai", tc.provider)
			tg := target("a", "p", "gpt-4o")
			tg.Auth = tc.auth
			_, err := Resolve(model([]api.ModelTarget{tg}, nil), l, strict())
			if len(tc.wantErr) == 0 {
				require.NoError(t, err)
				return
			}
			requireErrorMentioning(t, err, tc.wantErr...)
		})
	}
}

func TestResolve_TransformerIsNotPartOfTheModel(t *testing.T) {
	// Which transformer a provider gets depends on the endpoint serving the Model, so
	// resolving a Model accepts any template, including ones no endpoint transforms
	// for yet: that is decided per endpoint by Bind.
	for _, tmpl := range []string{"openai", "anthropic", "azure-openai", "azureai-foundry", "custom"} {
		l := newLookup()
		l.providers["p"] = provider("p", tmpl, unexposed)
		_, err := Resolve(model([]api.ModelTarget{target("a", "p", "x")}, nil), l, strict())
		require.NoError(t, err, tmpl)
	}
}

func TestResolve_RoutedProviderVhost(t *testing.T) {
	l := newLookup()
	l.providers["named"] = provider("named", "openai", func(p *api.LLMProviderConfiguration) {
		p.Spec.Vhost = ptr("llm.internal.example.com")
	})
	l.providers["wild"] = provider("wild", "openai", func(p *api.LLMProviderConfiguration) {
		p.Spec.Vhost = ptr("*.example.com")
	})

	m, err := Resolve(model([]api.ModelTarget{target("a", "named", "gpt-4o")}, nil), l, strict())
	require.NoError(t, err)
	assert.Equal(t, "llm.internal.example.com", m.Targets[0].RouteHostname)

	_, err = Resolve(model([]api.ModelTarget{target("a", "wild", "gpt-4o")}, nil), l, strict())
	requireErrorMentioning(t, err, "wildcard vhost")

	// A provider on the gateway's default vhost is reached on it.
	l.providers["plain"] = provider("plain", "openai")
	opts := strict()
	opts.DefaultVhost = "gw.example.com"
	m, err = Resolve(model([]api.ModelTarget{target("a", "plain", "gpt-4o")}, nil), l, opts)
	require.NoError(t, err)
	assert.Equal(t, "gw.example.com", m.Targets[0].RouteHostname)
}

func TestResolve_ProviderMustExistAndBeDeployed(t *testing.T) {
	l := newLookup()
	_, err := Resolve(model([]api.ModelTarget{target("a", "missing", "x")}, nil), l, strict())
	requireErrorMentioning(t, err, "'missing' not found")

	l.providers["down"] = provider("down", "openai")
	l.providers["down"].DesiredState = models.StateUndeployed
	_, err = Resolve(model([]api.ModelTarget{target("a", "down", "x")}, nil), l, strict())
	requireErrorMentioning(t, err, "not deployed")
}

func TestResolve_RoutingRequiredAndPassedAsWritten(t *testing.T) {
	l := newLookup()
	l.providers["a"] = provider("a", "openai", unexposed)
	l.providers["b"] = provider("b", "openai", unexposed)
	targets := []api.ModelTarget{target("one", "a", "gpt-4o"), target("two", "b", "gpt-4o-mini")}

	_, err := Resolve(model(targets, nil), l, strict())
	requireErrorMentioning(t, err, "needs a routing policy")

	// A target's name, an upstream model, and an upstream model with its provider:
	// the router receives each exactly as written, and the route job matches it to
	// a target at request time.
	params := map[string]interface{}{"models": []interface{}{
		map[string]interface{}{"model": "one"},
		map[string]interface{}{"model": "gpt-4o-mini"},
		map[string]interface{}{"model": "gpt-4o-mini", "provider": "b"},
	}, "suspendDuration": 30}
	m, err := Resolve(model(targets, &api.ModelRouting{
		Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1"}, Params: &params,
	}), l, strict())
	require.NoError(t, err)
	require.NotNil(t, m.Routing)
	assert.Equal(t, PhaseHeaders, m.Routing.Router.Phase)
	assert.Equal(t, params, m.Routing.Params)

	// The resolved params are a copy: a change to them never reaches the stored Model.
	m.Routing.Params["suspendDuration"] = 0
	assert.Equal(t, 30, params["suspendDuration"])

	_, err = Resolve(model(targets, &api.ModelRouting{
		Policy: api.ModelRoutingPolicy{Name: "set-headers", Version: "v1"},
	}), l, strict())
	requireErrorMentioning(t, err, "not a model routing policy")
}

func TestResolve_HeaderPhaseCredentialNeedsHeaderPhaseRouting(t *testing.T) {
	l := newLookup()
	oauth := func(p *api.LLMProviderConfiguration) {
		p.Spec.Upstream.Auth = &struct {
			Header        *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
			PolicyName    *string                                   `json:"policyName,omitempty" yaml:"policyName,omitempty"`
			PolicyParams  *map[string]interface{}                   `json:"policyParams,omitempty" yaml:"policyParams,omitempty"`
			PolicyVersion *string                                   `json:"policyVersion,omitempty" yaml:"policyVersion,omitempty"`
			Type          api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
			Value         *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
		}{Type: api.LLMProviderConfigDataUpstreamAuthTypeOauth2, PolicyParams: &map[string]interface{}{"tokenEndpoint": "x"}}
	}
	l.providers["a"] = provider("a", "openai", unexposed, oauth)
	l.providers["b"] = provider("b", "openai", unexposed)
	targets := []api.ModelTarget{target("one", "a", "m1"), target("two", "b", "m2")}
	params := map[string]interface{}{"routes": []interface{}{map[string]interface{}{"model": "one", "utterances": []interface{}{"x"}}}, "defaultModel": "two"}

	_, err := Resolve(model(targets, &api.ModelRouting{
		Policy: api.ModelRoutingPolicy{Name: "semantic-model-routing", Version: "v1"}, Params: &params,
	}), l, strict())
	requireErrorMentioning(t, err, "request-header phase", "semantic-model-routing")

	// A header-phase router commits the provider before the credential runs.
	headerParams := map[string]interface{}{"mappings": []interface{}{map[string]interface{}{"headerValue": "a", "provider": "a"}}}
	_, err = Resolve(model(targets, &api.ModelRouting{
		Policy: api.ModelRoutingPolicy{Name: "llm-header-router", Version: "v1"}, Params: &headerParams,
	}), l, strict())
	require.NoError(t, err)
}

func TestResolve_OneCredentialPerRoutedProvider(t *testing.T) {
	l := newLookup()
	l.providers["p"] = provider("p", "openai", withAPIKeyAuth(""))
	one, two := target("one", "p", "m1"), target("two", "p", "m2")
	one.Auth, two.Auth = apiKeyAuth("k1"), apiKeyAuth("k2")
	params := map[string]interface{}{"models": []interface{}{map[string]interface{}{"model": "one"}}}
	routing := &api.ModelRouting{Policy: api.ModelRoutingPolicy{Name: "model-round-robin", Version: "v1"}, Params: &params}

	_, err := Resolve(model([]api.ModelTarget{one, two}, routing), l, strict())
	requireErrorMentioning(t, err, "same credential")

	two.Auth = apiKeyAuth("k1")
	_, err = Resolve(model([]api.ModelTarget{one, two}, routing), l, strict())
	require.NoError(t, err)
}

func TestResolve_ReportsEveryProblem(t *testing.T) {
	l := newLookup()
	_, err := Resolve(model([]api.ModelTarget{target("a", "x", "m"), target("b", "y", "m")}, nil), l, strict())
	var errs Errors
	require.True(t, errors.As(err, &errs))
	assert.GreaterOrEqual(t, len(errs), 3, "two missing providers and the missing router: %v", err)
	assert.True(t, strings.HasPrefix(err.Error(), fmt.Sprintf("%d error(s)", len(errs))))
}

func TestAccessControlAllows(t *testing.T) {
	deny := &api.LLMAccessControl{Mode: api.DenyAll, Exceptions: &[]api.RouteException{
		{Path: "/chat/completions", Methods: []api.RouteExceptionMethods{"POST"}},
		{Path: "/models/{id}", Methods: []api.RouteExceptionMethods{"GET"}},
		{Path: "/v2/*", Methods: []api.RouteExceptionMethods{"POST"}},
	}}
	assert.True(t, accessControlAllows(deny, "POST", "/chat/completions"))
	assert.False(t, accessControlAllows(deny, "GET", "/chat/completions"))
	assert.False(t, accessControlAllows(deny, "POST", "/chat/completions/x"))
	assert.True(t, accessControlAllows(deny, "GET", "/models/gpt-4o"))
	assert.False(t, accessControlAllows(deny, "GET", "/models/a/b"))
	assert.True(t, accessControlAllows(deny, "POST", "/v2/anything/below"))

	allow := &api.LLMAccessControl{Mode: api.AllowAll, Exceptions: &[]api.RouteException{
		{Path: "/admin", Methods: []api.RouteExceptionMethods{"POST"}},
	}}
	assert.True(t, accessControlAllows(allow, "POST", "/chat/completions"))
	assert.False(t, accessControlAllows(allow, "POST", "/admin"))
	assert.False(t, accessControlAllows(nil, "POST", "/chat/completions"))
}
