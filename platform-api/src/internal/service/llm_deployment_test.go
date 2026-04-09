package service

import (
	"strings"
	"testing"

	"platform-api/src/internal/model"
)

func TestMapModelAuthToAPI_NormalizesApiKeyType(t *testing.T) {
	auth := &model.UpstreamAuth{Type: "apiKey", Header: "Authorization", Value: "secret"}

	out := mapModelAuthToAPI(auth)
	if out == nil || out.Type == nil {
		t.Fatal("expected auth type to be present")
	}
	if *out.Type != "api-key" {
		t.Fatalf("expected auth type to be api-key, got %q", *out.Type)
	}
}

func float32Ptr(f float32) *float32 { return &f }

// providerWithConsumerLimits builds a minimal LLMProvider model with the given
// consumer-level rate limiting config and no backend (provider-level) limits.
func providerWithConsumerLimits(rl *model.LLMRateLimitingConfig) *model.LLMProvider {
	return &model.LLMProvider{
		ID:      "test-provider",
		Name:    "Test Provider",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Context: strPtr("/test"),
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "https://api.anthropic.com",
					Auth: &model.UpstreamAuth{
						Type:   "api-key",
						Header: "x-api-key",
						Value:  "test-key",
					},
				},
			},
			AccessControl: &model.LLMAccessControl{Mode: "allow_all"},
			RateLimiting:  rl,
		},
	}
}

// TestGenerateYAML_ConsumerRequestLimit verifies that a consumer-only request limit
// generates a single advanced-ratelimit policy where the key extraction includes
// x-wso2-application-id (making it consumer-scoped). Unlike token/cost limits,
// the request limit does NOT use a consumerBased flag — it uses the application ID
// directly in the key extraction.
func TestGenerateYAML_ConsumerRequestLimit(t *testing.T) {
	count := 100
	rl := &model.LLMRateLimitingConfig{
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Request: &model.RequestRateLimit{
					Enabled: true,
					Count:   count,
					Reset:   model.RateLimitResetWindow{Duration: 2, Unit: "hour"},
				},
			},
		},
	}

	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(yaml, "advanced-ratelimit") {
		t.Error("expected advanced-ratelimit policy in generated YAML")
	}
	// Consumer-scoped: key extraction must include x-wso2-application-id
	if !strings.Contains(yaml, "x-wso2-application-id") {
		t.Error("expected x-wso2-application-id in key extraction for consumer request limit")
	}
	// Should NOT have a backend (non-consumer) advanced-ratelimit entry
	if strings.Count(yaml, "advanced-ratelimit") > 1 {
		t.Error("expected only one advanced-ratelimit policy (consumer), got more than one")
	}

	t.Logf("Generated YAML:\n%s", yaml)
}

// TestGenerateYAML_ConsumerTokenLimit verifies that a consumer token limit
// generates a token-based-ratelimit policy with consumerBased: true.
func TestGenerateYAML_ConsumerTokenLimit(t *testing.T) {
	count := 100
	rl := &model.LLMRateLimitingConfig{
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Token: &model.TokenRateLimit{
					Enabled: true,
					Count:   count,
					Reset:   model.RateLimitResetWindow{Duration: 2, Unit: "hour"},
				},
			},
		},
	}

	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(yaml, "consumerBased: true") {
		t.Error("expected consumerBased: true in generated YAML")
	}
	if !strings.Contains(yaml, "token-based-ratelimit") {
		t.Error("expected token-based-ratelimit policy in generated YAML")
	}

	t.Logf("Generated YAML:\n%s", yaml)
}

// TestGenerateYAML_ConsumerCostLimit verifies that a consumer cost limit
// generates a llm-cost-based-ratelimit policy with consumerBased: true.
func TestGenerateYAML_ConsumerCostLimit(t *testing.T) {
	rl := &model.LLMRateLimitingConfig{
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Cost: &model.CostRateLimit{
					Enabled: true,
					Amount:  0.1,
					Reset:   model.RateLimitResetWindow{Duration: 2, Unit: "hour"},
				},
			},
		},
	}

	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(yaml, "consumerBased: true") {
		t.Error("expected consumerBased: true in generated YAML")
	}
	if !strings.Contains(yaml, "llm-cost-based-ratelimit") {
		t.Error("expected llm-cost-based-ratelimit policy in generated YAML")
	}

	t.Logf("Generated YAML:\n%s", yaml)
}

