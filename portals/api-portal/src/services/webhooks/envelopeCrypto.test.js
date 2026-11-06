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
const crypto = require('node:crypto');
const { secretHash, SECRET_HASH_LABEL } = require('./envelopeCrypto');

test('secretHash is a labelled SHA3-256 hex digest of the secret', () => {
    const secret = 'whsec_' + 'a'.repeat(64);
    const expected = crypto.createHash('sha3-256').update(`${SECRET_HASH_LABEL}:${secret}`, 'utf8').digest('hex');
    assert.strictEqual(secretHash(secret), expected);
    assert.match(secretHash(secret), /^[0-9a-f]{64}$/);
});

test('secretHash is stable per secret, differs between secrets, and never contains the secret', () => {
    const a = 'whsec_' + 'a'.repeat(64);
    const b = 'whsec_' + 'b'.repeat(64);
    assert.strictEqual(secretHash(a), secretHash(a));
    assert.notStrictEqual(secretHash(a), secretHash(b));
    assert.ok(!secretHash(a).includes('aaaa'));
    // Not a bare digest of the secret: the label keeps it distinct.
    assert.notStrictEqual(secretHash(a), crypto.createHash('sha3-256').update(a).digest('hex'));
});

test('secretHash refuses an empty secret', () => {
    assert.throws(() => secretHash(''), /secret is required/);
    assert.throws(() => secretHash(undefined), /secret is required/);
});

// Shared with platform-api (internal/webhook/secret_hash_test.go). Change the algorithm or label
// on one side and its own copy of these vectors fails, so the receiver can never silently stop
// matching the portal's secret_hash.
test('secretHash matches the vectors platform-api verifies against', () => {
    const vectors = [
        ['whsec_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef', '32413deb36859f80321545a85378e517fad68ae583f7c3f3a0c6e50a1c96f824'],
        ['change-me-minimum-32-chars', 'bc6bb059212d79fd2689e6fe2adf8cc5cf24abe211e0ddf44de1f9c590e1e69f'],
        ['ünïcödé-secret-🔑-0123456789abcdef', '588a57d77853c7938c575e0135da9842fe4ee03010b825c9d6d6d12b56ef4c38'],
    ];
    for (const [secret, want] of vectors) {
        assert.strictEqual(secretHash(secret), want);
    }
});
