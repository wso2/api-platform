package service

import (
	"log/slog"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"gopkg.in/yaml.v3"
)

// roundTripYAML marshals a dto artifact to YAML and back, normalising all
// typed slices (e.g. []map[string]interface{}) to []interface{} the way a real
// YAML round-trip does. Tests that inspect nested policy params use this to
// keep their type assertions consistent with pre-Phase-8 behaviour.
func roundTripYAML(t *testing.T, artifact dto.LLMProviderDeploymentYAML) dto.LLMProviderDeploymentYAML {
	t.Helper()
	b, err := yaml.Marshal(artifact)
	if err != nil {
		t.Fatalf("roundTripYAML marshal: %v", err)
	}
	var out dto.LLMProviderDeploymentYAML
	if err := yaml.Unmarshal(b, &out); err != nil {
		t.Fatalf("roundTripYAML unmarshal: %v", err)
	}
	return out
}

func TestMapTemplateResourceMappingAPI_RejectsEmptyResource(t *testing.T) {
	mapped, err := mapTemplateResourceMappingAPI(&api.LLMProviderTemplateResourceMapping{Resource: "   "})
	if err == nil {
		t.Fatal("expected error for empty resource")
	}
	if mapped != nil {
		t.Fatal("expected mapped resource to be nil when validation fails")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestMapTemplateResourceMappingsAPI_StopsOnInvalidResource(t *testing.T) {
	resources := []api.LLMProviderTemplateResourceMapping{
		{Resource: "chat.completions"},
		{Resource: "\t\n"},
	}

	mapped, err := mapTemplateResourceMappingsAPI(&api.LLMProviderTemplateResourceMappings{Resources: &resources})
	if err == nil {
		t.Fatal("expected error for invalid resource in mappings")
	}
	if mapped != nil {
		t.Fatal("expected mapped resources to be nil when validation fails")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestNormalizeUpstreamAuthType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "api key camel case", input: "apiKey", expected: "api-key"},
		{name: "api key kebab case", input: "api-key", expected: "api-key"},
		{name: "api key upper with underscore", input: "API_KEY", expected: "api-key"},
		{name: "basic", input: "basic", expected: "basic"},
		{name: "bearer", input: "bearer", expected: "bearer"},
		{name: "unknown preserved", input: "custom", expected: "custom"},
		{name: "empty", input: "", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := normalizeUpstreamAuthType(tc.input)
			if actual != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestMapUpstreamAuthAPIToModel_NormalizesApiKeyType(t *testing.T) {
	authType := api.UpstreamAuthType("apiKey")
	in := &api.UpstreamAuth{
		Type:   &authType,
		Header: utils.StringPtrIfNotEmpty("Authorization"),
		Value:  utils.StringPtrIfNotEmpty("secret"),
	}

	out := mapUpstreamAuthAPIToModel(in)
	if out == nil {
		t.Fatal("expected output auth to be non-nil")
	}
	if out.Type != "api-key" {
		t.Fatalf("expected auth type to be normalized to api-key, got %q", out.Type)
	}
}

func TestPreserveUpstreamAuthValue(t *testing.T) {
	existing := &model.UpstreamConfig{
		Main: &model.UpstreamEndpoint{
			URL: "https://example.com",
			Auth: &model.UpstreamAuth{
				Type:   "api-key",
				Header: "Authorization",
				Value:  "secret",
			},
		},
	}

	t.Run("updated nil returns existing", func(t *testing.T) {
		out := preserveUpstreamAuthValue(existing, nil)
		if out != existing {
			t.Fatalf("expected existing config to be preserved")
		}
	})

	t.Run("existing nil returns updated", func(t *testing.T) {
		updated := &model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "https://new.example"}}
		out := preserveUpstreamAuthValue(nil, updated)
		if out != updated {
			t.Fatalf("expected updated config to be returned")
		}
	})

	t.Run("missing main preserves existing", func(t *testing.T) {
		updated := &model.UpstreamConfig{}
		out := preserveUpstreamAuthValue(existing, updated)
		if out != existing {
			t.Fatalf("expected existing config to be preserved when main is nil")
		}
	})

	t.Run("missing auth clears existing auth", func(t *testing.T) {
		updated := &model.UpstreamConfig{
			Main: &model.UpstreamEndpoint{URL: "https://example.com"},
		}
		out := preserveUpstreamAuthValue(existing, updated)
		if out.Main == nil {
			t.Fatalf("expected main upstream to be present")
		}
		if out.Main.Auth != nil {
			t.Fatalf("expected auth to be cleared when auth object is omitted")
		}
	})

	t.Run("empty auth value preserves existing", func(t *testing.T) {
		updated := &model.UpstreamConfig{
			Main: &model.UpstreamEndpoint{
				URL:  "https://example.com",
				Auth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: ""},
			},
		}
		out := preserveUpstreamAuthValue(existing, updated)
		if out.Main.Auth.Value != "secret" {
			t.Fatalf("expected auth value to be preserved")
		}
	})
}

func TestMapUpstreamConfigToDTO_DoesNotExposeAuthValue(t *testing.T) {
	in := &model.UpstreamConfig{
		Main: &model.UpstreamEndpoint{
			URL: "https://example.com",
			Auth: &model.UpstreamAuth{
				Type:   "api-key",
				Header: "Authorization",
				Value:  "super-secret",
			},
		},
		Sandbox: &model.UpstreamEndpoint{
			URL: "https://sandbox.example.com",
			Auth: &model.UpstreamAuth{
				Type:   "api-key",
				Header: "Authorization",
				Value:  "sandbox-secret",
			},
		},
	}

	out := mapUpstreamConfigToDTO(in)
	if out.Main.Auth == nil {
		t.Fatalf("expected main auth to be present")
	}
	if out.Main.Auth.Value != nil && *out.Main.Auth.Value != "" {
		t.Fatalf("expected main auth value to be redacted")
	}
	if out.Sandbox == nil || out.Sandbox.Auth == nil {
		t.Fatalf("expected sandbox auth to be present")
	}
	if out.Sandbox.Auth.Value != nil && *out.Sandbox.Auth.Value != "" {
		t.Fatalf("expected sandbox auth value to be redacted")
	}
}

func TestMapProviderModelToAPI_DoesNotExposeUpstreamAuthValue(t *testing.T) {
	in := &model.LLMProvider{
		ID:      "provider-1",
		Name:    "Provider One",
		Version: "v1",
		Configuration: model.LLMProviderConfig{
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "https://example.com",
					Auth: &model.UpstreamAuth{
						Type:   "api-key",
						Header: "Authorization",
						Value:  "super-secret",
					},
				},
				Sandbox: &model.UpstreamEndpoint{
					URL: "https://sandbox.example.com",
					Auth: &model.UpstreamAuth{
						Type:   "bearer",
						Header: "Authorization",
						Value:  "sandbox-secret",
					},
				},
			},
		},
	}

	out := mapProviderModelToAPI(in, "template-1")
	if out.Upstream.Main.Auth == nil {
		t.Fatalf("expected upstream main auth to be present")
	}
	if out.Upstream.Main.Auth.Value != nil && *out.Upstream.Main.Auth.Value != "" {
		t.Fatalf("expected upstream main auth value to be redacted")
	}
	if out.Upstream.Sandbox == nil || out.Upstream.Sandbox.Auth == nil {
		t.Fatalf("expected upstream sandbox auth to be present")
	}
	if out.Upstream.Sandbox.Auth.Value != nil && *out.Upstream.Sandbox.Auth.Value != "" {
		t.Fatalf("expected upstream sandbox auth value to be redacted")
	}
}

