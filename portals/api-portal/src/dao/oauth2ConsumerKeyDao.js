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
 * The row holds the identity of the registered client, plus whatever RFC 7592
 * gave the portal to manage it with. Two things are deliberately not stored:
 *
 *   - The client secret. The key manager returns it once, at registration, and
 *     never again, so the create response is the only place it can appear.
 *   - The client metadata (redirect_uris, grant_types, …). It lives at the key
 *     manager and is re-read from there, so there is one copy of it.
 *
 * The RFC 7592 registration access token and client configuration URI are kept:
 * `registration_access_token_enc` (encrypted at rest with
 * security.encryption_key) and `registration_client_uri`. Neither rides along on
 * an ordinary read — they are outside COLUMNS, and `getWithRegistration` is the
 * one path that opts in — so a list or detail response never carries a
 * credential it has no use for.
 *
 * Both are NULL for a key manager that issues no such token. A key in that state
 * is managed with the portal's own provisioning credential against a constructed
 * <registration_endpoint>/<consumer_key>, which is how every key behaved before
 * these columns existed and remains the fallback when a stored token is rejected.
 *
 * The key↔application association lives in oauth2_consumer_key_app_mappings, not
 * as a column here.
 *
 * Every query is scoped by `portal_id` AND `org_uuid`, and the ownership filter
 * (`created_by`) is applied in SQL rather than after the fact, so a row from
 * another organization or another user is never loaded into memory.
 */

const crypto = require('crypto');

const db = require('../db/driver');
const { getPortalId } = require('../utils/orgContext');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const { createCryptoUtil, bufferToUtf8 } = require('../utils/cryptoUtil');

const TABLE = 'oauth2_consumer_keys';

// Named columns, never SELECT * (R7-NO-SELECT-STAR): a schema change should
// surface here rather than as a silently different row shape.
/*
 * Built once. With no encryption key configured this throws on use rather than
 * silently storing a plaintext credential — the same fail-closed posture
 * keyManagerConfigurationDao takes.
 */
const keyCrypto = createCryptoUtil(config.security && config.security.encryptionKey);

// Named columns, never SELECT * (R7-NO-SELECT-STAR). The encrypted registration
// access token is NOT here: a read has to opt in to it, so the ordinary list and
// detail paths cannot carry a credential they have no use for.
const COLUMNS = [
    'uuid',
    'org_uuid',
    'key_manager_id',
    'consumer_key',
    'name',
    'key_type',
    'status',
    'registration_client_uri',
    'created_by',
    'created_at',
    'updated_by',
    'updated_at',
].join(', ');

const COLUMNS_WITH_REGISTRATION = `${COLUMNS}, registration_access_token_enc`;

function requireCrypto() {
    if (!keyCrypto.enabled) {
        throw new Error(
            'A registration access token cannot be stored: security.encryption_key is not '
            + 'configured. Set it to a 64-character hex string (openssl rand -hex 32).'
        );
    }
}

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
    /*
     * SQLite's "YYYY-MM-DD HH:MM:SS" is UTC by definition of CURRENT_TIMESTAMP, so
     * a Z has to be appended before parsing — without one, JS reads the string as
     * LOCAL time and the value silently shifts by the host's offset.
     *
     * The zone test is anchored to the end for that reason. An unanchored
     * /[Zz]|[+-]\d{2}/ matches the hyphen in the DATE itself ("2026-10-02"), so it
     * concluded every timestamp already carried an offset and never appended the Z
     * — shifting every SQLite timestamp by the server's UTC offset. It went
     * unnoticed while this column only ever rendered a date.
     */
    const hasZoneSuffix = /(?:[Zz]|[+-]\d{2}:?\d{2})$/.test(text);
    const asUtc = new Date(text.replace(' ', 'T') + (hasZoneSuffix ? '' : 'Z'));
    return Number.isNaN(asUtc.getTime()) ? undefined : asUtc.toISOString();
}

