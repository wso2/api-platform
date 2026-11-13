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
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const CONFIG_PATH = path.resolve(__dirname, '../config/configLoader.js');
const DB_SSL_PATH = path.resolve(__dirname, './dbSsl.js');

// Temporary CA cert file used by verify-ca / verify-full tests.
const FAKE_CA = '-----BEGIN CERTIFICATE-----\nFAKE\n-----END CERTIFICATE-----';
const TEMP_CERT = path.join(os.tmpdir(), `dbSsl-test-ca-${process.pid}.pem`);
fs.writeFileSync(TEMP_CERT, FAKE_CA);

function buildSsl(sslMode, sslRootCert) {
    const stub = (p, exports) => { require.cache[p] = { id: p, filename: p, loaded: true, exports }; };
    stub(CONFIG_PATH, { config: { database: { sslMode, sslRootCert } } });
    delete require.cache[DB_SSL_PATH];
    try {
        return require(DB_SSL_PATH).buildDbSsl();
    } finally {
        delete require.cache[CONFIG_PATH];
        delete require.cache[DB_SSL_PATH];
    }
}

test('disable returns undefined', () => {
    assert.strictEqual(buildSsl('disable', ''), undefined);
});

test('require returns { rejectUnauthorized: false } with no CA', () => {
    assert.deepStrictEqual(buildSsl('require', ''), { rejectUnauthorized: false });
});

test('verify-ca with ssl_root_cert returns rejectUnauthorized: true, ca content, and a checkServerIdentity no-op', () => {
    const ssl = buildSsl('verify-ca', TEMP_CERT);
    assert.strictEqual(ssl.rejectUnauthorized, true);
    assert.strictEqual(ssl.ca, FAKE_CA);
    assert.strictEqual(typeof ssl.checkServerIdentity, 'function');
    assert.strictEqual(ssl.checkServerIdentity('host', {}), undefined);
});

test('verify-ca without ssl_root_cert throws', () => {
    assert.throws(
        () => buildSsl('verify-ca', ''),
        /verify-ca.*ssl_root_cert/
    );
});

test('verify-full with ssl_root_cert returns rejectUnauthorized: true and ca content, with no checkServerIdentity override', () => {
    const ssl = buildSsl('verify-full', TEMP_CERT);
    assert.strictEqual(ssl.rejectUnauthorized, true);
    assert.strictEqual(ssl.ca, FAKE_CA);
    assert.strictEqual(ssl.checkServerIdentity, undefined);
});

test('verify-full without ssl_root_cert returns rejectUnauthorized: true with no ca', () => {
    const ssl = buildSsl('verify-full', '');
    assert.strictEqual(ssl.rejectUnauthorized, true);
    assert.strictEqual(ssl.ca, undefined);
    assert.strictEqual(ssl.checkServerIdentity, undefined);
});

test('an unsupported mode throws', () => {
    assert.throws(
        () => buildSsl('trust', ''),
        /Unsupported database ssl_mode/
    );
});
