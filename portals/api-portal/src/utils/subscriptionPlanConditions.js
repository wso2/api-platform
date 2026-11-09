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
 * Subscription plan conditions, validated by the same rules as the Platform API. A built-in
 * condition (request-count, tool-call-count, token-count) has the id its type fixes and its
 * windows as params; a custom condition has an id of its own and key-value pairs as params.
 */

const { ValidationError } = require('./errors/customErrors');

const TYPE = Object.freeze({
    REQUEST_COUNT: 'request-count',
    TOOL_CALL_COUNT: 'tool-call-count',
    TOKEN_COUNT: 'token-count',
    CUSTOM: 'custom',
});

const BUILT_IN_IDS = Object.freeze({
    [TYPE.REQUEST_COUNT]: 'requests',
    [TYPE.TOOL_CALL_COUNT]: 'tool-calls',
    [TYPE.TOKEN_COUNT]: 'tokens',
});

const MAX_CONDITIONS = 20;
const MAX_WINDOWS = 10;
const MAX_CUSTOM_PARAMS_BYTES = 4096;
const TIME_UNITS = ['MINUTE', 'HOUR', 'DAY', 'MONTH'];
const ON_EXCEED = ['reject', 'allow'];
const TOKEN_TYPES = ['total', 'prompt', 'completion'];
const CUSTOM_ID_PATTERN = /^[a-z][a-z0-9-]{0,62}$/;
const WINDOW_MINUTES = { MINUTE: 1, HOUR: 60, DAY: 24 * 60 };
const REQUEST_LIMIT_TYPES = ['REQUEST_COUNT', 'EVENT_COUNT'];

const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const isBuiltInId = (id) => Object.values(BUILT_IN_IDS).includes(id);

/** Names a window by its length, so 60 MINUTE and 1 HOUR match. Months match only months. */
function windowKey(amount, unit) {
    return unit in WINDOW_MINUTES ? `m${WINDOW_MINUTES[unit] * amount}` : `M${amount}`;
}

function checkKnownKeys(field, obj, allowed) {
    const unknown = Object.keys(obj).find((k) => !allowed.includes(k));
    if (unknown !== undefined) {
        throw new ValidationError(`${field}: invalid params: unknown field "${unknown}"`);
    }
}

function normalizeLimitParams(field, type, params) {
    if (params === undefined || params === null) {
        throw new ValidationError(`${field}: params with at least one limit is required`);
    }
    if (!isObject(params)) {
        throw new ValidationError(`${field}: invalid params: params must be an object`);
    }
    checkKnownKeys(field, params, ['limits', 'onExceed', 'tokenType']);
    const { limits, onExceed, tokenType } = params;
    if (limits !== undefined && limits !== null && !Array.isArray(limits)) {
        throw new ValidationError(`${field}: invalid params: limits must be a list`);
    }
    if (!limits || limits.length === 0) {
        throw new ValidationError(`${field}: params.limits needs at least one limit`);
    }
    if (limits.length > MAX_WINDOWS) {
        throw new ValidationError(`${field}: a condition can have at most ${MAX_WINDOWS} limits`);
    }
    const seen = new Set();
    const windows = limits.map((w, j) => {
        const limitField = `${field}.params.limits[${j}]`;
        if (!isObject(w)) throw new ValidationError(`${limitField}: invalid params: a limit must be an object`);
        checkKnownKeys(field, w, ['count', 'timeAmount', 'timeUnit']);
        const count = w.count ?? 0;
        if (!Number.isSafeInteger(count)) throw new ValidationError(`${limitField}: count must be a whole number`);
        if (count < 1) throw new ValidationError(`${limitField}: count must be at least 1`);
        let timeAmount = w.timeAmount ?? 0;
        if (!Number.isSafeInteger(timeAmount)) throw new ValidationError(`${limitField}: timeAmount must be a whole number`);
        if (timeAmount === 0) timeAmount = 1;
        else if (timeAmount < 0) throw new ValidationError(`${limitField}: timeAmount must be at least 1`);
        if (w.timeUnit !== undefined && w.timeUnit !== null && typeof w.timeUnit !== 'string') {
            throw new ValidationError(`${limitField}: timeUnit must be one of MINUTE, HOUR, DAY or MONTH`);
        }
        const timeUnit = String(w.timeUnit ?? '').trim().toUpperCase();
        if (!TIME_UNITS.includes(timeUnit)) {
            throw new ValidationError(`${limitField}: timeUnit must be one of MINUTE, HOUR, DAY or MONTH`);
        }
        const key = windowKey(timeAmount, timeUnit);
        if (seen.has(key)) {
            throw new ValidationError(`${limitField}: the condition already has a limit over ${timeAmount} ${timeUnit}; each limit needs a window of its own length`);
        }
        seen.add(key);
        return { count, timeAmount, timeUnit };
    });
    // A value that is given is checked as it is: false or 0 is refused, not taken as the default.
    const out = { limits: windows, onExceed: onExceed ?? 'reject' };
    if (!ON_EXCEED.includes(out.onExceed)) {
        throw new ValidationError(`${field}: params.onExceed must be reject or allow`);
    }
    if (type === TYPE.TOKEN_COUNT) {
        out.tokenType = tokenType ?? 'total';
        if (!TOKEN_TYPES.includes(out.tokenType)) {
            throw new ValidationError(`${field}: params.tokenType must be total, prompt or completion`);
        }
    } else if (tokenType !== undefined && tokenType !== null) {
        throw new ValidationError(`${field}: params.tokenType is for token-count conditions only`);
    }
    return out;
}

