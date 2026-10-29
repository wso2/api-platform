/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 */

package transform

import (
	"testing"

	management "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func failoverPolicy(params map[string]interface{}) *[]management.Policy {
	list := []management.Policy{{Name: FailoverPolicyName, Params: &params}}
	return &list
}

func everyProviderResolves(provider string) string { return "cluster_" + provider }

func noProviderResolves(string) string { return "" }

func TestBuildFailoverConfig_AbsentPolicyYieldsNil(t *testing.T) {
	list := []management.Policy{{Name: "model-roundrobin"}}
	if cfg := buildFailoverConfig(&list, everyProviderResolves); cfg != nil {
		t.Fatalf("expected nil for an operation without the policy, got %+v", cfg)
	}
	if cfg := buildFailoverConfig(nil, everyProviderResolves); cfg != nil {
		t.Fatalf("expected nil for an operation with no policies at all, got %+v", cfg)
	}
}

func TestBuildFailoverConfig_ResolvesTargetsToClusters(t *testing.T) {
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{
				"model":    "gpt-4o",
				"provider": "openai",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "claude-sonnet-4", "provider": "anthropic"},
				},
			},
		},
	}), everyProviderResolves)

	if cfg == nil {
		t.Fatal("expected a config")
	}
	if got := cfg.ChainLength(); got != 2 {
		t.Fatalf("chain length = %d, want 2", got)
	}
	if cfg.Targets[1].Cluster != "cluster_anthropic" {
		t.Fatalf("second target cluster = %q, want cluster_anthropic", cfg.Targets[1].Cluster)
	}
}

// An unresolvable provider must not fail the deployment, and must not take the rest of the
// chain down with it (FR-006a).
func TestBuildFailoverConfig_UnresolvableTargetIsDroppedNotRejected(t *testing.T) {
	resolve := func(provider string) string {
		if provider == "openai" {
			return "cluster_openai"
		}
		return ""
	}
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{
				"model":    "gpt-4o",
				"provider": "openai",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "mystery", "provider": "not-deployed-here"},
				},
			},
		},
	}), resolve)

	if cfg == nil {
		t.Fatal("expected the resolvable half of the chain to survive")
	}
	if got := cfg.ChainLength(); got != 1 {
		t.Fatalf("chain length = %d, want 1", got)
	}
}

// Nothing resolvable means the operation generates no specifier and no retry policy, and is
// indistinguishable from one the policy was never attached to (FR-030b, FR-030e).
func TestBuildFailoverConfig_NothingResolvableIsASilentNoOp(t *testing.T) {
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{"model": "gpt-4o", "provider": "openai"},
		},
	}), noProviderResolves)

	if cfg != nil {
		t.Fatalf("expected nil, got %+v", cfg)
	}
}

func TestBuildFailoverConfig_DefaultsApplyWhenUnspecified(t *testing.T) {
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{"model": "gpt-4o", "provider": "openai"},
		},
	}), everyProviderResolves)

	if cfg.PerAttemptTimeoutMs != defaultPerAttemptTimeoutMs {
		t.Fatalf("per-attempt timeout = %d, want %d", cfg.PerAttemptTimeoutMs, defaultPerAttemptTimeoutMs)
	}
	if len(cfg.FailureConditions.StatusCodes) != len(defaultFailoverStatusCodes) {
		t.Fatalf("status codes = %v, want %v", cfg.FailureConditions.StatusCodes, defaultFailoverStatusCodes)
	}
}

// statusCodes REPLACES the default list, and perTryTimeout is authored in seconds.
func TestBuildFailoverConfig_OverridesFailureConditions(t *testing.T) {
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{"model": "gpt-4o", "provider": "openai"},
		},
		"statusCodes":   []interface{}{float64(429), float64(503)},
		"perTryTimeout": float64(5),
	}), everyProviderResolves)

	if len(cfg.FailureConditions.StatusCodes) != 2 || cfg.FailureConditions.StatusCodes[0] != 429 {
		t.Fatalf("status codes = %v, want exactly [429 503]", cfg.FailureConditions.StatusCodes)
	}
	if cfg.PerAttemptTimeoutMs != 5000 {
		t.Fatalf("per-attempt timeout = %dms, want 5000 (5s)", cfg.PerAttemptTimeoutMs)
	}
}

