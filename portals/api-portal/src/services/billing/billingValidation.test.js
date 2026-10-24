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
 * Validation of meters and plan pricing, in isolation. The handlers and their database state
 * are covered by billing.handlers.test.js.
 */

const test = require('node:test');
const assert = require('node:assert');
const { validateMeter, handleFromName, MeterValidationError, TOKEN_EVENT_KEYS, DEFAULT_TOKEN_EVENT_KEY } = require('./meterValidation');
const { validatePricing, resolvePrices, priceAppliesToApiType, PricingValidationError } = require('./planPricingValidation');
const { FILTER_RULES } = require('./meterValidation');

const meter = (extra) => ({ displayName: 'Successful calls', type: 'REQUEST_COUNT', ...extra });

test('a meter id is derived from the display name when none is given', () => {
    const m = validateMeter(meter(), { create: true });
    assert.strictEqual(m.handle, 'successful-calls');
    assert.strictEqual(m.handleGiven, false);
    assert.strictEqual(handleFromName('  Tokens (GPT-4o) / In  '), 'tokens-gpt-4o-in');
    assert.match(handleFromName('AI'), /^meter-[0-9a-f]{6}$/);
});

test('a given meter id must be a 3 to 40 character slug', () => {
    assert.strictEqual(validateMeter(meter({ id: 'calls-2xx' }), { create: true }).handle, 'calls-2xx');
    for (const id of ['ab', 'Calls', 'calls_2xx', 'x'.repeat(41), 12]) {
        assert.throws(() => validateMeter(meter({ id }), { create: true }), MeterValidationError, `id ${id}`);
    }
});

test('a meter needs a known type and a display name', () => {
    assert.throws(() => validateMeter(meter({ type: 'BANDWIDTH' }), { create: true }), MeterValidationError);
    assert.throws(() => validateMeter(meter({ displayName: '  ' }), { create: true }), MeterValidationError);
    assert.strictEqual(validateMeter(meter({ type: 'tokens', typeKey: 'metadata.aiTokenUsage.totalTokens' }),
        { create: true }).type, 'TOKENS');
    assert.throws(() => validateMeter(meter({ type: 'TOTAL_TOKENS' }), { create: true }), MeterValidationError);
});

test('a Tokens meter needs the event key it adds up; an API requests meter takes none', () => {
    const tokens = (extra) => meter({ type: 'TOKENS', ...extra });
    assert.strictEqual(validateMeter(tokens({ typeKey: 'metadata.aiTokenUsage.promptTokens' }), { create: true })
        .typeKey, 'metadata.aiTokenUsage.promptTokens');
    // Any key the administrator writes, trimmed: another gateway may name its fields differently.
    assert.strictEqual(validateMeter(tokens({ typeKey: '  usage.tokens.in  ' }), { create: true }).typeKey,
        'usage.tokens.in');
    assert.strictEqual(validateMeter(tokens({ typeKey: 'k'.repeat(255) }), { create: true }).typeKey,
        'k'.repeat(255));
    assert.strictEqual(validateMeter(meter(), { create: true }).typeKey, null);
    assert.strictEqual(validateMeter(meter({ typeKey: null }), { create: true }).typeKey, null);
    for (const typeKey of [undefined, null]) {
        assert.throws(() => validateMeter(tokens({ typeKey }), { create: true }),
            { message: 'A TOKENS meter needs a typeKey' });
    }
    for (const typeKey of ['', '   ', 'usage tokens', 'usage\ttokens', 'usage\u0007', 'k'.repeat(256), 42]) {
        assert.throws(() => validateMeter(tokens({ typeKey }), { create: true }),
            { message: 'typeKey must be 1 to 255 characters with no spaces' }, JSON.stringify(typeKey));
    }
    for (const typeKey of ['metadata.aiTokenUsage.promptTokens', '']) {
        assert.throws(() => validateMeter(meter({ typeKey }), { create: true }),
            { message: 'typeKey applies only to a TOKENS meter' });
    }
});

test('the settings page offers our gateway\'s three token keys, the prompt tokens first and by default', () => {
    assert.deepStrictEqual(TOKEN_EVENT_KEYS.map((k) => [k.label, k.key]), [
        ['Prompt tokens', 'metadata.aiTokenUsage.promptTokens'],
        ['Completion tokens', 'metadata.aiTokenUsage.completionTokens'],
        ['Total tokens', 'metadata.aiTokenUsage.totalTokens'],
    ]);
    assert.strictEqual(DEFAULT_TOKEN_EVENT_KEY, 'metadata.aiTokenUsage.promptTokens');
    for (const { key } of TOKEN_EVENT_KEYS) {
        assert.strictEqual(validateMeter(meter({ type: 'TOKENS', typeKey: key }), { create: true }).typeKey, key);
    }
});

test('a meter request with a field a meter does not have is refused, the old eventProperty included', () => {
    assert.throws(() => validateMeter(meter({ type: 'TOKENS', eventProperty: 'INPUT_TOKENS' }), { create: true }),
        { message: 'eventProperty is not a meter field' });
    assert.throws(() => validateMeter(meter({ color: 'red' }), { create: true }), { message: 'color is not a meter field' });
    assert.throws(() => validateMeter(meter({ planCount: 0 }), { create: false }), { message: 'planCount is not a meter field' });
});

test('a meter has no conditions: each plan price gives its own filters', () => {
    for (const conditions of [[], [{ field: 'API_TYPE', operator: 'IN' }], null]) {
        assert.throws(() => validateMeter(meter({ conditions }), { create: true }),
            /A meter has no conditions; each plan price gives its own filters/);
    }
});

