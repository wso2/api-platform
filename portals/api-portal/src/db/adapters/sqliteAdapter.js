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

const fs = require('fs');
const path = require('path');
const { DatabaseSync } = require('node:sqlite');
const AsyncLock = require('async-lock');
const { isBinaryParam } = require('../paramTypes');

// SQLite result codes, named as better-sqlite3 named them in `err.code`
// (https://www.sqlite.org/rescode.html). node:sqlite sets code 'ERR_SQLITE_ERROR'
// and the number in `errcode`; callers such as rebind.isDuplicateKeyError() and
// the logs keep seeing the familiar name.
const SQLITE_CODE_NAMES = {
    1: 'SQLITE_ERROR',
    5: 'SQLITE_BUSY',
    6: 'SQLITE_LOCKED',
    8: 'SQLITE_READONLY',
    10: 'SQLITE_IOERR',
    11: 'SQLITE_CORRUPT',
    13: 'SQLITE_FULL',
    14: 'SQLITE_CANTOPEN',
    19: 'SQLITE_CONSTRAINT',
    26: 'SQLITE_NOTADB',
    261: 'SQLITE_BUSY_RECOVERY',
    262: 'SQLITE_LOCKED_SHAREDCACHE',
    275: 'SQLITE_CONSTRAINT_CHECK',
    517: 'SQLITE_BUSY_SNAPSHOT',
    787: 'SQLITE_CONSTRAINT_FOREIGNKEY',
    1043: 'SQLITE_CONSTRAINT_FUNCTION',
    1299: 'SQLITE_CONSTRAINT_NOTNULL',
    1555: 'SQLITE_CONSTRAINT_PRIMARYKEY',
    1811: 'SQLITE_CONSTRAINT_TRIGGER',
    2067: 'SQLITE_CONSTRAINT_UNIQUE',
    2579: 'SQLITE_CONSTRAINT_ROWID',
};

function withSqliteCode(err) {
    if (err && err.code === 'ERR_SQLITE_ERROR' && typeof err.errcode === 'number') {
        err.code = SQLITE_CODE_NAMES[err.errcode] || SQLITE_CODE_NAMES[err.errcode & 0xff] || err.code;
    }
    return err;
}

function toPlainValue(value) {
    if (typeof value === 'bigint') return Number(value);
    if (value instanceof Uint8Array && !Buffer.isBuffer(value)) {
        return Buffer.from(value.buffer, value.byteOffset, value.byteLength);
    }
    return value;
}

function toPlainRow(row) {
    const plain = {};
    for (const key of Object.keys(row)) plain[key] = toPlainValue(row[key]);
    return plain;
}

/**
 * SQLite adapter (node:sqlite, built into Node). node:sqlite is synchronous and
 * this uses a single shared connection (SQLite is single-writer, matching the
 * pool max=1/min=1 the previous Sequelize config used).
 *
 * This adapter used better-sqlite3 until the move to node:sqlite. Callers see
 * the same results as before, because the differences are smoothed over here:
 *   - BLOB columns come back as Uint8Array; they are returned as Buffer.
 *   - Rows are null-prototype objects; they are returned as plain objects.
 *   - Integers are read as BigInt and converted with Number(), which matches
 *     better-sqlite3 (a value above 2^53 loses precision instead of throwing).
 *   - Binding `undefined` throws; it is bound as NULL, as better-sqlite3 did.
 *   - Errors carry `code: 'ERR_SQLITE_ERROR'` and a numeric `errcode`; the code
 *     is replaced with better-sqlite3's name (e.g. SQLITE_CONSTRAINT_UNIQUE).
 *   - node:sqlite does not wait for a locked file by default; better-sqlite3
 *     waited 5 s. The same 5 s is passed when opening.
 *   - Node's SQLite build defaults to synchronous=FULL in WAL mode and a 2 MB
 *     page cache; better-sqlite3's build used NORMAL and 16 MB. Both are set
 *     explicitly below so write speed and durability stay as they were.
 *
 * Because JS can interleave other requests during any `await` inside a
 * transaction callback, a bare shared connection would let an unrelated
 * request's query execute *inside* another request's still-open
 * transaction — a silent-corruption / cross-tenant-isolation risk (a
 * rollback would undo the other request's writes too). `async-lock`
 * serializes all access to the connection: every standalone query acquires
 * and releases the lock instantly; withTransaction() holds it for the whole
 * BEGIN...COMMIT/ROLLBACK window, so nothing else can touch the connection
 * until the transaction finishes.
 */
