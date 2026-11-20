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

package budget

import (
	"context"
	"github.com/wso2/api-platform/common/budgetspend"
	"testing"

	"github.com/wso2/api-platform/sdk/core/counters"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// fakeUsage stands for sdk/ai/llmusage's Usage: the policy reads only its Model field.
type fakeUsage struct {
	Model string
}

// costRoute is an agent's route with cost stats on.
func costRoute(agent string) map[string]any {
	params := agentRoute(agent)
	params[paramCostStats] = true
	return params
}

// sendWithUsage admits c and ends its response with the cost, cost status and the model
// the usage extraction found.
func sendWithUsage(t *testing.T, p *BudgetSystemPolicy, c call, cost, status, usageStatus, model string) map[string]any {
	t.Helper()
	req := c.context()
	if _, refused := p.OnRequestHeaders(context.Background(), req, c.params).(policy.ImmediateResponse); refused {
		t.Fatal("refused")
	}
	req.Metadata[metadataLLMCost] = cost
	req.Metadata[metadataLLMCostStatus] = status
	if usageStatus != "" {
		req.Metadata[metadataUsageStatus] = usageStatus
	}
	if model != "" {
		req.Metadata[metadataUsage] = fakeUsage{Model: model}
	}
	resp := p.OnResponseBody(context.Background(), &policy.ResponseContext{SharedContext: req.SharedContext}, nil).(policy.DownstreamResponseModifications)
	return resp.AnalyticsMetadata
}

func costKey(namespace string, labels map[string]string) counters.Key {
	return counters.Key{Namespace: namespace, ID: costID(labels), Window: monthStart(wednesdayNoon)}
}

func TestCostStatsCountEveryRequestWithoutABudget(t *testing.T) {
	p, _, _ := testPolicy(t)
	mem := counters.NewMemory()
	p.tracker = newTracker(mem)

	support := call{provider: "openai", project: "api-platform", subject: "alice@wso2.com", params: costRoute("support-agent")}
	analytics := sendWithUsage(t, p, support, "1.5", costStatusCalculated, usageStatusExtracted, "gpt-4o")
	sendWithUsage(t, p, support, "0.5", costStatusCalculated, usageStatusExtracted, "gpt-4o")

	labels := map[string]string{
		costLabelProject: "api-platform", costLabelAgent: "support-agent", costLabelUser: "alice@wso2.com",
		costLabelLLMProvider: "openai", costLabelModel: "gpt-4o",
	}
	key := costKey(namespaceCost, labels)
	if total, _ := mem.Total(key); total != 2 {
		t.Fatalf("cost %v, want 2", total)
	}
	if got := mem.Labels(key); len(got) != len(labels) || got[costLabelModel] != "gpt-4o" || got[costLabelAgent] != "support-agent" {
		t.Fatalf("labels %v, want %v", got, labels)
	}
	// No budget covered it: analytics still gets the cost and the agent, for spend per agent.
	if analytics[analyticsBudgetAgent] != "api-platform/support-agent" || analytics[analyticsBudgetCostStatus] != costStatusPriced {
		t.Fatalf("analytics %v, want the agent and priced", analytics)
	}
	if _, ok := analytics[analyticsBudgetCostUSD].(float64); !ok {
		t.Fatalf("analytics %v, want costUsd as a number", analytics)
	}
	if _, ok := analytics[analyticsBudgetNames]; ok {
		t.Fatalf("analytics %v names a budget, but none covered it", analytics)
	}
}

func TestCostStatsAreOffUnlessTheRouteAsks(t *testing.T) {
	p, _, _ := testPolicy(t)
	mem := counters.NewMemory()
	p.tracker = newTracker(mem)
	sendWithUsage(t, p, call{provider: "openai", project: "api-platform", params: agentRoute("support-agent")},
		"1", costStatusCalculated, usageStatusExtracted, "gpt-4o")
	labels := map[string]string{
		costLabelProject: "api-platform", costLabelAgent: "support-agent", costLabelLLMProvider: "openai", costLabelModel: "gpt-4o",
	}
	if total, _ := mem.Total(costKey(namespaceCost, labels)); total != 0 || mem.Labels(costKey(namespaceCost, labels)) != nil {
		t.Fatal("counted without costStats on the route")
	}
}

func TestUnpricedRequestsAreCountedByReason(t *testing.T) {
	p, _, _ := testPolicy(t)
	mem := counters.NewMemory()
	p.tracker = newTracker(mem)
	key := call{provider: "openai", apiKey: "hash-1", params: map[string]any{paramCostStats: true}}
	p.apiKeyName = func(_, _ string) string { return "ci-key" }

	sendWithUsage(t, p, key, "0", "not_calculated", usageStatusExtracted, "gpt-4o")
	sendWithUsage(t, p, key, "0", "not_calculated", "no_usage", "")

	missing := map[string]string{costLabelAPIKey: "ci-key", costLabelLLMProvider: "openai", costLabelModel: "gpt-4o"}
	if total, _ := mem.Total(costKey(namespaceCostMissingPrice, missing)); total != 1 {
		t.Fatalf("missing price %v, want 1", total)
	}
	noUsage := map[string]string{costLabelAPIKey: "ci-key", costLabelLLMProvider: "openai"}
	if total, _ := mem.Total(costKey(namespaceCostUsageUnavailable, noUsage)); total != 1 {
		t.Fatalf("usage unavailable %v, want 1", total)
	}
	if total, _ := mem.Total(costKey(namespaceCost, missing)); total != 0 {
		t.Fatal("an unpriced request adds no cost")
	}
}

func TestABudgetAndTheCostStatsAreBothCharged(t *testing.T) {
	p, _, _ := testPolicy(t, budgetBody("all", testBudgetUUID, "10", "day", "allow", allProviders, nil))
	mem := counters.NewMemory()
	p.tracker = newTracker(mem)
	analytics := sendWithUsage(t, p, call{provider: "openai", params: map[string]any{paramCostStats: true}},
		"3", costStatusCalculated, usageStatusExtracted, "gpt-4o")

	if total, _ := mem.Total(costKey(namespaceCost, map[string]string{costLabelLLMProvider: "openai", costLabelModel: "gpt-4o"})); total != 3 {
		t.Fatalf("cost %v, want 3", total)
	}
	if analytics[analyticsBudgetCostStatus] != costStatusPriced {
		t.Fatalf("analytics %v", analytics)
	}
	allowances, _ := p.catalog.current()[0].matches("openai", caller{}, nil)
	spent, _ := mem.Total(spendCounter(spendKey{spendID: budgetspend.SpendID("all"), allowance: allowances[0].id, periodStart: periodStart(periodDay, wednesdayNoon)}))
	if spent != 3 {
		t.Fatalf("budget spend %v, want 3", spent)
	}
}

func TestUsageModel(t *testing.T) {
	model := fakeUsage{Model: "gpt-4o"}
	var none *fakeUsage
	for _, tc := range []struct {
		usage any
		want  string
	}{
		{model, "gpt-4o"}, {&model, "gpt-4o"}, {none, ""}, {nil, ""}, {"gpt-4o", ""}, {struct{ Model int }{1}, ""},
	} {
		if got := usageModel(tc.usage); got != tc.want {
			t.Fatalf("usageModel(%#v) = %q, want %q", tc.usage, got, tc.want)
		}
	}
}
