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
const jwt = require('jsonwebtoken');

/**
 * Decode and optionally verify a Platform API JWT.
 *
 * When jwtSecret is provided the token is verified with HS256 (prevents
 * tampered tokens from being accepted). Without a secret the payload is
 * base64-decoded without signature verification — suitable for tokens that
 * were just received directly from the Platform API over a trusted HTTPS
 * connection.
 *
 * Returns the full JWT payload spread together with a parsed `scopes` array,
 * or null if decoding / verification fails.
 */
function extractPlatformJwtClaims(token, jwtSecret) {
    try {
        const payload = jwtSecret
            ? jwt.verify(token, jwtSecret, { algorithms: ['HS256'] })
            : JSON.parse(Buffer.from(token.split('.')[1], 'base64url').toString('utf8'));
        return {
            ...payload,
            scopes: String(payload.scope || '').split(' ').filter(Boolean),
        };
    } catch (_) {
        return null;
    }
}

module.exports = { extractPlatformJwtClaims };
