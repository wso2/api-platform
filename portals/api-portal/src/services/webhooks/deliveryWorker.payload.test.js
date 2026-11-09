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
 * The signed delivery body carries secret_hash, so a receiver holding
 * several secrets can tell which one signed and encrypted it.
 */

const test = require('node:test');
const assert = require('node:assert');
const http = require('node:http');
const path = require('node:path');

const r = (p) => path.resolve(__dirname, p);
const workerConfig = {};
let subscriber = { id: 'sub-1' };
const signed = [];
const STUBBED = {
    [r('../../config/configLoader.js')]: { config: workerConfig },
    [r('../../db/driver.js')]: {},
    [r('../../dao/eventDao.js')]: {},
    [r('./subscriberRegistry.js')]: { getSubscriber: async () => subscriber },
    [r('./signer.js')]: {
        sign: (secret, body) => { signed.push({ secret, body }); return { header: 't=1,v1=sig' }; },
    },
    [r('../../config/logger.js')]: { info: () => {}, warn: () => {}, error: () => {}, debug: () => {} },
    [r('../../utils/orgContext.js')]: {},
};
const { DEFAULTS } = require('../../config/configDefaults');
const { secretHash, decryptField, SEALED_WITH_KEY } = require('./envelopeCrypto');
const { createCryptoUtil } = require('../../utils/cryptoUtil');

const AT_REST_KEY = 'ab'.repeat(32);
const SEALED_AT_REST_KEY = '__sealed_at_rest';
// What eventPublisher stores: the value under the portal key, bound to its field name.
const sealAtRest = (field, value) => createCryptoUtil(AT_REST_KEY).encrypt(value, `webhook-field:${field}`);
const WORKER = r('./deliveryWorker.js');

function loadWorker() {
    Object.assign(workerConfig, {
        httpClient: DEFAULTS.httpClient, webhooks: { delivery: {} }, security: { encryptionKey: AT_REST_KEY },
    });
    for (const [p, exports] of Object.entries(STUBBED)) {
        require.cache[p] = { id: p, filename: p, loaded: true, exports };
    }
    delete require.cache[WORKER];
    delete require.cache[r('./fieldAtRest.js')];
    try {
        return require(WORKER);
    } finally {
        for (const p of Object.keys(STUBBED)) delete require.cache[p];
        delete require.cache[r('./fieldAtRest.js')];
    }
}

// Posts one delivery to a local server and returns the request it received.
async function deliverOnce(encryptedFields = {}, expectOk = true) {
    const worker = loadWorker();
    let received;
    const server = http.createServer((req, res) => {
        const chunks = [];
        req.on('data', (c) => chunks.push(c));
        req.on('end', () => {
            received = { headers: req.headers, raw: Buffer.concat(chunks).toString('utf8') };
            res.writeHead(200).end();
        });
    });
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    try {
        const result = await worker.post(
            { uuid: 'd-1', subscriber_id: 'sub-1', target_url: `http://127.0.0.1:${server.address().port}/hook`, encrypted_fields: encryptedFields },
            { uuid: 'e-1', type: 'apikey.generated', occurred_at: '2026-01-01T00:00:00Z', org_uuid: 'org-1', payload: {} },
        );
        assert.strictEqual(result.ok, expectOk);
        if (!expectOk) return result;
    } finally {
        await new Promise((resolve) => server.close(resolve));
    }
    return received;
}

test('delivery to a subscriber with a secret carries the hash of that secret, inside the signed body', async () => {
    signed.length = 0;
    subscriber = { id: 'sub-1', secret: 'whsec_' + 'c'.repeat(64) };
    const received = await deliverOnce();
    const body = JSON.parse(received.raw);
    assert.strictEqual(body.secret_hash, secretHash(subscriber.secret));
    assert.ok(!received.raw.includes(subscriber.secret), 'the secret itself must never be sent');
    assert.strictEqual(signed.length, 1);
    assert.strictEqual(signed[0].body, received.raw, 'the hash must be part of the body that was signed');
});

test('delivery to a subscriber without a secret is unsigned and carries no hash', async () => {
    signed.length = 0;
    subscriber = { id: 'sub-1' };
    const received = await deliverOnce();
    assert.ok(!('secret_hash' in JSON.parse(received.raw)));
    assert.strictEqual(received.headers['x-api-portal-signature'], undefined);
    assert.strictEqual(signed.length, 0);
});

test('the record of which secret sealed the fields is not sent as a payload field', async () => {
    signed.length = 0;
    subscriber = { id: 'sub-1', secret: 'whsec_' + 'd'.repeat(64) };
    const received = await deliverOnce({ key: { iv: 'a', tag: 'b', ciphertext: 'c' }, [SEALED_WITH_KEY]: secretHash(subscriber.secret) });
    const body = JSON.parse(received.raw);
    assert.deepStrictEqual(body.encrypted_fields, ['key']);
    assert.ok(!(SEALED_WITH_KEY in body.data));
    assert.strictEqual(body.secret_hash, secretHash(subscriber.secret));
});

test('a delivery sealed under a secret the subscriber no longer has is failed, not signed with the new one', async () => {
    signed.length = 0;
    subscriber = { id: 'sub-1', secret: 'whsec_' + 'e'.repeat(64) };
    const result = await deliverOnce(
        { key: { iv: 'a', tag: 'b', ciphertext: 'c' }, [SEALED_WITH_KEY]: secretHash('whsec_' + 'f'.repeat(64)) },
        false,
    );
    assert.strictEqual(result.ok, false);
    assert.strictEqual(signed.length, 0, 'nothing may be signed with a secret that did not seal the fields');
});

test('fields sealed at rest are re-encrypted under the subscriber secret current at send time', async () => {
    signed.length = 0;
    // The secret was rotated after the event was published: only the portal key sealed the value.
    subscriber = { id: 'sub-1', secret: 'whsec_' + 'a'.repeat(64) };
    const received = await deliverOnce({ apiKey: sealAtRest('apiKey', 'key-value'), [SEALED_AT_REST_KEY]: 'v1' });
    const body = JSON.parse(received.raw);
    assert.deepStrictEqual(body.encrypted_fields, ['apiKey']);
    assert.ok(!(SEALED_AT_REST_KEY in body.data));
    assert.strictEqual(decryptField(subscriber.secret, body.data.apiKey), 'key-value');
    assert.strictEqual(body.secret_hash, secretHash(subscriber.secret));
    assert.strictEqual(signed.length, 1);
    assert.strictEqual(signed[0].secret, subscriber.secret);
    assert.ok(!received.raw.includes('key-value'), 'the plaintext must never be sent');
});

test('fields sealed at rest are failed, not sent unencrypted, when the subscriber has lost its secret', async () => {
    signed.length = 0;
    subscriber = { id: 'sub-1' };
    const result = await deliverOnce({ apiKey: sealAtRest('apiKey', 'key-value'), [SEALED_AT_REST_KEY]: 'v1' }, false);
    assert.strictEqual(result.ok, false);
    assert.strictEqual(signed.length, 0);
});

test('a stored value that cannot be opened fails the delivery', async () => {
    signed.length = 0;
    subscriber = { id: 'sub-1', secret: 'whsec_' + 'b'.repeat(64) };
    const result = await deliverOnce({ apiKey: sealAtRest('otherField', 'key-value'), [SEALED_AT_REST_KEY]: 'v1' }, false);
    assert.strictEqual(result.ok, false, 'a value bound to another field name must not open');
    assert.strictEqual(signed.length, 0);
});
