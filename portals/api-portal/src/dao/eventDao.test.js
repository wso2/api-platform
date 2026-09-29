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
 * Claim queries for the webhook dispatcher and delivery worker. The regression these
 * guard is a row handed to two concurrent claimers — which sends the same webhook
 * twice — so they pin down that each dialect gets its row-lock hint and that a row
 * already taken by another claimer is dropped from the batch rather than returned.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const DRIVER_PATH = path.resolve(__dirname, '../db/driver.js');
const EVENT_DAO_PATH = path.resolve(__dirname, './eventDao.js');
const ORG_CONTEXT_PATH = path.resolve(__dirname, '../utils/orgContext.js');
const PORTAL_ID = 'portal-a';

/**
 * Loads eventDao against a fake driver. `rows` is what the claim SELECT returns;
 * `takenElsewhere` lists uuids whose guarded UPDATE matches nothing, as if another
 * claimer had moved them out of PENDING first.
 */
function loadEventDao({ dialect, rows, takenElsewhere = [] }) {
    const calls = [];
    const tx = {
        query: async (sql, params) => { calls.push({ sql, params }); return rows; },
        execute: async (sql, params) => {
            calls.push({ sql, params });
            const uuid = params[params.length - 3];
            return { rowCount: takenElsewhere.includes(uuid) ? 0 : 1 };
        },
    };
    const fakeDriver = {
        getDialect: () => dialect,
        paginationClause: (limit) => ({ clause: 'LIMIT ?', params: [limit] }),
        withTransaction: (fn) => fn(tx),
    };
    delete require.cache[EVENT_DAO_PATH];
    require.cache[DRIVER_PATH] = { id: DRIVER_PATH, filename: DRIVER_PATH, loaded: true, exports: fakeDriver };
    require.cache[ORG_CONTEXT_PATH] = {
        id: ORG_CONTEXT_PATH, filename: ORG_CONTEXT_PATH, loaded: true, exports: { getPortalId: () => PORTAL_ID },
    };
    try {
        return { eventDao: require(EVENT_DAO_PATH), calls };
    } finally {
        delete require.cache[DRIVER_PATH];
        delete require.cache[ORG_CONTEXT_PATH];
        delete require.cache[EVENT_DAO_PATH];
    }
}

const pendingEvents = [
    { uuid: 'e1', payload: '{}' },
    { uuid: 'e2', payload: '{}' },
    { uuid: 'e3', payload: '{}' },
];

test('claimPending drops rows another claimer already took', async () => {
    const { eventDao } = loadEventDao({ dialect: 'sqlite', rows: pendingEvents, takenElsewhere: ['e2'] });
    const claimed = await eventDao.claimPending(50, 'org-1');
    assert.deepStrictEqual(claimed.map((e) => e.uuid), ['e1', 'e3']);
});

test('claimPending only moves rows that are still PENDING', async () => {
    const { eventDao, calls } = loadEventDao({ dialect: 'sqlite', rows: pendingEvents });
    await eventDao.claimPending(50, 'org-1');
    const updates = calls.filter((c) => c.sql.startsWith('UPDATE'));
    assert.strictEqual(updates.length, 3);
    for (const u of updates) {
        assert.match(u.sql, /WHERE uuid = \? AND portal_id = \? AND status = \?$/);
        assert.deepStrictEqual(u.params.slice(-2), [PORTAL_ID, 'PENDING']);
    }
});

