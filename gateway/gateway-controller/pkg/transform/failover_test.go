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

// Inertness turns on whether any MEMBER resolves, not on the kind.
//
// This is what makes attaching the policy to a kind with no model concept safe, and it is
// sharper than it looks. Both transformers resolve an EMPTY provider to the API's main
// upstream, so a bare `model:` with no provider resolves anywhere — including on an Mcp,
// which has no model concept at all. Such a config is accepted and harmless (a one-member
// chain cannot fail over), but it is NOT the inert case: it still generates a priority group
// and marks the virtual host. Only a config where every member names a provider that this
// API cannot resolve collapses to nil and leaves the route byte-identical to no policy.
//
// The Mcp scenario in model-failover.feature names a provider on the PRIMARY target for
// exactly this reason; dropping it would silently turn that test into the other case.
func TestBuildFailoverConfig_InertOnlyWhenNoMemberResolves(t *testing.T) {
	// Mirrors resolveFailoverCluster on an API with no upstreamDefinitions: the empty
	// provider finds the main upstream, every named one finds nothing.
	mainUpstreamOnly := func(provider string) string {
		if provider == "" {
			return "cluster_main"
		}
		return ""
	}

	bareModel := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{
				"model": "gpt-4o",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "claude-sonnet-4", "provider": "anthropic"},
				},
			},
		},
	}), mainUpstreamOnly)

	if bareModel == nil {
		t.Fatal("a target with no provider resolves to the main upstream, so the config must " +
			"not be nil — if this ever becomes nil, the Mcp feature scenario stops " +
			"distinguishing the two cases it was written to separate")
	}
	if got := bareModel.ChainLength(); got != 1 {
		t.Fatalf("chain length = %d, want 1 (primary via main upstream, fallback dropped)", got)
	}
	if bareModel.Inert() {
		t.Fatal("a one-member chain is not inert; it cannot fail over, but it still generates config")
	}

	// Name a provider on every member and nothing resolves — the genuinely inert case.
	everyMemberNamed := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{
				"model":    "gpt-4o",
				"provider": "openai-primary",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "claude-sonnet-4", "provider": "anthropic-secondary"},
				},
			},
		},
	}), mainUpstreamOnly)

	if everyMemberNamed != nil {
		t.Fatalf("every member names an unresolvable provider, so the config must be nil, got %+v",
			everyMemberNamed)
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

// The idle-timeout default is a correctness boundary, not a tuning preference, and nothing
// guarded it until this test — the one place the value appeared in a test passed it as a
// literal input rather than asserting the default.
//
// per_try_idle_timeout bounds SILENCE within an attempt, and silence means time to first
// byte (probed 2026-10-30; see poc-results.md "Timeouts"). A non-streaming completion sends
// nothing until the answer is ready, so the default is the effective ceiling on how long a
// legitimate non-streaming request may take before the gateway cancels it and fails over to
// a second target for an answer the first was about to give.
func TestBuildFailoverConfig_IdleTimeoutDefaultDoesNotCutLegitimateRequests(t *testing.T) {
	cfg := buildFailoverConfig(failoverPolicy(map[string]interface{}{
		"targets": []interface{}{map[string]interface{}{"model": "gpt-4o", "provider": "openai"}},
	}), everyProviderResolves)

	if cfg == nil {
		t.Fatal("expected a config")
	}

	// 10s was the original value and is the specific regression: non-streaming LLM
	// completions routinely exceed it, so every one of them was being failed over.
	const cutsLegitimateTraffic = 10000
	if cfg.PerAttemptIdleTimeoutMs <= cutsLegitimateTraffic {
		t.Fatalf("idle timeout default = %dms; at or below %dms a non-streaming completion "+
			"is cancelled mid-flight and failed over while healthy",
			cfg.PerAttemptIdleTimeoutMs, cutsLegitimateTraffic)
	}

	// Bounded above too: the route's overall timeout is terminal and fires first, so a value
	// at or beyond it never takes effect. Keeping the assertion here means raising the
	// default past the route timeout fails loudly rather than becoming silently inert.
	const routeTimeoutMs = 60000 // pkg/config/config.go RouteTimeoutMs default
	if cfg.PerAttemptIdleTimeoutMs >= routeTimeoutMs {
		t.Fatalf("idle timeout default = %dms, at or above the %dms route timeout; the route "+
			"timeout fires first and terminally, so this value would never apply",
			cfg.PerAttemptIdleTimeoutMs, routeTimeoutMs)
	}
}

// An operation with failover must get a fault-chain entry without the publisher asking: with
// handle_upstream_faults on, the engine skips every response policy for a failing response, so
// without this entry health stops being graded exactly when it matters.
func TestPrependFailoverFaultEntry_AttachesAutomaticallyAndFirst(t *testing.T) {
	resolved := models.Policy{
		Name:    FailoverPolicyName,
		Version: "v0.1.0",
		Params:  map[string]interface{}{ProviderClustersParam: map[string]string{"": "cluster_default"}},
	}
	chain := []models.Policy{{Name: "api-key-auth"}, resolved}
	authored := []models.Policy{{Name: "operator-notifier"}}

	got := prependFailoverFaultEntry(&models.FailoverConfig{}, chain, authored)

	if len(got) != 2 {
		t.Fatalf("fault chain has %d entries, want 2", len(got))
	}
	if got[0].Name != FailoverPolicyName {
		t.Fatalf("first entry is %q, want %q — the exhaustion error must be a default the "+
			"operator's own entries can still reshape", got[0].Name, FailoverPolicyName)
	}
	if got[1].Name != "operator-notifier" {
		t.Fatalf("the operator's entry was lost: %+v", got)
	}
	// Cloned from the resolved entry, so the fault instance resolves the same chain. Without
	// the injected cluster map it cannot resolve one at all.
	if got[0].Version != "v0.1.0" {
		t.Fatalf("version = %q, want the resolved one", got[0].Version)
	}
	if _, ok := got[0].Params[ProviderClustersParam]; !ok {
		t.Fatal("the injected provider-to-cluster map did not travel, so the fault instance cannot resolve a chain")
	}
}

// Inert failover means the policy publishes nothing and grades nothing. Adding a fault entry
// would put it on the error path of an API it does not apply to.
func TestPrependFailoverFaultEntry_LeavesInertOperationsAlone(t *testing.T) {
	chain := []models.Policy{{Name: FailoverPolicyName, Version: "v0.1.0"}}
	authored := []models.Policy{{Name: "operator-notifier"}}

	got := prependFailoverFaultEntry(nil, chain, authored)
	if len(got) != 1 || got[0].Name != "operator-notifier" {
		t.Fatalf("an operation with no resolved failover gained a fault entry: %+v", got)
	}
}

// An operation that resolved failover but somehow carries no such entry in its response chain
// must not fabricate one — a rebuilt entry would drift from the real configuration.
func TestPrependFailoverFaultEntry_NeverFabricatesAnEntry(t *testing.T) {
	authored := []models.Policy{{Name: "operator-notifier"}}
	got := prependFailoverFaultEntry(&models.FailoverConfig{}, []models.Policy{{Name: "api-key-auth"}}, authored)
	if len(got) != 1 || got[0].Name != "operator-notifier" {
		t.Fatalf("an entry was fabricated: %+v", got)
	}
}

// ── moveFailoverLast ─────────────────────────────────────────────────────────
//
// Chain order looks like it already decides whether a steering policy is honoured, and it only
// half does. The engine runs every policy's OnRequestHeaders before any policy's OnRequestBody,
// and model-failover resolves its chain in the body phase — so a steering policy that publishes
// in the header phase is honoured wherever it sits, while one that publishes only in the body
// phase is silently ignored if written afterwards. Same YAML, opposite outcomes, decided by
// which Go interfaces a policy implements, which an author cannot see and the controller cannot
// read. These pin the control plane deciding instead.

func TestMoveFailoverLast_MovesItBehindAPolicyWrittenAfterIt(t *testing.T) {
	chain := []models.Policy{
		{Name: FailoverPolicyName},
		{Name: "semantic-model-routing"}, // publishes in the body phase only
	}
	got, changed := moveFailoverLast(&models.FailoverConfig{}, chain)
	if !changed {
		t.Fatal("changed = false; the policy was first and had to move")
	}
	if len(got) != 2 || got[0].Name != "semantic-model-routing" || got[1].Name != FailoverPolicyName {
		t.Fatalf("chain = %+v, want the steering policy first and failover last", got)
	}
}

// Relative order of everything else is untouched: the control plane decides ONE position, it
// does not take over ordering generally.
func TestMoveFailoverLast_PreservesTheOrderOfEveryOtherPolicy(t *testing.T) {
	chain := []models.Policy{
		{Name: "api-key-auth"},
		{Name: FailoverPolicyName},
		{Name: "semantic-model-routing"},
		{Name: "pii-masking-regex"},
	}
	got, changed := moveFailoverLast(&models.FailoverConfig{}, chain)
	if !changed {
		t.Fatal("changed = false; failover was in the middle and had to move")
	}
	want := []string{"api-key-auth", "semantic-model-routing", "pii-masking-regex", FailoverPolicyName}
	for i, name := range want {
		if i >= len(got) || got[i].Name != name {
			t.Fatalf("chain = %+v, want %v", got, want)
		}
	}
}

// Already last: nothing moved, so the caller logs nothing. An author who followed the
// documented order should see no message telling them their configuration was rearranged.
func TestMoveFailoverLast_AlreadyLastReportsNoChange(t *testing.T) {
	chain := []models.Policy{
		{Name: "semantic-model-routing"},
		{Name: FailoverPolicyName},
	}
	got, changed := moveFailoverLast(&models.FailoverConfig{}, chain)
	if changed {
		t.Fatal("changed = true; failover was already last")
	}
	if len(got) != 2 || got[1].Name != FailoverPolicyName {
		t.Fatalf("chain = %+v, want it left alone", got)
	}
}

// An inert attachment is not repositioned. The same guard prependFailoverFaultEntry uses, for
// the same reason: with nothing resolved the policy publishes nothing and grades nothing, so
// there is no ordering requirement to enforce and no reason to touch the author's chain.
func TestMoveFailoverLast_InertAttachmentIsLeftWhereItWasWritten(t *testing.T) {
	chain := []models.Policy{
		{Name: FailoverPolicyName},
		{Name: "semantic-model-routing"},
	}
	got, changed := moveFailoverLast(nil, chain)
	if changed {
		t.Fatal("changed = true; an unresolved failover config must not reorder anything")
	}
	if got[0].Name != FailoverPolicyName {
		t.Fatalf("chain = %+v, want the authored order kept", got)
	}
}

// No failover entry at all: every other API on the gateway goes through this code path, so it
// must be a no-op rather than a reshuffle.
func TestMoveFailoverLast_ChainWithoutFailoverIsUntouched(t *testing.T) {
	chain := []models.Policy{{Name: "api-key-auth"}, {Name: "pii-masking-regex"}}
	got, changed := moveFailoverLast(&models.FailoverConfig{}, chain)
	if changed {
		t.Fatal("changed = true; there is no failover entry to move")
	}
	if len(got) != 2 || got[0].Name != "api-key-auth" {
		t.Fatalf("chain = %+v, want it untouched", got)
	}
}
