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

func TestFetchOpenAPISpecFromURL_BlocksInternalAddress(t *testing.T) {
	// The SSRF guard is active (ipIsAllowed not overridden), so a loopback URL must be
	// refused before any connection is made.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("openapi: 3.0.0"))
	}))
	defer srv.Close()

	if _, err := FetchOpenAPISpecFromURL(context.Background(), srv.URL, 0); err == nil {
		t.Fatal("expected loopback address to be blocked, got nil error")
	}
}

func TestFetchOpenAPISpecFromURL_FetchAndSizeLimit(t *testing.T) {
	// Relax the address check so the loopback test server is reachable, then restore it.
	orig := ipIsAllowed
	ipIsAllowed = func(net.IP) bool { return true }
	defer func() { ipIsAllowed = orig }()

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
