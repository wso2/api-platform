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
 * Sends subscription plan changes made in this portal to the Platform API as webhook events, so
 * the Platform API keeps the plan and sends it to the gateways. Without this, a plan created here
 * never reaches a gateway.
 *
 * The Platform API knows a plan by its handle, which this portal stores as the plan's ref_id.
 * A plan created here has no ref_id until it is first sent, so its own handle is used and
 * stored back as ref_id: subscription events carry ref_id, and from then on they name a plan the
 * Platform API has.
 */

const subscriptionPlanDao = require('../dao/subscriptionPlanDao');
const planPricingDao = require('../dao/planPricingDao');
const { publish } = require('./webhooks/eventPublisher');

const EVENT = Object.freeze({
    CREATED: 'subscription_plan.created',
    UPDATED: 'subscription_plan.updated',
    DELETED: 'subscription_plan.deleted',
});

/** The handle the Platform API knows this plan by. */
function platformHandle(plan) {
    return plan.ref_id || plan.handle;
}

function buildPlanPayload(plan) {
    return {
        handle: platformHandle(plan),
        display_name: plan.display_name,
        description: plan.description || null,
        limits: (plan.limits || []).map((l) => ({
            limit_type: l.limit_type,
            limit_count: Number(l.limit_count),
            time_unit: l.time_unit || null,
            time_amount: Number(l.time_amount) || 1,
        })),
    };
}

/** A paid plan's pricing (planPricingDao row) as the plan events carry it; null for a free plan. */
function buildPricingPayload(pricing) {
    if (!pricing) return null;
    return {
        pricing_type: pricing.pricing_type,
        currency: pricing.currency,
        billing_period: pricing.billing_period,
        flat_amount: pricing.flat_amount ?? null,
        prices: (pricing.prices || []).map((price) => ({
            meter_handle: price.meter_handle,
            filters: (price.filters || []).map((f) => ({ field: f.field, operator: f.operator, values: f.values })),
            pricing_model: price.pricing_model,
            unit_amount: price.unit_amount ?? null,
            unit_quantity: price.unit_quantity ?? null,
            tiers: price.tiers
                ? price.tiers.map((tier) => ({
                    up_to: tier.upTo ?? null,
                    unit_amount: tier.unitAmount ?? null,
                    flat_amount: tier.flatAmount ?? null,
                }))
                : null,
        })),
    };
}

/**
 * Publishes a created or updated event for `plan` (a DAO row with `.limits`, and `.pricing` when
 * it was just created) inside the caller's transaction, so the plan change and its event commit or
 * roll back together. Both events carry the plan's pricing, which never changes, so a Platform API
 * that missed the created event gets it with the next update.
 */
async function publishPlanUpsert(eventType, orgId, plan, transaction) {
    if (!plan.ref_id) {
        await subscriptionPlanDao.setRefId(orgId, plan.uuid, plan.handle, transaction);
        plan.ref_id = plan.handle;
    }
    const pricing = plan.pricing !== undefined ? plan.pricing : await planPricingDao.getByPlan(orgId, plan.uuid, transaction);
    const payload = { ...buildPlanPayload(plan), pricing: buildPricingPayload(pricing) };
    await publish(eventType, payload, {
        transaction,
        orgId,
        aggregateType: 'subscription_plan',
        aggregateId: plan.uuid,
    });
}

/** Publishes a deleted event for `plan` (read before it was deleted) inside the caller's transaction. */
async function publishPlanDeleted(orgId, plan, transaction) {
    await publish(EVENT.DELETED, { handle: platformHandle(plan) }, {
        transaction,
        orgId,
        aggregateType: 'subscription_plan',
        aggregateId: plan.uuid,
    });
}

module.exports = { EVENT, buildPlanPayload, publishPlanUpsert, publishPlanDeleted };
