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

package health

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// allowAllClients is the allowed-clients entry that admits every client. It must be the
// only entry.
const allowAllClients = "*"

// AllowedClients is the set of client addresses allowed to read the health answer. It is
// immutable once parsed. A nil *AllowedClients allows every client.
type AllowedClients struct {
	prefixes []netip.Prefix
}

// ParseAllowedClients parses the allowed clients configured in a health section. Each
// entry is an IP address or a CIDR range. No entries means loopback only (127.0.0.1 and
// ::1). "*" as the only entry allows every client and is returned as nil. section names
// the configuration section in error messages.
func ParseAllowedClients(section string, entries []string) (*AllowedClients, error) {
	if len(entries) == 0 {
		return &AllowedClients{prefixes: []netip.Prefix{
			netip.PrefixFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 32),
			netip.PrefixFrom(netip.IPv6Loopback(), 128),
		}}, nil
	}
	prefixes := make([]netip.Prefix, 0, len(entries))
	for i, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == allowAllClients {
			if len(entries) != 1 {
				return nil, fmt.Errorf("%s.allowed_clients: %q cannot be combined with other entries",
					section, allowAllClients)
			}
			return nil, nil
		}
		prefix, ok := parseClientEntry(entry)
		if !ok {
			return nil, fmt.Errorf("%s.allowed_clients[%d] is not a valid IP address or CIDR range", section, i)
		}
		prefixes = append(prefixes, prefix)
	}
	return &AllowedClients{prefixes: prefixes}, nil
}

// parseClientEntry parses one entry as a CIDR range or a single address, and returns it
// as a masked prefix with any IPv4-mapped IPv6 form turned into its IPv4 form. Zoned
// addresses are rejected.
func parseClientEntry(entry string) (netip.Prefix, bool) {
	if strings.Contains(entry, "/") {
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			return netip.Prefix{}, false
		}
		return unmapPrefix(prefix.Masked()), true
	}
	addr, err := netip.ParseAddr(entry)
	if err != nil || addr.Zone() != "" {
		return netip.Prefix{}, false
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), true
}

// unmapPrefix turns an IPv4-mapped IPv6 prefix that lies within the mapped range into the
// equivalent IPv4 prefix, so it matches unmapped client addresses. Any other prefix is
// returned unchanged.
func unmapPrefix(p netip.Prefix) netip.Prefix {
	const mappedBits = 96
	if !p.Addr().Is4In6() || p.Bits() < mappedBits {
		return p
	}
	return netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-mappedBits)
}

// Allows reports whether the client at remoteAddr, a host:port as in
// http.Request.RemoteAddr, is allowed. Forwarding headers are never consulted. An address
// that cannot be parsed is refused. A nil receiver allows every client.
func (a *AllowedClients) Allows(remoteAddr string) bool {
	if a == nil {
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
	addr = addr.WithZone("").Unmap()
	for _, p := range a.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// Prefixes returns a copy of the allowed ranges. It returns nil for a nil receiver, which
// allows every client.
func (a *AllowedClients) Prefixes() []netip.Prefix {
	if a == nil {
		return nil
	}
	return append([]netip.Prefix(nil), a.prefixes...)
}
