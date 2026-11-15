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
 * Destination guard for outbound requests whose destination is influenced by a
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
const url = require('node:url');
const ipaddr = require('ipaddr.js');
const { RE2JS } = require('re2js');

// Denied unconditionally, whatever the deployment looks like. 169.254.169.254
// (AWS/GCP/Azure IMDS) and fd00:ec2::254 (IPv6 IMDS) are the addresses a forged
// request is usually aimed at; there is no legitimate reason for a managed API endpoint
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
    if (addrDenied(addr, allowPrivate)) {
        return true;
    }
    // A 6to4 (2002::/16) or NAT64 (64:ff9b::/96) address routes onward to the
    // IPv4 address it carries, so that address must be allowed too:
    // 2002:a9fe:a9fe:: reaches 169.254.169.254.
    const embedded = embeddedIPv4(addr);
    return embedded !== null && addrDenied(embedded, allowPrivate);
}

function addrDenied(addr, allowPrivate) {
    if (matchesAny(addr, ALWAYS_DENIED)) {
        return true;
    }
    return !allowPrivate && matchesAny(addr, PRIVATE_DENIED);
}

const [SIX_TO_FOUR, SIX_TO_FOUR_BITS] = ipaddr.parseCIDR('2002::/16');
const [NAT64, NAT64_BITS] = ipaddr.parseCIDR('64:ff9b::/96');

/** The IPv4 address inside a 6to4 or NAT64 IPv6 address, or null. */
function embeddedIPv4(addr) {
    if (addr.kind() !== 'ipv6') return null;
    const b = addr.toByteArray();
    if (addr.match(SIX_TO_FOUR, SIX_TO_FOUR_BITS)) {
        return ipaddr.fromByteArray(b.slice(2, 6));
    }
    if (addr.match(NAT64, NAT64_BITS)) {
        return ipaddr.fromByteArray(b.slice(12, 16));
    }
    return null;
}

/*
 * Network access control: [api_portal.server.network_security.access_control].
 *
 * Applies to destinations that come from tenant or user input — try-it targets,
 * key managers created through the API, webhook subscriber URLs, the developer
 * token proxy. Destinations configured in config.toml (the IdP, Platform API,
 * billing providers, [[key_manager]] entries) are never subject to it.
 *
 * Hostname patterns are RE2 expressions (re2js: linear-time matching, no
 * backtracking), each anchored as ^(?:pattern)$ and matched case-insensitively
 * against the whole normalized hostname, never its port or path.
 */
const MAX_HOSTNAME_PATTERNS = 256;
const MAX_HOSTNAME_PATTERN_LENGTH = 255;

/**
 * Compile and validate an access-control configuration. Throws on an invalid
 * one, so configLoader can refuse to start.
 *
 * @param {object} [cfg]  server.networkSecurity.accessControl
 * @returns {Readonly<{enabled: boolean, mode: string, patterns: object[], denyPrivateNetworks: boolean, allowedSchemes: string[], maxRedirects: number}>}
 */
function compileAccessControl(cfg = {}) {
    const enabled = cfg.enabled === true;
    const mode = String(cfg.mode || 'deny').trim().toLowerCase();
    const hostnames = cfg.hostnames ?? [];
    const allowedSchemes = (cfg.allowedSchemes && cfg.allowedSchemes.length ? cfg.allowedSchemes : ['http', 'https'])
        .map((x) => String(x).trim().toLowerCase());
    const maxRedirects = Number(cfg.maxRedirects ?? 0);
    if (!enabled) {
        return Object.freeze({ enabled: false, mode, patterns: [], denyPrivateNetworks: false, allowedSchemes, maxRedirects: 0 });
    }
    if (mode !== 'deny' && mode !== 'allow') {
        throw new Error('mode must be "deny" or "allow"');
    }
    if (!Array.isArray(hostnames)) {
        throw new Error('hostnames must be an array of regular expressions');
    }
    if (hostnames.length > MAX_HOSTNAME_PATTERNS) {
        throw new Error(`hostnames may hold at most ${MAX_HOSTNAME_PATTERNS} patterns`);
    }
    if (mode === 'allow' && hostnames.length === 0) {
        throw new Error('mode "allow" requires at least one entry in hostnames');
    }
    for (const scheme of allowedSchemes) {
        if (scheme !== 'http' && scheme !== 'https') {
            throw new Error('allowed_schemes may only contain "http" and "https"');
        }
    }
    if (!Number.isInteger(maxRedirects) || maxRedirects < 0) {
        throw new Error('max_redirects must be a non-negative integer');
    }
    const patterns = hostnames.map((raw, i) => {
        if (typeof raw !== 'string' || raw.trim() === '') {
            throw new Error(`hostnames[${i}] is empty`);
        }
        if (raw.length > MAX_HOSTNAME_PATTERN_LENGTH) {
            throw new Error(`hostnames[${i}] is longer than ${MAX_HOSTNAME_PATTERN_LENGTH} characters`);
        }
        try {
            return RE2JS.compile(`^(?:${raw})$`, RE2JS.CASE_INSENSITIVE);
        } catch (err) {
            throw new Error(`hostnames[${i}] is not a valid regular expression: ${err.message}`, { cause: err });
        }
    });
    return Object.freeze({
        enabled: true,
        mode,
        patterns: Object.freeze(patterns),
        denyPrivateNetworks: cfg.denyPrivateNetworks === true,
        allowedSchemes: Object.freeze(allowedSchemes),
        maxRedirects,
    });
}

