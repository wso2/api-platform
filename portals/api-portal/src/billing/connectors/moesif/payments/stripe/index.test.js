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
 * The Stripe sub-connector's payment functions, calling Stripe through a test HTTP client:
 * checkouts, cancelling and refunding a subscription, invoice pages and the estimate.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

// The Stripe client reads [api_portal.billing] through the config loader; give it a fixed one.
const CONFIG_PATH = path.resolve(__dirname, '../../../../../config/configLoader.js');
require.cache[CONFIG_PATH] = {
    id: CONFIG_PATH, filename: CONFIG_PATH, loaded: true,
    exports: { config: { billing: {
        stripe: { baseUrl: 'https://stripe.test', apiVersion: '2024-06-20' },
    } } },
};

const stripe = require('./client');
const stripeConnector = require('./index');
const { BillingProviderError, BillingCursorError } = require('../../../../http');

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

/** The Stripe sub-connector's payment functions for a connection, calling Stripe through `http`. */
function stripePayments(http) {
    stripeConnector.clients.stripe = (apiKey) => stripe.createStripeClient({ apiKey, http });
    return stripeConnector.create({ stripeApiKey: 'rk_test_k' });
}

test('an open checkout carries its client secret; expiring it ends it, and one paid meanwhile is read again', async () => {
    const session = { id: 'cs_1', client_reference_id: 'chk-1', status: 'open', payment_status: 'unpaid', subscription: null,
        client_secret: 'cs_1_secret_x' };
    let http = recordingHttp([{ status: 200, data: session }, { status: 200, data: { ...session, status: 'expired' } }]);
    let payments = stripePayments(http);
    assert.deepStrictEqual(await payments.getCheckout('cs_1'),
        { ref: 'cs_1', reference: 'chk-1', state: 'OPEN', subscriptionRef: null, clientSecret: 'cs_1_secret_x' });
    assert.deepStrictEqual(await payments.expireCheckout('cs_1'),
        { ref: 'cs_1', reference: 'chk-1', state: 'EXPIRED', subscriptionRef: null, clientSecret: null });
    assert.deepStrictEqual(http.requests.map((r) => [r.method, r.url]), [
        ['GET', 'https://stripe.test/v1/checkout/sessions/cs_1'],
        ['POST', 'https://stripe.test/v1/checkout/sessions/cs_1/expire'],
    ]);

    // Stripe expires open sessions only: refused, the session is read again and found paid.
    http = recordingHttp([
        { status: 400, data: { error: { type: 'invalid_request_error',
            message: 'Only Checkout Sessions with a status in ["open"] can be expired.' } } },
        { status: 200, data: { ...session, status: 'complete', payment_status: 'paid', subscription: 'sub_1' } },
    ]);
    assert.deepStrictEqual(await stripePayments(http).expireCheckout('cs_1'),
        { ref: 'cs_1', reference: 'chk-1', state: 'PAID', subscriptionRef: 'sub_1', clientSecret: null });
    assert.deepStrictEqual(http.requests.map((r) => [r.method, r.url]), [
        ['POST', 'https://stripe.test/v1/checkout/sessions/cs_1/expire'],
        ['GET', 'https://stripe.test/v1/checkout/sessions/cs_1'],
    ]);

    payments = stripePayments(recordingHttp([{ status: 503, data: { error: { message: 'Service Unavailable' } } }]));
    await assert.rejects(payments.expireCheckout('cs_1'), (err) => err instanceof BillingProviderError && err.status === 503);
});

