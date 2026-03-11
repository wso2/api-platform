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
	"strings"
)

// AnthropicCalculator handles models with provider "anthropic".
//
// Anthropic uses different response field names from OpenAI:
//   - input_tokens  → PromptTokens
//   - output_tokens → CompletionTokens
//
// It also adds cache token fields and echoes inference_geo in the response
// usage object. The speed flag is NOT echoed — it must be read from the
// original request body via ctx.RequestBody.
//
// The Adjust step carves out cache costs before applying any geo/speed
// multiplier, then adds them back at their original rate.
type AnthropicCalculator struct{}

func (c *AnthropicCalculator) Normalize(responseBody []byte, requestBody []byte) (Usage, error) {
	var resp struct {
		Usage struct {
			InputTokens              int64  `json:"input_tokens"`
			OutputTokens             int64  `json:"output_tokens"`
			CacheCreationInputTokens int64  `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64  `json:"cache_read_input_tokens"`
			InferenceGeo             string `json:"inference_geo"`
			// Anthropic echoes the per-TTL breakdown of cache writes when the caller
			// used mixed TTLs. When present, these two fields sum to CacheCreationInputTokens.
			CacheCreation *struct {
				Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens"`
				Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
			// Built-in server tools (web search, tool search).
			// web_search_requests is the count of web search queries made during the call.
			ServerToolUse *struct {
				WebSearchRequests int64 `json:"web_search_requests"`
			} `json:"server_tool_use"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return Usage{}, err
	}

	// speed and web_search_options are request-side parameters Anthropic does not echo.
	// Read them from the original request body (available via ctx.RequestBody).
	var speed, searchContextSize string
	if len(requestBody) > 0 {
		var req struct {
			Speed            string `json:"speed"`
			WebSearchOptions *struct {
				SearchContextSize string `json:"search_context_size"`
			} `json:"web_search_options"`
		}
		if err := json.Unmarshal(requestBody, &req); err == nil {
			speed = req.Speed
			if req.WebSearchOptions != nil {
				searchContextSize = req.WebSearchOptions.SearchContextSize
			}
		}
	}

	u := resp.Usage
	total := u.InputTokens + u.OutputTokens
	// Anthropic's 200k tier threshold is based on all input token categories:
	// input_tokens + cache_creation_input_tokens + cache_read_input_tokens.
	// Output tokens do not affect the tier selection.
	inputForTiering := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens

	// Split cache writes into 5-minute and 1-hour TTL buckets.
	// When the granular breakdown is absent, treat all writes as 5-minute (the default TTL).
	var cacheWrite5m, cacheWrite1hr int64
	if u.CacheCreation != nil {
		cacheWrite5m = u.CacheCreation.Ephemeral5mInputTokens
		cacheWrite1hr = u.CacheCreation.Ephemeral1hInputTokens
	} else {
		cacheWrite5m = u.CacheCreationInputTokens
	}

	var webSearchRequests int64
	if u.ServerToolUse != nil {
		webSearchRequests = u.ServerToolUse.WebSearchRequests
	}

	// Anthropic's input_tokens is ONLY the regular (non-cached) input tokens.
	// Cache tokens are separate fields. genericCalculateCost expects PromptTokens
	// to include ALL input types (regular + cache writes + cache reads) and subtracts
	// the cache buckets to derive the regular count. We must add cache tokens here so
	// the subtraction yields the correct regular input count — same approach as LiteLLM.
	promptTokens := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens

	return Usage{
		PromptTokens:          promptTokens,
		CompletionTokens:      u.OutputTokens,
		TotalTokens:           total,
		InputTokensForTiering: inputForTiering,
		CachedReadTokens:      u.CacheReadInputTokens,
		CacheWriteTokens:      cacheWrite5m,
		CacheWrite1hrTokens:   cacheWrite1hr,
		InferenceGeo:          u.InferenceGeo,
		Speed:                 speed,
		WebSearchRequests:     webSearchRequests,
		SearchContextSize:     searchContextSize,
	}, nil
}

// Adjust applies Anthropic geo-routing and speed-mode multipliers.
//
// Cache costs are excluded from the multiplier — Anthropic charges the same
// cache rates regardless of geo or speed tier.
func (c *AnthropicCalculator) Adjust(baseCost float64, usage Usage, pricing ModelPricing) float64 {
	geoNormalized := strings.ToLower(usage.InferenceGeo)
	isGeoRouted := geoNormalized != "" &&
		geoNormalized != "global" &&
		geoNormalized != "not_available"
	isFastMode := strings.ToLower(usage.Speed) == "fast"

	if !isGeoRouted && !isFastMode {
		return baseCost
	}

	pse := pricing.ProviderSpecificEntry
	if len(pse) == 0 {
		return baseCost
	}

	multiplier := 1.0
	if isGeoRouted {
		if m, ok := pse[geoNormalized]; ok {
			multiplier *= m
		}
	}
	if isFastMode {
		if m, ok := pse["fast"]; ok {
			multiplier *= m
		}
	}
	if multiplier == 1.0 {
		return baseCost
	}

	// Resolve the actual cache rates used by genericCalculateCost for this request.
	// These depend on whether the >200k tier was triggered (based on InputTokensForTiering).
	// Using the wrong (base) rate here would understate the carve-out and over-multiply cost.
	cacheReadRate := pricing.CacheReadInputTokenCost
	cacheWrite5mRate := pricing.CacheCreationInputTokenCost
	cacheWrite1hrRate := pricing.CacheCreationInputTokenCostAbove1hr
	if cacheWrite1hrRate == 0 {
		cacheWrite1hrRate = cacheWrite5mRate
	}
	if usage.InputTokensForTiering > 200_000 && pricing.InputCostPerTokenAbove200k > 0 {
		if pricing.CacheReadInputTokenCostAbove200k > 0 {
			cacheReadRate = pricing.CacheReadInputTokenCostAbove200k
		}
		if pricing.CacheCreationInputTokenCostAbove200k > 0 {
			cacheWrite5mRate = pricing.CacheCreationInputTokenCostAbove200k
			cacheWrite1hrRate = pricing.CacheCreationInputTokenCostAbove200k
		}
	}

	// Carve out cache costs before applying multiplier.
	cacheCost := float64(usage.CachedReadTokens)*cacheReadRate +
		float64(usage.CacheWriteTokens)*cacheWrite5mRate +
		float64(usage.CacheWrite1hrTokens)*cacheWrite1hrRate

	nonCacheCost := baseCost - cacheCost
	if nonCacheCost < 0 {
		nonCacheCost = 0
	}

	return nonCacheCost*multiplier + cacheCost
}
