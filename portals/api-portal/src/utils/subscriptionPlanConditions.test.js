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
const {
    normalizeConditions, requestWindowsFromLimits, requestCondition, withRequestCondition,
    requestLimitsFromConditions, displayLimits, conditionChips, limitLines,
} = require('./subscriptionPlanConditions');
const { ValidationError } = require('./errors/customErrors');

const w = (count, timeAmount, timeUnit) => ({ count, timeAmount, timeUnit });
const limitsOf = (type, ...windows) => ({ type, params: { limits: windows } });

function refused(conditions, message) {
    assert.throws(() => normalizeConditions(conditions), (err) => {
        assert.ok(err instanceof ValidationError);
        assert.equal(err.message, message);
        return true;
    });
}

test('built-in conditions get their fixed ids and defaults', () => {
    assert.deepEqual(normalizeConditions([
        limitsOf('request-count', w(10, 0, ' minute ')),
        limitsOf('tool-call-count', w(5, 2, 'HOUR')),
        { id: 'tokens', type: 'token-count', params: { limits: [w(100, 1, 'MONTH')], onExceed: 'allow' } },
    ]), [
        { id: 'requests', type: 'request-count', params: { limits: [w(10, 1, 'MINUTE')], onExceed: 'reject' } },
        { id: 'tool-calls', type: 'tool-call-count', params: { limits: [w(5, 2, 'HOUR')], onExceed: 'reject' } },
        { id: 'tokens', type: 'token-count', params: { limits: [w(100, 1, 'MONTH')], onExceed: 'allow', tokenType: 'total' } },
    ]);
});

test('custom conditions keep their id and get their params keys sorted', () => {
    assert.deepEqual(normalizeConditions([{ id: 'region', type: 'custom', params: { z: { b: 1, a: 2 }, a: [1] } }]),
        [{ id: 'region', type: 'custom', params: { a: [1], z: { a: 2, b: 1 } } }]);
    assert.deepEqual(normalizeConditions([{ id: 'empty', type: 'custom', params: {} }])[0].params, {});
});

