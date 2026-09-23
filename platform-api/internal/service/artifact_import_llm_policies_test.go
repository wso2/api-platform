/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"gopkg.in/yaml.v3"
)

func bptr(b bool) *bool { return &b }

// TestLiftLLMPolicies_RoundTrip drives a provider config that uses the first-class
// Security and RateLimiting fields through the actual CP->DP forward conversion
// (generateLLMProviderDeploymentYAML), then through the DP->CP import decode + lift,
// and asserts the first-class fields are reconstructed. This guards the inverse
// mapping against drift in the forward conversion.
func TestLiftLLMPolicies_RoundTrip(t *testing.T) {
	provider := &model.LLMProvider{
		ID:      "round-trip-provider",
		Name:    "Round Trip Provider",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Upstream: &model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "https://api.openai.com"}},
			Security: &model.SecurityConfig{
				Enabled: bptr(true),
				APIKey:  &model.APIKeySecurity{Enabled: bptr(true), Key: "Authorization", In: "header", ValuePrefix: "Bearer"},
			},
			RateLimiting: &model.LLMRateLimitingConfig{
				// Provider scope: resource-wise (default token + one resource request).
				ProviderLevel: &model.RateLimitingScopeConfig{
					ResourceWise: &model.ResourceWiseRateLimitingConfig{
						Default: model.RateLimitingLimitConfig{
							Token: &model.TokenRateLimit{Enabled: true, Count: 1000, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
						},
						Resources: []model.RateLimitingResourceLimit{
							{
								Resource: "/v1/chat",
								Limit: model.RateLimitingLimitConfig{
									Request: &model.RequestRateLimit{Enabled: true, Count: 60, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "minute"}},
								},
							},
						},
					},
				},
				// Consumer scope: global cost.
				ConsumerLevel: &model.RateLimitingScopeConfig{
					Global: &model.RateLimitingLimitConfig{
						Cost: &model.CostRateLimit{Enabled: true, Amount: 50, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
					},
				},
			},
			// A genuine, non-system policy that must survive the round-trip untouched.
			Policies: []model.LLMPolicy{
				{Name: "custom-guardrail", Version: "v1", Paths: []model.LLMPolicyPath{{Path: "/*", Methods: []string{"*"}}}},
			},
		},
	}

	// Forward: CP -> gateway YAML (security/rate-limit become policies).
	yamlDoc, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("generateLLMProviderDeploymentYAML: %v", err)
	}
	yamlBytes, err := yaml.Marshal(yamlDoc)
	if err != nil {
		t.Fatalf("marshal forward YAML: %v", err)
	}

	// Extract the spec block, mirroring what the gateway pushes back on import.
	var doc map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		t.Fatalf("unmarshal forward YAML: %v", err)
	}
	specMap, _ := doc["spec"].(map[string]interface{})
	if specMap == nil {
		t.Fatalf("forward YAML missing spec block")
	}

	// Import decode + reverse map, exactly as the LLM provider importer does:
	// decode into the deployment spec, then reconstruct the stored config.
	var spec dto.LLMProviderDeploymentSpec
	if err := utils.DecodeSpec(specMap, &spec); err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	cfg := mapLLMProviderSpecToConfig(spec)
	sec, rl, remaining := cfg.Security, cfg.RateLimiting, cfg.Policies

	// --- Security reconstructed ---
	if sec == nil || sec.APIKey == nil {
		t.Fatalf("security not reconstructed: %+v", sec)
	}
	if sec.APIKey.Key != "Authorization" || sec.APIKey.In != "header" || sec.APIKey.ValuePrefix != "Bearer" {
		t.Errorf("security apiKey = %+v, want key=Authorization in=header valuePrefix=Bearer", sec.APIKey)
	}
	if sec.Enabled == nil || !*sec.Enabled {
		t.Errorf("security.Enabled = %v, want true", sec.Enabled)
	}

	// --- RateLimiting reconstructed ---
	if rl == nil || rl.ProviderLevel == nil || rl.ProviderLevel.ResourceWise == nil {
		t.Fatalf("provider rate limiting not reconstructed as resource-wise: %+v", rl)
	}
	pdef := rl.ProviderLevel.ResourceWise.Default
	if pdef.Token == nil || pdef.Token.Count != 1000 || pdef.Token.Reset.Unit != "hour" || pdef.Token.Reset.Duration != 1 {
		t.Errorf("provider default token = %+v, want count=1000 reset=1h", pdef.Token)
	}
	if len(rl.ProviderLevel.ResourceWise.Resources) != 1 {
		t.Fatalf("provider resources = %d, want 1", len(rl.ProviderLevel.ResourceWise.Resources))
	}
	res := rl.ProviderLevel.ResourceWise.Resources[0]
	if res.Resource != "/v1/chat" || res.Limit.Request == nil || res.Limit.Request.Count != 60 || res.Limit.Request.Reset.Unit != "minute" {
		t.Errorf("provider resource = %+v, want /v1/chat request count=60 reset=1m", res)
	}
	if rl.ConsumerLevel == nil || rl.ConsumerLevel.Global == nil || rl.ConsumerLevel.Global.Cost == nil {
		t.Fatalf("consumer cost not reconstructed: %+v", rl.ConsumerLevel)
	}
	if rl.ConsumerLevel.Global.Cost.Amount != 50 || rl.ConsumerLevel.Global.Cost.Reset.Unit != "hour" {
		t.Errorf("consumer cost = %+v, want amount=50 reset=1h", rl.ConsumerLevel.Global.Cost)
	}

	// --- Genuine policies preserved; security/rate-limit policies stripped ---
	// The llm-cost tracker that the forward conversion auto-attaches alongside cost
	// limits is preserved (not dropped) so the AI Workspace can surface it.
	names := make(map[string]bool, len(remaining))
	for _, p := range remaining {
		names[p.Name] = true
	}
	if !names["custom-guardrail"] {
		t.Errorf("remaining policies = %+v, want custom-guardrail preserved", remaining)
	}
	if !names[importPolicyLLMCost] {
		t.Errorf("remaining policies = %+v, want the llm-cost tracker preserved", remaining)
	}
	for _, p := range remaining {
		switch p.Name {
		case importPolicyAPIKeyAuth, importPolicyTokenRateLimit, importPolicyAdvancedRateLimit,
			importPolicyBasicRateLimit, importPolicyCostRateLimit:
			t.Errorf("security/rate-limit policy %q leaked into remaining policies", p.Name)
		}
	}
}

