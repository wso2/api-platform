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

/*
 * SSRF guard for outbound requests whose destination is influenced by a
 * browser-supplied value (currently the API try-it proxy, see
 * src/services/tryoutProxyService.js).
 *
 * Two checks are needed, and callers must apply both:
 *
 *   1. createGuardedLookup — a `lookup` hook handed to the http/https Agent, so
 *      the check fires at the moment the socket resolves the hostname rather
 *      than once at input-validation time. That is what closes the DNS-rebinding
 *      window: a hostname that resolved to a public address during validation
 *      cannot re-resolve to 169.254.169.254 for the actual connection without
 *      this hook seeing it.
 *   2. assertAllowedHost — an up-front check for hosts that are already IP
 *      literals. Node skips DNS resolution for those, so the hook in (1) is
 *      never invoked for them and cannot be relied on alone.
 */

const dns = require('node:dns');
const net = require('node:net');
const ipaddr = require('ipaddr.js');

// Denied unconditionally, whatever the deployment looks like. 169.254.169.254
// (AWS/GCP/Azure IMDS) and fd00:ec2::254 (IPv6 IMDS) are the addresses an SSRF
// is usually aimed at; there is no legitimate reason for a managed API endpoint
// to live on a link-local or unspecified address.
const ALWAYS_DENIED_CIDRS = [
    '169.254.0.0/16',   // link-local, includes the 169.254.169.254 metadata address
    '0.0.0.0/8',        // "this network"
    'fe80::/10',        // IPv6 link-local
    'fd00:ec2::254/128',// IPv6 cloud metadata address
    '::/128',           // IPv6 unspecified
];

// Denied only when private destinations are disallowed. These ranges are where
// a self-hosted gateway legitimately lives (docker-compose service names,
// Kubernetes cluster IPs, localhost during development), which is why the
// deployment gets to decide — see `tryout.allowPrivateEndpoints`.
const PRIVATE_CIDRS = [
    '10.0.0.0/8',
    '172.16.0.0/12',
    '192.168.0.0/16',
    '127.0.0.0/8',      // loopback
    '100.64.0.0/10',    // carrier-grade NAT
    '198.18.0.0/15',    // benchmarking
    '::1/128',          // IPv6 loopback
    'fc00::/7',         // IPv6 unique-local — the RFC 1918 analogue
    'fec0::/10',        // IPv6 site-local — deprecated by RFC 3879 but still routed on some networks
];

function parseCidrs(cidrs) {
    return cidrs.map((cidr) => ipaddr.parseCIDR(cidr));
}

const ALWAYS_DENIED = parseCidrs(ALWAYS_DENIED_CIDRS);
const PRIVATE_DENIED = parseCidrs(PRIVATE_CIDRS);

function matchesAny(addr, ranges) {
    return ranges.some(([range, bits]) => addr.kind() === range.kind() && addr.match(range, bits));
}

/**
 * True when `ip` must not be connected to.
 *
 * @param {string} ip                       resolved address, IPv4 or IPv6 literal
 * @param {object} [opts]
 * @param {boolean} [opts.allowPrivate]     permit RFC 1918 / loopback / ULA destinations
 * @returns {boolean}
 */
function isDenied(ip, { allowPrivate = false } = {}) {
    let addr;
    try {
        addr = ipaddr.parse(ip);
    } catch {
        return true; // Unparseable — fail closed rather than guess.
    }
    // Normalize IPv4-mapped IPv6 (::ffff:10.0.0.5) to its IPv4 form first:
    // ipaddr reports kind() === 'ipv6' for those, so they would otherwise slip
    // past every IPv4 CIDR above.
    if (addr.kind() === 'ipv6' && addr.isIPv4MappedAddress()) {
        addr = addr.toIPv4Address();
    }
    if (matchesAny(addr, ALWAYS_DENIED)) {
        return true;
    }
    return !allowPrivate && matchesAny(addr, PRIVATE_DENIED);
}

/**
 * Reject a target whose host is already an IP literal.
 *
 * The Agent `lookup` hook below cannot cover this case: Node skips DNS
 * resolution entirely when the host is a literal address, so `lookup` is never
 * invoked and a URL like "http://169.254.169.254/" would connect with the
 * denylist never consulted. Literals therefore have to be checked up front, by
 * the caller, before the request is issued.
 *
 * A non-literal hostname is a no-op here — the lookup hook handles it at dial
 * time, which is what closes the DNS-rebinding window.
 *
 * @param {string} hostname          URL.hostname (IPv6 may arrive bracketed)
 * @param {object} [opts]
 * @param {boolean} [opts.allowPrivate]
 * @throws {Error} with .statusCode when the literal is in a denied range
 */
function assertAllowedHost(hostname, { allowPrivate = false } = {}) {
    // URL.hostname keeps the brackets on an IPv6 literal ("[::1]").
    const host = String(hostname || '').replace(/^\[|\]$/g, '');
    if (!net.isIP(host)) {
        return;
    }
    if (isDenied(host, { allowPrivate })) {
        throw Object.assign(new Error('Destination is not allowed'), { statusCode: 422 });
    }
}

/**
 * Build a `lookup` implementation for a http/https Agent that rejects a
 * connection whose hostname resolves into a denied range.
 *
 * Only covers hostnames that actually get resolved — see assertAllowedHost for
 * IP literals, which never reach this hook.
 *
 * @param {object} [opts]
 * @param {boolean} [opts.allowPrivate]
 * @returns {Function} node dns.lookup-compatible callback
 */
function createGuardedLookup({ allowPrivate = false } = {}) {
    return function guardedLookup(hostname, options, callback) {
        dns.lookup(hostname, options, (err, address, family) => {
            if (err) return callback(err);

            // Node's Happy Eyeballs (autoSelectFamily, on by default since v20)
            // calls a custom lookup with `{ all: true }`, in which case `address`
            // is an array of { address, family }. Every candidate has to clear
            // the check — the socket may connect to any of them.
            if (options && options.all) {
                const denied = address.some((entry) => isDenied(entry.address, { allowPrivate }));
                if (denied) {
                    return callback(new Error('Destination is not allowed'));
                }
                return callback(null, address);
            }

            if (isDenied(address, { allowPrivate })) {
                return callback(new Error('Destination is not allowed'));
            }
            return callback(null, address, family);
        });
    };
}

/**
 * Throw unless `raw` uses a permitted scheme.
 *
 * @param {string} raw
 * @param {object} [opts]
 * @param {boolean} [opts.allowHttp]  also accept plain http:
 */
function assertAllowedScheme(raw, { allowHttp = false } = {}) {
    let protocol;
    try {
        ({ protocol } = new URL(raw));
    } catch {
        throw Object.assign(new Error('Malformed URL'), { statusCode: 422 });
    }
    const allowed = allowHttp ? ['https:', 'http:'] : ['https:'];
    if (!allowed.includes(protocol)) {
        throw Object.assign(new Error('URL scheme is not allowed'), { statusCode: 422 });
    }
}

module.exports = {
    isDenied,
    assertAllowedHost,
    createGuardedLookup,
    assertAllowedScheme,
};
