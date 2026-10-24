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
 * Validation of meter requests, and of the filters plan prices put on what a meter counts. Kept
 * free of database and configuration dependencies so the rules can be tested on their own.
 *
 * A meter says only what is counted: its type, and for a type that adds up a number, the event
 * key of the number it adds up (typeKey). It has no conditions: it lists the fields its plan
 * prices may filter on (filterFields), each with the event key it reads, and each price narrows it
 * with its own filters (planPricingValidation). Event keys are stored on the meter, so a gateway
 * that names its analytics fields differently can be metered too; the keys of our gateway's events
 * are the defaults.
 */

const crypto = require('crypto');
const { FIELD_LABELS, TOKEN_EVENT_KEYS } = require('../../utils/meterLabels');

// What a meter counts: API requests (REQUEST_COUNT), each counting one, or the tokens of AI API
// requests (TOKENS), adding up the number at the meter's typeKey.
const METER_TYPES = ['REQUEST_COUNT', 'TOKENS'];

// Our gateway's token counts (meterLabels.TOKEN_EVENT_KEYS) are what the settings page offers for a
// Token row or field; the first is a new TOKENS meter's key. They are suggestions only: any key is
// accepted.
const DEFAULT_TOKEN_EVENT_KEY = TOKEN_EVENT_KEYS[0].key;
const MAX_EVENT_KEY_LENGTH = 255;
// An event key has no whitespace or control characters.
const EVENT_KEY_REFUSED = /[\s\p{Cc}]/u;
// The fields a create or update request may carry.
const REQUEST_FIELDS = ['id', 'displayName', 'description', 'type', 'typeKey', 'filterFields'];

// How many values each filter operator takes: EQUALS, NOT_EQUALS and STARTS_WITH one, IN and
// NOT_IN one or more (matching any of them), and the number comparisons of a Token condition one.
const OPERATORS = {
    EQUALS: 'one', NOT_EQUALS: 'one', IN: 'many', NOT_IN: 'many', STARTS_WITH: 'one',
    GREATER_THAN: 'one', GREATER_THAN_OR_EQUAL: 'one', LESS_THAN: 'one', LESS_THAN_OR_EQUAL: 'one',
};
const MATCH = ['EQUALS', 'NOT_EQUALS', 'IN', 'NOT_IN'];
const COMPARE = ['GREATER_THAN', 'GREATER_THAN_OR_EQUAL', 'LESS_THAN', 'LESS_THAN_OR_EQUAL', 'EQUALS'];

// The fields a meter may let its prices filter on, the operators each accepts, and the default event
// key each reads: its full path in our gateway's events. The API type is the kind of API the gateway
// served, such as RestApi, LlmProvider or Mcp, as written. The MCP method is the JSON-RPC method of a
// request to an MCP server, such as tools/call, and the tool is the name of the tool a tool call runs.
// A Token field reads a number, such as a token count, at a key it must be given (it has no
// default); a meter may list it once per key, and prices compare that number.
const FILTER_RULES = {
    HTTP_STATUS_CODE: { operators: MATCH, eventKey: 'response.status' },
    HTTP_METHOD: { operators: MATCH, eventKey: 'request.verb' },
    RESOURCE: { operators: ['EQUALS', 'NOT_EQUALS', 'STARTS_WITH'], eventKey: 'metadata.apiResourceTemplate' },
    API_TYPE: { operators: MATCH, eventKey: 'metadata.apiType' },
    MCP_METHOD: { operators: MATCH, eventKey: 'metadata.mcpAnalytics.jsonRpcMethod' },
    MCP_TOOL: { operators: MATCH, eventKey: 'metadata.mcpAnalytics.capabilityName' },
    TOKEN: { operators: COMPARE, eventKey: null },
};
// The MCP method of a tool call.
const MCP_TOOL_CALL = 'tools/call';
const FILTER_FIELDS = Object.keys(FILTER_RULES);
const MAX_FILTER_FIELDS = 10;
// The largest number a Token condition compares with.
const MAX_TOKEN_VALUE = 1000000000000;

const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'];
const HANDLE_PATTERN = /^[a-z0-9-]{3,40}$/;
// One status code, from 100 to 599. Classes of codes ("2xx") are not accepted.
const STATUS_PATTERN = /^[1-5]\d\d$/;
const MAX_VALUES = 20;
const MAX_VALUE_LENGTH = 255;

class MeterValidationError extends Error {}

