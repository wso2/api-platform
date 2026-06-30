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
const logger = require('../config/logger');

/**
 * Create a new key manager for an organization.
 */
const create = async (orgId, kmData, createdBy) => {
    try {
        const record = await KeyManager.create({
            ORG_UUID: orgId,
            NAME: kmData.name,
            TYPE: kmData.type,
            ...(kmData.enabled !== undefined && { ENABLED: kmData.enabled ? 1 : 0 }),
            TOKEN_ENDPOINT: kmData.tokenEndpoint,
            CREATED_BY: createdBy,
            UPDATED_BY: createdBy,
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
 */
const update = async (kmId, kmData, updatedBy) => {
    try {
        const updatePayload = {
            ...(kmData.name && { NAME: kmData.name }),
            ...(kmData.type && { TYPE: kmData.type }),
            ...(kmData.enabled !== undefined && { ENABLED: kmData.enabled ? 1 : 0 }),
            ...(kmData.tokenEndpoint && { TOKEN_ENDPOINT: kmData.tokenEndpoint }),
            UPDATED_BY: updatedBy,
            UPDATED_AT: new Date(),
        };

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
            where: { ORG_UUID: orgId, ENABLED: 1 }
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

module.exports = {
    create,
    update,
    list,
    listEnabled,
    get,
    getByName,
    delete: deleteKm,
};
