/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

const APPLICATION_TABLE = 'dp_applications';
const KEY_MAPPING_TABLE = 'dp_app_key_mappings';

const create = async (orgId, userId, appData) => {
    const uuid = crypto.randomUUID();
    const handle = appData.handle || appData.displayName;
    await db.execute(
        `INSERT INTO ${APPLICATION_TABLE} (uuid, display_name, handle, org_uuid, description, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [uuid, appData.displayName, handle, orgId, appData.description, userId, userId]
    );
    return {
        uuid,
        display_name: appData.displayName,
        handle,
        org_uuid: orgId,
        description: appData.description,
        created_by: userId,
        updated_by: userId,
    };
};

const update = async (orgId, appId, userId, appData) => {
    const updatedAt = new Date();
    const { rowCount } = await db.execute(
        `UPDATE ${APPLICATION_TABLE} SET display_name = ?, description = ?, updated_by = ?, updated_at = ?
         WHERE org_uuid = ? AND uuid = ? AND created_by = ?`,
        [appData.displayName, appData.description, userId, updatedAt, orgId, appId, userId]
    );
    if (!rowCount) {
        return [rowCount, null];
    }
    const updatedApp = await db.queryOne(
        `SELECT * FROM ${APPLICATION_TABLE} WHERE org_uuid = ? AND uuid = ?`,
        [orgId, appId]
    );
    return [rowCount, [updatedApp]];
};

const get = async (orgId, appId, userId, t) => {
    const exec = t || db;
    return exec.queryOne(
        `SELECT * FROM ${APPLICATION_TABLE} WHERE org_uuid = ? AND uuid = ? AND created_by = ?`,
        [orgId, appId, userId]
    );
};

const getId = async (orgId, userId, handle) => {
    return db.queryOne(
        `SELECT uuid FROM ${APPLICATION_TABLE} WHERE org_uuid = ? AND created_by = ? AND handle = ?`,
        [orgId, userId, handle]
    );
};

const list = async (orgId, userId) => {
    return db.query(
        `SELECT * FROM ${APPLICATION_TABLE} WHERE org_uuid = ? AND created_by = ?`,
        [orgId, userId]
    );
};

const deleteApp = async (orgId, appId, userId, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `DELETE FROM ${APPLICATION_TABLE} WHERE org_uuid = ? AND uuid = ? AND created_by = ?`,
        [orgId, appId, userId]
    );
    if (rowCount < 1) {
        throw new NotFoundError('Application not found');
    }
    return rowCount;
};

/**
 * Application row with its key mappings attached as `dp_app_key_mappings`
 * (matching the table name — see src/dto/applicationDto.js). Mirrors the old
 * Sequelize `include` with a `where` on the association, which Sequelize
 * implicitly treats as an inner join: returns null (not the bare application)
 * when the application has zero key mappings.
 */
const getKeyMapping = async (orgId, appId, t) => {
    const exec = t || db;
    const application = await exec.queryOne(
        `SELECT * FROM ${APPLICATION_TABLE} WHERE org_uuid = ? AND uuid = ?`,
        [orgId, appId]
    );
    if (!application) return null;

    const mappings = await exec.query(
        `SELECT * FROM ${KEY_MAPPING_TABLE} WHERE app_uuid = ?`,
        [appId]
    );
    if (mappings.length === 0) return null;

    return { ...application, dp_app_key_mappings: mappings };
};

const upsertKeyMapping = async (mappingData, t) => {
    const exec = t || db;
    const kmId = mappingData.kmId ?? null;
    const kmCondition = kmId === null ? 'km_uuid IS NULL' : 'km_uuid = ?';
    const findParams = kmId === null
        ? [mappingData.appId, mappingData.type]
        : [mappingData.appId, kmId, mappingData.type];

    const existing = await exec.queryOne(
        `SELECT * FROM ${KEY_MAPPING_TABLE} WHERE app_uuid = ? AND ${kmCondition} AND type = ?`,
        findParams
    );

    if (existing) {
        const updatedAt = new Date();
        await exec.execute(
            `UPDATE ${KEY_MAPPING_TABLE} SET as_client_id = ?, updated_by = ?, updated_at = ? WHERE uuid = ?`,
            [mappingData.asClientId, mappingData.createdBy, updatedAt, existing.uuid]
        );
        return { ...existing, as_client_id: mappingData.asClientId, updated_by: mappingData.createdBy, updated_at: updatedAt };
    }

    const uuid = crypto.randomUUID();
    await exec.execute(
        `INSERT INTO ${KEY_MAPPING_TABLE} (uuid, app_uuid, km_uuid, as_client_id, type, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [uuid, mappingData.appId, mappingData.kmId || null, mappingData.asClientId, mappingData.type, mappingData.createdBy, mappingData.createdBy]
    );
    return {
        uuid,
        app_uuid: mappingData.appId,
        km_uuid: mappingData.kmId || null,
        as_client_id: mappingData.asClientId,
        type: mappingData.type,
        created_by: mappingData.createdBy,
        updated_by: mappingData.createdBy,
    };
};

const deleteMappings = async (orgId, appId, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(`DELETE FROM ${KEY_MAPPING_TABLE} WHERE app_uuid = ?`, [appId]);
    if (rowCount < 1) {
        logger.debug('No Application Key Mapping found', {
            orgId,
            appId,
            deletedRowsCount: rowCount,
            operation: 'deleteApplicationKeyMapping',
        });
    }
    return rowCount;
};

/**
 * Deletes only the given mapping ids that actually belong to an application in
 * this org — the join against dp_applications.org_uuid is a tenant-isolation
 * check, not just a convenience filter; do not drop it in favor of a bare
 * `uuid IN (...)` delete.
 */
const deleteMappingsByIds = async (orgId, mappingIds, t) => {
    if (!mappingIds || mappingIds.length === 0) return 0;
    const exec = t || db;
    const idPlaceholders = mappingIds.map(() => '?').join(', ');
    const ownedMappings = await exec.query(
        `SELECT m.uuid FROM ${KEY_MAPPING_TABLE} m
         JOIN ${APPLICATION_TABLE} a ON m.app_uuid = a.uuid
         WHERE m.uuid IN (${idPlaceholders}) AND a.org_uuid = ?`,
        [...mappingIds, orgId]
    );
    const ownedIds = ownedMappings.map((m) => m.uuid);
    if (ownedIds.length === 0) return 0;

    const ownedPlaceholders = ownedIds.map(() => '?').join(', ');
    const { rowCount } = await exec.execute(
        `DELETE FROM ${KEY_MAPPING_TABLE} WHERE uuid IN (${ownedPlaceholders})`,
        ownedIds
    );
    return rowCount;
};

const getKeyMappings = async (orgId, appId, t) => {
    const exec = t || db;
    return exec.query(`SELECT * FROM ${KEY_MAPPING_TABLE} WHERE app_uuid = ?`, [appId]);
};

const getKeyMappingById = async (appId, mappingId, t) => {
    const exec = t || db;
    return exec.queryOne(`SELECT * FROM ${KEY_MAPPING_TABLE} WHERE uuid = ? AND app_uuid = ?`, [mappingId, appId]);
};

const deleteKeyMappingById = async (appId, mappingId, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `DELETE FROM ${KEY_MAPPING_TABLE} WHERE uuid = ? AND app_uuid = ?`,
        [mappingId, appId]
    );
    return rowCount;
};

const createKeyMapping = async (mappingData, t) => {
    const exec = t || db;
    const uuid = crypto.randomUUID();
    await exec.execute(
        `INSERT INTO ${KEY_MAPPING_TABLE} (uuid, app_uuid, km_uuid, as_client_id, type, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [
            uuid, mappingData.appId, mappingData.kmId || null, mappingData.asClientId || null,
            mappingData.type || 'PRODUCTION', mappingData.createdBy, mappingData.createdBy,
        ]
    );
    return {
        uuid,
        app_uuid: mappingData.appId,
        km_uuid: mappingData.kmId || null,
        as_client_id: mappingData.asClientId || null,
        type: mappingData.type || 'PRODUCTION',
        created_by: mappingData.createdBy,
        updated_by: mappingData.createdBy,
    };
};

module.exports = {
    create,
    update,
    get,
    getId,
    list,
    delete: deleteApp,
    getKeyMapping,
    upsertKeyMapping,
    deleteMappings,
    deleteMappingsByIds,
    getKeyMappings,
    getKeyMappingById,
    deleteKeyMappingById,
    createKeyMapping,
};
