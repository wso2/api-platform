package service

import (
	"errors"
	"testing"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"

	"gopkg.in/yaml.v3"
)

func TestMapTemplateResourceMappingAPI_RejectsEmptyResource(t *testing.T) {
	mapped, err := mapTemplateResourceMappingAPI(&api.LLMProviderTemplateResourceMapping{Resource: "   "})
	if err == nil {
		t.Fatal("expected error for empty resource")
	}
	if mapped != nil {
		t.Fatal("expected mapped resource to be nil when validation fails")
	}
	if !errors.Is(err, constants.ErrInvalidInput) {
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
	if !errors.Is(err, constants.ErrInvalidInput) {
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
		err := validateLLMResourceLimit(4, constants.MaxLLMProvidersPerOrganization, constants.ErrLLMProviderLimitReached)
		if err != nil {
			t.Fatalf("expected no error below limit, got: %v", err)
		}
	})

	t.Run("at limit should fail", func(t *testing.T) {
		err := validateLLMResourceLimit(5, constants.MaxLLMProvidersPerOrganization, constants.ErrLLMProviderLimitReached)
		if err != constants.ErrLLMProviderLimitReached {
			t.Fatalf("expected ErrLLMProviderLimitReached, got: %v", err)
		}
	})

	t.Run("above limit should fail", func(t *testing.T) {
		err := validateLLMResourceLimit(6, constants.MaxLLMProxiesPerOrganization, constants.ErrLLMProxyLimitReached)
		if err != constants.ErrLLMProxyLimitReached {
			t.Fatalf("expected ErrLLMProxyLimitReached, got: %v", err)
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

	yamlStr, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var out dto.LLMProviderDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlStr), &out); err != nil {
		t.Fatalf("failed to unmarshal generated yaml: %v", err)
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

	if len(out.Spec.Policies) != 1 {
		t.Fatalf("expected 1 policy, got: %d", len(out.Spec.Policies))
	}

	policy := out.Spec.Policies[0]
	if policy.Name != "api-key-auth" {
		t.Fatalf("expected policy name api-key-auth, got: %s", policy.Name)
	}
	if policy.Version != "v0" {
		t.Fatalf("expected policy version v0, got: %s", policy.Version)
	}
	if len(policy.Paths) != 1 {
		t.Fatalf("expected 1 policy path, got: %d", len(policy.Paths))
	}

	path := policy.Paths[0]
	if path.Path != "/*" {
		t.Fatalf("expected policy path /*, got: %s", path.Path)
	}
	if len(path.Methods) != 1 || path.Methods[0] != "*" {
		t.Fatalf("expected methods [*], got: %#v", path.Methods)
	}
	if path.Params["key"] != "X-API-Key" {
		t.Fatalf("expected params.key X-API-Key, got: %#v", path.Params["key"])
	}
	if path.Params["in"] != "header" {
		t.Fatalf("expected params.in header, got: %#v", path.Params["in"])
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

	yamlStr, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var out dto.LLMProviderDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlStr), &out); err != nil {
		t.Fatalf("failed to unmarshal generated yaml: %v", err)
	}

	if len(out.Spec.Policies) != 2 {
		t.Fatalf("expected 2 policies, got: %d", len(out.Spec.Policies))
	}

	apiKeyPolicy := findPolicy(out.Spec.Policies, "api-key-auth", "v0")
	if apiKeyPolicy == nil {
		t.Fatalf("expected api-key-auth policy to exist")
	}
	if len(apiKeyPolicy.Paths) != 1 || apiKeyPolicy.Paths[0].Path != "/*" {
		t.Fatalf("expected api-key-auth path /*")
	}

	guardrailPolicy := findPolicy(out.Spec.Policies, "word-count-guardrail", "v0")
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

	yamlStr, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var out dto.LLMProviderDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlStr), &out); err != nil {
		t.Fatalf("failed to unmarshal generated yaml: %v", err)
	}

	if findPolicy(out.Spec.Policies, "policy-a", "v0") == nil {
		t.Fatalf("expected policy-a version to be normalized to v0")
	}
	if findPolicy(out.Spec.Policies, "policy-b", "v10") == nil {
		t.Fatalf("expected policy-b version to be normalized to v10")
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

	yamlStr, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var out dto.LLMProviderDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlStr), &out); err != nil {
		t.Fatalf("failed to unmarshal generated yaml: %v", err)
	}

	if len(out.Spec.Policies) != 2 {
		t.Fatalf("expected 2 policies, got: %d", len(out.Spec.Policies))
	}

	tokenPolicy := findPolicy(out.Spec.Policies, "token-based-ratelimit", "v0")
	if tokenPolicy == nil {
		t.Fatalf("expected token-based-ratelimit policy to exist")
	}
	if len(tokenPolicy.Paths) != 1 {
		t.Fatalf("expected token policy to have 1 path, got: %d", len(tokenPolicy.Paths))
	}
	tokenPath := tokenPolicy.Paths[0]
	if tokenPath.Path != "/*" {
		t.Fatalf("expected token policy path /*, got: %s", tokenPath.Path)
	}
	totalTokenLimits, ok := tokenPath.Params["totalTokenLimits"].([]interface{})
	if !ok || len(totalTokenLimits) != 1 {
		t.Fatalf("expected totalTokenLimits with one entry, got: %#v", tokenPath.Params["totalTokenLimits"])
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

	requestPolicy := findPolicy(out.Spec.Policies, "advanced-ratelimit", "v0")
	if requestPolicy == nil {
		t.Fatalf("expected advanced-ratelimit policy to exist")
	}
	if len(requestPolicy.Paths) != 1 {
		t.Fatalf("expected request policy to have 1 path, got: %d", len(requestPolicy.Paths))
	}
	requestPath := requestPolicy.Paths[0]
	if requestPath.Path != "/*" {
		t.Fatalf("expected request policy path /*, got: %s", requestPath.Path)
	}
	quotas, ok := requestPath.Params["quotas"].([]interface{})
	if !ok || len(quotas) != 1 {
		t.Fatalf("expected quotas with one entry, got: %#v", requestPath.Params["quotas"])
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

	yamlStr, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var out dto.LLMProviderDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlStr), &out); err != nil {
		t.Fatalf("failed to unmarshal generated yaml: %v", err)
	}

	if len(out.Spec.Policies) != 2 {
		t.Fatalf("expected 2 policies, got: %d", len(out.Spec.Policies))
	}

	tokenPolicy := findPolicy(out.Spec.Policies, "token-based-ratelimit", "v0")
	if tokenPolicy == nil {
		t.Fatalf("expected token-based-ratelimit policy to exist")
	}
	if len(tokenPolicy.Paths) != 2 {
		t.Fatalf("expected 2 token policy paths, got: %d", len(tokenPolicy.Paths))
	}

	assistantsTokenPath := findPath(tokenPolicy, "/assistants")
	if assistantsTokenPath == nil {
		t.Fatalf("expected token policy path /assistants")
	}
	audioTokenPath := findPath(tokenPolicy, "/audio/speech")
	if audioTokenPath == nil {
		t.Fatalf("expected token policy path /audio/speech")
	}

	requestPolicy := findPolicy(out.Spec.Policies, "advanced-ratelimit", "v0")
	if requestPolicy == nil {
		t.Fatalf("expected advanced-ratelimit policy to exist")
	}
	if len(requestPolicy.Paths) != 2 {
		t.Fatalf("expected 2 request policy paths, got: %d", len(requestPolicy.Paths))
	}

	assistantsRequestPath := findPath(requestPolicy, "/assistants")
	if assistantsRequestPath == nil {
		t.Fatalf("expected request policy path /assistants")
	}
	audioRequestPath := findPath(requestPolicy, "/audio/speech")
	if audioRequestPath == nil {
		t.Fatalf("expected request policy path /audio/speech")
	}

	for _, p := range []*api.LLMPolicyPath{assistantsTokenPath, audioTokenPath} {
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

	for _, p := range []*api.LLMPolicyPath{assistantsRequestPath, audioRequestPath} {
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

	yamlStr, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var out dto.LLMProviderDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlStr), &out); err != nil {
		t.Fatalf("failed to unmarshal generated yaml: %v", err)
	}

	if len(out.Spec.Policies) != 2 {
		t.Fatalf("expected 2 policies, got: %d", len(out.Spec.Policies))
	}

	tokenPolicy := findPolicy(out.Spec.Policies, "token-based-ratelimit", "v0")
	if tokenPolicy == nil {
		t.Fatalf("expected token-based-ratelimit policy to exist")
	}
	if len(tokenPolicy.Paths) != 3 {
		t.Fatalf("expected 3 token policy paths (default + 2 unique resources), got: %d", len(tokenPolicy.Paths))
	}

	requestPolicy := findPolicy(out.Spec.Policies, "advanced-ratelimit", "v0")
	if requestPolicy == nil {
		t.Fatalf("expected advanced-ratelimit policy to exist")
	}
	if len(requestPolicy.Paths) != 3 {
		t.Fatalf("expected 3 request policy paths (default + 2 unique resources), got: %d", len(requestPolicy.Paths))
	}

	for _, p := range []string{"/*", "/assistants", "/audio/speech"} {
		if findPath(tokenPolicy, p) == nil {
			t.Fatalf("expected token policy path %s", p)
		}
		if findPath(requestPolicy, p) == nil {
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

func findPolicy(policies []api.LLMPolicy, name, version string) *api.LLMPolicy {
	for i := range policies {
		if policies[i].Name == name && policies[i].Version == version {
			return &policies[i]
		}
	}
	return nil
}

func findPath(policy *api.LLMPolicy, path string) *api.LLMPolicyPath {
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

func TestLLMProviderServiceCreateRejectsMultipleModelProvidersForNativeTemplate(t *testing.T) {
	now := time.Now()
	providerRepo := &mockLLMProviderRepo{}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai", CreatedAt: now, UpdatedAt: now}, nil
		},
	}
	service := NewLLMProviderService(providerRepo, templateRepo, nil, nil)

	request := validProviderRequest("openai")
	request.ModelProviders = &[]api.LLMModelProvider{
		{Id: "openai", Models: &[]api.LLMModel{{Id: "gpt-4o"}}},
		{Id: "anthropic", Models: &[]api.LLMModel{{Id: "claude-3-5-sonnet"}}},
	}

	_, err := service.Create("org-1", "alice", request)
	if err != constants.ErrInvalidInput {
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
			return &model.LLMProviderTemplate{UUID: "tpl-agg", ID: "awsbedrock", CreatedAt: now, UpdatedAt: now}, nil
		},
	}
	orgRepo := &mockOrganizationRepo{org: &model.Organization{ID: "org-1"}}
	service := NewLLMProviderService(providerRepo, templateRepo, orgRepo, nil)

	request := validProviderRequest("awsbedrock")
	request.ModelProviders = &[]api.LLMModelProvider{
		{Id: "claude", Models: &[]api.LLMModel{{Id: "claude-3-5-sonnet"}}},
		{Id: "deepseek", Models: &[]api.LLMModel{{Id: "deepseek-r1"}}},
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

func TestLLMProviderServiceCreateReturnsConflictForDuplicateHandle(t *testing.T) {
	providerRepo := &mockLLMProviderRepo{existsResult: true}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai"}, nil
		},
	}
	service := NewLLMProviderService(providerRepo, templateRepo, nil, nil)

	_, err := service.Create("org-1", "alice", validProviderRequest("openai"))
	if err != constants.ErrLLMProviderExists {
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
	service := NewLLMProviderService(providerRepo, templateRepo, nil, nil)

	request := validProviderRequest("openai")
	request.Name = "Updated Provider"
	request.Upstream.Main.Auth = &api.UpstreamAuth{
		Type:   upstreamAuthTypePtr("api-key"),
		Header: stringPtr("Authorization"),
		Value:  stringPtr(""),
	}

	_, err := service.Update("org-1", "provider-1", request)
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
	service := NewLLMProxyService(proxyRepo, providerRepo, projectRepo)

	_, err := service.Create("org-1", "alice", validProxyRequest("provider-1", "project-1"))
	if err != constants.ErrLLMProviderNotFound {
		t.Fatalf("expected ErrLLMProviderNotFound, got: %v", err)
	}
}

func TestLLMProxyServiceCreateReturnsConflictForDuplicateHandle(t *testing.T) {
	proxyRepo := &mockLLMProxyRepo{existsResult: true}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil)

	_, err := service.Create("org-1", "alice", validProxyRequest("provider-1", "project-1"))
	if err != constants.ErrLLMProxyExists {
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
			Status:      "pending",
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
	service := NewLLMProxyService(proxyRepo, providerRepo, nil)

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
	service := NewLLMProxyService(proxyRepo, providerRepo, nil)

	request := validProxyRequest("provider-1", "project-1")
	request.Name = "Updated Proxy"
	request.Provider.Auth = &api.UpstreamAuth{
		Type:   upstreamAuthTypePtr("api-key"),
		Header: stringPtr("Authorization"),
		Value:  stringPtr(""),
	}

	_, err := service.Update("org-1", "proxy-1", request)
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

func validProviderRequest(template string) *api.LLMProvider {
	return &api.LLMProvider{
		Id:       "provider-1",
		Name:     "Test Provider",
		Version:  "v1.0",
		Template: template,
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{Url: stringPtr("https://example.com/openai/v1")},
		},
		AccessControl: api.LLMAccessControl{Mode: "allow_all"},
	}
}

func validProxyRequest(providerID, projectID string) *api.LLMProxy {
	return &api.LLMProxy{
		Id:        "proxy-1",
		Name:      "Test Proxy",
		Version:   "v1.0",
		ProjectId: projectID,
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
