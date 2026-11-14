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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
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
	if existing, ok := cfg.Configuration.(api.RestAPI); ok {
		restAPI = existing
	} else {
		var err error
		switch sc := cfg.SourceConfiguration.(type) {
		case api.LLMProviderConfiguration:
			_, err = t.llmTransformer.Transform(&sc, &restAPI)
		case api.LLMProxyConfiguration:
			_, err = t.llmTransformer.Transform(&sc, &restAPI)
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

// withBudgetEnforcement places the budget system policy on an LLM provider route the way
// llm-cost-based-ratelimit sits: after the route's authentication, so its request-header
// phase sees the caller, and just before the policy that prices the response, so in the
// reverse-order response phase it reads the cost that policy published. A route that does
// not price its responses (llm-cost or azure-llm-cost) gets llm-cost appended after it; a
// gateway build without llm-cost gets the budget policy alone, which then charges $0.
func withBudgetEnforcement(
	kind string,
	chain []policyenginev1.PolicyInstance,
	definitions map[string]models.PolicyDefinition,
	latestVersions map[string]string,
) []policyenginev1.PolicyInstance {
	if kind != models.KindLlmProvider {
		return chain
	}
	budget := policyenginev1.PolicyInstance{
		Name:       constants.BUDGET_SYSTEM_POLICY_NAME,
		Version:    constants.BUDGET_SYSTEM_POLICY_VERSION,
		Enabled:    true,
		Parameters: map[string]interface{}{},
	}
	out := make([]policyenginev1.PolicyInstance, 0, len(chain)+2)
	for i, p := range chain {
		if p.Name == constants.LLM_COST_POLICY_NAME || p.Name == constants.AZURE_LLM_COST_POLICY_NAME {
			out = append(out, chain[:i]...)
			out = append(out, budget)
			return append(out, chain[i:]...)
		}
	}
	out = append(append(out, chain...), budget)
	if _, ok := latestVersions[constants.LLM_COST_POLICY_NAME]; !ok {
		return out
	}
	cost := resolvePolicyInstances(definitions, latestVersions,
		&[]api.Policy{{Name: constants.LLM_COST_POLICY_NAME, Version: constants.LLM_COST_POLICY_VERSION}},
		policyv1alpha.LevelRoute)
	return append(out, cost...)
}
