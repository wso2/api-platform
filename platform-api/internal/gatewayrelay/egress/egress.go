/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package egress decides where the gateway relay may connect.
//
// Nothing is reachable unless an allow_hosts entry matches, so an empty list
// reaches nothing. Beneath the list, link-local (the cloud metadata endpoint),
// unspecified, multicast and broadcast addresses, and the IPv6 ranges that
// route onward to IPv4, are refused whatever it says. Private and loopback
// addresses stay dialable when listed, because that is where gateways live.
//
// Scheme, host and port are checked against the URL (CheckTarget); addresses
// are checked at dial time, on every resolved address, so DNS rebinding cannot
// slip past a pre-check.
package egress

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/httpkit/netguard"
)

// Rejections from CheckTarget. The caller gets one generic response; these name
// the reason in the log.
var (
	ErrTargetNotAllowed = errors.New("gateway target does not match any allow_hosts entry")
	ErrNoTargetHost     = errors.New("resolved target has no host")
)

// Spec is the operator's setting as written in config.toml, before parsing.
type Spec struct {
	// AllowHosts lists the relay's permitted targets. Empty permits none.
	AllowHosts []string
	// Logger receives the refusals, with the address and reason the caller is
	// never told. Nil uses slog.Default().
	Logger *slog.Logger
}

// Policy is a parsed, validated Spec.
type Policy struct {
	// netguardPolicy carries the always-refused ranges. AllowCIDRs is never set,
	// so no entry can re-open one.
	netguardPolicy netguard.Policy
	allow          []targetPattern
	logger         *slog.Logger
}

// targetPattern is one allow_hosts entry: a host glob, and a scheme and port that
// constrain only when the entry names them.
type targetPattern struct {
	scheme string // "" when the entry named none
	host   string // glob; "*" matches everything
	port   int    // 0 when the entry named none
	raw    string // as written, for Describe and error messages
}

// matchesHost reports whether host satisfies the glob. "*" crosses dots, but
// "*.example.com" still needs the dot, so it never admits "evilexample.com".
func (t targetPattern) matchesHost(host string) bool {
	return globMatch(t.host, host)
}

// globMatch is a backtracking glob over '*' only; path.Match would treat '/' as
// a separator, which a hostname has no use for.
func globMatch(pattern, s string) bool {
	var pIdx, sIdx, starIdx, matchIdx int
	starIdx = -1

	for sIdx < len(s) {
		switch {
		case pIdx < len(pattern) && pattern[pIdx] == '*':
			starIdx, matchIdx = pIdx, sIdx
			pIdx++
		case pIdx < len(pattern) && pattern[pIdx] == s[sIdx]:
			pIdx++
			sIdx++
		case starIdx >= 0:
			// Backtrack: let the last '*' absorb one more character.
			pIdx = starIdx + 1
			matchIdx++
			sIdx = matchIdx
		default:
			return false
		}
	}
	for pIdx < len(pattern) && pattern[pIdx] == '*' {
		pIdx++
	}
	return pIdx == len(pattern)
}

// Parse validates a Spec and builds its Policy. Entries only decide which names
// may be dialed; nothing widens the always-refused ranges.
func Parse(spec Spec) (*Policy, error) {
	policy := &Policy{netguardPolicy: netguard.PermitPrivateBlockMetadata(), logger: spec.Logger}
	if policy.logger == nil {
		policy.logger = slog.Default()
	}
	// netguard allows https alone by default; a gateway may serve plain http.
	policy.netguardPolicy.AllowedSchemes = []string{"http", "https"}
	// The preset has no category for IPv6 ranges that route onward to IPv4
	// (2002:a9fe:a9fe:: reaches 169.254.169.254), so they are denied explicitly.
	policy.netguardPolicy.DenyCIDRs = append(policy.netguardPolicy.DenyCIDRs, translatedIPv4Ranges()...)

	allow, err := parseAllowHosts(spec.AllowHosts)
	if err != nil {
		return nil, err
	}
	policy.allow = allow
	return policy, nil
}

// translatedIPv4Ranges are 6to4 (RFC 3056) and the NAT64 prefix (RFC 6052),
// which reach an IPv4 address without matching an IPv4 denylist entry. Mapped
// addresses (::ffff:a.b.c.d) need no entry: Go already treats them as IPv4.
func translatedIPv4Ranges() []*net.IPNet {
	return []*net.IPNet{mustCIDR("2002::/16"), mustCIDR("64:ff9b::/96")}
}

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic("egress: invalid built-in CIDR " + s + ": " + err.Error())
	}
	return n
}

// DialContext returns the relay client's dial function. It resolves the host
// itself and connects only to an address it checked, so a second lookup cannot
// return a different one.
func (p *Policy) DialContext(timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, errDialRefused
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			p.logger.Warn("gateway relay egress: host did not resolve", "host", host)
			return nil, errDialRefused
		}

		for _, ip := range ips {
			if err := netguard.Validate(ctx, p.netguardPolicy, ip.IP.String()); err != nil {
				p.logger.Warn("gateway relay egress refused",
					"host", host, "addr", ip.IP.String(), "reason", classify(ip.IP))
				return nil, ErrAddressNotAllowed
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
		p.logger.Warn("gateway relay egress: no resolved address accepted a connection",
			"host", host, "err", lastErr)
		return nil, errDialRefused
	}
}

