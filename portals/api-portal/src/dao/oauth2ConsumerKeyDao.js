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

/*
 * Data access for `oauth2_consumer_keys` — the portal's record of each OAuth
 * application it registered on a key manager over RFC 7591.
 *
 * The row holds the identity of the registered client and nothing else. Four
 * things are deliberately not stored:
 *
 *   - The client secret. The key manager returns it once, at registration, and
 *     never again, so the create response is the only place it can appear.
 *   - The client metadata (redirect_uris, grant_types, …). It lives at the key
 *     manager and is re-read from there, so there is one copy of it.
 *   - The RFC 7592 registration access token. Follow-up calls therefore
 *     authenticate with the portal's own provisioning credential instead — see
 *     the note in oauth2KeyService.js about what that costs.
 *   - The client configuration URI. The driver constructs
 *     <registration_endpoint>/<consumer_key> from config plus this row.
 *
 * The key↔application association lives in its own table (schema pending), not
 * as a column here.
 *
 * Every query is scoped by `portal_id` AND `org_uuid`, and the ownership filter
 * (`created_by`) is applied in SQL rather than after the fact, so a row from
 * another organization or another user is never loaded into memory.
 */

const crypto = require('crypto');

const db = require('../db/driver');
const { getPortalId } = require('../utils/orgContext');

const TABLE = 'oauth2_consumer_keys';

// Named columns, never SELECT * (R7-NO-SELECT-STAR): a schema change should
// surface here rather than as a silently different row shape.
const COLUMNS = [
    'uuid',
    'org_uuid',
    'key_manager_id',
    'consumer_key',
    'name',
    'status',
    'created_by',
    'created_at',
    'updated_by',
    'updated_at',
].join(', ');

const STATUS_ACTIVE = 'ACTIVE';

/*
 * Normalize a timestamp column to RFC 3339, which is what `format: date-time`
 * in the OpenAPI spec requires.
 *
 * Each driver hands back something different: node-postgres and mssql produce a
 * JS Date, while SQLite returns CURRENT_TIMESTAMP as the string
 * "YYYY-MM-DD HH:MM:SS" — no `T`, no offset, and so not a valid date-time.
 *
 * Done here rather than centrally on purpose: the portal's shared audit builder
 * (userIdpReferenceDao.buildSingleAuditFields) passes the raw column straight
 * through, so every timestamped response has the same defect on SQLite. Fixing
 * that centrally would touch every endpoint at once and deserves its own
 * change, not a side effect of this table.
 */
function toIsoUtc(value) {
    if (value === null || value === undefined || value === '') return undefined;
    if (value instanceof Date) return value.toISOString();
    const text = String(value);
    // Already RFC 3339 (a `T` plus either Z or a numeric offset).
    if (/^\d{4}-\d{2}-\d{2}T.*(?:Z|[+-]\d{2}:?\d{2})$/.test(text)) return text;
    // SQLite's "YYYY-MM-DD HH:MM:SS" is UTC by definition of CURRENT_TIMESTAMP.
    const asUtc = new Date(text.replace(' ', 'T') + (/[Zz]|[+-]\d{2}/.test(text) ? '' : 'Z'));
    return Number.isNaN(asUtc.getTime()) ? undefined : asUtc.toISOString();
}

function toRecord(row) {
    if (!row) return null;
    return {
        keyId: row.uuid,
        keyManagerId: row.key_manager_id,
        consumerKey: row.consumer_key,
        name: row.name || '',
        status: row.status,
        createdBy: row.created_by,
        createdAt: toIsoUtc(row.created_at),
        updatedBy: row.updated_by,
        updatedAt: toIsoUtc(row.updated_at),
    };
}

/**
 * Insert the portal's record of a newly registered OAuth application.
 *
 * @param {object} params
 * @param {string} params.orgId
 * @param {string} params.keyManagerId  the config entry's `id`
 * @param {string} params.consumerKey   client_id issued by the key manager
 * @param {string} params.createdBy
 * @returns {Promise<object>} the stored record
 */
