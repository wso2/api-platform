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
	"errors"
	"net/http"
	"sync"
	"time"
)

// defaultMCPResponseMaxBytes bounds an MCP initialize/JSON-RPC response body when the
// configured limit (Server.MCPResponseMaxBytes) is absent or non-positive. Matches the
// implicit 10 MiB cap httpclient.DefaultConfig() applied before the shared-client
// consolidation, so behavior is unchanged for an operator who doesn't set the new key.
const defaultMCPResponseMaxBytes int64 = 10 << 20 // 10 MiB

// defaultUpstreamFetchTimeout is the per-call budget fallback used when a caller of one of
// this package's guarded helpers (e.g. CheckURLReachability) supplies a non-positive
// timeout. Before the shared-client consolidation this fallback lived inside
// NewUpstreamFetchClient itself (it varied client construction); now that the client is
// built once at startup, the per-call budget is enforced by the caller via
// context.WithTimeout, so the fallback moves to the call site instead.
const defaultUpstreamFetchTimeout = 10 * time.Second

var (
	// sharedHTTPClient is the single outbound *http.Client built once at process startup
	// (see cmd/main.go) and used by every SSRF-guarded call this package makes. It is built
	// from platform_api.http_client in config.toml under netguard.PermitPrivateBlockMetadata()
	// by default, so private and in-cluster upstreams work normally (a Kubernetes ClusterIP,
	// a service-DNS name resolving into RFC 1918 space, a localhost port during development)
	// while link-local/metadata addresses stay unreachable — see InitSharedHTTPClient.
	sharedHTTPClientMu sync.RWMutex
	sharedHTTPClient   *http.Client

	// sharedMCPResponseMaxBytes is the configured byte ceiling for MCP initialize/JSON-RPC
	// response bodies, set alongside sharedHTTPClient by InitSharedHTTPClient.
	sharedMCPResponseMaxBytes int64
)

// InitSharedHTTPClient sets the single shared outbound *http.Client used by every
// SSRF-guarded call this package makes (MCP reachability/JSON-RPC calls, OpenAPI spec
// fetch), plus the byte ceiling applied to MCP response bodies. Must be called exactly once,
// at process startup (cmd/main.go, right after config is loaded), before any server/handler
// wiring that could reach NewUpstreamFetchClient, FetchOpenAPISpecFromURL, or the MCP
// utilities in this package.
func InitSharedHTTPClient(client *http.Client, mcpResponseMaxBytes int64) {
	sharedHTTPClientMu.Lock()
	defer sharedHTTPClientMu.Unlock()
	sharedHTTPClient = client
	if mcpResponseMaxBytes <= 0 {
		mcpResponseMaxBytes = defaultMCPResponseMaxBytes
	}
	sharedMCPResponseMaxBytes = mcpResponseMaxBytes
}

// errSharedHTTPClientNotInitialized is returned instead of panicking on a nil client when a
// caller reaches these guarded helpers before InitSharedHTTPClient has run (e.g. a test that
// forgot its setup) — a clear error is safer than a nil-pointer panic on client.Do.
var errSharedHTTPClientNotInitialized = errors.New("shared outbound HTTP client not initialized: InitSharedHTTPClient must be called at startup before this code path is reached")

// NewUpstreamFetchClient returns the shared http.Client used for calls to an operator- or
// tenant-configured backend whose URL is not fully trusted (MCP reachability probes, MCP
// JSON-RPC calls). It is built once at startup (see InitSharedHTTPClient) on httpkit's
// shared, secure-by-default outbound client: every connection — including each redirect hop
// — is dialed through netguard's SSRF guard, so private and in-cluster upstreams work
// normally (a Kubernetes ClusterIP, a service-DNS name resolving into RFC 1918 space, a
// localhost port during development) while link-local/metadata addresses stay unreachable
// under the default netguard.PermitPrivateBlockMetadata() policy. The guard resolves the
// host and dials the exact resolved IP in one step, which is what closes the
// DNS-rebinding window, and every redirect hop is re-validated the same way and bounded to
// the original host — callers pass their own auth headers on these requests, and net/http
// forwards a custom header name verbatim across a cross-host redirect, so a
// malicious/compromised upstream must not be able to redirect its way into a credential
// leak.
//
// timeout is accepted for call-site compatibility but no longer varies client construction
// — the shared client's own Timeouts.Overall config field is a safety net only, and callers
// enforce their real per-call budget via context.WithTimeout instead (see mcp.go and
// common.go). A non-positive timeout is treated the same way: it has no effect on the
// returned client.
func NewUpstreamFetchClient(timeout time.Duration) (*http.Client, error) {
	_ = timeout // retained for call-site compatibility; see doc comment above
	sharedHTTPClientMu.RLock()
	defer sharedHTTPClientMu.RUnlock()
	if sharedHTTPClient == nil {
		return nil, errSharedHTTPClientNotInitialized
	}
	return sharedHTTPClient, nil
}

// mcpResponseMaxBytes returns the configured byte ceiling for MCP response bodies, falling
// back to defaultMCPResponseMaxBytes if InitSharedHTTPClient has not run (should not happen
// outside of a misconfigured test).
func mcpResponseMaxBytes() int64 {
	sharedHTTPClientMu.RLock()
	defer sharedHTTPClientMu.RUnlock()
	if sharedMCPResponseMaxBytes <= 0 {
		return defaultMCPResponseMaxBytes
	}
	return sharedMCPResponseMaxBytes
}