// Every member of every chain gets a group, because the controller cannot know which chain a
// request will select — that depends on the model the client asks for.
func TestBuildFailoverConfig_FlattensEveryChain(t *testing.T) {
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{
				"model": "gpt-4o",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "claude-sonnet-4", "provider": "anthropic"},
				},
			},
			map[string]interface{}{
				"model": "gpt-4.1",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "gpt-4.1", "provider": "azure"},
				},
			},
		},
	}), everyProviderResolves)

	if got := cfg.ChainLength(); got != 4 {
		t.Fatalf("members = %d, want 4 — both chains and both fallbacks need groups", got)
	}
}

// Two chains falling back to the same member share one group; one group is what a shared
// health circuit requires.
func TestBuildFailoverConfig_DeduplicatesSharedMembers(t *testing.T) {
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{
				"model": "gpt-4o",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "rescue", "provider": "anthropic"},
				},
			},
			map[string]interface{}{
				"model": "gpt-4.1",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "rescue", "provider": "anthropic"},
				},
			},
		},
	}), everyProviderResolves)

	if got := cfg.ChainLength(); got != 3 {
		t.Fatalf("members = %d, want 3 — the shared fallback must appear once", got)
	}
}

// A fallback with no provider inherits the CHAIN'S provider, not the proxy primary. The
// controller must apply the same rule the policy does, or they disagree about which cluster a
// fallback belongs to.
func TestBuildFailoverConfig_FallbackInheritsTheChainsProvider(t *testing.T) {
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{
				"model":    "claude-sonnet-4",
				"provider": "anthropic",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "claude-opus-4"},
				},
			},
		},
	}), everyProviderResolves)

	if cfg.Targets[1].Cluster != "cluster_anthropic" {
		t.Fatalf("inherited cluster = %q, want cluster_anthropic", cfg.Targets[1].Cluster)
	}
}

// A target with no model names nothing; one with no provider means the API's own main
// upstream, which the resolver maps for us.
func TestBuildFailoverConfig_SkipsChainsWithoutAModel(t *testing.T) {
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{"provider": "openai"},
			map[string]interface{}{"model": "gpt-4o", "provider": "openai"},
		},
	}), everyProviderResolves)

	if got := cfg.ChainLength(); got != 1 {
		t.Fatalf("chain length = %d, want 1", got)
	}
	if cfg.Targets[0].Model != "gpt-4o" {
		t.Fatalf("target model = %q, want gpt-4o", cfg.Targets[0].Model)
	}
}

func TestBuildProviderClusterMap(t *testing.T) {
	defs := []management.UpstreamDefinition{
		{Name: "anthropic", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://anthropic.example:443"}}},
		{Name: "no-url", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: ""}}},
	}
	clusters := BuildProviderClusterMap("upstream_main_host_8080", &defs, "LlmProxy", "abc")

	// The empty key is the API's own main upstream — what a target naming no provider uses.
	if clusters[""] != "upstream_main_host_8080" {
		t.Fatalf("main upstream = %v", clusters[""])
	}
	if clusters["anthropic"] != "upstream_LlmProxy_abc_anthropic" {
		t.Fatalf("anthropic = %v", clusters["anthropic"])
	}
	// A definition with no usable URL yields no cluster, so the policy publishes a bare name
	// for it rather than pointing at a cluster that was never built.
	if _, present := clusters["no-url"]; present {
		t.Fatal("a definition with no URL must not appear in the map")
	}
}

// The map must reach the policy that needs it, and nothing else.
func TestInjectProviderClusters(t *testing.T) {
	chain := &models.PolicyChain{Policies: []models.Policy{
		{Name: "model-round-robin"},
		{Name: FailoverPolicyName, Params: map[string]interface{}{"targets": []interface{}{}}},
	}}
	InjectProviderClusters(chain, map[string]interface{}{"azure": "upstream_azure"})

	if _, present := chain.Policies[0].Params[ProviderClustersParam]; present {
		t.Fatal("the map must be injected only into model-failover")
	}
	injected, ok := chain.Policies[1].Params[ProviderClustersParam].(map[string]interface{})
	if !ok || injected["azure"] != "upstream_azure" {
		t.Fatalf("failover params = %v", chain.Policies[1].Params)
	}
}