/** Sorts object keys at every level, so equal params are written as equal JSON. */
function sortKeys(value) {
    if (Array.isArray(value)) return value.map(sortKeys);
    if (!isObject(value)) return value;
    return Object.fromEntries(Object.keys(value).sort().map((k) => [k, sortKeys(value[k])]));
}

function normalizeCustomParams(field, params) {
    if (params === undefined || params === null) {
        throw new ValidationError(`${field}: params is required; send {} for a custom condition with no values`);
    }
    if (!isObject(params)) {
        throw new ValidationError(`${field}: params must be a JSON object of key-value pairs`);
    }
    if (Object.keys(params).some((k) => k.trim() === '')) {
        throw new ValidationError(`${field}: params has an empty key`);
    }
    const out = sortKeys(params);
    if (Buffer.byteLength(JSON.stringify(out), 'utf8') > MAX_CUSTOM_PARAMS_BYTES) {
        throw new ValidationError(`${field}: params must be at most ${MAX_CUSTOM_PARAMS_BYTES} bytes`);
    }
    return out;
}

/**
 * Validates a plan's conditions and returns them with the defaults filled in. Throws a
 * ValidationError with the Platform API's message for the first problem found.
 */
function normalizeConditions(conditions) {
    if (!Array.isArray(conditions)) {
        throw new ValidationError('conditions must be a list');
    }
    if (conditions.length > MAX_CONDITIONS) {
        throw new ValidationError(`a plan can have at most ${MAX_CONDITIONS} conditions`);
    }
    const seenIds = new Set();
    return conditions.map((c, i) => {
        const field = `conditions[${i}]`;
        if (!isObject(c)) throw new ValidationError(`${field}: a condition must be an object`);
        const type = typeof c.type === 'string' ? c.type.trim() : '';
        let id = typeof c.id === 'string' ? c.id.trim() : '';
        let params;
        if (Object.prototype.hasOwnProperty.call(BUILT_IN_IDS, type)) {
            const fixedId = BUILT_IN_IDS[type];
            if (id !== '' && id !== fixedId) {
                throw new ValidationError(`${field}: the id of a ${type} condition is ${fixedId}`);
            }
            if (seenIds.has(fixedId)) {
                throw new ValidationError(`${field}: a plan can have only one ${type} condition`);
            }
            id = fixedId;
            params = normalizeLimitParams(field, type, c.params);
        } else if (type === TYPE.CUSTOM) {
            if (!CUSTOM_ID_PATTERN.test(id)) {
                throw new ValidationError(`${field}: a custom condition needs an id of lowercase letters, digits and hyphens, starting with a letter (at most 63 characters)`);
            }
            if (isBuiltInId(id)) {
                throw new ValidationError(`${field}: ${id} is the id of a built-in condition`);
            }
            if (seenIds.has(id)) {
                throw new ValidationError(`${field}: the plan already has a condition ${id}`);
            }
            params = normalizeCustomParams(field, c.params);
        } else if (type === '') {
            throw new ValidationError(`${field}: type is required`);
        } else {
            throw new ValidationError(`${field}: type must be one of request-count, tool-call-count, token-count or custom`);
        }
        seenIds.add(id);
        return { id, type, params };
    });
}

