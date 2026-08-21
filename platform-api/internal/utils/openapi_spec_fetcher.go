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
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// defaultOpenAPISpecMaxFetchBytes bounds the fetched OpenAPI spec body so a hostile
	// or misconfigured URL cannot exhaust memory. Used when the configured limit is absent
	// or non-positive.
	defaultOpenAPISpecMaxFetchBytes int64 = 5 << 20 // 5 MiB

	// openAPISpecFetchTimeout bounds the whole fetch (DNS + connect + TLS + body read).
	openAPISpecFetchTimeout = 15 * time.Second
)

// FetchOpenAPISpecFromURL fetches an OpenAPI specification from an external URL and
// returns its body. The URL is operator/tenant-influenced (it originates from an LLM
// provider template), so the fetch is hardened against SSRF and resource exhaustion:
//
//   - Only http/https schemes are allowed; every other scheme is rejected.
//   - The host is resolved and every candidate IP is checked at dial time (defeating
//     DNS-rebinding) against the shared client's configured SSRF policy — by default
//     netguard.PermitPrivateBlockMetadata(), which permits private/loopback/in-cluster
//     upstreams (a Kubernetes ClusterIP, a service-DNS name, localhost) and refuses only
//     link-local/metadata/unspecified/multicast addresses. See InitSharedHTTPClient /
//     Server.HTTPClient.SSRF for the operator-configurable policy.
//   - Redirects are bounded, may not leave the original host, and each hop is dialed
//     through the same guarded dialer.
//   - The response body is read through an io.LimitReader capped at maxBytes.
//   - Errors are returned sterile (no internal host/IP detail) so callers can log them
//     internally without leaking infrastructure information to clients.
//
// maxBytes <= 0 falls back to defaultOpenAPISpecMaxFetchBytes.
func FetchOpenAPISpecFromURL(ctx context.Context, rawURL string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = defaultOpenAPISpecMaxFetchBytes
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid OpenAPI spec URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("OpenAPI spec URL must use http or https")
	}
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("OpenAPI spec URL must include a host")
	}

	ctx, cancel := context.WithTimeout(ctx, openAPISpecFetchTimeout)
	defer cancel()

	client, err := NewUpstreamFetchClient(0)
	if err != nil {
		// Do not surface the underlying config error to the caller.
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to build OpenAPI spec request")
	}
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml, text/plain, */*")
	req.Header.Set("User-Agent", "wso2-api-platform")

	resp, err := client.Do(req)
	if err != nil {
		// Do not surface the underlying net error (it can leak resolved IPs/hosts).
		return "", fmt.Errorf("failed to fetch OpenAPI spec")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAPI spec URL returned an unexpected status")
	}

	// Bound the body: read one extra byte so we can detect an over-limit response.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return "", fmt.Errorf("failed to read OpenAPI spec response")
	}
	if int64(len(data)) > maxBytes {
		return "", fmt.Errorf("OpenAPI spec exceeds the maximum allowed size")
	}

	return string(data), nil
}

// cgnatRange is RFC 6598 shared address space (carrier-grade NAT): 100.64.0.0/10.
// net.IP.IsPrivate does not cover it, yet it can route to internal infrastructure, so
// it is refused explicitly.
var cgnatRange = func() *net.IPNet {
	_, n, _ := net.ParseCIDR("100.64.0.0/10")
	return n
}()

// isPublicIP reports whether ip is a routable public address safe to fetch from. It
// rejects loopback, private (RFC 1918 / IPv6 ULA), RFC 6598 shared address space
// (100.64.0.0/10), link-local (which includes the 169.254.169.254 cloud metadata
// endpoint), unspecified, multicast and broadcast ranges.
//
// This is the same policy netguard.PublicOnly() applies (stricter than the shared HTTP
// client's default netguard.PermitPrivateBlockMetadata() policy used at dial time for
// FetchOpenAPISpecFromURL); it is kept here as a standalone predicate because
// ValidateExternalURL (common.go) needs a pure IP check — with no dial involved — for a
// URL whose hostname is already an IP literal. ValidateExternalURL guards a different call
// path (LLM provider endpoint URL validation in internal/service/llm.go) and intentionally
// keeps the stricter public-only bar independent of this package's shared fetch client.
func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		ip.Equal(net.IPv4bcast) ||
		(cgnatRange != nil && cgnatRange.Contains(ip)) {
		return false
	}
	return true
}