// ErrAddressNotAllowed indicates a policy-refused address.
var ErrAddressNotAllowed = errors.New("resolved address is not allowed by the egress policy")

// errDialRefused indicates an unresolved or unreachable destination.
var errDialRefused = errors.New("destination could not be reached")

// classify names a refused address's category for the log. netguard.Validate
// makes the decision; this only labels it.
func classify(ip net.IP) string {
	switch {
	case ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast():
		return "link-local address"
	case ip.IsUnspecified():
		return "unspecified address"
	case ip.IsMulticast() || ip.Equal(net.IPv4bcast):
		return "multicast or broadcast address"
	default:
		return "address in a denied range"
	}
}

// CheckTarget applies allow_hosts to an invoke URL, before dialing, so a refusal
// reads as "not allowed" rather than "unreachable".
func (p *Policy) CheckTarget(u *url.URL) error {
	if u == nil || u.Hostname() == "" {
		return ErrNoTargetHost
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	scheme := strings.ToLower(u.Scheme)

	port, err := targetPort(u)
	if err != nil {
		return err
	}

	for _, pattern := range p.allow {
		if pattern.scheme != "" && pattern.scheme != scheme {
			continue
		}
		if pattern.port != 0 && pattern.port != port {
			continue
		}
		if pattern.matchesHost(host) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s://%s:%d", ErrTargetNotAllowed, scheme, host, port)
}

// Describe renders the policy for the startup log.
func (p *Policy) Describe() string {
	if len(p.allow) == 0 {
		return "allow_hosts=[] (nothing is reachable: set gateway_relay.egress.allow_hosts to use the gateway relay)"
	}
	entries := make([]string, 0, len(p.allow))
	for _, pattern := range p.allow {
		entries = append(entries, pattern.raw)
	}
	return "allow_hosts=[" + strings.Join(entries, " ") + "]"
}

// targetPort returns the port the URL dials, the scheme's default when it names
// none, so an entry pinning 443 matches "https://gw.example.com".
func targetPort(u *url.URL) (int, error) {
	if explicit := u.Port(); explicit != "" {
		port, err := strconv.Atoi(explicit)
		if err != nil || port < 1 || port > 65535 {
			return 0, fmt.Errorf("%w: invalid port %q", ErrTargetNotAllowed, explicit)
		}
		return port, nil
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return 443, nil
	case "http":
		return 80, nil
	default:
		return 0, fmt.Errorf("%w: unknown scheme %q", ErrTargetNotAllowed, u.Scheme)
	}
}

// parseAllowHosts validates allow_hosts entries, each "[scheme://]host[:port]"
// with "*" allowed anywhere in host.
func parseAllowHosts(in []string) ([]targetPattern, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]targetPattern, 0, len(in))
	for _, raw := range in {
		pattern, err := parseAllowHost(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, pattern)
	}
	return out, nil
}

func parseAllowHost(raw string) (targetPattern, error) {
	entry := strings.ToLower(strings.TrimSpace(raw))
	if entry == "" {
		return targetPattern{}, fmt.Errorf("allow_hosts contains an empty entry")
	}

	pattern := targetPattern{raw: strings.TrimSpace(raw)}

	if scheme, rest, found := strings.Cut(entry, "://"); found {
		if scheme != "http" && scheme != "https" {
			return targetPattern{}, fmt.Errorf(
				"allow_hosts entry %q names scheme %q; only http and https are supported", raw, scheme)
		}
		pattern.scheme = scheme
		entry = rest
	}

	// Nothing matches on a path, so one would silently widen the entry.
	if idx := strings.IndexAny(entry, "/?#"); idx >= 0 {
		return targetPattern{}, fmt.Errorf(
			"allow_hosts entry %q carries a path or query; entries match scheme, host and port only", raw)
	}

	// Split a trailing ":port", minding an IPv6 literal's own colons.
	host := entry
	if !strings.HasPrefix(entry, "[") {
		if h, p, found := strings.Cut(entry, ":"); found {
			if strings.Contains(p, ":") {
				return targetPattern{}, fmt.Errorf(
					"allow_hosts entry %q has more than one port; bracket an IPv6 literal as \"[::1]:8443\"", raw)
			}
			port, err := strconv.Atoi(p)
			if err != nil || port < 1 || port > 65535 {
				return targetPattern{}, fmt.Errorf(
					"allow_hosts entry %q names port %q, which is not a valid TCP port (1-65535)", raw, p)
			}
			host, pattern.port = h, port
		}
	} else if closing := strings.LastIndex(entry, "]"); closing >= 0 {
		host = entry[:closing+1]
		if rest := entry[closing+1:]; rest != "" {
			port, err := strconv.Atoi(strings.TrimPrefix(rest, ":"))
			if err != nil || port < 1 || port > 65535 {
				return targetPattern{}, fmt.Errorf(
					"allow_hosts entry %q names port %q, which is not a valid TCP port (1-65535)", raw, rest)
			}
			pattern.port = port
		}
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}

	if host == "" {
		return targetPattern{}, fmt.Errorf("allow_hosts entry %q names no host", raw)
	}
	pattern.host = strings.TrimSuffix(host, ".")
	return pattern, nil
}