/** A handle derived from the display name: "Successful calls" -> "successful-calls". */
function handleFromName(displayName) {
    const slug = String(displayName).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40)
        .replace(/-+$/, '');
    return slug.length >= 3 ? slug : `meter-${crypto.randomBytes(3).toString('hex')}`;
}

function normalizeValue(field, raw) {
    if (typeof raw !== 'string' && typeof raw !== 'number') {
        throw new MeterValidationError(`Filter values for ${field} must be strings`);
    }
    const value = String(raw).trim();
    if (!value || value.length > MAX_VALUE_LENGTH) {
        throw new MeterValidationError(`Filter values for ${field} must be 1 to ${MAX_VALUE_LENGTH} characters`);
    }
    switch (field) {
        case 'HTTP_STATUS_CODE':
            if (!STATUS_PATTERN.test(value)) {
                throw new MeterValidationError(`"${value}" is not a status code such as 200 or 404`);
            }
            return value;
        case 'HTTP_METHOD': {
            const method = value.toUpperCase();
            if (!HTTP_METHODS.includes(method)) {
                throw new MeterValidationError(`HTTP method must be one of ${HTTP_METHODS.join(', ')}`);
            }
            return method;
        }
        case 'RESOURCE':
            if (!value.startsWith('/')) {
                throw new MeterValidationError('A resource must start with /');
            }
            return value;
        default:
            return value;
    }
}

/**
 * A STARTS_WITH path as stored: without a trailing "/", so "/orders/" is "/orders"; "/" stays "/".
 * STARTS_WITH matches whole path segments (underPath).
 */
function normalizePath(path) {
    return path.replace(/\/+$/, '') || '/';
}

/**
 * Whether a resource is the path or under it, segment by segment: "/orders" covers "/orders" and
 * "/orders/{id}", not "/orders-archive"; "/" covers every resource. This is what STARTS_WITH counts,
 * as routers match a path prefix (Kubernetes Gateway API PathPrefix, Envoy path_separated_prefix).
 */
function underPath(resource, path) {
    const base = normalizePath(path);
    return base === '/' || resource === base || resource.startsWith(`${base}/`);
}

/**
 * The checked values of a filter with the operator: one value for EQUALS, NOT_EQUALS and
 * STARTS_WITH, 1 to MAX_VALUES for IN and NOT_IN; repeated values are dropped, the order kept. A
 * STARTS_WITH path loses a trailing "/" (normalizePath).
 */
function normalizeValues(field, operator, rawValues) {
    const count = Array.isArray(rawValues) ? rawValues.length : 0;
    if (OPERATORS[operator] === 'one') {
        if (count !== 1) throw new MeterValidationError(`Operator ${operator} takes exactly one value`);
    } else if (count === 0 || count > MAX_VALUES) {
        throw new MeterValidationError(`A filter on ${field} needs 1 to ${MAX_VALUES} values`);
    }
    const values = rawValues.map((v) => normalizeValue(field, v));
    return [...new Set(operator === 'STARTS_WITH' ? values.map(normalizePath) : values)];
}

/** An event key as written, trimmed, or null when it is not 1 to 255 characters with no spaces. */
function eventKey(raw) {
    if (typeof raw !== 'string') return null;
    const key = raw.trim();
    return key && key.length <= MAX_EVENT_KEY_LENGTH && !EVENT_KEY_REFUSED.test(key) ? key : null;
}

/**
 * The fields a meter's prices may filter on, from [{ field, eventKey }]: at most MAX_FILTER_FIELDS,
 * each a known field with the event key it reads. A fixed field is listed once, and a key left out or
 * empty is its default. A Token field needs a key, listed once and not the meter's Token row
 * (`typeKey`, which prices can compare too).
 */
function validateFilterFields(raw, typeKey) {
    if (raw === undefined || raw === null) return [];
    if (!Array.isArray(raw)) throw new MeterValidationError('filterFields must be an array');
    if (raw.length > MAX_FILTER_FIELDS) throw new MeterValidationError(`A meter has at most ${MAX_FILTER_FIELDS} filter fields`);
    const fields = [];
    for (const entry of raw) {
        if (!entry || typeof entry !== 'object' || Array.isArray(entry)
            || Object.keys(entry).some((k) => k !== 'field' && k !== 'eventKey')) {
            throw new MeterValidationError('Each entry of filterFields has only a field and an eventKey');
        }
        const field = String(entry.field || '').toUpperCase();
        if (!FILTER_RULES[field]) throw new MeterValidationError(`filterFields: unknown field ${entry.field}`);
        const given = entry.eventKey === undefined || entry.eventKey === null
            || (typeof entry.eventKey === 'string' && entry.eventKey.trim() === '') ? null : entry.eventKey;
        if (field === 'TOKEN') {
            fields.push({ field, eventKey: tokenFieldKey(given, typeKey, fields) });
            continue;
        }
        if (fields.some((f) => f.field === field)) throw new MeterValidationError(`filterFields: ${field} is listed twice`);
        const key = given === null ? FILTER_RULES[field].eventKey : eventKey(given);
        if (!key) {
            throw new MeterValidationError(
                `The event key of ${FIELD_LABELS[field]} must be 1 to ${MAX_EVENT_KEY_LENGTH} characters with no spaces`);
        }
        fields.push({ field, eventKey: key });
    }
    return fields;
}

