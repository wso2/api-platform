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

package upstreamguard

import (
	"errors"
	"strings"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/httpkit/netguard"
)

// testDefinitions holds one policy with a top-level URL parameter, a nested
// one inside an array, a non-URL string, and a URL-typed system parameter
// (which the guard must never read).
func testDefinitions() map[string]models.PolicyDefinition {
	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"endpoint": map[string]any{"type": "string", "format": "uri"},
			"header":   map[string]any{"type": "string"},
			"proxyURL": map[string]any{"type": "string", "x-wso2-url": true, "default": ""},
			// A URL declared through a composed schema.
			"callbackUrl": map[string]any{"allOf": []any{map[string]any{"type": "string"}, map[string]any{"format": "uri"}}},
			"mirrorUrl":   map[string]any{"anyOf": []any{map[string]any{"type": "null"}, map[string]any{"type": "string", "x-wso2-url": true}}},
			"label":       map[string]any{"oneOf": []any{map[string]any{"type": "string", "maxLength": 8}, "not-a-schema"}},
			// Object values checked through additionalProperties, and an array
			// whose schema declares no items (its values cannot be typed).
			"mirrors": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string", "format": "uri"}},
			"tags":    map[string]any{"type": "array"},
			"keyManagers": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"jwksUri": map[string]any{"type": "string", "format": "uri"},
					},
				},
			},
		},
	}
	system := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"collectorUrl": map[string]any{"type": "string", "format": "uri"},
		},
	}
	return map[string]models.PolicyDefinition{
		"interceptor|v1.0.0": {Name: "interceptor", Version: "v1.0.0", Parameters: &params, SystemParameters: &system},
		"noparams|v1.0.0":    {Name: "noparams", Version: "v1.0.0"},
	}
}

func guard(t *testing.T, gc netguard.GuardConfig) *Guard {
	t.Helper()
	policy, err := gc.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return New(policy, testDefinitions(), nil)
}

func restAPI(mainURL string, policies ...map[string]any) map[string]any {
	spec := map[string]any{
		"upstream": map[string]any{"main": map[string]any{"url": mainURL}},
	}
	if len(policies) > 0 {
		list := make([]any, len(policies))
		for i, p := range policies {
			list[i] = p
		}
		spec["operations"] = []any{map[string]any{"method": "GET", "path": "/x", "policies": list}}
	}
	return map[string]any{"apiVersion": "gateway.api-platform.wso2.com/v1alpha1", "kind": "RestApi", "spec": spec}
}

func interceptor(params map[string]any) map[string]any {
	return map[string]any{"name": "interceptor", "version": "v1", "params": params}
}

func refusedField(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	if !IsRefused(err) {
		t.Fatalf("got a non-refusal error: %v", err)
	}
	if err.Error() != refusedMessage {
		t.Fatalf("refusal message must be fixed, got %q", err.Error())
	}
	return RefusedFieldPath(err)
}

func TestCheck_AlwaysRefusedAddresses(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})
	for _, u := range []string{
		"http://169.254.169.254/latest/meta-data",
		"http://[fd00:ec2::254]/",
		"http://[fe80::1]:8080/",
		"http://0.0.0.0:9000/",
	} {
		if got := refusedField(t, g.Check(restAPI(u))); got != "spec.upstream.main.url" {
			t.Errorf("%s: refused field = %q, want spec.upstream.main.url", u, got)
		}
	}
	// Positive control: a private backend is fine while private networks are permitted.
	if err := g.Check(restAPI("http://10.0.0.5:8080/api")); err != nil {
		t.Fatalf("private upstream with private networks permitted: %v", err)
	}
}

func TestCheck_EveryUpstreamShape(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})
	bad := "http://169.254.169.254/"
	cases := map[string]map[string]any{
		"spec.upstream.url": {"spec": map[string]any{"upstream": map[string]any{"url": bad}}},
		"spec.upstream.sandbox.url": {"spec": map[string]any{"upstream": map[string]any{
			"main": map[string]any{"url": "http://ok.example.com"}, "sandbox": map[string]any{"url": bad},
		}}},
		"spec.upstreamDefinitions[1].upstreams[0].url": {"spec": map[string]any{"upstreamDefinitions": []any{
			map[string]any{"name": "a", "upstreams": []any{map[string]any{"url": "http://ok.example.com"}}},
			map[string]any{"name": "b", "upstreams": []any{map[string]any{"url": bad}}},
		}}},
	}
	for want, cfg := range cases {
		if got := refusedField(t, g.Check(cfg)); got != want {
			t.Errorf("refused field = %q, want %q", got, want)
		}
	}
}

