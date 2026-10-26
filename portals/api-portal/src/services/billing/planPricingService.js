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
 * Plan pricing: what a paid subscription plan costs. A plan with pricing is paid; a plan
 * without is free. A paid plan is flat rate (one amount per billing period) or usage based (up to
 * 20 prices, each for what one meter counts narrowed by its own filters; see planPricingValidation),
 * billed on one invoice per period.
 *
 * Amounts are decimal strings in the currency's main unit ("0.01" is one cent in USD), kept
 * exactly as entered and never converted to floating point.
 *
 * Pricing is set only when a plan is created (`pricing` on POST or PUT /subscription-plans) and
 * never changes afterwards: whether the plan is free or paid, flat rate or usage based, its prices
 * and meters. A different price is a new plan. Deleting a plan deletes its pricing. The pricing
 * reaches the Platform API with the plan's subscription_plan.* events (subscriptionPlanEvents).
 */

const subscriptionPlanDao = require('../../dao/subscriptionPlanDao');
const meterDao = require('../../dao/meterDao');
const planPricingDao = require('../../dao/planPricingDao');
const PlanPricingDTO = require('../../dto/planPricingDto');
const userIdpReferenceDao = require('../../dao/userIdpReferenceDao');
const constants = require('../../utils/constants');
const util = require('../../utils/util');
const logger = require('../../config/logger');
const { resolvePrices, validatePricing } = require('./planPricingValidation');

/**
 * Checks validated pricing's prices against the organization's meters and returns the pricing
 * ready to save (planPricingDao.create), with the meters locked until the caller's transaction ends.
 * Throws PricingValidationError for an unknown meter or values the meters refuse.
 */
async function resolvePricing(orgId, input, t) {
    const handles = [...new Set(input.prices.map((p) => p.meterId))];
    await meterDao.lock(orgId, handles, t);
    const meters = new Map((await meterDao.listByHandles(orgId, handles, t)).map((m) => [m.handle, m]));
    return { ...input, prices: resolvePrices(input.prices, meters) };
}

/** The pricing as the REST API returns it. */
async function toDto(pricing, plan) {
    const { createdBy } = await userIdpReferenceDao.buildSingleAuditFields({ created_by: plan.created_by });
    return new PlanPricingDTO(pricing, { createdBy, createdAt: pricing.created_at });
}

const PRICING_LOCKED = 'PLAN_PRICING_LOCKED';

/** The refusal for pricing sent for a plan that already exists. */
function pricingLockedMessage(handle) {
    return `The plan ${handle} already exists; its pricing can't be updated once a plan is created.`;
}

function planNotFound(res) {
    return util.sendError(res, 404, constants.ERROR_MESSAGE.SUBSCRIPTION_PLAN_NOT_FOUND);
}

/**
 * Sets the pricing of a plan created in the same request (`pricing` on POST or PUT
 * /subscription-plans), in the caller's transaction, so the plan and its pricing are saved together
 * or not at all. No API offers a new plan yet, so nothing is created with the billing provider.
 * Resolves to the saved pricing; throws PricingValidationError for pricing or a meter the portal
 * refuses.
 */
async function setNewPlanPricing(orgId, plan, body, t) {
    const input = await resolvePricing(orgId, validatePricing(body), t);
    const { pricing } = await planPricingDao.create(orgId, plan.uuid, input, t);
    return pricing;
}

const getPlanPricing = async (req, res) => {
    try {
        const plan = await subscriptionPlanDao.getByName(req.orgId, req.params.planId);
        if (!plan) return planNotFound(res);
        const pricing = await planPricingDao.getByPlan(req.orgId, plan.uuid);
        if (!pricing) return util.sendError(res, 404, 'The plan is free: it has no pricing');
        return res.status(200).json(await toDto(pricing, plan));
    } catch (error) {
        logger.error('Error while retrieving plan pricing', { error });
        return util.sendError(res, 500, 'Error while retrieving plan pricing');
    }
};

module.exports = {
    PRICING_LOCKED,
    pricingLockedMessage,
    setNewPlanPricing,
    getPlanPricing,
};
