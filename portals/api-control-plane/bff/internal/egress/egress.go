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

// Package egress decides where the Gateway relay's relay may connect.
//
// # The model
//
// One rule: nothing is reachable unless an allow_hosts entry matches it.
//
// An empty list therefore reaches nothing, which is the point — a deployment
// that configures no policy gets a console that refuses every target, not one
// that dials anywhere Platform API happened to name. An earlier version read
// an empty list as "unconstrained", so the safe-looking configuration was the
// open one.
//
// # Beneath the list
//
// A small set of addresses is refused however allow_hosts reads: link-local
// (where the cloud metadata endpoint lives), the unspecified address,
// multicast and broadcast, and the IPv6 prefixes that embed an IPv4 address
// and route onward to it. None is ever a legitimate gateway, and none is
// configurable — allow_hosts matches names, and a name an operator trusts can
// still resolve to one of these, deliberately or otherwise.
//
// # What is NOT refused by category
//
// Private, loopback and carrier-grade-NAT addresses are all dialable, and an
// IP literal is matched like any other host. That is deliberate: a gateway
// legitimately lives on a ClusterIP, and on loopback during development, so
// refusing those by category would break the ordinary case while allow_hosts
// already decides the question.
//
// It does mean a wide entry is a wide grant — "*" reaches anything the floor
// above does not refuse, the BFF's own host included. An operator listing
// localhost has chosen to reach localhost; the policy does not second-guess
// it, so the narrower the list, the better.
//
// # Why the checks live in two places
//
// Scheme, host and port are properties of the URL and do not depend on DNS,
// so they are knowable the moment the target is resolved. Checking them there
// buys a specific, loggable rejection instead of a generic dial failure.
//
// The address check cannot move there. A name that resolves to an approved
// address during a pre-check can resolve to a different one by the time the
// connection is made, so it belongs inside the dial — which is also where the
// connection is pinned to the address that was actually validated.
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

// Rejections from CheckTarget. The caller always receives one generic
// response; these exist so the server-side log names the actual reason.
var (
	ErrTargetNotAllowed = errors.New("gateway target does not match any allow_hosts entry")
	ErrNoTargetHost     = errors.New("resolved target has no host")
)

// Spec is the operator's setting as written in config.toml, before parsing.
type Spec struct {
	// AllowHosts lists the relay's permitted targets. Empty permits none.
	AllowHosts []string
}

// Policy is a parsed, validated Spec.
type Policy struct {
	// netguardPolicy carries the always-refused categories. Its widening
	// AllowCIDRs field is deliberately never set: an operator entry must not
	// be able to re-open a refused range.
	netguardPolicy netguard.Policy
	allow          []targetPattern
}

// targetPattern is one allow_hosts entry: a host glob, with an optional
// scheme and an optional port that must match exactly when present.
//
// Omitting a part leaves it unconstrained, so "gw.example.com" accepts both
// schemes on any port, while "https://gw.example.com:8443" accepts only that
// one combination.
type targetPattern struct {
	scheme string // "" when the entry named none
	host   string // glob; "*" matches everything
	port   int    // 0 when the entry named none
	raw    string // as written, for Describe and error messages
}

// matchesHost reports whether host satisfies the pattern's glob.
//
// "*" matches any run of characters, dots included, so "*.example.com"
// accepts "a.b.example.com" as well as "a.example.com". It does not accept
// "example.com" itself: the separating dot is part of what must match, which
// is what keeps the entry from also admitting "evilexample.com".
func (t targetPattern) matchesHost(host string) bool {
	return globMatch(t.host, host)
}

// globMatch is a linear, backtracking glob over '*' only. Written out rather
// than taken from path.Match because that treats '/' as a separator '*' will
// not cross, which is not how a hostname reads.
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

// Parse validates a Spec and builds the Policy that enforces it.
//
// The base policy refuses the ranges that are never a legitimate gateway and
// are never configurable. Everything the operator writes only ever decides
// which names may be dialed at all; nothing widens that base.
func Parse(spec Spec) (*Policy, error) {
	policy := &Policy{netguardPolicy: netguard.PermitPrivateBlockMetadata()}
	// Both schemes are ordinary serving modes for a gateway; netguard
	// defaults to https alone, which would refuse a plain-http one.
	policy.netguardPolicy.AllowedSchemes = []string{"http", "https"}
	// The preset refuses link-local, unspecified and multicast. It has no
	// category for the IPv6 ranges that embed an IPv4 address and route to
	// it, and an IPv4 denylist entry does not match them — 2002:a9fe:a9fe::
	// reaches 169.254.169.254 while being neither link-local nor IPv4 — so
	// they go in as ordinary denied ranges.
	policy.netguardPolicy.DenyCIDRs = append(policy.netguardPolicy.DenyCIDRs, translatedIPv4Ranges()...)

	allow, err := parseAllowHosts(spec.AllowHosts)
	if err != nil {
		return nil, err
	}
	policy.allow = allow
	return policy, nil
}

