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
	"fmt"
	"net"
	"net/http"
	"time"
)

// maxOutboundRedirects caps redirect hops on guarded clients. Every hop is still dialed
// through the guarded dialer, so this only bounds redirect loops.
const maxOutboundRedirects = 5

// guardedDialContext builds a DialContext that resolves the target host itself, refuses to
// connect when any resolved address fails the supplied policy, and then dials the exact IP
// it just approved. Doing the resolution and the connection in one step is what closes the
// DNS-rebinding window: a name that answers with an allowed address during a pre-check
// cannot answer with a different one at connect time.
//
// The address policy is a parameter because different call sites have genuinely different
// threat models — see isPublicIP (fetching from the public internet) versus
// isAllowedUpstreamIP (reaching a backend the operator deployed).
func guardedDialContext(allow func(net.IP) bool, timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address")
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve host")
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("host has no addresses")
		}
		for _, ip := range ips {
			if !allow(ip.IP) {
				return nil, fmt.Errorf("host resolves to a disallowed address")
			}
		}

		dialer := &net.Dialer{Timeout: timeout}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		if lastErr != nil {
			return nil, fmt.Errorf("failed to connect to host")
		}
		return nil, fmt.Errorf("failed to connect to host")
	}
}

// isAllowedUpstreamIP is the address policy for calls to a backend the operator configured
// — an MCP proxy upstream, for instance. Such a backend is normally *meant* to be private:
// a Kubernetes ClusterIP, a service-DNS name resolving into RFC 1918 space, or a localhost
// port during development. Refusing those would break the ordinary deployment shape, so
// private, ULA, shared-address-space and loopback addresses are all permitted.
//
// What stays refused is the set of addresses that is never a legitimate upstream but is a
// standard SSRF target or a hazard:
//
//   - link-local unicast — 169.254.0.0/16 and fe80::/10, which is where the cloud instance
//     metadata endpoint (169.254.169.254) lives
//   - the unspecified address (0.0.0.0, ::), which the OS reinterprets as "local host"
//   - multicast and the IPv4 broadcast address, which are not meaningful HTTP peers
func isAllowedUpstreamIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		ip.Equal(net.IPv4bcast) {
		return false
	}
	return true
}

// upstreamIPIsAllowed is the address check used by NewUpstreamFetchClient. It is a package
// variable only so tests can vary the policy; production code never reassigns it.
var upstreamIPIsAllowed = isAllowedUpstreamIP

// NewUpstreamFetchClient returns an http.Client for calls to an operator- or
// tenant-configured backend whose URL is not fully trusted (MCP reachability probes, MCP
// JSON-RPC calls). Every connection — including each redirect hop — is dialed through the
// guarded dialer under isAllowedUpstreamIP, so private and in-cluster upstreams work
// normally while link-local/metadata addresses stay unreachable.
//
// For fetching from the public internet, use the stricter isPublicIP-based path in
// FetchOpenAPISpecFromURL instead.
//
// A non-positive timeout falls back to defaultUpstreamFetchTimeout.
func NewUpstreamFetchClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = defaultUpstreamFetchTimeout
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: guardedDialContext(func(ip net.IP) bool {
				return upstreamIPIsAllowed(ip)
			}, timeout),
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
			// A one-off outbound probe/call has nothing to gain from connection reuse, and
			// pooling a connection whose response was malformed can corrupt a later request.
			DisableKeepAlives: true,
			// No proxy: dialing must go through the guarded dialer, not a forward proxy
			// that could bypass the address checks.
			Proxy: nil,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxOutboundRedirects {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to a disallowed scheme")
			}
			return nil
		},
	}
}

// defaultUpstreamFetchTimeout bounds a guarded upstream call end to end when the caller
// does not supply its own timeout.
const defaultUpstreamFetchTimeout = 10 * time.Second
