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
	"math"
	"os"
	"path/filepath"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const floatTolerance = 1e-12

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= floatTolerance
}

// ---------------------------------------------------------------------------
// Pricing lookup
// ---------------------------------------------------------------------------

func TestLookupPricing_ExactMatch(t *testing.T) {
	p, ok := lookupPricing("gpt-4o-mini-2024-07-18")
	if !ok {
		t.Fatal("expected exact match for gpt-4o-mini-2024-07-18")
	}
	if p.Provider != "openai" {
		t.Errorf("expected provider=openai, got %q", p.Provider)
	}
}

func TestLookupPricing_PrefixFallback(t *testing.T) {
	// "gpt-4o-mini-2024-07-18-custom" is not in the map; should fall back to
	// "gpt-4o-mini-2024-07-18" by progressive suffix stripping.
	p, ok := lookupPricing("gpt-4o-mini-2024-07-18-custom")
	if !ok {
		t.Fatal("expected prefix fallback to succeed")
	}
	if p.Provider != "openai" {
		t.Errorf("expected provider=openai after fallback, got %q", p.Provider)
	}
}

func TestLookupPricing_UnknownModel(t *testing.T) {
	_, ok := lookupPricing("totally-unknown-model-xyz")
	if ok {
		t.Error("expected lookup to fail for unknown model")
	}
}

func TestLookupPricing_ProviderPrefixStrip(t *testing.T) {
	// Responses from some providers echo the model as "openai/gpt-4o-mini"
	p, ok := lookupPricing("openai/gpt-4o-mini")
	if !ok {
		t.Fatal("expected lookup to succeed after stripping provider prefix")
	}
	if p.Provider != "openai" {
		t.Errorf("expected provider=openai, got %q", p.Provider)
	}
}

// ---------------------------------------------------------------------------
// OpenAI calculator
// ---------------------------------------------------------------------------

