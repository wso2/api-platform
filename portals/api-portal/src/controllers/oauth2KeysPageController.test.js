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
 * `takenTypes` on the associate picker's application list.
 *
 * The page greys out an application that already holds a key of the type being
 * associated, so the developer is not offered a choice the service would refuse
 * with a 409. That list is built here, from the two maps the key rows already
 * use, and nothing downstream recomputes it — a wrong entry here is either a
 * disabled option that should have been selectable, or an offered one that
 * fails on submit.
 *
 * The template is never rendered: `renderTemplateWithView` is replaced with a
 * capture, so what is asserted is exactly the model the page is handed.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const stub = (file, exports) => {
    require.cache[file] = { id: file, filename: file, loaded: true, exports };
};
const local = (relative) => path.resolve(__dirname, relative);

let keyRecords;      // what oauth2KeyDao.listByCreator returns
let keyToApp;        // keyId -> application uuid
let appsByUuid;      // application uuid -> row
let appList;         // what applicationDao.list returns
let captured;        // the model handed to the template

stub(local('../config/logger.js'), {
    warn: () => {}, error: () => {}, info: () => {}, debug: () => {},
});
stub(local('../utils/util.js'), {
    renderTemplateWithView: async (_tpl, _layout, content) => { captured = content; return '<html></html>'; },
    resolveActor: () => 'alice',
    pageErrorStatus: () => 500,
});
stub(local('../dao/organizationDao.js'), { get: async () => ({ uuid: 'org-1' }) });
stub(local('../services/keyManagerRegistry.js'), { list: async () => [] });
stub(local('../dao/oauth2ConsumerKeyDao.js'), { listByCreator: async () => { if (keyRecords instanceof Error) throw keyRecords; return keyRecords; } });
stub(local('../dao/oauth2KeyAppMappingDao.js'), { getByKeys: async () => keyToApp });
stub(local('../dao/applicationDao.js'), {
    get: async (_orgId, uuid) => appsByUuid.get(uuid) || null,
    list: async () => appList,
});

const { loadOAuth2Keys } = require('./oauth2KeysPageController');

/*
 * Render the page for one arrangement of keys and applications, and return the
 * `applications` entry for each handle, keyed by handle for readable assertions.
 */
async function pickerApplications({ keys, mappings, applications, resolvable = applications }) {
    keyRecords = keys;
    keyToApp = new Map(Object.entries(mappings));
    appsByUuid = new Map(resolvable.map((a) => [a.uuid, { handle: a.handle, display_name: a.name }]));
    appList = applications.map((a) => ({ handle: a.handle, display_name: a.name }));
    captured = undefined;

    const req = { params: { orgName: 'acme', viewName: 'default' }, user: { sub: 'alice' } };
    const res = { send: () => {}, redirect: () => {} };
    await loadOAuth2Keys(req, res, (err) => { throw err; });

    const byHandle = new Map();
    for (const app of captured.applications) byHandle.set(app.id, app);
    return byHandle;
}

const CHECKOUT = { uuid: 'uuid-checkout', handle: 'checkout-app', name: 'Checkout' };
const BILLING = { uuid: 'uuid-billing', handle: 'billing-app', name: 'Billing' };

test('an application reports the key types it already holds', async () => {
    const apps = await pickerApplications({
        keys: [{ keyId: 'k1', keyType: 'PRODUCTION' }],
        mappings: { k1: 'uuid-checkout' },
        applications: [CHECKOUT, BILLING],
    });

    assert.deepEqual(apps.get('checkout-app').takenTypes, ['PRODUCTION']);
    // An application holding nothing must come back with an empty list rather
    // than undefined: the picker indexes into it without a guard.
    assert.deepEqual(apps.get('billing-app').takenTypes, []);
});

test('an application holding both types has both slots taken', async () => {
    const apps = await pickerApplications({
        keys: [{ keyId: 'k1', keyType: 'PRODUCTION' }, { keyId: 'k2', keyType: 'SANDBOX' }],
        mappings: { k1: 'uuid-checkout', k2: 'uuid-checkout' },
        applications: [CHECKOUT],
    });

    assert.deepEqual(apps.get('checkout-app').takenTypes.sort(), ['PRODUCTION', 'SANDBOX']);
});

test('an unassociated key takes no slot anywhere', async () => {
    const apps = await pickerApplications({
        keys: [{ keyId: 'k1', keyType: 'PRODUCTION' }],
        mappings: {},
        applications: [CHECKOUT],
    });

    assert.deepEqual(apps.get('checkout-app').takenTypes, []);
});

test('a key mapped to an application that did not resolve takes no slot', async () => {
    // The mapping row outlives the application it points at — a deleted app, or
    // one the caller cannot read. Without the handle there is no list to add to,
    // and inventing one would grey out an unrelated application.
    const apps = await pickerApplications({
        keys: [{ keyId: 'k1', keyType: 'PRODUCTION' }],
        mappings: { k1: 'uuid-vanished' },
        applications: [CHECKOUT],
        resolvable: [CHECKOUT],
    });

    assert.deepEqual(apps.get('checkout-app').takenTypes, []);
});

test('a key stored without a type takes the production slot', async () => {
    const apps = await pickerApplications({
        keys: [{ keyId: 'k1' }],
        mappings: { k1: 'uuid-checkout' },
        applications: [CHECKOUT],
    });

    assert.deepEqual(apps.get('checkout-app').takenTypes, ['PRODUCTION']);
});

test('a failed key load degrades the page instead of failing it', async () => {
    // The picker and the key list come from the same try block. When it throws,
    // the page still renders — with the error banner and an empty list — rather
    // than becoming a 500, because the rest of the page is unaffected.
    keyRecords = new Error('database unavailable');
    keyToApp = new Map();
    appsByUuid = new Map();
    appList = [];
    captured = undefined;

    const req = { params: { orgName: 'acme', viewName: 'default' }, user: { sub: 'alice' } };
    const res = { send: () => {}, redirect: () => {} };
    await loadOAuth2Keys(req, res, (err) => { throw err; });

    assert.equal(captured.keysLoadError, true);
    assert.deepEqual(captured.oauth2Keys, []);
    assert.deepEqual(captured.applications, []);
});
