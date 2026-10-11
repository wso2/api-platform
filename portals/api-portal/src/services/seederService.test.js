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
function loadSeeder({ orgRows, onCreate }) {
    const calls = { create: 0, label: 0, view: 0 };
    const stubs = {
        orgDao: {
            getByHandle: async (handle) => {
                const row = orgRows.find((o) => o.handle === handle);
                if (!row) throw new NotFoundError('Organization not found');
                return row;
            },
            create: async (payload) => { calls.create++; return onCreate(payload); },
        },
        labelDao: {
            update: async () => { calls.label++; return { uuid: 'label-1' }; },
            addToView: async () => {},
        },
        viewDao: { update: async () => { calls.view++; return { uuid: 'view-1' }; } },
        planDao: { createMany: async () => {} },
        configLoader: { config: { organization: { autoCreateSubscriptionPlans: false } } },
        orgContext: {},
        logger: { error: () => {}, warn: () => {}, info: () => {} },
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
