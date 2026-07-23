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

const crypto = require('crypto');
const db = require('../db/driver');
const { indexBy } = require('../db/rows');
const { NotFoundError } = require('../utils/errors/customErrors');
const { createCryptoUtil } = require('../utils/cryptoUtil');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');

const subCrypto = createCryptoUtil(config.security.encryptionKey);

const SUBSCRIPTIONS_TABLE = 'dp_subscriptions';
const API_METADATA_TABLE = 'dp_api_metadata';
const SUBSCRIPTION_PLANS_TABLE = 'dp_subscription_plans';

// Matches the previous include's `attributes:` restriction on each association.
const API_METADATA_COLUMNS = 'uuid, name, version, handle, ref_id, type';
const SUBSCRIPTION_PLAN_COLUMNS = 'uuid, display_name, handle, ref_id';

function encryptToken(token) {
    return subCrypto.encrypt(token);
}

function decryptToken(value) {
    if (!value) return value;
    try {
        return subCrypto.decrypt(value);
    } catch (e) {
        logger.warn('Failed to decrypt subscription token — key mismatch or stale record', { error: e.message });
        return null;
    }
}

function decryptSubRecord(sub) {
    if (!sub) return sub;
    if (sub.token) sub.token = decryptToken(sub.token);
    return sub;
}

function generateSubToken() {
    return crypto.randomBytes(32).toString('hex');
}

/**
 * App-side "eager load" of each subscription's API and plan, mirroring the
 * previous INCLUDE_API_AND_PLAN shape: one query for the distinct API rows
 * and one for the distinct plan rows referenced by the batch, attached under
 * the same property names the old Sequelize associations produced —
 * `dp_api_metadata` (explicit `as:` alias) and `dp_subscription_plan`
 * (default singular association name, no `as:` was set on that belongsTo).
 * Both are `required: false` in the old include, so a subscription with no
 * matching row (e.g. plan_uuid is null) simply gets `undefined` attached.
 */
async function attachApiAndPlan(subs) {
    if (subs.length === 0) return subs;

    const apiIds = [...new Set(subs.map(s => s.api_uuid).filter(Boolean))];
    const planIds = [...new Set(subs.map(s => s.plan_uuid).filter(Boolean))];

    let apiByUuid = new Map();
    if (apiIds.length > 0) {
        const placeholders = apiIds.map(() => '?').join(', ');
        const apis = await db.query(
            `SELECT ${API_METADATA_COLUMNS} FROM ${API_METADATA_TABLE} WHERE uuid IN (${placeholders})`,
            apiIds
        );
        apiByUuid = indexBy(apis, 'uuid');
    }

    let planByUuid = new Map();
    if (planIds.length > 0) {
        const placeholders = planIds.map(() => '?').join(', ');
        const plans = await db.query(
            `SELECT ${SUBSCRIPTION_PLAN_COLUMNS} FROM ${SUBSCRIPTION_PLANS_TABLE} WHERE uuid IN (${placeholders})`,
            planIds
        );
        planByUuid = indexBy(plans, 'uuid');
    }

    for (const sub of subs) {
        sub.dp_api_metadata = apiByUuid.get(sub.api_uuid);
        sub.dp_subscription_plan = planByUuid.get(sub.plan_uuid);
    }
    return subs;
}

async function create(orgId, apiId, planId, createdBy, transaction, opts = {}) {
    const exec = transaction || db;
    const now = new Date();

    // If a token is provided externally (e.g. from Platform API), use it directly.
    if (opts.subToken) {
        const uuid = crypto.randomUUID();
        await exec.execute(
            `INSERT INTO ${SUBSCRIPTIONS_TABLE}
                (uuid, created_by, updated_by, org_uuid, api_uuid, plan_uuid, token, status, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
            [uuid, createdBy, createdBy, orgId, apiId, planId || null, encryptToken(opts.subToken), 'ACTIVE', now, now]
        );
        return {
            uuid,
            created_by: createdBy,
            updated_by: createdBy,
            org_uuid: orgId,
            api_uuid: apiId,
            plan_uuid: planId || null,
            // Expose the plaintext token to callers (never the encrypted form).
            token: opts.subToken,
            status: 'ACTIVE',
            created_at: now,
            updated_at: now,
        };
    }

    for (let attempt = 0; attempt < 3; attempt++) {
        const subToken = generateSubToken();
        const uuid = crypto.randomUUID();
        try {
            await exec.execute(
                `INSERT INTO ${SUBSCRIPTIONS_TABLE}
                    (uuid, created_by, updated_by, org_uuid, api_uuid, plan_uuid, token, status, created_at, updated_at)
                 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
                [uuid, createdBy, createdBy, orgId, apiId, planId || null, encryptToken(subToken), 'ACTIVE', now, now]
            );
            // Expose the plaintext token to callers (never the encrypted form).
            return {
                uuid,
                created_by: createdBy,
                updated_by: createdBy,
                org_uuid: orgId,
                api_uuid: apiId,
                plan_uuid: planId || null,
                token: subToken,
                status: 'ACTIVE',
                created_at: now,
                updated_at: now,
            };
        } catch (err) {
            const isTokenCollision = db.isDuplicateKeyError(err);
            if (isTokenCollision && attempt < 2) continue;
            throw err;
        }
    }
}

