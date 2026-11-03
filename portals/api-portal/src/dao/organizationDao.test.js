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
 * listByIdpRefId must match exactly even where the database doesn't: SQL Server's
 * default collation returns "acme" for "ACME" or "acme ", and an org claim that only
 * differs in case must not resolve to another organization.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const DRIVER_PATH = path.resolve(__dirname, '../db/driver.js');
const DAO_PATH = path.resolve(__dirname, './organizationDao.js');
const ORG_CONTEXT_PATH = path.resolve(__dirname, '../utils/orgContext.js');

// organizationDao reads the portal id lazily, on each query, so orgContext stays
// stubbed for the whole test file rather than only while the DAO loads.
require.cache[ORG_CONTEXT_PATH] = {
    id: ORG_CONTEXT_PATH, filename: ORG_CONTEXT_PATH, loaded: true, exports: { getPortalId: () => 'portal-a' },
};

function loadDao(rowsReturnedByDatabase, queries = []) {
    require.cache[DRIVER_PATH] = {
        id: DRIVER_PATH, filename: DRIVER_PATH, loaded: true,
        exports: { query: async (sql, params) => { queries.push({ sql, params }); return rowsReturnedByDatabase; } },
    };
    delete require.cache[DAO_PATH];
    try {
        return require(DAO_PATH);
    } finally {
        delete require.cache[DRIVER_PATH];
        delete require.cache[DAO_PATH];
    }
}

// What a case-insensitive, trailing-space-insensitive collation hands back for any of them.
const collationMatches = [{ uuid: 'u-acme', handle: 'acme', idp_ref_id: 'acme-id', configuration: '{}' }];

test('an exact idp_ref_id is returned', async () => {
    const rows = await loadDao(collationMatches).listByIdpRefId('acme-id');
    assert.deepStrictEqual(rows.map((r) => r.uuid), ['u-acme']);
});

test('a value differing only in case or trailing spaces is not', async () => {
    for (const claim of ['ACME-ID', 'Acme-Id', 'acme-id ', 'acme-id  ']) {
        assert.deepStrictEqual(await loadDao(collationMatches).listByIdpRefId(claim), [], claim);
    }
});

test('the lookup is scoped to this portal', async () => {
    const queries = [];
    await loadDao(collationMatches, queries).listByIdpRefId('acme-id');
    assert.match(queries[0].sql, /WHERE idp_ref_id = \? AND portal_id = \?/);
    assert.deepStrictEqual(queries[0].params, ['acme-id', 'portal-a']);
});