test('a second subscription is cancelled immediately, without a final invoice, and its paid invoices refunded', async () => {
    const http = recordingHttp([
        { status: 200, data: { id: 'sub_2', status: 'active' } },
        { status: 200, data: { id: 'sub_2', status: 'canceled' } },
        { status: 200, data: { data: [
            { id: 'in_3', amount_paid: 1900, payment_intent: 'pi_3' },
            { id: 'in_2', amount_paid: 0, payment_intent: null },
            { id: 'in_1', amount_paid: 500, payment_intent: { id: 'pi_1' } },
        ] } },
        { status: 200, data: { id: 're_1' } },
        { status: 200, data: { id: 're_2' } },
    ]);

    assert.strictEqual(await stripePayments(http).cancelAndRefund('sub_2'), 2);

    assert.deepStrictEqual(http.requests.map((r) => [r.method, r.url]), [
        ['GET', 'https://stripe.test/v1/subscriptions/sub_2'],
        ['DELETE', 'https://stripe.test/v1/subscriptions/sub_2'],
        ['GET', 'https://stripe.test/v1/invoices?subscription=sub_2&limit=10'],
        ['POST', 'https://stripe.test/v1/refunds'],
        ['POST', 'https://stripe.test/v1/refunds'],
    ]);
    assert.deepStrictEqual(http.requests.slice(3).map((r) => r.data), ['payment_intent=pi_3', 'payment_intent=pi_1']);
    assert.match(http.requests[3].headers['Idempotency-Key'], /^[0-9a-f-]{36}$/);
});

test('a subscription that has already ended is not cancelled again; its paid invoices are still refunded', async () => {
    const http = recordingHttp([
        { status: 200, data: { id: 'sub_3', status: 'canceled' } },
        { status: 200, data: { data: [{ id: 'in_4', amount_paid: 900, payment_intent: 'pi_4' }] } },
        { status: 200, data: { id: 're_3' } },
    ]);
    assert.strictEqual(await stripePayments(http).cancelAndRefund('sub_3'), 1);
    assert.deepStrictEqual(http.requests.map((r) => r.method), ['GET', 'GET', 'POST']);
});

test('a payment refunded by an earlier attempt is passed over; any other refusal of a refund fails the call', async () => {
    const alreadyRefunded = { status: 400, data: { error: { code: 'charge_already_refunded', message: 'Charge ch_5 has already been refunded.' } } };
    const invoices = { status: 200, data: { data: [
        { id: 'in_6', amount_paid: 900, payment_intent: 'pi_6' },
        { id: 'in_5', amount_paid: 900, payment_intent: 'pi_5' },
    ] } };
    const ended = { status: 200, data: { id: 'sub_4', status: 'canceled' } };

    let http = recordingHttp([ended, invoices, { status: 200, data: { id: 're_6' } }, alreadyRefunded]);
    assert.strictEqual(await stripePayments(http).cancelAndRefund('sub_4'), 1);
    assert.deepStrictEqual(http.requests.slice(2).map((r) => r.data), ['payment_intent=pi_6', 'payment_intent=pi_5']);

    http = recordingHttp([ended, invoices, { status: 402, data: { error: { code: 'card_declined', message: 'Declined.' } } }]);
    await assert.rejects(stripePayments(http).cancelAndRefund('sub_4'),
        (err) => err instanceof BillingProviderError && err.status === 402 && err.code === 'card_declined');
});

test('an invoice page after an invoice Stripe does not know is a cursor error; other refusals are not', async () => {
    const since = new Date(0);
    const missing = (param, message) => ({ status: 400, data: { error: { type: 'invalid_request_error', code: 'resource_missing',
        param, message } } });

    const http = recordingHttp([missing('starting_after', "No such invoice: 'in_x'")]);
    await assert.rejects(stripePayments(http).customerInvoicePage('cus_1', { since, limit: 10, startingAfter: 'in_x' }), (err) => {
        assert.ok(err instanceof BillingCursorError);
        assert.ok(!(err instanceof BillingProviderError));
        return true;
    });
    assert.match(http.requests[0].url, /^https:\/\/stripe\.test\/v1\/invoices\?/);
    assert.match(http.requests[0].url, /starting_after=in_x/);
    assert.match(http.requests[0].url, /customer=cus_1/);

    // Without a cursor, a missing resource (the customer) is Stripe refusing the call.
    await assert.rejects(stripePayments(recordingHttp([missing('customer', "No such customer: 'cus_1'")]))
        .customerInvoicePage('cus_1', { since, limit: 10 }), (err) => {
        assert.ok(err instanceof BillingProviderError);
        assert.strictEqual(err.code, 'resource_missing');
        return true;
    });
    // With a cursor, Stripe being unavailable is still Stripe being unavailable.
    await assert.rejects(stripePayments(recordingHttp([{ status: 503, data: { error: { message: 'Service Unavailable' } } }]))
        .customerInvoicePage('cus_1', { since, limit: 10, startingAfter: 'in_x' }),
    (err) => err instanceof BillingProviderError && err.status === 503);
});

