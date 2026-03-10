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
		} `json:"usage"`
	}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return Usage{}, err
	}

	// speed is a request-side parameter Anthropic does not echo in the response.
	// Read it from the original request body (available via ctx.RequestBody).
	var speed string
	if len(requestBody) > 0 {
		var req struct {
			Speed string `json:"speed"`
		}
		if err := json.Unmarshal(requestBody, &req); err == nil {
			speed = req.Speed
		}
	}

	u := resp.Usage
	total := u.InputTokens + u.OutputTokens
	// Anthropic's 200k tier threshold is based on all input token categories:
	// input_tokens + cache_creation_input_tokens + cache_read_input_tokens.
	// Output tokens do not affect the tier selection.
	inputForTiering := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
	return Usage{
		PromptTokens:          u.InputTokens,
		CompletionTokens:      u.OutputTokens,
		TotalTokens:           total,
		InputTokensForTiering: inputForTiering,
		CachedReadTokens:      u.CacheReadInputTokens,
		CacheWriteTokens:      u.CacheCreationInputTokens,
		InferenceGeo:          u.InferenceGeo,
		Speed:                 speed,
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

	// Carve out cache costs before applying multiplier.
	cacheCost := float64(usage.CachedReadTokens)*pricing.CacheReadInputTokenCost +
		float64(usage.CacheWriteTokens)*pricing.CacheCreationInputTokenCost

	nonCacheCost := baseCost - cacheCost
	if nonCacheCost < 0 {
		nonCacheCost = 0
	}

	return nonCacheCost*multiplier + cacheCost
}