test('the Platform API rules are enforced with its messages', () => {
    refused({}, 'conditions must be a list');
    refused(Array.from({ length: 21 }, (_, i) => ({ id: `c${i}`, type: 'custom', params: {} })), 'a plan can have at most 20 conditions');
    refused([{ params: {} }], 'conditions[0]: type is required');
    refused([{ type: 'quota', params: {} }], 'conditions[0]: type must be one of request-count, tool-call-count, token-count or custom');
    refused([{ id: 'calls', ...limitsOf('request-count', w(1, 1, 'MINUTE')) }], 'conditions[0]: the id of a request-count condition is requests');
    refused([limitsOf('tool-call-count', w(1, 1, 'MINUTE')), limitsOf('tool-call-count', w(1, 1, 'DAY'))],
        'conditions[1]: a plan can have only one tool-call-count condition');
    refused([{ type: 'request-count' }], 'conditions[0]: params with at least one limit is required');
    refused([limitsOf('request-count')], 'conditions[0]: params.limits needs at least one limit');
    refused([limitsOf('request-count', ...Array.from({ length: 11 }, (_, i) => w(1, i + 1, 'MINUTE')))],
        'conditions[0]: a condition can have at most 10 limits');
    refused([limitsOf('request-count', w(0, 1, 'MINUTE'))], 'conditions[0].params.limits[0]: count must be at least 1');
    refused([limitsOf('request-count', w(1.5, 1, 'MINUTE'))], 'conditions[0].params.limits[0]: count must be a whole number');
    refused([limitsOf('request-count', w(1, -1, 'MINUTE'))], 'conditions[0].params.limits[0]: timeAmount must be at least 1');
    refused([limitsOf('request-count', w(1, 1, 'WEEK'))], 'conditions[0].params.limits[0]: timeUnit must be one of MINUTE, HOUR, DAY or MONTH');
    refused([limitsOf('request-count', w(1, 24, 'HOUR'), w(2, 1, 'DAY'))],
        'conditions[0].params.limits[1]: the condition already has a limit over 1 DAY; each limit needs a window of its own length');
    refused([{ type: 'request-count', params: { limits: [w(1, 1, 'DAY')], onExceed: 'throttle' } }], 'conditions[0]: params.onExceed must be reject or allow');
    refused([{ type: 'token-count', params: { limits: [w(1, 1, 'DAY')], tokenType: 'cached' } }], 'conditions[0]: params.tokenType must be total, prompt or completion');
    refused([{ type: 'request-count', params: { limits: [w(1, 1, 'DAY')], tokenType: 'total' } }], 'conditions[0]: params.tokenType is for token-count conditions only');
    refused([{ type: 'request-count', params: { limits: [w(1, 1, 'DAY')], burst: 2 } }], 'conditions[0]: invalid params: unknown field "burst"');
    refused([{ id: 'Region', type: 'custom', params: {} }],
        'conditions[0]: a custom condition needs an id of lowercase letters, digits and hyphens, starting with a letter (at most 63 characters)');
    refused([{ id: 'tokens', type: 'custom', params: {} }], 'conditions[0]: tokens is the id of a built-in condition');
    refused([{ id: 'x', type: 'custom', params: {} }, { id: 'x', type: 'custom', params: {} }], 'conditions[1]: the plan already has a condition x');
    refused([{ id: 'x', type: 'custom' }], 'conditions[0]: params is required; send {} for a custom condition with no values');
    refused([{ id: 'x', type: 'custom', params: [1] }], 'conditions[0]: params must be a JSON object of key-value pairs');
    refused([{ id: 'x', type: 'custom', params: { ' ': 1 } }], 'conditions[0]: params has an empty key');
    refused([{ id: 'x', type: 'custom', params: { v: 'a'.repeat(4096) } }], 'conditions[0]: params must be at most 4096 bytes');
    refused([limitsOf('request-count', w(1e100, 1, 'MINUTE'))], 'conditions[0].params.limits[0]: count must be a whole number');
    refused([limitsOf('request-count', w(1, 2 ** 60, 'MINUTE'))], 'conditions[0].params.limits[0]: timeAmount must be a whole number');
    refused([{ type: 'request-count', params: { limits: [w(1, 1, 'DAY')], onExceed: false } }], 'conditions[0]: params.onExceed must be reject or allow');
    refused([{ type: 'token-count', params: { limits: [w(1, 1, 'DAY')], tokenType: 0 } }], 'conditions[0]: params.tokenType must be total, prompt or completion');
    refused([{ type: 'request-count', params: { limits: [w(1, 1, 'DAY')], tokenType: false } }], 'conditions[0]: params.tokenType is for token-count conditions only');
    refused([limitsOf('constructor', w(1, 1, 'DAY'))], 'conditions[0]: type must be one of request-count, tool-call-count, token-count or custom');
});

test('request windows come from the REQUEST_COUNT and EVENT_COUNT limits that have a window', () => {
    assert.deepEqual(requestWindowsFromLimits([
        { limit_type: 'REQUEST_COUNT', limit_count: '100', time_unit: 'MINUTE', time_amount: 1 },
        { limitType: 'REQUEST_COUNT', limitCount: 6000, timeUnit: 'hour' },
        { limitType: 'REQUEST_COUNT', limitCount: 50, timeUnit: 'MINUTE', timeAmount: 60 },
        { limitType: 'EVENT_COUNT', limitCount: 9, timeUnit: 'DAY' },
        { limitType: 'EVENT_COUNT', limitCount: 7, timeUnit: 'MINUTE' },
        { limitType: 'REQUEST_COUNT', limitCount: -1, timeUnit: 'DAY' },
        { limitType: 'REQUEST_COUNT', limitCount: 9, timeUnit: null },
        { limitType: 'BANDWIDTH', limitCount: 9, timeUnit: 'MONTH' },
    ]), [w(100, 1, 'MINUTE'), w(6000, 1, 'HOUR'), w(9, 1, 'DAY')]);
    assert.equal(requestCondition([]), null);
});

