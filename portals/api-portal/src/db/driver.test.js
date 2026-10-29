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
 * The driver records the outcome of every database operation for the health check.
 * What these pin down is that recording is invisible to the 40-odd callers — the same
 * values come back and the same errors are rethrown — and that each operation is
 * counted once, with connection failures told apart from statement errors.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const CONFIG_PATH = path.resolve(__dirname, '../config/configLoader.js');
const DRIVER_PATH = path.resolve(__dirname, './driver.js');
const PG_ADAPTER_PATH = path.resolve(__dirname, './adapters/postgresAdapter.js');
const SQLITE_ADAPTER_PATH = path.resolve(__dirname, './adapters/sqliteAdapter.js');

const refused = () => Object.assign(new Error('connect ECONNREFUSED'), { code: 'ECONNREFUSED' });
const duplicate = () => Object.assign(new Error('duplicate key'), { code: '23505' });

/**
 * A fake adapter whose next result is scripted per call: `next.push(value)` resolves
 * the next operation to `value`, `next.push(errorInstance)` rejects it. Transactions
 * run their callback against the same fake, unless `failBegin` or `failCommit` is set.
 */
function fakeAdapter() {
    const calls = [];
    const next = [];
    const respond = (method) => async (sqlText, params) => {
        calls.push({ method, sqlText, params });
        const outcome = next.length ? next.shift() : [];
        if (outcome instanceof Error) throw outcome;
        return outcome;
    };
    const adapter = {
        calls,
        next,
        failBegin: null,
        failCommit: null,
        query: respond('query'),
        queryOne: respond('queryOne'),
        execute: respond('execute'),
        withTransaction: async (fn) => {
            if (adapter.failBegin) throw adapter.failBegin;
            const result = await fn(adapter);
            if (adapter.failCommit) throw adapter.failCommit;
            return result;
        },
        poolStats: () => ({ max: 10, total: 3, idle: 1, waiting: 0 }),
        close: async () => {},
    };
    return adapter;
}

function loadDriver({ driver = 'postgres', designMode = false, adapter = fakeAdapter() } = {}) {
    let hooks;
    const fakeConfig = {
        config: {
            database: { driver },
            designMode: { enabled: designMode },
            health: { database: { failureWindowMs: 30000 } },
        },
    };
    const stub = (p, exports) => { require.cache[p] = { id: p, filename: p, loaded: true, exports }; };
    stub(CONFIG_PATH, fakeConfig);
    stub(PG_ADAPTER_PATH, { createPostgresAdapter: (cfg, h) => { hooks = h; return adapter; } });
    stub(SQLITE_ADAPTER_PATH, { createSqliteAdapter: () => adapter });
    delete require.cache[DRIVER_PATH];
    try {
        return { db: require(DRIVER_PATH), adapter, hooks: () => hooks };
    } finally {
        delete require.cache[DRIVER_PATH];
        delete require.cache[CONFIG_PATH];
        delete require.cache[PG_ADAPTER_PATH];
        delete require.cache[SQLITE_ADAPTER_PATH];
    }
}

const counts = (db) => {
    const { attempts, failures } = db.outcomeWindow.snapshot();
    return { attempts, failures };
};

test('query, queryOne and execute return the adapter result unchanged and are counted', async () => {
    const { db, adapter } = loadDriver();
    const rows = [{ id: 1 }];
    const one = { id: 2 };
    const exec = { rowCount: 1, insertId: null };
    adapter.next.push(rows, one, exec);

    assert.strictEqual(await db.query('SELECT * FROM t WHERE a = ?', [1]), rows);
    assert.strictEqual(await db.queryOne('SELECT * FROM t WHERE a = ?', [1]), one);
    assert.strictEqual(await db.execute('DELETE FROM t WHERE a = ?', [1]), exec);
    assert.equal(adapter.calls[0].sqlText, 'SELECT * FROM t WHERE a = $1');
    assert.deepStrictEqual(counts(db), { attempts: 3, failures: 0 });
    assert.ok(db.outcomeWindow.snapshot().lastActivity);
});

test('errors are rethrown as the same object; only connection failures count as failures', async () => {
    const { db, adapter } = loadDriver();
    const sqlErr = duplicate();
    const netErr = refused();
    adapter.next.push(sqlErr, netErr);

    await assert.rejects(db.execute('INSERT INTO t VALUES (?)', [1]), (err) => err === sqlErr);
    await assert.rejects(db.query('SELECT 1'), (err) => err === netErr);
    assert.deepStrictEqual(counts(db), { attempts: 2, failures: 1 });
});

test('a synchronous adapter throw is rethrown unchanged and counted', () => {
    const adapter = fakeAdapter();
    const err = refused();
    adapter.query = () => { throw err; };
    const { db } = loadDriver({ adapter });
    assert.throws(() => db.query('SELECT 1'), (thrown) => thrown === err);
    assert.deepStrictEqual(counts(db), { attempts: 1, failures: 1 });
});

test('a non-promise adapter result is passed through and counted', () => {
    const adapter = fakeAdapter();
    adapter.query = () => 'plain';
    const { db } = loadDriver({ adapter });
    assert.equal(db.query('SELECT 1'), 'plain');
    assert.deepStrictEqual(counts(db), { attempts: 1, failures: 0 });
});

