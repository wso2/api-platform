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

// One-shot init for docker-compose.test.mssql.yaml: waits for SQL Server, then
// creates one database per multi-tenancy portal instance and applies the SQL Server
// schema to each. Done from Node rather than sqlcmd because the ARM image used on
// Apple Silicon (azure-sql-edge) ships without the command-line tools.

const fs = require('fs');
const sql = require('mssql');

const DATABASES = ['api_portal_multi_tenancy', 'api_portal_multi_tenancy_strict'];
const base = {
    server: process.env.MSSQL_HOST || 'mssql',
    user: 'sa',
    password: process.env.MSSQL_SA_PASSWORD,
    options: { encrypt: false, trustServerCertificate: true },
};

async function connectWithRetry(database, attempts = 60) {
    for (let i = 1; ; i++) {
        try {
            return await new sql.ConnectionPool({ ...base, database }).connect();
        } catch (err) {
            if (i >= attempts) throw err;
            await new Promise((r) => setTimeout(r, 2000));
        }
    }
}

(async () => {
    const schema = fs.readFileSync('/schema/schema.sqlserver.sql', 'utf8');
    const master = await connectWithRetry('master');
    for (const db of DATABASES) {
        await master.request().batch(`IF DB_ID('${db}') IS NULL CREATE DATABASE [${db}]`);
    }
    await master.close();
    for (const db of DATABASES) {
        const pool = await connectWithRetry(db);
        for (const batch of schema.split(/^\s*GO\s*$/mi)) {
            if (batch.trim()) await pool.request().batch(batch);
        }
        await pool.close();
        console.log(`[mssql-init] ${db}: schema applied`);
    }
})().catch((err) => {
    console.error('[mssql-init] failed:', err.message);
    process.exit(1);
});
