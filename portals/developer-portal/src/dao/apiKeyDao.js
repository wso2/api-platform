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
const constants = require('../utils/constants');

const API_METADATA_INCLUDE = {
    model: APIMetadata,
    as: 'dp_api_metadata',
    attributes: ['uuid', 'name', 'version', 'handle', 'ref_id', 'type']
};

function appMappingInclude(required = false, appId = null) {
    const opts = {
        model: APIKeyAppMapping,
        required,
        include: [{ model: Application, attributes: ['uuid', 'display_name', 'handle'] }],
    };
    if (appId) opts.where = { app_uuid: appId };
    return opts;
}

async function create({ apiId, subscriptionId, appId, orgId, name, expiresAt, createdBy }, transaction) {
    const key = await APIKey.create(
        { api_uuid: apiId, subscription_uuid: subscriptionId || null, org_uuid: orgId,
          name: name, expires_at: expiresAt || null, created_by: createdBy, updated_by: createdBy, status: constants.API_KEY_STATUS.ACTIVE },
        { transaction }
    );
    if (appId) {
        await APIKeyAppMapping.create({ key_uuid: key.uuid, app_uuid: appId, created_by: createdBy }, { transaction });
    }
    return key;
}

async function get(orgId, keyId, transaction) {
    return APIKey.findOne({
        where: { uuid: keyId, org_uuid: orgId },
        include: [API_METADATA_INCLUDE, appMappingInclude()],
        transaction
    });
}

async function list(orgId, { apiId, subscriptionId, appId, status, limit } = {}, transaction) {
    const where = { org_uuid: orgId };
    if (apiId) where.api_uuid = apiId;
    if (subscriptionId) where.subscription_uuid = subscriptionId;
    if (status) where.status = status;
    return APIKey.findAll({
        where,
        order: [['created_at', 'DESC']],
        include: [API_METADATA_INCLUDE, appMappingInclude(!!appId, appId)],
        ...(limit && { limit }),
        transaction
    });
}

async function revoke(orgId, keyId, updatedBy, transaction) {
    const [count] = await APIKey.update(
        { status: constants.API_KEY_STATUS.REVOKED, revoked_at: new Date(), revoked_by: updatedBy, updated_by: updatedBy },
        { where: { uuid: keyId, org_uuid: orgId, status: constants.API_KEY_STATUS.ACTIVE }, transaction }
    );
    return count > 0;
}

async function setApplication(orgId, keyId, appId, updatedBy, transaction, { activeOnly = false } = {}) {
    const where = { uuid: keyId, org_uuid: orgId };
    if (activeOnly) where.status = constants.API_KEY_STATUS.ACTIVE;
    const key = await APIKey.findOne({ where, transaction, lock: transaction ? true : false });
    if (!key) return false;
    if (appId) {
        await APIKeyAppMapping.upsert({ key_uuid: keyId, app_uuid: appId, created_by: updatedBy }, { transaction });
    } else {
        await APIKeyAppMapping.destroy({ where: { key_uuid: keyId }, transaction });
    }
    return true;
}

async function updateExpiry(orgId, keyId, expiresAt, updatedBy, transaction) {
    const [count] = await APIKey.update(
        { expires_at: expiresAt, updated_by: updatedBy, updated_at: new Date() },
        { where: { uuid: keyId, org_uuid: orgId, status: constants.API_KEY_STATUS.ACTIVE }, transaction }
    );
    return count > 0;
}

module.exports = { create, get, list, revoke, setApplication, updateExpiry };
