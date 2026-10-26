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
 * The Moesif catalog payloads: the shapes sent to Moesif's management API, and the search a
 * billing meter runs on API call events.
 */

const test = require('node:test');
const assert = require('node:assert');
const catalog = require('./catalog');
const stripeConnector = require('./payments/stripe');

const monthly = { currency: 'USD', billing_period: 'MONTH' };
const price = (extra) => ({ display_name: 'Calls', ...extra });

test('a catalog plan is a continuously reported Stripe plan with the name it is given', () => {
    assert.deepStrictEqual(catalog.catalogPlanPayload('stripe', 'Weather API v1.0 - Gold'), {
        name: 'Weather API v1.0 - Gold',
        provider: 'stripe',
        status: 'active',
        billing_type: 'continuous_reporting',
        reporting_period: '15m',
    });
});

test('a flat fee is charged every period, in lower-case currency', () => {
    assert.deepStrictEqual(catalog.flatPricePayload('stripe', 'plan_1', 'Weather API v1.0 - Gold',
        { ...monthly, billing_period: 'YEAR', flat_amount: '99.00' }), {
        name: 'Weather API v1.0 - Gold',
        plan_id: 'plan_1',
        status: 'active',
        provider: 'stripe',
        currency: 'usd',
        period: 1,
        period_units: 'y',
        pricing_model: 'flat',
        usage_aggregator: 'sum',
        price_in_decimal: '99.00',
    });
});

test('a per-unit price sums usage and divides by the unit quantity', () => {
    const payload = catalog.usagePricePayload('stripe', 'plan_1', 'Calls', monthly,
        price({ pricing_model: 'PER_UNIT', unit_amount: '0.50', unit_quantity: 1000 }));
    assert.strictEqual(payload.name, 'Calls');
    assert.strictEqual(payload.usage_aggregator, 'sum');
    assert.strictEqual(payload.pricing_model, 'per_unit');
    assert.strictEqual(payload.price_in_decimal, '0.50');
    assert.strictEqual(payload.period_units, 'M');
    assert.deepStrictEqual(payload.transform_quantity, { divide_by: 1000, round: 'up' });

    const perCall = catalog.usagePricePayload('stripe', 'plan_1', 'Calls', monthly,
        price({ pricing_model: 'PER_UNIT', unit_amount: '0.01', unit_quantity: 1 }));
    assert.strictEqual(perCall.transform_quantity, undefined);
});

test('graduated prices are Moesif "tiered" prices with an open last tier', () => {
    const payload = catalog.usagePricePayload('stripe', 'plan_1', 'Calls', monthly, price({
        pricing_model: 'GRADUATED',
        tiers: [{ upTo: 1000, unitAmount: '0', flatAmount: null }, { upTo: null, unitAmount: '0.002', flatAmount: '5' }],
    }));
    assert.strictEqual(payload.pricing_model, 'tiered');
    assert.strictEqual(payload.usage_aggregator, 'sum');
    assert.strictEqual(payload.price_in_decimal, undefined);
    assert.deepStrictEqual(payload.tiers, [
        { up_to: 1000, unit_price_in_decimal: '0' },
        { up_to: 'inf', unit_price_in_decimal: '0.002', flat_price_in_decimal: '5' },
    ]);
    assert.strictEqual(catalog.usagePricePayload('stripe', 'p', 'Calls', monthly, price({ pricing_model: 'VOLUME', tiers: [] }))
        .pricing_model, 'volume');
});
const filter = (field, operator, ...values) => ({ field, operator, values });
// Meters as the database holds them, with our gateway's properties under their default names.
const defaultKeys = [
    { field: 'Status code', eventKey: 'response.status' },
    { field: 'HTTP method', eventKey: 'request.verb' },
    { field: 'Resource', eventKey: 'metadata.apiResourceTemplate' },
    { field: 'API type', eventKey: 'metadata.apiType' },
    { field: 'Prompt tokens', eventKey: 'metadata.aiTokenUsage.promptTokens' },
    { field: 'Total tokens', eventKey: 'metadata.aiTokenUsage.totalTokens' },
];
const calls = { handle: 'calls', display_name: 'Calls', type: 'REQUEST_COUNT', type_key: null, filter_fields: defaultKeys };
const tokens = (typeKey, filterFields = defaultKeys) => ({ handle: 'llm', display_name: 'LLM tokens', type: 'TOKENS',
    type_key: typeKey, filter_fields: filterFields });
