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
const { groupBy, toBlobBuffer } = require('../db/rows');
const constants = require('../utils/constants');

const CONTENT_TABLE = 'dp_api_contents';
const API_METADATA_TABLE = 'dp_api_metadata';

// Every content row is tenant-scoped through the API it belongs to — this
// correlated EXISTS clause (not a JOIN alias, which sqlite's UPDATE grammar
// doesn't support portably) is appended to UPDATE/DELETE statements that need
// to verify org ownership. Requires org_uuid as the LAST bind param.
const TENANT_SCOPE_EXISTS =
    `EXISTS (SELECT 1 FROM ${API_METADATA_TABLE} m WHERE m.uuid = ${CONTENT_TABLE}.api_uuid AND m.org_uuid = ?)`;

const store = async (apiFile, fileName, apiId, type, createdBy, t, key) => {
    const exec = t || db;
    const uuid = crypto.randomUUID();
    const content = toBlobBuffer(apiFile);
    await exec.execute(
        `INSERT INTO ${CONTENT_TABLE} (uuid, file_content, file_name, api_uuid, type, lookup_key, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        [uuid, content, fileName, apiId, type, key ?? null, createdBy, createdBy]
    );
    return {
        uuid, file_content: content, file_name: fileName, api_uuid: apiId, type,
        lookup_key: key ?? null, created_by: createdBy, updated_by: createdBy,
    };
};

const storeMany = async (files, apiId, createdBy, t) => {
    const exec = t || db;
    const created = [];
    for (const file of files) {
        const uuid = crypto.randomUUID();
        const content = toBlobBuffer(file.content);
        await exec.execute(
            `INSERT INTO ${CONTENT_TABLE} (uuid, file_content, file_name, type, api_uuid, lookup_key, created_by, updated_by)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
            [uuid, content, file.fileName, file.type, apiId, file.key ?? null, createdBy, createdBy]
        );
        created.push({
            uuid, file_content: content, file_name: file.fileName, type: file.type,
            api_uuid: apiId, lookup_key: file.key ?? null, created_by: createdBy, updated_by: createdBy,
        });
    }
    return created;
};

const get = async (fileName, type, orgId, apiId, t) => {
    const exec = t || db;
    return exec.queryOne(
        `SELECT c.* FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE c.file_name = ? AND c.api_uuid = ? AND c.type = ? AND m.org_uuid = ?`,
        [fileName, apiId, type, orgId]
    );
};

const getByType = async (type, orgId, apiId, t) => {
    const exec = t || db;
    return exec.queryOne(
        `SELECT c.* FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE c.api_uuid = ? AND c.type = ? AND m.org_uuid = ?`,
        [apiId, type, orgId]
    );
};

/**
 * Find a single content row by its lookup_key (e.g. a named image slot like 'api-icon').
 */
const getByKey = async (key, apiId, t) => {
    const exec = t || db;
    return exec.queryOne(
        `SELECT * FROM ${CONTENT_TABLE} WHERE api_uuid = ? AND type = ? AND lookup_key = ?`,
        [apiId, constants.DOC_TYPES.IMAGES, key]
    );
};

/**
 * Delete a single content row by its lookup_key (e.g. a named image slot like 'api-icon').
 */
const deleteByKey = async (key, apiId, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `DELETE FROM ${CONTENT_TABLE} WHERE api_uuid = ? AND type = ? AND lookup_key = ?`,
        [apiId, constants.DOC_TYPES.IMAGES, key]
    );
    return rowCount;
};

