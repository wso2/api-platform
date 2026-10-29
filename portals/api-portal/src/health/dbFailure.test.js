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

const test = require('node:test');
const assert = require('node:assert');
const { isConnectionFailure, failureKind } = require('./dbFailure');

function error(props) {
    return Object.assign(new Error(props.message || 'boom'), props);
}

test('network failures are counted', () => {
    for (const code of ['ECONNREFUSED', 'ECONNRESET', 'ETIMEDOUT', 'EPIPE', 'ENOTFOUND', 'EAI_AGAIN']) {
        assert.equal(isConnectionFailure(error({ code })), true, code);
    }
});

test('an AggregateError from a multi-address connect is counted through its inner errors', () => {
    const err = new AggregateError([error({ code: 'ECONNREFUSED' }), error({ code: 'ECONNREFUSED' })], 'connect failed');
    assert.equal(isConnectionFailure(err), true);
    assert.equal(isConnectionFailure(new AggregateError([error({ code: '23505' })])), false);
});

test('PostgreSQL connection, authentication and shutdown states are counted', () => {
    for (const code of ['08000', '08006', '08001', '28P01', '28000', '57P01', '57P02', '57P03']) {
        assert.equal(isConnectionFailure(error({ code })), true, code);
    }
});

test('node-postgres connection messages are counted', () => {
    assert.equal(isConnectionFailure(new Error('Connection terminated unexpectedly')), true);
    assert.equal(isConnectionFailure(new Error('Connection terminated due to connection timeout')), true);
    assert.equal(isConnectionFailure(new Error('timeout exceeded when trying to connect')), true);
});

test('SQL Server connection failures are counted', () => {
    assert.equal(isConnectionFailure(error({ name: 'ConnectionError', code: 'EINSTLOOKUP' })), true);
    for (const code of ['ESOCKET', 'ETIMEOUT', 'ELOGIN']) {
        assert.equal(isConnectionFailure(error({ code })), true, code);
    }
});

test('a SQL Server statement outliving its request timeout is not counted', () => {
    assert.equal(isConnectionFailure(error({ name: 'RequestError', code: 'ETIMEOUT' })), false);
});

test('SQLite open, I/O and corruption failures are counted', () => {
    for (const code of ['SQLITE_CANTOPEN', 'SQLITE_IOERR', 'SQLITE_IOERR_READ', 'SQLITE_NOTADB', 'SQLITE_CORRUPT']) {
        assert.equal(isConnectionFailure(error({ code })), true, code);
    }
});

test('statement errors and everything else are not counted', () => {
    const notCounted = [
        error({ code: '23505' }),            // unique violation
        error({ code: '42601' }),            // syntax error
        error({ code: '40001' }),            // serialization failure
        error({ code: '57014' }),            // query cancelled
        error({ code: 'SQLITE_CONSTRAINT_UNIQUE' }),
        error({ code: 'SQLITE_BUSY' }),
        error({ name: 'RequestError', code: 'EREQUEST' }),
        error({ name: 'AbortError', code: 'ABORT_ERR' }),
        new Error('relation "x" does not exist'),
        null,
        undefined,
        'ECONNREFUSED',
    ];
    for (const err of notCounted) {
        assert.equal(isConnectionFailure(err), false, String(err && (err.code || err.message)));
    }
});

test('failureKind tells timeouts apart from other failures', () => {
    assert.equal(failureKind(error({ code: 'ETIMEDOUT' })), 'timeout');
    assert.equal(failureKind(error({ code: 'ETIMEOUT' })), 'timeout');
    assert.equal(failureKind(new Error('timeout exceeded when trying to connect')), 'timeout');
    assert.equal(failureKind(new Error('Connection terminated due to connection timeout')), 'timeout');
    assert.equal(failureKind(error({ code: 'ECONNREFUSED' })), 'unreachable');
    assert.equal(failureKind(undefined), 'unreachable');
});