// A gateway-wide Backend's endpoints are upstream URLs the router dials, so they are checked like
// any other; the field names the refused endpoint.
func TestCheckArtifact_BackendEndpoints(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})
	backend := map[string]any{"spec": map[string]any{"endpoints": []any{
		map[string]any{"url": "http://ok.example.com:9080"},
		map[string]any{"url": "http://169.254.169.254"},
	}}}
	if got := refusedField(t, g.CheckArtifact(string(models.KindBackend), backend)); got != "spec.endpoints[1].url" {
		t.Errorf("refused field = %q, want spec.endpoints[1].url", got)
	}
	ok := map[string]any{"spec": map[string]any{"endpoints": []any{map[string]any{"url": "http://ok.example.com:9080"}}}}
	if err := g.CheckArtifact(string(models.KindBackend), ok); err != nil {
		t.Fatalf("positive control: a permitted endpoint is accepted: %v", err)
	}
}

// With private networks denied, a Backend endpoint at a loopback, private, link-local or cloud
// metadata address is refused, as an API upstream URL is; a public address is permitted.
func TestCheckArtifact_BackendEndpointsDenyPrivateNetworks(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})
	backend := func(url string) map[string]any {
		return map[string]any{"spec": map[string]any{"endpoints": []any{map[string]any{"url": url}}}}
	}
	for _, url := range []string{
		"http://127.0.0.1:9080",     // loopback
		"http://10.0.0.5:9080",      // private
		"http://192.168.1.10:9080",  // private
		"http://169.254.10.10:9080", // link-local
		"http://169.254.169.254",    // cloud metadata
		"http://[::1]:9080",         // IPv6 loopback
		"http://[fd00::1]:9080",     // IPv6 unique-local
	} {
		if got := refusedField(t, g.CheckArtifact(string(models.KindBackend), backend(url))); got != "spec.endpoints[0].url" {
			t.Errorf("%s: refused field = %q, want spec.endpoints[0].url", url, got)
		}
	}
	if err := g.CheckArtifact(string(models.KindBackend), backend("http://93.184.216.34:9080")); err != nil {
		t.Fatalf("positive control: a public address is permitted: %v", err)
	}
}

func TestCheck_HostRuleModes(t *testing.T) {
	deny := guard(t, netguard.GuardConfig{Enabled: true, Mode: netguard.ModeDeny, Hostnames: []string{`[a-z]+\.internal\.example`}})
	if refusedField(t, deny.Check(restAPI("https://billing.internal.example/v1"))) == "" {
		t.Fatal("deny mode must refuse a listed host")
	}
	if err := deny.Check(restAPI("https://10.0.0.5/v1")); err != nil {
		t.Fatalf("positive control: deny mode permits an unlisted host: %v", err)
	}

	allow := guard(t, netguard.GuardConfig{Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{`10\.0\.0\.5`}})
	if refusedField(t, allow.Check(restAPI("https://10.0.0.6/v1"))) == "" {
		t.Fatal("allow mode must refuse an unlisted host")
	}
	if err := allow.Check(restAPI("https://10.0.0.5/v1")); err != nil {
		t.Fatalf("positive control: allow mode permits a listed host: %v", err)
	}

	anchored := guard(t, netguard.GuardConfig{Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{`api\.example\.com`}})
	for _, raw := range []string{
		"https://api.example.com.evil.test/v1",
		"https://evil.test/?x=api.example.com",
		"https://apiXexample.com/v1",
	} {
		if refusedField(t, anchored.Check(restAPI(raw))) == "" {
			t.Fatalf("a pattern must match the whole hostname, but %q was permitted", raw)
		}
	}
}

