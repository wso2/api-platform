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
const { toBlobBuffer, parseJsonColumn } = require('../db/rows');
const { NotFoundError } = require('../utils/errors/customErrors');
const viewDao = require('./viewDao');
const constants = require('../utils/constants');

const ORG_TABLE = 'organizations';
const ORG_CONTENT_TABLE = 'organization_assets';

const getPortalId = () => require('../utils/orgContext').getPortalId();

const create = async (orgData, t) => {
    const exec = t || db;
    const orgHandle = orgData.handle ? orgData.handle.toLowerCase() : '';
    const uuid = crypto.randomUUID();

    const portalId = getPortalId();
    await exec.execute(
        `INSERT INTO ${ORG_TABLE}
            (uuid, portal_id, display_name, business_owner, business_owner_contact, business_owner_email,
             handle, idp_ref_id, cp_ref_id, configuration, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
            uuid, portalId, orgData.displayName, orgData.businessOwner, orgData.businessOwnerContact,
            orgData.businessOwnerEmail, orgHandle, orgData.idpRefId, orgData.cpRefId,
            orgData.configuration, orgData.createdBy, orgData.createdBy,
        ]
    );
    return {
        uuid,
        portal_id: portalId,
        display_name: orgData.displayName,
        business_owner: orgData.businessOwner,
        business_owner_contact: orgData.businessOwnerContact,
        business_owner_email: orgData.businessOwnerEmail,
        handle: orgHandle,
        idp_ref_id: orgData.idpRefId,
        cp_ref_id: orgData.cpRefId,
        configuration: orgData.configuration,
        created_by: orgData.createdBy,
        updated_by: orgData.createdBy,
    };
};

/**
 * Normalizes an organization row. `configuration` is a JSON column: postgres
 * returns it already parsed, but sqlite (TEXT) and mssql (NVARCHAR) return a
 * string. Without this, every `org.configuration?.<key>` read is silently
 * undefined on those dialects. The API contract also declares `configuration`
 * as an object.
 */
const normalizeOrgRow = (row) => {
    if (!row) {
        return row;
    }
    return { ...row, configuration: parseJsonColumn(row.configuration) };
};

// Matches by handle, then name, then idp_ref_id, in that priority order — deterministic
// even if one org's handle happens to equal another org's name or idp_ref_id, unlike a
// single Op.or query (which returns whichever row the DB orders first).
const findOrgByIdentifier = async (param, t) => {
    const exec = t || db;
    const handle = typeof param === 'string' ? param.toLowerCase() : param;
    const portalId = getPortalId();
    return normalizeOrgRow(
        (await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE handle = ? AND portal_id = ?`, [handle, portalId])) ||
        (await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE display_name = ? AND portal_id = ?`, [param, portalId])) ||
        (await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE idp_ref_id = ? AND portal_id = ?`, [param, portalId]))
    );
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
    const organization = await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE uuid = ? AND portal_id = ?`, [uuid, getPortalId()]);
    if (!organization) {
        throw new NotFoundError('Organization not found');
    }
    return organization;
};

// Exact handle match only — no fallback to display_name/idp_ref_id. Used to resolve
// the single organization this instance is pinned to (src/utils/orgContext.js): in a
// shared multi-organization database, one org's handle can legitimately equal
// another's display name, so findOrgByIdentifier's priority ladder is too loose to
// establish the pin itself.
const getByHandle = async (handle, t) => {
    const exec = t || db;
    const organization = normalizeOrgRow(
        await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE handle = ? AND portal_id = ?`, [String(handle).toLowerCase(), getPortalId()])
    );
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
    return (await db.query(`SELECT * FROM ${ORG_TABLE} WHERE portal_id = ?`, [getPortalId()])).map(normalizeOrgRow);
};

const update = async (orgData, t) => {
    const exec = t || db;
    const existing = await get(orgData.orgId, t);
    const orgHandle = orgData.handle ? orgData.handle.toLowerCase() : existing.handle;
    const updatedAt = new Date();

    // cp_ref_id is written unconditionally, exactly like idp_ref_id: both are
    // plain optional reference fields with no derived default, so a blank one
    // clears the stored value rather than silently keeping the old one.
    const setClauses = [
        'display_name = ?', 'business_owner = ?', 'business_owner_contact = ?',
        'business_owner_email = ?', 'handle = ?', 'idp_ref_id = ?', 'cp_ref_id = ?',
        'updated_by = ?', 'updated_at = ?',
    ];
    const params = [
        orgData.displayName, orgData.businessOwner, orgData.businessOwnerContact,
        orgData.businessOwnerEmail, orgHandle, orgData.idpRefId, orgData.cpRefId,
        orgData.updatedBy, updatedAt,
    ];
    if (orgData.configuration !== undefined) {
        setClauses.push('configuration = ?');
        params.push(orgData.configuration);
    }
    params.push(existing.uuid, getPortalId());

    const { rowCount } = await exec.execute(
        `UPDATE ${ORG_TABLE} SET ${setClauses.join(', ')} WHERE uuid = ? AND portal_id = ?`,
        params
    );
    if (rowCount < 1) {
        throw new NotFoundError('Organization not found');
    }
    // Some dialects don't support RETURNING on UPDATE — re-fetch explicitly instead
    // (same pattern as applicationDao.update).
    const updatedOrg = normalizeOrgRow(await exec.queryOne(`SELECT * FROM ${ORG_TABLE} WHERE uuid = ? AND portal_id = ?`, [existing.uuid, getPortalId()]));
    return [rowCount, [updatedOrg]];
};

