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

// Package egress turns the operator's [test_console.egress] settings into the
// two things that enforce them: a netguard.Policy applied at dial time, and a
// target check applied to the resolved invoke URL.
//
// # The model
//
// Two rules, and that is all of it:
//
//  1. deny always wins.
//  2. allow narrows when set, and is ignored when empty.
//
// There is deliberately no setting that re-opens something deny closed. An
// earlier version had one — a carve-out list evaluated ahead of the built-in
// refusals, which meant naming the link-local range in it re-opened the cloud
// metadata endpoint. Dropping it entirely makes that configuration
// unrepresentable rather than merely rejected, and costs nothing: a carve-out
// is only ever needed when there is no way to narrow, and allow_* narrows.
//
// # Why the checks live in two places
//
// Host and port are properties of the URL and do not depend on DNS, so they
// are knowable the moment the target is resolved — and checking them there
// buys a specific, loggable rejection (ErrHostNotAllowed, ErrPortNotAllowed)
// instead of a generic dial failure. The address checks cannot move there: a
// name that resolves to an approved address during a pre-check can resolve to
// a different one by the time the connection is made, so those belong inside
// the dial, which is where netguard performs them.
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

// Rejections from CheckTarget. The caller always answers with one generic
// response; these exist so the server-side log names the actual reason.
var (
	ErrHostNotAllowed = errors.New("gateway host is not in the configured allow_hosts")
	ErrPortNotAllowed = errors.New("gateway port is not in the configured allow_ports")
	ErrNoTargetHost   = errors.New("resolved target has no host")
)

// Named address groups a deny entry may use instead of spelling out CIDRs.
//
// They map onto netguard's own category flags rather than to literal ranges,
// which is the point of having them: "private" through netguard covers IPv6
// unique-local space as well as the three RFC 1918 ranges, and a hand-written
// list almost always forgets the former.
const (
	GroupPrivate  = "private"
	GroupLoopback = "loopback"
	GroupCGNAT    = "cgnat"
)

var denyGroupNames = []string{GroupPrivate, GroupLoopback, GroupCGNAT}

// Spec is the operator's settings as written in config.toml, before parsing.
type Spec struct {
	// AllowHosts, AllowCIDRs and AllowPorts each constrain one dimension of
	// the target. A non-empty list means the target must match it; an empty
	// list leaves that dimension unconstrained.
	AllowHosts []string
	AllowCIDRs []string
	AllowPorts []int
	// Deny holds CIDRs and named groups. Nothing overrides it.
	Deny []string
}

// Policy is a parsed, validated Spec.
type Policy struct {
	// netguardPolicy carries everything netguard can already express: the
	// always-refused categories, the deny groups, and deny_cidrs. Its
	// widening AllowCIDRs field is deliberately never set, which is what
	// keeps a carve-out from re-opening a refused range.
	netguardPolicy netguard.Policy
	// requireCIDRs is allow_cidrs. netguard has no "must be inside" concept,
	// so this one dimension is enforced here — inside the same dial, against
	// the same single resolution, so it cannot be bypassed by rebinding.
	requireCIDRs []*net.IPNet
	allowHosts   []hostPattern
	allowPorts   map[int]bool
	denyGroups   []string
}

// hostPattern is one allow_hosts entry: an exact host, or a single leading
// "*." wildcard matching one-or-more leading labels.
type hostPattern struct {
	// suffix is the part after "*." for a wildcard, or the whole host for an
	// exact entry. Always lowercase.
	suffix string
	// wildcard distinguishes "*.gw.internal" from "gw.internal".
	wildcard bool
}

func (h hostPattern) matches(host string) bool {
	if !h.wildcard {
		return host == h.suffix
	}
	// "*.gw.internal" matches "a.gw.internal" and "a.b.gw.internal", but not
	// "gw.internal" itself and not "evilgw.internal" — the separating dot is
	// part of what must match, which is what keeps this from being a
	// substring test.
	return strings.HasSuffix(host, "."+h.suffix)
}