/**
 * The request-count windows of a plan's REQUEST_COUNT and EVENT_COUNT limits (DAO rows or request
 * limits): the subscription rate limit counts events as requests. Unit-less limits are per minute.
 * Unlimited, unsupported and repeated-length limits are left out, and at most ten are kept.
 */
function requestWindowsFromLimits(limits) {
    const windows = [];
    const seen = new Set();
    for (const l of limits || []) {
        const type = String(l.limit_type ?? l.limitType ?? 'REQUEST_COUNT').toUpperCase();
        const count = Number(l.limit_count ?? l.limitCount);
        if (!REQUEST_LIMIT_TYPES.includes(type) || !Number.isInteger(count) || count < 1) continue;
        // A stored limit with no unit came from the YAML shorthand (requestCount), which is per minute.
        const unit = String(l.time_unit ?? l.timeUnit ?? '').trim().toUpperCase() || 'MINUTE';
        if (!TIME_UNITS.includes(unit)) continue;
        const amount = Math.max(1, Number(l.time_amount ?? l.timeAmount) || 1);
        const key = windowKey(amount, unit);
        if (seen.has(key) || windows.length === MAX_WINDOWS) continue;
        seen.add(key);
        windows.push({ count, timeAmount: amount, timeUnit: unit });
    }
    return windows;
}

/** The request-count condition of the windows, or null when there are none (unlimited). */
function requestCondition(windows, onExceed = 'reject') {
    if (!windows || windows.length === 0) return null;
    return { id: BUILT_IN_IDS[TYPE.REQUEST_COUNT], type: TYPE.REQUEST_COUNT, params: { limits: windows, onExceed } };
}

/** Replaces the request-count condition with `request`, or removes it when null; keeps the others. */
function withRequestCondition(conditions, request) {
    const index = conditions.findIndex((c) => c.type === TYPE.REQUEST_COUNT);
    const others = conditions.filter((c) => c.type !== TYPE.REQUEST_COUNT);
    if (!request) return others;
    if (index === -1) return [request, ...others];
    others.splice(index, 0, request);
    return others;
}

/** The REQUEST_COUNT limits that mirror a plan's request-count condition. */
function requestLimitsFromConditions(conditions) {
    const request = conditions.find((c) => c.type === TYPE.REQUEST_COUNT);
    if (!request) return [];
    return request.params.limits.map((w) => ({
        limitType: 'REQUEST_COUNT',
        limitCount: w.count,
        timeUnit: w.timeUnit,
        timeAmount: w.timeAmount,
    }));
}

const DISPLAY_LIMIT_TYPES = Object.freeze({
    [TYPE.REQUEST_COUNT]: () => 'REQUEST_COUNT',
    [TYPE.TOOL_CALL_COUNT]: () => 'TOOL_CALL_COUNT',
    [TYPE.TOKEN_COUNT]: (params) => ({ prompt: 'PROMPT_TOKEN_COUNT', completion: 'COMPLETION_TOKEN_COUNT' }[params.tokenType] || 'TOTAL_TOKEN_COUNT'),
});

/**
 * The limits to show for a plan, in the {limitType, limitCount, timeAmount, timeUnit} shape the
 * plan cards render: each window of its built-in conditions. On an event-based API the
 * request-count condition counts events, and shows as such.
 */