test('claimPending uses the MSSQL table hint and the Postgres trailing clause', async () => {
    const mssql = loadEventDao({ dialect: 'mssql', rows: [] });
    await mssql.eventDao.claimPending(50, 'org-1');
    assert.match(mssql.calls[0].sql, /FROM events WITH \(UPDLOCK, READPAST, ROWLOCK\) WHERE/);
    assert.doesNotMatch(mssql.calls[0].sql, /FOR UPDATE/);

    const pg = loadEventDao({ dialect: 'postgres', rows: [] });
    await pg.eventDao.claimPending(50, 'org-1');
    assert.match(pg.calls[0].sql, /FOR UPDATE SKIP LOCKED$/);
    assert.doesNotMatch(pg.calls[0].sql, /WITH \(/);

    const sqlite = loadEventDao({ dialect: 'sqlite', rows: [] });
    await sqlite.eventDao.claimPending(50, 'org-1');
    assert.doesNotMatch(sqlite.calls[0].sql, /FOR UPDATE|WITH \(/);
});

test('claimDueDeliveries drops rows another claimer already took', async () => {
    const deliveries = [
        { uuid: 'd1', encrypted_fields: null },
        { uuid: 'd2', encrypted_fields: null },
    ];
    const { eventDao, calls } = loadEventDao({ dialect: 'mssql', rows: deliveries, takenElsewhere: ['d1'] });
    const claimed = await eventDao.claimDueDeliveries(50, 'org-1');
    assert.deepStrictEqual(claimed.map((d) => d.uuid), ['d2']);

    const select = calls.find((c) => c.sql.includes('SELECT d.*'));
    assert.match(select.sql, /event_deliveries d WITH \(UPDLOCK, READPAST, ROWLOCK\)/);
    const updates = calls.filter((c) => c.sql.startsWith('UPDATE event_deliveries SET status = ?, last_attempt_at'));
    assert.strictEqual(updates.length, 2);
    for (const u of updates) assert.strictEqual(u.params[0], 'IN_FLIGHT');
});

test('an unscoped claim (multi-tenancy mode) filters on its portal_id but no organization', async () => {
    const { eventDao, calls } = loadEventDao({ dialect: 'postgres', rows: pendingEvents });
    const claimed = await eventDao.claimPending(50, null);
    assert.strictEqual(claimed.length, 3);
    assert.doesNotMatch(calls[0].sql, /org_uuid/);
    assert.match(calls[0].sql, /portal_id = \?/);
    assert.deepStrictEqual(calls[0].params, ['PENDING', PORTAL_ID, 50]);

    const d = loadEventDao({ dialect: 'postgres', rows: [{ uuid: 'd1', encrypted_fields: null }] });
    await d.eventDao.claimDueDeliveries(50, undefined);
    const [sweep, select] = d.calls;
    assert.doesNotMatch(sweep.sql, /org_uuid/);
    assert.deepStrictEqual(sweep.params.slice(4), [PORTAL_ID]);
    assert.doesNotMatch(select.sql, /org_uuid/);
    assert.deepStrictEqual(select.params, ['PENDING', PORTAL_ID, 50]);
});

test('a scoped claim still filters on its organization', async () => {
    const d = loadEventDao({ dialect: 'postgres', rows: [] });
    await d.eventDao.claimDueDeliveries(50, 'org-1');
    const [sweep, select] = d.calls;
    assert.match(sweep.sql, /WHERE org_uuid = \? AND portal_id = \?\)/);
    assert.deepStrictEqual(sweep.params.slice(4), [PORTAL_ID, 'org-1', PORTAL_ID]);
    assert.match(select.sql, /AND e\.org_uuid = \?/);
    assert.deepStrictEqual(select.params, ['PENDING', PORTAL_ID, 'org-1', 50]);
});

test('recordUndeliverable writes one FAILED delivery per subscriber, under this portal', async () => {
    const { eventDao } = loadEventDao({ dialect: 'postgres', rows: [] });
    const calls = [];
    const tx = { execute: async (sql, params) => { calls.push({ sql, params }); return { rowCount: 1 }; } };
    await eventDao.recordUndeliverable('e1', [
        { id: 's1', url: 'https://a.example.com/hook' },
        { id: 's2', url: 'https://b.example.com/hook' },
    ], 'Subscriber secret could not be decrypted', tx);
    assert.strictEqual(calls.length, 2);
    for (const [i, call] of calls.entries()) {
        assert.match(call.sql, /^INSERT INTO event_deliveries \(uuid, portal_id, event_uuid, subscriber_id, target_url, status, last_error\)/);
        const [, portalId, eventId, subscriberId, , status, lastError] = call.params;
        assert.deepStrictEqual([portalId, eventId, subscriberId, status, lastError],
            [PORTAL_ID, 'e1', `s${i + 1}`, 'FAILED', 'Subscriber secret could not be decrypted']);
    }
});

test('recordUndeliverable writes nothing when every subscriber is readable', async () => {
    const { eventDao } = loadEventDao({ dialect: 'postgres', rows: [] });
    const calls = [];
    await eventDao.recordUndeliverable('e1', [], 'reason', { execute: async (sql) => { calls.push(sql); } });
    assert.strictEqual(calls.length, 0);
});
