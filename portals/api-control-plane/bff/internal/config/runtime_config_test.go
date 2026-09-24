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

import "testing"

func TestLoadPolicyHubRuntimeConfig(t *testing.T) {
	for _, baseURL := range []string{"", "https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-dev.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0/"} {
		t.Run(baseURL, func(t *testing.T) {
			t.Setenv("APIP_ACP_POLICY_HUB_BASE_URL", baseURL)
			cfg, err := Load("../../../configs/config.toml")
			if err != nil {
				t.Fatal(err)
			}
			got, present := cfg.RuntimeConfig["POLICY_HUB_BASE_URL"]
			if baseURL == "" {
				if present {
					t.Fatal("unconfigured Policy Hub should not emit a runtime override")
				}
			} else if got != "https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-dev.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0" {
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