async function list(orgId, { apiId, createdBy } = {}) {
    const where = ['org_uuid = ?'];
    const params = [orgId];
    if (apiId) {
        where.push('api_uuid = ?');
        params.push(apiId);
    }
    if (createdBy) {
        where.push('created_by = ?');
        params.push(createdBy);
    }
    const rows = await db.query(
        `SELECT * FROM ${SUBSCRIPTIONS_TABLE} WHERE ${where.join(' AND ')} ORDER BY uuid ASC`,
        params
    );
    await attachApiAndPlan(rows);
    return rows.map(decryptSubRecord);
}

async function get(orgId, subId, createdBy) {
    const where = ['uuid = ?', 'org_uuid = ?'];
    const params = [subId, orgId];
    if (createdBy) {
        where.push('created_by = ?');
        params.push(createdBy);
    }
    const sub = await db.queryOne(`SELECT * FROM ${SUBSCRIPTIONS_TABLE} WHERE ${where.join(' AND ')}`, params);
    if (!sub) return null;
    await attachApiAndPlan([sub]);
    return decryptSubRecord(sub);
}

async function updateStatus(orgId, subId, status, createdBy, transaction) {
    const exec = transaction || db;
    const where = ['uuid = ?', 'org_uuid = ?'];
    const params = [subId, orgId];
    if (createdBy) {
        where.push('created_by = ?');
        params.push(createdBy);
    }
    const { rowCount } = await exec.execute(
        `UPDATE ${SUBSCRIPTIONS_TABLE} SET status = ?, updated_by = ?, updated_at = ? WHERE ${where.join(' AND ')}`,
        [status, createdBy, new Date(), ...params]
    );
    return rowCount > 0;
}

async function updatePlan(orgId, subId, planId, updatedBy, transaction) {
    const exec = transaction || db;
    const { rowCount } = await exec.execute(
        `UPDATE ${SUBSCRIPTIONS_TABLE} SET plan_uuid = ?, updated_by = ?, updated_at = ?
         WHERE uuid = ? AND org_uuid = ? AND created_by = ?`,
        [planId, updatedBy, new Date(), subId, orgId, updatedBy]
    );
    return rowCount > 0;
}

async function regenerateToken(orgId, subId, updatedBy, transaction) {
    const exec = transaction || db;
    for (let attempt = 0; attempt < 3; attempt++) {
        const newToken = generateSubToken();
        try {
            const { rowCount } = await exec.execute(
                `UPDATE ${SUBSCRIPTIONS_TABLE} SET token = ?, updated_by = ?, updated_at = ?
                 WHERE uuid = ? AND org_uuid = ? AND created_by = ?`,
                [encryptToken(newToken), updatedBy, new Date(), subId, orgId, updatedBy]
            );
            if (rowCount === 0) return null;
            return newToken;
        } catch (err) {
            const isTokenCollision = db.isDuplicateKeyError(err);
            if (isTokenCollision && attempt < 2) continue;
            throw err;
        }
    }
}

async function deleteSubscription(orgId, subId, createdBy, transaction) {
    const exec = transaction || db;
    const where = ['uuid = ?', 'org_uuid = ?'];
    const params = [subId, orgId];
    if (createdBy) {
        where.push('created_by = ?');
        params.push(createdBy);
    }
    const { rowCount } = await exec.execute(
        `DELETE FROM ${SUBSCRIPTIONS_TABLE} WHERE ${where.join(' AND ')}`,
        params
    );
    return rowCount > 0;
}

async function getById(orgId, subId) {
    const sub = await db.queryOne(
        `SELECT * FROM ${SUBSCRIPTIONS_TABLE} WHERE uuid = ? AND org_uuid = ?`,
        [subId, orgId]
    );
    if (!sub) return null;
    await attachApiAndPlan([sub]);
    return decryptSubRecord(sub);
}

const listByApi = async (orgId, apiId) => {
    return db.query(
        `SELECT * FROM ${SUBSCRIPTIONS_TABLE} WHERE org_uuid = ? AND api_uuid = ?`,
        [orgId, apiId]
    );
};

const listByOrg = async (orgId) => {
    return db.query(`SELECT * FROM ${SUBSCRIPTIONS_TABLE} WHERE org_uuid = ?`, [orgId]);
};

const listByUser = async (orgId, userId) => {
    try {
        return await db.query(
            `SELECT * FROM ${SUBSCRIPTIONS_TABLE} WHERE org_uuid = ? AND created_by = ?`,
            [orgId, userId]
        );
    } catch (error) {
        logger.error('listByUser failed', { error, orgId, userId });
        throw error;
    }
};

const findByKey = async (orgId, apiId, planId, t) => {
    const exec = t || db;
    try {
        return await exec.queryOne(
            `SELECT * FROM ${SUBSCRIPTIONS_TABLE} WHERE org_uuid = ? AND api_uuid = ? AND plan_uuid = ?`,
            [orgId, apiId, planId]
        );
    } catch (error) {
        if (error instanceof NotFoundError) return null;
        throw error;
    }
};

module.exports = {
    create,
    list,
    get,
    getById,
    updateStatus,
    updatePlan,
    regenerateToken,
    delete: deleteSubscription,
    listByApi,
    listByOrg,
    listByUser,
    findByKey,
};