func TestMapProxyModelToAPI_DoesNotExposeProviderAuthValue(t *testing.T) {
	in := &model.LLMProxy{
		ID:      "proxy-1",
		Name:    "Proxy One",
		Version: "v1",
		Configuration: model.LLMProxyConfig{
			Provider: "provider-1",
			UpstreamAuth: &model.UpstreamAuth{
				Type:   "api-key",
				Header: "Authorization",
				Value:  "super-secret-proxy",
			},
		},
	}

	out := mapProxyModelToAPI(in)
	if out.Provider.Auth == nil {
		t.Fatalf("expected provider auth to be present")
	}
	if out.Provider.Auth.Value != nil && *out.Provider.Auth.Value != "" {
		t.Fatalf("expected provider auth value to be redacted")
	}
}

func TestValidateLLMResourceLimit(t *testing.T) {
	t.Run("below limit should pass", func(t *testing.T) {
		err := validateLLMResourceLimit(4, 5, apperror.LLMProviderLimitReached.New())
		if err != nil {
			t.Fatalf("expected no error below limit, got: %v", err)
		}
	})

	t.Run("at limit should fail", func(t *testing.T) {
		err := validateLLMResourceLimit(5, 5, apperror.LLMProviderLimitReached.New())
		if !apperror.LLMProviderLimitReached.Is(err) {
			t.Fatalf("expected ErrLLMProviderLimitReached, got: %v", err)
		}
	})

	t.Run("above limit should fail", func(t *testing.T) {
		err := validateLLMResourceLimit(6, 5, apperror.LLMProxyLimitReached.New())
		if !apperror.LLMProxyLimitReached.Is(err) {
			t.Fatalf("expected ErrLLMProxyLimitReached, got: %v", err)
		}
	})

	t.Run("unlimited (limit <= 0) should always pass", func(t *testing.T) {
		for _, limit := range []int{0, -1} {
			if err := validateLLMResourceLimit(1_000_000, limit, apperror.LLMProviderLimitReached.New()); err != nil {
				t.Fatalf("expected no error for unlimited (limit=%d), got: %v", limit, err)
			}
		}
	})
}

func TestGenerateLLMProviderDeploymentYAML_WithSecurityAPIKeyPolicy(t *testing.T) {
	trueValue := true

	provider := &model.LLMProvider{
		ID:             "tt",
		Name:           "tt",
		Description:    "",
		Version:        "v1.0",
		OpenAPISpec:    "openapi: 3.0.0\n",
		ModelProviders: []model.LLMModelProvider{},
		Configuration: model.LLMProviderConfig{
			Context:  strPtr("/"),
			Template: "openai",
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "https://api.openai.com",
					Ref: "",
					Auth: &model.UpstreamAuth{
						Type:   "apiKey",
						Header: "Authorization",
						Value:  "Bearer tt",
					},
				},
			},
			AccessControl: &model.LLMAccessControl{
				Mode:       "allow_all",
				Exceptions: []model.RouteException{},
			},
			Policies:     []model.LLMPolicy{},
			RateLimiting: &model.LLMRateLimitingConfig{},
			Security: &model.SecurityConfig{
				Enabled: &trueValue,
				APIKey: &model.APIKeySecurity{
					Enabled: &trueValue,
					Key:     "X-API-Key",
					In:      "header",
				},
			},
		},
	}

	out, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if out.Metadata.Name != "tt" {
		t.Fatalf("expected metadata name tt, got: %s", out.Metadata.Name)
	}
	if out.Spec.DisplayName != "tt" {
		t.Fatalf("expected displayName tt, got: %s", out.Spec.DisplayName)
	}
	if out.Spec.Version != "v1.0" {
		t.Fatalf("expected version v1.0, got: %s", out.Spec.Version)
	}
	if out.Spec.Context != "/" {
		t.Fatalf("expected context '/', got: %s", out.Spec.Context)
	}
	if out.Spec.Template != "openai" {
		t.Fatalf("expected template openai, got: %s", out.Spec.Template)
	}
	if out.Spec.Upstream.URL != "https://api.openai.com" {
		t.Fatalf("expected upstream url https://api.openai.com, got: %s", out.Spec.Upstream.URL)
	}
	if out.Spec.Upstream.Auth == nil {
		t.Fatalf("expected upstream auth to be present")
	}
	if out.Spec.Upstream.Auth.Header == nil || *out.Spec.Upstream.Auth.Header != "Authorization" {
		t.Fatalf("expected upstream auth header Authorization")
	}
	if out.Spec.Upstream.Auth.Value == nil || *out.Spec.Upstream.Auth.Value != "Bearer tt" {
		t.Fatalf("expected upstream auth value Bearer tt")
	}

	if out.Spec.AccessControl.Mode != "allow_all" {
		t.Fatalf("expected access control mode allow_all, got: %s", out.Spec.AccessControl.Mode)
	}

	if len(out.Spec.OperationPolicies) != 0 {
		t.Fatalf("expected 0 operation policies, got: %d", len(out.Spec.OperationPolicies))
	}
	if len(out.Spec.GlobalPolicies) != 1 {
		t.Fatalf("expected 1 global policy, got: %d", len(out.Spec.GlobalPolicies))
	}

	policy := out.Spec.GlobalPolicies[0]
	if policy.Name != "api-key-auth" {
		t.Fatalf("expected policy name api-key-auth, got: %s", policy.Name)
	}
	if policy.Version != "" {
		t.Fatalf("expected policy version empty, got: %s", policy.Version)
	}
	if policy.Params == nil {
		t.Fatalf("expected policy params to be present")
	}
	if (*policy.Params)["key"] != "X-API-Key" {
		t.Fatalf("expected params.key X-API-Key, got: %#v", (*policy.Params)["key"])
	}
	if (*policy.Params)["in"] != "header" {
		t.Fatalf("expected params.in header, got: %#v", (*policy.Params)["in"])
	}
}

