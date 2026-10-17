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

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-sub-dao-test-'));
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

const PATHS = {
    driver: path.resolve(__dirname, '../db/driver.js'),
    orgContext: path.resolve(__dirname, '../utils/orgContext.js'),
    dao: path.resolve(__dirname, './subscriptionDao.js'),
};

function loadDao(queryOneResult, statements = []) {
    const stubs = {
        driver: {
            execute: async (sql, params) => { statements.push({ sql, params }); return { rowCount: 1 }; },
            queryOne: async (sql, params) => { statements.push({ sql, params }); return queryOneResult; },
            query: async () => [],
            isDuplicateKeyError: (err) => Boolean(err && (err.code === 'SQLITE_CONSTRAINT_UNIQUE' || err.message?.includes('UNIQUE constraint failed'))),
        },
        orgContext: { getPortalId: () => 'portal-a' },
    };
    for (const [k, exports] of Object.entries(stubs)) {
        require.cache[PATHS[k]] = { id: PATHS[k], filename: PATHS[k], loaded: true, exports };
    }
    delete require.cache[PATHS.dao];
    try {
        return require(PATHS.dao);
    } finally {
        for (const p of Object.values(PATHS)) delete require.cache[p];
    }
}

test('findByApiAndUser queries by org_uuid, portal_id, api_uuid and created_by', async () => {
    const statements = [];
    const expected = { uuid: 'sub-1', created_by: 'user-1', api_uuid: 'api-1' };
    const dao = loadDao(expected, statements);

    const result = await dao.findByApiAndUser('org-1', 'api-1', 'user-1');
    assert.deepStrictEqual(result, expected);
    assert.strictEqual(statements.length, 1);
    assert.match(statements[0].sql, /WHERE org_uuid = \? AND portal_id = \? AND api_uuid = \? AND created_by = \?/);
    assert.deepStrictEqual(statements[0].params, ['org-1', 'portal-a', 'api-1', 'user-1']);
});

test('findByApiAndUser returns null when not found', async () => {
    const statements = [];
    const dao = loadDao(null, statements);

    const result = await dao.findByApiAndUser('org-1', 'api-1', 'user-1');
    assert.strictEqual(result, null);
});

test('create throws immediately on subscription unique violation without retrying', async () => {
    let attempts = 0;
    const stubs = {
        driver: {
            execute: async () => {
                attempts++;
                const err = new Error('UNIQUE constraint failed: subscriptions.org_uuid, subscriptions.api_uuid, subscriptions.created_by, subscriptions.portal_id');
                err.code = 'SQLITE_CONSTRAINT_UNIQUE';
                throw err;
            },
            queryOne: async () => null,
            query: async () => [],
            isDuplicateKeyError: (err) => Boolean(err && (err.code === 'SQLITE_CONSTRAINT_UNIQUE' || err.message?.includes('UNIQUE constraint failed'))),
        },
        orgContext: { getPortalId: () => 'portal-a' },
    };
    for (const [k, exports] of Object.entries(stubs)) {
        require.cache[PATHS[k]] = { id: PATHS[k], filename: PATHS[k], loaded: true, exports };
    }
    delete require.cache[PATHS.dao];
    const dao = require(PATHS.dao);

    await assert.rejects(
        () => dao.create('org-1', 'api-1', 'plan-1', 'user-1'),
        (err) => {
            assert.strictEqual(attempts, 1);
            assert.strictEqual(err.isTokenCollision, undefined);
            return true;
        }
    );
});

test('create marks exhausted token collisions with isTokenCollision = true', async () => {
    let attempts = 0;
    const stubs = {
        driver: {
            execute: async () => {
                attempts++;
                const err = new Error('UNIQUE constraint failed: subscriptions.token');
                err.code = 'SQLITE_CONSTRAINT_UNIQUE';
                throw err;
            },
            queryOne: async () => null,
            query: async () => [],
            isDuplicateKeyError: (err) => Boolean(err && (err.code === 'SQLITE_CONSTRAINT_UNIQUE' || err.message?.includes('UNIQUE constraint failed'))),
        },
        orgContext: { getPortalId: () => 'portal-a' },
    };
    for (const [k, exports] of Object.entries(stubs)) {
        require.cache[PATHS[k]] = { id: PATHS[k], filename: PATHS[k], loaded: true, exports };
    }
    delete require.cache[PATHS.dao];
    const dao = require(PATHS.dao);

    await assert.rejects(
        () => dao.create('org-1', 'api-1', 'plan-1', 'user-1'),
        (err) => {
            assert.strictEqual(attempts, 3);
            assert.strictEqual(err.isTokenCollision, true);
            return true;
        }
    );
});
