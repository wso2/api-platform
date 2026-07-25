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

const session = require('express-session');
const db = require('./driver');
const logger = require('../config/logger');

const SESSIONS_TABLE = 'sessions';
const DEFAULT_TTL_MS = 60 * 60 * 1000; // 1 hour — matches the app's own cookie maxAge (src/app.js)

// Built once at module load — buildUpsert only depends on the (fixed) dialect and
// column list, not on any per-call data.
const UPSERT_SESSION_SQL = db.buildUpsert(SESSIONS_TABLE, ['sid', 'sess', 'expire'], ['sid'], ['sess', 'expire']);

function resolveExpiry(sessionData) {
    const expires = sessionData?.cookie?.expires;
    return expires ? new Date(expires) : new Date(Date.now() + DEFAULT_TTL_MS);
}

/**
 * Single express-session Store backing every dialect (sqlite/postgres/mssql),
 * replacing the previous per-dialect connect-session-sequelize / connect-pg-simple
 * split now that Sequelize is gone. Reads/writes the same `sessions` table
 * (sid/sess/expire) defined in every database/schema.*.sql file.
 */
class SqlSessionStore extends session.Store {
    constructor(options = {}) {
        super(options);
        const pruneIntervalMs = (options.pruneSessionInterval || 3600) * 1000;
        this._pruneTimer = setInterval(() => {
            db.execute(`DELETE FROM ${SESSIONS_TABLE} WHERE expire < ?`, [new Date()])
                .catch((err) => logger.warn('Session prune failed', { error: err.message }));
        }, pruneIntervalMs);
        // Don't hold the process open just for pruning (Node/browser env parity, matches
        // how the previous stores' background timers behaved).
        this._pruneTimer.unref?.();
    }

    async get(sid, callback) {
        try {
            const row = await db.queryOne(`SELECT sess, expire FROM ${SESSIONS_TABLE} WHERE sid = ?`, [sid]);
            if (!row) return callback(null, null);
            if (new Date(row.expire).getTime() <= Date.now()) {
                await this.destroy(sid, () => { /* best-effort cleanup of the expired row */ });
                return callback(null, null);
            }
            const sessionData = typeof row.sess === 'string' ? JSON.parse(row.sess) : row.sess;
            return callback(null, sessionData);
        } catch (err) {
            return callback(err);
        }
    }

    async set(sid, sessionData, callback) {
        try {
            const expire = resolveExpiry(sessionData);
            await db.execute(UPSERT_SESSION_SQL, [sid, JSON.stringify(sessionData), expire]);
            if (callback) callback(null);
        } catch (err) {
            if (callback) callback(err);
        }
    }

    async destroy(sid, callback) {
        try {
            await db.execute(`DELETE FROM ${SESSIONS_TABLE} WHERE sid = ?`, [sid]);
            if (callback) callback(null);
        } catch (err) {
            if (callback) callback(err);
        }
    }

    async touch(sid, sessionData, callback) {
        try {
            const expire = resolveExpiry(sessionData);
            await db.execute(`UPDATE ${SESSIONS_TABLE} SET expire = ? WHERE sid = ?`, [expire, sid]);
            if (callback) callback(null);
        } catch (err) {
            if (callback) callback(err);
        }
    }
}

module.exports = { SqlSessionStore };
