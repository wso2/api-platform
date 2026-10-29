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
const { parseDuration, formatDuration } = require('./duration');

test('parseDuration reads Go-style durations', () => {
    assert.equal(parseDuration('3s'), 3000);
    assert.equal(parseDuration('500ms'), 500);
    assert.equal(parseDuration('1m30s'), 90000);
    assert.equal(parseDuration('1.5s'), 1500);
    assert.equal(parseDuration('2h'), 7200000);
    assert.equal(parseDuration(' 10s '), 10000);
    assert.equal(parseDuration('0s'), 0);
});

test('parseDuration rejects anything without a unit', () => {
    for (const bad of ['3', 3, '', 's', '3x', '-1s', '1s ago', null, undefined]) {
        assert.ok(Number.isNaN(parseDuration(bad)), String(bad));
    }
});

test('formatDuration prints like a Go duration', () => {
    assert.equal(formatDuration(0), '0s');
    assert.equal(formatDuration(-5), '0s');
    assert.equal(formatDuration(250), '250ms');
    assert.equal(formatDuration(2000), '2s');
    assert.equal(formatDuration(1500), '1.5s');
    assert.equal(formatDuration(65000), '1m5s');
    assert.equal(formatDuration(3600000), '1h0m0s');
});
