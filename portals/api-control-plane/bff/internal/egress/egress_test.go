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

package egress

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/wso2/api-platform/httpkit/netguard"
)

func mustPolicy(t *testing.T, spec Spec) *Policy {
	t.Helper()
	p, err := Parse(spec)
	if err != nil {
		t.Fatalf("Parse(%+v) = %v, want nil", spec, err)
	}
	return p
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) = %v", raw, err)
	}
	return u
}

// allows runs the resulting netguard policy against one address the same way
// a dial would. netguard exports no predicate, so this goes through the
// exported dialer's own decision indirectly via the policy's fields — which
// is why the assertions below are written against CheckTarget and the policy
// shape rather than against a raw IP verdict.
func addrIn(cidrs []*net.IPNet, raw string) bool {
	ip := net.ParseIP(raw)
	for _, c := range cidrs {
		if c.Contains(ip) {
			return true
		}
	}
	return false
}

func TestAllowCIDRsNarrowAndNothingWidens(t *testing.T) {
	// allow_cidrs becomes netguard's RequireCIDRs, which can only remove
	// addresses from the permitted set. The widening field is never set by
	// this package at all, which is what makes "a carve-out re-opened the
	// metadata endpoint" unrepresentable rather than merely rejected.
	p := mustPolicy(t, Spec{AllowCIDRs: []string{"10.42.0.0/16"}})

	if len(p.requireCIDRs) != 1 || !addrIn(p.requireCIDRs, "10.42.0.1") {
		t.Errorf("allow_cidrs should become the local require list, got %+v", p.requireCIDRs)
	}
	// netguard's AllowCIDRs is the field that widens, and it is evaluated
	// ahead of every built-in refusal. Never populating it is what keeps a
	// configuration from re-opening a refused range.
	if len(p.NetguardPolicy().AllowCIDRs) != 0 {
		t.Errorf("nothing may populate netguard's widening AllowCIDRs, got %+v", p.NetguardPolicy().AllowCIDRs)
	}
}

func TestDenyGroupsMapOntoNetguardCategories(t *testing.T) {
	// Groups exist so "private" covers IPv6 unique-local space too; a
	// hand-written CIDR list almost always forgets fc00::/7.
	for _, tc := range []struct {
		group string
		check func(n netguardFlags) bool
	}{
		{GroupPrivate, func(n netguardFlags) bool { return n.private }},
		{GroupLoopback, func(n netguardFlags) bool { return n.loopback }},
		{GroupCGNAT, func(n netguardFlags) bool { return n.cgnat }},
	} {
		t.Run(tc.group, func(t *testing.T) {
			ng := mustPolicy(t, Spec{Deny: []string{tc.group}}).NetguardPolicy()
			if !tc.check(flagsOf(ng)) {
				t.Errorf("deny = [%q] should set the matching netguard category", tc.group)
			}
		})
	}
}

func TestDenyGroupNamesAreCaseInsensitive(t *testing.T) {
	ng := mustPolicy(t, Spec{Deny: []string{"PRIVATE", " Loopback "}}).NetguardPolicy()
	if !ng.BlockPrivate || !ng.BlockLoopback {
		t.Error("group names should be matched case-insensitively and trimmed")
	}
}

func TestDenyAcceptsCIDRsAlongsideGroups(t *testing.T) {
	ng := mustPolicy(t, Spec{Deny: []string{"private", "10.42.9.0/24"}}).NetguardPolicy()
	if !ng.BlockPrivate {
		t.Error("group entry should still apply")
	}
	// Alongside the built-in translated ranges this package always denies.
	if !addrIn(ng.DenyCIDRs, "10.42.9.1") {
		t.Errorf("CIDR entry should land in DenyCIDRs, got %+v", ng.DenyCIDRs)
	}
}

