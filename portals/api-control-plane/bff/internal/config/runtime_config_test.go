/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsInvalidPolicyHubURL(t *testing.T) {
	for name, baseURL := range map[string]string{
		"missing slash":      strings.Replace(defaultPolicyHubBaseURL, "https://", "https:/", 1),
		"relative URL":       strings.TrimPrefix(defaultPolicyHubBaseURL, "https://"),
		"unsupported scheme": strings.Replace(defaultPolicyHubBaseURL, "https://", "ftp://", 1),
		"missing host":       "https:///api-platform/policy-hub-api/policy-hub-public/v1.0",
		"invalid escape":     defaultPolicyHubBaseURL + "%",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("APIP_ACP_POLICY_HUB_BASE_URL", baseURL)
			cfg, err := Load("../../../configs/config.toml")
			if err == nil || !strings.Contains(err.Error(), "[policy_hub] base_url") {
				t.Fatalf("Load error = %v, want Policy Hub URL validation error", err)
			}
			if cfg != nil {
				t.Fatal("Load must not return runtime configuration for an invalid URL")
			}
		})
	}
}

func TestLoadGatewayControlPlaneHostRuntimeConfig(t *testing.T) {
	for name, tc := range map[string]struct{ env, want string }{
		"host with path, as WSO2 Cloud routes it": {
			env:  "dev-wso2cloud.gateway.example.com/apip-platform-api-apip-platform-api-internal-endpoint",
			want: "dev-wso2cloud.gateway.example.com/apip-platform-api-apip-platform-api-internal-endpoint",
		},
		"host and port":               {env: "cp.example.com:9243", want: "cp.example.com:9243"},
		"highest port":                {env: "cp.example.com:65535", want: "cp.example.com:65535"},
		"surrounding space and slash": {env: "  cp.example.com/ ", want: "cp.example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("APIP_ACP_GATEWAY_CONTROLPLANE_HOST", tc.env)
			cfg, err := Load("../../../configs/config.toml")
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.RuntimeConfig["GATEWAY_CONTROL_PLANE_HOST"]; got != tc.want {
				t.Fatalf("GATEWAY_CONTROL_PLANE_HOST = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoadOmitsGatewayControlPlaneHostWhenUnset(t *testing.T) {
	t.Setenv("APIP_ACP_GATEWAY_CONTROLPLANE_HOST", "")
	cfg, err := Load("../../../configs/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	// Absent, not empty: the SPA reads a missing key as "not configured" and
	// keeps a placeholder in the cloud setup commands.
	if _, ok := cfg.RuntimeConfig["GATEWAY_CONTROL_PLANE_HOST"]; ok {
		t.Fatal("GATEWAY_CONTROL_PLANE_HOST present for an unset host, want it omitted")
	}
}

func TestLoadRejectsUnsafeGatewayControlPlaneHost(t *testing.T) {
	// The value is pasted into a user's shell, so anything beyond
	// host[:port][/path] must stop startup rather than reach a terminal.
	for name, host := range map[string]string{
		"scheme":            "https://cp.example.com",
		"command separator": "cp.example.com; rm -rf ~",
		"quote":             `cp.example.com"`,
		"inner space":       "cp.example.com /x",
		"substitution":      "$(whoami).example.com",
		"heredoc break":     "cp.example.com\nENVFILE",
		"port zero":         "cp.example.com:0",
		"port out of range": "cp.example.com:99999",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("APIP_ACP_GATEWAY_CONTROLPLANE_HOST", host)
			cfg, err := Load("../../../configs/config.toml")
			if err == nil || !strings.Contains(err.Error(), "[gateway] controlplane_host") {
				t.Fatalf("Load error = %v, want controlplane_host validation error", err)
			}
			if cfg != nil {
				t.Fatal("Load must not return configuration for an invalid control plane host")
			}
		})
	}
}

func TestLoadPolicyHubRuntimeConfig(t *testing.T) {
	for _, baseURL := range []string{"", "https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-dev.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0/"} {
		t.Run(baseURL, func(t *testing.T) {
			t.Setenv("APIP_ACP_POLICY_HUB_BASE_URL", baseURL)
			cfg, err := Load("../../../configs/config.toml")
			if err != nil {
				t.Fatal(err)
			}
			got := cfg.RuntimeConfig["POLICY_HUB_BASE_URL"]
			if got != defaultPolicyHubBaseURL {
				t.Fatalf("Policy Hub runtime URL = %q, want normalized configured URL", got)
			}
		})
	}
}

func TestBuildRuntimeConfig_KeysMatchFrontendVocabulary(t *testing.T) {
	cfg := &Config{
		Auth:         AuthConfig{Mode: "oidc"},
		ControlPlane: ControlPlaneConfig{ProxyPrefix: "/proxy"},
	}
	out := buildRuntimeConfig(cfg)

	if out["authMode"] != "oidc" {
		t.Errorf(`out["authMode"] = %q, want "oidc"`, out["authMode"])
	}
	if out["platformApiBaseUrl"] != "/proxy" {
		t.Errorf(`out["platformApiBaseUrl"] = %q, want "/proxy"`, out["platformApiBaseUrl"])
	}
	if _, present := out["billingProxyEnabled"]; present {
		t.Error(`out["billingProxyEnabled"] should be absent when no "billing" upstream is configured`)
	}
}

func TestBuildRuntimeConfig_BillingProxyEnabledWhenUpstreamConfigured(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{Mode: "basic"},
		ControlPlane: ControlPlaneConfig{
			ProxyPrefix: "/proxy",
			Upstreams:   []UpstreamConfig{{Name: "billing", URL: "https://billing.example.com"}},
		},
	}
	out := buildRuntimeConfig(cfg)

	if out["billingProxyEnabled"] != "true" {
		t.Errorf(`out["billingProxyEnabled"] = %q, want "true"`, out["billingProxyEnabled"])
	}
}

func TestBuildRuntimeConfig_CloudProxyEnabledWhenUpstreamConfigured(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{Mode: "basic"},
		ControlPlane: ControlPlaneConfig{
			ProxyPrefix: "/proxy",
			Upstreams:   []UpstreamConfig{{Name: "cloud", URL: "https://platform-api.example.com"}},
		},
	}
	out := buildRuntimeConfig(cfg)

	if out["cloudProxyEnabled"] != "true" {
		t.Errorf(`out["cloudProxyEnabled"] = %q, want "true"`, out["cloudProxyEnabled"])
	}
	if _, present := out["billingProxyEnabled"]; present {
		t.Error(`out["billingProxyEnabled"] should be absent when only "cloud" upstream is configured`)
	}
}

func TestBuildRuntimeConfig_MoesifAppUrlWhenConfigured(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{Mode: "basic"},
		ControlPlane: ControlPlaneConfig{
			ProxyPrefix:  "/proxy",
			MoesifAppURL: "https://www.moesif.com",
		},
	}
	out := buildRuntimeConfig(cfg)

	if out["moesifAppUrl"] != "https://www.moesif.com" {
		t.Errorf(`out["moesifAppUrl"] = %q, want "https://www.moesif.com"`, out["moesifAppUrl"])
	}
}

