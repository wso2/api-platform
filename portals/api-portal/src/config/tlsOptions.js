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
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

'use strict';

/**
 * Translates the platform-wide HTTPSListener TLS fields (minimumProtocolVersion /
 * maximumProtocolVersion / ciphers / ecdhCurves — same names, same comma-separated
 * shape as platform-api's and ai-workspace's Go HTTPSListener struct) into the
 * options https.createServer()/tls.createServer() actually accept. Kept as a
 * separate module so server.js's happy path stays readable and so the mapping
 * tables here are the one place that needs updating if Node's option names change.
 */

const tls = require('tls');

// TLS version name -> Node's tls minVersion/maxVersion string. Same vocabulary
// ("TLS1_2", etc.) as platform-api/ai-workspace's HTTPSListener fields, even
// though each is enforced by a different TLS stack (Go crypto/tls there, Node's
// own tls module here).
const TLS_VERSION_BY_NAME = {
    TLS1_0: 'TLSv1',
    TLS1_1: 'TLSv1.1',
    TLS1_2: 'TLSv1.2',
    TLS1_3: 'TLSv1.3',
};

const TLS_VERSION_ORDER = { TLS1_0: 0, TLS1_1: 1, TLS1_2: 2, TLS1_3: 3 };

// Config section used in error messages when the caller does not name one.
// The same validators back both [api_portal.server.https] (inbound listener)
// and [api_portal.http_client.tls] (outbound client), so callers pass the
// section they are validating to make the error point at the right place.
const DEFAULT_SECTION = 'server.https';

/**
 * Validates that minVersion/maxVersion are recognized names and that minVersion
 * does not come after maxVersion. Throws on failure so an invalid config fails
 * closed at startup rather than silently falling back to Node's default range.
 * `section` names the config section in error messages.
 */
function validateTLSVersions(minVersion, maxVersion, section = DEFAULT_SECTION) {
    if (!Object.prototype.hasOwnProperty.call(TLS_VERSION_BY_NAME, minVersion)) {
        throw new Error(`${section}.minimum_protocol_version must be one of TLS1_0, TLS1_1, TLS1_2, TLS1_3, got: "${minVersion}"`);
    }
    if (!Object.prototype.hasOwnProperty.call(TLS_VERSION_BY_NAME, maxVersion)) {
        throw new Error(`${section}.maximum_protocol_version must be one of TLS1_0, TLS1_1, TLS1_2, TLS1_3, got: "${maxVersion}"`);
    }
    if (TLS_VERSION_ORDER[minVersion] > TLS_VERSION_ORDER[maxVersion]) {
        throw new Error(`${section}.minimum_protocol_version (${minVersion}) cannot be greater than ${section}.maximum_protocol_version (${maxVersion})`);
    }
}

/** Converts a validated version name to Node's minVersion/maxVersion string. */
function parseTLSVersion(name) {
    return TLS_VERSION_BY_NAME[name];
}

// ECDH/group name -> the OpenSSL curve name Node's `ecdhCurve` option expects.
// X25519MLKEM768 is the FIPS 203 ML-KEM-768 + X25519 hybrid group (Node 22+ /
// OpenSSL 3.2+). Same curve vocabulary as platform-api/ai-workspace's EcdhCurves.
const ECDH_CURVE_BY_NAME = {
    X25519: 'X25519',
    'P-256': 'prime256v1',
    'P-384': 'secp384r1',
    'P-521': 'secp521r1',
    X25519MLKEM768: 'X25519MLKEM768',
};

/**
 * Parses a comma-separated EcdhCurves preference list (e.g.
 * "X25519MLKEM768,X25519,P-256") into the colon-delimited string Node's
 * `ecdhCurve` option expects. Throws on an unrecognized name or an empty list —
 * fail closed rather than silently falling back to Node's default curve list.
 * `section` names the config section in error messages.
 */
function parseEcdhCurves(raw, section = DEFAULT_SECTION) {
    const names = String(raw || '').split(',').map((s) => s.trim()).filter(Boolean);
    if (names.length === 0) {
        throw new Error(`${section}.ecdh_curves must specify at least one curve`);
    }
    const curves = names.map((name) => {
        const curve = ECDH_CURVE_BY_NAME[name];
        if (!curve) {
            throw new Error(`${section}.ecdh_curves: unsupported curve "${name}" (supported: X25519, P-256, P-384, P-521, X25519MLKEM768)`);
        }
        return curve;
    });
    return curves.join(':');
}

/**
 * Parses a comma-separated list of cipher suite names into the colon-delimited
 * string Node's `ciphers` option expects, validated against tls.getCiphers() so
 * a typo fails closed at startup rather than silently negotiating Node's default
 * set. An empty string is valid and returns undefined, meaning Node's own
 * default cipher set/order applies.
 * `section` names the config section in error messages.
 */
function parseCiphers(raw, section = DEFAULT_SECTION) {
    const trimmed = String(raw || '').trim();
    if (trimmed === '') {
        return undefined;
    }
    const names = trimmed.split(',').map((s) => s.trim()).filter(Boolean);
    if (names.length === 0) {
        throw new Error(`${section}.ciphers must specify at least one cipher suite, or be left empty to use Node's default set`);
    }
    const supported = new Set(tls.getCiphers());
    for (const name of names) {
        if (!supported.has(name.toLowerCase())) {
            throw new Error(`${section}.ciphers: unsupported cipher suite "${name}" (see tls.getCiphers() for the supported list)`);
        }
    }
    return names.join(':');
}

/**
 * Builds the { minVersion, maxVersion, ecdhCurve, ciphers? } options object for
 * https.createServer() from an HTTPSListener-shaped config object. Throws on any
 * invalid field — callers should let that abort startup rather than serve a
 * listener with a silently-degraded TLS posture.
 *
 * `section` is the config section being validated (e.g. 'server.https' or
 * 'http_client.tls'); every error message is prefixed with it so the operator
 * is pointed at the section that actually holds the bad value.
 */
function buildTLSOptions(httpsCfg, section = DEFAULT_SECTION) {
    validateTLSVersions(httpsCfg.minimumProtocolVersion, httpsCfg.maximumProtocolVersion, section);
    const options = {
        minVersion: parseTLSVersion(httpsCfg.minimumProtocolVersion),
        maxVersion: parseTLSVersion(httpsCfg.maximumProtocolVersion),
        ecdhCurve: parseEcdhCurves(httpsCfg.ecdhCurves, section),
    };
    const ciphers = parseCiphers(httpsCfg.ciphers, section);
    if (ciphers) {
        options.ciphers = ciphers;
    }
    return options;
}

module.exports = {
    validateTLSVersions,
    parseTLSVersion,
    parseEcdhCurves,
    parseCiphers,
    buildTLSOptions,
};