// NetguardPolicy returns the address policy the dialer applies. Exported for
// tests that assert on what the operator's settings produced.
func (p *Policy) NetguardPolicy() netguard.Policy { return p.netguardPolicy }

// translatedIPv4Ranges are the IPv6 prefixes that embed an IPv4 address and
// are routed onward to it: 6to4 (RFC 3056) and the NAT64 well-known prefix
// (RFC 6052). Neither is matched by an IPv4 denylist entry naming the address
// it reaches, and neither is ever a legitimate gateway here.
//
// IPv4-mapped addresses (::ffff:a.b.c.d) are deliberately absent: Go
// normalises those, so net.IP's predicates and net.IPNet.Contains already see
// them as the IPv4 address they carry.
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

// DialContext returns the relay HTTP client's dial function.
//
// Resolution happens here so each address can be checked against the
// always-refused ranges, and so the connection is made to an address that was
// actually validated rather than to whatever a second lookup would return.
func (p *Policy) DialContext(timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, errDialRefused
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			slog.Warn("gateway relay egress: host did not resolve", "host", host)
			return nil, errDialRefused
		}

		for _, ip := range ips {
			if err := netguard.Validate(ctx, p.netguardPolicy, ip.IP.String()); err != nil {
				slog.Warn("gateway relay egress refused",
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
		slog.Warn("gateway relay egress: no resolved address accepted a connection",
			"host", host, "err", lastErr)
		return nil, errDialRefused
	}
}

// ErrAddressNotAllowed indicates a policy-refused address.
var ErrAddressNotAllowed = errors.New("resolved address is not allowed by the egress policy")

// errDialRefused indicates an unresolved or unreachable destination.
var errDialRefused = errors.New("destination could not be reached")

// classify names the category an address fell into, for the log line only.
//
// It is not the security decision — netguard.Validate already made that. A
// mismatch between this and netguard would mislabel a log entry and nothing
// else, which is why restating the categories is acceptable here and would
// not be in the dial path.
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

// CheckTarget applies allow_hosts to a resolved invoke URL.
//
// Called once per request, before the relay dials, so a refusal is reported
// as a target that is not allowed rather than as a gateway that could not be
// reached — the two are different problems for whoever reads the log.
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
	// Includes the empty-list case, which is the deny-all default.
	return fmt.Errorf("%w: %s://%s:%d", ErrTargetNotAllowed, scheme, host, port)
}

// Describe renders the effective policy for the startup log, so an operator
// can confirm what is in force without re-reading config.toml.
func (p *Policy) Describe() string {
	if len(p.allow) == 0 {
		return "allow_hosts=[] (nothing is reachable — set allow_hosts to use the gateway relay)"
	}
	entries := make([]string, 0, len(p.allow))
	for _, pattern := range p.allow {
		entries = append(entries, pattern.raw)
	}
	return "allow_hosts=[" + strings.Join(entries, " ") + "]"
}

// targetPort returns the port the URL dials, filling in the scheme default
// when it carries none — "https://gw.example.com" dials 443, and an entry
// that pinned 443 should match it.
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

// parseAllowHosts validates allow_hosts entries.
//
// An entry is "[scheme://]host[:port]", where host may carry "*" anywhere.
// Scheme and port constrain only when written, so the shortest useful entry
// is a bare hostname and the broadest is "*".
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

	// A path would silently widen the entry, since nothing below matches on
	// one — refuse it rather than accept something that does not do what it
	// reads as doing.
	if idx := strings.IndexAny(entry, "/?#"); idx >= 0 {
		return targetPattern{}, fmt.Errorf(
			"allow_hosts entry %q carries a path or query; entries match scheme, host and port only", raw)
	}

	// Split a trailing ":port", taking care not to mistake an IPv6 literal's
	// own colons for one.
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
