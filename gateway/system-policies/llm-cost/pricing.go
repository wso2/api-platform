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
	_ "embed"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
)

//go:embed pricing/model_prices.json
var modelPricesJSON []byte

// pricingMap is the global map from model key → ModelPricing, loaded once at init.
var pricingMap map[string]ModelPricing

func init() {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(modelPricesJSON, &raw); err != nil {
		slog.Error("llm-cost: failed to parse embedded model_prices.json", "error", err)
		pricingMap = map[string]ModelPricing{}
		return
	}
	pricingMap = make(map[string]ModelPricing, len(raw))
	for key, msg := range raw {
		var p ModelPricing
		if err := json.Unmarshal(msg, &p); err != nil {
			continue
		}
		pricingMap[key] = p
	}
	slog.Info("llm-cost: embedded pricing map loaded", "entries", len(pricingMap))
}

// mergeCustomPricing reads a JSON file at path and merges its entries into
// pricingMap, overriding existing keys and adding new ones. If the file does
// not exist the function is a no-op. Any parse error is logged and skipped.
func mergeCustomPricing(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("llm-cost: could not read custom pricing file", "path", path, "error", err)
		}
		return
	}

	custom := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &custom); err != nil {
		slog.Error("llm-cost: failed to parse custom pricing file", "path", path, "error", err)
		return
	}

	added, overridden := 0, 0
	for key, msg := range custom {
		var p ModelPricing
		if err := json.Unmarshal(msg, &p); err != nil {
			slog.Warn("llm-cost: skipping invalid entry in custom pricing file", "key", key, "error", err)
			continue
		}
		if _, exists := pricingMap[key]; exists {
			overridden++
		} else {
			added++
		}
		pricingMap[key] = p
	}
	slog.Info("llm-cost: custom pricing file merged",
		"path", path,
		"overridden", overridden,
		"added", added,
	)
}

