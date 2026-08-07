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
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIsAllowedUpstreamIP pins the upstream address policy in both directions. An MCP
// upstream is normally a service the operator deployed, so private/in-cluster/loopback
// addresses MUST stay reachable — a regression to a public-only policy would break every
// cluster-internal MCP proxy. The metadata endpoint must stay unreachable regardless.
func TestIsAllowedUpstreamIP(t *testing.T) {
	allowed := []string{
		"10.4.2.9",        // k8s ClusterIP / RFC 1918
		"172.16.0.5",      // RFC 1918
		"192.168.1.20",    // RFC 1918
		"100.64.0.1",      // RFC 6598 shared address space
		"127.0.0.1",       // loopback — local development
		"::1",             // IPv6 loopback
		"fd00::1",         // IPv6 ULA — in-cluster
		"93.184.216.34",   // public
		"2606:2800:220::", // public IPv6
	}
	for _, s := range allowed {
		if ip := net.ParseIP(s); !isAllowedUpstreamIP(ip) {
			t.Errorf("expected %s to be an allowed upstream address", s)
		}
	}

	denied := []string{
		"169.254.169.254", // cloud instance metadata
		"169.254.1.1",     // link-local
		"fe80::1",         // IPv6 link-local
		"0.0.0.0",         // unspecified
		"::",              // IPv6 unspecified
		"224.0.0.1",       // multicast
		"255.255.255.255", // broadcast
	}
	for _, s := range denied {
		if ip := net.ParseIP(s); isAllowedUpstreamIP(ip) {
			t.Errorf("expected %s to be a denied upstream address", s)
		}
	}

	if isAllowedUpstreamIP(nil) {
		t.Error("expected a nil address to be denied")
	}
}

// TestUpstreamFetchClientReachesLoopbackBackend is the end-to-end counterpart: the client
// must actually connect to a private/loopback backend, not merely rate the address as
// allowed. This is the regression guard for MCP proxies with in-cluster upstreams.
func TestUpstreamFetchClientReachesLoopbackBackend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := NewUpstreamFetchClient(0).Get(srv.URL)
	if err != nil {
		t.Fatalf("guarded upstream client failed to reach a loopback backend: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from the test backend, got %d", resp.StatusCode)
	}
}

// TestUpstreamFetchClientRefusesDeniedAddress confirms the dialer refuses a denied address
// even though the policy is otherwise permissive.
func TestUpstreamFetchClientRefusesDeniedAddress(t *testing.T) {
	restore := upstreamIPIsAllowed
	upstreamIPIsAllowed = func(net.IP) bool { return false }
	defer func() { upstreamIPIsAllowed = restore }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	if _, err := NewUpstreamFetchClient(0).Get(srv.URL); err == nil {
		t.Fatal("expected the guarded client to refuse a disallowed address")
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

	if _, err := NewUpstreamFetchClient(0).Get(crossHost.URL); err == nil {
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

	resp, err := NewUpstreamFetchClient(0).Get(sameHost.URL + "/moved")
	if err != nil {
		t.Fatalf("expected a same-host redirect to be followed: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 after a same-host redirect, got %d", resp.StatusCode)
	}
}