// TestLiftLLMPolicies_NoSpecialPolicies verifies plain policies pass through and no
// security/rate-limiting is fabricated.
func TestLiftLLMPolicies_NoSpecialPolicies(t *testing.T) {
	in := []model.LLMPolicy{
		{Name: "custom-a"},
		{Name: "custom-b"},
	}
	sec, rl, remaining := liftLLMPolicies(in, true)
	if sec != nil {
		t.Errorf("security = %+v, want nil", sec)
	}
	if rl != nil {
		t.Errorf("rateLimiting = %+v, want nil", rl)
	}
	if len(remaining) != 2 {
		t.Errorf("remaining = %d, want 2", len(remaining))
	}
}

// TestLiftLLMPolicies_ProxyKeepsRateLimits verifies that with rate-limit lifting
// disabled (LLM proxies have no rate-limiting field) rate-limit policies are preserved
// as ordinary policies instead of being dropped, while security is still lifted. This
// guards the fix for gateway-pushed proxies losing e.g. llm-cost-based-ratelimit.
func TestLiftLLMPolicies_ProxyKeepsRateLimits(t *testing.T) {
	in := []model.LLMPolicy{
		{Name: importPolicyAPIKeyAuth, Paths: []model.LLMPolicyPath{{Path: "/*", Methods: []string{"*"},
			Params: map[string]interface{}{"key": "X-API-Key", "in": "header"}}}},
		{Name: importPolicyCostRateLimit, Paths: []model.LLMPolicyPath{{Path: "/*", Methods: []string{"*"},
			Params: map[string]interface{}{"budgetLimits": []interface{}{map[string]interface{}{"amount": 100, "duration": "1h"}}}}}},
		{Name: "content-length-guardrail", Paths: []model.LLMPolicyPath{{Path: "/chat/completions", Methods: []string{"POST"}}}},
	}
	sec, rl, remaining := liftLLMPolicies(in, false)
	if sec == nil || sec.APIKey == nil {
		t.Fatalf("security not lifted: %+v", sec)
	}
	if rl != nil {
		t.Errorf("rateLimiting = %+v, want nil (proxies have no rate-limiting field)", rl)
	}
	names := make(map[string]bool, len(remaining))
	for _, p := range remaining {
		names[p.Name] = true
	}
	if !names[importPolicyCostRateLimit] {
		t.Errorf("remaining = %+v, want %q preserved for proxies", remaining, importPolicyCostRateLimit)
	}
	if !names["content-length-guardrail"] {
		t.Errorf("remaining = %+v, want content-length-guardrail preserved", remaining)
	}
	if names[importPolicyAPIKeyAuth] {
		t.Errorf("api-key-auth leaked into remaining; it should be lifted to Security")
	}
}

