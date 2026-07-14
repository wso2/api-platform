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
            org_uuid: orgId,
            handle: kmData.handle,
            display_name: kmData.displayName,
            ...(kmData.enabled !== undefined && { enabled: kmData.enabled ? 1 : 0 }),
            token_endpoint: kmData.tokenEndpoint,
            created_by: createdBy,
            updated_by: createdBy,
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
            ...(kmData.handle && { handle: kmData.handle }),
            ...(kmData.displayName && { display_name: kmData.displayName }),
            ...(kmData.enabled !== undefined && { enabled: kmData.enabled ? 1 : 0 }),
            ...(kmData.tokenEndpoint && { token_endpoint: kmData.tokenEndpoint }),
            updated_by: updatedBy,
            updated_at: new Date(),
        };

        const [updatedRowsCount] = await KeyManager.update(updatePayload, {
            where: { uuid: kmId }
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
            where: { org_uuid: orgId }
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
            where: { org_uuid: orgId, enabled: 1 }
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
 * Get a key manager by handle within an organization.
 */
const getByHandle = async (orgId, handle) => {
    try {
        const km = await KeyManager.findOne({
            where: { org_uuid: orgId, handle }
        });
        if (!km) {
            throw new Sequelize.EmptyResultError('Key manager not found');
        }
        return km;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        logger.error('Error fetching key manager by handle', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Resolve a key manager's handle to its internal uuid, or null if not found.
 */
const getIdByHandle = async (orgId, handle) => {
    const km = await KeyManager.findOne({ where: { org_uuid: orgId, handle }, attributes: ['uuid'] });
    return km ? km.uuid : null;
};

/**
 * Delete a key manager.
 */
const deleteKm = async (kmId) => {
    try {
        const deleted = await KeyManager.destroy({
            where: { uuid: kmId }
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
    getByHandle,
    getIdByHandle,
    delete: deleteKm,
};