func TestOpenAICalculator_Normalize(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o-mini-2024-07-18",
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"total_tokens": 150,
			"prompt_tokens_details": {"cached_tokens": 20},
			"completion_tokens_details": {"reasoning_tokens": 10}
		}
	}`)
	c := &OpenAICalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.PromptTokens != 100 || u.CompletionTokens != 50 || u.TotalTokens != 150 {
		t.Errorf("basic token counts wrong: %+v", u)
	}
	if u.CachedReadTokens != 20 {
		t.Errorf("expected CachedReadTokens=20, got %d", u.CachedReadTokens)
	}
	if u.ReasoningTokens != 10 {
		t.Errorf("expected ReasoningTokens=10, got %d", u.ReasoningTokens)
	}
}

func TestOpenAICalculator_Cost_Basic(t *testing.T) {
	// gpt-4o-mini-2024-07-18: input=1.5e-7, output=6e-7
	pricing, _ := lookupPricing("gpt-4o-mini-2024-07-18")
	usage := Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
	cost := genericCalculateCost(usage, pricing)
	// 1000 * 1.5e-7 + 500 * 6e-7 = 0.00015 + 0.00030 = 0.00045
	expected := 1000*1.5e-7 + 500*6e-7
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

func TestOpenAICalculator_Cost_WithCachedTokens(t *testing.T) {
	// gpt-4o-mini: cache_read=7.5e-8
	pricing, _ := lookupPricing("gpt-4o-mini-2024-07-18")
	usage := Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		TotalTokens:      1200,
		CachedReadTokens: 400,
	}
	cost := genericCalculateCost(usage, pricing)
	// regular prompt = 1000-400 = 600 tokens at 1.5e-7
	// cached = 400 at 7.5e-8
	// completion = 200 at 6e-7
	expected := 600*1.5e-7 + 400*7.5e-8 + 200*6e-7
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

func TestOpenAICalculator_Adjust_PassThrough(t *testing.T) {
	c := &OpenAICalculator{}
	if c.Adjust(0.42, Usage{}, ModelPricing{}) != 0.42 {
		t.Error("Adjust should be a pass-through for OpenAI")
	}
}

// ---------------------------------------------------------------------------
// Azure OpenAI calculator
// ---------------------------------------------------------------------------

func TestAzureOpenAICalculator_Cost(t *testing.T) {
	// azure/gpt-4o-mini-2024-07-18: input=1.65e-7, output=6.6e-7
	pricing, ok := lookupPricing("azure/gpt-4o-mini-2024-07-18")
	if !ok {
		t.Skip("azure/gpt-4o-mini-2024-07-18 not in pricing map")
	}
	usage := Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
	cost := genericCalculateCost(usage, pricing)
	expected := 1000*1.65e-7 + 500*6.6e-7
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// ---------------------------------------------------------------------------
// Mistral calculator
// ---------------------------------------------------------------------------

func TestMistralCalculator_Normalize(t *testing.T) {
	body := []byte(`{"model":"mistral/mistral-small-latest","usage":{"prompt_tokens":200,"completion_tokens":100,"total_tokens":300}}`)
	c := &MistralCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.PromptTokens != 200 || u.CompletionTokens != 100 || u.TotalTokens != 300 {
		t.Errorf("unexpected usage: %+v", u)
	}
}

func TestMistralCalculator_Cost(t *testing.T) {
	// mistral/mistral-small-latest: input=6e-8, output=1.8e-7
	pricing, ok := lookupPricing("mistral/mistral-small-latest")
	if !ok {
		t.Skip("mistral/mistral-small-latest not in pricing map")
	}
	usage := Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
	cost := genericCalculateCost(usage, pricing)
	expected := 1000*6e-8 + 500*1.8e-7
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// ---------------------------------------------------------------------------
// Anthropic calculator
// ---------------------------------------------------------------------------

func TestAnthropicCalculator_Normalize(t *testing.T) {
	respBody := []byte(`{
		"model": "claude-3-5-haiku-20241022",
		"usage": {
			"input_tokens": 300,
			"output_tokens": 150,
			"cache_creation_input_tokens": 50,
			"cache_read_input_tokens": 100,
			"inference_geo": "us"
		}
	}`)
	reqBody := []byte(`{"model":"claude-3-5-haiku-20241022","speed":"fast"}`)
	c := &AnthropicCalculator{}
	u, err := c.Normalize(respBody, reqBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.PromptTokens != 450 || u.CompletionTokens != 150 {
		// PromptTokens = input_tokens(300) + cache_creation(50) + cache_read(100) = 450
		// genericCalculateCost subtracts cache buckets to recover regular input count.
		t.Errorf("wrong token counts: %+v", u)
	}
	if u.TotalTokens != 450 {
		t.Errorf("expected TotalTokens=450, got %d", u.TotalTokens)
	}
	if u.CacheWriteTokens != 50 {
		t.Errorf("expected CacheWriteTokens=50, got %d", u.CacheWriteTokens)
	}
	if u.CachedReadTokens != 100 {
		t.Errorf("expected CachedReadTokens=100, got %d", u.CachedReadTokens)
	}
	if u.InferenceGeo != "us" {
		t.Errorf("expected InferenceGeo=us, got %q", u.InferenceGeo)
	}
	if u.Speed != "fast" {
		t.Errorf("expected Speed=fast, got %q", u.Speed)
	}
}

func TestAnthropicCalculator_Normalize_SpeedMissingFromRequest(t *testing.T) {
	respBody := []byte(`{"model":"claude-3-5-haiku-20241022","usage":{"input_tokens":100,"output_tokens":50}}`)
	c := &AnthropicCalculator{}
	u, err := c.Normalize(respBody, nil) // no request body
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Speed != "" {
		t.Errorf("expected Speed to be empty without request body, got %q", u.Speed)
	}
}

func TestAnthropicCalculator_Cost_Basic(t *testing.T) {
	// claude-3-5-haiku: input=8e-7, output=4e-6
	pricing, _ := lookupPricing("claude-3-5-haiku-20241022")
	usage := Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
	cost := genericCalculateCost(usage, pricing)
	expected := 1000*8e-7 + 500*4e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

func TestAnthropicCalculator_Cost_WithCacheTokens(t *testing.T) {
	// claude-3-5-haiku: input=8e-7, cache_read=8e-8, cache_write=1e-6, output=4e-6
	pricing, _ := lookupPricing("claude-3-5-haiku-20241022")
	usage := Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		TotalTokens:      1200,
		CachedReadTokens: 300,
		CacheWriteTokens: 100,
	}
	cost := genericCalculateCost(usage, pricing)
	regularPrompt := int64(1000 - 300 - 100) // 600
	expected := float64(regularPrompt)*8e-7 + 300*8e-8 + 100*1e-6 + 200*4e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

func TestAnthropicCalculator_Cost_LongContextTiering(t *testing.T) {
	// claude-sonnet-4-6: ≤200k in=3e-6, out=1.5e-5; >200k in=6e-6, out=2.25e-5
	pricing, ok := lookupPricing("claude-sonnet-4-6")
	if !ok {
		t.Skip("claude-sonnet-4-6 not in pricing map")
	}

	// 150k input_tokens + 100k cache_read = 250k total input → should hit >200k tier.
	// Output tokens (5k) must NOT affect tier selection per Anthropic's definition.
	usage := Usage{
		PromptTokens:          150_000,
		CompletionTokens:      5_000,
		TotalTokens:           155_000,
		CachedReadTokens:      100_000,
		InputTokensForTiering: 250_000, // 150k input + 100k cache_read
	}
	cost := genericCalculateCost(usage, pricing)

	// At >200k rates: in=6e-6, out=2.25e-5, cache_read_above200k=6e-7
	regularPrompt := int64(150_000 - 100_000) // 50k regular prompt tokens
	expected := float64(regularPrompt)*6e-6 + float64(100_000)*6e-7 + float64(5_000)*2.25e-5
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}

	// Sanity check: same usage WITHOUT InputTokensForTiering set should use standard rates.
	usageStd := Usage{
		PromptTokens:     150_000,
		CompletionTokens: 5_000,
		TotalTokens:      155_000,
		CachedReadTokens: 100_000,
	}
	costStd := genericCalculateCost(usageStd, pricing)
	expectedStd := float64(50_000)*3e-6 + float64(100_000)*3e-7 + float64(5_000)*1.5e-5
	if !almostEqual(costStd, expectedStd) {
		t.Errorf("standard tier: expected %.10f, got %.10f", expectedStd, costStd)
	}
}

func TestAnthropicCalculator_Normalize_SetsInputTokensForTiering(t *testing.T) {
	// Verify that Normalize correctly sets InputTokensForTiering to
	// input_tokens + cache_creation_input_tokens + cache_read_input_tokens.
	body := []byte(`{
		"usage": {
			"input_tokens": 150000,
			"output_tokens": 5000,
			"cache_creation_input_tokens": 20000,
			"cache_read_input_tokens": 80000
		}
	}`)
	c := &AnthropicCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	// 150000 + 20000 + 80000 = 250000
	if u.InputTokensForTiering != 250_000 {
		t.Errorf("InputTokensForTiering: got %d, want 250000", u.InputTokensForTiering)
	}
}

func TestAnthropicCalculator_Adjust_GeoAndSpeed(t *testing.T) {
	// claude-opus-4-6 has provider_specific_entry: {us: 1.1, fast: 6.0}
	pricing, ok := lookupPricing("claude-opus-4-6")
	if !ok {
		t.Skip("claude-opus-4-6 not in pricing map")
	}

	usage := Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
		InferenceGeo:     "us",
		Speed:            "fast",
	}
	baseCost := genericCalculateCost(usage, pricing)

	c := &AnthropicCalculator{}
	finalCost := c.Adjust(baseCost, usage, pricing)

	// multiplier = 1.1 * 6.0 = 6.6 applied only to non-cache cost
	// (no cache tokens here, so finalCost = baseCost * 6.6)
	expected := baseCost * 6.6
	if !almostEqual(finalCost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, finalCost)
	}
}

func TestAnthropicCalculator_Adjust_CacheCarveOut(t *testing.T) {
	// Cache costs must NOT be multiplied by geo/speed factors.
	pricing, ok := lookupPricing("claude-opus-4-6")
	if !ok {
		t.Skip("claude-opus-4-6 not in pricing map")
	}

	usage := Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
		CachedReadTokens: 200,
		InferenceGeo:     "us", // multiplier = 1.1
	}
	baseCost := genericCalculateCost(usage, pricing)
	cacheCost := 200 * pricing.CacheReadInputTokenCost

	c := &AnthropicCalculator{}
	finalCost := c.Adjust(baseCost, usage, pricing)

	// Only (baseCost - cacheCost) is multiplied by 1.1; cacheCost is added back as-is.
	expected := (baseCost-cacheCost)*1.1 + cacheCost
	if !almostEqual(finalCost, expected) {
		t.Errorf("expected %.10f (cache carve-out), got %.10f", expected, finalCost)
	}
}

func TestAnthropicCalculator_Adjust_GlobalGeo_NoMultiplier(t *testing.T) {
	pricing, ok := lookupPricing("claude-opus-4-6")
	if !ok {
		t.Skip("claude-opus-4-6 not in pricing map")
	}
	usage := Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500, InferenceGeo: "global"}
	baseCost := genericCalculateCost(usage, pricing)
	c := &AnthropicCalculator{}
	if c.Adjust(baseCost, usage, pricing) != baseCost {
		t.Error("global geo should not apply any multiplier")
	}
}

func TestAnthropicCalculator_Adjust_NoProviderSpecificEntry(t *testing.T) {
	// Model with no provider_specific_entry — Adjust must be a pass-through.
	pricing, _ := lookupPricing("claude-3-5-haiku-20241022")
	usage := Usage{PromptTokens: 100, CompletionTokens: 50, InferenceGeo: "us", Speed: "fast"}
	baseCost := genericCalculateCost(usage, pricing)
	c := &AnthropicCalculator{}
	if c.Adjust(baseCost, usage, pricing) != baseCost {
		t.Error("Adjust should pass through when no provider_specific_entry is present")
	}
}

func TestAnthropicCalculator_Normalize_CacheWrite5mAnd1hr(t *testing.T) {
	// When the response includes usage.cache_creation with per-TTL breakdown,
	// Normalize must split them into CacheWriteTokens (5m) and CacheWrite1hrTokens (1hr).
	body := []byte(`{
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"cache_creation_input_tokens": 1200,
			"cache_read_input_tokens": 0,
			"cache_creation": {
				"ephemeral_5m_input_tokens": 200,
				"ephemeral_1h_input_tokens": 1000
			}
		}
	}`)
	c := &AnthropicCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if u.CacheWriteTokens != 200 {
		t.Errorf("CacheWriteTokens (5m): got %d, want 200", u.CacheWriteTokens)
	}
	if u.CacheWrite1hrTokens != 1000 {
		t.Errorf("CacheWrite1hrTokens (1hr): got %d, want 1000", u.CacheWrite1hrTokens)
	}
}

func TestAnthropicCalculator_Normalize_CacheWriteFallback_No1hrBreakdown(t *testing.T) {
	// When cache_creation sub-object is absent, all writes go into CacheWriteTokens (5m).
	body := []byte(`{
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"cache_creation_input_tokens": 300,
			"cache_read_input_tokens": 0
		}
	}`)
	c := &AnthropicCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if u.CacheWriteTokens != 300 {
		t.Errorf("CacheWriteTokens: got %d, want 300", u.CacheWriteTokens)
	}
	if u.CacheWrite1hrTokens != 0 {
		t.Errorf("CacheWrite1hrTokens: got %d, want 0", u.CacheWrite1hrTokens)
	}
}

func TestAnthropicCalculator_Cost_Mixed5mAnd1hrCacheWrites(t *testing.T) {
	// Verify that 5m and 1hr cache write tokens are billed at their respective rates.
	// claude-opus-4-6: cache_creation_input_token_cost=6.25e-6, above_1hr=1e-5
	pricing, ok := lookupPricing("claude-opus-4-6")
	if !ok {
		t.Skip("claude-opus-4-6 not in pricing map")
	}

	usage := Usage{
		PromptTokens:        100,
		CompletionTokens:    50,
		TotalTokens:         150,
		CacheWriteTokens:    200,  // 5-minute TTL
		CacheWrite1hrTokens: 1000, // 1-hour TTL
	}
	cost := genericCalculateCost(usage, pricing)

	// 100 regular input tokens (prompt - cache writes = 100 - 200 - 1000 < 0, clamped to 0)
	// 50 output tokens
	// 200 × 6.25e-6 (5m write) + 1000 × 1e-5 (1hr write)
	expected5m := 200 * pricing.CacheCreationInputTokenCost
	expected1hr := 1000 * pricing.CacheCreationInputTokenCostAbove1hr
	expectedOutput := 50 * pricing.OutputCostPerToken
	// regularPrompt clamped to 0 (100 - 200 - 1000 < 0)
	expectedTotal := expected5m + expected1hr + expectedOutput
	if !almostEqual(cost, expectedTotal) {
		t.Errorf("expected %.10f, got %.10f (5m=%.10f, 1hr=%.10f, output=%.10f)",
			expectedTotal, cost, expected5m, expected1hr, expectedOutput)
	}
}

func TestAnthropicCalculator_Normalize_WebSearchRequests(t *testing.T) {
	// Verify that web_search_requests is read from usage.server_tool_use
	// and search_context_size is read from the request body.
	responseBody := []byte(`{
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"server_tool_use": {
				"web_search_requests": 3
			}
		}
	}`)
	requestBody := []byte(`{
		"model": "claude-opus-4-6",
		"web_search_options": {"search_context_size": "high"}
	}`)
	c := &AnthropicCalculator{}
	u, err := c.Normalize(responseBody, requestBody)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if u.WebSearchRequests != 3 {
		t.Errorf("WebSearchRequests: got %d, want 3", u.WebSearchRequests)
	}
	if u.SearchContextSize != "high" {
		t.Errorf("SearchContextSize: got %q, want \"high\"", u.SearchContextSize)
	}
}

func TestAnthropicCalculator_Normalize_WebSearch_DefaultsToMedium(t *testing.T) {
	// When web_search_options is absent, SearchContextSize should be empty
	// so that genericCalculateCost defaults to "medium".
	responseBody := []byte(`{
		"usage": {
			"input_tokens": 10,
			"output_tokens": 5,
			"server_tool_use": {"web_search_requests": 1}
		}
	}`)
	c := &AnthropicCalculator{}
	u, err := c.Normalize(responseBody, nil)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if u.WebSearchRequests != 1 {
		t.Errorf("WebSearchRequests: got %d, want 1", u.WebSearchRequests)
	}
	if u.SearchContextSize != "" {
		t.Errorf("SearchContextSize: got %q, want empty (defaults to medium at calc time)", u.SearchContextSize)
	}
}

func TestAnthropicCalculator_Cost_WebSearch_MediumDefault(t *testing.T) {
	// When SearchContextSize is empty, genericCalculateCost should default to "medium".
	// claude-opus-4-6: search_context_cost_per_query.medium = 0.01
	pricing, ok := lookupPricing("claude-opus-4-6")
	if !ok {
		t.Skip("claude-opus-4-6 not in pricing map")
	}
	if len(pricing.SearchContextCostPerQuery) == 0 {
		t.Skip("no search_context_cost_per_query for claude-opus-4-6")
	}

	usage := Usage{
		PromptTokens:      100,
		CompletionTokens:  50,
		TotalTokens:       150,
		WebSearchRequests: 2,
		SearchContextSize: "", // should default to medium
	}
	cost := genericCalculateCost(usage, pricing)

	tokenCost := float64(100)*pricing.InputCostPerToken + float64(50)*pricing.OutputCostPerToken
	mediumRate := pricing.SearchContextCostPerQuery["search_context_size_medium"]
	expected := tokenCost + 2*mediumRate
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f (2 × medium=%.4f), got %.10f", expected, mediumRate, cost)
	}
}

func TestAnthropicCalculator_Cost_WebSearch_HighContextSize(t *testing.T) {
	// When SearchContextSize is "high", the high rate should be used.
	// claude-opus-4-6: search_context_cost_per_query.high = 0.01 (same tier for Anthropic)
	pricing, ok := lookupPricing("claude-opus-4-6")
	if !ok {
		t.Skip("claude-opus-4-6 not in pricing map")
	}
	if len(pricing.SearchContextCostPerQuery) == 0 {
		t.Skip("no search_context_cost_per_query for claude-opus-4-6")
	}

	usage := Usage{
		PromptTokens:      50,
		CompletionTokens:  20,
		TotalTokens:       70,
		WebSearchRequests: 5,
		SearchContextSize: "high",
	}
	cost := genericCalculateCost(usage, pricing)

	tokenCost := float64(50)*pricing.InputCostPerToken + float64(20)*pricing.OutputCostPerToken
	highRate := pricing.SearchContextCostPerQuery["search_context_size_high"]
	expected := tokenCost + 5*highRate
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f (5 × high=%.4f), got %.10f", expected, highRate, cost)
	}
}

func TestAnthropicCalculator_Cost_WebSearch_ZeroRequests(t *testing.T) {
	// When WebSearchRequests is 0, no search cost should be added.
	pricing, ok := lookupPricing("claude-opus-4-6")
	if !ok {
		t.Skip("claude-opus-4-6 not in pricing map")
	}

	usageWithSearch := Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, WebSearchRequests: 0}
	usageNoSearch := Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150}
	if genericCalculateCost(usageWithSearch, pricing) != genericCalculateCost(usageNoSearch, pricing) {
		t.Error("zero WebSearchRequests should not add any cost")
	}
}

func TestGeminiCalculator_Normalize(t *testing.T) {
	// INCLUSIVE case: promptTokenCount + candidatesTokenCount == totalTokenCount
	// meaning candidatesTokenCount already contains thinking tokens.
	body := []byte(`{
		"model": "gemini-2.0-flash",
		"usageMetadata": {
			"promptTokenCount": 400,
			"candidatesTokenCount": 200,
			"totalTokenCount": 600,
			"thoughtsTokenCount": 30,
			"cachedContentTokenCount": 50
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Inclusive: CompletionTokens stays at 200 (thinking already included)
	if u.PromptTokens != 400 || u.CompletionTokens != 200 || u.TotalTokens != 600 {
		t.Errorf("wrong token counts: %+v", u)
	}
	if u.ReasoningTokens != 30 {
		t.Errorf("expected ReasoningTokens=30, got %d", u.ReasoningTokens)
	}
	if u.CachedReadTokens != 50 {
		t.Errorf("expected CachedReadTokens=50, got %d", u.CachedReadTokens)
	}
}

// TestGeminiCalculator_Normalize_ThinkingExclusive verifies the exclusive case where
// candidatesTokenCount does NOT include thinking tokens (Gemini 2.5 series behaviour).
// In this case totalTokenCount = promptTokenCount + candidatesTokenCount + thoughtsTokenCount.
// We must add reasoning tokens to CompletionTokens so genericCalculateCost can subtract
// them correctly and still arrive at the right text output count.
func TestGeminiCalculator_Normalize_ThinkingExclusive(t *testing.T) {
	// prompt=100, candidates=100 (text only), thoughts=50, total=250
	// 100+100 != 250 → exclusive
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 100,
			"candidatesTokenCount": 100,
			"totalTokenCount": 250,
			"thoughtsTokenCount": 50
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Exclusive: CompletionTokens must be adjusted to candidates + thoughts = 150
	if u.CompletionTokens != 150 {
		t.Errorf("exclusive: expected CompletionTokens=150 (100 text + 50 thinking), got %d", u.CompletionTokens)
	}
	if u.ReasoningTokens != 50 {
		t.Errorf("expected ReasoningTokens=50, got %d", u.ReasoningTokens)
	}
	// After genericCalculateCost subtracts: regularOutput = 150 - 50 = 100 (correct text count)
}

// TestGeminiCalculator_Cost_ThinkingExclusive verifies that an exclusive-mode thinking
// response is billed correctly: text output tokens at output rate, thinking at reasoning rate.
func TestGeminiCalculator_Cost_ThinkingExclusive(t *testing.T) {
	pricing, ok := lookupPricing("gemini/gemini-2.5-flash")
	if !ok {
		t.Skip("gemini/gemini-2.5-flash not in pricing map")
	}
	// Exclusive: 100 text output, 50 thinking — CompletionTokens adjusted to 150
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 150, // candidates(100) + thoughts(50) after exclusive adjustment
		TotalTokens:      250,
		ReasoningTokens:  50,
	}
	cost := genericCalculateCost(usage, pricing)
	// regularOutput = 150 - 50 = 100 text tokens at output rate
	// reasoning = 50 tokens at reasoning rate (= output rate for this model)
	expectedOutputRate := pricing.OutputCostPerToken
	expected := float64(100)*pricing.InputCostPerToken +
		float64(100)*expectedOutputRate +
		float64(50)*expectedOutputRate
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_Normalize_ModelVersion verifies that Gemini responses using
// the $.modelVersion field (instead of $.model) are correctly handled by lookupPricing.
func TestGeminiCalculator_Normalize_ModelVersion(t *testing.T) {
	// Gemini native API responses carry "modelVersion" not "model".
	body := []byte(`{
		"modelVersion": "gemini-2.0-flash",
		"usageMetadata": {
			"promptTokenCount": 300,
			"candidatesTokenCount": 100,
			"totalTokenCount": 400
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.PromptTokens != 300 || u.CompletionTokens != 100 || u.TotalTokens != 400 {
		t.Errorf("wrong token counts when using modelVersion field: %+v", u)
	}
	// Verify lookupPricing works with the modelVersion value.
	_, found := lookupPricing("gemini-2.0-flash")
	if !found {
		t.Skip("gemini-2.0-flash not in pricing map — skipping lookup assertion")
	}
}

func TestGeminiCalculator_Cost_BelowTier(t *testing.T) {
	// gemini-1.5-flash: input=7.5e-8, output=3e-7 (below 128k)
	pricing, ok := lookupPricing("gemini/gemini-1.5-flash")
	if !ok {
		t.Skip("gemini/gemini-1.5-flash not in pricing map")
	}
	usage := Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
	cost := genericCalculateCost(usage, pricing)
	expected := 1000*7.5e-8 + 500*3e-7
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

func TestGeminiCalculator_Cost_Above128k(t *testing.T) {
	// gemini-1.5-flash: tiering is triggered by PROMPT tokens > 128k (not total).
	// prompt=150k > 128k → above-128k rates apply.
	pricing, ok := lookupPricing("gemini/gemini-1.5-flash")
	if !ok {
		t.Skip("gemini/gemini-1.5-flash not in pricing map")
	}
	usage := Usage{PromptTokens: 150_000, CompletionTokens: 50_000, TotalTokens: 200_000}
	cost := genericCalculateCost(usage, pricing)
	expected := 150_000*1.5e-7 + 50_000*6e-7
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f (above-128k rate), got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_Cost_TotalAbove128k_PromptBelow verifies the key distinction:
// tiering uses PROMPT tokens only. When prompt < 128k the base rate applies even
// if total tokens > 128k. This was a bug before (total was used as the threshold).
func TestGeminiCalculator_Cost_TotalAbove128k_PromptBelow(t *testing.T) {
	// prompt=80k < 128k, completion=60k → total=140k > 128k — base rates must apply.
	pricing, ok := lookupPricing("gemini/gemini-1.5-flash")
	if !ok {
		t.Skip("gemini/gemini-1.5-flash not in pricing map")
	}
	usage := Usage{PromptTokens: 80_000, CompletionTokens: 60_000, TotalTokens: 140_000}
	cost := genericCalculateCost(usage, pricing)
	// Base rates (prompt below 128k threshold)
	expected := 80_000*7.5e-8 + 60_000*3e-7
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f (base rate, prompt below 128k), got %.10f", expected, cost)
	}
	// Sanity: tiered rate would give a different result
	tieredWrong := 80_000*1.5e-7 + 60_000*6e-7
	if almostEqual(cost, tieredWrong) {
		t.Errorf("got tiered rate when prompt is below threshold — regression!")
	}
}

func TestGeminiCalculator_Cost_Above200k(t *testing.T) {
	// gemini-2.5-pro: tiering is triggered by PROMPT tokens > 200k.
	// prompt=210k > 200k → above-200k rates apply.
	pricing, ok := lookupPricing("gemini/gemini-2.5-pro")
	if !ok {
		t.Skip("gemini/gemini-2.5-pro not in pricing map")
	}
	usage := Usage{PromptTokens: 210_000, CompletionTokens: 50_000, TotalTokens: 260_000}
	cost := genericCalculateCost(usage, pricing)
	expected := 210_000*2.5e-6 + 50_000*1.5e-5
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f (above-200k rate), got %.10f", expected, cost)
	}
}

func TestGeminiCalculator_Adjust_PassThrough(t *testing.T) {
	c := &GeminiCalculator{}
	// No grounding queries → cost passes through unchanged
	if c.Adjust(1.23, Usage{GeminiWebSearchRequests: 0}, ModelPricing{Provider: "gemini"}) != 1.23 {
		t.Error("Adjust with no grounding queries should be a pass-through")
	}
}

// TestGeminiCalculator_Normalize_AudioInput verifies that promptTokensDetails is
// parsed and audio input tokens are separated from regular text tokens.
func TestGeminiCalculator_Normalize_AudioInput(t *testing.T) {
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 500,
			"candidatesTokenCount": 100,
			"totalTokenCount": 600,
			"promptTokensDetails": [
				{"modality": "TEXT",  "tokenCount": 300},
				{"modality": "AUDIO", "tokenCount": 200}
			]
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.AudioInputTokens != 200 {
		t.Errorf("expected AudioInputTokens=200, got %d", u.AudioInputTokens)
	}
	if u.PromptTokens != 500 {
		t.Errorf("expected PromptTokens=500 (total prompt count unchanged), got %d", u.PromptTokens)
	}
}

// TestGeminiCalculator_Normalize_AudioInput_CachedSubtracted verifies that
// cached audio tokens (from cacheTokensDetails) are subtracted so only
// non-cached audio tokens are billed at the audio rate.
func TestGeminiCalculator_Normalize_AudioInput_CachedSubtracted(t *testing.T) {
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 500,
			"candidatesTokenCount": 100,
			"totalTokenCount": 600,
			"promptTokensDetails": [
				{"modality": "TEXT",  "tokenCount": 300},
				{"modality": "AUDIO", "tokenCount": 200}
			],
			"cacheTokensDetails": [
				{"modality": "AUDIO", "tokenCount": 50}
			]
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 200 total audio - 50 cached = 150 billable at audio rate
	if u.AudioInputTokens != 150 {
		t.Errorf("expected AudioInputTokens=150 (200-50 cached), got %d", u.AudioInputTokens)
	}
}

// TestGeminiCalculator_Normalize_AudioOutput verifies parsing of candidatesTokensDetails
// for native audio output models (e.g. gemini-2.0-flash-live).
func TestGeminiCalculator_Normalize_AudioOutput(t *testing.T) {
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 100,
			"candidatesTokenCount": 300,
			"totalTokenCount": 400,
			"candidatesTokensDetails": [
				{"modality": "TEXT",  "tokenCount": 100},
				{"modality": "AUDIO", "tokenCount": 200}
			]
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.AudioOutputTokens != 200 {
		t.Errorf("expected AudioOutputTokens=200, got %d", u.AudioOutputTokens)
	}
}

// TestGeminiCalculator_Normalize_ImageOutput verifies parsing of candidatesTokensDetails
// for image generation models (e.g. gemini-2.5-flash-image).
func TestGeminiCalculator_Normalize_ImageOutput(t *testing.T) {
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 200,
			"candidatesTokenCount": 350,
			"totalTokenCount": 550,
			"candidatesTokensDetails": [
				{"modality": "TEXT",  "tokenCount": 50},
				{"modality": "IMAGE", "tokenCount": 300}
			]
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ImageOutputTokens != 300 {
		t.Errorf("expected ImageOutputTokens=300, got %d", u.ImageOutputTokens)
	}
}

// TestGeminiCalculator_Cost_AudioInput verifies that audio input tokens are billed
// at the model's audio rate and excluded from the standard input rate.
func TestGeminiCalculator_Cost_AudioInput(t *testing.T) {
	// gemini-2.5-flash has input_cost_per_token and input_cost_per_audio_token
	pricing, ok := lookupPricing("gemini/gemini-2.5-flash")
	if !ok {
		t.Skip("gemini/gemini-2.5-flash not in pricing map")
	}
	if pricing.InputCostPerAudioToken == 0 {
		t.Skip("gemini/gemini-2.5-flash has no audio input rate")
	}

	// 300 text input + 200 audio input, 100 output
	usage := Usage{
		PromptTokens:     500,
		CompletionTokens: 100,
		TotalTokens:      600,
		AudioInputTokens: 200,
	}
	cost := genericCalculateCost(usage, pricing)

	// 300 text tokens at standard rate + 200 audio at audio rate + 100 output at output rate
	expected := float64(300)*pricing.InputCostPerToken +
		float64(200)*pricing.InputCostPerAudioToken +
		float64(100)*pricing.OutputCostPerToken
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_Cost_AudioOutput verifies that audio output tokens are billed
// at the model's audio output rate.
func TestGeminiCalculator_Cost_AudioOutput(t *testing.T) {
	// gemini-2.0-flash-live-preview has output_cost_per_audio_token
	pricing, ok := lookupPricing("gemini-2.0-flash-live-preview-04-09")
	if !ok {
		t.Skip("gemini-2.0-flash-live-preview-04-09 not in pricing map")
	}
	if pricing.OutputCostPerAudioToken == 0 {
		t.Skip("model has no audio output rate")
	}

	// 100 text output + 200 audio output
	usage := Usage{
		PromptTokens:      100,
		CompletionTokens:  300,
		TotalTokens:       400,
		AudioOutputTokens: 200,
	}
	cost := genericCalculateCost(usage, pricing)

	// 100 prompt at input rate + 100 text output at output rate + 200 audio output at audio output rate
	expected := float64(100)*pricing.InputCostPerToken +
		float64(100)*pricing.OutputCostPerToken +
		float64(200)*pricing.OutputCostPerAudioToken
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_Cost_ImageOutput verifies that image output tokens are billed
// at the model's image output rate.
func TestGeminiCalculator_Cost_ImageOutput(t *testing.T) {
	pricing, ok := lookupPricing("gemini/gemini-2.5-flash-image")
	if !ok {
		t.Skip("gemini/gemini-2.5-flash-image not in pricing map")
	}
	if pricing.OutputCostPerImageToken == 0 {
		t.Skip("model has no image output rate")
	}

	// 200 prompt, 50 text output + 300 image output
	usage := Usage{
		PromptTokens:      200,
		CompletionTokens:  350,
		TotalTokens:       550,
		ImageOutputTokens: 300,
	}
	cost := genericCalculateCost(usage, pricing)

	// 200 prompt at input rate + 50 text output at output rate + 300 image at image output rate
	expected := float64(200)*pricing.InputCostPerToken +
		float64(50)*pricing.OutputCostPerToken +
		float64(300)*pricing.OutputCostPerImageToken
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_Normalize_ResponseTokensDetails verifies Gemini Live's
// responseTokensDetails (streaming audio) is parsed and audio output set.
func TestGeminiCalculator_Normalize_ResponseTokensDetails(t *testing.T) {
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 100,
			"candidatesTokenCount": 200,
			"responseTokenCount": 250,
			"totalTokenCount": 350,
			"responseTokensDetails": [
				{"modality": "TEXT",  "tokenCount": 50},
				{"modality": "AUDIO", "tokenCount": 200}
			]
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// responseTokenCount wins over candidatesTokenCount for Gemini Live
	if u.CompletionTokens != 250 {
		t.Errorf("expected CompletionTokens=250 (from responseTokenCount), got %d", u.CompletionTokens)
	}
	if u.AudioOutputTokens != 200 {
		t.Errorf("expected AudioOutputTokens=200 (from responseTokensDetails), got %d", u.AudioOutputTokens)
	}
}

// TestGeminiCalculator_Normalize_ToolUsePromptTokens verifies that
// toolUsePromptTokenCount from Gemini Live sessions is parsed into ToolUsePromptTokens.
func TestGeminiCalculator_Normalize_ToolUsePromptTokens(t *testing.T) {
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 100,
			"candidatesTokenCount": 50,
			"totalTokenCount": 150,
			"toolUsePromptTokenCount": 25
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ToolUsePromptTokens != 25 {
		t.Errorf("expected ToolUsePromptTokens=25, got %d", u.ToolUsePromptTokens)
	}
	// Normal prompt/completion tokens are unaffected
	if u.PromptTokens != 100 || u.CompletionTokens != 50 {
		t.Errorf("unexpected base token counts: %+v", u)
	}
}

// TestGeminiCalculator_Normalize_ToolUsePromptTokensDetails verifies that
// toolUsePromptTokensDetails is parsed without error and does NOT alter the
// billing calculation — all tool-use tokens are billed as a unit via
// ToolUsePromptTokens (matching LiteLLM's behaviour).
func TestGeminiCalculator_Normalize_ToolUsePromptTokensDetails(t *testing.T) {
	// Response with both toolUsePromptTokenCount and its per-modality breakdown.
	// The billing should use only the aggregate count (25), not split by modality.
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 100,
			"candidatesTokenCount": 50,
			"totalTokenCount": 150,
			"toolUsePromptTokenCount": 25,
			"toolUsePromptTokensDetails": [
				{"modality": "TEXT",  "tokenCount": 15},
				{"modality": "AUDIO", "tokenCount": 10}
			]
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Aggregate count is preserved; per-modality breakdown doesn't split the value
	if u.ToolUsePromptTokens != 25 {
		t.Errorf("expected ToolUsePromptTokens=25, got %d", u.ToolUsePromptTokens)
	}
	// Base token counts are unaffected
	if u.PromptTokens != 100 || u.CompletionTokens != 50 {
		t.Errorf("unexpected base token counts: %+v", u)
	}
}
func TestGeminiCalculator_Cost_ToolUse_TokenFallback(t *testing.T) {
	pricing, ok := lookupPricing("gemini/gemini-2.0-flash")
	if !ok {
		t.Skip("gemini/gemini-2.0-flash not in pricing map")
	}
	// Confirm no fixed web search fee on this model
	if pricing.WebSearchCostPerRequest != 0 {
		t.Skip("model has WebSearchCostPerRequest set — fixed-fee path not tested here")
	}

	usage := Usage{
		PromptTokens:        100,
		CompletionTokens:    50,
		TotalTokens:         150,
		ToolUsePromptTokens: 25,
	}
	cost := genericCalculateCost(usage, pricing)

	// Expected: (100 prompt + 25 tool) * inputRate + 50 * outputRate
	expected := float64(100)*pricing.InputCostPerToken +
		float64(50)*pricing.OutputCostPerToken +
		float64(25)*pricing.InputCostPerToken
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_Cost_ToolUse_FixedFee verifies that when WebSearchCostPerRequest
// is set in pricing, a flat fee is charged instead of per-token billing.
func TestGeminiCalculator_Cost_ToolUse_FixedFee(t *testing.T) {
	pricing := ModelPricing{
		InputCostPerToken:       1e-6,
		OutputCostPerToken:      2e-6,
		WebSearchCostPerRequest: 0.01,
	}
	usage := Usage{
		PromptTokens:        100,
		CompletionTokens:    50,
		TotalTokens:         150,
		ToolUsePromptTokens: 25,
	}
	cost := genericCalculateCost(usage, pricing)

	// Tool tokens billed as flat fee, NOT per-token
	expected := float64(100)*1e-6 + float64(50)*2e-6 + 0.01
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
	// Sanity: per-token billing would have given a different (much smaller) result
	tokenBased := float64(100)*1e-6 + float64(50)*2e-6 + float64(25)*1e-6
	if almostEqual(cost, tokenBased) {
		t.Errorf("expected flat fee to differ from per-token billing")
	}
}

// TestGeminiCalculator_Normalize_TrafficType verifies that trafficType values
// are correctly mapped to ServiceTier.
func TestGeminiCalculator_Normalize_TrafficType(t *testing.T) {
	tests := []struct {
		trafficType  string
		expectedTier string
	}{
		{"ON_DEMAND_PRIORITY", "priority"},
		{"FLEX", "flex"},
		{"BATCH", "flex"},
		{"ON_DEMAND", ""},
		{"", ""},
	}
	for _, tc := range tests {
		body, _ := json.Marshal(map[string]any{
			"usageMetadata": map[string]any{
				"promptTokenCount":     100,
				"candidatesTokenCount": 50,
				"totalTokenCount":      150,
				"trafficType":          tc.trafficType,
			},
		})
		c := &GeminiCalculator{}
		u, err := c.Normalize(body, nil)
		if err != nil {
			t.Fatalf("trafficType=%q: unexpected error: %v", tc.trafficType, err)
		}
		if u.ServiceTier != tc.expectedTier {
			t.Errorf("trafficType=%q: expected ServiceTier=%q, got %q",
				tc.trafficType, tc.expectedTier, u.ServiceTier)
		}
	}
}

// TestGeminiCalculator_Cost_Priority verifies that ON_DEMAND_PRIORITY requests
// are billed at _priority rate variants instead of standard rates.
func TestGeminiCalculator_Cost_Priority(t *testing.T) {
	p := ModelPricing{
		InputCostPerToken:               1e-6,
		OutputCostPerToken:              2e-6,
		InputCostPerTokenPriority:       3e-6, // 3× standard input
		OutputCostPerTokenPriority:      6e-6, // 3× standard output
		CacheReadInputTokenCostPriority: 0.5e-6,
	}
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		ServiceTier:      "priority",
	}
	cost := genericCalculateCost(usage, p)
	expected := float64(100)*3e-6 + float64(50)*6e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
	// Confirm it differs from standard rates
	standardCost := float64(100)*1e-6 + float64(50)*2e-6
	if almostEqual(cost, standardCost) {
		t.Errorf("priority cost should differ from standard cost")
	}
}

// TestGeminiCalculator_Cost_Priority_Above200k verifies >200k tiered priority rates.
func TestGeminiCalculator_Cost_Priority_Above200k(t *testing.T) {
	p := ModelPricing{
		InputCostPerToken:                   1e-6,
		OutputCostPerToken:                  2e-6,
		InputCostPerTokenAbove200k:          1.5e-6,
		OutputCostPerTokenAbove200k:         3e-6,
		InputCostPerTokenPriority:           3e-6,
		OutputCostPerTokenPriority:          6e-6,
		InputCostPerTokenAbove200kPriority:  4e-6,
		OutputCostPerTokenAbove200kPriority: 8e-6,
	}
	usage := Usage{
		PromptTokens:     210_000,
		CompletionTokens: 50,
		TotalTokens:      210_050,
		ServiceTier:      "priority",
	}
	cost := genericCalculateCost(usage, p)
	// Tiering uses promptTokens (210k > 200k threshold) → above_200k_priority rates
	expected := float64(210_000)*4e-6 + float64(50)*8e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_Cost_Priority_AudioRate verifies that audio input uses the
// standard audio rate even when ServiceTier == "priority". LiteLLM does not apply
// the _priority suffix to audio token rates.
func TestGeminiCalculator_Cost_Priority_AudioRate(t *testing.T) {
	p := ModelPricing{
		InputCostPerToken:              1e-6,
		OutputCostPerToken:             2e-6,
		InputCostPerAudioToken:         1.5e-6,
		InputCostPerTokenPriority:      3e-6,
		OutputCostPerTokenPriority:     6e-6,
		InputCostPerAudioTokenPriority: 4e-6, // stored in pricing but NOT applied (LiteLLM parity)
	}
	usage := Usage{
		PromptTokens:     100,
		AudioInputTokens: 50,
		CompletionTokens: 20,
		ServiceTier:      "priority",
	}
	cost := genericCalculateCost(usage, p)
	// text prompt: (100-50) × 3e-6 (priority), audio: 50 × 1.5e-6 (standard!), completion: 20 × 6e-6 (priority)
	expected := float64(100-50)*3e-6 + float64(50)*1.5e-6 + float64(20)*6e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_Normalize_WebSearchRequests verifies that
// groundingMetadata.webSearchQueries are counted across candidates.
func TestGeminiCalculator_Normalize_WebSearchRequests(t *testing.T) {
	body := []byte(`{
		"candidates": [
			{"groundingMetadata": {"webSearchQueries": ["what is Go", "golang specs"]}},
			{"groundingMetadata": {"webSearchQueries": ["goroutines"]}}
		],
		"usageMetadata": {
			"promptTokenCount": 100,
			"candidatesTokenCount": 50,
			"totalTokenCount": 150
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.GeminiWebSearchRequests != 3 {
		t.Errorf("expected GeminiWebSearchRequests=3, got %d", u.GeminiWebSearchRequests)
	}
}

// TestGeminiCalculator_Adjust_Grounding_GoogleAI verifies per-query grounding fee
// for Google AI Studio (provider="gemini").
func TestGeminiCalculator_Adjust_Grounding_GoogleAI(t *testing.T) {
	p := ModelPricing{Provider: "gemini"}
	usage := Usage{GeminiWebSearchRequests: 3}
	c := &GeminiCalculator{}
	cost := c.Adjust(0.001, usage, p)
	// 0.001 base + 3 × $0.035 grounding
	expected := 0.001 + 3*0.035
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.6f, got %.6f", expected, cost)
	}
}

// TestGeminiCalculator_Adjust_Grounding_VertexAI verifies flat grounding fee
// for Vertex AI (provider="vertex_ai*"), regardless of query count.
func TestGeminiCalculator_Adjust_Grounding_VertexAI(t *testing.T) {
	p := ModelPricing{Provider: "vertex_ai"}
	usage := Usage{GeminiWebSearchRequests: 5}
	c := &GeminiCalculator{}
	cost := c.Adjust(0.001, usage, p)
	// 0.001 base + flat $0.035 (Vertex AI doesn't multiply by query count)
	expected := 0.001 + 0.035
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.6f, got %.6f", expected, cost)
	}
}

// TestGeminiCalculator_Adjust_Grounding_NoQueries verifies no extra cost when
// there are no grounding queries.
func TestGeminiCalculator_Adjust_Grounding_NoQueries(t *testing.T) {
	p := ModelPricing{Provider: "gemini"}
	usage := Usage{GeminiWebSearchRequests: 0}
	c := &GeminiCalculator{}
	cost := c.Adjust(0.001, usage, p)
	if !almostEqual(cost, 0.001) {
		t.Errorf("expected 0.001 (no grounding), got %.6f", cost)
	}
}

// ---------------------------------------------------------------------------
// Azure AI calculator
// ---------------------------------------------------------------------------

// TestGeminiCalculator_Cost_CacheRead_Above200k verifies that cached tokens use
// the tiered cache read rate (cache_read_input_token_cost_above_200k_tokens)
// when the prompt is above the 200k threshold.
func TestGeminiCalculator_Cost_CacheRead_Above200k(t *testing.T) {
	// gemini/gemini-2.5-pro: has both >200k input rate AND >200k cache read rate
	pricing, ok := lookupPricing("gemini/gemini-2.5-pro")
	if !ok {
		t.Skip("gemini/gemini-2.5-pro not in pricing map")
	}
	if pricing.CacheReadInputTokenCostAbove200k == 0 {
		t.Skip("model has no above-200k cache read rate")
	}

	// prompt=210k (above 200k threshold), with 50k cached tokens
	usage := Usage{
		PromptTokens:     210_000,
		CompletionTokens: 1_000,
		TotalTokens:      211_000,
		CachedReadTokens: 50_000,
	}
	cost := genericCalculateCost(usage, pricing)

	// 160k regular text tokens at above-200k input rate
	// 50k cached tokens at above-200k cache read rate
	// 1k completion at above-200k output rate
	expectedInput := float64(160_000) * pricing.InputCostPerTokenAbove200k
	expectedCache := float64(50_000) * pricing.CacheReadInputTokenCostAbove200k
	expectedOutput := float64(1_000) * pricing.OutputCostPerTokenAbove200k
	expected := expectedInput + expectedCache + expectedOutput
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
	// Sanity: base rate cache read would differ
	baseRateCache := float64(160_000)*pricing.InputCostPerToken +
		float64(50_000)*pricing.CacheReadInputTokenCost +
		float64(1_000)*pricing.OutputCostPerToken
	if almostEqual(cost, baseRateCache) {
		t.Errorf("expected tiered rate to differ from base rate — regression!")
	}
}

// TestGeminiCalculator_Cost_Priority_CacheRead verifies that cached tokens use
// the priority cache read rate when ServiceTier == "priority".
func TestGeminiCalculator_Cost_Priority_CacheRead(t *testing.T) {
	p := ModelPricing{
		InputCostPerToken:               1e-6,
		OutputCostPerToken:              2e-6,
		CacheReadInputTokenCost:         0.25e-6,
		InputCostPerTokenPriority:       3e-6,
		OutputCostPerTokenPriority:      6e-6,
		CacheReadInputTokenCostPriority: 0.75e-6, // 3× standard cache read rate
	}
	usage := Usage{
		PromptTokens:     1_000,
		CompletionTokens: 200,
		TotalTokens:      1_200,
		CachedReadTokens: 500,
		ServiceTier:      "priority",
	}
	cost := genericCalculateCost(usage, p)
	// regular text: (1000-500) × 3e-6, cache: 500 × 0.75e-6, output: 200 × 6e-6
	expected := float64(500)*3e-6 + float64(500)*0.75e-6 + float64(200)*6e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
	// Confirm differs from standard rates
	standard := float64(500)*1e-6 + float64(500)*0.25e-6 + float64(200)*2e-6
	if almostEqual(cost, standard) {
		t.Errorf("priority cost should differ from standard cost")
	}
}

// TestGeminiCalculator_Normalize_CachedAudioInput verifies that cached audio
// tokens from cacheTokensDetails are stored in CachedAudioInputTokens.
func TestGeminiCalculator_Normalize_CachedAudioInput(t *testing.T) {
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 500,
			"candidatesTokenCount": 100,
			"totalTokenCount": 600,
			"cachedContentTokenCount": 200,
			"promptTokensDetails": [
				{"modality": "TEXT",  "tokenCount": 300},
				{"modality": "AUDIO", "tokenCount": 200}
			],
			"cacheTokensDetails": [
				{"modality": "AUDIO", "tokenCount": 150}
			]
		}
	}`)
	c := &GeminiCalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 200 raw audio - 150 cached = 50 non-cached audio input
	if u.AudioInputTokens != 50 {
		t.Errorf("expected AudioInputTokens=50, got %d", u.AudioInputTokens)
	}
	// 150 cached audio tokens tracked separately
	if u.CachedAudioInputTokens != 150 {
		t.Errorf("expected CachedAudioInputTokens=150, got %d", u.CachedAudioInputTokens)
	}
	// Total cached read tokens = 200 (the cachedContentTokenCount)
	if u.CachedReadTokens != 200 {
		t.Errorf("expected CachedReadTokens=200, got %d", u.CachedReadTokens)
	}
}

// TestGeminiCalculator_Cost_CachedAudioRate verifies that cached audio tokens are
// billed at the dedicated audio cache read rate (cache_read_input_token_cost_per_audio_token).
func TestGeminiCalculator_Cost_CachedAudioRate(t *testing.T) {
	p := ModelPricing{
		InputCostPerToken:                    1e-6,
		OutputCostPerToken:                   2e-6,
		InputCostPerAudioToken:               5e-7,
		CacheReadInputTokenCost:              2.5e-8, // text cache read rate
		CacheReadInputTokenCostPerAudioToken: 5e-8,   // audio cache read rate (2× text)
	}
	usage := Usage{
		PromptTokens:           500,
		CompletionTokens:       100,
		TotalTokens:            600,
		CachedReadTokens:       200,
		CachedAudioInputTokens: 150, // 150 of the 200 cached tokens are audio
		AudioInputTokens:       50,  // 50 non-cached audio tokens
	}
	cost := genericCalculateCost(usage, p)
	// text input: (500 - 200 - 50) = 250 × 1e-6
	// non-cached audio: 50 × 5e-7
	// text cached: (200-150)=50 × 2.5e-8
	// audio cached: 150 × 5e-8
	// output: 100 × 2e-6
	expected := float64(250)*1e-6 +
		float64(50)*5e-7 +
		float64(50)*2.5e-8 +
		float64(150)*5e-8 +
		float64(100)*2e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
	// Confirm differs from billing all cached tokens at text rate
	allTextCache := float64(250)*1e-6 + float64(50)*5e-7 + float64(200)*2.5e-8 + float64(100)*2e-6
	if almostEqual(cost, allTextCache) {
		t.Errorf("audio cache rate should produce different result from text-only cache rate")
	}
}

// TestGeminiCalculator_Cost_CachedAudioRate_Fallback verifies that when no dedicated
// audio cache rate is set, all cached tokens fall back to the standard cache read rate.
func TestGeminiCalculator_Cost_CachedAudioRate_Fallback(t *testing.T) {
	p := ModelPricing{
		InputCostPerToken:       1e-6,
		OutputCostPerToken:      2e-6,
		CacheReadInputTokenCost: 2.5e-8,
		// No CacheReadInputTokenCostPerAudioToken set
	}
	usage := Usage{
		PromptTokens:           300,
		CompletionTokens:       50,
		TotalTokens:            350,
		CachedReadTokens:       100,
		CachedAudioInputTokens: 60,
	}
	cost := genericCalculateCost(usage, p)
	// All 100 cached tokens at text rate (fallback)
	// regular: (300-100)=200 × 1e-6, cache: 100 × 2.5e-8, output: 50 × 2e-6
	expected := float64(200)*1e-6 + float64(100)*2.5e-8 + float64(50)*2e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_EndToEnd_Grounding verifies the full Normalize → genericCalculateCost
// → Adjust pipeline for a response with grounding (web search queries).
// This is critical because Adjust() adds grounding cost, but Cost_* tests bypass it
// by calling genericCalculateCost directly.
func TestGeminiCalculator_EndToEnd_Grounding(t *testing.T) {
	pricing, ok := lookupPricing("gemini/gemini-2.0-flash")
	if !ok {
		t.Skip("gemini/gemini-2.0-flash not in pricing map")
	}

	// Response with 2 grounding queries in candidates[0].groundingMetadata
	body := []byte(`{
		"modelVersion": "gemini-2.0-flash",
		"candidates": [
			{
				"groundingMetadata": {
					"webSearchQueries": ["golang generics", "go 1.18 features"]
				}
			}
		],
		"usageMetadata": {
			"promptTokenCount": 100,
			"candidatesTokenCount": 50,
			"totalTokenCount": 150
		}
	}`)

	c := &GeminiCalculator{}
	usage, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if usage.GeminiWebSearchRequests != 2 {
		t.Errorf("expected GeminiWebSearchRequests=2, got %d", usage.GeminiWebSearchRequests)
	}

	baseCost := genericCalculateCost(usage, pricing)
	finalCost := c.Adjust(baseCost, usage, pricing)

	// Google AI Studio (provider=gemini): $0.035 × 2 queries on top of token cost
	expectedGrounding := 2 * 0.035
	expectedBase := float64(100)*pricing.InputCostPerToken + float64(50)*pricing.OutputCostPerToken
	expected := expectedBase + expectedGrounding
	if !almostEqual(finalCost, expected) {
		t.Errorf("expected %.10f (%.10f base + %.10f grounding), got %.10f",
			expected, expectedBase, expectedGrounding, finalCost)
	}
}

// TestGeminiCalculator_EndToEnd_Priority verifies the full Normalize → Calculate
// pipeline for a response with ON_DEMAND_PRIORITY trafficType.
func TestGeminiCalculator_EndToEnd_Priority(t *testing.T) {
	pricing, ok := lookupPricing("vertex_ai/gemini-3-flash-preview")
	if !ok {
		t.Skip("vertex_ai/gemini-3-flash-preview not in pricing map")
	}
	if pricing.InputCostPerTokenPriority == 0 {
		t.Skip("model has no priority input rate")
	}

	body := []byte(`{
		"modelVersion": "gemini-3-flash-preview",
		"usageMetadata": {
			"promptTokenCount": 1000,
			"candidatesTokenCount": 500,
			"totalTokenCount": 1500,
			"trafficType": "ON_DEMAND_PRIORITY"
		}
	}`)

	c := &GeminiCalculator{}
	usage, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if usage.ServiceTier != "priority" {
		t.Errorf("expected ServiceTier=priority, got %q", usage.ServiceTier)
	}

	cost := genericCalculateCost(usage, pricing)
	c.Adjust(cost, usage, pricing) // no grounding, just verifies no panic

	expected := float64(1000)*pricing.InputCostPerTokenPriority +
		float64(500)*pricing.OutputCostPerTokenPriority
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// TestGeminiCalculator_EndToEnd_Thinking verifies the full Normalize → Calculate
// pipeline for a thinking model response with exclusive thoughtsTokenCount.
func TestGeminiCalculator_EndToEnd_Thinking(t *testing.T) {
	// Use synthetic rates to keep the arithmetic clear
	pricing := ModelPricing{
		InputCostPerToken:           1e-6,
		OutputCostPerToken:          2e-6,
		OutputCostPerReasoningToken: 3e-6, // thinking billed at 3× output rate
	}

	// Exclusive case: prompt=100, candidates=80 (text), thoughts=20, total=200
	body := []byte(`{
		"usageMetadata": {
			"promptTokenCount":     100,
			"candidatesTokenCount": 80,
			"thoughtsTokenCount":   20,
			"totalTokenCount":      200
		}
	}`)

	c := &GeminiCalculator{}
	usage, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	// Exclusive: completionTokens = 80+20 = 100, reasoningTokens = 20
	if usage.CompletionTokens != 100 || usage.ReasoningTokens != 20 {
		t.Errorf("unexpected usage: CompletionTokens=%d ReasoningTokens=%d",
			usage.CompletionTokens, usage.ReasoningTokens)
	}

	cost := genericCalculateCost(usage, pricing)
	// 100 prompt × 1e-6, 80 text output × 2e-6, 20 reasoning × 3e-6
	expected := float64(100)*1e-6 + float64(80)*2e-6 + float64(20)*3e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// ---------------------------------------------------------------------------
// Azure AI calculator
// ---------------------------------------------------------------------------

func TestAzureAICalculator_Normalize_OpenAIModel(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}}`)
	c := &AzureAICalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.PromptTokens != 100 || u.CompletionTokens != 50 {
		t.Errorf("unexpected usage for openai-format model: %+v", u)
	}
}

