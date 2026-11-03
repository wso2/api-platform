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
 * A subscriber whose secret can't be decrypted gets a failed delivery rather than
 * none, so a key event whose only subscribers are unreadable ends FAILED instead of
 * reading as delivered to everyone, while the publishing request itself still succeeds.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const EVENT_DAO_PATH = path.resolve(__dirname, '../../dao/eventDao.js');
const REGISTRY_PATH = path.resolve(__dirname, './subscriberRegistry.js');
const CRYPTO_PATH = path.resolve(__dirname, './envelopeCrypto.js');
const LOGGER_PATH = path.resolve(__dirname, '../../config/logger.js');
const PUBLISHER_PATH = path.resolve(__dirname, './eventPublisher.js');

const UNREADABLE = 'Subscriber secret could not be decrypted';

function stub(modulePath, exports) {
    require.cache[modulePath] = { id: modulePath, filename: modulePath, loaded: true, exports };
}

function loadPublisher({ subscribers, unreadable }) {
    const calls = { failed: [], deliveries: [] };
    const event = { uuid: 'e1', status: 'PENDING', save: async () => event };
    stub(EVENT_DAO_PATH, {
        create: async () => event,
        recordUndeliverable: async (eventId, subs, reason) => calls.failed.push({ eventId, ids: subs.map((s) => s.id), reason }),
        createDeliveries: async (eventId, subs) => calls.deliveries.push({ eventId, ids: subs.map((s) => s.id) }),
    });
    stub(REGISTRY_PATH, { matchSubscribers: async () => ({ subscribers, unreadable }), UNREADABLE_SECRET: UNREADABLE });
    stub(CRYPTO_PATH, { encryptField: (secret, value) => ({ sealed: `${secret}:${value}` }) });
    stub(LOGGER_PATH, { info() {}, warn() {}, error() {} });
    delete require.cache[PUBLISHER_PATH];
    try {
        return { publisher: require(PUBLISHER_PATH), calls, event };
    } finally {
        for (const p of [EVENT_DAO_PATH, REGISTRY_PATH, CRYPTO_PATH, LOGGER_PATH, PUBLISHER_PATH]) delete require.cache[p];
    }
}

function publishKey(publisher) {
    return publisher.publish('apikey.generated', { keyId: 'k1' }, {
        transaction: {}, orgId: 'org-1', aggregateId: 'k1', secretFields: { apiKey: 'secret-value' },
    });
}

const readable = { id: 's1', url: 'https://a.example.com/hook', secret: 'secret-s1' };
const broken = { id: 's2', url: 'https://b.example.com/hook' };

test('a key event whose only subscriber is unreadable is recorded as a failed delivery', async () => {
    const { publisher, calls, event } = loadPublisher({ subscribers: [], unreadable: [broken] });
    assert.strictEqual(await publishKey(publisher), 'e1');
    assert.deepStrictEqual(calls.failed, [{ eventId: 'e1', ids: ['s2'], reason: UNREADABLE }]);
    assert.deepStrictEqual(calls.deliveries, []);
    assert.strictEqual(event.status, 'FAILED');
});

test('readable subscribers are still delivered alongside a failed unreadable one', async () => {
    const { publisher, calls, event } = loadPublisher({ subscribers: [readable], unreadable: [broken] });
    await publishKey(publisher);
    assert.deepStrictEqual(calls.failed, [{ eventId: 'e1', ids: ['s2'], reason: UNREADABLE }]);
    assert.deepStrictEqual(calls.deliveries, [{ eventId: 'e1', ids: ['s1'] }]);
    assert.strictEqual(event.status, 'DISPATCHED');
});

test('without unreadable subscribers no failed delivery is recorded', async () => {
    const { publisher, calls, event } = loadPublisher({ subscribers: [readable], unreadable: [] });
    await publishKey(publisher);
    assert.deepStrictEqual(calls.failed, [{ eventId: 'e1', ids: [], reason: UNREADABLE }]);
    assert.deepStrictEqual(calls.deliveries, [{ eventId: 'e1', ids: ['s1'] }]);
    assert.strictEqual(event.status, 'DISPATCHED');

    const none = loadPublisher({ subscribers: [], unreadable: [] });
    await publishKey(none.publisher);
    assert.deepStrictEqual(none.calls.deliveries, []);
    assert.strictEqual(none.event.status, 'ALL_DELIVERED');
});
