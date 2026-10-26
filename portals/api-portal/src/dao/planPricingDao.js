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
 * Data access for plan pricing: `subscription_plan_pricing`, one row per paid plan with its currency,
 * billing period and prices. A plan without a row is free.
 *
 * Prices are stored as a JSON array, each with its own uuid; the flat fee is one FLAT price. A
 * pricing read here is { pricing_type, currency, billing_period, flat_amount, flat_price_uuid,
 * prices, created_at }; each usage price has `name`, the name subscribers see.
 */

const crypto = require('crypto');
const db = require('../db/driver');
const { groupBy } = require('../db/rows');
const { bufferToUtf8 } = require('../utils/cryptoUtil');
const { getPortalId } = require('../utils/orgContext');
const { priceName } = require('../utils/meterLabels');

const TABLE = 'subscription_plan_pricing';
const METERS_TABLE = 'meters';
const FLAT = 'FLAT';

const COLUMNS = 'subscription_plan_uuid, org_uuid, currency, billing_period, prices, created_at';

/** A JSON array stored as binary. */
function parseArray(value) {
    try {
        const parsed = JSON.parse(bufferToUtf8(value) || '[]');
        return Array.isArray(parsed) ? parsed : [];
    } catch (_err) {
        return [];
    }
}

function jsonParam(value) {
    return db.binaryParam(Buffer.from(JSON.stringify(value), 'utf8'));
}

const storedFilter = (f) => ({ field: f.field, operator: f.operator, values: f.values });
const readFilter = (f) => ({ field: f.field, operator: f.operator, values: f.values || [] });

const storedTier = (tier) => ({ up_to: tier.upTo ?? null, unit_amount: tier.unitAmount ?? null, flat_amount: tier.flatAmount ?? null });
const readTier = (tier) => ({ upTo: tier.up_to ?? null, unitAmount: tier.unit_amount ?? null, flatAmount: tier.flat_amount ?? null });

/** The prices to store for validated pricing: the flat fee as one FLAT price, or each usage price. */
function storedPrices(pricing) {
    if (pricing.pricingType === 'FLAT_RATE') {
        return [{ uuid: crypto.randomUUID(), pricing_model: FLAT, amount: pricing.flatAmount }];
    }
    return pricing.prices.map((price) => ({
        uuid: crypto.randomUUID(),
        meter_handle: price.meter.handle,
        filters: (price.filters || []).map(storedFilter),
        pricing_model: price.pricingModel,
        unit_amount: price.unitAmount ?? null,
        unit_quantity: price.unitQuantity ?? null,
        tiers: price.tiers ? price.tiers.map(storedTier) : null,
    }));
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

/** A stored pricing row as read (`meterNames`: meter handle to name). */
function toPricing(row, meterNames) {
    const stored = parseArray(row.prices);
    const flat = stored.length === 1 && stored[0].pricing_model === FLAT ? stored[0] : null;
    const prices = flat ? [] : stored.map((price) => {
        const filters = (price.filters || []).map(readFilter);
        return {
            uuid: price.uuid,
            name: priceName(meterNames.get(price.meter_handle) || price.meter_handle, filters),
            meter_handle: price.meter_handle,
            filters,
            pricing_model: price.pricing_model,
            unit_amount: price.unit_amount ?? null,
            unit_quantity: price.unit_quantity === null || price.unit_quantity === undefined ? null : Number(price.unit_quantity),
            tiers: price.tiers ? price.tiers.map(readTier) : null,
        };
    }).sort(byName);
    return {
        subscription_plan_uuid: row.subscription_plan_uuid,
        org_uuid: row.org_uuid,
        pricing_type: flat ? 'FLAT_RATE' : 'USAGE_BASED',
        currency: row.currency,
        billing_period: row.billing_period,
        flat_amount: flat ? flat.amount : null,
        flat_price_uuid: flat ? flat.uuid : null,
        prices,
        created_at: row.created_at,
    };
}

/** The display names of the organization's meters with the given handles, as a Map of handle to name. */
async function meterNames(exec, orgId, handles) {
    if (handles.length === 0) return new Map();
    const placeholders = handles.map(() => '?').join(', ');
    const rows = await exec.query(
        `SELECT handle, display_name FROM ${METERS_TABLE} WHERE handle IN (${placeholders}) AND org_uuid = ? AND portal_id = ?`,
        [...handles, orgId, getPortalId()]
    );
    return new Map(rows.map((r) => [r.handle, r.display_name]));
}

/** The pricing of each given plan with its prices, as a Map of plan uuid to pricing. A free plan is absent. */
const listByPlans = async (orgId, planIds, t) => {
    if (!planIds || planIds.length === 0) return new Map();
    const exec = t || db;
    const placeholders = planIds.map(() => '?').join(', ');
    const rows = await exec.query(
        `SELECT ${COLUMNS} FROM ${TABLE}
         WHERE subscription_plan_uuid IN (${placeholders}) AND org_uuid = ? AND portal_id = ?`,
        [...planIds, orgId, getPortalId()]
    );
    if (rows.length === 0) return new Map();
    const handles = [...new Set(rows.flatMap((r) => parseArray(r.prices).map((p) => p.meter_handle).filter(Boolean)))];
    const names = await meterNames(exec, orgId, handles);
    return new Map(rows.map((r) => [r.subscription_plan_uuid, toPricing(r, names)]));
};

/** The pricing of each of the organization's paid plans, as a Map of plan uuid to pricing. */
const listByOrg = async (orgId, t) => {
    const exec = t || db;
    const rows = await exec.query(`SELECT ${COLUMNS} FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]);
    const handles = [...new Set(rows.flatMap((r) => parseArray(r.prices).map((p) => p.meter_handle).filter(Boolean)))];
    const names = await meterNames(exec, orgId, handles);
    return new Map(rows.map((r) => [r.subscription_plan_uuid, toPricing(r, names)]));
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
 * Creates the pricing of a new plan. `pricing` is validated pricing whose prices are resolved against their meters
 * (planPricingValidation.resolvePrices): each has `meter` (the meter row) and `filters`.
 */
const create = async (orgId, planId, pricing, t) => {
    const exec = t || db;
    await exec.execute(
        `INSERT INTO ${TABLE} (subscription_plan_uuid, org_uuid, portal_id, currency, billing_period, prices, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [planId, orgId, getPortalId(), pricing.currency, pricing.billingPeriod, jsonParam(storedPrices(pricing)), new Date()]
    );
    return { pricing: await getByPlan(orgId, planId, t) };
};

module.exports = {
    listByPlans,
    listByOrg,
    attachPricing,
    getByPlan,
    create,
};
