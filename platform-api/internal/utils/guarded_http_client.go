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
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
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
	// (see server.InitSharedHTTPClient) and used by every SSRF-guarded call this package makes. It is built
	// from platform_api.http_client in config.toml under netguard.PermitPrivateBlockMetadata()
	// by default, so private and in-cluster upstreams work normally (a Kubernetes ClusterIP,
	// a service-DNS name resolving into RFC 1918 space, a localhost port during development)
	// while link-local/metadata addresses stay unreachable — see InitSharedHTTPClient.
	sharedHTTPClientMu sync.RWMutex
	sharedHTTPClient   *http.Client

	// sharedMCPResponseMaxBytes is the configured byte ceiling for MCP initialize/JSON-RPC
	// response bodies, set alongside sharedHTTPClient by InitSharedHTTPClient.
	sharedMCPResponseMaxBytes int64

	// sharedHTTPClientConfig is the httpkit config sharedHTTPClient was built from, or nil when
	// the client was set without it (InitSharedHTTPClient).
	sharedHTTPClientConfig *httpclient.Config

	// userURLClient serves destinations taken from tenant or user input, under
	// server.network_security.access_control (see InitUserURLClientFromConfig). When it is
	// not set, those destinations use sharedHTTPClient, as they did before access control
	// existed.
	userURLClient       *http.Client
	userURLClientConfig *httpclient.Config
)

// InitUserURLClientFromConfig builds the client for destinations taken from tenant or user
// input (spec URLs, MCP and agent-card URLs, GraphQL introspection) from
// cfg, which carries server.network_security.access_control as its destination policy, and keeps
// cfg so clients with other timeouts can be built with the same policy.
func InitUserURLClientFromConfig(cfg httpclient.Config) error {
	client, err := httpclient.New(cfg)
	if err != nil {
		return err
	}
	sharedHTTPClientMu.Lock()
	defer sharedHTTPClientMu.Unlock()
	userURLClient = client
	userURLClientConfig = &cfg
	return nil
}

// NewConfiguredDestinationClient returns the shared client for a destination configured in
// config.toml rather than taken from tenant or user input, such as the IdP JWKS URL. It
// carries platform_api.http_client's own policy and never server.network_security.access_control.
func NewConfiguredDestinationClient() (*http.Client, error) {
	sharedHTTPClientMu.RLock()
	defer sharedHTTPClientMu.RUnlock()
	if sharedHTTPClient == nil {
		return nil, errSharedHTTPClientNotInitialized
	}
	return sharedHTTPClient, nil
}

// CheckUserURL applies server.network_security.access_control to a tenant- or
// user-supplied URL without any network activity: its scheme, its hostname, and an
// IP-literal host's address. A hostname's resolved addresses are checked when the URL is
// dialed. It returns apperror.DestinationNotAllowed for a refused URL and nil otherwise,
// including when access control is off or the URL is malformed (the caller's own
// validation reports that).
func CheckUserURL(ctx context.Context, rawURL string) error {
	sharedHTTPClientMu.RLock()
	cfg := userURLClientConfig
	sharedHTTPClientMu.RUnlock()
	if cfg == nil {
		return nil
	}
	if err := cfg.SSRF.Policy.CheckURL(ctx, rawURL, false); errors.Is(err, netguard.ErrDestinationNotAllowed) {
		return apperror.DestinationNotAllowed.Wrap(err)
	}
	return nil
}

// DestinationNotAllowedError returns apperror.DestinationNotAllowed when err, from a
// request made with one of this package's clients, is a refused destination, and nil
// otherwise.
func DestinationNotAllowedError(err error) error {
	if errors.Is(err, netguard.ErrDestinationNotAllowed) {
		return apperror.DestinationNotAllowed.Wrap(err)
	}
	return nil
}

// userURLClientLocked returns the client and config for tenant- or user-supplied
// destinations: the access-control client when one was set, the shared client otherwise.
// The caller holds sharedHTTPClientMu.
func userURLClientLocked() (*http.Client, *httpclient.Config) {
	if userURLClient != nil {
		return userURLClient, userURLClientConfig
	}
	return sharedHTTPClient, sharedHTTPClientConfig
}

// InitSharedHTTPClient sets the single shared outbound *http.Client used by every
// SSRF-guarded call this package makes (MCP reachability/JSON-RPC calls, OpenAPI spec
// fetch), plus the byte ceiling applied to MCP response bodies, without the config the client
// was built from. Every entry point (cmd/main.go, platform.New) sets the shared client at
// startup through server.InitSharedHTTPClient, which uses InitSharedHTTPClientFromConfig,
// before any wiring that could reach NewUpstreamFetchClient, FetchOpenAPISpecFromURL, or the
// MCP utilities in this package.
func InitSharedHTTPClient(client *http.Client, mcpResponseMaxBytes int64) {
	initSharedHTTPClient(client, nil, mcpResponseMaxBytes)
}

