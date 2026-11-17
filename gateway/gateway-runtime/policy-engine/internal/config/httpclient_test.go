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
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
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
	if cfg.DestinationGuard.Enabled {
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

func TestBuildHTTPClientConfigGuardEnabledRequiresValidPreset(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.DestinationGuard.Enabled = true
	hc.DestinationGuard.Preset = "not-a-real-preset"

	if _, err := BuildHTTPClientConfig(hc); err == nil {
		t.Fatal("expected error for unrecognized destination_guard.preset, got nil")
	}

	hc.DestinationGuard.Preset = "public_only"
	hc.DestinationGuard.MaxRedirects = 3
	hc.DestinationGuard.AllowedSchemes = []string{"https"}
	cfg, err := BuildHTTPClientConfig(hc)
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error with valid preset: %v", err)
	}
	if !cfg.DestinationGuard.Enabled {
		t.Fatal("expected SSRF.Enabled to be true")
	}
	if cfg.DestinationGuard.Policy.BlockLoopback != netguard.PublicOnly().BlockLoopback {
		t.Errorf("expected DestinationGuard.Policy to be derived from the public_only preset, got %+v", cfg.DestinationGuard.Policy)
	}
	if cfg.DestinationGuard.Policy.AllowedSchemes[0] != "https" || cfg.DestinationGuard.MaxRedirects != 3 {
		t.Errorf("expected destination guard policy/redirects carried through, got %+v maxRedirects=%d", cfg.DestinationGuard.Policy, cfg.DestinationGuard.MaxRedirects)
	}
}

func TestBuildHTTPClientConfigRejectsUnknownProxyMode(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.Proxy.Mode = "socks5"

	if _, err := BuildHTTPClientConfig(hc); err == nil {
		t.Fatal("expected error for unrecognized proxy.mode, got nil")
	}
}

func TestBuildHTTPClientConfigRequiresEgressWhenProxyAndGuardConfigured(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.Proxy.Mode = "url"
	hc.Proxy.URL = "https://proxy.example.com:3128"
	hc.Proxy.Egress = ""
	hc.DestinationGuard.Enabled = true
	hc.DestinationGuard.Preset = "permit_private_block_metadata"

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

func TestBuildHTTPClientConfigSkipVerifyIsOffByDefault(t *testing.T) {
	cfg, err := BuildHTTPClientConfig(defaultHTTPClientConfigForTest())
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
	}
	if cfg.TLS.InsecureSkipVerify || cfg.TLS.InsecureSkipVerifyAcknowledged {
		t.Fatalf("expected skip-verify off by default, got skip=%v acknowledged=%v",
			cfg.TLS.InsecureSkipVerify, cfg.TLS.InsecureSkipVerifyAcknowledged)
	}
}

// One operator-facing flag is enough: it also supplies the acknowledgement httpclient.New
// requires, so the client builds.
func TestBuildHTTPClientConfigSkipVerifyNeedsOnlyOneFlag(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.TLS.InsecureSkipVerify = true

	cfg, err := BuildHTTPClientConfig(hc)
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
	}
	if !cfg.TLS.InsecureSkipVerify || !cfg.TLS.InsecureSkipVerifyAcknowledged {
		t.Fatalf("expected skip and acknowledgement set from the one flag, got skip=%v acknowledged=%v",
			cfg.TLS.InsecureSkipVerify, cfg.TLS.InsecureSkipVerifyAcknowledged)
	}
	if _, err := httpclient.New(cfg); err != nil {
		t.Fatalf("expected httpclient.New to build with skip-verify enabled, got %v", err)
	}
}

// Against a server with a certificate no root trusts, the default client must refuse the
// connection and the skip-verify client must complete it.
func TestSharedHTTPClientSkipVerifyAgainstSelfSignedServer(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	build := func(skip bool) *http.Client {
		hc := defaultHTTPClientConfigForTest()
		hc.TLS.InsecureSkipVerify = skip
		cfg, err := BuildHTTPClientConfig(hc)
		if err != nil {
			t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
		}
		client, err := httpclient.New(cfg)
		if err != nil {
			t.Fatalf("httpclient.New returned error: %v", err)
		}
		return client
	}

	if resp, err := build(false).Get(server.URL); err == nil {
		resp.Body.Close()
		t.Fatal("expected the default client to reject a self-signed certificate")
	}
	resp, err := build(true).Get(server.URL)
	if err != nil {
		t.Fatalf("expected the skip-verify client to connect, got %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestBuildHTTPClientConfigSkipHostnameVerifyIsOffByDefault(t *testing.T) {
	cfg, err := BuildHTTPClientConfig(defaultHTTPClientConfigForTest())
	if err != nil {
		t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
	}
	if cfg.TLS.InsecureSkipHostnameVerify {
		t.Fatal("expected hostname-only skip off by default")
	}
}

func TestBuildHTTPClientConfigRejectsBothSkipVerifyOptions(t *testing.T) {
	hc := defaultHTTPClientConfigForTest()
	hc.TLS.InsecureSkipVerify = true
	hc.TLS.InsecureSkipHostnameVerify = true
	if _, err := BuildHTTPClientConfig(hc); err == nil {
		t.Fatal("expected setting both skip-verify options to be rejected")
	}
}

// A certificate the client trusts but that names a different host is accepted by the
// hostname-only option and refused by default. The certificate chain is still checked, so a
// certificate nothing trusts is refused even with the option on.
func TestSharedHTTPClientSkipHostnameVerify(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// httptest's certificate names 127.0.0.1 and example.com, so dialing "localhost" is a
	// name mismatch against an otherwise trusted certificate.
	target := "https://localhost:" + server.URL[strings.LastIndex(server.URL, ":")+1:]

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())

	build := func(trusted, skipHostname bool) *http.Client {
		hc := defaultHTTPClientConfigForTest()
		hc.TLS.InsecureSkipHostnameVerify = skipHostname
		cfg, err := BuildHTTPClientConfig(hc)
		if err != nil {
			t.Fatalf("BuildHTTPClientConfig returned error: %v", err)
		}
		if trusted {
			cfg.TLS.RootCAs = roots
		}
		client, err := httpclient.New(cfg)
		if err != nil {
			t.Fatalf("httpclient.New returned error: %v", err)
		}
		return client
	}

	if resp, err := build(true, false).Get(target); err == nil {
		resp.Body.Close()
		t.Fatal("expected the default client to refuse a certificate that does not name the host")
	}
	resp, err := build(true, true).Get(target)
	if err != nil {
		t.Fatalf("expected the hostname-only option to accept a trusted certificate for another name, got %v", err)
	}
	resp.Body.Close()
	if resp, err := build(false, true).Get(target); err == nil {
		resp.Body.Close()
		t.Fatal("expected the hostname-only option to still refuse an untrusted certificate")
	}
}
