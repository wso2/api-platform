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
 *
 */

/*
 * Auth pipeline for the spec-driven API Portal REST router (/api/v0.9).
 *
 *   authResolver  →  OpenAPI validator (calls OAuth2Security / apiKeyAuth)  →  handler
 *
 * `authResolver` runs once per API Portal REST request and resolves credentials in the
 * order: local session → bearer → api-key → mTLS. It populates `req.auth` with
 * `{ mode, scopes, preauthorized, userId, rawSub }` but does NOT enforce scopes —
 * that is the job of `OAuth2Security`, which the validator invokes with the
 * operation-declared scope list. `userId` is the durable user_idp_references uuid
 * (what flows into created_by/updated_by); `rawSub` is the original, unresolved
 * IDP `sub` claim, kept around for telemetry/analytics that need the real identity
 * rather than the internal bookkeeping uuid.
 *
 */

const { safeDecodeJwt, getNestedClaim } = require('../utils/jwtDecode');
const { jwtVerify, createRemoteJWKSet } = require('jose');

const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const { verifyPlatformJwtClaims, decodePlatformJwtClaims } = require('../utils/platformJwt');
const { accessTokenPresent, refreshAccessToken, verifyWithCertificate, resolveOrgIdp } = require('../utils/tokenUtil');
const orgDao = require('../dao/organizationDao');
const orgContext = require('../utils/orgContext');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const { effectiveScopes, isAuthorizationEnabled, isRoleMode } = require('./authorization');
const { NotFoundError } = require('../utils/errors/customErrors');
const userOrganizationMappingDao = require('../dao/userOrganizationMappingDao');

// In-process cache so an already-known (sub, org) pair doesn't re-hit the DB on
// every request from the same session — resolveUserUuid runs on every
// authenticated request, including plain GETs. Bounded and TTL'd: entries expire
// after USER_UUID_CACHE_TTL_MS, and the cache is cleared outright once it grows
// past USER_UUID_CACHE_MAX_ENTRIES rather than tracking per-entry recency.
const USER_UUID_CACHE_TTL_MS = 5 * 60 * 1000;
const USER_UUID_CACHE_MAX_ENTRIES = 5000;
const userUuidCache = new Map(); // sub -> { uuid, expiresAt }
const orgMappingCache = new Map(); // `${userUuid}:${orgId}` -> expiresAt
const pendingUserUuidLookups = new Map(); // sub -> in-flight resolveUuid() promise

function getCached(cache, key) {
    const entry = cache.get(key);
    if (entry === undefined) return undefined;
    if (entry.expiresAt <= Date.now()) {
        cache.delete(key);
        return undefined;
    }
    return entry;
}

function setCached(cache, key, entry, maxEntries) {
    if (cache.size >= maxEntries) cache.clear();
    cache.set(key, { ...entry, expiresAt: Date.now() + USER_UUID_CACHE_TTL_MS });
}

/**
 * Resolves the durable user_idp_references uuid for this sub claim, and
 * records that the user has been seen in the current org (if known). This
 * uuid — not the raw sub — is what flows into created_by/updated_by columns.
 *
 * Identity bookkeeping (this function) is not security-critical, so a failure
 * here degrades gracefully — logged and swallowed — rather than failing
 * authentication for an otherwise valid token/session. A resource write made
 * with the resulting undefined userId will fail at that write with a clear
 * validation error instead of taking down login.
 */
