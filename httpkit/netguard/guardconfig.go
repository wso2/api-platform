/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package netguard

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"golang.org/x/net/idna"
)

// ErrDestinationNotAllowed is returned, possibly wrapped, whenever a guard
// refuses a destination: a hostname the HostRule refuses, a scheme outside
// the allowed set, or an address the Policy refuses. Callers test for it with
// errors.Is to tell a refused destination apart from an unreachable one. Its
// message is deliberately generic: which rule refused the destination, and
// the address it resolved to, must stay out of client-facing responses.
var ErrDestinationNotAllowed = errors.New("netguard: destination is not allowed")

// Host-rule modes.
const (
	// ModeDeny refuses a host that matches any pattern and permits the rest.
	ModeDeny = "deny"
	// ModeAllow permits only a host that matches a pattern.
	ModeAllow = "allow"
)

// Limits on the hostname pattern list, enforced when it is compiled.
const (
	MaxHostnamePatterns      = 256
	MaxHostnamePatternLength = 255
)

// GuardConfig is the operator-facing description of outbound access control
// for destinations that come from tenant or user input. It is the single
// place configuration is turned into a Policy (see Build), so every component
// that loads it enforces the same semantics.
type GuardConfig struct {
	// Enabled turns access control on. When false, Build returns a zero
	// Policy and callers must not guard the client at all.
	Enabled bool
	// Mode is ModeDeny or ModeAllow. Empty means ModeDeny.
	Mode string
	// Hostnames are RE2 regular expressions. Each is matched,
	// case-insensitively, against the whole normalized hostname of a
	// destination — never its port, path or userinfo — because Build anchors
	// it as ^(?:pattern)$.
	Hostnames []string
	// DenyPrivateNetworks additionally refuses private, loopback,
	// carrier-grade NAT, unique-local, site-local and benchmarking addresses.
	// Link-local (cloud metadata), unspecified and multicast/broadcast
	// addresses are refused whatever this is set to.
	DenyPrivateNetworks bool
	// MaxRedirects bounds redirect hops; 0 uses CheckRedirect's default.
	MaxRedirects int
	// AllowedSchemes lists the URL schemes a destination may use. Empty
	// means {"http", "https"}.
	AllowedSchemes []string
}

// Validate reports whether the configuration is usable, without building a
// Policy. A disabled configuration is always valid.
func (c GuardConfig) Validate() error {
	_, err := c.Build()
	return err
}

// Build turns the configuration into a Policy. It fails on an unknown mode,
// an allow mode with no patterns, an invalid or oversized pattern, a negative
// redirect limit, or a scheme other than http/https. A disabled configuration
// builds the zero Policy and no error.
func (c GuardConfig) Build() (Policy, error) {
	if !c.Enabled {
		return Policy{}, nil
	}
	rule, err := CompileHostRule(c.Mode, c.Hostnames)
	if err != nil {
		return Policy{}, err
	}
	if c.MaxRedirects < 0 {
		return Policy{}, fmt.Errorf("max_redirects must not be negative")
	}
	schemes, err := normalizeSchemes(c.AllowedSchemes)
	if err != nil {
		return Policy{}, err
	}

	policy := Policy{
		BlockLinkLocal:          true,
		BlockUnspecified:        true,
		BlockMulticastBroadcast: true,
		AllowedSchemes:          schemes,
		Hosts:                   rule,
	}
	if c.DenyPrivateNetworks {
		policy.BlockPrivate = true
		policy.BlockLoopback = true
		policy.BlockCGNAT = true
		policy.DenyCIDRs = append(policy.DenyCIDRs, extraPrivateRanges...)
	}
	return policy, nil
}

// extraPrivateRanges are non-public ranges that net.IP.IsPrivate does not
// cover: RFC 2544 benchmarking space and deprecated IPv6 site-local
// addresses, which some networks still route internally.
var extraPrivateRanges = []*net.IPNet{
	mustParseCIDR("198.18.0.0/15"),
	mustParseCIDR("fec0::/10"),
}

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic("netguard: invalid built-in CIDR " + s + ": " + err.Error())
	}
	return n
}

func normalizeSchemes(in []string) ([]string, error) {
	if len(in) == 0 {
		return []string{"http", "https"}, nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "http" && s != "https" {
			return nil, fmt.Errorf("allowed_schemes may only contain \"http\" and \"https\"")
		}
		out = append(out, s)
	}
	return out, nil
}

