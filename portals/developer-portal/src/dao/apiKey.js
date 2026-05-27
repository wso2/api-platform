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
const { Op } = require('sequelize');
const APIKey = require('../models/apiKey');
const { APIMetadata } = require('../models/apiMetadata');

async function createKey({ apiId, subscriptionId, orgId, name, expiresAt, createdBy }, transaction) {
    return APIKey.create(
        { API_ID: apiId, SUBSCRIPTION_ID: subscriptionId || null, ORG_ID: orgId,
          NAME: name, EXPIRES_AT: expiresAt || null, CREATED_BY: createdBy, STATUS: 'ACTIVE' },
        { transaction }
    );
}

async function getKey(orgId, keyId, transaction) {
    return APIKey.findOne({
        where: { KEY_ID: keyId, ORG_ID: orgId },
        include: [{ model: APIMetadata, attributes: ['API_ID', 'GATEWAY_TYPE', 'API_NAME'] }],
        transaction
    });
}

async function listKeys(orgId, { apiId, subscriptionId, status } = {}) {
    const where = { ORG_ID: orgId };
    if (apiId) where.API_ID = apiId;
    if (subscriptionId) where.SUBSCRIPTION_ID = subscriptionId;
    if (status) where.STATUS = status;
    return APIKey.findAll({
        where,
        order: [['CREATED_AT', 'DESC']],
        include: [{ model: APIMetadata, attributes: ['API_ID', 'GATEWAY_TYPE', 'API_NAME'] }]
    });
}

async function revokeKey(orgId, keyId, transaction) {
    const [count] = await APIKey.update(
        { STATUS: 'REVOKED', REVOKED_AT: new Date() },
        { where: { KEY_ID: keyId, ORG_ID: orgId, STATUS: 'ACTIVE' }, transaction }
    );
    return count > 0;
}

module.exports = { createKey, getKey, listKeys, revokeKey };
