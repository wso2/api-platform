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
const { Sequelize } = require('sequelize');
const { KeyManager } = require('../models/keyManager');
const { createCryptoUtil } = require('../utils/cryptoUtil');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');

const kmCrypto = createCryptoUtil(config.advanced.encryptionKey);

/**
 * Create a new key manager for an organization.
 * Admin credentials are encrypted before storage.
 */
const create = async (orgId, kmData) => {
    try {
        if (!kmCrypto.enabled) {
            throw new Error('Key manager encryption key is not configured. ' +
                'Set config.advanced.encryptionKey to a 64-char hex string.');
        }
        const record = await KeyManager.create({
            ORG_UUID: orgId,
            NAME: kmData.name,
            TYPE: kmData.type,
            ...(kmData.enabled !== undefined && { ENABLED: kmData.enabled }),
            TOKEN_ENDPOINT: kmData.tokenEndpoint,
            CLIENT_REG_ENDPOINT: kmData.clientRegistrationEndpoint,
            ...(kmData.issuer && { ISSUER: kmData.issuer }),
            ...(kmData.jwksURL && { JWKS_URL: kmData.jwksURL }),
            ADMIN_CLIENT_ID_ENC: kmCrypto.encrypt(kmData.adminClientId),
            ADMIN_CLIENT_SECRET_ENC: kmCrypto.encrypt(kmData.adminClientSecret),
            ...(kmData.supportedGrantTypes && { SUPPORTED_GRANT_TYPES: kmData.supportedGrantTypes }),
            ...(kmData.supportedScopes && { SUPPORTED_SCOPES: kmData.supportedScopes }),
            ...(kmData.additionalProperties && { ADDITIONAL_PROPERTIES: kmData.additionalProperties }),
        });
        return record;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        logger.error('Error creating key manager', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Update an existing key manager.
 * Re-encrypts admin credentials if they are provided.
 */
const update = async (kmId, kmData) => {
    try {
        const updatePayload = {
            ...(kmData.name && { NAME: kmData.name }),
            ...(kmData.type && { TYPE: kmData.type }),
            ...(kmData.enabled !== undefined && { ENABLED: kmData.enabled }),
            ...(kmData.tokenEndpoint && { TOKEN_ENDPOINT: kmData.tokenEndpoint }),
            ...(kmData.clientRegistrationEndpoint && { CLIENT_REG_ENDPOINT: kmData.clientRegistrationEndpoint }),
            ...(kmData.issuer !== undefined && { ISSUER: kmData.issuer }),
            ...(kmData.jwksURL !== undefined && { JWKS_URL: kmData.jwksURL }),
            ...(kmData.supportedGrantTypes && { SUPPORTED_GRANT_TYPES: kmData.supportedGrantTypes }),
            ...(kmData.supportedScopes && { SUPPORTED_SCOPES: kmData.supportedScopes }),
            ...(kmData.additionalProperties && { ADDITIONAL_PROPERTIES: kmData.additionalProperties }),
        };

        // Re-encrypt admin credentials if provided
        if (kmData.adminClientId) {
            if (!kmCrypto.enabled) {
                throw new Error('Key manager encryption key is not configured.');
            }
            updatePayload.ADMIN_CLIENT_ID_ENC = kmCrypto.encrypt(kmData.adminClientId);
        }
        if (kmData.adminClientSecret) {
            if (!kmCrypto.enabled) {
                throw new Error('Key manager encryption key is not configured.');
            }
            updatePayload.ADMIN_CLIENT_SECRET_ENC = kmCrypto.encrypt(kmData.adminClientSecret);
        }

        const [updatedRowsCount] = await KeyManager.update(updatePayload, {
            where: { UUID: kmId }
        });
        if (updatedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('Key manager not found');
        }
        // `returning: true` only yields row instances on Postgres; re-fetch
        // explicitly so the result is reliable on SQLite too.
        const updated = await KeyManager.findByPk(kmId);
        return [updatedRowsCount, [updated]];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        logger.error('Error updating key manager', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * List all key managers for an organization.
 * Returns raw records (encrypted fields included for internal use).
 */
const list = async (orgId) => {
    try {
        return await KeyManager.findAll({
            where: { ORG_UUID: orgId }
        });
    } catch (error) {
        logger.error('Error fetching key managers', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * List only enabled key managers for an organization.
 */
const listEnabled = async (orgId) => {
    try {
        return await KeyManager.findAll({
            where: { ORG_UUID: orgId, ENABLED: true }
        });
    } catch (error) {
        logger.error('Error fetching enabled key managers', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Get a single key manager by UUID.
 */
const get = async (kmId) => {
    try {
        const km = await KeyManager.findByPk(kmId);
        if (!km) {
            throw new Sequelize.EmptyResultError('Key manager not found');
        }
        return km;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        logger.error('Error fetching key manager', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Get a key manager by name within an organization.
 */
const getByName = async (orgId, name) => {
    try {
        const km = await KeyManager.findOne({
            where: { ORG_UUID: orgId, NAME: name }
        });
        if (!km) {
            throw new Sequelize.EmptyResultError('Key manager not found');
        }
        return km;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        logger.error('Error fetching key manager by name', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Delete a key manager.
 */
const deleteKm = async (kmId) => {
    try {
        const deleted = await KeyManager.destroy({
            where: { UUID: kmId }
        });
        if (deleted < 1) {
            throw new Sequelize.EmptyResultError('Key manager not found');
        }
        return deleted;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        logger.error('Error deleting key manager', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Decrypt admin credentials for a key manager record.
 * Used internally by adapters to make admin API calls.
 */
const decryptCredentials = (kmRecord) => {
    if (!kmCrypto.enabled) {
        throw new Error('Key manager encryption key is not configured.');
    }
    return {
        adminClientId: kmCrypto.decrypt(kmRecord.ADMIN_CLIENT_ID_ENC),
        adminClientSecret: kmCrypto.decrypt(kmRecord.ADMIN_CLIENT_SECRET_ENC),
    };
};

module.exports = {
    create,
    update,
    list,
    listEnabled,
    get,
    getByName,
    delete: deleteKm,
    decryptCredentials,
};
