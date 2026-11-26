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
 * `associateExclusive` — the one-key-per-key-type rule, and the locking that
 * makes it a rule rather than a hope.
 *
 * Two things are asserted here that a service-level test cannot see: that the
 * application row is locked, with the right hint for each dialect, before
 * anything is read or written; and that the lock comes first in the statement
 * order, since a lock taken after the check protects nothing.
 *
 * The driver is faked the way eventDao.test.js fakes it — the SQL is the thing
 * under test, so every statement is recorded and asserted rather than run.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const DRIVER_PATH = path.resolve(__dirname, '../db/driver.js');
const DAO_PATH = path.resolve(__dirname, './oauth2KeyAppMappingDao.js');
const ORG_CONTEXT_PATH = path.resolve(__dirname, '../utils/orgContext.js');
const PORTAL_ID = 'portal-a';

/**
 * Load the DAO against a fake driver.
 *
 * `currentApp` is the application the key is already on (null when unattached),
 * and `conflict` is the key already holding this type's slot (null when free).
 */
function loadDao({ dialect, currentApp = null, conflict = null }) {
    const calls = [];
    const tx = {
        queryOne: async (sql, params) => {
            calls.push({ sql, params });
            if (/FROM applications/.test(sql)) return { uuid: params[1] };
            if (/JOIN oauth2_consumer_keys/.test(sql)) return conflict ? { key_uuid: conflict } : null;
            if (/SELECT app_uuid/.test(sql)) return currentApp ? { app_uuid: currentApp } : null;
            return null;
        },
        execute: async (sql, params) => { calls.push({ sql, params }); return { rowCount: 1 }; },
    };
    const fakeDriver = {
        getDialect: () => dialect,
        buildUpsert: () => 'UPSERT_SQL',
        withTransaction: (fn) => fn(tx),
        queryOne: async () => null,
        query: async () => [],
        execute: async () => ({ rowCount: 0 }),
    };

    delete require.cache[DAO_PATH];
    require.cache[DRIVER_PATH] = { id: DRIVER_PATH, filename: DRIVER_PATH, loaded: true, exports: fakeDriver };
    require.cache[ORG_CONTEXT_PATH] = {
        id: ORG_CONTEXT_PATH, filename: ORG_CONTEXT_PATH, loaded: true, exports: { getPortalId: () => PORTAL_ID },
    };
    try {
        return { dao: require(DAO_PATH), calls };
    } finally {
        delete require.cache[DRIVER_PATH];
        delete require.cache[ORG_CONTEXT_PATH];
        delete require.cache[DAO_PATH];
    }
}

const lockOf = (calls) => calls.find((c) => /FROM applications/.test(c.sql));
const wrote = (calls) => calls.some((c) => c.sql === 'UPSERT_SQL');

test('PostgreSQL locks the application row with FOR UPDATE', async () => {
    const { dao, calls } = loadDao({ dialect: 'postgres' });
    await dao.associateExclusive('key-1', 'app-1', 'PRODUCTION', 'alice');

    assert.match(lockOf(calls).sql, /FOR UPDATE\s*$/);
    assert.deepStrictEqual(lockOf(calls).params, [PORTAL_ID, 'app-1']);
});

test('SQL Server locks it with UPDLOCK, ROWLOCK', async () => {
    const { dao, calls } = loadDao({ dialect: 'mssql' });
    await dao.associateExclusive('key-1', 'app-1', 'PRODUCTION', 'alice');

    assert.match(lockOf(calls).sql, /WITH \(UPDLOCK, ROWLOCK\)/);
    assert.doesNotMatch(lockOf(calls).sql, /FOR UPDATE/);
});

test('SQLite takes no hint, since its adapter serializes transactions', async () => {
    const { dao, calls } = loadDao({ dialect: 'sqlite' });
    await dao.associateExclusive('key-1', 'app-1', 'PRODUCTION', 'alice');

    assert.doesNotMatch(lockOf(calls).sql, /FOR UPDATE|UPDLOCK/);
});

test('the lock is taken before anything is read or written', async () => {
    // A lock acquired after the check protects nothing: the window this closes is
    // exactly between the check and the write.
    const { dao, calls } = loadDao({ dialect: 'postgres' });
    await dao.associateExclusive('key-1', 'app-1', 'PRODUCTION', 'alice');

    assert.match(calls[0].sql, /FROM applications/);
    assert.ok(calls.length > 1, 'expected the check and write to follow the lock');
});

test('a free slot is written, and reports no conflict', async () => {
    const { dao, calls } = loadDao({ dialect: 'postgres' });
    const conflict = await dao.associateExclusive('key-1', 'app-1', 'PRODUCTION', 'alice');

    assert.strictEqual(conflict, null);
    assert.ok(wrote(calls));
    const upsert = calls.find((c) => c.sql === 'UPSERT_SQL');
    assert.deepStrictEqual(upsert.params, [PORTAL_ID, 'key-1', 'app-1', 'alice']);
});

test('a taken slot reports the incumbent and writes nothing', async () => {
    const { dao, calls } = loadDao({ dialect: 'postgres', conflict: 'incumbent-key' });
    const conflict = await dao.associateExclusive('key-1', 'app-1', 'PRODUCTION', 'alice');

    assert.strictEqual(conflict, 'incumbent-key');
    assert.ok(!wrote(calls), 'the incumbent must stay attached');
});

test('the conflict lookup excludes the key being associated', async () => {
    const { dao, calls } = loadDao({ dialect: 'postgres', conflict: 'incumbent-key' });
    await dao.associateExclusive('key-1', 'app-1', 'SANDBOX', 'alice');

    const lookup = calls.find((c) => /JOIN oauth2_consumer_keys/.test(c.sql));
    assert.match(lookup.sql, /m\.key_uuid <> \?/);
    assert.deepStrictEqual(lookup.params, [PORTAL_ID, 'app-1', 'SANDBOX', 'key-1']);
});

test('a key already on this application succeeds without touching the row', async () => {
    /*
     * An application written before this rule existed can hold two keys of one
     * type. Checking the slot first would find the *other* one and refuse, so a
     * key already attached here could never be re-associated — the one case the
     * API documents as succeeding unchanged. Returning early also leaves the
     * original created_by/created_at alone.
     */
    const { dao, calls } = loadDao({
        dialect: 'postgres', currentApp: 'app-1', conflict: 'sibling-production-key',
    });
    const conflict = await dao.associateExclusive('key-1', 'app-1', 'PRODUCTION', 'alice');

    assert.strictEqual(conflict, null);
    assert.ok(!wrote(calls), 're-associating in place must not rewrite the row');
    assert.ok(
        !calls.some((c) => /JOIN oauth2_consumer_keys/.test(c.sql)),
        'the slot check must be skipped for a key that is already here'
    );
});

test('a key moving from another application is still checked', async () => {
    const { dao, calls } = loadDao({
        dialect: 'postgres', currentApp: 'app-other', conflict: 'incumbent-key',
    });
    const conflict = await dao.associateExclusive('key-1', 'app-1', 'PRODUCTION', 'alice');

    assert.strictEqual(conflict, 'incumbent-key');
    assert.ok(!wrote(calls));
});