// ModelPricing holds all cost rate fields for a single model entry.
// Fields map directly to keys in model_prices.json.
type ModelPricing struct {
	Provider string `json:"provider"`

	// Standard token rates (per token, not per 1k)
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`

	// Tiered rates — above 128k context window (Gemini 1.x, some OpenAI)
	InputCostPerTokenAbove128k  float64 `json:"input_cost_per_token_above_128k_tokens"`
	OutputCostPerTokenAbove128k float64 `json:"output_cost_per_token_above_128k_tokens"`

	// Tiered rates — above 200k context window (Gemini 2.x, Claude Opus 4)
	InputCostPerTokenAbove200k  float64 `json:"input_cost_per_token_above_200k_tokens"`
	OutputCostPerTokenAbove200k float64 `json:"output_cost_per_token_above_200k_tokens"`

	// Prompt caching
	CacheReadInputTokenCost              float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost          float64 `json:"cache_creation_input_token_cost"`
	CacheCreationInputTokenCostAbove1hr  float64 `json:"cache_creation_input_token_cost_above_1hr"`
	CacheReadInputTokenCostAbove200k     float64 `json:"cache_read_input_token_cost_above_200k_tokens"`
	CacheCreationInputTokenCostAbove200k float64 `json:"cache_creation_input_token_cost_above_200k_tokens"`

	// Reasoning tokens (o-series, Claude 3.7+, Gemini thinking)
	OutputCostPerReasoningToken float64 `json:"output_cost_per_reasoning_token"`

	// Batch API discount (OpenAI, Azure)
	InputCostPerTokenBatches  float64 `json:"input_cost_per_token_batches"`
	OutputCostPerTokenBatches float64 `json:"output_cost_per_token_batches"`

	// Azure AI Foundry model router flat cost
	// Stored under input_cost_per_token for the azure_ai/model_router entry.
	// Handled separately in AzureAICalculator.

	// Anthropic geo/speed multipliers — stored in provider_specific_entry in the JSON.
	// We decode this sub-object into ProviderSpecificEntry.
	ProviderSpecificEntry map[string]float64 `json:"provider_specific_entry"`

	// Context window limits (used for tiering decisions)
	MaxInputTokens int64 `json:"max_input_tokens"`
	MaxTokens      int64 `json:"max_tokens"`
}

// Usage holds the normalised token counts extracted from an LLM response.
// Every provider calculator maps its raw response fields into this struct.
type Usage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64

	// InputTokensForTiering is the token count used to decide which pricing
	// tier applies (>128k or >200k). Providers set this explicitly because the
	// threshold definition varies:
	//   - Anthropic: input_tokens + cache_creation_input_tokens + cache_read_input_tokens
	//     (output tokens are excluded; all input categories count toward the threshold)
	//   - Others: total prompt tokens (PromptTokens)
	// If zero, genericCalculateCost falls back to TotalTokens.
	InputTokensForTiering int64

	// Cached / reasoning tokens
	CachedReadTokens int64
	CacheWriteTokens int64
	ReasoningTokens  int64

	// Anthropic-specific: geo routing and speed mode
	InferenceGeo string // echoed in response usage.inference_geo
	Speed        string // NOT echoed — read from ctx.RequestBody ($.speed)
}

// providerCalculator is implemented by each provider-specific calculator file.
type providerCalculator interface {
	// Normalize extracts token counts from the raw response (and optionally request)
	// body and returns a normalised Usage struct.
	Normalize(responseBody []byte, requestBody []byte) (Usage, error)

	// Adjust applies any provider-specific post-calculation corrections
	// (e.g. geo/speed multipliers for Anthropic, model-router flat cost for Azure AI)
	// and returns the final cost in USD.
	Adjust(baseCost float64, usage Usage, pricing ModelPricing) float64
}

// selectCalculator returns the appropriate calculator for a given provider value.
func selectCalculator(provider string) providerCalculator {
	switch provider {
	case "anthropic":
		return &AnthropicCalculator{}
	case "gemini",
		"vertex_ai",
		"vertex_ai-language-models",
		"vertex_ai-chat-models",
		"vertex_ai-code-chat-models",
		"vertex_ai-vision-models",
		"vertex_ai-embedding-models":
		return &GeminiCalculator{}
	case "azure", "azure_text":
		return &AzureOpenAICalculator{}
	case "azure_ai":
		return &AzureAICalculator{}
	case "mistral":
		return &MistralCalculator{}
	default: // "openai", "text-completion-openai"
		return &OpenAICalculator{}
	}
}

// lookupPricing finds the ModelPricing entry for a given model name.
// It first tries an exact match, then strips common suffixes (version dates,
// deployment slugs) to find a prefix match.
//
// Returns (pricing, true) on success, (zero, false) if no entry found.
func lookupPricing(modelName string) (ModelPricing, bool) {
	// 1. Exact match
	if p, ok := pricingMap[modelName]; ok {
		return p, true
	}

	// 2. Strip provider-prefix duplicates: some responses echo "openai/gpt-4o"
	//    but the JSON key is "gpt-4o".
	if idx := strings.Index(modelName, "/"); idx != -1 {
		bare := modelName[idx+1:]
		if p, ok := pricingMap[bare]; ok {
			return p, true
		}
	}

	// 3. Progressive suffix stripping: "gpt-4o-2024-11-20" → "gpt-4o-2024-11" → "gpt-4o-2024" → "gpt-4o"
	parts := strings.Split(modelName, "-")
	for i := len(parts) - 1; i >= 1; i-- {
		candidate := strings.Join(parts[:i], "-")
		if p, ok := pricingMap[candidate]; ok {
			return p, true
		}
	}

	return ModelPricing{}, false
}

// genericCalculateCost computes cost in USD from a normalised Usage and ModelPricing.
// It handles:
//   - Context-window tiering (>128k, >200k)
//   - Cached read tokens (discounted rate)
//   - Cache write tokens (creation rate)
//   - Reasoning tokens (separate rate)
//
// This function is provider-agnostic; provider-specific adjustments are done
// in calculator.Adjust() after this call.
func genericCalculateCost(usage Usage, pricing ModelPricing) float64 {
	totalTokens := usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	// Use provider-specific input token count for tier decisions when available.
	// Anthropic defines the 200k threshold as input_tokens + cache tokens (no outputs).
	// Other providers fall back to totalTokens.
	tierTokens := usage.InputTokensForTiering
	if tierTokens == 0 {
		tierTokens = totalTokens
	}

	// Select tiered rates based on total token count.
	inputRate := pricing.InputCostPerToken
	outputRate := pricing.OutputCostPerToken
	cacheReadRate := pricing.CacheReadInputTokenCost
	cacheWriteRate := pricing.CacheCreationInputTokenCost

	switch {
	case tierTokens > 200_000 && pricing.InputCostPerTokenAbove200k > 0:
		inputRate = pricing.InputCostPerTokenAbove200k
		outputRate = pricing.OutputCostPerTokenAbove200k
		{
			cacheReadRate = pricing.CacheReadInputTokenCostAbove200k
		}
		if pricing.CacheCreationInputTokenCostAbove200k > 0 {
			cacheWriteRate = pricing.CacheCreationInputTokenCostAbove200k
		}
	case tierTokens > 128_000 && pricing.InputCostPerTokenAbove128k > 0:
		inputRate = pricing.InputCostPerTokenAbove128k
		outputRate = pricing.OutputCostPerTokenAbove128k
	}

	// Regular (non-cached, non-reasoning) prompt tokens
	regularPromptTokens := usage.PromptTokens - usage.CachedReadTokens - usage.CacheWriteTokens
	if regularPromptTokens < 0 {
		regularPromptTokens = 0
	}

	// Regular (non-reasoning) completion tokens
	regularCompletionTokens := usage.CompletionTokens - usage.ReasoningTokens
	if regularCompletionTokens < 0 {
		regularCompletionTokens = 0
	}

	promptCost := float64(regularPromptTokens) * inputRate
	completionCost := float64(regularCompletionTokens) * outputRate
	cacheReadCost := float64(usage.CachedReadTokens) * cacheReadRate
	cacheWriteCost := float64(usage.CacheWriteTokens) * cacheWriteRate

	// Reasoning tokens billed at their own rate if defined, otherwise at output rate
	reasoningRate := pricing.OutputCostPerReasoningToken
	if reasoningRate == 0 {
		reasoningRate = outputRate
	}
	reasoningCost := float64(usage.ReasoningTokens) * reasoningRate

	return promptCost + completionCost + cacheReadCost + cacheWriteCost + reasoningCost
}