const must = (meter, ...filters) => catalog.meterQuery(catalog.priceMeter(meter, filters)).query.bool.must.slice(2);

test('starts with matches values that begin with the text, on the keyword field', () => {
    assert.deepStrictEqual(must(calls, filter('Resource', 'STARTS_WITH', '/orders')),
        [{ prefix: { 'metadata.apiResourceTemplate.raw': '/orders' } }]);
    assert.deepStrictEqual(must(calls, filter('Resource', 'STARTS_WITH', '/')), [{ prefix: { 'metadata.apiResourceTemplate.raw': '/' } }]);
});

test('a price counts its meter\'s calls of one subscription that match all of its filters', () => {
    const query = catalog.meterQuery(catalog.priceMeter(calls, [
        filter('Status code', 'IN', '200', '304'),
        filter('HTTP method', 'NOT_EQUALS', 'OPTIONS'),
        filter('Resource', 'STARTS_WITH', '/orders'),
    ]));
    assert.deepStrictEqual(query.query.bool.must, [
        { terms: { 'metadata.billingCustomerId.raw': ['{{company_id}}'] } },
        { terms: { 'metadata.billingSubscriptionId.raw': ['{{subscription_id}}'] } },
        { terms: { 'response.status.raw': ['200', '304'] } },
        { exists: { field: 'request.verb.raw' } },
        { prefix: { 'metadata.apiResourceTemplate.raw': '/orders' } },
    ]);
    assert.deepStrictEqual(query.query.bool.must_not, [{ terms: { 'request.verb.raw': ['OPTIONS'] } }]);
    assert.deepStrictEqual(query.aggs.seg.aggs.usage_value, { sum: { field: 'weight', missing: 1 } });
    assert.strictEqual(query.size, 0);
});

// "Status code does not equal 500" means calls with another status code: a call that has no status code is not one of them.
test('equals and is one of match their values as text; does not equal and is not one of count the calls that have the property with another value', () => {
    const query = catalog.meterQuery(catalog.priceMeter(calls, [
        filter('Status code', 'NOT_EQUALS', '500'),
        filter('HTTP method', 'EQUALS', 'POST'),
        filter('API type', 'NOT_IN', 'LlmProvider', 'LlmProxy'),
    ]));
    assert.deepStrictEqual(query.query.bool.must.slice(2), [
        { exists: { field: 'response.status.raw' } },
        { terms: { 'request.verb.raw': ['POST'] } },
        { exists: { field: 'metadata.apiType.raw' } },
    ]);
    assert.deepStrictEqual(query.query.bool.must_not, [
        { terms: { 'response.status.raw': ['500'] } },
        { terms: { 'metadata.apiType.raw': ['LlmProvider', 'LlmProxy'] } },
    ]);
});

test('a price without filters counts every call of the subscription its meter counts', () => {
    const query = catalog.meterQuery(catalog.priceMeter(calls, []));
    assert.strictEqual(query.query.bool.must.length, 2);
    assert.strictEqual(query.query.bool.must_not, undefined);
});

test('a Tokens meter adds up the number at its event key, regardless of the key', () => {
    const sum = (key) => catalog.meterQuery(catalog.priceMeter(tokens(key), [])).aggs.seg.aggs.usage_value;
    assert.deepStrictEqual(sum('metadata.aiTokenUsage.promptTokens'), { sum: { field: 'metadata.aiTokenUsage.promptTokens', missing: 0 } });
    assert.deepStrictEqual(sum('usage.tokens.out'), { sum: { field: 'usage.tokens.out', missing: 0 } });
    assert.throws(() => sum(null), /The meter llm does not say which event key it adds up/);
});