func TestLiftAPIKeySecurity_RequiresKeyOrIn(t *testing.T) {
	t.Run("valuePrefix only is ignored", func(t *testing.T) {
		policy := model.LLMPolicy{
			Name: importPolicyAPIKeyAuth,
			Paths: []model.LLMPolicyPath{{
				Path:    "/*",
				Methods: []string{"*"},
				Params:  map[string]interface{}{"valuePrefix": "Bearer"},
			}},
		}

		if sec := liftAPIKeySecurity(policy); sec != nil {
			t.Fatalf("expected nil security when only valuePrefix is present, got %+v", sec)
		}
	})

	t.Run("key only still imports", func(t *testing.T) {
		policy := model.LLMPolicy{
			Name: importPolicyAPIKeyAuth,
			Paths: []model.LLMPolicyPath{{
				Path:    "/*",
				Methods: []string{"*"},
				Params:  map[string]interface{}{"key": "Authorization", "valuePrefix": "Bearer"},
			}},
		}

		sec := liftAPIKeySecurity(policy)
		if sec == nil || sec.APIKey == nil {
			t.Fatalf("expected security to be reconstructed")
		}
		if sec.APIKey.Key != "Authorization" || sec.APIKey.In != "" || sec.APIKey.ValuePrefix != "Bearer" {
			t.Fatalf("unexpected API key security: %+v", sec.APIKey)
		}
	})

	t.Run("in only still imports", func(t *testing.T) {
		policy := model.LLMPolicy{
			Name: importPolicyAPIKeyAuth,
			Paths: []model.LLMPolicyPath{{
				Path:    "/*",
				Methods: []string{"*"},
				Params:  map[string]interface{}{"in": "header", "valuePrefix": "Bearer"},
			}},
		}

		sec := liftAPIKeySecurity(policy)
		if sec == nil || sec.APIKey == nil {
			t.Fatalf("expected security to be reconstructed")
		}
		if sec.APIKey.Key != "" || sec.APIKey.In != "header" || sec.APIKey.ValuePrefix != "Bearer" {
			t.Fatalf("unexpected API key security: %+v", sec.APIKey)
		}
	})
}

// globalAPIKeyAuth builds an api-level api-key-auth attachment, the shape
// mapGlobalPoliciesAPIToLLMPolicies produces from spec.globalPolicies.
func globalAPIKeyAuth(key string) model.LLMPolicy {
	return model.LLMPolicy{
		Name:    importPolicyAPIKeyAuth,
		Version: "v1",
		Paths: []model.LLMPolicyPath{{
			Path:    "/*",
			Methods: []string{"*"},
			Params:  map[string]interface{}{"key": key, "in": "header"},
		}},
	}
}

