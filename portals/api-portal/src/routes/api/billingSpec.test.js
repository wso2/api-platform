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
 * The Billing operations' request schemas, as the OpenAPI validator in front of the handlers
 * applies them. The handlers re-validate everything (billingValidation.test.js); this guards
 * the published contract and that the schemas compile at all.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const path = require('node:path');
const express = require('express');
const OpenApiValidator = require('express-openapi-validator');
const { formatValidationErrors } = require('../../utils/validationErrors');

const SPEC_PATH = path.join(__dirname, '..', '..', '..', 'docs', 'api-portal-openapi-spec-v0.9.yaml');
const BASE = '/api-portal/api/v0.9';

let server;

// Same request-validation settings as apiPortalRouter, minus auth and handlers.
test.before(async () => {
    const app = express();
    app.use(express.json());
    app.use(OpenApiValidator.middleware({
        apiSpec: SPEC_PATH,
        validateRequests: { allowUnknownQueryParameters: false },
        validateResponses: false,
        validateSecurity: false,
    }));
    app.post(`${BASE}/meters`, (req, res) => res.json({}));
    app.put(`${BASE}/meters/:meterId`, (req, res) => res.json({}));
    app.post(`${BASE}/subscription-plans`, (req, res) => res.json({}));
    app.put(`${BASE}/subscription-plans`, (req, res) => res.json({}));
    // Every method, so only the spec decides which reach a handler.
    app.all(`${BASE}/subscription-plans/:planId/pricing`, (req, res) => res.json({}));
    // The refusal as apiPortalRouter words it.
    app.use((err, req, res, next) => res.status(err.status || 500)
        .json(Array.isArray(err.errors) ? formatValidationErrors(err.errors) : { message: err.message }));
    await new Promise((resolve) => { server = app.listen(0, resolve); });
});

test.after(() => server.close());

/** The status of the request; with `withMessage`, [status, the refusal's message]. */
function send(method, urlPath, body, withMessage = false) {
    return new Promise((resolve, reject) => {
        const data = body === undefined ? '' : JSON.stringify(body);
        const req = http.request({
            port: server.address().port, path: BASE + urlPath, method,
            headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(data) },
        }, (res) => {
            let text = '';
            res.setEncoding('utf8');
            res.on('data', (chunk) => { text += chunk; });
            res.on('end', () => resolve(withMessage ? [res.statusCode, JSON.parse(text || '{}').message] : res.statusCode));
        });
        req.on('error', reject);
        req.end(data);
    });
}

const meter = { displayName: 'API calls', type: 'REQUEST_COUNT' };

test('a meter request with a known type and an event key is accepted', async () => {
    assert.equal(await send('POST', '/meters', meter), 200);
    assert.equal(await send('POST', '/meters', { ...meter, id: 'api-calls', typeKey: null }), 200);
    assert.equal(await send('PUT', '/meters/api-calls', meter), 200);
    assert.equal(await send('POST', '/meters', { displayName: 'Input tokens', type: 'TOKENS',
        typeKey: 'metadata.aiTokenUsage.promptTokens' }), 200);
});

// The service answers a missing, misplaced or malformed event key with its own message.
test('a meter request with any event key reaches the service', async () => {
    assert.equal(await send('POST', '/meters', { ...meter, type: 'TOKENS' }), 200);
    assert.equal(await send('POST', '/meters', { ...meter, typeKey: 'usage.in' }), 200);
    assert.equal(await send('POST', '/meters', { ...meter, type: 'TOKENS', typeKey: 'has spaces ' + 'k'.repeat(300) }), 200);
});

test('a meter request with an unknown type, no name, a bad id or a field a meter does not have is refused', async () => {
    assert.equal(await send('POST', '/meters', { ...meter, type: 'BYTES' }), 400);
    assert.equal(await send('POST', '/meters', { type: 'REQUEST_COUNT' }), 400);
    assert.equal(await send('POST', '/meters', { ...meter, type: 'TOKENS', eventProperty: 'INPUT_TOKENS' }), 400);
    assert.equal(await send('PUT', '/meters/api-calls', { ...meter, color: 'red' }), 400);
});

// The service answers this with its own message (meterValidation), so the schema lets it through.
test('a meter request with a malformed id reaches the service', async () => {
    assert.equal(await send('POST', '/meters', { ...meter, id: 'Bad_Id' }), 200);
});

test('a meter request with named filter properties and their event keys is accepted', async () => {
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [] }), 200);
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [{ field: 'HTTP method', eventKey: 'request.verb' },
        { field: 'Region', eventKey: 'metadata.region' }] }), 200);
    assert.equal(await send('PUT', '/meters/api-calls', { ...meter,
        filterFields: [{ field: 'n'.repeat(40), eventKey: 'metadata.' + 'k'.repeat(246) }] }), 200);
    assert.equal(await send('POST', '/meters', { ...meter,
        filterFields: Array.from({ length: 10 }, (_, i) => ({ field: `Property ${i}`, eventKey: `usage.k${i}` })) }), 200);
});

