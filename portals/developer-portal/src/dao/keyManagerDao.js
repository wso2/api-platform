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
'use strict';

const crypto = require('crypto');
const db = require('../db/driver');
const { NotFoundError } = require('../utils/errors/customErrors');
const logger = require('../config/logger');

const TABLE = 'dp_key_managers';

/**
 * Create a new key manager for an organization.
 */
const create = async (orgId, kmData, createdBy) => {
    const uuid = crypto.randomUUID();
    const now = new Date();
    // Mirrors the previous conditional spread: an omitted `enabled` falls back to
    // the column's own default (1) rather than being written explicitly.
    const enabled = kmData.enabled !== undefined ? (kmData.enabled ? 1 : 0) : 1;

    try {
        await db.execute(
            `INSERT INTO ${TABLE} (uuid, org_uuid, handle, display_name, enabled, token_endpoint, created_by, created_at, updated_by, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
            [uuid, orgId, kmData.handle, kmData.displayName, enabled, kmData.tokenEndpoint, createdBy, now, createdBy, now]
        );
    } catch (error) {
        // Let the raw driver error (pg 23505 / sqlite UNIQUE / mssql 2601-2627) propagate
        // unchanged — callers classify it with db.isDuplicateKeyError(error), which inspects
        // those driver-specific fields directly and would not recognize a wrapped error.
        if (!db.isDuplicateKeyError(error)) {
            logger.error('Error creating key manager', { error });
        }
        throw error;
    }

    return {
        uuid,
        org_uuid: orgId,
        handle: kmData.handle,
        display_name: kmData.displayName,
        enabled,
        token_endpoint: kmData.tokenEndpoint,
        created_by: createdBy,
        created_at: now,
        updated_by: createdBy,
        updated_at: now,
    };
};

/**
 * Update an existing key manager.
 */
const update = async (kmId, kmData, updatedBy) => {
    const now = new Date();
    const setClauses = ['updated_by = ?', 'updated_at = ?'];
    const params = [updatedBy, now];
    if (kmData.handle) { setClauses.push('handle = ?'); params.push(kmData.handle); }
    if (kmData.displayName) { setClauses.push('display_name = ?'); params.push(kmData.displayName); }
    if (kmData.enabled !== undefined) { setClauses.push('enabled = ?'); params.push(kmData.enabled ? 1 : 0); }
    if (kmData.tokenEndpoint) { setClauses.push('token_endpoint = ?'); params.push(kmData.tokenEndpoint); }
    params.push(kmId);

    try {
        const { rowCount: updatedRowsCount } = await db.execute(
            `UPDATE ${TABLE} SET ${setClauses.join(', ')} WHERE uuid = ?`,
            params
        );
        if (updatedRowsCount < 1) {
            throw new NotFoundError('Key manager not found');
        }
        // Re-fetch explicitly so the result is reliable across every dialect.
        const updated = await db.queryOne(`SELECT * FROM ${TABLE} WHERE uuid = ?`, [kmId]);
        return [updatedRowsCount, [updated]];
    } catch (error) {
        if (error instanceof NotFoundError || db.isDuplicateKeyError(error)) {
            throw error;
        }
        logger.error('Error updating key manager', { error });
        throw error;
    }
};

/**
 * List all key managers for an organization.
 */
const list = async (orgId) => {
    try {
        return await db.query(`SELECT * FROM ${TABLE} WHERE org_uuid = ?`, [orgId]);
    } catch (error) {
        logger.error('Error fetching key managers', { error });
        throw error;
    }
};

/**
 * List only enabled key managers for an organization.
 */
const listEnabled = async (orgId) => {
    try {
        return await db.query(`SELECT * FROM ${TABLE} WHERE org_uuid = ? AND enabled = ?`, [orgId, 1]);
    } catch (error) {
        logger.error('Error fetching enabled key managers', { error });
        throw error;
    }
};

/**
 * Get a single key manager by UUID.
 */
const get = async (kmId) => {
    try {
        const km = await db.queryOne(`SELECT * FROM ${TABLE} WHERE uuid = ?`, [kmId]);
        if (!km) {
            throw new NotFoundError('Key manager not found');
        }
        return km;
    } catch (error) {
        if (error instanceof NotFoundError) {
            throw error;
        }
        logger.error('Error fetching key manager', { error });
        throw error;
    }
};

/**
 * Get a key manager by handle within an organization.
 */
const getByHandle = async (orgId, handle) => {
    try {
        const km = await db.queryOne(`SELECT * FROM ${TABLE} WHERE org_uuid = ? AND handle = ?`, [orgId, handle]);
        if (!km) {
            throw new NotFoundError('Key manager not found');
        }
        return km;
    } catch (error) {
        if (error instanceof NotFoundError) {
            throw error;
        }
        logger.error('Error fetching key manager by handle', { error });
        throw error;
    }
};

/**
 * Resolve a key manager's handle to its internal uuid, or null if not found.
 */
const getIdByHandle = async (orgId, handle) => {
    const km = await db.queryOne(`SELECT uuid FROM ${TABLE} WHERE org_uuid = ? AND handle = ?`, [orgId, handle]);
    return km ? km.uuid : null;
};

/**
 * Delete a key manager.
 */
const deleteKm = async (kmId) => {
    try {
        const { rowCount: deleted } = await db.execute(`DELETE FROM ${TABLE} WHERE uuid = ?`, [kmId]);
        if (deleted < 1) {
            throw new NotFoundError('Key manager not found');
        }
        return deleted;
    } catch (error) {
        if (error instanceof NotFoundError) {
            throw error;
        }
        logger.error('Error deleting key manager', { error });
        throw error;
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
