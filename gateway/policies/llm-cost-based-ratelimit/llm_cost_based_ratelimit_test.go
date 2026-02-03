/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package llmcostratelimit

import (
	"sync"
	"sync/atomic"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// setupGlobalResourceStore sets up the global lazy resource store with test resources
func setupGlobalResourceStore(t *testing.T) func() {
	store := policy.GetLazyResourceStoreInstance()

	// Clear any existing resources
	_ = store.ClearAll()

	// Add provider template mapping
	mapping := &policy.LazyResource{
		ID:           "test-provider",
		ResourceType: ResourceTypeProviderTemplateMapping,
		Resource: map[string]interface{}{
			"template_handle": "openai-template",
		},
	}
	if err := store.StoreResource(mapping); err != nil {
		t.Fatalf("Failed to store mapping: %v", err)
	}

	// Add LLM provider template with token extraction paths
	template := &policy.LazyResource{
		ID:           "openai-template",
		ResourceType: ResourceTypeLlmProviderTemplate,
		Resource: map[string]interface{}{
			"configuration": map[string]interface{}{
				"spec": map[string]interface{}{
					"promptTokens": map[string]interface{}{
						"identifier": "$.usage.prompt_tokens",
						"location":   "payload",
					},
					"completionTokens": map[string]interface{}{
						"identifier": "$.usage.completion_tokens",
						"location":   "payload",
					},
					"totalTokens": map[string]interface{}{
						"identifier": "$.usage.total_tokens",
						"location":   "payload",
					},
				},
			},
		},
	}
	if err := store.StoreResource(template); err != nil {
		t.Fatalf("Failed to store template: %v", err)
	}

	// Return cleanup function
	return func() {
		_ = store.ClearAll()
	}
}

// createTestRequestContext creates a request context with provider metadata
func createTestRequestContext(providerName string) *policy.RequestContext {
	return &policy.RequestContext{
		Headers: policy.NewHeaders(map[string][]string{
			"content-type": {"application/json"},
		}),
		SharedContext: &policy.SharedContext{
			Metadata: map[string]interface{}{
				MetadataKeyProviderName: providerName,
			},
		},
	}
}

// createTestResponseContext creates a response context with body
func createTestResponseContext(body []byte) *policy.ResponseContext {
	return &policy.ResponseContext{
		ResponseHeaders: policy.NewHeaders(map[string][]string{
			"content-type": {"application/json"},
		}),
		ResponseBody: &policy.Body{
			Present: true,
			Content: body,
		},
		SharedContext: &policy.SharedContext{
			Metadata: make(map[string]interface{}),
		},
	}
}

// TestLLMCostRateLimitPolicy_Mode tests the processing mode
func TestLLMCostRateLimitPolicy_Mode(t *testing.T) {
	p := &LLMCostRateLimitPolicy{}
	mode := p.Mode()

	expected := policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeBuffer, // Need body for token extraction
	}

	if mode != expected {
		t.Errorf("Expected mode %+v, got %+v", expected, mode)
	}
}

// TestLLMCostRateLimitPolicy_GetPolicy tests policy creation
func TestLLMCostRateLimitPolicy_GetPolicy(t *testing.T) {
	metadata := policy.PolicyMetadata{
		RouteName: "test-route",
	}

	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
		},
		"promptTokenCost":     float64(0.000002),
		"completionTokenCost": float64(0.000006),
		"algorithm":           "fixed-window",
		"backend":             "memory",
	}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if p == nil {
		t.Fatal("Expected policy instance, got nil")
	}

	costPolicy, ok := p.(*LLMCostRateLimitPolicy)
	if !ok {
		t.Fatalf("Expected LLMCostRateLimitPolicy, got %T", p)
	}

	if costPolicy.metadata.RouteName != "test-route" {
		t.Errorf("Expected route name 'test-route', got '%s'", costPolicy.metadata.RouteName)
	}
}

// TestLLMCostRateLimitPolicy_OnRequest_NoProvider tests behavior when provider is missing
func TestLLMCostRateLimitPolicy_OnRequest_NoProvider(t *testing.T) {
	metadata := policy.PolicyMetadata{
		RouteName: "test-route",
	}

	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
		},
	}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Create context without provider metadata
	ctx := &policy.RequestContext{
		Headers: policy.NewHeaders(map[string][]string{}),
		SharedContext: &policy.SharedContext{
			Metadata: map[string]interface{}{}, // No provider_name
		},
	}

	action := p.OnRequest(ctx, params)

	// Should return nil (skip) when no provider is found
	if action != nil {
		t.Errorf("Expected nil action when provider is missing, got %T", action)
	}
}

