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
    attributes: ['UUID', 'NAME', 'VERSION', 'HANDLE']
};

function appMappingInclude(required = false, appId = null) {
    const opts = {
        model: APIKeyAppMapping,
        required,
        include: [{ model: Application, attributes: ['UUID', 'NAME'] }],
    };
    if (appId) opts.where = { APP_UUID: appId };
    return opts;
}

async function create({ apiId, subscriptionId, appId, orgId, name, expiresAt, createdBy }, transaction) {
    const key = await APIKey.create(
        { API_UUID: apiId, SUBSCRIPTION_UUID: subscriptionId || null, ORG_UUID: orgId,
          NAME: name, EXPIRES_AT: expiresAt || null, CREATED_BY: createdBy, UPDATED_BY: createdBy, STATUS: 'ACTIVE' },
        { transaction }
    );
    if (appId) {
        await APIKeyAppMapping.create({ KEY_UUID: key.UUID, APP_UUID: appId, CREATED_BY: createdBy }, { transaction });
    }
    return key;
}

async function get(orgId, keyId, transaction) {
    return APIKey.findOne({
        where: { UUID: keyId, ORG_UUID: orgId },
        include: [API_METADATA_INCLUDE, appMappingInclude()],
        transaction
    });
}

async function list(orgId, { apiId, subscriptionId, appId, status, limit } = {}, transaction) {
    const where = { ORG_UUID: orgId };
    if (apiId) where.API_UUID = apiId;
    if (subscriptionId) where.SUBSCRIPTION_UUID = subscriptionId;
    if (status) where.STATUS = status;
    return APIKey.findAll({
        where,
        order: [['CREATED_AT', 'DESC']],
        include: [API_METADATA_INCLUDE, appMappingInclude(!!appId, appId)],
        ...(limit && { limit }),
        transaction
    });
}

async function revoke(orgId, keyId, updatedBy, transaction) {
    const [count] = await APIKey.update(
        { STATUS: 'REVOKED', REVOKED_AT: new Date(), UPDATED_BY: updatedBy },
        { where: { UUID: keyId, ORG_UUID: orgId, STATUS: 'ACTIVE' }, transaction }
    );
    return count > 0;
}

async function setApplication(orgId, keyId, appId, updatedBy, transaction, { activeOnly = false } = {}) {
    const where = { UUID: keyId, ORG_UUID: orgId };
    if (activeOnly) where.STATUS = 'ACTIVE';
    const key = await APIKey.findOne({ where, transaction, lock: transaction ? true : false });
    if (!key) return false;
    if (appId) {
        await APIKeyAppMapping.upsert({ KEY_UUID: keyId, APP_UUID: appId, CREATED_BY: updatedBy }, { transaction });
    } else {
        await APIKeyAppMapping.destroy({ where: { KEY_UUID: keyId }, transaction });
    }
    return true;
}

module.exports = { create, get, list, revoke, setApplication };
