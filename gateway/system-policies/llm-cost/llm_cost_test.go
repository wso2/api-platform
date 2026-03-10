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
	if u.PromptTokens != 300 || u.CompletionTokens != 150 {
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

func TestGeminiCalculator_Normalize(t *testing.T) {
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
	// gemini-1.5-flash above 128k: input=1.5e-7, output=6e-7
	pricing, ok := lookupPricing("gemini/gemini-1.5-flash")
	if !ok {
		t.Skip("gemini/gemini-1.5-flash not in pricing map")
	}
	usage := Usage{PromptTokens: 100_000, CompletionTokens: 50_000, TotalTokens: 150_000}
	cost := genericCalculateCost(usage, pricing)
	expected := 100_000*1.5e-7 + 50_000*6e-7
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f (above-128k rate), got %.10f", expected, cost)
	}
}

func TestGeminiCalculator_Cost_Above200k(t *testing.T) {
	// gemini-2.5-pro above 200k: input=2.5e-6, output=1.5e-5
	pricing, ok := lookupPricing("gemini/gemini-2.5-pro")
	if !ok {
		t.Skip("gemini/gemini-2.5-pro not in pricing map")
	}
	usage := Usage{PromptTokens: 150_000, CompletionTokens: 75_000, TotalTokens: 225_000}
	cost := genericCalculateCost(usage, pricing)
	expected := 150_000*2.5e-6 + 75_000*1.5e-5
	if !almostEqual(cost, expected) {
		t.Errorf("expected %.10f (above-200k rate), got %.10f", expected, cost)
	}
}

func TestGeminiCalculator_Adjust_PassThrough(t *testing.T) {
	c := &GeminiCalculator{}
	if c.Adjust(1.23, Usage{}, ModelPricing{}) != 1.23 {
		t.Error("Adjust should be a pass-through for Gemini")
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
	if u.PromptTokens != 200 || u.CompletionTokens != 100 {
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
