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

// Read-only access to the devportal database so specs can assert on rows
// (DP_EVENTS, DP_EVENT_DELIVERIES, etc.) that aren't exposed via the REST API.
// Branches on DEVPORTAL_DB_DIALECT since IT runs against both SQLite and Postgres.

const DIALECT = process.env.DEVPORTAL_DB_DIALECT || 'sqlite';

let pool;
let sqliteDb;

function getSqlite() {
    if (!sqliteDb) {
        const Database = require('better-sqlite3');
        sqliteDb = new Database(process.env.DEVPORTAL_DB_STORAGE, { readonly: true, fileMustExist: true });
    }
    return sqliteDb;
}

function getPgPool() {
    if (!pool) {
        const { Pool } = require('pg');
        pool = new Pool({
            host: process.env.DEVPORTAL_DB_HOST,
            port: Number(process.env.DEVPORTAL_DB_PORT || 5432),
            user: process.env.DEVPORTAL_DB_USERNAME,
            password: process.env.DEVPORTAL_DB_PASSWORD,
            database: process.env.DEVPORTAL_DB_DATABASE,
        });
    }
    return pool;
}

// Runs a SELECT and returns rows as plain objects, regardless of dialect.
// `sql` must use $1/$2-style placeholders; they're rewritten to `?` for SQLite.
async function query(sql, params = []) {
    if (DIALECT === 'postgres') {
        const { rows } = await getPgPool().query(sql, params);
        return rows;
    }
    const sqliteSql = sql.replace(/\$\d+/g, '?');
    return getSqlite().prepare(sqliteSql).all(...params);
}

// dp_organizations.id (the REST `id`/handle) is not the internal uuid stored as
// org_uuid on dp_events etc. — resolve it once per org via its unique handle.
async function findOrgUuidByHandle(handle) {
    const rows = await query('SELECT uuid FROM dp_organizations WHERE handle = $1', [handle]);
    return rows[0]?.uuid;
}

// Columns per database/schema.postgres.sql: dp_events(uuid, type, org_uuid,
// aggregate_type, aggregate_uuid, payload, occurred_at, status).
// `since` (a Date) scopes results to events from the current test only, since
// aggregate_uuid is an internal id the REST responses never expose.
async function findEvents({ orgUuid, type, aggregateUuid, since }) {
    const clauses = [];
    const params = [];
    if (orgUuid) { params.push(orgUuid); clauses.push(`org_uuid = $${params.length}`); }
    if (type) { params.push(type); clauses.push(`type = $${params.length}`); }
    if (aggregateUuid) { params.push(aggregateUuid); clauses.push(`aggregate_uuid = $${params.length}`); }
    if (since) {
        params.push(since.toISOString());
        // SQLite stores occurred_at as "YYYY-MM-DD HH:MM:SS.SSS +00:00" (space-
        // separated, explicit offset) — a plain string >= against an ISO
        // "...T...Z" value compares wrong lexicographically. datetime() normalizes
        // both sides. Postgres compares timestamptz natively; no wrapping needed.
        clauses.push(DIALECT === 'postgres'
            ? `occurred_at >= $${params.length}`
            : `datetime(occurred_at) >= datetime($${params.length})`);
    }
    const where = clauses.length ? `WHERE ${clauses.join(' AND ')}` : '';
    return query(`SELECT * FROM dp_events ${where} ORDER BY occurred_at DESC`, params);
}

// Columns per database/schema.postgres.sql: dp_event_deliveries(uuid, event_uuid,
// subscriber_id, target_url, encrypted_fields, status, last_http_status,
// last_error, last_attempt_at, delivered_at).
async function findDeliveries({ eventUuid, subscriberId }) {
    const clauses = [];
    const params = [];
    if (eventUuid) { params.push(eventUuid); clauses.push(`event_uuid = $${params.length}`); }
    if (subscriberId) { params.push(subscriberId); clauses.push(`subscriber_id = $${params.length}`); }
    const where = clauses.length ? `WHERE ${clauses.join(' AND ')}` : '';
    return query(`SELECT * FROM dp_event_deliveries ${where}`, params);
}

async function close() {
    if (sqliteDb) sqliteDb.close();
    if (pool) await pool.end();
}

module.exports = { query, findEvents, findDeliveries, findOrgUuidByHandle, close };
