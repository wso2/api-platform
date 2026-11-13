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

// Package budget is the gateway's budget system policy: it enforces spend limits on LLM
// traffic. The gateway controller attaches it to every LLM provider route, with llm-cost,
// and publishes the active budgets to the policy engine as lazy resources of type Budget.
// An LLM proxy call reaches its provider's route over the internal loopback, so it is
// counted there, once.
//
// It sits in the chain the way llm-cost-based-ratelimit does: after the route's
// authentication, so the request-header phase, where a request is admitted or refused, sees
// the caller; and just before the policy that prices the response. Response phases run in
// reverse chain order, so when the response ends llm-cost has published x-llm-cost and the
// policy charges it. It never reads a body.
//
// Spend is kept in memory in this policy engine. It resets on restart and is not shared with
// other gateways; the shared spend tracker replaces it.
package budget

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Metadata keys read from, and written to, the request's shared metadata.
const (
	// metadataProviderName is the LLM provider handle the route serves, set by the engine.
	metadataProviderName = "provider_name"
	// metadataLLMCost and metadataLLMCostStatus are published by llm-cost.
	metadataLLMCost       = "x-llm-cost"
	metadataLLMCostStatus = "x-llm-cost-status"
	costStatusCalculated  = "calculated"
	// metadataAllowances carries the allowances a request was admitted against to the
	// response phase.
	metadataAllowances = "x-wso2-budget-allowances"
	// metadataUsageStatus is published by llm-cost's usage extraction (sdk/ai llmusage):
	// "extracted" when the response carried token usage.
	metadataUsageStatus  = "llm:usage:status"
	usageStatusExtracted = "extracted"
)

// Analytics properties added to the request's analytics event: the budgets it counted
// against, never allowance values, and how it was priced.
const (
	analyticsBudgetNames      = "budgetNames"
	analyticsBudgetCostStatus = "budgetCostStatus"

	costStatusPriced           = "priced"
	costStatusMissingPrice     = "missing_price"
	costStatusUsageUnavailable = "usage_unavailable"
)

// BudgetSystemPolicy enforces budgets. Every attachment shares one instance, so spend is
// counted once per policy engine.
type BudgetSystemPolicy struct {
	catalog *catalog
	tracker *tracker
	now     func() time.Time
}

var ins = newPolicy(policy.GetLazyResourceStoreInstance())

func newPolicy(source resourceSource) *BudgetSystemPolicy {
	c, err := newCatalog(source)
	if err != nil {
		// The environment declares one variable; it cannot fail at run time.
		panic("budget: creating the CEL environment: " + err.Error())
	}
	return &BudgetSystemPolicy{catalog: c, tracker: newTracker(), now: time.Now}
}

// GetPolicy returns the shared instance. The policy takes no parameters.
func GetPolicy(_ policy.PolicyMetadata, _ map[string]any) (policy.Policy, error) {
	return ins, nil
}

// GetPolicyV2 is an alias for GetPolicy, provided for compatibility with plugin registries
// that call GetPolicyV2 on all plugins.
func GetPolicyV2(metadata policy.PolicyMetadata, params map[string]any) (policy.Policy, error) {
	return GetPolicy(metadata, params)
}

// Mode admits in the request-header phase and charges when the streamed response ends, as
// llm-cost-based-ratelimit does. Bodies are never read: the cost comes from metadata.
func (p *BudgetSystemPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeStream,
	}
}

// admitted is one allowance a request was admitted against, in the period it was admitted in.
type admitted = spendKey

// OnRequestHeaders finds the budgets that cover the request and refuses it when a Block
// budget's allowance has spent its block point.
func (p *BudgetSystemPolicy) OnRequestHeaders(_ context.Context, reqCtx *policy.RequestHeaderContext, _ map[string]any) policy.RequestHeaderAction {
	if reqCtx == nil || reqCtx.SharedContext == nil {
		return policy.UpstreamRequestHeaderModifications{}
	}
	var headers map[string][]string
	if reqCtx.Headers != nil {
		headers = reqCtx.Headers.GetAll()
	}
	provider, _ := reqCtx.Metadata[metadataProviderName].(string)
	if provider == "" {
		return policy.UpstreamRequestHeaderModifications{}
	}
	budgets := p.catalog.current()
	if len(budgets) == 0 {
		return policy.UpstreamRequestHeaderModifications{}
	}

	now := p.now().UTC()
	vars := activation(headers, reqCtx.Path, reqCtx.Method, reqCtx.Metadata, reqCtx.AuthContext)
	var keys []admitted
	var exhausted []*budget
	var names []any
	for _, b := range budgets {
		allowances, ok := b.matches(provider, vars)
		if !ok {
			continue
		}
		names = append(names, b.name)
		start := periodStart(b.period, now)
		used := false
		for _, allowance := range allowances {
			key := admitted{allowance: allowance, periodStart: start}
			keys = append(keys, key)
			if b.action == actionBlock && p.tracker.spentIn(key) >= b.blockPoint() {
				used = true
			}
		}
		if used {
			exhausted = append(exhausted, b)
		}
	}
	if len(names) == 0 {
		return policy.UpstreamRequestHeaderModifications{}
	}
	analytics := map[string]any{analyticsBudgetNames: names}
	if len(exhausted) > 0 {
		refused := refusal(exhausted, now)
		refused.AnalyticsMetadata = analytics
		return refused
	}
	if reqCtx.Metadata == nil {
		reqCtx.Metadata = map[string]any{}
	}
	reqCtx.Metadata[metadataAllowances] = keys
	return policy.UpstreamRequestHeaderModifications{AnalyticsMetadata: analytics}
}

