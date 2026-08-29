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
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	cases := []struct {
		ip      string
		allowed bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"93.184.216.34", true},    // example.com
		{"100.64.0.1", false},      // RFC 6598 shared address space (CGNAT)
		{"100.127.255.255", false}, // CGNAT upper bound
		{"100.63.255.255", true},   // just below the CGNAT range — public
		{"100.128.0.1", true},      // just above the CGNAT range — public
		{"127.0.0.1", false},       // loopback
		{"::1", false},             // loopback v6
		{"10.0.0.1", false},        // private
		{"172.16.5.4", false},      // private
		{"192.168.1.1", false},     // private
		{"169.254.169.254", false}, // link-local / cloud metadata endpoint
		{"fe80::1", false},         // link-local v6
		{"fd00::1", false},         // unique local v6 (private)
		{"0.0.0.0", false},         // unspecified
		{"224.0.0.1", false},       // multicast
		{"255.255.255.255", false}, // broadcast
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("failed to parse test IP %q", c.ip)
		}
		if got := isPublicIP(ip); got != c.allowed {
			t.Errorf("isPublicIP(%s) = %v, want %v", c.ip, got, c.allowed)
		}
	}
}

func TestFetchOpenAPISpecFromURL_RejectsBadURLs(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"file scheme", "file:///etc/passwd"},
		{"ftp scheme", "ftp://example.com/spec.yaml"},
		{"no host", "http://"},
		{"gopher", "gopher://example.com/"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := FetchOpenAPISpecFromURL(context.Background(), c.url, 0); err == nil {
				t.Fatalf("expected error for %q, got nil", c.url)
			}
		})
	}
}

// TestFetchOpenAPISpecFromURL_ReachesPrivateAddress asserts the CURRENT, intended behavior:
// the shared HTTP client's default SSRF policy is netguard.PermitPrivateBlockMetadata(), not
// the stricter netguard.PublicOnly() this fetch used before the shared-client consolidation
// (see the platform-api HTTP client consolidation task). A private/loopback backend — an
// httptest server counts as one — must now be REACHABLE, matching how an operator-configured
// LLM provider template's OpenAPI spec URL can legitimately point at an in-cluster service.
// Link-local/metadata addresses are still refused; see TestFetchOpenAPISpecFromURL_BlocksLinkLocalAddress.
func TestFetchOpenAPISpecFromURL_ReachesPrivateAddress(t *testing.T) {
	const body = "openapi: 3.0.0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	got, err := FetchOpenAPISpecFromURL(context.Background(), srv.URL, 0)
	if err != nil {
		t.Fatalf("expected a private/loopback address to be reachable, got error: %v", err)
	}
	if got != body {
		t.Fatalf("body mismatch:\n got: %q\nwant: %q", got, body)
	}
}

// TestFetchOpenAPISpecFromURL_BlocksLinkLocalAddress confirms link-local/metadata addresses
// (169.254.169.254, the cloud instance metadata endpoint) stay refused even though
// netguard.PermitPrivateBlockMetadata() otherwise permits private/loopback addresses. The
// target is an IP literal, so no real network access is required for this to fail closed.
func TestFetchOpenAPISpecFromURL_BlocksLinkLocalAddress(t *testing.T) {
	if _, err := FetchOpenAPISpecFromURL(context.Background(), "http://169.254.169.254/latest/meta-data/", 0); err == nil {
		t.Fatal("expected a link-local/metadata address to be blocked, got nil error")
	}
}

func TestFetchOpenAPISpecFromURL_FetchAndSizeLimit(t *testing.T) {
	// No policy override needed: the shared test client (see TestMain) already permits
	// loopback/private addresses under netguard.PermitPrivateBlockMetadata(), exactly what's
	// needed to exercise the fetch/size-limit logic against an httptest server.
	const body = "openapi: 3.0.3\ninfo:\n  title: Test\n  version: v1.0\npaths: {}\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	// Happy path.
	got, err := FetchOpenAPISpecFromURL(context.Background(), srv.URL, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != body {
		t.Fatalf("body mismatch:\n got: %q\nwant: %q", got, body)
	}

	// Size limit: cap below the body length and expect rejection.
	if _, err := FetchOpenAPISpecFromURL(context.Background(), srv.URL, 4); err == nil {
		t.Fatal("expected size-limit error, got nil")
	}
}

func TestValidateExternalURL(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"public ipv4 literal", "https://8.8.8.8", false},
		{"cloud metadata", "http://169.254.169.254/latest/meta-data/", true},
		{"loopback ipv4", "http://127.0.0.1", true},
		{"loopback ipv4 with port and path", "http://127.0.0.1:8080/x", true},
		{"private rfc1918 10", "http://10.0.0.5", true},
		{"private rfc1918 192", "http://192.168.1.10:9000", true},
		{"cgnat rfc6598", "http://100.64.0.1", true},
		{"loopback ipv6", "http://[::1]", true},
		{"link-local ipv6", "http://[fe80::1]", true},
		{"unspecified ipv4", "http://0.0.0.0", true},
		{"malformed", "not-a-url", true},
		{"non-http scheme", "ftp://8.8.8.8", true},
		{"userinfo credentials", "http://admin:value@8.8.8.8", true},
		{"empty", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateExternalURL(ctx, tc.url)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateExternalURL(%q): expected error, got nil", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateExternalURL(%q): expected no error, got %v", tc.url, err)
			}
		})
	}
}
