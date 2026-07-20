package service

import (
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"gopkg.in/yaml.v3"
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

// TestGenerateLLMProviderDeploymentYAML_OtherAuthEmitsTypeOnly verifies that when the
// upstream auth type is "other", the deployment artifact emits an explicit auth block that
// carries only the type - no header/value. Authentication is handled by user-attached
// policies, so the gateway attaches no header-setting policy of its own.
func TestGenerateLLMProviderDeploymentYAML_OtherAuthEmitsTypeOnly(t *testing.T) {
	provider := &model.LLMProvider{
		ID:      "test-provider",
		Name:    "Test Provider",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Context: strPtr("/test"),
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL:  "https://api.anthropic.com",
					Auth: &model.UpstreamAuth{Type: "other"},
				},
			},
			AccessControl: &model.LLMAccessControl{Mode: "allow_all"},
		},
	}

	yamlArtifact, err := generateLLMProviderDeploymentYAML(provider, "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	auth := yamlArtifact.Spec.Upstream.Auth
	if auth == nil || auth.Type == nil || *auth.Type != "other" {
		t.Fatalf("expected upstream auth type 'other', got %+v", auth)
	}
	if auth.Header != nil || auth.Value != nil {
		t.Fatalf("expected no header/value for 'other', got header=%v value=%v", auth.Header, auth.Value)
	}
}

// TestGenerateLLMProviderDeploymentYAML_NoAuthDefaultsToNone verifies that a provider with no
// configured upstream auth deploys with an explicit auth type of "none".
func TestGenerateLLMProviderDeploymentYAML_NoAuthDefaultsToNone(t *testing.T) {
	provider := &model.LLMProvider{
		ID:      "test-provider",
		Name:    "Test Provider",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Context: strPtr("/test"),
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://api.anthropic.com"},
			},
			AccessControl: &model.LLMAccessControl{Mode: "allow_all"},
		},
	}

	yamlArtifact, err := generateLLMProviderDeploymentYAML(provider, "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	auth := yamlArtifact.Spec.Upstream.Auth
	if auth == nil || auth.Type == nil || *auth.Type != "none" {
		t.Fatalf("expected upstream auth type 'none' when no auth configured, got %+v", auth)
	}
	if auth.Header != nil || auth.Value != nil {
		t.Fatalf("expected no header/value for 'none', got header=%v value=%v", auth.Header, auth.Value)
	}
}

// TestGenerateLLMProxyDeploymentYAML_OtherAndNoneEmitTypeOnly is the LLM Proxy counterpart:
// "other" and absent auth (=> "none") both emit an explicit, credential-less type.
func TestGenerateLLMProxyDeploymentYAML_OtherAndNoneEmitTypeOnly(t *testing.T) {
	cases := []struct {
		name     string
		auth     *model.UpstreamAuth
		wantType string
	}{
		{name: "other", auth: &model.UpstreamAuth{Type: "other"}, wantType: "other"},
		{name: "absent defaults to none", auth: nil, wantType: "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proxy := &model.LLMProxy{
				ID:      "test-proxy",
				Name:    "Test Proxy",
				Version: "v1.0",
				Configuration: model.LLMProxyConfig{
					Provider:     "test-provider",
					UpstreamAuth: tc.auth,
				},
			}

			yamlArtifact, err := generateLLMProxyDeploymentYAML(proxy)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			auth := yamlArtifact.Spec.Provider.Auth
			if auth == nil || auth.Type == nil || string(*auth.Type) != tc.wantType {
				t.Fatalf("expected provider auth type %q, got %+v", tc.wantType, auth)
			}
			if auth.Header != nil || auth.Value != nil {
				t.Fatalf("expected no header/value for %q, got header=%v value=%v", tc.wantType, auth.Header, auth.Value)
			}
		})
	}
}

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