/**
 * Narrow, targeted write of idp_ref_id alone — used by the startup seeder to
 * reconcile the stored value with auth.idp_org_id in config. update() above writes
 * display_name, the business_owner fields, and cp_ref_id unconditionally, so
 * reusing it here would clear whatever an operator set through the settings UI.
 */
const updateIdpRefId = async (orgUuid, idpRefId, actor, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `UPDATE ${ORG_TABLE} SET idp_ref_id = ?, updated_by = ?, updated_at = ? WHERE uuid = ? AND portal_id = ?`,
        [idpRefId, actor, new Date(), orgUuid, getPortalId()]
    );
    if (rowCount < 1) {
        throw new NotFoundError('Organization not found');
    }
};

/**
 * Returns another organization that findOrgByIdentifier would resolve `value` to —
 * i.e. one whose handle, display_name, or idp_ref_id already equals it — or null.
 *
 * A shared multi-organization database is the case this guards: pointing this
 * instance's idp_ref_id at a value another organization already answers to would
 * shadow that organization's own identifier resolution, so the seeder refuses the
 * change rather than breaking a neighbouring tenant.
 */
const findOtherOrgClaimingIdentifier = async (value, excludeUuid, t) => {
    const exec = t || db;
    const rows = await exec.query(
        `SELECT * FROM ${ORG_TABLE} WHERE (handle = ? OR display_name = ? OR idp_ref_id = ?) AND uuid <> ? AND portal_id = ?`,
        [String(value).toLowerCase(), value, value, excludeUuid, getPortalId()]
    );
    return rows.length ? normalizeOrgRow(rows[0]) : null;
};

// Tables whose org_uuid FK is ON DELETE NO ACTION (database/schema.*.sql) block
// deleting the organization row unless their rows are removed first.
// api_metadata.org_uuid and subscription_plans.org_uuid are nullable and use
// ON DELETE NO ACTION (composite FKs cannot partially SET NULL while portal_id is
// NOT NULL), so we nullify them here before deleting the org row.
// Tables with ON DELETE CASCADE (audit, user_organization_mappings, and the
// *_mappings join tables) are left to the database to handle.
const deleteOrgDependents = async (orgUuid, t) => {
    const exec = t || db;

    const events = await exec.query('SELECT uuid FROM events WHERE org_uuid = ?', [orgUuid]);
    if (events.length) {
        const placeholders = events.map(() => '?').join(', ');
        await exec.execute(
            `DELETE FROM event_deliveries WHERE event_uuid IN (${placeholders})`,
            events.map((e) => e.uuid)
        );
    }
    await exec.execute('DELETE FROM events WHERE org_uuid = ?', [orgUuid]);

    // Nullify nullable org_uuid references before deleting the org row.
    // The DB constraint is ON DELETE NO ACTION; application code owns the nullification.
    await exec.execute('UPDATE api_metadata SET org_uuid = NULL WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('UPDATE subscription_plans SET org_uuid = NULL WHERE org_uuid = ?', [orgUuid]);

    await exec.execute('DELETE FROM api_keys WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM subscriptions WHERE org_uuid = ?', [orgUuid]);

    // Sequential, not Promise.all: both queries share the same connection/transaction
    // handle (sqlite's single connection, or an open tx on postgres/mssql), so running
    // them concurrently would interleave two statements on one session.
    const apps = await exec.query('SELECT uuid FROM applications WHERE org_uuid = ?', [orgUuid]);
    const keyManagers = await exec.query('SELECT uuid FROM key_managers WHERE org_uuid = ?', [orgUuid]);
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
        await exec.execute(`DELETE FROM app_key_mappings WHERE ${conditions.join(' OR ')}`, params);
    }
    await exec.execute('DELETE FROM applications WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM key_managers WHERE org_uuid = ?', [orgUuid]);

    await exec.execute('DELETE FROM api_workflows WHERE org_uuid = ?', [orgUuid]);
    await exec.execute(`DELETE FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ?`, [orgUuid]);
    // view_label_mappings/api_label_mappings cascade automatically from
    // views/labels ON DELETE CASCADE.
    await exec.execute('DELETE FROM views WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM labels WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM tags WHERE org_uuid = ?', [orgUuid]);
    await exec.execute('DELETE FROM webhook_subscribers WHERE org_uuid = ?', [orgUuid]);
};

