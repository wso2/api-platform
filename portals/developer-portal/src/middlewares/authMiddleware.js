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
 * Auth pipeline for the spec-driven /devportal router.
 *
 *   authResolver  →  OpenAPI validator (calls OAuth2Security / apiKeyAuth)  →  handler
 *
 * `authResolver` runs once per /devportal request and resolves credentials in the
 * order: local session → bearer → api-key → mTLS. It populates `req.auth` with
 * `{ mode, scopes, preauthorized, userId, rawSub }` but does NOT enforce scopes —
 * that is the job of `OAuth2Security`, which the validator invokes with the
 * operation-declared scope list. `userId` is the durable user_idp_references uuid
 * (what flows into created_by/updated_by); `rawSub` is the original, unresolved
 * IDP `sub` claim, kept around for telemetry/analytics that need the real identity
 * rather than the internal bookkeeping uuid.
 *
 */

const { safeDecodeJwt } = require('../utils/jwtDecode');
const { jwtVerify, createRemoteJWKSet } = require('jose');

const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const { verifyPlatformJwtClaims, decodePlatformJwtClaims } = require('../utils/platformJwt');
const { accessTokenPresent, refreshAccessToken, verifyWithCertificate, resolveOrgIdp } = require('../utils/tokenUtil');
const orgDao = require('../dao/organizationDao');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
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
        if (config.idp?.issuer) jwtVerifyOptions.issuer = config.idp.issuer;
        if (config.idp?.audience) jwtVerifyOptions.audience = config.idp.audience;
        const { payload } = await jwtVerify(token, jwks, jwtVerifyOptions);
        const rawScope = payload.scope ?? payload.scp;
        const scopes = Array.isArray(rawScope) ? rawScope.join(' ') : (rawScope || '');
        return { valid: true, scopes };
    } catch (err) {
        if (err.code === 'ERR_JWT_EXPIRED' && req.user && req.user.refreshToken) {
            try {
                logger.info('Access token expired during /devportal request, refreshing');
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
    if (!idp || !idp.clientId) {
        // Local auth mode: verify the Platform API JWT with the shared secret.
        const jwtSecret = config.platformApi?.jwtSecret;
        if (jwtSecret) {
            const claims = await verifyPlatformJwtClaims(token, jwtSecret);
            if (!claims) return { valid: false, scopes: '' };
            return { valid: true, scopes: claims.scopes?.join(' ') ?? '' };
        }
        // Platform API now signs its admin tokens with RS256 (asymmetric), so there
        // is no shared HMAC secret to verify against. When platformApi.insecure is
        // explicitly enabled, decode the payload without verifying its signature,
        // trusting the direct HTTPS connection to Platform API instead (mirrors the
        // session-based local-auth branch above, which already does this via
        // decodePlatformJwtClaims). Fail closed otherwise — never accept an
        // unverified token by default.
        if (!config.platformApi?.insecure) return { valid: false, scopes: '' };
        const claims = decodePlatformJwtClaims(token);
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
 * Resolves the org UUID from an IDP claim value (IDP_REF_ID) and sets req.orgId.
 * Returns null on success, or an Error (with .status) on failure.
 */
async function resolveOrgFromClaim(req, orgClaim) {
    if (!orgClaim) return null;
    try {
        req.orgId = await orgDao.getId(orgClaim);
        return null;
    } catch (e) {
        if (e.name === 'SequelizeEmptyResultError') {
            const err = new Error('Organization not found');
            err.status = 404;
            return err;
        }
        logger.error('Org lookup failed', { error: e.message, orgClaim });
        const err = new Error('Internal Server Error');
        err.status = 500;
        return err;
    }
}

/**
 * Resolves the org UUID from the `organization` request header and sets req.orgId.
 * Used for API-key, mTLS, and local-auth requests that carry no token org claim.
 * Returns null when the header is absent (allows public endpoints through).
 */
async function resolveOrgFromHeader(req) {
    const orgHeader = req.headers.organization;
    if (!orgHeader) return null;
    try {
        req.orgId = await orgDao.getId(orgHeader);
        return null;
    } catch (e) {
        if (e.name === 'SequelizeEmptyResultError') {
            const err = new Error('Organization not found');
            err.status = 404;
            return err;
        }
        logger.error('Org lookup failed from header', { error: e.message, orgHeader });
        const err = new Error('Internal Server Error');
        err.status = 500;
        return err;
    }
}

/**
 * Pre-validator middleware that establishes `req.auth`. Runs once per
 * /devportal request before the OpenAPI validator security check.
 */
async function authResolver(req, res, next) {
    try {
        // 1. Local auth users (platform JWT in session, no IdP configured).
        // The session stores the org handle in the same ORGANIZATION_CLAIM slot used by IDP
        // sessions, so resolveOrgFromClaim works via the HANDLE lookup in orgDao.getId.
        if (req.isAuthenticated && req.isAuthenticated() &&
            req.user?.isLocalAuth && !config.idp?.clientId) {
            const platformToken = req.user[constants.ACCESS_TOKEN];
            const claims = platformToken ? decodePlatformJwtClaims(platformToken) : null;
            const orgHandle = req.user[constants.ROLES.ORGANIZATION_CLAIM];
            const orgErr = await resolveOrgFromClaim(req, orgHandle);
            if (orgErr) return next(orgErr);
            const userUuid = await resolveUserUuid(req, req.user[constants.USER_ID]);
            req.auth = {
                mode: 'platform-jwt',
                preauthorized: false,
                scopes: claims?.scopes ?? [],
                userId: userUuid,
                rawSub: req.user[constants.USER_ID],
            };
            return next();
        }

        // 2. Session fast-path: browser login via IDP — role check is done by ensureAuthenticated
        // on page routes, so scope enforcement here is redundant and would require listing all
        // dp:* scopes in the OIDC scope config. Set preauthorized to bypass the per-operation
        // scope check for session users (same as API key and mTLS paths).
        if (req.isAuthenticated && req.isAuthenticated() && req.user?.grantedScopes !== undefined && config.idp?.clientId) {
            const orgIDClaim = config.idp?.claims?.orgId;
            if (orgIDClaim) {
                const sessionOrgClaim = req.user[constants.ROLES.ORGANIZATION_CLAIM];
                if (!sessionOrgClaim) {
                    const err = new Error('Missing organization claim in session');
                    err.status = 403;
                    return next(err);
                }
                const orgErr = await resolveOrgFromClaim(req, sessionOrgClaim);
                if (orgErr) return next(orgErr);
            }
            const rawSub = req.user[constants.USER_ID];
            const userUuid = await resolveUserUuid(req, rawSub);
            req[constants.USER_ID] = userUuid;
            req.auth = {
                mode: 'oauth2',
                preauthorized: true,
                scopes: String(req.user.grantedScopes || '').split(' ').filter(Boolean),
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
            // Resolve org UUID from the token's org claim (IDP_REF_ID).
            // Only in IDP mode — local-auth and platform-JWT tokens carry no org claim.
            const orgIDClaim = config.idp?.claims?.orgId;
            if (config.idp?.clientId && orgIDClaim) {
                const tokenOrgClaim = decoded[orgIDClaim];
                if (!tokenOrgClaim) {
                    const err = new Error('Missing organization claim in token');
                    err.status = 403;
                    return next(err);
                }
                const orgErr = await resolveOrgFromClaim(req, tokenOrgClaim);
                if (orgErr) return next(orgErr);
            } else if (decoded.org_handle) {
                const orgErr = await resolveOrgFromClaim(req, decoded.org_handle);
                if (orgErr) return next(orgErr);
            }
            const rawSub = decoded[constants.USER_ID];
            const userUuid = await resolveUserUuid(req, rawSub);
            req[constants.USER_ID] = userUuid;
            req.auth = {
                mode: 'oauth2',
                scopes: String(scopes || '').split(' ').filter(Boolean),
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
                    const orgErr = await resolveOrgFromHeader(req);
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
                    const orgErr = await resolveOrgFromHeader(req);
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
};
