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
 * seedOrg: create-if-missing plus defaults, safe under concurrent calls for the same
 * handle (replicas starting together, two logins provisioning one org).
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const { NotFoundError } = require('../utils/errors/customErrors');

const r = (p) => path.resolve(__dirname, p);
const PATHS = {
    orgDao: r('../dao/organizationDao.js'),
    labelDao: r('../dao/labelDao.js'),
    viewDao: r('../dao/viewDao.js'),
    planDao: r('../dao/subscriptionPlanDao.js'),
    configLoader: r('../config/configLoader.js'),
    orgContext: r('../utils/orgContext.js'),
    logger: r('../config/logger.js'),
    driver: r('../db/driver.js'),
    seeder: r('./seederService.js'),
};

function duplicateKeyError() {
    return Object.assign(new Error('duplicate key'), { duplicate: true });
}

/** `orgRows` is the mutable "table"; `onCreate` decides what create() does. */
function loadSeeder({ orgRows, onCreate, orgDao = {}, orgContext = {}, logger = {} }) {
    const calls = { create: 0, label: 0, view: 0 };
    const stubs = {
        orgDao: {
            getByHandle: async (handle) => {
                const row = orgRows.find((o) => o.handle === handle);
                if (!row) throw new NotFoundError('Organization not found');
                return row;
            },
            create: async (payload) => { calls.create++; return onCreate(payload); },
            ...orgDao,
        },
        labelDao: {
            update: async () => { calls.label++; return { uuid: 'label-1' }; },
            addToView: async () => {},
        },
        viewDao: { update: async () => { calls.view++; return { uuid: 'view-1' }; } },
        planDao: { createMany: async () => {} },
        configLoader: { config: { organization: { autoCreateSubscriptionPlans: false } } },
        orgContext,
        logger: { error: () => {}, warn: () => {}, info: () => {}, ...logger },
        driver: { isDuplicateKeyError: (e) => !!e.duplicate },
    };
    for (const [k, exports] of Object.entries(stubs)) {
        require.cache[PATHS[k]] = { id: PATHS[k], filename: PATHS[k], loaded: true, exports };
    }
    delete require.cache[PATHS.seeder];
    try {
        return { seeder: require(PATHS.seeder), calls };
    } finally {
        for (const p of Object.values(PATHS)) delete require.cache[p];
    }
}

const acme = { handle: 'acme', displayName: 'Acme', idpRefId: 'acme-id' };

test('an existing org is returned as-is and its defaults are ensured', async () => {
    const row = { uuid: 'u-acme', handle: 'acme' };
    const { seeder, calls } = loadSeeder({ orgRows: [row], onCreate: () => assert.fail('must not create') });
    const { org, existed } = await seeder.seedOrg(acme);
    assert.strictEqual(org, row);
    assert.strictEqual(existed, true);
    assert.strictEqual(calls.label, 1);
    assert.strictEqual(calls.view, 1);
});

test('a missing org is created with the given identity', async () => {
    const rows = [];
    const { seeder, calls } = loadSeeder({
        orgRows: rows,
        onCreate: (payload) => { const row = { uuid: 'u-new', ...payload }; rows.push(row); return row; },
    });
    const { org, existed } = await seeder.seedOrg(acme);
    assert.strictEqual(existed, false);
    assert.strictEqual(calls.create, 1);
    assert.strictEqual(org.handle, 'acme');
    assert.strictEqual(org.displayName, 'Acme');
    assert.strictEqual(org.idpRefId, 'acme-id');
});

test('losing a concurrent create re-reads the winner\'s row', async () => {
    const rows = [];
    const winner = { uuid: 'u-winner', handle: 'acme' };
    const { seeder } = loadSeeder({
        orgRows: rows,
        // The other process's insert lands between our lookup and our create.
        onCreate: () => { rows.push(winner); throw duplicateKeyError(); },
    });
    const { org, existed } = await seeder.seedOrg(acme);
    assert.strictEqual(org, winner);
    assert.strictEqual(existed, true);
});

