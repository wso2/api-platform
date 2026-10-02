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

// Sanitiser for `req.session.returnTo`, the post-login destination.
//
// That value is handed straight to res.redirect() by authController's login,
// silent-SSO, and sign-up flows, and every writer derives it from req.originalUrl —
// which is NOT guaranteed to be a same-origin path. An absolute-form request target
//
//     GET http://evil.com/api-portal/acme/views/default/page HTTP/1.1
//
// is routed on its parsed pathname, so the route matches and req.params fill in
// normally, while req.originalUrl keeps the attacker's scheme and host verbatim.
// Stored unvalidated, that turns the post-login redirect into an open redirect
// (JS-AUTH-010). Only a same-origin path survives here; anything else is replaced by
// the caller's safe default rather than echoed back.
//
// Lives in utils/ rather than beside its callers so it stays free of the config
// dependency the middleware carries, which is what lets returnToGuard.test.js
// exercise it directly.

// Any absolute origin works as the resolution base; it only has to be one no
// candidate can legitimately resolve to, so that "same origin as the base" is a
// reliable proxy for "no scheme or authority of its own".
const SENTINEL_ORIGIN = 'https://returnto.invalid';

/**
 * @param {unknown} candidate - untrusted destination (typically req.originalUrl)
 * @param {string} fallback   - server-constructed default used when candidate is unsafe
 * @returns {string} a same-origin path+query, or `fallback`
 */
function sanitizeReturnTo(candidate, fallback) {
    if (typeof candidate !== 'string' || candidate === '' || candidate[0] !== '/') return fallback;

    let parsed;
    try {
        parsed = new URL(candidate, SENTINEL_ORIGIN);
    } catch {
        return fallback;
    }

    // A candidate carrying its own scheme or authority — including the "//host" and
    // "/\host" spellings the URL parser reads as an authority — resolves off the
    // sentinel origin and is rejected here.
    if (parsed.origin !== SENTINEL_ORIGIN) return fallback;

    const resolved = `${parsed.pathname}${parsed.search}`;

    // Re-checked AFTER normalisation rather than only on the input: dot-segment
    // resolution can turn an accepted path into a protocol-relative target that
    // leaves this origin ("/..//evil.com" normalises to "//evil.com").
    if (!resolved.startsWith('/') || resolved.startsWith('//')) return fallback;

    return resolved;
}

module.exports = { sanitizeReturnTo };
