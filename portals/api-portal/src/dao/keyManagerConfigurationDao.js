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
 * Data access for `key_manager_configurations` — the driver configuration that
 * makes an API-created key manager usable for Dynamic Client Registration.
 *
 * One row per `key_managers` row, at most. A key manager without one is still a
 * valid record; it simply cannot register clients, which is the state every
 * key manager created before this table existed is in.
 *
 * Credentials are encrypted at rest with the portal's own key (AES-256-GCM, the
 * same `security.encryption_key` webhook_subscribers uses) and are decrypted in
 * exactly one place: `getWithSecrets`, which the driver builder calls. Every
 * other read goes through `get`, which never decrypts and never returns a
 * credential field — so a caller that only needs to render a form cannot leak
 * one by accident.
 *
 * Two things about the storage shape are deliberate and worth not undoing:
 *
 *   - The FK pair IS the primary key. There is no surrogate `uuid`, because the
 *     row has no identity apart from the key manager it configures, and a
 *     surrogate would need a separate UNIQUE constraint to re-impose the 1:0..1
 *     invariant the key already gives for free.
 *   - The method-specific credential fields live in one `auth_config` JSON
 *     column rather than a column each. They differ per auth method and will
 *     grow — mTLS needs cert paths, client_credentials may need
 *     `send_credentials_in_body` — and every addition as a column is three
 *     dialect ALTERs plus a migration this component has no framework for.
 *     Non-negotiable exception: the secret itself is never in the JSON. It has
 *     its own encrypted column so the JSON can be read, logged and diffed
 *     freely.
 */

const db = require('../db/driver');
const logger = require('../config/logger');
const { config } = require('../config/configLoader');
const { createCryptoUtil, bufferToUtf8 } = require('../utils/cryptoUtil');
const { parseJsonColumn } = require('../db/rows');
const { getPortalId } = require('../utils/orgContext');

const TABLE = 'key_manager_configurations';

// Built once. When no encryption key is configured this object throws on use
// rather than silently storing plaintext — the same fail-closed posture
// webhookSubscriberDao takes.
const kmCrypto = createCryptoUtil(config.security && config.security.encryptionKey);

// Named columns, never SELECT * (R7-NO-SELECT-STAR). The two *_enc columns are
// listed separately so a read has to opt in to them.
const PUBLIC_COLUMNS = [
    'key_manager_uuid',
    'org_uuid',
    'driver_type',
    'registration_endpoint',
    'authorize_endpoint',
    'description',
    'auth_method',
    'auth_config',
    'created_by',
    'created_at',
    'updated_by',
    'updated_at',
    // Presence, not the value. Derived in SQL so the public read can report
    // "a secret is held" without the ciphertext ever entering the process —
    // selecting the column and merely not returning it would put it one careless
    // log line away. CASE rather than a boolean expression, because SQL Server
    // has no boolean type to select.
    'CASE WHEN auth_secret_enc IS NULL THEN 0 ELSE 1 END AS has_secret',
].join(', ');

const SECRET_COLUMNS = `${PUBLIC_COLUMNS}, auth_secret_enc`;

function requireCrypto() {
    if (!kmCrypto.enabled) {
        throw new Error(
            'Key manager credentials cannot be stored: security.encryption_key is not configured. '
            + 'Set it to a 64-character hex string (openssl rand -hex 32).'
        );
    }
}

/**
 * The non-secret half of the credential, as stored in `auth_config`.
 *
 * Written with the key names the in-process record uses, so the mapping is one
 * line each way and there is no second vocabulary to keep straight. An unknown
 * key read back is preserved by nothing — this is the whole value — so a field
 * added by a newer build and read by an older one is simply ignored rather than
 * corrupting anything.
 */
function serializeAuthConfig(cfg) {
    const authConfig = {};
    if (cfg.authClientId) authConfig.clientId = cfg.authClientId;
    if (cfg.authUsername) authConfig.username = cfg.authUsername;
    if (Array.isArray(cfg.authScopes) && cfg.authScopes.length) authConfig.scopes = cfg.authScopes;
    if (cfg.authResource) authConfig.resource = cfg.authResource;
    // The header an API key travels in, and any scheme word before it. Neither
    // is secret — both are public facts about the key manager — so they belong
    // in the readable JSON, not the encrypted column.
    if (cfg.authHeaderName) authConfig.headerName = cfg.authHeaderName;
    if (cfg.authScheme) authConfig.scheme = cfg.authScheme;
    return Object.keys(authConfig).length ? JSON.stringify(authConfig) : null;
}

