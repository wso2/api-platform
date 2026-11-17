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
// Spend is counted in the gateway's shared counters (sdk/core/counters): the policy engine
// reports it to its gateway-controller about once a second and reads back the totals, which
// every gateway sharing the controller's Redis sees. Admission never waits on the network.
package budget

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/wso2/api-platform/common/apikey"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Parameters the gateway controller sets on each route it places the policy on.
const (
	// paramAgent names the agent of an agent's hidden LLM proxy route, from the annotation
	// the control plane sets on it. The route's project is the request's project ID, from
	// the route's existing project-id annotation.
	paramAgent = "agent"
	// paramRole is roleProxy on an LLM proxy route and roleProvider on a provider route.
	paramRole = "role"
	// paramAdmittedToken is the controller's secret the proxy route stamps a request with,
	// so the provider route the proxy forwards it to does not count it again.
	paramAdmittedToken = "admittedToken"
	// paramCostStats is true on a route whose requests' costs are counted for the
	// control plane's cost stats ([ai_cost_management.tracking] enabled on the gateway).
	paramCostStats = "costStats"

	roleProxy    = "proxy"
	roleProvider = "provider"

	// headerAdmitted carries the stamp from the proxy route to the provider route.
	headerAdmitted = "x-wso2-budget-admitted"
	// authTypeAPIKey is the auth type api-key-auth records.
	authTypeAPIKey = "apikey"
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
	// apiKeyName returns the name of an API's key from the hash of its value: the token ID
	// api-key-auth records. Empty when unknown.
	apiKeyName func(apiID, hash string) string
}

var ins = newPolicy(policy.GetLazyResourceStoreInstance())

func newPolicy(source resourceSource) *BudgetSystemPolicy {
	c, err := newCatalog(source)
	if err != nil {
		// The environment declares one variable; it cannot fail at run time.
		panic("budget: creating the CEL environment: " + err.Error())
	}
	return &BudgetSystemPolicy{catalog: c, tracker: newTracker(installedCounters{}), now: time.Now, apiKeyName: storedAPIKeyName}
}

// storedAPIKeyName reads the key's name from the policy engine's API key store.
func storedAPIKeyName(apiID, hash string) string {
	key, err := apikey.GetAPIkeyStoreInstance().GetAPIKeyByHash(apiID, hash)
	if err != nil {
		return ""
	}
	return key.Name
}

// routeParams are the policy's parameters on one route.
type routeParams struct {
	agent, role, admittedToken string
	costStats                  bool
}

func readParams(params map[string]any) routeParams {
	get := func(k string) string { v, _ := params[k].(string); return v }
	costStats, _ := params[paramCostStats].(bool)
	return routeParams{agent: get(paramAgent), role: get(paramRole), admittedToken: get(paramAdmittedToken), costStats: costStats}
}

// tokenSubject is the verified token's subject: the user for cost stats. Empty for an API
// key, which names no user.
func tokenSubject(auth *policy.AuthContext) string {
	if auth == nil || !auth.Authenticated || auth.AuthType == authTypeAPIKey {
		return ""
	}
	return auth.Subject
}

// stampedByProxy reports whether the proxy route already evaluated the request: it carries
// this controller's token. A forged or missing value is not.
func stampedByProxy(headers map[string][]string, token string) bool {
	if token == "" {
		return false
	}
	for name, values := range headers {
		if len(name) == len(headerAdmitted) && http.CanonicalHeaderKey(name) == http.CanonicalHeaderKey(headerAdmitted) {
			for _, v := range values {
				if subtle.ConstantTimeCompare([]byte(v), []byte(token)) == 1 {
					return true
				}
			}
		}
	}
	return false
}

// callerOf names the request's caller: on a proxy route, its project (the route's project
// ID) and agent; and the API key it authenticated with. A provider route names no project:
// its project ID is the provider's, not the caller's.
func (p *BudgetSystemPolicy) callerOf(reqCtx *policy.RequestHeaderContext, rp routeParams) caller {
	who := caller{}
	if rp.role == roleProxy {
		who.project, who.agent = reqCtx.SharedContext.ProjectID, rp.agent
	}
	auth := reqCtx.AuthContext
	if auth != nil && auth.Authenticated && auth.AuthType == authTypeAPIKey && auth.TokenId != "" && p.apiKeyName != nil {
		who.apiKey = p.apiKeyName(reqCtx.SharedContext.APIId, auth.TokenId)
	}
	return who
}

