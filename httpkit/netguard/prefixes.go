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
	"net"
	"net/netip"
)

// DeniedPrefixes lists the address ranges the policy refuses, for a component that filters
// addresses by range rather than by calling the policy, such as a proxy that drops resolved
// addresses in a list of CIDR ranges. An address is refused by the policy exactly when it falls in
// one of the returned prefixes:
//
//   - the Block* categories and DenyCIDRs, with the AllowCIDRs carve-outs subtracted;
//   - the same IPv4 ranges as IPv4-mapped IPv6 addresses (::ffff:a.b.c.d), which a range matcher
//     treats as IPv6 while the policy treats them as IPv4;
//   - the same IPv4 ranges embedded in 6to4 (2002::/16) and NAT64 (64:ff9b::/96) addresses, which
//     the policy refuses whatever AllowCIDRs says about the IPv6 address itself.
//
// The zero Policy refuses every address and has no range form; it returns nil, as does a policy
// that refuses nothing.
func (p Policy) DeniedPrefixes() []netip.Prefix {
	var deny []netip.Prefix
	add := func(cidrs ...string) {
		for _, c := range cidrs {
			deny = append(deny, netip.MustParsePrefix(c))
		}
	}
	if p.BlockLoopback {
		add("127.0.0.0/8", "::1/128")
	}
	if p.BlockPrivate {
		add("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7")
	}
	if p.BlockLinkLocal {
		add("169.254.0.0/16", "fe80::/10", "224.0.0.0/24", "fd00:ec2::254/128")
		// net.IP.IsLinkLocalMulticast: ff02::/16 under every flag value, ffX2::/16.
		for flags := 0; flags < 16; flags++ {
			deny = append(deny, netip.PrefixFrom(netip.AddrFrom16([16]byte{0xff, byte(flags<<4 | 0x02)}), 16))
		}
	}
	if p.BlockUnspecified {
		add("0.0.0.0/32", "::/128")
	}
	if p.BlockMulticastBroadcast {
		add("224.0.0.0/4", "ff00::/8", "255.255.255.255/32")
	}
	if p.BlockCGNAT {
		add("100.64.0.0/10")
	}
	for _, cidr := range p.DenyCIDRs {
		if prefix, ok := prefixOf(cidr); ok {
			deny = append(deny, prefix)
		}
	}

	var holes []netip.Prefix
	for _, cidr := range p.AllowCIDRs {
		if prefix, ok := prefixOf(cidr); ok {
			holes = append(holes, prefix)
		}
	}

	var out []netip.Prefix
	for _, prefix := range deny {
		for _, kept := range subtractPrefixes(prefix, holes) {
			out = append(out, kept)
			if kept.Addr().Is4() {
				v4 := kept.Addr().As4()
				out = append(out,
					embedIPv4([16]byte{10: 0xff, 11: 0xff}, 12, v4, kept.Bits()),        // ::ffff:0:0/96
					embedIPv4([16]byte{0: 0x20, 1: 0x02}, 2, v4, kept.Bits()),           // 2002::/16
					embedIPv4([16]byte{1: 0x64, 2: 0xff, 3: 0x9b}, 12, v4, kept.Bits()), // 64:ff9b::/96
				)
			}
		}
	}
	return out
}

// prefixOf converts a net.IPNet into a netip.Prefix, with IPv4 networks in their 4-byte form.
func prefixOf(cidr *net.IPNet) (netip.Prefix, bool) {
	if cidr == nil {
		return netip.Prefix{}, false
	}
	addr, ok := netip.AddrFromSlice(cidr.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	ones, _ := cidr.Mask.Size()
	addr = addr.Unmap()
	if addr.Is4() && ones > 32 {
		ones -= 96
	}
	return netip.PrefixFrom(addr, ones).Masked(), true
}

// embedIPv4 places an IPv4 prefix at byte offset at inside an IPv6 base address.
func embedIPv4(base [16]byte, at int, v4 [4]byte, bits int) netip.Prefix {
	copy(base[at:at+4], v4[:])
	return netip.PrefixFrom(netip.AddrFrom16(base), at*8+bits).Masked()
}

// subtractPrefixes returns prefix without the addresses in holes, as the fewest prefixes the
// halving split yields.
func subtractPrefixes(prefix netip.Prefix, holes []netip.Prefix) []netip.Prefix {
	for _, hole := range holes {
		if hole.Addr().Is4() != prefix.Addr().Is4() || !prefix.Overlaps(hole) {
			continue
		}
		if hole.Bits() <= prefix.Bits() {
			return nil // the hole covers the whole prefix
		}
		low, high := splitPrefix(prefix)
		return append(subtractPrefixes(low, holes), subtractPrefixes(high, holes)...)
	}
	return []netip.Prefix{prefix}
}

// splitPrefix halves a prefix into its two prefixes one bit longer.
func splitPrefix(prefix netip.Prefix) (netip.Prefix, netip.Prefix) {
	bits := prefix.Bits() + 1
	low := netip.PrefixFrom(prefix.Addr(), bits).Masked()
	raw := low.Addr().As16()
	offset := 0
	if low.Addr().Is4() {
		offset = 96
	}
	bit := offset + bits - 1
	raw[bit/8] |= 0x80 >> (bit % 8)
	high := netip.AddrFrom16(raw)
	if low.Addr().Is4() {
		high = high.Unmap()
	}
	return low, netip.PrefixFrom(high, bits)
}