function toRecord(row) {
    if (!row) return null;
    return {
        keyId: row.uuid,
        keyManagerId: row.key_manager_id,
        consumerKey: row.consumer_key,
        name: row.name || '',
        // Copied from the key manager when this key was created, never derived at
        // read time — see the column comment in database/schema.*.sql.
        keyType: row.key_type || constants.KEY_TYPE.PRODUCTION,
        status: row.status,
        // The URI is not a credential — it is the address the token is used at, and
        // the driver needs it to build the request. The token itself arrives only
        // through getWithRegistration below.
        registrationClientUri: row.registration_client_uri || '',
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
const create = async ({ orgId, keyManagerId, consumerKey, name, keyType, createdBy, registration }) => {
    const uuid = crypto.randomUUID();
    // Only a key manager that implements RFC 7592 sends these. Absent, both stay
    // NULL and this key is managed with the portal's provisioning credential.
    const accessToken = (registration && registration.accessToken) || '';
    const clientUri = (registration && registration.clientUri) || '';
    if (accessToken) requireCrypto();
    await db.execute(
        `INSERT INTO ${TABLE} (
            uuid, org_uuid, portal_id, key_manager_id, consumer_key, name, key_type,
            status, registration_access_token_enc, registration_client_uri,
            created_by, updated_by
         ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [uuid, orgId, getPortalId(), keyManagerId, consumerKey, name || null,
            keyType || constants.KEY_TYPE.PRODUCTION,
            STATUS_ACTIVE,
            accessToken ? keyCrypto.encrypt(accessToken) : null,
            clientUri || null,
            createdBy, createdBy]
    );
    return get(orgId, uuid, createdBy);
};

/**
 * The key, plus its decrypted registration access token.
 *
 * Separate from `get` so the credential is fetched only where it is about to be
 * used — the four driver calls — rather than riding along on every list and
 * detail response.
 *
 * A token that cannot be decrypted is reported as absent rather than thrown:
 * after `security.encryption_key` is rotated every stored token is unreadable,
 * and failing here would make those keys permanently unmanageable. Absent means
 * the caller falls back to the provisioning credential, which is exactly the
 * recovery path a stale token takes.
 */
const getWithRegistration = async (orgId, keyId, createdBy) => {
    const rows = await db.query(
        `SELECT ${COLUMNS_WITH_REGISTRATION} FROM ${TABLE}
          WHERE portal_id = ? AND org_uuid = ? AND uuid = ? AND created_by = ?`,
        [getPortalId(), orgId, keyId, createdBy]
    );
    if (!rows.length) return null;
    const record = toRecord(rows[0]);
    let accessToken = '';
    const payload = bufferToUtf8(rows[0].registration_access_token_enc);
    if (payload) {
        try {
            accessToken = keyCrypto.decrypt(payload);
        } catch (error) {
            logger.warn('Stored registration access token could not be decrypted; '
                + 'falling back to the provisioning credential', {
                keyId, orgId, error: error.message,
            });
        }
    }
    return {
        ...record,
        registration: { accessToken, clientUri: record.registrationClientUri },
    };
};

/**
 * Replace the stored registration credentials.
 *
 * Called after every driver response that carried one, because RFC 7592 §5 lets a
 * server rotate the token on any read or update — Keycloak does so on update and
 * kills the previous token immediately, so a missed write leaves the key
 * unmanageable until the fallback clears it.
 *
 * `{ accessToken: '' }` clears the pair, which is how a rejected token is
 * discarded.
 */
const setRegistration = async (orgId, keyId, createdBy, { accessToken, clientUri }, updatedBy) => {
    if (accessToken) requireCrypto();
    await db.execute(
        `UPDATE ${TABLE}
            SET registration_access_token_enc = ?, registration_client_uri = ?,
                updated_by = ?, updated_at = CURRENT_TIMESTAMP
          WHERE portal_id = ? AND org_uuid = ? AND uuid = ? AND created_by = ?`,
        [accessToken ? keyCrypto.encrypt(accessToken) : null, clientUri || null,
            updatedBy, getPortalId(), orgId, keyId, createdBy]
    );
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
    getWithRegistration,
    setRegistration,
    countByKeyManager,
    setName,
    setStatus,
    remove,
    STATUS_ACTIVE,
};