func TestHazardsStayRefusedWithNoConfigurationAtAll(t *testing.T) {
	// The base policy is what makes an empty config safe. These are not
	// reachable by any setting.
	p := mustPolicy(t, Spec{})
	ng := p.NetguardPolicy()
	for name, on := range map[string]bool{
		"link-local":  ng.BlockLinkLocal,
		"unspecified": ng.BlockUnspecified,
		"multicast":   ng.BlockMulticastBroadcast,
	} {
		if !on {
			t.Errorf("%s must be refused even with no egress configuration", name)
		}
	}
	// 6to4 and NAT64 have no netguard category, so this package denies them
	// as ranges. 2002:a9fe:a9fe:: reaches 169.254.169.254 while being
	// neither link-local nor IPv4, so without these the metadata refusal
	// above is bypassable.
	for _, addr := range []string{"2002:a9fe:a9fe::", "64:ff9b::a9fe:a9fe"} {
		if !addrIn(ng.DenyCIDRs, addr) {
			t.Errorf("%s must be refused even with no egress configuration", addr)
		}
	}
	// ...and the stance categories are NOT on by default, because a gateway
	// legitimately lives on a ClusterIP.
	if ng.BlockPrivate || ng.BlockLoopback || ng.BlockCGNAT {
		t.Error("private space must stay dialable by default")
	}
}

func TestParseRefusesAnUnknownDenyEntry(t *testing.T) {
	// A typo must not silently widen the policy by being ignored.
	_, err := Parse(Spec{Deny: []string{"intranet"}})
	if err == nil {
		t.Fatal("Parse should refuse a deny entry that is neither a CIDR nor a group")
	}
	if !strings.Contains(err.Error(), "private") {
		t.Errorf("the error should list the valid group names, got %q", err)
	}
}

func TestParseRefusesADenyCoveringEverything(t *testing.T) {
	// It would disable the console behind a generic 403 rather than a clear
	// "the feature is off".
	for _, cidr := range []string{"0.0.0.0/0", "::/0"} {
		if _, err := Parse(Spec{Deny: []string{cidr}}); err == nil {
			t.Errorf("Parse should refuse deny = [%q]", cidr)
		}
	}
}

func TestParseRefusesAnAllowCIDRCoveringEverything(t *testing.T) {
	// A constraint that constrains nothing is never what an operator means.
	for _, cidr := range []string{"0.0.0.0/0", "::/0"} {
		if _, err := Parse(Spec{AllowCIDRs: []string{cidr}}); err == nil {
			t.Errorf("Parse should refuse allow_cidrs = [%q]", cidr)
		}
	}
}

func TestBothSchemesStayDialable(t *testing.T) {
	// netguard defaults AllowedSchemes to https alone, which would refuse a
	// plain-http gateway — an ordinary serving mode here.
	ng := mustPolicy(t, Spec{}).NetguardPolicy()
	if len(ng.AllowedSchemes) != 2 {
		t.Errorf("AllowedSchemes = %v, want http and https", ng.AllowedSchemes)
	}
}

// netguardFlags is a readable view of the three stance categories.
type netguardFlags struct{ private, loopback, cgnat bool }

func flagsOf(n netguard.Policy) netguardFlags {
	return netguardFlags{private: n.BlockPrivate, loopback: n.BlockLoopback, cgnat: n.BlockCGNAT}
}

func TestCheckTargetHostConstraint(t *testing.T) {
	p := mustPolicy(t, Spec{AllowHosts: []string{"gw.example.com", "*.gw.svc.cluster.local"}})

	for _, tc := range []struct {
		raw  string
		want bool // true = allowed
	}{
		{"https://gw.example.com/pizza", true},
		{"https://GW.EXAMPLE.COM/pizza", true},   // host comparison is case-insensitive
		{"https://gw.example.com./pizza", true},  // trailing root dot is the same host
		{"https://a.gw.svc.cluster.local", true}, // wildcard matches one label
		{"https://a.b.gw.svc.cluster.local", true},
		// The separating dot is part of the match, so a lookalike host that
		// merely ends with the same characters is refused. This is the
		// substring-matching bug go-cors-validation.md exists to prevent.
		{"https://evilgw.example.com", false},
		{"https://gw.example.com.evil.net", false},
		{"https://gw.svc.cluster.local", false}, // the bare suffix is not a match for "*."
		{"https://other.internal", false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			err := p.CheckTarget(mustURL(t, tc.raw))
			if tc.want && err != nil {
				t.Errorf("CheckTarget(%s) = %v, want allowed", tc.raw, err)
			}
			if !tc.want && !errors.Is(err, ErrHostNotAllowed) {
				t.Errorf("CheckTarget(%s) = %v, want ErrHostNotAllowed", tc.raw, err)
			}
		})
	}
}

