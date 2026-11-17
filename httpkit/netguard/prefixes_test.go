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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package netguard

import (
	"math/rand"
	"net"
	"net/netip"
	"testing"
)

func cidrOf(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func inPrefixes(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// probeAddresses are the edges of every range the policies below use, the embedded and mapped
// forms of IPv4 addresses, and random addresses of both families.
func probeAddresses() []netip.Addr {
	edges := []string{
		"0.0.0.0", "0.0.0.1", "9.255.255.255", "10.0.0.0", "10.255.255.255", "11.0.0.0",
		"100.63.255.255", "100.64.0.0", "100.127.255.255", "100.128.0.0",
		"126.255.255.255", "127.0.0.1", "128.0.0.0", "169.253.255.255", "169.254.0.0", "169.254.169.254",
		"169.255.0.0", "172.15.255.255", "172.16.0.0", "172.31.255.255", "172.32.0.0",
		"192.167.255.255", "192.168.0.0", "192.168.1.10", "192.168.255.255", "192.169.0.0",
		"198.17.255.255", "198.18.0.0", "198.19.255.255", "198.20.0.0",
		"203.0.113.7", "203.0.113.8", "223.255.255.255", "224.0.0.0", "224.0.0.255", "224.0.1.0",
		"239.255.255.255", "240.0.0.0", "255.255.255.254", "255.255.255.255", "93.184.216.34",
		"::", "::1", "::2", "fc00::", "fdff:ffff::1", "fe00::", "fe80::1", "febf::1", "fec0::1", "feff::1",
		"ff02::1", "ff12::1", "ff05::1", "fd00:ec2::254", "fd00:ec2::253", "2001:db8::1", "2606:4700::1",
	}
	var out []netip.Addr
	for _, e := range edges {
		addr := netip.MustParseAddr(e)
		out = append(out, addr)
		if addr.Is4() {
			v4 := addr.As4()
			out = append(out,
				netip.AddrFrom16([16]byte{10: 0xff, 11: 0xff, 12: v4[0], 13: v4[1], 14: v4[2], 15: v4[3]}),
				netip.AddrFrom16([16]byte{0: 0x20, 1: 0x02, 2: v4[0], 3: v4[1], 4: v4[2], 5: v4[3]}),
				netip.AddrFrom16([16]byte{1: 0x64, 2: 0xff, 3: 0x9b, 12: v4[0], 13: v4[1], 14: v4[2], 15: v4[3]}),
			)
		}
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		var b4 [4]byte
		var b16 [16]byte
		r.Read(b4[:])
		r.Read(b16[:])
		out = append(out, netip.AddrFrom4(b4), netip.AddrFrom16(b16))
	}
	return out
}

// DeniedPrefixes must refuse exactly the addresses the policy refuses, so a proxy filtering
// resolved addresses by range enforces the same rules as a guarded dial.
func TestDeniedPrefixes_MatchesThePolicy(t *testing.T) {
	deny, err := GuardConfig{Enabled: true}.Build()
	if err != nil {
		t.Fatal(err)
	}
	private, err := GuardConfig{Enabled: true, DenyPrivateNetworks: true}.Build()
	if err != nil {
		t.Fatal(err)
	}
	carved := private
	carved.DenyCIDRs = []*net.IPNet{cidrOf(t, "203.0.113.0/24"), cidrOf(t, "2001:db8::/32")}
	carved.AllowCIDRs = []*net.IPNet{cidrOf(t, "10.20.0.0/16"), cidrOf(t, "192.168.1.10/32"),
		cidrOf(t, "203.0.113.8/29"), cidrOf(t, "fd00:1::/32")}

	for name, policy := range map[string]Policy{
		"default access control":       deny,
		"private networks denied":      private,
		"deny and allow CIDRs":         carved,
		"loopback only":                {BlockLoopback: true},
		"allow carve-out covers a /8":  {BlockPrivate: true, AllowCIDRs: []*net.IPNet{cidrOf(t, "10.0.0.0/8")}},
		"IPv4 network in 16-byte form": {DenyCIDRs: []*net.IPNet{{IP: net.ParseIP("198.51.100.0"), Mask: net.CIDRMask(120, 128)}}},
	} {
		t.Run(name, func(t *testing.T) {
			prefixes := policy.DeniedPrefixes()
			for _, addr := range probeAddresses() {
				refused := !policy.allowed(net.IP(addr.AsSlice()))
				if got := inPrefixes(prefixes, addr); got != refused {
					t.Fatalf("%s: in denied prefixes = %v, policy refuses = %v", addr, got, refused)
				}
			}
		})
	}
}

func TestDeniedPrefixes_NothingDeniedIsEmpty(t *testing.T) {
	if got := (Policy{}).DeniedPrefixes(); len(got) != 0 {
		t.Fatalf("a policy without categories or CIDRs has no ranges, got %v", got)
	}
}