// TestCheck_SchemesThatAreNeverDialedAreLeftToTheValidators: a non-http(s)
// scheme is not refused here, so the validators' specific message reaches the
// caller, while allowed_schemes still narrows http and https.
func TestCheck_SchemesThatAreNeverDialedAreLeftToTheValidators(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})
	for _, raw := range []string{"ftp://169.254.169.254/x", "file://localhost/etc/passwd", "gopher://10.0.0.1/"} {
		if err := g.Check(restAPI(raw)); err != nil {
			t.Fatalf("%q must be left to the validators, got %v", raw, err)
		}
	}
	if refusedField(t, g.Check(restAPI("http://169.254.169.254/x"))) == "" {
		t.Fatal("positive control: an http URL to the metadata address must be refused")
	}

	httpsOnly := guard(t, netguard.GuardConfig{Enabled: true, AllowedSchemes: []string{"https"}})
	if refusedField(t, httpsOnly.Check(restAPI("http://api.example.com/v1"))) == "" {
		t.Fatal("allowed_schemes = [\"https\"] must refuse an http upstream")
	}
	if err := httpsOnly.Check(restAPI("https://api.example.com/v1")); err != nil {
		t.Fatalf("positive control: allowed_schemes = [\"https\"] permits https: %v", err)
	}
}

// TestCheck_AllowedHostMustAlsoPassAddressRules: with deny_private_networks an
// allow-listed hostname that resolves to loopback is still refused.
func TestCheck_AllowedHostMustAlsoPassAddressRules(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{"localhost"}, DenyPrivateNetworks: true})
	if refusedField(t, g.Check(restAPI("http://localhost:8080/"))) == "" {
		t.Fatal("an allow-listed host resolving to loopback must be refused when private networks are denied")
	}
	permit := guard(t, netguard.GuardConfig{Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{"localhost"}})
	if err := permit.Check(restAPI("http://localhost:8080/")); err != nil {
		t.Fatalf("positive control with private networks permitted: %v", err)
	}
}

func TestCheck_UnresolvableHostIsNotRefused(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})
	if err := g.Check(restAPI("http://backend.invalid:8080/")); err != nil {
		t.Fatalf("a host the controller cannot resolve is left to the router: %v", err)
	}
}

func TestCheck_PolicyURLParameters(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})

	err := g.Check(restAPI("http://ok.example.com", interceptor(map[string]any{"endpoint": "http://169.254.169.254/"})))
	if got := refusedField(t, err); got != "spec.operations[0].policies[0].params.endpoint" {
		t.Fatalf("refused field = %q", got)
	}

	nested := interceptor(map[string]any{"keyManagers": []any{
		map[string]any{"jwksUri": "https://idp.example.com/jwks"},
		map[string]any{"jwksUri": "http://[fe80::1]/jwks"},
	}})
	if got := refusedField(t, g.Check(restAPI("http://ok.example.com", nested))); got != "spec.operations[0].policies[0].params.keyManagers[1].jwksUri" {
		t.Fatalf("nested refused field = %q", got)
	}

	// A string parameter without a URL format is never treated as a URL.
	plain := interceptor(map[string]any{"header": "http://169.254.169.254/"})
	if err := g.Check(restAPI("http://ok.example.com", plain)); err != nil {
		t.Fatalf("a non-URL parameter must not be checked: %v", err)
	}
	// An unknown policy is the policy validator's to report.
	unknown := map[string]any{"name": "nope", "version": "v1", "params": map[string]any{"endpoint": "http://169.254.169.254/"}}
	if err := g.Check(restAPI("http://ok.example.com", unknown)); err != nil {
		t.Fatalf("an unknown policy must not be refused here: %v", err)
	}
}

// TestCheck_SystemParametersAreNeverRead pins the scope decision: system
// parameters come from config.toml and are not subject to access control.
func TestCheck_SystemParametersAreNeverRead(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{`ok\.example\.com`}})
	p := interceptor(map[string]any{})
	p["systemParameters"] = map[string]any{"collectorUrl": "http://localhost:4318"}
	if err := g.Check(restAPI("http://ok.example.com", p)); err != nil {
		t.Fatalf("system parameters must not be checked: %v", err)
	}
}

func TestRenderSpecHookRefusesAndLeavesConfigurationUnchanged(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})
	templateengine.SetPostRenderCheck(g.CheckArtifact)
	t.Cleanup(func() { templateengine.SetPostRenderCheck(nil) })

	original := restAPI("http://169.254.169.254/")
	cfg := &models.StoredConfig{Handle: "h", Kind: "RestApi", Configuration: original}
	err := templateengine.RenderSpec(cfg, nil, nil)

	var renderErr *templateengine.RenderError
	if !errors.As(err, &renderErr) || !IsRefused(err) || !IsArtifactRefusal(err) {
		t.Fatalf("RenderSpec = %v, want a RenderError wrapping a refusal", err)
	}
	if !errors.Is(err, netguard.ErrDestinationNotAllowed) {
		t.Fatal("a refusal must match netguard.ErrDestinationNotAllowed")
	}
	if RefusalReason(err) != "controller.server.network_security.access_control" {
		t.Fatalf("RefusalReason = %q", RefusalReason(err))
	}
	if strings.Contains(err.Error(), "169.254") {
		t.Fatalf("the error must not carry the URL: %v", err)
	}

	ok := &models.StoredConfig{Handle: "ok", Kind: "RestApi", Configuration: restAPI("http://10.0.0.5:8080/")}
	if err := templateengine.RenderSpec(ok, nil, nil); err != nil {
		t.Fatalf("positive control: %v", err)
	}
}

