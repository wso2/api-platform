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
 * The Moesif management API client with a test HTTP client: what is sent, and how Moesif's
 * refusals and network failures come back.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

// The client reads [api_portal.billing] through the config loader; give it a fixed one.
const CONFIG_PATH = path.resolve(__dirname, '../../../config/configLoader.js');
require.cache[CONFIG_PATH] = {
    id: CONFIG_PATH, filename: CONFIG_PATH, loaded: true,
    exports: { config: { billing: {
        moesif: { baseUrl: 'https://moesif.test', stripeWebhookUrl: 'https://moesif.test/v1/partners/stripe' },
    } } },
};

const moesif = require('./client');

/** An HTTP client that records each request and answers with the next of `responses`. */
function recordingHttp(responses) {
    const requests = [];
    return {
        requests,
        async request(req) {
            requests.push(req);
            const next = responses.shift();
            if (next instanceof Error) throw next;
            return next;
        },
    };
}

test('a Moesif refusal with no message field loses the management key and any Stripe key it repeats', async () => {
    const http = recordingHttp([{ status: 400, data: { partner: 'stripe', secret_key: 'sk_test_STRIPEKEY1234567', key: 'moesif-management-key' } }]);
    const client = moesif.createMoesifClient({ managementKey: 'moesif-management-key', http });

    await assert.rejects(client.savePartnerAccount('stripe', { secret_key: 'sk_test_STRIPEKEY1234567' }), (err) => {
        assert.strictEqual(err.message.includes('sk_test_STRIPEKEY1234567'), false);
        assert.strictEqual(err.message.includes('moesif-management-key'), false);
        assert.match(err.message, /^Moesif: .*"partner":"stripe"/);
        return true;
    });
});

test('a network failure is reported without detail from the transport', async () => {
    const timeout = Object.assign(new Error('timeout of 15000ms exceeded'), { code: 'ECONNABORTED' });
    const client = moesif.createMoesifClient({ managementKey: 'mk', http: recordingHttp([timeout]) });
    await assert.rejects(client.listCatalogPlans('stripe'), (err) => {
        assert.strictEqual(err.status, 0);
        assert.strictEqual(err.message, 'Moesif: the request timed out.');
        return true;
    });
});

test('Moesif is called on the token\'s organization with the management key', async () => {
    const http = recordingHttp([{ status: 200, data: [] }]);
    await moesif.createMoesifClient({ managementKey: 'mk', http }).listCatalogPlans('stripe');
    const [req] = http.requests;
    assert.strictEqual(req.url, 'https://moesif.test/v1/~/billing/catalog/plans');
    assert.deepStrictEqual(req.params, { provider: 'stripe' });
    assert.strictEqual(req.headers.Authorization, 'Bearer mk');
    assert.strictEqual(moesif.stripeWebhookUrl('app 1'), 'https://moesif.test/v1/partners/stripe?moesif_application_id=app%201');
});
