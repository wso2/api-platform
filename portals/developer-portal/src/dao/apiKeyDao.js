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
const { indexBy } = require('../db/rows');
const constants = require('../utils/constants');

const API_KEYS_TABLE = 'dp_api_keys';
const APP_KEY_MAPPINGS_TABLE = 'dp_api_key_app_mappings';
const API_METADATA_TABLE = 'dp_api_metadata';
const APPLICATIONS_TABLE = 'dp_applications';

// Built once at module load — buildUpsert only depends on the (fixed) dialect
// and column list, not on any per-call data. Used both by create() (first-time
// association, never conflicts) and setApplication() (re-association, may
// conflict on the key_uuid primary key).
const UPSERT_KEY_APP_MAPPING_SQL = db.buildUpsert(
    APP_KEY_MAPPINGS_TABLE,
    ['key_uuid', 'app_uuid', 'created_by', 'created_at'],
    ['key_uuid'],
    ['app_uuid', 'created_by']
);

/**
 * Mirrors the previous Sequelize `include:` eager-loading (API_METADATA_INCLUDE +
 * appMappingInclude) by batching two extra queries — scoped to the set of keys
 * already fetched — and stitching the results onto each row in place, instead of
 * a single multi-join query. Mutates `keys` and returns it for convenience.
 */
async function attachAssociations(exec, keys) {
    if (keys.length === 0) return keys;

    const apiIds = [...new Set(keys.map((k) => k.api_uuid))];
    const metadataRows = apiIds.length
        ? await exec.query(
            `SELECT uuid, name, version, handle, ref_id, type FROM ${API_METADATA_TABLE} WHERE uuid IN (${apiIds.map(() => '?').join(', ')})`,
            apiIds
        )
        : [];
    const metadataByUuid = indexBy(metadataRows, 'uuid');

    const keyIds = keys.map((k) => k.uuid);
    const mappingRows = keyIds.length
        ? await exec.query(
            `SELECT * FROM ${APP_KEY_MAPPINGS_TABLE} WHERE key_uuid IN (${keyIds.map(() => '?').join(', ')})`,
            keyIds
        )
        : [];
    const mappingByKeyUuid = indexBy(mappingRows, 'key_uuid');

    const appIds = [...new Set(mappingRows.map((m) => m.app_uuid))];
    const appRows = appIds.length
        ? await exec.query(
            `SELECT uuid, display_name, handle FROM ${APPLICATIONS_TABLE} WHERE uuid IN (${appIds.map(() => '?').join(', ')})`,
            appIds
        )
        : [];
    const appByUuid = indexBy(appRows, 'uuid');

    for (const key of keys) {
        key.dp_api_metadata = metadataByUuid.get(key.api_uuid) || null;
        const mapping = mappingByKeyUuid.get(key.uuid);
        key.dp_api_key_app_mapping = mapping
            ? { ...mapping, dp_application: appByUuid.get(mapping.app_uuid) || null }
            : null;
    }
    return keys;
}

