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
const { toBlobBuffer } = require('../db/rows');
const { NotFoundError } = require('../utils/errors/customErrors');
const viewDao = require('./viewDao');
const constants = require('../utils/constants');

const ORG_TABLE = 'dp_organizations';
const ORG_CONTENT_TABLE = 'dp_organization_assets';

const create = async (orgData, t) => {
    const exec = t || db;
    const devPortalId = orgData.handle ? orgData.handle.toLowerCase() : '';
    const uuid = crypto.randomUUID();

    await exec.execute(
        `INSERT INTO ${ORG_TABLE}
            (uuid, display_name, business_owner, business_owner_contact, business_owner_email,
             handle, idp_ref_id, cp_ref_id, configuration, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
            uuid, orgData.displayName, orgData.businessOwner, orgData.businessOwnerContact,
            orgData.businessOwnerEmail, devPortalId, orgData.idpRefId, orgData.cpRefId,
            orgData.configuration, orgData.createdBy, orgData.createdBy,
        ]
    );
    return {
        uuid,
        display_name: orgData.displayName,
        business_owner: orgData.businessOwner,
        business_owner_contact: orgData.businessOwnerContact,
        business_owner_email: orgData.businessOwnerEmail,
        handle: devPortalId,
        idp_ref_id: orgData.idpRefId,
        cp_ref_id: orgData.cpRefId,
        configuration: orgData.configuration,
        created_by: orgData.createdBy,
        updated_by: orgData.createdBy,
    };
};

// Matches by handle, then name, then idp_ref_id, in that priority order — deterministic
// even if one org's handle happens to equal another org's name or idp_ref_id, unlike a
// single Op.or query (which returns whichever row the DB orders first).
const findOrgByIdentifier = async (param, t) => {
    const exec = t || db;
    const handle = typeof param === 'string' ? param.toLowerCase() : param;
    return (await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE handle = ?`, [handle])) ||
        (await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE display_name = ?`, [param])) ||
        (await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE idp_ref_id = ?`, [param]));
};

const get = async (param, t) => {
    const organization = await findOrgByIdentifier(param, t);
    if (!organization) {
        throw new NotFoundError('Organization not found');
    }
    return organization;
};

// For internal callers that already hold a resolved org uuid (e.g. req.orgId set by
// auth middleware) — not for public REST lookups, which should use get()/handle instead.
const getByUuid = async (uuid, t) => {
    const exec = t || db;
    const organization = await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE uuid = ?`, [uuid]);
    if (!organization) {
        throw new NotFoundError('Organization not found');
    }
    return organization;
};

const getId = async (orgName) => {
    const organization = await findOrgByIdentifier(orgName);
    if (!organization) {
        throw new NotFoundError('Organization not found');
    }
    return organization.uuid;
};

const list = async () => {
    return db.query(`SELECT * FROM ${ORG_TABLE}`);
};

const update = async (orgData, t) => {
    const exec = t || db;
    const existing = await get(orgData.orgId, t);
    const devPortalId = orgData.handle ? orgData.handle.toLowerCase() : existing.handle;
    const updatedAt = new Date();

    const setClauses = [
        'display_name = ?', 'business_owner = ?', 'business_owner_contact = ?',
        'business_owner_email = ?', 'handle = ?', 'idp_ref_id = ?', 'updated_by = ?', 'updated_at = ?',
    ];
    const params = [
        orgData.displayName, orgData.businessOwner, orgData.businessOwnerContact,
        orgData.businessOwnerEmail, devPortalId, orgData.idpRefId, orgData.updatedBy, updatedAt,
    ];
    if (orgData.cpRefId !== undefined) {
        setClauses.push('cp_ref_id = ?');
        params.push(orgData.cpRefId);
    }
    if (orgData.configuration !== undefined) {
        setClauses.push('configuration = ?');
        params.push(orgData.configuration);
    }
    params.push(existing.uuid);

    const { rowCount } = await exec.execute(
        `UPDATE ${ORG_TABLE} SET ${setClauses.join(', ')} WHERE uuid = ?`,
        params
    );
    if (rowCount < 1) {
        throw new NotFoundError('Organization not found');
    }
    // Some dialects don't support RETURNING on UPDATE — re-fetch explicitly instead
    // (same pattern as applicationDao.update).
    const updatedOrg = await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE uuid = ?`, [existing.uuid]);
    return [rowCount, [updatedOrg]];
};

