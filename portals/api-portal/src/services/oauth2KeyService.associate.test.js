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
 * The one-key-per-key-type rule in associateOAuth2KeyApplication, exercised in
 * process.
 *
 * oauth2KeyService.handlers.test.js already drives this rule end to end, but it
 * does so by spawning a child process and reading a PROBE_JSON line back out.
 * `node --test --experimental-test-coverage` instruments only the test process,
 * so every line those scenarios execute is invisible to coverage — the rule is
 * tested, and reported as untested. These cases call the handler directly so the
 * branch is counted, and assert the things the spawned suite cannot see from
 * outside: which arguments reach the conflict query, and whether `associate` was
 * reached at all.
 *
 * The DAOs are replaced through require.cache, the same mechanism
 * authController.test.js uses. `util` is left real — the 409 body and status
 * come from its own sendError, so a change in that shape fails here rather than
 * being papered over by a stub that agrees with the assertion.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const stub = (file, exports) => {
    require.cache[file] = { id: file, filename: file, loaded: true, exports };
};
const local = (relative) => path.resolve(__dirname, relative);

// Mutable per-test state the stubs read. Reset by `scenario()` below.
let keyRecord;
let appRow;
let conflictKeyId;
let conflictArgs;
let associateArgs;

stub(local('../config/configLoader.js'), { config: { database: { driver: 'sqlite' } } });
stub(local('../config/logger.js'), {
    warn: () => {}, error: () => {}, info: () => {}, debug: () => {},
});
stub(local('../db/driver.js'), {
    queryOne: async () => null, query: async () => [], execute: async () => ({}),
});
stub(local('../dao/organizationDao.js'), {});
stub(local('../dao/apiDao.js'), {});
stub(local('../dao/subscriptionPlanDao.js'), {});
stub(local('../middlewares/auditLogger.js'), { logUserAction: () => {} });
stub(local('../keymanagers/index.js'), { getFactory: () => null });
stub(local('../keymanagers/core/keyManager.js'), { KeyManagerCallError: class extends Error {} });
stub(local('../dao/keyManagerConfigurationDao.js'), {});
stub(local('./keyManagerRegistry.js'), {});

stub(local('../dao/oauth2ConsumerKeyDao.js'), { get: async () => keyRecord });
stub(local('../dao/applicationDao.js'), {
    getId: async () => appRow,
    get: async () => ({ handle: 'checkout-app', display_name: 'Checkout App' }),
});
stub(local('../dao/oauth2KeyAppMappingDao.js'), {
    findConflictingKeyId: async (...args) => { conflictArgs = args; return conflictKeyId; },
    associate: async (...args) => { associateArgs = args; },
});

const constants = require('../utils/constants');
const { associateOAuth2KeyApplication } = require('./oauth2KeyService');

/* A response double shaped like the slice of Express the handler touches. */
function response() {
    return {
        statusCode: null,
        body: null,
        status(code) { this.statusCode = code; return this; },
        json(payload) { this.body = payload; return this; },
    };
}

/*
 * One association attempt. `key` is the row oauth2KeyDao.get returns, `conflict`
 * is the id findConflictingKeyId reports (null when the slot is free).
 */
async function associate({ key, conflict }) {
    keyRecord = key;
    appRow = { uuid: 'app-uuid-1' };
    conflictKeyId = conflict;
    conflictArgs = undefined;
    associateArgs = undefined;

    const req = {
        orgId: 'org-1',
        params: { keyId: 'key-being-added' },
        body: { applicationId: 'checkout-app' },
        auth: { userId: 'alice' },
    };
    const res = response();
    await associateOAuth2KeyApplication(req, res);
    return res;
}

test('a second key of a type the application already holds is refused', async () => {
    const res = await associate({
        key: { keyId: 'key-being-added', keyType: constants.KEY_TYPE.PRODUCTION },
        conflict: 'incumbent-key',
    });

    assert.equal(res.statusCode, 409);
    assert.match(res.body.message, /already associated with this application/);
    // The incumbent is what the rule exists to protect: it must still be
    // attached, and its id must not leak to a caller who cannot already read it.
    assert.equal(associateArgs, undefined);
    assert.ok(!JSON.stringify(res.body).includes('incumbent-key'));
});

test('the conflict is looked up for this application, type and key', async () => {
    await associate({
        key: { keyId: 'key-being-added', keyType: constants.KEY_TYPE.PRODUCTION },
        conflict: 'incumbent-key',
    });

    // Excluding the key being associated is what makes re-associating a key to
    // the application it already sits on a no-op rather than a self-conflict.
    assert.deepEqual(conflictArgs, ['app-uuid-1', constants.KEY_TYPE.PRODUCTION, 'key-being-added']);
});

test('a free slot associates and returns the application', async () => {
    const res = await associate({
        key: { keyId: 'key-being-added', keyType: constants.KEY_TYPE.SANDBOX },
        conflict: null,
    });

    assert.equal(res.statusCode, 200);
    assert.deepEqual(res.body, { application: { id: 'checkout-app', displayName: 'Checkout App' } });
    assert.deepEqual(associateArgs, ['key-being-added', 'app-uuid-1', 'alice']);
});

test('the slot checked follows the key own type, not a fixed one', async () => {
    await associate({
        key: { keyId: 'key-being-added', keyType: constants.KEY_TYPE.SANDBOX },
        conflict: null,
    });

    assert.equal(conflictArgs[1], constants.KEY_TYPE.SANDBOX);
});

test('a key stored without a type is treated as production', async () => {
    // Rows predating the key-type column read back with keyType undefined. They
    // must occupy the production slot rather than a third, typeless one that
    // would let an unlimited number of them onto one application.
    await associate({ key: { keyId: 'key-being-added' }, conflict: null });

    assert.equal(conflictArgs[1], constants.KEY_TYPE.PRODUCTION);
});
