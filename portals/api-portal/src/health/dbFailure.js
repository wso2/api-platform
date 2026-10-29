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
 * Decides which database errors say "the database cannot be reached", as opposed to
 * "this statement failed". Only the first kind counts against database health: a
 * syntax error, a constraint violation, a missing row or a slow query is the caller's
 * problem, not an outage.
 */

// Socket and resolver failures, from Node itself or surfaced by any of the drivers.
const NETWORK_CODES = new Set(['ECONNREFUSED', 'ECONNRESET', 'ETIMEDOUT', 'EPIPE', 'ENOTFOUND', 'EAI_AGAIN']);

// PostgreSQL SQLSTATEs: admin shutdown, crash shutdown, cannot connect now. Classes 08
// (connection exception) and 28 (invalid authorization) are matched by prefix.
const PG_SHUTDOWN_CODES = new Set(['57P01', '57P02', '57P03']);
const PG_CONNECTION_CLASSES = ['08', '28'];

// node-postgres reports a dropped or unobtainable connection with these messages only.
const PG_CONNECTION_MESSAGES = [/^Connection terminated/, /timeout exceeded when trying to connect/];

// SQL Server driver codes for a broken socket, a connect timeout and a failed login.
const MSSQL_CODES = new Set(['ESOCKET', 'ETIMEOUT', 'ELOGIN']);

// better-sqlite3 codes for a database file that cannot be opened, read or trusted.
const SQLITE_CODES = new Set(['SQLITE_CANTOPEN', 'SQLITE_NOTADB']);
const SQLITE_CODE_PREFIXES = ['SQLITE_IOERR', 'SQLITE_CORRUPT'];

const TIMEOUT_CODES = new Set(['ETIMEDOUT', 'ETIMEOUT']);
// node-postgres: "timeout exceeded when trying to connect" (pool) and "Connection
// terminated due to connection timeout" (client).
const TIMEOUT_MESSAGE = /timeout exceeded when trying to connect|due to connection timeout/;

function isPgSqlState(code) {
    return typeof code === 'string' && /^[0-9A-Z]{5}$/.test(code);
}

function matchesOne(err) {
    const code = err.code;
    if (NETWORK_CODES.has(code)) return true;
    if (isPgSqlState(code) && (PG_SHUTDOWN_CODES.has(code) || PG_CONNECTION_CLASSES.includes(code.slice(0, 2)))) {
        return true;
    }
    if (typeof err.message === 'string' && PG_CONNECTION_MESSAGES.some((re) => re.test(err.message))) {
        return true;
    }
    if (err.name === 'ConnectionError') return true;
    // A RequestError ETIMEOUT is a statement outliving its own request timeout, which is
    // a slow query rather than an unreachable database.
    if (MSSQL_CODES.has(code) && err.name !== 'RequestError') return true;
    if (typeof code === 'string' &&
        (SQLITE_CODES.has(code) || SQLITE_CODE_PREFIXES.some((prefix) => code.startsWith(prefix)))) {
        return true;
    }
    return false;
}

/**
 * Reports whether `err` is a connection-class failure. A connect attempt that tried
 * several addresses fails with an AggregateError, so its inner errors are checked too.
 */
function isConnectionFailure(err) {
    if (!err || typeof err !== 'object') return false;
    if (matchesOne(err)) return true;
    return Array.isArray(err.errors) && err.errors.some((inner) => inner && typeof inner === 'object' && matchesOne(inner));
}

/** Classifies a connection failure for its reason text: 'timeout' or 'unreachable'. */
function failureKind(err) {
    if (err && (TIMEOUT_CODES.has(err.code) || TIMEOUT_MESSAGE.test(String(err.message)))) {
        return 'timeout';
    }
    return 'unreachable';
}

module.exports = { isConnectionFailure, failureKind };