func TestAzureAICalculator_Normalize_ClaudeModel(t *testing.T) {
	body := []byte(`{
		"model":"claude-haiku-4-5",
		"usage":{"input_tokens":200,"output_tokens":100,"cache_read_input_tokens":30}
	}`)
	c := &AzureAICalculator{}
	u, err := c.Normalize(body, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After Anthropic convention fix: PromptTokens = input_tokens + cache_creation + cache_read
	// = 200 + 0 + 30 = 230; genericCalculateCost then subtracts cached tokens to get regular input
	if u.PromptTokens != 230 || u.CompletionTokens != 100 {
		t.Errorf("expected Anthropic field mapping for claude model: %+v", u)
	}
	if u.CachedReadTokens != 30 {
		t.Errorf("expected CachedReadTokens=30, got %d", u.CachedReadTokens)
	}
}

func TestAzureAICalculator_Cost_ClaudeModel(t *testing.T) {
	// azure_ai/claude-haiku-4-5: input=1e-6, output=5e-6
	pricing, ok := lookupPricing("azure_ai/claude-haiku-4-5")
	if !ok {
		t.Skip("azure_ai/claude-haiku-4-5 not in pricing map")
	}
	usage := Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
	cost := genericCalculateCost(usage, pricing)
	expected := 1000*1e-6 + 500*5e-6
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestGenericCalculateCost_ZeroTokens(t *testing.T) {
	pricing, _ := lookupPricing("gpt-4o-mini-2024-07-18")
	cost := genericCalculateCost(Usage{}, pricing)
	if cost != 0.0 {
		t.Errorf("expected 0 cost for zero tokens, got %f", cost)
	}
}

func TestGenericCalculateCost_NegativeRegularTokens(t *testing.T) {
	// CachedReadTokens > PromptTokens — regularPromptTokens must not go negative.
	pricing, _ := lookupPricing("gpt-4o-mini-2024-07-18")
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		CachedReadTokens: 200, // exceeds PromptTokens
	}
	cost := genericCalculateCost(usage, pricing)
	if cost < 0 {
		t.Errorf("cost must not be negative, got %f", cost)
	}
}

func TestOpenAICalculator_Normalize_EmptyBody(t *testing.T) {
	c := &OpenAICalculator{}
	_, err := c.Normalize([]byte(`{}`), nil)
	if err != nil {
		t.Errorf("empty body should not return error, got %v", err)
	}
}

func TestOpenAICalculator_Normalize_MalformedBody(t *testing.T) {
	c := &OpenAICalculator{}
	_, err := c.Normalize([]byte(`not-json`), nil)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

// ---------------------------------------------------------------------------
// Policy mode
// ---------------------------------------------------------------------------

func TestLLMCostPolicy_Mode(t *testing.T) {
	p := &LLMCostPolicy{}
	mode := p.Mode()
	if mode.RequestBodyMode != policy.BodyModeBuffer {
		t.Errorf("expected RequestBodyMode=BUFFER, got %v", mode.RequestBodyMode)
	}
	if mode.ResponseBodyMode != policy.BodyModeBuffer {
		t.Errorf("expected ResponseBodyMode=BUFFER, got %v", mode.ResponseBodyMode)
	}
	if mode.ResponseHeaderMode != policy.HeaderModeProcess {
		t.Errorf("expected ResponseHeaderMode=PROCESS, got %v", mode.ResponseHeaderMode)
	}
}

// ---------------------------------------------------------------------------
// setCostHeader formatting
// ---------------------------------------------------------------------------

func TestSetCostHeader_Formatting(t *testing.T) {
	cases := []struct {
		cost     float64
		expected string
	}{
		{0.0, "0.0000000000"},
		{0.00004231, "0.0000423100"},
		{1.23456789012345, "1.2345678901"},
	}
	for _, tc := range cases {
		result := setCostHeader(tc.cost)
		mods, ok := result.(policy.UpstreamResponseModifications)
		if !ok {
			t.Fatalf("unexpected action type")
		}
		got := mods.SetHeaders[HeaderLLMCost]
		if got != tc.expected {
			t.Errorf("cost=%.15f: expected header %q, got %q", tc.cost, tc.expected, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Custom pricing file (mergeCustomPricing)
// ---------------------------------------------------------------------------

// writeCustomPricingFile writes a temporary JSON pricing file and returns its path.
func writeCustomPricingFile(t *testing.T, entries map[string]ModelPricing) string {
	t.Helper()
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("failed to marshal custom pricing: %v", err)
	}
	f := filepath.Join(t.TempDir(), "model_prices.json")
	if err := os.WriteFile(f, data, 0o600); err != nil {
		t.Fatalf("failed to write custom pricing file: %v", err)
	}
	return f
}

func TestMergeCustomPricing_AddNewModel(t *testing.T) {
	const key = "test-custom-model-xyz"
	// Ensure the key is not already in the embedded map.
	if _, exists := pricingMap[key]; exists {
		t.Skip("test key already in embedded map — choose a different key")
	}

	path := writeCustomPricingFile(t, map[string]ModelPricing{
		key: {Provider: "openai", InputCostPerToken: 1.23e-6, OutputCostPerToken: 4.56e-6},
	})
	mergeCustomPricing(path)
	defer delete(pricingMap, key) // clean up so other tests are not affected

	p, ok := pricingMap[key]
	if !ok {
		t.Fatal("expected new model to be added to pricingMap")
	}
	if p.Provider != "openai" {
		t.Errorf("expected provider=openai, got %q", p.Provider)
	}
	if !almostEqual(p.InputCostPerToken, 1.23e-6) {
		t.Errorf("expected InputCostPerToken=1.23e-6, got %v", p.InputCostPerToken)
	}
}

func TestMergeCustomPricing_OverrideExistingModel(t *testing.T) {
	const key = "gpt-4o-mini-2024-07-18"
	original, ok := pricingMap[key]
	if !ok {
		t.Skip("gpt-4o-mini-2024-07-18 not in embedded map — test cannot proceed")
	}
	// Use a sentinel rate that differs from the real one.
	const sentinelRate = 9.99e-5

	path := writeCustomPricingFile(t, map[string]ModelPricing{
		key: {Provider: "openai", InputCostPerToken: sentinelRate, OutputCostPerToken: sentinelRate},
	})
	mergeCustomPricing(path)
	defer func() { pricingMap[key] = original }() // restore original entry

	p := pricingMap[key]
	if !almostEqual(p.InputCostPerToken, sentinelRate) {
		t.Errorf("expected overridden InputCostPerToken=%v, got %v", sentinelRate, p.InputCostPerToken)
	}
}

func TestMergeCustomPricing_NonExistentFile(t *testing.T) {
	// A path that does not exist must be a silent no-op (no panic, no change).
	before := len(pricingMap)
	mergeCustomPricing(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if len(pricingMap) != before {
		t.Errorf("pricingMap size changed after reading non-existent file")
	}
}

func TestMergeCustomPricing_MalformedFile(t *testing.T) {
	// A file with invalid JSON must not modify pricingMap.
	f := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(f, []byte("not json at all"), 0o600); err != nil {
		t.Fatalf("failed to write bad JSON file: %v", err)
	}
	before := len(pricingMap)
	mergeCustomPricing(f)
	if len(pricingMap) != before {
		t.Errorf("pricingMap size changed after reading malformed file")
	}
}

func TestMergeCustomPricing_InvalidEntry(t *testing.T) {
	// A file where one entry has a type mismatch (string where float expected)
	// must skip that entry and leave the rest of pricingMap intact.
	const key = "test-invalid-entry-xyz"
	raw := []byte(`{"` + key + `": {"input_cost_per_token": "not-a-number"}}`)
	f := filepath.Join(t.TempDir(), "invalid_entry.json")
	if err := os.WriteFile(f, raw, 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	before := len(pricingMap)
	mergeCustomPricing(f)
	if len(pricingMap) != before {
		t.Errorf("pricingMap size changed after skipping invalid entry")
	}
	if _, exists := pricingMap[key]; exists {
		t.Error("invalid entry should not have been added to pricingMap")
	}
}

// ---------------------------------------------------------------------------
// selectCalculator routing
// ---------------------------------------------------------------------------

func TestSelectCalculator_Routing(t *testing.T) {
	cases := []struct {
		provider string
		wantType string
	}{
		{"openai", "*llmcost.OpenAICalculator"},
		{"text-completion-openai", "*llmcost.OpenAICalculator"},
		{"anthropic", "*llmcost.AnthropicCalculator"},
		{"azure", "*llmcost.AzureOpenAICalculator"},
		{"azure_text", "*llmcost.AzureOpenAICalculator"},
		{"azure_ai", "*llmcost.AzureAICalculator"},
		{"mistral", "*llmcost.MistralCalculator"},
		{"gemini", "*llmcost.GeminiCalculator"},
		{"vertex_ai", "*llmcost.GeminiCalculator"},
		{"vertex_ai-language-models", "*llmcost.GeminiCalculator"},
		{"unknown-provider", "*llmcost.OpenAICalculator"}, // default fallback
	}
	for _, tc := range cases {
		c := selectCalculator(tc.provider)
		got := fmt.Sprintf("%T", c)
		if got != tc.wantType {
			t.Errorf("provider=%q: expected %s, got %s", tc.provider, tc.wantType, got)
		}
	}
}
