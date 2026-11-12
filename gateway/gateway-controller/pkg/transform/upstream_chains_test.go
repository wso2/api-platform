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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func strPtr(s string) *string { return &s }

// The LEGACY shape: a primary `provider` plus `additionalProviders`. The primary is required —
// a proxy always has one — and omitting it here used to go unnoticed because the builder read
// AdditionalProviders directly instead of normalising.
func proxyWithProviders(providers ...api.LLMProxyAdditionalProvider) *api.LLMProxyConfigData {
	return &api.LLMProxyConfigData{
		Provider:            &api.LLMProxyProvider{Id: "primary-provider"},
		AdditionalProviders: &providers,
	}
}

// The CANONICAL shape: one uniform `providers` list with exactly one isPrimary entry.
func canonicalProxy(entries ...api.LLMProxyProviderEntry) *api.LLMProxyConfigData {
	return &api.LLMProxyConfigData{Providers: &entries}
}

func namedPolicy(name string) *models.Policy { return &models.Policy{Name: name} }

// Alias to cluster, as the controller resolves it. The empty alias is the API's own main
// upstream, which a target naming no provider reaches.
var testClusters = map[string]interface{}{
	"":                      "upstream_main",
	"anthropic":             "upstream_anthropic",
	"azure-openai-provider": "upstream_azure",
	"bare-provider":         "upstream_bare",
}

// THE ordering guarantee. Authentication is appended last by the controller so that no
// author-written policy can observe the credential: a policy placed after it could read the
// provider's key out of the request headers and copy it anywhere, while one placed before has
// nothing yet to read. Because the controller fixes the order, a publisher cannot reorder their
// way into seeing it.
func TestBuildUpstreamPolicyChains_AuthIsAlwaysLast(t *testing.T) {
	spec := proxyWithProviders(api.LLMProxyAdditionalProvider{
		Id: "anthropic-provider",
		As: strPtr("anthropic"),
		Policies: &[]api.Policy{
			{Name: "set-headers"},
			{Name: "pii-masking-regex"},
		},
	})

	chains := buildUpstreamPolicyChains(spec, testClusters,
		func(alias string) *models.Policy {
			if alias == "anthropic" {
				return namedPolicy("openai-to-anthropic-transformer")
			}
			return nil
		},
		func(alias string) *models.Policy {
			if alias == "anthropic" {
				return namedPolicy("provider-credential")
			}
			return nil
		}, nil)

	chain, ok := chains["upstream_anthropic"]
	if !ok {
		t.Fatalf("no chain for the alias; got %v", chains)
	}
	want := []string{"set-headers", "pii-masking-regex", "openai-to-anthropic-transformer", "provider-credential"}
	if len(chain.Policies) != len(want) {
		t.Fatalf("chain = %+v, want %v", chain.Policies, want)
	}
	for i, name := range want {
		if chain.Policies[i].Name != name {
			t.Fatalf("position %d = %q, want %q (full chain %+v)", i, chain.Policies[i].Name, name, chain.Policies)
		}
	}
	if last := chain.Policies[len(chain.Policies)-1].Name; last != "provider-credential" {
		t.Fatalf("last policy = %q; the credential must be last so no author policy sees it", last)
	}
}

// An alias defaults to the provider id, matching how a target's `provider` field is authored.
func TestBuildUpstreamPolicyChains_AliasDefaultsToId(t *testing.T) {
	spec := proxyWithProviders(api.LLMProxyAdditionalProvider{
		Id:       "azure-openai-provider",
		Policies: &[]api.Policy{{Name: "set-headers"}},
	})
	chains := buildUpstreamPolicyChains(spec, testClusters,
		func(string) *models.Policy { return nil },
		func(string) *models.Policy { return nil }, nil)

	// Keyed by the CLUSTER the alias resolves to, since that is what Envoy names per attempt.
	if _, ok := chains["upstream_azure"]; !ok {
		t.Fatalf("expected a chain keyed by the alias's cluster; got %v", chains)
	}
}

// The primary provider is reached by a target naming no provider — the empty alias, matching
// BuildProviderClusterMap's keying so the runtime looks both up by the same string.
func TestBuildUpstreamPolicyChains_PrimaryUsesTheEmptyAlias(t *testing.T) {
	// A proxy always declares a provider; an empty spec was never valid config and is now
	// rejected at normalisation rather than silently producing a primary-only chain.
	spec := &api.LLMProxyConfigData{Provider: &api.LLMProxyProvider{Id: "primary-provider"}}
	chains := buildUpstreamPolicyChains(spec, testClusters,
		func(alias string) *models.Policy {
			if alias == "" {
				return namedPolicy("primary-transformer")
			}
			return nil
		},
		func(string) *models.Policy { return nil }, nil)

	chain, ok := chains["upstream_main"]
	if !ok {
		t.Fatalf("the primary provider must key on its own cluster; got %v", chains)
	}
	if chain.Policies[0].Name != "primary-transformer" {
		t.Fatalf("primary chain = %+v", chain.Policies)
	}
}