func TestInjectProviderClusters_HandlesAChainWithoutTheParamsMap(t *testing.T) {
	chain := &models.PolicyChain{Policies: []models.Policy{{Name: FailoverPolicyName}}}
	InjectProviderClusters(chain, map[string]interface{}{"azure": "upstream_azure"})
	if chain.Policies[0].Params[ProviderClustersParam] == nil {
		t.Fatal("a policy with nil Params must still receive the map")
	}
}

// The main upstream slot's cluster key is derived from host and port alone, so every LlmProxy
// with a loopback primary shares `upstream_main_127.0.0.1_8080`. Per-attempt state keyed on a
// shared cluster collides ACROSS APIs: two proxies publish different chains under the same key,
// the last write wins, and one API's per-attempt policies — including its credential — are
// handed to another API's request.
//
// The primary also has its own API-scoped loopback definition. That is the one to key on.
func TestProviderClusterMap_EmptyAliasMustNotUseTheSharedMainCluster(t *testing.T) {
	// What BuildProviderClusterMap produces before the LLM layer corrects it.
	clusters := BuildProviderClusterMap("upstream_main_127.0.0.1_8080", nil, "LlmProxy", "api-one")
	if clusters[""] != "upstream_main_127.0.0.1_8080" {
		t.Fatalf("precondition changed: empty alias = %v", clusters[""])
	}

	// A second API would produce the IDENTICAL key, which is the collision.
	other := BuildProviderClusterMap("upstream_main_127.0.0.1_8080", nil, "LlmProxy", "api-two")
	if clusters[""] != other[""] {
		t.Fatal("expected the shared main cluster to collide across APIs; if it no longer " +
			"does, the correction in llm.go may be unnecessary")
	}

	// Whereas an API-scoped definition cluster differs per API, which is why the LLM layer
	// remaps the empty alias onto the primary's own definition.
	defs := []management.UpstreamDefinition{{
		Name: "openai-primary",
		Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://127.0.0.1:8080"}},
	}}
	one := BuildProviderClusterMap("upstream_main_127.0.0.1_8080", &defs, "LlmProxy", "api-one")
	two := BuildProviderClusterMap("upstream_main_127.0.0.1_8080", &defs, "LlmProxy", "api-two")
	if one["openai-primary"] == two["openai-primary"] {
		t.Fatalf("the primary's own definition cluster must be API-scoped, got %q for both",
			one["openai-primary"])
	}
}

// Only APIs that actually attach failover get the per-attempt instance: a chain containing it
// declares RequiresRequestBody, so adding it unconditionally would make every provider on every
// LlmProxy buffer request bodies for a policy with nothing to do.
func TestPerAttemptPolicies_OnlyWhenFailoverIsAttached(t *testing.T) {
	withFailover := []management.Operation{{Policies: &[]management.Policy{{Name: FailoverPolicyName, Version: "v0"}}}}
	got := perAttemptPolicies(&withFailover)
	if len(got) != 1 || got[0].Name != FailoverPolicyName {
		t.Fatalf("got %v, want one model-failover entry", got)
	}
	// The engine rejects a chain whose policy carries no version, failing the WHOLE chain
	// closed — so an unversioned injected entry silently removes per-attempt processing.
	if got[0].Version != "v0" {
		t.Fatalf("version = %q, want the publisher's own v0", got[0].Version)
	}

	without := []management.Operation{{Policies: &[]management.Policy{{Name: "model-round-robin", Version: "v1"}}}}
	if got := perAttemptPolicies(&without); got != nil {
		t.Fatalf("got %v, want nil for an API without failover", got)
	}

	if got := perAttemptPolicies(nil); got != nil {
		t.Fatalf("got %v, want nil for no operations", got)
	}
}

// One instance serves every operation, with no params: the per-operation configuration was
// already consumed by the downstream pass that produced the ranking, and the upstream instance
// reads that ranking out of dynamic metadata instead.
func TestPerAttemptPolicies_SingleParameterlessInstance(t *testing.T) {
	ops := []management.Operation{
		{Policies: &[]management.Policy{{Name: FailoverPolicyName, Version: "v0"}}},
		{Policies: &[]management.Policy{{Name: FailoverPolicyName, Version: "v0"}}},
	}

	got := perAttemptPolicies(&ops)
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 regardless of how many operations attach it", len(got))
	}
	if len(got[0].Params) != 0 {
		t.Fatalf("params = %v, want none", got[0].Params)
	}
}
