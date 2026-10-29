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

package transform

import (
	"fmt"
	"log/slog"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// LLMTransformer transforms LLM Provider or LLM Proxy StoredConfig into RuntimeDeployConfig.
// It first uses the existing LLMProviderTransformer to produce a RestAPI, then runs
// RestAPITransformer on the result, and finally enriches the metadata with LLM-specific fields.
type LLMTransformer struct {
	llmTransformer  *utils.LLMProviderTransformer
	restTransformer *RestAPITransformer
	store           *storage.ConfigStore
}

// NewLLMTransformer creates a new LLMTransformer.
func NewLLMTransformer(
	store *storage.ConfigStore,
	db storage.Storage,
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
	policyVersionResolver utils.PolicyVersionResolver,
) *LLMTransformer {
	return &LLMTransformer{
		llmTransformer:  utils.NewLLMProviderTransformer(store, db, routerConfig, policyVersionResolver),
		restTransformer: NewRestAPITransformer(routerConfig, systemConfig, policyDefinitions),
		store:           store,
	}
}

// Transform converts a StoredConfig (LLM Provider or LLM Proxy) into RuntimeDeployConfig.
func (t *LLMTransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	// Step 1: Obtain the RestAPI representation.
	// If cfg.Configuration is already a RestAPI (e.g. after hydration + policy resolution),
	// use it directly so that resolved policy state is preserved.
	// Otherwise, re-derive from SourceConfiguration.
	var restAPI api.RestAPI
	// Per-provider policies for the PER-ATTEMPT chain. Empty unless per-attempt placement is
	// enabled, in which case the provider's transformer and credential are NOT on the
	// downstream chain and must be supplied here instead.
	var upstreamProviderPolicies map[string]utils.ProviderPolicies
	if existing, ok := cfg.Configuration.(api.RestAPI); ok {
		restAPI = existing
		// The RestAPI arrived already hydrated (HydrateLLMConfig runs before this, at
		// startup and on deploy), so the branch below never ran and the per-provider
		// policies were never computed. Derive them from the source configuration, which
		// is still present, into a throwaway RestAPI.
		//
		// Without this the per-attempt chains are silently empty for every LlmProxy: no
		// cluster carries the upstream filter, and a cross-vendor fallback is neither
		// transformed nor credentialed. The symptom is indistinguishable from the feature
		// being switched off.
		if proxy, isProxy := cfg.SourceConfiguration.(api.LLMProxyConfiguration); isProxy {
			var discard api.RestAPI
			if _, policies, derr := t.llmTransformer.TransformWithUpstreamPolicies(&proxy, &discard); derr == nil {
				upstreamProviderPolicies = policies
			} else {
				slog.Error("failed to derive per-attempt provider policies",
					slog.String("proxy", cfg.Handle), slog.Any("error", derr))
			}
		}
	} else {
		var err error
		switch sc := cfg.SourceConfiguration.(type) {
		case api.LLMProviderConfiguration:
			_, err = t.llmTransformer.Transform(&sc, &restAPI)
		case api.LLMProxyConfiguration:
			_, upstreamProviderPolicies, err = t.llmTransformer.TransformWithUpstreamPolicies(&sc, &restAPI)
		default:
			return nil, fmt.Errorf("unsupported LLM source configuration type: %T", cfg.SourceConfiguration)
		}
		if err != nil {
			return nil, fmt.Errorf("LLM transformation failed: %w", err)
		}
	}

	// Step 2: Build a temporary StoredConfig with the RestAPI result
	tempCfg := &models.StoredConfig{
		UUID:                cfg.UUID,
		Kind:                cfg.Kind,
		Handle:              cfg.Handle,
		DisplayName:         cfg.DisplayName,
		Version:             cfg.Version,
		Configuration:       restAPI,
		SourceConfiguration: cfg.SourceConfiguration,
		DesiredState:        cfg.DesiredState,
		CreatedAt:           cfg.CreatedAt,
		UpdatedAt:           cfg.UpdatedAt,
	}

	// Step 3: Use RestAPITransformer to build RuntimeDeployConfig
	rdc, err := t.restTransformer.Transform(tempCfg)
	if err != nil {
		return nil, fmt.Errorf("RestAPI transformation for LLM failed: %w", err)
	}

	// Step 4: Enrich metadata with LLM-specific fields
	rdc.Metadata.Kind = cfg.Kind // Restore original kind (LlmProvider/LlmProxy)
	llmMeta := t.extractLLMMetadata(cfg)
	if llmMeta != nil {
		rdc.Metadata.LLM = llmMeta
	}
	rdc.SensitiveValues = cfg.SensitiveValues

	// Step 5: Compile the per-attempt chains, keyed by the provider alias an attempt is
	// routed to.
	//
	// Carried on the RDC but NOT yet consumed: the transformer and credential still run
	// downstream, gated on the selected provider, exactly as before. Moving them is a
	// behaviour change that only becomes safe once the runtime dispatches these chains
	// (T024d/T024e), and doing both at once would leave no working state in between.
	if proxy, ok := cfg.SourceConfiguration.(api.LLMProxyConfiguration); ok {
		// Point the empty alias at the PRIMARY PROVIDER'S OWN cluster, not the API's main
		// upstream slot.
		//
		// Both exist. The main slot's key is derived from host and port alone
		// (`upstream_main_127.0.0.1_8080`), carrying no API identity — so every LlmProxy whose
		// primary is a loopback SHARES it. The primary also gets its own loopback definition,
		// keyed `upstream_<Kind>_<uuid>_<name>`, which is API-scoped.
		//
		// Keying per-attempt state on the shared one collides across APIs: two proxies would
		// publish different chains under the same `upstream:<cluster>` key and the last write
		// would win, handing one API's per-attempt policies — including its credential — to
		// another's request. The API-scoped cluster cannot collide.
		//
		// Resolved through NORMALISATION, not spec.Provider. The canonical `providers` shape
		// leaves spec.Provider nil, so dereferencing it panicked — recovered by the event
		// listener, which meant the proxy reported "deployed successfully" while producing no
		// routes whatsoever. Matching on EffectiveName rather than Id also fixes the lookup
		// for a legacy primary that sets `as`: the primary's own loopback definition is named
		// by EffectiveName, so an aliased primary missed here and left the empty alias on the
		// SHARED main slot — the cross-API collision this block exists to prevent.
		if primaryAttachment, err := models.PrimaryLLMProxyAttachment(proxy.Spec); err == nil {
			if primary := primaryAttachment.EffectiveName(); primary != "" && rdc.ProviderClusters != nil {
				if cluster, ok := rdc.ProviderClusters[primary].(string); ok && cluster != "" {
					rdc.ProviderClusters[""] = cluster
				}
			}
		}

		rdc.UpstreamPolicyChains = buildUpstreamPolicyChains(&proxy.Spec, rdc.ProviderClusters,
			transformerFor(upstreamProviderPolicies),
			authFor(upstreamProviderPolicies),
			perAttemptPolicies(&restAPI.Spec.Operations))
	}

	return rdc, nil
}

// extractLLMMetadata extracts LLM-specific metadata from the source configuration.
func (t *LLMTransformer) extractLLMMetadata(cfg *models.StoredConfig) *models.LLMMetadata {
	meta := &models.LLMMetadata{}

	switch sc := cfg.SourceConfiguration.(type) {
	case api.LLMProviderConfiguration:
		meta.TemplateHandle = sc.Spec.Template
		meta.ProviderName = sc.Metadata.Name

	case api.LLMProxyConfiguration:
		// Get provider name and template handle from referenced provider
		primary, err := models.PrimaryLLMProxyAttachment(sc.Spec)
		if err != nil {
			return meta
		}
		providerCfg, err := t.store.GetByKindAndHandle(string(api.LLMProviderConfigurationKindLlmProvider), primary.Id)
		if err != nil || providerCfg == nil {
			return meta
		}
		if provSrc, ok := providerCfg.SourceConfiguration.(api.LLMProviderConfiguration); ok {
			meta.TemplateHandle = provSrc.Spec.Template
			meta.ProviderName = provSrc.Metadata.Name
		}
	}

	if meta.TemplateHandle == "" && meta.ProviderName == "" {
		return nil
	}
	return meta
}
