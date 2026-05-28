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
const { SubscriptionMapping } = require('../models/application');
const { APIMetadata } = require('../models/apiMetadata');
const SubscriptionPolicy = require('../models/subscriptionPolicy');

const INCLUDE_API_AND_POLICY = [
    {
        model: APIMetadata,
        as: 'DP_API_METADATA',
        attributes: ['API_ID', 'API_NAME', 'API_VERSION', 'API_HANDLE', 'REFERENCE_ID', 'GATEWAY_TYPE'],
        required: false,
    },
    {
        model: SubscriptionPolicy,
        as: 'DP_SUBSCRIPTION_POLICY',
        attributes: ['POLICY_ID', 'POLICY_NAME', 'DISPLAY_NAME', 'REF_ID'],
        required: false,
    },
];

function generateSubToken() {
    return crypto.randomBytes(32).toString('hex');
}

async function createSubscription(orgId, apiId, policyId, transaction) {
    for (let attempt = 0; attempt < 3; attempt++) {
        const subToken = generateSubToken();
        try {
            return await SubscriptionMapping.create(
                {
                    APP_ID: null,
                    ORG_ID: orgId,
                    API_ID: apiId,
                    POLICY_ID: policyId || null,
                    SUB_TOKEN: subToken,
                    STATUS: 'ACTIVE',
                },
                { transaction }
            );
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

async function listSubscriptions(orgId, { apiId } = {}) {
    const where = { ORG_ID: orgId, APP_ID: null };
    if (apiId) where.API_ID = apiId;
    return SubscriptionMapping.findAll({
        where,
        include: INCLUDE_API_AND_POLICY,
        order: [['SUB_ID', 'ASC']],
    });
}

async function getSubscription(orgId, subId) {
    return SubscriptionMapping.findOne({
        where: { SUB_ID: subId, ORG_ID: orgId, APP_ID: null },
        include: INCLUDE_API_AND_POLICY,
    });
}

async function updateSubscriptionStatus(orgId, subId, status, transaction) {
    const [count] = await SubscriptionMapping.update(
        { STATUS: status },
        { where: { SUB_ID: subId, ORG_ID: orgId, APP_ID: null }, transaction }
    );
    return count > 0;
}

async function deleteSubscription(orgId, subId, transaction) {
    const count = await SubscriptionMapping.destroy({
        where: { SUB_ID: subId, ORG_ID: orgId, APP_ID: null },
        transaction,
    });
    return count > 0;
}

async function getSubscriptionById(orgId, subId) {
    return SubscriptionMapping.findOne({
        where: { SUB_ID: subId, ORG_ID: orgId },
        include: INCLUDE_API_AND_POLICY,
    });
}

module.exports = {
    createSubscription,
    listSubscriptions,
    getSubscription,
    getSubscriptionById,
    updateSubscriptionStatus,
    deleteSubscription,
};
