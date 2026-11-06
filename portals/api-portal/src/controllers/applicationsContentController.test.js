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
 * Which OAuth2 keys the application page offers for association.
 *
 * An application holds at most one key per key type, so a key whose type this
 * application already has would be refused with a 409 — the picker must not list
 * it. "Available" still includes keys sitting on *another* application, because
 * a key belongs to one application at a time and choosing one here moves it;
 * hiding those would make a key the caller owns look as though it had vanished.
 *
 * The template is never rendered: `renderTemplate` is replaced with a capture, so
 * what is asserted is exactly the model the page is handed.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const stub = (file, exports) => {
    require.cache[file] = { id: file, filename: file, loaded: true, exports };
};
const local = (relative) => path.resolve(__dirname, relative);

let keyRecords;   // every key the caller created
let keyToApp;     // keyId -> application uuid
let captured;     // the model handed to the template

const THIS_APP = 'uuid-this-app';
const OTHER_APP = 'uuid-other-app';

stub(local('../config/logger.js'), {
    warn: () => {}, error: () => {}, info: () => {}, debug: () => {},
});
stub(local('../config/configLoader.js'), { config: {} });
stub(local('../utils/util.js'), {
    // Both render paths capture: which one runs depends on whether the view has
    // a layout of its own, and this assertion is about neither.
    renderTemplate: (_tpl, _layout, content) => { captured = content; return '<html></html>'; },
    renderGivenTemplate: async (_tpl, _layout, content) => { captured = content; return '<html></html>'; },
    loadLayoutFromAPI: async () => '',
    resolveActor: () => 'alice',
    pageErrorStatus: () => 500,
});
stub(local('../dao/organizationDao.js'), { get: async () => ({ uuid: 'org-1' }) });
stub(local('../dao/applicationDao.js'), {
    getId: async () => ({ uuid: THIS_APP }),
    get: async (_orgId, uuid) => ({ handle: `handle-${uuid}`, display_name: `Name ${uuid}` }),
    getKeyMappings: async () => [],
});
stub(local('../dao/keyManagerDao.js'), { get: async () => ({ handle: 'km' }), listEnabled: async () => [] });
stub(local('../services/adminService.js'), { getApplicationKeyMap: async () => ({ appKeyMappings: [] }) });
stub(local('../services/apiKeyService.js'), {});
stub(local('../utils/sampleApiLoader.js'), {});
stub(local('../services/keyManagerRegistry.js'), { list: async () => [] });
stub(local('../dao/oauth2ConsumerKeyDao.js'), { listByCreator: async () => keyRecords });
stub(local('../dao/oauth2KeyAppMappingDao.js'), { getByKeys: async () => keyToApp });

const { loadApplication } = require('./applicationsContentController');

/*
 * `templateResponseValue` resolves the page template from `require.main.filename`,
 * which under the test runner is this file rather than the server entry point.
 * Pointing it at the real entry point lets the controller read the real template,
 * so the path it builds stays exercised instead of stubbed away. Set after the
 * require above, which resolves against the same field.
 */
require.main.filename = path.resolve(__dirname, '../server.js');

/*
 * Render the application page for one set of keys, and return the key ids the
 * associate picker would offer.
 */
async function offeredKeyIds({ keys, mappings }) {
    keyRecords = keys;
    keyToApp = new Map(Object.entries(mappings));
    captured = undefined;

    const req = {
        params: { orgName: 'acme', viewName: 'default', applicationId: 'checkout-app' },
        user: { sub: 'alice' },
        isAuthenticated: () => true,
    };
    const res = { send: () => {}, redirect: () => {} };
    await loadApplication(req, res, (err) => { throw err; });

    return (captured.availableOAuth2Keys || []).map((k) => k.keyId);
}

test('a key whose type this application already holds is not offered', async () => {
    const offered = await offeredKeyIds({
        keys: [
            { keyId: 'mine-prod', keyType: 'PRODUCTION' },
            { keyId: 'spare-prod', keyType: 'PRODUCTION' },
            { keyId: 'spare-sandbox', keyType: 'SANDBOX' },
        ],
        mappings: { 'mine-prod': THIS_APP },
    });

    // The production slot is taken, so only the sandbox key is selectable.
    assert.deepEqual(offered, ['spare-sandbox']);
});

test('a key on another application is not offered', async () => {
    // Taking it would detach it from an application the caller is not looking at —
    // the same side effect the 409 refuses within one application. Moving a key is
    // the OAuth2 Keys page's job, and the modal hint says so.
    const offered = await offeredKeyIds({
        keys: [{ keyId: 'elsewhere-sandbox', keyType: 'SANDBOX' }],
        mappings: { 'elsewhere-sandbox': OTHER_APP },
    });

    assert.deepEqual(offered, []);
});

test('a key on another application is not offered even when its type is free here', async () => {
    // The type being free is what used to let it through: the slot filter had
    // nothing to say about it, and the "available" split sent it on regardless of
    // where it already was.
    const offered = await offeredKeyIds({
        keys: [
            { keyId: 'elsewhere-prod', keyType: 'PRODUCTION' },
            { keyId: 'unattached-prod', keyType: 'PRODUCTION' },
        ],
        mappings: { 'elsewhere-prod': OTHER_APP },
    });

    assert.deepEqual(offered, ['unattached-prod']);
});

test('both slots taken leaves nothing to offer', async () => {
    const offered = await offeredKeyIds({
        keys: [
            { keyId: 'mine-prod', keyType: 'PRODUCTION' },
            { keyId: 'mine-sandbox', keyType: 'SANDBOX' },
            { keyId: 'spare-prod', keyType: 'PRODUCTION' },
            { keyId: 'spare-sandbox', keyType: 'SANDBOX' },
        ],
        mappings: { 'mine-prod': THIS_APP, 'mine-sandbox': THIS_APP },
    });

    assert.deepEqual(offered, []);
});

test('with no key associated yet, every unattached key of the caller is offered', async () => {
    const offered = await offeredKeyIds({
        keys: [
            { keyId: 'spare-prod', keyType: 'PRODUCTION' },
            { keyId: 'spare-sandbox', keyType: 'SANDBOX' },
        ],
        mappings: {},
    });

    assert.deepEqual(offered.sort(), ['spare-prod', 'spare-sandbox']);
});

test('keys parked on other applications are counted for the picker', async () => {
    // Counted rather than listed: the picker needs to explain an empty list without
    // this view loading applications it has no other reason to know about.
    await offeredKeyIds({
        keys: [
            { keyId: 'elsewhere-a', keyType: 'PRODUCTION' },
            { keyId: 'elsewhere-b', keyType: 'SANDBOX' },
            { keyId: 'unattached', keyType: 'PRODUCTION' },
        ],
        mappings: { 'elsewhere-a': OTHER_APP, 'elsewhere-b': OTHER_APP },
    });

    assert.equal(captured.oauth2KeysElsewhere, 2);
});

test('a key on this application is not counted as elsewhere', async () => {
    await offeredKeyIds({
        keys: [{ keyId: 'mine', keyType: 'PRODUCTION' }],
        mappings: { mine: THIS_APP },
    });

    assert.equal(captured.oauth2KeysElsewhere, 0);
});