function displayLimits(conditions, { events = false } = {}) {
    const out = [];
    for (const c of conditions || []) {
        const limitType = events && c.type === TYPE.REQUEST_COUNT ? () => 'EVENT_COUNT' : DISPLAY_LIMIT_TYPES[c.type];
        if (!limitType) continue;
        for (const w of c.params?.limits || []) {
            out.push({ limitType: limitType(c.params), limitCount: w.count, timeAmount: w.timeAmount, timeUnit: w.timeUnit });
        }
    }
    return out;
}

const CHIP_NAMES = { [TYPE.REQUEST_COUNT]: 'Requests', [TYPE.TOOL_CALL_COUNT]: 'Tool calls', [TYPE.TOKEN_COUNT]: 'Tokens' };
const TOKEN_NAMES = { total: 'Tokens', prompt: 'Prompt tokens', completion: 'Completion tokens' };
const SHORT_UNITS = { MINUTE: 'min', HOUR: 'hr', DAY: 'day', MONTH: 'mo' };

/**
 * One chip per condition for the plans table: its first limit and how many more it has, with every
 * limit (or a custom condition's values) for the chip's hover card.
 */
const conditionChips = (conditions) => (conditions || []).map((c) => {
    const params = c.params || {};
    if (c.type === TYPE.CUSTOM) {
        const values = Object.entries(params).map(([k, v]) => `${k} = ${typeof v === 'string' ? v : JSON.stringify(v)}`);
        return { label: 'Custom', first: c.id, more: 0, details: values.length ? values : ['No values'], custom: true };
    }
    const name = c.type === TYPE.TOKEN_COUNT ? TOKEN_NAMES[params.tokenType] || 'Tokens' : CHIP_NAMES[c.type] || c.type;
    const windows = (params.limits || []).map((w) =>
        `${Number(w.count).toLocaleString('en-US')} / ${w.timeAmount > 1 ? w.timeAmount + ' ' : ''}${SHORT_UNITS[w.timeUnit] || String(w.timeUnit).toLowerCase()}`);
    const details = windows.concat(params.onExceed === 'allow' ? ['Allowed past the limit'] : []);
    return { label: name, first: windows[0] || 'No limit', more: Math.max(windows.length - 1, 0), details };
});

const UNIT_WORDS = { MINUTE: 'minute', HOUR: 'hour', DAY: 'day', MONTH: 'month' };
const LINE_WHAT = { [TYPE.REQUEST_COUNT]: 'requests', [TYPE.TOOL_CALL_COUNT]: 'tool calls' };
const TOKEN_WHAT = { total: 'tokens', prompt: 'prompt tokens', completion: 'completion tokens' };

/**
 * One line per limit, as plan cards show them to subscribers ("1,000 requests / hour"), flagged when
 * the plan lets calls through past it. Request limits read as events on event-based APIs. Custom
 * conditions are for policies, not subscribers, so they have no line.
 */
const limitLines = (conditions, { events = false } = {}) => (conditions || []).flatMap((c) => {
    if (c.type === TYPE.CUSTOM) return [];
    const params = c.params || {};
    let what = c.type === TYPE.TOKEN_COUNT ? TOKEN_WHAT[params.tokenType] || 'tokens' : LINE_WHAT[c.type] || c.type;
    if (c.type === TYPE.REQUEST_COUNT && events) what = 'events';
    return (params.limits || []).map((w) => {
        const word = UNIT_WORDS[w.timeUnit] || String(w.timeUnit).toLowerCase();
        const window = w.timeAmount > 1 ? `${w.timeAmount} ${word}s` : word;
        return { text: `${Number(w.count).toLocaleString('en-US')} ${what} / ${window}`, allow: params.onExceed === 'allow' };
    });
});

module.exports = {
    conditionChips,
    limitLines,
    displayLimits,
    TYPE,
    BUILT_IN_IDS,
    normalizeConditions,
    requestWindowsFromLimits,
    requestCondition,
    withRequestCondition,
    requestLimitsFromConditions,
};
