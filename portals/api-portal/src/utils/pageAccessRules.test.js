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
 * The page-access gate, asserted against the paths the page routers actually
 * register.
 *
 * ensureAuthenticated decides a page's tier by glob-matching the request path
 * against SYSTEM_AUTHENTICATED_PAGES and SYSTEM_AUTHORIZED_PAGES. A path that
 * matches NEITHER list does not fall back to "authenticated" -- it skips the
 * whole auth block (the trailing `else { return next(); }`), so the page is
 * served with no authentication, no role tier, no cross-portal session check and
 * no organization check.
 *
 * That failure is silent: the page renders, nothing errors, and nothing in the
 * route file hints that a second edit was required. The OAuth2 Keys page shipped
 * in exactly that state -- its router had ensureAuthenticated like every sibling,
 * and the patterns were never added here.
 *
 * So the real assertion is the last one: every consumer page a router registers
 * is covered. A new page added without a pattern fails here rather than reaching
 * a deployment unguarded.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const { minimatch } = require('minimatch');

const constants = require('./constants');

const AUTHENTICATED = constants.ROUTE.SYSTEM_AUTHENTICATED_PAGES;
const AUTHORIZED = constants.ROUTE.SYSTEM_AUTHORIZED_PAGES;

/** How ensureAuthenticated matches: query stripped, BASE_PATH stripped. */
function matches(patterns, pathname) {
    return patterns.some((p) => minimatch(pathname, p));
}

const ORG = '/acme/views/default';

// ---------------------------------------------------------------------------
// The page that was missed
// ---------------------------------------------------------------------------

test('the OAuth2 Keys page is gated like every other consumer page', () => {
    /*
     * It lists OAuth2 client credentials, so it belongs in the same tier as API
     * keys. It was in neither list when the page shipped, which left it reachable
     * without authentication at all.
     */
    assert.ok(matches(AUTHENTICATED, `${ORG}/oauth2-keys`), 'must require authentication');
    assert.ok(matches(AUTHORIZED, `${ORG}/oauth2-keys`), 'must require the subscriber/admin tier');
});

test('a query string does not drop the OAuth2 Keys page out of the gate', () => {
    /*
     * ensureAuthenticated strips the query before matching, but the lists carry an
     * explicit "?**" variant for the pages that are linked with one -- the key
     * manager filter links here as ?km=<handle>. Asserted against both forms so a
     * future change to that stripping cannot silently unguard the page.
     */
    for (const p of [`${ORG}/oauth2-keys?km=thunder`, `${ORG}/oauth2-keys?view=default&km=x`]) {
        assert.ok(matches(AUTHENTICATED, p), `${p} must require authentication`);
        assert.ok(matches(AUTHORIZED, p), `${p} must require the tier`);
    }
});

test('the gate still applies under the mount prefix', () => {
    // ensureAuthenticated strips BASE_PATH before matching, so the patterns are
    // written against the bare path. Assert the stripped form is what matches.
    const prefixed = `${constants.ROUTE.BASE_PATH}${ORG}/oauth2-keys`;
    const stripped = prefixed.slice(constants.ROUTE.BASE_PATH.length) || '/';
    assert.ok(matches(AUTHORIZED, stripped), 'the path ensureAuthenticated matches is the stripped one');
});

// ---------------------------------------------------------------------------
// The two lists agree
// ---------------------------------------------------------------------------

test('every authorized page is also an authenticated page', () => {
    // A page in AUTHORIZED but not AUTHENTICATED would never reach the tier check,
    // because the outer `if` tests AUTHENTICATED first.
    for (const pattern of AUTHORIZED) {
        assert.ok(AUTHENTICATED.includes(pattern),
            `${pattern} is in AUTHORIZED_PAGES but not AUTHENTICATED_PAGES, so its tier check never runs`);
    }
});

// ---------------------------------------------------------------------------
// Nothing is left out
// ---------------------------------------------------------------------------

/*
 * The consumer pages, as their routers register them. Kept as a literal list
 * rather than parsed from the routers: the point is to state what the tier
 * SHOULD be, which no amount of reading the route file can tell you.
 */
const CONSUMER_PAGES = [
    `${ORG}/applications`,
    `${ORG}/applications/abc-123`,
    `${ORG}/api-keys`,
    `${ORG}/oauth2-keys`,
    `${ORG}/subscriptions`,
    '/acme/settings',
];

test('every consumer page is covered by both lists', () => {
    for (const p of CONSUMER_PAGES) {
        assert.ok(matches(AUTHENTICATED, p), `${p} is not in SYSTEM_AUTHENTICATED_PAGES`);
        assert.ok(matches(AUTHORIZED, p), `${p} is not in SYSTEM_AUTHORIZED_PAGES`);
    }
});

test('every page router registering a view-scoped page has a pattern covering it', () => {
    /*
     * The guard against the next omission. Reads the page routers, pulls the paths
     * they register, and asserts each one is matched. A page deliberately left
     * public (the catalogue, login) is listed as an exemption here, so leaving a
     * page out becomes a visible decision in this file rather than an oversight in
     * another one.
     */
    const PUBLIC_BY_DESIGN = [
        // Auth entry points -- reachable before there is a session to check.
        /\/login$/, /\/logout$/, /\/callback$/, /\/signin$/, /\/signup$/,
        // The catalogue and its documents: this portal exists to show these to
        // anonymous visitors, which is why no tier applies.
        /\/apis?(\/|\.|$)/, /\/mcps?(\/|\.|$)/, /\/mcp-servers?(\/|\.|$)/,
        /\/docs(\/|$)/, /\/specification/,
        /\/api-workflows(\/|\.|$)/,
        // Guarded by enforceSecurity(scope) in the router instead of by this gate.
        /\/llms(-config|\.txt)/, /\/v0\.1\//,
        // Tryout proxy and design-mode preview, both gated in their own routers.
        /\/tryout/, /\/design/,
        // Catch-alls serving operator-authored custom pages, which are public
        // content by definition.
        /\*splat/,
        // Landing pages: the org root, a view root, and the portal root.
        /^\/$/, /:orgName\/?$/, /views\/:viewName\/?$/, /^\/portal\//,
    ];

    const dir = path.join(__dirname, '..', 'routes', 'pages');
    const registered = new Set();
    for (const file of fs.readdirSync(dir).filter((f) => f.endsWith('.js'))) {
        const src = fs.readFileSync(path.join(dir, file), 'utf8');
        for (const m of src.matchAll(/router\.(?:get|post|put|delete)\(\s*'([^']+)'/g)) {
            registered.add(m[1]);
        }
    }
    assert.ok(registered.size > 0, 'found no registered page routes to check');

    const uncovered = [];
    for (const route of registered) {
        if (PUBLIC_BY_DESIGN.some((re) => re.test(route))) continue;
        // Substitute the params to get a concrete path the globs can be tested on.
        const concrete = route
            .replace(/:orgName/g, 'acme')
            .replace(/:viewName/g, 'default')
            .replace(/:[A-Za-z]+/g, 'x');
        if (!matches(AUTHENTICATED, concrete) || !matches(AUTHORIZED, concrete)) {
            uncovered.push(`${route}  (as ${concrete})`);
        }
    }

    assert.deepEqual(uncovered, [],
        'these page routes match neither access list, so ensureAuthenticated skips its ' +
        'entire auth block for them -- add a pattern to constants.js, or an exemption above');
});
