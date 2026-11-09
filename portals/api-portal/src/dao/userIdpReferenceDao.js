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
const { findOrCreateSafe } = require('./findOrCreateHelper');
const { getPortalId } = require('../utils/orgContext');
const logger = require('../config/logger');

const TABLE = 'user_idp_references';
const DELETED_USER = 'deleted_user';

// Width of user_idp_references.display_name in UTF-16 code units, the unit SQL
// Server's NVARCHAR(255) counts (and JS string length measures). Capping in these
// units also fits Postgres VARCHAR(255), which counts characters.
const MAX_DISPLAY_NAME_UNITS = 255;

// Control characters, NUL among them (Postgres rejects it in text).
const CONTROL_CHARS = /\p{Cc}/gu;

/**
 * Trims a token-supplied display name, drops control characters and caps it to
 * MAX_DISPLAY_NAME_UNITS UTF-16 code units without splitting a character, so an
 * overlong claim fits the column on every engine. Non-strings become ''.
 */
const normalizeDisplayName = (name) => {
    if (typeof name !== 'string') return '';
    const cleaned = name.replace(CONTROL_CHARS, '').trim();
    if (cleaned.length <= MAX_DISPLAY_NAME_UNITS) return cleaned;
    let capped = '';
    for (const char of cleaned) {
        if (capped.length + char.length > MAX_DISPLAY_NAME_UNITS) break;
        capped += char;
    }
    return capped.trim();
};

/**
 * Find-or-create the idp reference row for this idp_id, returning its uuid.
 * Portal-scoped: each portal maintains its own view of an IdP sub claim; the
 * same idp_id used across two portals yields two rows, one per portal. Falls
 * back to a plain lookup on a unique-constraint race between concurrent
 * requests for the same (portal_id, idp_id) pair.
 *
 * displayName (the configured display-name claim) is optional. The row is found
 * or created without it, so a name the database rejects can never stop the user
 * being mapped; the name is then saved by a separate update, made only when it
 * is non-empty and differs from the stored one, so an empty value never clears a
 * stored name. A failure to save the name is logged (without the driver message,
 * which can quote the value) and does not fail the lookup.
 *
 * Resolves to { uuid, displayNameSaved }: displayNameSaved is false only when a
 * name was given and could not be saved.
 */
const resolveReference = async (idpId, displayName) => {
    const portalId = getPortalId();
    const name = normalizeDisplayName(displayName);
    const reference = await findOrCreateSafe(
        TABLE,
        { portal_id: portalId, idp_id: idpId },
        { portal_id: portalId, uuid: crypto.randomUUID(), idp_id: idpId }
    );
    if (!name || reference.display_name === name) {
        return { uuid: reference.uuid, displayNameSaved: true };
    }
    try {
        await db.execute(
            `UPDATE ${TABLE} SET display_name = ? WHERE portal_id = ? AND uuid = ?`,
            [name, portalId, reference.uuid]
        );
        return { uuid: reference.uuid, displayNameSaved: true };
    } catch (err) {
        logger.warn('Failed to save the user\'s display name; continuing without it', {
            code: err.code, operation: 'resolveReference',
        });
        return { uuid: reference.uuid, displayNameSaved: false };
    }
};

/** resolveReference, resolving to the uuid alone. */
const resolveUuid = async (idpId, displayName) => (await resolveReference(idpId, displayName)).uuid;

/**
 * Resolve a single created_by/updated_by-style uuid to its raw idp_id, returning
 * "deleted_user" when the reference no longer exists in this portal. For
 * identities other systems match on (e.g. the subscription webhook's
 * subscriber_id); REST responses show display names via the audit builders.
 */
const resolveDisplay = async (uuid) => {
    if (!uuid) return DELETED_USER;
    const reference = await db.queryOne(
        `SELECT * FROM ${TABLE} WHERE portal_id = ? AND uuid = ?`,
        [getPortalId(), uuid]
    );
    return reference ? reference.idp_id : DELETED_USER;
};

/**
 * Batch-resolve multiple uuids to their raw idp_id in one query. Returns a
 * Map<uuid, idp_id | "deleted_user"> covering every uuid passed in.
 */
const resolveMany = async (uuids) => {
    const distinctUuids = [...new Set((uuids || []).filter(Boolean))];
    const result = new Map(distinctUuids.map((uuid) => [uuid, DELETED_USER]));
    if (distinctUuids.length === 0) return result;

    const placeholders = distinctUuids.map(() => '?').join(', ');
    const references = await db.query(
        `SELECT * FROM ${TABLE} WHERE portal_id = ? AND uuid IN (${placeholders})`,
        [getPortalId(), ...distinctUuids]
    );
    for (const reference of references) {
        result.set(reference.uuid, reference.idp_id);
    }
    return result;
};

/**
 * Batch-resolve created_by/updated_by-style uuids to what a REST response shows:
 * the stored display name, else the idp_id (so rows from before display names
 * were captured, or users whose token never carried the claim, render exactly
 * as before), else "deleted_user" when the reference no longer exists in this
 * portal. Returns a Map with an entry for every distinct non-empty uuid.
 */
const resolveDisplayNames = async (uuids) => {
    const distinctUuids = [...new Set((uuids || []).filter(Boolean))];
    const result = new Map(distinctUuids.map((uuid) => [uuid, DELETED_USER]));
    if (distinctUuids.length === 0) return result;

    const placeholders = distinctUuids.map(() => '?').join(', ');
    const references = await db.query(
        `SELECT uuid, idp_id, display_name FROM ${TABLE} WHERE portal_id = ? AND uuid IN (${placeholders})`,
        [getPortalId(), ...distinctUuids]
    );
    for (const reference of references) {
        const name = typeof reference.display_name === 'string' ? reference.display_name.trim() : '';
        result.set(reference.uuid, name || reference.idp_id || DELETED_USER);
    }
    return result;
};

/**
 * Resolved createdBy/updatedBy/createdAt/updatedAt for a single-resource GET
 * response, with createdBy/updatedBy as display names (see resolveDisplayNames).
 * Batches both lookups into one query.
 */
const buildSingleAuditFields = async (model) => {
    const display = await resolveDisplayNames([model.created_by, model.updated_by]);
    return {
        createdBy: display.get(model.created_by) ?? DELETED_USER,
        updatedBy: display.get(model.updated_by) ?? DELETED_USER,
        createdAt: model.created_at,
        updatedAt: model.updated_at,
    };
};

/**
 * Resolved createdBy/createdAt/updatedAt (no updatedBy) for each model in a
 * list response, with createdBy as a display name (see resolveDisplayNames),
 * batching the created_by lookup into a single query.
 */
const buildListAuditFields = async (models) => {
    const display = await resolveDisplayNames(models.map((model) => model.created_by));
    return models.map((model) => ({
        createdBy: display.get(model.created_by) ?? DELETED_USER,
        createdAt: model.created_at,
        updatedAt: model.updated_at,
    }));
};

module.exports = {
    DELETED_USER,
    resolveUuid,
    resolveReference,
    resolveDisplay,
    resolveMany,
    resolveDisplayNames,
    normalizeDisplayName,
    buildSingleAuditFields,
    buildListAuditFields,
};