// InitSharedHTTPClientFromConfig builds the shared outbound client from cfg and sets it as
// InitSharedHTTPClient does, and keeps cfg, so a client with other timeouts can be built with the
// same policy.
func InitSharedHTTPClientFromConfig(cfg httpclient.Config, mcpResponseMaxBytes int64) error {
	client, err := httpclient.New(cfg)
	if err != nil {
		return err
	}
	initSharedHTTPClient(client, &cfg, mcpResponseMaxBytes)
	return nil
}

func initSharedHTTPClient(client *http.Client, cfg *httpclient.Config, mcpResponseMaxBytes int64) {
	sharedHTTPClientMu.Lock()
	defer sharedHTTPClientMu.Unlock()
	sharedHTTPClient = client
	sharedHTTPClientConfig = cfg
	// A new shared client replaces any access-control client built alongside the previous
	// one; InitUserURLClientFromConfig sets it again when access control is enabled.
	userURLClient = nil
	userURLClientConfig = nil
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
//
// The returned client enforces server.network_security.access_control when it is enabled
// (see InitUserURLClientFromConfig); a refused destination fails with an error matching
// netguard.ErrDestinationNotAllowed.
func NewUpstreamFetchClient(timeout time.Duration) (*http.Client, error) {
	_ = timeout // retained for call-site compatibility; see doc comment above
	sharedHTTPClientMu.RLock()
	defer sharedHTTPClientMu.RUnlock()
	client, _ := userURLClientLocked()
	if client == nil {
		return nil, errSharedHTTPClientNotInitialized
	}
	return client, nil
}

// NewUpstreamClientWithTimeout returns a client with the shared client's policy (SSRF guard,
// redirects, TLS, pooling settings, response size bound) that waits up to timeout for a call,
// response headers included. It is built from the shared client's config, so the timeout applies
// to whatever transport httpkit builds. For a shared client set without its config, only a plain
// *http.Transport can take the response-header timeout, and any other transport is refused.
//
// It serves tenant- and user-supplied destinations, so it carries
// server.network_security.access_control when that is enabled.
func NewUpstreamClientWithTimeout(timeout time.Duration) (*http.Client, error) {
	sharedHTTPClientMu.RLock()
	client, cfg := userURLClientLocked()
	sharedHTTPClientMu.RUnlock()
	return clientWithTimeout(client, cfg, timeout, false)
}

// NewConfiguredDestinationClientWithTimeout is NewConfiguredDestinationClient with a per-call
// timeout, built as NewUpstreamClientWithTimeout is. It serves a destination that is deployment
// configuration rather than a tenant destination, such as a registered API portal, so it keeps
// platform_api.http_client's own policy and never carries server.network_security.access_control.
func NewConfiguredDestinationClientWithTimeout(timeout time.Duration) (*http.Client, error) {
	sharedHTTPClientMu.RLock()
	client, cfg := sharedHTTPClient, sharedHTTPClientConfig
	sharedHTTPClientMu.RUnlock()
	return clientWithTimeout(client, cfg, timeout, false)
}

// NewOperatorUpstreamClientWithTimeout is NewUpstreamClientWithTimeout for a destination the
// operator configured rather than a tenant, such as the feature-usage collector and its token
// endpoint. Those are often in-cluster services, so private, loopback and carrier-grade-NAT
// addresses stay reachable even when platform_api.http_client.ssrf.preset is "public_only",
// which is meant for tenant-supplied URLs. Every other part of the shared policy is kept:
// link-local/metadata, unspecified and multicast addresses are still refused at dial time,
// and the schemes, redirects, TLS and pooling settings are the shared client's.
func NewOperatorUpstreamClientWithTimeout(timeout time.Duration) (*http.Client, error) {
	sharedHTTPClientMu.RLock()
	client, cfg := sharedHTTPClient, sharedHTTPClientConfig
	sharedHTTPClientMu.RUnlock()
	return clientWithTimeout(client, cfg, timeout, true)
}

func clientWithTimeout(client *http.Client, cfg *httpclient.Config, timeout time.Duration, permitPrivate bool) (*http.Client, error) {
	if client == nil {
		return nil, errSharedHTTPClientNotInitialized
	}
	if cfg != nil {
		own := *cfg
		own.Timeouts.Overall = timeout
		own.Timeouts.ResponseHeader = timeout
		if permitPrivate && own.SSRF.Enabled {
			own.SSRF.Policy.BlockPrivate = false
			own.SSRF.Policy.BlockLoopback = false
			own.SSRF.Policy.BlockCGNAT = false
		}
		return httpclient.New(own)
	}
	// Set without its config, the shared client's policy can't be rebuilt, so an operator
	// client keeps exactly the shared client's policy.
	copied := *client
	copied.Timeout = timeout
	switch transport := client.Transport.(type) {
	case nil:
		// http.DefaultTransport has no response-header timeout.
	case *http.Transport:
		own := transport.Clone()
		own.ResponseHeaderTimeout = timeout
		copied.Transport = own
	default:
		return nil, fmt.Errorf("the shared HTTP client's transport (%T) can't take a response-header timeout", transport)
	}
	return &copied, nil
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
