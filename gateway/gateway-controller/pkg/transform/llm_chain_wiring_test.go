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

	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// llmProviderRows is the slice of storage.Storage an LlmProxy transform actually reads: the
// template behind each provider, and each provider's own stored configuration. Everything else
// is left to the embedded nil interface, so a path that grows a new dependency fails loudly
// here rather than being quietly satisfied by a zero value.
type llmProviderRows struct {
	storage.Storage
	templates map[string]*models.StoredLLMProviderTemplate
	configs   map[string]*models.StoredConfig
}

func (r *llmProviderRows) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	return r.templates[handle], nil
}

func (r *llmProviderRows) GetConfigByKindAndHandle(kind, handle string) (*models.StoredConfig, error) {
	return r.configs[kind+"/"+handle], nil
}

func (r *llmProviderRows) ListCertificatesByUsage(string) ([]*models.StoredCertificate, error) {
	return nil, nil
}

// twoProviderProxyConfig is an LlmProxy fronting two deployed providers, each carrying its own
// credential AND its own authored policy, attaching NO failover.
//
// Canonical `providers` shape so the PRIMARY can carry both: the legacy primary type has no
// `policies` field, and the authored policy is what distinguishes "no chain built" from "a chain
// built from empty inputs" — without it a single-provider proxy produces no chain either way and
// the single-provider assertion below cannot fail.
func twoProviderProxyConfig(t *testing.T) (*models.StoredConfig, *llmProviderRows) {
	t.Helper()

	rows := &llmProviderRows{
		templates: map[string]*models.StoredLLMProviderTemplate{
			"openai": {UUID: "tmpl-1", Configuration: api.LLMProviderTemplate{
				Metadata: api.Metadata{Name: "openai"},
				Spec:     api.LLMProviderTemplateData{DisplayName: "openai"},
			}},
		},
		configs: map[string]*models.StoredConfig{},
	}
	for _, name := range []string{"openai-provider", "anthropic-provider"} {
		rows.configs[string(api.LLMProviderConfigurationKindLlmProvider)+"/"+name] = &models.StoredConfig{
			UUID: name + "-uuid", Kind: string(api.LLMProviderConfigurationKindLlmProvider),
			Handle: name, DisplayName: name, Version: "v1.0", DesiredState: models.StateDeployed,
			SourceConfiguration: api.LLMProviderConfiguration{
				Metadata: api.Metadata{Name: name},
				Spec: api.LLMProviderConfigData{
					DisplayName: name, Version: "v1.0", Context: strPtr("/" + name), Template: "openai",
					Upstream:      api.LLMProviderConfigData_Upstream{Url: strPtr("https://example.com")},
					AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
				},
			},
		}
	}

	key, hdr := "sk-secret", "x-api-key"
	entry := func(id string, primary bool) api.LLMProxyProviderEntry {
		value := key + "-" + id
		return api.LLMProxyProviderEntry{
			Id: id, IsPrimary: primary,
			Auth:     &api.LLMUpstreamAuth{Type: api.LLMUpstreamAuthTypeApiKey, Value: &value, Header: &hdr},
			Policies: &[]api.Policy{{Name: "add-header", Version: "v1"}},
		}
	}
	proxy := api.LLMProxyConfiguration{
		Metadata: api.Metadata{Name: "chat-api"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "chat-api", Version: "v1.0", Context: strPtr("/chat-api"),
			Providers: &[]api.LLMProxyProviderEntry{
				entry("openai-provider", true),
				entry("anthropic-provider", false),
			},
		},
	}
	return &models.StoredConfig{
		UUID: "chat-api-uuid", Kind: models.KindLlmProxy, Handle: "chat-api",
		DisplayName: "chat-api", Version: "v1.0", DesiredState: models.StateDeployed,
		SourceConfiguration: proxy,
	}, rows
}

func llmTransformerForTest(rows *llmProviderRows) *LLMTransformer {
	routerCfg := testRouterCfg()
	routerCfg.ListenerPort = 8080
	systemCfg := &config.Config{Router: *routerCfg}
	return NewLLMTransformer(storage.NewConfigStore(), rows, routerCfg, systemCfg,
		map[string]models.PolicyDefinition{},
		utils.NewStaticPolicyVersionResolver(map[string]string{
			constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME: "v9.9.9",
			constants.ACCESS_CONTROL_DENY_POLICY_NAME:  "v9.9.9",
		}))
}

// FR-012i, END TO END THROUGH THE REAL TRANSFORM. This is the one assertion that catches the
// two halves of the rule disagreeing.
//
// utils.ProxyUsesPerAttemptPlacement takes a two-provider proxy's credential OFF the downstream
// chain. If this call site does not also compile the per-attempt chains, the credential is in
// neither place: the proxy deploys successfully, every route is generated, and every request
// reaches its provider unauthenticated. Nothing in the generated output says so — the absence
// of a policy looks exactly like a policy that was never configured.
//
// A unit test of either half alone passes against that state, which is why this exercises
// LLMTransformer.Transform rather than the predicate or the builder.
func TestLLMTransformer_TwoProvidersWithoutFailoverStillCompileChains(t *testing.T) {
	cfg, rows := twoProviderProxyConfig(t)

	rdc, err := llmTransformerForTest(rows).Transform(cfg)
	require.NoError(t, err)

	require.NotEmpty(t, rdc.UpstreamPolicyChains,
		"two providers and no failover: the credential was taken off the downstream chain, "+
			"so a per-attempt chain must exist to carry it")

	var credentials int
	for cluster, chain := range rdc.UpstreamPolicyChains {
		require.NotEmpty(t, chain.Policies, "chain for %q is present but empty", cluster)
		for _, p := range chain.Policies {
			if p.Name == constants.UPSTREAM_AUTH_APIKEY_POLICY_NAME {
				credentials++
			}
		}
	}
	require.Equal(t, 2, credentials,
		"each provider's credential must appear in exactly one per-attempt chain")
}

// The other side of the same wiring: ONE provider must still compile nothing, or every
// single-provider LlmProxy starts paying an ext_proc round trip per request for a decision that
// could not have been wrong (Probe E3: ~0.6ms warm, 17ms on a cold stream).
func TestLLMTransformer_SingleProviderWithoutFailoverCompilesNoChains(t *testing.T) {
	cfg, rows := twoProviderProxyConfig(t)
	proxy := cfg.SourceConfiguration.(api.LLMProxyConfiguration)
	only := (*proxy.Spec.Providers)[:1]
	proxy.Spec.Providers = &only
	cfg.SourceConfiguration = proxy

	rdc, err := llmTransformerForTest(rows).Transform(cfg)
	require.NoError(t, err)

	// An authored policy is the one piece of per-provider work the chain builder reads straight
	// from the spec rather than from the cleared per-provider map, so building a chain here
	// would not merely be empty and wasteful — it would run that policy a SECOND time, once
	// downstream and once per attempt.
	require.Empty(t, rdc.UpstreamPolicyChains,
		"one provider, no failover: the work stays downstream, so a chain here would duplicate it")
}
