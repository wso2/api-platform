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
const crypto = require('crypto');
const { SubscriptionMapping, Application } = require('../models/application');
const { APIMetadata } = require('../models/apiMetadata');
const SubscriptionPlan = require('../models/subscriptionPlan');
const { createCryptoUtil } = require('../utils/cryptoUtil');
const { config } = require('../config/configLoader');
const { Sequelize } = require('sequelize');
const logger = require('../config/logger');

const subCrypto = createCryptoUtil(config.advanced.encryptionKey);

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
    const dv = sub.dataValues || sub;
    if (dv.token) dv.token = decryptToken(dv.token);
    return sub;
}

const INCLUDE_API_AND_PLAN = [
    {
        model: APIMetadata,
        attributes: ['uuid', 'name', 'version', 'handle', 'ref_id'],
        required: false,
    },
    {
        model: SubscriptionPlan,
        attributes: ['uuid', 'name', 'ref_id'],
        required: false,
    },
];

function generateSubToken() {
    return crypto.randomBytes(32).toString('hex');
}

async function create(orgId, apiId, planId, createdBy, transaction, opts = {}) {
    // If a token is provided externally (e.g. from Platform API), use it directly.
    if (opts.subToken) {
        const record = await SubscriptionMapping.create(
            {
                created_by: createdBy,
                updated_by: createdBy,
                org_uuid: orgId,
                api_uuid: apiId,
                plan_uuid: planId || null,
                token: encryptToken(opts.subToken),
                status: 'ACTIVE',
            },
            { transaction }
        );
        record.dataValues.token = opts.subToken;
        return record;
    }

    for (let attempt = 0; attempt < 3; attempt++) {
        const subToken = generateSubToken();
        try {
            const record = await SubscriptionMapping.create(
                {
                    created_by: createdBy,
                    updated_by: createdBy,
                    org_uuid: orgId,
                    api_uuid: apiId,
                    plan_uuid: planId || null,
                    token: encryptToken(subToken),
                    status: 'ACTIVE',
                },
                { transaction }
            );
            // Expose the plaintext token to callers (never the encrypted form).
            record.dataValues.token = subToken;
            return record;
        } catch (err) {
            const isTokenCollision =
                err.name === 'SequelizeUniqueConstraintError' &&
                err.fields && Object.keys(err.fields).some(
                    f => f.includes('token') || f.includes('sub_token')
                );
            if (isTokenCollision && attempt < 2) continue;
            throw err;
        }
    }
}

async function list(orgId, { apiId, createdBy } = {}) {
    const where = { org_uuid: orgId };
    if (apiId) where.api_uuid = apiId;
    if (createdBy) where.created_by = createdBy;
    const rows = await SubscriptionMapping.findAll({
        where,
        include: INCLUDE_API_AND_PLAN,
        order: [['uuid', 'ASC']],
    });
    return rows.map(decryptSubRecord);
}

async function get(orgId, subId, createdBy) {
    const where = { uuid: subId, org_uuid: orgId };
    if (createdBy) where.created_by = createdBy;
    return decryptSubRecord(await SubscriptionMapping.findOne({
        where,
        include: INCLUDE_API_AND_PLAN,
    }));
}

async function updateStatus(orgId, subId, status, createdBy, transaction) {
    const where = { uuid: subId, org_uuid: orgId };
    if (createdBy) where.created_by = createdBy;
    const [count] = await SubscriptionMapping.update(
        { status: status, updated_by: createdBy, updated_at: new Date() },
        { where, transaction }
    );
    return count > 0;
}

async function updatePlan(orgId, subId, planId, updatedBy, transaction) {
    const where = { UUID: subId, ORG_UUID: orgId, CREATED_BY: updatedBy };
    const [count] = await SubscriptionMapping.update(
        { PLAN_UUID: planId, UPDATED_BY: updatedBy, UPDATED_AT: new Date() },
        { where, transaction }
    );
    return count > 0;
}

async function regenerateToken(orgId, subId, updatedBy, transaction) {
    const where = { UUID: subId, ORG_UUID: orgId, CREATED_BY: updatedBy };
    for (let attempt = 0; attempt < 3; attempt++) {
        const newToken = generateSubToken();
        try {
            const [count] = await SubscriptionMapping.update(
                { TOKEN: encryptToken(newToken), UPDATED_BY: updatedBy, UPDATED_AT: new Date() },
                { where, transaction }
            );
            if (count === 0) return null;
            return newToken;
        } catch (err) {
            const isTokenCollision =
                err.name === 'SequelizeUniqueConstraintError' &&
                err.fields && Object.keys(err.fields).some(
                    f => f.includes('TOKEN') || f.includes('sub_token')
                );
            if (isTokenCollision && attempt < 2) continue;
            throw err;
        }
    }
}

async function deleteSubscription(orgId, subId, createdBy, transaction) {
    const where = { uuid: subId, org_uuid: orgId };
    if (createdBy) where.created_by = createdBy;
    const count = await SubscriptionMapping.destroy({ where, transaction });
    return count > 0;
}

async function getById(orgId, subId) {
    return decryptSubRecord(await SubscriptionMapping.findOne({
        where: { uuid: subId, org_uuid: orgId },
        include: INCLUDE_API_AND_PLAN,
    }));
}

const listByApi = async (orgId, apiId) => {
    try {
        return await SubscriptionMapping.findAll(
            {
                where: {
                    org_uuid: orgId,
                    api_uuid: apiId,
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const listByOrg = async (orgId) => {
    try {
        return await SubscriptionMapping.findAll({
            where: { org_uuid: orgId },
        });
    } catch (error) {
        throw new Sequelize.DatabaseError(error);
    }
};

const listByUser = async (orgId, userId) => {
    try {
        return await SubscriptionMapping.findAll({
            where: { org_uuid: orgId, created_by: userId },
        });
    } catch (error) {
        logger.error('listByUser failed', { error, orgId, userId });
        throw new Sequelize.DatabaseError(error);
    }
};

const findByKey = async (orgId, apiId, planId, t) => {
    try {
        return await SubscriptionMapping.findOne({
            where: { org_uuid: orgId, api_uuid: apiId, plan_uuid: planId },
            transaction: t,
        });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) return null;
        throw new Sequelize.DatabaseError(error);
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
