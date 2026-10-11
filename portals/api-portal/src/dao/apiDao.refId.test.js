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
 * ref_id defaults for API rows. Outbound webhook events carry data.api.ref_id, which
 * Platform API resolves by handle, so a portal-authored API defaults ref_id to its
 * handle. MCP servers are the exception: a null ref_id is what marks a server as
 * authored in this portal (mcpRegistryService only exposes those), so they keep it null.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const PATHS = {
    driver: path.resolve(__dirname, '../db/driver.js'),
    orgContext: path.resolve(__dirname, '../utils/orgContext.js'),
    logger: path.resolve(__dirname, '../config/logger.js'),
    dao: path.resolve(__dirname, './apiDao.js'),
};

function loadDao(statements) {
    const stubs = {
        driver: {
            execute: async (sql, params) => { statements.push({ sql, params }); return { rowCount: 1 }; },
            // update() re-reads the row it wrote; the shape is irrelevant to these tests.
            queryOne: async () => ({ uuid: 'api-1' }),
            query: async () => [],
        },
        orgContext: { getPortalId: () => 'portal-a' },
        logger: { error: () => {}, warn: () => {}, info: () => {}, debug: () => {} },
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

const metadata = (overrides = {}) => ({
    name: 'Pets',
    handle: 'pets-v1',
    version: 'v1',
    status: 'PUBLISHED',
    type: 'RestApi',
    endPoints: { sandboxURL: 'https://sandbox.example.com', productionURL: 'https://prod.example.com' },
    ...overrides,
});

// The INSERT binds ref_id second, right after the generated uuid.
const insertedRefId = (statements) => statements[0].params[1];

test('a portal-authored API defaults ref_id to its handle', async () => {
    const statements = [];
    const row = await loadDao(statements).create('org-1', metadata(), 'admin');
    assert.strictEqual(row.ref_id, 'pets-v1');
    assert.strictEqual(insertedRefId(statements), 'pets-v1');
});

test('an explicit referenceId wins over the handle', async () => {
    const statements = [];
    const row = await loadDao(statements).create('org-1', metadata({ referenceId: 'cp-pets' }), 'admin');
    assert.strictEqual(row.ref_id, 'cp-pets');
});

test('a portal-authored MCP server keeps a null ref_id so the registry still exposes it', async () => {
    const statements = [];
    const row = await loadDao(statements).create('org-1', metadata({ type: 'Mcp', handle: 'weather-v1' }), 'admin');
    assert.strictEqual(row.ref_id, null);
    assert.strictEqual(insertedRefId(statements), null);
});

test('an update without referenceId keeps the stored ref_id, falling back to the handle', async () => {
    const statements = [];
    await loadDao(statements).update('org-1', 'api-1', metadata(), 'admin');
    const { sql, params } = statements[0];
    assert.match(sql, /SET ref_id = COALESCE\(\?, ref_id, \?\)/);
    assert.deepStrictEqual(params.slice(0, 2), [null, 'pets-v1']);
});

test('an MCP update never backfills ref_id with the handle', async () => {
    const statements = [];
    await loadDao(statements).update('org-1', 'api-1', metadata({ type: 'Mcp', handle: 'weather-v1' }), 'admin');
    assert.deepStrictEqual(statements[0].params.slice(0, 2), [null, null]);
});
