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
const { findOrCreateSafe } = require('./findOrCreateHelper');
const constants = require('../utils/constants');
const { CustomError } = require('../utils/errors/customErrors');

const LABELS_TABLE = 'dp_labels';
const API_LABELS_TABLE = 'dp_api_label_mappings';
const VIEW_LABELS_TABLE = 'dp_view_label_mappings';

// Built once at module load — buildUpsert only depends on the (fixed) dialect
// and column list, not on any per-call data.
const UPSERT_API_LABEL_SQL = db.buildUpsert(
    API_LABELS_TABLE,
    ['uuid', 'label_uuid', 'api_uuid', 'created_by'],
    ['label_uuid', 'api_uuid'],
    [] // ignoreDuplicates semantics — leave the existing mapping row untouched on conflict
);

const create = async (orgId, label, createdBy, t) => {
    const exec = t || db;
    const uuid = crypto.randomUUID();
    await exec.execute(
        `INSERT INTO ${LABELS_TABLE} (uuid, handle, display_name, org_uuid, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?)`,
        [uuid, label.handle, label.displayName, orgId, createdBy, createdBy]
    );
    return {
        uuid,
        handle: label.handle,
        display_name: label.displayName,
        org_uuid: orgId,
        created_by: createdBy,
        updated_by: createdBy,
    };
};

const findById = async (orgId, labelId, t) => {
    const exec = t || db;
    const record = await exec.queryOne(
        `SELECT * FROM ${LABELS_TABLE} WHERE uuid = ? AND org_uuid = ?`,
        [labelId, orgId]
    );
    if (!record) {
        throw new CustomError(404, constants.ERROR_CODE[404], 'Label not found');
    }
    return record;
};

const getIdByHandle = async (orgId, handle) => {
    const label = await db.queryOne(
        `SELECT uuid FROM ${LABELS_TABLE} WHERE org_uuid = ? AND handle = ?`,
        [orgId, handle]
    );
    return label ? label.uuid : null;
};

const updateById = async (orgId, labelId, label, updatedBy, t) => {
    const exec = t || db;
    const record = await findById(orgId, labelId, t);
    const updatedAt = new Date();
    await exec.execute(
        `UPDATE ${LABELS_TABLE} SET display_name = ?, updated_by = ?, updated_at = ? WHERE uuid = ? AND org_uuid = ?`,
        [label.displayName, updatedBy, updatedAt, labelId, orgId]
    );
    return { ...record, display_name: label.displayName, updated_by: updatedBy, updated_at: updatedAt };
};

const deleteById = async (orgId, labelId) => {
    const { rowCount } = await db.execute(
        `DELETE FROM ${LABELS_TABLE} WHERE uuid = ? AND org_uuid = ?`,
        [labelId, orgId]
    );
    if (rowCount === 0) {
        throw new CustomError(404, constants.ERROR_CODE[404], 'Label not found');
    }
    return rowCount;
};

const createMany = async (orgId, labels, createdBy, t) => {
    const exec = t || db;
    const created = [];
    for (const label of labels) {
        const uuid = crypto.randomUUID();
        await exec.execute(
            `INSERT INTO ${LABELS_TABLE} (uuid, handle, display_name, org_uuid, created_by, updated_by)
             VALUES (?, ?, ?, ?, ?, ?)`,
            [uuid, label.handle, label.displayName, orgId, createdBy, createdBy]
        );
        created.push({
            uuid,
            handle: label.handle,
            display_name: label.displayName,
            org_uuid: orgId,
            created_by: createdBy,
            updated_by: createdBy,
        });
    }
    return created;
};

const createApiMapping = async (orgId, apiId, labels, createdBy, t) => {
    const exec = t || db;
    const idList = await getId(orgId, labels, t);
    for (const labelId of idList) {
        await exec.execute(UPSERT_API_LABEL_SQL, [crypto.randomUUID(), labelId, apiId, createdBy]);
    }
    return idList;
};

/**
 * Update-or-create a label by handle. Mirrors the previous Sequelize
 * findOrCreate-then-conditionally-update flow: insert first, and if another
 * request already created the same (handle, org_uuid) row (unique-constraint
 * race), fall back to updating the existing row instead of failing.
 */
const update = async (orgId, label, updatedBy, t) => {
    const exec = t || db;
    const existing = await exec.queryOne(
        `SELECT * FROM ${LABELS_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [label.handle, orgId]
    );

    let row = existing;
    if (!row) {
        const uuid = crypto.randomUUID();
        try {
            await exec.execute(
                `INSERT INTO ${LABELS_TABLE} (uuid, handle, display_name, org_uuid, created_by, updated_by)
                 VALUES (?, ?, ?, ?, ?, ?)`,
                [uuid, label.handle, label.displayName, orgId, updatedBy, updatedBy]
            );
            return {
                uuid,
                handle: label.handle,
                display_name: label.displayName,
                org_uuid: orgId,
                created_by: updatedBy,
                updated_by: updatedBy,
            };
        } catch (error) {
            if (!db.isDuplicateKeyError(error)) throw error;
            // Lost a race to create this label — fall through to the update path below.
            row = await exec.queryOne(
                `SELECT * FROM ${LABELS_TABLE} WHERE handle = ? AND org_uuid = ?`,
                [label.handle, orgId]
            );
        }
    }

    const updatedAt = new Date();
    await exec.execute(
        `UPDATE ${LABELS_TABLE} SET display_name = ?, updated_by = ?, updated_at = ? WHERE uuid = ?`,
        [label.displayName, updatedBy, updatedAt, row.uuid]
    );
    return { ...row, display_name: label.displayName, updated_by: updatedBy, updated_at: updatedAt };
};

const getId = async (orgId, labels, t) => {
    const idList = [];
    for (const label of labels) {
        idList.push(await getIdList(orgId, label, t));
    }
    return idList;
};

const getIdList = async (orgId, label, t) => {
    const exec = t || db;
    const labelResponse = await exec.queryOne(
        `SELECT uuid FROM ${LABELS_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [label, orgId]
    );
    if (!labelResponse) {
        throw new CustomError(404, constants.ERROR_CODE[404], 'Label not found');
    }
    return labelResponse.uuid;
};

const list = async (orgId) => {
    return db.query(`SELECT * FROM ${LABELS_TABLE} WHERE org_uuid = ?`, [orgId]);
};

const deleteApiMapping = async (orgId, apiId, labels, t) => {
    const exec = t || db;
    const idList = await getId(orgId, labels, t);
    if (idList.length === 0) return 0;
    const placeholders = idList.map(() => '?').join(', ');
    const { rowCount } = await exec.execute(
        `DELETE FROM ${API_LABELS_TABLE} WHERE label_uuid IN (${placeholders}) AND api_uuid = ?`,
        [...idList, apiId]
    );
    return rowCount;
};

const addToView = async (orgId, labelId, viewId, createdBy, t) => {
    const exec = t || db;
    return findOrCreateSafe(
        VIEW_LABELS_TABLE,
        { label_uuid: labelId, view_uuid: viewId },
        { uuid: crypto.randomUUID(), label_uuid: labelId, view_uuid: viewId, created_by: createdBy },
        exec
    );
};

module.exports = {
    create,
    createMany,
    createApiMapping,
    update,
    updateById,
    findById,
    getIdByHandle,
    getId,
    getIdList,
    deleteById,
    list,
    deleteApiMapping,
    addToView,
};