// TestGenerateYAML_ConsumerRequestLimit verifies that a consumer provider-wide request limit
// is emitted as a GLOBAL advanced-ratelimit policy keyed on apiname + x-wso2-application-id
// — one shared bucket per consumer across all routes.
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

	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	if !strings.Contains(yamlStr, "advanced-ratelimit") {
		t.Error("expected advanced-ratelimit policy in generated YAML")
	}
	// Consumer-scoped: key extraction must include x-wso2-application-id
	if !strings.Contains(yamlStr, "x-wso2-application-id") {
		t.Error("expected x-wso2-application-id in key extraction for consumer request limit")
	}
	// Provider-wide (global) per-consumer bucket must key on apiname, not routename,
	// so it is shared across all routes for a given consumer.
	if !strings.Contains(yamlStr, "apiname") {
		t.Errorf("expected apiname key extraction for consumer GLOBAL request limit:\n%s", yamlStr)
	}
	if strings.Contains(yamlStr, "routename") {
		t.Errorf("expected no routename key for consumer GLOBAL request limit (would silo per route):\n%s", yamlStr)
	}
	// A provider-wide per-consumer limit is a GLOBAL policy: emitted into globalPolicies,
	// never operationPolicies.
	if findGlobalPolicy(yamlArtifact.Spec.GlobalPolicies, "advanced-ratelimit") == nil {
		t.Errorf("expected consumer global request in globalPolicies:\n%s", yamlStr)
	}
	if findOperationPolicy(yamlArtifact.Spec.OperationPolicies, "advanced-ratelimit") != nil {
		t.Errorf("expected consumer global request NOT in operationPolicies:\n%s", yamlStr)
	}
	// Should NOT have a backend (non-consumer) advanced-ratelimit entry
	if strings.Count(yamlStr, "advanced-ratelimit") > 1 {
		t.Error("expected only one advanced-ratelimit policy (consumer), got more than one")
	}
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

	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	if !strings.Contains(yamlStr, "consumerBased: true") {
		t.Error("expected consumerBased: true in generated YAML")
	}
	if !strings.Contains(yamlStr, "token-based-ratelimit") {
		t.Error("expected token-based-ratelimit policy in generated YAML")
	}

	t.Logf("Generated YAML:\n%s", yamlStr)
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

	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	if !strings.Contains(yamlStr, "consumerBased: true") {
		t.Error("expected consumerBased: true in generated YAML")
	}
	if !strings.Contains(yamlStr, "llm-cost-based-ratelimit") {
		t.Error("expected llm-cost-based-ratelimit policy in generated YAML")
	}

	t.Logf("Generated YAML:\n%s", yamlStr)
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

	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	// Should have two token-based-ratelimit policies
	if strings.Count(yamlStr, "token-based-ratelimit") < 2 {
		t.Errorf("expected two token-based-ratelimit entries, got:\n%s", yamlStr)
	}
	// One must be consumer-based
	if !strings.Contains(yamlStr, "consumerBased: true") {
		t.Error("expected consumerBased: true in generated YAML")
	}

	t.Logf("Generated YAML:\n%s", yamlStr)
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
	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)
	if !strings.Contains(yamlStr, "token-based-ratelimit") {
		t.Error("expected token-based-ratelimit in generated YAML")
	}
	if strings.Contains(yamlStr, "consumerBased") {
		t.Error("expected no consumerBased for backend-only limit")
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
}

// TestGenerateYAML_BackendOnlyRequestLimit verifies that a backend (provider-level) global
// request limit produces a basic-ratelimit global policy with a plain {requests, duration}
// limit — basic-ratelimit shares one bucket across all routes by keying on the API by
// default, so no keyExtraction or per-consumer key is part of its shape.
func TestGenerateYAML_BackendOnlyRequestLimit(t *testing.T) {
	count := 500
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Request: &model.RequestRateLimit{Enabled: true, Count: count, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}
	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(yamlArtifact.Spec.GlobalPolicies) != 1 {
		t.Fatalf("expected exactly one global policy, got: %d", len(yamlArtifact.Spec.GlobalPolicies))
	}
	requestPolicy := findGlobalPolicy(yamlArtifact.Spec.GlobalPolicies, "basic-ratelimit")
	if requestPolicy == nil {
		t.Fatalf("expected basic-ratelimit global policy, got: %+v", yamlArtifact.Spec.GlobalPolicies)
	}
	if requestPolicy.Params == nil {
		t.Fatalf("expected basic-ratelimit policy to have params")
	}
	if len(*requestPolicy.Params) != 1 {
		t.Fatalf("expected params to contain only 'limits', got: %#v", *requestPolicy.Params)
	}
	limits, ok := (*requestPolicy.Params)["limits"].([]map[string]interface{})
	if !ok || len(limits) != 1 {
		t.Fatalf("expected limits with one entry, got: %#v", (*requestPolicy.Params)["limits"])
	}
	if limits[0]["requests"] != count {
		t.Errorf("expected requests %d, got: %#v", count, limits[0]["requests"])
	}
	if limits[0]["duration"] != "1h" {
		t.Errorf("expected duration 1h, got: %#v", limits[0]["duration"])
	}
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
	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)
	if !strings.Contains(yamlStr, "llm-cost-based-ratelimit") {
		t.Error("expected llm-cost-based-ratelimit in generated YAML")
	}
	if strings.Contains(yamlStr, "consumerBased") {
		t.Error("expected no consumerBased for backend-only cost limit")
	}
	if strings.Count(yamlStr, "llm-cost") < 2 {
		t.Error("expected llm-cost policy alongside llm-cost-based-ratelimit")
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
}