func TestBuildRuntimeConfig_NeverEmitsClientSecretOrAuthority(t *testing.T) {
	// The BFF performs the whole OIDC handshake server-side — the SPA must
	// never receive the client identity or secret.
	cfg := &Config{
		Auth: AuthConfig{
			Mode: "oidc",
			OIDC: OIDCConfig{
				Authority:    "https://idp.example.com",
				ClientID:     "should-not-leak",
				ClientSecret: "super-secret",
			},
		},
		ControlPlane: ControlPlaneConfig{ProxyPrefix: "/proxy"},
	}
	out := buildRuntimeConfig(cfg)

	for key, value := range out {
		if value == cfg.Auth.OIDC.ClientID || value == cfg.Auth.OIDC.ClientSecret || value == cfg.Auth.OIDC.Authority {
			t.Errorf("runtime config key %q leaked an OIDC client secret/identity value", key)
		}
	}
}

// loadWithSanitize loads config.toml plus an overlay with the given
// [sanitize_html_docs] body.
func loadWithSanitize(t *testing.T, body string) *Config {
	t.Helper()
	overlay := filepath.Join(t.TempDir(), "overlay.toml")
	if err := os.WriteFile(overlay, []byte("[api_control_plane.sanitize_html_docs]\n"+body+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("../../../configs/config.toml", overlay)
	if err != nil {
		t.Fatalf("Load error = %v, want sanitize settings never to fail startup", err)
	}
	return cfg
}

func TestBuildRuntimeConfig_SanitizeHTMLDocsAbsentByDefault(t *testing.T) {
	cfg, err := Load("../../../configs/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"sanitizeHtmlDocsEnabled",
		"sanitizeHtmlDocsAdditionalAllowedTags",
		"sanitizeHtmlDocsAdditionalAllowedAttributes",
	} {
		if v, present := cfg.RuntimeConfig[key]; present {
			t.Errorf("%s = %q, want absent", key, v)
		}
	}
}

func TestBuildRuntimeConfig_SanitizeHTMLDocsPassThrough(t *testing.T) {
	for name, tc := range map[string]struct{ body, key, want string }{
		"bool false":       {`enabled = false`, "sanitizeHtmlDocsEnabled", "false"},
		"string FALSE":     {`enabled = 'FALSE'`, "sanitizeHtmlDocsEnabled", "FALSE"},
		"string no":        {`enabled = 'no'`, "sanitizeHtmlDocsEnabled", "no"},
		"tags TOML array":  {`additional_allowed_tags = ["video", "iframe"]`, "sanitizeHtmlDocsAdditionalAllowedTags", `["video","iframe"]`},
		"tags JSON string": {`additional_allowed_tags = '["video"]'`, "sanitizeHtmlDocsAdditionalAllowedTags", `["video"]`},
		"tags comma list":  {`additional_allowed_tags = 'video,iframe'`, "sanitizeHtmlDocsAdditionalAllowedTags", `video,iframe`},
		"attrs JSON string": {`additional_allowed_attributes = '{"iframe":["src"]}'`,
			"sanitizeHtmlDocsAdditionalAllowedAttributes", `{"iframe":["src"]}`},
		"attrs TOML table": {"[api_control_plane.sanitize_html_docs.additional_allowed_attributes]\niframe = [\"src\"]\ndiv = \"hidden\"",
			"sanitizeHtmlDocsAdditionalAllowedAttributes", `{"div":"hidden","iframe":["src"]}`},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := loadWithSanitize(t, tc.body)
			if got := cfg.RuntimeConfig[tc.key]; got != tc.want {
				t.Errorf("%s = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestBuildRuntimeConfig_SanitizeHTMLDocsBlankEnvIsAbsent(t *testing.T) {
	t.Setenv("APIP_ACP_SANITIZE_HTML_DOCS_ENABLED", "")
	cfg := loadWithSanitize(t, `enabled = '{{ env "APIP_ACP_SANITIZE_HTML_DOCS_ENABLED" "" }}'`)
	if v, present := cfg.RuntimeConfig["sanitizeHtmlDocsEnabled"]; present {
		t.Errorf("sanitizeHtmlDocsEnabled = %q, want absent", v)
	}
}

func loadWithDisableFeatures(t *testing.T, line string) *Config {
	t.Helper()
	overlay := filepath.Join(t.TempDir(), "overlay.toml")
	if err := os.WriteFile(overlay, []byte("[api_control_plane]\n"+line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("../../../configs/config.toml", overlay)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestBuildRuntimeConfig_DisableFeaturesAbsentByDefault(t *testing.T) {
	cfg, err := Load("../../../configs/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	if v, present := cfg.RuntimeConfig["disableFeatures"]; present {
		t.Errorf("disableFeatures = %q, want absent so every feature stays on", v)
	}
}

func TestBuildRuntimeConfig_DisableFeaturesPassThrough(t *testing.T) {
	for name, tc := range map[string]struct{ line, want string }{
		"TOML array":  {`disable_features = ["metrics", "meters"]`, `["metrics","meters"]`},
		"JSON string": {`disable_features = '["metrics"]'`, `["metrics"]`},
		"comma list":  {`disable_features = 'metrics,meters'`, `metrics,meters`},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := loadWithDisableFeatures(t, tc.line)
			if got := cfg.RuntimeConfig["disableFeatures"]; got != tc.want {
				t.Errorf("disableFeatures = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildRuntimeConfig_DisableFeaturesBlankEnvIsAbsent(t *testing.T) {
	t.Setenv("APIP_ACP_DISABLE_FEATURES", "")
	cfg, err := Load("../../../configs/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	if v, present := cfg.RuntimeConfig["disableFeatures"]; present {
		t.Errorf("disableFeatures = %q, want absent", v)
	}
}

func TestBuildRuntimeConfig_DisableFeaturesFromEnv(t *testing.T) {
	t.Setenv("APIP_ACP_DISABLE_FEATURES", "metrics,logs")
	cfg, err := Load("../../../configs/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.RuntimeConfig["disableFeatures"]; got != "metrics,logs" {
		t.Errorf("disableFeatures = %q, want metrics,logs", got)
	}
}
