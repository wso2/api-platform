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
    whDao: r('../dao/webhookSubscriberDao.js'),
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
function loadSeeder({ orgRows, onCreate, orgContext = {}, configLoader, whDao }) {
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
        configLoader: configLoader || { config: { organization: { autoCreateSubscriptionPlans: false } } },
        orgContext,
        logger: { error: () => {}, warn: () => {}, info: () => {} },
        driver: { isDuplicateKeyError: (e) => !!e.duplicate },
        whDao: whDao || {
            get: async () => null,
            create: async () => { throw new Error('whDao.create called unexpectedly'); },
            update: async () => { throw new Error('whDao.update called unexpectedly'); },
        },
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

test('a new default org is created with cp_ref_id defaulted to its handle', async () => {
    const rows = [];
    const { seeder } = loadSeeder({
        orgRows: rows,
        onCreate: (payload) => { const row = { uuid: 'u-new', ...payload }; rows.push(row); return row; },
        orgContext: defaultOrgContext(),
    });
    await seeder.seedDefaultOrg();
    assert.strictEqual(rows[0].cpRefId, 'acme');
});

// cp_ref_id is set on create only, so a value an admin set (or cleared) in settings
// survives restarts.
test('an existing default org keeps its cp_ref_id across restarts', async () => {
    const row = { uuid: 'u-acme', handle: 'acme', idp_ref_id: 'acme-id', cp_ref_id: '' };
    const { seeder, calls } = loadSeeder({
        orgRows: [row],
        onCreate: () => assert.fail('must not create'),
        orgContext: defaultOrgContext(),
    });
    await seeder.seedDefaultOrg();
    assert.strictEqual(calls.create, 0);
    assert.strictEqual(row.cp_ref_id, '');
});

// --------------------------------------------------------------------------------
// seedPlatformApiWebhookSubscriber: the OSS self-seed path. Covers FR-011 (upsert
// when both config.webhooks.platformApi.url and .secret are set) and FR-013
// (no-op when either is empty). The isPlatformApiSubscriberConfigOwned() helper
// is tested at the same time because its live-config predicate is what the
// admin API and the DTO flag read.
// --------------------------------------------------------------------------------

// orgContextForPlatformApi stubs the fields seedPlatformApiWebhookSubscriber reads:
// getOrgUuid() is awaited, and the function also calls getHandle for logging context.
function orgContextForPlatformApi(orgUuid = 'u-acme', handle = 'acme') {
    return {
        getHandle: () => handle,
        getOrgUuid: async () => orgUuid,
        resetCache: () => {},
    };
}

// configWithPlatformApi returns the configLoader stub with the [api_portal.webhooks.platform_api]
// block populated. Omit `secret` or `url` to exercise the no-op path.
function configWithPlatformApi({ url, secret, events } = {}) {
    return {
        config: {
            organization: { autoCreateSubscriptionPlans: false },
            webhooks: {
                platformApi: {
                    url: url ?? '',
                    secret: secret ?? '',
                    events: events ?? [],
                },
            },
        },
    };
}

// recordingWhDao stubs whDao with mutable in-memory state so a test can assert
// which operation ran (create/update/none) and inspect the payload. get() returns
// null until create or update sets it.
function recordingWhDao() {
    const calls = { get: 0, create: 0, update: 0 };
    let row = null;
    return {
        calls,
        getRow: () => row,
        get: async (_orgId, handle) => { calls.get++; return row && row.handle === handle ? row : null; },
        create: async (orgId, payload) => {
            calls.create++;
            row = { ...payload, org_uuid: orgId };
            return row;
        },
        update: async (orgId, _subscriberHandle, payload) => {
            calls.update++;
            row = { ...row, ...payload, org_uuid: orgId };
            return [1, [row]];
        },
    };
}

test('platform-api subscriber is seeded when both url and secret are set', async () => {
    const whDao = recordingWhDao();
    const { seeder } = loadSeeder({
        orgRows: [{ uuid: 'u-acme', handle: 'acme' }],
        onCreate: () => assert.fail('must not touch orgDao'),
        orgContext: orgContextForPlatformApi(),
        configLoader: configWithPlatformApi({
            url: 'https://pa.example.com/api/internal/v0.9/webhook/portals/uuid-x/events',
            secret: '9f3cfbbd7a1e4bbd8f3c5c6d8e4e5a3f9f3cfbbd7a1e4bbd8f3c5c6d8e4e5a3f',
            events: ['subscription.*', 'apikey.*'],
        }),
        whDao,
    });
    await seeder.seedPlatformApiWebhookSubscriber();
    assert.strictEqual(whDao.calls.create, 1, 'expected one whDao.create');
    assert.strictEqual(whDao.calls.update, 0, 'expected no whDao.update on first seed');
    const stored = whDao.getRow();
    assert.strictEqual(stored.handle, 'platform-api');
    assert.strictEqual(stored.targetUrl, 'https://pa.example.com/api/internal/v0.9/webhook/portals/uuid-x/events');
    assert.strictEqual(stored.enabled, true);
    assert.deepStrictEqual(stored.events, ['subscription.*', 'apikey.*']);
    assert.strictEqual(seeder.isPlatformApiSubscriberConfigOwned(), true);
});

test('platform-api subscriber upsert overwrites an existing row from config', async () => {
    const whDao = recordingWhDao();
    // Pre-seed a prior row — a prior boot's self-seed, or a hand-POSTed one.
    whDao.create('u-acme', {
        handle: 'platform-api',
        targetUrl: 'https://old.example.com/old-path',
        secret: 'old-secret',
        events: ['subscription.*'],
        enabled: true,
    });
    whDao.calls.create = 0; // reset so the next assertion isolates the test's own seed call

    const { seeder } = loadSeeder({
        orgRows: [{ uuid: 'u-acme', handle: 'acme' }],
        onCreate: () => assert.fail('must not touch orgDao'),
        orgContext: orgContextForPlatformApi(),
        configLoader: configWithPlatformApi({
            url: 'https://new.example.com/api/internal/v0.9/webhook/portals/uuid-y/events',
            secret: 'new-secret',
            events: ['subscription.*', 'apikey.*', 'application.*'],
        }),
        whDao,
    });
    await seeder.seedPlatformApiWebhookSubscriber();
    assert.strictEqual(whDao.calls.update, 1, 'expected one whDao.update when a prior row existed');
    assert.strictEqual(whDao.calls.create, 0);
    const stored = whDao.getRow();
    assert.strictEqual(stored.targetUrl, 'https://new.example.com/api/internal/v0.9/webhook/portals/uuid-y/events');
    assert.deepStrictEqual(stored.events, ['subscription.*', 'apikey.*', 'application.*']);
});

test('platform-api subscriber self-seed is a no-op when secret is empty', async () => {
    const whDao = recordingWhDao();
    const { seeder } = loadSeeder({
        orgRows: [{ uuid: 'u-acme', handle: 'acme' }],
        onCreate: () => assert.fail('must not touch orgDao'),
        orgContext: orgContextForPlatformApi(),
        configLoader: configWithPlatformApi({ url: 'https://pa.example.com/x', secret: '' }),
        whDao,
    });
    await seeder.seedPlatformApiWebhookSubscriber();
    assert.strictEqual(whDao.calls.get, 0, 'empty secret must not reach whDao');
    assert.strictEqual(whDao.calls.create, 0);
    assert.strictEqual(whDao.calls.update, 0);
    assert.strictEqual(seeder.isPlatformApiSubscriberConfigOwned(), false);
});

test('platform-api subscriber self-seed is a no-op when url is empty', async () => {
    const whDao = recordingWhDao();
    const { seeder } = loadSeeder({
        orgRows: [{ uuid: 'u-acme', handle: 'acme' }],
        onCreate: () => assert.fail('must not touch orgDao'),
        orgContext: orgContextForPlatformApi(),
        configLoader: configWithPlatformApi({ url: '', secret: 'something' }),
        whDao,
    });
    await seeder.seedPlatformApiWebhookSubscriber();
    assert.strictEqual(whDao.calls.get, 0, 'empty url must not reach whDao');
    assert.strictEqual(whDao.calls.create, 0);
    assert.strictEqual(whDao.calls.update, 0);
    assert.strictEqual(seeder.isPlatformApiSubscriberConfigOwned(), false);
});

test('isPlatformApiSubscriberConfigOwned follows the live config predicate', async () => {
    // Not full self-seed — just the predicate. The admin API and the DTO flag
    // read this at request time, so changing config and restarting is enough
    // to flip the admin-API's refusal behaviour.
    {
        const { seeder } = loadSeeder({
            orgRows: [], onCreate: () => {},
            orgContext: orgContextForPlatformApi(),
            configLoader: configWithPlatformApi({ url: 'https://x', secret: 's' }),
            whDao: recordingWhDao(),
        });
        assert.strictEqual(seeder.isPlatformApiSubscriberConfigOwned(), true);
    }
    {
        const { seeder } = loadSeeder({
            orgRows: [], onCreate: () => {},
            orgContext: orgContextForPlatformApi(),
            configLoader: configWithPlatformApi({ url: 'https://x', secret: '' }),
            whDao: recordingWhDao(),
        });
        assert.strictEqual(seeder.isPlatformApiSubscriberConfigOwned(), false);
    }
});