async function resolveUserUuid(req, sub) {
    if (!sub) return undefined;

    let userUuid = getCached(userUuidCache, sub)?.uuid;
    if (userUuid === undefined) {
        let pending = pendingUserUuidLookups.get(sub);
        if (!pending) {
            pending = userIdpReferenceDao.resolveUuid(sub).finally(() => {
                pendingUserUuidLookups.delete(sub);
            });
            pendingUserUuidLookups.set(sub, pending);
        }
        try {
            userUuid = await pending;
        } catch (err) {
            logger.error('Failed to resolve user identity reference; continuing without one', {
                error: err.message, operation: 'resolveUserUuid',
            });
            return undefined;
        }
        setCached(userUuidCache, sub, { uuid: userUuid }, USER_UUID_CACHE_MAX_ENTRIES);
    }

    if (req.orgId) {
        const mappingKey = `${userUuid}:${req.orgId}`;
        if (!getCached(orgMappingCache, mappingKey)) {
            try {
                await userOrganizationMappingDao.ensureMapping(userUuid, req.orgId);
                setCached(orgMappingCache, mappingKey, {}, USER_UUID_CACHE_MAX_ENTRIES);
            } catch (err) {
                logger.error('Failed to record user-organization mapping; continuing', {
                    error: err.message, operation: 'resolveUserUuid',
                });
            }
        }
    }

    return userUuid;
}

async function verifyJwksWithRefresh(token, jwksURL, req) {
    try {
        const jwks = await createRemoteJWKSet(new URL(jwksURL));
        const jwtVerifyOptions = { algorithms: constants.JWT_ASYMMETRIC_ALGORITHMS };
        if (config.auth.idp?.issuer) jwtVerifyOptions.issuer = config.auth.idp.issuer;
        if (config.auth.idp?.audience) jwtVerifyOptions.audience = config.auth.idp.audience;
        const { payload } = await jwtVerify(token, jwks, jwtVerifyOptions);
        const rawScope = payload.scope ?? payload.scp;
        const scopes = Array.isArray(rawScope) ? rawScope.join(' ') : (rawScope || '');
        return { valid: true, scopes };
    } catch (err) {
        if (err.code === 'ERR_JWT_EXPIRED' && req.user && req.user.refreshToken) {
            try {
                logger.info('Access token expired during API Portal REST request, refreshing');
                const refreshed = await refreshAccessToken(req.user.refreshToken);
                req.user[constants.ACCESS_TOKEN] = refreshed.access_token;
                req.user[constants.REFRESH_TOKEN] = refreshed.refresh_token;
                return { valid: true, scopes: refreshed.scope || '', refreshed };
            } catch (refreshErr) {
                logger.error('Refresh token flow failed', {
                    error: refreshErr.message,
                    stack: refreshErr.stack,
                    operation: 'refreshAccessToken',
                });
                return { valid: false, scopes: '' };
            }
        }
        logger.error('Bearer token validation failed', {
            error: err.message,
            operation: 'verifyJwksWithRefresh',
        });
        return { valid: false, scopes: '' };
    }
}

async function verifyBearerToken(token, req) {
    const idp = resolveOrgIdp();
    if (config.auth.mode !== 'idp') {
        // Local auth mode: verify the Platform API JWT against its RS256 public key.
        // Fail closed if no key path is configured — never accept an unverified token.
        const publicKeyPath = config.auth.local?.publicKeyPath;
        if (!publicKeyPath) return { valid: false, scopes: '' };
        const claims = await verifyPlatformJwtClaims(token, publicKeyPath);
        if (!claims) return { valid: false, scopes: '' };
        return { valid: true, scopes: claims.scopes?.join(' ') ?? '' };
    }
    if (idp.certificate) {
        return verifyWithCertificate(token, idp.certificate);
    }
    if (idp.jwksUrl) {
        return verifyJwksWithRefresh(token, idp.jwksUrl, req);
    }
    return { valid: false, scopes: '' };
}

/**
 * Resolves a caller-supplied organization identifier and sets req.orgId — but only
 * if it names the single organization this instance serves.
 *
 * The database is shared across organizations, so a token minted by a trusted IDP
 * for a *different* organization still passes signature, expiry, and audience
 * checks. Without this comparison, "the IDP is trusted" would collapse into "any
 * token it issues is valid here", and the holder would read and write this
 * organization's data under their own tenant's credentials.
 *
 * The resolved uuid is compared rather than the raw identifier: `identifier` may be
 * a handle, a display name, or an idp_ref_id depending on which claim it came from,
 * and orgDao resolves all three. Comparing after resolution makes every spelling of
 * this organization match and every spelling of any other organization not match.
 *
 * @returns {Promise<Error|null>} null on success, or an Error with .status
 */
