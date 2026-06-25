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
    if (dv.SUB_TOKEN) dv.SUB_TOKEN = decryptToken(dv.SUB_TOKEN);
    return sub;
}

const INCLUDE_API_AND_PLAN = [
    {
        model: APIMetadata,
        as: 'DP_API_METADATA',
        attributes: ['ID', 'NAME', 'VERSION', 'HANDLE', 'REFERENCE_ID'],
        required: false,
    },
    {
        model: SubscriptionPlan,
        as: 'DP_SUBSCRIPTION_PLAN',
        attributes: ['ID', 'NAME', 'DISPLAY_NAME', 'REF_ID'],
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
                CREATED_BY: createdBy,
                ORG_ID: orgId,
                API_ID: apiId,
                PLAN_ID: planId || null,
                SUB_TOKEN: encryptToken(opts.subToken),
                STATUS: 'ACTIVE',
            },
            { transaction }
        );
        record.dataValues.SUB_TOKEN = opts.subToken;
        return record;
    }

    for (let attempt = 0; attempt < 3; attempt++) {
        const subToken = generateSubToken();
        try {
            const record = await SubscriptionMapping.create(
                {
                    CREATED_BY: createdBy,
                    ORG_ID: orgId,
                    API_ID: apiId,
                    PLAN_ID: planId || null,
                    SUB_TOKEN: encryptToken(subToken),
                    STATUS: 'ACTIVE',
                },
                { transaction }
            );
            // Expose the plaintext token to callers (never the encrypted form).
            record.dataValues.SUB_TOKEN = subToken;
            return record;
        } catch (err) {
            const isTokenCollision =
                err.name === 'SequelizeUniqueConstraintError' &&
                err.fields && Object.keys(err.fields).some(
                    f => f.includes('SUB_TOKEN') || f.includes('sub_token')
                );
            if (isTokenCollision && attempt < 2) continue;
            throw err;
        }
    }
}

async function list(orgId, { apiId, createdBy } = {}) {
    const where = { ORG_ID: orgId };
    if (apiId) where.API_ID = apiId;
    if (createdBy) where.CREATED_BY = createdBy;
    const rows = await SubscriptionMapping.findAll({
        where,
        include: INCLUDE_API_AND_PLAN,
        order: [['ID', 'ASC']],
    });
    return rows.map(decryptSubRecord);
}

async function get(orgId, subId, createdBy) {
    const where = { ID: subId, ORG_ID: orgId };
    if (createdBy) where.CREATED_BY = createdBy;
    return decryptSubRecord(await SubscriptionMapping.findOne({
        where,
        include: INCLUDE_API_AND_PLAN,
    }));
}

async function updateStatus(orgId, subId, status, createdBy, transaction) {
    const where = { ID: subId, ORG_ID: orgId };
    if (createdBy) where.CREATED_BY = createdBy;
    const [count] = await SubscriptionMapping.update(
        { STATUS: status },
        { where, transaction }
    );
    return count > 0;
}

async function deleteSubscription(orgId, subId, createdBy, transaction) {
    const where = { ID: subId, ORG_ID: orgId };
    if (createdBy) where.CREATED_BY = createdBy;
    const count = await SubscriptionMapping.destroy({ where, transaction });
    return count > 0;
}

async function getById(orgId, subId) {
    return decryptSubRecord(await SubscriptionMapping.findOne({
        where: { ID: subId, ORG_ID: orgId },
        include: INCLUDE_API_AND_PLAN,
    }));
}

const listByApi = async (orgID, apiID) => {
    try {
        return await SubscriptionMapping.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    API_ID: apiID,
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const listByOrg = async (orgID) => {
    try {
        return await SubscriptionMapping.findAll({
            where: { ORG_ID: orgID },
        });
    } catch (error) {
        throw new Sequelize.DatabaseError(error);
    }
};

const listByUser = async (orgID, userID) => {
    try {
        return await SubscriptionMapping.findAll({
            where: { ORG_ID: orgID, CREATED_BY: userID },
        });
    } catch (error) {
        logger.error('listByUser failed', { error, orgID, userID });
        throw new Sequelize.DatabaseError(error);
    }
};

const findByKey = async (orgID, apiID, planID, t) => {
    try {
        return await SubscriptionMapping.findOne({
            where: { ORG_ID: orgID, API_ID: apiID, PLAN_ID: planID },
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
    delete: deleteSubscription,
    listByApi,
    listByOrg,
    listByUser,
    findByKey,
};