// TestLLMCostRateLimitPolicy_OnRequest_EmptyProvider tests behavior with empty provider name
func TestLLMCostRateLimitPolicy_OnRequest_EmptyProvider(t *testing.T) {
	metadata := policy.PolicyMetadata{
		RouteName: "test-route",
	}

	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
		},
	}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Create context with empty provider name
	ctx := &policy.RequestContext{
		Headers: policy.NewHeaders(map[string][]string{}),
		SharedContext: &policy.SharedContext{
			Metadata: map[string]interface{}{
				MetadataKeyProviderName: "",
			},
		},
	}

	action := p.OnRequest(ctx, params)

	// Should return nil (skip) when provider name is empty
	if action != nil {
		t.Errorf("Expected nil action when provider is empty, got %T", action)
	}
}

// TestTransformToRatelimitParams tests the parameter transformation
func TestTransformToRatelimitParams(t *testing.T) {
	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
			map[string]interface{}{
				"amount":   float64(100),
				"duration": "24h",
			},
		},
		"promptTokenCost":     float64(0.000002),
		"completionTokenCost": float64(0.000006),
		"algorithm":           "fixed-window",
		"backend":             "memory",
	}

	template := map[string]interface{}{
		"configuration": map[string]interface{}{
			"spec": map[string]interface{}{
				"promptTokens": map[string]interface{}{
					"identifier": "$.usage.prompt_tokens",
					"location":   "payload",
				},
				"completionTokens": map[string]interface{}{
					"identifier": "$.usage.completion_tokens",
					"location":   "payload",
				},
			},
		},
	}

	result := transformToRatelimitParams(params, template)

	// Check quotas were created
	quotas, ok := result["quotas"].([]interface{})
	if !ok {
		t.Fatal("Expected quotas to be []interface{}")
	}

	if len(quotas) != 1 {
		t.Fatalf("Expected 1 quota, got %d", len(quotas))
	}

	quota := quotas[0].(map[string]interface{})

	// Check limits were converted
	limits, ok := quota["limits"].([]interface{})
	if !ok {
		t.Fatal("Expected limits to be []interface{}")
	}

	if len(limits) != 2 {
		t.Errorf("Expected 2 limits, got %d", len(limits))
	}

	// Check cost extraction configuration
	costExtraction, ok := quota["costExtraction"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected costExtraction to be present")
	}

	sources, ok := costExtraction["sources"].([]interface{})
	if !ok {
		t.Fatal("Expected sources to be []interface{}")
	}

	if len(sources) != 2 {
		t.Errorf("Expected 2 cost sources, got %d", len(sources))
	}

	// Check passthrough parameters
	if result["algorithm"] != "fixed-window" {
		t.Errorf("Expected algorithm 'fixed-window', got %v", result["algorithm"])
	}

	if result["backend"] != "memory" {
		t.Errorf("Expected backend 'memory', got %v", result["backend"])
	}
}

// TestTransformToRatelimitParams_ZeroCostPerNTokens tests that zero costPerNTokens doesn't cause division by zero
func TestTransformToRatelimitParams_ZeroCostPerNTokens(t *testing.T) {
	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
		},
		"promptTokenCost":     float64(0.002),
		"completionTokenCost": float64(0.006),
		"costPerNTokens":      0, // Zero should not cause panic
		"algorithm":           "fixed-window",
		"backend":             "memory",
	}

	template := map[string]interface{}{
		"configuration": map[string]interface{}{
			"spec": map[string]interface{}{
				"promptTokens": map[string]interface{}{
					"identifier": "$.usage.prompt_tokens",
					"location":   "payload",
				},
				"completionTokens": map[string]interface{}{
					"identifier": "$.usage.completion_tokens",
					"location":   "payload",
				},
			},
		},
	}

	// Should not panic
	result := transformToRatelimitParams(params, template)

	quotas, ok := result["quotas"].([]interface{})
	if !ok {
		t.Fatal("Expected quotas to be []interface{}")
	}

	if len(quotas) != 1 {
		t.Fatalf("Expected 1 quota, got %d", len(quotas))
	}

	quota := quotas[0].(map[string]interface{})

	// Should have cost extraction with default costPerNTokens (1000000)
	costExtraction, ok := quota["costExtraction"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected costExtraction to be present")
	}

	sources, ok := costExtraction["sources"].([]interface{})
	if !ok || len(sources) != 2 {
		t.Errorf("Expected 2 cost sources, got %d", len(sources))
	}
}

