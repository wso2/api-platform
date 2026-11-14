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

// FR-012h. No failover: exactly one attempt, so per-attempt and per-request are the same thing.
// The work stays downstream and NO per-attempt chain is generated for it — otherwise the cluster
// gets an ext_proc filter, and a round trip per request, for nothing.
func TestPlacement_NoFailoverKeepsProviderWorkDownstreamOnly(t *testing.T) {
	out, perProvider := placementFixture(t, false)

	require.NotZero(t, countDownstream(out, "openai-to-anthropic"),
		"without failover the transformer must remain on the downstream chain")
	pp := perProvider["anthropic-provider"]
	require.Nil(t, pp.Transformer, "no failover means no per-attempt transformer")
	require.Nil(t, pp.Auth, "no failover means no per-attempt credential")
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

// FR-012h. With no failover the publisher's OWN per-provider policies move downstream too, and
// the proxy generates no per-attempt chain at all — so no cluster carries an ext_proc filter.
//
// "Once per upstream attempt" still holds: there is one attempt, so it is the same statement as
// once per request. "Only on attempts routed to this provider" holds through the execution
// condition, the same gate the transformer and credential already use downstream.
func TestPlacement_NoFailoverMovesAuthoredProviderPoliciesDownstreamToo(t *testing.T) {
	out, _ := placementFixture(t, false)

	require.NotZero(t, countDownstream(out, "add-header"),
		"an authored per-provider policy must run downstream when there is no failover")

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
func TestPlacement_NoFailoverKeepsAuthoredPoliciesBeforeTheCredential(t *testing.T) {
	out, _ := placementFixture(t, false)

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