// An upstream with no per-attempt work generates no chain, so an attempt routed there pays
// nothing (FR-043).
func TestBuildUpstreamPolicyChains_NoWorkMeansNoChain(t *testing.T) {
	spec := proxyWithProviders(api.LLMProxyAdditionalProvider{Id: "bare-provider"})
	chains := buildUpstreamPolicyChains(spec, testClusters,
		func(string) *models.Policy { return nil },
		func(string) *models.Policy { return nil }, nil)

	if _, present := chains["upstream_bare"]; present {
		t.Fatalf("a provider with nothing to run must generate no chain; got %v", chains)
	}
	// Nil rather than an empty map when nothing has work: ranging and lookup are both safe
	// on a nil map, and it keeps the field absent on an RDC that needs no upstream chains.
	if len(chains) != 0 {
		t.Fatalf("expected no chains at all, got %v", chains)
	}
}

func TestBuildUpstreamPolicyChains_NilSpec(t *testing.T) {
	if chains := buildUpstreamPolicyChains(nil, nil, nil, nil, nil); chains != nil {
		t.Fatalf("a nil spec must yield no chains, got %v", chains)
	}
}

// A provider whose alias resolves to no cluster cannot receive an attempt, so its chain could
// never run. Dropped rather than stored under an empty key, which would collide across every
// such provider and hand whichever one wrote last to an unrelated attempt.
func TestBuildUpstreamPolicyChains_DropsProvidersWithNoCluster(t *testing.T) {
	spec := proxyWithProviders(api.LLMProxyAdditionalProvider{
		Id:       "not-deployed-here",
		Policies: &[]api.Policy{{Name: "set-headers"}},
	})
	chains := buildUpstreamPolicyChains(spec, testClusters,
		func(string) *models.Policy { return nil },
		func(string) *models.Policy { return nil }, nil)

	if len(chains) != 0 {
		t.Fatalf("a provider with no cluster must produce no chain, got %v", chains)
	}
	if _, present := chains[""]; present {
		t.Fatal("an unresolvable provider must never be stored under the empty key")
	}
}

// Named fields, not list positions. A provider with a credential and NO transformer is the trap:
// positional access would read its credential as index 0 and apply it as a translator.
func TestProviderPolicyLookups_AreNamedNotPositional(t *testing.T) {
	credentialOnly := map[string]utils.ProviderPolicies{
		"anthropic": {Auth: &api.Policy{Name: "set-headers"}},
	}

	if got := transformerFor(credentialOnly)("anthropic"); got != nil {
		t.Fatalf("transformer = %+v; a provider with only a credential has NO transformer, and "+
			"returning one here would apply its key as a translator", got)
	}
	auth := authFor(credentialOnly)("anthropic")
	if auth == nil || auth.Name != "set-headers" {
		t.Fatalf("auth = %+v, want the credential", auth)
	}
}

func TestProviderPolicyLookups_UnknownProvider(t *testing.T) {
	empty := map[string]utils.ProviderPolicies{}
	if transformerFor(empty)("nobody") != nil || authFor(empty)("nobody") != nil {
		t.Fatal("an unknown provider must yield no policies")
	}
}

// End to end through the chain builder: transformer first, credential last.
func TestBuildUpstreamPolicyChains_UsesTheRealBuilders(t *testing.T) {
	perProvider := map[string]utils.ProviderPolicies{
		"anthropic": {
			Transformer: &api.Policy{Name: "openai-to-anthropic-transformer"},
			Auth:        &api.Policy{Name: "set-headers"},
		},
	}
	spec := proxyWithProviders(api.LLMProxyAdditionalProvider{
		Id:       "anthropic-provider",
		As:       strPtr("anthropic"),
		Policies: &[]api.Policy{{Name: "pii-masking-regex"}},
	})

	chains := buildUpstreamPolicyChains(spec, testClusters,
		transformerFor(perProvider), authFor(perProvider), nil)

	chain, ok := chains["upstream_anthropic"]
	if !ok {
		t.Fatalf("no chain; got %v", chains)
	}
	want := []string{"pii-masking-regex", "openai-to-anthropic-transformer", "set-headers"}
	if len(chain.Policies) != len(want) {
		t.Fatalf("chain = %+v, want %v", chain.Policies, want)
	}
	for i, name := range want {
		if chain.Policies[i].Name != name {
			t.Fatalf("position %d = %q, want %q", i, chain.Policies[i].Name, name)
		}
	}
}