test('a meter lists the fields its prices can filter on, each once, with the default event key of each', () => {
    assert.deepStrictEqual(validateMeter(meter(), { create: true }).filterFields, []);
    assert.deepStrictEqual(validateMeter(meter({
        filterFields: [{ field: 'HTTP_STATUS_CODE' }, { field: 'http_method' }, { field: 'RESOURCE' }, { field: 'api_type' }],
    }), { create: true }).filterFields, [
        { field: 'HTTP_STATUS_CODE', eventKey: 'response.status' },
        { field: 'HTTP_METHOD', eventKey: 'request.verb' },
        { field: 'RESOURCE', eventKey: 'metadata.apiResourceTemplate' },
        { field: 'API_TYPE', eventKey: 'metadata.apiType' },
    ]);
    // The fields in the order lists show them: API type after Resource, then Token.
    assert.deepStrictEqual(Object.keys(FILTER_RULES), ['HTTP_STATUS_CODE', 'HTTP_METHOD', 'RESOURCE', 'API_TYPE', 'TOKEN']);
    // Any meter may list the API type, its key editable like every key.
    assert.deepStrictEqual(validateMeter(meter({ type: 'TOKENS', typeKey: 'metadata.aiTokenUsage.promptTokens',
        filterFields: [{ field: 'API_TYPE', eventKey: 'metadata.kind' }] }), { create: true }).filterFields,
    [{ field: 'API_TYPE', eventKey: 'metadata.kind' }]);
    const bad = [
        ['API_TYPE', /filterFields must be an array/],
        [[{ field: 'COLOR' }], /filterFields: unknown field COLOR/],
        [[{ field: 'MODEL' }], /filterFields: unknown field MODEL/],
        [[{ field: 'API_TYPE' }, { field: 'api_type' }], /filterFields: API_TYPE is listed twice/],
        [[{ field: 'API_TYPE', operator: 'EQUALS' }], /Each entry of filterFields has only a field and an eventKey/],
        [['API_TYPE'], /Each entry of filterFields has only a field and an eventKey/],
    ];
    for (const [filterFields, message] of bad) {
        assert.throws(() => validateMeter(meter({ filterFields }), { create: true }), message, JSON.stringify(filterFields));
    }
});

test('a filter field reads the event key given, or its default when the key is left out or empty', () => {
    const fields = (filterFields) => validateMeter(meter({ filterFields }), { create: true }).filterFields;
    assert.deepStrictEqual(fields([{ field: 'API_TYPE', eventKey: '  metadata.kind ' }, { field: 'HTTP_STATUS_CODE', eventKey: 'status' }]), [
        { field: 'API_TYPE', eventKey: 'metadata.kind' },
        { field: 'HTTP_STATUS_CODE', eventKey: 'status' },
    ]);
    for (const eventKey of [undefined, null, '', '  ']) {
        assert.deepStrictEqual(fields([{ field: 'RESOURCE', eventKey }]), [{ field: 'RESOURCE', eventKey: 'metadata.apiResourceTemplate' }],
            JSON.stringify(eventKey));
    }
    const refused = [
        ['HTTP_STATUS_CODE', 'response status', 'Status code'],
        ['HTTP_METHOD', 'k'.repeat(256), 'HTTP method'],
        ['RESOURCE', 'api\nResource', 'Resource'],
        ['API_TYPE', 7, 'API type'],
    ];
    for (const [field, eventKey, label] of refused) {
        assert.throws(() => fields([{ field, eventKey }]),
            { message: `The event key of ${label} must be 1 to 255 characters with no spaces` }, field);
    }
});

const perUnit = (extra) => ({ pricingModel: 'PER_UNIT', meterId: 'calls', unitAmount: '0.008', ...extra });
const tiered = (model, tiers, extra) => ({ pricingModel: model, meterId: 'calls', tiers, ...extra });
const usage = (extra) => ({ pricingType: 'USAGE_BASED', currency: 'usd', billingPeriod: 'month', prices: [perUnit()], ...extra });
const flat = (extra) => ({ pricingType: 'flat_rate', currency: 'usd', billingPeriod: 'month', flatAmount: '99.00', ...extra });
const filter = (field, operator, ...values) => ({ field, operator, values });

test('pricing is normalized and keeps amounts as the exact strings given', () => {
    assert.deepStrictEqual(validatePricing(flat()), {
        pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '99.00', prices: [],
    });
    assert.deepStrictEqual(validatePricing(usage({ prices: [perUnit({ pricingModel: 'per_unit', unitQuantity: 1000 })] })), {
        pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH', flatAmount: null, prices: [{
            meterId: 'calls', filters: [], pricingModel: 'PER_UNIT', unitAmount: '0.008', unitQuantity: 1000, tiers: null,
        }],
    });
    assert.strictEqual(validatePricing(usage()).prices[0].unitQuantity, 1);
    assert.strictEqual(validatePricing(usage({ billingPeriod: undefined })).billingPeriod, 'MONTH');
});

