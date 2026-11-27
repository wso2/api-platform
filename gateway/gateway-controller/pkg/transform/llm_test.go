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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

func TestWithBudgetEnforcement(t *testing.T) {
	withCost := map[string]models.PolicyDefinition{
		"llm-cost|v1.2.0": {Name: "llm-cost", Version: "v1.2.0"},
	}
	names := func(chain []policyenginev1.PolicyInstance) []string {
		var out []string
		for _, p := range chain {
			out = append(out, p.Name+"@"+p.Version)
		}
		return out
	}
	auth := policyenginev1.PolicyInstance{Name: "api-key-auth", Version: "v1"}
	guardrail := policyenginev1.PolicyInstance{Name: "pii-masking", Version: "v1"}
	costLimit := policyenginev1.PolicyInstance{Name: "llm-cost-based-ratelimit", Version: "v1"}
	const budget = "wso2_apip_sys_budget@v1"

	provider := &models.StoredConfig{Kind: models.KindLlmProvider}
	oneProviderProxy := &models.StoredConfig{
		Kind:                models.KindLlmProxy,
		SourceConfiguration: api.LLMProxyConfiguration{Spec: api.LLMProxyConfigData{Provider: &api.LLMProxyProvider{Id: "openai"}}},
	}
	twoProviderProxy := &models.StoredConfig{
		Kind: models.KindLlmProxy,
		SourceConfiguration: api.LLMProxyConfiguration{Spec: api.LLMProxyConfigData{
			Provider:            &api.LLMProxyProvider{Id: "openai"},
			AdditionalProviders: &[]api.LLMProxyAdditionalProvider{{Id: "anthropic"}},
		}},
	}
	restAPI := &models.StoredConfig{Kind: models.KindRestApi}

	cases := []struct {
		name  string
		kind  *models.StoredConfig
		chain []policyenginev1.PolicyInstance
		defs  map[string]models.PolicyDefinition
		want  []string
	}{
		{"an unpriced provider gets the budget policy, then llm-cost, after its own policies", provider,
			[]policyenginev1.PolicyInstance{auth, guardrail}, withCost,
			[]string{"api-key-auth@v1", "pii-masking@v1", budget, "llm-cost@v1"}},
		{"a priced provider gets the budget policy just before its llm-cost, as the cost rate limit sits", provider,
			[]policyenginev1.PolicyInstance{auth, costLimit, {Name: "llm-cost", Version: "v1"}, guardrail}, withCost,
			[]string{"api-key-auth@v1", "llm-cost-based-ratelimit@v1", budget, "llm-cost@v1", "pii-masking@v1"}},
		{"azure pricing counts as priced", provider,
			[]policyenginev1.PolicyInstance{auth, {Name: "azure-llm-cost", Version: "v1"}}, withCost,
			[]string{"api-key-auth@v1", budget, "azure-llm-cost@v1"}},
		{"a build without llm-cost gets the budget policy alone", provider,
			[]policyenginev1.PolicyInstance{auth}, map[string]models.PolicyDefinition{},
			[]string{"api-key-auth@v1", budget}},
		{"an empty provider chain", provider,
			nil, withCost, []string{budget, "llm-cost@v1"}},
		{"a one-provider proxy gets the budget policy, then llm-cost: it is the first hop", oneProviderProxy,
			[]policyenginev1.PolicyInstance{auth, guardrail}, withCost, []string{"api-key-auth@v1", "pii-masking@v1", budget, "llm-cost@v1"}},
		{"a proxy with several providers is left alone: it is counted on the provider route", twoProviderProxy,
			[]policyenginev1.PolicyInstance{auth, guardrail}, withCost, []string{"api-key-auth@v1", "pii-masking@v1"}},
		{"REST APIs are left alone", restAPI,
			[]policyenginev1.PolicyInstance{auth}, withCost, []string{"api-key-auth@v1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := withBudgetEnforcement(tc.kind, tc.chain, tc.defs, config.BuildLatestVersionIndex(tc.defs), false)
			assert.Equal(t, tc.want, names(got))
		})
	}
}

func TestBudgetPlacementParameters(t *testing.T) {
	annotations := map[string]string{
		commonconstants.AnnotationProjectID: "api-platform",
		commonconstants.AnnotationAgentID:   "support-agent",
	}
	proxy := &models.StoredConfig{
		Kind:                models.KindLlmProxy,
		Configuration:       api.RestAPI{Metadata: api.Metadata{Annotations: &annotations}},
		SourceConfiguration: api.LLMProxyConfiguration{Spec: api.LLMProxyConfigData{Provider: &api.LLMProxyProvider{Id: "openai"}}},
	}
	params, ok := budgetPlacement(proxy)
	assert.True(t, ok)
	assert.Equal(t, "proxy", params["role"])
	assert.Equal(t, "support-agent", params["agent"])
	assert.NotContains(t, params, "project", "the policy reads the project from the route's project ID")
	assert.Len(t, params["admittedToken"], 64)

	params, ok = budgetPlacement(&models.StoredConfig{Kind: models.KindLlmProvider})
	assert.True(t, ok)
	assert.Equal(t, "provider", params["role"])
	assert.Equal(t, budgetAdmittedToken, params["admittedToken"], "both hops share the controller's token")
	assert.NotContains(t, params, "project")
}

// Budgets off (ai_cost_management.budgets.enabled, the default): an LLM route's chain is exactly
// what was configured, with no budget policy and no llm-cost added. On: the budget policy
// joins it.
func TestBudgetsAreOnlyPlacedWhenEnabled(t *testing.T) {
	defs := map[string]models.PolicyDefinition{"llm-cost|v1.2.0": {Name: "llm-cost", Version: "v1.2.0"}}
	provider := &models.StoredConfig{Kind: models.KindLlmProvider}
	chain := []policyenginev1.PolicyInstance{{Name: "api-key-auth", Version: "v1"}}

	off := NewRestAPITransformer(testRouterCfg(), &config.Config{}, defs)
	assert.Equal(t, chain, off.withBudgets(provider, chain))

	cfg := &config.Config{}
	cfg.AICostManagement.Budgets.Enabled = true
	on := NewRestAPITransformer(testRouterCfg(), cfg, defs)
	var names []string
	for _, p := range on.withBudgets(provider, chain) {
		names = append(names, p.Name)
	}
	assert.Equal(t, []string{"api-key-auth", "wso2_apip_sys_budget", "llm-cost"}, names)
}

// AI cost stats on, budgets off: the policy is placed too, told to count every request.
func TestCostStatsPlaceThePolicyWithoutBudgets(t *testing.T) {
	defs := map[string]models.PolicyDefinition{"llm-cost|v1.2.0": {Name: "llm-cost", Version: "v1.2.0"}}
	provider := &models.StoredConfig{Kind: models.KindLlmProvider}
	cfg := &config.Config{}
	cfg.AICostManagement.Tracking.Enabled = true
	chain := NewRestAPITransformer(testRouterCfg(), cfg, defs).withBudgets(provider, nil)
	require.Len(t, chain, 2)
	assert.Equal(t, "wso2_apip_sys_budget", chain[0].Name)
	assert.Equal(t, true, chain[0].Parameters["costStats"])

	budgetsOnly := &config.Config{}
	budgetsOnly.AICostManagement.Budgets.Enabled = true
	chain = NewRestAPITransformer(testRouterCfg(), budgetsOnly, defs).withBudgets(provider, nil)
	assert.NotContains(t, chain[0].Parameters, "costStats", "budgets alone count no costs")
}
