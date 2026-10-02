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
 * Which portal, and which organization(s), this portal instance serves.
 *
 * One shared database can hold many organizations, each served by its own portal
 * instance. Every instance is pinned to exactly one portal, identified by
 * `organization.portal_id` in config.toml (resolved from the
 * APIP_AP_ORGANIZATION_PORTAL_ID env var via the {{ env }} template token), and by
 * default to exactly one org within it, named by `organization.handle`. This
 * module is the only place that resolves both, and every org/portal-scoped surface
 * goes through here:
 *
 *   - authMiddleware.js  — verifies a token/header-supplied org resolves to the pin
 *   - orgGuard.js        — verifies the {orgHandle} URL segment matches the pin
 *   - webhook workers    — scope their global claim queries to the pinned org
 *
 * With multi_organization.enabled (IDP mode only, see isMultiOrganizationEnabled) one
 * instance serves every organization under its portal_id instead: the same call sites
 * ask the multi-organization-aware questions below (resolveClaimOrg,
 * requireKnownOrg, requireCallerOrg, resolvePublicContentOrg) and get "any
 * organization that exists" where the default mode answers "only the pinned one". The
 * configured organization stays the default and fallback either way.
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

const crypto = require('crypto');
const constants = require('./constants');
const { getNestedClaim } = require('./jwtDecode');
const { config, ORG_HANDLE_PATTERN, RESERVED_ORG_HANDLES } = require('../config/configLoader');
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
 * @param {string} [orgUuid] the organization whose views to choose from — the one a
 *   page URL resolved to. Defaults to this instance's configured organization.
 * @returns {Promise<string>}
 */
