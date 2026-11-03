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
 * What the signed-in user may do through the portal's REST API, so that a page shows only the
 * actions that will work. Decided as the REST API decides it: the request is resolved by
 * authResolver, and each operation's scopes, read from the OpenAPI spec, are checked by
 * OAuth2Security. The REST API still enforces them; this only keeps pages from offering what it
 * would refuse.
 */

const fs = require('node:fs');
const path = require('node:path');
const yaml = require('./yaml');
const logger = require('../config/logger');
const { authResolver, OAuth2Security } = require('../middlewares/authMiddleware');

const SPEC_PATH = path.join(__dirname, '..', '..', 'docs', 'api-portal-openapi-spec-v0.9.yaml');

// The page actions, each with the REST operation it calls.
const ACTIONS = {
    subscribe: ['post', '/subscriptions'],
    checkout: ['post', '/checkout-sessions'],
    readInvoices: ['get', '/billing/invoices'],
    readUsage: ['get', '/usage'],
    manageBilling: ['post', '/billing/billing-portal-sessions'],
};

let actionScopes;

/** The scopes each action's operation accepts (any one of them), read from the spec once. */
function scopesByAction() {
    if (!actionScopes) {
        const spec = yaml.load(fs.readFileSync(SPEC_PATH, 'utf8'));
        actionScopes = {};
        for (const [action, [method, route]] of Object.entries(ACTIONS)) {
            const operation = spec.paths && spec.paths[route] && spec.paths[route][method];
            if (!operation) throw new Error(`The OpenAPI spec has no ${method.toUpperCase()} ${route}`);
            const security = operation.security || spec.security || [];
            actionScopes[action] = [...new Set(security.flatMap((requirement) => Object.values(requirement).flat()))];
        }
    }
    return actionScopes;
}

/**
 * What the user of `req` may do: { signedIn, subscribe, checkout, readInvoices, readUsage,
 * manageBilling, offerSubscribe }. A visitor who is not signed in may do none of them, but is
 * offered Subscribe (offerSubscribe), which asks them to sign in first.
 */
async function pageAccess(req) {
    const access = Object.fromEntries(Object.keys(ACTIONS).map((action) => [action, false]));
    access.signedIn = Boolean(req && typeof req.isAuthenticated === 'function' && req.isAuthenticated());
    if (access.signedIn) {
        try {
            // Resolved on a copy, since the resolver records what it finds on the request it is given.
            const probe = Object.create(req);
            const resolved = await new Promise((resolve) => {
                const res = { status() { return this; }, json() { resolve(false); return this; } };
                Promise.resolve(authResolver(probe, res, (err) => resolve(!err))).catch(() => resolve(false));
            });
            if (resolved && probe.auth) {
                for (const [action, scopes] of Object.entries(scopesByAction())) {
                    access[action] = await OAuth2Security(probe, scopes).catch(() => false);
                }
            }
        } catch (err) {
            logger.warn('Could not resolve what the user may do; the page offers no account actions', { error: err.message });
        }
    }
    access.offerSubscribe = !access.signedIn || access.subscribe || access.checkout;
    return access;
}

module.exports = { pageAccess, scopesByAction };
