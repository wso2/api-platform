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
const { findSubscription, setSubscriptionStatus, planCardState } = require('./subscription-state');

const SUBS = [
    { subscriptionId: 's1', policyName: 'Gold', status: 'ACTIVE' },
    { subscriptionId: 's2', policyName: 'Silver', status: 'ACTIVE' },
    { subscriptionId: 's3', subscriptionPlanName: 'Gold', status: 'INACTIVE' },
];

test('findSubscription resolves by subscription id, not plan name', () => {
    assert.strictEqual(findSubscription(SUBS, 's3').subscriptionId, 's3');
    assert.strictEqual(findSubscription(SUBS, 's1').subscriptionId, 's1');
});

test('findSubscription returns null for unknown ids and bad input', () => {
    assert.strictEqual(findSubscription(SUBS, 'nope'), null);
    assert.strictEqual(findSubscription(undefined, 's1'), null);
});

test('setSubscriptionStatus updates only the targeted subscription', () => {
    const subs = SUBS.map(s => ({ ...s }));
    setSubscriptionStatus(subs, 's1', 'INACTIVE');
    assert.deepStrictEqual(subs.map(s => s.status), ['INACTIVE', 'ACTIVE', 'INACTIVE']);
});

test('planCardState counts every subscription on the plan', () => {
    assert.deepStrictEqual(planCardState('Gold', SUBS), { count: 2, subscribed: true, suspended: false });
});

test('planCardState is suspended only when every subscription on the plan is inactive', () => {
    const allOff = [{ policyName: 'Gold', status: 'INACTIVE' }, { policyName: 'Gold', status: 'INACTIVE' }];
    assert.strictEqual(planCardState('Gold', allOff).suspended, true);
    assert.strictEqual(planCardState('Gold', SUBS).suspended, false);
});

test('planCardState for an unsubscribed plan', () => {
    assert.deepStrictEqual(planCardState('Bronze', SUBS), { count: 0, subscribed: false, suspended: false });
    assert.deepStrictEqual(planCardState('Gold', null), { count: 0, subscribed: false, suspended: false });
});
