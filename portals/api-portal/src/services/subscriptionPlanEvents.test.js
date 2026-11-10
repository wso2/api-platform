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
 * Plan changes are sent to the Platform API under the handle it knows the plan by: ref_id when
 * the plan came from there, otherwise this portal's handle, which is then stored as ref_id so
 * later subscription events name the same plan.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const PLAN_DAO_PATH = path.resolve(__dirname, '../dao/subscriptionPlanDao.js');
const PRICING_DAO_PATH = path.resolve(__dirname, '../dao/planPricingDao.js');
const PUBLISHER_PATH = path.resolve(__dirname, './webhooks/eventPublisher.js');
const EVENTS_PATH = path.resolve(__dirname, './subscriptionPlanEvents.js');

function stub(modulePath, exports) {
    require.cache[modulePath] = { id: modulePath, filename: modulePath, loaded: true, exports };
}

// `stored` is the pricing kept for each plan uuid; a plan missing from it is free.
function load(stored = {}) {
    const calls = { refIds: [], published: [] };
    stub(PLAN_DAO_PATH, {
        setRefId: async (orgId, planId, refId, t) => calls.refIds.push({ orgId, planId, refId, t }),
    });
    stub(PRICING_DAO_PATH, { getByPlan: async (orgId, planId) => stored[planId] || null });
    stub(PUBLISHER_PATH, {
        publish: async (eventType, payload, opts) => calls.published.push({ eventType, payload, opts }),
    });
    delete require.cache[EVENTS_PATH];
    try {
        return { events: require(EVENTS_PATH), calls };
    } finally {
        for (const p of [PLAN_DAO_PATH, PRICING_DAO_PATH, PUBLISHER_PATH, EVENTS_PATH]) delete require.cache[p];
    }
}

const limits = [{ limit_type: 'REQUEST_COUNT', limit_count: '1000', time_unit: 'MINUTE', time_amount: '1' }];

test('a plan created in the portal is sent under its own handle, which becomes its ref_id', async () => {
    const { events, calls } = load();
    const plan = { uuid: 'p1', handle: 'gold', ref_id: null, display_name: 'Gold', description: null, limits };
    const tx = {};

    await events.publishPlanUpsert(events.EVENT.CREATED, 'org-1', plan, tx);

    assert.deepStrictEqual(calls.refIds, [{ orgId: 'org-1', planId: 'p1', refId: 'gold', t: tx }]);
    assert.strictEqual(plan.ref_id, 'gold');
    assert.strictEqual(calls.published.length, 1);
    const { eventType, payload, opts } = calls.published[0];
    assert.strictEqual(eventType, 'subscription_plan.created');
    assert.deepStrictEqual(payload, {
        handle: 'gold',
        display_name: 'Gold',
        description: null,
        limits: [{ limit_type: 'REQUEST_COUNT', limit_count: 1000, time_unit: 'MINUTE', time_amount: 1 }],
        pricing: null,
    });
    assert.deepStrictEqual(opts, { transaction: tx, orgId: 'org-1', aggregateType: 'subscription_plan', aggregateId: 'p1' });
});

// The pricing never changes, so an update carries the same pricing: a Platform API that missed the
// created event gets it with the update.
test('a paid plan\'s created and updated events both carry its pricing', async () => {
    const pricing = {
        pricing_type: 'USAGE_BASED', currency: 'USD', billing_period: 'MONTH', flat_amount: null,
        prices: [{
            meter_handle: 'llm-input', pricing_model: 'PER_UNIT', unit_amount: '2.50', unit_quantity: 1000000, tiers: null,
            filters: [{ field: 'Prompt tokens', operator: 'GREATER_THAN', values: ['100'] }],
        }, {
            meter_handle: 'calls', pricing_model: 'GRADUATED', unit_amount: null, unit_quantity: null, filters: [],
            tiers: [{ upTo: 1000, unitAmount: '0', flatAmount: null }, { upTo: null, unitAmount: '0.001', flatAmount: null }],
        }],
    };
    const { events, calls } = load({ p3: pricing });

    // Created: the pricing saved in the same request. Updated: the row has none, so it is read.
    await events.publishPlanUpsert(events.EVENT.CREATED, 'org-1',
        { uuid: 'p3', handle: 'gold', ref_id: 'gold', display_name: 'Gold', limits: [], pricing }, {});
    await events.publishPlanUpsert(events.EVENT.UPDATED, 'org-1',
        { uuid: 'p3', handle: 'gold', ref_id: 'gold', display_name: 'Gold Plus', limits: [] }, {});

    assert.deepStrictEqual(calls.published[0].payload.pricing, {
        pricing_type: 'USAGE_BASED', currency: 'USD', billing_period: 'MONTH', flat_amount: null,
        prices: [{
            meter_handle: 'llm-input',
            filters: [{ field: 'Prompt tokens', operator: 'GREATER_THAN', values: ['100'] }],
            pricing_model: 'PER_UNIT', unit_amount: '2.50', unit_quantity: 1000000, tiers: null,
        }, {
            meter_handle: 'calls', filters: [], pricing_model: 'GRADUATED', unit_amount: null, unit_quantity: null,
            tiers: [{ up_to: 1000, unit_amount: '0', flat_amount: null }, { up_to: null, unit_amount: '0.001', flat_amount: null }],
        }],
    });
    assert.strictEqual(calls.published[1].eventType, 'subscription_plan.updated');
    assert.deepStrictEqual(calls.published[1].payload.pricing, calls.published[0].payload.pricing);
});

test('a plan that came from the Platform API keeps its ref_id and is sent under it', async () => {
    const { events, calls } = load();
    const plan = { uuid: 'p2', handle: 'local-handle', ref_id: 'platinum', display_name: 'Platinum', limits: [] };

    await events.publishPlanUpsert(events.EVENT.UPDATED, 'org-1', plan, {});

    assert.deepStrictEqual(calls.refIds, []);
    assert.strictEqual(calls.published[0].eventType, 'subscription_plan.updated');
    assert.strictEqual(calls.published[0].payload.handle, 'platinum');
    assert.deepStrictEqual(calls.published[0].payload.limits, []);
    assert.strictEqual(calls.published[0].payload.pricing, null, 'a free plan carries no pricing');
});

test('an unlimited limit keeps -1 and a missing time unit stays null', () => {
    const { events } = load();
    const payload = events.buildPlanPayload({
        handle: 'unlimited', display_name: 'Unlimited',
        limits: [{ limit_type: 'REQUEST_COUNT', limit_count: -1, time_unit: null, time_amount: null }],
    });
    assert.deepStrictEqual(payload.limits, [{ limit_type: 'REQUEST_COUNT', limit_count: -1, time_unit: null, time_amount: 1 }]);
});

test('a deleted plan is sent with only its Platform API handle', async () => {
    const { events, calls } = load();
    await events.publishPlanDeleted('org-1', { uuid: 'p3', handle: 'h', ref_id: 'silver' }, {});
    assert.deepStrictEqual(calls.published.map((c) => [c.eventType, c.payload]), [['subscription_plan.deleted', { handle: 'silver' }]]);
});