// Tables whose org_uuid FK is ON DELETE NO ACTION (database/schema.*.sql) block
// deleting the organization row unless their rows are removed first. Tables with
// ON DELETE CASCADE/SET NULL (dp_api_metadata, dp_subscription_plans, dp_audit,
// dp_user_organization_mappings, and the *_mappings join tables) are left to the
// database to handle and aren't touched here.
const deleteOrgDependents = async (orgUuid, t) => {
    const exec = t || db;

    const events = await exec.query('SELECT uuid FROM dp_events WHERE org_uuid = ?', [orgUuid]);
    if (events.length) {
        const placeholders = events.map(() => '?').join(', ');
        await exec.execute(
            `DELETE FROM dp_event_deliveries WHERE event_uuid IN (${placeholders})`,
            events.map((e) => e.uuid)
        );
    }
    await exec.execute('DELETE FROM dp_events WHERE org_uuid = ?', [orgUuid]);

    await exec.execute('DELETE FROM dp_api_keys WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM dp_subscriptions WHERE org_uuid = ?', [orgUuid]);

    const [apps, keyManagers] = await Promise.all([
        exec.query('SELECT uuid FROM dp_applications WHERE org_uuid = ?', [orgUuid]),
        exec.query('SELECT uuid FROM dp_key_managers WHERE org_uuid = ?', [orgUuid]),
    ]);
    if (apps.length || keyManagers.length) {
        const conditions = [];
        const params = [];
        if (apps.length) {
            conditions.push(`app_uuid IN (${apps.map(() => '?').join(', ')})`);
            params.push(...apps.map((a) => a.uuid));
        }
        if (keyManagers.length) {
            conditions.push(`km_uuid IN (${keyManagers.map(() => '?').join(', ')})`);
            params.push(...keyManagers.map((k) => k.uuid));
        }
        await exec.execute(`DELETE FROM dp_app_key_mappings WHERE ${conditions.join(' OR ')}`, params);
    }
    await exec.execute('DELETE FROM dp_applications WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM dp_key_managers WHERE org_uuid = ?', [orgUuid]);

    await exec.execute('DELETE FROM dp_api_workflows WHERE org_uuid = ?', [orgUuid]);
    await exec.execute(`DELETE FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ?`, [orgUuid]);
    // dp_view_label_mappings/dp_api_label_mappings cascade automatically from
    // dp_views/dp_labels ON DELETE CASCADE.
    await exec.execute('DELETE FROM dp_views WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM dp_labels WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM dp_tags WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM dp_webhook_subscribers WHERE org_uuid = ?', [orgUuid]);
};

const deleteOrg = async (orgId, t) => {
    const exec = t || db;
    const existing = await get(orgId, t);
    await deleteOrgDependents(existing.uuid, t);
    const { rowCount } = await exec.execute(`DELETE FROM ${ORG_TABLE} WHERE uuid = ?`, [existing.uuid]);
    if (rowCount < 1) {
        throw new NotFoundError('Organization not found');
    }
    return rowCount;
};

