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
const { groupBy } = require('../db/rows');
const constants = require('../utils/constants');
const { CustomError } = require('../utils/errors/customErrors');

const VIEWS_TABLE = 'views';
// The conventional handle of the view every org is seeded with, and the first choice of
// getFallbackHandle. Not special-cased anywhere else — it can be renamed or deleted.
const DEFAULT_VIEW_HANDLE = 'default';
const VIEW_LABELS_TABLE = 'view_label_mappings';
const LABELS_TABLE = 'labels';
const ORG_ASSETS_TABLE = 'organization_assets';

const create = async (orgId, payload, createdBy, t) => {
    const exec = t || db;
    const displayName = payload.displayName ? payload.displayName : payload.handle;
    const uuid = crypto.randomUUID();

    await exec.execute(
        `INSERT INTO ${VIEWS_TABLE} (uuid, handle, display_name, org_uuid, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?)`,
        [uuid, payload.handle, displayName, orgId, createdBy, createdBy]
    );

    return {
        uuid,
        handle: payload.handle,
        display_name: displayName,
        org_uuid: orgId,
        created_by: createdBy,
        updated_by: createdBy,
    };
};

/**
 * Update-or-create a view by handle. Mirrors the previous Sequelize
 * findOrCreate-then-conditionally-update flow (same pattern as labelDao.update):
 * insert first, and if another request already created the same (handle, org_uuid)
 * row (unique-constraint race), or the row already existed, fall back to updating
 * the existing row instead.
 */