// TestGenerateYAML_BothBackendAndConsumerLimits verifies that when both a backend
// and a consumer limit are configured, two separate policies are generated — one
// without consumerBased and one with consumerBased: true.
func TestGenerateYAML_BothBackendAndConsumerLimits(t *testing.T) {
	count := 100
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Token: &model.TokenRateLimit{
					Enabled: true,
					Count:   count,
					Reset:   model.RateLimitResetWindow{Duration: 1, Unit: "hour"},
				},
			},
		},
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Token: &model.TokenRateLimit{
					Enabled: true,
					Count:   count,
					Reset:   model.RateLimitResetWindow{Duration: 1, Unit: "hour"},
				},
			},
		},
	}

	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have two token-based-ratelimit policies
	if strings.Count(yaml, "token-based-ratelimit") < 2 {
		t.Errorf("expected two token-based-ratelimit entries, got:\n%s", yaml)
	}
	// One must be consumer-based
	if !strings.Contains(yaml, "consumerBased: true") {
		t.Error("expected consumerBased: true in generated YAML")
	}

	t.Logf("Generated YAML:\n%s", yaml)
}

// ---------------------------------------------------------------------------
// Regression: backend-only limits (no consumer)
// ---------------------------------------------------------------------------

// TestGenerateYAML_BackendOnlyTokenLimit verifies that a backend-only token limit
// generates a token-based-ratelimit policy without consumerBased.
func TestGenerateYAML_BackendOnlyTokenLimit(t *testing.T) {
	count := 500
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Token: &model.TokenRateLimit{Enabled: true, Count: count, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}
	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(yaml, "token-based-ratelimit") {
		t.Error("expected token-based-ratelimit in generated YAML")
	}
	if strings.Contains(yaml, "consumerBased") {
		t.Error("expected no consumerBased for backend-only limit")
	}
	t.Logf("Generated YAML:\n%s", yaml)
}

// TestGenerateYAML_BackendOnlyRequestLimit verifies that a backend-only request limit
// generates an advanced-ratelimit policy with quota name "request-limit" (not "consumer-request-limit")
// and without x-wso2-application-id in the key extraction.
func TestGenerateYAML_BackendOnlyRequestLimit(t *testing.T) {
	count := 500
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Request: &model.RequestRateLimit{Enabled: true, Count: count, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}
	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(yaml, "advanced-ratelimit") {
		t.Error("expected advanced-ratelimit in generated YAML")
	}
	if strings.Contains(yaml, "x-wso2-application-id") {
		t.Error("expected no x-wso2-application-id for backend-only request limit")
	}
	if !strings.Contains(yaml, "request-limit") {
		t.Error("expected quota name 'request-limit' in generated YAML")
	}
	if strings.Contains(yaml, "consumer-request-limit") {
		t.Error("expected no 'consumer-request-limit' for backend-only request limit")
	}
	t.Logf("Generated YAML:\n%s", yaml)
}

// TestGenerateYAML_BackendOnlyCostLimit verifies that a backend-only cost limit
// generates an llm-cost-based-ratelimit policy without consumerBased, plus one llm-cost policy.
func TestGenerateYAML_BackendOnlyCostLimit(t *testing.T) {
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Cost: &model.CostRateLimit{Enabled: true, Amount: 1.0, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}
	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(yaml, "llm-cost-based-ratelimit") {
		t.Error("expected llm-cost-based-ratelimit in generated YAML")
	}
	if strings.Contains(yaml, "consumerBased") {
		t.Error("expected no consumerBased for backend-only cost limit")
	}
	if strings.Count(yaml, "llm-cost") < 2 {
		t.Error("expected llm-cost policy alongside llm-cost-based-ratelimit")
	}
	t.Logf("Generated YAML:\n%s", yaml)
}

// ---------------------------------------------------------------------------
// Backend + consumer for individual limit types
// ---------------------------------------------------------------------------

