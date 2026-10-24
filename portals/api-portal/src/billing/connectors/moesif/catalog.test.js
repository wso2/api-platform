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
// Meters as the database holds them, reading our gateway's default event keys.
const defaultKeys = [
    { field: 'HTTP_STATUS_CODE', eventKey: 'response.status' },
    { field: 'HTTP_METHOD', eventKey: 'request.verb' },
    { field: 'RESOURCE', eventKey: 'metadata.apiResourceTemplate' },
    { field: 'API_TYPE', eventKey: 'metadata.apiType' },
];
const calls = { handle: 'calls', display_name: 'Calls', type: 'REQUEST_COUNT', type_key: null, filter_fields: defaultKeys };
const tokens = (typeKey, filterFields = defaultKeys) => ({ handle: 'llm', display_name: 'LLM tokens', type: 'TOKENS',
    type_key: typeKey, filter_fields: filterFields });

// Like a router's path prefix: "/orders" is the resource itself or one under it, never "/orders-archive".
test('a starts-with filter counts its path and the resources under it, by whole segments; "/" counts every resource', () => {
    const under = (path) => catalog.meterQuery(catalog.priceMeter(calls, [filter('RESOURCE', 'STARTS_WITH', path)])).query.bool.must[2];
    assert.deepStrictEqual(under('/orders/items'), { bool: { should: [
        { terms: { 'metadata.apiResourceTemplate.raw': ['/orders/items'] } },
        { prefix: { 'metadata.apiResourceTemplate.raw': '/orders/items/' } }], minimum_should_match: 1 } });
    assert.deepStrictEqual(under('/'), { prefix: { 'metadata.apiResourceTemplate.raw': '/' } });
});

test('a price counts its meter\'s calls of one subscription that match all of its filters', () => {
    const query = catalog.meterQuery(catalog.priceMeter(calls, [
        filter('HTTP_STATUS_CODE', 'IN', '200', '304'),
        filter('HTTP_METHOD', 'NOT_EQUALS', 'OPTIONS'),
        filter('RESOURCE', 'STARTS_WITH', '/orders'),
    ]));
    assert.deepStrictEqual(query.query.bool.must, [
        { terms: { 'metadata.billingCustomerId.raw': ['{{company_id}}'] } },
        { terms: { 'metadata.billingSubscriptionId.raw': ['{{subscription_id}}'] } },
        { terms: { 'response.status': [200, 304] } },
        { exists: { field: 'request.verb.raw' } },
        { bool: { should: [{ terms: { 'metadata.apiResourceTemplate.raw': ['/orders'] } }, { prefix: { 'metadata.apiResourceTemplate.raw': '/orders/' } }],
            minimum_should_match: 1 } },
    ]);
    assert.deepStrictEqual(query.query.bool.must_not, [{ terms: { 'request.verb.raw': ['OPTIONS'] } }]);
    assert.deepStrictEqual(query.aggs.seg.aggs.usage_value, { sum: { field: 'weight', missing: 1 } });
    assert.strictEqual(query.size, 0);
});

