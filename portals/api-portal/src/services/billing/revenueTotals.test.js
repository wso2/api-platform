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
 * How Settings › Usage & Revenue adds up the invoices of GET /billing-reports/invoices
 * (src/scripts/revenue-totals.js, which the page loads).
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const totals = require('../../scripts/revenue-totals');

const calls = (quantity, amount) => ({ name: 'Successful calls · 200', amount, quantity, unit: 'request', countsRequests: true });
const tokens = (quantity, amount) => ({ name: 'LLM input tokens · gpt-4o', amount, quantity, unit: 'token', countsRequests: false });
const fee = (amount) => ({ name: 'Monthly fee', amount, quantity: null, unit: null, countsRequests: false });

const invoice = (fields) => ({ number: null, currency: 'USD', paid: true, subscriber: { name: null, email: null },
    subscriptionId: null, api: 'Weather API v1.0', plan: 'Gold', items: [], ...fields });

const INVOICES = [
    invoice({ number: 'INV-4', created: '2026-10-05T10:00:00.000Z', total: '28.12', subscriptionId: 'sub-a',
        subscriber: { name: 'Alice Example', email: 'alice@example.com' }, items: [calls(2800, '28.00'), calls(12, '0.12')] }),
    invoice({ number: 'INV-3', created: '2026-10-02T10:00:00.000Z', total: '59.00', subscriptionId: 'sub-b', plan: 'Silver',
        subscriber: { name: 'Bob Builder', email: 'bob@example.com' }, items: [fee('49.00'), tokens(1000000, '10.00')] }),
    invoice({ number: 'INV-2', created: '2026-09-20T10:00:00.000Z', total: '15.00', subscriptionId: 'sub-a',
        subscriber: { name: 'Alice Example', email: 'alice@example.com' }, items: [calls(1500, '15.00')] }),
    invoice({ number: 'INV-1', created: '2026-09-01T10:00:00.000Z', total: '5.00', api: null, plan: null }),
];

test('amounts are added as whole minor units', () => {
    assert.deepEqual(totals.minorOf('28.12'), { minor: 2812, digits: 2 });
    assert.deepEqual(totals.minorOf('-0.50'), { minor: -50, digits: 2 });
    assert.deepEqual(totals.minorOf('0.10'), { minor: 10, digits: 2 });
    assert.equal(totals.formatMoney(2812, 2, 'USD'), '$28.12');
    assert.equal(totals.formatMoney(-50, 2, 'EUR'), '-€0.50');
});

test('the months of a range run from the month of its start to the month of its end, across years', () => {
    assert.deepEqual(totals.monthsBetween(new Date('2025-11-15T00:00:00Z'), new Date('2026-02-01T00:00:00Z')),
        ['2025-11', '2025-12', '2026-01', '2026-02']);
    assert.deepEqual(totals.monthsBetween(new Date('2026-10-01T00:00:00Z'), new Date('2026-10-22T08:00:00Z')), ['2026-10']);
});

test('the report is in the currency the most subscriptions pay in, not the one with the larger amount', () => {
    const mixed = [
        invoice({ currency: 'INR', total: '10000.00', subscriptionId: 'sub-inr', created: '2026-09-10T00:00:00Z' }),
        invoice({ currency: 'USD', total: '9.00', subscriptionId: 'sub-1', created: '2026-09-10T00:00:00Z' }),
        invoice({ currency: 'USD', total: '9.00', subscriptionId: 'sub-2', created: '2026-09-10T00:00:00Z' }),
    ];
    assert.deepEqual(totals.currencies(mixed), ['USD', 'INR']);
    assert.equal(totals.summarize(mixed, 'USD', { '2026-09': true }).months['2026-09'], 1800);
});

test('a month adds up its invoices and the requests billed by request-count prices', () => {
    const october = totals.summarize(INVOICES, 'USD', { '2026-10': true });
    assert.deepEqual(october.months, { '2026-10': 8712, '2026-09': 2000 });
    assert.equal(october.invoiced, 8712);
    assert.equal(october.billableRequests, 2812);
});

test('a range of months adds up every month in it', () => {
    const both = totals.summarize(INVOICES, 'USD', { '2026-09': true, '2026-10': true });
    assert.equal(both.invoiced, 10712);
    assert.equal(both.billableRequests, 4312);
});

test('search matches the subscriber\'s name or email, the API or the plan, ignoring case', () => {
    const found = (text) => INVOICES.filter((inv) => totals.matches(inv, text)).map((inv) => inv.number);
    assert.deepEqual(found('alice'), ['INV-4', 'INV-2']);
    assert.deepEqual(found('BOB@EXAMPLE'), ['INV-3']);
    assert.deepEqual(found('silver'), ['INV-3']);
    assert.deepEqual(found('weather api'), ['INV-4', 'INV-3', 'INV-2']);
    assert.deepEqual(found('  '), ['INV-4', 'INV-3', 'INV-2', 'INV-1']);
});
