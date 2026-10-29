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
 * An idle pooled connection that drops — the database restarting under the portal —
 * makes the pg pool emit 'error'. Without a listener that emit throws, which surfaces
 * as an uncaughtException and exits the process. These run against the real `pg`
 * Pool (no connection is ever opened), so they prove the listener is on the pool the
 * adapter actually uses.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const CONFIG_PATH = path.resolve(__dirname, '../../config/configLoader.js');
const DB_SSL_PATH = path.resolve(__dirname, '../dbSsl.js');
const ADAPTER_PATH = path.resolve(__dirname, './postgresAdapter.js');

const DATABASE = {
    host: '127.0.0.1', port: 1, name: 'x', user: 'x', password: 'x',
    maxOpenConns: 7, minOpenConns: 0, poolIdleTimeoutMs: 1000, poolConnectionTimeoutMs: 1000,
    sslMode: 'disable',
};

function loadAdapter(hooks) {
    const stub = (p, exports) => { require.cache[p] = { id: p, filename: p, loaded: true, exports }; };
    stub(CONFIG_PATH, { config: { database: DATABASE } });
    delete require.cache[DB_SSL_PATH];
    delete require.cache[ADAPTER_PATH];
    try {
        return require(ADAPTER_PATH).createPostgresAdapter({ database: DATABASE }, hooks);
    } finally {
        delete require.cache[CONFIG_PATH];
        delete require.cache[DB_SSL_PATH];
        delete require.cache[ADAPTER_PATH];
    }
}

test('an idle-client error is handed to the hook instead of being thrown', async (t) => {
    const { Pool } = require('pg');
    const originalOn = Pool.prototype.on;
    let pool;
    Pool.prototype.on = function on(...args) {
        pool = this;
        return originalOn.apply(this, args);
    };
    t.after(() => { Pool.prototype.on = originalOn; });

    const seen = [];
    const adapter = loadAdapter({ onPoolError: (err) => seen.push(err) });
    t.after(() => adapter.close());
    assert.ok(pool, 'the adapter registered a listener on its pool');

    const err = new Error('Connection terminated unexpectedly');
    assert.doesNotThrow(() => pool.emit('error', err, {}));
    assert.deepStrictEqual(seen, [err]);
});

test('without a hook the error is still absorbed', async (t) => {
    const { Pool } = require('pg');
    const originalOn = Pool.prototype.on;
    let pool;
    Pool.prototype.on = function on(...args) {
        pool = this;
        return originalOn.apply(this, args);
    };
    t.after(() => { Pool.prototype.on = originalOn; });

    const adapter = loadAdapter();
    t.after(() => adapter.close());
    assert.doesNotThrow(() => pool.emit('error', new Error('terminating connection due to administrator command')));
});

test('a pool emitting error with no listener would have thrown', () => {
    const { Pool } = require('pg');
    const bare = new Pool({ max: 1 });
    assert.throws(() => bare.emit('error', new Error('boom')), /boom/);
    return bare.end();
});

test('poolStats reports the configured maximum and the live counters', async (t) => {
    const adapter = loadAdapter();
    t.after(() => adapter.close());
    assert.deepStrictEqual(adapter.poolStats(), { max: 7, total: 0, idle: 0, waiting: 0 });
});
