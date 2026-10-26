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
 * Data access for `billing_api_plans`: one paid plan as sold on one API, with its items, one per
 * price, each naming what the billing provider created for it. A row is written once by
 * billingCatalogService and never changes; billingCatalogService.retire takes it off sale.
 *
 * Items are stored as JSON ({ priceUuid, planRef, priceRef, meterRef }) and read as
 * { price_uuid, plan_ref, price_ref, meter_ref }.
 */

const db = require('../db/driver');
const { bufferToUtf8 } = require('../utils/cryptoUtil');
const { getPortalId } = require('../utils/orgContext');

const API_PLANS = 'billing_api_plans';
const COLUMNS = 'ap.org_uuid, ap.api_uuid, ap.plan_uuid, ap.items, ap.created_at';

function readItems(value) {
    try {
        const parsed = JSON.parse(bufferToUtf8(value) || '[]');
        return (Array.isArray(parsed) ? parsed : []).map((item) => ({
            price_uuid: item.priceUuid,
            plan_ref: item.planRef || null,
            price_ref: item.priceRef,
            meter_ref: item.meterRef || null,
        }));
    } catch (_err) {
        return [];
    }
}

function toApiPlan(row) {
    return row ? { ...row, items: readItems(row.items) } : null;
}

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
        `SELECT m.api_uuid, m.plan_uuid, a.name AS api_name, a.version AS api_version
         FROM api_subscription_plan_mappings m
         JOIN subscription_plan_pricing pp ON pp.subscription_plan_uuid = m.plan_uuid AND pp.portal_id = m.portal_id
         JOIN api_metadata a ON a.uuid = m.api_uuid AND a.portal_id = m.portal_id
         WHERE a.org_uuid = ? AND m.portal_id = ?${filter}`,
        params
    );
};

/** Writes an API plan: `apiPlan` is { orgId, apiUuid, planUuid, items }. */
const insertApiPlan = async (apiPlan, t) => {
    const exec = t || db;
    const items = apiPlan.items.map(({ priceUuid, planRef, priceRef, meterRef }) => ({
        priceUuid, planRef, priceRef, meterRef: meterRef || null,
    }));
    await exec.execute(
        `INSERT INTO ${API_PLANS} (org_uuid, portal_id, api_uuid, plan_uuid, items, created_at)
         VALUES (?, ?, ?, ?, ?, ?)`,
        [apiPlan.orgId, getPortalId(), apiPlan.apiUuid, apiPlan.planUuid,
            db.binaryParam(Buffer.from(JSON.stringify(items), 'utf8')), new Date()]
    );
};

/** The API plan of (API, plan), or null. */
const getApiPlan = async (orgId, apiUuid, planUuid, t) => {
    const exec = t || db;
    return toApiPlan(await exec.queryOne(
        `SELECT ${COLUMNS} FROM ${API_PLANS} ap
         WHERE ap.org_uuid = ? AND ap.api_uuid = ? AND ap.plan_uuid = ? AND ap.portal_id = ?`,
        [orgId, apiUuid, planUuid, getPortalId()]
    ));
};

/** The items of the API plan of (API, plan); none when there is no API plan. */
const listItems = async (orgId, apiUuid, planUuid, t) => {
    const apiPlan = await getApiPlan(orgId, apiUuid, planUuid, t);
    return apiPlan ? apiPlan.items : [];
};

/**
 * The organization's API plans, narrowed to one API with `apiUuid`, with their items and the API's and
 * plan's names (api_name, api_version, plan_name), ordered by them.
 */
const listByOrg = async (orgId, { apiUuid } = {}, t) => {
    const exec = t || db;
    const params = [orgId, getPortalId()];
    let filter = '';
    if (apiUuid) {
        filter = ' AND ap.api_uuid = ?';
        params.push(apiUuid);
    }
    const rows = await exec.query(
        `SELECT ${COLUMNS}, a.name AS api_name, a.version AS api_version, p.display_name AS plan_name
         FROM ${API_PLANS} ap
         JOIN api_metadata a ON a.uuid = ap.api_uuid AND a.portal_id = ap.portal_id
         JOIN subscription_plans p ON p.uuid = ap.plan_uuid AND p.portal_id = ap.portal_id
         WHERE ap.org_uuid = ? AND ap.portal_id = ?${filter}
         ORDER BY a.name, a.version, p.display_name`,
        params
    );
    return rows.map(toApiPlan);
};

/** The plan uuids of the API that have an API plan. */
const listPlanIds = async (orgId, apiUuid, t) => {
    const exec = t || db;
    const rows = await exec.query(
        `SELECT plan_uuid FROM ${API_PLANS} WHERE org_uuid = ? AND api_uuid = ? AND portal_id = ?`,
        [orgId, apiUuid, getPortalId()]
    );
    return new Set(rows.map((r) => r.plan_uuid));
};

/** Deletes the API plans of the given (API, plan) pairs ([{ apiUuid, planUuid }]). */
const deleteApiPlans = async (orgId, pairs, t) => {
    const exec = t || db;
    for (const { apiUuid, planUuid } of pairs) {
        await exec.execute(
            `DELETE FROM ${API_PLANS} WHERE org_uuid = ? AND api_uuid = ? AND plan_uuid = ? AND portal_id = ?`,
            [orgId, apiUuid, planUuid, getPortalId()]
        );
    }
};

module.exports = {
    listPaidPairs,
    insertApiPlan,
    getApiPlan,
    listItems,
    listByOrg,
    listPlanIds,
    deleteApiPlans,
};
