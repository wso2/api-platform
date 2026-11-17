/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/*
 * DELETE /apis/{apiId} with and without ?force=true. Without it, an API with
 * subscriptions or active keys is refused; with it, its free subscriptions are
 * removed and its active keys revoked first, through the same calls a manual
 * removal makes, and a paid subscription still refuses. Driven directly, with
 * the transaction and the data layer replaced.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-force-delete-'));
test.after(() => fs.rmSync(tmpDir, { recursive: true, force: true }));

const cfgFile = path.join(tmpDir, 'config.toml');
fs.writeFileSync(cfgFile, `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"

[api_portal.database]
type = "sqlite"
path = ${JSON.stringify(path.join(tmpDir, 'portal.db'))}
`);
process.argv.push('--config', cfgFile);

const db = require('../db/driver');
const apiDao = require('../dao/apiDao');
const subDao = require('../dao/subscriptionDao');
const apiKeyDao = require('../dao/apiKeyDao');
const billingSubscriptionDao = require('../dao/billingSubscriptionDao');
const billingApiPlanDao = require('../dao/billingApiPlanDao');
const checkoutService = require('./billing/checkoutService');
const subscriptionService = require('./subscriptionService');
const apiKeyService = require('./apiKeyService');
const { deleteAPIMetadata } = require('./apiMetadataService');

const API_UUID = 'api-uuid-1';

function call(query = {}) {
    return new Promise((resolve) => {
        const res = {
            statusCode: 200,
            status(code) { this.statusCode = code; return this; },
            json(payload) { resolve({ status: this.statusCode, body: payload }); return this; },
            send(payload) { resolve({ status: this.statusCode, body: payload }); return this; },
        };
        const req = { orgId: 'org-1', user: { sub: 'admin-1' }, query, params: { apiId: 'orders' }, body: {} };
        Promise.resolve(deleteAPIMetadata(req, res)).catch((error) => resolve({ status: 'THREW', body: error.message }));
    });
}

/**
 * Replaces the data layer for an API with `subs` (subscription rows), `keys` (active key
 * rows) and `paid` (the uuids of subs billed through the provider). Returns what was
 * removed, revoked and deleted.
 */
function stubApi(t, { subs = [], keys = [], paid = [] } = {}) {
    const done = { removedSubs: [], revokedKeys: [], apiDeleted: false };
    t.mock.method(db, 'withTransaction', (fn) => fn({}));
    t.mock.method(apiDao, 'getIdExcludingType', async () => API_UUID);
    t.mock.method(apiDao, 'delete', async () => { done.apiDeleted = true; return 1; });
    t.mock.method(subDao, 'listByApi', async () => subs);
    t.mock.method(subDao, 'getById', async (orgId, uuid) => ({ uuid, api_metadata: {}, subscription_plan: {} }));
    t.mock.method(apiKeyDao, 'list', async () => keys);
    t.mock.method(apiKeyDao, 'deleteByApi', async () => 0);
    t.mock.method(billingSubscriptionDao, 'listBySubscriptions', async (uuids) =>
        new Map(uuids.filter((uuid) => paid.includes(uuid)).map((uuid) => [uuid, { subscription_uuid: uuid }])));
    t.mock.method(billingApiPlanDao, 'listByOrg', async () => []);
    t.mock.method(checkoutService, 'endOpenCheckoutsFor', async () => {});
    t.mock.method(subscriptionService, 'removeSubscription', async (orgId, sub) => { done.removedSubs.push(sub.uuid); return true; });
    t.mock.method(apiKeyService, 'revokeInTransaction', async ({ existing }) => { done.revokedKeys.push(existing.uuid); return true; });
    return done;
}

test('without force, an API with subscriptions is refused with 409 and kept', async (t) => {
    const done = stubApi(t, { subs: [{ uuid: 'sub-1' }] });
    const res = await call();
    assert.strictEqual(res.status, 409);
    assert.strictEqual(done.apiDeleted, false);
    assert.deepStrictEqual(done.removedSubs, []);
});

test('without force, an API with active keys is refused with 409 and kept', async (t) => {
    const done = stubApi(t, { keys: [{ uuid: 'key-1' }] });
    const res = await call();
    assert.strictEqual(res.status, 409);
    assert.strictEqual(done.apiDeleted, false);
    assert.deepStrictEqual(done.revokedKeys, []);
});

test('with force, free subscriptions are removed and active keys revoked before the API is deleted', async (t) => {
    const done = stubApi(t, { subs: [{ uuid: 'sub-1' }, { uuid: 'sub-2' }], keys: [{ uuid: 'key-1' }] });
    const res = await call({ force: 'true' });
    assert.strictEqual(res.status, 200);
    assert.deepStrictEqual(done.removedSubs, ['sub-1', 'sub-2']);
    assert.deepStrictEqual(done.revokedKeys, ['key-1']);
    assert.strictEqual(done.apiDeleted, true);
});

test('with force, a paid subscription still refuses the delete and nothing is removed', async (t) => {
    const done = stubApi(t, { subs: [{ uuid: 'sub-1' }, { uuid: 'sub-paid' }], keys: [{ uuid: 'key-1' }], paid: ['sub-paid'] });
    const res = await call({ force: true });
    assert.strictEqual(res.status, 409);
    assert.strictEqual(done.apiDeleted, false);
    assert.deepStrictEqual(done.removedSubs, []);
    assert.deepStrictEqual(done.revokedKeys, []);
});

test('with force, an API with nothing attached is deleted as before', async (t) => {
    const done = stubApi(t);
    const res = await call({ force: 'true' });
    assert.strictEqual(res.status, 200);
    assert.strictEqual(done.apiDeleted, true);
});
