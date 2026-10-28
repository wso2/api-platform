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
 * The Stripe API client with a test HTTP client: key types, billing statuses, form encoding, what
 * is sent (headers, pinned version), how Stripe's refusals come back, and webhook signatures.
 */

const test = require('node:test');
const assert = require('node:assert');
const crypto = require('node:crypto');
const path = require('node:path');

// The client reads [api_portal.billing] through the config loader; give it a fixed one.
const CONFIG_PATH = path.resolve(__dirname, '../../../../../config/configLoader.js');
require.cache[CONFIG_PATH] = {
    id: CONFIG_PATH, filename: CONFIG_PATH, loaded: true,
    exports: { config: { billing: {
        stripe: { baseUrl: 'https://stripe.test', apiVersion: '2024-06-20' },
    } } },
};

const stripe = require('./client');
const { BillingProviderError } = require('../../../../http');

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

test('the key type comes from the prefix; publishable keys are not accepted', () => {
    assert.strictEqual(stripe.keyType('rk_test_abc123'), 'TEST');
    assert.strictEqual(stripe.keyType('sk_live_abc123'), 'LIVE');
    assert.strictEqual(stripe.keyType('pk_test_abc123'), null);
    assert.strictEqual(stripe.keyType('rk_test_'), null);
    assert.strictEqual(stripe.keyType(undefined), null);
});

// Access stops only when Stripe gives up on a renewal; a subscription whose first payment has not gone through
// is not served at all.
test('a Stripe subscription status gives its billing status; incomplete is UNPAID, past_due still PAST_DUE', () => {
    assert.deepStrictEqual(['active', 'trialing', 'past_due', 'incomplete', 'unpaid', 'paused', 'incomplete_expired', 'canceled', 'new']
        .map(stripe.billingStatus), ['ACTIVE', 'ACTIVE', 'PAST_DUE', 'UNPAID', 'UNPAID', 'UNPAID', 'UNPAID', 'CANCELLED', 'UNPAID']);
});

test('bodies are form-encoded with Stripe\'s bracket notation', () => {
    const encoded = stripe.formEncode({
        url: 'https://p.example/x?y=1', enabled_events: ['a.b', 'c.d'], metadata: { k: 'v' }, skip: null,
    }, '').join('&');
    assert.strictEqual(encoded,
        'url=https%3A%2F%2Fp.example%2Fx%3Fy%3D1&enabled_events%5B0%5D=a.b&enabled_events%5B1%5D=c.d&metadata%5Bk%5D=v');
});

test('a webhook endpoint is created with the pinned version, the key and an idempotency key', async () => {
    const http = recordingHttp([{ status: 200, data: { id: 'we_1', secret: 'whsec_x' } }]);
    const client = stripe.createStripeClient({ apiKey: 'rk_test_k', http });

    const created = await client.createWebhookEndpoint({ url: 'https://p.example/h', events: ['invoice.paid'] });

    assert.deepStrictEqual(created, { id: 'we_1', secret: 'whsec_x' });
    const [req] = http.requests;
    assert.strictEqual(req.method, 'POST');
    assert.strictEqual(req.url, 'https://stripe.test/v1/webhook_endpoints');
    assert.strictEqual(req.headers.Authorization, 'Bearer rk_test_k');
    assert.strictEqual(req.headers['Stripe-Version'], '2024-06-20');
    assert.match(req.headers['Idempotency-Key'], /^[0-9a-f-]{36}$/);
    assert.match(req.data, /api_version=2024-06-20/);
});

test('a Stripe refusal carries Stripe\'s message and status, never the key', async () => {
    const http = recordingHttp([{ status: 401, data: { error: { type: 'invalid_request_error', message: 'Invalid API Key provided: rk_test_****k' } } }]);
    const client = stripe.createStripeClient({ apiKey: 'rk_test_k', http });

    await assert.rejects(client.listWebhookEndpoints(), (err) => {
        assert.ok(err instanceof BillingProviderError);
        assert.strictEqual(err.status, 401);
        assert.strictEqual(err.isAuthError, true);
        assert.match(err.message, /^Stripe: Invalid API Key/);
        return true;
    });
});

// Provider messages are logged, and their wording can change: a whole key in one never reaches the error.
test('a Stripe refusal loses any whole key it repeats; Stripe\'s masked fragment stays', async () => {
    const appId = 'eyJhcHAiOiIxMjM6NDU2IiwidmVyIjoiMi4wIn0.c2ln';
    const http = recordingHttp([{ status: 400, data: { error: { message: `Invalid URL https://moesif.test/v1/partners/stripe?moesif_application_id=${appId} `
        + 'sent with rk_test_callkey123 (rk_test_****y123); also sk_live_OTHERKEY12345678 and whsec_SECRET12345678' } } }]);
    const client = stripe.createStripeClient({ apiKey: 'rk_test_callkey123', http });

    await assert.rejects(client.listWebhookEndpoints(), (err) => {
        for (const secret of [appId, 'rk_test_callkey123', 'sk_live_OTHERKEY12345678', 'whsec_SECRET12345678']) {
            assert.strictEqual(err.message.includes(secret), false, secret);
        }
        assert.match(err.message, /rk_test_\*\*\*\*y123/);
        assert.match(err.message, /^Stripe: Invalid URL/);
        return true;
    });
});

test('deleting an endpoint that is already gone succeeds', async () => {
    const http = recordingHttp([{ status: 404, data: { error: { message: 'No such webhook endpoint' } } }]);
    await stripe.createStripeClient({ apiKey: 'rk_test_k', http }).deleteWebhookEndpoint('we_gone');
});

test('a Stripe signature is accepted with any current secret and refused when old or altered', () => {
    const body = Buffer.from('{"id":"evt_1"}');
    const now = 1760000000;
    const sig = (secret, t) => crypto.createHmac('sha256', secret).update(`${t}.${body}`).digest('hex');
    const header = `t=${now},v1=${sig('whsec_old', now)},v1=${sig('whsec_new', now)}`;

    assert.strictEqual(stripe.verifyWebhookSignature('whsec_new', body, header, 300, now), true);
    assert.strictEqual(stripe.verifyWebhookSignature('whsec_old', body, header, 300, now + 299), true);
    assert.strictEqual(stripe.verifyWebhookSignature('whsec_new', body, header, 300, now + 301), false);
    assert.strictEqual(stripe.verifyWebhookSignature('whsec_other', body, header, 300, now), false);
    assert.strictEqual(stripe.verifyWebhookSignature('whsec_new', Buffer.from('{"id":"evt_2"}'), header, 300, now), false);
    assert.strictEqual(stripe.verifyWebhookSignature('whsec_new', body, 't=abc,v1=zz', 300, now), false);
    assert.strictEqual(stripe.verifyWebhookSignature('whsec_new', body, undefined, 300, now), false);
});
