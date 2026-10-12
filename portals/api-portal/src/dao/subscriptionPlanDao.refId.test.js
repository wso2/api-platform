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
 * ref_id on subscription plans. Platform API resolves subscription_plan.ref_id in
 * webhook events as the plan handle, so a new plan defaults to its handle. Updates
 * only touch ref_id when the caller sends refId, so a value an admin set survives
 * saves that omit it.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const PATHS = {
    driver: path.resolve(__dirname, '../db/driver.js'),
    orgContext: path.resolve(__dirname, '../utils/orgContext.js'),
    dao: path.resolve(__dirname, './subscriptionPlanDao.js'),
};

function loadDao(statements) {
    const stubs = {
        driver: {
            execute: async (sql, params) => { statements.push({ sql, params }); return { rowCount: 1 }; },
            // create/update re-read the plan; the shape is irrelevant here.
            queryOne: async () => null,
            query: async () => [],
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

const insertOf = (statements) => statements.find((s) => /INSERT INTO subscription_plans/.test(s.sql));
const updateOf = (statements) => statements.find((s) => /UPDATE subscription_plans/.test(s.sql));

// The INSERT binds ref_id seventh: uuid, org_uuid, portal_id, handle, display_name, description, ref_id.
const REF_ID_INSERT_INDEX = 6;

test('a new plan defaults ref_id to its handle', async () => {
    const statements = [];
    await loadDao(statements).create('org-1', { handle: 'Gold', displayName: 'Gold' }, 'admin');
    assert.strictEqual(insertOf(statements).params[REF_ID_INSERT_INDEX], 'Gold');
});

test('a new plan with refId keeps it', async () => {
    const statements = [];
    await loadDao(statements).create('org-1', { handle: 'Gold', displayName: 'Gold', refId: 'cp-gold' }, 'admin');
    assert.strictEqual(insertOf(statements).params[REF_ID_INSERT_INDEX], 'cp-gold');
});

test('an update without refId leaves ref_id untouched', async () => {
    const statements = [];
    await loadDao(statements).update('org-1', 'plan-1', { handle: 'Gold', displayName: 'Gold' }, 'admin');
    assert.doesNotMatch(updateOf(statements).sql, /ref_id/);
});

test('an update with refId writes exactly that value, null included', async () => {
    for (const refId of ['cp-gold', null]) {
        const statements = [];
        await loadDao(statements).update('org-1', 'plan-1', { handle: 'Gold', displayName: 'Gold', refId }, 'admin');
        const { sql, params } = updateOf(statements);
        assert.match(sql, /ref_id = \?/);
        assert.strictEqual(params[3], refId, String(refId));
    }
});
