/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package llmcost

import (
	"encoding/json"
	"fmt"
	"log/slog"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	// HeaderLLMCost is the response header set by this policy.
	// Value is a USD float formatted to 10 decimal places.
	HeaderLLMCost = "x-llm-cost"
)

// LLMCostPolicy calculates the cost of an LLM API call from the response body
// and injects the result as an x-llm-cost response header in USD.
type LLMCostPolicy struct{}

var instance = &LLMCostPolicy{}

// GetPolicy returns the singleton policy instance. Params are unused — this
// policy has no user-configurable parameters.
func GetPolicy(
	_ policy.PolicyMetadata,
	_ map[string]interface{},
) (policy.Policy, error) {
	return instance, nil
}

// Mode declares the SDK processing requirements:
//   - RequestBodyMode=Buffer: buffer the request so ctx.RequestBody is available
//     in OnResponse (needed for Anthropic speed parameter).
//   - ResponseBodyMode=Buffer: buffer the full response body so we can parse
//     the usage object and model name.
//   - ResponseHeaderMode=Process: we write a response header.
func (p *LLMCostPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseBodyMode:   policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeProcess,
	}
}

// OnRequest is a no-op — all work is done in OnResponse.
func (p *LLMCostPolicy) OnRequest(ctx *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return policy.UpstreamRequestModifications{}
}

// OnResponse reads the LLM response, looks up model pricing, calculates cost,
// and sets the x-llm-cost header.
func (p *LLMCostPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Apply custom pricing file if the gateway admin configured one.
	// This merges the file's entries on top of the embedded pricing map.
	if pricingFile, ok := params["pricing_file"].(string); ok && pricingFile != "" {
		mergeCustomPricing(pricingFile)
	}

	if ctx.ResponseBody == nil || !ctx.ResponseBody.Present || len(ctx.ResponseBody.Content) == 0 {
		slog.Warn("llm-cost: empty or missing response body, skipping cost calculation")
		return policy.UpstreamResponseModifications{}
	}

	responseBody := ctx.ResponseBody.Content

	// Extract model name from response body.
	// OpenAI-compatible providers use $.model; Gemini uses $.modelVersion.
	var probe struct {
		Model        string `json:"model"`
		ModelVersion string `json:"modelVersion"`
	}
	if err := json.Unmarshal(responseBody, &probe); err != nil {
		slog.Warn("llm-cost: could not parse response body", "error", err)
		return policy.UpstreamResponseModifications{}
	}
	modelName := probe.Model
	if modelName == "" {
		modelName = probe.ModelVersion
	}
	if modelName == "" {
		slog.Warn("llm-cost: no model name found in response body ($.model or $.modelVersion)")
		return policy.UpstreamResponseModifications{}
	}

	// Look up pricing entry.
	pricing, found := lookupPricing(modelName)
	if !found {
		slog.Warn("llm-cost: no pricing entry for model, setting cost to 0", "model", modelName)
		return setCostHeader(0.0)
	}

	// Select provider calculator.
	calc := selectCalculator(pricing.Provider)

	// Get buffered request body (may be nil for providers that don't need it).
	var requestBody []byte
	if ctx.RequestBody != nil && ctx.RequestBody.Present {
		requestBody = ctx.RequestBody.Content
	}

	// Normalize provider-specific usage fields into our common Usage struct.
	usage, err := calc.Normalize(responseBody, requestBody)
	if err != nil {
		slog.Warn("llm-cost: failed to normalize usage", "model", modelName, "error", err)
		return setCostHeader(0.0)
	}

	// Calculate base cost using the provider-agnostic generic calculator.
	baseCost := genericCalculateCost(usage, pricing)

	// Apply provider-specific adjustments (geo/speed multipliers, router flat cost, etc.).
	finalCost := calc.Adjust(baseCost, usage, pricing)

	slog.Debug("llm-cost: calculated cost",
		"model", modelName,
		"provider", pricing.Provider,
		"prompt_tokens", usage.PromptTokens,
		"completion_tokens", usage.CompletionTokens,
		"cost_usd", finalCost,
	)

	return setCostHeader(finalCost)
}

// setCostHeader returns a ResponseAction that sets x-llm-cost to the given USD value.
func setCostHeader(costUSD float64) policy.ResponseAction {
	return policy.UpstreamResponseModifications{
		SetHeaders: map[string]string{
			HeaderLLMCost: fmt.Sprintf("%.10f", costUSD),
		},
	}
}
