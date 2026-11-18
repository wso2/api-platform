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
 * Key generation takes the API's row lock before it inserts a key, the same lock a
 * forced API delete takes before it snapshots the active keys, so a key cannot be
 * committed in between and then removed without a revocation event. Driven directly,
 * with the transaction and the data layer replaced.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-key-lock-'));
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
const apiKeyDao = require('../dao/apiKeyDao');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const apiKeyService = require('./apiKeyService');

const API_UUID = 'api-uuid-1';

/** Replaces the data layer; `apiGone` makes the lock find no API, as after a delete. */
function stub(t, { apiGone = false } = {}) {
    const calls = { locked: [], created: 0 };
    t.mock.method(db, 'withTransaction', (fn) => fn({}));
    t.mock.method(apiDao, 'get', async () => [{ uuid: API_UUID, name: 'Orders', version: 'v1', ref_id: 'orders', type: 'REST' }]);
    t.mock.method(apiDao, 'lockForUpdate', async (orgId, apiId) => { calls.locked.push(apiId); return !apiGone; });
    t.mock.method(apiKeyDao, 'getIdByHandle', async () => null);
    t.mock.method(apiKeyDao, 'create', async () => { calls.created++; return { uuid: 'key-uuid-1' }; });
    t.mock.method(userIdpReferenceDao, 'buildSingleAuditFields', async () => ({}));
    return calls;
}

const request = { orgId: 'org-1', apiId: API_UUID, handle: 'k1', actor: 'user-1' };

test('generate locks the API row before inserting the key', async (t) => {
    const calls = stub(t);
    // Only the lock is under test; what follows the insert may fail against the stub transaction.
    await apiKeyService.generate(request).catch(() => {});
    assert.deepStrictEqual(calls.locked, [API_UUID]);
});

test('generate refuses with 404, and creates no key, when the API was deleted first', async (t) => {
    const calls = stub(t, { apiGone: true });
    await assert.rejects(apiKeyService.generate(request), (err) => err.status === 404);
    assert.strictEqual(calls.created, 0);
});