let installedAccessControl = compileAccessControl({ enabled: false });

/** Install the compiled access control. Called by configLoader at startup. */
function setAccessControl(compiled) {
    installedAccessControl = compiled || compileAccessControl({ enabled: false });
}

/** The installed access control (disabled until configLoader installs one). */
function getAccessControl() {
    return installedAccessControl;
}

/**
 * The form host patterns are matched against: no brackets or trailing dots,
 * lowercase, punycode. Trailing dots are stripped after the IDNA conversion too:
 * it maps Unicode label separators (U+3002, U+FF0E, U+FF61) to ".", so
 * "example.com\u3002" would otherwise become "example.com." and slip past an
 * anchored "example\\.com" pattern while still resolving to the same host.
 */
function normalizeHost(hostname) {
    let host = String(hostname || '').trim().replace(/^\[|\]$/g, '').replace(/\.+$/, '');
    if (!host) return '';
    if (net.isIP(host)) {
        return ipaddr.parse(host).toString();
    }
    host = url.domainToASCII(host).replace(/\.+$/, '');
    return host.toLowerCase();
}

/**
 * Whether access control permits a hostname. Always true when it is disabled.
 *
 * @param {string} hostname
 * @param {object} [accessControl]  a compileAccessControl result; defaults to the installed one
 */
function isHostAllowed(hostname, accessControl = installedAccessControl) {
    if (!accessControl || !accessControl.enabled) return true;
    const host = normalizeHost(hostname);
    if (!host) return false;
    const matched = accessControl.patterns.some((re) => re.matches(host));
    return accessControl.mode === 'allow' ? matched : !matched;
}

/**
 * The private-address stance for a tenant- or user-supplied destination: a
 * feature's own allow_private_endpoints flag, which access control can only
 * tighten (deny_private_networks), never widen.
 */
function effectiveAllowPrivate(featureAllowsPrivate, accessControl = installedAccessControl) {
    const denied = accessControl && accessControl.enabled && accessControl.denyPrivateNetworks;
    return featureAllowsPrivate === true && !denied;
}

function notAllowed(message) {
    return Object.assign(new Error(message), { statusCode: 422, code: 'DESTINATION_NOT_ALLOWED' });
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
function assertAllowedHost(hostname, { allowPrivate = false, accessControl = null } = {}) {
    // Access control's host rule applies to every host, literal or not.
    if (accessControl && !isHostAllowed(hostname, accessControl)) {
        throw notAllowed('Destination is not allowed');
    }
    // URL.hostname keeps the brackets on an IPv6 literal ("[::1]").
    const host = String(hostname || '').replace(/^\[|\]$/g, '');
    if (!net.isIP(host)) {
        return;
    }
    if (isDenied(host, { allowPrivate })) {
        throw notAllowed('Destination is not allowed');
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
function createGuardedLookup({ allowPrivate = false, accessControl = null } = {}) {
    return function guardedLookup(hostname, options, callback) {
        if (accessControl && !isHostAllowed(hostname, accessControl)) {
            return callback(notAllowed('Destination is not allowed'));
        }
        dns.lookup(hostname, options, (err, address, family) => {
            if (err) return callback(err);

            // Node's Happy Eyeballs (autoSelectFamily, on by default since v20)
            // calls a custom lookup with `{ all: true }`, in which case `address`
            // is an array of { address, family }. Every candidate has to clear
            // the check — the socket may connect to any of them.
            if (options && options.all) {
                const denied = address.some((entry) => isDenied(entry.address, { allowPrivate }));
                if (denied) {
                    return callback(notAllowed('Destination is not allowed'));
                }
                return callback(null, address);
            }

            if (isDenied(address, { allowPrivate })) {
                return callback(notAllowed('Destination is not allowed'));
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
function assertAllowedScheme(raw, { allowHttp = false, accessControl = null } = {}) {
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
    if (accessControl && accessControl.enabled && !accessControl.allowedSchemes.includes(protocol.slice(0, -1))) {
        throw notAllowed('URL scheme is not allowed');
    }
}

/**
 * Check a tenant- or user-supplied URL against the scheme, access control's
 * host rule and, for an IP-literal host, the address ranges — without any
 * network activity. A hostname's resolved addresses are checked when it is
 * dialled (createGuardedLookup). Used where such a URL is saved, so a refused
 * one is rejected up front rather than at first use.
 *
 * @param {string} raw
 * @param {object} [opts]
 * @param {boolean} [opts.allowHttp=true]
 * @param {boolean} [opts.allowPrivate]   the feature's own stance; access control can only tighten it
 * @param {object} [opts.accessControl]   defaults to the installed access control
 * @throws {Error} with .statusCode 422 when the URL is malformed or refused
 */
function assertUserUrl(raw, { allowHttp = true, allowPrivate = true, accessControl = installedAccessControl } = {}) {
    assertAllowedScheme(raw, { allowHttp, accessControl });
    assertAllowedHost(new URL(raw).hostname, {
        allowPrivate: effectiveAllowPrivate(allowPrivate, accessControl),
        accessControl,
    });
}

module.exports = {
    isDenied,
    assertAllowedHost,
    createGuardedLookup,
    assertAllowedScheme,
    assertUserUrl,
    compileAccessControl,
    setAccessControl,
    getAccessControl,
    isHostAllowed,
    effectiveAllowPrivate,
    normalizeHost,
};