func TestGenerateLLMProviderDeploymentYAML_WithSecurityAndAdditionalPolicy(t *testing.T) {
	trueValue := true

	provider := &model.LLMProvider{
		ID:             "tt",
		Name:           "tt",
		Version:        "v1.0",
		OpenAPISpec:    "openapi: 3.0.0\n",
		ModelProviders: []model.LLMModelProvider{},
		Configuration: model.LLMProviderConfig{
			Context:  strPtr("/"),
			Template: "openai",
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "https://api.openai.com",
					Auth: &model.UpstreamAuth{
						Type:   "apiKey",
						Header: "Authorization",
						Value:  "Bearer tt",
					},
				},
			},
			AccessControl: &model.LLMAccessControl{Mode: "allow_all"},
			Security: &model.SecurityConfig{
				Enabled: &trueValue,
				APIKey: &model.APIKeySecurity{
					Enabled: &trueValue,
					Key:     "X-API-Key",
					In:      "header",
				},
			},
			Policies: []model.LLMPolicy{
				{
					Name:    "word-count-guardrail",
					Version: "0.1",
					Paths: []model.LLMPolicyPath{
						{
							Path:    "/*",
							Methods: []string{"GET"},
							Params: map[string]interface{}{
								"request": map[string]interface{}{
									"invert":         false,
									"jsonPath":       "",
									"max":            0,
									"min":            0,
									"showAssessment": false,
								},
								"response": map[string]interface{}{
									"invert":         false,
									"jsonPath":       "",
									"max":            0,
									"min":            0,
									"showAssessment": false,
								},
							},
						},
					},
				},
			},
		},
	}

	out, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(out.Spec.OperationPolicies) != 1 {
		t.Fatalf("expected 1 operation policy, got: %d", len(out.Spec.OperationPolicies))
	}
	if len(out.Spec.GlobalPolicies) != 1 {
		t.Fatalf("expected 1 global policy, got: %d", len(out.Spec.GlobalPolicies))
	}

	apiKeyPolicy := out.Spec.GlobalPolicies[0]
	if apiKeyPolicy.Name != "api-key-auth" {
		t.Fatalf("expected api-key-auth global policy, got: %s", apiKeyPolicy.Name)
	}
	if apiKeyPolicy.Params == nil || (*apiKeyPolicy.Params)["key"] != "X-API-Key" {
		t.Fatalf("expected api-key-auth params.key X-API-Key")
	}

	guardrailPolicy := findOperationPolicy(out.Spec.OperationPolicies, "word-count-guardrail")
	if guardrailPolicy == nil {
		t.Fatalf("expected word-count-guardrail policy to exist")
	}
	if len(guardrailPolicy.Paths) != 1 {
		t.Fatalf("expected 1 path in word-count-guardrail policy, got: %d", len(guardrailPolicy.Paths))
	}
	if guardrailPolicy.Paths[0].Path != "/*" {
		t.Fatalf("expected word-count-guardrail path /*, got: %s", guardrailPolicy.Paths[0].Path)
	}
	if len(guardrailPolicy.Paths[0].Methods) != 1 || guardrailPolicy.Paths[0].Methods[0] != "GET" {
		t.Fatalf("expected word-count-guardrail methods [GET], got: %#v", guardrailPolicy.Paths[0].Methods)
	}

	request, ok := guardrailPolicy.Paths[0].Params["request"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected request params object")
	}
	if request["showAssessment"] != false {
		t.Fatalf("expected request.showAssessment=false, got: %#v", request["showAssessment"])
	}

	response, ok := guardrailPolicy.Paths[0].Params["response"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected response params object")
	}
	if response["showAssessment"] != false {
		t.Fatalf("expected response.showAssessment=false, got: %#v", response["showAssessment"])
	}
}

func TestGenerateLLMProviderDeploymentYAML_NormalizesPolicyVersionToMajor(t *testing.T) {
	provider := &model.LLMProvider{
		ID:      "tt",
		Name:    "tt",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Context:  strPtr("/"),
			Template: "openai",
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://api.openai.com"},
			},
			Policies: []model.LLMPolicy{
				{
					Name:    "policy-a",
					Version: "0.1.0",
					Paths: []model.LLMPolicyPath{{
						Path:    "/*",
						Methods: []string{"GET"},
					}},
				},
				{
					Name:    "policy-b",
					Version: "v10.2.3",
					Paths: []model.LLMPolicyPath{{
						Path:    "/chat",
						Methods: []string{"POST"},
					}},
				},
			},
		},
	}

	out, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	policyA := findOperationPolicy(out.Spec.OperationPolicies, "policy-a")
	if policyA == nil {
		t.Fatalf("expected policy-a to be present")
	}
	if policyA.Version != "v0" {
		t.Fatalf("expected policy-a version to be normalized to v0, got: %s", policyA.Version)
	}

	policyB := findOperationPolicy(out.Spec.OperationPolicies, "policy-b")
	if policyB == nil {
		t.Fatalf("expected policy-b to be present")
	}
	if policyB.Version != "v10" {
		t.Fatalf("expected policy-b version to be normalized to v10, got: %s", policyB.Version)
	}
}

func TestGenerateLLMProviderDeploymentYAML_WithProviderGlobalRateLimit(t *testing.T) {
	provider := &model.LLMProvider{
		ID:      "tt",
		Name:    "tt",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Context:  strPtr("/"),
			Template: "openai",
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://api.openai.com"},
			},
			RateLimiting: &model.LLMRateLimitingConfig{
				ProviderLevel: &model.RateLimitingScopeConfig{
					Global: &model.RateLimitingLimitConfig{
						Request: &model.RequestRateLimit{
							Enabled: true,
							Count:   1,
							Reset: model.RateLimitResetWindow{
								Duration: 1,
								Unit:     "hour",
							},
						},
						Token: &model.TokenRateLimit{
							Enabled: true,
							Count:   1,
							Reset: model.RateLimitResetWindow{
								Duration: 1,
								Unit:     "hour",
							},
						},
					},
				},
			},
		},
	}

	out, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	out = roundTripYAML(t, out)

	if len(out.Spec.GlobalPolicies) != 2 {
		t.Fatalf("expected 2 global policies, got: %d", len(out.Spec.GlobalPolicies))
	}

	tokenPolicy := findGlobalPolicy(out.Spec.GlobalPolicies, "token-based-ratelimit")
	if tokenPolicy == nil {
		t.Fatalf("expected token-based-ratelimit global policy to exist")
	}
	if tokenPolicy.Params == nil {
		t.Fatalf("expected token policy to have params")
	}
	totalTokenLimits, ok := (*tokenPolicy.Params)["totalTokenLimits"].([]interface{})
	if !ok || len(totalTokenLimits) != 1 {
		t.Fatalf("expected totalTokenLimits with one entry, got: %#v", (*tokenPolicy.Params)["totalTokenLimits"])
	}
	firstTokenLimit, ok := totalTokenLimits[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first token limit as object, got: %#v", totalTokenLimits[0])
	}
	if firstTokenLimit["count"] != 1 {
		t.Fatalf("expected token count 1, got: %#v", firstTokenLimit["count"])
	}
	if firstTokenLimit["duration"] != "1h" {
		t.Fatalf("expected token duration 1h, got: %#v", firstTokenLimit["duration"])
	}

	requestPolicy := findGlobalPolicy(out.Spec.GlobalPolicies, "advanced-ratelimit")
	if requestPolicy == nil {
		t.Fatalf("expected advanced-ratelimit global policy to exist")
	}
	if requestPolicy.Params == nil {
		t.Fatalf("expected request policy to have params")
	}
	quotas, ok := (*requestPolicy.Params)["quotas"].([]interface{})
	if !ok || len(quotas) != 1 {
		t.Fatalf("expected quotas with one entry, got: %#v", (*requestPolicy.Params)["quotas"])
	}
	firstQuota, ok := quotas[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first quota as object, got: %#v", quotas[0])
	}
	if firstQuota["name"] != "request-limit" {
		t.Fatalf("expected quota name request-limit, got: %#v", firstQuota["name"])
	}
	limits, ok := firstQuota["limits"].([]interface{})
	if !ok || len(limits) != 1 {
		t.Fatalf("expected quota limits with one entry, got: %#v", firstQuota["limits"])
	}
	firstRequestLimit, ok := limits[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first request limit as object, got: %#v", limits[0])
	}
	if firstRequestLimit["limit"] != 1 {
		t.Fatalf("expected request limit 1, got: %#v", firstRequestLimit["limit"])
	}
	if firstRequestLimit["duration"] != "1h" {
		t.Fatalf("expected request duration 1h, got: %#v", firstRequestLimit["duration"])
	}
}