// scopedAPIKeyAuth builds a resource-scoped api-key-auth attachment, the shape
// mapOperationPoliciesAPIToLLMPolicies produces from spec.operationPolicies.
func scopedAPIKeyAuth(key, path string, methods ...string) model.LLMPolicy {
	return model.LLMPolicy{
		Name:    importPolicyAPIKeyAuth,
		Version: "v1",
		Paths: []model.LLMPolicyPath{{
			Path:    path,
			Methods: methods,
			Params:  map[string]interface{}{"key": key, "in": "header"},
		}},
	}
}

// findPolicy returns the first policy with the given name, or nil.
func findPolicy(policies []model.LLMPolicy, name string) *model.LLMPolicy {
	for i := range policies {
		if policies[i].Name == name {
			return &policies[i]
		}
	}
	return nil
}

// TestLiftLLMPolicies_APIKeyAuthScope pins the scope rule for api-key-auth: an
// api-level attachment becomes first-class Security, a resource-scoped one stays a
// policy. Before this was enforced, a resource-scoped attachment was lifted with its
// path/method binding dropped — reporting an api-wide key the gateway never enforced —
// and, when both scopes were present, the plain `security = s` overwrite let the
// resource-scoped one silently replace the genuine api-level one.
func TestLiftLLMPolicies_APIKeyAuthScope(t *testing.T) {
	// liftRateLimits is irrelevant to api-key-auth, so both provider (true) and
	// proxy (false) importers must behave identically here.
	for _, liftRateLimits := range []bool{true, false} {
		name := "proxy"
		if liftRateLimits {
			name = "provider"
		}
		t.Run(name, func(t *testing.T) {
			t.Run("api-level attachment is lifted to Security", func(t *testing.T) {
				security, _, remaining := liftLLMPolicies(
					[]model.LLMPolicy{globalAPIKeyAuth("X-API-Key")}, liftRateLimits)

				if security == nil || security.APIKey == nil {
					t.Fatalf("expected Security to be reconstructed, got %+v", security)
				}
				if got := security.APIKey.Key; got != "X-API-Key" {
					t.Errorf("security.APIKey.Key = %q, want %q", got, "X-API-Key")
				}
				if p := findPolicy(remaining, importPolicyAPIKeyAuth); p != nil {
					t.Errorf("api-level api-key-auth leaked into remaining: %+v", p)
				}
			})

			t.Run("resource-scoped attachment stays a policy", func(t *testing.T) {
				security, _, remaining := liftLLMPolicies(
					[]model.LLMPolicy{scopedAPIKeyAuth("X-Resource-API-Key", "/models", "GET")},
					liftRateLimits)

				if security != nil {
					t.Fatalf("resource-scoped api-key-auth must not become api-wide Security, got %+v", security)
				}
				p := findPolicy(remaining, importPolicyAPIKeyAuth)
				if p == nil {
					t.Fatal("resource-scoped api-key-auth was dropped; want it preserved as a policy")
				}
				if len(p.Paths) != 1 || p.Paths[0].Path != "/models" {
					t.Fatalf("path binding not preserved: %+v", p.Paths)
				}
				if got := p.Paths[0].Methods; len(got) != 1 || got[0] != "GET" {
					t.Errorf("methods = %v, want [GET]", got)
				}
				if got := asString(p.Paths[0].Params["key"]); got != "X-Resource-API-Key" {
					t.Errorf("params.key = %q, want %q", got, "X-Resource-API-Key")
				}
			})

			t.Run("both scopes: neither overwrites the other", func(t *testing.T) {
				// Ordered as mapLLMProviderSpecToConfig builds liftInput:
				// globalPolicies first, then operationPolicies.
				security, _, remaining := liftLLMPolicies([]model.LLMPolicy{
					globalAPIKeyAuth("X-API-Key"),
					scopedAPIKeyAuth("X-Resource-API-Key", "/models", "GET"),
				}, liftRateLimits)

				if security == nil || security.APIKey == nil {
					t.Fatalf("expected the api-level attachment to be lifted, got %+v", security)
				}
				if got := security.APIKey.Key; got != "X-API-Key" {
					t.Errorf("security.APIKey.Key = %q, want the api-level %q — the "+
						"resource-scoped attachment must not overwrite it", got, "X-API-Key")
				}
				p := findPolicy(remaining, importPolicyAPIKeyAuth)
				if p == nil {
					t.Fatal("resource-scoped api-key-auth was dropped")
				}
				if len(p.Paths) != 1 || p.Paths[0].Path != "/models" {
					t.Fatalf("resource-scoped paths = %+v, want only /models", p.Paths)
				}
				if got := asString(p.Paths[0].Params["key"]); got != "X-Resource-API-Key" {
					t.Errorf("resource-scoped params.key = %q, want %q", got, "X-Resource-API-Key")
				}
			})

			t.Run("one policy carrying both scopes is split", func(t *testing.T) {
				// A legacy `spec.policies` entry may list an api-level path and a
				// resource path under one policy; each half must go to its own home.
				security, _, remaining := liftLLMPolicies([]model.LLMPolicy{{
					Name:    importPolicyAPIKeyAuth,
					Version: "v1",
					Paths: []model.LLMPolicyPath{
						{Path: "/*", Methods: []string{"*"},
							Params: map[string]interface{}{"key": "X-API-Key", "in": "header"}},
						{Path: "/models", Methods: []string{"GET"},
							Params: map[string]interface{}{"key": "X-Resource-API-Key", "in": "header"}},
					},
				}}, liftRateLimits)

				if security == nil || security.APIKey == nil || security.APIKey.Key != "X-API-Key" {
					t.Fatalf("api-level half not lifted to Security: %+v", security)
				}
				p := findPolicy(remaining, importPolicyAPIKeyAuth)
				if p == nil {
					t.Fatal("resource-scoped half was dropped")
				}
				if len(p.Paths) != 1 || p.Paths[0].Path != "/models" {
					t.Fatalf("remaining paths = %+v, want only the resource-scoped /models entry", p.Paths)
				}
			})

			t.Run("/* with explicit methods is resource-scoped", func(t *testing.T) {
				// splitLegacyPoliciesForRead treats "/*" with non-wildcard methods as
				// an operation policy; the lift must agree, or the entry would be
				// lifted here yet expected in operationPolicies on read.
				security, _, remaining := liftLLMPolicies(
					[]model.LLMPolicy{scopedAPIKeyAuth("X-Post-Key", "/*", "POST")}, liftRateLimits)

				if security != nil {
					t.Fatalf("\"/*\" + [POST] is not api-level; got Security %+v", security)
				}
				if findPolicy(remaining, importPolicyAPIKeyAuth) == nil {
					t.Fatal("entry was dropped instead of kept as a policy")
				}
			})
		})
	}
}

