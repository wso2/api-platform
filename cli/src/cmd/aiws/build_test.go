/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package aiws

import "testing"

func newProxyRuntime() aiWorkspaceRuntime {
	var rt aiWorkspaceRuntime
	rt.Spec.Context = "/default/claude-proxy2"
	rt.Spec.Provider = runtimeProvider{
		ID:   "wso2-claude-provider",
		Auth: &runtimeProviderAuth{Type: "api-key", Header: "X-API-Key", Value: "{{ secret \"abc\" }}"},
	}
	rt.Spec.GlobalPolicies = []runtimeProviderPolicy{
		{Name: "api-key-auth", Version: "", Params: map[string]interface{}{"in": "header", "key": "X-API-Key"}},
		{Name: "aws-bedrock-guardrail", Version: "v1", Params: map[string]interface{}{
			"request": map[string]interface{}{"enabled": true, "jsonPath": "$.messages[-1].content"},
		}},
	}
	rt.Spec.OperationPolicies = []runtimeProviderPolicy{
		{Name: "basic-auth", Version: "v1", Paths: []runtimePolicyPath{
			{Path: "/v1/messages", Methods: []string{"POST"}, Params: map[string]interface{}{"username": "admin", "password": "admin"}},
		}},
	}
	return rt
}

func newProxyMetadata() aiWorkspaceMetadata {
	var md aiWorkspaceMetadata
	md.Metadata.Name = "claude-proxy2"
	md.Spec.DisplayName = "claude proxy2"
	md.Spec.Version = "v1.0"
	return md
}

func TestBuildLLMProxyPayload_MapsGlobalAndOperationPolicies(t *testing.T) {
	payload := buildLLMProxyPayload("claude-proxy2", newProxyMetadata(), newProxyRuntime(), "")

	if payload.ID != "claude-proxy2" || payload.DisplayName != "claude proxy2" || payload.Version != "v1.0" {
		t.Fatalf("unexpected identity fields: %+v", payload)
	}
	if payload.Description != defaultProxyDescription {
		t.Fatalf("expected default description, got %q", payload.Description)
	}

	// api-key-auth is lifted into security, not left in globalPolicies.
	if payload.Security == nil || payload.Security.APIKey == nil {
		t.Fatalf("expected security block from api-key-auth, got %+v", payload.Security)
	}
	if !payload.Security.Enabled || payload.Security.APIKey.In != "header" || payload.Security.APIKey.Key != "X-API-Key" {
		t.Fatalf("unexpected security block: %+v", payload.Security.APIKey)
	}

	// Remaining global policies pass through with their free-form params intact.
	if len(payload.GlobalPolicies) != 1 || payload.GlobalPolicies[0].Name != "aws-bedrock-guardrail" {
		t.Fatalf("expected 1 global policy (aws-bedrock-guardrail), got %+v", payload.GlobalPolicies)
	}
	req, ok := payload.GlobalPolicies[0].Params["request"].(map[string]interface{})
	if !ok || req["jsonPath"] != "$.messages[-1].content" {
		t.Fatalf("global policy params not preserved verbatim: %+v", payload.GlobalPolicies[0].Params)
	}

	// Operation policies keep their per-path free-form params.
	if len(payload.OperationPolicies) != 1 || payload.OperationPolicies[0].Name != "basic-auth" {
		t.Fatalf("expected 1 operation policy (basic-auth), got %+v", payload.OperationPolicies)
	}
	op := payload.OperationPolicies[0]
	if len(op.Paths) != 1 || op.Paths[0].Path != "/v1/messages" || op.Paths[0].Params["username"] != "admin" {
		t.Fatalf("operation policy path/params not preserved: %+v", op.Paths)
	}

	// The provider auth carries type/header but never the secret value.
	if payload.Provider.Auth == nil || payload.Provider.Auth.Value != "" {
		t.Fatalf("expected provider auth without secret value, got %+v", payload.Provider.Auth)
	}
}

