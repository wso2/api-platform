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
 * Turns the OpenAPI request checks (express-openapi-validator) into plain sentences that name the
 * field: "meterId of price 1 is required", "billingPeriod must be one of MONTH, YEAR", "color is not
 * a known field".
 *
 * The validator reports every rule the request broke, for every shape the operation accepts. For
 * example, a body that may be one plan or a list of plans, sent as one plan, is also told it "must be
 * array" and must "match exactly one schema in oneOf". Those errors only say the request is not the
 * other shape, so they are left out: only the errors deepest in the request are kept, since those
 * come from the shape that was sent.
 */

// Where in the request an error is: the first segment of its path, left out of the field's name.
const LOCATIONS = new Set(['body', 'query', 'path', 'params', 'headers', 'cookies']);

/** "prices" -> "price", "tiers" -> "tier", "values" -> "value"; any other name stays as it is. */
function singular(name) {
    return name.length > 1 && name.endsWith('s') ? name.slice(0, -1) : name;
}

/**
 * The field an error path points to, in words. A list entry is named after its list and counted
 * from 1:
 *   - "/body/pricing/billingPeriod" is "billingPeriod";
 *   - "/body/pricing/prices/0/unitQuantity" is "unitQuantity of price 1";
 *   - "/body/pricing/prices/0/filters/1/values/0" is "value 1 of filter 2 of price 1".
 * An entry of a body that is itself a list is "item 1", and the body itself is "The request body".
 */
function fieldName(path) {
    const segments = String(path || '').split('/').filter(Boolean);
    if (segments.length > 0 && LOCATIONS.has(segments[0])) segments.shift();
    if (segments.length === 0) return 'The request body';
    const parts = [];
    let leaf = null;
    for (let i = 0; i < segments.length; i++) {
        const segment = segments[i];
        if (!/^\d+$/.test(segment)) continue;
        const list = i > 0 && !/^\d+$/.test(segments[i - 1]) ? singular(segments[i - 1]) : 'item';
        const element = `${list} ${Number(segment) + 1}`;
        if (i === segments.length - 1) leaf = element;
        else parts.unshift(element);
    }
    if (leaf === null) leaf = segments[segments.length - 1];
    return [leaf, ...parts].join(' of ');
}

/** What is wrong, from the validator's keyword and its message ("must be >= 1", "must NOT have more than 10 items"). */
function problem(error) {
    const keyword = String(error.errorCode || '').replace(/\.openapi\.validation$/, '');
    const message = String(error.message || '');
    const number = (message.match(/-?\d+(\.\d+)?/) || [])[0];
    switch (keyword) {
        case 'required':
            return 'is required';
        case 'additionalProperties':
        case 'unevaluatedProperties':
            return 'is not a known field';
        case 'enum': {
            const allowed = message.split(': ').slice(1).join(': ');
            return allowed ? `must be one of ${allowed}` : 'is not one of the allowed values';
        }
        case 'type': {
            const type = message.replace(/^must be /, '');
            return `must be ${/^[aeiou]/.test(type) ? 'an' : 'a'} ${type}`;
        }
        case 'minimum':
        case 'exclusiveMinimum':
            return keyword === 'minimum' ? `must be at least ${number}` : `must be more than ${number}`;
        case 'maximum':
        case 'exclusiveMaximum':
            return keyword === 'maximum' ? `must be at most ${number}` : `must be less than ${number}`;
        case 'minLength':
            return Number(number) === 1 ? 'must not be empty' : `must be at least ${number} characters`;
        case 'maxLength':
            return `must be at most ${number} characters`;
        case 'minItems':
            return Number(number) === 1 ? 'must have at least 1 entry' : `must have at least ${number} entries`;
        case 'maxItems':
            return Number(number) === 1 ? 'must have at most 1 entry' : `must have at most ${number} entries`;
        case 'pattern':
        case 'format':
            return 'is not in the expected format';
        default:
            return message.replace(/^request\/\S+\s/, '') || 'is not valid';
    }
}

/**
 * The validator's errors ([{ path, message, errorCode }]) as { message, errors }: `message` is the first
 * kept error as a sentence, `errors` each kept error as { path, message }. Errors that only say the request
 * did not fit another accepted shape are left out (see the file's comment).
 */
function formatValidationErrors(errors) {
    const all = (Array.isArray(errors) ? errors : []).filter((e) => e && typeof e === 'object');
    const kept = all.filter((e) => {
        if (String(e.errorCode || '').startsWith('oneOf.') || String(e.errorCode || '').startsWith('anyOf.')) return false;
        const path = String(e.path || '');
        // An error on a part of the request that holds another error's field is about another shape.
        return !all.some((other) => other !== e && String(other.path || '').startsWith(path + '/'));
    });
    const chosen = kept.length > 0 ? kept : all;
    const sentences = chosen.map((e) => ({ path: e.path, message: `${fieldName(e.path)} ${problem(e)}` }));
    return { message: sentences.length > 0 ? sentences[0].message : 'The request is not valid', errors: sentences };
}

module.exports = { formatValidationErrors, fieldName };
