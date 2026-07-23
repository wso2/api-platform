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

const { Pool } = require('pg');
const { buildDbSsl } = require('../dbSsl');

/** PostgreSQL adapter (`pg`). Pool-backed; transactions check out a dedicated client. */
function createPostgresAdapter(config) {
    const poolConfig = {
        user: config.database.user,
        host: config.database.host,
        database: config.database.name,
        password: config.database.password,
        port: config.database.port,
        max: 50,
        min: 2,
        idleTimeoutMillis: 10000,
        connectionTimeoutMillis: 30000,
    };

    // buildDbSsl() reads config.database.sslMode/sslRootCert and fails closed
    // on an unrecognized mode — the same helper dbSsl.js's docstring says is
    // shared across the database layer, so this pool's TLS handling matches it.
    const ssl = buildDbSsl();
    if (ssl) {
        poolConfig.ssl = ssl;
    }

    const pool = new Pool(poolConfig);

    // `pg` auto-stringifies plain JS objects for us, but we JSON.stringify explicitly
    // here anyway so JSONB column behavior doesn't silently depend on that driver
    // internal — same coercion point exists on the sqlite/mssql adapters, where it
    // is NOT optional (their drivers have no such auto-serialization).
    function coerceParams(params) {
        return (params || []).map((value) => {
            if (value !== null && typeof value === 'object' &&
                !(value instanceof Date) && !Buffer.isBuffer(value) && !Array.isArray(value)) {
                return JSON.stringify(value);
            }
            return value;
        });
    }

    function makeHandle(runner) {
        return {
            getDialect: () => 'postgres',
            query: async (sqlText, params) => (await runner.query(sqlText, coerceParams(params))).rows,
            queryOne: async (sqlText, params) => {
                const rows = (await runner.query(sqlText, coerceParams(params))).rows;
                return rows[0] || null;
            },
            execute: async (sqlText, params) => {
                const result = await runner.query(sqlText, coerceParams(params));
                return { rowCount: result.rowCount, insertId: null };
            },
        };
    }

    const handle = makeHandle(pool);

    handle.withTransaction = async (fn) => {
        const client = await pool.connect();
        try {
            await client.query('BEGIN');
            const result = await fn(makeHandle(client));
            await client.query('COMMIT');
            return result;
        } catch (err) {
            try {
                await client.query('ROLLBACK');
            } catch (_rollbackErr) {
                // Connection may already be broken (e.g. COMMIT itself failed) —
                // nothing more we can do; surface the original error.
            }
            throw err;
        } finally {
            client.release();
        }
    };

    handle.close = async () => pool.end();

    return handle;
}

module.exports = { createPostgresAdapter };