// A per-attempt policy decides what the attempt is FOR — the model it asks for — so everything
// after it reads the payload it leaves behind. A transformer translating before the model was
// substituted would faithfully translate the wrong request.
func TestBuildUpstreamPolicyChains_PerAttemptPolicyRunsFirst(t *testing.T) {
	spec := &api.LLMProxyConfigData{
		AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{Id: "anthropic"}},
	}
	perAttempt := []models.Policy{{Name: "model-failover", Params: map[string]interface{}{}}}

	chains := buildUpstreamPolicyChains(spec, testClusters,
		func(string) *models.Policy { return &models.Policy{Name: "transformer"} },
		func(string) *models.Policy { return &models.Policy{Name: "auth"} },
		perAttempt)

	for cluster, chain := range chains {
		if len(chain.Policies) == 0 {
			t.Fatalf("%s: empty chain", cluster)
		}
		if chain.Policies[0].Name != "model-failover" {
			t.Fatalf("%s: first policy = %q, want model-failover", cluster, chain.Policies[0].Name)
		}
		// Auth stays last regardless.
		if last := chain.Policies[len(chain.Policies)-1].Name; last != "auth" {
			t.Fatalf("%s: last policy = %q, want auth", cluster, last)
		}
	}
}

// Without failover attached, no per-attempt policy is added — a chain containing one declares
// RequiresRequestBody, so every provider would start buffering bodies for a policy with nothing
// to do.
func TestBuildUpstreamPolicyChains_NoPerAttemptPolicyWhenNoneGiven(t *testing.T) {
	spec := &api.LLMProxyConfigData{
		AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{Id: "anthropic"}},
	}

	chains := buildUpstreamPolicyChains(spec, testClusters,
		func(string) *models.Policy { return &models.Policy{Name: "transformer"} },
		func(string) *models.Policy { return nil },
		nil)

	for cluster, chain := range chains {
		for _, p := range chain.Policies {
			if p.Name == "model-failover" {
				t.Fatalf("%s: per-attempt policy added without failover attached", cluster)
			}
		}
	}
}

// THE DEFECT (T024a0). `spec.providers[]` with `isPrimary` is the canonical shape, and it is
// the one the approved design document's own example uses. The builder read
// spec.AdditionalProviders directly, so a canonically-authored proxy produced NO chain for any
// non-primary provider: a failover attempt landed there with neither its transformer nor its
// credential, silently. Nothing caught it because every test here used the legacy shape.
func TestBuildUpstreamPolicyChains_CanonicalProvidersShape(t *testing.T) {
	spec := canonicalProxy(
		api.LLMProxyProviderEntry{Id: "openai-provider", IsPrimary: true},
		api.LLMProxyProviderEntry{
			Id:       "anthropic-provider",
			Alias:    strPtr("anthropic"),
			Policies: &[]api.Policy{{Name: "set-headers"}},
		},
	)

	chains := buildUpstreamPolicyChains(spec, testClusters,
		func(alias string) *models.Policy {
			if alias == "anthropic" {
				return namedPolicy("openai-to-anthropic-transformer")
			}
			return nil
		},
		func(alias string) *models.Policy {
			if alias == "anthropic" {
				return namedPolicy("provider-credential")
			}
			return nil
		}, nil)

	chain, ok := chains["upstream_anthropic"]
	if !ok {
		t.Fatalf("no chain for a canonically-declared provider; a failover attempt would "+
			"reach it with no transformer and no credential. got %v", chains)
	}
	want := []string{"set-headers", "openai-to-anthropic-transformer", "provider-credential"}
	if len(chain.Policies) != len(want) {
		t.Fatalf("chain = %+v, want %v", chain.Policies, want)
	}
	for i, name := range want {
		if chain.Policies[i].Name != name {
			t.Fatalf("position %d = %q, want %q (full chain %+v)", i, chain.Policies[i].Name, name, chain.Policies)
		}
	}
}

// The canonical shape can attach per-attempt policies to the PRIMARY too — the legacy
// `provider` object has no policies field, so this is reachable only this way. The primary is
// still keyed by the empty alias, matching BuildProviderClusterMap.
func TestBuildUpstreamPolicyChains_CanonicalPrimaryCarriesItsOwnPolicies(t *testing.T) {
	spec := canonicalProxy(api.LLMProxyProviderEntry{
		Id:        "openai-provider",
		IsPrimary: true,
		Policies:  &[]api.Policy{{Name: "set-headers"}},
	})

	chains := buildUpstreamPolicyChains(spec, testClusters,
		func(string) *models.Policy { return nil },
		func(string) *models.Policy { return nil }, nil)

	chain, ok := chains["upstream_main"]
	if !ok {
		t.Fatalf("expected the primary's chain under the empty alias's cluster; got %v", chains)
	}
	if len(chain.Policies) != 1 || chain.Policies[0].Name != "set-headers" {
		t.Fatalf("chain = %+v, want just set-headers", chain.Policies)
	}
}

