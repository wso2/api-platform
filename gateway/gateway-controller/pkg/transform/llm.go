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
	"crypto/rand"
	"encoding/hex"
	"fmt"

	commonconstants "github.com/wso2/api-platform/common/constants"
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

// budgetAdmittedToken is this controller's secret: the budget policy on an LLM proxy route
// stamps the requests it evaluated with it, and the policy on the provider route they are
// forwarded to skips them, so each request is counted once. A forged stamp does not match.
var budgetAdmittedToken = newBudgetAdmittedToken()

func newBudgetAdmittedToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("budget: generating the admitted token: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// budgetPlacement returns the budget policy's parameters for a route, or false when it does
// not run there. It runs at the first hop a request enters: every LLM provider route, and
// every LLM proxy route with one provider, which knows the caller (an agent's hidden proxy
// carries its project-id and agent-id annotations). A proxy with several providers picks one
// per request, after admission, so its calls are counted on the provider route instead.
func budgetPlacement(cfg *models.StoredConfig) (map[string]interface{}, bool) {
	if cfg == nil {
		return nil, false
	}
	params := map[string]interface{}{"admittedToken": budgetAdmittedToken}
	switch cfg.Kind {
	case models.KindLlmProvider:
		params["role"] = "provider"
		return params, true
	case models.KindLlmProxy:
		proxy, ok := cfg.SourceConfiguration.(api.LLMProxyConfiguration)
		if !ok {
			return nil, false
		}
		attachments, err := models.NormaliseLLMProxyAttachments(proxy.Spec)
		if err != nil || len(attachments) != 1 {
			return nil, false
		}
		params["role"] = "proxy"
		// The project is the route's project ID (the project-id annotation), which the
		// policy reads from the request.
		if annotations := cfg.GetAnnotations(); annotations != nil {
			if v := (*annotations)[commonconstants.AnnotationAgentID]; v != "" {
				params["agent"] = v
			}
		}
		return params, true
	}
	return nil, false
}

// withBudgetEnforcement places the budget system policy where budgetPlacement says, the way
// llm-cost-based-ratelimit sits: after the route's authentication, so its request-header
// phase sees the caller, and just before the policy that prices the response, so in the
// reverse-order response phase it reads the cost that policy published. A route that does
// not price its responses (llm-cost or azure-llm-cost) gets llm-cost appended after it; a
// gateway build without llm-cost gets the budget policy alone, which then charges $0.
// withBudgets places the budget system policy on an LLM route when budgets or AI cost
// stats are on ([ai_cost_management.budgets] / [ai_cost_management.tracking] enabled); with both off, every chain is left
// as it is.
func (t *RestAPITransformer) withBudgets(cfg *models.StoredConfig, chain []policyenginev1.PolicyInstance) []policyenginev1.PolicyInstance {
	if t.systemConfig == nil || (!t.systemConfig.AICostManagement.Budgets.Enabled && !t.systemConfig.AICostManagement.Tracking.Enabled) {
		return chain
	}
	return withBudgetEnforcement(cfg, chain, t.policyDefinitions, t.latestVersions, t.systemConfig.AICostManagement.Tracking.Enabled)
}

func withBudgetEnforcement(
	cfg *models.StoredConfig,
	chain []policyenginev1.PolicyInstance,
	definitions map[string]models.PolicyDefinition,
	latestVersions map[string]string,
	costStats bool,
) []policyenginev1.PolicyInstance {
	params, ok := budgetPlacement(cfg)
	if !ok {
		return chain
	}
	if costStats {
		// The policy also counts every request's cost ([ai_cost_management.tracking] enabled).
		params["costStats"] = true
	}
	budget := policyenginev1.PolicyInstance{
		Name:       constants.BUDGET_SYSTEM_POLICY_NAME,
		Version:    constants.BUDGET_SYSTEM_POLICY_VERSION,
		Enabled:    true,
		Parameters: params,
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