async function create({ apiId, subscriptionId, appId, orgId, handle, displayName, expiresAt, createdBy }, transaction) {
    const exec = transaction || db;
    const uuid = crypto.randomUUID();
    const now = new Date();

    await exec.execute(
        `INSERT INTO ${API_KEYS_TABLE}
            (uuid, api_uuid, subscription_uuid, org_uuid, handle, display_name, status, expires_at,
             created_by, updated_by, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [uuid, apiId, subscriptionId || null, orgId, handle, displayName, constants.API_KEY_STATUS.ACTIVE,
            expiresAt || null, createdBy, createdBy, now, now]
    );
    if (appId) {
        await exec.execute(UPSERT_KEY_APP_MAPPING_SQL, [uuid, appId, createdBy, now]);
    }

    return {
        uuid,
        api_uuid: apiId,
        subscription_uuid: subscriptionId || null,
        org_uuid: orgId,
        handle,
        display_name: displayName,
        status: constants.API_KEY_STATUS.ACTIVE,
        expires_at: expiresAt || null,
        created_by: createdBy,
        updated_by: createdBy,
        revoked_at: null,
        revoked_by: null,
        created_at: now,
        updated_at: now,
    };
}

async function get(orgId, keyId, transaction) {
    const exec = transaction || db;
    const key = await exec.queryOne(
        `SELECT * FROM ${API_KEYS_TABLE} WHERE uuid = ? AND org_uuid = ?`,
        [keyId, orgId]
    );
    if (!key) return null;
    await attachAssociations(exec, [key]);
    return key;
}

// Resolves a key's handle (scoped to the given API) to its uuid, or null if not found.
async function getIdByHandle(orgId, apiId, handle) {
    const key = await db.queryOne(
        `SELECT uuid FROM ${API_KEYS_TABLE} WHERE org_uuid = ? AND api_uuid = ? AND handle = ?`,
        [orgId, apiId, handle]
    );
    return key ? key.uuid : null;
}

async function list(orgId, { apiId, subscriptionId, appId, status, createdBy, limit } = {}, transaction) {
    const exec = transaction || db;
    const conditions = ['org_uuid = ?'];
    const params = [orgId];
    if (apiId) { conditions.push('api_uuid = ?'); params.push(apiId); }
    if (subscriptionId) { conditions.push('subscription_uuid = ?'); params.push(subscriptionId); }
    if (status) { conditions.push('status = ?'); params.push(status); }
    if (createdBy) { conditions.push('created_by = ?'); params.push(createdBy); }
    if (appId) {
        // Mirrors the previous `required: true` app-mapping include — only keys
        // actually associated with this app are returned.
        conditions.push(`uuid IN (SELECT key_uuid FROM ${APP_KEY_MAPPINGS_TABLE} WHERE app_uuid = ?)`);
        params.push(appId);
    }

    let sql = `SELECT * FROM ${API_KEYS_TABLE} WHERE ${conditions.join(' AND ')} ORDER BY created_at DESC`;
    if (limit) {
        const { clause, params: limitParams } = db.paginationClause(limit, 0);
        sql += ` ${clause}`;
        params.push(...limitParams);
    }

    const keys = await exec.query(sql, params);
    await attachAssociations(exec, keys);
    return keys;
}

async function revoke(orgId, keyId, updatedBy, transaction) {
    const exec = transaction || db;
    const { rowCount } = await exec.execute(
        `UPDATE ${API_KEYS_TABLE} SET status = ?, revoked_at = ?, revoked_by = ?, updated_by = ?
         WHERE uuid = ? AND org_uuid = ? AND status = ?`,
        [constants.API_KEY_STATUS.REVOKED, new Date(), updatedBy, updatedBy, keyId, orgId, constants.API_KEY_STATUS.ACTIVE]
    );
    return rowCount > 0;
}

async function setApplication(orgId, keyId, appId, updatedBy, transaction, { activeOnly = false } = {}) {
    const exec = transaction || db;
    const conditions = ['uuid = ?', 'org_uuid = ?'];
    const params = [keyId, orgId];
    if (activeOnly) { conditions.push('status = ?'); params.push(constants.API_KEY_STATUS.ACTIVE); }

    const key = await exec.queryOne(`SELECT * FROM ${API_KEYS_TABLE} WHERE ${conditions.join(' AND ')}`, params);
    if (!key) return false;

    if (appId) {
        await exec.execute(UPSERT_KEY_APP_MAPPING_SQL, [keyId, appId, updatedBy, new Date()]);
    } else {
        await exec.execute(`DELETE FROM ${APP_KEY_MAPPINGS_TABLE} WHERE key_uuid = ?`, [keyId]);
    }
    return true;
}

async function updateExpiry(orgId, keyId, expiresAt, updatedBy, transaction) {
    const exec = transaction || db;
    const { rowCount } = await exec.execute(
        `UPDATE ${API_KEYS_TABLE} SET expires_at = ?, updated_by = ?, updated_at = ?
         WHERE uuid = ? AND org_uuid = ? AND status = ?`,
        [expiresAt, updatedBy, new Date(), keyId, orgId, constants.API_KEY_STATUS.ACTIVE]
    );
    return rowCount > 0;
}

module.exports = { create, get, getIdByHandle, list, revoke, setApplication, updateExpiry };