test('a duplicate on another column (not this handle) is not swallowed', async () => {
    const { seeder } = loadSeeder({ orgRows: [], onCreate: () => { throw duplicateKeyError(); } });
    await assert.rejects(seeder.seedOrg(acme), NotFoundError);
});

test('any other create failure propagates', async () => {
    const { seeder } = loadSeeder({ orgRows: [], onCreate: () => { throw new Error('connection reset'); } });
    await assert.rejects(seeder.seedOrg(acme), /connection reset/);
});

// seedDefaultOrg seeds the instance's own org from config. cp_ref_id is what outbound
// webhook events carry as org.ref_id, and Platform API resolves it by handle.
function defaultOrgContext(handle = 'acme') {
    return {
        getHandle: () => handle,
        getDisplayName: () => 'Acme',
        getIdpOrgId: () => 'acme-id',
        getConfiguredIdpOrgId: () => 'acme-id',
        resetCache: () => {},
    };
}

function recordingLogger() {
    const logs = { info: [], error: [] };
    return {
        logs,
        logger: {
            info: (msg, meta) => logs.info.push({ msg, meta }),
            error: (msg, meta) => logs.error.push({ msg, meta }),
        },
    };
}

test('a new default org is created with cp_ref_id defaulted to its handle', async () => {
    const rows = [];
    let backfilled = false;
    const { seeder } = loadSeeder({
        orgRows: rows,
        onCreate: (payload) => { const row = { uuid: 'u-new', ...payload }; rows.push(row); return row; },
        orgDao: { backfillCpRefId: async () => { backfilled = true; return false; } },
        orgContext: defaultOrgContext(),
    });
    await seeder.seedDefaultOrg();
    assert.strictEqual(rows[0].cpRefId, 'acme');
    assert.strictEqual(backfilled, false, 'a freshly created row needs no backfill');
});

test('an existing default org with an empty cp_ref_id is backfilled with the handle', async () => {
    const row = { uuid: 'u-acme', handle: 'acme', idp_ref_id: 'acme-id', cp_ref_id: null };
    const backfillCalls = [];
    const { logs, logger } = recordingLogger();
    const { seeder } = loadSeeder({
        orgRows: [row],
        onCreate: () => assert.fail('must not create'),
        orgDao: { backfillCpRefId: async (...args) => { backfillCalls.push(args); return true; } },
        orgContext: defaultOrgContext(),
        logger,
    });
    await seeder.seedDefaultOrg();
    assert.deepStrictEqual(backfillCalls.map((a) => a.slice(0, 2)), [['u-acme', 'acme']]);
    assert.ok(logs.info.some((l) => /cp_ref_id was empty/.test(l.msg)));
});

test('an existing default org whose cp_ref_id is already set is left alone', async () => {
    const row = { uuid: 'u-acme', handle: 'acme', idp_ref_id: 'acme-id', cp_ref_id: 'custom-ref' };
    const { logs, logger } = recordingLogger();
    const { seeder } = loadSeeder({
        orgRows: [row],
        onCreate: () => assert.fail('must not create'),
        // The DAO only writes while the column is empty, so it reports no change here.
        orgDao: { backfillCpRefId: async () => false },
        orgContext: defaultOrgContext(),
        logger,
    });
    await seeder.seedDefaultOrg();
    assert.ok(!logs.info.some((l) => /cp_ref_id was empty/.test(l.msg)));
});

test('a failed cp_ref_id backfill is logged and does not stop startup', async () => {
    const row = { uuid: 'u-acme', handle: 'acme', idp_ref_id: 'acme-id', cp_ref_id: '' };
    const { logs, logger } = recordingLogger();
    const { seeder } = loadSeeder({
        orgRows: [row],
        onCreate: () => assert.fail('must not create'),
        orgDao: { backfillCpRefId: async () => { throw new Error('db down'); } },
        orgContext: defaultOrgContext(),
        logger,
    });
    await seeder.seedDefaultOrg();
    const failure = logs.error.find((l) => /backfill organization cp_ref_id/.test(l.msg));
    assert.ok(failure, 'expected the backfill failure to be logged');
    assert.strictEqual(failure.meta.error, 'db down');
});