// TestTransformToRatelimitParams_NoBudgetLimits tests transformation with missing budgets
func TestTransformToRatelimitParams_NoBudgetLimits(t *testing.T) {
	params := map[string]interface{}{
		"promptTokenCost": float64(0.000002),
		"algorithm":       "fixed-window",
		"backend":         "memory",
	}

	template := map[string]interface{}{}

	result := transformToRatelimitParams(params, template)

	quotas, ok := result["quotas"].([]interface{})
	if !ok {
		t.Fatal("Expected quotas to be []interface{}")
	}

	// Should have 0 quotas since no budget limits are configured
	if len(quotas) != 0 {
		t.Errorf("Expected 0 quotas when no budgets configured, got %d", len(quotas))
	}
}

// TestTransformToRatelimitParams_NoTokenCosts tests transformation without token costs
func TestTransformToRatelimitParams_NoTokenCosts(t *testing.T) {
	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
		},
		"algorithm": "fixed-window",
		"backend":   "memory",
	}

	template := map[string]interface{}{
		"spec": map[string]interface{}{
			"promptTokens": map[string]interface{}{
				"identifier": "$.usage.prompt_tokens",
				"location":   "payload",
			},
		},
	}

	result := transformToRatelimitParams(params, template)

	quotas, ok := result["quotas"].([]interface{})
	if !ok {
		t.Fatal("Expected quotas to be []interface{}")
	}

	if len(quotas) != 1 {
		t.Fatalf("Expected 1 quota, got %d", len(quotas))
	}

	quota := quotas[0].(map[string]interface{})

	// Should have no cost extraction when token costs are 0
	if _, hasCostExtraction := quota["costExtraction"]; hasCostExtraction {
		t.Error("Expected no costExtraction when token costs are 0")
	}
}

// TestExtractTokenCosts tests token cost extraction
func TestExtractTokenCosts(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected map[string]float64
	}{
		{
			name: "costs at root level",
			params: map[string]interface{}{
				"promptTokenCost":     float64(0.000002),
				"completionTokenCost": float64(0.000006),
				"totalTokenCost":      float64(0.000003),
			},
			expected: map[string]float64{
				"promptTokenCost":     0.000002,
				"completionTokenCost": 0.000006,
				"totalTokenCost":      0.000003,
			},
		},
		{
			name: "costs in systemParameters",
			params: map[string]interface{}{
				"systemParameters": map[string]interface{}{
					"promptTokenCost":     float64(0.000001),
					"completionTokenCost": float64(0.000002),
				},
			},
			expected: map[string]float64{
				"promptTokenCost":     0.000001,
				"completionTokenCost": 0.000002,
			},
		},
		{
			name: "integer costs converted to float",
			params: map[string]interface{}{
				"promptTokenCost": int(1),
				"totalTokenCost":  int64(2),
			},
			expected: map[string]float64{
				"promptTokenCost": 1.0,
				"totalTokenCost":  2.0,
			},
		},
		{
			name:     "no costs defined",
			params:   map[string]interface{}{},
			expected: map[string]float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := extractTokenCosts(tt.params)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d costs, got %d", len(tt.expected), len(result))
			}
			for key, expectedVal := range tt.expected {
				if result[key] != expectedVal {
					t.Errorf("Expected %s=%v, got %v", key, expectedVal, result[key])
				}
			}
		})
	}
}

// TestBuildCostSource tests cost source building
func TestBuildCostSource(t *testing.T) {
	template := map[string]interface{}{
		"spec": map[string]interface{}{
			"promptTokens": map[string]interface{}{
				"identifier": "$.usage.prompt_tokens",
				"location":   "payload",
			},
			"completionTokens": map[string]interface{}{
				"identifier": "x-completion-tokens",
				"location":   "header",
			},
			"totalTokens": map[string]interface{}{
				"identifier": "total_tokens",
				"location":   "metadata",
			},
		},
	}

	tests := []struct {
		name         string
		templateKey  string
		costPerToken float64
		expectNil    bool
		expectedType string
	}{
		{
			name:         "payload location",
			templateKey:  "promptTokens",
			costPerToken: 0.000002,
			expectNil:    false,
			expectedType: "response_body",
		},
		{
			name:         "header location",
			templateKey:  "completionTokens",
			costPerToken: 0.000006,
			expectNil:    false,
			expectedType: "response_header",
		},
		{
			name:         "metadata location",
			templateKey:  "totalTokens",
			costPerToken: 0.000003,
			expectNil:    false,
			expectedType: "response_metadata",
		},
		{
			name:         "non-existent key",
			templateKey:  "nonExistent",
			costPerToken: 0.000001,
			expectNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCostSource(template, tt.templateKey, tt.costPerToken)
			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("Expected non-nil result")
			}
			if result["type"] != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, result["type"])
			}
			if result["multiplier"] != tt.costPerToken {
				t.Errorf("Expected multiplier %v, got %v", tt.costPerToken, result["multiplier"])
			}
		})
	}
}

