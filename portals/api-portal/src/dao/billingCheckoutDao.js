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
 * Data access for `billing_customers` (the payment provider's customer of a portal user, one per user
 * per organization) and `billing_checkouts` (each checkout started with the payment provider for a
 * paid plan). A checkout is open until it names the subscription it created; one that ends unpaid
 * is deleted. Checkout rows read here carry `status`, OPEN or COMPLETED, worked out from that.
 */

const crypto = require('crypto');
const db = require('../db/driver');
const { getPortalId } = require('../utils/orgContext');

const CUSTOMERS = 'billing_customers';
const CHECKOUTS = 'billing_checkouts';

const STATUS = Object.freeze({ OPEN: 'OPEN', COMPLETED: 'COMPLETED' });

const CUSTOMER_COLUMNS = 'org_uuid, user_uuid, customer_ref, created_at';
const CHECKOUT_COLUMNS = 'uuid, org_uuid, user_uuid, api_uuid, plan_uuid, checkout_ref, subscription_uuid, created_at';

function toCheckout(row) {
    if (!row) return null;
    return { ...row, status: row.subscription_uuid ? STATUS.COMPLETED : STATUS.OPEN };
}

/** The user's customer in the organization, or null. */
const findCustomer = async (orgId, userUuid, t) => {
    const exec = t || db;
    return (await exec.queryOne(
        `SELECT ${CUSTOMER_COLUMNS} FROM ${CUSTOMERS} WHERE org_uuid = ? AND user_uuid = ? AND portal_id = ?`,
        [orgId, userUuid, getPortalId()]
    )) || null;
};

/** The organization's customers, as a Map of customer reference to row. */
const listCustomers = async (orgId) => {
    const rows = await db.query(
        `SELECT ${CUSTOMER_COLUMNS} FROM ${CUSTOMERS} WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]
    );
    return new Map(rows.map((r) => [r.customer_ref, r]));
};

/**
 * Records the user's customer. Two checkouts racing for the same user both create a customer with
 * the provider; the first record wins and is returned to both, so the user has one.
 */
const createCustomer = async ({ orgId, userUuid, customerRef }) => {
    try {
        await db.execute(
            `INSERT INTO ${CUSTOMERS} (org_uuid, portal_id, user_uuid, customer_ref, created_at) VALUES (?, ?, ?, ?, ?)`,
            [orgId, getPortalId(), userUuid, customerRef, new Date()]
        );
    } catch (err) {
        if (!db.isDuplicateKeyError(err)) throw err;
    }
    return findCustomer(orgId, userUuid);
};

const createCheckout = async ({ orgId, userUuid, apiUuid, planUuid }, t) => {
    const exec = t || db;
    const uuid = crypto.randomUUID();
    await exec.execute(
        `INSERT INTO ${CHECKOUTS} (uuid, org_uuid, portal_id, user_uuid, api_uuid, plan_uuid, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [uuid, orgId, getPortalId(), userUuid, apiUuid, planUuid, new Date()]
    );
    return getCheckout(uuid, t);
};

/**
 * Locks the customer's row until the caller's transaction ends, by updating it, so that the
 * checkout starts of one subscriber take turns (startCheckout). Every database keeps an updated
 * row locked until commit, so this works the same on each.
 */
const lockCustomer = async (orgId, userUuid, t) => {
    await t.execute(
        `UPDATE ${CUSTOMERS} SET customer_ref = customer_ref WHERE org_uuid = ? AND user_uuid = ? AND portal_id = ?`,
        [orgId, userUuid, getPortalId()]
    );
};

/** Deletes a checkout that is still open. */
const deleteOpenCheckout = async (checkoutUuid) => {
    await db.execute(`DELETE FROM ${CHECKOUTS} WHERE uuid = ? AND portal_id = ? AND subscription_uuid IS NULL`,
        [checkoutUuid, getPortalId()]);
};

const getCheckout = async (checkoutUuid, t) => {
    const exec = t || db;
    return toCheckout(await exec.queryOne(
        `SELECT ${CHECKOUT_COLUMNS} FROM ${CHECKOUTS} WHERE uuid = ? AND portal_id = ?`,
        [checkoutUuid, getPortalId()]
    ));
};

const setCheckoutRef = async (checkoutUuid, checkoutRef) => {
    await db.execute(
        `UPDATE ${CHECKOUTS} SET checkout_ref = ? WHERE uuid = ? AND portal_id = ?`,
        [checkoutRef, checkoutUuid, getPortalId()]
    );
};

/** Completes an open checkout with its subscription. Resolves to false when it is no longer open. */
const markCompleted = async (checkoutUuid, subscriptionUuid, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `UPDATE ${CHECKOUTS} SET subscription_uuid = ? WHERE uuid = ? AND portal_id = ? AND subscription_uuid IS NULL`,
        [subscriptionUuid, checkoutUuid, getPortalId()]
    );
    return rowCount === 1;
};

/** The organization's open checkouts: started, not yet paid or ended. */
const listOpenByOrg = async (orgId) => {
    const rows = await db.query(
        `SELECT ${CHECKOUT_COLUMNS} FROM ${CHECKOUTS} WHERE org_uuid = ? AND subscription_uuid IS NULL AND portal_id = ?`,
        [orgId, getPortalId()]
    );
    return rows.map(toCheckout);
};

/** The organization's open checkouts for an API (`apiUuid`) or for a plan (`planUuid`). */
const listOpenFor = async (orgId, { apiUuid, planUuid }) => {
    const column = apiUuid ? 'api_uuid' : 'plan_uuid';
    const rows = await db.query(
        `SELECT ${CHECKOUT_COLUMNS} FROM ${CHECKOUTS}
         WHERE org_uuid = ? AND ${column} = ? AND subscription_uuid IS NULL AND portal_id = ?`,
        [orgId, apiUuid || planUuid, getPortalId()]
    );
    return rows.map(toCheckout);
};

/** The user's open checkout for the API, if one was started and has not ended. */
const findOpenCheckout = async (orgId, userUuid, apiUuid, t) => {
    const exec = t || db;
    return toCheckout(await exec.queryOne(
        `SELECT ${CHECKOUT_COLUMNS} FROM ${CHECKOUTS}
         WHERE org_uuid = ? AND user_uuid = ? AND api_uuid = ? AND subscription_uuid IS NULL AND portal_id = ?
         ORDER BY created_at DESC`,
        [orgId, userUuid, apiUuid, getPortalId()]
    ));
};

module.exports = {
    STATUS,
    findCustomer,
    listCustomers,
    createCustomer,
    createCheckout,
    lockCustomer,
    deleteOpenCheckout,
    getCheckout,
    setCheckoutRef,
    markCompleted,
    findOpenCheckout,
    listOpenByOrg,
    listOpenFor,
};
