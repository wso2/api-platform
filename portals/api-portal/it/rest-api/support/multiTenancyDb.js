// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// Read-only access to the multi-tenancy portal's own database (it has one, unlike the
// shared instance db.js reads), so multi-tenancy/multi-tenancy.spec.js can assert on which
// organization rows exist. Configured by MULTI_TENANCY_DB_* in the compose files; the
// dialect is sqlite, postgres or mssql. Queries take `?` placeholders.

const DIALECT = process.env.MULTI_TENANCY_DB_DIALECT || 'sqlite';
let handle;

async function connect() {
    if (handle) return handle;
    if (DIALECT === 'sqlite') {
        const Database = require('better-sqlite3');
        const db = new Database(process.env.MULTI_TENANCY_DB_STORAGE, { readonly: true, fileMustExist: true });
        handle = { query: async (sql, params) => db.prepare(sql).all(...params), close: async () => db.close() };
    } else if (DIALECT === 'postgres') {
        const { Pool } = require('pg');
        const pool = new Pool({
            host: process.env.MULTI_TENANCY_DB_HOST,
            port: Number(process.env.MULTI_TENANCY_DB_PORT || 5432),
            user: process.env.MULTI_TENANCY_DB_USERNAME,
            password: process.env.MULTI_TENANCY_DB_PASSWORD,
            database: process.env.MULTI_TENANCY_DB_DATABASE,
        });
        handle = {
            query: async (sql, params) => {
                let i = 0;
                return (await pool.query(sql.replace(/\?/g, () => `$${++i}`), params)).rows;
            },
            close: () => pool.end(),
        };
    } else if (DIALECT === 'mssql') {
        const sql = require('mssql');
        const pool = await new sql.ConnectionPool({
            server: process.env.MULTI_TENANCY_DB_HOST,
            port: Number(process.env.MULTI_TENANCY_DB_PORT || 1433),
            user: process.env.MULTI_TENANCY_DB_USERNAME,
            password: process.env.MULTI_TENANCY_DB_PASSWORD,
            database: process.env.MULTI_TENANCY_DB_DATABASE,
            options: { encrypt: false, trustServerCertificate: true },
        }).connect();
        handle = {
            query: async (text, params) => {
                const req = pool.request();
                let i = 0;
                params.forEach((v, idx) => req.input(`p${idx + 1}`, v));
                return (await req.query(text.replace(/\?/g, () => `@p${++i}`))).recordset;
            },
            close: () => pool.close(),
        };
    } else {
        throw new Error(`Unsupported MULTI_TENANCY_DB_DIALECT: ${DIALECT}`);
    }
    return handle;
}

async function query(sql, params = []) {
    return (await connect()).query(sql, params);
}

/** Organization rows whose idp_ref_id is exactly `idpRefId`. */
function orgsByIdpRefId(idpRefId) {
    return query('SELECT uuid, handle, display_name, idp_ref_id FROM organizations WHERE idp_ref_id = ?', [idpRefId]);
}

async function close() {
    if (handle) await handle.close();
    handle = undefined;
}

module.exports = { query, orgsByIdpRefId, close };
