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

package health

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// allowAll is the allowed_clients entry that lets every client through.
const allowAll = "*"

// loopback is the allowed set used when none is configured.
var loopback = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.1/32"),
	netip.MustParsePrefix("::1/128"),
}

// AllowedClients is the set of client addresses that may call the health endpoints. A
// nil set allows every client.
type AllowedClients struct {
	all      bool
	prefixes []netip.Prefix
}

// ParseAllowedClients builds the allowed set from configured entries, each an IP
// address or a CIDR range. No entries means loopback only; "*" as the only entry allows
// every client. section names the config section in error messages.
func ParseAllowedClients(section string, entries []string) (*AllowedClients, error) {
	if len(entries) == 0 {
		return &AllowedClients{prefixes: loopback}, nil
	}
	set := &AllowedClients{}
	for i, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == allowAll {
			if len(entries) > 1 {
				return nil, fmt.Errorf("%s.allowed_clients: %q cannot be combined with other entries", section, allowAll)
			}
			return &AllowedClients{all: true}, nil
		}
		prefix, err := parseClientEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("%s.allowed_clients[%d] is not a valid IP address or CIDR range", section, i)
		}
		set.prefixes = append(set.prefixes, prefix)
	}
	return set, nil
}

// parseClientEntry parses a CIDR range, or a single address as a one-address range.
// IPv4-mapped IPv6 forms are reduced to IPv4 so they match the unmapped client address.
func parseClientEntry(entry string) (netip.Prefix, error) {
	if strings.Contains(entry, "/") {
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			return netip.Prefix{}, err
		}
		addr := prefix.Addr()
		if addr.Is4In6() {
			bits := prefix.Bits() - 96
			if bits < 0 {
				return netip.Prefix{}, fmt.Errorf("prefix %s is wider than the IPv4-mapped range", entry)
			}
			return netip.PrefixFrom(addr.Unmap(), bits).Masked(), nil
		}
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, err
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// Allows reports whether the client at remoteAddr, a request's RemoteAddr in host:port
// form, is in the set. Only the connection's peer address counts; forwarding headers
// are never consulted. An IPv6 zone is ignored, and an address that cannot be parsed is
// refused.
func (a *AllowedClients) Allows(remoteAddr string) bool {
	if a == nil || a.all {
		return true
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap().WithZone("")
	for _, p := range a.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
