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

// The adapter's job is to give callers what better-sqlite3 gave them, on top of
// node:sqlite. These tests pin each of those normalisations against a real
// database file.

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { createSqliteAdapter } = require('./sqliteAdapter');
const { binaryParam } = require('../paramTypes');

/** Runs fn with an adapter on a fresh database file, removed afterwards. */
async function withAdapter(fn) {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'sqlite-adapter-'));
    const db = createSqliteAdapter({ database: { path: path.join(dir, 'test.db') } });
    try {
        await fn(db);
    } finally {
        await db.close();
        fs.rmSync(dir, { recursive: true, force: true });
    }
}

test('rows come back as plain objects with integers as numbers and BLOBs as Buffers', async () => {
    await withAdapter(async (db) => {
        await db.execute('CREATE TABLE t (id INTEGER PRIMARY KEY, n INTEGER, b BLOB, s TEXT)');
        await db.execute('INSERT INTO t (id, n, b, s) VALUES (?, ?, ?, ?)', [1, 42, Buffer.from('hi'), 'x']);
        const row = await db.queryOne('SELECT * FROM t WHERE id = ?', [1]);
        assert.equal(Object.getPrototypeOf(row), Object.prototype);
        assert.equal(typeof row.n, 'number');
        assert.equal(row.n, 42);
        assert.ok(Buffer.isBuffer(row.b));
        assert.equal(row.b.toString(), 'hi');
    });
});

test('bind values are coerced as better-sqlite3 callers expect', async () => {
    await withAdapter(async (db) => {
        await db.execute('CREATE TABLE t (id INTEGER PRIMARY KEY, a TEXT, flag INTEGER, at TEXT, obj TEXT, bin BLOB)');
        const at = new Date('2026-01-02T03:04:05.000Z');
        const { rowCount, insertId } = await db.execute(
            'INSERT INTO t (a, flag, at, obj, bin) VALUES (?, ?, ?, ?, ?)',
            [undefined, true, at, { k: 1 }, binaryParam(Buffer.from('z'))]
        );
        assert.equal(rowCount, 1);
        assert.equal(insertId, 1);
        const row = await db.queryOne('SELECT * FROM t');
        assert.equal(row.a, null); // undefined binds as NULL instead of throwing
        assert.equal(row.flag, 1);
        assert.equal(row.at, at.toISOString());
        assert.equal(row.obj, '{"k":1}');
        assert.equal(row.bin.toString(), 'z');
    });
});

test('constraint errors carry better-sqlite3 code names', async () => {
    await withAdapter(async (db) => {
        await db.execute('CREATE TABLE t (id INTEGER PRIMARY KEY, u TEXT UNIQUE)');
        await db.execute('INSERT INTO t (id, u) VALUES (1, ?)', ['a']);
        await assert.rejects(db.execute('INSERT INTO t (id, u) VALUES (1, ?)', ['b']),
            { code: 'SQLITE_CONSTRAINT_PRIMARYKEY' });
        await assert.rejects(db.execute('INSERT INTO t (id, u) VALUES (2, ?)', ['a']),
            { code: 'SQLITE_CONSTRAINT_UNIQUE' });
    });
});

test('a failed transaction rolls back', async () => {
    await withAdapter(async (db) => {
        await db.execute('CREATE TABLE t (id INTEGER PRIMARY KEY)');
        await assert.rejects(db.withTransaction(async (tx) => {
            await tx.execute('INSERT INTO t (id) VALUES (1)');
            throw new Error('boom');
        }), /boom/);
        assert.deepEqual(await db.query('SELECT * FROM t'), []);
    });
});

test('an error from COMMIT itself carries the mapped code, and nothing is kept', async () => {
    await withAdapter(async (db) => {
        // A deferred foreign key is checked at COMMIT, so the failure comes from
        // the transaction statement, not from the INSERT.
        await db.execute('CREATE TABLE parent (id INTEGER PRIMARY KEY)');
        await db.execute(
            'CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED)'
        );
        await assert.rejects(db.withTransaction(async (tx) => {
            await tx.execute('INSERT INTO child (id, parent_id) VALUES (1, 99)');
        }), { code: 'SQLITE_CONSTRAINT_FOREIGNKEY' });
        assert.deepEqual(await db.query('SELECT * FROM child'), []);
    });
});