test('a meter request with eleven filter properties is refused', async () => {
    assert.equal(await send('POST', '/meters', { ...meter,
        filterFields: Array.from({ length: 11 }, (_, i) => ({ field: `Property ${i}`, eventKey: `usage.k${i}` })) }), 400);
});

test('a meter request with a filter property lacking a name or an event key, or with anything else, is refused', async () => {
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [{ field: 'Color' }] }), 400);
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [{ eventKey: 'metadata.color' }] }), 400);
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [{ field: '', eventKey: 'metadata.color' }] }), 400);
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [{ field: 'n'.repeat(41), eventKey: 'metadata.color' }] }), 400);
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [{ field: 'Color', eventKey: '' }] }), 400);
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [{ field: 'Color', eventKey: null }] }), 400);
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [{ field: 'Color', eventKey: 42 }] }), 400);
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: ['API type'] }), 400);
    assert.equal(await send('POST', '/meters', { ...meter, filterFields: [{}] }), 400);
    assert.equal(await send('POST', '/meters', { ...meter,
        filterFields: [{ field: 'API type', eventKey: 'metadata.apiType', operator: 'EQUALS' }] }), 400);
});

const perUnit = {
    meterId: 'llm-input',
    filters: [{ field: 'API type', operator: 'EQUALS', values: ['LlmProvider'] }],
    pricingModel: 'PER_UNIT',
    unitAmount: '2.50',
    unitQuantity: 1000000,
};
const graduated = {
    meterId: 'api-calls',
    pricingModel: 'GRADUATED',
    tiers: [{ upTo: 1000, unitAmount: '0' }, { upTo: null, unitAmount: '0.002', flatAmount: '5' }],
};
const flatRate = { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '20.00' };
const usageBased = { pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH', prices: [perUnit, graduated] };

// A plan is created with its pricing; the pricing then cannot change, so there is no pricing PUT or DELETE.
const createPlan = (pricing) => send('POST', '/subscription-plans', { displayName: 'Gold', pricing });

test('a plan create or upsert with a flat rate or usage based pricing is accepted', async () => {
    assert.equal(await createPlan(flatRate), 200);
    assert.equal(await createPlan({ pricingType: 'FLAT_RATE', currency: 'EUR', flatAmount: '10' }), 200);
    assert.equal(await createPlan(usageBased), 200);
    assert.equal(await createPlan({ ...usageBased, flatAmount: null, prices: [graduated] }), 200);
    assert.equal(await createPlan({
        ...usageBased,
        prices: [perUnit, { ...perUnit, filters: [{ field: 'API type', operator: 'IN', values: ['LlmProxy', 'Mcp'] }] },
            { ...graduated, filters: [
                { field: 'HTTP method', operator: 'NOT_IN', values: ['DELETE', 'OPTIONS'] },
                { field: 'Resource', operator: 'STARTS_WITH', values: ['/orders'] },
                { field: 'Status code', operator: 'NOT_EQUALS', values: ['500'] },
                { field: 'API type', operator: 'NOT_IN', values: ['LlmProvider', 'LlmProxy'] },
            ] }],
    }), 200);
    // Comparisons: the number operators, one value; 20 filters in all.
    const condition = (operator, value, field = 'Prompt tokens') => ({ field, operator, values: [value] });
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit, filters: [
        condition('GREATER_THAN', '100'), condition('LESS_THAN_OR_EQUAL', '500'), condition('GREATER_THAN_OR_EQUAL', '1', 'Total tokens'),
        condition('LESS_THAN', '9', 'Total tokens'), condition('EQUALS', '0', 'Cached tokens'),
    ] }] }), 200);
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit,
        filters: Array.from({ length: 20 }, (_, i) => condition('GREATER_THAN', '1', `Property ${i}`)) }] }), 200);
    assert.equal(await send('POST', '/subscription-plans', [{ displayName: 'Gold', pricing: flatRate }, { displayName: 'Free' }]), 200);
    assert.equal(await send('PUT', '/subscription-plans', { id: 'gold', displayName: 'Gold', pricing: usageBased }), 200);
    assert.equal(await send('PUT', '/subscription-plans', [{ id: 'gold', displayName: 'Gold', pricing: flatRate }]), 200);
});