// Stricter returns a policy that refuses everything either a or b refuses.
// It exists so a component can introduce a new policy without loosening one
// an operator already configured: combining them can only tighten. Address
// categories and DenyCIDRs are unioned; an AllowCIDRs exception survives only
// when both policies grant it; a's host rule is kept, or b's when a has none.
// Schemes are a's alone: b is typically a legacy policy whose scheme list
// governed only redirect hops, so it says nothing about a first request.
func Stricter(a, b Policy) Policy {
	out := Policy{
		BlockPrivate:            a.BlockPrivate || b.BlockPrivate,
		BlockLoopback:           a.BlockLoopback || b.BlockLoopback,
		BlockLinkLocal:          a.BlockLinkLocal || b.BlockLinkLocal,
		BlockUnspecified:        a.BlockUnspecified || b.BlockUnspecified,
		BlockMulticastBroadcast: a.BlockMulticastBroadcast || b.BlockMulticastBroadcast,
		BlockCGNAT:              a.BlockCGNAT || b.BlockCGNAT,
		Hosts:                   a.Hosts,
	}
	if out.Hosts == nil {
		out.Hosts = b.Hosts
	}
	out.DenyCIDRs = append(append([]*net.IPNet{}, a.DenyCIDRs...), b.DenyCIDRs...)
	for _, ac := range a.AllowCIDRs {
		for _, bc := range b.AllowCIDRs {
			if ac.String() == bc.String() {
				out.AllowCIDRs = append(out.AllowCIDRs, ac)
				break
			}
		}
	}
	out.AllowedSchemes = append([]string(nil), a.AllowedSchemes...)
	return out
}

// HostRule decides which hostnames a destination may use. The nil *HostRule
// permits every hostname.
type HostRule struct {
	mode     string
	patterns []*regexp.Regexp
}

// CompileHostRule compiles hostname patterns for mode. Every pattern is an
// RE2 expression (Go's regexp package, so matching is linear-time and cannot
// backtrack) wrapped as (?i)^(?:pattern)$, so it must match the entire
// hostname: "api\.example\.com" does not match "api.example.com.evil.com".
// An empty mode means ModeDeny, and a deny rule with no patterns compiles to
// nil (permits every hostname).
func CompileHostRule(mode string, patterns []string) (*HostRule, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = ModeDeny
	}
	if mode != ModeDeny && mode != ModeAllow {
		return nil, fmt.Errorf("mode must be %q or %q", ModeDeny, ModeAllow)
	}
	if len(patterns) > MaxHostnamePatterns {
		return nil, fmt.Errorf("hostnames may hold at most %d patterns", MaxHostnamePatterns)
	}
	if mode == ModeAllow && len(patterns) == 0 {
		return nil, fmt.Errorf("mode %q requires at least one entry in hostnames", ModeAllow)
	}
	if len(patterns) == 0 {
		return nil, nil
	}

	rule := &HostRule{mode: mode, patterns: make([]*regexp.Regexp, 0, len(patterns))}
	for i, p := range patterns {
		if strings.TrimSpace(p) == "" {
			return nil, fmt.Errorf("hostnames[%d] is empty", i)
		}
		if len(p) > MaxHostnamePatternLength {
			return nil, fmt.Errorf("hostnames[%d] is longer than %d characters", i, MaxHostnamePatternLength)
		}
		re, err := regexp.Compile("(?i)^(?:" + p + ")$")
		if err != nil {
			return nil, fmt.Errorf("hostnames[%d] is not a valid regular expression: %w", i, err)
		}
		rule.patterns = append(rule.patterns, re)
	}
	return rule, nil
}

// Allows reports whether host is permitted. host may be a raw URL hostname:
// it is normalized first (see NormalizeHost), and a host that cannot be
// normalized is refused.
func (r *HostRule) Allows(host string) bool {
	if r == nil {
		return true
	}
	normalized, err := NormalizeHost(host)
	if err != nil {
		return false
	}
	matched := false
	for _, re := range r.patterns {
		if re.MatchString(normalized) {
			matched = true
			break
		}
	}
	if r.mode == ModeAllow {
		return matched
	}
	return !matched
}

// NormalizeHost returns the form of a URL hostname that host patterns are
// matched against: IPv6 brackets and trailing dots removed, lowercased, and an
// internationalized name converted to its ASCII (punycode) form. An IP literal
// is returned in its canonical text form.
//
// Trailing dots are removed after the IDNA conversion as well as before it:
// the conversion maps Unicode label separators (U+3002, U+FF0E, U+FF61) to
// ".", so "example.com。" would otherwise normalize to "example.com." and
// slip past an anchored "example\.com" pattern, while still resolving to the
// same host when dialed.
func NormalizeHost(host string) (string, error) {
	h := strings.TrimSpace(host)
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")
	h = strings.TrimRight(h, ".")
	if h == "" {
		return "", fmt.Errorf("empty host")
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.String(), nil
	}
	ascii, err := idna.Lookup.ToASCII(h)
	if err != nil {
		return "", fmt.Errorf("invalid host")
	}
	ascii = strings.TrimRight(ascii, ".")
	if ascii == "" {
		return "", fmt.Errorf("empty host")
	}
	return strings.ToLower(ascii), nil
}
