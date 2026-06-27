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
const APIKeyAppMapping = require('../models/apiKeyAppMapping');
const { APIMetadata } = require('../models/apiMetadata');
const { Application } = require('../models/application');

const API_METADATA_INCLUDE = {
    model: APIMetadata, as: 'DP_API_METADATA',
    attributes: ['ID', 'NAME', 'VERSION', 'HANDLE']
};

function appMappingInclude(required = false, appId = null) {
    const opts = {
        model: APIKeyAppMapping,
        required,
        include: [{ model: Application, attributes: ['ID', 'NAME'] }],
    };
    if (appId) opts.where = { APP_ID: appId };
    return opts;
}

async function create({ apiId, subscriptionId, appId, orgId, name, expiresAt, createdBy }, transaction) {
    const key = await APIKey.create(
        { API_ID: apiId, SUBSCRIPTION_ID: subscriptionId || null, ORG_ID: orgId,
          NAME: name, EXPIRES_AT: expiresAt || null, CREATED_BY: createdBy, STATUS: 'ACTIVE' },
        { transaction }
    );
    if (appId) {
        await APIKeyAppMapping.create({ KEY_ID: key.ID, APP_ID: appId }, { transaction });
    }
    return key;
}

async function get(orgId, keyId, transaction) {
    return APIKey.findOne({
        where: { ID: keyId, ORG_ID: orgId },
        include: [API_METADATA_INCLUDE, appMappingInclude()],
        transaction
    });
}

async function list(orgId, { apiId, subscriptionId, appId, status, limit } = {}, transaction) {
    const where = { ORG_ID: orgId };
    if (apiId) where.API_ID = apiId;
    if (subscriptionId) where.SUBSCRIPTION_ID = subscriptionId;
    if (status) where.STATUS = status;
    return APIKey.findAll({
        where,
        order: [['CREATED_AT', 'DESC']],
        include: [API_METADATA_INCLUDE, appMappingInclude(!!appId, appId)],
        ...(limit && { limit }),
        transaction
    });
}

async function revoke(orgId, keyId, transaction) {
    const [count] = await APIKey.update(
        { STATUS: 'REVOKED', REVOKED_AT: new Date() },
        { where: { ID: keyId, ORG_ID: orgId, STATUS: 'ACTIVE' }, transaction }
    );
    return count > 0;
}

async function setApplication(orgId, keyId, appId, transaction, { activeOnly = false } = {}) {
    const where = { ID: keyId, ORG_ID: orgId };
    if (activeOnly) where.STATUS = 'ACTIVE';
    const key = await APIKey.findOne({ where, transaction, lock: transaction ? true : false });
    if (!key) return false;
    if (appId) {
        await APIKeyAppMapping.upsert({ KEY_ID: keyId, APP_ID: appId }, { transaction });
    } else {
        await APIKeyAppMapping.destroy({ where: { KEY_ID: keyId }, transaction });
    }
    return true;
}

module.exports = { create, get, list, revoke, setApplication };
