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
 * Data access for `billing_customers` (the payment provider's customer of a portal user, per billing
 * configuration) and `billing_checkouts` (each checkout started with the payment provider for a
 * paid plan).
 */

const crypto = require('crypto');
const db = require('../db/driver');
const { getPortalId } = require('../utils/orgContext');

const CUSTOMERS = 'billing_customers';
const CHECKOUTS = 'billing_checkouts';

const STATUS = Object.freeze({ OPEN: 'OPEN', COMPLETED: 'COMPLETED', EXPIRED: 'EXPIRED' });

const CHECKOUT_COLUMNS = 'uuid, org_uuid, user_uuid, api_uuid, plan_uuid, customer_uuid, '
    + 'checkout_ref, status, subscription_uuid, created_at, updated_at';

/** The user's customer under the billing configuration, or null. */
const findCustomer = async (configurationUuid, userUuid) => {
    return (await db.queryOne(
        `SELECT uuid, org_uuid, configuration_uuid, user_uuid, customer_ref FROM ${CUSTOMERS}
         WHERE configuration_uuid = ? AND user_uuid = ? AND portal_id = ?`,
        [configurationUuid, userUuid, getPortalId()]
    )) || null;
};

/** The customers of a billing configuration, as a Map of customer reference to row. */
const listCustomers = async (configurationUuid) => {
    const rows = await db.query(
        `SELECT uuid, org_uuid, configuration_uuid, user_uuid, customer_ref FROM ${CUSTOMERS}
         WHERE configuration_uuid = ? AND portal_id = ?`,
        [configurationUuid, getPortalId()]
    );
    return new Map(rows.map((r) => [r.customer_ref, r]));
};

const getCustomer = async (customerUuid) => {
    return (await db.queryOne(
        `SELECT uuid, org_uuid, configuration_uuid, user_uuid, customer_ref FROM ${CUSTOMERS}
         WHERE uuid = ? AND portal_id = ?`,
        [customerUuid, getPortalId()]
    )) || null;
};

/**
 * Records the user's customer. Two checkouts racing for the same user both create a customer with
 * the provider; the first record wins and is returned to both, so the user has one.
 */
const createCustomer = async ({ orgId, configurationUuid, userUuid, customerRef }) => {
    const now = new Date();
    try {
        await db.execute(
            `INSERT INTO ${CUSTOMERS} (uuid, org_uuid, portal_id, configuration_uuid, user_uuid,
                customer_ref, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
            [crypto.randomUUID(), orgId, getPortalId(), configurationUuid, userUuid, customerRef, now, now]
        );
    } catch (err) {
        if (!db.isDuplicateKeyError(err)) throw err;
    }
    return findCustomer(configurationUuid, userUuid);
};

const createCheckout = async ({ orgId, userUuid, apiUuid, planUuid, customerUuid }, t) => {
    const exec = t || db;
    const uuid = crypto.randomUUID();
    const now = new Date();
    await exec.execute(
        `INSERT INTO ${CHECKOUTS} (uuid, org_uuid, portal_id, user_uuid, api_uuid, plan_uuid,
            customer_uuid, status, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [uuid, orgId, getPortalId(), userUuid, apiUuid, planUuid, customerUuid, STATUS.OPEN, now, now]
    );
    return getCheckout(uuid, t);
};

/**
 * Locks the customer's row until the caller's transaction ends, by updating it, so that the
 * checkout starts of one subscriber take turns (startCheckout). Every database keeps an updated
 * row locked until commit, so this works the same on each.
 */
const lockCustomer = async (customerUuid, t) => {
    await t.execute(`UPDATE ${CUSTOMERS} SET updated_at = ? WHERE uuid = ? AND portal_id = ?`,
        [new Date(), customerUuid, getPortalId()]);
};

/** Deletes a checkout that never reached the payment provider. */
const deleteCheckout = async (checkoutUuid) => {
    await db.execute(`DELETE FROM ${CHECKOUTS} WHERE uuid = ? AND portal_id = ? AND checkout_ref IS NULL`,
        [checkoutUuid, getPortalId()]);
};

const getCheckout = async (checkoutUuid, t) => {
    const exec = t || db;
    return (await exec.queryOne(
        `SELECT ${CHECKOUT_COLUMNS} FROM ${CHECKOUTS} WHERE uuid = ? AND portal_id = ?`,
        [checkoutUuid, getPortalId()]
    )) || null;
};

const setCheckoutRef = async (checkoutUuid, checkoutRef) => {
    await db.execute(
        `UPDATE ${CHECKOUTS} SET checkout_ref = ?, updated_at = ? WHERE uuid = ? AND portal_id = ?`,
        [checkoutRef, new Date(), checkoutUuid, getPortalId()]
    );
};

/**
 * Moves an OPEN checkout to COMPLETED with its subscription. Resolves to false when another caller
 * completed (or expired) it first, so a checkout activates exactly once.
 */
const markCompleted = async (checkoutUuid, subscriptionUuid, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `UPDATE ${CHECKOUTS} SET status = ?, subscription_uuid = ?, updated_at = ?
         WHERE uuid = ? AND portal_id = ? AND status = ?`,
        [STATUS.COMPLETED, subscriptionUuid, new Date(), checkoutUuid, getPortalId(), STATUS.OPEN]
    );
    return rowCount === 1;
};

const markExpired = async (checkoutUuid) => {
    await db.execute(
        `UPDATE ${CHECKOUTS} SET status = ?, updated_at = ? WHERE uuid = ? AND portal_id = ? AND status = ?`,
        [STATUS.EXPIRED, new Date(), checkoutUuid, getPortalId(), STATUS.OPEN]
    );
};

/** The organization's open checkouts: started, not yet paid or ended. */
const listOpenByOrg = async (orgId) => {
    return db.query(
        `SELECT ${CHECKOUT_COLUMNS} FROM ${CHECKOUTS} WHERE org_uuid = ? AND status = ? AND portal_id = ?`,
        [orgId, STATUS.OPEN, getPortalId()]
    );
};

/** The organization's open checkouts for an API (`apiUuid`) or for a plan (`planUuid`). */
const listOpenFor = async (orgId, { apiUuid, planUuid }) => {
    const column = apiUuid ? 'api_uuid' : 'plan_uuid';
    return db.query(
        `SELECT ${CHECKOUT_COLUMNS} FROM ${CHECKOUTS} WHERE org_uuid = ? AND ${column} = ? AND status = ? AND portal_id = ?`,
        [orgId, apiUuid || planUuid, STATUS.OPEN, getPortalId()]
    );
};

/** The user's open checkout for the API, if one was started and has not ended. */
const findOpenCheckout = async (userUuid, apiUuid, t) => {
    const exec = t || db;
    return (await exec.queryOne(
        `SELECT ${CHECKOUT_COLUMNS} FROM ${CHECKOUTS}
         WHERE user_uuid = ? AND api_uuid = ? AND status = ? AND portal_id = ? ORDER BY created_at DESC`,
        [userUuid, apiUuid, STATUS.OPEN, getPortalId()]
    )) || null;
};

module.exports = {
    deleteCheckout,
    listCustomers,
    STATUS,
    findCustomer,
    getCustomer,
    createCustomer,
    createCheckout,
    lockCustomer,
    getCheckout,
    setCheckoutRef,
    markCompleted,
    markExpired,
    findOpenCheckout,
    listOpenByOrg,
    listOpenFor,
};
