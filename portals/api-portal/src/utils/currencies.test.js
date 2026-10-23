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
const { CURRENCIES, isOfferedCurrency } = require('./currencies');

test('every currency is a distinct three-letter code with a name, in code order', () => {
    const codes = CURRENCIES.map(c => c.code);
    assert.strictEqual(CURRENCIES.length, 116);
    assert.strictEqual(new Set(codes).size, codes.length);
    assert.deepStrictEqual(codes, [...codes].sort());
    for (const c of CURRENCIES) {
        assert.match(c.code, /^[A-Z]{3}$/);
        assert.ok(c.name.length > 0, `${c.code} has no name`);
    }
});

test('the default currency of a new paid plan is in the list', () => {
    assert.ok(CURRENCIES.some(c => c.code === 'USD'));
});

test('only currencies Stripe counts in hundredths are offered', () => {
    // Stripe's zero-decimal and three-decimal currencies, and ISK (no fractions).
    const notHundredths = ['BIF', 'CLP', 'DJF', 'GNF', 'JPY', 'KMF', 'KRW', 'MGA', 'PYG', 'RWF', 'UGX', 'VND',
        'VUV', 'XAF', 'XOF', 'XPF', 'BHD', 'JOD', 'KWD', 'OMR', 'TND', 'ISK'];
    for (const code of notHundredths) {
        assert.ok(!isOfferedCurrency(code), `${code} is offered`);
    }
    assert.ok(isOfferedCurrency('USD'));
    assert.ok(!isOfferedCurrency('XYZ'));
    assert.ok(!isOfferedCurrency('usd'));
});
