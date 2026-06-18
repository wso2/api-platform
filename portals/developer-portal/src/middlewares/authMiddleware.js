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
 * `{ mode, scopes, preauthorized, userId }` but does NOT enforce scopes — that is
 * the job of `OAuth2Security`, which the validator invokes with the operation-declared
 * scope list.
 *
 */

const jwt = require('jsonwebtoken');
const { jwtVerify, createRemoteJWKSet } = require('jose');

const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const { extractPlatformJwtClaims } = require('../utils/platformJwt');
const { accessTokenPresent, refreshAccessToken, verifyWithCertificate, resolveOrgIdp } = require('../utils/tokenUtil');

const DEFAULT_TOKEN_REFRESH_TIMEOUT_MS = 10000;

function resolveTokenRefreshTimeoutMs() {
    const timeout = Number(config.identityProvider?.tokenRefreshTimeoutMs);
    if (Number.isFinite(timeout) && timeout > 0) {
        return timeout;
    }
    return DEFAULT_TOKEN_REFRESH_TIMEOUT_MS;
}


async function verifyJwksWithRefresh(token, jwksURL, req) {
    try {
        const jwks = await createRemoteJWKSet(new URL(jwksURL));
        const { payload } = await jwtVerify(token, jwks);
        return { valid: true, scopes: payload.scope || '' };
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
    const idp = await resolveOrgIdp(req);
    if (!idp || !idp.clientId) {
        // Local auth mode: verify Platform API JWT with shared secret when configured.
        const jwtSecret = config.platformApi?.jwtSecret;
        const claims = extractPlatformJwtClaims(token, jwtSecret || null);
        if (jwtSecret && !claims) return { valid: false, scopes: '' };
        return { valid: true, scopes: claims?.scopes?.join(' ') ?? '' };
    }
    if (idp.certificate) {
        return verifyWithCertificate(token, idp.certificate);
    }
    if (idp.jwksURL) {
        return verifyJwksWithRefresh(token, idp.jwksURL, req);
    }
    return { valid: false, scopes: '' };
}

function checkOrgMembership(req) {
    if (!req.user) return true;
    const tokenOrg = req.user[constants.ROLES.ORGANIZATION_CLAIM];
    const targetOrg = req.user[constants.ORG_IDENTIFIER];
    if (!targetOrg || tokenOrg === targetOrg) return true;
    const authorizedOrgs = req.user.authorizedOrgs;
    return Array.isArray(authorizedOrgs) && authorizedOrgs.includes(targetOrg);
}

/**
 * Pre-validator middleware that establishes `req.auth`. Runs once per
 * /devportal request before the OpenAPI validator security check.
 */
async function authResolver(req, res, next) {
    try {
        // 1. Local auth users (platform JWT in session, no IdP configured)
        if (req.isAuthenticated && req.isAuthenticated() &&
            req.user?.isLocalAuth && !config.identityProvider?.clientId) {
            const platformToken = req.user[constants.ACCESS_TOKEN];
            const claims = platformToken ? extractPlatformJwtClaims(platformToken, null) : null;
            req.auth = {
                mode: 'platform-jwt',
                preauthorized: false,
                scopes: claims?.scopes ?? [],
                userId: req.user[constants.USER_ID],
            };
            return next();
        }

        // 2. Bearer token (session-attached or Authorization header)
        const token = accessTokenPresent(req);
        if (token) {
            if (!checkOrgMembership(req)) {
                const err = new Error('Authentication required');
                err.status = 401;
                return next(err);
            }
            const { valid, scopes } = await verifyBearerToken(token, req);
            if (!valid) {
                const err = new Error('Authentication required');
                err.status = 401;
                return next(err);
            }
            const decoded = jwt.decode(req.user?.[constants.ACCESS_TOKEN] || token) || {};
            req[constants.USER_ID] = decoded[constants.USER_ID];
            req.auth = {
                mode: 'oauth2',
                scopes: String(scopes || '').split(' ').filter(Boolean),
                userId: decoded[constants.USER_ID],
            };
            return next();
        }

        // 3. API key
        if (config.advanced?.apiKey?.enabled) {
            const keyType = config.advanced.apiKey.keyType;
            if (keyType && config.advanced?.apiKey?.keyValue) {
                const apiKey = req.headers[keyType.toLowerCase()];
                if (apiKey && apiKey === config.advanced?.apiKey?.keyValue) {
                    if (req.headers.organization && req.params && !req.params.orgId) {
                        req.params.orgId = req.headers.organization;
                    }
                    req.auth = { mode: 'apikey', preauthorized: true, scopes: [] };
                    return next();
                }
            }
        }

        // 4. mTLS
        if (typeof req.socket?.getPeerCertificate === 'function') {
            const cert = req.socket.getPeerCertificate(true);
            if (cert && Object.keys(cert).length > 0 && req.client?.authorized) {
                const now = new Date();
                if (new Date(cert.valid_from) <= now && new Date(cert.valid_to) >= now) {
                    req.auth = { mode: 'mtls', preauthorized: true, scopes: [] };
                    return next();
                }
            }
        }

        // 5. No usable credential — pass through as anonymous so the OpenAPI
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
};