const upsertMany = async (files, apiId, orgId, updatedBy, t) => {
    const exec = t || db;
    const filesToCreate = [];

    for (const file of files) {
        // A keyed file (e.g. a named image slot) is identified by its lookup_key, since its
        // file_name can change between uploads. Unkeyed files (docs, specs) are
        // identified by file_name as before.
        const existing = file.key
            ? await getByKey(file.key, apiId, t)
            : await get(file.fileName, file.type, orgId, apiId, t);

        if (existing == null) {
            filesToCreate.push({
                file_content: toBlobBuffer(file.content), file_name: file.fileName, api_uuid: apiId, type: file.type,
                lookup_key: file.key ?? null, created_by: updatedBy, updated_by: updatedBy,
            });
        } else {
            const updatedAt = new Date();
            const { rowCount } = await exec.execute(
                `UPDATE ${CONTENT_TABLE}
                 SET file_content = ?, file_name = ?, lookup_key = ?, updated_by = ?, updated_at = ?
                 WHERE api_uuid = ? AND file_name = ? AND type = ? AND ${TENANT_SCOPE_EXISTS}`,
                [
                    toBlobBuffer(file.content), file.fileName, file.key ?? existing.lookup_key, updatedBy, updatedAt,
                    apiId, existing.file_name, existing.type, orgId,
                ]
            );
            if (!rowCount) {
                throw new Error('Error while updating API files');
            }
        }
    }

    for (const file of filesToCreate) {
        const uuid = crypto.randomUUID();
        await exec.execute(
            `INSERT INTO ${CONTENT_TABLE} (uuid, file_content, file_name, api_uuid, type, lookup_key, created_by, updated_by)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
            [uuid, file.file_content, file.file_name, file.api_uuid, file.type, file.lookup_key, file.created_by, file.updated_by]
        );
    }
};

const upsert = async (apiFile, fileName, apiId, orgId, type, updatedBy, t, key) => {
    const exec = t || db;
    const existing = await getByType(type, orgId, apiId, t);
    const content = toBlobBuffer(apiFile);

    if (existing == null) {
        const uuid = crypto.randomUUID();
        await exec.execute(
            `INSERT INTO ${CONTENT_TABLE} (uuid, file_content, file_name, api_uuid, type, lookup_key, created_by, updated_by)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
            [uuid, content, fileName, apiId, type, key ?? null, updatedBy, updatedBy]
        );
        return {
            uuid, file_content: content, file_name: fileName, api_uuid: apiId, type,
            lookup_key: key ?? null, created_by: updatedBy, updated_by: updatedBy,
        };
    }

    const updatedAt = new Date();
    const { rowCount } = await exec.execute(
        `UPDATE ${CONTENT_TABLE}
         SET file_content = ?, file_name = ?, lookup_key = ?, updated_by = ?, updated_at = ?
         WHERE api_uuid = ? AND type = ? AND ${TENANT_SCOPE_EXISTS}`,
        [content, fileName, key ?? existing.lookup_key, updatedBy, updatedAt, apiId, type, orgId]
    );
    return rowCount;
};

