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
 * Data access for the billing API plans: `billing_api_plans`, one paid plan as sold on one API for
 * one billing configuration, and its `billing_api_plan_items`, one for a flat rate plan's price or one
 * per usage price, each with the provider's price. An API plan is written once, complete with its
 * items, by billingCatalogService in the request that enabled the plan; it is never partly written
 * or changed afterwards.
 */

const crypto = require('crypto');
const db = require('../db/driver');
const { getPortalId } = require('../utils/orgContext');

const API_PLANS = 'billing_api_plans';
const ITEMS = 'billing_api_plan_items';

const API_PLAN_COLUMNS = 'ap.uuid, ap.org_uuid, ap.configuration_uuid, ap.api_uuid, ap.plan_uuid, '
    + 'ap.created_at, ap.updated_at';
const ITEM_COLUMNS = 'i.uuid, i.api_plan_uuid, i.price_uuid, i.price_ref';

/**
 * The (API, paid plan) pairs of the organization: the API offers the plan and the plan has
 * pricing. Narrowed to one plan with `planUuid`.
 */
const listPaidPairs = async (orgId, { planUuid } = {}, t) => {
    const exec = t || db;
    const params = [orgId, getPortalId()];
    let filter = '';
    if (planUuid) {
        filter = ' AND m.plan_uuid = ?';
        params.push(planUuid);
    }
    return exec.query(
        `SELECT m.api_uuid, m.plan_uuid, a.name AS api_name, a.version AS api_version, a.type AS api_type
         FROM api_subscription_plan_mappings m
         JOIN subscription_plan_pricing pp ON pp.subscription_plan_uuid = m.plan_uuid AND pp.portal_id = m.portal_id
         JOIN api_metadata a ON a.uuid = m.api_uuid AND a.portal_id = m.portal_id
         WHERE a.org_uuid = ? AND m.portal_id = ?${filter}`,
        params
    );
};

/**
 * Writes a complete API plan with its items: `apiPlan` is { uuid, orgId, configurationUuid, apiUuid, planUuid,
 * items }, each item { priceUuid (null for a flat rate price), priceRef }. The provider's
 * plan and billing meter of an item are not stored: nothing reads them back.
 */
const insertApiPlan = async (apiPlan, t) => {
    const exec = t || db;
    const portalId = getPortalId();
    const now = new Date();
    await exec.execute(
        `INSERT INTO ${API_PLANS} (uuid, org_uuid, portal_id, configuration_uuid, api_uuid, plan_uuid,
            created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        [apiPlan.uuid, apiPlan.orgId, portalId, apiPlan.configurationUuid, apiPlan.apiUuid, apiPlan.planUuid, now, now]
    );
    for (const item of apiPlan.items) {
        await exec.execute(
            `INSERT INTO ${ITEMS} (uuid, api_plan_uuid, org_uuid, portal_id, price_uuid, price_ref, created_at)
             VALUES (?, ?, ?, ?, ?, ?, ?)`,
            [crypto.randomUUID(), apiPlan.uuid, apiPlan.orgId, portalId, item.priceUuid || null, item.priceRef, now]
        );
    }
};

function toItem(row) {
    return { ...row, price_uuid: row.price_uuid || null };
}

/**
 * The items of an API plan: a flat rate plan's one (price_uuid null), or each usage price's. They
 * keep no order; the checkout lists them in the plan's price order (checkoutService).
 */
const listItems = async (apiPlanUuid, t) => {
    const exec = t || db;
    const rows = await exec.query(
        `SELECT ${ITEM_COLUMNS} FROM ${ITEMS} i WHERE i.api_plan_uuid = ? AND i.portal_id = ? ORDER BY i.uuid`,
        [apiPlanUuid, getPortalId()]
    );
    return rows.map(toItem);
};

/** The items of the given API plans, as a Map of API plan uuid to its items. */
const listItemsByApiPlans = async (apiPlanUuids, t) => {
    if (!apiPlanUuids || apiPlanUuids.length === 0) return new Map();
    const exec = t || db;
    const placeholders = apiPlanUuids.map(() => '?').join(', ');
    const rows = await exec.query(
        `SELECT ${ITEM_COLUMNS} FROM ${ITEMS} i
         WHERE i.api_plan_uuid IN (${placeholders}) AND i.portal_id = ?
         ORDER BY i.api_plan_uuid, i.uuid`,
        [...apiPlanUuids, getPortalId()]
    );
    const result = new Map();
    for (const row of rows) {
        if (!result.has(row.api_plan_uuid)) result.set(row.api_plan_uuid, []);
        result.get(row.api_plan_uuid).push(toItem(row));
    }
    return result;
};

/** The organization's API plans for one billing configuration, with the API and plan names. */
const listForConfiguration = async (orgId, configurationUuid, t) => {
    const exec = t || db;
    return exec.query(
        `SELECT ${API_PLAN_COLUMNS}, a.name AS api_name, a.version AS api_version, a.handle AS api_handle,
                p.handle AS plan_handle, p.display_name AS plan_name
         FROM ${API_PLANS} ap
         JOIN api_metadata a ON a.uuid = ap.api_uuid AND a.portal_id = ap.portal_id
         JOIN subscription_plans p ON p.uuid = ap.plan_uuid AND p.portal_id = ap.portal_id
         WHERE ap.org_uuid = ? AND ap.configuration_uuid = ? AND ap.portal_id = ?
         ORDER BY a.name, a.version, p.display_name`,
        [orgId, configurationUuid, getPortalId()]
    );
};

/** The API plan of (billing configuration, API, plan), or null. */
const findApiPlan = async (configurationUuid, apiUuid, planUuid, t) => {
    const exec = t || db;
    return (await exec.queryOne(
        `SELECT ${API_PLAN_COLUMNS} FROM ${API_PLANS} ap
         WHERE ap.configuration_uuid = ? AND ap.api_uuid = ? AND ap.plan_uuid = ? AND ap.portal_id = ?`,
        [configurationUuid, apiUuid, planUuid, getPortalId()]
    )) || null;
};

/** The plan uuids of the API that have an API plan for the billing configuration. */
const listPlanIds = async (configurationUuid, apiUuid, t) => {
    const exec = t || db;
    const rows = await exec.query(
        `SELECT plan_uuid FROM ${API_PLANS}
         WHERE configuration_uuid = ? AND api_uuid = ? AND portal_id = ?`,
        [configurationUuid, apiUuid, getPortalId()]
    );
    return new Set(rows.map((r) => r.plan_uuid));
};

module.exports = {
    listPaidPairs,
    insertApiPlan,
    listItems,
    listItemsByApiPlans,
    listForConfiguration,
    findApiPlan,
    listPlanIds,
};
