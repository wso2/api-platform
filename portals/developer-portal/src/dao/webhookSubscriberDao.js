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
const { parseJsonColumn } = require('../db/rows');
const { createCryptoUtil, bufferToUtf8 } = require('../utils/cryptoUtil');
const { config } = require('../config/configLoader');
const { NotFoundError } = require('../utils/errors/customErrors');

const TABLE = 'dp_webhook_subscribers';

const whCrypto = createCryptoUtil(config.security.encryptionKey);

/**
 * Normalizes a raw dp_webhook_subscribers row: BLOB columns back to utf8 strings
 * (mirrors the previous Sequelize attribute `get()`), and the JSON `event_patterns`
 * column back to a JS array.
 */
function toSubscriber(row) {
    if (!row) return row;
    return {
        ...row,
        secret_enc: bufferToUtf8(row.secret_enc),
        public_key: bufferToUtf8(row.public_key),
        event_patterns: parseJsonColumn(row.event_patterns),
    };
}

/**
 * Create a new webhook subscriber for an organization.
 * The secret is encrypted before storage.
 */
const create = async (orgId, subData, createdBy) => {
    if (subData.secret && !whCrypto.enabled) {
        throw new Error('Webhook subscriber encryption key is not configured. ' +
            'Set config.security.encryptionKey to a 64-char hex string.');
    }

    const uuid = crypto.randomUUID();
    const row = {
        uuid,
        org_uuid: orgId,
        handle: subData.handle,
        display_name: subData.displayName,
        target_url: subData.targetUrl,
        secret_enc: subData.secret ? whCrypto.encrypt(subData.secret) : null,
        public_key: subData.publicKey ? subData.publicKey : null,
        event_patterns: subData.events ? subData.events : [],
        enabled: subData.enabled !== undefined ? (subData.enabled ? 1 : 0) : 1,
        timeout_ms: subData.timeoutMs ? subData.timeoutMs : 5000,
        created_by: createdBy,
        updated_by: createdBy,
    };

    await db.execute(
        `INSERT INTO ${TABLE}
            (uuid, org_uuid, handle, display_name, target_url, secret_enc, public_key, event_patterns, enabled, timeout_ms, created_by, updated_by)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
            row.uuid, row.org_uuid, row.handle, row.display_name, row.target_url,
            row.secret_enc !== null ? Buffer.from(row.secret_enc, 'utf8') : null,
            row.public_key !== null ? Buffer.from(row.public_key, 'utf8') : null,
            JSON.stringify(row.event_patterns),
            row.enabled, row.timeout_ms, row.created_by, row.updated_by,
        ]
    );

    return row;
};

/**
 * Update an existing webhook subscriber.
 * Re-encrypts the secret if it is provided.
 */
const update = async (orgId, subscriberHandle, subData, updatedBy) => {
    const updatePayload = {
        ...(subData.handle && { handle: subData.handle }),
        ...(subData.displayName && { display_name: subData.displayName }),
        ...(subData.targetUrl && { target_url: subData.targetUrl }),
        ...(subData.publicKey !== undefined && { public_key: subData.publicKey }),
        ...(subData.events && { event_patterns: subData.events }),
        ...(subData.enabled !== undefined && { enabled: subData.enabled ? 1 : 0 }),
        ...(subData.timeoutMs && { timeout_ms: subData.timeoutMs }),
        updated_by: updatedBy,
        updated_at: new Date(),
    };

    if (subData.secret) {
        if (!whCrypto.enabled) {
            throw new Error('Webhook subscriber encryption key is not configured.');
        }
        updatePayload.secret_enc = whCrypto.encrypt(subData.secret);
    }

    const columns = Object.keys(updatePayload);
    const setClause = columns.map((c) => `${c} = ?`).join(', ');
    const values = columns.map((c) => {
        const value = updatePayload[c];
        if (c === 'event_patterns') return JSON.stringify(value);
        if (c === 'secret_enc' || c === 'public_key') {
            return value === null || value === undefined ? null : Buffer.from(value, 'utf8');
        }
        return value;
    });

    const { rowCount } = await db.execute(
        `UPDATE ${TABLE} SET ${setClause} WHERE handle = ? AND org_uuid = ?`,
        [...values, subscriberHandle, orgId]
    );
    if (rowCount < 1) {
        throw new NotFoundError('Webhook subscriber not found');
    }
    // Re-fetch explicitly rather than relying on a `RETURNING`-style clause, so the
    // result is reliable across every dialect (including sqlite, which has no
    // portable equivalent wired up here).
    const updated = await db.queryOne(
        `SELECT * FROM ${TABLE} WHERE handle = ? AND org_uuid = ?`,
        [updatePayload.handle || subscriberHandle, orgId]
    );
    return [rowCount, [toSubscriber(updated)]];
};

/**
 * List all webhook subscribers for an organization.
 */
const list = async (orgId) => {
    const rows = await db.query(`SELECT * FROM ${TABLE} WHERE org_uuid = ?`, [orgId]);
    return rows.map(toSubscriber);
};

/**
 * List enabled webhook subscribers across all organizations that match the
 * given event type. Used by the dispatcher fan-out.
 */
const matchSubscribers = async (orgId, eventType) => {
    const rows = await db.query(
        `SELECT * FROM ${TABLE} WHERE org_uuid = ? AND enabled = 1`,
        [orgId]
    );
    return rows.map(toSubscriber).filter((sub) => {
        const patterns = sub.event_patterns;
        if (Array.isArray(patterns) && patterns.length > 0) {
            const matches = patterns.some((pattern) => {
                if (pattern.endsWith('.*')) {
                    return eventType.startsWith(pattern.slice(0, -1));
                }
                return pattern === eventType;
            });
            if (!matches) return false;
        }
        return true;
    });
};

/**
 * Get a single webhook subscriber by UUID.
 */
const get = async (orgId, subscriberHandle) => {
    const sub = await db.queryOne(
        `SELECT * FROM ${TABLE} WHERE handle = ? AND org_uuid = ?`,
        [subscriberHandle, orgId]
    );
    if (!sub) {
        throw new NotFoundError('Webhook subscriber not found');
    }
    return toSubscriber(sub);
};

/**
 * Get a single webhook subscriber by UUID only, without scoping to an org.
 * UUID is a globally unique UUID primary key, so this is safe.
 * Used by the delivery worker, which only has the subscriber UUID (from the
 * delivery row) and not the org UUID in scope.
 */
const getById = async (subscriberId) => {
    const sub = await db.queryOne(`SELECT * FROM ${TABLE} WHERE uuid = ?`, [subscriberId]);
    if (!sub) {
        throw new NotFoundError('Webhook subscriber not found');
    }
    return toSubscriber(sub);
};

/**
 * Delete a webhook subscriber.
 */
const deleteSubscriber = async (orgId, subscriberHandle) => {
    const { rowCount } = await db.execute(
        `DELETE FROM ${TABLE} WHERE handle = ? AND org_uuid = ?`,
        [subscriberHandle, orgId]
    );
    if (rowCount < 1) {
        throw new NotFoundError('Webhook subscriber not found');
    }
    return rowCount;
};

/**
 * Decrypt the secret for a webhook subscriber record.
 * Used internally by the delivery worker to sign outgoing requests.
 */
const decryptSecret = (subRecord) => {
    if (!subRecord.secret_enc) return null;
    if (!whCrypto.enabled) {
        throw new Error('Webhook subscriber encryption key is not configured.');
    }
    return whCrypto.decrypt(subRecord.secret_enc);
};

module.exports = {
    create,
    update,
    list,
    matchSubscribers,
    get,
    getById,
    delete: deleteSubscriber,
    decryptSecret,
};
