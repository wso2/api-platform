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

/* The OpenAPI document's request checks as plain sentences (validationErrors.js), from errors shaped as
 * express-openapi-validator gives them. */

const test = require('node:test');
const assert = require('node:assert/strict');
const { formatValidationErrors, fieldName } = require('./validationErrors');

const err = (path, message, keyword) => ({ path, message, errorCode: `${keyword}.openapi.validation` });
// What a body that may be one plan or a list of plans is also told when it is one plan.
const otherShape = [err('/body', 'must be array', 'type'), err('/body', 'must match exactly one schema in oneOf', 'oneOf')];

test('the error for the shape sent is kept; those about the other accepted shape are left out', () => {
    const r = formatValidationErrors([
        err('/body/pricing/billingPeriod', 'must be equal to one of the allowed values: MONTH, YEAR', 'enum'), ...otherShape]);
    assert.equal(r.message, 'billingPeriod must be one of MONTH, YEAR');
    assert.deepEqual(r.errors, [{ path: '/body/pricing/billingPeriod', message: 'billingPeriod must be one of MONTH, YEAR' }]);
});

test('each kind of rule reads as a sentence that names the field', () => {
    const cases = [
        [err('/body/pricing/prices/0/meterId', "must have required property 'meterId'", 'required'), 'meterId of price 1 is required'],
        [err('/body/color', 'must NOT have additional properties', 'additionalProperties'), 'color is not a known field'],
        [err('/body/pricing/prices/1/colour', 'must NOT have additional properties', 'additionalProperties'),
            'colour of price 2 is not a known field'],
        [err('/body/pricing/prices/0/pricingModel', 'must be equal to one of the allowed values: PER_UNIT, VOLUME, GRADUATED', 'enum'),
            'pricingModel of price 1 must be one of PER_UNIT, VOLUME, GRADUATED'],
        [err('/body/pricing/prices/0/filters/0/operator', 'must be equal to one of the allowed values: EQUALS, IN', 'enum'),
            'operator of filter 1 of price 1 must be one of EQUALS, IN'],
        [err('/body/pricing/prices/0/unitQuantity', 'must be integer', 'type'), 'unitQuantity of price 1 must be an integer'],
        [err('/body/displayName', 'must be string', 'type'), 'displayName must be a string'],
        [err('/query/limit', 'must be >= 1', 'minimum'), 'limit must be at least 1'],
        [err('/query/limit', 'must be <= 100', 'maximum'), 'limit must be at most 100'],
        [err('/body/pricing/prices/0/filters/1/values/0', 'must NOT have more than 255 characters', 'maxLength'),
            'value 1 of filter 2 of price 1 must be at most 255 characters'],
        [err('/body/displayName', 'must NOT have fewer than 1 characters', 'minLength'), 'displayName must not be empty'],
        [err('/body/pricing/prices', 'must NOT have more than 20 items', 'maxItems'), 'prices must have at most 20 entries'],
        [err('/body/id', 'must match pattern "^[a-z]+$"', 'pattern'), 'id is not in the expected format'],
        [err('/body/1/displayName', "must have required property 'displayName'", 'required'), 'displayName of item 2 is required'],
        [err('/body', 'must be object', 'type'), 'The request body must be an object'],
    ];
    for (const [error, sentence] of cases) {
        assert.equal(formatValidationErrors([error, ...otherShape]).message, sentence, error.path);
    }
});

test('several errors of the shape sent are all kept, in order', () => {
    const r = formatValidationErrors([
        err('/body/pricing/currency', 'must be string', 'type'),
        err('/body/pricing/prices/0/meterId', "must have required property 'meterId'", 'required'),
        ...otherShape,
    ]);
    assert.equal(r.message, 'currency must be a string');
    assert.deepEqual(r.errors.map((e) => e.message), ['currency must be a string', 'meterId of price 1 is required']);
});

test('errors that are all about the accepted shapes are still reported', () => {
    const r = formatValidationErrors(otherShape);
    assert.equal(r.message, 'The request body must be an array');
    assert.equal(formatValidationErrors([]).message, 'The request is not valid');
});

test('a field path reads in words', () => {
    assert.equal(fieldName('/body/pricing/prices/0/tiers/2/upTo'), 'upTo of tier 3 of price 1');
    assert.equal(fieldName('/body/pricing/prices/3'), 'price 4');
    assert.equal(fieldName('/body'), 'The request body');
});