test('the request-count condition is replaced in place, added first, or removed', () => {
    const tools = { id: 'tool-calls', type: 'tool-call-count', params: {} };
    const custom = { id: 'x', type: 'custom', params: {} };
    const old = requestCondition([w(1, 1, 'DAY')]);
    const next = requestCondition([w(2, 1, 'DAY')], 'allow');
    assert.deepEqual(withRequestCondition([tools, old, custom], next), [tools, next, custom]);
    assert.deepEqual(withRequestCondition([tools], next), [next, tools]);
    assert.deepEqual(withRequestCondition([tools, old], null), [tools]);
    assert.deepEqual(requestLimitsFromConditions([tools, next]),
        [{ limitType: 'REQUEST_COUNT', limitCount: 2, timeUnit: 'DAY', timeAmount: 1 }]);
    assert.deepEqual(requestLimitsFromConditions([tools]), []);
});

test('plan cards show each window of the built-in conditions', () => {
    const conditions = normalizeConditions([
        limitsOf('request-count', w(10, 1, 'MINUTE')),
        limitsOf('tool-call-count', w(5, 2, 'HOUR')),
        { type: 'token-count', params: { limits: [w(100, 1, 'DAY')], tokenType: 'prompt' } },
        { id: 'region', type: 'custom', params: { zone: 'eu' } },
    ]);
    assert.deepEqual(displayLimits(conditions), [
        { limitType: 'REQUEST_COUNT', limitCount: 10, timeAmount: 1, timeUnit: 'MINUTE' },
        { limitType: 'TOOL_CALL_COUNT', limitCount: 5, timeAmount: 2, timeUnit: 'HOUR' },
        { limitType: 'PROMPT_TOKEN_COUNT', limitCount: 100, timeAmount: 1, timeUnit: 'DAY' },
    ]);
    assert.deepEqual(displayLimits(conditions, { events: true }).map((l) => l.limitType),
        ['EVENT_COUNT', 'TOOL_CALL_COUNT', 'PROMPT_TOKEN_COUNT']);
    assert.deepEqual(displayLimits(undefined), []);
});

test('the plans table shows one chip per condition, with every limit for its hover card', () => {
    const chips = conditionChips(normalizeConditions([
        { type: 'request-count', params: { limits: [w(344, 1, 'MINUTE'), w(43543, 2, 'HOUR')], onExceed: 'allow' } },
        { type: 'token-count', params: { limits: [w(1000000, 1, 'MONTH')], tokenType: 'prompt' } },
        { id: 'business-hours', type: 'custom', params: { peakStart: '08:00', peakLimit: 50 } },
        { id: 'empty', type: 'custom', params: {} },
    ]));
    assert.deepEqual(chips, [
        { label: 'Requests', first: '344 / min', more: 1, details: ['344 / min', '43,543 / 2 hr', 'Allowed past the limit'] },
        { label: 'Prompt tokens', first: '1,000,000 / mo', more: 0, details: ['1,000,000 / mo'] },
        { label: 'Custom', first: 'business-hours', more: 0, details: ['peakLimit = 50', 'peakStart = 08:00'], custom: true },
        { label: 'Custom', first: 'empty', more: 0, details: ['No values'], custom: true },
    ]);
    assert.deepEqual(conditionChips(undefined), []);
});

test('plan cards list every limit as subscribers read it, events on event-based APIs', () => {
    const conditions = normalizeConditions([
        { type: 'request-count', params: { limits: [w(1000, 1, 'HOUR'), w(5000, 2, 'DAY')] } },
        { type: 'tool-call-count', params: { limits: [w(500, 1, 'DAY')], onExceed: 'allow' } },
        { type: 'token-count', params: { limits: [w(1000000, 1, 'MONTH')], tokenType: 'completion' } },
        { id: 'region', type: 'custom', params: { zone: 'eu' } },
    ]);
    assert.deepEqual(limitLines(conditions), [
        { text: '1,000 requests / hour', allow: false },
        { text: '5,000 requests / 2 days', allow: false },
        { text: '500 tool calls / day', allow: true },
        { text: '1,000,000 completion tokens / month', allow: false },
    ]);
    assert.equal(limitLines(conditions, { events: true })[0].text, '1,000 events / hour');
    assert.deepEqual(limitLines(undefined), []);
});
