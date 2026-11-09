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
 * The billing HTTP client's address policy: private addresses are refused unless
 * [api_portal.billing.client] allow_private_endpoints is true.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const config = { billing: {} };
const CONFIG_PATH = path.resolve(__dirname, '../config/configLoader.js');
require.cache[CONFIG_PATH] = { id: CONFIG_PATH, filename: CONFIG_PATH, loaded: true, exports: { config } };

const policies = [];
const CLIENT_PATH = path.resolve(__dirname, '../keymanagers/core/httpClient.js');
require.cache[CLIENT_PATH] = {
    id: CLIENT_PATH, filename: CLIENT_PATH, loaded: true,
    exports: { buildClient: (policy) => { policies.push(policy); return {}; } },
};

const { createHttpClient } = require('./http');

function policyFor(client) {
    config.billing = client === undefined ? {} : { client };
    policies.length = 0;
    createHttpClient();
    return policies[0];
}

test('private addresses are refused by default', () => {
    assert.strictEqual(policyFor(undefined).allowPrivateEndpoints, false);
    assert.strictEqual(policyFor({ timeoutMs: 1000 }).allowPrivateEndpoints, false);
});

test('private addresses are allowed only when the setting is true', () => {
    assert.strictEqual(policyFor({ allowPrivateEndpoints: true }).allowPrivateEndpoints, true);
    assert.strictEqual(policyFor({ allowPrivateEndpoints: 'true' }).allowPrivateEndpoints, false);
    assert.strictEqual(policyFor({ allowPrivateEndpoints: false }).allowPrivateEndpoints, false);
});

test('the client limits are passed on', () => {
    const client = { timeoutMs: 1000, maxRequestBytes: 10, maxResponseBytes: 20 };
    assert.deepStrictEqual(policyFor(client).limits, client);
});