function toRecord(row) {
    if (!row) return null;
    // Postgres hands back a parsed object for JSONB; SQLite and SQL Server hand
    // back the string. parseJsonColumn absorbs the difference, and returns an
    // empty object rather than throwing on a row someone edited by hand.
    const authConfig = parseJsonColumn(row.auth_config) || {};
    // Comes from the SQL CASE above; a driver may hand back 1/0, true/false or
    // "1" depending on the dialect, so it is coerced rather than trusted.
    const hasSecret = Boolean(Number(row.has_secret));
    return {
        keyManagerUuid: row.key_manager_uuid,
        type: row.driver_type,
        registrationEndpoint: row.registration_endpoint,
        authorizeEndpoint: row.authorize_endpoint || '',
        description: row.description || '',
        authMethod: row.auth_method,
        authClientId: authConfig.clientId || '',
        authUsername: authConfig.username || '',
        authScopes: Array.isArray(authConfig.scopes) ? authConfig.scopes : [],
        authResource: authConfig.resource || '',
        authHeaderName: authConfig.headerName || '',
        authScheme: authConfig.scheme || '',
        // Not the credential itself — only whether one is held, which is what a
        // form needs in order to render "leave blank to keep the current secret".
        // One column holds the secret for whichever method is stored, so these
        // two are that one fact reported against the method it belongs to: after
        // a switch from basic to client_credentials, the held password is not a
        // client secret and must not read as one.
        hasSecret,
        hasClientSecret: hasSecret && row.auth_method === 'client_credentials',
        hasPassword: hasSecret && row.auth_method === 'basic',
        hasApiKey: hasSecret && row.auth_method === 'api_key',
        createdBy: row.created_by,
        createdAt: row.created_at,
        updatedBy: row.updated_by,
        updatedAt: row.updated_at,
    };
}

/**
 * The credential this configuration's auth method actually uses.
 *
 * One encrypted column, so which field it came from is decided by the method
 * rather than by the caller happening to populate one or the other.
 */
function secretFor(cfg) {
    if (cfg.authMethod === 'basic') return cfg.authPassword || '';
    if (cfg.authMethod === 'api_key') return cfg.authApiKey || '';
    return cfg.authClientSecret || '';
}

/**
 * Decrypt one stored credential.
 *
 * A failure here is not the caller's to interpret: a wrong or rotated encryption
 * key, or a truncated column, all mean the same thing operationally — this key
 * manager cannot be used until someone re-enters its credential. The cause is
 * logged; the caller gets a single clear error with no cipher detail in it.
 */
function decryptField(value, field, keyManagerUuid) {
    const payload = bufferToUtf8(value);
    if (!payload) return '';
    try {
        return kmCrypto.decrypt(payload);
    } catch (error) {
        logger.error('Failed to decrypt a stored key manager credential', {
            field, keyManagerUuid, error: error.message,
        });
        throw new Error(
            'A stored key manager credential could not be decrypted. '
            + 'Re-enter it in Settings — the portal\'s encryption key may have changed.'
        );
    }
}

/**
 * Insert the configuration for a key manager.
 *
 * @param {object} params
 * @param {string} params.orgId
 * @param {string} params.keyManagerUuid
 * @param {object} params.cfg  normalized configuration (see keyManagerService)
 * @param {string} params.createdBy
 */