func TestGenerateYAML_BackendResourceWiseDefaultCostLimit(t *testing.T) {
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			ResourceWise: &model.ResourceWiseRateLimitingConfig{
				Default: model.RateLimitingLimitConfig{
					Cost: &model.CostRateLimit{Enabled: true, Amount: 0.10, Reset: model.RateLimitResetWindow{Duration: 24, Unit: "hour"}},
				},
			},
		},
	}
	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)
	if !strings.Contains(yamlStr, "llm-cost-based-ratelimit") {
		t.Error("expected llm-cost-based-ratelimit in generated YAML")
	}
	if !strings.Contains(yamlStr, "budgetLimits") {
		t.Error("expected budgetLimits in generated YAML")
	}
	if strings.Contains(yamlStr, "consumerBased") {
		t.Error("expected no consumerBased for backend-only cost limit")
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
}

func TestGenerateYAML_BackendPerResourceCostLimit(t *testing.T) {
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			ResourceWise: &model.ResourceWiseRateLimitingConfig{
				Default: model.RateLimitingLimitConfig{},
				Resources: []model.RateLimitingResourceLimit{
					{
						Resource: "/v1/messages",
						Limit: model.RateLimitingLimitConfig{
							Cost: &model.CostRateLimit{Enabled: true, Amount: 0.02, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
						},
					},
				},
			},
		},
	}
	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)
	if !strings.Contains(yamlStr, "llm-cost-based-ratelimit") {
		t.Error("expected llm-cost-based-ratelimit in generated YAML")
	}
	if !strings.Contains(yamlStr, "budgetLimits") {
		t.Error("expected budgetLimits in generated YAML")
	}
	if !strings.Contains(yamlStr, "/v1/messages") {
		t.Error("expected resource path /v1/messages in generated YAML")
	}
	if strings.Contains(yamlStr, "consumerBased") {
		t.Error("expected no consumerBased for backend-only cost limit")
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
}

// ---------------------------------------------------------------------------
// Backend + consumer for individual limit types
// ---------------------------------------------------------------------------

// TestGenerateYAML_BothBackendAndConsumerRequestLimits verifies that a backend request
// limit and a consumer request limit produce distinct policies: the backend limit is a
// basic-ratelimit (apiname-shared, no keyExtraction) while the consumer limit remains an
// advanced-ratelimit with quota name "consumer-request-limit" keyed on x-wso2-application-id.
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
	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)
	if strings.Count(yamlStr, "advanced-ratelimit") != 1 {
		t.Error("expected exactly one advanced-ratelimit policy (consumer only)")
	}
	if !strings.Contains(yamlStr, "basic-ratelimit") {
		t.Error("expected a basic-ratelimit policy for the backend request limit")
	}
	if !strings.Contains(yamlStr, "consumer-request-limit") {
		t.Error("expected 'consumer-request-limit' quota name for consumer policy")
	}
	if !strings.Contains(yamlStr, "x-wso2-application-id") {
		t.Error("expected x-wso2-application-id in consumer policy key extraction")
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
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
	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)
	if strings.Count(yamlStr, "llm-cost-based-ratelimit") < 2 {
		t.Error("expected two llm-cost-based-ratelimit policies (backend + consumer)")
	}
	if !strings.Contains(yamlStr, "consumerBased: true") {
		t.Error("expected consumerBased: true on consumer cost policy")
	}
	// llm-cost must appear exactly once — hasPolicy check prevents duplication
	if strings.Count(yamlStr, "name: llm-cost\n") != 1 {
		t.Errorf("expected exactly one llm-cost policy, got:\n%s", yamlStr)
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
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
	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)
	if strings.Contains(yamlStr, "token-based-ratelimit") {
		t.Error("expected no token-based-ratelimit for disabled token limit")
	}
	if strings.Contains(yamlStr, "llm-cost-based-ratelimit") {
		t.Error("expected no llm-cost-based-ratelimit for disabled cost limit")
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
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

	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	checks := []string{
		"advanced-ratelimit",
		"token-based-ratelimit",
		"llm-cost-based-ratelimit",
		"consumerBased: true",
	}
	for _, want := range checks {
		if !strings.Contains(yamlStr, want) {
			t.Errorf("expected %q in generated YAML", want)
		}
	}

	t.Logf("Generated YAML:\n%s", yamlStr)
}

