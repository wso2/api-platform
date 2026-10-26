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
 * Pages offer only the account actions the REST API will allow: pageAccess decides from the scopes
 * the request resolves to, against each action's operation in the spec. The
 * resolver and the scope check are replaced by test versions that apply OAuth2Security's rule: any
 * one of the operation's scopes is enough.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const stub = (relative, exports) => {
    const file = path.resolve(__dirname, relative);
    require.cache[file] = { id: file, filename: file, loaded: true, exports };
};
stub('../config/configLoader.js', { config: { designMode: { enabled: false } } });
let resolved; // the scopes the request resolves to, or 'refused'
stub('../middlewares/authMiddleware.js', {
    async authResolver(req, res, next) {
        if (resolved === 'refused') return next(Object.assign(new Error('Forbidden'), { status: 403 }));
        req.auth = { scopes: resolved };
        return next();
    },
    async OAuth2Security(req, scopes) {
        if (!scopes.some((scope) => req.auth.scopes.includes(scope))) throw Object.assign(new Error('Forbidden'), { status: 403 });
        return true;
    },
});

const { pageAccess } = require('./pageAccess');

const signedIn = () => ({ isAuthenticated: () => true });
const accessFor = (scopes) => { resolved = scopes; return pageAccess(signedIn()); };

test('a visitor who is not signed in may do nothing, but is offered Subscribe, which asks them to sign in', async () => {
    const access = await pageAccess({ isAuthenticated: () => false });
    assert.deepEqual(access, { subscribe: false, checkout: false, readInvoices: false, readUsage: false, manageBilling: false,
        signedIn: false, offerSubscribe: true });
});

test('a subscriber may subscribe, buy, read invoices and usage, and change payment details', async () => {
    const access = await accessFor(['dp:subscription:manage', 'dp:subscription_plan:read']);
    assert.deepEqual(access, { subscribe: true, checkout: true, readInvoices: true, readUsage: true, manageBilling: true,
        signedIn: true, offerSubscribe: true });
});

test('a user who may only read subscriptions sees invoices and usage, and is offered nothing to buy or change', async () => {
    const access = await accessFor(['dp:subscription:read']);
    assert.deepEqual(access, { subscribe: false, checkout: false, readInvoices: true, readUsage: true, manageBilling: false,
        signedIn: true, offerSubscribe: false });
});

test('a request the resolver refuses is offered nothing, and the resolver works on a copy of the request', async () => {
    resolved = 'refused';
    const req = signedIn();
    const access = await pageAccess(req);
    assert.equal(Object.values(access).filter(Boolean).length, 1, JSON.stringify(access));
    assert.equal(access.signedIn, true);
    resolved = ['dp:subscription:manage'];
    await pageAccess(req);
    assert.equal(req.auth, undefined, 'what the resolver finds stays off the page request');
});
