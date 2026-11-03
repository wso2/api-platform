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

// The meters come from the database; the cards only need their names and what they count.
const meterDaoPath = require.resolve('../../dao/meterDao');
require.cache[meterDaoPath] = {
    id: meterDaoPath, filename: meterDaoPath, loaded: true,
    exports: { list: async () => [
        { handle: 'calls', display_name: 'API Requests', type: 'REQUEST_COUNT' },
        { handle: 'tokens', display_name: 'Tokens', type: 'TOKENS' },
    ] },
};
const { decoratePlans } = require('./planPresentation');

const usagePlan = (prices, billingPeriod = 'MONTH') => ({
    id: 'p', paid: true, pricing: { pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod, prices },
});

test('a tiered price is a table under Pricing details, headed by its least price per request', async () => {
    const plan = usagePlan([{ meterId: 'calls', pricingModel: 'VOLUME', tiers: [
        { upTo: 100, unitAmount: '0.02', flatAmount: '5' },
        { upTo: 200, unitAmount: '0.01' },
        { upTo: null, unitAmount: '0.005', flatAmount: '1' },
    ] }]);
    await decoratePlans('org', [plan]);
    assert.deepStrictEqual([plan.pricePrefix, plan.priceAmount, plan.priceUnit], ['From', '$0.005', '/ request']);
    assert.strictEqual(plan.billingNote, 'Billed monthly');
    const [item] = plan.priceItems;
    // One price: its meter's name tells a subscriber nothing the unit doesn't.
    assert.strictEqual(item.name, null);
    assert.strictEqual(item.text, 'Volume pricing');
    assert.deepStrictEqual(item.tiers.headers, ['Requests', 'Per request', 'Flat fee']);
    assert.deepStrictEqual(item.tiers.rows.map((r) => r.cells), [
        ['1–100', '$0.02', '$5.00'],
        ['101–200', '$0.01', '–'],
        ['201+', '$0.005', '$1.00'],
    ]);
    // The subscribe dialog lists each tier as one line.
    assert.deepStrictEqual(item.tiers.rows.map((r) => `${r.label}: ${r.amounts}`), [
        '1–100 requests: $0.02 per request + $5.00 flat',
        '101–200 requests: $0.01 per request',
        '201+ requests: $0.005 per request + $1.00 flat',
    ]);
    assert.strictEqual(item.hint, 'All requests in the period are charged at the tier their total reaches.');
});

test('graduated tiers without flat fees have no Flat fee column', async () => {
    const plan = usagePlan([{ meterId: 'calls', pricingModel: 'GRADUATED', tiers: [
        { upTo: 1000, unitAmount: '0.01' },
        { upTo: null, unitAmount: '0.008' },
    ] }], 'YEAR');
    await decoratePlans('org', [plan]);
    assert.strictEqual(plan.billingNote, 'Billed yearly');
    assert.deepStrictEqual([plan.pricePrefix, plan.priceAmount, plan.priceUnit], ['From', '$0.008', '/ request']);
    const [item] = plan.priceItems;
    assert.deepStrictEqual(item.tiers.headers, ['Requests', 'Per request']);
    assert.deepStrictEqual(item.tiers.rows.map((r) => r.cells), [['1–1,000', '$0.01'], ['1,001+', '$0.008']]);
    assert.strictEqual(item.hint, "Requests in each tier are charged at that tier's price.");
});

test('a flat rate plan names its period in the price and has no billing note', async () => {
    const plan = { id: 'f', paid: true, pricing: { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH',
        flatAmount: '12', prices: [] } };
    await decoratePlans('org', [plan]);
    assert.strictEqual(plan.priceAmount, '$12.00');
    assert.strictEqual(plan.priceUnit, '/ month');
    assert.strictEqual(plan.billingNote, null);
    assert.deepStrictEqual(plan.priceItems, []);
});

test('one per-unit price is the headline, with nothing under Pricing details', async () => {
    const plan = usagePlan([{ meterId: 'calls', pricingModel: 'PER_UNIT', unitAmount: '0.01' }]);
    await decoratePlans('org', [plan]);
    assert.strictEqual(plan.priceAmount, '$0.01');
    assert.strictEqual(plan.priceUnit, '/ request');
    assert.strictEqual(plan.billingNote, 'Billed monthly');
    assert.deepStrictEqual(plan.priceItems, []);
});

test('several prices are named; the headline is the least per unit, or Pay per use across units', async () => {
    const sameUnit = usagePlan([
        { meterId: 'calls', pricingModel: 'PER_UNIT', unitAmount: '0.03',
            filters: [{ field: 'HTTP method', operator: 'EQUALS', values: ['GET'] }] },
        { meterId: 'calls', pricingModel: 'GRADUATED', tiers: [{ upTo: 100, unitAmount: '0' }, { upTo: null, unitAmount: '0.02' }],
            filters: [{ field: 'HTTP method', operator: 'EQUALS', values: ['POST'] }] },
    ]);
    const mixedUnits = usagePlan([
        { meterId: 'calls', pricingModel: 'PER_UNIT', unitAmount: '0.01' },
        { meterId: 'tokens', pricingModel: 'PER_UNIT', unitAmount: '0.00002' },
    ]);
    await decoratePlans('org', [sameUnit, mixedUnits]);
    // A free first tier is not the least price: the least that is charged is.
    assert.deepStrictEqual([sameUnit.pricePrefix, sameUnit.priceAmount, sameUnit.priceUnit], ['From', '$0.02', '/ request']);
    assert.deepStrictEqual(sameUnit.priceItems.map((i) => i.name), ['API Requests · GET', 'API Requests · POST']);
    assert.deepStrictEqual([mixedUnits.pricePrefix, mixedUnits.priceAmount, mixedUnits.priceUnit], [null, 'Pay per use', '']);
    assert.strictEqual(mixedUnits.priceItems.length, 2);
});