const create = async ({ orgId, keyManagerId, consumerKey, name, createdBy }) => {
    const uuid = crypto.randomUUID();
    await db.execute(
        `INSERT INTO ${TABLE} (
            uuid, org_uuid, portal_id, key_manager_id, consumer_key, name,
            status, created_by, updated_by
         ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [uuid, orgId, getPortalId(), keyManagerId, consumerKey, name || null,
            STATUS_ACTIVE, createdBy, createdBy]
    );
    return get(orgId, uuid, createdBy);
};

/**
 * One key, scoped to org + portal + creator.
 *
 * `createdBy` is part of the WHERE clause rather than a check on the returned
 * row: a key belonging to another user must not be loaded at all, and the
 * caller answers 404 for it so the response cannot confirm it exists.
 */
const get = async (orgId, keyId, createdBy) => {
    const row = await db.queryOne(
        `SELECT ${COLUMNS} FROM ${TABLE}
          WHERE portal_id = ? AND org_uuid = ? AND uuid = ? AND created_by = ?`,
        [getPortalId(), orgId, keyId, createdBy]
    );
    return toRecord(row);
};

/**
 * The caller's keys, newest first, optionally narrowed to one key manager.
 */
const listByCreator = async (orgId, createdBy, { keyManagerId } = {}) => {
    const params = [getPortalId(), orgId, createdBy];
    let sql = `SELECT ${COLUMNS} FROM ${TABLE}
                WHERE portal_id = ? AND org_uuid = ? AND created_by = ?`;
    if (keyManagerId) {
        sql += ' AND key_manager_id = ?';
        params.push(keyManagerId);
    }
    sql += ' ORDER BY created_at DESC';
    const rows = await db.query(sql, params);
    return rows.map(toRecord);
};

/**
 * How many keys in this organization reference a key manager, by ANY user.
 *
 * Deliberately not scoped to a creator: this answers whether a key manager is
 * still in use before it is deleted, and one user's keys are just as much in use
 * as another's. Matches on the handle, because that is what the column stores —
 * there is no foreign key here, which is precisely why deleting a key manager
 * would otherwise leave keys pointing at nothing.
 */
const countByKeyManager = async (orgId, keyManagerId) => {
    const row = await db.queryOne(
        `SELECT COUNT(*) AS total FROM ${TABLE}
          WHERE portal_id = ? AND org_uuid = ? AND key_manager_id = ?`,
        [getPortalId(), orgId, keyManagerId]
    );
    // COUNT comes back as a string on some drivers and a number on others.
    return Number((row && row.total) || 0);
};

/**
 * Rename a key.
 *
 * The name is the one piece of client metadata the portal keeps a copy of — the
 * rest lives at the key manager and is read back on demand. So when an update
 * changes `client_name` upstream, this is what stops the copy going stale.
 *
 * Scoped by creator like every other mutation here, so a rename cannot reach a
 * key the caller does not own.
 */
const setName = async (orgId, keyId, createdBy, name, updatedBy) => {
    const { rowCount } = await db.execute(
        `UPDATE ${TABLE}
            SET name = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP
          WHERE portal_id = ? AND org_uuid = ? AND uuid = ? AND created_by = ?`,
        [name || null, updatedBy, getPortalId(), orgId, keyId, createdBy]
    );
    return rowCount > 0;
};

/**
 * Move a key to a terminal state without deleting the row.
 * `created_by`/`created_at` never appear in an UPDATE (R7-CREATED-IMMUTABLE).
 */
const setStatus = async (orgId, keyId, createdBy, status, updatedBy) => {
    const { rowCount } = await db.execute(
        `UPDATE ${TABLE}
            SET status = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP
          WHERE portal_id = ? AND org_uuid = ? AND uuid = ? AND created_by = ?`,
        [status, updatedBy, getPortalId(), orgId, keyId, createdBy]
    );
    return rowCount > 0;
};

/**
 * Remove the portal's record. The OAuth application at the key manager is
 * deleted by the driver before this is called — dropping the row first would
 * leave a live client that nothing here can reach.
 */
const remove = async (orgId, keyId, createdBy) => {
    const { rowCount } = await db.execute(
        `DELETE FROM ${TABLE}
          WHERE portal_id = ? AND org_uuid = ? AND uuid = ? AND created_by = ?`,
        [getPortalId(), orgId, keyId, createdBy]
    );
    return rowCount > 0;
};

module.exports = {
    create,
    get,
    listByCreator,
    countByKeyManager,
    setName,
    setStatus,
    remove,
    STATUS_ACTIVE,
};