/** The key of a Token field: given, valid, not the Token row's and not listed before in `fields`. */
function tokenFieldKey(given, typeKey, fields) {
    if (given === null) throw new MeterValidationError('A Token field needs an eventKey');
    const key = eventKey(given);
    if (!key) {
        throw new MeterValidationError(`The event key of ${FIELD_LABELS.TOKEN} must be 1 to ${MAX_EVENT_KEY_LENGTH} characters with no spaces`);
    }
    if (key === typeKey) throw new MeterValidationError(`The key ${key} is already the meter's Token key`);
    if (fields.some((f) => f.field === 'TOKEN' && f.eventKey === key)) {
        throw new MeterValidationError(`The meter lists the Token key ${key} more than once`);
    }
    return key;
}

/** The event key a meter adds up: required for TOKENS, refused for REQUEST_COUNT. */
function validateTypeKey(type, raw) {
    const given = raw !== undefined && raw !== null;
    if (type !== 'TOKENS') {
        if (given) throw new MeterValidationError('typeKey applies only to a TOKENS meter');
        return null;
    }
    if (!given) throw new MeterValidationError('A TOKENS meter needs a typeKey');
    const key = eventKey(raw);
    if (!key) {
        throw new MeterValidationError(`typeKey must be 1 to ${MAX_EVENT_KEY_LENGTH} characters with no spaces`);
    }
    return key;
}

/**
 * Validates a create or update request body and returns the normalized meter. On create the
 * handle comes from `id`, or is derived from the display name when `id` is absent.
 * Throws MeterValidationError with a message fit for the caller.
 */
function validateMeter(body, { create }) {
    if (!body || typeof body !== 'object' || Array.isArray(body)) {
        throw new MeterValidationError('Request body is missing or invalid');
    }
    const displayName = typeof body.displayName === 'string' ? body.displayName.trim() : '';
    if (!displayName || displayName.length > 255) {
        throw new MeterValidationError('displayName is required and must be at most 255 characters');
    }
    if (body.description !== undefined && body.description !== null &&
        (typeof body.description !== 'string' || body.description.length > 1023)) {
        throw new MeterValidationError('description must be a string of at most 1023 characters');
    }
    if (body.conditions !== undefined) {
        throw new MeterValidationError('A meter has no conditions; each plan price gives its own filters');
    }
    const unknown = Object.keys(body).find((k) => !REQUEST_FIELDS.includes(k));
    if (unknown !== undefined) throw new MeterValidationError(`${unknown} is not a meter field`);
    const type = String(body.type || '').toUpperCase();
    if (!METER_TYPES.includes(type)) {
        throw new MeterValidationError(`type must be one of ${METER_TYPES.join(', ')}`);
    }
    const typeKey = validateTypeKey(type, body.typeKey);
    const filterFields = validateFilterFields(body.filterFields, typeKey);
    const meter = { displayName, description: body.description || null, type, typeKey, filterFields };
    if (create) {
        meter.handle = body.id ?? handleFromName(displayName);
        meter.handleGiven = body.id !== undefined && body.id !== null;
        if (typeof meter.handle !== 'string' || !HANDLE_PATTERN.test(meter.handle)) {
            throw new MeterValidationError('id must be 3 to 40 lowercase letters, digits or hyphens');
        }
    }
    return meter;
}

module.exports = {
    METER_TYPES,
    TOKEN_EVENT_KEYS,
    DEFAULT_TOKEN_EVENT_KEY,
    MAX_FILTER_FIELDS,
    MAX_TOKEN_VALUE,
    HTTP_METHODS,
    MCP_TOOL_CALL,
    OPERATORS,
    FILTER_RULES,
    FILTER_FIELDS,
    MAX_VALUES,
    MeterValidationError,
    validateMeter,
    normalizeValues,
    normalizePath,
    underPath,
    handleFromName,
};