// TestRoundTrip_ConsumerGlobalRequestLimit verifies the consumer global request limit
// survives the CP->DP forward conversion (now via globalPolicies) and the DP->CP import,
// landing back in ConsumerLevel.Global.Request.
func TestRoundTrip_ConsumerGlobalRequestLimit(t *testing.T) {
	rl := &model.LLMRateLimitingConfig{
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Request: &model.RequestRateLimit{Enabled: true, Count: 100, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}
	out, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := mapLLMProviderSpecToConfig(out.Spec)
	if cfg.RateLimiting == nil || cfg.RateLimiting.ConsumerLevel == nil || cfg.RateLimiting.ConsumerLevel.Global == nil ||
		cfg.RateLimiting.ConsumerLevel.Global.Request == nil {
		t.Fatalf("consumer global request not reconstructed: %+v", cfg.RateLimiting)
	}
	req := cfg.RateLimiting.ConsumerLevel.Global.Request
	if !req.Enabled || req.Count != 100 || req.Reset.Unit != "hour" || req.Reset.Duration != 1 {
		t.Errorf("consumer global request = %+v, want enabled count=100 reset=1h", req)
	}
	// It must NOT be misclassified as a provider-level (backend) limit.
	if cfg.RateLimiting.ProviderLevel != nil {
		t.Errorf("expected no provider-level limit, got: %+v", cfg.RateLimiting.ProviderLevel)
	}
}

// TestGenerateYAML_ConsumerResourceWiseRequestLimitKeepsRoutename verifies that a
// consumer RESOURCE-WISE request limit stays keyed on routename + application id (an
// independent per-route, per-consumer bucket) — the counterpart to the apiname-keyed
// consumer global limit.
func TestGenerateYAML_ConsumerResourceWiseRequestLimitKeepsRoutename(t *testing.T) {
	rl := &model.LLMRateLimitingConfig{
		ConsumerLevel: &model.RateLimitingScopeConfig{
			ResourceWise: &model.ResourceWiseRateLimitingConfig{
				Resources: []model.RateLimitingResourceLimit{
					{
						Resource: "/v1/chat",
						Limit: model.RateLimitingLimitConfig{
							Request: &model.RequestRateLimit{Enabled: true, Count: 50, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
						},
					},
				},
			},
		},
	}
	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)
	if !strings.Contains(yamlStr, "routename") {
		t.Errorf("expected routename key for consumer resource-wise request limit:\n%s", yamlStr)
	}
	if strings.Contains(yamlStr, "apiname") {
		t.Errorf("expected no apiname key for consumer resource-wise request limit:\n%s", yamlStr)
	}
	if !strings.Contains(yamlStr, "x-wso2-application-id") {
		t.Errorf("expected x-wso2-application-id key for consumer resource-wise request limit:\n%s", yamlStr)
	}
}

// ---------------------------------------------------------------------------
// llm-cost deduplication
// ---------------------------------------------------------------------------

// providerWithCostRLAndDefaultPolicies builds a provider with a cost-based rate limit
// configured AND llm-cost already in Configuration.Policies — which is exactly what
// happens in production when the frontend attaches llm-cost by default at creation time
// and the user later enables cost-based rate limiting via the rate limit tab.
func providerWithCostRLAndDefaultPolicies(rl *model.LLMRateLimitingConfig) *model.LLMProvider {
	p := providerWithConsumerLimits(rl)
	p.Configuration.Policies = []model.LLMPolicy{
		{
			Name:    "llm-cost",
			Version: "v1",
			Paths:   []model.LLMPolicyPath{{Path: "/*", Methods: []string{"*"}, Params: map[string]interface{}{}}},
		},
	}
	return p
}

// TestGenerateYAML_LLMCostNotDuplicatedWithProviderCostLimit verifies that when a
// backend cost limit is configured (auto-adds llm-cost) and llm-cost is also present
// in Configuration.Policies (added by the frontend by default), the final YAML
// contains llm-cost exactly once.
func TestGenerateYAML_LLMCostNotDuplicatedWithProviderCostLimit(t *testing.T) {
	rl := &model.LLMRateLimitingConfig{
		ProviderLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Cost: &model.CostRateLimit{Enabled: true, Amount: 1.0, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}

	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	if strings.Count(yamlStr, "name: llm-cost\n") != 1 {
		t.Errorf("expected exactly one llm-cost policy, got:\n%s", yamlStr)
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
}

// TestGenerateYAML_LLMCostNotDuplicatedWithConsumerCostLimit verifies the same
// deduplication holds when the cost limit is on the consumer level.
func TestGenerateYAML_LLMCostNotDuplicatedWithConsumerCostLimit(t *testing.T) {
	rl := &model.LLMRateLimitingConfig{
		ConsumerLevel: &model.RateLimitingScopeConfig{
			Global: &model.RateLimitingLimitConfig{
				Cost: &model.CostRateLimit{Enabled: true, Amount: 0.5, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
			},
		},
	}

	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	if strings.Count(yamlStr, "name: llm-cost\n") != 1 {
		t.Errorf("expected exactly one llm-cost policy, got:\n%s", yamlStr)
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
}

// TestGenerateYAML_LLMCostNotDuplicatedWithBothProviderAndConsumerCostLimits verifies
// that even when both backend and consumer cost limits are configured, llm-cost
// still appears only once alongside two llm-cost-based-ratelimit entries.
func TestGenerateYAML_LLMCostNotDuplicatedWithBothProviderAndConsumerCostLimits(t *testing.T) {
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

	yamlArtifact, err := generateLLMProviderDeploymentYAML(providerWithConsumerLimits(rl), "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	if strings.Count(yamlStr, "name: llm-cost\n") != 1 {
		t.Errorf("expected exactly one llm-cost policy, got:\n%s", yamlStr)
	}
	if strings.Count(yamlStr, "llm-cost-based-ratelimit") < 2 {
		t.Errorf("expected two llm-cost-based-ratelimit policies (backend + consumer), got:\n%s", yamlStr)
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
}

// TestGenerateYAML_LLMCostKeptWhenNoCostRLConfigured verifies that when no cost-based
// rate limit is configured, the llm-cost policy from Configuration.Policies is still
// included in the output (it should only be skipped if already added by the RL block).
func TestGenerateYAML_LLMCostKeptWhenNoCostRLConfigured(t *testing.T) {
	p := providerWithConsumerLimits(nil)
	p.Configuration.Policies = []model.LLMPolicy{
		{Name: "llm-cost", Version: "v1", Paths: []model.LLMPolicyPath{
			{Path: "/*", Methods: []string{"*"}, Params: map[string]interface{}{}},
		}},
	}
	yamlArtifact, err := generateLLMProviderDeploymentYAML(p, "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	if strings.Count(yamlStr, "name: llm-cost\n") != 1 {
		t.Errorf("expected exactly one llm-cost policy when no RL is configured, got:\n%s", yamlStr)
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
}

// TestGenerateYAML_OtherCustomPoliciesNotDeduplicated verifies that the llm-cost
// deduplication does not affect other guardrail policies — two entries with the same
// name but different params must both be preserved.
func TestGenerateYAML_OtherCustomPoliciesNotDeduplicated(t *testing.T) {
	p := providerWithConsumerLimits(nil)
	p.Configuration.Policies = []model.LLMPolicy{
		{
			Name:    "my-guardrail",
			Version: "v1",
			Paths:   []model.LLMPolicyPath{{Path: "/*", Methods: []string{"*"}, Params: map[string]interface{}{"threshold": 0.5}}},
		},
		{
			Name:    "my-guardrail",
			Version: "v1",
			Paths:   []model.LLMPolicyPath{{Path: "/*", Methods: []string{"*"}, Params: map[string]interface{}{"threshold": 0.9}}},
		},
	}

	yamlArtifact, err := generateLLMProviderDeploymentYAML(p, "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yamlBytes, _ := yaml.Marshal(yamlArtifact)
	yamlStr := string(yamlBytes)

	if strings.Count(yamlStr, "name: my-guardrail") < 2 {
		t.Errorf("expected both my-guardrail entries to be present, got:\n%s", yamlStr)
	}
	t.Logf("Generated YAML:\n%s", yamlStr)
}
