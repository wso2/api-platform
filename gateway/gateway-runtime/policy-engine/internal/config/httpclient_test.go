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

package config

import (
	"testing"
	"time"

	"github.com/wso2/go-httpkit/netguard"
)

func defaultHTTPClientConfigForTest() HTTPClientConfig {
	return defaultConfig().PolicyEngine.HTTPClient
}

func TestBuildHTTPClientConfigDefaultsSSRFDisabled(t *testing.T) {
	cfg, err := BuildHTTPClientConfig(defaultHTTPClientConfigForTest())
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
	}

	// Unlike the WebSub delivery/verifier clients (which always dial a
	// tenant-supplied CallbackURL), this shared client is used by arbitrary
	// policies — some targeting a fixed operator-configured backend, some
	// potentially targeting a tenant/request-derived URL — so SSRF guarding is
	// an explicit per-deployment opt-in rather than always-on.
	if cfg.SSRF.Enabled {
		t.Fatal("expected SSRF.Enabled to default to false, got true")
	}
	if cfg.Proxy.Mode != "" && cfg.Proxy.Mode != "none" {
		t.Fatalf("expected no proxy mode by default, got %q", cfg.Proxy.Mode)
	}
}

func TestBuildHTTPClientConfigCarriesPoolingTimeoutsAndTLS(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.Pooling.MaxIdleConns = 42
	hc.Timeouts.Dial = 5 * time.Second
	hc.TLS.MinVersion = "TLS1_3"

	cfg, err := BuildHTTPClientConfig(hc)
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
	}

	if cfg.Pooling.MaxIdleConns != 42 {
		t.Errorf("expected Pooling.MaxIdleConns=42, got %d", cfg.Pooling.MaxIdleConns)
	}
	if cfg.Timeouts.Dial != 5*time.Second {
		t.Errorf("expected Timeouts.Dial=5s, got %v", cfg.Timeouts.Dial)
	}
	if cfg.TLS.MinVersion != "TLS1_3" {
		t.Errorf("expected TLS.MinVersion=TLS1_3, got %q", cfg.TLS.MinVersion)
	}
}

func TestBuildHTTPClientConfigSSRFEnabledRequiresValidPreset(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.SSRF.Enabled = true
	hc.SSRF.Preset = "not-a-real-preset"

	if _, err := BuildHTTPClientConfig(hc); err == nil {
		t.Fatal("expected error for unrecognized ssrf.preset, got nil")
	}

	hc.SSRF.Preset = "public_only"
	hc.SSRF.MaxRedirects = 3
	hc.SSRF.AllowedSchemes = []string{"https"}
	cfg, err := BuildHTTPClientConfig(hc)
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error with valid preset: %v", err)
	}
	if !cfg.SSRF.Enabled {
		t.Fatal("expected SSRF.Enabled to be true")
	}
	if cfg.SSRF.Policy.BlockLoopback != netguard.PublicOnly().BlockLoopback {
		t.Errorf("expected SSRF.Policy to be derived from the public_only preset, got %+v", cfg.SSRF.Policy)
	}
	if cfg.SSRF.Policy.AllowedSchemes[0] != "https" || cfg.SSRF.MaxRedirects != 3 {
		t.Errorf("expected SSRF policy/redirects carried through, got %+v maxRedirects=%d", cfg.SSRF.Policy, cfg.SSRF.MaxRedirects)
	}
}

func TestBuildHTTPClientConfigRejectsUnknownProxyMode(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.Proxy.Mode = "socks5"

	if _, err := BuildHTTPClientConfig(hc); err == nil {
		t.Fatal("expected error for unrecognized proxy.mode, got nil")
	}
}

func TestBuildHTTPClientConfigRequiresEgressWhenProxyAndSSRFConfigured(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.Proxy.Mode = "url"
	hc.Proxy.URL = "https://proxy.example.com:3128"
	hc.Proxy.Egress = ""
	hc.SSRF.Enabled = true
	hc.SSRF.Preset = "permit_private_block_metadata"

	if _, err := BuildHTTPClientConfig(hc); err == nil {
		t.Fatal("expected error when proxy+SSRF are configured without an explicit egress policy, got nil")
	}

	hc.Proxy.Egress = "delegated"
	cfg, err := BuildHTTPClientConfig(hc)
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error with valid egress: %v", err)
	}
	if cfg.Proxy.Mode != "url" || cfg.Proxy.URL != hc.Proxy.URL {
		t.Errorf("expected proxy url mode carried through, got %+v", cfg.Proxy)
	}
}

func TestBuildHTTPClientConfigEnvironmentProxyMode(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.Proxy.Mode = "environment"

	cfg, err := BuildHTTPClientConfig(hc)
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
	}
	if cfg.Proxy.Mode != "environment" {
		t.Errorf("expected proxy mode environment, got %q", cfg.Proxy.Mode)
	}
}