// NeedsMoreResponseData is false: the cost is read from metadata, not the body.
func (p *BudgetSystemPolicy) NeedsMoreResponseData(_ []byte) bool { return false }

// OnResponseBodyChunk charges the request when its response ends.
func (p *BudgetSystemPolicy) OnResponseBodyChunk(
	_ context.Context, respCtx *policy.ResponseStreamContext, chunk *policy.StreamBody, _ map[string]any,
) policy.StreamingResponseAction {
	if chunk != nil && chunk.EndOfStream && respCtx != nil {
		return policy.ForwardResponseChunk{AnalyticsMetadata: p.charge(respCtx.SharedContext)}
	}
	return policy.ForwardResponseChunk{}
}

// OnResponseBody charges a request whose response was delivered whole.
func (p *BudgetSystemPolicy) OnResponseBody(_ context.Context, respCtx *policy.ResponseContext, _ map[string]any) policy.ResponseAction {
	if respCtx != nil {
		return policy.DownstreamResponseModifications{AnalyticsMetadata: p.charge(respCtx.SharedContext)}
	}
	return policy.DownstreamResponseModifications{}
}

// charge adds the cost llm-cost published to every allowance the request was admitted
// against; without a calculated cost the request is charged $0 and counted as unpriced.
// It charges once: the allowances are removed from the metadata. It returns the analytics
// property naming how the request was priced, or nil when no budget covered it.
func (p *BudgetSystemPolicy) charge(shared *policy.SharedContext) map[string]any {
	if shared == nil || shared.Metadata == nil {
		return nil
	}
	keys, _ := shared.Metadata[metadataAllowances].([]admitted)
	if len(keys) == 0 {
		return nil
	}
	delete(shared.Metadata, metadataAllowances)

	now := p.now().UTC()
	status, _ := shared.Metadata[metadataLLMCostStatus].(string)
	costText, _ := shared.Metadata[metadataLLMCost].(string)
	cost, err := strconv.ParseFloat(costText, 64)
	if status != costStatusCalculated || err != nil || cost < 0 || math.IsNaN(cost) || math.IsInf(cost, 0) {
		p.tracker.countUnpriced(keys, now)
		return map[string]any{analyticsBudgetCostStatus: unpricedReason(shared)}
	}
	p.tracker.charge(keys, cost, now)
	return map[string]any{analyticsBudgetCostStatus: costStatusPriced}
}

// unpricedReason tells a response without usage from one whose model has no price: the
// usage extraction llm-cost runs records whether the response carried token usage.
func unpricedReason(shared *policy.SharedContext) string {
	if status, _ := shared.Metadata[metadataUsageStatus].(string); status == usageStatusExtracted {
		return costStatusMissingPrice
	}
	return costStatusUsageUnavailable
}

// refusalBody is the 429 body: each used-up budget once, in name order; allowances are
// never shown.
type refusalBody struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Budgets []refusedBudget `json:"budgets"`
}

type refusedBudget struct {
	Budget   string `json:"budget"`
	Limit    string `json:"limit"`
	Currency string `json:"currency"`
	Period   string `json:"period"`
	ResetsAt string `json:"resetsAt"`
}

// refusal answers 429 with Retry-After until the last listed budget resets.
func refusal(exhausted []*budget, now time.Time) policy.ImmediateResponse {
	body := refusalBody{
		Code:    "BUDGET_EXCEEDED",
		Message: "Budget used up. Retry after every listed budget resets.",
	}
	var latest time.Time
	for _, b := range exhausted {
		resets := periodEnd(b.period, periodStart(b.period, now))
		if resets.After(latest) {
			latest = resets
		}
		body.Budgets = append(body.Budgets, refusedBudget{
			Budget: b.name, Limit: b.limit, Currency: b.currency, Period: b.period,
			ResetsAt: resets.Format(time.RFC3339),
		})
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		slog.Error("Failed to encode the budget refusal", slog.Any("error", err))
		encoded = []byte(`{"code":"BUDGET_EXCEEDED"}`)
	}
	retryAfter := int64(math.Ceil(latest.Sub(now).Seconds()))
	return policy.ImmediateResponse{
		StatusCode: http.StatusTooManyRequests,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Retry-After":  strconv.FormatInt(retryAfter, 10),
		},
		Body: encoded,
	}
}
