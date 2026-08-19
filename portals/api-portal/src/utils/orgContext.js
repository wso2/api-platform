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
 * The single organization and portal this portal instance serves.
 *
 * The database schema is multi-organization — one shared database can hold many
 * organizations, each served by its own portal instance — but a given instance is
 * pinned to exactly one org, named by `organization.handle` in config, and exactly
 * one portal, identified by `organization.portal_id` in config.toml, resolved
 * from the APIP_AP_ORGANIZATION_PORTAL_ID env var via the {{ env }} template token.
 * This module is the only place that resolves both,
 * and every org/portal-scoped surface goes through here:
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
 * getPortalId() is synchronous — env vars and config are stable at startup —
 * so DAO callers do not need to await it. The value is never accepted from
 * request input: that would be equivalent to accepting org_id from the request,
 * which is the IDOR vulnerability class described in JS-AUTH-005.
 */

const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const orgDao = require('../dao/organizationDao');
const viewDao = require('../dao/viewDao');
const { CustomError, NotFoundError } = require('./errors/customErrors');

// Resolved once and reused: getOrgUuid() is called on essentially every request,
// and the uuid of a given handle never changes (updateOrganization cannot change
// the handle, and the row is never re-created under a different uuid). Cleared on
// a NotFound so a database that was reset out from under a running process
// recovers on the next request instead of failing until restart.
let cachedOrgUuid = null;
let pendingLookup = null;

// Cached after first call. The value is stable for the lifetime of the process:
// it is read from the environment or config at startup and never changes.
let cachedPortalId = null;

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
 * Handle of the view to land on when a URL names no view — see
 * viewDao.getFallbackHandle for the choice it makes.
 *
 * Never throws: two of its callers are the bare-org redirect and the error page's home
 * link, and neither has anywhere useful to fail to. A lookup failure (database not yet
 * reachable, organization not seeded) degrades to the conventional 'default' handle,
 * which is what these sites hardcoded before this existed.
 *
 * @returns {Promise<string>}
 */
async function getFallbackViewHandle() {
    try {
        return await viewDao.getFallbackHandle(await getOrgUuid());
    } catch (err) {
        logger.warn('Falling back to the default view handle', {
            handle: getHandle(),
            error: err.message,
            operation: 'getFallbackViewHandle',
        });
        return 'default';
    }
}

/**
 * The explicitly configured auth.idp_org_id, or '' when unset.
 *
 * Kept separate from getIdpOrgId() because the two questions differ: "what should a
 * brand-new organization row be seeded with" (getIdpOrgId, which falls back to the
 * handle) versus "did the operator actually ask for a specific value" (this one). The
 * startup reconcile in seederService.js needs the latter — with only the falling-back
 * form it could not tell an unset setting from one deliberately set to the handle, and
 * would silently rewrite a stored idp_ref_id back to the handle whenever the setting
 * was absent.
 *
 * @returns {string}
 */
function getConfiguredIdpOrgId() {
    const configured = config.auth?.idpOrgId;
    return (typeof configured === 'string' && configured.trim()) || '';
}

/**
 * The IdP's organization identifier for this instance's organization — the value of
 * the org claim the IdP asserts at SSO login, which is what incoming tokens are
 * matched against (see organizationDao.findOrgByIdentifier). Falls back to the handle
 * when unset, so a deployment whose IdP claim equals the handle needs no extra config.
 *
 * Read from [api_portal.auth] rather than [api_portal.organization]: it describes the
 * identity provider's naming of this organization, and pairs with
 * auth.claim_mappings.organization — that names the claim, this is the value expected
 * in it. It is persisted as the organization row's idp_ref_id column, which is the
 * name the REST API and database schema use for the same thing.
 *
 * NOT lowercased, unlike the handle: the stored value is compared verbatim
 * (case-sensitive) against the token claim, so config must be preserved exactly.
 *
 * Consulted at startup only: the seeder writes it when creating the organization and
 * reconciles it on later boots (seederService.js). The admin API never changes it, so
 * config stays the single writer of this field.
 *
 * @returns {string}
 */
function getIdpOrgId() {
    return getConfiguredIdpOrgId() || getHandle();
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
 * The API portal identifier this instance is pinned to.
 *
 * config.organization.portalId is populated by the config.toml template:
 *   portal_id = '{{ env "APIP_AP_ORGANIZATION_PORTAL_ID" "default_portal_id" }}'
 *
 * so env var resolution and the sentinel fallback are already handled before this
 * function runs — mirroring how getHandle() reads config.organization.handle without
 * separately checking process.env.APIP_AP_ORGANIZATION_HANDLE.
 *
 * Synchronous: env vars and config are stable after startup, so no await is needed
 * and every DAO method can call this inline. Never accept a portalId from request
 * input — that is the same IDOR class as accepting org_id from the request.
 *
 * @returns {string}
 */
function getPortalId() {
    if (cachedPortalId) return cachedPortalId;
    const fromConfig = config.organization?.portalId;
    cachedPortalId = (typeof fromConfig === 'string' && fromConfig.trim()) || 'default_portal_id';
    return cachedPortalId;
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
    getFallbackViewHandle,
    getConfiguredIdpOrgId,
    getIdpOrgId,
    getOrgUuid,
    isPinnedOrg,
    requirePinnedOrg,
    resetCache,
    getPortalId,
};