// Parse validates a Spec and builds the Policy that enforces it.
//
// The base policy already refuses the ranges that are never a legitimate
// gateway and are never configurable: link-local (where the cloud metadata
// endpoint lives), the unspecified address, multicast/broadcast, and the IPv6
// ranges that translate onward to IPv4. Everything below only ever adds to
// that.
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

	if err := policy.applyDeny(spec.Deny); err != nil {
		return nil, err
	}

	require, err := parseCIDRs("allow_cidrs", spec.AllowCIDRs)
	if err != nil {
		return nil, err
	}
	for i, network := range require {
		if ones, _ := network.Mask.Size(); ones == 0 {
			return nil, fmt.Errorf("allow_cidrs entry %q covers every address, so it constrains nothing; "+
				"list the ranges you mean, or leave allow_cidrs unset", spec.AllowCIDRs[i])
		}
	}
	policy.requireCIDRs = require

	if policy.allowPorts, err = parsePorts(spec.AllowPorts); err != nil {
		return nil, err
	}
	if policy.allowHosts, err = parseHosts(spec.AllowHosts); err != nil {
		return nil, err
	}
	return policy, nil
}

// applyDeny folds each deny entry into the policy: a named group flips the
// matching netguard category, anything else must parse as a CIDR.
func (p *Policy) applyDeny(entries []string) error {
	for _, raw := range entries {
		entry := strings.ToLower(strings.TrimSpace(raw))
		switch entry {
		case "":
			return fmt.Errorf("deny contains an empty entry")
		case GroupPrivate:
			p.netguardPolicy.BlockPrivate = true
			p.denyGroups = append(p.denyGroups, entry)
			continue
		case GroupLoopback:
			p.netguardPolicy.BlockLoopback = true
			p.denyGroups = append(p.denyGroups, entry)
			continue
		case GroupCGNAT:
			p.netguardPolicy.BlockCGNAT = true
			p.denyGroups = append(p.denyGroups, entry)
			continue
		}

		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return fmt.Errorf("deny entry %q is neither a CIDR nor one of the group names %s",
				raw, strings.Join(denyGroupNames, ", "))
		}
		if ones, _ := network.Mask.Size(); ones == 0 {
			return fmt.Errorf("deny entry %q refuses every address, which disables the console behind a "+
				"generic rejection rather than a clear one; set [test_console] enabled = false instead", raw)
		}
		p.netguardPolicy.DenyCIDRs = append(p.netguardPolicy.DenyCIDRs, network)
	}
	return nil
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
// Resolution occurs here so each address can be checked with netguard's
// predicate and the optional allow_cidrs restriction. Validating IP literals
// avoids an additional DNS lookup and reuses netguard's policy logic.
//
// The connection uses the approved address directly, preventing DNS
// rebinding between validation and connection.
func (p *Policy) DialContext(timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, errDialRefused
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			slog.Warn("test console egress: host did not resolve", "host", host)
			return nil, errDialRefused
		}

		for _, ip := range ips {
			if err := netguard.Validate(ctx, p.netguardPolicy, ip.IP.String()); err != nil {
				slog.Warn("test console egress refused",
					"host", host, "addr", ip.IP.String(), "reason", classify(ip.IP))
				return nil, errDialRefused
			}
			if len(p.requireCIDRs) > 0 && !containsIP(p.requireCIDRs, ip.IP) {
				slog.Warn("test console egress refused",
					"host", host, "addr", ip.IP.String(), "reason", "address outside allow_cidrs")
				return nil, errDialRefused
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
		slog.Warn("test console egress: no resolved address accepted a connection",
			"host", host, "err", lastErr)
		return nil, errDialRefused
	}
}

// errDialRefused is what every refusal returns. It names neither the address
// nor the cause: the handler turns a dial failure into one generic response,
// and the specifics are in the log line above instead.
var errDialRefused = errors.New("destination is not allowed")

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
	case ip.IsLoopback():
		return "loopback address"
	case ip.IsPrivate():
		return "private address"
	default:
		return "address in a denied range"
	}
}

func containsIP(cidrs []*net.IPNet, ip net.IP) bool {
	for _, cidr := range cidrs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// CheckTarget applies the host and port constraints to a resolved invoke URL.
//
// Called once per request, before the relay dials, so a refusal is reported as
// a target that is not allowed rather than as a gateway that could not be
// reached — the two are different problems for whoever reads the log.
func (p *Policy) CheckTarget(u *url.URL) error {
	if u == nil || u.Hostname() == "" {
		return ErrNoTargetHost
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))

	if len(p.allowHosts) > 0 {
		matched := false
		for _, pattern := range p.allowHosts {
			if pattern.matches(host) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
		}
	}

	if len(p.allowPorts) > 0 {
		port, err := targetPort(u)
		if err != nil {
			return err
		}
		if !p.allowPorts[port] {
			return fmt.Errorf("%w: %d", ErrPortNotAllowed, port)
		}
	}
	return nil
}

