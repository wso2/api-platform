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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// placementFixture builds a two-provider proxy whose fallback carries a transformer and a
// credential, optionally attaching model-failover at the API level.
func placementFixture(t *testing.T, withFailover bool) (api.RestAPI, map[string]ProviderPolicies) {
	t.Helper()
	store := storage.NewConfigStore()
	db := newTestMockDB()
	require.NoError(t, db.SaveLLMProviderTemplate(&models.StoredLLMProviderTemplate{
		UUID: "tmpl-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{DisplayName: "openai"},
		},
	}))
	save := func(name, ctx string) {
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID: name + "-uuid", Kind: string(api.LLMProviderConfigurationKindLlmProvider),
			Handle: name, DisplayName: name, Version: "v1.0", DesiredState: models.StateDeployed,
			SourceConfiguration: api.LLMProviderConfiguration{
				Metadata: api.Metadata{Name: name},
				Spec: api.LLMProviderConfigData{
					DisplayName: name, Version: "v1.0", Context: stringPtr(ctx), Template: "openai",
					Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
					AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
				},
			},
		}))
	}
	save("openai-provider", "/openai-provider")
	save("anthropic-provider", "/anthropic-provider")

	tr := NewLLMProviderTransformer(store, db,
		&config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	key, hdr := "sk-secret", "x-api-key"
	spec := api.LLMProxyConfigData{
		DisplayName: "p", Version: "v1.0",
		Provider: &api.LLMProxyProvider{Id: "openai-provider"},
		AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{
			Id:          "anthropic-provider",
			Transformer: &api.LLMProxyTransformer{Type: "openai-to-anthropic", Version: "v1"},
			Auth:        &api.LLMUpstreamAuth{Type: api.LLMUpstreamAuthTypeApiKey, Value: &key, Header: &hdr},
			Policies:    &[]api.Policy{{Name: "add-header", Version: "v1"}},
		}},
	}
	if withFailover {
		spec.GlobalPolicies = &[]api.Policy{{Name: config.FailoverPolicyName, Version: "v0"}}
	}

	var out api.RestAPI
	_, perProvider, err := tr.TransformWithUpstreamPolicies(
		&api.LLMProxyConfiguration{Metadata: api.Metadata{Name: "p"}, Spec: spec}, &out)
	require.NoError(t, err)
	return out, perProvider
}

