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

import (
	"encoding/json"
	"testing"
)

// Round-trip contract tests: every field the gateway-controller requires must
// survive the runtime.yaml -> payload translation. A field dropped here passes
// `ap ai-workspace build` and only fails at deploy time, when the controller
// validates the artifact it was handed.

// proxyRuntimeWithProviderAuth returns a proxy runtime whose provider auth
// carries the supplied credential value.
func proxyRuntimeWithProviderAuth(auth *runtimeProviderAuth) aiWorkspaceRuntime {
	rt := newProxyRuntime()
	rt.Spec.Provider = runtimeProvider{ID: "wso2-claude-provider", Auth: auth}
	return rt
}

// providerRuntime returns a minimally valid LlmProvider runtime.
func providerRuntime() aiWorkspaceRuntime {
	var rt aiWorkspaceRuntime
	rt.Spec.Context = "/mistral"
	rt.Spec.Template = "mistralai"
	rt.Spec.Upstream = &runtimeUpstream{
		URL:  "https://api.mistral.ai",
		Auth: &runtimeProviderAuth{Type: "api-key", Header: "Authorization", Value: "Bearer vendor-key"},
	}
	rt.Spec.AccessControl = &runtimeAccessControl{
		Mode:       "deny_all",
		Exceptions: []runtimeRouteException{{Path: "/v1/chat/completions", Methods: []string{"POST"}}},
	}
	return rt
}

func providerMetadata() aiWorkspaceMetadata {
	var md aiWorkspaceMetadata
	md.Metadata.Name = "mistral-provider"
	md.Spec.DisplayName = "Mistral Provider"
	md.Spec.Version = "v1.0"
	return md
}

// The regression guard for the reported defect: the proxy's provider auth value
// is the credential for the internal loopback hop into the provider, and the
// controller rejects the artifact without it.
func TestProxyPayload_ForwardsProviderAuthValue(t *testing.T) {
	rt := proxyRuntimeWithProviderAuth(&runtimeProviderAuth{
		Type: "api-key", Header: "X-API-Key", Value: "b6aef2b11e5ab5fda03692485072e1d3",
	})

	payload := buildLLMProxyPayload("mistral-proxy", newProxyMetadata(), rt, "")

	if payload.Provider.Auth == nil {
		t.Fatal("expected provider auth block in payload, got nil")
	}
	if got := payload.Provider.Auth.Value; got != "b6aef2b11e5ab5fda03692485072e1d3" {
		t.Fatalf("provider auth value not forwarded: got %q", got)
	}
	if payload.Provider.Auth.Type != "api-key" || payload.Provider.Auth.Header != "X-API-Key" {
		t.Fatalf("provider auth type/header not preserved: %+v", payload.Provider.Auth)
	}
}

