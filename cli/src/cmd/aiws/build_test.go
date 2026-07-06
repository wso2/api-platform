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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func writeTestEnvFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}
	return path
}

func TestResolveEnvPlaceholders_AllFormsFromProjectDotEnv(t *testing.T) {
	root := t.TempDir()
	writeTestEnvFile(t, root, ".env", "ENV_CLI_A=alpha\nENV_CLI_B=beta\nENV_CLI_C=gamma\n")

	body := []byte(`{"a":"${ENV_CLI_A}","b":"$ENV_CLI_B","c":"ENV_CLI_C"}`)
	got, err := resolveEnvPlaceholders(body, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"a":"alpha","b":"beta","c":"gamma"}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestResolveEnvPlaceholders_ExplicitFileWinsProcessEnvFillsGaps(t *testing.T) {
	root := t.TempDir()
	// A .env in the project root must be ignored when --env-file is given.
	writeTestEnvFile(t, root, ".env", "ENV_CLI_KEY=from-dotenv\n")
	explicit := writeTestEnvFile(t, root, "custom.env", "ENV_CLI_KEY=from-file\n")

	t.Setenv("ENV_CLI_KEY", "from-process")
	t.Setenv("ENV_CLI_ONLY_PROCESS", "process-value")

	body := []byte(`{"k":"${ENV_CLI_KEY}","p":"${ENV_CLI_ONLY_PROCESS}"}`)
	got, err := resolveEnvPlaceholders(body, root, explicit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"k":"from-file","p":"process-value"}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestResolveEnvPlaceholders_MissingVariablesError(t *testing.T) {
	root := t.TempDir()
	body := []byte(`{"a":"${ENV_CLI_MISSING_ONE}","b":"$ENV_CLI_MISSING_TWO"}`)
	_, err := resolveEnvPlaceholders(body, root, "")
	if err == nil {
		t.Fatal("expected an error for unresolved placeholders")
	}
	for _, name := range []string{"ENV_CLI_MISSING_ONE", "ENV_CLI_MISSING_TWO"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error should name %s: %v", name, err)
		}
	}
}

func TestResolveEnvPlaceholders_NoPlaceholdersUnchangedAndNoEnvNeeded(t *testing.T) {
	root := t.TempDir()
	body := []byte(`{"a":"plain"}`)
	got, err := resolveEnvPlaceholders(body, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("body changed: %s", got)
	}
}

func TestResolveEnvPlaceholders_JSONEscapesValues(t *testing.T) {
	root := t.TempDir()
	writeTestEnvFile(t, root, ".env", `ENV_CLI_SECRET=va"l\ue`+"\n")

	body := []byte(`{"s":"${ENV_CLI_SECRET}"}`)
	got, err := resolveEnvPlaceholders(body, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("resolved payload is not valid JSON: %v (%s)", err, got)
	}
	if decoded["s"] != `va"l\ue` {
		t.Fatalf("got %q, want %q", decoded["s"], `va"l\ue`)
	}
}

func TestResolveEnvPlaceholders_ExplicitEnvFileMissingIsError(t *testing.T) {
	root := t.TempDir()
	body := []byte(`{"a":"${ENV_CLI_A}"}`)
	if _, err := resolveEnvPlaceholders(body, root, filepath.Join(root, "nope.env")); err == nil {
		t.Fatal("expected an error for a missing --env-file")
	}
}

func TestParseEnvFile_CommentsExportAndQuotes(t *testing.T) {
	root := t.TempDir()
	path := writeTestEnvFile(t, root, "vals.env", strings.Join([]string{
		"# comment",
		"",
		"export ENV_CLI_EXPORTED=one",
		`ENV_CLI_DOUBLE="two words"`,
		"ENV_CLI_SINGLE='three'",
		"ENV_CLI_EQ=a=b",
		"not-a-pair",
	}, "\n"))

	values, err := parseEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{
		"ENV_CLI_EXPORTED": "one",
		"ENV_CLI_DOUBLE":   "two words",
		"ENV_CLI_SINGLE":   "three",
		"ENV_CLI_EQ":       "a=b",
	}
	for k, v := range want {
		if values[k] != v {
			t.Fatalf("%s: got %q, want %q", k, values[k], v)
		}
	}
	if len(values) != len(want) {
		t.Fatalf("unexpected extra entries: %#v", values)
	}
}

func TestResolveEnvPlaceholders_BareFormRequiresWordBoundary(t *testing.T) {
	root := t.TempDir()
	writeTestEnvFile(t, root, ".env", "ENV_CLI_FOO=resolved\n")

	// A bare placeholder at a boundary resolves; the same token embedded in a
	// larger identifier (MY_ENV_CLI_FOO) must be left untouched.
	body := []byte(`{"a":"ENV_CLI_FOO","b":"MY_ENV_CLI_FOO"}`)
	got, err := resolveEnvPlaceholders(body, root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"a":"resolved","b":"MY_ENV_CLI_FOO"}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
