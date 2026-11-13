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
func TestProxyAttachesFailover_SeesBothAttachmentLevels(t *testing.T) {
	require.False(t, proxyAttachesFailover(api.LLMProxyConfigData{}), "nothing attached")

	require.True(t, proxyAttachesFailover(api.LLMProxyConfigData{
		GlobalPolicies: &[]api.Policy{{Name: config.FailoverPolicyName}},
	}), "API-level attachment must count")

	require.True(t, proxyAttachesFailover(api.LLMProxyConfigData{
		OperationPolicies: &[]api.OperationPolicy{{Name: config.FailoverPolicyName}},
	}), "operation-level attachment must count")

	require.False(t, proxyAttachesFailover(api.LLMProxyConfigData{
		GlobalPolicies: &[]api.Policy{{Name: "model-round-robin"}},
	}), "an unrelated policy must not count")
}