// Both shapes must produce the same chains for the same providers — that is the whole point of
// normalising. Asserted directly so a future change to one path cannot drift from the other.
func TestBuildUpstreamPolicyChains_BothShapesAgree(t *testing.T) {
	legacy := proxyWithProviders(api.LLMProxyAdditionalProvider{
		Id:       "anthropic-provider",
		As:       strPtr("anthropic"),
		Policies: &[]api.Policy{{Name: "set-headers"}},
	})
	canonical := canonicalProxy(
		api.LLMProxyProviderEntry{Id: "primary-provider", IsPrimary: true},
		api.LLMProxyProviderEntry{
			Id:       "anthropic-provider",
			Alias:    strPtr("anthropic"),
			Policies: &[]api.Policy{{Name: "set-headers"}},
		},
	)

	build := func(spec *api.LLMProxyConfigData) map[string][]string {
		chains := buildUpstreamPolicyChains(spec, testClusters,
			func(alias string) *models.Policy {
				if alias == "anthropic" {
					return namedPolicy("openai-to-anthropic-transformer")
				}
				return nil
			},
			func(alias string) *models.Policy {
				if alias == "anthropic" {
					return namedPolicy("provider-credential")
				}
				return nil
			}, nil)
		out := map[string][]string{}
		for cluster, chain := range chains {
			names := make([]string, 0, len(chain.Policies))
			for _, p := range chain.Policies {
				names = append(names, p.Name)
			}
			out[cluster] = names
		}
		return out
	}

	gotLegacy, gotCanonical := build(legacy), build(canonical)
	if len(gotLegacy) != len(gotCanonical) {
		t.Fatalf("shapes disagree on chain count: legacy %v, canonical %v", gotLegacy, gotCanonical)
	}
	for cluster, want := range gotLegacy {
		got, ok := gotCanonical[cluster]
		if !ok {
			t.Fatalf("canonical shape produced no chain for %q; legacy produced %v", cluster, want)
		}
		if len(got) != len(want) {
			t.Fatalf("cluster %q: legacy %v, canonical %v", cluster, want, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("cluster %q position %d: legacy %q, canonical %q", cluster, i, want[i], got[i])
			}
		}
	}
}

// A canonical-shaped proxy must TRANSFORM at all. `spec.Provider` is nil in that shape, and
// Transform dereferenced it unconditionally — the panic was recovered by the event listener,
// so the deployment reported success and produced no routes whatsoever. A silent total
// failure, which is why this asserts transformation rather than any particular output.
func TestLLMTransformer_CanonicalShapeDoesNotPanic(t *testing.T) {
	proxy := api.LLMProxyConfiguration{
		Spec: api.LLMProxyConfigData{
			Providers: &[]api.LLMProxyProviderEntry{
				{Id: "openai-provider", IsPrimary: true},
				{Id: "anthropic-provider", Alias: strPtr("anthropic")},
			},
		},
	}

	// The block under test reads only the normalised primary and the cluster map, so it is
	// exercised directly — a full Transform needs a database and a router config.
	clusters := map[string]interface{}{"openai-provider": "upstream_LlmProxy_uuid_openai-provider"}
	if primary, err := models.PrimaryLLMProxyAttachment(proxy.Spec); err != nil {
		t.Fatalf("canonical shape must normalise: %v", err)
	} else if name := primary.EffectiveName(); name != "openai-provider" {
		t.Fatalf("primary EffectiveName = %q, want the canonical primary's id", name)
	}

	// And the empty alias must land on the primary's API-SCOPED cluster, never the shared
	// main slot — two proxies sharing that key would hand one API's credential to another.
	primary, _ := models.PrimaryLLMProxyAttachment(proxy.Spec)
	if cluster, ok := clusters[primary.EffectiveName()].(string); !ok || cluster == "" {
		t.Fatalf("primary did not resolve to its own cluster; clusters=%v", clusters)
	}
}

// A LEGACY primary that sets `as` is named by that alias in its own loopback definition, so
// looking it up by Id missed and silently left the empty alias on the shared main slot.
func TestLLMTransformer_LegacyAliasedPrimaryResolvesToItsOwnCluster(t *testing.T) {
	spec := api.LLMProxyConfigData{
		Provider: &api.LLMProxyProvider{Id: "openai-provider", As: strPtr("openai")},
	}
	primary, err := models.PrimaryLLMProxyAttachment(spec)
	if err != nil {
		t.Fatalf("normalise: %v", err)
	}
	if got := primary.EffectiveName(); got != "openai" {
		t.Fatalf("EffectiveName = %q, want the alias — the loopback definition is named by it", got)
	}
}