func TestCheckTargetPortConstraint(t *testing.T) {
	// Port is the only dimension separating a gateway from any other
	// in-cluster HTTP service, because the stance has to permit private space.
	p := mustPolicy(t, Spec{AllowPorts: []int{443, 9443}})

	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{"https://gw.internal:9443/x", true},
		{"https://gw.internal/x", true}, // https defaults to 443, which is listed
		{"https://gw.internal:9200/x", false},
		// Port 0 is not a dialable port. It used to be refused only because
		// allow_ports can never contain it; targetPort now rejects it outright.
		{"https://gw.internal:0/x", false},
		{"http://gw.internal/x", false}, // http defaults to 80, which is not
	} {
		t.Run(tc.raw, func(t *testing.T) {
			err := p.CheckTarget(mustURL(t, tc.raw))
			if tc.want && err != nil {
				t.Errorf("CheckTarget(%s) = %v, want allowed", tc.raw, err)
			}
			if !tc.want && !errors.Is(err, ErrPortNotAllowed) {
				t.Errorf("CheckTarget(%s) = %v, want ErrPortNotAllowed", tc.raw, err)
			}
		})
	}
}

func TestCheckTargetIsOpenWhenNothingIsConstrained(t *testing.T) {
	// An empty list means "no constraint", not "allow nothing" — otherwise
	// the shipped default would refuse every gateway.
	p := mustPolicy(t, Spec{})
	if err := p.CheckTarget(mustURL(t, "https://anything.example.com:8443/x")); err != nil {
		t.Errorf("CheckTarget() = %v, want nil with no host/port constraints", err)
	}
}

func TestCheckTargetRejectsAHostlessURL(t *testing.T) {
	if err := mustPolicy(t, Spec{}).CheckTarget(mustURL(t, "/relative")); !errors.Is(err, ErrNoTargetHost) {
		t.Errorf("CheckTarget(relative) = %v, want ErrNoTargetHost", err)
	}
	if err := mustPolicy(t, Spec{}).CheckTarget(nil); !errors.Is(err, ErrNoTargetHost) {
		t.Errorf("CheckTarget(nil) = %v, want ErrNoTargetHost", err)
	}
}

func TestParseRefusesOverBroadHostPatterns(t *testing.T) {
	for _, host := range []string{
		"*",                // matches everything
		"*.com",            // one label short of meaning anything
		"gw.*.example.com", // interior wildcard
		"*.gw.*.com",       // more than one
		"",                 // empty entry
	} {
		t.Run(host, func(t *testing.T) {
			if _, err := Parse(Spec{AllowHosts: []string{host}}); err == nil {
				t.Errorf("Parse should refuse allow_hosts entry %q", host)
			}
		})
	}
}

func TestParseRefusesInvalidPortsAndCIDRs(t *testing.T) {
	for name, spec := range map[string]Spec{
		"port zero":       {AllowPorts: []int{0}},
		"port too large":  {AllowPorts: []int{65536}},
		"negative port":   {AllowPorts: []int{-1}},
		"bare ip as cidr": {Deny: []string{"10.0.0.1"}},
		"nonsense cidr":   {AllowCIDRs: []string{"nonsense"}},
		"empty deny":      {Deny: []string{"  "}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(spec); err == nil {
				t.Errorf("Parse(%+v) should refuse this", spec)
			}
		})
	}
}

func TestDescribeNamesTheEffectivePolicy(t *testing.T) {
	// The startup line is how an operator confirms what is in force without
	// re-reading config.toml, so it has to mention the carve-out.
	p := mustPolicy(t, Spec{
		AllowHosts: []string{"gw.example.com"},
		AllowPorts: []int{443},
		Deny:       []string{"private", "10.0.7.0/24"},
	})
	got := p.Describe()
	for _, want := range []string{"allow_hosts=1", "allow_ports=1", "deny_cidrs=1", "deny_groups=private"} {
		if !strings.Contains(got, want) {
			t.Errorf("Describe() = %q, want it to mention %q", got, want)
		}
	}
}
