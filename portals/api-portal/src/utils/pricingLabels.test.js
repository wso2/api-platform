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
const { formatAmount, pricingSummary, priceLabel, pricingLines } = require('./pricingLabels');
const { filterFieldsLabel, filterLabel, meterTypeLabel, meterUnit, priceName } = require('./meterLabels');

test('amounts show the currency\'s own decimals, and more only where the digits given matter', () => {
    assert.strictEqual(formatAmount('99', 'USD'), '$99.00');
    assert.strictEqual(formatAmount('0.008', 'USD'), '$0.008');
    assert.strictEqual(formatAmount('1000', 'JPY'), '¥1,000');
    assert.strictEqual(formatAmount('12.5', 'EUR'), '€12.50');
    // The currency's own decimals; a trailing zero beyond them is dropped, a digit that matters kept.
    assert.strictEqual(formatAmount('12.00', 'JPY'), '¥12');
    assert.strictEqual(formatAmount('0.5', 'JPY'), '¥0.5');
    assert.strictEqual(formatAmount('12', 'KWD'), 'KWD\u00a012.000'); // a code is followed by a no-break space
    assert.strictEqual(formatAmount('0.0100', 'USD'), '$0.01');
    assert.strictEqual(formatAmount('12', 'AUD'), 'A$12.00');
    assert.strictEqual(formatAmount('5', 'ZZ'), '5 ZZ');
});

const PER_UNIT = { pricingModel: 'PER_UNIT', unitAmount: '0.01', unitQuantity: 1, meterId: 'calls', filters: [] };
const GRADUATED = { pricingModel: 'GRADUATED', meterId: 'calls', filters: [], tiers: [
    { upTo: 1000, unitAmount: '0', flatAmount: null },
    { upTo: null, unitAmount: '0.002', flatAmount: null },
] };
const INPUT = { pricingModel: 'PER_UNIT', unitAmount: '2.50', unitQuantity: 1000000, meterId: 'llm-input',
    filters: [{ field: 'API_TYPE', operator: 'EQUALS', values: ['LlmProvider'] }] };
const flat = (extra) => ({ pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '99.00', prices: [],
    ...extra });
const usage = (prices, extra) => ({ pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH', flatAmount: null, prices,
    ...extra });
const meters = new Map([
    ['calls', { display_name: 'Calls', type: 'REQUEST_COUNT', type_key: null }],
    ['llm-input', { display_name: 'LLM input tokens', type: 'TOKENS', type_key: 'metadata.aiTokenUsage.promptTokens' }],
]);

test('a plan is summarised as free, its flat rate, its one usage price, or how many prices it has', () => {
    assert.strictEqual(pricingSummary(null), 'Free');
    assert.strictEqual(pricingSummary(flat()), '$99.00 / month');
    assert.strictEqual(pricingSummary(flat({ billingPeriod: 'YEAR', flatAmount: '990' })), '$990.00 / year');
    assert.strictEqual(pricingSummary(usage([PER_UNIT]), meters), '$0.01 per request, billed monthly');
    assert.strictEqual(pricingSummary(usage([INPUT]), meters), '$2.50 per 1M tokens, billed monthly');
    assert.strictEqual(pricingSummary(usage([PER_UNIT])), '$0.01 per unit, billed monthly');
    assert.strictEqual(pricingSummary(usage([{ ...GRADUATED, pricingModel: 'VOLUME' }], { billingPeriod: 'YEAR' })),
        'Volume tiers, billed yearly');
    assert.strictEqual(pricingSummary(usage([GRADUATED, INPUT])), '2 usage prices, billed monthly');
});

test('meters read as what they count, their filter fields by name, and filters in full', () => {
    assert.strictEqual(meterTypeLabel('REQUEST_COUNT'), 'API requests');
    assert.strictEqual(meterTypeLabel('TOKENS'), 'Tokens');
    assert.strictEqual(filterFieldsLabel([{ field: 'API_TYPE', eventKey: 'metadata.kind' }, { field: 'HTTP_METHOD', eventKey: 'request.verb' }]),
        'API type, HTTP method');
    assert.strictEqual(filterFieldsLabel([]), '');
    assert.strictEqual(filterFieldsLabel([{ field: 'RESOURCE' }, { field: 'API_TYPE' }]), 'Resource, API type');
    assert.strictEqual(filterLabel({ field: 'API_TYPE', operator: 'IN', values: ['RestApi', 'Mcp'] }), 'API type is one of RestApi, Mcp');
    assert.strictEqual(filterLabel({ field: 'HTTP_STATUS_CODE', operator: 'IN', values: ['200', '201'] }), 'Status code is one of 200, 201');
    assert.strictEqual(filterLabel({ field: 'HTTP_METHOD', operator: 'NOT_EQUALS', values: ['GET'] }), 'HTTP method does not equal GET');
    assert.strictEqual(filterLabel({ field: 'RESOURCE', operator: 'STARTS_WITH', values: ['/orders'] }), 'Resource starts with /orders');
});

