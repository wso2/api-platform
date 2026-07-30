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
 * The single organization this portal instance serves.
 *
 * The database schema is multi-organization — one shared database can hold many
 * organizations, each served by its own portal instance — but a given instance is
 * pinned to exactly one, named by `organization.handle` in config. This module is
 * the only place that resolves it, and every org-scoped surface goes through here:
 *
 *   - authMiddleware.js  — verifies a token/header-supplied org resolves to the pin
 *   - orgGuard.js        — verifies the {orgHandle} URL segment matches the pin
 *   - webhook workers    — scope their global claim queries to the pinned org
 *
 * The organization row itself is seeded on startup (seederService.js), so
 * getOrgUuid() is expected to succeed from then on. It is resolved lazily rather
 * than at import time because this module is required by middleware that loads
 * before the database is ready.
 *
 * A note on the eventual portalId: the plan is for one organization to hold several
 * portals, with an instance pinned to one portal under one organization. That lands
 * as a getPortalId()/assertPortalPinned() pair alongside these, so callers keep
 * asking this module "what am I scoped to?" and nothing else has to change.
 */

const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const orgDao = require('../dao/organizationDao');
const { CustomError, NotFoundError } = require('./errors/customErrors');

// Resolved once and reused: getOrgUuid() is called on essentially every request,
// and the uuid of a given handle never changes (updateOrganization cannot change
// the handle, and the row is never re-created under a different uuid). Cleared on
// a NotFound so a database that was reset out from under a running process
// recovers on the next request instead of failing until restart.
let cachedOrgUuid = null;
let pendingLookup = null;

/**
 * Handle of the organization this instance serves. Always lowercase — normalized
 * and format-validated at config load (configLoader.js#resolveOrganizationConfig),
 * which also refuses to start when it is missing, so this is never empty outside
 * design mode.
 *
 * @returns {string}
 */
function getHandle() {
    return config.organization?.handle || '';
}

/**
 * Display name to use when seeding the organization for the first time. Falls back
 * to the handle so a deployment that sets only `handle` still gets a sensible name.
 *
 * @returns {string}
 */
function getDisplayName() {
    return config.organization?.displayName || getHandle();
}

/**
 * Resolves — and caches — the uuid of this instance's organization.
 *
 * Throws NotFoundError if the organization row doesn't exist. That is expected only
 * in the window before seeding completes; callers on the request path treat it as a
 * server error rather than translating it into a client-visible 404, since it means
 * the portal itself is misconfigured, not that the caller asked for something absent.
 *
 * @returns {Promise<string>} the organization's uuid
 */
async function getOrgUuid() {
    if (cachedOrgUuid) return cachedOrgUuid;

    // Collapse concurrent first-time lookups into one query — without this, a burst
    // of requests at startup would each issue their own SELECT.
    if (!pendingLookup) {
        pendingLookup = orgDao
            .getByHandle(getHandle())
            .then((org) => {
                cachedOrgUuid = org.uuid;
                return cachedOrgUuid;
            })
            .finally(() => {
                pendingLookup = null;
            });
    }
    return pendingLookup;
}

/**
 * Drops the cached uuid, forcing the next getOrgUuid() to re-query. Called by the
 * seeder once it has created/verified the organization, so a uuid cached during the
 * pre-seed window can't go stale.
 */
function resetCache() {
    cachedOrgUuid = null;
}

/**
 * True when `uuid` is this instance's organization.
 *
 * Compares uuids rather than handles on purpose: an org identifier arriving from a
 * token claim is often the idp_ref_id (or display name), not the handle, so the
 * caller resolves it through orgDao first and compares the resolved row. That makes
 * every equivalent spelling of "this organization" match, and every spelling of any
 * other organization not match.
 *
 * @param {string} uuid
 * @returns {Promise<boolean>}
 */
async function isPinnedOrg(uuid) {
    if (!uuid) return false;
    try {
        return uuid === (await getOrgUuid());
    } catch (err) {
        if (err instanceof NotFoundError) {
            resetCache();
            logger.error('Pinned organization could not be resolved', {
                handle: getHandle(),
                operation: 'isPinnedOrg',
            });
            return false;
        }
        throw err;
    }
}

/**
 * Resolves a caller-supplied organization identifier (a handle, display name, or
 * idp_ref_id — whatever an {orgId} path parameter carries) and returns its uuid,
 * throwing CustomError(403) unless it names this instance's organization.
 *
 * Lives here, and is called from the service layer, so every entry point that
 * reaches an organization-scoped service method is covered — not just the REST
 * handler that happens to be wired up today.
 *
 * An unknown organization and a known-but-foreign one both yield the same 403: a
 * caller must not be able to use the difference to discover which organizations
 * exist in the shared database.
 *
 * @param {string} identifier
 * @returns {Promise<string>} the resolved uuid, guaranteed to be the pinned org
 * @throws {CustomError} 403 when it is any other organization
 */
async function requirePinnedOrg(identifier) {
    let resolvedUuid = null;
    try {
        resolvedUuid = await orgDao.getId(identifier);
    } catch (err) {
        if (!(err instanceof NotFoundError)) throw err;
    }

    if (resolvedUuid && (await isPinnedOrg(resolvedUuid))) return resolvedUuid;

    logger.warn('Rejected operation on a non-local organization', {
        requested: identifier,
        expected: getHandle(),
        operation: 'requirePinnedOrg',
    });
    throw new CustomError(403, 'Forbidden', 'This operation is not available for the requested organization.');
}

module.exports = {
    getHandle,
    getDisplayName,
    getOrgUuid,
    isPinnedOrg,
    requirePinnedOrg,
    resetCache,
};