test('a committed transaction counts its statements plus the transaction itself', async () => {
    const { db, adapter } = loadDriver();
    adapter.next.push([{ n: 1 }], { rowCount: 1 });
    const result = await db.withTransaction(async (tx) => {
        await tx.query('SELECT 1');
        await db.execute('UPDATE t SET a = ?', [2]); // ambient: routed to the transaction
        return 'done';
    });
    assert.equal(result, 'done');
    assert.equal(adapter.calls.length, 2);
    assert.deepStrictEqual(counts(db), { attempts: 3, failures: 0 });
});

test('a transaction callback that returns a plain value still commits and is counted', async () => {
    const { db } = loadDriver();
    assert.equal(await db.withTransaction(() => 7), 7);
    assert.deepStrictEqual(counts(db), { attempts: 1, failures: 0 });
});

test('an error thrown by the transaction callback is rethrown and not counted twice', async () => {
    const { db, adapter } = loadDriver();
    const netErr = refused();
    adapter.next.push(netErr);
    await assert.rejects(
        db.withTransaction(async (tx) => { await tx.query('SELECT 1'); }),
        (err) => err === netErr
    );
    assert.deepStrictEqual(counts(db), { attempts: 1, failures: 1 });

    const appErr = new Error('validation failed');
    await assert.rejects(db.withTransaction(() => { throw appErr; }), (err) => err === appErr);
    assert.deepStrictEqual(counts(db), { attempts: 1, failures: 1 });
});

test('a connection that cannot be checked out is counted as a failure', async () => {
    const { db, adapter } = loadDriver();
    adapter.failBegin = Object.assign(new Error('timeout exceeded when trying to connect'), {});
    await assert.rejects(db.withTransaction(async () => 'never'), (err) => err === adapter.failBegin);
    const snap = db.outcomeWindow.snapshot();
    assert.deepStrictEqual([snap.attempts, snap.failures, snap.lastFailureKind], [1, 1, 'timeout']);
});

test('a failed commit is counted even though the callback succeeded', async () => {
    const { db, adapter } = loadDriver();
    adapter.failCommit = Object.assign(new Error('Connection terminated unexpectedly'), {});
    await assert.rejects(db.withTransaction(async () => 'ok'), (err) => err === adapter.failCommit);
    assert.deepStrictEqual(counts(db), { attempts: 1, failures: 1 });
});

test('savepoint statements are counted through the transaction handle', async () => {
    const { db } = loadDriver();
    await db.withTransaction((tx) => db.withSavepoint(tx, async () => 'inner'));
    // SAVEPOINT + RELEASE + the transaction itself.
    assert.deepStrictEqual(counts(db), { attempts: 3, failures: 0 });
});

test('the health ping runs SELECT 1 outside any transaction and is not activity', async () => {
    const { db, adapter } = loadDriver();
    await db.withTransaction(async () => {
        await db.healthPing({ timeoutMs: 1000 });
    });
    assert.deepStrictEqual(adapter.calls.map((c) => c.sqlText), ['SELECT 1']);
    const snap = db.outcomeWindow.snapshot();
    assert.ok(snap.lastPing);

    const fresh = loadDriver();
    await fresh.db.healthPing();
    const freshSnap = fresh.db.outcomeWindow.snapshot();
    assert.equal(freshSnap.lastActivity, null);
    assert.equal(freshSnap.attempts, 1);
});

test('a failed health ping is rejected with the driver error and counted', async () => {
    const { db, adapter } = loadDriver();
    const netErr = refused();
    adapter.next.push(netErr);
    await assert.rejects(db.healthPing({ timeoutMs: 1000 }), (err) => err === netErr);
    assert.deepStrictEqual(counts(db), { attempts: 1, failures: 1 });

    adapter.query = () => { throw netErr; };
    await assert.rejects(db.healthPing(), (err) => err === netErr);
    assert.deepStrictEqual(counts(db), { attempts: 2, failures: 2 });
});

test('a health ping that outlasts its timeout is rejected as timed out and counted once', async () => {
    const adapter = fakeAdapter();
    let finish;
    adapter.query = () => new Promise((resolve) => { finish = resolve; });
    const { db } = loadDriver({ adapter });
    await assert.rejects(db.healthPing({ timeoutMs: 10 }), (err) => err.timedOut === true);
    finish([]);
    await new Promise((resolve) => setImmediate(resolve));
    const snap = db.outcomeWindow.snapshot();
    assert.deepStrictEqual([snap.attempts, snap.failures, snap.lastFailureKind], [1, 1, 'timeout']);
    assert.equal(snap.lastActivity, null);
});

test('pool errors reported by the adapter count as failures but not as activity', () => {
    const { db, hooks } = loadDriver();
    hooks().onPoolError(new Error('Connection terminated unexpectedly'));
    const snap = db.outcomeWindow.snapshot();
    assert.deepStrictEqual([snap.attempts, snap.failures, snap.lastActivity], [1, 1, null]);
});

test('poolStats comes from the adapter, and is null when it has none', () => {
    const { db } = loadDriver();
    assert.deepStrictEqual(db.poolStats(), { max: 10, total: 3, idle: 1, waiting: 0 });

    const adapter = fakeAdapter();
    delete adapter.poolStats;
    assert.equal(loadDriver({ driver: 'sqlite', adapter }).db.poolStats(), null);
    assert.equal(loadDriver({ designMode: true }).db.poolStats(), null);
});

test('design mode still refuses every query synchronously', () => {
    const { db } = loadDriver({ designMode: true });
    assert.throws(() => db.query('SELECT 1'), /not available in design mode/);
    assert.throws(() => db.withTransaction(() => {}), /not available in design mode/);
});