test('each filter reads the event key of the property it names, whatever its name', () => {
    const custom = tokens('usage.in', [
        { field: 'Code', eventKey: 'res.code' },
        { field: 'Route', eventKey: 'route' },
        { field: 'Region', eventKey: 'meta.region' },
    ]);
    const query = catalog.meterQuery(catalog.priceMeter(custom, [
        filter('Code', 'IN', '200', '201'),
        filter('Route', 'STARTS_WITH', '/chat'),
        filter('Region', 'EQUALS', 'eu-west'),
    ]));
    assert.deepStrictEqual(query.query.bool.must.slice(2), [
        { terms: { 'res.code.raw': ['200', '201'] } },
        { prefix: { 'route.raw': '/chat' } },
        { terms: { 'meta.region.raw': ['eu-west'] } },
    ]);
    assert.deepStrictEqual(query.aggs.seg.aggs.usage_value, { sum: { field: 'usage.in', missing: 0 } });
    // The usage a subscriber sees is counted the same way.
    const search = catalog.usageSearch(catalog.priceMeter(custom, [filter('Region', 'EQUALS', 'eu-west')]),
        { customerId: 'cus_1', subscriptionId: 'sub_1' });
    assert.deepStrictEqual(search.query.bool.must.slice(2), [{ terms: { 'meta.region.raw': ['eu-west'] } }]);
    assert.deepStrictEqual(search.aggs.series.aggs.usage, { sum: { field: 'usage.in', missing: 0 } });
});

test('each comparison is a range on the number at the property\'s key itself, in must', () => {
    const compare = (operator, value, field = 'Prompt tokens') => ({ field, operator, values: [value] });
    const meter = tokens('metadata.aiTokenUsage.promptTokens');
    assert.deepStrictEqual(must(meter, compare('GREATER_THAN', '333')), [{ range: { 'metadata.aiTokenUsage.promptTokens': { gt: 333 } } }]);
    assert.deepStrictEqual(must(meter, compare('GREATER_THAN_OR_EQUAL', '333')), [{ range: { 'metadata.aiTokenUsage.promptTokens': { gte: 333 } } }]);
    assert.deepStrictEqual(must(meter, compare('LESS_THAN', '333')), [{ range: { 'metadata.aiTokenUsage.promptTokens': { lt: 333 } } }]);
    assert.deepStrictEqual(must(meter, compare('LESS_THAN_OR_EQUAL', '333')), [{ range: { 'metadata.aiTokenUsage.promptTokens': { lte: 333 } } }]);
    // A status code compared as a number, beside a range on one key and a match on another.
    const query = catalog.meterQuery(catalog.priceMeter(meter, [
        filter('API type', 'EQUALS', 'LlmProvider'),
        compare('GREATER_THAN', '100'),
        compare('LESS_THAN_OR_EQUAL', '500'),
        compare('GREATER_THAN_OR_EQUAL', '400', 'Status code'),
    ]));
    assert.deepStrictEqual(query.query.bool.must.slice(2), [
        { terms: { 'metadata.apiType.raw': ['LlmProvider'] } },
        { range: { 'metadata.aiTokenUsage.promptTokens': { gt: 100 } } },
        { range: { 'metadata.aiTokenUsage.promptTokens': { lte: 500 } } },
        { range: { 'response.status': { gte: 400 } } },
    ]);
    assert.strictEqual(query.query.bool.must_not, undefined);
});

test('a filter on a property its meter does not have is refused before Moesif is called', () => {
    assert.throws(() => catalog.meterQuery(catalog.priceMeter(tokens('usage.in', []), [filter('API type', 'EQUALS', 'LlmProvider')])),
        /The meter llm has no event key for the API type filter/);
});

test('the billing meter reports to the plan\'s Stripe product and price', () => {
    const name = 'Weather API v1.0 - Gold - Successful calls';
    const payload = catalog.billingMeterPayload({
        provider: 'stripe',
        providerParams: stripeConnector.moesifBillingPlanParams({ productRef: 'prod_123', productName: name, priceRef: 'price_456' }),
        name,
        meter: { handle: 'successful-calls', display_name: 'Successful calls', type: 'REQUEST_COUNT', conditions: [] },
    });
    // Moesif refuses a billing meter whose slug is not one of its own, or that has no url_query.
    assert.strictEqual(payload.slug, 'hourly_usage');
    assert.match(payload.url_query, /^\?seg\[metrics\]/);
    assert.strictEqual(payload.name, 'Weather API v1.0 - Gold - Successful calls');
    assert.strictEqual(payload.zero_balance_behavior, 'bill_customer');
    assert.deepStrictEqual(payload.billing_plan, {
        provider_slug: 'stripe',
        params: {
            usage_multiplier: '1.0',
            usage_rounding_mode: 'up',
            stripe_params: {
                product: { id: 'prod_123', name: 'Weather API v1.0 - Gold - Successful calls', usage_type: 'metered' },
                prices: [{ price_id: 'price_456' }],
                reporting: { reporting_period: '15m' },
            },
        },
    });
});
