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

const crypto = require('node:crypto');
const session = require('express-session');
const db = require('./driver');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const { getPortalId } = require('../utils/orgContext');
const { createCryptoUtil } = require('../utils/cryptoUtil');

const SESSIONS_TABLE = 'sessions';
const DEFAULT_TTL_MS = 60 * 60 * 1000; // 1 hour — matches the app's own cookie maxAge (src/app.js)

// Built once at module load — buildUpsert only depends on the (fixed) dialect and
// column list, not on any per-call data.
const UPSERT_SESSION_SQL = db.buildUpsert(
    SESSIONS_TABLE,
    ['portal_id', 'sid', 'sess', 'expire'],
    ['portal_id', 'sid'],
    ['sess', 'expire']
);

// Record format. Each row stores:
//   - `sess`: the session as a versioned, AES-256-GCM encrypted payload (security.encryption_key),
//     with portal_id and the stored sid bound in as AAD, so a row moved to another sid or
//     portal does not decrypt.
//   - `sid`: a SHA-256 hash of the cookie's session id, prefixed with SID_PREFIX.
// Rows in the earlier format have no SID_PREFIX, so they never match a lookup; they are
// removed at startup and the affected users simply sign in again.
const SID_PREFIX = 'h1:';
const PAYLOAD_VERSION = 1;

const storedSid = (sid) => SID_PREFIX + crypto.createHash('sha256').update(String(sid)).digest('hex');
const aad = (portalId, sidKey) => `${portalId}|${sidKey}`;

function resolveExpiry(sessionData) {
    const expires = sessionData?.cookie?.expires;
    return expires ? new Date(expires) : new Date(Date.now() + DEFAULT_TTL_MS);
}

/**
 * Single express-session Store backing every dialect (sqlite/postgres/mssql).
 * Reads and writes the `sessions` table (portal_id/sid/sess/expire) defined in
 * every database/schema.*.sql file; every query is scoped by portal_id so a
 * process can never observe another portal's session rows.
 */
class SqlSessionStore extends session.Store {
    constructor(options = {}) {
        super(options);
        this._crypto = options.crypto || createCryptoUtil(config.security && config.security.encryptionKey);
        if (!this._crypto.enabled) {
            // The record format requires a key; there is no unencrypted mode.
            throw new Error('SqlSessionStore requires a valid security.encryption_key (64-char hex).');
        }
        // Remove this portal's rows in the earlier format (no SID_PREFIX). Rows in the current
        // format, other replicas' included, and other portals' rows are left alone.
        db.execute(`DELETE FROM ${SESSIONS_TABLE} WHERE portal_id = ? AND sid NOT LIKE ?`,
            [getPortalId(), `${SID_PREFIX}%`])
            .catch((err) => logger.warn('Session record cleanup failed', { error: err.message }));
        const pruneIntervalMs = (options.pruneSessionInterval || 3600) * 1000;
        this._pruneTimer = setInterval(() => {
            // Pruning expired rows is safe to run unscoped: an expired row is dead
            // for every portal, and DELETE by expire only touches expired rows.
            db.execute(`DELETE FROM ${SESSIONS_TABLE} WHERE expire < ?`, [new Date()])
                .catch((err) => logger.warn('Session prune failed', { error: err.message }));
        }, pruneIntervalMs);
        // Don't hold the process open just for pruning (Node/browser env parity, matches
        // how the previous stores' background timers behaved).
        this._pruneTimer.unref?.();
    }

    async get(sid, callback) {
        try {
            const portalId = getPortalId();
            const sidKey = storedSid(sid);
            const row = await db.queryOne(
                `SELECT sess, expire FROM ${SESSIONS_TABLE} WHERE portal_id = ? AND sid = ?`,
                [portalId, sidKey]
            );
            if (!row) return callback(null, null);
            if (new Date(row.expire).getTime() <= Date.now()) {
                await this.destroy(sid, () => { /* best-effort cleanup of the expired row */ });
                return callback(null, null);
            }
            // A row that is malformed, fails authentication, or was written under another key (after a
            // key rotation) is "no session": the user signs in again. It is not a server error.
            try {
                const payload = typeof row.sess === 'string' ? JSON.parse(row.sess) : row.sess;
                if (!payload || payload.v !== PAYLOAD_VERSION || typeof payload.enc !== 'string') {
                    throw new Error('unrecognised session payload');
                }
                return callback(null, JSON.parse(this._crypto.decrypt(payload.enc, aad(portalId, sidKey))));
            } catch (err) {
                logger.warn('Ignoring unreadable session record', { error: err.message });
                return callback(null, null);
            }
        } catch (err) {
            return callback(err);
        }
    }

    async set(sid, sessionData, callback) {
        try {
            const expire = resolveExpiry(sessionData);
            const portalId = getPortalId();
            const sidKey = storedSid(sid);
            const enc = this._crypto.encrypt(JSON.stringify(sessionData), aad(portalId, sidKey));
            await db.execute(UPSERT_SESSION_SQL, [portalId, sidKey, JSON.stringify({ v: PAYLOAD_VERSION, enc }), expire]);
            if (callback) callback(null);
        } catch (err) {
            if (callback) callback(err);
        }
    }

    async destroy(sid, callback) {
        try {
            await db.execute(
                `DELETE FROM ${SESSIONS_TABLE} WHERE portal_id = ? AND sid = ?`,
                [getPortalId(), storedSid(sid)]
            );
            if (callback) callback(null);
        } catch (err) {
            if (callback) callback(err);
        }
    }

    async touch(sid, sessionData, callback) {
        try {
            const expire = resolveExpiry(sessionData);
            await db.execute(
                `UPDATE ${SESSIONS_TABLE} SET expire = ? WHERE portal_id = ? AND sid = ?`,
                [expire, getPortalId(), storedSid(sid)]
            );
            if (callback) callback(null);
        } catch (err) {
            if (callback) callback(err);
        }
    }
}

module.exports = { SqlSessionStore, storedSid };
