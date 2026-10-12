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
 * ref_id on API updates. A publisher push sets ref_id (Platform API sends its API
 * handle as referenceId) and webhook events carry it, so an update that omits
 * referenceId, such as an edit in the portal UI, must keep the stored value.
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

test('an update with referenceId writes it', async () => {
    const statements = [];
    await loadDao(statements).update('org-1', 'api-1', metadata({ referenceId: 'cp-pets' }), 'admin');
    const { sql, params } = statements[0];
    assert.match(sql, /SET ref_id = COALESCE\(\?, ref_id\)/);
    assert.strictEqual(params[0], 'cp-pets');
});

test('an update without referenceId keeps the stored ref_id', async () => {
    const statements = [];
    await loadDao(statements).update('org-1', 'api-1', metadata(), 'admin');
    const { sql, params } = statements[0];
    // COALESCE(NULL, ref_id) leaves the column as it was.
    assert.match(sql, /SET ref_id = COALESCE\(\?, ref_id\)/);
    assert.strictEqual(params[0], null);
    // The next bind is status: no extra fallback parameter.
    assert.strictEqual(params[1], 'PUBLISHED');
});

test('an empty referenceId on update is treated as omitted', async () => {
    const statements = [];
    await loadDao(statements).update('org-1', 'api-1', metadata({ referenceId: '' }), 'admin');
    assert.strictEqual(statements[0].params[0], null);
});

test('create writes the given referenceId and no default', async () => {
    const withRef = [];
    await loadDao(withRef).create('org-1', metadata({ referenceId: 'cp-pets' }), 'admin');
    assert.strictEqual(withRef[0].params[1], 'cp-pets');

    const withoutRef = [];
    const row = await loadDao(withoutRef).create('org-1', metadata(), 'admin');
    assert.strictEqual(withoutRef[0].params[1], undefined);
    assert.strictEqual(row.ref_id, undefined);
});