// TestLiftLLMPolicies_APIKeyAuthScopeSurvivesReadSplit closes the loop: a
// resource-scoped api-key-auth left in the policy list by the lift must land in
// operationPolicies (not globalPolicies) when the stored list is split for a read
// response, so the UI renders it against its own resource.
func TestLiftLLMPolicies_APIKeyAuthScopeSurvivesReadSplit(t *testing.T) {
	security, _, remaining := liftLLMPolicies([]model.LLMPolicy{
		globalAPIKeyAuth("X-API-Key"),
		scopedAPIKeyAuth("X-Resource-API-Key", "/models", "GET"),
	}, true)

	if security == nil || security.APIKey == nil || security.APIKey.Key != "X-API-Key" {
		t.Fatalf("api-level attachment not lifted: %+v", security)
	}

	global, operation := splitLegacyPoliciesForRead(remaining)

	for _, p := range global {
		if p.Name == importPolicyAPIKeyAuth {
			t.Errorf("resource-scoped api-key-auth surfaced as a global policy: %+v", p)
		}
	}
	var found *model.OperationPolicy
	for i := range operation {
		if operation[i].Name == importPolicyAPIKeyAuth {
			found = &operation[i]
		}
	}
	if found == nil {
		t.Fatal("resource-scoped api-key-auth did not surface in operationPolicies")
	}
	if len(found.Paths) != 1 || found.Paths[0].Path != "/models" {
		t.Fatalf("operation policy paths = %+v, want only /models", found.Paths)
	}
}

