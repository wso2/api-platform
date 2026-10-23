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
 * Data access for `subscription_billing`: the payment provider's subscription and the billing status
 * of each paid portal subscription.
 */

const db = require('../db/driver');
const { getPortalId } = require('../utils/orgContext');

const TABLE = 'subscription_billing';
const COLUMNS = 'subscription_uuid, org_uuid, configuration_uuid, customer_uuid, api_plan_uuid, '
    + 'billing_subscription_id, billing_status, current_period_start, current_period_end, ends_at, '
    + 'created_at, updated_at';

const create = async (b, t) => {
    const exec = t || db;
    const now = new Date();
    await exec.execute(
        `INSERT INTO ${TABLE} (subscription_uuid, org_uuid, portal_id, configuration_uuid, customer_uuid,
            api_plan_uuid, billing_subscription_id, billing_status, current_period_start, current_period_end,
            created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [b.subscriptionUuid, b.orgId, getPortalId(), b.configurationUuid, b.customerUuid, b.apiPlanUuid,
            b.subscriptionRef, b.billingStatus, b.currentPeriodStart || null, b.currentPeriodEnd || null,
            now, now]
    );
};

const getBySubscription = async (subscriptionUuid, t) => {
    const exec = t || db;
    return (await exec.queryOne(
        `SELECT ${COLUMNS} FROM ${TABLE} WHERE subscription_uuid = ? AND portal_id = ?`,
        [subscriptionUuid, getPortalId()]
    )) || null;
};

/** The billing rows of the given subscriptions, as a Map of subscription uuid to row. */
const listBySubscriptions = async (subscriptionUuids, t) => {
    if (!subscriptionUuids || subscriptionUuids.length === 0) return new Map();
    const exec = t || db;
    const placeholders = subscriptionUuids.map(() => '?').join(', ');
    const rows = await exec.query(
        `SELECT ${COLUMNS} FROM ${TABLE} WHERE subscription_uuid IN (${placeholders}) AND portal_id = ?`,
        [...subscriptionUuids, getPortalId()]
    );
    return new Map(rows.map((r) => [r.subscription_uuid, r]));
};

/** The billing rows of an organization's paid subscriptions. */
const listByOrg = async (orgId, t) => {
    const exec = t || db;
    return exec.query(`SELECT ${COLUMNS} FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ?`, [orgId, getPortalId()]);
};

const getBySubscriptionRef = async (subscriptionRef) => {
    return (await db.queryOne(
        `SELECT ${COLUMNS} FROM ${TABLE} WHERE billing_subscription_id = ? AND portal_id = ?`,
        [subscriptionRef, getPortalId()]
    )) || null;
};

/**
 * Records the latest state Stripe reports, including when the subscription is set to end (null
 * when it is not).
 */
const updateState = async (subscriptionUuid, {
    billingStatus, currentPeriodStart, currentPeriodEnd, endsAt,
}, t) => {
    const exec = t || db;
    await exec.execute(
        `UPDATE ${TABLE} SET billing_status = ?, current_period_start = ?, current_period_end = ?, ends_at = ?,
            updated_at = ?
         WHERE subscription_uuid = ? AND portal_id = ?`,
        [billingStatus, currentPeriodStart || null, currentPeriodEnd || null, endsAt || null,
            new Date(), subscriptionUuid, getPortalId()]
    );
};

/** Records when the subscription ends, once its end has been scheduled with the payment provider. */
const setEndsAt = async (subscriptionUuid, endsAt, t) => {
    const exec = t || db;
    await exec.execute(
        `UPDATE ${TABLE} SET ends_at = ?, updated_at = ? WHERE subscription_uuid = ? AND portal_id = ?`,
        [endsAt, new Date(), subscriptionUuid, getPortalId()]
    );
};

module.exports = {
    create, getBySubscription, listBySubscriptions, listByOrg, getBySubscriptionRef, updateState, setEndsAt,
};
