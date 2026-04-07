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

func TestGenerateLLMProviderDeploymentYAML_CostRateLimitGlobal(t *testing.T) {
	context := "/test"
	enabled := true
	amount := 0.05
	provider := &model.LLMProvider{
		ID:   "test-provider",
		Name: "Test Provider",
		Configuration: model.LLMProviderConfig{
			Context: &context,
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://api.anthropic.com"},
			},
			RateLimiting: &model.LLMRateLimitingConfig{
				ProviderLevel: &model.RateLimitingScopeConfig{
					Global: &model.RateLimitingLimitConfig{
						Cost: &model.CostRateLimit{
							Enabled: enabled,
							Amount:  amount,
							Reset:   model.RateLimitResetWindow{Duration: 1, Unit: "hour"},
						},
					},
				},
			},
		},
	}

	yaml, err := generateLLMProviderDeploymentYAML(provider, "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(yaml, llmCostBasedRateLimitPolicyName) {
		t.Errorf("expected YAML to contain %q, got:\n%s", llmCostBasedRateLimitPolicyName, yaml)
	}
	if !strings.Contains(yaml, "budgetLimits") {
		t.Errorf("expected YAML to contain budgetLimits, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, llmCostPolicyName) {
		t.Errorf("expected YAML to contain %q, got:\n%s", llmCostPolicyName, yaml)
	}
}

func TestGenerateLLMProviderDeploymentYAML_CostRateLimitResourceWiseDefault(t *testing.T) {
	context := "/test"
	enabled := true
	amount := 0.10
	provider := &model.LLMProvider{
		ID:   "test-provider",
		Name: "Test Provider",
		Configuration: model.LLMProviderConfig{
			Context: &context,
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://api.anthropic.com"},
			},
			RateLimiting: &model.LLMRateLimitingConfig{
				ProviderLevel: &model.RateLimitingScopeConfig{
					ResourceWise: &model.ResourceWiseRateLimitingConfig{
						Default: model.RateLimitingLimitConfig{
							Cost: &model.CostRateLimit{
								Enabled: enabled,
								Amount:  amount,
								Reset:   model.RateLimitResetWindow{Duration: 24, Unit: "hour"},
							},
						},
					},
				},
			},
		},
	}

	yaml, err := generateLLMProviderDeploymentYAML(provider, "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(yaml, llmCostBasedRateLimitPolicyName) {
		t.Errorf("expected YAML to contain %q, got:\n%s", llmCostBasedRateLimitPolicyName, yaml)
	}
	if !strings.Contains(yaml, "budgetLimits") {
		t.Errorf("expected YAML to contain budgetLimits, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, llmCostPolicyName) {
		t.Errorf("expected YAML to contain %q, got:\n%s", llmCostPolicyName, yaml)
	}
}

func TestGenerateLLMProviderDeploymentYAML_CostRateLimitPerResource(t *testing.T) {
	context := "/test"
	enabled := true
	amount := 0.02
	provider := &model.LLMProvider{
		ID:   "test-provider",
		Name: "Test Provider",
		Configuration: model.LLMProviderConfig{
			Context: &context,
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://api.anthropic.com"},
			},
			RateLimiting: &model.LLMRateLimitingConfig{
				ProviderLevel: &model.RateLimitingScopeConfig{
					ResourceWise: &model.ResourceWiseRateLimitingConfig{
						Default: model.RateLimitingLimitConfig{},
						Resources: []model.RateLimitingResourceLimit{
							{
								Resource: "/v1/messages",
								Limit: model.RateLimitingLimitConfig{
									Cost: &model.CostRateLimit{
										Enabled: enabled,
										Amount:  amount,
										Reset:   model.RateLimitResetWindow{Duration: 1, Unit: "hour"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	yaml, err := generateLLMProviderDeploymentYAML(provider, "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(yaml, llmCostBasedRateLimitPolicyName) {
		t.Errorf("expected YAML to contain %q, got:\n%s", llmCostBasedRateLimitPolicyName, yaml)
	}
	if !strings.Contains(yaml, "budgetLimits") {
		t.Errorf("expected YAML to contain budgetLimits, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, llmCostPolicyName) {
		t.Errorf("expected YAML to contain %q, got:\n%s", llmCostPolicyName, yaml)
	}
	if !strings.Contains(yaml, "/v1/messages") {
		t.Errorf("expected YAML to contain resource path /v1/messages, got:\n%s", yaml)
	}
}