func TestGenerateLLMProviderDeploymentYAML_WithProviderResourceWiseRateLimit(t *testing.T) {
	provider := &model.LLMProvider{
		ID:      "tt",
		Name:    "tt",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Context:  strPtr("/"),
			Template: "openai",
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://api.openai.com"},
			},
			RateLimiting: &model.LLMRateLimitingConfig{
				ProviderLevel: &model.RateLimitingScopeConfig{
					ResourceWise: &model.ResourceWiseRateLimitingConfig{
						Resources: []model.RateLimitingResourceLimit{
							{
								Resource: "/assistants",
								Limit: model.RateLimitingLimitConfig{
									Request: &model.RequestRateLimit{
										Enabled: true,
										Count:   1,
										Reset:   model.RateLimitResetWindow{Duration: 1, Unit: "hour"},
									},
									Token: &model.TokenRateLimit{
										Enabled: true,
										Count:   1,
										Reset:   model.RateLimitResetWindow{Duration: 1, Unit: "hour"},
									},
								},
							},
							{
								Resource: "/audio/speech",
								Limit: model.RateLimitingLimitConfig{
									Request: &model.RequestRateLimit{
										Enabled: true,
										Count:   1,
										Reset:   model.RateLimitResetWindow{Duration: 1, Unit: "hour"},
									},
									Token: &model.TokenRateLimit{
										Enabled: true,
										Count:   1,
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

	out, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	out = roundTripYAML(t, out)

	if len(out.Spec.OperationPolicies) != 2 {
		t.Fatalf("expected 2 operation policies, got: %d", len(out.Spec.OperationPolicies))
	}

	tokenPolicy := findOperationPolicy(out.Spec.OperationPolicies, "token-based-ratelimit")
	if tokenPolicy == nil {
		t.Fatalf("expected token-based-ratelimit operation policy to exist")
	}
	if len(tokenPolicy.Paths) != 2 {
		t.Fatalf("expected 2 token policy paths, got: %d", len(tokenPolicy.Paths))
	}

	assistantsTokenPath := findOperationPath(tokenPolicy, "/assistants")
	if assistantsTokenPath == nil {
		t.Fatalf("expected token policy path /assistants")
	}
	audioTokenPath := findOperationPath(tokenPolicy, "/audio/speech")
	if audioTokenPath == nil {
		t.Fatalf("expected token policy path /audio/speech")
	}

	requestPolicy := findOperationPolicy(out.Spec.OperationPolicies, "advanced-ratelimit")
	if requestPolicy == nil {
		t.Fatalf("expected advanced-ratelimit operation policy to exist")
	}
	if len(requestPolicy.Paths) != 2 {
		t.Fatalf("expected 2 request policy paths, got: %d", len(requestPolicy.Paths))
	}

	assistantsRequestPath := findOperationPath(requestPolicy, "/assistants")
	if assistantsRequestPath == nil {
		t.Fatalf("expected request policy path /assistants")
	}
	audioRequestPath := findOperationPath(requestPolicy, "/audio/speech")
	if audioRequestPath == nil {
		t.Fatalf("expected request policy path /audio/speech")
	}

	for _, p := range []*api.OperationPolicyPath{assistantsTokenPath, audioTokenPath} {
		totalTokenLimits, ok := p.Params["totalTokenLimits"].([]interface{})
		if !ok || len(totalTokenLimits) != 1 {
			t.Fatalf("expected totalTokenLimits with one entry, got: %#v", p.Params["totalTokenLimits"])
		}
		firstTokenLimit, ok := totalTokenLimits[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected token limit object, got: %#v", totalTokenLimits[0])
		}
		if firstTokenLimit["count"] != 1 {
			t.Fatalf("expected token count 1, got: %#v", firstTokenLimit["count"])
		}
		if firstTokenLimit["duration"] != "1h" {
			t.Fatalf("expected token duration 1h, got: %#v", firstTokenLimit["duration"])
		}
	}

	for _, p := range []*api.OperationPolicyPath{assistantsRequestPath, audioRequestPath} {
		quotas, ok := p.Params["quotas"].([]interface{})
		if !ok || len(quotas) != 1 {
			t.Fatalf("expected quotas with one entry, got: %#v", p.Params["quotas"])
		}
		firstQuota, ok := quotas[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected first quota object, got: %#v", quotas[0])
		}
		limits, ok := firstQuota["limits"].([]interface{})
		if !ok || len(limits) != 1 {
			t.Fatalf("expected limits with one entry, got: %#v", firstQuota["limits"])
		}
		firstRequestLimit, ok := limits[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected request limit object, got: %#v", limits[0])
		}
		if firstRequestLimit["limit"] != 1 {
			t.Fatalf("expected request limit 1, got: %#v", firstRequestLimit["limit"])
		}
		if firstRequestLimit["duration"] != "1h" {
			t.Fatalf("expected request duration 1h, got: %#v", firstRequestLimit["duration"])
		}
	}
}

func TestGenerateLLMProviderDeploymentYAML_WithProviderResourceWiseRateLimitAndDefault(t *testing.T) {
	provider := &model.LLMProvider{
		ID:      "tt",
		Name:    "tt",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Context:  strPtr("/"),
			Template: "openai",
			Upstream: &model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://api.openai.com"},
			},
			RateLimiting: &model.LLMRateLimitingConfig{
				ProviderLevel: &model.RateLimitingScopeConfig{
					ResourceWise: &model.ResourceWiseRateLimitingConfig{
						Default: model.RateLimitingLimitConfig{
							Request: &model.RequestRateLimit{
								Enabled: true,
								Count:   1,
								Reset:   model.RateLimitResetWindow{Duration: 1, Unit: "hour"},
							},
							Token: &model.TokenRateLimit{
								Enabled: true,
								Count:   1,
								Reset:   model.RateLimitResetWindow{Duration: 1, Unit: "hour"},
							},
						},
						Resources: []model.RateLimitingResourceLimit{
							{
								Resource: "/assistants",
								Limit: model.RateLimitingLimitConfig{
									Request: &model.RequestRateLimit{Enabled: true, Count: 1, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
									Token:   &model.TokenRateLimit{Enabled: true, Count: 1, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
								},
							},
							{
								Resource: "/assistants",
								Limit: model.RateLimitingLimitConfig{
									Request: &model.RequestRateLimit{Enabled: true, Count: 1, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
									Token:   &model.TokenRateLimit{Enabled: true, Count: 1, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
								},
							},
							{
								Resource: "/audio/speech",
								Limit: model.RateLimitingLimitConfig{
									Request: &model.RequestRateLimit{Enabled: true, Count: 1, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
									Token:   &model.TokenRateLimit{Enabled: true, Count: 1, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
								},
							},
						},
					},
				},
			},
		},
	}

	out, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	out = roundTripYAML(t, out)

	if len(out.Spec.OperationPolicies) != 2 {
		t.Fatalf("expected 2 operation policies, got: %d", len(out.Spec.OperationPolicies))
	}

	tokenPolicy := findOperationPolicy(out.Spec.OperationPolicies, "token-based-ratelimit")
	if tokenPolicy == nil {
		t.Fatalf("expected token-based-ratelimit operation policy to exist")
	}
	if len(tokenPolicy.Paths) != 3 {
		t.Fatalf("expected 3 token policy paths (default + 2 unique resources), got: %d", len(tokenPolicy.Paths))
	}

	requestPolicy := findOperationPolicy(out.Spec.OperationPolicies, "advanced-ratelimit")
	if requestPolicy == nil {
		t.Fatalf("expected advanced-ratelimit operation policy to exist")
	}
	if len(requestPolicy.Paths) != 3 {
		t.Fatalf("expected 3 request policy paths (default + 2 unique resources), got: %d", len(requestPolicy.Paths))
	}

	for _, p := range []string{"/*", "/assistants", "/audio/speech"} {
		if findOperationPath(tokenPolicy, p) == nil {
			t.Fatalf("expected token policy path %s", p)
		}
		if findOperationPath(requestPolicy, p) == nil {
			t.Fatalf("expected request policy path %s", p)
		}
	}

	for _, p := range tokenPolicy.Paths {
		totalTokenLimits, ok := p.Params["totalTokenLimits"].([]interface{})
		if !ok || len(totalTokenLimits) != 1 {
			t.Fatalf("expected totalTokenLimits with one entry, got: %#v", p.Params["totalTokenLimits"])
		}
		firstTokenLimit, ok := totalTokenLimits[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected token limit object, got: %#v", totalTokenLimits[0])
		}
		if firstTokenLimit["count"] != 1 || firstTokenLimit["duration"] != "1h" {
			t.Fatalf("expected token limit {count:1,duration:1h}, got: %#v", firstTokenLimit)
		}
	}

	for _, p := range requestPolicy.Paths {
		quotas, ok := p.Params["quotas"].([]interface{})
		if !ok || len(quotas) != 1 {
			t.Fatalf("expected quotas with one entry, got: %#v", p.Params["quotas"])
		}
		firstQuota, ok := quotas[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected first quota object, got: %#v", quotas[0])
		}
		limits, ok := firstQuota["limits"].([]interface{})
		if !ok || len(limits) != 1 {
			t.Fatalf("expected limits with one entry, got: %#v", firstQuota["limits"])
		}
		firstRequestLimit, ok := limits[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected request limit object, got: %#v", limits[0])
		}
		if firstRequestLimit["limit"] != 1 || firstRequestLimit["duration"] != "1h" {
			t.Fatalf("expected request limit {limit:1,duration:1h}, got: %#v", firstRequestLimit)
		}
	}
}

func findGlobalPolicy(policies []api.Policy, name string) *api.Policy {
	for i := range policies {
		if policies[i].Name == name {
			return &policies[i]
		}
	}
	return nil
}

func findOperationPolicy(policies []api.OperationPolicy, name string) *api.OperationPolicy {
	for i := range policies {
		if policies[i].Name == name {
			return &policies[i]
		}
	}
	return nil
}

func findOperationPath(policy *api.OperationPolicy, path string) *api.OperationPolicyPath {
	if policy == nil {
		return nil
	}
	for i := range policy.Paths {
		if policy.Paths[i].Path == path {
			return &policy.Paths[i]
		}
	}
	return nil
}

type mockLLMProviderRepo struct {
	repository.LLMProviderRepository
	existsResult bool
	countResult  int
	getByIDFunc  func(providerID, orgUUID string) (*model.LLMProvider, error)
	createCalled bool
	created      *model.LLMProvider
	updated      *model.LLMProvider
}

func (m *mockLLMProviderRepo) Exists(providerID, orgUUID string) (bool, error) {
	return m.existsResult, nil
}

func (m *mockLLMProviderRepo) Count(orgUUID string) (int, error) {
	return m.countResult, nil
}

func (m *mockLLMProviderRepo) Create(p *model.LLMProvider) error {
	m.createCalled = true
	m.created = p
	return nil
}

func (m *mockLLMProviderRepo) GetByID(providerID, orgUUID string) (*model.LLMProvider, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(providerID, orgUUID)
	}
	return nil, nil
}

func (m *mockLLMProviderRepo) Update(p *model.LLMProvider) error {
	m.updated = p
	return nil
}

type mockLLMTemplateRepo struct {
	repository.LLMProviderTemplateRepository
	getByIDFunc   func(templateID, orgUUID string) (*model.LLMProviderTemplate, error)
	getByUUIDFunc func(uuid, orgUUID string) (*model.LLMProviderTemplate, error)
}

func (m *mockLLMTemplateRepo) GetByID(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(templateID, orgUUID)
	}
	return nil, nil
}

func (m *mockLLMTemplateRepo) GetByUUID(uuid, orgUUID string) (*model.LLMProviderTemplate, error) {
	if m.getByUUIDFunc != nil {
		return m.getByUUIDFunc(uuid, orgUUID)
	}
	return nil, nil
}

type mockOrganizationRepo struct {
	repository.OrganizationRepository
	org *model.Organization
}

func (m *mockOrganizationRepo) GetOrganizationByUUID(orgID string) (*model.Organization, error) {
	return m.org, nil
}

type mockLLMProxyRepo struct {
	repository.LLMProxyRepository
	existsResult         bool
	countResult          int
	countByProviderValue int
	listByProviderItems  []*model.LLMProxy
	lastListProviderUUID string
	getByIDFunc          func(proxyID, orgUUID string) (*model.LLMProxy, error)
	created              *model.LLMProxy
	updated              *model.LLMProxy
}

func (m *mockLLMProxyRepo) Exists(proxyID, orgUUID string) (bool, error) {
	return m.existsResult, nil
}

func (m *mockLLMProxyRepo) Count(orgUUID string) (int, error) {
	return m.countResult, nil
}

func (m *mockLLMProxyRepo) Create(p *model.LLMProxy) error {
	m.created = p
	return nil
}

func (m *mockLLMProxyRepo) GetByID(proxyID, orgUUID string) (*model.LLMProxy, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(proxyID, orgUUID)
	}
	return nil, nil
}

func (m *mockLLMProxyRepo) Update(p *model.LLMProxy) error {
	m.updated = p
	return nil
}

func (m *mockLLMProxyRepo) ListByProvider(orgUUID, providerUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	m.lastListProviderUUID = providerUUID
	return m.listByProviderItems, nil
}

func (m *mockLLMProxyRepo) CountByProvider(orgUUID, providerUUID string) (int, error) {
	return m.countByProviderValue, nil
}

type mockProjectRepo struct {
	repository.ProjectRepository
	project *model.Project
}

func (m *mockProjectRepo) GetProjectByUUID(projectID string) (*model.Project, error) {
	return m.project, nil
}

func (m *mockProjectRepo) GetProjectByHandleAndOrgID(handle, orgID string) (*model.Project, error) {
	return m.project, nil
}

type noopAuditRepo struct{}

func (n *noopAuditRepo) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	return nil
}

func TestLLMProviderServiceCreateRejectsMultipleModelProvidersForNativeTemplate(t *testing.T) {
	now := time.Now()
	providerRepo := &mockLLMProviderRepo{}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai", CreatedAt: now, UpdatedAt: now}, nil
		},
	}
	service := NewLLMProviderService(providerRepo, templateRepo, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProviderRequest("openai")
	request.ModelProviders = &[]api.LLMModelProvider{
		{Id: strPointer("openai"), DisplayName: "OpenAI", Models: &[]api.LLMModel{{Id: strPointer("gpt-4o"), DisplayName: "GPT-4o"}}},
		{Id: strPointer("anthropic"), DisplayName: "Anthropic", Models: &[]api.LLMModel{{Id: strPointer("claude-3-5-sonnet"), DisplayName: "Claude 3.5 Sonnet"}}},
	}

	_, err := service.Create("org-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
	if providerRepo.createCalled {
		t.Fatalf("did not expect repository create to be called")
	}
}

func TestLLMProviderServiceCreateAllowsAggregatorTemplate(t *testing.T) {
	now := time.Now()
	providerRepo := &mockLLMProviderRepo{}
	providerRepo.getByIDFunc = func(providerID, orgUUID string) (*model.LLMProvider, error) {
		if providerRepo.created == nil {
			return nil, nil
		}
		created := *providerRepo.created
		created.UUID = "prov-uuid"
		created.CreatedAt = now
		created.UpdatedAt = now
		return &created, nil
	}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-agg", ID: "awsbedrock", Enabled: true, CreatedAt: now, UpdatedAt: now}, nil
		},
	}
	orgRepo := &mockOrganizationRepo{org: &model.Organization{ID: "org-1"}}
	service := NewLLMProviderService(providerRepo, templateRepo, orgRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProviderRequest("awsbedrock")
	request.ModelProviders = &[]api.LLMModelProvider{
		{Id: strPointer("claude"), DisplayName: "Claude", Models: &[]api.LLMModel{{Id: strPointer("claude-3-5-sonnet"), DisplayName: "Claude 3.5 Sonnet"}}},
		{Id: strPointer("deepseek"), DisplayName: "DeepSeek", Models: &[]api.LLMModel{{Id: strPointer("deepseek-r1"), DisplayName: "DeepSeek R1"}}},
	}

	response, err := service.Create("org-1", "alice", request)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if response == nil || response.ModelProviders == nil || len(*response.ModelProviders) != 2 {
		t.Fatalf("expected two model providers in response, got: %#v", response)
	}
	if providerRepo.created == nil || providerRepo.created.TemplateUUID != "tpl-agg" {
		t.Fatalf("expected created provider to reference aggregator template UUID")
	}
}

// TestLLMProviderServiceCreateMigratesLegacyPolicies verifies Phase 10 save-time
// migration: a provider created with the deprecated `policies` list has it split into
// globalPolicies (for path "/*" + methods ["*"]) and operationPolicies (all other paths).
// After save, `policies` is cleared in the stored config.
func TestLLMProviderServiceCreateMigratesLegacyPolicies(t *testing.T) {
	now := time.Now()
	providerRepo := &mockLLMProviderRepo{}
	providerRepo.getByIDFunc = func(providerID, orgUUID string) (*model.LLMProvider, error) {
		if providerRepo.created == nil {
			return nil, nil
		}
		created := *providerRepo.created
		created.UUID = "prov-uuid"
		created.CreatedAt = now
		created.UpdatedAt = now
		return &created, nil
	}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai", Enabled: true, CreatedAt: now, UpdatedAt: now}, nil
		},
	}
	orgRepo := &mockOrganizationRepo{org: &model.Organization{ID: "org-1"}}
	service := NewLLMProviderService(providerRepo, templateRepo, orgRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProviderRequest("openai")
	request.Policies = &[]api.LLMPolicy{
		// path "/*" + methods ["*"] → globalPolicies
		{
			Name:    "basic-ratelimit",
			Version: "v1",
			Paths: []api.LLMPolicyPath{
				{Path: "/*", Methods: []api.LLMPolicyPathMethods{"*"}, Params: map[string]interface{}{"requests": 10}},
			},
		},
		// specific path → operationPolicies
		{
			Name:    "token-ratelimit",
			Version: "v1",
			Paths: []api.LLMPolicyPath{
				{Path: "/chat/completions", Methods: []api.LLMPolicyPathMethods{"POST"}, Params: map[string]interface{}{"tokens": 1000}},
			},
		},
	}

	if _, err := service.Create("org-1", "alice", request); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if providerRepo.created == nil {
		t.Fatal("expected provider to be created")
	}
	cfg := providerRepo.created.Configuration

	// Deprecated list cleared after migration.
	if len(cfg.Policies) != 0 {
		t.Fatalf("expected policies cleared after migration, got: %d", len(cfg.Policies))
	}
	// "/*" + ["*"] → globalPolicies
	if len(cfg.GlobalPolicies) != 1 {
		t.Fatalf("expected 1 globalPolicy, got: %d", len(cfg.GlobalPolicies))
	}
	if cfg.GlobalPolicies[0].Name != "basic-ratelimit" {
		t.Fatalf("expected globalPolicy name basic-ratelimit, got: %s", cfg.GlobalPolicies[0].Name)
	}
	// specific path → operationPolicies
	if len(cfg.OperationPolicies) != 1 {
		t.Fatalf("expected 1 operationPolicy, got: %d", len(cfg.OperationPolicies))
	}
	if cfg.OperationPolicies[0].Name != "token-ratelimit" {
		t.Fatalf("expected operationPolicy name token-ratelimit, got: %s", cfg.OperationPolicies[0].Name)
	}
	if len(cfg.OperationPolicies[0].Paths) != 1 || cfg.OperationPolicies[0].Paths[0].Path != "/chat/completions" {
		t.Fatalf("expected operationPolicy path /chat/completions, got: %+v", cfg.OperationPolicies[0].Paths)
	}
}

func TestLLMProviderServiceCreateRejectsInvalidGlobalPolicyVersion(t *testing.T) {
	service := NewLLMProviderService(&mockLLMProviderRepo{}, &mockLLMTemplateRepo{}, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProviderRequest("openai")
	request.GlobalPolicies = &[]api.Policy{{Name: "api-key-auth", Version: "v1.0.0"}}

	_, err := service.Create("org-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}

func TestLLMProviderServiceCreateRejectsInvalidOperationPolicyVersion(t *testing.T) {
	service := NewLLMProviderService(&mockLLMProviderRepo{}, &mockLLMTemplateRepo{}, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProviderRequest("openai")
	request.OperationPolicies = &[]api.OperationPolicy{{Name: "token-ratelimit", Version: "1"}}

	_, err := service.Create("org-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}

func TestLLMProviderServiceCreateRejectsInvalidLegacyPolicyVersion(t *testing.T) {
	service := NewLLMProviderService(&mockLLMProviderRepo{}, &mockLLMTemplateRepo{}, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProviderRequest("openai")
	request.Policies = &[]api.LLMPolicy{{Name: "basic-ratelimit", Version: "V1"}}

	_, err := service.Create("org-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}

func TestLLMProviderServiceUpdateRejectsInvalidPolicyVersion(t *testing.T) {
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "prov-uuid", ID: providerID, TemplateUUID: "tpl-openai"}, nil
		},
	}
	service := NewLLMProviderService(providerRepo, &mockLLMTemplateRepo{}, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProviderRequest("openai")
	request.GlobalPolicies = &[]api.Policy{{Name: "api-key-auth", Version: "v1.0.0"}}

	_, err := service.Update("org-1", "provider-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}

func TestLLMProviderServiceCreateReturnsConflictForDuplicateHandle(t *testing.T) {
	providerRepo := &mockLLMProviderRepo{existsResult: true}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai"}, nil
		},
	}
	service := NewLLMProviderService(providerRepo, templateRepo, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	_, err := service.Create("org-1", "alice", validProviderRequest("openai"))
	if !apperror.LLMProviderExists.Is(err) {
		t.Fatalf("expected ErrLLMProviderExists, got: %v", err)
	}
}

func TestLLMProviderServiceUpdatePreservesUpstreamAuthValue(t *testing.T) {
	now := time.Now()
	providerRepo := &mockLLMProviderRepo{}
	providerRepo.getByIDFunc = func(providerID, orgUUID string) (*model.LLMProvider, error) {
		if providerRepo.updated == nil {
			return &model.LLMProvider{
				UUID:         "prov-uuid",
				ID:           providerID,
				Name:         "Old Provider",
				Version:      "v1.0",
				TemplateUUID: "tpl-openai",
				CreatedAt:    now,
				UpdatedAt:    now,
				Configuration: model.LLMProviderConfig{
					Upstream: &model.UpstreamConfig{
						Main: &model.UpstreamEndpoint{
							URL:  "https://example.com/openai/v1",
							Auth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: "Bearer old-secret"},
						},
					},
				},
			}, nil
		}
		updated := *providerRepo.updated
		updated.UUID = "prov-uuid"
		updated.CreatedAt = now
		updated.UpdatedAt = now
		return &updated, nil
	}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai"}, nil
		},
	}
	service := NewLLMProviderService(providerRepo, templateRepo, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProviderRequest("openai")
	request.DisplayName = "Updated Provider"
	request.Upstream.Main.Auth = &api.UpstreamAuth{
		Type:   upstreamAuthTypePtr("api-key"),
		Header: stringPtr("Authorization"),
		Value:  stringPtr(""),
	}

	_, err := service.Update("org-1", "provider-1", "test-user", request)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if providerRepo.updated == nil || providerRepo.updated.Configuration.Upstream == nil || providerRepo.updated.Configuration.Upstream.Main == nil || providerRepo.updated.Configuration.Upstream.Main.Auth == nil {
		t.Fatalf("expected updated provider upstream auth to be set")
	}
	if providerRepo.updated.Configuration.Upstream.Main.Auth.Value != "Bearer old-secret" {
		t.Fatalf("expected upstream auth value to be preserved, got %q", providerRepo.updated.Configuration.Upstream.Main.Auth.Value)
	}
}

func TestLLMProxyServiceCreateFailsWhenProviderNotFound(t *testing.T) {
	proxyRepo := &mockLLMProxyRepo{}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return nil, nil
		},
	}
	projectRepo := &mockProjectRepo{project: &model.Project{ID: "project-1", OrganizationID: "org-1"}}
	service := NewLLMProxyService(proxyRepo, providerRepo, projectRepo, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	_, err := service.Create("org-1", "alice", validProxyRequest("provider-1", "project-1"))
	if !apperror.LLMProviderNotFound.Is(err) {
		t.Fatalf("expected ErrLLMProviderNotFound, got: %v", err)
	}
}

func TestLLMProxyServiceCreateRejectsInvalidPolicyVersion(t *testing.T) {
	proxyRepo := &mockLLMProxyRepo{}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProxyRequest("provider-1", "project-1")
	request.GlobalPolicies = &[]api.Policy{{Name: "api-key-auth", Version: "v1.0.0"}}

	_, err := service.Create("org-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}

func TestLLMProxyServiceUpdateRejectsInvalidPolicyVersion(t *testing.T) {
	proxyRepo := &mockLLMProxyRepo{}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProxyRequest("provider-1", "project-1")
	request.OperationPolicies = &[]api.OperationPolicy{{Name: "token-ratelimit", Version: "1"}}

	_, err := service.Update("org-1", "proxy-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ErrInvalidPolicyVersion, got: %v", err)
	}
}

func TestLLMProxyServiceCreateReturnsConflictForDuplicateHandle(t *testing.T) {
	proxyRepo := &mockLLMProxyRepo{existsResult: true}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	_, err := service.Create("org-1", "alice", validProxyRequest("provider-1", "project-1"))
	if !apperror.LLMProxyExists.Is(err) {
		t.Fatalf("expected ErrLLMProxyExists, got: %v", err)
	}
}

func TestLLMProxyServiceListByProviderUsesProviderUUID(t *testing.T) {
	now := time.Now()
	proxyRepo := &mockLLMProxyRepo{
		listByProviderItems: []*model.LLMProxy{{
			UUID:        "proxy-uuid",
			ID:          "proxy-1",
			Name:        "Proxy One",
			Version:     "v1.0",
			ProjectUUID: "project-1",
			CreatedAt:   now,
			UpdatedAt:   now,
			Configuration: model.LLMProxyConfig{
				Provider: "provider-1",
				Context:  stringPtr("/assistant"),
			},
		}},
		countByProviderValue: 1,
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	resp, err := service.ListByProvider("org-1", "provider-1", 10, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if proxyRepo.lastListProviderUUID != "provider-uuid" {
		t.Fatalf("expected list by provider to use provider UUID, got: %q", proxyRepo.lastListProviderUUID)
	}
	if resp == nil || resp.Count != 1 || len(resp.List) != 1 {
		t.Fatalf("expected one proxy in response, got: %#v", resp)
	}
}

func TestLLMProxyServiceUpdatePreservesProviderAuthValue(t *testing.T) {
	now := time.Now()
	proxyRepo := &mockLLMProxyRepo{}
	proxyRepo.getByIDFunc = func(proxyID, orgUUID string) (*model.LLMProxy, error) {
		if proxyRepo.updated == nil {
			return &model.LLMProxy{
				UUID:         "proxy-uuid",
				ID:           proxyID,
				Name:         "Old Proxy",
				Version:      "v1.0",
				ProjectUUID:  "project-1",
				ProviderUUID: "provider-uuid",
				CreatedAt:    now,
				UpdatedAt:    now,
				Configuration: model.LLMProxyConfig{
					Provider:     "provider-1",
					UpstreamAuth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: "Bearer old-secret"},
				},
			}, nil
		}
		updated := *proxyRepo.updated
		updated.UUID = "proxy-uuid"
		updated.CreatedAt = now
		updated.UpdatedAt = now
		return &updated, nil
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	request := validProxyRequest("provider-1", "project-1")
	request.DisplayName = "Updated Proxy"
	request.Provider.Auth = &api.UpstreamAuth{
		Type:   upstreamAuthTypePtr("api-key"),
		Header: stringPtr("Authorization"),
		Value:  stringPtr(""),
	}

	_, err := service.Update("org-1", "proxy-1", "test-user", request)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if proxyRepo.updated == nil || proxyRepo.updated.Configuration.UpstreamAuth == nil {
		t.Fatalf("expected updated proxy auth to be set")
	}
	if proxyRepo.updated.Configuration.UpstreamAuth.Value != "Bearer old-secret" {
		t.Fatalf("expected proxy auth value to be preserved, got %q", proxyRepo.updated.Configuration.UpstreamAuth.Value)
	}
}

// TestLLMProviderServiceCreate_PolicySecretRef_Rejected proves secret-ref
// validation now covers the whole request, not just upstream.auth — a
// placeholder embedded in a policy param (not upstream) must also be rejected.
func TestLLMProviderServiceCreate_PolicySecretRef_Rejected(t *testing.T) {
	now := time.Now()
	providerRepo := &mockLLMProviderRepo{}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-1", ID: templateID, Enabled: true, CreatedAt: now, UpdatedAt: now}, nil
		},
	}
	orgRepo := &mockOrganizationRepo{org: &model.Organization{ID: "org-1"}}
	secretService := NewSecretService(newMockRepo(), &mockVault{}, newTestIdentityService())
	service := NewLLMProviderService(providerRepo, templateRepo, orgRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	service.SetSecretService(secretService)

	request := validProviderRequest("openai")
	params := map[string]interface{}{"value": `{{ secret "nonexistent-policy-secret" }}`}
	request.GlobalPolicies = &[]api.Policy{{Name: "set-headers", Version: "v1", Params: &params}}

	_, err := service.Create("org-1", "alice", request)
	if err == nil {
		t.Fatal("expected error for non-existent secret placeholder in a policy param, got nil")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected a validation error for missing secret ref, got: %v", err)
	}
	if providerRepo.created != nil {
		t.Error("expected provider creation to be aborted, but repo.Create was called")
	}
}

func TestLLMProxyServiceCreate_MissingSecretRef_Rejected(t *testing.T) {
	proxyRepo := &mockLLMProxyRepo{}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	secretService := NewSecretService(newMockRepo(), &mockVault{}, newTestIdentityService())
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	service.SetSecretService(secretService)

	request := validProxyRequest("provider-1", "project-1")
	request.Provider.Auth = &api.UpstreamAuth{
		Type:   upstreamAuthTypePtr("api-key"),
		Header: stringPtr("Authorization"),
		Value:  stringPtr(`{{ secret "nonexistent-proxy-secret" }}`),
	}

	_, err := service.Create("org-1", "alice", request)
	if err == nil {
		t.Fatal("expected error for non-existent secret placeholder, got nil")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected a validation error for missing secret ref, got: %v", err)
	}
	if proxyRepo.created != nil {
		t.Error("expected proxy creation to be aborted, but repo.Create was called")
	}
}

func TestLLMProxyServiceUpdate_MissingSecretRef_Rejected(t *testing.T) {
	now := time.Now()
	proxyRepo := &mockLLMProxyRepo{
		getByIDFunc: func(proxyID, orgUUID string) (*model.LLMProxy, error) {
			return &model.LLMProxy{
				UUID: "proxy-uuid", ID: proxyID, Name: "Old Proxy", Version: "v1.0",
				ProjectUUID: "project-1", ProviderUUID: "provider-uuid",
				CreatedAt: now, UpdatedAt: now,
				Configuration: model.LLMProxyConfig{
					Provider:     "provider-1",
					UpstreamAuth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: `{{ secret "existing-handle" }}`},
				},
			}, nil
		},
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	secretService := NewSecretService(newMockRepo(), &mockVault{}, newTestIdentityService())
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	service.SetSecretService(secretService)

	request := validProxyRequest("provider-1", "project-1")
	request.Provider.Auth = &api.UpstreamAuth{
		Type:   upstreamAuthTypePtr("api-key"),
		Header: stringPtr("Authorization"),
		Value:  stringPtr(`{{ secret "nonexistent-proxy-secret" }}`),
	}

	_, err := service.Update("org-1", "proxy-1", "alice", request)
	if err == nil {
		t.Fatal("expected error for non-existent secret placeholder, got nil")
	}
	if !apperror.ValidationFailed.Is(err) {
		t.Errorf("expected a validation error for missing secret ref, got: %v", err)
	}
	if proxyRepo.updated != nil {
		t.Error("expected proxy update to be aborted, but repo.Update was called")
	}
}

// TestLLMProviderServiceUpdate_CleansUpRotatedSecret proves rotating a
// provider's upstream credential deprecates the secret it replaced — the
// same cleanupRotatedSecret path LLM Proxy and REST API are also tested
// against, exercised here for the Provider service specifically.
func TestLLMProviderServiceUpdate_CleansUpRotatedSecret(t *testing.T) {
	now := time.Now()
	providerRepo := &mockLLMProviderRepo{}
	providerRepo.getByIDFunc = func(providerID, orgUUID string) (*model.LLMProvider, error) {
		if providerRepo.updated == nil {
			return &model.LLMProvider{
				UUID:         "prov-uuid",
				ID:           providerID,
				Name:         "Old Provider",
				Version:      "v1.0",
				TemplateUUID: "tpl-openai",
				CreatedAt:    now,
				UpdatedAt:    now,
				Configuration: model.LLMProviderConfig{
					Upstream: &model.UpstreamConfig{
						Main: &model.UpstreamEndpoint{
							URL:  "https://example.com/openai/v1",
							Auth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: `{{ secret "old-handle" }}`},
						},
					},
				},
			}, nil
		}
		updated := *providerRepo.updated
		updated.UUID = "prov-uuid"
		updated.CreatedAt = now
		updated.UpdatedAt = now
		return &updated, nil
	}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai"}, nil
		},
	}
	secretRepo := newMockRepo()
	secretRepo.secrets["old-handle"] = &model.Secret{Handle: "old-handle", Status: model.SecretStatusActive}
	secretRepo.secrets["new-handle"] = &model.Secret{Handle: "new-handle", Status: model.SecretStatusActive}
	secretService := NewSecretService(secretRepo, &mockVault{}, newTestIdentityService())

	service := NewLLMProviderService(providerRepo, templateRepo, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	service.SetSecretService(secretService)

	request := validProviderRequest("openai")
	request.Upstream.Main.Auth = &api.UpstreamAuth{
		Type:   upstreamAuthTypePtr("api-key"),
		Header: stringPtr("Authorization"),
		Value:  stringPtr(`{{ secret "new-handle" }}`),
	}

	if _, err := service.Update("org-1", "provider-1", "alice", request); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if secretRepo.secrets["old-handle"].Status != model.SecretStatusDeprecated {
		t.Fatalf("expected old secret to be deprecated, got status=%v", secretRepo.secrets["old-handle"].Status)
	}
	if secretRepo.secrets["new-handle"].Status != model.SecretStatusActive {
		t.Fatalf("expected new secret to remain active, got status=%v", secretRepo.secrets["new-handle"].Status)
	}
}

func TestLLMProxyServiceUpdate_CleansUpRotatedSecret(t *testing.T) {
	now := time.Now()
	proxyRepo := &mockLLMProxyRepo{}
	proxyRepo.getByIDFunc = func(proxyID, orgUUID string) (*model.LLMProxy, error) {
		if proxyRepo.updated == nil {
			return &model.LLMProxy{
				UUID:         "proxy-uuid",
				ID:           proxyID,
				Name:         "Old Proxy",
				Version:      "v1.0",
				ProjectUUID:  "project-1",
				ProviderUUID: "provider-uuid",
				CreatedAt:    now,
				UpdatedAt:    now,
				Configuration: model.LLMProxyConfig{
					Provider:     "provider-1",
					UpstreamAuth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: `{{ secret "old-handle" }}`},
				},
			}, nil
		}
		updated := *proxyRepo.updated
		updated.UUID = "proxy-uuid"
		updated.CreatedAt = now
		updated.UpdatedAt = now
		return &updated, nil
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	secretRepo := newMockRepo()
	secretRepo.secrets["old-handle"] = &model.Secret{Handle: "old-handle", Status: model.SecretStatusActive}
	secretRepo.secrets["new-handle"] = &model.Secret{Handle: "new-handle", Status: model.SecretStatusActive}
	secretService := NewSecretService(secretRepo, &mockVault{}, newTestIdentityService())

	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	service.SetSecretService(secretService)

	request := validProxyRequest("provider-1", "project-1")
	request.Provider.Auth = &api.UpstreamAuth{
		Type:   upstreamAuthTypePtr("api-key"),
		Header: stringPtr("Authorization"),
		Value:  stringPtr(`{{ secret "new-handle" }}`),
	}

	_, err := service.Update("org-1", "proxy-1", "test-user", request)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if secretRepo.secrets["old-handle"].Status != model.SecretStatusDeprecated {
		t.Fatalf("expected old secret to be deprecated, got status=%v", secretRepo.secrets["old-handle"].Status)
	}
}

func validProviderRequest(template string) *api.LLMProvider {
	return &api.LLMProvider{
		Id:          strPointer("provider-1"),
		DisplayName: "Test Provider",
		Version:     "v1.0",
		Template:    template,
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{Url: stringPtr("https://example.com/openai/v1")},
		},
		AccessControl: api.LLMAccessControl{Mode: "allow_all"},
	}
}

func validProxyRequest(providerID, projectID string) *api.LLMProxy {
	return &api.LLMProxy{
		Id:          strPointer("proxy-1"),
		DisplayName: "Test Proxy",
		Version:     "v1.0",
		ProjectId:   projectID,
		Provider: api.LLMProxyProvider{
			Id: providerID,
		},
	}
}

func stringPtr(s string) *string {
	return &s
}

func upstreamAuthTypePtr(v string) *api.UpstreamAuthType {
	t := api.UpstreamAuthType(v)
	return &t
}
