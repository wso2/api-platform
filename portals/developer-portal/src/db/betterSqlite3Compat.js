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

/**
 * Thin adapter that makes better-sqlite3 (synchronous API) look like the
 * sqlite3 package (async callback API) that Sequelize v6 expects.
 *
 * All callbacks are fired via process.nextTick so that callers which capture
 * the constructor's return value (e.g. Sequelize's connection manager does
 * `const connection = new lib.Database(..., cb)` then reads `connection`
 * inside cb) see the assigned variable before the callback runs.
 *
 * Sequelize reads this.lastID / this.changes from the run() callback context,
 * so we call callback.call({ lastID, changes }, err) to match that.
 */
'use strict';
const BetterSqlite3 = require('better-sqlite3');

// better-sqlite3 only accepts numbers, strings, bigints, buffers, and null as bind
// values — unlike the sqlite3 package, it does not coerce booleans or serialize
// plain objects/arrays (used for Sequelize BOOLEAN and JSON column types).
function coerceBindValue(value) {
    if (typeof value === 'boolean') return value ? 1 : 0;
    if (value instanceof Date) return value.toISOString();
    if (value !== null && typeof value === 'object' && !Buffer.isBuffer(value)) {
        return JSON.stringify(value);
    }
    return value;
}

function normalizeParams(params) {
    if (params === null || params === undefined) return [];
    // better-sqlite3 strips the $/@/: prefix from SQL param names when doing lookups,
    // so { $1: val } will fail to match $1 in SQL — strip prefixes from object keys.
    if (!Array.isArray(params) && typeof params === 'object') {
        const out = {};
        for (const key of Object.keys(params)) {
            out[key.replace(/^[$@:]/, '')] = coerceBindValue(params[key]);
        }
        return out;
    }
    return params.map(coerceBindValue);
}

function Database(path, _flags, callback) {
    if (typeof _flags === 'function') { callback = _flags; }

    let db;
    try {
        db = new BetterSqlite3(path);
    } catch (err) {
        if (callback) process.nextTick(() => callback(err));
        return;
    }

    const proxy = {
        serialize(fn) { fn(); },

        run(sql, params, cb) {
            if (typeof params === 'function') { cb = params; params = []; }
            const p = normalizeParams(params);
            try {
                const stmt = db.prepare(sql);
                const info = Array.isArray(p) ? stmt.run(...p) : stmt.run(p);
                if (cb) process.nextTick(() => cb.call({ lastID: info.lastInsertRowid, changes: info.changes }, null));
            } catch (err) {
                if (cb) process.nextTick(() => cb(err));
            }
        },

        all(sql, params, cb) {
            if (typeof params === 'function') { cb = params; params = []; }
            const p = normalizeParams(params);
            try {
                const stmt = db.prepare(sql);
                if (stmt.reader) {
                    const rows = Array.isArray(p) ? stmt.all(...p) : stmt.all(p);
                    // Pass changes = rows.length so Sequelize gets the correct affected-rows
                    // count for UPDATE…RETURNING queries (BULKUPDATE type reads metaData.changes).
                    if (cb) process.nextTick(() => cb.call({ lastID: null, changes: rows.length }, null, rows));
                } else {
                    // Sequelize occasionally routes DDL/DML through all() — handle it
                    const info = Array.isArray(p) ? stmt.run(...p) : stmt.run(p);
                    if (cb) process.nextTick(() => cb.call({ lastID: info.lastInsertRowid, changes: info.changes }, null));
                }
            } catch (err) {
                if (cb) process.nextTick(() => cb(err));
            }
        },

        get(sql, params, cb) {
            if (typeof params === 'function') { cb = params; params = []; }
            const p = normalizeParams(params);
            try {
                const row = Array.isArray(p) ? db.prepare(sql).get(...p) : db.prepare(sql).get(p);
                if (cb) process.nextTick(() => cb(null, row));
            } catch (err) {
                if (cb) process.nextTick(() => cb(err));
            }
        },

        exec(sql, cb) {
            try {
                db.exec(sql);
                if (cb) process.nextTick(() => cb(null));
            } catch (err) {
                if (cb) process.nextTick(() => cb(err));
            }
        },

        close(cb) {
            try {
                db.close();
                if (cb) process.nextTick(() => cb(null));
            } catch (err) {
                if (cb) process.nextTick(() => cb(err));
            }
        }
    };

    if (callback) process.nextTick(() => callback(null));
    return proxy;
}

module.exports = { Database };
