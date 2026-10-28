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
 * The guard branches of the bulk subscription plan handlers. The route validator
 * already rejects an empty list and the DAO either returns a plan or throws, so
 * no request reaches these branches; they exist so a bulk call can never answer
 * success for plans that were not saved. Driven directly, with the transaction
 * and the DAO replaced.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-plans-'));
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
const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const { addSubscriptionPlans, putSubscriptionPlans } = require('./apiMetadataService');

function call(handler, body, files) {
    return new Promise((resolve) => {
        const res = {
            statusCode: 200,
            status(code) { this.statusCode = code; return this; },
            json(payload) { resolve({ status: this.statusCode, body: payload }); return this; },
            send(payload) { resolve({ status: this.statusCode, body: payload }); return this; },
        };
        const req = { orgId: 'org-1', user: { sub: 'admin-1' }, query: {}, params: {}, body, files };
        Promise.resolve(handler(req, res)).catch((error) => resolve({ status: 'THREW', body: error.message }));
    });
}

const inTransaction = (t) => t.mock.method(db, 'withTransaction', (fn) => fn({}));

test('bulk create refuses an empty list with 400', async () => {
    const res = await call(addSubscriptionPlans, []);
    assert.strictEqual(res.status, 400);
});

test('bulk create answers 500 when a plan was not created', async (t) => {
    inTransaction(t);
    t.mock.method(subscriptionPlanDao, 'create', async () => null);

    const res = await call(addSubscriptionPlans, [{ id: 'gold', displayName: 'Gold' }]);
    assert.strictEqual(res.status, 500);
    assert.strictEqual(res.body.errors[0].message, 'Failed to create plan: gold');
});

test('bulk update refuses an empty list with 400', async () => {
    const res = await call(putSubscriptionPlans, []);
    assert.strictEqual(res.status, 400);
});

test('bulk update answers 500 when a plan was not saved', async (t) => {
    inTransaction(t);
    t.mock.method(subscriptionPlanDao, 'put', async () => ({}));

    const res = await call(putSubscriptionPlans, [{ id: 'gold', displayName: 'Gold' }]);
    assert.strictEqual(res.status, 500);
    assert.strictEqual(res.body.errors[0].message, 'Failed to upsert plan: gold');
});

// A plan from YAML takes metadata.name as its id, as APIs, applications and key managers do, so the
// same YAML applied again updates that plan instead of creating another.
const yamlFile = (text) => ({ subscriptionPlan: [{ buffer: Buffer.from(text) }] });
const handlesSaved = (t, method) => {
    const handles = [];
    t.mock.method(subscriptionPlanDao, method, async (orgId, plan) => { handles.push(plan.handle); return null; });
    return handles;
};

test('a plan created from YAML takes metadata.name as its id', async (t) => {
    inTransaction(t);
    const handles = handlesSaved(t, 'create');
    await call(addSubscriptionPlans, {}, yamlFile('kind: SubscriptionPlan\nmetadata:\n  name: gold\nspec:\n  displayName: Gold\n'));
    assert.deepStrictEqual(handles, ['gold']);
});

test('a plan in a YAML list takes its metadata.name as its id', async (t) => {
    inTransaction(t);
    const handles = handlesSaved(t, 'create');
    await call(addSubscriptionPlans, {}, yamlFile('kind: SubscriptionPlanList\nitems:\n'
        + '  - metadata:\n      name: gold\n    spec:\n      displayName: Gold\n'
        + '  - metadata:\n      name: silver\n    spec:\n      displayName: Silver\n'));
    assert.deepStrictEqual(handles, ['gold']);
});

test('a plan updated from YAML is found by its metadata.name', async (t) => {
    inTransaction(t);
    const handles = handlesSaved(t, 'put');
    await call(putSubscriptionPlans, {}, yamlFile('kind: SubscriptionPlan\nmetadata:\n  name: gold\nspec:\n  displayName: Gold plus\n'));
    assert.deepStrictEqual(handles, ['gold']);
});

test('a plan from YAML without metadata.name gets a generated id', async (t) => {
    inTransaction(t);
    const handles = handlesSaved(t, 'create');
    await call(addSubscriptionPlans, {}, yamlFile('kind: SubscriptionPlan\nspec:\n  displayName: Gold\n'));
    assert.strictEqual(handles.length, 1);
    assert.match(handles[0], /^[0-9a-f-]{36}$/);
});