test('a plan pricing with no or an unknown type, a non-string amount, an unknown model, or a filter without a property name or with an unknown operator is refused', async () => {
    assert.equal(await createPlan({ currency: 'USD', flatAmount: '20.00' }), 400);
    assert.equal(await createPlan({ ...flatRate, pricingType: 'HYBRID' }), 400);
    assert.equal(await createPlan({ ...flatRate, flatAmount: 99 }), 400);
    assert.equal(await createPlan({ ...usageBased, billingPeriod: 'WEEK' }), 400);
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit, pricingModel: 'FLAT' }] }), 400);
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit, meterId: undefined }] }), 400);
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit, unitAmount: 2.5 }] }), 400);
    const withFilter = (filter) => ({ ...usageBased, prices: [{ ...perUnit, filters: [filter] }] });
    assert.equal(await createPlan(withFilter({ field: 'API type', operator: 'IN' })), 400);
    assert.equal(await createPlan(withFilter({ field: 'API type', values: ['LlmProvider'] })), 400);
    assert.equal(await createPlan(withFilter({ operator: 'EQUALS', values: ['gpt-4o'] })), 400);
    assert.equal(await createPlan(withFilter({ field: '', operator: 'EQUALS', values: ['gpt-4o'] })), 400);
    assert.equal(await createPlan(withFilter({ field: 'n'.repeat(41), operator: 'EQUALS', values: ['gpt-4o'] })), 400);
    assert.equal(await createPlan(withFilter({ field: 'API type', operator: 'LIKE', values: ['Llm'] })), 400);
    assert.equal(await createPlan(withFilter({ field: 'Prompt tokens', operator: 'BETWEEN', values: ['1'] })), 400);
    assert.equal(await createPlan({
        ...usageBased, prices: [{ ...graduated, tiers: [{ upTo: 'many', unitAmount: '1' }] }],
    }), 400);
    assert.equal(await createPlan({ pricingType: 'FLAT_RATE', flatAmount: '1' }), 400);
});

// The service answers these with its own message (planPricingValidation), so the schema lets them through.
// Pricing can't change once the plan exists, so a mistyped field is refused rather than ignored.
test('a plan pricing with a field the pricing, a price, a filter or a tier does not take is refused, naming its place', async () => {
    const refusal = (pricing) => send('POST', '/subscription-plans', { displayName: 'Gold', pricing }, true);
    assert.deepEqual(await refusal({ ...flatRate, period: 'YEAR' }), [400, 'period is not a known field']);
    assert.deepEqual(await refusal({ ...flatRate, createdAt: '2026-10-22T00:00:00Z' }), [400, 'createdAt is read-only']);
    assert.deepEqual(await refusal({ ...usageBased, prices: [perUnit, { ...graduated, displayName: 'Calls' }] }),
        [400, 'displayName of price 2 is not a known field']);
    assert.deepEqual(await refusal({ ...usageBased, prices: [{ ...perUnit, unitQuantitiy: 1000 }] }),
        [400, 'unitQuantitiy of price 1 is not a known field']);
    assert.deepEqual(await refusal({ ...usageBased, prices: [{ ...perUnit, filters: [{ ...perUnit.filters[0], value: 'o1' }] }] }),
        [400, 'value of filter 1 of price 1 is not a known field']);
    assert.deepEqual(await refusal({ ...usageBased, prices: [{ ...perUnit, filters: [{ ...perUnit.filters[0], eventKey: 'metadata.apiType' }] }] }),
        [400, 'eventKey of filter 1 of price 1 is not a known field']);
    assert.deepEqual(await refusal({ ...usageBased, prices: [{ ...graduated, tiers: [graduated.tiers[0], { upto: null, unitAmount: '1' }] }] }),
        [400, 'upto of tier 2 of price 1 is not a known field']);
});

test('a plan pricing with a malformed amount, too many prices, filters or filter values, or none, or any property name, reaches the service', async () => {
    assert.equal(await createPlan({ ...flatRate, flatAmount: '-1' }), 200);
    assert.equal(await createPlan({ ...flatRate, flatAmount: '1234567890123' }), 200);
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit, unitAmount: '2.5.0' }] }), 200);
    assert.equal(await createPlan({
        ...usageBased, prices: [{ ...graduated, tiers: [{ upTo: null, unitAmount: 'free', flatAmount: '0.0000000000001' }] }],
    }), 200);
    assert.equal(await createPlan({ ...usageBased, prices: Array(21).fill(graduated) }), 200);
    const values = Array.from({ length: 21 }, (_, i) => `type-${i}`);
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit, filters: [{ field: 'API type', operator: 'IN', values }] }] }), 200);
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit, filters: [{ field: 'API type', operator: 'IN', values: [] }] }] }), 200);
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit, filters: Array.from({ length: 21 },
        (_, i) => ({ field: `Property ${i}`, operator: 'GREATER_THAN', values: ['1'] })) }] }), 200);
    assert.equal(await createPlan({ ...usageBased, prices: [{ ...perUnit, filters: [{ field: 'Model', operator: 'EQUALS', values: ['gpt-4o'] }] }] }), 200);
});

test('the pricing of a plan can be read but not set or deleted', async () => {
    assert.equal(await send('GET', '/subscription-plans/gold/pricing'), 200);
    assert.equal(await send('PUT', '/subscription-plans/gold/pricing', flatRate), 405);
    assert.equal(await send('DELETE', '/subscription-plans/gold/pricing'), 405);
});
