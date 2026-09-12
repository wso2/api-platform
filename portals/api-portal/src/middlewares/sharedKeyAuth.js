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

// Grant is defined in role-to-scope-mapping.yaml so the YAML stays the single source of truth.
const SHARED_KEY_ROLE = 'platform-api-system';

// Distinct from 'oauth2' / 'platform-jwt' so downstream checks and audit logs can tell them apart.
const SHARED_KEY_AUTH_MODE = 'shared-key';

// Custom scheme (not Bearer) so the dispatcher can discriminate by scheme name.
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
 * Unset or malformed hash short-circuits to false so a disabled portal never verifies.
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
 * Returns the fixed req.auth for a verified shared-key call.
 * preauthorized=false so the per-operation scope check still runs, limiting the mechanism
 * to the five dp:*:manage scopes the role grants.
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
 * Attempts shared-key auth on a request.
 * Returns { matched: false } (no SharedKey scheme), { matched: true, auth } (verified),
 * or { matched: true, auth: null } (bad key). Splitting the last two prevents a bad
 * SharedKey token from silently retrying against another auth path.
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
