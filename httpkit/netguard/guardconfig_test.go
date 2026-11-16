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
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func mustBuild(t *testing.T, c GuardConfig) Policy {
	t.Helper()
	p, err := c.Build()
	if err != nil {
		t.Fatalf("Build(%+v): %v", c, err)
	}
	return p
}

func TestGuardConfigBuild_RejectsInvalidConfiguration(t *testing.T) {
	tooMany := make([]string, MaxHostnamePatterns+1)
	for i := range tooMany {
		tooMany[i] = "a"
	}
	tests := []struct {
		name string
		cfg  GuardConfig
		want string
	}{
		{"unknown mode", GuardConfig{Enabled: true, Mode: "block"}, "mode must be"},
		{"allow mode with no patterns", GuardConfig{Enabled: true, Mode: ModeAllow}, "requires at least one entry"},
		{"invalid regex", GuardConfig{Enabled: true, Hostnames: []string{"api.(example"}}, "not a valid regular expression"},
		{"empty pattern", GuardConfig{Enabled: true, Hostnames: []string{"  "}}, "is empty"},
		{"oversized pattern", GuardConfig{Enabled: true, Hostnames: []string{strings.Repeat("a", MaxHostnamePatternLength+1)}}, "longer than"},
		{"too many patterns", GuardConfig{Enabled: true, Hostnames: tooMany}, "at most"},
		{"negative redirects", GuardConfig{Enabled: true, MaxRedirects: -1}, "max_redirects"},
		{"non-http scheme", GuardConfig{Enabled: true, AllowedSchemes: []string{"ftp"}}, "allowed_schemes"},
		{"lookaround is not RE2", GuardConfig{Enabled: true, Hostnames: []string{"(?!internal).*"}}, "not a valid regular expression"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.cfg.Build()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Build() error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestGuardConfigBuild_DisabledIsZeroAndAlwaysValid(t *testing.T) {
	p, err := GuardConfig{Enabled: false, Mode: "bogus", Hostnames: []string{"("}}.Build()
	if err != nil {
		t.Fatalf("a disabled configuration must not fail: %v", err)
	}
	if p.Hosts != nil || p.BlockLinkLocal {
		t.Fatalf("a disabled configuration must build the zero Policy, got %+v", p)
	}
}

func TestGuardConfigBuild_Defaults(t *testing.T) {
	p := mustBuild(t, GuardConfig{Enabled: true})
	if !p.schemeAllowed("http") || !p.schemeAllowed("https") || p.schemeAllowed("ftp") {
		t.Fatalf("default schemes = %v, want http and https", p.AllowedSchemes)
	}
	if p.Hosts != nil {
		t.Fatal("deny mode with no patterns should not build a host rule")
	}
}

func TestHostRule(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		patterns []string
		host     string
		want     bool
	}{
		{"deny refuses a match", ModeDeny, []string{`internal\.example\.com`}, "internal.example.com", false},
		{"deny permits a non-match", ModeDeny, []string{`internal\.example\.com`}, "api.example.com", true},
		{"allow permits a match", ModeAllow, []string{`api\.example\.com`}, "api.example.com", true},
		{"allow refuses a non-match", ModeAllow, []string{`api\.example\.com`}, "other.example.com", false},
		{"anchored: no suffix extension", ModeAllow, []string{`api\.example\.com`}, "api.example.com.evil.com", false},
		{"anchored: no prefix extension", ModeAllow, []string{`example\.com`}, "evil-example.com", false},
		{"unescaped dot stays within the hostname", ModeAllow, []string{`api.example.com`}, "apiXexample.com", true},
		{"wildcard subdomain", ModeAllow, []string{`[a-z0-9-]+\.svc\.cluster\.local`}, "orders.svc.cluster.local", true},
		{"wildcard subdomain does not cross labels", ModeAllow, []string{`[a-z0-9-]+\.svc\.cluster\.local`}, "a.b.svc.cluster.local", false},
		{"case-insensitive", ModeAllow, []string{`api\.example\.com`}, "API.Example.COM", true},
		{"trailing dot normalized", ModeAllow, []string{`api\.example\.com`}, "api.example.com.", true},
		{"repeated trailing dots normalized", ModeDeny, []string{`example\.com`}, "example.com..", false},
		{"deny: ideographic full stop separator", ModeDeny, []string{`example\.com`}, "example.com\u3002", false},
		{"deny: fullwidth full stop separator", ModeDeny, []string{`example\.com`}, "example.com\uff0e", false},
		{"deny: halfwidth ideographic full stop separator", ModeDeny, []string{`example\.com`}, "example.com\uff61", false},
		{"allow: ideographic full stop separator", ModeAllow, []string{`example\.com`}, "example.com\u3002", true},
		{"allow: unicode separator does not widen", ModeAllow, []string{`example\.com`}, "evil.example.com\u3002", false},
		{"only dots is refused", ModeDeny, []string{`x`}, "..", false},
		{"IPv6 literal brackets stripped", ModeDeny, []string{`::1`}, "[::1]", false},
		{"IPv4 literal matched as text", ModeAllow, []string{`10\.0\.0\.5`}, "10.0.0.5", true},
		{"IDN matched as punycode", ModeAllow, []string{`xn--bcher-kva\.example`}, "bücher.example", true},
		{"empty host refused in deny mode", ModeDeny, []string{`x`}, "", false},
		{"alternation is grouped before anchoring", ModeAllow, []string{`a\.com|b\.com`}, "b.com.evil", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := CompileHostRule(tt.mode, tt.patterns)
			if err != nil {
				t.Fatalf("CompileHostRule: %v", err)
			}
			if got := rule.Allows(tt.host); got != tt.want {
				t.Fatalf("Allows(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestGuardConfigBuild_AddressCategories(t *testing.T) {
	permit := mustBuild(t, GuardConfig{Enabled: true})
	deny := mustBuild(t, GuardConfig{Enabled: true, DenyPrivateNetworks: true})

	tests := []struct {
		ip          string
		wantPermit  bool // deny_private_networks = false
		wantDenyPri bool // deny_private_networks = true
	}{
		{"8.8.8.8", true, true},
		{"2001:4860:4860::8888", true, true},
		{"10.0.0.5", true, false},
		{"172.16.3.4", true, false},
		{"192.168.1.1", true, false},
		{"127.0.0.1", true, false},
		{"::1", true, false},
		{"100.64.0.1", true, false},
		{"198.18.0.1", true, false},
		{"fd00::1", true, false},
		{"fec0::1", true, false},
		{"::ffff:10.0.0.5", true, false},
		// Always refused, whatever deny_private_networks says.
		{"169.254.169.254", false, false},
		{"fd00:ec2::254", false, false}, // EC2 IPv6 metadata
		{"fe80::1", false, false},
		{"0.0.0.0", false, false},
		{"::", false, false},
		{"224.0.0.1", false, false},
		{"255.255.255.255", false, false},
		{"::ffff:169.254.169.254", false, false},
		// 6to4 and NAT64 forms are judged by the IPv4 address they carry.
		{"2002:a9fe:a9fe::", false, false},   // 6to4 of 169.254.169.254
		{"2002:0a00:0005::", true, false},    // 6to4 of 10.0.0.5
		{"2002:0808:0808::", true, true},     // 6to4 of 8.8.8.8
		{"64:ff9b::a9fe:a9fe", false, false}, // NAT64 of 169.254.169.254
		{"64:ff9b::7f00:1", true, false},     // NAT64 of 127.0.0.1
		{"64:ff9b::808:808", true, true},     // NAT64 of 8.8.8.8
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", tt.ip)
		}
		if got := permit.allowed(ip); got != tt.wantPermit {
			t.Errorf("deny_private_networks=false: allowed(%s) = %v, want %v", tt.ip, got, tt.wantPermit)
		}
		if got := deny.allowed(ip); got != tt.wantDenyPri {
			t.Errorf("deny_private_networks=true: allowed(%s) = %v, want %v", tt.ip, got, tt.wantDenyPri)
		}
	}
}

// TestGuardConfigBuild_AllowModeStillAppliesAddressRules proves that in allow
// mode a listed hostname must ALSO resolve to an allowed address: the host
// rule never exempts a destination from deny_private_networks.
func TestGuardConfigBuild_AllowModeStillAppliesAddressRules(t *testing.T) {
	p := mustBuild(t, GuardConfig{Enabled: true, Mode: ModeAllow, Hostnames: []string{"localhost"}, DenyPrivateNetworks: true})
	err := Validate(context.Background(), p, "localhost")
	if !errors.Is(err, ErrDestinationNotAllowed) {
		t.Fatalf("Validate(localhost) = %v, want ErrDestinationNotAllowed", err)
	}

	permit := mustBuild(t, GuardConfig{Enabled: true, Mode: ModeAllow, Hostnames: []string{"localhost"}})
	if err := Validate(context.Background(), permit, "localhost"); err != nil {
		t.Fatalf("positive control: Validate(localhost) with private networks permitted = %v", err)
	}
}

func TestCheckURL(t *testing.T) {
	ctx := context.Background()
	p := mustBuild(t, GuardConfig{Enabled: true, Hostnames: []string{`blocked\.example`}, AllowedSchemes: []string{"https"}})

	refusedCases := []string{
		"http://api.example/",            // scheme
		"https://blocked.example/x",      // host rule
		"https://BLOCKED.example./x",     // host rule after normalization
		"https://169.254.169.254/latest", // literal metadata address
		"https://[fe80::1]/",             // literal link-local
		"https://169。254。169。254/",       // literal metadata address with ideographic full stops
		"https://１６９．２５４．１６９．２５４/",       // and in full-width digits and stops
	}
	for _, raw := range refusedCases {
		if err := p.CheckURL(ctx, raw, false); !errors.Is(err, ErrDestinationNotAllowed) {
			t.Errorf("CheckURL(%q) = %v, want ErrDestinationNotAllowed", raw, err)
		}
	}

	// Without resolution an unresolvable hostname is not refused: the URL
	// may be dialed from another network.
	if err := p.CheckURL(ctx, "https://backend.invalid/", false); err != nil {
		t.Errorf("CheckURL without resolution = %v, want nil", err)
	}
	if err := p.CheckURL(ctx, "https://10.0.0.5:8443/api", false); err != nil {
		t.Errorf("positive control: private literal with private networks permitted = %v", err)
	}
	if err := p.CheckURL(ctx, "https://10。0。0。5/api", false); err != nil {
		t.Errorf("positive control: a permitted literal with ideographic full stops = %v", err)
	}

	// A malformed URL is an error but not a refusal.
	if err := p.CheckURL(ctx, "://nohost", false); err == nil || errors.Is(err, ErrDestinationNotAllowed) {
		t.Errorf("CheckURL(malformed) = %v, want a non-refusal error", err)
	}
}

func TestRefusalsMatchSentinel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	p := mustBuild(t, GuardConfig{Enabled: true, DenyPrivateNetworks: true})
	_, err = DialContext(p, time.Second)(context.Background(), "tcp", ln.Addr().String())
	if !errors.Is(err, ErrDestinationNotAllowed) {
		t.Fatalf("DialContext refusal = %v, want ErrDestinationNotAllowed", err)
	}
	if strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("refusal must not name the address: %v", err)
	}

	// A connection failure is not a refusal.
	permit := mustBuild(t, GuardConfig{Enabled: true})
	ln.Close()
	if _, err := DialContext(permit, time.Second)(context.Background(), "tcp", ln.Addr().String()); err == nil || errors.Is(err, ErrDestinationNotAllowed) {
		t.Fatalf("dial to a closed port = %v, want a non-refusal error", err)
	}
}

func TestCheckRedirect_AppliesHostRule(t *testing.T) {
	p := mustBuild(t, GuardConfig{Enabled: true, Mode: ModeAllow, Hostnames: []string{`api\.example\.com`}})
	check := CheckRedirect(p, 5)

	allowed, _ := http.NewRequest(http.MethodGet, "https://api.example.com/start", nil)
	next, _ := http.NewRequest(http.MethodGet, "https://api.example.com/next", nil)
	if err := check(next, []*http.Request{allowed}); err != nil {
		t.Fatalf("same-host hop to an allowed host = %v", err)
	}

	// A chain that started on an unlisted host (e.g. reached through a
	// client built without the round-tripper) is still refused per hop.
	unlisted, _ := http.NewRequest(http.MethodGet, "https://other.example.com/start", nil)
	hop, _ := http.NewRequest(http.MethodGet, "https://other.example.com/next", nil)
	if err := check(hop, []*http.Request{unlisted}); !errors.Is(err, ErrDestinationNotAllowed) {
		t.Fatalf("hop to an unlisted host = %v, want ErrDestinationNotAllowed", err)
	}
}

func TestProxyHop_DropsHostRuleAndPrivateCategories(t *testing.T) {
	p := mustBuild(t, GuardConfig{Enabled: true, Mode: ModeAllow, Hostnames: []string{"x"}, DenyPrivateNetworks: true})
	hop := p.ProxyHop()
	if hop.Hosts != nil || hop.BlockPrivate || hop.BlockLoopback || hop.BlockCGNAT || len(hop.DenyCIDRs) != 0 {
		t.Fatalf("ProxyHop kept tenant-destination rules: %+v", hop)
	}
	if hop.allowed(net.ParseIP("169.254.169.254")) {
		t.Fatal("ProxyHop must still refuse the metadata address")
	}
	if !hop.allowed(net.ParseIP("10.0.0.1")) {
		t.Fatal("ProxyHop must permit a private proxy address")
	}
}

func TestStricter_OnlyTightens(t *testing.T) {
	newPolicy := mustBuild(t, GuardConfig{Enabled: true, Hostnames: []string{`blocked\.example`}})
	merged := Stricter(newPolicy, PublicOnly())

	if merged.allowed(net.ParseIP("10.0.0.5")) {
		t.Fatal("a legacy public_only policy must keep refusing private addresses")
	}
	if !merged.allowed(net.ParseIP("8.8.8.8")) {
		t.Fatal("positive control: a public address stays allowed")
	}
	if merged.CheckHost("blocked.example") == nil {
		t.Fatal("the new policy's host rule must survive the merge")
	}
	// Schemes come from the new policy only.
	if !merged.schemeAllowed("http") || !merged.schemeAllowed("https") {
		t.Fatalf("schemes = %v, want the new policy's http and https", merged.AllowedSchemes)
	}

	permissive := Stricter(newPolicy, PermitPrivateBlockMetadata())
	if !permissive.allowed(net.ParseIP("10.0.0.5")) {
		t.Fatal("merging with permit_private must not refuse private addresses")
	}

}