// singleProviderPlacementFixture builds a ONE-provider proxy carrying the same transformer,
// credential and authored policy. One provider is the shape that still places that work
// downstream: there is nothing to choose between, so the execution condition is true for every
// request that reaches the proxy and the placement is exact without a per-attempt chain.
//
// Canonical `providers` shape, not the legacy pair: the legacy primary type carries no
// `policies` field at all, so a single-provider legacy proxy cannot express the authored case.
func singleProviderPlacementFixture(t *testing.T) (api.RestAPI, map[string]ProviderPolicies) {
	t.Helper()
	store := storage.NewConfigStore()
	db := newTestMockDB()
	require.NoError(t, db.SaveLLMProviderTemplate(&models.StoredLLMProviderTemplate{
		UUID: "tmpl-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec:     api.LLMProviderTemplateData{DisplayName: "openai"},
		},
	}))
	require.NoError(t, db.SaveConfig(&models.StoredConfig{
		UUID: "openai-provider-uuid", Kind: string(api.LLMProviderConfigurationKindLlmProvider),
		Handle: "openai-provider", DisplayName: "openai-provider", Version: "v1.0",
		DesiredState: models.StateDeployed,
		SourceConfiguration: api.LLMProviderConfiguration{
			Metadata: api.Metadata{Name: "openai-provider"},
			Spec: api.LLMProviderConfigData{
				DisplayName: "openai-provider", Version: "v1.0",
				Context: stringPtr("/openai-provider"), Template: "openai",
				Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
	}))

	tr := NewLLMProviderTransformer(store, db,
		&config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	key, hdr := "sk-secret", "x-api-key"
	spec := api.LLMProxyConfigData{
		DisplayName: "p", Version: "v1.0",
		Providers: &[]api.LLMProxyProviderEntry{{
			Id:          "openai-provider",
			IsPrimary:   true,
			Transformer: &api.LLMProxyTransformer{Type: "openai-to-anthropic", Version: "v1"},
			Auth:        &api.LLMUpstreamAuth{Type: api.LLMUpstreamAuthTypeApiKey, Value: &key, Header: &hdr},
			Policies:    &[]api.Policy{{Name: "add-header", Version: "v1"}},
		}},
	}

	var out api.RestAPI
	_, perProvider, err := tr.TransformWithUpstreamPolicies(
		&api.LLMProxyConfiguration{Metadata: api.Metadata{Name: "p"}, Spec: spec}, &out)
	require.NoError(t, err)
	return out, perProvider
}

func countDownstream(out api.RestAPI, name string) int {
	n := 0
	if out.Spec.Policies != nil {
		for _, p := range *out.Spec.Policies {
			if p.Name == name {
				n++
			}
		}
	}
	for _, op := range out.Spec.Operations {
		if op.Policies == nil {
			continue
		}
		for _, p := range *op.Policies {
			if p.Name == name {
				n++
			}
		}
	}
	return n
}

// FR-012h. Failover attached: the transformer and credential belong to the per-attempt chain and
// must NOT also remain downstream — both copies running is what applied the transformer twice.
func TestPlacement_FailoverAttachedMovesProviderWorkPerAttempt(t *testing.T) {
	out, perProvider := placementFixture(t, true)

	require.Zero(t, countDownstream(out, "openai-to-anthropic"),
		"with failover attached the downstream copy must be gone; two copies apply it twice")
	pp := perProvider["anthropic-provider"]
	require.NotNil(t, pp.Transformer, "per-attempt chain must carry the transformer")
	require.NotNil(t, pp.Auth, "per-attempt chain must carry the credential")
}

// FR-012i. ONE provider, no failover: nothing to choose between, so the downstream condition is
// true for every request that reaches the proxy and the placement is exact. The work stays
// downstream and NO per-attempt chain is generated for it — otherwise the cluster gets an
// ext_proc filter, and a round trip per request, buying correctness it already had. Probe E3
// measured that filter at ~0.6ms warm and 17ms on a cold stream.
func TestPlacement_SingleProviderKeepsProviderWorkDownstreamOnly(t *testing.T) {
	out, perProvider := singleProviderPlacementFixture(t)

	require.NotZero(t, countDownstream(out, "openai-to-anthropic"),
		"with one provider the transformer must remain on the downstream chain")
	pp := perProvider["openai-provider"]
	require.Nil(t, pp.Transformer, "one provider means no per-attempt transformer")
	require.Nil(t, pp.Auth, "one provider means no per-attempt credential")
}

// FR-012i. TWO providers and no failover: the work moves per attempt anyway. Downstream, scoping
// it to one provider needs a condition on the provider selected so far, and that condition is
// evaluated in the request-header phase — before a policy that selects from the request body has
// run. Failover is not what makes the downstream gate a guess; a second provider is.
func TestPlacement_MultiProviderMovesProviderWorkPerAttemptWithoutFailover(t *testing.T) {
	out, perProvider := placementFixture(t, false)

	require.Zero(t, countDownstream(out, "openai-to-anthropic"),
		"with two providers the downstream copy runs on a prediction and must be gone")
	require.Zero(t, countDownstream(out, "add-header"),
		"an authored per-provider policy must not run on a prediction either")

	pp := perProvider["anthropic-provider"]
	require.NotNil(t, pp.Transformer, "the per-attempt chain must carry the transformer")
	require.NotNil(t, pp.Auth, "the per-attempt chain must carry the credential")
	require.Nil(t, pp.Transformer.ExecutionCondition,
		"per attempt the cluster decides, so no provider gate belongs on the transformer")
	require.Nil(t, pp.Auth.ExecutionCondition,
		"per attempt the cluster decides, so no provider gate belongs on the credential")
}

// The attachment scan must see BOTH levels, matching transform.ResolveFailoverAttachment: where
// the policy is written decides which operations it covers, never whether the per-attempt half
// is installed (FR-030r).
func TestProxyAttachesFailover_SeesEveryAttachmentLevel(t *testing.T) {
	require.False(t, proxyAttachesFailover(api.LLMProxyConfigData{}), "nothing attached")

	require.True(t, proxyAttachesFailover(api.LLMProxyConfigData{
		GlobalPolicies: &[]api.Policy{{Name: config.FailoverPolicyName}},
	}), "API-level attachment must count")

	require.True(t, proxyAttachesFailover(api.LLMProxyConfigData{
		OperationPolicies: &[]api.OperationPolicy{{Name: config.FailoverPolicyName}},
	}), "operation-level attachment must count")

	// DEPRECATED `policies`, which the API documents as "treated identically to
	// operationPolicies" and which collectOperationLevelLLMPolicies merges into the generated
	// operations. Missing it gave a proxy the retry policy with downstream placement, so a
	// fallback attempt reached its provider with no transformer and no credential.
	require.True(t, proxyAttachesFailover(api.LLMProxyConfigData{
		Policies: &[]api.LLMPolicy{{Name: config.FailoverPolicyName}},
	}), "attachment through the deprecated policies list must count")

	require.False(t, proxyAttachesFailover(api.LLMProxyConfigData{
		GlobalPolicies: &[]api.Policy{{Name: "model-round-robin"}},
	}), "an unrelated policy must not count")

	require.False(t, proxyAttachesFailover(api.LLMProxyConfigData{
		Policies: &[]api.LLMPolicy{{Name: "model-round-robin"}},
	}), "an unrelated policy in the deprecated list must not count either")
}

// FR-012i. With ONE provider the publisher's own per-provider policies move downstream too, and
// the proxy generates no per-attempt chain at all — so no cluster carries an ext_proc filter.
//
// "Once per upstream attempt" still holds: there is one attempt, so it is the same statement as
// once per request. "Only on attempts routed to this provider" holds through the execution
// condition, which cannot be wrong when there is only one provider to route to.
func TestPlacement_SingleProviderMovesAuthoredProviderPoliciesDownstreamToo(t *testing.T) {
	out, _ := singleProviderPlacementFixture(t)

	require.NotZero(t, countDownstream(out, "add-header"),
		"an authored per-provider policy must run downstream on a single-provider proxy")

	// Gated, not global: it must not run for requests routed to a different provider.
	var found *api.Policy
	for i := range out.Spec.Operations {
		if out.Spec.Operations[i].Policies == nil {
			continue
		}
		for j, p := range *out.Spec.Operations[i].Policies {
			if p.Name == "add-header" {
				found = &(*out.Spec.Operations[i].Policies)[j]
			}
		}
	}
	require.NotNil(t, found, "authored policy not found on any operation")
	require.NotNil(t, found.ExecutionCondition, "it must carry a provider gate")
	require.Contains(t, *found.ExecutionCondition, "selected_provider",
		"the gate must scope it to the provider it was authored under")
}

// FR-039 in the downstream placement: the credential is appended LAST, so an authored policy
// cannot be positioned after it and read the key out of the request headers.
func TestPlacement_SingleProviderKeepsAuthoredPoliciesBeforeTheCredential(t *testing.T) {
	out, _ := singleProviderPlacementFixture(t)

	indexOf := func(ops []api.Operation, name string) int {
		for _, op := range ops {
			if op.Policies == nil {
				continue
			}
			for i, p := range *op.Policies {
				if p.Name == name {
					return i
				}
			}
		}
		return -1
	}

	authored := indexOf(out.Spec.Operations, "add-header")
	require.NotEqual(t, -1, authored, "authored policy must be present")

	// The credential policy's name comes from the api-key auth builder; locate whatever was
	// appended last for the provider and assert the authored one precedes it.
	transformer := indexOf(out.Spec.Operations, "openai-to-anthropic")
	require.NotEqual(t, -1, transformer, "transformer must be present downstream")
	require.Less(t, authored, transformer,
		"authored per-provider policies must run BEFORE the transformer and the credential")
}

// With failover attached the authored policy belongs to the per-attempt chain instead, and must
// NOT also be left downstream — two copies would run it twice for the serving attempt.
func TestPlacement_FailoverKeepsAuthoredProviderPoliciesOffTheDownstreamChain(t *testing.T) {
	out, _ := placementFixture(t, true)

	require.Zero(t, countDownstream(out, "add-header"),
		"with failover attached the authored policy runs per attempt, not downstream")
}

// A publisher's own executionCondition must survive the provider gate.
//
// The gate narrows WHICH requests a provider policy applies to. It was never meant to decide
// whether the publisher wanted the policy at all -- but replacing the authored condition did
// exactly that: an explicit "false" became the gate, which is true on a default-primary request,
// so a policy the publisher had disabled ran.
func TestProviderPoliciesDownstream_KeepsTheAuthoredExecutionCondition(t *testing.T) {
	disabled := "false"
	tr := &LLMProviderTransformer{}
	spec := api.LLMProxyConfigData{
		Provider: &api.LLMProxyProvider{Id: "primary-provider"},
		AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{
			Id: "anthropic-provider",
			As: strPtrPlacement("anthropic"),
			Policies: &[]api.Policy{
				{Name: "set-headers", ExecutionCondition: &disabled},
				{Name: "pii-masking-regex"},
			},
		}},
	}

	out, err := tr.providerPoliciesDownstream(spec)
	require.NoError(t, err)
	require.Len(t, out, 2)

	var withAuthored, withoutAuthored *api.Policy
	for i := range out {
		switch out[i].Name {
		case "set-headers":
			withAuthored = &out[i]
		case "pii-masking-regex":
			withoutAuthored = &out[i]
		}
	}
	require.NotNil(t, withAuthored)
	require.NotNil(t, withoutAuthored)

	require.Contains(t, *withAuthored.ExecutionCondition, "false",
		"the authored condition was dropped, so a disabled policy would run")
	require.Contains(t, *withAuthored.ExecutionCondition, "selected_provider",
		"the provider gate must still narrow which requests it applies to")
	require.NotContains(t, *withoutAuthored.ExecutionCondition, "&& ()",
		"a policy with no authored condition gets the bare gate")
}

func strPtrPlacement(s string) *string { return &s }

// FR-012h and FR-012i are two independent reasons for the same placement, and either alone is
// enough. Read as one predicate because pkg/transform reads it too, to decide whether to build
// the chains the placement assumes exist.
func TestProxyUsesPerAttemptPlacement_EitherReasonAlone(t *testing.T) {
	failover := &[]api.Policy{{Name: config.FailoverPolicyName, Version: "v0"}}

	cases := []struct {
		name string
		spec api.LLMProxyConfigData
		want bool
		why  string
	}{
		{
			name: "one provider, no failover",
			spec: api.LLMProxyConfigData{Provider: &api.LLMProxyProvider{Id: "openai"}},
			want: false,
			why:  "nothing to choose between, so the downstream condition cannot be wrong",
		},
		{
			name: "one provider, canonical shape",
			spec: api.LLMProxyConfigData{Providers: &[]api.LLMProxyProviderEntry{
				{Id: "openai", IsPrimary: true}}},
			want: false,
			why:  "the shape must not change the answer",
		},
		{
			name: "one provider plus failover",
			spec: api.LLMProxyConfigData{
				Provider: &api.LLMProxyProvider{Id: "openai"}, GlobalPolicies: failover},
			want: true,
			why:  "attempts can reach different targets even with one provider configured",
		},
		{
			name: "two providers, no failover",
			spec: api.LLMProxyConfigData{
				Provider: &api.LLMProxyProvider{Id: "openai"},
				AdditionalProviders: &[]api.LLMProxyAdditionalProvider{
					{Id: "anthropic"}}},
			want: true,
			why:  "the downstream gate would have to predict the provider before the body is read",
		},
		{
			name: "two providers, canonical shape",
			spec: api.LLMProxyConfigData{Providers: &[]api.LLMProxyProviderEntry{
				{Id: "openai", IsPrimary: true}, {Id: "anthropic"}}},
			want: true,
			why:  "the shape must not change the answer",
		},
		{
			name: "two providers plus failover",
			spec: api.LLMProxyConfigData{
				Provider: &api.LLMProxyProvider{Id: "openai"},
				AdditionalProviders: &[]api.LLMProxyAdditionalProvider{
					{Id: "anthropic"}},
				GlobalPolicies: failover},
			want: true,
			why:  "both reasons at once must not cancel out",
		},
		{
			name: "a shape that does not normalise",
			spec: api.LLMProxyConfigData{},
			want: false,
			why: "validation rejects this before translation; downstream is where the work ran " +
				"before this rule existed, so an unreadable shape degrades to that rather than " +
				"to no provider work at all",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ProxyUsesPerAttemptPlacement(tc.spec), tc.why)
		})
	}
}
