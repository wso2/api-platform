package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
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
		{name: "other", input: "other", expected: "other"},
		{name: "none", input: "none", expected: "none"},
		{name: "none upper", input: "NONE", expected: "none"},
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

// TestMapLLMUpstreamYAMLToModel_DefaultsToNone verifies the DP->CP import default:
// a gateway-pushed provider whose upstream.auth block is absent (or empty-typed)
// is stored with auth type "none", while an explicit "other" type is preserved.
func TestMapLLMUpstreamYAMLToModel_DefaultsToNone(t *testing.T) {
	// No auth block => "none".
	got := mapLLMUpstreamYAMLToModel(dto.LLMUpstreamYAML{URL: "https://api.openai.com/v1"})
	if got == nil || got.Main == nil || got.Main.Auth == nil || got.Main.Auth.Type != "none" {
		t.Fatalf("expected auth type 'none' for absent auth, got %+v", got)
	}

	// Explicit "other" is preserved.
	otherType := api.Other
	got = mapLLMUpstreamYAMLToModel(dto.LLMUpstreamYAML{
		URL:  "https://api.openai.com/v1",
		Auth: &api.UpstreamAuth{Type: &otherType},
	})
	if got == nil || got.Main == nil || got.Main.Auth == nil || got.Main.Auth.Type != "other" {
		t.Fatalf("expected auth type 'other' preserved, got %+v", got)
	}
}

