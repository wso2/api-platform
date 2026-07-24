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

const sql = require('mssql');

/**
 * SQL Server adapter (`mssql`, Tedious-based). Unlike `pg`, the driver has no
 * positional-`?` query mode — parameters are bound by name and referenced as
 * `@p1, @p2, ...` in the SQL text. rebind() (see ../rebind.js) already
 * produces exactly that placeholder form for the mssql dialect, so binding
 * here just needs to name each incoming param `p1, p2, ...` in order.
 */
function buildConnectionConfig(config) {
    const db = config.database;
    // sslMode is shared with the postgres adapter's config.database.sslMode
    // (see ../dbSsl.js) — 'verify-full' here maps to encrypt+cert verification,
    // 'disable' to a plain, unencrypted connection.
    const verifyFull = db.sslMode === 'verify-full';
    return {
        server: db.host,
        port: db.port,
        database: db.name,
        user: db.user,
        password: db.password,
        options: {
            encrypt: verifyFull,
            trustServerCertificate: !verifyFull,
        },
        pool: { max: db.maxOpenConns, min: db.minOpenConns, idleTimeoutMillis: db.poolIdleTimeoutMs },
        connectionTimeout: db.poolConnectionTimeoutMs,
        requestTimeout: db.poolRequestTimeoutMs,
    };
}

function createMssqlAdapter(config) {
    const pool = new sql.ConnectionPool(buildConnectionConfig(config));
    const ready = pool.connect();
    // Every caller awaits `ready` before issuing a query (see run()/withTransaction()
    // below), so a connect() failure still surfaces correctly to the first caller —
    // but until that first await happens, an unhandled rejection here would crash
    // the process. Attach a no-op catch immediately to prevent that; the real error
    // still propagates through every `await ready` call site.
    ready.catch(() => { /* surfaced to callers via query rejections */ });
    // Surface pool-level connection errors instead of letting them crash the
    // process as unhandled 'error' events (mssql/tedious convention).
    pool.on('error', () => { /* connection errors surface to callers via query rejections */ });

    // Unlike `pg`, the `mssql` package does not serialize plain JS objects for
    // NVARCHAR(MAX) "JSON column" storage (this schema has no native JSON type on
    // SQL Server) — it infers a SQL type from the JS value and has no case for a
    // generic object, so an un-stringified object would fail to bind. Arrays,
    // Dates, and Buffers are left alone (mssql/tedious handle those natively).
    function coerceParamValue(value) {
        if (value !== null && typeof value === 'object' &&
            !(value instanceof Date) && !Buffer.isBuffer(value) && !Array.isArray(value)) {
            return JSON.stringify(value);
        }
        return value;
    }

    function bindParams(request, params) {
        (params || []).forEach((value, idx) => {
            request.input(`p${idx + 1}`, coerceParamValue(value));
        });
    }

    async function run(requestFactory, sqlText, params) {
        await ready;
        const request = requestFactory();
        bindParams(request, params);
        return request.query(sqlText);
    }

    function makeHandle(requestFactory) {
        return {
            getDialect: () => 'mssql',
            query: async (sqlText, params) => (await run(requestFactory, sqlText, params)).recordset || [],
            queryOne: async (sqlText, params) => {
                const rows = (await run(requestFactory, sqlText, params)).recordset || [];
                return rows[0] || null;
            },
            execute: async (sqlText, params) => {
                const result = await run(requestFactory, sqlText, params);
                return { rowCount: result.rowsAffected?.[0] ?? 0, insertId: null };
            },
        };
    }

    const handle = makeHandle(() => pool.request());

    handle.withTransaction = async (fn) => {
        await ready;
        const transaction = new sql.Transaction(pool);
        await transaction.begin();
        try {
            const result = await fn(makeHandle(() => new sql.Request(transaction)));
            await transaction.commit();
            return result;
        } catch (err) {
            try {
                await transaction.rollback();
            } catch (_rollbackErr) {
                // Transaction may already be aborted by SQL Server (e.g. a doomed
                // transaction after certain errors) — nothing more we can do.
            }
            throw err;
        }
    };

    handle.close = async () => pool.close();

    return handle;
}

module.exports = { createMssqlAdapter };
