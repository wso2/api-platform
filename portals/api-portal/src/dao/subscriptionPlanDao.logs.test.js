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
const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const PATHS = {
    driver: path.resolve(__dirname, '../db/driver.js'),
    orgContext: path.resolve(__dirname, '../utils/orgContext.js'),
    logger: path.resolve(__dirname, '../config/logger.js'),
    dao: path.resolve(__dirname, './subscriptionPlanDao.js'),
};

const PLAN = { uuid: 'plan-1', org_uuid: 'org-1', name: 'Gold' };

/** The DAO over a stub database; `conditionsTable` says whether the conditions table exists. */
function loadDao({ conditionsTable, conditionRows = [] }) {
    const warnings = [];
    const stubs = {
        driver: {
            execute: async () => ({ rowCount: 1 }),
            queryOne: async (sql, params = []) => {
                if (/sqlite_master/.test(sql)) {
                    return params[0] === 'subscription_plan_conditions' && !conditionsTable ? null : { found: 1 };
                }
                return /FROM subscription_plans\b/.test(sql) ? { ...PLAN } : null;
            },
            query: async (sql) => (/subscription_plan_conditions/.test(sql) ? conditionRows : []),
            getDialect: () => 'sqlite',
            binaryParam: (value) => value,
        },
        orgContext: { getPortalId: () => 'portal-a' },
        logger: { warn: (message, meta) => warnings.push({ message, meta }), info() {}, error() {}, debug() {} },
    };
    for (const [k, exports] of Object.entries(stubs)) {
        require.cache[PATHS[k]] = { id: PATHS[k], filename: PATHS[k], loaded: true, exports };
    }
    delete require.cache[PATHS.dao];
    try {
        return { dao: require(PATHS.dao), warnings };
    } finally {
        for (const p of Object.values(PATHS)) delete require.cache[p];
    }
}

test('a condition with unreadable params still fails the read, and a warning names it', async () => {
    const { dao, warnings } = loadDao({
        conditionsTable: true,
        conditionRows: [
            { plan_uuid: 'plan-1', condition_id: 'region-limit', condition_type: 'custom', params: '{not json' },
        ],
    });

    await assert.rejects(dao.getByUuid('org-1', 'plan-1'), SyntaxError);
    const warning = warnings.find((w) => /Unreadable subscription plan condition params/.test(w.message));
    assert.ok(warning, 'expected a warning');
    assert.strictEqual(warning.meta.planId, 'plan-1');
    assert.strictEqual(warning.meta.conditionId, 'region-limit');
});

test('warns once that the database is not upgraded, however often plans are read', async () => {
    const { dao, warnings } = loadDao({ conditionsTable: false });

    await dao.getByUuid('org-1', 'plan-1');
    await dao.getByUuid('org-1', 'plan-1');

    const schemaWarnings = warnings.filter((w) => /not upgraded for subscription plan conditions/.test(w.message));
    assert.strictEqual(schemaWarnings.length, 1);
});