const update = async (apiFile, fileName, apiId, orgId, type, updatedBy, t, key) => {
    const exec = t || db;
    const existing = await get(fileName, type, orgId, apiId, t);
    const content = toBlobBuffer(apiFile);

    if (existing == null) {
        const uuid = crypto.randomUUID();
        await exec.execute(
            `INSERT INTO ${CONTENT_TABLE} (uuid, file_content, file_name, api_uuid, type, lookup_key, created_by, updated_by)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
            [uuid, content, fileName, apiId, type, key ?? null, updatedBy, updatedBy]
        );
        return {
            uuid, file_content: content, file_name: fileName, api_uuid: apiId, type,
            lookup_key: key ?? null, created_by: updatedBy, updated_by: updatedBy,
        };
    }

    const updatedAt = new Date();
    const { rowCount } = await exec.execute(
        `UPDATE ${CONTENT_TABLE}
         SET file_content = ?, file_name = ?, lookup_key = ?, updated_by = ?, updated_at = ?
         WHERE api_uuid = ? AND file_name = ? AND type = ? AND ${TENANT_SCOPE_EXISTS}`,
        [content, fileName, key ?? existing.lookup_key, updatedBy, updatedAt, apiId, fileName, type, orgId]
    );
    return rowCount;
};

const deleteFile = async (fileName, type, orgId, apiId, t) => {
    const exec = t || db;
    const contentsToDelete = await exec.query(
        `SELECT c.* FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE c.file_name = ? AND c.api_uuid = ? AND c.type LIKE ? AND m.org_uuid = ?`,
        [fileName, apiId, `%${type}%`, orgId]
    );
    let rowCount;
    for (const content of contentsToDelete) {
        ({ rowCount } = await exec.execute(
            `DELETE FROM ${CONTENT_TABLE} WHERE api_uuid = ? AND file_name = ? AND type = ?`,
            [content.api_uuid, content.file_name, content.type]
        ));
    }
    return rowCount;
};

const deleteAll = async (type, orgId, apiId, t) => {
    const exec = t || db;
    const contentsToDelete = await exec.query(
        `SELECT c.* FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE c.api_uuid = ? AND c.type LIKE ? AND m.org_uuid = ?`,
        [apiId, `%${type}%`, orgId]
    );
    let rowCount;
    for (const content of contentsToDelete) {
        ({ rowCount } = await exec.execute(
            `DELETE FROM ${CONTENT_TABLE} WHERE api_uuid = ? AND file_name = ? AND type = ?`,
            [content.api_uuid, content.file_name, content.type]
        ));
    }
    return rowCount;
};

/**
 * Delete every content row of an exact type for an API (e.g. clear all images
 * before re-storing a freshly uploaded set). Exact match on type, scoped to
 * api_uuid, and participates in the caller's transaction.
 */
const deleteAllByType = async (type, apiId, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `DELETE FROM ${CONTENT_TABLE} WHERE api_uuid = ? AND type = ?`,
        [apiId, type]
    );
    return rowCount;
};

const getDoc = async (type, orgId, apiId, t) => {
    const exec = t || db;
    return exec.queryOne(
        `SELECT c.* FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE c.api_uuid = ? AND c.type = ? AND m.org_uuid = ?`,
        [apiId, type, orgId]
    );
};

const getDocByName = async (type, name, orgId, apiId, t) => {
    const exec = t || db;
    return exec.queryOne(
        `SELECT c.* FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE c.api_uuid = ? AND c.type = ? AND c.file_name = ? AND m.org_uuid = ?`,
        [apiId, type, name, orgId]
    );
};

/**
 * Per-type file name lists, one row per `type` with a `file_names` array —
 * `file_name` is text, so aggregating it as a delimited string (then splitting)
 * is safe on every dialect. postgres uses ARRAY_AGG directly; sqlite/mssql
 * concat with a separator unlikely to appear in a file name and split it back.
 */
const getDocTypes = async (orgId, apiId) => {
    const dialect = db.getDialect();
    const whereSql = 'c.api_uuid = ? AND (c.type LIKE ? OR c.type LIKE ?) AND m.org_uuid = ?';
    const params = [apiId, 'DOC_%', constants.DOC_TYPES.API_DEFINITION, orgId];

    if (dialect === 'postgres') {
        return db.query(
            `SELECT c.type AS type, ARRAY_AGG(c.file_name) AS file_names
             FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
             WHERE ${whereSql} GROUP BY c.type`,
            params
        );
    }

    const aggFn = dialect === 'mssql' ? 'STRING_AGG' : 'GROUP_CONCAT';
    const rows = await db.query(
        `SELECT c.type AS type, ${aggFn}(c.file_name, '|||') AS file_names
         FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE ${whereSql} GROUP BY c.type`,
        params
    );
    for (const row of rows) {
        row.file_names = row.file_names ? row.file_names.split('|||') : [];
    }
    return rows;
};

/**
 * Per-type { file_names, api_files } groups. file_content is BLOB/BYTEA — never
 * delimiter-concatenate binary data, so the sqlite/mssql path fetches flat rows
 * and groups them in app code instead of using GROUP_CONCAT/STRING_AGG. postgres
 * can aggregate both columns directly since ARRAY_AGG produces a real array,
 * not a concatenated string.
 */
const getDocs = async (orgId, apiId) => {
    const dialect = db.getDialect();
    const whereSql = 'c.api_uuid = ? AND (c.type LIKE ? OR c.file_name LIKE ?) AND m.org_uuid = ?';
    const params = [apiId, 'DOC_%', 'LINK_%', orgId];

    if (dialect === 'postgres') {
        return db.query(
            `SELECT c.type AS type,
                    ARRAY_AGG(c.file_name) AS file_names,
                    ARRAY_AGG(c.file_content) AS api_files
             FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
             WHERE ${whereSql} GROUP BY c.type`,
            params
        );
    }

    const rows = await db.query(
        `SELECT c.type AS type, c.file_name AS file_name, c.file_content AS file_content
         FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE ${whereSql}`,
        params
    );
    const grouped = groupBy(rows, 'type');
    return Array.from(grouped.entries()).map(([type, groupRows]) => ({
        type,
        file_names: groupRows.map((r) => r.file_name),
        api_files: groupRows.map((r) => r.file_content),
    }));
};

/** Same shape as getDocs, scoped to file_name LIKE 'LINK_%' only. */
const getDocLinks = async (orgId, apiId) => {
    const dialect = db.getDialect();
    const whereSql = "c.api_uuid = ? AND c.file_name LIKE ? AND m.org_uuid = ?";
    const params = [apiId, 'LINK_%', orgId];

    if (dialect === 'postgres') {
        return db.query(
            `SELECT c.type AS type,
                    ARRAY_AGG(c.file_name) AS file_names,
                    ARRAY_AGG(c.file_content) AS api_files
             FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
             WHERE ${whereSql} GROUP BY c.type`,
            params
        );
    }

    const rows = await db.query(
        `SELECT c.type AS type, c.file_name AS file_name, c.file_content AS file_content
         FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE ${whereSql}`,
        params
    );
    const grouped = groupBy(rows, 'type');
    return Array.from(grouped.entries()).map(([type, groupRows]) => ({
        type,
        file_names: groupRows.map((r) => r.file_name),
        api_files: groupRows.map((r) => r.file_content),
    }));
};

const listDocNames = async (orgId, apiId) => {
    const rows = await db.query(
        `SELECT c.file_name AS file_name
         FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE c.api_uuid = ? AND c.type LIKE ? AND m.org_uuid = ?`,
        [apiId, `${constants.DOC_TYPES.DOC_ID}%`, orgId]
    );
    return rows.map((r) => r.file_name);
};

const listDocNamesForApis = async (orgId, apiIds) => {
    const docNamesByApiId = {};
    for (const apiId of apiIds) docNamesByApiId[apiId] = [];
    if (apiIds.length === 0) return docNamesByApiId;

    const placeholders = apiIds.map(() => '?').join(', ');
    const rows = await db.query(
        `SELECT c.file_name AS file_name, c.api_uuid AS api_uuid
         FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE c.api_uuid IN (${placeholders}) AND c.type LIKE ? AND m.org_uuid = ?`,
        [...apiIds, `${constants.DOC_TYPES.DOC_ID}%`, orgId]
    );
    for (const row of rows) {
        docNamesByApiId[row.api_uuid].push(row.file_name);
    }
    return docNamesByApiId;
};

const deleteByFileName = async (fileName, orgId, apiId, t) => {
    const exec = t || db;
    // Scope to document rows only (type LIKE 'DOC_%'), matching listDocNames. Without this,
    // a non-doc row (image, spec) that happens to share the file_name would also be deleted.
    const contentsToDelete = await exec.query(
        `SELECT c.* FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
         WHERE c.file_name = ? AND c.api_uuid = ? AND c.type LIKE ? AND m.org_uuid = ?`,
        [fileName, apiId, `${constants.DOC_TYPES.DOC_ID}%`, orgId]
    );
    for (const content of contentsToDelete) {
        await exec.execute(
            `DELETE FROM ${CONTENT_TABLE} WHERE api_uuid = ? AND file_name = ? AND type = ?`,
            [apiId, content.file_name, content.type]
        );
    }
};

module.exports = {
    store,
    storeMany,
    upsertMany,
    get,
    getByType,
    getByKey,
    deleteByKey,
    upsert,
    update,
    delete: deleteFile,
    deleteAll,
    deleteAllByType,
    getDoc,
    getDocByName,
    getDocTypes,
    getDocs,
    getDocLinks,
    listDocNames,
    listDocNamesForApis,
    deleteByFileName,
};
