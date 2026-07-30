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
const { decodeJwt } = require('jose');

/**
 * Decode a JWT payload WITHOUT verifying its signature.
 *
 * Mirrors the semantics of jsonwebtoken's `jwt.decode(token)`: returns the
 * payload claims object, or `null` when the token is missing or malformed
 * (jose's `decodeJwt` throws on malformed input, so it is wrapped here).
 *
 * Use only for reading claims from a token whose authenticity has already been
 * established elsewhere — never as a substitute for signature verification.
 */
function safeDecodeJwt(token) {
    if (!token) {
        return null;
    }
    try {
        return decodeJwt(token);
    } catch (_) {
        return null;
    }
}

/**
 * Resolves a dot-notation claim path (e.g. Keycloak's "realm_access.roles") from a
 * decoded JWT, falling back gracefully so a plain claim name ("roles") still works.
 *
 * Lives here rather than in passportConfig.js — where it started, next to its first
 * caller — because the authorization layer needs it to read the configured roles
 * claim, and requiring passportConfig from there would make the middleware graph
 * circular (passportConfig -> authorization -> passportConfig).
 */
function getNestedClaim(obj, path) {
    if (!path || typeof obj !== 'object' || obj === null) return undefined;
    const parts = String(path).split('.');
    let cur = obj;
    for (const part of parts) {
        if (typeof cur !== 'object' || cur === null) return undefined;
        cur = cur[part];
    }
    return cur;
}

module.exports = { safeDecodeJwt, getNestedClaim };
