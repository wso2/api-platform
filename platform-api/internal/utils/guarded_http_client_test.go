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
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestUpstreamFetchClientReachesLoopbackBackend is the end-to-end regression guard for MCP
// proxies with in-cluster upstreams: the client must actually connect to a private/loopback
// backend under netguard.PermitPrivateBlockMetadata(), not merely rate the address as
// allowed. The address-policy predicate itself (RFC 1918/loopback permitted, link-local/
// metadata/unspecified/multicast/broadcast denied) is covered by netguard's own test suite
// (httpkit/netguard/netguard_test.go); this test only needs to confirm this package wires
// that policy in correctly end to end.
func TestUpstreamFetchClientReachesLoopbackBackend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := NewUpstreamFetchClient(0)
	if err != nil {
		t.Fatalf("failed to build guarded upstream client: %v", err)
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("guarded upstream client failed to reach a loopback backend: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from the test backend, got %d", resp.StatusCode)
	}
}

// TestUpstreamFetchClientRefusesDeniedAddress confirms the dialer refuses a link-local
// address (the class that is never a legitimate upstream but is a standard SSRF target —
// 169.254.169.254 is the cloud instance metadata endpoint) even though
// netguard.PermitPrivateBlockMetadata() is otherwise permissive of private/loopback
// addresses. The target is an IP literal, so this does not require real network access: the
// guarded dialer's resolution step returns the literal itself without a DNS round trip, and
// the policy check rejects it before any connection is attempted.
func TestUpstreamFetchClientRefusesDeniedAddress(t *testing.T) {
	client, err := NewUpstreamFetchClient(0)
	if err != nil {
		t.Fatalf("failed to build guarded upstream client: %v", err)
	}

	if _, err := client.Get("http://169.254.169.254/latest/meta-data/"); err == nil {
		t.Fatal("expected the guarded client to refuse a link-local/metadata address")
	}
}

// TestUpstreamFetchClientRefusesCrossHostRedirect guards the credential-leak case: callers
// pass a custom auth header on MCP calls, and net/http forwards a custom header name across
// hosts, so a redirect off the original host must not be followed. A same-host redirect
// still is.
func TestUpstreamFetchClientRefusesCrossHostRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	crossHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/stolen", http.StatusFound)
	}))
	defer crossHost.Close()

	client, err := NewUpstreamFetchClient(0)
	if err != nil {
		t.Fatalf("failed to build guarded upstream client: %v", err)
	}

	if _, err := client.Get(crossHost.URL); err == nil {
		t.Fatal("expected the guarded client to refuse a cross-host redirect")
	}

	sameHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/moved" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer sameHost.Close()

	resp, err := client.Get(sameHost.URL + "/moved")
	if err != nil {
		t.Fatalf("expected a same-host redirect to be followed: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 after a same-host redirect, got %d", resp.StatusCode)
	}
}
