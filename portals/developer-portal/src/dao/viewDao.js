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

const VIEWS_TABLE = 'dp_views';
const VIEW_LABELS_TABLE = 'dp_view_label_mappings';
const LABELS_TABLE = 'dp_labels';
const ORG_ASSETS_TABLE = 'dp_organization_assets';

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
    return { ...view, dp_labels: labels };
};

const getId = async (orgId, viewName, t) => {
    // `view` is an optional query param on /apis and /mcp-servers (apiViewQuery in the
    // OpenAPI spec) — a bare handle/display_name lookup with `undefined` throws at the
    // Sequelize layer ("WHERE parameter has invalid undefined value") rather than the
    // 404 below, so short-circuit before ever building that query.
    if (!viewName) return undefined;

    const exec = t || db;
    let view = await exec.queryOne(
        `SELECT uuid FROM ${VIEWS_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [viewName, orgId]
    );
    if (!view) {
        view = await exec.queryOne(
            `SELECT uuid FROM ${VIEWS_TABLE} WHERE display_name = ? AND org_uuid = ?`,
            [viewName, orgId]
        );
    }
    if (!view) {
        throw new CustomError(404, constants.ERROR_CODE[404], "View not found");
    }
    return view.uuid;
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
        dp_labels: (labelsByView.get(v.uuid) || []).map((r) => ({ handle: r.handle })),
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
    delete: deleteView,
    get,
    getId,
    list,
    addLabels,
    replaceLabels,
};