const create = async ({ orgId, keyManagerUuid, cfg, createdBy }) => {
    requireCrypto();
    const secret = secretFor(cfg);
    await db.execute(
        `INSERT INTO ${TABLE} (
            key_manager_uuid, portal_id, org_uuid, driver_type,
            registration_endpoint, authorize_endpoint, description,
            auth_method, auth_config, auth_secret_enc,
            created_by, updated_by
         ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
            keyManagerUuid, getPortalId(), orgId, cfg.type,
            cfg.registrationEndpoint, cfg.authorizeEndpoint || null, cfg.description || null,
            cfg.authMethod, serializeAuthConfig(cfg),
            secret ? kmCrypto.encrypt(secret) : null,
            createdBy, createdBy,
        ]
    );
    return keyManagerUuid;
};

/**
 * Replace the configuration for a key manager.
 *
 * An omitted credential leaves the stored one alone rather than clearing it:
 * the form never renders a secret back, so "unchanged" is what an empty field
 * means. Clearing one is done by switching auth method, not by blanking a box.
 */
const update = async ({ orgId, keyManagerUuid, cfg, updatedBy }) => {
    requireCrypto();
    const sets = [
        'driver_type = ?', 'registration_endpoint = ?', 'authorize_endpoint = ?',
        'description = ?', 'auth_method = ?', 'auth_config = ?',
        'updated_by = ?', 'updated_at = CURRENT_TIMESTAMP',
    ];
    const params = [
        cfg.type, cfg.registrationEndpoint, cfg.authorizeEndpoint || null,
        cfg.description || null, cfg.authMethod, serializeAuthConfig(cfg),
        updatedBy,
    ];
    const secret = secretFor(cfg);
    if (secret) {
        sets.push('auth_secret_enc = ?');
        params.push(kmCrypto.encrypt(secret));
    }
    params.push(getPortalId(), orgId, keyManagerUuid);

    const { rowCount } = await db.execute(
        `UPDATE ${TABLE} SET ${sets.join(', ')}
          WHERE portal_id = ? AND org_uuid = ? AND key_manager_uuid = ?`,
        params
    );
    return rowCount > 0;
};

/** The configuration for one key manager, without any credential. */
const get = async (orgId, keyManagerUuid) => {
    const row = await db.queryOne(
        `SELECT ${PUBLIC_COLUMNS} FROM ${TABLE}
          WHERE portal_id = ? AND org_uuid = ? AND key_manager_uuid = ?`,
        [getPortalId(), orgId, keyManagerUuid]
    );
    return toRecord(row);
};

/**
 * Every configuration in the organization, keyed by key manager uuid.
 *
 * One query for the whole list, rather than a lookup per key manager: the
 * `/key-managers` listing needs only to know which entries have a configuration,
 * and N round trips for a boolean is the kind of thing that only shows up once
 * an organization has a few dozen of them.
 *
 * @returns {Promise<Map<string, object>>}
 */
const listByOrg = async (orgId) => {
    const rows = await db.query(
        `SELECT ${PUBLIC_COLUMNS} FROM ${TABLE} WHERE portal_id = ? AND org_uuid = ?`,
        [getPortalId(), orgId]
    );
    const byKeyManager = new Map();
    for (const row of rows || []) {
        byKeyManager.set(row.key_manager_uuid, toRecord(row));
    }
    return byKeyManager;
};

/**
 * The configuration WITH its credentials decrypted, for building a driver.
 *
 * The only decrypting read in the portal. Keep it that way: everything that
 * merely displays or lists a key manager uses `get`.
 */
const getWithSecrets = async (orgId, keyManagerUuid) => {
    const row = await db.queryOne(
        `SELECT ${SECRET_COLUMNS} FROM ${TABLE}
          WHERE portal_id = ? AND org_uuid = ? AND key_manager_uuid = ?`,
        [getPortalId(), orgId, keyManagerUuid]
    );
    if (!row) return null;
    requireCrypto();
    const record = toRecord(row);
    const secret = decryptField(row.auth_secret_enc, 'auth_secret_enc', keyManagerUuid);
    // Routed to the field the stored method uses. The others stay empty, which
    // is what buildAuthenticator expects — it reads only the one its method needs.
    record.authClientSecret = record.authMethod === 'client_credentials' ? secret : '';
    record.authPassword = record.authMethod === 'basic' ? secret : '';
    record.authApiKey = record.authMethod === 'api_key' ? secret : '';
    return record;
};

/**
 * Remove the configuration for a key manager.
 *
 * The foreign key cascades, so deleting the key manager already removes this
 * row. This exists for the narrower case: keeping the key manager but dropping
 * its ability to register clients.
 */
const removeByKeyManager = async (orgId, keyManagerUuid) => {
    const { rowCount } = await db.execute(
        `DELETE FROM ${TABLE} WHERE portal_id = ? AND org_uuid = ? AND key_manager_uuid = ?`,
        [getPortalId(), orgId, keyManagerUuid]
    );
    return rowCount > 0;
};

module.exports = {
    create,
    update,
    get,
    listByOrg,
    getWithSecrets,
    removeByKeyManager,
    encryptionAvailable: () => kmCrypto.enabled,
};