const update = async (orgId, handle, displayName, updatedBy, t) => {
    const exec = t || db;
    const existing = await exec.queryOne(
        `SELECT * FROM ${VIEWS_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [handle, orgId]
    );

    let row = existing;
    if (!row) {
        const uuid = crypto.randomUUID();
        const initialDisplayName = displayName ? displayName : handle;
        try {
            await db.withSavepoint(exec, () => exec.execute(
                `INSERT INTO ${VIEWS_TABLE} (uuid, handle, display_name, org_uuid, created_by, updated_by)
                 VALUES (?, ?, ?, ?, ?, ?)`,
                [uuid, handle, initialDisplayName, orgId, updatedBy, updatedBy]
            ));
            return {
                uuid,
                handle,
                display_name: initialDisplayName,
                org_uuid: orgId,
                created_by: updatedBy,
                updated_by: updatedBy,
            };
        } catch (error) {
            if (!db.isDuplicateKeyError(error)) throw error;
            // Lost a race to create this view — fall through to the update path below.
            row = await exec.queryOne(
                `SELECT * FROM ${VIEWS_TABLE} WHERE handle = ? AND org_uuid = ?`,
                [handle, orgId]
            );
        }
    }

    const updatedAt = new Date();
    const newDisplayName = displayName ? displayName : row.display_name;
    await exec.execute(
        `UPDATE ${VIEWS_TABLE} SET display_name = ?, updated_by = ?, updated_at = ? WHERE uuid = ? AND org_uuid = ?`,
        [newDisplayName, updatedBy, updatedAt, row.uuid, orgId]
    );
    return { ...row, display_name: newDisplayName, updated_by: updatedBy, updated_at: updatedAt };
};

const deleteView = async (orgId, handle, t) => {
    const exec = t || db;
    const view = await exec.queryOne(
        `SELECT * FROM ${VIEWS_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [handle, orgId]
    );
    if (!view) {
        return 0;
    }
    // Explicit cleanup of dependents before deleting the view row itself, regardless
    // of whether the active dialect's schema also cascades these FKs at the DB level —
    // same defensive pattern organizationDao's whole-org delete uses for OrgContent.
    await exec.execute(`DELETE FROM ${VIEW_LABELS_TABLE} WHERE view_uuid = ?`, [view.uuid]);
    await exec.execute(`DELETE FROM ${ORG_ASSETS_TABLE} WHERE view_uuid = ?`, [view.uuid]);
    const { rowCount } = await exec.execute(
        `DELETE FROM ${VIEWS_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [handle, orgId]
    );
    return rowCount;
};

const get = async (orgId, handle) => {
    const view = await db.queryOne(
        `SELECT * FROM ${VIEWS_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [handle, orgId]
    );
    if (!view) {
        return null;
    }
    const labels = await db.query(
        `SELECT l.handle AS handle
         FROM ${LABELS_TABLE} l
         INNER JOIN ${VIEW_LABELS_TABLE} vl ON vl.label_uuid = l.uuid
         WHERE vl.view_uuid = ?`,
        [view.uuid]
    );
    return { ...view, labels: labels };
};

const getId = async (orgId, viewName, t) => {
    // `view` is an optional query param on /apis and /mcp-servers (viewQuery in the
    // OpenAPI spec) — a bare handle lookup with `undefined` throws at the driver layer
    // ("WHERE parameter has invalid undefined value") rather than the 404 below, so
    // short-circuit before ever building that query.
    if (!viewName) return undefined;

    const exec = t || db;
    const view = await exec.queryOne(
        `SELECT uuid FROM ${VIEWS_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [viewName, orgId]
    );
    if (!view) {
        throw new CustomError(404, constants.ERROR_CODE[404], "View not found");
    }
    return view.uuid;
};

// The handle of the view the portal falls back to when a URL names no view — the bare
// org root (/{orgName}), the error page's home link, and the org-scoped settings page's
// chrome. 'default' used to be hardcoded at each of those sites, which is why the
// 'default' view could not be deleted or renamed; this resolves it instead:
//
//   1. the view whose handle is 'default', when it still exists (unchanged behaviour
//      for every existing deployment, since the seeder creates it), else
//   2. the org's earliest-created view, handle breaking a same-timestamp tie so the
//      answer is stable across requests and dialects.
//
// Falls back to the literal 'default' only for an org with no views at all, which the
// last-view delete guard (apiMetadataService.deleteView) prevents reaching through the
// API — a fresh org is seeded with one.
const getFallbackHandle = async (orgId, t) => {
    const exec = t || db;
    const preferred = await exec.queryOne(
        `SELECT handle FROM ${VIEWS_TABLE} WHERE org_uuid = ? AND handle = ?`,
        [orgId, DEFAULT_VIEW_HANDLE]
    );
    if (preferred) {
        return preferred.handle;
    }
    const earliest = await exec.queryOne(
        `SELECT handle FROM ${VIEWS_TABLE} WHERE org_uuid = ? ORDER BY created_at ASC, handle ASC`,
        [orgId]
    );
    return earliest ? earliest.handle : DEFAULT_VIEW_HANDLE;
};

// Number of views in the org — the last-view delete guard's input.
const count = async (orgId, t) => {
    const exec = t || db;
    const row = await exec.queryOne(`SELECT COUNT(*) AS total FROM ${VIEWS_TABLE} WHERE org_uuid = ?`, [orgId]);
    return Number(row?.total ?? 0);
};

/**
 * Renames a view's handle in place, keeping its uuid — so every reference survives:
 * organization_assets, view_label_mappings and api_workflows all key on view_uuid and
 * no table stores the handle, so this is a single-row update with nothing to migrate.
 *
 * URLs are the thing that does NOT survive: every portal page embeds the handle, so
 * links to the old one 404 afterwards. That is the caller's (and the operator's)
 * decision to make, which is why the settings UI warns before saving a rename.
 *
 * Returns null when no view carries `oldHandle`; throws CustomError(409) when
 * `newHandle` is already taken in this organization.
 */
const rename = async (orgId, oldHandle, newHandle, updatedBy, t) => {
    const exec = t || db;
    const existing = await exec.queryOne(
        `SELECT * FROM ${VIEWS_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [oldHandle, orgId]
    );
    if (!existing) {
        return null;
    }
    if (newHandle === oldHandle) {
        return existing;
    }
    const updatedAt = new Date();
    try {
        await db.withSavepoint(exec, () => exec.execute(
            `UPDATE ${VIEWS_TABLE} SET handle = ?, updated_by = ?, updated_at = ? WHERE uuid = ? AND org_uuid = ?`,
            [newHandle, updatedBy, updatedAt, existing.uuid, orgId]
        ));
    } catch (error) {
        // uq_view_handle_org_uuid — another view already answers to this handle. Report
        // it as a conflict rather than letting a raw driver error surface.
        if (db.isDuplicateKeyError(error)) {
            throw new CustomError(409, constants.ERROR_CODE[409], `A view with the handle '${newHandle}' already exists`);
        }
        throw error;
    }
    return { ...existing, handle: newHandle, updated_by: updatedBy, updated_at: updatedAt };
};

const list = async (orgId) => {
    const views = await db.query(`SELECT * FROM ${VIEWS_TABLE} WHERE org_uuid = ?`, [orgId]);
    if (views.length === 0) return views;

    const viewIds = views.map((v) => v.uuid);
    const placeholders = viewIds.map(() => '?').join(', ');
    const labelRows = await db.query(
        `SELECT vl.view_uuid AS view_uuid, l.handle AS handle
         FROM ${VIEW_LABELS_TABLE} vl
         INNER JOIN ${LABELS_TABLE} l ON l.uuid = vl.label_uuid
         WHERE vl.view_uuid IN (${placeholders})`,
        viewIds
    );
    const labelsByView = groupBy(labelRows, 'view_uuid');

    return views.map((v) => ({
        ...v,
        labels: (labelsByView.get(v.uuid) || []).map((r) => ({ handle: r.handle })),
    }));
};

const addLabels = async (orgId, viewId, labels, createdBy, t) => {
    const exec = t || db;
    const idList = await getLabelId(orgId, labels, t);
    const created = [];
    for (const labelId of idList) {
        const uuid = crypto.randomUUID();
        await exec.execute(
            `INSERT INTO ${VIEW_LABELS_TABLE} (uuid, label_uuid, view_uuid, created_by) VALUES (?, ?, ?, ?)`,
            [uuid, labelId, viewId, createdBy]
        );
        created.push({ uuid, label_uuid: labelId, view_uuid: viewId, created_by: createdBy });
    }
    return created;
};

const replaceLabels = async (orgId, viewId, labelNames, createdBy, t) => {
    const exec = t || db;
    await exec.execute(`DELETE FROM ${VIEW_LABELS_TABLE} WHERE view_uuid = ?`, [viewId]);
    if (labelNames?.length) {
        await addLabels(orgId, viewId, labelNames, createdBy, t);
    }
};

// Internal helper used by addLabels, replaceLabels
async function getLabelId(orgId, labels, t) {
    const labelDao = require('./labelDao');
    return labelDao.getId(orgId, labels, t);
}

module.exports = {
    create,
    update,
    rename,
    delete: deleteView,
    get,
    getId,
    getFallbackHandle,
    count,
    list,
    addLabels,
    replaceLabels,
};