test('an invoice page reaches up to its end: the second it ends in is included, a whole second is not', async () => {
    const since = new Date(1000500);
    const page = { status: 200, data: { data: [], has_more: false } };
    const http = recordingHttp([page, { ...page }]);
    const payments = stripePayments(http);
    await payments.invoicePage({ since, until: new Date(2000500), customerRefs: new Set(), limit: 10 });
    await payments.invoicePage({ since, until: new Date(2000000), customerRefs: new Set(), limit: 10 });
    assert.match(http.requests[0].url, /created%5Bgte%5D=1000(&|$)/);
    assert.match(http.requests[0].url, /created%5Blt%5D=2001(&|$)/);
    assert.match(http.requests[1].url, /created%5Blt%5D=2000(&|$)/);
});

test('a checkout is embedded in the portal\'s page, in the plan\'s own currency (Adaptive Pricing off)', async () => {
    const http = recordingHttp([{ status: 200, data: { id: 'cs_2', status: 'open', payment_status: 'unpaid', client_secret: 'cs_2_secret_x' } }]);
    const started = await stripePayments(http).startCheckout({ customerRef: 'cus_1', prices: [{ ref: 'price_1', fixed: true }],
        reference: 'chk-2', returnUrl: 'https://portal.test/api/weather?checkout=chk-2&checkoutResult=success' });
    assert.deepStrictEqual(started, { ref: 'cs_2', clientSecret: 'cs_2_secret_x' });
    const [req] = http.requests;
    assert.strictEqual(req.url, 'https://stripe.test/v1/checkout/sessions');
    const sent = new URLSearchParams(String(req.data));
    assert.strictEqual(sent.get('ui_mode'), 'embedded');
    assert.strictEqual(sent.get('return_url'), 'https://portal.test/api/weather?checkout=chk-2&checkoutResult=success');
    assert.strictEqual(sent.get('redirect_on_completion'), 'if_required');
    assert.strictEqual(sent.get('success_url'), null);
    assert.strictEqual(sent.get('cancel_url'), null);
    assert.strictEqual(sent.get('adaptive_pricing[enabled]'), 'false');
});

test('the estimate reads every line of the preview, not only the 10 Stripe embeds', async () => {
    const line = (n) => ({ id: `il_tmp_${n}`, price: { id: `price_${n}` }, quantity: n, amount: n * 10, description: `line ${n}` });
    const all = Array.from({ length: 21 }, (_, i) => line(i + 1));
    const http = recordingHttp([
        { status: 200, data: { id: 'upcoming_in_1', currency: 'usd', total: 2310, period_start: 1760000000, period_end: 1762600000,
            lines: { data: all.slice(0, 10), has_more: true } } },
        { status: 200, data: { data: all.slice(10), has_more: false } },
    ]);
    const estimate = await stripePayments(http).estimate('sub_1');
    assert.deepStrictEqual(estimate.lines.map((l) => l.priceRef), all.map((l) => l.price.id));
    assert.equal(http.requests.length, 2);
    assert.equal(http.requests[1].method, 'GET');
    assert.match(http.requests[1].url, /^https:\/\/stripe\.test\/v1\/invoices\/upcoming_in_1\/lines\?/);
    assert.match(http.requests[1].url, /starting_after=il_tmp_10/);
});

test('an estimate whose lines all fit in the preview reads no further page', async () => {
    const http = recordingHttp([{ status: 200, data: { id: 'upcoming_in_2', currency: 'usd', total: 10,
        lines: { data: [{ id: 'il_tmp_1', price: { id: 'price_1' }, quantity: 1, amount: 10 }], has_more: false } } }]);
    const estimate = await stripePayments(http).estimate('sub_2');
    assert.equal(estimate.lines.length, 1);
    assert.equal(http.requests.length, 1);
});