// TestMapUpstreamConfigToDTO_ReturnsAuthAsIs verifies the read (GET) path returns the stored
// upstream config as-is: no auth block is synthesised when none is stored, and a stored type
// is returned unchanged.
func TestMapUpstreamConfigToDTO_ReturnsAuthAsIs(t *testing.T) {
	// No stored auth -> no auth block in the response (not defaulted to "none").
	out := mapUpstreamConfigToDTO(&model.UpstreamConfig{
		Main: &model.UpstreamEndpoint{URL: "https://api.openai.com/v1"},
	})
	if out.Main.Auth != nil {
		t.Fatalf("expected no auth block for stored nil auth, got %+v", out.Main.Auth)
	}

	// A stored explicit type is returned unchanged.
	out = mapUpstreamConfigToDTO(&model.UpstreamConfig{
		Main: &model.UpstreamEndpoint{URL: "https://api.openai.com/v1", Auth: &model.UpstreamAuth{Type: "none"}},
	})
	if out.Main.Auth == nil || out.Main.Auth.Type == nil || string(*out.Main.Auth.Type) != "none" {
		t.Fatalf("expected stored auth type 'none' returned as-is, got %+v", out.Main.Auth)
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

func TestMapSecurityModelToAPI_PreservesValuePrefix(t *testing.T) {
	in := &model.SecurityConfig{
		Enabled: utils.BoolPtr(true),
		APIKey: &model.APIKeySecurity{
			Enabled:     utils.BoolPtr(true),
			Key:         "Authorization",
			In:          "header",
			ValuePrefix: "Bearer",
		},
	}

	out := mapSecurityModelToAPI(in)
	if out == nil || out.ApiKey == nil {
		t.Fatal("expected api key security to be present")
	}
	if out.ApiKey.ValuePrefix == nil || *out.ApiKey.ValuePrefix != "Bearer" {
		t.Fatalf("expected inbound value prefix to be preserved, got %v", out.ApiKey.ValuePrefix)
	}
}

func TestMapSecurityAPIToModel_PreservesValuePrefix(t *testing.T) {
	inLoc := api.APIKeySecurityInHeader
	in := &api.SecurityConfig{
		Enabled: utils.BoolPtr(true),
		ApiKey: &api.APIKeySecurity{
			Enabled:     utils.BoolPtr(true),
			Key:         utils.StringPtrIfNotEmpty("Authorization"),
			In:          &inLoc,
			ValuePrefix: utils.StringPtrIfNotEmpty("Bearer"),
		},
	}

	out := mapSecurityAPIToModel(in)
	if out == nil || out.APIKey == nil {
		t.Fatal("expected api key security to be present")
	}
	if out.APIKey.ValuePrefix != "Bearer" {
		t.Fatalf("expected inbound value prefix to be preserved, got %q", out.APIKey.ValuePrefix)
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
					Enabled:     &trueValue,
					Key:         "Authorization",
					In:          "header",
					ValuePrefix: "Bearer",
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
	if (*policy.Params)["key"] != "Authorization" {
		t.Fatalf("expected params.key Authorization, got: %#v", (*policy.Params)["key"])
	}
	if (*policy.Params)["in"] != "header" {
		t.Fatalf("expected params.in header, got: %#v", (*policy.Params)["in"])
	}
	if (*policy.Params)["valuePrefix"] != "Bearer" {
		t.Fatalf("expected params.valuePrefix Bearer, got: %#v", (*policy.Params)["valuePrefix"])
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
					Enabled:     &trueValue,
					Key:         "Authorization",
					In:          "header",
					ValuePrefix: "Bearer",
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
	if apiKeyPolicy.Params == nil || (*apiKeyPolicy.Params)["key"] != "Authorization" {
		t.Fatalf("expected api-key-auth params.key Authorization")
	}
	if (*apiKeyPolicy.Params)["valuePrefix"] != "Bearer" {
		t.Fatalf("expected api-key-auth params.valuePrefix Bearer")
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

	requestPolicy := findGlobalPolicy(out.Spec.GlobalPolicies, "basic-ratelimit")
	if requestPolicy == nil {
		t.Fatalf("expected basic-ratelimit global policy to exist")
	}
	if requestPolicy.Params == nil {
		t.Fatalf("expected request policy to have params")
	}
	if _, ok := (*requestPolicy.Params)["keyExtraction"]; ok {
		t.Fatalf("expected no keyExtraction on global basic-ratelimit, got: %#v", (*requestPolicy.Params)["keyExtraction"])
	}
	limits, ok := (*requestPolicy.Params)["limits"].([]interface{})
	if !ok || len(limits) != 1 {
		t.Fatalf("expected limits with one entry, got: %#v", (*requestPolicy.Params)["limits"])
	}
	firstRequestLimit, ok := limits[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first request limit as object, got: %#v", limits[0])
	}
	if firstRequestLimit["requests"] != 1 {
		t.Fatalf("expected request count 1, got: %#v", firstRequestLimit["requests"])
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

	requestPolicy := findOperationPolicy(out.Spec.OperationPolicies, "basic-ratelimit")
	if requestPolicy == nil {
		t.Fatalf("expected basic-ratelimit operation policy to exist")
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
		limits, ok := p.Params["limits"].([]interface{})
		if !ok || len(limits) != 1 {
			t.Fatalf("expected limits with one entry, got: %#v", p.Params["limits"])
		}
		firstRequestLimit, ok := limits[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected request limit object, got: %#v", limits[0])
		}
		if firstRequestLimit["requests"] != 1 {
			t.Fatalf("expected request count 1, got: %#v", firstRequestLimit["requests"])
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

	requestPolicy := findOperationPolicy(out.Spec.OperationPolicies, "basic-ratelimit")
	if requestPolicy == nil {
		t.Fatalf("expected basic-ratelimit operation policy to exist")
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
		limits, ok := p.Params["limits"].([]interface{})
		if !ok || len(limits) != 1 {
			t.Fatalf("expected limits with one entry, got: %#v", p.Params["limits"])
		}
		firstRequestLimit, ok := limits[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected request limit object, got: %#v", limits[0])
		}
		if firstRequestLimit["requests"] != 1 || firstRequestLimit["duration"] != "1h" {
			t.Fatalf("expected request limit {requests:1,duration:1h}, got: %#v", firstRequestLimit)
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
	deleted      string
}

func (m *mockLLMProviderRepo) Delete(providerID, orgUUID string) error {
	m.deleted = providerID
	return nil
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

func (m *mockLLMProviderRepo) CreateWithCustomPolicyUsages(p *model.LLMProvider, _ []string) error {
	return m.Create(p)
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

func (m *mockLLMProviderRepo) UpdateWithCustomPolicyUsages(p *model.LLMProvider, _ []string) error {
	return m.Update(p)
}

type mockLLMTemplateRepo struct {
	repository.LLMProviderTemplateRepository
	getByIDFunc   func(templateID, orgUUID string) (*model.LLMProviderTemplate, error)
	getByUUIDFunc func(uuid, orgUUID string) (*model.LLMProviderTemplate, error)
	// known answers the existence question the inbound-interface check asks.
	// Like the real Exists, it ignores the enabled flag, which is
	// what makes a disabled template acceptable.
	known map[string]bool
}

func (m *mockLLMTemplateRepo) Exists(templateID, orgUUID string) (bool, error) {
	return m.known[templateID], nil
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
	listItems            []*model.LLMProxy
	listByProviderItems  []*model.LLMProxy
	lastListProviderUUID string
	getByIDFunc          func(proxyID, orgUUID string) (*model.LLMProxy, error)
	created              *model.LLMProxy
	updated              *model.LLMProxy
}

func (m *mockLLMProxyRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	if offset >= len(m.listItems) {
		return nil, nil
	}
	items := m.listItems[offset:]
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
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

func (m *mockProjectRepo) GetProjectByUUIDAndOrgID(projectID, orgID string) (*model.Project, error) {
	if m.project != nil && m.project.OrganizationID != "" && m.project.OrganizationID != orgID {
		return nil, nil
	}
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
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai", Enabled: true}, nil
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
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai", Enabled: true}, nil
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

// TestLLMProxyServiceListByProviderReportsEitherRole: the
// provider-proxies listing must report a proxy that references the provider as
// an **additional** provider, not only as its primary. This query previously ran
// on the provider_uuid column, which sees the primary alone — so the listing
// disagreed with what a deletion guard would need to enforce.
func TestLLMProxyServiceListByProviderReportsEitherRole(t *testing.T) {
	now := time.Now()
	proxyRepo := &mockLLMProxyRepo{
		listItems: []*model.LLMProxy{
			{
				UUID: "proxy-uuid", ID: "proxy-1", Name: "Proxy One", Version: "v1.0",
				ProjectUUID: "project-1", CreatedAt: now, UpdatedAt: now,
				Configuration: model.LLMProxyConfig{
					Providers: []model.LLMProxyAttachment{{ID: "provider-1", IsPrimary: true}},
					Context:   stringPtr("/assistant"),
				},
			},
			{
				UUID: "proxy-uuid-2", ID: "proxy-2", Name: "Proxy Two", Version: "v1.0",
				ProjectUUID: "project-1", CreatedAt: now, UpdatedAt: now,
				Configuration: model.LLMProxyConfig{
					Providers: []model.LLMProxyAttachment{
						{ID: "other-provider", IsPrimary: true},
						{ID: "provider-1"},
					},
					Context: stringPtr("/secondary"),
				},
			},
			{
				UUID: "proxy-uuid-3", ID: "proxy-3", Name: "Unrelated", Version: "v1.0",
				ProjectUUID: "project-1", CreatedAt: now, UpdatedAt: now,
				Configuration: model.LLMProxyConfig{
					Providers: []model.LLMProxyAttachment{{ID: "other-provider", IsPrimary: true}},
				},
			},
		},
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	projectRepo := &mockProjectRepo{project: &model.Project{ID: "project-1", Handle: "test-project", OrganizationID: "org-1"}}
	service := NewLLMProxyService(proxyRepo, providerRepo, projectRepo, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	resp, err := service.ListByProvider("org-1", "provider-1", 10, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil || resp.Count != 2 || len(resp.List) != 2 {
		t.Fatalf("expected both the primary and additional references, got: %#v", resp)
	}
	if *resp.List[1].Id != "proxy-2" {
		t.Fatalf("expected the additional-provider reference to be listed, got: %q", *resp.List[1].Id)
	}
	// projectId must be the project handle, not the stored UUID: clients route
	// on handles and cannot resolve a project UUID back to one.
	if resp.List[0].ProjectId == nil || *resp.List[0].ProjectId != "test-project" {
		t.Fatalf("expected projectId to be the project handle, got: %v", resp.List[0].ProjectId)
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
	if proxyRepo.updated == nil {
		t.Fatalf("expected the proxy to reach the repository")
	}
	// Preservation now works per attachment: reads redact every
	// credential, so a client writing back what it read sends an empty value for
	// each, and each must be carried forward from the stored attachment.
	primary, err := model.PrimaryLLMProxyAttachment(proxyRepo.updated.Configuration)
	if err != nil {
		t.Fatalf("normalise updated proxy: %v", err)
	}
	if primary.Auth == nil {
		t.Fatalf("expected updated proxy auth to be set")
	}
	if primary.Auth.Value != "Bearer old-secret" {
		t.Fatalf("expected proxy auth value to be preserved, got %q", primary.Auth.Value)
	}
}

// TestLLMProviderServiceCreate_DisabledTemplate_Rejected proves a provider
// cannot be created against a disabled template.
func TestLLMProviderServiceCreate_DisabledTemplate_Rejected(t *testing.T) {
	providerRepo := &mockLLMProviderRepo{}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai", Enabled: false}, nil
		},
	}
	service := NewLLMProviderService(providerRepo, templateRepo, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	_, err := service.Create("org-1", "alice", validProviderRequest("openai"))
	if !apperror.LLMProviderTemplateDisabled.Is(err) {
		t.Fatalf("expected LLMProviderTemplateDisabled, got: %v", err)
	}
	if providerRepo.created != nil {
		t.Error("expected provider creation to be aborted, but repo.Create was called")
	}
}

// TestLLMProviderServiceUpdate_DisabledTemplate_Rejected proves a provider
// cannot be updated to reference a disabled template.
func TestLLMProviderServiceUpdate_DisabledTemplate_Rejected(t *testing.T) {
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "prov-uuid", ID: providerID, TemplateUUID: "tpl-openai"}, nil
		},
	}
	templateRepo := &mockLLMTemplateRepo{
		getByIDFunc: func(templateID, orgUUID string) (*model.LLMProviderTemplate, error) {
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai", Enabled: false}, nil
		},
	}
	service := NewLLMProviderService(providerRepo, templateRepo, nil, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	_, err := service.Update("org-1", "provider-1", "alice", validProviderRequest("openai"))
	if !apperror.LLMProviderTemplateDisabled.Is(err) {
		t.Fatalf("expected LLMProviderTemplateDisabled, got: %v", err)
	}
	if providerRepo.updated != nil {
		t.Error("expected provider update to be aborted, but repo.Update was called")
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
			return &model.LLMProviderTemplate{UUID: "tpl-openai", ID: "openai", Enabled: true}, nil
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
		Provider: &api.LLMProxyProvider{
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

func TestLLMProxyServiceCreateFailsWhenAdditionalProviderNotFound(t *testing.T) {
	proxyRepo := &mockLLMProxyRepo{}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			if providerID == "provider-1" {
				return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
			}
			return nil, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	req := validProxyRequest("provider-1", "project-1")
	req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{{Id: "missing-provider"}}

	if _, err := service.Create("org-1", "alice", req); !apperror.LLMProviderRefNotFound.Is(err) {
		t.Fatalf("expected LLMProviderRefNotFound, got: %v", err)
	}
}

func TestLLMProxyServiceCreateFailsWhenAdditionalProviderNameCollides(t *testing.T) {
	proxyRepo := &mockLLMProxyRepo{}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	req := validProxyRequest("provider-1", "project-1")
	// The additional provider exists, but its upstream `as` name collides with
	// the primary provider id, which the gateway rejects at transform time.
	req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{
		{Id: "provider-2", As: stringPtr("provider-1")},
	}

	if _, err := service.Create("org-1", "alice", req); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ValidationFailed, got: %v", err)
	}
}

func TestLLMProxyServiceUpdateFailsWhenAdditionalProviderNotFound(t *testing.T) {
	now := time.Now()
	proxyRepo := &mockLLMProxyRepo{}
	proxyRepo.getByIDFunc = func(proxyID, orgUUID string) (*model.LLMProxy, error) {
		return &model.LLMProxy{
			UUID:          "proxy-uuid",
			ID:            proxyID,
			Name:          "Old Proxy",
			Version:       "v1.0",
			ProjectUUID:   "project-1",
			ProviderUUID:  "provider-uuid",
			CreatedAt:     now,
			UpdatedAt:     now,
			Configuration: model.LLMProxyConfig{Provider: "provider-1"},
		}, nil
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			if providerID == "provider-1" {
				return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
			}
			return nil, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	req := validProxyRequest("provider-1", "project-1")
	req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{{Id: "missing-provider"}}

	if _, err := service.Update("org-1", "proxy-1", "test-user", req); !apperror.LLMProviderRefNotFound.Is(err) {
		t.Fatalf("expected LLMProviderRefNotFound, got: %v", err)
	}
	if proxyRepo.updated != nil {
		t.Fatalf("expected update to be rejected before persisting, but proxy was updated")
	}
}

func TestLLMProxyServiceUpdateFailsWhenAdditionalProviderNameCollides(t *testing.T) {
	now := time.Now()
	proxyRepo := &mockLLMProxyRepo{}
	proxyRepo.getByIDFunc = func(proxyID, orgUUID string) (*model.LLMProxy, error) {
		return &model.LLMProxy{
			UUID:          "proxy-uuid",
			ID:            proxyID,
			Name:          "Old Proxy",
			Version:       "v1.0",
			ProjectUUID:   "project-1",
			ProviderUUID:  "provider-uuid",
			CreatedAt:     now,
			UpdatedAt:     now,
			Configuration: model.LLMProxyConfig{Provider: "provider-1"},
		}, nil
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "provider-uuid", ID: providerID}, nil
		},
	}
	service := NewLLMProxyService(proxyRepo, providerRepo, nil, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	req := validProxyRequest("provider-1", "project-1")
	// Two additional providers resolve to the same upstream `as` name.
	req.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{
		{Id: "provider-2", As: stringPtr("shared")},
		{Id: "provider-3", As: stringPtr("shared")},
	}

	if _, err := service.Update("org-1", "proxy-1", "test-user", req); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ValidationFailed, got: %v", err)
	}
	if proxyRepo.updated != nil {
		t.Fatalf("expected update to be rejected before persisting, but proxy was updated")
	}
}

// A project UUID stored on a proxy is not itself proof of tenancy — resolving
// it must be scoped to the caller's organization, so a UUID belonging to another
// org resolves to nothing rather than leaking that org's project handle.
func TestLLMProxyServiceResolveProjectHandleIsOrgScoped(t *testing.T) {
	projectRepo := &mockProjectRepo{project: &model.Project{ID: "project-1", Handle: "test-project", OrganizationID: "org-1"}}
	service := NewLLMProxyService(nil, nil, projectRepo, nil, nil, nil, slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())

	handle, err := service.resolveProjectHandle("org-1", "project-1", nil)
	if err != nil || handle != "test-project" {
		t.Fatalf("expected same-org resolution to succeed, got handle=%q err=%v", handle, err)
	}

	if _, err := service.resolveProjectHandle("org-2", "project-1", nil); !apperror.ProjectNotFound.Is(err) {
		t.Fatalf("expected ProjectNotFound for a cross-org project uuid, got: %v", err)
	}
}

// The sandbox upstream is optional, but when supplied it carries the same
// exactly-one-of-url-or-ref constraint as main — an unvalidated sandbox would let
// an ambiguous (both) or empty (neither) endpoint through.
func TestValidateUpstreamValidatesSandbox(t *testing.T) {
	url := "https://api.example.com"
	ref := "openai-default"
	main := api.UpstreamDefinition{Url: &url}

	if err := validateUpstream(api.Upstream{Main: main}); err != nil {
		t.Fatalf("expected an absent sandbox to be accepted: %v", err)
	}
	if err := validateUpstream(api.Upstream{Main: main, Sandbox: &api.UpstreamDefinition{Ref: &ref}}); err != nil {
		t.Fatalf("expected a ref-only sandbox to be accepted: %v", err)
	}
	if err := validateUpstream(api.Upstream{Main: main, Sandbox: &api.UpstreamDefinition{Url: &url, Ref: &ref}}); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ValidationFailed for a sandbox with both url and ref, got: %v", err)
	}
	if err := validateUpstream(api.Upstream{Main: main, Sandbox: &api.UpstreamDefinition{}}); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected ValidationFailed for a sandbox with neither url nor ref, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Multi-provider proxy support
//
// The layered rule these tests hold the service to: either shape in, both
// shapes out, canonical in storage.
// ---------------------------------------------------------------------------

// newProxyServiceForShapeTest builds the proxy service over in-memory repositories
// that echo a created or updated proxy back on read, which is what lets one test
// cover the whole write-then-read path.
func newProxyServiceForShapeTest(t *testing.T, stored *model.LLMProxy) (*LLMProxyService, *mockLLMProxyRepo) {
	t.Helper()

	proxyRepo := &mockLLMProxyRepo{}
	proxyRepo.getByIDFunc = func(proxyID, orgUUID string) (*model.LLMProxy, error) {
		switch {
		case proxyRepo.updated != nil:
			return proxyRepo.updated, nil
		case proxyRepo.created != nil:
			return proxyRepo.created, nil
		default:
			return stored, nil
		}
	}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: providerID + "-uuid", ID: providerID}, nil
		},
	}
	projectRepo := &mockProjectRepo{project: &model.Project{
		ID: "project-1", Handle: "project-1", OrganizationID: "org-1",
	}}
	service := NewLLMProxyService(proxyRepo, providerRepo, projectRepo, nil, nil, nil,
		slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	return service, proxyRepo
}

// legacyShapedProxyRequest and canonicalShapedProxyRequest describe the *same*
// three-provider proxy in each shape.
func legacyShapedProxyRequest() *api.LLMProxy {
	return &api.LLMProxy{
		Id:          strPointer("shape-proxy"),
		DisplayName: "Shape Proxy",
		Version:     "v1.0",
		ProjectId:   "project-1",
		Provider: &api.LLMProxyProvider{
			Id:   "openai-provider",
			Auth: &api.UpstreamAuth{Type: upstreamAuthTypePtr("api-key"), Header: stringPtr("Authorization"), Value: stringPtr("{{ secret \"openai\" }}")},
		},
		AdditionalProviders: &[]api.LLMProxyAdditionalProvider{
			{
				Id:          "anthropic-provider",
				As:          stringPtr("claude"),
				Auth:        &api.UpstreamAuth{Type: upstreamAuthTypePtr("api-key"), Header: stringPtr("x-api-key"), Value: stringPtr("{{ secret \"anthropic\" }}")},
				Transformer: &api.LLMProxyTransformer{Type: "openai-to-anthropic", Version: "v0"},
			},
			{Id: "gemini-provider"},
		},
	}
}

func canonicalShapedProxyRequest() *api.LLMProxy {
	return &api.LLMProxy{
		Id:          strPointer("shape-proxy"),
		DisplayName: "Shape Proxy",
		Version:     "v1.0",
		ProjectId:   "project-1",
		Providers: &[]api.LLMProxyProviderEntry{
			{
				Id:        "openai-provider",
				IsPrimary: true,
				Auth:      &api.UpstreamAuth{Type: upstreamAuthTypePtr("api-key"), Header: stringPtr("Authorization"), Value: stringPtr("{{ secret \"openai\" }}")},
			},
			{
				Id:          "anthropic-provider",
				Alias:       stringPtr("claude"),
				Auth:        &api.UpstreamAuth{Type: upstreamAuthTypePtr("api-key"), Header: stringPtr("x-api-key"), Value: stringPtr("{{ secret \"anthropic\" }}")},
				Transformer: &api.LLMProxyTransformer{Type: "openai-to-anthropic", Version: "v0"},
			},
			{Id: "gemini-provider"},
		},
	}
}

// TestLLMProxyBothShapesStoreIdenticalRows is the equivalence property at the
// storage layer: the same proxy described either way must
// produce a byte-identical stored configuration, so nothing downstream — the
// response, the artefact, the deletion guard — can tell which shape was used.
func TestLLMProxyBothShapesStoreIdenticalRows(t *testing.T) {
	legacyService, legacyRepo := newProxyServiceForShapeTest(t, nil)
	if _, err := legacyService.Create("org-1", "alice", legacyShapedProxyRequest()); err != nil {
		t.Fatalf("legacy create failed: %v", err)
	}
	canonicalService, canonicalRepo := newProxyServiceForShapeTest(t, nil)
	if _, err := canonicalService.Create("org-1", "alice", canonicalShapedProxyRequest()); err != nil {
		t.Fatalf("canonical create failed: %v", err)
	}

	fromLegacy, err := json.Marshal(legacyRepo.created.Configuration)
	if err != nil {
		t.Fatalf("marshal legacy configuration: %v", err)
	}
	fromCanonical, err := json.Marshal(canonicalRepo.created.Configuration)
	if err != nil {
		t.Fatalf("marshal canonical configuration: %v", err)
	}
	if !bytes.Equal(fromLegacy, fromCanonical) {
		t.Fatalf("the two shapes stored different rows:\nlegacy:    %s\ncanonical: %s", fromLegacy, fromCanonical)
	}
}

// TestLLMProxyStoresCanonicalShapeOnly: whatever arrived, the
// database receives the canonical list and none of the legacy fields.
func TestLLMProxyStoresCanonicalShapeOnly(t *testing.T) {
	service, repo := newProxyServiceForShapeTest(t, nil)
	if _, err := service.Create("org-1", "alice", legacyShapedProxyRequest()); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	stored := repo.created.Configuration
	if len(stored.Providers) != 3 {
		t.Fatalf("expected the canonical list to be stored, got %d entries", len(stored.Providers))
	}
	if stored.Provider != "" || stored.UpstreamAuth != nil || stored.AdditionalProviders != nil {
		t.Fatalf("expected no legacy field to be persisted, got provider=%q upstreamAuth=%+v additional=%+v",
			stored.Provider, stored.UpstreamAuth, stored.AdditionalProviders)
	}
	if !stored.Providers[0].IsPrimary {
		t.Fatal("expected the primary to lead the stored list")
	}
}

// TestLLMProxyReadReturnsBothRepresentations covers a row
// stored in **each** shape: both views are present and they agree.
func TestLLMProxyReadReturnsBothRepresentations(t *testing.T) {
	cases := []struct {
		name   string
		stored model.LLMProxyConfig
	}{
		{
			name: "canonical row",
			stored: model.LLMProxyConfig{
				Providers: []model.LLMProxyAttachment{
					{ID: "openai-provider", IsPrimary: true, Auth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: "secret"}},
					{ID: "anthropic-provider", Alias: "claude", Transformer: &model.LLMProxyTransformer{Type: "openai-to-anthropic", Version: "v0"}},
				},
			},
		},
		{
			// An older row normalises on read, so it is
			// indistinguishable from a canonical one at the API surface.
			name: "legacy row",
			stored: model.LLMProxyConfig{
				Provider:     "openai-provider",
				UpstreamAuth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: "secret"},
				AdditionalProviders: []model.LLMProxyAdditionalProvider{
					{ID: "anthropic-provider", As: "claude", Transformer: &model.LLMProxyTransformer{Type: "openai-to-anthropic", Version: "v0"}},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := mapProxyModelToAPI(&model.LLMProxy{ID: "p", Name: "P", Version: "v1.0", Configuration: tc.stored})

			if out.Provider == nil {
				t.Fatal("expected the legacy provider field to be populated on read")
			}
			if out.Providers == nil {
				t.Fatal("expected the canonical providers list to be populated on read")
			}
			providers := *out.Providers
			if len(providers) != 2 {
				t.Fatalf("expected two attachments, got %d", len(providers))
			}
			// The two views must name the same primary...
			if providers[0].Id != out.Provider.Id || !providers[0].IsPrimary {
				t.Fatalf("representations disagree on the primary: %q vs %q", providers[0].Id, out.Provider.Id)
			}
			// ...and the same additional providers, under either field name.
			if out.AdditionalProviders == nil || len(*out.AdditionalProviders) != 1 {
				t.Fatalf("expected one additional provider, got %+v", out.AdditionalProviders)
			}
			additional := (*out.AdditionalProviders)[0]
			if additional.Id != providers[1].Id {
				t.Fatalf("representations disagree on the additional provider: %q vs %q", additional.Id, providers[1].Id)
			}
			if *additional.As != *providers[1].Alias {
				t.Fatalf("`as` and `alias` disagree: %q vs %q", *additional.As, *providers[1].Alias)
			}
			if providers[1].IsPrimary {
				t.Fatal("expected a non-primary entry to carry isPrimary false explicitly")
			}
		})
	}
}

// TestLLMProxyResponseNeverDisclosesCredential scans
// the **whole** serialised response for a "value" key rather than checking each
// field, so a future shape that carries a credential cannot slip past by being
// somewhere this test did not think to look.
func TestLLMProxyResponseNeverDisclosesCredential(t *testing.T) {
	const credential = "super-secret-credential"
	stored := model.LLMProxyConfig{
		Providers: []model.LLMProxyAttachment{
			{ID: "openai-provider", IsPrimary: true, Auth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: credential}},
			{ID: "anthropic-provider", Alias: "claude", Auth: &model.UpstreamAuth{Type: "api-key", Header: "x-api-key", Value: credential}},
			{ID: "gemini-provider", Auth: &model.UpstreamAuth{Type: "api-key", Header: "x-goog-api-key", Value: credential}},
		},
	}

	out := mapProxyModelToAPI(&model.LLMProxy{ID: "p", Name: "P", Version: "v1.0", Configuration: stored})
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if bytes.Contains(encoded, []byte(credential)) {
		t.Fatalf("a credential value survived into the response: %s", encoded)
	}

	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if path := findAuthValueKey("$", decoded); path != "" {
		t.Fatalf("an auth object carried a value field at %s: %s", path, encoded)
	}
}

// findAuthValueKey walks a decoded response and reports the path of the first
// auth object carrying a "value", or "" when none does.
func findAuthValueKey(path string, node any) string {
	switch typed := node.(type) {
	case map[string]any:
		if auth, ok := typed["auth"].(map[string]any); ok {
			if _, present := auth["value"]; present {
				return path + ".auth.value"
			}
		}
		for key, value := range typed {
			if found := findAuthValueKey(path+"."+key, value); found != "" {
				return found
			}
		}
	case []any:
		for i, value := range typed {
			if found := findAuthValueKey(fmt.Sprintf("%s[%d]", path, i), value); found != "" {
				return found
			}
		}
	}
	return ""
}

// TestLLMProxyLegacyRowAutoMigratesOnFirstWrite runs against a genuine older
// stored payload: a legacy row updated in **either** shape is stored
// canonically afterwards, with no migration job and without the client knowing.
func TestLLMProxyLegacyRowAutoMigratesOnFirstWrite(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "compat", "legacy-stored-proxy.json"))
	if err != nil {
		t.Fatalf("read the legacy seed: %v", err)
	}

	cases := []struct {
		name    string
		request func() *api.LLMProxy
	}{
		{name: "legacy-shaped write", request: legacyShapedProxyRequest},
		{name: "canonical-shaped write", request: canonicalShapedProxyRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var config model.LLMProxyConfig
			if err := json.Unmarshal(raw, &config); err != nil {
				t.Fatalf("parse the legacy seed: %v", err)
			}
			// The seed really is in the older shape.
			if config.Provider == "" || len(config.Providers) != 0 {
				t.Fatalf("expected a legacy seed, got %+v", config)
			}

			service, repo := newProxyServiceForShapeTest(t, &model.LLMProxy{
				UUID: "proxy-uuid", ID: "shape-proxy", Name: "Legacy Stored Proxy",
				Version: "v1.0", ProviderUUID: "openai-provider-uuid", Configuration: config,
			})

			// A legacy-shaped write is only permitted here because the seed is a
			// single-provider... it is not, so the guard must refuse it.
			request := tc.request()
			_, err := service.Update("org-1", "shape-proxy", "alice", request)
			if requestUsesLegacyProviderShape(request) {
				// The seed has two providers, which the legacy shape cannot
				// express, so the guard refuses the write outright.
				if !apperror.ValidationFailed.Is(err) {
					t.Fatalf("expected the full-replace guard to refuse this write, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("canonical update failed: %v", err)
			}
			stored := repo.updated.Configuration
			if len(stored.Providers) == 0 {
				t.Fatal("expected the row to migrate to the canonical shape")
			}
			if stored.Provider != "" || stored.UpstreamAuth != nil || stored.AdditionalProviders != nil {
				t.Fatalf("expected the legacy fields to be gone after the write, got %+v", stored)
			}
		})
	}
}

// TestLLMProxyLegacyRowMigratesOnSingleProviderWrite is the case a deployed
// console actually hits: a single-provider legacy row, saved by a client that
// still speaks the old shape, migrates without the client knowing — and the
// guard does not get in the way.
func TestLLMProxyLegacyRowMigratesOnSingleProviderWrite(t *testing.T) {
	service, repo := newProxyServiceForShapeTest(t, &model.LLMProxy{
		UUID: "proxy-uuid", ID: "single", Name: "Single", Version: "v1.0",
		ProviderUUID: "openai-provider-uuid",
		Configuration: model.LLMProxyConfig{
			Provider:     "openai-provider",
			UpstreamAuth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: "stored"},
		},
	})

	request := &api.LLMProxy{
		DisplayName: "Single Renamed",
		Version:     "v1.0",
		ProjectId:   "project-1",
		Provider:    &api.LLMProxyProvider{Id: "openai-provider"},
	}
	if _, err := service.Update("org-1", "single", "alice", request); err != nil {
		t.Fatalf("expected a legacy single-provider update to be accepted, got: %v", err)
	}
	stored := repo.updated.Configuration
	if len(stored.Providers) != 1 || stored.Provider != "" {
		t.Fatalf("expected the row to migrate to the canonical shape, got %+v", stored)
	}
}

// TestLLMProxyUpdateRejectsLegacyWriteItCannotExpress covers the guard in both
// directions. Too strict would block an ordinary single-provider edit; too loose
// would let PUT's full replace silently destroy a multi-provider proxy.
func TestLLMProxyUpdateRejectsLegacyWriteItCannotExpress(t *testing.T) {
	cases := []struct {
		name       string
		stored     model.LLMProxyConfig
		request    func() *api.LLMProxy
		wantReject bool
	}{
		{
			name: "legacy write against a multi-provider proxy is refused",
			stored: model.LLMProxyConfig{Providers: []model.LLMProxyAttachment{
				{ID: "openai-provider", IsPrimary: true},
				{ID: "anthropic-provider"},
			}},
			request:    func() *api.LLMProxy { r := legacyShapedProxyRequest(); return r },
			wantReject: true,
		},
		{
			name: "legacy write against a proxy with an inbound interface is refused",
			stored: model.LLMProxyConfig{
				Providers:       []model.LLMProxyAttachment{{ID: "openai-provider", IsPrimary: true}},
				InboundTemplate: "openai",
			},
			request:    func() *api.LLMProxy { r := legacyShapedProxyRequest(); return r },
			wantReject: true,
		},
		{
			name: "legacy write against a single-provider proxy is accepted",
			stored: model.LLMProxyConfig{Providers: []model.LLMProxyAttachment{
				{ID: "openai-provider", IsPrimary: true},
			}},
			request: func() *api.LLMProxy {
				return &api.LLMProxy{
					DisplayName: "Renamed", Version: "v1.0", ProjectId: "project-1",
					Provider: &api.LLMProxyProvider{Id: "openai-provider"},
				}
			},
			wantReject: false,
		},
		{
			name: "canonical write against a multi-provider proxy is accepted",
			stored: model.LLMProxyConfig{Providers: []model.LLMProxyAttachment{
				{ID: "openai-provider", IsPrimary: true},
				{ID: "anthropic-provider"},
			}},
			request:    canonicalShapedProxyRequest,
			wantReject: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, repo := newProxyServiceForShapeTest(t, &model.LLMProxy{
				UUID: "proxy-uuid", ID: "guarded", Name: "Guarded", Version: "v1.0",
				ProviderUUID: "openai-provider-uuid", Configuration: tc.stored,
			})

			_, err := service.Update("org-1", "guarded", "alice", tc.request())
			if tc.wantReject {
				if !apperror.ValidationFailed.Is(err) {
					t.Fatalf("expected the write to be refused, got: %v", err)
				}
				if !strings.Contains(err.Error(), "providers") {
					t.Fatalf("expected the refusal to name what the client should send, got: %v", err)
				}
				// A refused write must change nothing. The guard runs before the
				// replacement configuration is built, so the stored proxy keeps
				// every provider and its interface.
				if repo.updated != nil {
					t.Fatalf("a refused write reached the repository: %+v", repo.updated.Configuration)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected the write to be accepted, got: %v", err)
			}
		})
	}
}

// TestLLMProxyRejectsMalformedProviderList: a
// canonical list with no primary, several primaries, or no entries at all, and a
// request declaring no provider in either shape.
func TestLLMProxyRejectsMalformedProviderList(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*api.LLMProxy)
		wantPart string
	}{
		{
			name: "no primary marked",
			mutate: func(r *api.LLMProxy) {
				entries := *r.Providers
				entries[0].IsPrimary = false
				r.Providers = &entries
			},
			wantPart: "none does",
		},
		{
			name: "two primaries marked",
			mutate: func(r *api.LLMProxy) {
				entries := *r.Providers
				entries[1].IsPrimary = true
				r.Providers = &entries
			},
			wantPart: "2 do",
		},
		{
			name:     "empty list",
			mutate:   func(r *api.LLMProxy) { r.Providers = &[]api.LLMProxyProviderEntry{} },
			wantPart: "must not be empty",
		},
		{
			name:     "no provider in either shape",
			mutate:   func(r *api.LLMProxy) { r.Providers = nil; r.Provider = nil },
			wantPart: "must declare at least one provider",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := newProxyServiceForShapeTest(t, nil)
			request := canonicalShapedProxyRequest()
			tc.mutate(request)

			_, err := service.Create("org-1", "alice", request)
			if !apperror.ValidationFailed.Is(err) {
				t.Fatalf("expected a validation failure, got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantPart) {
				t.Fatalf("expected the error to name the problem (%q), got: %v", tc.wantPart, err)
			}
		})
	}
}

// TestLLMProxyProvidersWinsWhenBothShapesSupplied. Rejecting would
// break the read-modify-write round trip, since every read returns both.
func TestLLMProxyProvidersWinsWhenBothShapesSupplied(t *testing.T) {
	service, repo := newProxyServiceForShapeTest(t, nil)

	request := canonicalShapedProxyRequest()
	// Stale legacy fields, as a client echoing an earlier read would send.
	request.Provider = &api.LLMProxyProvider{Id: "stale-provider"}
	request.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{{Id: "stale-additional"}}

	if _, err := service.Create("org-1", "alice", request); err != nil {
		t.Fatalf("expected both shapes to be accepted, got: %v", err)
	}
	stored := repo.created.Configuration
	if stored.Providers[0].ID != "openai-provider" {
		t.Fatalf("expected providers to win, got primary %q", stored.Providers[0].ID)
	}
	for _, attachment := range stored.Providers {
		if strings.HasPrefix(attachment.ID, "stale-") {
			t.Fatalf("a legacy field leaked into storage: %q", attachment.ID)
		}
	}
}

// TestLLMProxyPrimaryCarriesTransformerAndAlias: the primary is now
// structurally equal to any other attachment.
func TestLLMProxyPrimaryCarriesTransformerAndAlias(t *testing.T) {
	service, repo := newProxyServiceForShapeTest(t, nil)

	request := legacyShapedProxyRequest()
	request.Provider.As = stringPtr("openai-upstream")
	request.Provider.Transformer = &api.LLMProxyTransformer{Type: "openai-to-openai", Version: "v0"}

	created, err := service.Create("org-1", "alice", request)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	primary := repo.created.Configuration.Providers[0]
	if primary.Alias != "openai-upstream" {
		t.Fatalf("expected the primary's alias to persist, got %q", primary.Alias)
	}
	if primary.Transformer == nil || primary.Transformer.Type != "openai-to-openai" {
		t.Fatalf("expected the primary's transformer to persist, got %+v", primary.Transformer)
	}
	if created.Provider == nil || created.Provider.As == nil || *created.Provider.As != "openai-upstream" {
		t.Fatalf("expected the alias to be returned on read, got %+v", created.Provider)
	}
	if created.Provider.Transformer == nil {
		t.Fatal("expected the transformer to be returned on read")
	}
}

// TestLLMProxyRejectsAliasCollisionWithPrimary: uniqueness now spans every
// attachment. Previously the primary had no alias to collide with,
// so this collision was unreachable.
func TestLLMProxyRejectsAliasCollisionWithPrimary(t *testing.T) {
	service, _ := newProxyServiceForShapeTest(t, nil)

	request := legacyShapedProxyRequest()
	request.Provider.As = stringPtr("claude") // already used by an additional provider

	_, err := service.Create("org-1", "alice", request)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected the collision to be rejected, got: %v", err)
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Fatalf("expected the error to name the colliding name, got: %v", err)
	}
}

// TestLLMProxyRejectsMalformedAlias: validation here must not be
// looser than the gateway's, or an accepted proxy fails at deployment.
func TestLLMProxyRejectsMalformedAlias(t *testing.T) {
	for _, alias := range []string{"has spaces", "has/slash", strings.Repeat("a", 101)} {
		service, _ := newProxyServiceForShapeTest(t, nil)
		request := legacyShapedProxyRequest()
		request.Provider.As = stringPtr(alias)

		if _, err := service.Create("org-1", "alice", request); !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected alias %q to be rejected, got: %v", alias, err)
		}
	}
}

// TestLLMProxyInboundTemplate covers US4.
func TestLLMProxyInboundTemplate(t *testing.T) {
	t.Run("round-trips and reaches storage", func(t *testing.T) {
		service, repo := newProxyServiceForShapeTest(t, nil)
		service.SetTemplateRepository(&mockLLMTemplateRepo{known: map[string]bool{"openai": true}})

		request := canonicalShapedProxyRequest()
		request.InboundTemplate = stringPtr("openai")

		created, err := service.Create("org-1", "alice", request)
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if repo.created.Configuration.InboundTemplate != "openai" {
			t.Fatalf("expected the inbound template to persist, got %q", repo.created.Configuration.InboundTemplate)
		}
		if created.InboundTemplate == nil || *created.InboundTemplate != "openai" {
			t.Fatalf("expected the inbound template to be returned on read, got %v", created.InboundTemplate)
		}
	})

	t.Run("a disabled template is accepted", func(t *testing.T) {
		service, _ := newProxyServiceForShapeTest(t, nil)
		// Exists reports the template regardless of its enabled flag.
		service.SetTemplateRepository(&mockLLMTemplateRepo{known: map[string]bool{"disabled-template": true}})

		request := canonicalShapedProxyRequest()
		request.InboundTemplate = stringPtr("disabled-template")

		if _, err := service.Create("org-1", "alice", request); err != nil {
			t.Fatalf("expected a disabled template to be accepted, got: %v", err)
		}
	})

	t.Run("an unresolvable template is rejected, naming it", func(t *testing.T) {
		service, _ := newProxyServiceForShapeTest(t, nil)
		service.SetTemplateRepository(&mockLLMTemplateRepo{known: map[string]bool{"openai": true}})

		request := canonicalShapedProxyRequest()
		request.InboundTemplate = stringPtr("no-such-template")

		_, err := service.Create("org-1", "alice", request)
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected the template to be rejected, got: %v", err)
		}
		if !strings.Contains(err.Error(), "no-such-template") {
			t.Fatalf("expected the error to name the template, got: %v", err)
		}
	})

	t.Run("a proxy stored without one has none invented for it", func(t *testing.T) {
		service, repo := newProxyServiceForShapeTest(t, nil)
		if _, err := service.Create("org-1", "alice", canonicalShapedProxyRequest()); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if repo.created.Configuration.InboundTemplate != "" {
			t.Fatalf("expected no inbound template to be invented, got %q", repo.created.Configuration.InboundTemplate)
		}
		out := mapProxyModelToAPI(repo.created)
		if out.InboundTemplate != nil {
			t.Fatalf("expected no inbound template in the response, got %v", *out.InboundTemplate)
		}
	})

	t.Run("it is updatable", func(t *testing.T) {
		service, repo := newProxyServiceForShapeTest(t, &model.LLMProxy{
			UUID: "proxy-uuid", ID: "shape-proxy", Name: "Shape Proxy", Version: "v1.0",
			ProviderUUID: "openai-provider-uuid",
			Configuration: model.LLMProxyConfig{
				Providers:       []model.LLMProxyAttachment{{ID: "openai-provider", IsPrimary: true}},
				InboundTemplate: "openai",
			},
		})
		service.SetTemplateRepository(&mockLLMTemplateRepo{known: map[string]bool{"openai": true, "anthropic": true}})

		request := canonicalShapedProxyRequest()
		request.InboundTemplate = stringPtr("anthropic")

		if _, err := service.Update("org-1", "shape-proxy", "alice", request); err != nil {
			t.Fatalf("update failed: %v", err)
		}
		if repo.updated.Configuration.InboundTemplate != "anthropic" {
			t.Fatalf("expected the inbound template to be updated, got %q", repo.updated.Configuration.InboundTemplate)
		}
	})
}

// TestLLMProviderDeleteRefusesWhileReferenced covers US2.
// The additional-provider case is the live defect: those references live inside
// the proxy's configuration payload, where no database constraint can see them.
func TestLLMProviderDeleteRefusesWhileReferenced(t *testing.T) {
	proxiesIn := func(configs ...model.LLMProxyConfig) []*model.LLMProxy {
		proxies := make([]*model.LLMProxy, 0, len(configs))
		for i, config := range configs {
			proxies = append(proxies, &model.LLMProxy{
				UUID: fmt.Sprintf("uuid-%d", i), ID: fmt.Sprintf("proxy-%d", i), Configuration: config,
			})
		}
		return proxies
	}

	cases := []struct {
		name       string
		proxies    []*model.LLMProxy
		wantRefuse bool
		wantNames  []string
	}{
		{
			name: "referenced as an additional provider only",
			proxies: proxiesIn(model.LLMProxyConfig{Providers: []model.LLMProxyAttachment{
				{ID: "other-provider", IsPrimary: true},
				{ID: "target-provider"},
			}}),
			wantRefuse: true,
			wantNames:  []string{"proxy-0"},
		},
		{
			name: "referenced as the primary only",
			proxies: proxiesIn(model.LLMProxyConfig{Providers: []model.LLMProxyAttachment{
				{ID: "target-provider", IsPrimary: true},
			}}),
			wantRefuse: true,
			wantNames:  []string{"proxy-0"},
		},
		{
			name: "referenced in both roles across several proxies",
			proxies: proxiesIn(
				model.LLMProxyConfig{Providers: []model.LLMProxyAttachment{{ID: "target-provider", IsPrimary: true}}},
				model.LLMProxyConfig{Providers: []model.LLMProxyAttachment{
					{ID: "other-provider", IsPrimary: true},
					{ID: "target-provider"},
				}},
			),
			wantRefuse: true,
			wantNames:  []string{"proxy-0", "proxy-1"},
		},
		{
			name: "referenced by a legacy row",
			proxies: proxiesIn(model.LLMProxyConfig{
				Provider:            "other-provider",
				AdditionalProviders: []model.LLMProxyAdditionalProvider{{ID: "target-provider"}},
			}),
			wantRefuse: true,
			wantNames:  []string{"proxy-0"},
		},
		{
			name: "unreferenced",
			proxies: proxiesIn(model.LLMProxyConfig{Providers: []model.LLMProxyAttachment{
				{ID: "other-provider", IsPrimary: true},
			}}),
			wantRefuse: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			providerRepo := &mockLLMProviderRepo{
				getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
					return &model.LLMProvider{UUID: "target-uuid", ID: providerID}, nil
				},
			}
			service := NewLLMProviderService(providerRepo, nil, nil, nil, nil, nil, nil,
				slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
			service.SetProxyRepository(&mockLLMProxyRepo{listItems: tc.proxies})

			err := service.Delete("org-1", "target-provider", "alice")
			if !tc.wantRefuse {
				if err != nil {
					t.Fatalf("expected an unreferenced provider to be deletable, got: %v", err)
				}
				return
			}
			if !apperror.ValidationFailed.Is(err) {
				t.Fatalf("expected the deletion to be refused, got: %v", err)
			}
			for _, name := range tc.wantNames {
				if !strings.Contains(err.Error(), name) {
					t.Fatalf("expected the refusal to name %q, got: %v", name, err)
				}
			}
		})
	}
}

// TestLLMProviderDeleteRespectsOrganizationScoping: the scan
// inherits the repository's org filter and never widens visibility.
func TestLLMProviderDeleteRespectsOrganizationScoping(t *testing.T) {
	var scannedOrg string
	proxyRepo := &scopeRecordingProxyRepo{onList: func(orgUUID string) { scannedOrg = orgUUID }}
	providerRepo := &mockLLMProviderRepo{
		getByIDFunc: func(providerID, orgUUID string) (*model.LLMProvider, error) {
			return &model.LLMProvider{UUID: "target-uuid", ID: providerID}, nil
		},
	}
	service := NewLLMProviderService(providerRepo, nil, nil, nil, nil, nil, nil,
		slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
	service.SetProxyRepository(proxyRepo)

	if err := service.Delete("org-1", "target-provider", "alice"); err != nil {
		t.Fatalf("expected the delete to succeed, got: %v", err)
	}
	if scannedOrg != "org-1" {
		t.Fatalf("expected the scan to be scoped to org-1, got %q", scannedOrg)
	}
}

type scopeRecordingProxyRepo struct {
	repository.LLMProxyRepository
	onList func(orgUUID string)
}

func (r *scopeRecordingProxyRepo) List(orgUUID string, limit, offset int) ([]*model.LLMProxy, error) {
	r.onList(orgUUID)
	return nil, nil
}

// TestLLMProxyUpdateOfGatewayOriginatedLegacyRow is the regression guard for a
// panic: updating a gateway-originated proxy whose stored row predates the
// canonical provider list.
//
// That branch adopts the stored configuration verbatim, so before the fix it was
// the one path that reached the primary-auth default without having been through
// normalisation — and a stored row with no `providers` entry made it index an
// empty slice. Nothing migrates stored rows, so every gateway-originated proxy in
// an existing installation is in exactly that shape.
func TestLLMProxyUpdateOfGatewayOriginatedLegacyRow(t *testing.T) {
	stored := &model.LLMProxy{
		UUID: "proxy-uuid", ID: "dp-proxy", Name: "DP Proxy", Version: "v1.0",
		ProviderUUID: "openai-provider-uuid",
		Origin:       constants.OriginDP,
		Configuration: model.LLMProxyConfig{
			Provider:     "openai-provider",
			UpstreamAuth: &model.UpstreamAuth{Type: "api-key", Header: "Authorization", Value: "stored-credential"},
			AdditionalProviders: []model.LLMProxyAdditionalProvider{
				{ID: "anthropic-provider", As: "claude"},
			},
		},
	}
	service, repo := newProxyServiceForShapeTest(t, stored)

	request := &api.LLMProxy{
		DisplayName: "Renamed By Client",
		Version:     "v9.9",
		ProjectId:   "project-1",
		Description: stringPtr("control-plane metadata the client may change"),
		Provider:    &api.LLMProxyProvider{Id: "someone-elses-provider"},
	}

	if _, err := service.Update("org-1", "dp-proxy", "alice", request); err != nil {
		t.Fatalf("expected the update to succeed, got: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("expected the proxy to reach the repository")
	}

	// The gateway still owns the runtime configuration: the request's provider,
	// name and version are all ignored.
	if repo.updated.Name != "DP Proxy" || repo.updated.Version != "v1.0" {
		t.Fatalf("expected gateway-owned metadata to be preserved, got name=%q version=%q",
			repo.updated.Name, repo.updated.Version)
	}

	// The row is stored canonically afterwards, like any other write, and it
	// describes exactly what the gateway had.
	stored2 := repo.updated.Configuration
	if len(stored2.Providers) != 2 {
		t.Fatalf("expected the stored row to normalise to two attachments, got %d", len(stored2.Providers))
	}
	if stored2.Provider != "" || stored2.UpstreamAuth != nil || stored2.AdditionalProviders != nil {
		t.Fatalf("expected the pre-canonical fields to be gone after the write, got %+v", stored2)
	}
	primary := stored2.Providers[0]
	if primary.ID != "openai-provider" || !primary.IsPrimary {
		t.Fatalf("expected the gateway's own primary to survive, got %+v", primary)
	}
	if primary.Auth == nil || primary.Auth.Value != "stored-credential" {
		t.Fatalf("expected the gateway's credential to survive, got %+v", primary.Auth)
	}
	if stored2.Providers[1].ID != "anthropic-provider" || stored2.Providers[1].EffectiveName() != "claude" {
		t.Fatalf("expected the additional provider to survive with its alias, got %+v", stored2.Providers[1])
	}
}

// TestLLMProxyUpdateOfGatewayOriginatedRowWithoutAnyProvider covers the residual
// case the guard exists for: a stored row that names no provider at all cannot be
// normalised, and must still be reported rather than panicking.
func TestLLMProxyUpdateOfGatewayOriginatedRowWithoutAnyProvider(t *testing.T) {
	stored := &model.LLMProxy{
		UUID: "proxy-uuid", ID: "dp-proxy", Name: "DP Proxy", Version: "v1.0",
		Origin:        constants.OriginDP,
		Configuration: model.LLMProxyConfig{},
	}
	service, _ := newProxyServiceForShapeTest(t, stored)

	request := &api.LLMProxy{
		DisplayName: "DP Proxy", Version: "v1.0", ProjectId: "project-1",
		Provider: &api.LLMProxyProvider{Id: "openai-provider"},
	}

	// No panic, whatever else happens.
	if _, err := service.Update("org-1", "dp-proxy", "alice", request); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
