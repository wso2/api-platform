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
 * matchSubscribers runs inside the publishing request's transaction, so a subscriber
 * whose secret can't be decrypted must be set aside, not thrown — otherwise one broken
 * subscriber fails the user's own request (e.g. API key generation). It is returned as
 * unreadable so the caller can record its delivery as failed.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const DAO_PATH = path.resolve(__dirname, '../../dao/webhookSubscriberDao.js');
const LOGGER_PATH = path.resolve(__dirname, '../../config/logger.js');
const REGISTRY_PATH = path.resolve(__dirname, './subscriberRegistry.js');

function stub(modulePath, exports) {
    require.cache[modulePath] = { id: modulePath, filename: modulePath, loaded: true, exports };
}

function loadRegistry(records, undecryptable) {
    const errors = [];
    stub(DAO_PATH, {
        matchSubscribers: async () => records,
        decryptSecret: (record) => {
            if (undecryptable.includes(record.uuid)) throw new Error('Unsupported state or unable to authenticate data');
            return `secret-${record.uuid}`;
        },
    });
    stub(LOGGER_PATH, { error: (msg, meta) => errors.push({ msg, meta }) });
    delete require.cache[REGISTRY_PATH];
    try {
        return { registry: require(REGISTRY_PATH), errors };
    } finally {
        delete require.cache[DAO_PATH];
        delete require.cache[LOGGER_PATH];
        delete require.cache[REGISTRY_PATH];
    }
}

const records = [
    { uuid: 's1', target_url: 'https://a.example.com/hook', event_patterns: ['apikey.*'], timeout_ms: 5000 },
    { uuid: 's2', target_url: 'https://b.example.com/hook', event_patterns: ['apikey.*'], timeout_ms: 5000 },
];

test('a subscriber whose secret cannot be decrypted is returned as unreadable and logged', async () => {
    const { registry, errors } = loadRegistry(records, ['s1']);
    const { subscribers, unreadable } = await registry.matchSubscribers('org-1', 'apikey.generated');
    assert.deepStrictEqual(subscribers.map((s) => s.id), ['s2']);
    assert.strictEqual(subscribers[0].secret, 'secret-s2');
    assert.deepStrictEqual(unreadable, [{ id: 's1', url: 'https://a.example.com/hook' }]);
    assert.strictEqual(errors.length, 1);
    assert.strictEqual(errors[0].meta.subscriberId, 's1');
});

test('decryptable subscribers are all returned and none is unreadable', async () => {
    const { registry, errors } = loadRegistry(records, []);
    const { subscribers, unreadable } = await registry.matchSubscribers('org-1', 'apikey.generated');
    assert.deepStrictEqual(subscribers.map((s) => s.id), ['s1', 's2']);
    assert.deepStrictEqual(unreadable, []);
    assert.strictEqual(errors.length, 0);
});
