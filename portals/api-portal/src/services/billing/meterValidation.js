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
 * key of the number it adds up (typeKey). It has no conditions: it lists the properties its plan
 * prices may filter on (filterFields), each a name the administrator gives it and the event key it
 * reads, and each price narrows it with its own filters (planPricingValidation). Any operator works
 * on any property; the values a price gives must fit what the event field holds.
 */

const crypto = require('crypto');
const { TOKEN_EVENT_KEYS } = require('../../utils/meterLabels');

// What a meter counts: requests (REQUEST_COUNT), each counting one, or the tokens of AI API
// requests (TOKENS), adding up the number at the meter's typeKey.
const METER_TYPES = ['REQUEST_COUNT', 'TOKENS'];

// Our gateway's token counts (meterLabels.TOKEN_EVENT_KEYS) are what the settings page offers for a
// TOKENS meter's key; the first is a new TOKENS meter's key. They are suggestions only: any key is
// accepted.
const DEFAULT_TOKEN_EVENT_KEY = TOKEN_EVENT_KEYS[0].key;
const MAX_EVENT_KEY_LENGTH = 255;
// An event key has no whitespace or control characters.
const EVENT_KEY_REFUSED = /[\s\p{Cc}]/u;
// A property's name has no control characters.
const NAME_REFUSED = /\p{Cc}/u;
const MAX_NAME_LENGTH = 40;
// The fields a create or update request may carry.
const REQUEST_FIELDS = ['id', 'displayName', 'description', 'type', 'typeKey', 'filterFields'];

// How many values each filter operator takes: EQUALS, NOT_EQUALS, STARTS_WITH and the number
// comparisons one, IN and NOT_IN one or more (matching any of them).
const OPERATORS = {
    EQUALS: 'one', NOT_EQUALS: 'one', IN: 'many', NOT_IN: 'many', STARTS_WITH: 'one',
    GREATER_THAN: 'one', GREATER_THAN_OR_EQUAL: 'one', LESS_THAN: 'one', LESS_THAN_OR_EQUAL: 'one',
};
// The operators that compare a number; the others match text.
const COMPARE_OPERATORS = ['GREATER_THAN', 'GREATER_THAN_OR_EQUAL', 'LESS_THAN', 'LESS_THAN_OR_EQUAL'];
const MAX_FILTER_FIELDS = 10;
// The largest number a comparison takes.
const MAX_COMPARE_VALUE = 1000000000000;

const HANDLE_PATTERN = /^[a-z0-9-]{3,40}$/;
const MAX_VALUES = 20;
const MAX_VALUE_LENGTH = 255;

class MeterValidationError extends Error {}

/** A handle derived from the display name: "Successful calls" -> "successful-calls". */
function handleFromName(displayName) {
    const slug = String(displayName).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40)
        .replace(/-+$/, '');
    return slug.length >= 3 ? slug : `meter-${crypto.randomBytes(3).toString('hex')}`;
}

/** A property's name as written, trimmed, or null when it is not 1 to 40 characters. */
function propertyName(raw) {
    if (typeof raw !== 'string') return null;
    const name = raw.trim();
    return name && [...name].length <= MAX_NAME_LENGTH && !NAME_REFUSED.test(name) ? name : null;
}

/** Whether two property names are the same, ignoring case. */
function sameName(a, b) {
    return a.toLowerCase() === b.toLowerCase();
}

function textValue(name, raw) {
    if (typeof raw !== 'string' && typeof raw !== 'number') {
        throw new MeterValidationError(`Filter values for ${name} must be strings`);
    }
    const value = String(raw).trim();
    if (!value || value.length > MAX_VALUE_LENGTH) {
        throw new MeterValidationError(`Filter values for ${name} must be 1 to ${MAX_VALUE_LENGTH} characters`);
    }
    return value;
}

/** A comparison's value: one whole number from 0 to MAX_COMPARE_VALUE, as written in digits. */
function compareValue(name, raw) {
    const text = typeof raw === 'number' ? String(raw) : typeof raw === 'string' ? raw.trim() : '';
    if (!/^\d+$/.test(text) || Number(text) > MAX_COMPARE_VALUE) {
        throw new MeterValidationError(`A comparison on ${name} takes a whole number from 0 to ${MAX_COMPARE_VALUE}`);
    }
    return String(Number(text));
}

/**
 * The checked values of a filter on the property `name` with the operator: one value for EQUALS,
 * NOT_EQUALS, STARTS_WITH and the comparisons, 1 to MAX_VALUES for IN and NOT_IN; repeated values are
 * dropped, the order kept. A comparison's value is a whole number.
 */
function normalizeValues(name, operator, rawValues) {
    const count = Array.isArray(rawValues) ? rawValues.length : 0;
    if (OPERATORS[operator] === 'one') {
        if (count !== 1) throw new MeterValidationError(`Operator ${operator} takes exactly one value`);
    } else if (count === 0 || count > MAX_VALUES) {
        throw new MeterValidationError(`A filter on ${name} needs 1 to ${MAX_VALUES} values`);
    }
    if (COMPARE_OPERATORS.includes(operator)) return [compareValue(name, rawValues[0])];
    return [...new Set(rawValues.map((v) => textValue(name, v)))];
}

/** An event key as written, trimmed, or null when it is not 1 to 255 characters with no spaces. */
function eventKey(raw) {
    if (typeof raw !== 'string') return null;
    const key = raw.trim();
    return key && key.length <= MAX_EVENT_KEY_LENGTH && !EVENT_KEY_REFUSED.test(key) ? key : null;
}

/**
 * The properties a meter's prices may filter on, from [{ field, eventKey }]: at most
 * MAX_FILTER_FIELDS, each a name and the event key it reads. Names are unique ignoring case, and keys
 * are unique.
 */
function validateFilterFields(raw) {
    if (raw === undefined || raw === null) return [];
    if (!Array.isArray(raw)) throw new MeterValidationError('filterFields must be an array');
    if (raw.length > MAX_FILTER_FIELDS) throw new MeterValidationError(`A meter has at most ${MAX_FILTER_FIELDS} filter properties`);
    const fields = [];
    for (const entry of raw) {
        if (!entry || typeof entry !== 'object' || Array.isArray(entry)
            || Object.keys(entry).some((k) => k !== 'field' && k !== 'eventKey')) {
            throw new MeterValidationError('Each entry of filterFields has only a field and an eventKey');
        }
        const name = propertyName(entry.field);
        if (!name) throw new MeterValidationError(`A filter property's name must be 1 to ${MAX_NAME_LENGTH} characters`);
        if (fields.some((f) => sameName(f.field, name))) {
            throw new MeterValidationError(`The meter has the property ${name} more than once`);
        }
        const key = eventKey(entry.eventKey);
        if (!key) {
            throw new MeterValidationError(
                `The event key of ${name} must be 1 to ${MAX_EVENT_KEY_LENGTH} characters with no spaces`);
        }
        const duplicate = fields.find((f) => f.eventKey === key);
        if (duplicate) throw new MeterValidationError(`${name} and ${duplicate.field} read the same event key ${key}`);
        fields.push({ field: name, eventKey: key });
    }
    return fields;
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
    const filterFields = validateFilterFields(body.filterFields);
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
    MAX_COMPARE_VALUE,
    MAX_NAME_LENGTH,
    OPERATORS,
    COMPARE_OPERATORS,
    MAX_VALUES,
    MeterValidationError,
    validateMeter,
    normalizeValues,
    propertyName,
    sameName,
    handleFromName,
};
