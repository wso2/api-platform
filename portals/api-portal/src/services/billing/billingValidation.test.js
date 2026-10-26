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
const { validatePricing, resolvePrices, PricingValidationError } = require('./planPricingValidation');

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

test('a Tokens meter needs the event key it adds up; a Requests meter takes none', () => {
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
    for (const conditions of [[], [{ field: 'API type', operator: 'IN' }], null]) {
        assert.throws(() => validateMeter(meter({ conditions }), { create: true }),
            /A meter has no conditions; each plan price gives its own filters/);
    }
});

test('a meter lists the properties its prices can filter on, each a name and the event key it reads', () => {
    const fields = (filterFields) => validateMeter(meter({ filterFields }), { create: true }).filterFields;
    assert.deepStrictEqual(fields([{ field: ' Status code ', eventKey: ' response.status ' }, { field: 'Region', eventKey: 'meta.region' }]),
        [{ field: 'Status code', eventKey: 'response.status' }, { field: 'Region', eventKey: 'meta.region' }]);
    assert.deepStrictEqual(fields(undefined), []);
    assert.deepStrictEqual(fields([]), []);
    // Any name of 1 to 40 characters.
    assert.deepStrictEqual(fields([{ field: 'x'.repeat(40), eventKey: 'k' }]), [{ field: 'x'.repeat(40), eventKey: 'k' }]);
    for (const field of ['', '   ', 'x'.repeat(41), 7, null, 'a\u0007b']) {
        assert.throws(() => fields([{ field, eventKey: 'k' }]), /A filter property's name must be 1 to 40 characters/, String(field));
    }
    // Names are unique ignoring case; keys are unique.
    assert.throws(() => fields([{ field: 'Region', eventKey: 'a' }, { field: 'region', eventKey: 'b' }]),
        /The meter has the property region more than once/);
    assert.throws(() => fields([{ field: 'Code', eventKey: 'response.status' }, { field: 'Status', eventKey: 'response.status' }]),
        /Status and Code read the same event key response.status/);
    // A key is required, with no default.
    for (const eventKey of [undefined, null, '', '  ', 'has space', 'x'.repeat(256)]) {
        assert.throws(() => fields([{ field: 'Status code', eventKey }]), /The event key of Status code must be 1 to 255 characters with no spaces/,
            String(eventKey));
    }
    assert.throws(() => fields([{ field: 'Region', eventKey: 'k', extra: 1 }]), /Each entry of filterFields has only a field and an eventKey/);
    assert.throws(() => fields('Region'), /filterFields must be an array/);
    assert.throws(() => fields(Array.from({ length: 11 }, (_, k) => ({ field: `P${k}`, eventKey: `k${k}` }))),
        /A meter has at most 10 filter properties/);
    // A property may read a Tokens meter's own key, so prices can compare the number it adds up.
    assert.deepStrictEqual(validateMeter(meter({ type: 'TOKENS', typeKey: 'usage.in', filterFields: [{ field: 'Input', eventKey: 'usage.in' }] }),
        { create: true }).filterFields, [{ field: 'Input', eventKey: 'usage.in' }]);
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

test('a price filter names a property and takes any operator, with the values the operator takes', () => {
    const filters = (...given) => validatePricing(usage({ prices: [perUnit({ filters: given })] })).prices[0].filters;
    // Text values are kept as written, trimmed; "is one of" drops repeats.
    assert.deepStrictEqual(filters(filter(' HTTP method ', 'in', ' get ', 'POST', ' get ')),
        [filter('HTTP method', 'IN', 'get', 'POST')]);
    assert.deepStrictEqual(filters(filter('Resource', 'STARTS_WITH', '/orders/')), [filter('Resource', 'STARTS_WITH', '/orders/')]);
    assert.deepStrictEqual(filters(filter('API type', 'NOT_EQUALS', 'RestApi')), [filter('API type', 'NOT_EQUALS', 'RestApi')]);
    // Any operator on any property: a status code may start with a text, a method may be compared.
    assert.deepStrictEqual(filters(filter('Status code', 'STARTS_WITH', '2')), [filter('Status code', 'STARTS_WITH', '2')]);
    assert.deepStrictEqual(filters(filter('Status code', 'GREATER_THAN_OR_EQUAL', '0400')),
        [filter('Status code', 'GREATER_THAN_OR_EQUAL', '400')]);
    assert.deepStrictEqual(filters(filter('Prompt tokens', 'LESS_THAN', 333)), [filter('Prompt tokens', 'LESS_THAN', '333')]);
    // The values each operator takes.
    for (const [given, message] of [
        [filter('Region', 'EQUALS', 'a', 'b'), /Operator EQUALS takes exactly one value/],
        [filter('Region', 'STARTS_WITH'), /Operator STARTS_WITH takes exactly one value/],
        [filter('Region', 'IN'), /A filter on Region needs 1 to 20 values/],
        [filter('Region', 'NOT_IN', ...Array.from({ length: 21 }, (_, k) => `v${k}`)), /A filter on Region needs 1 to 20 values/],
        [filter('Region', 'EQUALS', ' '), /Filter values for Region must be 1 to 255 characters/],
        [filter('Region', 'EQUALS', 'x'.repeat(256)), /Filter values for Region must be 1 to 255 characters/],
        [filter('Tokens', 'GREATER_THAN', '1.5'), /A comparison on Tokens takes a whole number from 0 to 1000000000000/],
        [filter('Tokens', 'GREATER_THAN', '-1'), /A comparison on Tokens takes a whole number/],
        [filter('Tokens', 'LESS_THAN', '1000000000001'), /A comparison on Tokens takes a whole number/],
        [filter('Tokens', 'LESS_THAN', '1', '2'), /Operator LESS_THAN takes exactly one value/],
        [filter('Region', 'MATCHES', 'a'), /The operator of filter 1 of price 1 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN, STARTS_WITH/],
        [filter('', 'EQUALS', 'a'), /field of filter 1 of price 1 must be 1 to 40 characters/],
        [{ ...filter('Region', 'EQUALS', 'a'), eventKey: 'meta.region' }, /Each filter of price 1 has only a field, an operator and values/],
    ]) {
        assert.throws(() => filters(given), message, JSON.stringify(given));
    }
});

test('on one property a price has one text filter, or one or two comparisons that can hold together', () => {
    const filters = (...given) => validatePricing(usage({ prices: [perUnit({ filters: given })] })).prices[0].filters;
    assert.deepStrictEqual(filters(filter('Tokens', 'GREATER_THAN', '100'), filter('tokens', 'LESS_THAN_OR_EQUAL', '500')),
        [filter('Tokens', 'GREATER_THAN', '100'), filter('tokens', 'LESS_THAN_OR_EQUAL', '500')]);
    assert.throws(() => filters(filter('Region', 'EQUALS', 'a'), filter('region', 'NOT_EQUALS', 'b')), /price 1 filters on Region more than once/);
    assert.throws(() => filters(filter('Code', 'EQUALS', '200'), filter('Code', 'GREATER_THAN', '100')), /price 1 filters on Code more than once/);
    assert.throws(() => filters(filter('T', 'GREATER_THAN', '1'), filter('T', 'GREATER_THAN', '2'), filter('T', 'LESS_THAN', '9')),
        /a price puts at most 2 comparisons on T/);
    assert.throws(() => filters(filter('T', 'GREATER_THAN', '500'), filter('T', 'LESS_THAN', '100')), /the comparisons on T can't all be true/);
    assert.throws(() => filters(filter('T', 'GREATER_THAN', '5'), filter('T', 'LESS_THAN', '6')), /the comparisons on T can't all be true/);
    assert.strictEqual(filters(filter('T', 'GREATER_THAN_OR_EQUAL', '5'), filter('T', 'LESS_THAN_OR_EQUAL', '5')).length, 2);
});

const meters = new Map([
    ['calls', { handle: 'calls', type: 'REQUEST_COUNT', type_key: null, filter_fields: [
        { field: 'Status code', eventKey: 'response.status' },
        { field: 'HTTP method', eventKey: 'request.verb' },
        { field: 'Resource', eventKey: 'metadata.apiResourceTemplate' },
        { field: 'API type', eventKey: 'metadata.apiType' },
        { field: 'Prompt tokens', eventKey: 'metadata.aiTokenUsage.promptTokens' },
    ] }],
    ['bare', { handle: 'bare', type: 'REQUEST_COUNT', type_key: null, filter_fields: [] }],
]);
const resolve = (prices) => resolvePrices(validatePricing(usage({ prices })).prices, meters);
const on = (meterId, ...filters) => perUnit({ meterId, filters });

test('a plan may price a meter several times, as long as no request is counted twice', () => {
    const resolved = resolve([on('calls', filter('HTTP method', 'EQUALS', 'GET')), on('calls', filter('HTTP method', 'EQUALS', 'POST'))]);
    assert.strictEqual(resolved.length, 2);
    assert.strictEqual(resolved[0].meter.handle, 'calls');
    assert.throws(() => resolve([on('nope')]), /Unknown meter: nope/);
});

test('a price filters only on its meter\'s properties, named as the meter spells them', () => {
    assert.deepStrictEqual(resolve([on('calls', filter('http METHOD', 'EQUALS', 'GET'))])[0].filters, [filter('HTTP method', 'EQUALS', 'GET')]);
    assert.throws(() => resolve([on('bare', filter('HTTP method', 'EQUALS', 'GET'))]), /price 1: the meter bare has no property HTTP method/);
    assert.throws(() => resolve([on('calls', filter('Region', 'EQUALS', 'eu'))]), /price 1: the meter calls has no property Region/);
});

test('two prices on one meter count the same request when, on every property both filter on, their filters can match one value', () => {
    const f = filter;
    // [price A, price B, whether one request can be counted by both]
    const cases = [
        [[f('HTTP method', 'EQUALS', 'GET')], [f('HTTP method', 'EQUALS', 'POST')], false],
        [[f('HTTP method', 'IN', 'GET', 'POST')], [f('HTTP method', 'IN', 'POST', 'PUT')], true],
        [[f('HTTP method', 'EQUALS', 'GET')], [f('HTTP method', 'NOT_EQUALS', 'GET')], false],
        [[f('HTTP method', 'IN', 'GET', 'POST')], [f('HTTP method', 'NOT_IN', 'GET')], true],
        // Two exclusions can always overlap: the values are not known.
        [[f('HTTP method', 'NOT_IN', 'GET', 'HEAD', 'OPTIONS')], [f('HTTP method', 'NOT_IN', 'POST', 'PUT', 'PATCH', 'DELETE')], true],
        // A plain text prefix: "/orders" also starts "/orders-archive".
        [[f('Resource', 'STARTS_WITH', '/orders')], [f('Resource', 'EQUALS', '/orders-archive')], true],
        [[f('Resource', 'STARTS_WITH', '/orders/')], [f('Resource', 'EQUALS', '/orders-archive')], false],
        [[f('Resource', 'STARTS_WITH', '/orders')], [f('Resource', 'STARTS_WITH', '/orders/items')], true],
        [[f('Resource', 'STARTS_WITH', '/orders')], [f('Resource', 'STARTS_WITH', '/users')], false],
        [[f('Resource', 'STARTS_WITH', '/orders')], [f('Resource', 'NOT_EQUALS', '/orders')], true],
        // Comparisons: ranges that share no number are apart.
        [[f('Prompt tokens', 'LESS_THAN_OR_EQUAL', '333')], [f('Prompt tokens', 'GREATER_THAN', '333')], false],
        [[f('Prompt tokens', 'GREATER_THAN', '100')], [f('Prompt tokens', 'GREATER_THAN', '333')], true],
        [[f('Prompt tokens', 'GREATER_THAN', '100'), f('Prompt tokens', 'LESS_THAN_OR_EQUAL', '500')], [f('Prompt tokens', 'GREATER_THAN', '500')], false],
        [[f('Prompt tokens', 'GREATER_THAN', '100'), f('Prompt tokens', 'LESS_THAN_OR_EQUAL', '500')],
            [f('Prompt tokens', 'GREATER_THAN', '400'), f('Prompt tokens', 'LESS_THAN_OR_EQUAL', '900')], true],
        // A comparison and a text filter: apart when none of the text values is a number in the range.
        [[f('Status code', 'GREATER_THAN_OR_EQUAL', '400')], [f('Status code', 'IN', '200', '304')], false],
        [[f('Status code', 'GREATER_THAN_OR_EQUAL', '400')], [f('Status code', 'IN', '200', '404')], true],
        [[f('Status code', 'GREATER_THAN_OR_EQUAL', '400')], [f('Status code', 'NOT_EQUALS', '500')], true],
        // A property only one price filters on matches anything for the other.
        [[f('HTTP method', 'EQUALS', 'GET')], [], true],
        [[f('HTTP method', 'EQUALS', 'GET')], [f('API type', 'EQUALS', 'RestApi')], true],
        // Apart on any one property is enough.
        [[f('HTTP method', 'EQUALS', 'GET'), f('API type', 'EQUALS', 'RestApi')], [f('HTTP method', 'EQUALS', 'GET'), f('API type', 'EQUALS', 'Mcp')], false],
    ];
    for (const [a, b, overlap] of cases) {
        const attempt = () => resolve([on('calls', ...a), on('calls', ...b)]);
        if (overlap) assert.throws(attempt, /Prices 1 and 2 both count the same requests of the meter calls/, JSON.stringify([a, b]));
        else assert.strictEqual(attempt().length, 2, JSON.stringify([a, b]));
    }
    // Prices on different meters never overlap.
    assert.strictEqual(resolve([on('calls'), on('bare')]).length, 2);
});