// The value must survive JSON marshalling too — an `omitempty` on a populated
// field is still a drop by the time the request leaves the CLI.
func TestProxyPayload_ProviderAuthValueSurvivesMarshalling(t *testing.T) {
	rt := proxyRuntimeWithProviderAuth(&runtimeProviderAuth{
		Type: "api-key", Header: "X-API-Key", Value: "loopback-key",
	})

	raw, err := json.Marshal(buildLLMProxyPayload("mistral-proxy", newProxyMetadata(), rt, ""))
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var decoded struct {
		Provider struct {
			Auth *struct {
				Type   string `json:"type"`
				Header string `json:"header"`
				Value  string `json:"value"`
			} `json:"auth"`
		} `json:"provider"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if decoded.Provider.Auth == nil || decoded.Provider.Auth.Value != "loopback-key" {
		t.Fatalf("provider auth value missing from marshalled payload: %s", raw)
	}
}

// Secret references and ENV_CLI_* placeholders reach this field as opaque
// strings; neither may be rewritten or stripped during payload construction.
// The ENV_CLI_ form is resolved later, by resolveEnvPlaceholders at apply time.
func TestProxyPayload_ForwardsCredentialReferencesVerbatim(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"secret reference", `{{ secret "d4f1c0a2-9b3e-4a7c-8f21-6e5b0c9a7d31" }}`},
		{"env placeholder braced", "${ENV_CLI_MISTRAL_PROVIDER_KEY}"},
		{"env placeholder bare", "ENV_CLI_MISTRAL_PROVIDER_KEY"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rt := proxyRuntimeWithProviderAuth(&runtimeProviderAuth{
				Type: "api-key", Header: "X-API-Key", Value: tc.value,
			})

			payload := buildLLMProxyPayload("mistral-proxy", newProxyMetadata(), rt, "")

			if payload.Provider.Auth == nil || payload.Provider.Auth.Value != tc.value {
				t.Fatalf("expected %q forwarded verbatim, got %+v", tc.value, payload.Provider.Auth)
			}
		})
	}
}

// The controller only validates provider auth when the block is present
// (llm_validator.go:595), so an omitted auth must stay omitted rather than
// becoming an empty block that then fails the api-key required-field checks.
func TestProxyPayload_OmitsAuthBlockWhenRuntimeOmitsIt(t *testing.T) {
	rt := proxyRuntimeWithProviderAuth(nil)

	payload := buildLLMProxyPayload("mistral-proxy", newProxyMetadata(), rt, "")

	if payload.Provider.Auth != nil {
		t.Fatalf("expected no auth block when runtime omits it, got %+v", payload.Provider.Auth)
	}
	if payload.Provider.ID != "wso2-claude-provider" {
		t.Fatalf("provider id not preserved: %q", payload.Provider.ID)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	
	provider, ok := decoded["provider"].(map[string]interface{})
	if !ok {
		t.Fatalf("provider key should be present in the marshalled payload: %s", raw)
	}
	if _, present := provider["auth"]; present {
		t.Fatalf("auth key should be absent from the marshalled payload: %s", raw)
	}
}

// Same credential class on the provider side: upstream.auth is the vendor
// credential, subject to the identical api-key required-field checks.
func TestProviderPayload_ForwardsUpstreamAuthValue(t *testing.T) {
	payload := buildLLMProviderPayload("mistral-provider", providerMetadata(), providerRuntime(), "")

	if payload.Upstream == nil {
		t.Fatal("expected upstream block in payload, got nil")
	}
	if payload.Upstream.Main.URL != "https://api.mistral.ai" {
		t.Fatalf("upstream url not forwarded: %q", payload.Upstream.Main.URL)
	}
	if payload.Upstream.Main.Auth == nil {
		t.Fatal("expected upstream auth block in payload, got nil")
	}
	if got := payload.Upstream.Main.Auth.Value; got != "Bearer vendor-key" {
		t.Fatalf("upstream auth value not forwarded: got %q", got)
	}
}

// template and accessControl are required by both the controller and
// platform-api, and are exactly the fields `ap project init` omits from a
// scaffolded provider — so pin that they are carried through when present.
func TestProviderPayload_ForwardsTemplateAndAccessControl(t *testing.T) {
	payload := buildLLMProviderPayload("mistral-provider", providerMetadata(), providerRuntime(), "")

	if payload.Template != "mistralai" {
		t.Fatalf("template not forwarded: %q", payload.Template)
	}
	// modelProviders is derived from the template, so a dropped template would
	// silently empty this too.
	if len(payload.ModelProviders) != 1 || payload.ModelProviders[0].ID != "mistralai" {
		t.Fatalf("modelProviders not derived from template: %+v", payload.ModelProviders)
	}
	if payload.AccessControl == nil || payload.AccessControl.Mode != "deny_all" {
		t.Fatalf("accessControl not forwarded: %+v", payload.AccessControl)
	}
	if len(payload.AccessControl.Exceptions) != 1 ||
		payload.AccessControl.Exceptions[0].Path != "/v1/chat/completions" {
		t.Fatalf("accessControl exceptions not forwarded: %+v", payload.AccessControl.Exceptions)
	}
}