func TestIsArtifactRefusalCoversEnvAccess(t *testing.T) {
	if !IsArtifactRefusal(&funcs.EnvAccessError{}) {
		t.Fatal("an env access refusal is an artifact refusal")
	}
	if RefusalReason(&funcs.EnvAccessError{}) != "policy_engine.env_access.allow_list" {
		t.Fatal("env access refusals keep their reason")
	}
	if IsArtifactRefusal(errors.New("boom")) {
		t.Fatal("an unrelated error is not an artifact refusal")
	}
}

// TestCheck_URLMarkerWithoutFormat covers an optional URL parameter marked with
// x-wso2-url instead of format: uri (whose validation would reject "").
func TestCheck_URLMarkerWithoutFormat(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})
	err := g.Check(restAPI("http://ok.example.com", interceptor(map[string]any{"proxyURL": "http://169.254.169.254:3128"})))
	if got := refusedField(t, err); got != "spec.operations[0].policies[0].params.proxyURL" {
		t.Fatalf("refused field = %q", got)
	}
	if err := g.Check(restAPI("http://ok.example.com", interceptor(map[string]any{"proxyURL": ""}))); err != nil {
		t.Fatalf("an unset optional URL must not be checked: %v", err)
	}
}

// TestCheck_ComposedURLSchemas covers a URL parameter declared through allOf,
// anyOf or oneOf rather than at the top level of its schema.
func TestCheck_ComposedURLSchemas(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})
	for _, param := range []string{"callbackUrl", "mirrorUrl"} {
		err := g.Check(restAPI("http://ok.example.com", interceptor(map[string]any{param: "http://169.254.169.254/"})))
		if got := refusedField(t, err); got != "spec.operations[0].policies[0].params."+param {
			t.Fatalf("%s: refused field = %q", param, got)
		}
		if err := g.Check(restAPI("http://ok.example.com", interceptor(map[string]any{param: "https://hooks.example.com/"}))); err != nil {
			t.Fatalf("%s: positive control: %v", param, err)
		}
	}
	// A composed schema with no URL subschema is not a URL.
	if err := g.Check(restAPI("http://ok.example.com", interceptor(map[string]any{"label": "http://169.254.169.254/"}))); err != nil {
		t.Fatalf("a composed non-URL parameter must not be checked: %v", err)
	}
}

// TestCheckArtifact_LlmProxyRoutingIsNotChecked covers the RestAPI derived from
// an LlmProxy: its upstreams are the gateway's own loopback listener, generated
// by the controller, so they are not refused even when private networks are
// denied. The proxy's policy parameters are still checked, and the same
// upstream on any other kind is refused.
func TestCheckArtifact_LlmProxyRoutingIsNotChecked(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})
	derived := func(policies ...map[string]any) map[string]any {
		cfg := restAPI("http://127.0.0.1:8080/provider-ctx", policies...)
		cfg["spec"].(map[string]any)["upstreamDefinitions"] = []any{
			map[string]any{"name": "primary", "upstreams": []any{map[string]any{"url": "http://127.0.0.1:8080"}}},
		}
		return cfg
	}

	if err := g.CheckArtifact(string(models.KindLlmProxy), derived()); err != nil {
		t.Fatalf("generated loopback routing of an LlmProxy must not be refused: %v", err)
	}
	if refusedField(t, g.CheckArtifact(string(models.KindRestApi), derived())) != "spec.upstream.main.url" {
		t.Fatal("control: the same loopback upstream on a RestApi must be refused")
	}
	err := g.CheckArtifact(string(models.KindLlmProxy), derived(interceptor(map[string]any{"endpoint": "http://10.0.0.5/"})))
	if got := refusedField(t, err); got != "spec.operations[0].policies[0].params.endpoint" {
		t.Fatalf("an LlmProxy's policy URL must still be checked, refused field = %q", got)
	}
}

