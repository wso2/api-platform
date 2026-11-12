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
    normalizeConditions, windowsFromLimits, limitCondition, withCondition, conditionsFromLimits,
    legacyLimitsFromConditions, displayLimits, conditionChips, limitLines,
    conditionsFromStoredLimits, storedLimitsFromConditions,
} = require('./subscriptionPlanConditions');
const { ValidationError } = require('./errors/customErrors');
const constants = require('./constants');

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
        { id: 'tool-calls', type: 'tool-call-count', params: { limits: [w(50, 0, 'hour'), w(500, 1, 'DAY')], onExceed: 'allow' } },
        { id: 'tokens', type: 'token-count', params: { limits: [w(100, 1, 'MONTH')], onExceed: 'allow' } },
    ]), [
        { id: 'requests', type: 'request-count', params: { limits: [w(10, 1, 'MINUTE')], onExceed: 'reject' } },
        { id: 'tool-calls', type: 'tool-call-count', params: { limits: [w(50, 1, 'HOUR'), w(500, 1, 'DAY')], onExceed: 'allow' } },
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
    // event-count is not a condition type.
    refused([limitsOf('event-count', w(1, 1, 'MINUTE'))], 'conditions[0]: type must be one of request-count, tool-call-count, token-count or custom');
    refused([limitsOf('tool-call-count', w(1, 60, 'MINUTE'), w(2, 1, 'HOUR'))],
        'conditions[0].params.limits[1]: the condition already has a limit over 1 HOUR; each limit needs a window of its own length');
    refused([{ id: 'tool-calls', type: 'custom', params: {} }], 'conditions[0]: tool-calls is the id of a built-in condition');
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

test('request-count windows come from the REQUEST_COUNT limits; EVENT_COUNT limits have no condition', () => {
    const limits = [
        { limit_type: 'REQUEST_COUNT', limit_count: '100', time_unit: 'MINUTE', time_amount: 1 },
        { limitType: 'REQUEST_COUNT', limitCount: 6000, timeUnit: 'hour' },
        { limitType: 'REQUEST_COUNT', limitCount: 50, timeUnit: 'MINUTE', timeAmount: 60 },
        { limitType: 'EVENT_COUNT', limitCount: 9, timeUnit: 'DAY' },
        { limitType: 'EVENT_COUNT', limitCount: 7, timeUnit: 'MINUTE', timeAmount: 1440 },
        { limitType: 'EVENT_COUNT', limitCount: -1, timeUnit: 'HOUR' },
        { limitType: 'REQUEST_COUNT', limitCount: -1, timeUnit: 'DAY' },
        { limitType: 'REQUEST_COUNT', limitCount: 9, timeUnit: 'WEEK' },
        { limitType: 'BANDWIDTH', limitCount: 9, timeUnit: 'MONTH' },
    ];
    assert.deepEqual(windowsFromLimits(limits), [w(100, 1, 'MINUTE'), w(6000, 1, 'HOUR')]);
    assert.deepEqual(windowsFromLimits(limits, 'request-count'), [w(100, 1, 'MINUTE'), w(6000, 1, 'HOUR')]);
    assert.equal(limitCondition('request-count', []), null);
    // A stored limit with no unit came from the YAML shorthand: it is per minute, not dropped.
    assert.deepEqual(windowsFromLimits([{ limit_type: 'REQUEST_COUNT', limit_count: 10000, time_unit: null, time_amount: 1 }]),
        [w(10000, 1, 'MINUTE')]);
    // A limit without a type is a request limit.
    assert.deepEqual(windowsFromLimits([{ limitCount: 5, timeUnit: 'DAY' }]), [w(5, 1, 'DAY')]);
});

test('a condition is replaced in place, added first, or removed', () => {
    const tools = { id: 'tool-calls', type: 'tool-call-count', params: {} };
    const custom = { id: 'x', type: 'custom', params: {} };
    const old = limitCondition('request-count', [w(1, 1, 'DAY')]);
    const next = limitCondition('request-count', [w(2, 1, 'DAY')], 'allow');
    assert.deepEqual(withCondition([tools, old, custom], 'request-count', next), [tools, next, custom]);
    assert.deepEqual(withCondition([tools], 'request-count', next), [next, tools]);
    assert.deepEqual(withCondition([tools, old], 'request-count', null), [tools]);
    assert.deepEqual(next, { id: 'requests', type: 'request-count', params: { limits: [w(2, 1, 'DAY')], onExceed: 'allow' } });
});

test('limits sent alone set the request-count condition, and keep the others', () => {
    const tools = { id: 'tool-calls', type: 'tool-call-count', params: {} };
    const custom = { id: 'x', type: 'custom', params: {} };
    const requests = limitCondition('request-count', [w(1, 1, 'DAY')], 'allow');
    const both = [
        { limitType: 'EVENT_COUNT', limitCount: 20, timeUnit: 'HOUR' },
        { limitType: 'REQUEST_COUNT', limitCount: 10, timeUnit: 'MINUTE' },
    ];
    // It keeps its place and its onExceed; the EVENT_COUNT limit has no condition.
    assert.deepEqual(conditionsFromLimits([tools, custom, requests], both), [
        tools, custom, limitCondition('request-count', [w(10, 1, 'MINUTE')], 'allow'),
    ]);
    // A new one goes first.
    assert.deepEqual(conditionsFromLimits([tools], both), [limitCondition('request-count', [w(10, 1, 'MINUTE')]), tools]);
    // With no usable request limits, it is removed.
    assert.deepEqual(conditionsFromLimits([requests, tools], [both[0]]), [tools]);
    assert.deepEqual(conditionsFromLimits([requests, tools], []), [tools]);
    // An AsyncUnlimited plan saved before conditions: an unlimited event limit is no condition.
    assert.deepEqual(conditionsFromLimits([], [{ limitType: 'EVENT_COUNT', timeUnit: null, timeAmount: 1, limitCount: -1 }]), []);
});

test('the request-count windows are read back as REQUEST_COUNT limits', () => {
    const tools = { id: 'tool-calls', type: 'tool-call-count', params: { limits: [w(1, 1, 'DAY')] } };
    assert.deepEqual(legacyLimitsFromConditions([
        tools,
        limitCondition('request-count', [w(2, 1, 'DAY'), w(30, 1, 'MONTH')], 'allow'),
    ]), [
        { limitType: 'REQUEST_COUNT', limitCount: 2, timeUnit: 'DAY', timeAmount: 1 },
        { limitType: 'REQUEST_COUNT', limitCount: 30, timeUnit: 'MONTH', timeAmount: 1 },
    ]);
    assert.deepEqual(legacyLimitsFromConditions([tools]), []);
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
    assert.deepEqual(displayLimits(undefined), []);
});

test('the plans table shows one chip per condition, with every limit for its hover card', () => {
    const chips = conditionChips(normalizeConditions([
        { type: 'request-count', params: { limits: [w(344, 1, 'MINUTE'), w(43543, 2, 'HOUR')], onExceed: 'allow' } },
        limitsOf('tool-call-count', w(9000, 1, 'DAY')),
        { type: 'token-count', params: { limits: [w(1000000, 1, 'MONTH')], tokenType: 'prompt' } },
        { id: 'business-hours', type: 'custom', params: { peakStart: '08:00', peakLimit: 50 } },
        { id: 'empty', type: 'custom', params: {} },
    ]));
    assert.deepEqual(chips, [
        { label: 'Requests', first: '344 / min', more: 1, details: ['344 / min', '43,543 / 2 hr', 'Allowed past the limit'] },
        { label: 'Tool calls', first: '9,000 / day', more: 0, details: ['9,000 / day'] },
        { label: 'Prompt tokens', first: '1,000,000 / mo', more: 0, details: ['1,000,000 / mo'] },
        { label: 'Custom', first: 'business-hours', more: 0, details: ['peakLimit = 50', 'peakStart = 08:00'], custom: true },
        { label: 'Custom', first: 'empty', more: 0, details: ['No values'], custom: true },
    ]);
    assert.deepEqual(conditionChips(undefined), []);
});

test('plan cards list every limit as subscribers read it', () => {
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
    assert.deepEqual(limitLines(undefined), []);
});

test('the default plans are valid conditions, stored as written', () => {
    const plans = Object.fromEntries(constants.DEFAULT_SUBSCRIPTION_PLANS.map((p) => [p.handle, p.conditions]));
    for (const [handle, conditions] of Object.entries(plans)) {
        assert.deepEqual(normalizeConditions(conditions), conditions, handle);
    }
    assert.deepEqual(plans.Bronze, [limitCondition('request-count', [w(1000, 1, 'MINUTE')])]);
    assert.deepEqual(plans.Silver, [limitCondition('request-count', [w(2000, 1, 'MINUTE')])]);
    assert.deepEqual(plans.Gold, [limitCondition('request-count', [w(5000, 1, 'MINUTE')])]);
    assert.deepEqual(plans.Unlimited, []);
    assert.deepEqual(plans.AsyncUnlimited, []);
});

test('stored limits of earlier releases: TOTAL_TOKEN_COUNT is a total token-count condition, EVENT_COUNT and BANDWIDTH none', () => {
    const rows = [
        { limit_type: 'REQUEST_COUNT', limit_count: 100, time_unit: 'MINUTE', time_amount: 1 },
        { limit_type: 'BANDWIDTH', limit_count: 1024, time_unit: 'MONTH', time_amount: 1 },
        { limit_type: 'EVENT_COUNT', limit_count: 50, time_unit: 'MINUTE', time_amount: 1 },
        { limit_type: 'TOTAL_TOKEN_COUNT', limit_count: 100000, time_unit: 'DAY', time_amount: 1 },
        { limit_type: 'TOTAL_TOKEN_COUNT', limit_count: -1, time_unit: null, time_amount: 1 },
    ];
    const tokens = { id: 'tokens', type: 'token-count', params: { limits: [w(100000, 1, 'DAY')], onExceed: 'reject', tokenType: 'total' } };
    assert.deepEqual(conditionsFromStoredLimits(rows), [limitCondition('request-count', [w(100, 1, 'MINUTE')]), tokens]);
    assert.deepEqual(normalizeConditions([tokens]), [tokens]);
    assert.deepEqual(conditionsFromStoredLimits([rows[1], rows[2]]), []);
    // Limits sent alone by older clients still set only the request-count condition.
    assert.deepEqual(conditionsFromLimits([], rows), [limitCondition('request-count', [w(100, 1, 'MINUTE')])]);
});

test('a database not upgraded yet keeps request and total token limits, nothing else', () => {
    const tokens = (tokenType) => ({ id: 'tokens', type: 'token-count', params: { limits: [w(5, 1, 'DAY')], onExceed: 'reject', tokenType } });
    assert.deepEqual(storedLimitsFromConditions([limitCondition('request-count', [w(3, 1, 'HOUR')]), tokens('total')]), [
        { limitType: 'REQUEST_COUNT', limitCount: 3, timeUnit: 'HOUR', timeAmount: 1 },
        { limitType: 'TOTAL_TOKEN_COUNT', limitCount: 5, timeUnit: 'DAY', timeAmount: 1 },
    ]);
    assert.equal(storedLimitsFromConditions([tokens('prompt')]), null);
    assert.equal(storedLimitsFromConditions([limitCondition('request-count', [w(1, 1, 'DAY')], 'allow')]), null);
    assert.equal(storedLimitsFromConditions([{ id: 'tool-calls', type: 'tool-call-count', params: { limits: [w(1, 1, 'DAY')] } }]), null);
});