function createSqliteAdapter(config) {
    const storage = config.database.path || './api-portal.db';
    const dir = path.dirname(storage);
    if (dir && dir !== '.') {
        fs.mkdirSync(dir, { recursive: true });
    }

    // timeout: how long opening the file and the PRAGMAs below wait for another
    // connection's lock. better-sqlite3 waited 5 s by default; node:sqlite waits
    // not at all, so a second process opening a busy file would fail at once.
    const conn = new DatabaseSync(storage, { timeout: 5000 });
    conn.exec('PRAGMA journal_mode = WAL');
    conn.exec('PRAGMA busy_timeout = 5000');
    conn.exec('PRAGMA foreign_keys = ON');
    conn.exec('PRAGMA synchronous = NORMAL');
    conn.exec('PRAGMA cache_size = -16000');

    const lock = new AsyncLock();
    const LOCK_KEY = 'sqlite-conn';

    function coerceBindValue(value) {
        // paramTypes.binaryParam() only exists to give the mssql adapter an
        // explicit type for a nullable VARBINARY(MAX) bind — SQLite has no such
        // inference gap, so just unwrap to the underlying value.
        if (isBinaryParam(value)) value = value.value;
        if (value === undefined) return null;
        if (typeof value === 'boolean') return value ? 1 : 0;
        if (value instanceof Date) return value.toISOString();
        if (value !== null && typeof value === 'object' && !Buffer.isBuffer(value)) {
            return JSON.stringify(value);
        }
        return value;
    }

    function runSync(sqlText, params) {
        try {
            const stmt = conn.prepare(sqlText);
            stmt.setReadBigInts(true);
            const boundParams = (params || []).map(coerceBindValue);
            if (stmt.columns().length > 0) {
                const rows = stmt.all(...boundParams).map(toPlainRow);
                return { rows, rowCount: rows.length, insertId: null };
            }
            const info = stmt.run(...boundParams);
            return { rows: [], rowCount: Number(info.changes), insertId: Number(info.lastInsertRowid) };
        } catch (err) {
            throw withSqliteCode(err);
        }
    }

    function execSync(sqlText) {
        try {
            conn.exec(sqlText);
        } catch (err) {
            throw withSqliteCode(err);
        }
    }

    // Raw (non-locking) handle — used internally, either because the caller
    // already holds the lock (inside withTransaction) or for the top-level
    // handle's own lock-wrapped methods below.
    const raw = {
        getDialect: () => 'sqlite',
        query: async (sqlText, params) => runSync(sqlText, params).rows,
        queryOne: async (sqlText, params) => runSync(sqlText, params).rows[0] || null,
        execute: async (sqlText, params) => {
            const { rowCount, insertId } = runSync(sqlText, params);
            return { rowCount, insertId };
        },
    };

    return {
        getDialect: () => 'sqlite',
        query: (sqlText, params) => lock.acquire(LOCK_KEY, () => raw.query(sqlText, params)),
        queryOne: (sqlText, params) => lock.acquire(LOCK_KEY, () => raw.queryOne(sqlText, params)),
        execute: (sqlText, params) => lock.acquire(LOCK_KEY, () => raw.execute(sqlText, params)),
        withTransaction: (fn) => lock.acquire(LOCK_KEY, async () => {
            execSync('BEGIN IMMEDIATE');
            try {
                const result = await fn(raw); // raw handle: lock is already held for the duration
                execSync('COMMIT');
                return result;
            } catch (err) {
                try {
                    execSync('ROLLBACK');
                } catch (_rollbackErr) {
                    // Connection may already be out of a transaction (e.g. COMMIT itself
                    // failed) — nothing more we can do; surface the original error.
                }
                throw err;
            }
        }),
        close: async () => conn.close(),
    };
}

module.exports = { createSqliteAdapter };
