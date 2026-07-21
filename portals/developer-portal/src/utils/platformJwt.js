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
const { jwtVerify, decodeJwt } = require('jose');

function toClaims(payload) {
    return {
        ...payload,
        scopes: String(payload.scope || '').split(' ').filter(Boolean),
    };
}

/**
 * Verify a Platform API JWT with the shared HS256 secret.
 *
 * The Platform API signs its tokens with this shared symmetric secret, so the
 * algorithm is pinned to HS256 (never `none` or any other algorithm). Returns
 * the payload spread together with a parsed `scopes` array, or null if
 * verification fails.
 *
 * Use this to authenticate a request-supplied token.
 */
async function verifyPlatformJwtClaims(token, secret) {
    try {
        const key = new TextEncoder().encode(secret);
        const { payload } = await jwtVerify(token, key, { algorithms: ['HS256'] });
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