test('a price is named after its meter and its filters in short form, and counts in its meter\'s unit', () => {
    assert.strictEqual(priceName('LLM input tokens', INPUT.filters), 'LLM input tokens · LlmProvider');
    assert.strictEqual(priceName('API calls', [{ field: 'HTTP_METHOD', operator: 'IN', values: ['GET', 'POST'] }]), 'API calls · GET, POST');
    assert.strictEqual(priceName('API calls', [{ field: 'HTTP_METHOD', operator: 'NOT_IN', values: ['DELETE', 'OPTIONS'] },
        { field: 'RESOURCE', operator: 'STARTS_WITH', values: ['/orders'] }]), 'API calls · not DELETE, OPTIONS · /orders…');
    assert.strictEqual(priceName('API calls', []), 'API calls');
    // An API type reads as its value.
    assert.strictEqual(priceName('API requests', [{ field: 'API_TYPE', operator: 'EQUALS', values: ['RestApi'] }]), 'API requests · RestApi');
    assert.strictEqual(priceName('API requests', [{ field: 'API_TYPE', operator: 'NOT_EQUALS', values: ['LlmProvider'] }]),
        'API requests · not LlmProvider');
    // A Token condition reads as its key's label in lower case, or the key, and the comparison.
    const condition = (operator, value, key = 'metadata.aiTokenUsage.promptTokens') => ({ field: 'TOKEN', eventKey: key, operator, values: [value] });
    assert.strictEqual(priceName('Input tokens', [{ field: 'API_TYPE', operator: 'EQUALS', values: ['LlmProvider'] }, condition('GREATER_THAN', '333')]),
        'Input tokens · LlmProvider · prompt tokens > 333');
    assert.strictEqual(priceName('Calls', [condition('GREATER_THAN_OR_EQUAL', '1', 'metadata.aiTokenUsage.totalTokens'),
        condition('LESS_THAN', '10', 'metadata.aiTokenUsage.completionTokens'), condition('LESS_THAN_OR_EQUAL', '5', 'usage.cached'),
        condition('EQUALS', '0', 'usage.cached')]),
    'Calls · total tokens \u2265 1 · completion tokens < 10 · usage.cached \u2264 5 · usage.cached = 0');
    assert.strictEqual(filterLabel(condition('GREATER_THAN', '333')), 'Prompt tokens more than 333');
    assert.strictEqual(filterLabel(condition('EQUALS', '0', 'usage.cached')), 'usage.cached equal to 0');
    assert.strictEqual(filterFieldsLabel([{ field: 'API_TYPE' }, { field: 'TOKEN', eventKey: 'metadata.aiTokenUsage.totalTokens' },
        { field: 'TOKEN', eventKey: 'usage.cached' }]), 'API type, Token \u00b7 Total tokens, Token \u00b7 usage.cached');
    assert.strictEqual(priceName('LLM input tokens', [{ field: 'HTTP_METHOD', operator: 'EQUALS', values: ['POST'] },
        { field: 'API_TYPE', operator: 'IN', values: ['LlmProvider', 'LlmProxy'] }]), 'LLM input tokens · POST · LlmProvider, LlmProxy');
    // A Tokens meter counts tokens, regardless of the event key it adds up.
    assert.strictEqual(meterUnit({ type: 'TOKENS', type_key: 'metadata.aiTokenUsage.promptTokens' }), 'token');
    assert.strictEqual(meterUnit({ type: 'TOKENS', type_key: 'usage.out' }), 'token');
    assert.strictEqual(meterUnit({ type: 'REQUEST_COUNT' }), 'request');
    assert.strictEqual(meterUnit(null), 'request');
});

test('prices read as a price per unit, per batch of units, or by tiers', () => {
    assert.strictEqual(priceLabel(PER_UNIT, 'USD', 'request'), '$0.01 per request');
    assert.strictEqual(priceLabel({ ...PER_UNIT, unitAmount: '0.50', unitQuantity: 1000000 }, 'USD', 'token'), '$0.50 per 1M tokens');
    assert.strictEqual(priceLabel(GRADUATED, 'USD', 'request'), 'graduated: up to 1,000 at $0.00 per request, then $0.002 per request');
    assert.strictEqual(priceLabel({ ...GRADUATED, pricingModel: 'VOLUME', tiers: [
        { upTo: 100, unitAmount: null, flatAmount: '5' },
        { upTo: null, unitAmount: '0.01', flatAmount: '5' },
    ] }, 'USD', 'request'), 'volume: up to 100 at $5.00 flat, then $0.01 per request + $5.00 flat');
});

test('a plan lists its flat rate, or each usage price under its name', () => {
    assert.deepStrictEqual(pricingLines(null, meters), []);
    assert.deepStrictEqual(pricingLines(flat(), meters), ['$99.00 / month']);
    assert.deepStrictEqual(pricingLines(usage([PER_UNIT, INPUT]), meters), [
        'Calls · $0.01 per request',
        'LLM input tokens · LlmProvider · $2.50 per 1M tokens',
        'Billed monthly',
    ]);
});