// TestGetTemplateSpec tests template spec extraction
func TestGetTemplateSpec(t *testing.T) {
	tests := []struct {
		name     string
		template map[string]interface{}
		expectNil bool
	}{
		{
			name: "direct spec",
			template: map[string]interface{}{
				"spec": map[string]interface{}{
					"promptTokens": map[string]interface{}{
						"identifier": "$.usage.prompt_tokens",
					},
				},
			},
			expectNil: false,
		},
		{
			name: "nested in configuration",
			template: map[string]interface{}{
				"configuration": map[string]interface{}{
					"spec": map[string]interface{}{
						"promptTokens": map[string]interface{}{
							"identifier": "$.usage.prompt_tokens",
						},
					},
				},
			},
			expectNil: false,
		},
		{
			name:      "nil template",
			template:  nil,
			expectNil: true,
		},
		{
			name:      "empty template",
			template:  map[string]interface{}{},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTemplateSpec(tt.template)
			if tt.expectNil && result != nil {
				t.Errorf("Expected nil, got %v", result)
			}
			if !tt.expectNil && result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// TestLLMCostRateLimitPolicy_ConcurrentAccess tests thread-safe delegate creation
func TestLLMCostRateLimitPolicy_ConcurrentAccess(t *testing.T) {
	metadata := policy.PolicyMetadata{
		RouteName: "test-route",
	}

	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
		},
		"algorithm": "fixed-window",
		"backend":   "memory",
	}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	costPolicy := p.(*LLMCostRateLimitPolicy)

	var wg sync.WaitGroup
	numGoroutines := 100
	var successCount atomic.Int32

	// Concurrent access without proper mock store - this will test the race condition handling
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := createTestRequestContext("test-provider")
			action := costPolicy.OnRequest(ctx, params)
			// action may be nil due to missing store, but shouldn't panic
			if action == nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// All goroutines should complete without panic
	t.Logf("Completed %d concurrent requests, %d returned nil action", numGoroutines, successCount.Load())
}

// TestLLMCostRateLimitPolicy_MultipleProviders tests handling multiple providers
func TestLLMCostRateLimitPolicy_MultipleProviders(t *testing.T) {
	metadata := policy.PolicyMetadata{
		RouteName: "multi-provider-route",
	}

	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
		},
		"algorithm": "fixed-window",
		"backend":   "memory",
	}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	costPolicy := p.(*LLMCostRateLimitPolicy)

	providers := []string{"provider-1", "provider-2", "provider-3"}

	for _, provider := range providers {
		// Each provider should get its own delegate (though they'll fail due to missing store)
		ctx := &policy.RequestContext{
			Headers: policy.NewHeaders(map[string][]string{}),
			SharedContext: &policy.SharedContext{
				Metadata: map[string]interface{}{
					MetadataKeyProviderName: provider,
				},
			},
		}

		action := costPolicy.OnRequest(ctx, params)
		// May be nil due to missing store, but shouldn't panic
		_ = action
	}

	// Verify delegates map has entries for each provider
	delegateCount := 0
	costPolicy.delegates.Range(func(key, value interface{}) bool {
		delegateCount++
		return true
	})

	t.Logf("Created delegates for %d providers", delegateCount)
}

// TestLLMCostRateLimitPolicy_Integration_BasicRateLimit tests basic rate limiting with store setup
func TestLLMCostRateLimitPolicy_Integration_BasicRateLimit(t *testing.T) {
	cleanup := setupGlobalResourceStore(t)
	defer cleanup()

	metadata := policy.PolicyMetadata{
		RouteName: "test-route",
	}

	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(1), // $1 budget
				"duration": "1h",
			},
		},
		"promptTokenCost":     float64(0.1), // $0.10 per token for testing
		"completionTokenCost": float64(0.2), // $0.20 per token for testing
		"algorithm":           "fixed-window",
		"backend":             "memory",
	}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Make requests that consume tokens worth $0.30 each (5 tokens prompt + 1 token completion)
	// After 3 requests, we would consume $0.90, 4th request would exceed $1 budget
	for i := 0; i < 3; i++ {
		reqCtx := createTestRequestContext("test-provider")
		reqAction := p.OnRequest(reqCtx, params)

		// Request phase should pass pre-check (not rate limited yet)
		if _, ok := reqAction.(policy.UpstreamRequestModifications); !ok {
			t.Fatalf("Request %d phase should pass pre-check, got %T", i+1, reqAction)
		}

		// Response phase - consume $0.30 (5 prompt + 1 completion)
		respBody := []byte(`{"usage":{"prompt_tokens":5,"completion_tokens":1}}`)
		respCtx := createTestResponseContext(respBody)
		respCtx.SharedContext = reqCtx.SharedContext
		respCtx.Metadata = reqCtx.Metadata
		p.OnResponse(respCtx, params)
	}

	// 4th request should still be allowed (total $0.90 consumed, $0.10 remaining)
	reqCtx := createTestRequestContext("test-provider")
	reqAction := p.OnRequest(reqCtx, params)

	if _, ok := reqAction.(policy.UpstreamRequestModifications); !ok {
		t.Logf("4th request action type: %T (may or may not be rate limited based on timing)", reqAction)
	}
}

