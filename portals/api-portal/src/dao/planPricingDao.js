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
 * Data access for plan pricing: `subscription_plan_pricing`, one row per paid plan with its pricing type,
 * currency, billing period and, for a flat rate plan, its flat amount, and `subscription_plan_pricing_usage`, a usage
 * based plan's prices in order (see planPricingValidation). A plan without a `subscription_plan_pricing` row is
 * free. Each price names the meter that counts its units (meter_uuid) and the filters that narrow
 * it (filters). A price read here also has `name`, the name subscribers see, made from its meter's
 * name and its filters.
 */

const crypto = require('crypto');
const db = require('../db/driver');
const { groupBy } = require('../db/rows');
const { bufferToUtf8 } = require('../utils/cryptoUtil');
const { getPortalId } = require('../utils/orgContext');
const { priceName } = require('../utils/meterLabels');

const TABLE = 'subscription_plan_pricing';
const PRICES_TABLE = 'subscription_plan_pricing_usage';
const METERS_TABLE = 'meters';

const PRICING_COLUMNS = 'p.subscription_plan_uuid, p.org_uuid, p.pricing_type, p.currency, p.billing_period, p.flat_amount, '
    + 'p.created_by, p.created_at, p.updated_by, p.updated_at';
const PRICE_COLUMNS = 'c.uuid, c.subscription_plan_uuid, c.meter_uuid, m.handle AS meter_handle, '
    + 'm.display_name AS meter_name, c.filters, c.pricing_model, c.unit_amount, c.unit_quantity, c.tiers';

/** A JSON array stored as binary; unreadable JSON reads as `fallback`. */
function parseArray(value, fallback) {
    const text = bufferToUtf8(value);
    if (!text) return fallback;
    try {
        const parsed = JSON.parse(text);
        return Array.isArray(parsed) ? parsed : fallback;
    } catch (_err) {
        return fallback;
    }
}

function jsonParam(value) {
    return db.binaryParam(value ? Buffer.from(JSON.stringify(value), 'utf8') : null);
}

function toPrice(row) {
    const filters = parseArray(row.filters, []);
    return {
        uuid: row.uuid,
        name: priceName(row.meter_name || row.meter_handle, filters),
        meter_uuid: row.meter_uuid,
        meter_handle: row.meter_handle || null,
        filters,
        pricing_model: row.pricing_model,
        unit_amount: row.unit_amount ?? null,
        unit_quantity: row.unit_quantity === null || row.unit_quantity === undefined ? null : Number(row.unit_quantity),
        tiers: parseArray(row.tiers, null),
    };
}

/**
 * The order of a plan's usage prices: by name (its meter and filters, as priceName makes it),
 * ignoring case, then by meter handle (two meters may share a name; one meter's prices never do,
 * as they would count the same requests). A plan stores no order of its own, so every reader, and
 * the Management Portal (pricesByName), lists them alike.
 */
function byName(a, b) {
    const handleOrder = a.meter_handle < b.meter_handle ? -1 : Number(a.meter_handle > b.meter_handle);
    return a.name.localeCompare(b.name, 'en', { sensitivity: 'base' }) || handleOrder
        || (a.uuid < b.uuid ? -1 : Number(a.uuid > b.uuid));
}

/** The pricing of each given plan with its prices, as a Map of plan uuid to pricing. A free plan is absent. */
const listByPlans = async (orgId, planIds, t) => {
    if (!planIds || planIds.length === 0) return new Map();
    const exec = t || db;
    const placeholders = planIds.map(() => '?').join(', ');
    const rows = await exec.query(
        `SELECT ${PRICING_COLUMNS} FROM ${TABLE} p
         WHERE p.subscription_plan_uuid IN (${placeholders}) AND p.org_uuid = ? AND p.portal_id = ?`,
        [...planIds, orgId, getPortalId()]
    );
    if (rows.length === 0) return new Map();
    const priceRows = await exec.query(
        `SELECT ${PRICE_COLUMNS} FROM ${PRICES_TABLE} c
         LEFT JOIN ${METERS_TABLE} m ON m.uuid = c.meter_uuid AND m.portal_id = c.portal_id
         WHERE c.subscription_plan_uuid IN (${placeholders}) AND c.org_uuid = ? AND c.portal_id = ?
         ORDER BY c.subscription_plan_uuid, c.uuid`,
        [...planIds, orgId, getPortalId()]
    );
    const prices = groupBy(priceRows, 'subscription_plan_uuid');
    return new Map(rows.map((r) => [r.subscription_plan_uuid, {
        ...r,
        flat_amount: r.flat_amount ?? null,
        prices: (prices.get(r.subscription_plan_uuid) || []).map(toPrice).sort(byName),
    }]));
};

/**
 * Sets `.pricing` on each plan row: its pricing, or null for a free plan. The plans may belong to
 * different organizations.
 */
const attachPricing = async (plans, t) => {
    const byOrg = groupBy(plans, 'org_uuid');
    for (const [orgId, orgPlans] of byOrg) {
        const pricing = await listByPlans(orgId, orgPlans.map((p) => p.uuid), t);
        for (const plan of orgPlans) {
            plan.pricing = pricing.get(plan.uuid) || null;
        }
    }
    return plans;
};

/** The pricing of one plan, or null when the plan is free. */
const getByPlan = async (orgId, planId, t) => {
    const pricing = await listByPlans(orgId, [planId], t);
    return pricing.get(planId) || null;
};

/**
 * Creates the pricing of a new plan, prices included: pricing is set only when a plan is created
 * and never changes. `pricing` is validated pricing whose prices are resolved against their meters
 * (planPricingValidation.resolvePrices): each has `meter` (the meter row) and `filters`.
 */
const create = async (orgId, planId, pricing, userId, t) => {
    const exec = t || db;
    const portalId = getPortalId();
    const now = new Date();
    await exec.execute(
        `INSERT INTO ${TABLE}
           (subscription_plan_uuid, org_uuid, portal_id, pricing_type, currency, billing_period, flat_amount, created_by,
            created_at, updated_by, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [planId, orgId, portalId, pricing.pricingType, pricing.currency, pricing.billingPeriod,
            pricing.flatAmount ?? null, userId, now, userId, now]
    );
    for (const price of pricing.prices) {
        await exec.execute(
            `INSERT INTO ${PRICES_TABLE}
               (uuid, subscription_plan_uuid, org_uuid, portal_id, meter_uuid, filters, pricing_model,
                unit_amount, unit_quantity, tiers, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
            [crypto.randomUUID(), planId, orgId, portalId, price.meter.uuid,
                jsonParam(price.filters), price.pricingModel, price.unitAmount ?? null,
                price.unitQuantity ?? null, jsonParam(price.tiers), now, now]
        );
    }
    return { pricing: await getByPlan(orgId, planId, t) };
};

module.exports = {
    listByPlans,
    attachPricing,
    getByPlan,
    create,
};