const deleteOrg = async (orgId, t) => {
    const exec = t || db;
    const existing = await get(orgId, t);
    await deleteOrgDependents(existing.uuid, t);
    const { rowCount } = await exec.execute(`DELETE FROM ${ORG_TABLE} WHERE uuid = ? AND portal_id = ?`, [existing.uuid, getPortalId()]);
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
            (uuid, file_type, file_name, file_content, file_path, org_uuid, view_uuid, portal_id, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
            uuid, orgData.fileType, orgData.fileName, content, orgData.filePath,
            orgData.orgId, viewId, getPortalId(), orgData.createdBy, orgData.createdBy,
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
        portal_id: getPortalId(),
        created_by: orgData.createdBy,
        updated_by: orgData.createdBy,
    };
};

const updateContent = async (orgData) => {
    const viewId = await viewDao.getId(orgData.orgId, orgData.viewName);
    const updatedAt = new Date();
    const content = toBlobBuffer(orgData.fileContent);
    const portalId = getPortalId();
    const { rowCount } = await db.execute(
        `UPDATE ${ORG_CONTENT_TABLE}
         SET file_type = ?, file_name = ?, file_content = ?, file_path = ?, updated_by = ?, updated_at = ?
         WHERE file_type = ? AND file_name = ? AND file_path = ? AND org_uuid = ? AND view_uuid = ? AND portal_id = ?`,
        [
            orgData.fileType, orgData.fileName, content, orgData.filePath, orgData.updatedBy, updatedAt,
            orgData.fileType, orgData.fileName, orgData.filePath, orgData.orgId, viewId, portalId,
        ]
    );
    if (rowCount < 1) {
        throw new NotFoundError('No new resources found');
    }
    const updatedOrgContent = await db.query(
        `SELECT * FROM ${ORG_CONTENT_TABLE}
         WHERE file_type = ? AND file_name = ? AND file_path = ? AND org_uuid = ? AND view_uuid = ? AND portal_id = ?`,
        [orgData.fileType, orgData.fileName, orgData.filePath, orgData.orgId, viewId, portalId]
    );
    return [rowCount, updatedOrgContent];
};

const getContent = async (orgData) => {
    const viewId = await viewDao.getId(orgData.orgId, orgData.viewName);
    const portalId = getPortalId();
    if (orgData.fileName || orgData.filePath) {
        const conditions = ['org_uuid = ?', 'view_uuid = ?', 'file_type = ?', 'portal_id = ?'];
        const params = [orgData.orgId, viewId, orgData.fileType, portalId];
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
        `SELECT * FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ? AND view_uuid = ? AND file_type = ? AND portal_id = ?`,
        [orgData.orgId, viewId, orgData.fileType, portalId]
    );
};

const deleteContent = async (orgId, viewName, fileName) => {
    const viewId = await viewDao.getId(orgId, viewName);
    const { rowCount } = await db.execute(
        `DELETE FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ? AND view_uuid = ? AND file_name = ? AND portal_id = ?`,
        [orgId, viewId, fileName, getPortalId()]
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
        `DELETE FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ? AND view_uuid = ? AND portal_id = ? AND file_type IN (${placeholders})`,
        [orgId, viewId, getPortalId(), ...constants.THEME_FILE_TYPES]
    );
    return rowCount;
};

const hasThemeContent = async (orgId, viewName) => {
    const viewId = await viewDao.getId(orgId, viewName);
    if (!viewId) return false;
    const placeholders = constants.THEME_FILE_TYPES.map(() => '?').join(', ');
    const rows = await db.query(
        `SELECT 1 AS found FROM ${ORG_CONTENT_TABLE} WHERE org_uuid = ? AND view_uuid = ? AND portal_id = ? AND file_type IN (${placeholders})`,
        [orgId, viewId, getPortalId(), ...constants.THEME_FILE_TYPES]
    );
    return rows.length > 0;
};

module.exports = {
    create,
    get,
    getByUuid,
    getByHandle,
    getId,
    list,
    update,
    updateIdpRefId,
    findOtherOrgClaimingIdentifier,
    delete: deleteOrg,
    createContent,
    updateContent,
    getContent,
    deleteContent,
    deleteThemeContent,
    hasThemeContent,
};
