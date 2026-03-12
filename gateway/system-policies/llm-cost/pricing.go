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

	// Tiered rates — above 272k context window (gpt-5.4, gpt-5.4-pro with 1.05M context)
	InputCostPerTokenAbove272k  float64 `json:"input_cost_per_token_above_272k_tokens"`
	OutputCostPerTokenAbove272k float64 `json:"output_cost_per_token_above_272k_tokens"`

	// ON_DEMAND_PRIORITY service tier rates (Vertex AI Gemini, OpenAI priority).
	// When usageMetadata.trafficType == "ON_DEMAND_PRIORITY" the _priority variants
	// are billed instead of the standard rates.
	InputCostPerTokenPriority                float64 `json:"input_cost_per_token_priority"`
	OutputCostPerTokenPriority               float64 `json:"output_cost_per_token_priority"`
	CacheReadInputTokenCostPriority          float64 `json:"cache_read_input_token_cost_priority"`
	InputCostPerTokenAbove200kPriority       float64 `json:"input_cost_per_token_above_200k_tokens_priority"`
	OutputCostPerTokenAbove200kPriority      float64 `json:"output_cost_per_token_above_200k_tokens_priority"`
	CacheReadInputTokenCostAbove200kPriority float64 `json:"cache_read_input_token_cost_above_200k_tokens_priority"`
	InputCostPerAudioTokenPriority           float64 `json:"input_cost_per_audio_token_priority"`
	InputCostPerTokenAbove272kPriority       float64 `json:"input_cost_per_token_above_272k_tokens_priority"`
	OutputCostPerTokenAbove272kPriority      float64 `json:"output_cost_per_token_above_272k_tokens_priority"`
	CacheReadInputTokenCostAbove272kPriority float64 `json:"cache_read_input_token_cost_above_272k_tokens_priority"`

	// Flex service tier rates (OpenAI flex processing — lower price, higher latency).
	InputCostPerTokenFlex       float64 `json:"input_cost_per_token_flex"`
	OutputCostPerTokenFlex      float64 `json:"output_cost_per_token_flex"`
	CacheReadInputTokenCostFlex float64 `json:"cache_read_input_token_cost_flex"`

	// Prompt caching
	CacheReadInputTokenCost              float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost          float64 `json:"cache_creation_input_token_cost"`
	CacheCreationInputTokenCostAbove1hr  float64 `json:"cache_creation_input_token_cost_above_1hr"`
	CacheReadInputTokenCostAbove200k     float64 `json:"cache_read_input_token_cost_above_200k_tokens"`
	CacheCreationInputTokenCostAbove200k float64 `json:"cache_creation_input_token_cost_above_200k_tokens"`
	CacheReadInputTokenCostAbove272k     float64 `json:"cache_read_input_token_cost_above_272k_tokens"`

	// Cached audio token read rate (Gemini models with separate audio caching cost).
	// When set, cached audio input tokens are billed at this rate instead of
	// the standard CacheReadInputTokenCost.
	CacheReadInputTokenCostPerAudioToken float64 `json:"cache_read_input_token_cost_per_audio_token"`

	// Reasoning tokens (o-series, Claude 3.7+, Gemini thinking)
	OutputCostPerReasoningToken float64 `json:"output_cost_per_reasoning_token"`

	// Batch API discount (OpenAI, Azure)
	InputCostPerTokenBatches  float64 `json:"input_cost_per_token_batches"`
	OutputCostPerTokenBatches float64 `json:"output_cost_per_token_batches"`

	// Modality-specific token rates (Gemini audio/image models)
	InputCostPerAudioToken  float64 `json:"input_cost_per_audio_token"`
	OutputCostPerAudioToken float64 `json:"output_cost_per_audio_token"`
	OutputCostPerImageToken float64 `json:"output_cost_per_image_token"`

	// InputCostPerAudioPerSecond is used for providers (e.g. Mistral Voxtral)
	// that bill audio input by duration rather than by token count. The response
	// includes a prompt_audio_seconds field; cost = seconds × this rate.
	// Maps to the existing input_cost_per_audio_per_second JSON field.
	InputCostPerAudioPerSecond float64 `json:"input_cost_per_audio_per_second"`

	// Non-token pricing units for specialised model types.
	// These are stored for reference and future billing support; current calculators
	// handle them via provider-specific paths rather than generic_calculate_cost.
	InputCostPerCharacter         float64 `json:"input_cost_per_character"`          // TTS models ($/character)
	InputCostPerSecond            float64 `json:"input_cost_per_second"`             // Whisper transcription ($/second)
	OutputCostPerSecond           float64 `json:"output_cost_per_second"`            // Whisper output ($/second)
	InputCostPerImage             float64 `json:"input_cost_per_image"`              // Image generation ($/image)
	InputCostPerPixel             float64 `json:"input_cost_per_pixel"`              // DALL-E pixel-based pricing
	OutputCostPerPixel            float64 `json:"output_cost_per_pixel"`             // DALL-E pixel-based output pricing
	OutputCostPerVideoPerSecond   float64 `json:"output_cost_per_video_per_second"`  // Video generation ($/second)
	CodeInterpreterCostPerSession float64 `json:"code_interpreter_cost_per_session"` // Container/code interpreter ($/session)

	// Built-in web search tool cost (Anthropic, OpenAI).
	// The JSON value is an object keyed by search_context_size: low / medium / high.
	// We decode it as a map and pick the right entry at runtime.
	SearchContextCostPerQuery map[string]float64 `json:"search_context_cost_per_query"`

	// Gemini Live: fixed per-invocation fee for grounding / web search tool calls.
	// When set, any toolUsePromptTokenCount > 0 triggers this flat fee instead of
	// per-token billing.
	WebSearchCostPerRequest float64 `json:"web_search_cost_per_request"`

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

	// InputTokensForTiering is used to decide the pricing tier (>128k, >200k).
	// Anthropic includes all input categories (regular + cache writes + reads);
	// other providers use PromptTokens. Falls back to PromptTokens when zero.
	InputTokensForTiering int64

	// Cached / reasoning tokens.
	// CacheWrite1hrTokens holds 1-hour TTL writes billed at the higher
	// cache_creation_input_token_cost_above_1hr rate; 0 means all writes are 5-min.
	CachedReadTokens    int64
	CacheWriteTokens    int64 // 5-min TTL cache write tokens
	CacheWrite1hrTokens int64 // 1-hr TTL cache write tokens
	ReasoningTokens     int64

	// Modality-specific tokens (Gemini multi-modal models).
	// Audio/image tokens are included in PromptTokens/CompletionTokens;
	// genericCalculateCost re-bills them at their respective modality rates.
	AudioInputTokens  int64
	AudioOutputTokens int64
	ImageOutputTokens int64

	// CachedAudioInputTokens is the subset of CachedReadTokens that are audio.
	// Billed at CacheReadInputTokenCostPerAudioToken when that rate is defined.
	CachedAudioInputTokens int64

	// AudioInputSeconds is audio duration for providers that bill by time (e.g. Mistral Voxtral).
	// Cost = AudioInputSeconds × InputCostPerAudioPerSecond.
	AudioInputSeconds float64

	// ToolUsePromptTokens is the Gemini Live search tool token count.
	// Separate from PromptTokens; billed at WebSearchCostPerRequest or standard input rate.
	ToolUsePromptTokens int64

	// ServiceTier selects rate variants:
	//   "priority" → _priority fields, "flex" → _flex fields, "" → standard.
	ServiceTier string

	// GeminiWebSearchRequests is the grounding query count from candidates[].groundingMetadata.
	// Google AI Studio: $0.035 × N; Vertex AI: $0.035 flat per call.
	GeminiWebSearchRequests int64

	// InferenceGeo and Speed are Anthropic-specific routing fields.
	InferenceGeo string // echoed in response usage.inference_geo
	Speed        string // NOT echoed — read from ctx.RequestBody ($.speed)

	// WebSearchRequests and SearchContextSize are set for built-in web search tool calls.
	// SearchContextSize ("low"/"medium"/"high") comes from the request body.
	WebSearchRequests int64
	SearchContextSize string
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
// It tries: exact match → strip provider prefix → prepend known prefixes → progressive suffix stripping.
// knownProviderPrefixes are namespaces where the API returns bare model names
// but the pricing key is namespaced (e.g. "mistral-large-latest" → "mistral/mistral-large-latest").
var knownProviderPrefixes = []string{
	"mistral/",
	"vertex_ai/",
	"azure_ai/",
	"bedrock/",
}

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

	// 3. Try prepending known provider prefixes. Providers such as Mistral
	//    return bare model names (e.g. "mistral-large-latest") in $.model, but
	//    the pricing JSON stores them under a namespaced key
	//    (e.g. "mistral/mistral-large-latest").
	if !strings.Contains(modelName, "/") {
		for _, prefix := range knownProviderPrefixes {
			if p, ok := pricingMap[prefix+modelName]; ok {
				return p, true
			}
		}
	}

	// 4. Progressive suffix stripping: "gpt-4o-2024-11-20" → "gpt-4o-2024-11" → "gpt-4o-2024" → "gpt-4o"
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
// Handles context-window tiering, service tiers, cache costs, reasoning tokens,
// audio/image modality tokens, and web search fees. Provider-specific adjustments
// are applied in calculator.Adjust() after this call.
func genericCalculateCost(usage Usage, pricing ModelPricing) float64 {
	totalTokens := usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	// Use provider-specific input token count for tier decisions when available.
	// Anthropic defines the 200k threshold as input_tokens + cache tokens (no outputs).
	// All other providers (Gemini, OpenAI, etc.) tier on prompt tokens only.
	tierTokens := usage.InputTokensForTiering
	if tierTokens == 0 {
		tierTokens = usage.PromptTokens
	}

	// Select tiered rates based on total token count.
	inputRate := pricing.InputCostPerToken
	outputRate := pricing.OutputCostPerToken
	cacheReadRate := pricing.CacheReadInputTokenCost
	cacheWrite5mRate := pricing.CacheCreationInputTokenCost
	cacheWrite1hrRate := pricing.CacheCreationInputTokenCostAbove1hr
	if cacheWrite1hrRate == 0 {
		// Fallback: if no distinct 1hr rate is defined, use the standard write rate.
		cacheWrite1hrRate = cacheWrite5mRate
	}

	switch {
	case tierTokens > 272_000 && pricing.InputCostPerTokenAbove272k > 0:
		inputRate = pricing.InputCostPerTokenAbove272k
		outputRate = pricing.OutputCostPerTokenAbove272k
		cacheReadRate = pricing.CacheReadInputTokenCostAbove272k
	case tierTokens > 200_000 && pricing.InputCostPerTokenAbove200k > 0:
		inputRate = pricing.InputCostPerTokenAbove200k
		outputRate = pricing.OutputCostPerTokenAbove200k
		cacheReadRate = pricing.CacheReadInputTokenCostAbove200k
		if pricing.CacheCreationInputTokenCostAbove200k > 0 {
			cacheWrite5mRate = pricing.CacheCreationInputTokenCostAbove200k
			// TODO: if Anthropic ever defines cache_creation_input_token_cost_above_1hr_above_200k_tokens,
			// select it here. For now, the >200k write rate applies to both TTLs.
			cacheWrite1hrRate = pricing.CacheCreationInputTokenCostAbove200k
		}
	case tierTokens > 128_000 && pricing.InputCostPerTokenAbove128k > 0:
		inputRate = pricing.InputCostPerTokenAbove128k
		outputRate = pricing.OutputCostPerTokenAbove128k
	}

	// Service tier override: priority and flex requests use their respective rate variants.
	// Priority tiers are checked from the narrowest threshold downward so that a >272k
	// prompt on a priority tier gets the right compounding rate.
	switch usage.ServiceTier {
	case "priority":
		switch {
		case tierTokens > 272_000 && pricing.InputCostPerTokenAbove272kPriority > 0:
			inputRate = pricing.InputCostPerTokenAbove272kPriority
			if pricing.OutputCostPerTokenAbove272kPriority > 0 {
				outputRate = pricing.OutputCostPerTokenAbove272kPriority
			}
			if pricing.CacheReadInputTokenCostAbove272kPriority > 0 {
				cacheReadRate = pricing.CacheReadInputTokenCostAbove272kPriority
			}
		case tierTokens > 200_000 && pricing.InputCostPerTokenAbove200kPriority > 0:
			inputRate = pricing.InputCostPerTokenAbove200kPriority
			if pricing.OutputCostPerTokenAbove200kPriority > 0 {
				outputRate = pricing.OutputCostPerTokenAbove200kPriority
			}
			if pricing.CacheReadInputTokenCostAbove200kPriority > 0 {
				cacheReadRate = pricing.CacheReadInputTokenCostAbove200kPriority
			}
		case pricing.InputCostPerTokenPriority > 0:
			inputRate = pricing.InputCostPerTokenPriority
			if pricing.OutputCostPerTokenPriority > 0 {
				outputRate = pricing.OutputCostPerTokenPriority
			}
			if pricing.CacheReadInputTokenCostPriority > 0 {
				cacheReadRate = pricing.CacheReadInputTokenCostPriority
			}
		}
	case "flex":
		if pricing.InputCostPerTokenFlex > 0 {
			inputRate = pricing.InputCostPerTokenFlex
			if pricing.OutputCostPerTokenFlex > 0 {
				outputRate = pricing.OutputCostPerTokenFlex
			}
			if pricing.CacheReadInputTokenCostFlex > 0 {
				cacheReadRate = pricing.CacheReadInputTokenCostFlex
			}
		}
	case "batch":
		if pricing.InputCostPerTokenBatches > 0 {
			inputRate = pricing.InputCostPerTokenBatches
			if pricing.OutputCostPerTokenBatches > 0 {
				outputRate = pricing.OutputCostPerTokenBatches
			}
		}
	}

	// Regular (non-cached, non-reasoning) prompt tokens (audio tokens also excluded
	// so they can be billed at their own modality rate below)
	regularPromptTokens := usage.PromptTokens - usage.CachedReadTokens - usage.CacheWriteTokens - usage.CacheWrite1hrTokens - usage.AudioInputTokens
	if regularPromptTokens < 0 {
		regularPromptTokens = 0
	}

	// Regular (non-reasoning, non-audio, non-image) completion tokens
	regularCompletionTokens := usage.CompletionTokens - usage.ReasoningTokens - usage.AudioOutputTokens - usage.ImageOutputTokens
	if regularCompletionTokens < 0 {
		regularCompletionTokens = 0
	}

	promptCost := float64(regularPromptTokens) * inputRate
	completionCost := float64(regularCompletionTokens) * outputRate

	// Cache read cost: when the model defines a per-audio cache rate, split
	// cached tokens by modality; otherwise bill all at cacheReadRate.
	var cacheReadCost float64
	if pricing.CacheReadInputTokenCostPerAudioToken > 0 {
		textCachedTokens := usage.CachedReadTokens - usage.CachedAudioInputTokens
		if textCachedTokens < 0 {
			textCachedTokens = 0
		}
		cacheReadCost = float64(textCachedTokens)*cacheReadRate +
			float64(usage.CachedAudioInputTokens)*pricing.CacheReadInputTokenCostPerAudioToken
	} else {
		cacheReadCost = float64(usage.CachedReadTokens) * cacheReadRate
	}
	cacheWriteCost := float64(usage.CacheWriteTokens)*cacheWrite5mRate + float64(usage.CacheWrite1hrTokens)*cacheWrite1hrRate

	// Reasoning tokens billed at their own rate if defined, otherwise at output rate
	reasoningRate := pricing.OutputCostPerReasoningToken
	if reasoningRate == 0 {
		reasoningRate = outputRate
	}
	reasoningCost := float64(usage.ReasoningTokens) * reasoningRate

	// Audio input rate (falls back to standard input rate when absent)
	audioInputRate := pricing.InputCostPerAudioToken
	if audioInputRate == 0 {
		audioInputRate = inputRate
	}
	// Note: service-tier (_priority) suffix does not apply to audio token rates.
	audioInputCost := float64(usage.AudioInputTokens) * audioInputRate

	audioOutputRate := pricing.OutputCostPerAudioToken
	if audioOutputRate == 0 {
		audioOutputRate = outputRate
	}
	audioOutputCost := float64(usage.AudioOutputTokens) * audioOutputRate

	imageOutputRate := pricing.OutputCostPerImageToken
	if imageOutputRate == 0 {
		imageOutputRate = outputRate
	}
	imageOutputCost := float64(usage.ImageOutputTokens) * imageOutputRate

	// Web search: variable rate keyed by context size, or flat rate per call.
	var webSearchCost float64
	if usage.WebSearchRequests > 0 {
		if len(pricing.SearchContextCostPerQuery) > 0 {
			size := usage.SearchContextSize
			if size == "" {
				size = "medium"
			}
			jsonKey := "search_context_size_" + size
			if rate, ok := pricing.SearchContextCostPerQuery[jsonKey]; ok {
				webSearchCost = float64(usage.WebSearchRequests) * rate
			}
		} else if pricing.WebSearchCostPerRequest > 0 {
			webSearchCost = float64(usage.WebSearchRequests) * pricing.WebSearchCostPerRequest
		}
	}

	// Gemini Live tool-use tokens: flat fee when defined, otherwise standard input rate.
	var toolUseCost float64
	if usage.ToolUsePromptTokens > 0 {
		if pricing.WebSearchCostPerRequest > 0 {
			toolUseCost = pricing.WebSearchCostPerRequest
		} else {
			toolUseCost = float64(usage.ToolUsePromptTokens) * inputRate
		}
	}

	// Audio billed by duration (e.g. Mistral Voxtral).
	audioSecondsCost := usage.AudioInputSeconds * pricing.InputCostPerAudioPerSecond

	return promptCost + completionCost + cacheReadCost + cacheWriteCost + reasoningCost + webSearchCost + toolUseCost + audioInputCost + audioOutputCost + imageOutputCost + audioSecondsCost
}