async function resolveScopedOrg(req, identifier, source) {
    if (!identifier) return null;
    let resolvedUuid;
    try {
        resolvedUuid = await orgDao.getId(identifier);
    } catch (e) {
        if (e instanceof NotFoundError) {
            // Same response as a known-but-foreign organization below: a caller must
            // not be able to tell "no such organization" from "not this portal's
            // organization", which would turn this into an existence oracle over
            // every tenant in the shared database.
            logger.warn('Rejected request naming an unknown organization', { source });
            const err = new Error('Forbidden');
            err.status = 403;
            return err;
        }
        logger.error('Org lookup failed', { error: e.message, source });
        const err = new Error('Internal Server Error');
        err.status = 500;
        return err;
    }

    let pinned;
    try {
        pinned = await orgContext.isPinnedOrg(resolvedUuid);
    } catch (e) {
        logger.error('Pinned org comparison failed', { error: e.message, source });
        const err = new Error('Internal Server Error');
        err.status = 500;
        return err;
    }
    if (!pinned) {
        logger.warn('Rejected credential scoped to a different organization', {
            source,
            expected: orgContext.getHandle(),
        });
        const err = new Error('Forbidden');
        err.status = 403;
        return err;
    }

    req.orgId = resolvedUuid;
    return null;
}

/**
 * Sets req.orgId for credentials that carry no organization of their own (service
 * API key, mTLS) — they are authenticated as this portal's operator, so the only
 * organization they can be acting on is the one this instance serves.
 *
 * An `organization` header is no longer what selects the organization; it is only
 * checked for disagreement. Honouring it would let a single API key address every
 * tenant in the shared database. Rejecting a mismatch rather than ignoring it keeps
 * a caller from believing it wrote to the organization it named.
 *
 * @returns {Promise<Error|null>} null on success, or an Error with .status
 */
async function resolvePortalOrg(req) {
    const orgHeader = req.headers.organization;
    if (orgHeader) {
        return resolveScopedOrg(req, orgHeader, 'organization header');
    }
    try {
        req.orgId = await orgContext.getOrgUuid();
        return null;
    } catch (e) {
        logger.error('Configured organization could not be resolved', {
            error: e.message,
            handle: orgContext.getHandle(),
        });
        const err = new Error('Internal Server Error');
        err.status = 500;
        return err;
    }
}

/**
 * Pre-validator middleware that establishes `req.auth`. Runs once per
 * API Portal REST request before the OpenAPI validator security check.
 */
