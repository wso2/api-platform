/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
'use strict';

const ipaddr = require('ipaddr.js');

// The allowed_clients entry that lets every client through.
const ALLOW_ALL = '*';

// The allowed set used when none is configured.
const LOOPBACK = [ipaddr.parseCIDR('127.0.0.1/32'), ipaddr.parseCIDR('::1/128')];

/**
 * Parses one address the way the Go components do: IPv4 only in dotted-quad form
 * (ipaddr.js would also take octal, hex and shortened forms), IPv6 without a zone.
 */
function parseAddress(text) {
    if (ipaddr.IPv4.isValidFourPartDecimal(text)) {
        return ipaddr.IPv4.parse(text);
    }
    if (!text.includes('%') && ipaddr.IPv6.isValid(text)) {
        return ipaddr.IPv6.parse(text);
    }
    throw new Error('invalid address');
}

/**
 * Parses a CIDR range, or a single address as a one-address range. IPv4-mapped IPv6
 * forms are reduced to IPv4 so they match the unmapped client address.
 */
function parseEntry(entry) {
    const slash = entry.indexOf('/');
    if (slash === -1) {
        const addr = unmap(parseAddress(entry));
        return [addr, addr.kind() === 'ipv4' ? 32 : 128];
    }
    const addr = parseAddress(entry.slice(0, slash));
    const bitsText = entry.slice(slash + 1);
    const maxBits = addr.kind() === 'ipv4' ? 32 : 128;
    if (!/^\d{1,3}$/.test(bitsText) || Number(bitsText) > maxBits) {
        throw new Error('invalid prefix length');
    }
    const bits = Number(bitsText);
    if (addr.kind() === 'ipv6' && addr.isIPv4MappedAddress()) {
        if (bits < 96) {
            throw new Error('prefix is wider than the IPv4-mapped range');
        }
        return [addr.toIPv4Address(), bits - 96];
    }
    return [addr, bits];
}

function unmap(addr) {
    return addr.kind() === 'ipv6' && addr.isIPv4MappedAddress() ? addr.toIPv4Address() : addr;
}

/**
 * Builds the set of clients that may call the health endpoints from configured entries,
 * each an IP address or a CIDR range. No entries means loopback only; "*" as the only
 * entry allows every client. `section` names the config section in error messages,
 * which match the Go components' wording.
 *
 * @returns {{ allowsAll: boolean, allows: (address: string) => boolean }}
 */
function parseAllowedClients(section, entries) {
    const list = entries || [];
    if (list.length === 0) {
        return createSet(false, LOOPBACK);
    }
    const ranges = [];
    for (let i = 0; i < list.length; i++) {
        const entry = String(list[i]).trim();
        if (entry === ALLOW_ALL) {
            if (list.length > 1) {
                throw new Error(`${section}.allowed_clients: "${ALLOW_ALL}" cannot be combined with other entries`);
            }
            return createSet(true, []);
        }
        try {
            ranges.push(parseEntry(entry));
        } catch (err) {
            throw new Error(`${section}.allowed_clients[${i}] is not a valid IP address or CIDR range`, { cause: err });
        }
    }
    return createSet(false, ranges);
}

function createSet(allowsAll, ranges) {
    return {
        allowsAll,
        /**
         * Reports whether the peer at `address` (a socket's remoteAddress) is in the set.
         * Only the connection's own peer address may be passed here; forwarding headers
         * are never consulted. An IPv6 zone is ignored, and an address that cannot be
         * parsed is refused.
         */
        allows(address) {
            if (allowsAll) return true;
            if (typeof address !== 'string' || address === '') return false;
            let addr;
            try {
                addr = unmap(ipaddr.parse(address.split('%')[0]));
            } catch (_) {
                return false;
            }
            return ranges.some(([range, bits]) => range.kind() === addr.kind() && addr.match(range, bits));
        },
    };
}

module.exports = { parseAllowedClients };
