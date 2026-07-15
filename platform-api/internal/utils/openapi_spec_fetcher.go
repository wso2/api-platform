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

	// openAPISpecMaxRedirects caps redirect hops; every hop is still re-validated by the
	// SSRF-guarded dialer, so this only bounds redirect loops.
	openAPISpecMaxRedirects = 5
)

// FetchOpenAPISpecFromURL fetches an OpenAPI specification from an external URL and
// returns its body. The URL is operator/tenant-influenced (it originates from an LLM
// provider template), so the fetch is hardened against SSRF and resource exhaustion:
//
//   - Only http/https schemes are allowed; every other scheme is rejected.
//   - The host is resolved and every candidate IP is checked at dial time (defeating
//     DNS-rebinding) — loopback, private (RFC 1918 / ULA), link-local (incl. the cloud
//     metadata endpoint 169.254.169.254), unspecified, multicast and broadcast addresses
//     are refused.
//   - Redirects are bounded and each hop is dialed through the same guarded dialer.
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

	client := &http.Client{
		Timeout: openAPISpecFetchTimeout,
		Transport: &http.Transport{
			DialContext:           ssrfSafeDialContext,
			TLSHandshakeTimeout:   openAPISpecFetchTimeout,
			ResponseHeaderTimeout: openAPISpecFetchTimeout,
			DisableKeepAlives:     true,
			// No proxy: dialing must go through the guarded dialer, not a forward proxy
			// that could bypass the IP checks.
			Proxy: nil,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= openAPISpecMaxRedirects {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to a disallowed scheme")
			}
			return nil
		},
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

// ipIsAllowed is the address allow-check used by the dialer. It is a package variable
// only so tests can exercise the fetch/size-limit paths against a loopback test server;
// production code never reassigns it.
var ipIsAllowed = isPublicIP

// ssrfSafeDialContext resolves the target host and refuses to connect to any non-public
// address. It performs the resolution itself and dials the resolved IP directly, so a
// DNS name that resolves to a public IP during a pre-check cannot be re-resolved to an
// internal IP at connect time (DNS rebinding).
func ssrfSafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
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
		if !ipIsAllowed(ip.IP) {
			return nil, fmt.Errorf("host resolves to a disallowed address")
		}
	}

	dialer := &net.Dialer{Timeout: openAPISpecFetchTimeout}
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