func TestBuildLLMProxyPayload_UsesRuntimeDescriptionWhenSet(t *testing.T) {
	rt := newProxyRuntime()
	rt.Spec.Description = "custom proxy description"
	payload := buildLLMProxyPayload("claude-proxy2", newProxyMetadata(), rt, "")
	if payload.Description != "custom proxy description" {
		t.Fatalf("expected runtime description, got %q", payload.Description)
	}
}

func TestBuildLLMProxyPayload_OmitsPoliciesWhenNone(t *testing.T) {
	var md aiWorkspaceMetadata
	md.Spec.DisplayName = "p"
	md.Spec.Version = "v1.0"
	payload := buildLLMProxyPayload("p", md, aiWorkspaceRuntime{}, "")
	if payload.Security != nil {
		t.Fatalf("expected no security block, got %+v", payload.Security)
	}
	if payload.GlobalPolicies != nil || payload.OperationPolicies != nil {
		t.Fatalf("expected no policies, got global=%+v operation=%+v", payload.GlobalPolicies, payload.OperationPolicies)
	}
}

func TestModelProvidersForTemplate_KnownTemplate(t *testing.T) {
	got := modelProvidersForTemplate("openai")
	if len(got) != 1 {
		t.Fatalf("expected 1 model provider, got %d", len(got))
	}
	provider := got[0]
	if provider.ID != "openai" || provider.DisplayName != "openai" {
		t.Fatalf("expected provider id/displayName %q, got id=%q displayName=%q", "openai", provider.ID, provider.DisplayName)
	}

	wantModels := []string{"gpt-4o-mini", "gpt-4.1-mini", "o4-mini"}
	if len(provider.Models) != len(wantModels) {
		t.Fatalf("expected %d models, got %d", len(wantModels), len(provider.Models))
	}
	for i, want := range wantModels {
		if provider.Models[i].ID != want || provider.Models[i].DisplayName != want {
			t.Fatalf("model[%d]: expected id/displayName %q, got id=%q displayName=%q", i, want, provider.Models[i].ID, provider.Models[i].DisplayName)
		}
	}
}

func TestModelProvidersForTemplate_TrimsAndAllTemplates(t *testing.T) {
	// Every documented template maps to a non-empty model provider, and the
	// template is trimmed before lookup.
	for _, template := range []string{"meta", "openai", "anthropic", "google-vertex", "aws-bedrock", "mistralai"} {
		if got := modelProvidersForTemplate("  " + template + "  "); len(got) != 1 || len(got[0].Models) == 0 {
			t.Fatalf("template %q: expected one provider with models, got %#v", template, got)
		}
	}
}

func TestModelProvidersForTemplate_UnknownTemplateOmitted(t *testing.T) {
	if got := modelProvidersForTemplate("custom-template"); got != nil {
		t.Fatalf("expected nil for unknown template, got %#v", got)
	}
	if got := modelProvidersForTemplate(""); got != nil {
		t.Fatalf("expected nil for empty template, got %#v", got)
	}
}

func TestBuildLLMProviderPayload_IncludesModelProviders(t *testing.T) {
	var metadata aiWorkspaceMetadata
	metadata.Metadata.Name = "wso2-claude-provider"
	metadata.Spec.Version = "v1.0"

	var runtime aiWorkspaceRuntime
	runtime.Spec.Template = "anthropic"

	payload := buildLLMProviderPayload("wso2-claude-provider", metadata, runtime, "")
	if len(payload.ModelProviders) != 1 {
		t.Fatalf("expected modelProviders populated for template %q, got %#v", runtime.Spec.Template, payload.ModelProviders)
	}
	if payload.ModelProviders[0].ID != "anthropic" {
		t.Fatalf("expected model provider id %q, got %q", "anthropic", payload.ModelProviders[0].ID)
	}
}

func TestBuildLLMProviderPayload_OmitsModelProvidersForUnknownTemplate(t *testing.T) {
	var metadata aiWorkspaceMetadata
	metadata.Spec.Version = "v1.0"

	var runtime aiWorkspaceRuntime
	runtime.Spec.Template = "my-custom-template"

	payload := buildLLMProviderPayload("p", metadata, runtime, "")
	if payload.ModelProviders != nil {
		t.Fatalf("expected no modelProviders for unknown template, got %#v", payload.ModelProviders)
	}
}
