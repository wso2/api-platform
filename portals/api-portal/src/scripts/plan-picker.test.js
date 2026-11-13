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

/* Which plans the Subscribe / Switch plan dialog lets a subscriber pick. */

const test = require('node:test');
const assert = require('node:assert');
const { blockedReason } = require('./plan-picker');

const free = (id) => ({ id, paid: false });
const paid = (id) => ({ id, paid: true });

test('with no subscription, every plan can be picked, paid or free', () => {
    assert.strictEqual(blockedReason(free('basic'), '', false), '');
    assert.strictEqual(blockedReason(paid('gold'), '', false), '');
});

test('the subscribed plan is marked current and cannot be picked again', () => {
    assert.strictEqual(blockedReason(free('basic'), 'basic', false), 'Current plan');
});

test('a switch moves between free plans only', () => {
    assert.strictEqual(blockedReason(free('pro'), 'basic', false), '');
    assert.match(blockedReason(paid('gold'), 'basic', false), /unsubscribing first/);
    assert.match(blockedReason(free('basic'), 'gold', true), /unsubscribing first/);
});
