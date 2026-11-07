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

'use strict';

/*
 * SqlSessionStore record format: the payload is encrypted and bound to its portal and sid,
 * and the sid is stored hashed. The driver is replaced by a small in-memory table so the
 * SQL the store issues is what is exercised.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const stub = (file, exports) => {
    require.cache[file] = { id: file, filename: file, loaded: true, exports };
};
const local = (relative) => path.resolve(__dirname, relative);

const KEY = '7f40672c96fd437dc33550755e218d586014232ceb5b01c9405aab575bcb1f4a';
const OTHER_KEY = '527948cfb2478dbd4dfcd9d466df65d8c70511236fe63dba0da0f5385676f593';

let portalId = 'portal-a';
const table = new Map(); // `${portal_id}|${sid}` -> { portal_id, sid, sess, expire }
const warnings = [];

const fakeDb = {
    buildUpsert: () => 'UPSERT',
    async execute(sql, params) {
        if (sql === 'UPSERT') {
            const [portal, sid, sess, expire] = params;
            table.set(`${portal}|${sid}`, { portal_id: portal, sid, sess, expire });
        } else if (sql.startsWith('DELETE') && sql.includes('NOT LIKE')) {
            const [portal, pattern] = params;
            const prefix = pattern.replace('%', '');
            for (const [k, r] of table) if (r.portal_id === portal && !r.sid.startsWith(prefix)) table.delete(k);
        } else if (sql.startsWith('DELETE') && sql.includes('expire <')) {
            for (const [k, r] of table) if (new Date(r.expire) < params[0]) table.delete(k);
        } else if (sql.startsWith('DELETE')) {
            table.delete(`${params[0]}|${params[1]}`);
        } else if (sql.startsWith('UPDATE')) {
            const r = table.get(`${params[1]}|${params[2]}`);
            if (r) r.expire = params[0];
        }
    },
    async queryOne(sql, [portal, sid]) {
        return table.get(`${portal}|${sid}`) || null;
    },
};

stub(local('./driver.js'), fakeDb);
stub(local('../config/configLoader.js'), { config: { security: { encryptionKey: KEY } } });
stub(local('../config/logger.js'), {
    warn: (message, meta) => warnings.push({ message, meta }),
    error: () => {}, info: () => {}, debug: () => {},
});
stub(local('../utils/orgContext.js'), { getPortalId: () => portalId });

const { SqlSessionStore, storedSid } = require('./sqlSessionStore');
const { createCryptoUtil } = require('../utils/cryptoUtil');

const futureSession = () => ({
    cookie: { expires: new Date(Date.now() + 3600_000).toISOString() },
    passport: { user: { email: 'a@example.com', accessToken: 'ACCESS-TOKEN-VALUE', refreshToken: 'REFRESH-TOKEN-VALUE' } },
    csrfSecret: 'CSRF-SECRET-VALUE',
});

const call = (fn, ...args) => new Promise((resolve, reject) =>
    fn(...args, (err, result) => (err ? reject(err) : resolve(result))));

function newStore(options) {
    const store = new SqlSessionStore(options);
    store._pruneTimer.unref();
    return store;
}

test.beforeEach(() => { table.clear(); warnings.length = 0; portalId = 'portal-a'; });

test('round trip: what is set is what get returns', async () => {
    const store = newStore();
    const data = futureSession();
    await call(store.set.bind(store), 'sid-1', data);
    assert.deepEqual(await call(store.get.bind(store), 'sid-1'), data);
});

test('stored record is opaque: no session fields, no raw sid', async () => {
    const store = newStore();
    await call(store.set.bind(store), 'sid-1', futureSession());
    const [row] = [...table.values()];
    for (const value of ['ACCESS-TOKEN-VALUE', 'REFRESH-TOKEN-VALUE', 'CSRF-SECRET-VALUE', 'a@example.com', 'passport']) {
        assert.ok(!row.sess.includes(value), `sess must not contain ${value}`);
    }
    assert.ok(row.sid.startsWith('h1:') && !row.sid.includes('sid-1'));
    assert.equal(row.sid, storedSid('sid-1'));
    assert.match(JSON.parse(row.sess).enc, /^gcm:/);
});

test('every write uses a fresh IV', async () => {
    const store = newStore();
    await call(store.set.bind(store), 'sid-1', futureSession());
    const first = [...table.values()][0].sess;
    await call(store.set.bind(store), 'sid-1', futureSession());
    assert.notEqual([...table.values()][0].sess, first);
});

test('a modified payload is no session, not an error', async () => {
    const store = newStore();
    await call(store.set.bind(store), 'sid-1', futureSession());
    const row = [...table.values()][0];
    const payload = JSON.parse(row.sess);
    payload.enc = payload.enc.slice(0, -4) + 'AAAA';
    row.sess = JSON.stringify(payload);
    assert.equal(await call(store.get.bind(store), 'sid-1'), null);
    assert.equal(warnings.length, 1);
});

test('a row copied to another sid does not decrypt', async () => {
    const store = newStore();
    await call(store.set.bind(store), 'sid-1', futureSession());
    const row = [...table.values()][0];
    table.set(`portal-a|${storedSid('sid-2')}`, { ...row, sid: storedSid('sid-2') });
    assert.equal(await call(store.get.bind(store), 'sid-2'), null);
});

test('a row copied to another portal does not decrypt', async () => {
    const store = newStore();
    await call(store.set.bind(store), 'sid-1', futureSession());
    const row = [...table.values()][0];
    table.set(`portal-b|${row.sid}`, { ...row, portal_id: 'portal-b' });
    portalId = 'portal-b';
    assert.equal(await call(store.get.bind(store), 'sid-1'), null);
});

test('a session written under another key is no session', async () => {
    const writer = newStore({ crypto: createCryptoUtil(OTHER_KEY) });
    await call(writer.set.bind(writer), 'sid-1', futureSession());
    const reader = newStore();
    assert.equal(await call(reader.get.bind(reader), 'sid-1'), null);
});

test('rows in the earlier format are never served and are removed at startup', async () => {
    const earlier = futureSession();
    table.set('portal-a|sid-1', {
        portal_id: 'portal-a', sid: 'sid-1', sess: JSON.stringify(earlier), expire: earlier.cookie.expires,
    });
    table.set('portal-b|sid-9', {
        portal_id: 'portal-b', sid: 'sid-9', sess: JSON.stringify(earlier), expire: earlier.cookie.expires,
    });
    const store = newStore();
    assert.equal(await call(store.get.bind(store), 'sid-1'), null); // not served, even before cleanup lands
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(table.has('portal-a|sid-1'), false);
    assert.equal(table.has('portal-b|sid-9'), true, 'other portals are untouched');
});

test('new-format rows survive the startup cleanup (rolling deploy with replicas)', async () => {
    const first = newStore();
    await call(first.set.bind(first), 'sid-1', futureSession());
    newStore(); // a second replica starting up
    await new Promise((resolve) => setImmediate(resolve));
    assert.deepEqual(await call(first.get.bind(first), 'sid-1'), futureSession());
});

test('destroy and touch address the hashed sid; expired rows are not served', async () => {
    const store = newStore();
    await call(store.set.bind(store), 'sid-1', futureSession());
    const later = new Date(Date.now() + 7200_000);
    await call(store.touch.bind(store), 'sid-1', { cookie: { expires: later.toISOString() } });
    assert.equal(new Date([...table.values()][0].expire).getTime(), later.getTime());
    await call(store.destroy.bind(store), 'sid-1');
    assert.equal(table.size, 0);

    const expired = { ...futureSession(), cookie: { expires: new Date(Date.now() - 1000).toISOString() } };
    await call(store.set.bind(store), 'sid-2', expired);
    assert.equal(await call(store.get.bind(store), 'sid-2'), null);
    assert.equal(table.size, 0);
});

test('refuses to start without a valid encryption key', () => {
    assert.throws(() => new SqlSessionStore({ crypto: createCryptoUtil('') }), /encryption_key/);
});