// GetPolicy returns the shared instance. Route parameters (project, agent, role, admitted
// token) arrive with each request.
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

// admitted is one allowance a request was admitted against, in the period it was admitted
// in, with the labels that describe it to the control plane.
type admitted struct {
	spendKey
	labels map[string]string
}

// OnRequestHeaders finds the budgets that cover the request and refuses it when a Block
// budget's allowance has spent its block point.
func (p *BudgetSystemPolicy) OnRequestHeaders(_ context.Context, reqCtx *policy.RequestHeaderContext, params map[string]any) policy.RequestHeaderAction {
	if reqCtx == nil || reqCtx.SharedContext == nil {
		return policy.UpstreamRequestHeaderModifications{}
	}
	rp := readParams(params)
	var headers map[string][]string
	if reqCtx.Headers != nil {
		headers = reqCtx.Headers.GetAll()
	}
	// pass is what the request goes on with when no budget refuses it. A proxy route stamps
	// it for the provider route it forwards to; a provider route always removes the stamp,
	// so the token never reaches the LLM provider.
	pass := policy.UpstreamRequestHeaderModifications{}
	switch rp.role {
	case roleProxy:
		if rp.admittedToken != "" {
			pass.HeadersToSet = map[string]string{headerAdmitted: rp.admittedToken}
		}
	case roleProvider:
		pass.HeadersToRemove = []string{headerAdmitted}
		if stampedByProxy(headers, rp.admittedToken) {
			return pass // counted on the proxy route, once
		}
	}
	provider, _ := reqCtx.Metadata[metadataProviderName].(string)
	if provider == "" {
		return pass
	}
	who := p.callerOf(reqCtx, rp)
	if rp.costStats {
		// Every request is counted in the cost stats, with or without a budget.
		reqCtx.Metadata[metadataCostCaller] = costCaller{
			project: who.project, agent: who.agent, apiKey: who.apiKey,
			user: tokenSubject(reqCtx.AuthContext), provider: provider,
		}
	}
	budgets := p.catalog.current()
	if len(budgets) == 0 {
		return pass
	}

	now := p.now().UTC()
	var vars map[string]any
	lazyVars := func() map[string]any {
		if vars == nil {
			vars = activation(headers, reqCtx.Path, reqCtx.Method, reqCtx.Metadata, reqCtx.AuthContext)
		}
		return vars
	}
	var keys []admitted
	var exhausted []*budget
	var names []any
	for _, b := range budgets {
		allowances, ok := b.matches(provider, who, lazyVars)
		if !ok {
			continue
		}
		names = append(names, b.name)
		start := periodStart(b.period, now)
		used := false
		for _, a := range allowances {
			key := admitted{spendKey: spendKey{allowance: a.id, periodStart: start}, labels: a.labels(b.name)}
			keys = append(keys, key)
			if b.action == actionBlock && p.tracker.spentIn(key.spendKey) >= b.blockPoint() {
				used = true
			}
		}
		if used {
			exhausted = append(exhausted, b)
		}
	}
	if len(names) == 0 {
		return pass
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
	pass.AnalyticsMetadata = analytics
	return pass
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
	caller, counted := shared.Metadata[metadataCostCaller].(costCaller)
	if len(keys) == 0 && !counted {
		return nil
	}
	delete(shared.Metadata, metadataAllowances)
	delete(shared.Metadata, metadataCostCaller)

	status, _ := shared.Metadata[metadataLLMCostStatus].(string)
	costText, _ := shared.Metadata[metadataLLMCost].(string)
	cost, err := strconv.ParseFloat(costText, 64)
	priced := status == costStatusCalculated && err == nil && cost >= 0 && !math.IsNaN(cost) && !math.IsInf(cost, 0)
	costStatus := costStatusPriced
	if !priced {
		costStatus, cost = unpricedReason(shared), 0
	}
	if counted {
		p.tracker.recordCost(caller, usageModel(shared.Metadata[metadataUsage]), monthStart(p.now()), cost, costStatus)
	}
	if len(keys) == 0 {
		// Counted in the cost stats only: no budget covered it, so analytics gets nothing.
		return nil
	}
	if !priced {
		p.tracker.countUnpriced(keys)
	} else {
		p.tracker.charge(keys, cost)
	}
	return map[string]any{analyticsBudgetCostStatus: costStatus}
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