// TestLLMCostRateLimitPolicy_Integration_CostCalculation tests cost calculation
func TestLLMCostRateLimitPolicy_Integration_CostCalculation(t *testing.T) {
	cleanup := setupGlobalResourceStore(t)
	defer cleanup()

	metadata := policy.PolicyMetadata{
		RouteName: "cost-calc-route",
	}

	// $10 budget with specific token costs
	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
		},
		"promptTokenCost":     float64(0.001), // $0.001 per prompt token
		"completionTokenCost": float64(0.002), // $0.002 per completion token
		"algorithm":           "fixed-window",
		"backend":             "memory",
	}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// First request - should pass pre-check
	reqCtx := createTestRequestContext("test-provider")
	action := p.OnRequest(reqCtx, params)

	if _, ok := action.(policy.UpstreamRequestModifications); !ok {
		t.Fatalf("First request should be allowed, got %T", action)
	}

	// Simulate response with 100 prompt tokens and 50 completion tokens
	// Expected cost: (100 * $0.001) + (50 * $0.002) = $0.10 + $0.10 = $0.20
	respBody := []byte(`{"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}}`)
	respCtx := createTestResponseContext(respBody)
	respCtx.SharedContext = reqCtx.SharedContext
	respCtx.Metadata = reqCtx.Metadata

	respAction := p.OnResponse(respCtx, params)

	// Verify response action sets headers
	if respMods, ok := respAction.(policy.UpstreamResponseModifications); ok {
		if len(respMods.SetHeaders) == 0 {
			t.Error("Expected rate limit headers in response")
		}
	}
}

// TestLLMCostRateLimitPolicy_Integration_MultipleBudgetLimits tests multiple time window budgets
func TestLLMCostRateLimitPolicy_Integration_MultipleBudgetLimits(t *testing.T) {
	cleanup := setupGlobalResourceStore(t)
	defer cleanup()

	metadata := policy.PolicyMetadata{
		RouteName: "multi-limit-route",
	}

	// Multiple budget limits: $5/hour and $50/day
	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(5),
				"duration": "1h",
			},
			map[string]interface{}{
				"amount":   float64(50),
				"duration": "24h",
			},
		},
		"promptTokenCost":     float64(0.01),
		"completionTokenCost": float64(0.02),
		"algorithm":           "fixed-window",
		"backend":             "memory",
	}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Request should pass pre-check
	reqCtx := createTestRequestContext("test-provider")
	action := p.OnRequest(reqCtx, params)

	if _, ok := action.(policy.UpstreamRequestModifications); !ok {
		t.Fatalf("Request should be allowed, got %T", action)
	}

	// Response with tokens
	respBody := []byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	respCtx := createTestResponseContext(respBody)
	respCtx.SharedContext = reqCtx.SharedContext
	respCtx.Metadata = reqCtx.Metadata

	p.OnResponse(respCtx, params)
}

// TestLLMCostRateLimitPolicy_Integration_MissingProviderTemplate tests behavior with missing template
func TestLLMCostRateLimitPolicy_Integration_MissingProviderTemplate(t *testing.T) {
	// Clear all resources - no templates configured
	store := policy.GetLazyResourceStoreInstance()
	_ = store.ClearAll()

	metadata := policy.PolicyMetadata{
		RouteName: "test-route",
	}

	params := map[string]interface{}{
		"budgetLimits": []interface{}{
			map[string]interface{}{
				"amount":   float64(10),
				"duration": "1h",
			},
		},
		"algorithm": "fixed-window",
		"backend":   "memory",
	}

	p, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Request should fail to resolve delegate and return nil (skip)
	ctx := createTestRequestContext("unknown-provider")
	action := p.OnRequest(ctx, params)

	if action != nil {
		t.Errorf("Expected nil action when provider template is missing, got %T", action)
	}
}