test('a paid plan is flat rate or usage based, never both', () => {
    assert.throws(() => validatePricing(usage({ pricingType: undefined })), /pricingType must be one of FLAT_RATE, USAGE_BASED/);
    assert.throws(() => validatePricing(flat({ flatAmount: '0.00' })),
        { message: 'flatAmount must be more than 0. Create a free plan instead.' });
    assert.throws(() => validatePricing(flat({ flatAmount: undefined })), /flatAmount is required/);
    assert.throws(() => validatePricing(flat({ prices: [perUnit()] })), /A flat rate plan has no usage prices/);
    assert.throws(() => validatePricing(usage({ flatAmount: '5.00' })), /A usage based plan has no flat amount/);
    assert.throws(() => validatePricing(usage({ prices: [] })), /A usage based plan needs 1 to 20 prices/);
    assert.strictEqual(validatePricing(usage({ prices: Array.from({ length: 20 }, () => perUnit()) })).prices.length, 20);
    assert.throws(() => validatePricing(usage({ prices: Array.from({ length: 21 }, () => perUnit()) })), /needs 1 to 20 prices/);
    assert.throws(() => validatePricing(usage({ prices: 'calls' })), /prices must be an array/);
});

test('a usage price needs a meter, and no model takes the fields of another', () => {
    assert.throws(() => validatePricing(usage({ prices: [perUnit({ meterId: '' })] })), /meterId of price 1 is required/);
    assert.throws(() => validatePricing(usage({ prices: [perUnit(), tiered('VOLUME', [{ upTo: null, unitAmount: '1' }],
        { meterId: undefined })] })), /meterId of price 2 is required/);
    assert.throws(() => validatePricing(usage({ prices: [perUnit({ meterId: 'm'.repeat(41) })] })),
        /^Error: meterId of price 1 must be at most 40 characters$/);
    const mixed = [
        perUnit({ tiers: [{ upTo: null, unitAmount: '1' }] }),
        tiered('GRADUATED', [{ upTo: null, unitAmount: '1' }], { unitAmount: '1' }),
        tiered('VOLUME', [{ upTo: null, unitAmount: '1' }], { unitQuantity: 10 }),
    ];
    for (const price of mixed) {
        assert.throws(() => validatePricing(usage({ prices: [price] })), /does not take/, JSON.stringify(price));
    }
});

test('pricing refuses bad models, amounts, currencies, periods and unit quantities', () => {
    const bad = [
        usage({ prices: [perUnit({ pricingModel: 'FLAT' })] }),
        usage({ prices: [perUnit({ pricingModel: undefined })] }),
        usage({ currency: 'US' }),
        usage({ currency: 'JPY' }),
        flat({ currency: 'kwd' }),
        flat({ currency: 'XYZ' }),
        usage({ billingPeriod: 'WEEK' }),
        flat({ flatAmount: '-1' }),
        flat({ flatAmount: '1e3' }),
        flat({ flatAmount: '1.0000000000001' }),
        usage({ prices: [perUnit({ unitAmount: undefined })] }),
        usage({ prices: [perUnit({ unitQuantity: 0 })] }),
        usage({ prices: [perUnit({ unitQuantity: 1.5 })] }),
        usage({ prices: [perUnit({ unitQuantity: 1000000001 })] }),
    ];
    for (const body of bad) {
        assert.throws(() => validatePricing(body), PricingValidationError, JSON.stringify(body));
    }
});

test('tiers need rising upper bounds, an open last tier and a price each', () => {
    const ok = validatePricing(usage({ prices: [tiered('GRADUATED', [
        { upTo: 1000, unitAmount: '0' },
        { upTo: 10000, unitAmount: '0.002', flatAmount: '5' },
        { upTo: null, flatAmount: '20' },
    ])] })).prices[0];
    assert.strictEqual(ok.pricingModel, 'GRADUATED');
    assert.deepStrictEqual(ok.tiers, [
        { upTo: 1000, unitAmount: '0', flatAmount: null },
        { upTo: 10000, unitAmount: '0.002', flatAmount: '5' },
        { upTo: null, unitAmount: null, flatAmount: '20' },
    ]);
    const bad = [
        [],
        Array.from({ length: 11 }, (_, i) => ({ upTo: i === 10 ? null : (i + 1) * 10, unitAmount: '1' })),
        [{ upTo: 100, unitAmount: '1' }],
        [{ upTo: 100, unitAmount: '1' }, { upTo: 50, unitAmount: '1' }, { upTo: null, unitAmount: '1' }],
        [{ upTo: 100 }, { upTo: null, unitAmount: '1' }],
        [{ upTo: 1.5, unitAmount: '1' }, { upTo: null, unitAmount: '1' }],
    ];
    for (const tiers of bad) {
        assert.throws(() => validatePricing(usage({ prices: [tiered('VOLUME', tiers)] })), PricingValidationError,
            JSON.stringify(tiers));
    }
    // A tier is named by its place, as the Platform API names it.
    assert.throws(() => validatePricing(usage({ prices: [tiered('VOLUME', [{ upTo: 100, unitAmount: '1' }, { upTo: null }])] })),
        /^Error: Tier 2 of price 1 needs a unitAmount, a flatAmount or both$/);
    assert.throws(() => validatePricing(usage({ prices: [tiered('VOLUME', [{ upTo: 100, unitAmount: '1' },
        { upTo: null, flatAmount: '1e3' }])] })), /^Error: flatAmount of tier 2 of price 1 must be a decimal amount such as 9\.99$/);
});

