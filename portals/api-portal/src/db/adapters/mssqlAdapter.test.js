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
 * Parameter binding for SQL Server. ASCII strings must bind as VARCHAR — as NVARCHAR
 * they force a CONVERT_IMPLICIT on every VARCHAR key column, turning primary-key seeks
 * into scans that deadlock concurrent writers — while any non-ASCII string must keep
 * NVARCHAR so it isn't squeezed into the database code page.
 */

const test = require('node:test');
const assert = require('node:assert');

const MSSQL_PATH = require.resolve('mssql');
const ADAPTER_PATH = require.resolve('./mssqlAdapter');
const { binaryParam } = require('../paramTypes');

/** Loads the adapter against a fake `mssql` that records every request.input() call. */
function loadAdapter() {
    const inputs = [];
    const fakeMssql = {
        MAX: 'MAX',
        VarChar: (len) => ({ type: 'VarChar', len }),
        VarBinary: (len) => ({ type: 'VarBinary', len }),
        ConnectionPool: class {
            connect() { return Promise.resolve(); }
            on() {}
            request() {
                return {
                    input: (...args) => inputs.push(args),
                    query: async () => ({ recordset: [], rowsAffected: [1] }),
                };
            }
        },
    };
    require.cache[MSSQL_PATH] = { id: MSSQL_PATH, filename: MSSQL_PATH, loaded: true, exports: fakeMssql };
    delete require.cache[ADAPTER_PATH];
    try {
        const { createMssqlAdapter } = require(ADAPTER_PATH);
        return { adapter: createMssqlAdapter({ database: {} }), inputs };
    } finally {
        delete require.cache[MSSQL_PATH];
        delete require.cache[ADAPTER_PATH];
    }
}

test('an ASCII string binds as VARCHAR(8000)', async () => {
    const { adapter, inputs } = loadAdapter();
    await adapter.execute('UPDATE events SET status = @p1 WHERE uuid = @p2', ['DISPATCHED', '3f6c2b1e-9d1a-4c1e-8a57-0e2f1b9d7c44']);
    assert.deepStrictEqual(inputs, [
        ['p1', { type: 'VarChar', len: 8000 }, 'DISPATCHED'],
        ['p2', { type: 'VarChar', len: 8000 }, '3f6c2b1e-9d1a-4c1e-8a57-0e2f1b9d7c44'],
    ]);
});

test('a string with any non-ASCII character keeps the driver\'s NVARCHAR inference', async () => {
    const { adapter, inputs } = loadAdapter();
    await adapter.execute('UPDATE organizations SET display_name = @p1', ['Défault 組織']);
    assert.deepStrictEqual(inputs, [['p1', 'Défault 組織']]);
});

test('an over-long string keeps the default binding', async () => {
    const { adapter, inputs } = loadAdapter();
    const long = 'x'.repeat(8001);
    await adapter.execute('UPDATE t SET body = @p1', [long]);
    assert.deepStrictEqual(inputs, [['p1', long]]);
});

test('non-string values are untouched, and an ASCII JSON object binds as VARCHAR', async () => {
    const { adapter, inputs } = loadAdapter();
    const when = new Date(0);
    const buf = Buffer.from('ab');
    await adapter.execute('X', [null, 7, when, buf, { a: 1 }, binaryParam(null)]);
    assert.deepStrictEqual(inputs, [
        ['p1', null],
        ['p2', 7],
        ['p3', when],
        ['p4', buf],
        ['p5', { type: 'VarChar', len: 8000 }, '{"a":1}'],
        ['p6', { type: 'VarBinary', len: 'MAX' }, null],
    ]);
});