// TestGenerateYAML_BothBackendAndConsumerRequestLimits verifies that backend and consumer
// request limits produce two advanced-ratelimit policies with distinct quota names:
// "request-limit" (backend, no app-id key) and "consumer-request-limit" (consumer, with app-id key).
func TestGenerateYAML_BothBackendAndConsumerRequestLimits(t *testing.T) {
	count := 100
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Request: &model.RequestRateLimit{Enabled: true, Count: count, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Request: &model.RequestRateLimit{Enabled: true, Count: count, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}
	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(yaml, "advanced-ratelimit") < 2 {
		t.Error("expected two advanced-ratelimit policies (one backend, one consumer)")
	}
	if !strings.Contains(yaml, "consumer-request-limit") {
		t.Error("expected 'consumer-request-limit' quota name for consumer policy")
	}
	if !strings.Contains(yaml, "x-wso2-application-id") {
		t.Error("expected x-wso2-application-id in consumer policy key extraction")
	}
	t.Logf("Generated YAML:\n%s", yaml)
}

// TestGenerateYAML_BothBackendAndConsumerCostLimits verifies that backend and consumer
// cost limits produce two llm-cost-based-ratelimit policies (one with consumerBased: true)
// and exactly one llm-cost policy (not duplicated).
func TestGenerateYAML_BothBackendAndConsumerCostLimits(t *testing.T) {
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Cost: &model.CostRateLimit{Enabled: true, Amount: 1.0, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Cost: &model.CostRateLimit{Enabled: true, Amount: 0.1, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}
	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(yaml, "llm-cost-based-ratelimit") < 2 {
		t.Error("expected two llm-cost-based-ratelimit policies (backend + consumer)")
	}
	if !strings.Contains(yaml, "consumerBased: true") {
		t.Error("expected consumerBased: true on consumer cost policy")
	}
	// llm-cost must appear exactly once — hasPolicy check prevents duplication
	if strings.Count(yaml, "name: llm-cost\n") != 1 {
		t.Errorf("expected exactly one llm-cost policy, got:\n%s", yaml)
	}
	t.Logf("Generated YAML:\n%s", yaml)
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

// TestGenerateYAML_DisabledLimitIsSkipped verifies that a limit with Enabled: false
// produces no rate limiting policies.
func TestGenerateYAML_DisabledLimitIsSkipped(t *testing.T) {
	count := 100
	rl := &model.LLMRateLimitingConfig{
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Token: &model.TokenRateLimit{Enabled: false, Count: count, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
				Cost:  &model.CostRateLimit{Enabled: false, Amount: 0.5, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}
	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(yaml, "token-based-ratelimit") {
		t.Error("expected no token-based-ratelimit for disabled token limit")
	}
	if strings.Contains(yaml, "llm-cost-based-ratelimit") {
		t.Error("expected no llm-cost-based-ratelimit for disabled cost limit")
	}
	t.Logf("Generated YAML:\n%s", yaml)
}

// TestGenerateYAML_AllThreeConsumerLimits verifies the full UI scenario from the
// screenshot: consumer request + token + cost all enabled, no backend limits.
func TestGenerateYAML_AllThreeConsumerLimits(t *testing.T) {
	count := 100
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{},
		},
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Request: &model.RequestRateLimit{
					Enabled: true,
					Count:   count,
					Reset:   model.RateLimitResetWindow{Duration: 2, Unit: "hour"},
				},
				Token: &model.TokenRateLimit{
					Enabled: true,
					Count:   count,
					Reset:   model.RateLimitResetWindow{Duration: 2, Unit: "hour"},
				},
				Cost: &model.CostRateLimit{
					Enabled: true,
					Amount:  0.1,
					Reset:   model.RateLimitResetWindow{Duration: 2, Unit: "hour"},
				},
			},
		},
	}

	yaml, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"advanced-ratelimit",
		"token-based-ratelimit",
		"llm-cost-based-ratelimit",
		"consumerBased: true",
	}
	for _, want := range checks {
		if !strings.Contains(yaml, want) {
			t.Errorf("expected %q in generated YAML", want)
		}
	}

	t.Logf("Generated YAML:\n%s", yaml)
}