async function authResolver(req, res, next) {
    try {
        // 1. Local auth users (platform JWT in session, no IdP configured).
        // The session stores the org handle in the same ORGANIZATION_CLAIM slot used by IDP
        // sessions, so resolveScopedOrg works via the HANDLE lookup in orgDao.getId.
        if (req.isAuthenticated && req.isAuthenticated() &&
            req.user?.isLocalAuth && config.auth.mode !== 'idp') {
            const platformToken = req.user[constants.ACCESS_TOKEN];
            const claims = platformToken ? decodePlatformJwtClaims(platformToken) : null;
            const orgHandle = req.user[constants.ROLES.ORGANIZATION_CLAIM];
            // Fail closed on a session with no org claim, exactly as the IDP branch
            // below does. resolveScopedOrg treats a falsy identifier as "nothing to
            // check" and returns null, which would leave req.orgId undefined and let
            // every tenant-scoped query run unscoped rather than be rejected.
            if (!orgHandle) {
                const err = new Error('Missing organization claim in session');
                err.status = 403;
                return next(err);
            }
            const orgErr = await resolveScopedOrg(req, orgHandle, 'platform-jwt session');
            if (orgErr) return next(orgErr);
            const userUuid = await resolveUserUuid(req, req.user[constants.USER_ID]);
            req.auth = {
                mode: 'platform-jwt',
                preauthorized: false,
                // Platform API tokens carry both a scope claim and (since the
                // authentication/authorization split) a roles claim, so either
                // authorization mode can be applied to the same token.
                scopes: effectiveScopes(claims?.scopes ?? [], claims),
                userId: userUuid,
                rawSub: req.user[constants.USER_ID],
            };
            return next();
        }

        // 2. Session fast-path: browser login via IDP.
        //
        // In "scope" mode the per-operation check is bypassed (preauthorized, same as the
        // API key and mTLS paths): the IDP mints whatever scopes its client is registered
        // for, which would mean listing all dp:* scopes in the OIDC scope config, so the
        // authorization that actually applies to these sessions is ensureAuthenticated's
        // page role check.
        //
        // In "role" mode the grant table makes the session's own roles claim sufficient to
        // derive dp:* scopes, so the operation-level check is enforced here instead of
        // bypassed — that is the gap role mode exists to close.
        if (req.isAuthenticated && req.isAuthenticated() && req.user?.grantedScopes !== undefined && config.auth.mode === 'idp') {
            // The session's org claim is populated at login from
            // config.auth.claimMappings.organization (see passportConfig) and stored
            // under ORGANIZATION_CLAIM. Resolve req.orgId from it directly — do NOT
            // gate on config.auth.idp.claims.orgId, which has no default and is unset
            // in typical IDP configs, which would leave req.orgId empty and break every
            // tenant-scoped operation (reads return the wrong scope; writes fail the
            // org_uuid foreign key). Fail closed when no org claim is present.
            const sessionOrgClaim = req.user[constants.ROLES.ORGANIZATION_CLAIM];
            if (!sessionOrgClaim) {
                const err = new Error('Missing organization claim in session');
                err.status = 403;
                return next(err);
            }
            const orgErr = await resolveScopedOrg(req, sessionOrgClaim, 'idp session');
            if (orgErr) return next(orgErr);
            const rawSub = req.user[constants.USER_ID];
            const userUuid = await resolveUserUuid(req, rawSub);
            req[constants.USER_ID] = userUuid;
            req.auth = {
                mode: 'oauth2',
                preauthorized: !isRoleMode(),
                scopes: effectiveScopes(req.user.grantedScopes, req.user),
                userId: userUuid,
                rawSub,
            };
            return next();
        }

        // 3. Bearer token (session-attached or Authorization header)
        const token = accessTokenPresent(req);
        if (token) {
            const { valid, scopes } = await verifyBearerToken(token, req);
            if (!valid) {
                const err = new Error('Authentication required');
                err.status = 401;
                return next(err);
            }
            const decoded = safeDecodeJwt(req.user?.[constants.ACCESS_TOKEN] || token) || {};
            // Resolve org UUID from the token's org claim. Use the same claim
            // mapping login uses (config.auth.claimMappings.organization) rather
            // than config.auth.idp.claims.orgId, which has no default and is
            // typically unset — gating on it left req.orgId empty and broke every
            // tenant-scoped operation. Only in IDP mode — local-auth and
            // platform-JWT tokens carry no org claim.
            if (config.auth.mode === 'idp') {
                const orgClaimKey = config.auth.claimMappings?.organization;
                const tokenOrgClaim = (orgClaimKey ? getNestedClaim(decoded, orgClaimKey) : undefined) || decoded.org_handle;
                if (!tokenOrgClaim) {
                    const err = new Error('Missing organization claim in token');
                    err.status = 403;
                    return next(err);
                }
                const orgErr = await resolveScopedOrg(req, tokenOrgClaim, 'bearer token claim');
                if (orgErr) return next(orgErr);
            } else if (decoded.org_handle) {
                const orgErr = await resolveScopedOrg(req, decoded.org_handle, 'bearer token org_handle');
                if (orgErr) return next(orgErr);
            }
            const rawSub = decoded[constants.USER_ID];
            const userUuid = await resolveUserUuid(req, rawSub);
            req[constants.USER_ID] = userUuid;
            req.auth = {
                mode: 'oauth2',
                // `decoded` is the same payload verifyBearerToken just verified, so the
                // roles claim role mode expands is a verified one — never a claim read
                // out of an unverified token.
                scopes: effectiveScopes(scopes, decoded),
                userId: userUuid,
                rawSub,
            };
            return next();
        }

        // 4. API key — org resolved from the `organization` request header
        if (config.security?.serviceApiKey?.enabled) {
            const keyType = config.security.serviceApiKey.headerName;
            if (keyType && config.security?.serviceApiKey?.value) {
                const apiKey = req.headers[keyType.toLowerCase()];
                if (apiKey && apiKey === config.security?.serviceApiKey?.value) {
                    const orgErr = await resolvePortalOrg(req);
                    if (orgErr) return next(orgErr);
                    req.auth = { mode: 'apikey', preauthorized: true, scopes: [] };
                    return next();
                }
            }
        }

        // 5. mTLS — org resolved from the `organization` request header
        if (typeof req.socket?.getPeerCertificate === 'function') {
            const cert = req.socket.getPeerCertificate(true);
            if (cert && Object.keys(cert).length > 0 && req.client?.authorized) {
                const now = new Date();
                if (new Date(cert.valid_from) <= now && new Date(cert.valid_to) >= now) {
                    const orgErr = await resolvePortalOrg(req);
                    if (orgErr) return next(orgErr);
                    req.auth = { mode: 'mtls', preauthorized: true, scopes: [] };
                    return next();
                }
            }
        }

        // 6. No usable credential — pass through as anonymous so the OpenAPI
        // validator can enforce security on a per-operation basis. Operations
        // with `security: []` (public endpoints) will proceed; operations that
        // declare a security scheme will have their handler invoked by the
        // validator and throw 401 if req.auth is absent.
        req.auth = null;
        return next();
    } catch (err) {
        logger.error('authResolver failed', {
            error: err.message,
            stack: err.stack,
            operation: 'authResolver',
        });
        return res.status(500).json({ error: 'Internal Server Error' });
    }
}

