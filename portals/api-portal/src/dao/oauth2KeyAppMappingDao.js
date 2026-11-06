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
 * Data access for `oauth2_consumer_key_app_mappings` — which application an
 * OAuth2 key belongs to.
 *
 * A key belongs to at most one application, so the key is the primary key here
 * and re-associating updates the row in place. In the other direction an
 * application holds at most one key per key type — one PRODUCTION and one
 * SANDBOX. That second rule has no constraint behind it: `key_type` lives on
 * `oauth2_consumer_keys`, not on this table, so a UNIQUE constraint cannot
 * express it without denormalising the column, and this table has shipped.
 * `associateExclusive` below is what enforces it: the check and the write run
 * inside one transaction that first locks the application row, because a read
 * followed by an unserialized write is a race, not a rule.
 *
 * This table carries no `org_uuid`. Both ends are already org-scoped by their own
 * rows, and the service resolves each of them through its own DAO — which applies
 * the organization and ownership filters — before this module is reached. Adding
 * a third copy of the organization here would be a value that could disagree with
 * the two that actually govern access. The same reasoning `app_key_mappings`, the
 * table this one supersedes, already follows.
 */

const db = require('../db/driver');
const { getPortalId } = require('../utils/orgContext');

const TABLE = 'oauth2_consumer_key_app_mappings';

/*
 * Built once at module load: buildUpsert depends only on the dialect and the
 * column list, not on per-call data. It emits ON CONFLICT for postgres/sqlite and
 * a MERGE for SQL Server, so associate() needs no dialect branch — and a
 * hand-rolled update-then-insert here would have a race the primary key would
 * then surface as a duplicate-key error on the loser.
 *
 * `created_by` is in the update list on purpose: a re-association is the same row
 * changing hands, so the column records who last attached the key rather than who
 * first did. api_key_app_mappings makes exactly the same choice.
 */
const UPSERT_SQL = db.buildUpsert(
    TABLE,
    ['portal_id', 'key_uuid', 'app_uuid', 'created_by'],
    ['portal_id', 'key_uuid'],
    ['app_uuid', 'created_by']
);

/**
 * Attach a key to an application, enforcing one key per key type.
 *
 * The check and the write are one transaction that first locks the application
 * row. Done as two unlocked statements, two requests associating a production
 * key with the same application both pass the check and both write — the row
 * lock is what makes them queue instead. The application row is the thing to
 * lock because the invariant belongs to the application, not to either key.
 *
 * Locked per dialect, as `subscriptionPlanDao.deletePlan` already does it:
 * PostgreSQL takes `FOR UPDATE`, SQL Server `UPDLOCK, ROWLOCK`, and SQLite needs
 * neither — its adapter holds a process-wide lock for the whole BEGIN..COMMIT
 * window, so transactions there are already serialized.
 *
 * Deliberately NOT scoped by creator. The rule is a property of the application
 * — "this application has a production key" — not of whoever is looking.
 * Filtering by `created_by` would let a second production key through whenever
 * the incumbent belonged to someone else, which is the case the rule exists to
 * catch.
 *
 * @returns {Promise<string|null>} the id of the key already occupying this
 *          type's slot, or null when the association was made.
 */
const associateExclusive = async (keyId, appId, keyType, actor) => db.withTransaction(async (tx) => {
    const dialect = db.getDialect();
    const lockHint = dialect === 'mssql' ? ' WITH (UPDLOCK, ROWLOCK)' : '';
    const forUpdate = dialect === 'postgres' ? ' FOR UPDATE' : '';
    await tx.queryOne(
        `SELECT uuid FROM applications${lockHint} WHERE portal_id = ? AND uuid = ?${forUpdate}`,
        [getPortalId(), appId]
    );

    /*
     * Already on this application: a no-op that succeeds, decided before the
     * conflict check rather than after it. An application written before this
     * rule existed can hold two keys of one type, and the check below would then
     * refuse to re-associate either of them — the one case the API explicitly
     * promises succeeds unchanged. Returning here also leaves the original
     * `created_by`/`created_at` alone, which an upsert would overwrite for an
     * association that did not change.
     */
    const current = await tx.queryOne(
        `SELECT app_uuid FROM ${TABLE} WHERE portal_id = ? AND key_uuid = ?`,
        [getPortalId(), keyId]
    );
    if (current && current.app_uuid === appId) return null;

    const conflict = await tx.queryOne(
        `SELECT m.key_uuid FROM ${TABLE} m
           JOIN oauth2_consumer_keys k
             ON k.portal_id = m.portal_id AND k.uuid = m.key_uuid
          WHERE m.portal_id = ? AND m.app_uuid = ? AND k.key_type = ? AND m.key_uuid <> ?`,
        [getPortalId(), appId, keyType, keyId]
    );
    if (conflict) return conflict.key_uuid;

    await tx.execute(UPSERT_SQL, [getPortalId(), keyId, appId, actor]);
    return null;
});

/**
 * Detach a key from whatever application it was on.
 *
 * @returns {Promise<boolean>} whether a row was actually removed. The caller
 *          answers 204 either way — the spec makes dissociate idempotent — but
 *          the distinction is worth having for the audit log.
 */
const dissociate = async (keyId) => {
    const { rowCount } = await db.execute(
        `DELETE FROM ${TABLE} WHERE portal_id = ? AND key_uuid = ?`,
        [getPortalId(), keyId]
    );
    return rowCount > 0;
};

/** The application a key is attached to, or null. */
const getByKey = async (keyId) => {
    const row = await db.queryOne(
        `SELECT app_uuid FROM ${TABLE} WHERE portal_id = ? AND key_uuid = ?`,
        [getPortalId(), keyId]
    );
    return row ? row.app_uuid : null;
};

/**
 * The application each of these keys is attached to, as a Map.
 *
 * One query for a whole page rather than a lookup per key. The id list is
 * expanded into placeholders rather than interpolated — these are uuids from the
 * caller's own rows, but a DAO that builds SQL by concatenation is one refactor
 * away from doing it with something that isn't.
 *
 * @returns {Promise<Map<string, string>>} keyId → appId
 */
const getByKeys = async (keyIds) => {
    const result = new Map();
    if (!keyIds || !keyIds.length) return result;
    const placeholders = keyIds.map(() => '?').join(', ');
    const rows = await db.query(
        `SELECT key_uuid, app_uuid FROM ${TABLE}
          WHERE portal_id = ? AND key_uuid IN (${placeholders})`,
        [getPortalId(), ...keyIds]
    );
    for (const row of rows || []) {
        result.set(row.key_uuid, row.app_uuid);
    }
    return result;
};

/** Every key attached to one application, newest association first. */
const listKeyIdsByApplication = async (appId) => {
    const rows = await db.query(
        `SELECT key_uuid FROM ${TABLE}
          WHERE portal_id = ? AND app_uuid = ?
          ORDER BY created_at DESC`,
        [getPortalId(), appId]
    );
    return (rows || []).map((row) => row.key_uuid);
};

module.exports = {
    associateExclusive,
    dissociate,
    getByKey,
    getByKeys,
    listKeyIdsByApplication,
};