// TestRenderSpecPassesTheStoredKind checks that RenderSpec hands the stored
// artifact's kind to the post-render check, so a hydrated LlmProxy is
// recognised although it renders as a RestAPI.
func TestRenderSpecPassesTheStoredKind(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})
	templateengine.SetPostRenderCheck(g.CheckArtifact)
	t.Cleanup(func() { templateengine.SetPostRenderCheck(nil) })

	proxy := &models.StoredConfig{Handle: "p", Kind: string(models.KindLlmProxy), Configuration: restAPI("http://127.0.0.1:8080/ctx")}
	if err := templateengine.RenderSpec(proxy, nil, nil); err != nil {
		t.Fatalf("hydrated LlmProxy: %v", err)
	}
	api := &models.StoredConfig{Handle: "a", Kind: string(models.KindRestApi), Configuration: restAPI("http://127.0.0.1:8080/ctx")}
	if err := templateengine.RenderSpec(api, nil, nil); !IsRefused(err) {
		t.Fatalf("control: RestApi with a loopback upstream = %v, want a refusal", err)
	}
}

// TestCheck_InputsItCannotOrNeedNotInspect covers rendered values the guard
// cannot read and specs with nothing to check.
func TestCheck_InputsItCannotOrNeedNotInspect(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})

	if err := g.Check(make(chan int)); err == nil || IsRefused(err) {
		t.Fatalf("an unmarshalable value = %v, want an inspection error", err)
	}
	if err := g.Check("not an object"); err == nil || IsRefused(err) {
		t.Fatalf("a non-object value = %v, want an inspection error", err)
	}
	if err := g.Check(map[string]any{"kind": "RestApi"}); err != nil {
		t.Fatalf("a config without a spec has nothing to check: %v", err)
	}
	if RefusedFieldPath(errors.New("boom")) != "" {
		t.Fatal("RefusedFieldPath of a non-refusal must be empty")
	}

	// A malformed or host-less URL is the validators' to report.
	for _, raw := range []string{"http://[::1", "/relative/path", "http://"} {
		if err := g.Check(restAPI(raw)); err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
	}
}

func TestCheck_UpstreamDefinitions(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})
	cfg := restAPI("http://ok.example.com")
	cfg["spec"].(map[string]any)["upstreamDefinitions"] = []any{
		"not-an-object",
		map[string]any{"name": "a", "upstreams": []any{map[string]any{"url": "https://a.example.com"}}},
		map[string]any{"name": "b", "upstreams": []any{map[string]any{"url": "https://b.example.com"}, map[string]any{"url": "http://169.254.169.254"}}},
	}
	if got := refusedField(t, g.Check(cfg)); got != "spec.upstreamDefinitions[2].upstreams[1].url" {
		t.Fatalf("refused field = %q", got)
	}
	cfg["spec"].(map[string]any)["upstreamDefinitions"] = []any{
		map[string]any{"name": "a", "upstreams": []any{map[string]any{"url": "https://a.example.com"}}},
	}
	if err := g.Check(cfg); err != nil {
		t.Fatalf("positive control: %v", err)
	}
}

func TestCheck_ParameterSchemaShapes(t *testing.T) {
	g := guard(t, netguard.GuardConfig{Enabled: true})

	err := g.Check(restAPI("http://ok.example.com", interceptor(map[string]any{
		"mirrors": map[string]any{"eu": "https://eu.example.com", "us": "http://169.254.169.254"},
	})))
	if got := refusedField(t, err); got != "spec.operations[0].policies[0].params.mirrors.us" {
		t.Fatalf("additionalProperties refused field = %q", got)
	}
	ok := interceptor(map[string]any{"mirrors": map[string]any{"eu": "https://eu.example.com"}})
	if err := g.Check(restAPI("http://ok.example.com", ok)); err != nil {
		t.Fatalf("positive control: %v", err)
	}

	untyped := interceptor(map[string]any{"tags": []any{"http://169.254.169.254"}})
	if err := g.Check(restAPI("http://ok.example.com", untyped)); err != nil {
		t.Fatalf("array values without an items schema are not URLs: %v", err)
	}
	noParams := map[string]any{"name": "noparams", "version": "v1", "params": map[string]any{"endpoint": "http://169.254.169.254"}}
	if err := g.Check(restAPI("http://ok.example.com", noParams)); err != nil {
		t.Fatalf("a policy that declares no parameters has no URL parameters: %v", err)
	}
}
