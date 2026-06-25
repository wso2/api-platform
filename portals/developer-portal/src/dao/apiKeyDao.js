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
const { Application } = require('../models/application');

const APPLICATION_INCLUDE = { model: Application, attributes: ['APP_ID', 'NAME'] };
const API_METADATA_INCLUDE = {
    model: APIMetadata, as: 'DP_API_METADATA',
    attributes: ['API_ID', 'API_NAME', 'API_VERSION', 'API_HANDLE']
};

async function create({ apiId, subscriptionId, appId, orgId, name, expiresAt, createdBy }, transaction) {
    return APIKey.create(
        { API_ID: apiId, SUBSCRIPTION_ID: subscriptionId || null, APP_ID: appId || null, ORG_ID: orgId,
          NAME: name, EXPIRES_AT: expiresAt || null, CREATED_BY: createdBy, STATUS: 'ACTIVE' },
        { transaction }
    );
}

async function get(orgId, keyId, transaction) {
    return APIKey.findOne({
        where: { KEY_ID: keyId, ORG_ID: orgId },
        include: [API_METADATA_INCLUDE, APPLICATION_INCLUDE],
        transaction
    });
}

async function list(orgId, { apiId, subscriptionId, appId, status, limit } = {}, transaction) {
    const where = { ORG_ID: orgId };
    if (apiId) where.API_ID = apiId;
    if (subscriptionId) where.SUBSCRIPTION_ID = subscriptionId;
    if (appId) where.APP_ID = appId;
    if (status) where.STATUS = status;
    return APIKey.findAll({
        where,
        order: [['CREATED_AT', 'DESC']],
        include: [API_METADATA_INCLUDE, APPLICATION_INCLUDE],
        ...(limit && { limit }),
        transaction
    });
}

async function revoke(orgId, keyId, transaction) {
    const [count] = await APIKey.update(
        { STATUS: 'REVOKED', REVOKED_AT: new Date() },
        { where: { KEY_ID: keyId, ORG_ID: orgId, STATUS: 'ACTIVE' }, transaction }
    );
    return count > 0;
}

async function setApplication(orgId, keyId, appId, transaction, { activeOnly = false } = {}) {
    const where = { KEY_ID: keyId, ORG_ID: orgId };
    if (activeOnly) where.STATUS = 'ACTIVE';
    const [count] = await APIKey.update(
        { APP_ID: appId || null },
        { where, transaction }
    );
    return count > 0;
}

module.exports = { create, get, list, revoke, setApplication };
