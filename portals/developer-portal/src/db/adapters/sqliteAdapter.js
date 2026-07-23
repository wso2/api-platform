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
const BetterSqlite3 = require('better-sqlite3');
const AsyncLock = require('async-lock');

/**
 * SQLite adapter (better-sqlite3). better-sqlite3 is synchronous and this
 * uses a single shared connection (SQLite is single-writer, matching the
 * pool max=1/min=1 the previous Sequelize config used).
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
    const storage = config.database.path || './devportal.db';
    const dir = path.dirname(storage);
    if (dir && dir !== '.') {
        fs.mkdirSync(dir, { recursive: true });
    }

    const conn = new BetterSqlite3(storage);
    conn.pragma('journal_mode = WAL');
    conn.pragma('busy_timeout = 5000');
    conn.pragma('foreign_keys = ON');

    const lock = new AsyncLock();
    const LOCK_KEY = 'sqlite-conn';

    function coerceBindValue(value) {
        if (typeof value === 'boolean') return value ? 1 : 0;
        if (value instanceof Date) return value.toISOString();
        if (value !== null && typeof value === 'object' && !Buffer.isBuffer(value)) {
            return JSON.stringify(value);
        }
        return value;
    }

    function runSync(sqlText, params) {
        const stmt = conn.prepare(sqlText);
        const boundParams = (params || []).map(coerceBindValue);
        if (stmt.reader) {
            const rows = stmt.all(...boundParams);
            return { rows, rowCount: rows.length, insertId: null };
        }
        const info = stmt.run(...boundParams);
        return { rows: [], rowCount: info.changes, insertId: info.lastInsertRowid };
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
            conn.exec('BEGIN IMMEDIATE');
            try {
                const result = await fn(raw); // raw handle: lock is already held for the duration
                conn.exec('COMMIT');
                return result;
            } catch (err) {
                try {
                    conn.exec('ROLLBACK');
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
