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

const constants = require('./constants');

// Named explicitly rather than left to express-session's default, so the logout path
// below expires exactly the cookie app.js sets instead of assuming the library's name.
const SESSION_COOKIE_NAME = 'connect.sid';
const CSRF_COOKIE_NAME = 'XSRF-TOKEN';

// Every path a portal cookie may be sitting at in a live browser. BASE_PATH is where
// both cookies are written now; '/' is where every release before the portal gained its
// mount prefix wrote them.
//
// A browser keys a cookie by (name, domain, path), so a Set-Cookie expiry written for
// one path creates a SEPARATE cookie rather than removing one at another path. Left
// alone, a pre-upgrade cookie at '/' keeps being sent on every request under the prefix
// with nothing able to remove it, and the consequences differ per cookie:
//
//   - connect.sid — express-session emits no Set-Cookie at all once req.session has been
//     destroyed (it returns early on `if (!req.session)`), so it never expires even its
//     own cookie. A stale root-scoped one can therefore still resolve to a session row
//     and read as signed-in after logout.
//   - XSRF-TOKEN — scripts/common.js reads the FIRST match out of document.cookie, so a
//     stale root-scoped token can shadow the live one and fail CSRF on every mutating
//     request.
//
// Hence: expire both names at both paths.
const CLEARED_COOKIE_PATHS = [constants.ROUTE.BASE_PATH, '/'];

// Number of proxy hops in front of the portal whose X-Forwarded-* headers are trusted
// (Express `trust proxy`). One: the hop that terminates TLS for us, kgateway in the
// cloud or the operator's own load balancer on-prem. That is what lets req.secure, and
// with it the cookies' Secure flag below, follow the scheme the browser actually used.
const TRUSTED_PROXY_HOPS = 1;

/**
 * Cookie options for express-session's session cookie.
 *
 * secure: 'auto' sets Secure whenever the request arrived over HTTPS, terminated by
 * this process or by the trusted proxy. Keying it to server.https.enabled instead left
 * it off wherever TLS ends upstream, which is every cloud portal.
 */
function sessionCookieOptions({ designMode = false } = {}) {
    return {
        secure: designMode ? false : 'auto',
        httpOnly: true,
        // Lax still sends the cookie on the top-level redirect back from the IdP.
        sameSite: 'lax',
        maxAge: 60 * 60 * 1000,
        // Must match the paths the browser actually requests — a mismatched cookie path
        // is a silent login loop. See app.js on why the session mount matches it.
        path: constants.ROUTE.BASE_PATH,
    };
}

/**
 * Cookie options for the browser-readable XSRF cookie. res.cookie has no 'auto', so
 * Secure follows req.secure, which honours the trusted proxy the same way.
 */
function csrfCookieOptions(req, { designMode = false } = {}) {
    return {
        sameSite: 'lax',
        secure: req.secure && !designMode,
        // Same scope as the session cookie — the double-submit token is only ever echoed
        // back from pages served under BASE_PATH.
        path: constants.ROUTE.BASE_PATH,
    };
}

/**
 * Expires the session and CSRF cookies at every path they may have been written at.
 * Call on any path that ends a session (logout, logout landing, a 401 that destroys it).
 */
function clearPortalCookies(res) {
    for (const path of CLEARED_COOKIE_PATHS) {
        res.clearCookie(SESSION_COOKIE_NAME, { path, httpOnly: true });
        res.clearCookie(CSRF_COOKIE_NAME, { path });
    }
}

module.exports = {
    SESSION_COOKIE_NAME,
    CSRF_COOKIE_NAME,
    CLEARED_COOKIE_PATHS,
    TRUSTED_PROXY_HOPS,
    sessionCookieOptions,
    csrfCookieOptions,
    clearPortalCookies,
};