const createContent = async (orgData, t) => {
    const exec = t || db;
    const viewId = await viewDao.getId(orgData.orgId, orgData.viewName);
    const uuid = crypto.randomUUID();
    const content = toBlobBuffer(orgData.fileContent);
    await exec.execute(
        `INSERT INTO ${ORG_CONTENT_TABLE}
            (uuid, file_type, file_name, file_content, file_path, org_uuid, view_uuid, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
            uuid, orgData.fileType, orgData.fileName, content, orgData.filePath,
            orgData.orgId, viewId, orgData.createdBy, orgData.createdBy,
        ]
    );
    return {
        uuid,
        file_type: orgData.fileType,
        file_name: orgData.fileName,
        file_content: content,
        file_path: orgData.filePath,
        org_uuid: orgData.orgId,
        view_uuid: viewId,
        created_by: orgData.createdBy,
        updated_by: orgData.createdBy,
    };
};

const updateContent = async (orgData) => {
    const viewId = await viewDao.getId(orgData.orgId, orgData.viewName);
    const updatedAt = new Date();
    const content = toBlobBuffer(orgData.fileContent);
    const { rowCount } = await db.execute(
        `UPDATE ${ORG_CONTENT_TABLE}
         SET file_type = ?, file_name = ?, file_content = ?, file_path = ?, updated_by = ?, updated_at = ?
         WHERE file_type = ? AND file_name = ? AND file_path = ? AND org_uuid = ? AND view_uuid = ?`,
        [
            orgData.fileType, orgData.fileName, content, orgData.filePath, orgData.updatedBy, updatedAt,
            orgData.fileType, orgData.fileName, orgData.filePath, orgData.orgId, viewId,
        ]
    );
    if (rowCount < 1) {
        throw new NotFoundError('No new resources found');
    }
    const updatedOrgContent = await db.query(
        `SELECT * FROM ${ORG_CONTENT_TABLE}
         WHERE file_type = ? AND file_name = ? AND file_path = ? AND org_uuid = ? AND view_uuid = ?`,
        [orgData.fileType, orgData.fileName, orgData.filePath, orgData.orgId, viewId]
    );
    return [rowCount, updatedOrgContent];
};

const getContent = async (orgData) => {
    const viewId = await viewDao.getId(orgData.orgId, orgData.viewName);
    if (orgData.fileName || orgData.filePath) {
        const conditions = ['org_uuid = ?', 'view_uuid = ?', 'file_type = ?'];
        const params = [orgData.orgId, viewId, orgData.fileType];
        if (orgData.fileName) {
            conditions.push('file_name = ?');
            params.push(orgData.fileName);
        }
        if (orgData.filePath) {
            conditions.push('file_path = ?');
            params.push(orgData.filePath);
        }
        return db.queryOne(`SELECT * FROM ${ORG_CONTENT_TABLE} WHERE ${conditions.join(' AND ')}`, params);
    }
    return db.query(
        `SELECT * FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ? AND view_uuid = ? AND file_type = ?`,
        [orgData.orgId, viewId, orgData.fileType]
    );
};

const deleteContent = async (orgId, viewName, fileName) => {
    const viewId = await viewDao.getId(orgId, viewName);
    const { rowCount } = await db.execute(
        `DELETE FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ? AND view_uuid = ? AND file_name = ?`,
        [orgId, viewId, fileName]
    );
    if (rowCount < 1) {
        throw new NotFoundError('Organization content not found');
    }
    return rowCount;
};

// Deletes only theme-related content rows (style/layout/partial/markDown/template/image) for
// the view — scoped so a theme reset/replace never touches unrelated per-view assets like
// llms-config.json, which shares this same table.
const deleteThemeContent = async (orgId, viewName, t) => {
    const exec = t || db;
    const viewId = await viewDao.getId(orgId, viewName);
    const placeholders = constants.THEME_FILE_TYPES.map(() => '?').join(', ');
    const { rowCount } = await exec.execute(
        `DELETE FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ? AND view_uuid = ? AND file_type IN (${placeholders})`,
        [orgId, viewId, ...constants.THEME_FILE_TYPES]
    );
    return rowCount;
};

const hasThemeContent = async (orgId, viewName) => {
    const viewId = await viewDao.getId(orgId, viewName);
    if (!viewId) return false;
    const placeholders = constants.THEME_FILE_TYPES.map(() => '?').join(', ');
    const rows = await db.query(
        `SELECT 1 AS found FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ? AND view_uuid = ? AND file_type IN (${placeholders})`,
        [orgId, viewId, ...constants.THEME_FILE_TYPES]
    );
    return rows.length > 0;
};

module.exports = {
    create,
    get,
    getByUuid,
    getId,
    list,
    update,
    delete: deleteOrg,
    createContent,
    updateContent,
    getContent,
    deleteContent,
    deleteThemeContent,
    hasThemeContent,
};