test('a price filters on each field at most once, with the operators and values the field takes', () => {
    const one = (...filters) => validatePricing(usage({ prices: [perUnit({ filters })] })).prices[0].filters;
    assert.deepStrictEqual(one(filter('http_status_code', 'in', '200', '304', '200'), filter('HTTP_METHOD', 'not_in', 'options', 'head'),
        filter('RESOURCE', 'STARTS_WITH', '/orders'), filter('API_TYPE', 'EQUALS', 'RestApi')), [
        filter('HTTP_STATUS_CODE', 'IN', '200', '304'), filter('HTTP_METHOD', 'NOT_IN', 'OPTIONS', 'HEAD'),
        filter('RESOURCE', 'STARTS_WITH', '/orders'), filter('API_TYPE', 'EQUALS', 'RestApi'),
    ]);
    // A starts-with path is kept without a trailing "/": "/orders/" covers what "/orders" covers.
    assert.deepStrictEqual(one(filter('RESOURCE', 'STARTS_WITH', '/orders/')), [filter('RESOURCE', 'STARTS_WITH', '/orders')]);
    assert.deepStrictEqual(one(filter('RESOURCE', 'STARTS_WITH', '/')), [filter('RESOURCE', 'STARTS_WITH', '/')]);
    assert.deepStrictEqual(one(filter('RESOURCE', 'EQUALS', '/orders/')), [filter('RESOURCE', 'EQUALS', '/orders/')]);
    const bad = [
        [[filter('HTTP_STATUS_CODE', 'STARTS_WITH', '2')], /^Error: The HTTP_STATUS_CODE operator of price 1 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN$/],
        [[filter('RESOURCE', 'IN', '/a')], /^Error: The RESOURCE operator of price 1 must be one of EQUALS, NOT_EQUALS, STARTS_WITH$/],
        [[filter('TOKEN_TYPE', 'EQUALS', 'INPUT')], /field of filter 1 of price 1 must be one of/],
        [[filter('MODEL', 'EQUALS', 'gpt-4o')], /field of filter 1 of price 1 must be one of/],
        [[filter('API_TYPE', 'IN', 'a'), filter('API_TYPE', 'NOT_IN', 'b')], /price 1 filters on API_TYPE more than once/],
        [[filter('API_TYPE', 'EQUALS', 'a', 'b')], /price 1: Operator EQUALS takes exactly one value/],
        [[filter('API_TYPE', 'IN')], /price 1: A filter on API_TYPE needs 1 to 20 values/],
        [[filter('API_TYPE', 'EQUALS')], /price 1: Operator EQUALS takes exactly one value/],
        [[filter('HTTP_STATUS_CODE', 'IN', '2xx')], /is not a status code/],
        [[filter('RESOURCE', 'EQUALS', 'orders')], /A resource must start with \//],
        [[filter('HTTP_METHOD', 'EQUALS', 'FETCH')], /HTTP method must be one of/],
        [['API_TYPE'], /Each filter of price 1 needs a field, an operator and values/],
    ];
    for (const [filters, message] of bad) {
        assert.throws(() => validatePricing(usage({ prices: [perUnit({ filters })] })), message, JSON.stringify(filters));
    }
    assert.throws(() => validatePricing(usage({ prices: [perUnit({ filters: 'API_TYPE' })] })), /filters of price 1 must be an array/);
});

test('an API type filter takes the matching operators and free-text values, kept exactly as written', () => {
    const one = (...filters) => validatePricing(usage({ prices: [perUnit({ filters })] })).prices[0].filters;
    assert.deepStrictEqual(one(filter('api_type', 'in', 'RestApi', 'restapi', ' Mcp ', 'RestApi')),
        [filter('API_TYPE', 'IN', 'RestApi', 'restapi', 'Mcp')]);
    for (const operator of ['EQUALS', 'NOT_EQUALS', 'NOT_IN']) {
        assert.deepStrictEqual(one(filter('API_TYPE', operator, 'LlmProvider')), [filter('API_TYPE', operator, 'LlmProvider')]);
    }
    assert.deepStrictEqual(one(filter('API_TYPE', 'IN', ...Array.from({ length: 20 }, (_, i) => 'Kind' + i)))[0].values.length, 20);
    const bad = [
        [filter('API_TYPE', 'STARTS_WITH', 'Llm'), /^Error: The API_TYPE operator of price 1 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN$/],
        [filter('API_TYPE', 'EQUALS', 'RestApi', 'Mcp'), /^Error: price 1: Operator EQUALS takes exactly one value$/],
        [filter('API_TYPE', 'IN'), /^Error: price 1: A filter on API_TYPE needs 1 to 20 values$/],
        [filter('API_TYPE', 'IN', ...Array.from({ length: 21 }, (_, i) => 'Kind' + i)), /^Error: price 1: A filter on API_TYPE needs 1 to 20 values$/],
        [filter('API_TYPE', 'EQUALS', 'k'.repeat(256)), /^Error: price 1: Filter values for API_TYPE must be 1 to 255 characters$/],
        [filter('API_TYPE', 'EQUALS', ' '), /^Error: price 1: Filter values for API_TYPE must be 1 to 255 characters$/],
    ];
    for (const [f, message] of bad) {
        assert.throws(() => validatePricing(usage({ prices: [perUnit({ filters: [f] })] })), message, JSON.stringify(f));
    }
});

// Meters as the database holds them.
const meters = new Map([
    ['calls', { uuid: 'm-calls', handle: 'calls', display_name: 'Calls', type: 'REQUEST_COUNT', type_key: null,
        filter_fields: ['HTTP_STATUS_CODE', 'HTTP_METHOD', 'RESOURCE', 'API_TYPE'].map((field) => ({ field, eventKey: field })) }],
    ['llm-input', { uuid: 'm-in', handle: 'llm-input', display_name: 'LLM input tokens', type: 'TOKENS',
        type_key: 'metadata.aiTokenUsage.promptTokens', filter_fields: [{ field: 'API_TYPE', eventKey: 'metadata.apiType' }] }],
]);
const resolve = (prices) => resolvePrices(validatePricing(usage({ prices })).prices, meters);
const on = (meterId, ...filters) => perUnit({ meterId, filters });

test('a plan may price a meter several times, as long as no request is counted twice', () => {
    const [a, b] = resolve([on('llm-input', filter('API_TYPE', 'EQUALS', 'LlmProvider')), on('llm-input', filter('API_TYPE', 'IN', 'LlmProxy', 'Mcp'))]);
    assert.strictEqual(a.meter.uuid, 'm-in');
    assert.strictEqual(b.meter.uuid, 'm-in');
    assert.throws(() => resolve([perUnit({ meterId: 'no-such-meter' })]), /Unknown meter: no-such-meter/);
    assert.throws(() => resolve([on('llm-input', filter('API_TYPE', 'IN', 'LlmProvider', 'LlmProxy')), on('calls'), on('llm-input', filter('API_TYPE', 'EQUALS', 'LlmProxy'))]),
        /^Error: Prices 1 and 3 both count the same requests of the meter llm-input; give them different filters\.$/);
    // Different meters are never compared.
    assert.strictEqual(resolve([on('calls'), on('llm-input')]).length, 2);
});

test('a price filters only on the fields its meter lets its prices filter on', () => {
    assert.throws(() => resolve([on('llm-input', filter('API_TYPE', 'EQUALS', 'LlmProvider')), on('llm-input', filter('HTTP_METHOD', 'EQUALS', 'GET'))]),
        /^Error: price 2: the meter llm-input does not allow filtering on HTTP_METHOD$/);
});

test('two prices on one meter count the same request when every field either filters on can match', () => {
    // [price A's filters, price B's filters, whether one request can be counted by both]
    const cases = [
        [[], [], true],
        [[], [filter('API_TYPE', 'EQUALS', 'a')], true],
        [[filter('API_TYPE', 'IN', 'a', 'b')], [filter('API_TYPE', 'IN', 'b', 'c')], true],
        [[filter('API_TYPE', 'EQUALS', 'a')], [filter('API_TYPE', 'EQUALS', 'b')], false],
        [[filter('API_TYPE', 'IN', 'a')], [filter('API_TYPE', 'NOT_IN', 'a')], false],
        [[filter('API_TYPE', 'IN', 'a', 'b')], [filter('API_TYPE', 'NOT_EQUALS', 'a')], true],
        [[filter('RESOURCE', 'EQUALS', '/orders/1')], [filter('RESOURCE', 'STARTS_WITH', '/orders')], true],
        [[filter('RESOURCE', 'EQUALS', '/users')], [filter('RESOURCE', 'STARTS_WITH', '/orders')], false],
        [[filter('RESOURCE', 'STARTS_WITH', '/orders')], [filter('RESOURCE', 'STARTS_WITH', '/orders/items')], true],
        [[filter('RESOURCE', 'STARTS_WITH', '/orders')], [filter('RESOURCE', 'STARTS_WITH', '/users')], false],
        // Whole path segments: "/orders" is not under "/order", and "/orders-archive" not under "/orders".
        [[filter('RESOURCE', 'STARTS_WITH', '/order')], [filter('RESOURCE', 'STARTS_WITH', '/orders')], false],
        [[filter('RESOURCE', 'EQUALS', '/orders-archive')], [filter('RESOURCE', 'STARTS_WITH', '/orders')], false],
        [[filter('RESOURCE', 'EQUALS', '/orders')], [filter('RESOURCE', 'STARTS_WITH', '/orders/')], true],
        [[filter('RESOURCE', 'EQUALS', '/users')], [filter('RESOURCE', 'STARTS_WITH', '/')], true],
        [[filter('RESOURCE', 'NOT_EQUALS', '/a')], [filter('RESOURCE', 'STARTS_WITH', '/b')], true],
        [[filter('API_TYPE', 'NOT_IN', 'a')], [filter('API_TYPE', 'NOT_EQUALS', 'b')], true],
        [[filter('HTTP_STATUS_CODE', 'NOT_IN', '500')], [filter('HTTP_STATUS_CODE', 'NOT_IN', '200')], true],
        [[filter('HTTP_METHOD', 'NOT_IN', 'GET', 'POST')], [filter('HTTP_METHOD', 'NOT_IN', 'PUT')], true],
        [[filter('HTTP_METHOD', 'NOT_IN', 'GET', 'POST', 'PUT')], [filter('HTTP_METHOD', 'NOT_IN', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS')], false],
        // Apart on one field is enough.
        [[filter('API_TYPE', 'EQUALS', 'a'), filter('HTTP_METHOD', 'EQUALS', 'GET')],
            [filter('API_TYPE', 'EQUALS', 'a'), filter('HTTP_METHOD', 'EQUALS', 'POST')], false],
        [[filter('API_TYPE', 'EQUALS', 'a'), filter('HTTP_METHOD', 'EQUALS', 'GET')], [filter('API_TYPE', 'IN', 'a', 'b')], true],
        // The API type is matched case-sensitively.
        [[filter('API_TYPE', 'EQUALS', 'RestApi')], [filter('API_TYPE', 'EQUALS', 'LlmProvider')], false],
        [[filter('API_TYPE', 'EQUALS', 'RestApi')], [filter('API_TYPE', 'EQUALS', 'restapi')], false],
        [[filter('API_TYPE', 'IN', 'RestApi', 'Mcp')], [filter('API_TYPE', 'IN', 'Mcp')], true],
        [[filter('API_TYPE', 'EQUALS', 'RestApi')], [filter('API_TYPE', 'NOT_EQUALS', 'RestApi')], false],
        [[filter('API_TYPE', 'IN', 'LlmProvider')], [filter('API_TYPE', 'NOT_IN', 'RestApi', 'Mcp')], true],
        [[filter('API_TYPE', 'NOT_IN', 'RestApi')], [filter('API_TYPE', 'NOT_IN', 'LlmProvider')], true],
        [[filter('API_TYPE', 'EQUALS', 'RestApi')], [filter('HTTP_STATUS_CODE', 'EQUALS', '200')], true],
        [[filter('API_TYPE', 'EQUALS', 'RestApi'), filter('HTTP_STATUS_CODE', 'EQUALS', '200')],
            [filter('API_TYPE', 'EQUALS', 'LlmProvider'), filter('HTTP_STATUS_CODE', 'EQUALS', '200')], false],
    ];
    for (const [a, b, meet] of cases) {
        const attempt = () => resolve([on('calls', ...a), on('calls', ...b)]);
        if (meet) assert.throws(attempt, /both count the same requests/, JSON.stringify([a, b]));
        else assert.strictEqual(attempt().length, 2, JSON.stringify([a, b]));
    }
});

test('a price applies to an API unless its API type filter, read at the default key, rules the API\'s type out', () => {
    const typed = { filter_fields: [{ field: 'API_TYPE', eventKey: 'metadata.apiType' }] };
    const custom = { filter_fields: [{ field: 'API_TYPE', eventKey: 'metadata.kind' }] };
    const applies = (f, apiType, meterRow = typed) => priceAppliesToApiType(f ? [f] : [], meterRow, apiType);
    assert.strictEqual(applies(null, 'RestApi'), true, 'no API type filter');
    assert.strictEqual(applies(filter('HTTP_METHOD', 'EQUALS', 'GET'), 'RestApi'), true, 'another field only');
    assert.strictEqual(applies(filter('API_TYPE', 'EQUALS', 'RestApi'), 'RestApi'), true);
    assert.strictEqual(applies(filter('API_TYPE', 'EQUALS', 'LlmProvider'), 'RestApi'), false);
    assert.strictEqual(applies(filter('API_TYPE', 'IN', 'LlmProvider', 'RestApi'), 'RestApi'), true);
    assert.strictEqual(applies(filter('API_TYPE', 'IN', 'LlmProvider', 'LlmProxy'), 'RestApi'), false);
    assert.strictEqual(applies(filter('API_TYPE', 'NOT_EQUALS', 'RestApi'), 'RestApi'), false);
    assert.strictEqual(applies(filter('API_TYPE', 'NOT_EQUALS', 'Mcp'), 'RestApi'), true);
    assert.strictEqual(applies(filter('API_TYPE', 'NOT_IN', 'RestApi', 'Mcp'), 'RestApi'), false);
    assert.strictEqual(applies(filter('API_TYPE', 'NOT_IN', 'RestApi', 'Mcp'), 'LlmProvider'), true);
    // Matched exactly as written.
    assert.strictEqual(applies(filter('API_TYPE', 'EQUALS', 'restapi'), 'RestApi'), false);
    // A meter that reads the type at its own key: the API's value is unknown, so the price applies.
    assert.strictEqual(applies(filter('API_TYPE', 'EQUALS', 'LlmProvider'), 'RestApi', custom), true);
    // An unknown API type rules nothing out.
    assert.strictEqual(applies(filter('API_TYPE', 'EQUALS', 'LlmProvider'), null), true);
    // Prices name the gateway's API types, which differ from the portal's for GraphQL and WebSocket APIs.
    assert.strictEqual(applies(filter('API_TYPE', 'EQUALS', 'GraphQLApi'), 'GRAPHQL'), true);
    assert.strictEqual(applies(filter('API_TYPE', 'NOT_EQUALS', 'GraphQLApi'), 'GRAPHQL'), false);
    assert.strictEqual(applies(filter('API_TYPE', 'EQUALS', 'GRAPHQL'), 'GRAPHQL'), false, 'the gateway never sends GRAPHQL');
    assert.strictEqual(applies(filter('API_TYPE', 'IN', 'WebSocketApi', 'Mcp'), 'WS'), true);
    assert.strictEqual(applies(filter('API_TYPE', 'EQUALS', 'Mcp'), 'Mcp'), true);
    assert.strictEqual(applies(filter('API_TYPE', 'NOT_IN', 'RestApi', 'Mcp'), 'Mcp'), false);
    // A SOAP API has no gateway counterpart, so no price is ruled out for it.
    assert.strictEqual(applies(filter('API_TYPE', 'EQUALS', 'LlmProvider'), 'SOAP'), true);
});

// ---------------------------------------------------------------------------
// Token fields and conditions
// ---------------------------------------------------------------------------

const PROMPT = 'metadata.aiTokenUsage.promptTokens';
const TOTAL = 'metadata.aiTokenUsage.totalTokens';
const COMPLETION = 'metadata.aiTokenUsage.completionTokens';

test('a meter may list a Token field once per key, with a key, on any meter', () => {
    const fields = (filterFields, extra) => validateMeter(meter({ filterFields, ...extra }), { create: true }).filterFields;
    assert.deepStrictEqual(fields([{ field: 'token', eventKey: ` ${TOTAL} ` }, { field: 'TOKEN', eventKey: 'usage.cached' },
        { field: 'API_TYPE' }]), [
        { field: 'TOKEN', eventKey: TOTAL },
        { field: 'TOKEN', eventKey: 'usage.cached' },
        { field: 'API_TYPE', eventKey: 'metadata.apiType' },
    ]);
    const tokens = { type: 'TOKENS', typeKey: PROMPT };
    assert.deepStrictEqual(fields([{ field: 'TOKEN', eventKey: TOTAL }], tokens), [{ field: 'TOKEN', eventKey: TOTAL }]);
    const refused = [
        [[{ field: 'TOKEN' }], {}, 'A Token field needs an eventKey'],
        [[{ field: 'TOKEN', eventKey: '  ' }], {}, 'A Token field needs an eventKey'],
        [[{ field: 'TOKEN', eventKey: 'usage tokens' }], {}, 'The event key of Token must be 1 to 255 characters with no spaces'],
        [[{ field: 'TOKEN', eventKey: TOTAL }, { field: 'TOKEN', eventKey: ` ${TOTAL}` }], {},
            `The meter lists the Token key ${TOTAL} more than once`],
        [[{ field: 'TOKEN', eventKey: PROMPT }], tokens, `The key ${PROMPT} is already the meter's Token key`],
        [[{ field: 'API_TYPE' }, { field: 'API_TYPE' }], {}, 'filterFields: API_TYPE is listed twice'],
        [Array.from({ length: 11 }, (_, i) => ({ field: 'TOKEN', eventKey: `usage.k${i}` })), {}, 'A meter has at most 10 filter fields'],
    ];
    for (const [filterFields, extra, message] of refused) {
        assert.throws(() => fields(filterFields, extra), { message }, JSON.stringify(filterFields));
    }
    // Ten fields: the four fixed ones and six Token keys.
    const ten = ['HTTP_STATUS_CODE', 'HTTP_METHOD', 'RESOURCE', 'API_TYPE'].map((field) => ({ field }))
        .concat(Array.from({ length: 6 }, (_, i) => ({ field: 'TOKEN', eventKey: `usage.k${i}` })));
    assert.strictEqual(fields(ten).length, 10);
});

const tokenFilter = (operator, value, key = PROMPT) => ({ field: 'TOKEN', eventKey: key, operator, values: [value] });

test('a Token condition names its key, compares with a number operator and takes one whole number', () => {
    const one = (...filters) => validatePricing(usage({ prices: [perUnit({ filters })] })).prices[0].filters;
    assert.deepStrictEqual(one({ field: 'token', eventKey: ` ${PROMPT} `, operator: 'greater_than', values: ['0100'] },
        filter('API_TYPE', 'EQUALS', 'LlmProvider')), [
        { field: 'TOKEN', eventKey: PROMPT, operator: 'GREATER_THAN', values: ['100'] },
        filter('API_TYPE', 'EQUALS', 'LlmProvider'),
    ]);
    for (const operator of ['GREATER_THAN', 'GREATER_THAN_OR_EQUAL', 'LESS_THAN', 'LESS_THAN_OR_EQUAL', 'EQUALS']) {
        assert.deepStrictEqual(one(tokenFilter(operator, '333')), [tokenFilter(operator, '333')], operator);
    }
    assert.deepStrictEqual(one(tokenFilter('LESS_THAN_OR_EQUAL', '1000000000000')), [tokenFilter('LESS_THAN_OR_EQUAL', '1000000000000')]);
    assert.deepStrictEqual(one(tokenFilter('EQUALS', '0')), [tokenFilter('EQUALS', '0')]);
    const value = 'price 1: the Token value must be a whole number from 0 to 1000000000000';
    const bad = [
        [[{ field: 'TOKEN', operator: 'GREATER_THAN', values: ['1'] }], 'price 1: a Token filter needs an eventKey'],
        [[{ ...tokenFilter('GREATER_THAN', '1'), eventKey: ' ' }], 'price 1: a Token filter needs an eventKey'],
        [[{ ...filter('API_TYPE', 'EQUALS', 'LlmProvider'), eventKey: PROMPT }], 'price 1: only a Token filter takes an eventKey'],
        [[tokenFilter('IN', '1')], 'The TOKEN operator of price 1 must be one of GREATER_THAN, GREATER_THAN_OR_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL, EQUALS'],
        [[tokenFilter('NOT_EQUALS', '1')], 'The TOKEN operator of price 1 must be one of GREATER_THAN, GREATER_THAN_OR_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL, EQUALS'],
        [[filter('API_TYPE', 'GREATER_THAN', '1')], 'The API_TYPE operator of price 1 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN'],
        [[filter('HTTP_STATUS_CODE', 'LESS_THAN', '500')], 'The HTTP_STATUS_CODE operator of price 1 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN'],
        [[tokenFilter('GREATER_THAN', '-1')], value],
        [[tokenFilter('GREATER_THAN', '1.5')], value],
        [[tokenFilter('GREATER_THAN', '1000000000001')], value],
        [[tokenFilter('GREATER_THAN', 'many')], value],
        [[{ ...tokenFilter('GREATER_THAN', '1'), values: ['1', '2'] }], value],
        [[{ ...tokenFilter('GREATER_THAN', '1'), values: [] }], value],
        [[tokenFilter('GREATER_THAN', '100'), tokenFilter('LESS_THAN', '500'), tokenFilter('LESS_THAN', '400')],
            `price 1: a price puts at most 2 Token conditions on ${PROMPT}`],
        [[tokenFilter('GREATER_THAN', '500'), tokenFilter('LESS_THAN_OR_EQUAL', '500')], `price 1: the Token conditions on ${PROMPT} can't all be true`],
        [[tokenFilter('EQUALS', '5'), tokenFilter('EQUALS', '6')], `price 1: the Token conditions on ${PROMPT} can't all be true`],
        [[tokenFilter('LESS_THAN', '0')], `price 1: the Token conditions on ${PROMPT} can't all be true`],
        [Array.from({ length: 21 }, (_, i) => tokenFilter('GREATER_THAN', '1', `usage.k${i}`)), 'price 1: a price has at most 20 filters'],
    ];
    for (const [filters, message] of bad) {
        assert.throws(() => validatePricing(usage({ prices: [perUnit({ filters })] })), { message }, JSON.stringify(filters));
    }
    // Two conditions on one key make a range; conditions on other keys are counted apart.
    assert.strictEqual(one(tokenFilter('GREATER_THAN', '100'), tokenFilter('LESS_THAN_OR_EQUAL', '500'),
        tokenFilter('GREATER_THAN', '0', TOTAL), tokenFilter('LESS_THAN', '9', TOTAL)).length, 4);
});

test('a Token condition compares one of the meter\'s Token keys: its Token row or a Token field', () => {
    const tokenMeters = new Map([
        ['llm', { uuid: 'm-llm', handle: 'llm', type: 'TOKENS', type_key: PROMPT,
            filter_fields: [{ field: 'TOKEN', eventKey: TOTAL }, { field: 'API_TYPE', eventKey: 'metadata.apiType' }] }],
        ['calls', { uuid: 'm-calls', handle: 'calls', type: 'REQUEST_COUNT', type_key: null,
            filter_fields: [{ field: 'TOKEN', eventKey: TOTAL }] }],
    ]);
    const priced = (meterId, ...filters) => resolvePrices(validatePricing(usage({ prices: [perUnit({ meterId, filters })] })).prices, tokenMeters);
    assert.strictEqual(priced('llm', tokenFilter('GREATER_THAN', '333')).length, 1, 'the Token row');
    assert.strictEqual(priced('llm', tokenFilter('GREATER_THAN', '0', TOTAL)).length, 1, 'a Token field');
    assert.strictEqual(priced('calls', tokenFilter('GREATER_THAN', '0', TOTAL)).length, 1, 'a Token field on a request meter');
    assert.throws(() => priced('llm', tokenFilter('GREATER_THAN', '0', COMPLETION)),
        { message: `price 1: the meter has no Token field with the key ${COMPLETION}` });
    assert.throws(() => priced('calls', tokenFilter('GREATER_THAN', '0', PROMPT)),
        { message: `price 1: the meter has no Token field with the key ${PROMPT}` });
});

test('two prices on one meter are apart on a Token key when their ranges share no count', () => {
    const tokenMeters = new Map([['llm', { uuid: 'm-llm', handle: 'llm', type: 'TOKENS', type_key: PROMPT,
        filter_fields: [{ field: 'TOKEN', eventKey: TOTAL }, { field: 'API_TYPE', eventKey: 'metadata.apiType' }] }]]);
    const both = (a, b) => resolvePrices(validatePricing(usage({ prices: [perUnit({ meterId: 'llm', filters: a }),
        perUnit({ meterId: 'llm', filters: b })] })).prices, tokenMeters);
    const lte = (v, key) => tokenFilter('LESS_THAN_OR_EQUAL', v, key);
    const gt = (v, key) => tokenFilter('GREATER_THAN', v, key);
    // [price A, price B, whether one request can be counted by both]
    const cases = [
        [[lte('333')], [gt('333')], false],
        [[gt('100')], [gt('333')], true],
        [[tokenFilter('LESS_THAN', '333')], [tokenFilter('GREATER_THAN_OR_EQUAL', '333')], false],
        [[tokenFilter('EQUALS', '333')], [gt('333')], false],
        [[tokenFilter('EQUALS', '333')], [tokenFilter('GREATER_THAN_OR_EQUAL', '333')], true],
        [[gt('100'), lte('500')], [gt('500')], false],
        [[gt('100'), lte('500')], [gt('400'), lte('900')], true],
        // A price with no condition on a key covers every count.
        [[gt('333')], [], true],
        // Different keys do not separate them.
        [[lte('333')], [gt('333', TOTAL)], true],
        // Mixed with the API type: apart on any one field or key is enough.
        [[lte('333'), filter('API_TYPE', 'EQUALS', 'LlmProvider')], [gt('333'), filter('API_TYPE', 'EQUALS', 'LlmProvider')], false],
        [[gt('100'), filter('API_TYPE', 'EQUALS', 'LlmProvider')], [gt('333'), filter('API_TYPE', 'EQUALS', 'LlmProxy')], false],
        [[gt('100'), filter('API_TYPE', 'EQUALS', 'LlmProvider')], [gt('333'), filter('API_TYPE', 'IN', 'LlmProvider', 'LlmProxy')], true],
    ];
    for (const [a, b, meet] of cases) {
        const attempt = () => both(a, b);
        if (meet) assert.throws(attempt, /both count the same requests of the meter llm/, JSON.stringify([a, b]));
        else assert.strictEqual(attempt().length, 2, JSON.stringify([a, b]));
    }
});