async function getFallbackViewHandle(orgUuid) {
    try {
        return await viewDao.getFallbackHandle(orgUuid || await getOrgUuid());
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
 * True when this instance serves every organization under its portal_id rather than only
 * the configured one: multi_organization.enabled, in IDP mode. Local auth has no
 * per-user organization claim to route by, so it stays single-organization regardless
 * (the config loader warns when the flag is set there).
 *
 * @returns {boolean}
 */
function isMultiOrganizationEnabled() {
    return config.multiOrganization?.enabled === true && config.auth?.mode === 'idp';
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
 *   portal_id = '{{ env "APIP_AP_ORGANIZATION_PORTAL_ID" "portal_id" }}'
 *
 * NOTE: Every INSERT into a portal-scoped table MUST supply portal_id from
 * getPortalId() explicitly. If a row is ever written without it, it silently
 * falls back to its DEFAULT value 'portal_id' and if the configured
 * organization.portal_id for this deployment resolves to anything else,
 * that row becomes unreachable to every portal-scoped query.
 *
 * @returns {string}
 */
function getPortalId() {
    if (cachedPortalId) return cachedPortalId;
    const fromConfig = config.organization?.portalId;
    cachedPortalId = typeof fromConfig === 'string' ? fromConfig.trim() : '';
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

function forbiddenOrg() {
    return new CustomError(403, 'Forbidden', 'This operation is not available for the requested organization.');
}

/**
 * Resolves the organization named by a verified credential's org claim (a token, or a
 * session populated from one) and returns its uuid.
 *
 * Default mode: the claim must name this instance's organization — resolved through
 * orgDao's handle/display_name/idp_ref_id ladder and compared by uuid, exactly as
 * before multi-organization mode existed.
 *
 * Multi-organization mode: the claim is matched against idp_ref_id exactly — the one
 * field the IDP's organization identifier is stored in — and any organization it names
 * is accepted. No handle/display_name fallback: a claim value that happened to equal
 * some other organization's display name must not resolve to it. idp_ref_id has no
 * unique constraint, so more than one match is treated as ambiguous and refused.
 *
 * An unknown organization and a refused one both surface as the same 403, so the
 * difference can't be used to probe which organizations exist.
 *
 * With `provision` set (a credential channel, 'login' or 'bearer') and provisioning
 * enabled (isOrgProvisioningEnabled), a claim naming no organization creates it
 * instead — provided the channel passes provisioningChannelProblem.
 *
 * @param {string} claim the org claim value
 * @param {string} source where the claim came from, for the log
 * @param {{ provision?: 'login'|'bearer'|false, orgNames?: { name?: string, handle?: string } }} [options]
 *   orgNames: the organization-name and -handle claims (orgNameClaims), used to name a
 *   provisioned organization
 * @returns {Promise<string>} the organization's uuid
 * @throws {CustomError} 403 when the claim is not accepted
 */
async function resolveClaimOrg(claim, source, { provision = false, orgNames } = {}) {
    if (!isMultiOrganizationEnabled()) {
        return requirePinnedOrg(claim);
    }
    const matches = await orgDao.listByIdpRefId(claim);
    if (matches.length === 1) return matches[0].uuid;
    if (matches.length > 1) {
        logger.error('Org claim matches more than one organization\'s idp_ref_id — refusing it', {
            source, count: matches.length, handles: matches.map((o) => o.handle), operation: 'resolveClaimOrg',
        });
        throw forbiddenOrg();
    }
    if (provision && isOrgProvisioningEnabled()) {
        const untrusted = provisioningChannelProblem(provision);
        if (!untrusted) return provisionOrg(claim, orgNames, source);
        logger.warn('Refused to provision an organization from an org claim: ' + untrusted, {
            source, operation: 'resolveClaimOrg',
        });
        throw forbiddenOrg();
    }
    logger.warn('Rejected org claim naming an unknown organization', { source, operation: 'resolveClaimOrg' });
    throw forbiddenOrg();
}

/**
 * Multi-organization mode: reduces a raw org claim to the single organization
 * identifier it names. IDPs differ in shape — WSO2 IS/Asgardeo and Auth0 send a
 * string, Keycloak's Organizations feature sends a list of aliases or a map keyed by
 * alias — so a list or map naming exactly one organization is that organization. One
 * naming several is refused rather than resolved to an arbitrary member: this portal
 * scopes a session to one organization, and picking the first entry could silently put
 * a user in the wrong one.
 *
 * @param {*} raw the claim value from the token
 * @returns {string} the identifier, or '' when the claim is absent/empty
 * @throws {CustomError} 403 when it names more than one organization
 */
function normalizeOrgClaim(raw) {
    if (typeof raw === 'string') return raw;
    let entries = [];
    if (Array.isArray(raw)) entries = raw;
    else if (raw && typeof raw === 'object') entries = Object.keys(raw);
    entries = entries.filter((e) => typeof e === 'string' && e !== '');
    if (entries.length > 1) {
        logger.warn('Rejected org claim naming more than one organization', {
            count: entries.length, operation: 'normalizeOrgClaim',
        });
        throw forbiddenOrg();
    }
    return entries[0] || '';
}

/**
 * True when an org claim naming an organization that doesn't exist yet provisions it:
 * always in multi-organization mode, never in the default mode.
 *
 * @returns {boolean}
 */
function isOrgProvisioningEnabled() {
    return isMultiOrganizationEnabled();
}

/**
 * The value the configured organization's claim carries — its stored idp_ref_id — so a
 * credential that fell back to it (no org claim) can be
 * recorded as belonging to it. Read from the row rather than config: with
 * auth.idp_org_id unset the row keeps whatever it was seeded with, which is what every
 * other check (ensureAuthenticated.belongsToTargetOrg) compares against.
 *
 * @returns {Promise<string>}
 */
async function getConfiguredOrgIdpRefId() {
    return (await orgDao.getByUuid(await getOrgUuid())).idp_ref_id;
}

const LOOPBACK_HOSTS = new Set(['localhost', '127.0.0.1', '[::1]']);

/** Why `url` can't vouch for what it returns, or null when it can. */
function insecureUrlProblem(url, setting) {
    let parsed;
    try {
        parsed = new URL(url);
    } catch {
        return `${setting} is not a valid URL`;
    }
    if (parsed.protocol === 'https:') return null;
    // Plain http never leaves the host on loopback, so nothing off-host can tamper with
    // it — which is what makes a local IDP usable in development.
    if (parsed.protocol === 'http:' && LOOPBACK_HOSTS.has(parsed.hostname)) return null;
    return `${setting} is not https`;
}

/**
 * Why org claims arriving through `channel` can't be trusted to create organizations,
 * or null when they can. Provisioning turns a claim into new data, so it is only done
 * when the claim's integrity rests on a verified TLS connection to the IDP:
 *
 *   'login'  — the ID token arrives straight from the token endpoint over the code
 *              exchange: auth.idp.token_url must be https. (passportConfig also
 *              verifies it against auth.idp.jwks_url, but it is the token endpoint
 *              connection that guarantees the token was issued for this login.)
 *   'bearer' — the token is signature-checked against a pinned certificate
 *              (auth.idp.certificate), or against keys fetched from auth.idp.jwks_url,
 *              which must then be https.
 *
 * Either way, TLS certificate verification must not be switched off process-wide
 * (NODE_TLS_REJECT_UNAUTHORIZED=0), or "https" verifies nothing.
 *
 * @param {'login'|'bearer'} channel
 * @returns {string|null}
 */
function provisioningChannelProblem(channel) {
    if (process.env.NODE_TLS_REJECT_UNAUTHORIZED === '0') {
        return 'NODE_TLS_REJECT_UNAUTHORIZED=0 disables TLS certificate verification';
    }
    const idp = config.auth?.idp || {};
    if (channel === 'login') return insecureUrlProblem(idp.tokenUrl, 'auth.idp.token_url');
    if (channel === 'bearer') {
        if (idp.certificate) return null;
        return insecureUrlProblem(idp.jwksUrl, 'auth.idp.jwks_url');
    }
    return 'unknown credential channel';
}

// Top-level portal paths that RESERVED_ORG_HANDLES (which also validates a configured
// organization.handle, so can't grow without refusing an existing configuration) doesn't
// cover: an organization provisioned as "login" would have its front page shadowed by
// the portal's own /login route (routes/pages/authRoute.js).
const PROVISIONING_RESERVED_HANDLES = new Set(['login']);

// A handle derived from a claim is capped well below the column width, leaving room for
// the collision suffix and keeping URLs readable.
const MAX_DERIVED_HANDLE_LENGTH = 48;

// A handle taken as-is from the organization-handle claim may be longer — it is the
// IDP's own URL name for the organization — but still leaves the column room for the
// collision suffix.
const MAX_CLAIMED_HANDLE_LENGTH = 128;

/**
 * The organization-name and -handle claims of a verified token
 * (auth.claim_mappings.org_name / org_handle), each only when mapped
 * and a non-empty string. They only ever name an organization being provisioned; which
 * organization a token belongs to is decided by the organization claim alone.
 *
 * @param {object} claims decoded token payload
 * @returns {{ name?: string, handle?: string }}
 */
function orgNameClaims(claims) {
    const read = (key) => {
        const value = key ? getNestedClaim(claims || {}, key) : undefined;
        return typeof value === 'string' && value.trim() ? value.trim() : undefined;
    };
    const mappings = config.auth?.claimMappings || {};
    return { name: read(mappings.orgName), handle: read(mappings.orgHandle) };
}

/** Whether `handle` may be given to a provisioned organization. */
function isProvisionableHandle(handle) {
    return ORG_HANDLE_PATTERN.test(handle) && !RESERVED_ORG_HANDLES.has(handle)
        && !PROVISIONING_RESERVED_HANDLES.has(handle);
}

/**
 * The organization-handle claim as a provisioned organization's handle: lowercased
 * (handles are matched case-insensitively and stored lowercase), otherwise verbatim.
 * '' when it is absent or not usable as one — outside the handle alphabet, too long, or
 * reserved — and the caller derives the handle instead. Never altered to fit: a
 * "fixed-up" IDP handle would look like the IDP's own while naming a different URL.
 *
 * @param {string|undefined} handle
 * @param {string} source where the claim came from, for the log
 * @returns {string}
 */
function claimedHandle(handle, source) {
    if (!handle) return '';
    const candidate = handle.toLowerCase();
    if (candidate.length <= MAX_CLAIMED_HANDLE_LENGTH && isProvisionableHandle(candidate)) return candidate;
    logger.warn('Organization handle claim is not a usable portal handle — deriving one instead', {
        handle, source, operation: 'provisionOrg',
    });
    return '';
}

/**
 * Derives a URL handle from an organization name or id claim: lowercase, accents
 * stripped, every run of characters outside the handle alphabet collapsed to one '-',
 * and trimmed so it starts and ends with a letter or digit. '' when nothing usable is
 * left (the caller then falls back to 'org').
 *
 * @param {string} value
 * @returns {string}
 */
function deriveHandle(value) {
    return String(value || '')
        .normalize('NFKD').replace(/[̀-ͯ]/g, '')
        .toLowerCase()
        .replace(/[^a-z0-9._-]+/g, '-')
        .slice(0, MAX_DERIVED_HANDLE_LENGTH)
        .replace(/^[^a-z0-9]+|[^a-z0-9]+$/g, '');
}

/** Short, stable suffix derived from the claim — see provisionOrg. */
function claimSuffix(claim) {
    return crypto.createHash('sha256').update(String(claim)).digest('hex').slice(0, 6);
}

/**
 * Creates the organization an org claim names, with its defaults (seederService.seedOrg),
 * and returns its uuid. Its idp_ref_id is the claim, verbatim — what every later
 * resolveClaimOrg and belongsToTargetOrg matches against. Its handle is the
 * organization-handle claim when that is a usable handle (claimedHandle), otherwise
 * derived from the organization-name claim, or failing that from the claim itself. Its
 * display name is the organization-name claim, else the handle claim, else the claim.
 *
 * Both handle and display name are unique columns, and another organization may already
 * hold the natural choice, so there are two candidates: that handle, then the same
 * handle plus a suffix taken from a hash of the claim. The suffix is
 * deterministic on purpose — two logins provisioning the same new organization at the
 * same moment compute the same candidates, so the loser of the insert race finds the
 * winner's row (seedOrg re-reads on a duplicate key) instead of creating a second one.
 * A row found under a candidate handle is only accepted if its idp_ref_id is this very
 * claim; any other organization there just moves on to the next candidate.
 *
 * @param {string} claim the org claim value
 * @param {{ name?: string, handle?: string }|undefined} orgNames the organization-name
 *   and -handle claims (orgNameClaims), if any
 * @param {string} source where the claim came from, for the log
 * @returns {Promise<string>} the organization's uuid
 * @throws {Error} when no candidate could be used
 */
async function provisionOrg(claim, orgNames, source) {
    // Required lazily: seederService requires this module.
    const { seedOrg } = require('../services/seederService');
    const { name, handle } = orgNames || {};
    const label = name || handle || String(claim);
    const base = claimedHandle(handle, source) || deriveHandle(name || claim) || deriveHandle(claim) || 'org';
    const suffix = claimSuffix(claim);
    const candidates = [base, `${base}-${suffix}`].filter(isProvisionableHandle);
    if (!candidates.includes(`${base}-${suffix}`)) candidates.push(`org-${suffix}`);

    for (const handle of candidates) {
        let existing = null;
        try {
            existing = await orgDao.getByHandle(handle);
        } catch (err) {
            if (!(err instanceof NotFoundError)) throw err;
        }
        if (existing) {
            if (existing.idp_ref_id === claim) return existing.uuid;
            continue;
        }
        // A different organization may already use this display name; disambiguate the
        // same deterministic way rather than failing the provision on the unique index.
        const nameTaken = !!(await orgDao.findByDisplayName(label));
        const displayName = nameTaken ? `${label} (${suffix})` : label;
        let org;
        try {
            ({ org } = await seedOrg({ handle, displayName, idpRefId: claim }, 'provisionOrg'));
        } catch (err) {
            if (err instanceof NotFoundError) continue; // lost a race on another unique column
            throw err;
        }
        if (org.idp_ref_id === claim) {
            logger.info('Org: provisioned organization from an org claim', {
                handle: org.handle, source, operation: 'provisionOrg',
            });
            return org.uuid;
        }
    }
    logger.error('Could not provision an organization for an org claim — every candidate handle is taken', {
        source, candidates, operation: 'provisionOrg',
    });
    throw new Error('Organization could not be provisioned');
}

/**
 * Multi-organization mode: resolves the {orgHandle} segment of a page URL to the
 * organization with exactly that handle — never a display name or idp_ref_id, since a
 * page has one canonical URL. A URL never creates an organization. (The default mode
 * never gets here: orgGuard compares against getHandle() without a lookup.)
 *
 * @param {string} value the URL's org segment
 * @returns {Promise<object>} the organization row
 * @throws {NotFoundError} when no organization has exactly that handle
 */
async function requireKnownOrg(value) {
    return orgDao.getByHandle(String(value || ''));
}

/**
 * For REST operations addressing an organization by id ({orgId}): resolves it and
 * returns its uuid, throwing 403 unless the caller may act on it.
 *
 * Default mode: it must be this instance's organization (requirePinnedOrg).
 * Multi-organization mode: it must be the caller's *own* organization — the one
 * authResolver resolved from the caller's verified org claim (req.orgId). "Any known
 * organization" would let an administrator of one tenant read and rewrite another's
 * settings just by changing the path parameter.
 *
 * @param {string} identifier the {orgId} path parameter
 * @param {string} callerOrgUuid req.orgId
 * @returns {Promise<string>} the resolved uuid
 * @throws {CustomError} 403 otherwise
 */
async function requireCallerOrg(identifier, callerOrgUuid) {
    if (!isMultiOrganizationEnabled()) return requirePinnedOrg(identifier);
    let resolvedUuid = null;
    try {
        resolvedUuid = await orgDao.getId(identifier);
    } catch (err) {
        if (!(err instanceof NotFoundError)) throw err;
    }
    if (resolvedUuid && callerOrgUuid && resolvedUuid === callerOrgUuid) return resolvedUuid;
    logger.warn('Rejected operation on an organization other than the caller\'s own', {
        requested: identifier,
        operation: 'requireCallerOrg',
    });
    throw forbiddenOrg();
}

/**
 * The organization whose PUBLIC content (theme stylesheets, API icons) an anonymous-
 * capable asset endpoint should read from, or null.
 *
 * Default mode: the caller's session organization, else this instance's own — the
 * `?orgId` query parameter is ignored, since it would otherwise be an unauthenticated
 * selector for any tenant's content in a shared database.
 *
 * Multi-organization mode: every organization's public pages are open to anonymous
 * visitors, so their branding is too. `?orgId` (an organization uuid, as the portal's
 * own templates emit it) is honoured when it names an existing organization — the page
 * being rendered, which may not be the session's own — and otherwise falls back as
 * above.
 *
 * @param {string|undefined} queryOrgId the request's ?orgId
 * @param {string|undefined} sessionOrgUuid req.orgId
 * @returns {Promise<string|null>}
 */
async function resolvePublicContentOrg(queryOrgId, sessionOrgUuid) {
    if (isMultiOrganizationEnabled() && typeof queryOrgId === 'string' && queryOrgId) {
        try {
            return (await orgDao.getByUuid(queryOrgId)).uuid;
        } catch (err) {
            if (!(err instanceof NotFoundError)) throw err;
        }
    }
    if (sessionOrgUuid) return sessionOrgUuid;
    return getOrgUuid().catch(() => null);
}

/**
 * The idp_ref_ids more than one organization carries, each with those organizations'
 * handles. resolveClaimOrg refuses such a claim as ambiguous, so every sign-in to any of
 * them fails — typically after auth.idp_org_id was pointed at an identifier an
 * already-provisioned organization holds (the startup reconcile writes it regardless).
 * Compared exactly, as resolveClaimOrg does; organizations without one are ignored.
 *
 * @param {Array<{ handle: string, idp_ref_id?: string }>} orgs
 * @returns {Array<{ idpRefId: string, handles: string[] }>}
 */
function findSharedIdpRefIds(orgs) {
    const byRef = new Map();
    for (const org of orgs || []) {
        if (!org?.idp_ref_id) continue;
        if (!byRef.has(org.idp_ref_id)) byRef.set(org.idp_ref_id, []);
        byRef.get(org.idp_ref_id).push(org.handle);
    }
    return [...byRef].filter(([, handles]) => handles.length > 1)
        .map(([idpRefId, handles]) => ({ idpRefId, handles }));
}

/**
 * Whether a signed-in user's recorded org claim names `org` (an organization row) —
 * the rule ensureAuthenticated.belongsToTargetOrg applies to protected pages, shared
 * here so what the page chrome shows can never disagree with what the server allows.
 * A claim matches the organization's idp_ref_id exactly, or appears in the user's
 * authorizedOrgs.
 *
 * @param {object} user req.user
 * @param {object} org an organization row (idp_ref_id)
 * @returns {boolean}
 */
function claimBelongsToOrg(user, org) {
    const claim = user?.[constants.ROLES.ORGANIZATION_CLAIM];
    const identifier = org?.idp_ref_id;
    const authorizedOrgs = user?.authorizedOrgs;
    return !!claim && !!identifier && (
        claim === identifier || (Array.isArray(authorizedOrgs) && authorizedOrgs.includes(identifier))
    );
}

/**
 * For page chrome: true when a signed-in user is browsing an organization other than
 * their own, where they are no administrator (the sidebar's Settings link is hidden,
 * as for an anonymous visitor; the signed-in pages answer 403). orgGuard records the
 * answer on the request as req.foreignOrgSession for multi-organization page routes;
 * it is never set in the default single-organization mode (there is only one
 * organization to be in) or for anonymous visitors.
 *
 * @param {object} user req.user
 * @param {object} org the page's organization row
 * @returns {boolean}
 */
function isForeignOrgSession(user, org) {
    return !!user && !claimBelongsToOrg(user, org);
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
    isMultiOrganizationEnabled,
    normalizeOrgClaim,
    orgNameClaims,
    claimBelongsToOrg,
    findSharedIdpRefIds,
    isForeignOrgSession,
    isOrgProvisioningEnabled,
    getConfiguredOrgIdpRefId,
    deriveHandle,
    resolveClaimOrg,
    requireKnownOrg,
    requireCallerOrg,
    resolvePublicContentOrg,
    resetCache,
    getPortalId,
};
