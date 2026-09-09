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

const crypto = require('crypto');

const { config } = require('../config/configLoader');
const roleScopeMap = require('../config/roleScopeMap');

// The role name synthesised for a verified shared-key caller. The corresponding
// entry in role-to-scope-mapping.yaml grants the five dp:*:manage scopes
// platform-api needs to publish APIs / API content / MCP servers / MCP server
// content / subscription plans, and nothing else. Keeping the scope grant in the
// YAML (not hard-coded here) means the shipped grant table stays the single
// source of truth for what any role, including this service identity, may do.
const SHARED_KEY_ROLE = 'platform-api-system';

// The auth mode string used in req.auth when a request authenticates via a
// verified SharedKey scheme. Distinct from 'oauth2' and 'platform-jwt' so an
// audit log or downstream check can tell shared-key traffic from user/session
// traffic. OAuth2Security in authMiddleware.js allows this mode alongside the
// other two.
const SHARED_KEY_AUTH_MODE = 'shared-key';

// HTTP Authorization scheme name (case-insensitive) reserved for this mechanism.
// A custom scheme rather than reusing `Bearer` — RFC 6750 defines Bearer for
// OAuth 2.0 access tokens, and a static pre-shared secret isn't one. RFC 7235
// explicitly permits custom schemes on the Authorization header, and using one
// lets the request-time dispatcher (authResolver / csrf) discriminate by scheme
// name instead of sniffing for a magic prefix inside a bearer value.
const SHARED_KEY_SCHEME = 'sharedkey';

function parseAuthorizationScheme(header) {
    if (typeof header !== 'string') return null;
    const trimmed = header.trim();
    const space = trimmed.indexOf(' ');
    if (space <= 0) return null;
    return {
        scheme: trimmed.slice(0, space).toLowerCase(),
        value: trimmed.slice(space + 1).trim(),
    };
}

function isSharedKeyRequest(req) {
    const parsed = parseAuthorizationScheme(req?.headers?.authorization);
    return parsed !== null && parsed.scheme === SHARED_KEY_SCHEME;
}

function hexToBuffer(hex) {
    if (typeof hex !== 'string' || hex.length % 2 !== 0 || !/^[0-9a-fA-F]+$/.test(hex)) {
        return null;
    }
    return Buffer.from(hex, 'hex');
}

/**
 * Constant-time check: does sha256(raw) equal the configured hex hash?
 *
 * Both branches of the compare run in fixed time relative to the input length,
 * so an attacker can't distinguish "wrong length" from "wrong bytes" from timing.
 * A configured hash that isn't 64-char hex (guarded at boot in configLoader) or
 * an unset hash both short-circuit to false: a caller sending SharedKey against
 * a portal that disabled the feature always fails to verify.
 */
function verifyHash(rawToken, configuredHashHex) {
    if (!rawToken || !configuredHashHex) return false;
    const expected = hexToBuffer(configuredHashHex);
    if (!expected) return false;
    const actual = crypto.createHash('sha256').update(rawToken, 'utf8').digest();
    if (expected.length !== actual.length) return false;
    return crypto.timingSafeEqual(expected, actual);
}

/**
 * Returns the fixed req.auth shape a verified shared-key call is granted.
 *
 * `mode` marks the request as shared-key for downstream checks / audit logs.
 * `preauthorized = false` on purpose: OAuth2Security still runs the scope check
 * against the operation's declared security, and shared-key only carries the
 * five dp:*:manage scopes — so a valid shared-key call can reach the publishing
 * write operations and nothing else, without any per-route wrapping.
 * `scopes` come from expanding the `platform-api-system` role through the
 * shipped role-to-scope map, so the grant stays defined in one file (YAML).
 * `userId` is null — no portal user represents this identity.
 * `rawSub` is the role name so audit logs record the service identity.
 */
function synthesiseSharedKeyPrincipal() {
    const scopes = roleScopeMap.expandRoles([SHARED_KEY_ROLE]);
    return {
        mode: SHARED_KEY_AUTH_MODE,
        preauthorized: false,
        scopes,
        userId: null,
        rawSub: SHARED_KEY_ROLE,
    };
}

/**
 * Attempts to authenticate an incoming request via the shared-key mechanism.
 *
 * Returns one of:
 *   { matched: false }                    — Authorization is missing or uses a
 *                                            different scheme; caller should
 *                                            continue to the next auth path.
 *   { matched: true, auth: <req.auth> }   — verified; caller should assign
 *                                            `req.auth = result.auth` and pass
 *                                            control on.
 *   { matched: true, auth: null }         — SharedKey scheme was sent but the
 *                                            value failed to verify (or the
 *                                            portal has no hash configured);
 *                                            caller should reject with 401.
 *
 * Splitting the "no scheme sent" and "scheme sent but wrong" cases lets the
 * caller (authResolver) treat only the second as a hard 401 rather than
 * falling through to another auth path — a valid non-SharedKey token can't
 * accidentally satisfy a SharedKey attempt, and a bad SharedKey token can't
 * be quietly retried against the OAuth path.
 */
function tryAuthenticate(req) {
    const parsed = parseAuthorizationScheme(req?.headers?.authorization);
    if (!parsed || parsed.scheme !== SHARED_KEY_SCHEME) {
        return { matched: false };
    }
    const configuredHash = config.internalAuth?.hash;
    if (!verifyHash(parsed.value, configuredHash)) {
        return { matched: true, auth: null };
    }
    return { matched: true, auth: synthesiseSharedKeyPrincipal() };
}

module.exports = {
    SHARED_KEY_AUTH_MODE,
    SHARED_KEY_ROLE,
    SHARED_KEY_SCHEME,
    isSharedKeyRequest,
    parseAuthorizationScheme,
    tryAuthenticate,
    verifyHash,
    synthesiseSharedKeyPrincipal,
};
