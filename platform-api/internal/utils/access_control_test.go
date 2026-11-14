/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package utils

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// useAccessControl installs a user-URL client built from gc for the duration of the test,
// alongside TestMain's shared client, and removes it afterwards.
func useAccessControl(t *testing.T, gc netguard.GuardConfig) {
	t.Helper()
	policy, err := gc.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cfg := httpclient.DefaultConfig()
	cfg.Pooling.DisableKeepAlives = true
	cfg.Timeouts.MaxResponseBytes = -1
	cfg.DestinationGuard = httpclient.DestinationGuardConfig{Enabled: true, Policy: policy, MaxRedirects: gc.MaxRedirects}
	if err := InitUserURLClientFromConfig(cfg); err != nil {
		t.Fatalf("InitUserURLClientFromConfig: %v", err)
	}
	t.Cleanup(func() {
		sharedHTTPClientMu.Lock()
		userURLClient, userURLClientConfig = nil, nil
		sharedHTTPClientMu.Unlock()
	})
}

func loopbackBackend(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte("openapi: 3.0.0\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestAccessControl_AppliesOnlyToTenantURLs is the scope guarantee: with private networks
// denied, the client for tenant- and user-supplied URLs refuses a loopback backend, while
// the clients for destinations configured in config.toml (the IdP JWKS URL, the
// feature-usage collector) still reach it.
func TestAccessControl_AppliesOnlyToTenantURLs(t *testing.T) {
	srv := loopbackBackend(t)
	useAccessControl(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})

	tenant, err := NewUpstreamFetchClient(0)
	if err != nil {
		t.Fatalf("NewUpstreamFetchClient: %v", err)
	}
	if _, err := tenant.Get(srv.URL); !errors.Is(err, netguard.ErrDestinationNotAllowed) {
		t.Fatalf("tenant client to a loopback backend = %v, want ErrDestinationNotAllowed", err)
	}
	withTimeout, err := NewUpstreamClientWithTimeout(5 * time.Second)
	if err != nil {
		t.Fatalf("NewUpstreamClientWithTimeout: %v", err)
	}
	if _, err := withTimeout.Get(srv.URL); !errors.Is(err, netguard.ErrDestinationNotAllowed) {
		t.Fatalf("tenant client with timeout to a loopback backend = %v, want ErrDestinationNotAllowed", err)
	}

	configured, err := NewConfiguredDestinationClient()
	if err != nil {
		t.Fatalf("NewConfiguredDestinationClient: %v", err)
	}
	resp, err := configured.Get(srv.URL)
	if err != nil {
		t.Fatalf("config.toml destination client must still reach a private backend: %v", err)
	}
	resp.Body.Close() //nolint:errcheck

	operator, err := NewOperatorUpstreamClientWithTimeout(5 * time.Second)
	if err != nil {
		t.Fatalf("NewOperatorUpstreamClientWithTimeout: %v", err)
	}
	resp, err = operator.Get(srv.URL)
	if err != nil {
		t.Fatalf("operator client must still reach a private backend: %v", err)
	}
	resp.Body.Close() //nolint:errcheck
}

func TestCheckUserURL(t *testing.T) {
	ctx := context.Background()

	// Without access control nothing is checked here.
	if err := CheckUserURL(ctx, "https://anything.example/"); err != nil {
		t.Fatalf("CheckUserURL without access control = %v, want nil", err)
	}

	useAccessControl(t, netguard.GuardConfig{Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{`api\.example\.com`}})

	if err := CheckUserURL(ctx, "https://api.example.com/spec.yaml"); err != nil {
		t.Fatalf("listed host = %v, want nil", err)
	}
	for _, raw := range []string{
		"https://other.example.com/spec.yaml",
		"https://api.example.com.evil.com/spec.yaml",
		"ftp://api.example.com/spec.yaml",
	} {
		err := CheckUserURL(ctx, raw)
		if !apperror.DestinationNotAllowed.Is(err) {
			t.Errorf("CheckUserURL(%q) = %v, want DestinationNotAllowed", raw, err)
		}
	}
	if err := CheckUserURL(ctx, "://not a url"); err != nil {
		t.Errorf("a malformed URL is left to the caller's validation, got %v", err)
	}
}

func TestFetchDocumentFromURL_RefusedDestinationIsA400(t *testing.T) {
	srv := loopbackBackend(t)

	// Positive control: the same fetch succeeds while private networks are permitted.
	useAccessControl(t, netguard.GuardConfig{Enabled: true})
	if _, err := FetchDocumentFromURL(context.Background(), srv.URL, 0, "OpenAPI spec"); err != nil {
		t.Fatalf("fetch with private networks permitted: %v", err)
	}

	useAccessControl(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})
	_, err := FetchDocumentFromURL(context.Background(), srv.URL, 0, "OpenAPI spec")
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonDestinationNotAllowed || appErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("FetchDocumentFromURL = %v, want a 400 DESTINATION_NOT_ALLOWED", err)
	}
	if containsAny(appErr.Message, "127.0.0.1", "loopback", "private") {
		t.Fatalf("the client-facing message must not say why: %q", appErr.Message)
	}
}

func TestCheckURLReachability_RefusedDestination(t *testing.T) {
	srv := loopbackBackend(t)
	useAccessControl(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})
	if err := CheckURLReachability(srv.URL, time.Second); !apperror.DestinationNotAllowed.Is(err) {
		t.Fatalf("CheckURLReachability = %v, want DestinationNotAllowed", err)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}

// MCP discovery reports a refused destination as DESTINATION_NOT_ALLOWED, not as a
// generic discovery failure, however deep in the initialize/JSON-RPC calls it fails.
func TestFetchMCPServerInfo_RefusedDestination(t *testing.T) {
	srv := loopbackBackend(t)
	useAccessControl(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})

	_, err := FetchMCPServerInfo(srv.URL, "", "")
	if !apperror.DestinationNotAllowed.Is(err) {
		t.Fatalf("FetchMCPServerInfo = %v, want DestinationNotAllowed", err)
	}
}

// TestFetchMCPServerInfo_DiscoveryRefusalIsDestinationNotAllowed: a refusal on the discovery
// POST, whether of the URL itself or of a redirect hop, reaches the caller as
// DESTINATION_NOT_ALLOWED rather than as a generic discovery failure.
func TestFetchMCPServerInfo_DiscoveryRefusalIsDestinationNotAllowed(t *testing.T) {
	srv := loopbackBackend(t)
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://localhost"+srv.URL[len("http://127.0.0.1"):]+"/mcp", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirector.Close)

	// Positive control: with the loopback address permitted, discovery reaches the server and
	// fails for a reason that is not access control.
	useAccessControl(t, netguard.GuardConfig{Enabled: true, Mode: netguard.ModeAllow, Hostnames: []string{`127\.0\.0\.1`}, MaxRedirects: 5})
	if _, err := FetchMCPServerInfo(srv.URL, "", ""); err == nil || apperror.DestinationNotAllowed.Is(err) {
		t.Fatalf("permitted discovery = %v, want a non-access-control failure", err)
	}

	if _, err := FetchMCPServerInfo(redirector.URL, "", ""); !apperror.DestinationNotAllowed.Is(err) {
		t.Fatalf("discovery redirected to an unlisted host = %v, want DestinationNotAllowed", err)
	}

	useAccessControl(t, netguard.GuardConfig{Enabled: true, DenyPrivateNetworks: true})
	if _, err := FetchMCPServerInfo(srv.URL, "", ""); !apperror.DestinationNotAllowed.Is(err) {
		t.Fatalf("discovery of a refused address = %v, want DestinationNotAllowed", err)
	}
}