// TestAPIKeyAuthScope_RoundTrip drives a provider carrying BOTH an api-level
// api-key-auth (via the first-class Security field) and a resource-scoped one (as an
// operation policy) through the real CP->DP forward conversion and back through the
// DP->CP import, asserting each lands in its own home. This is the end-to-end guard:
// the unit tests above pin liftLLMPolicies, this pins the whole pipeline the gateway
// artifact actually travels.
func TestAPIKeyAuthScope_RoundTrip(t *testing.T) {
	provider := &model.LLMProvider{
		ID:      "api-key-scope-provider",
		Name:    "API Key Scope Provider",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Upstream: &model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "https://api.openai.com"}},
			// api-level: becomes a global api-key-auth policy on the wire.
			Security: &model.SecurityConfig{
				Enabled: bptr(true),
				APIKey:  &model.APIKeySecurity{Enabled: bptr(true), Key: "X-API-Key", In: "header"},
			},
			// resource-scoped: stays an operation policy on the wire.
			OperationPolicies: []model.OperationPolicy{{
				Name:    importPolicyAPIKeyAuth,
				Version: "v1",
				Paths: []model.OperationPolicyPath{{
					Path:    "/models",
					Methods: []string{"GET"},
					Params:  map[string]interface{}{"key": "X-Resource-API-Key", "in": "header"},
				}},
			}},
		},
	}

	yamlDoc, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("generateLLMProviderDeploymentYAML: %v", err)
	}
	yamlBytes, err := yaml.Marshal(yamlDoc)
	if err != nil {
		t.Fatalf("marshal forward YAML: %v", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		t.Fatalf("unmarshal forward YAML: %v", err)
	}
	specMap, _ := doc["spec"].(map[string]interface{})
	if specMap == nil {
		t.Fatal("forward YAML missing spec block")
	}
	var spec dto.LLMProviderDeploymentSpec
	if err := utils.DecodeSpec(specMap, &spec); err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}

	cfg := mapLLMProviderSpecToConfig(spec)

	// The api-level attachment is the one — and the only one — that becomes Security.
	if cfg.Security == nil || cfg.Security.APIKey == nil {
		t.Fatalf("api-level api-key-auth not reconstructed into Security: %+v", cfg.Security)
	}
	if got := cfg.Security.APIKey.Key; got != "X-API-Key" {
		t.Errorf("security.APIKey.Key = %q, want the api-level %q", got, "X-API-Key")
	}

	// The resource-scoped one surfaces as an operation policy, binding intact.
	_, operation := splitLegacyPoliciesForRead(cfg.Policies)
	var scoped *model.OperationPolicy
	for i := range operation {
		if operation[i].Name == importPolicyAPIKeyAuth {
			scoped = &operation[i]
		}
	}
	if scoped == nil {
		t.Fatalf("resource-scoped api-key-auth missing from operation policies: %+v", operation)
	}
	if len(scoped.Paths) != 1 || scoped.Paths[0].Path != "/models" {
		t.Fatalf("operation policy paths = %+v, want only /models", scoped.Paths)
	}
	if got := asString(scoped.Paths[0].Params["key"]); got != "X-Resource-API-Key" {
		t.Errorf("operation policy params.key = %q, want %q", got, "X-Resource-API-Key")
	}
}