// Describe renders the effective policy for the startup log, so an operator
// can confirm what is in force without re-reading config.toml.
func (p *Policy) Describe() string {
	groups := "none"
	if len(p.denyGroups) > 0 {
		groups = strings.Join(p.denyGroups, "+")
	}
	// deny_cidrs excludes the always-refused translated ranges this package
	// adds itself, so the number matches what the operator wrote.
	denyCIDRs := len(p.netguardPolicy.DenyCIDRs) - len(translatedIPv4Ranges())
	return fmt.Sprintf("allow_hosts=%d allow_cidrs=%d allow_ports=%d deny_cidrs=%d deny_groups=%s",
		len(p.allowHosts), len(p.requireCIDRs),
		len(p.allowPorts), denyCIDRs, groups)
}

// targetPort returns the port the URL dials, filling in the scheme default
// when it carries none — "https://gw.example.com" dials 443, and an
// allow_ports list that did not account for that would refuse every ordinary
// gateway.
func targetPort(u *url.URL) (int, error) {
	if explicit := u.Port(); explicit != "" {
		port, err := strconv.Atoi(explicit)
		if err != nil || port < 1 || port > 65535 {
			return 0, fmt.Errorf("%w: %s", ErrPortNotAllowed, explicit)
		}
		return port, nil
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return 443, nil
	case "http":
		return 80, nil
	default:
		return 0, fmt.Errorf("%w: unknown scheme %q", ErrPortNotAllowed, u.Scheme)
	}
}

func parseCIDRs(key string, in []string) ([]*net.IPNet, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]*net.IPNet, 0, len(in))
	for _, raw := range in {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil, fmt.Errorf("%s contains an empty entry", key)
		}
		_, network, err := net.ParseCIDR(trimmed)
		if err != nil {
			return nil, fmt.Errorf("%s entry %q is not a valid CIDR: %w", key, raw, err)
		}
		out = append(out, network)
	}
	return out, nil
}

func parsePorts(in []int) (map[int]bool, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[int]bool, len(in))
	for _, port := range in {
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("allow_ports entry %d is not a valid TCP port (1-65535)", port)
		}
		out[port] = true
	}
	return out, nil
}

// parseHosts validates allow_hosts entries.
//
// An entry is an exact host or a single leading "*." wildcard. A bare "*", an
// interior or trailing wildcard, and anything with more than one are all
// refused: the point of this list is to narrow, and a pattern that matches
// more than the operator reads it as matching is the bug this rule exists to
// prevent (see .claude/rules/go-cors-validation.md, which is the same bug
// class in origin matching).
func parseHosts(in []string) ([]hostPattern, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]hostPattern, 0, len(in))
	for _, raw := range in {
		entry := strings.ToLower(strings.TrimSpace(raw))
		if entry == "" {
			return nil, fmt.Errorf("allow_hosts contains an empty entry")
		}
		if entry == "*" {
			return nil, fmt.Errorf("allow_hosts entry %q matches every host; remove the list instead of wildcarding it", raw)
		}
		if strings.Count(entry, "*") > 1 {
			return nil, fmt.Errorf("allow_hosts entry %q has more than one wildcard; only a single leading \"*.\" is supported", raw)
		}
		if idx := strings.Index(entry, "*"); idx >= 0 {
			if !strings.HasPrefix(entry, "*.") {
				return nil, fmt.Errorf("allow_hosts entry %q may only wildcard a leading label, as in \"*.gw.example.com\"", raw)
			}
			suffix := strings.TrimPrefix(entry, "*.")
			if suffix == "" || !strings.Contains(suffix, ".") {
				return nil, fmt.Errorf("allow_hosts entry %q is too broad; wildcard at least two labels, as in \"*.gw.example.com\"", raw)
			}
			out = append(out, hostPattern{suffix: suffix, wildcard: true})
			continue
		}
		out = append(out, hostPattern{suffix: strings.TrimSuffix(entry, ".")})
	}
	return out, nil
}
