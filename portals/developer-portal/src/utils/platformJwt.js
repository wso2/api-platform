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
const fs = require('fs').promises;
const path = require('path');
const { jwtVerify, decodeJwt, importSPKI } = require('jose');
const constants = require('./constants');

function toClaims(payload) {
    return {
        ...payload,
        scopes: String(payload.scope || '').split(' ').filter(Boolean),
    };
}

// Imported public keys cached by path — verification is on the hot path (every
// bearer request), so the PEM file is read and parsed once, not per request. Only
// successful imports are cached; a read/parse failure is left uncached so a later
// request retries (e.g. once the key file is provisioned).
const publicKeyCache = new Map();

async function importPublicKeyFromPath(publicKeyPath) {
    const cached = publicKeyCache.get(publicKeyPath);
    if (cached) return cached;
    if (typeof publicKeyPath !== 'string' || publicKeyPath.includes('\0') || path.normalize(publicKeyPath).includes('..')) {
        throw new Error('invalid public key path');
    }
    const pem = await fs.readFile(publicKeyPath, 'utf8');
    const key = await importSPKI(pem, constants.JWT_ASYMMETRIC_ALGORITHMS[0]);
    publicKeyCache.set(publicKeyPath, key);
    return key;
}

/**
 * Verify a Platform API JWT against the Platform API's RSA public key.
 *
 * The Platform API mints its tokens with the private half of an RS256 keypair
 * ([platform_api.auth.jwt].private_key) and rejects symmetric ("HS*") and
 * unsigned ("none") tokens outright, so verification here is pinned to the same
 * asymmetric allowlist — the public key must never be accepted as an HMAC
 * secret. `publicKeyPath` is the filesystem path to the SPKI PEM matching that
 * keypair ([platform_api.auth.jwt].public_key); the file is read once and cached.
 * Returns the payload spread together with a parsed `scopes` array, or null if
 * the key cannot be read or verification fails.
 *
 * Use this to authenticate a request-supplied token.
 */
async function verifyPlatformJwtClaims(token, publicKeyPath) {
    try {
        const key = await importPublicKeyFromPath(publicKeyPath);
        const { payload } = await jwtVerify(token, key, {
            algorithms: constants.JWT_ASYMMETRIC_ALGORITHMS,
        });
        return toClaims(payload);
    } catch (_) {
        return null;
    }
}

/**
 * Decode a Platform API JWT WITHOUT verifying its signature.
 *
 * Returns the payload spread together with a parsed `scopes` array, or null on
 * malformed input. Use only for a token whose authenticity is already
 * established — e.g. one just received directly from the Platform API over a
 * trusted HTTPS connection — never to authenticate a request-supplied token.
 */
function decodePlatformJwtClaims(token) {
    try {
        return toClaims(decodeJwt(token));
    } catch (_) {
        return null;
    }
}

module.exports = { verifyPlatformJwtClaims, decodePlatformJwtClaims };