// "Status code does not equal 500" means calls with another status code: a call that has no status code is not one of them.
test('equals and is one of match their values; does not equal and is not one of count the calls that have the field with another value', () => {
    const query = catalog.meterQuery(catalog.priceMeter(calls, [
        filter('HTTP_STATUS_CODE', 'NOT_EQUALS', '500'),
        filter('HTTP_METHOD', 'EQUALS', 'POST'),
        filter('RESOURCE', 'NOT_EQUALS', '/health'),
    ]));
    assert.deepStrictEqual(query.query.bool.must.slice(2), [
        { exists: { field: 'response.status' } },
        { terms: { 'request.verb.raw': ['POST'] } },
        { exists: { field: 'metadata.apiResourceTemplate.raw' } },
    ]);
    assert.deepStrictEqual(query.query.bool.must_not, [
        { terms: { 'response.status': [500] } },
        { terms: { 'metadata.apiResourceTemplate.raw': ['/health'] } },
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

test('each filter matches the meter\'s own event key: a status code as a number, other values on the keyword field', () => {
    const custom = tokens('usage.in', [
        { field: 'HTTP_STATUS_CODE', eventKey: 'res.code' },
        { field: 'HTTP_METHOD', eventKey: 'req.method' },
        { field: 'RESOURCE', eventKey: 'route' },
        { field: 'API_TYPE', eventKey: 'api.kind' },
    ]);
    const query = catalog.meterQuery(catalog.priceMeter(custom, [
        filter('HTTP_STATUS_CODE', 'IN', '200', '201'),
        filter('HTTP_METHOD', 'NOT_IN', 'DELETE'),
        filter('RESOURCE', 'STARTS_WITH', '/chat'),
        filter('API_TYPE', 'EQUALS', 'LlmProvider'),
    ]));
    assert.deepStrictEqual(query.query.bool.must, [
        { terms: { 'metadata.billingCustomerId.raw': ['{{company_id}}'] } },
        { terms: { 'metadata.billingSubscriptionId.raw': ['{{subscription_id}}'] } },
        { terms: { 'res.code': [200, 201] } },
        { exists: { field: 'req.method.raw' } },
        { bool: { should: [{ terms: { 'route.raw': ['/chat'] } }, { prefix: { 'route.raw': '/chat/' } }], minimum_should_match: 1 } },
        { terms: { 'api.kind.raw': ['LlmProvider'] } },
    ]);
    assert.deepStrictEqual(query.query.bool.must_not, [{ terms: { 'req.method.raw': ['DELETE'] } }]);
    assert.deepStrictEqual(query.aggs.seg.aggs.usage_value, { sum: { field: 'usage.in', missing: 0 } });
    // The usage a subscriber sees is counted the same way.
    const search = catalog.usageSearch(catalog.priceMeter(custom, [filter('API_TYPE', 'EQUALS', 'LlmProvider')]),
        { customerId: 'cus_1', subscriptionId: 'sub_1' });
    assert.deepStrictEqual(search.query.bool.must.slice(2), [{ terms: { 'api.kind.raw': ['LlmProvider'] } }]);
    assert.deepStrictEqual(search.aggs.series.aggs.usage, { sum: { field: 'usage.in', missing: 0 } });
});

test('an API type filter matches the key\'s keyword field exactly', () => {
    const query = catalog.meterQuery(catalog.priceMeter(calls, [
        filter('API_TYPE', 'IN', 'RestApi', 'WebSubApi'),
        filter('HTTP_METHOD', 'NOT_EQUALS', 'OPTIONS'),
    ]));
    assert.deepStrictEqual(query.query.bool.must.slice(2), [
        { terms: { 'metadata.apiType.raw': ['RestApi', 'WebSubApi'] } },
        { exists: { field: 'request.verb.raw' } },
    ]);
    assert.deepStrictEqual(query.query.bool.must_not, [{ terms: { 'request.verb.raw': ['OPTIONS'] } }]);
    const excluding = catalog.meterQuery(catalog.priceMeter(tokens('usage.in', [{ field: 'API_TYPE', eventKey: 'metadata.kind' }]),
        [filter('API_TYPE', 'NOT_IN', 'LlmProvider', 'LlmProxy')]));
    assert.deepStrictEqual(excluding.query.bool.must.slice(2), [{ exists: { field: 'metadata.kind.raw' } }]);
    assert.deepStrictEqual(excluding.query.bool.must_not, [{ terms: { 'metadata.kind.raw': ['LlmProvider', 'LlmProxy'] } }]);
});

test('an MCP method or Tool filter matches the key\'s keyword field', () => {
    const mcp = { handle: 'mcp', display_name: 'MCP calls', type: 'REQUEST_COUNT', type_key: null, filter_fields: [
        { field: 'MCP_METHOD', eventKey: 'metadata.mcpAnalytics.jsonRpcMethod' },
        { field: 'MCP_TOOL', eventKey: 'metadata.mcpAnalytics.capabilityName' },
    ] };
    const query = catalog.meterQuery(catalog.priceMeter(mcp, [
        filter('MCP_TOOL', 'NOT_IN', 'add'),
        filter('MCP_METHOD', 'EQUALS', 'tools/call'),
    ]));
    assert.deepStrictEqual(query.query.bool.must.slice(2), [
        { exists: { field: 'metadata.mcpAnalytics.capabilityName.raw' } },
        { terms: { 'metadata.mcpAnalytics.jsonRpcMethod.raw': ['tools/call'] } },
    ]);
    assert.deepStrictEqual(query.query.bool.must_not, [{ terms: { 'metadata.mcpAnalytics.capabilityName.raw': ['add'] } }]);
});

test('each Token condition is a range on the number at its key, in must', () => {
    const condition = (operator, value, key = 'metadata.aiTokenUsage.promptTokens') => ({ field: 'TOKEN', eventKey: key, operator, values: [value] });
    const must = (...filters) => catalog.meterQuery(catalog.priceMeter(tokens('metadata.aiTokenUsage.promptTokens'), filters)).query.bool.must.slice(2);
    assert.deepStrictEqual(must(condition('GREATER_THAN', '333')), [{ range: { 'metadata.aiTokenUsage.promptTokens': { gt: 333 } } }]);
    assert.deepStrictEqual(must(condition('GREATER_THAN_OR_EQUAL', '333')), [{ range: { 'metadata.aiTokenUsage.promptTokens': { gte: 333 } } }]);
    assert.deepStrictEqual(must(condition('LESS_THAN', '333')), [{ range: { 'metadata.aiTokenUsage.promptTokens': { lt: 333 } } }]);
    assert.deepStrictEqual(must(condition('LESS_THAN_OR_EQUAL', '333')), [{ range: { 'metadata.aiTokenUsage.promptTokens': { lte: 333 } } }]);
    assert.deepStrictEqual(must(condition('EQUALS', '0')), [{ range: { 'metadata.aiTokenUsage.promptTokens': { gte: 0, lte: 0 } } }]);
    // A range on one key with conditions on another key and an API type.
    const query = catalog.meterQuery(catalog.priceMeter(tokens('metadata.aiTokenUsage.promptTokens'), [
        filter('API_TYPE', 'EQUALS', 'LlmProvider'),
        condition('GREATER_THAN', '100'),
        condition('LESS_THAN_OR_EQUAL', '500'),
        condition('GREATER_THAN', '0', 'metadata.aiTokenUsage.totalTokens'),
    ]));
    assert.deepStrictEqual(query.query.bool.must.slice(2), [
        { terms: { 'metadata.apiType.raw': ['LlmProvider'] } },
        { range: { 'metadata.aiTokenUsage.promptTokens': { gt: 100 } } },
        { range: { 'metadata.aiTokenUsage.promptTokens': { lte: 500 } } },
        { range: { 'metadata.aiTokenUsage.totalTokens': { gt: 0 } } },
    ]);
    assert.strictEqual(query.query.bool.must_not, undefined);
    // The usage a subscriber sees is counted with the same conditions.
    const search = catalog.usageSearch(catalog.priceMeter(calls, [condition('GREATER_THAN', '0', 'metadata.aiTokenUsage.totalTokens')]),
        { customerId: 'cus_1', subscriptionId: 'sub_1' });
    assert.deepStrictEqual(search.query.bool.must.slice(2), [{ range: { 'metadata.aiTokenUsage.totalTokens': { gt: 0 } } }]);
});

test('a filter on a field its meter has no event key for is refused before Moesif is called', () => {
    assert.throws(() => catalog.meterQuery(catalog.priceMeter(tokens('usage.in', []), [filter('API_TYPE', 'EQUALS', 'LlmProvider')])),
        /The meter llm has no event key for the API_TYPE filter/);
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