/**
 * OAuth2 security handler invoked by express-openapi-validator with the
 * scope list declared on the operation. Implements any-of semantics over
 * a single security requirement object, matching the OpenAPI spec.
 */
async function OAuth2Security(req /* , requiredScopes, schema */) {
    const requiredScopes = arguments[1] || [];
    if (!req.auth) {
        const err = new Error('Authentication required');
        err.status = 401;
        throw err;
    }
    if (req.auth.preauthorized) return true;
    if (req.auth.mode !== 'oauth2' && req.auth.mode !== 'platform-jwt') {
        const err = new Error('Authentication required');
        err.status = 401;
        throw err;
    }
    // Authentication is still required above — only the per-operation scope check is
    // waived, and only by an explicit opt-out (auth.authorization.enabled = false,
    // which logs a warning at startup).
    if (!isAuthorizationEnabled()) return true;
    if (!requiredScopes || requiredScopes.length === 0) return true;
    const tokenScopes = req.auth.scopes || [];
    const ok = requiredScopes.some(s => tokenScopes.includes(s));
    if (!ok) {
        const err = new Error('Forbidden');
        err.status = 403;
        throw err;
    }
    return true;
}

/**
 * API key security handler. Accepts the request if authResolver already
 * authenticated it via API key (or any preauthorized non-OAuth mode, to
 * mirror legacy behaviour where API key endpoints also accepted basic/mTLS).
 */
/*
 * TODO: once the API key support introduces with scope support, change the method
 * to check for scopes as well, and rename it to ApiKeySecurity for clarity.
 */
async function apiKeyAuth(req /* , scopes, schema */) {
    if (req.auth?.mode === 'apikey' || req.auth?.preauthorized) return true;
    const err = new Error('Authentication required');
    err.status = 401;
    throw err;
}

module.exports = {
    authResolver,
    OAuth2Security,
    apiKeyAuth,
    resolveUserUuid,
    verifyBearerToken,
};
