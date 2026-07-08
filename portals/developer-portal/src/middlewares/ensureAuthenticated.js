/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
const minimatch = require('minimatch');
const constants = require('../utils/constants');
const { config } = require('../config/configLoader');
const orgDao = require('../dao/organizationDao');
const { validationResult } = require('express-validator');
const { jwtVerify, createRemoteJWKSet } = require('jose');
const util = require('../utils/util');
const { CustomError } = require('../utils/errors/customErrors');
const jwt = require('jsonwebtoken');
const logger = require('../config/logger');
const { extractPlatformJwtClaims } = require('../utils/platformJwt');
const { accessTokenPresent, refreshAccessToken, verifyWithCertificate } = require('../utils/tokenUtil');
const { resolveUserUuid } = require('./authMiddleware');

// System page-access gates (constants.js) merged with any deployer-supplied additions
// (config.pageAccessRules, via config.toml) — the config side only ever adds patterns,
// never replaces the fixed system list. Computed once; config is static after startup.
const AUTHENTICATED_PAGES = [
    ...constants.ROUTE.SYSTEM_AUTHENTICATED_PAGES,
    ...(config.pageAccessRules?.authenticated || []),
];
const AUTHORIZED_PAGES = [
    ...constants.ROUTE.SYSTEM_AUTHORIZED_PAGES,
    ...(config.pageAccessRules?.authorized || []),
];

function enforceSecurity(scope) {
    return async function (req, res, next) {
        try {
            const rules = util.validateRequestParameters();
            for (let validation of rules) {
                await validation.run(req);
            }
            const errors = validationResult(req);
            if (!errors.isEmpty()) {
                return res.status(400).json(util.getErrors(errors));
            }
            // Local auth users: validate dp:* scope from platform JWT
            if (req.isAuthenticated() && req.user && req.user.isLocalAuth && !config.idp?.clientId) {
                const platformToken = req.user[constants.ACCESS_TOKEN];
                if (!platformToken) return util.handleError(res, new CustomError(401, constants.ERROR_CODE[401], constants.ERROR_MESSAGE.UNAUTHENTICATED));
                const tokenScopes = extractPlatformJwtClaims(platformToken, null)?.scopes ?? [];
                if (!scope || tokenScopes.includes(scope)) return next();
                return util.handleError(res, new CustomError(403, constants.ERROR_CODE[403], constants.ERROR_MESSAGE.FORBIDDEN));
            }
            const token = accessTokenPresent(req);
            if (token) {
                if (req.user && req.user[constants.ORG_IDENTIFIER] && req.user[constants.ROLES.ORGANIZATION_CLAIM] !== req.user[constants.ORG_IDENTIFIER]) {
                    const authorizedOrgs = req.user.authorizedOrgs;
                    if ((authorizedOrgs && !(authorizedOrgs.includes(req.user[constants.ORG_IDENTIFIER]))) || !authorizedOrgs) {
                        const err = new Error('Forbidden');
                        err.status = 403;
                        return next(err);
                    }
                }
                const decodedAccessToken = jwt.decode(token);
                req[constants.USER_ID] = await resolveUserUuid(req, decodedAccessToken?.[constants.USER_ID]);
                return validateAuthentication(scope)(req, res, next);
            } else if (config.security.serviceApiKey.enabled) {
                enforceAPIKey(req, res, next);
            } else if (typeof req.socket?.getPeerCertificate === 'function' && req.socket.getPeerCertificate(true)) {
                enforceMTLS(req, res, next);
            } else {
                req.session.returnTo = req.originalUrl || `/${req.params.orgName}`;
                if (req.params.orgName) {
                    res.redirect(`/${req.params.orgName}/views/${req.session.view}/login`);
                }
            }
        } catch (err) {
            logger.error("Error checking access token", { error: err.message, stack: err.stack, operation: "checkAccessToken" });
            return res.status(500).json({ error: "Internal Server Error" });
        }
    }
}

// Checks whether a role claim value (string or array) contains an exact role name.
function hasRole(roleClaimValue, roleName) {
    if (!roleClaimValue || !roleName) return false;
    if (Array.isArray(roleClaimValue)) return roleClaimValue.includes(roleName);
    return String(roleClaimValue).split(/[\s,]+/).includes(roleName);
}

const ensurePermission = (currentPage, role, req) => {
    let adminRole, superAdminRole, subscriberRole;
    if (req.user) {
        adminRole = req.user[constants.ROLES.ADMIN];
        superAdminRole = req.user[constants.ROLES.SUPER_ADMIN];
        subscriberRole = req.user[constants.ROLES.SUBSCRIBER];
        if (constants.ROUTE.DEVPORTAL_CONFIGURE.some(pattern => minimatch.minimatch(currentPage, pattern))) {
            return hasRole(role, superAdminRole) || hasRole(role, adminRole);
        } else if (constants.ROUTE.DEVPORTAL_ROOT.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
            return hasRole(role, superAdminRole);
        } else if (AUTHORIZED_PAGES.some(pattern => minimatch.minimatch(currentPage, pattern))) {
            return hasRole(role, subscriberRole) || hasRole(role, adminRole) || hasRole(role, superAdminRole);
        }
    }
    return false;
}

const ensureAuthenticated = async (req, res, next) => {
    let adminRole = config.idp?.roles?.admin;
    let superAdminRole = config.idp?.roles?.superAdmin;
    let subscriberRole = config.idp?.roles?.subscriber;
    const rules = util.validateRequestParameters();
    for (let validation of rules) {
        await validation.run(req);
    }
    const errors = validationResult(req);
    if (!errors.isEmpty()) {
        return res.status(400).json(util.getErrors(errors));
    }
    if (req.user && Array.isArray(req.user.authorizedOrgs) && req.user.userOrg) {
        if (req.user.authorizedOrgs.includes(req.user.userOrg)) {
            req.user[constants.ORG_IDENTIFIER] = req.user.userOrg;
        }
    }
    // Resolve the acting user's internal UUID for every authenticated request, not just
    // pages gated by authenticatedPages below — resolveActor() (used for created_by/updated_by
    // audit columns and "my resources" filters like subscriptions) must return the same
    // identity here as it does on /api/v0.9 REST routes, where authResolver always resolves it.
    if (req.isAuthenticated() && req.user && !req[constants.USER_ID]) {
        if (req.user.isLocalAuth && !config.idp?.clientId) {
            req[constants.USER_ID] = await resolveUserUuid(req, req.user[constants.USER_ID]);
        } else {
            const earlyToken = accessTokenPresent(req);
            if (earlyToken) {
                const earlyDecoded = jwt.decode(earlyToken);
                req[constants.USER_ID] = await resolveUserUuid(req, earlyDecoded?.[constants.USER_ID]);
            }
        }
    }
    if (req.originalUrl !== '/favicon.ico' && req.originalUrl !== '/images' &&
        AUTHENTICATED_PAGES.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
        const orgId = req.params.orgName;
        let orgDetails;
        if (orgId !== undefined) {
            orgDetails = await orgDao.get(orgId);
        }
        let role;
        logger.debug("Request authentication status", { isAuthenticated: req.isAuthenticated() });
        if (req.isAuthenticated()) {
            // Config-auth: skip all token/exchange checks; roles already in session
            if (req.user && req.user.isLocalAuth && !config.idp?.clientId) {
                req.orgId = req.orgId || orgDetails?.uuid;
                req[constants.USER_ID] = await resolveUserUuid(req, req.user[constants.USER_ID]);
                if (AUTHORIZED_PAGES.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
                    if (req.user) {
                        req.user[constants.ROLES.ADMIN] = adminRole;
                        req.user[constants.ROLES.SUPER_ADMIN] = superAdminRole;
                        req.user[constants.ROLES.SUBSCRIBER] = subscriberRole;
                        if (orgDetails) {
                            req.user[constants.ORG_UUID] = orgDetails.uuid;
                            req.user[constants.ORG_IDENTIFIER] = orgDetails.idp_ref_id;
                        }
                    }
                    if (config.security.roleValidation) {
                        role = req.user[constants.ROLES.ROLE_CLAIM];
                        if (ensurePermission(req.originalUrl, role, req)) {
                            return next();
                        } else {
                            const err = new Error('Forbidden');
                            err.status = 403;
                            return next(err);
                        }
                    }
                }
                return next();
            }
            const token = accessTokenPresent(req);
            if (token) {
                const decodedAccessToken = jwt.decode(token);
                req.orgId = req.orgId || orgDetails?.uuid;
                req[constants.USER_ID] = await resolveUserUuid(req, decodedAccessToken?.[constants.USER_ID]);
            }
            if (AUTHORIZED_PAGES.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
                role = req.user[constants.ROLES.ROLE_CLAIM];
                if (req.user) {
                    req.user[constants.ROLES.ADMIN] = adminRole;
                    req.user[constants.ROLES.SUPER_ADMIN] = superAdminRole;
                    req.user[constants.ROLES.SUBSCRIBER] = subscriberRole;
                    if (orgDetails) {
                        req.user[constants.ORG_UUID] = orgDetails.uuid;
                        req.user[constants.ORG_IDENTIFIER] = orgDetails.idp_ref_id;
                    }
                }
                const isMatch = constants.ROUTE.DEVPORTAL_ROOT.some(pattern => minimatch.minimatch(req.originalUrl, pattern));
                if (!isMatch) {
                    const orgIdentifier = orgDetails?.idp_ref_id;
                    const tokenOrgClaim = req.user[constants.ROLES.ORGANIZATION_CLAIM];
                    if (orgIdentifier && tokenOrgClaim && tokenOrgClaim !== orgIdentifier) {
                        const err = new Error('Forbidden');
                        err.status = 403;
                        return next(err);
                    }
                }
                if (config.security.roleValidation) {
                    if (ensurePermission(req.originalUrl, role, req)) {
                        return next();
                    } else {
                        const err = new Error('Forbidden');
                        err.status = 403;
                        return next(err);
                    }
                }
            }
            return next();
        } else {
            req.session.returnTo = req.originalUrl || `/${req.params.orgName}`;
            req.session.save((err) => {
                if (err) {
                    logger.error('Session save failed before login redirect', { error: err.message });
                }
                if (req.params.orgName) {
                    res.redirect(`/${req.params.orgName}/views/${req.params.viewName}/login`);
                } else {
                    res.redirect(303, `/portal/login`);
                }
            });
        }
    } else {
        return next();
    };
};

function validateAuthentication(scope) {
    return async function (req, res, next) {
        const rules = util.validateRequestParameters();
        for (let validation of rules) {
            await validation.run(req);
        }
        const errors = validationResult(req);
        if (!errors.isEmpty()) {
            return res.status(400).json(util.getErrors(errors));
        }
        let IDP, valid, scopes;
        IDP = config.idp || {};

        let accessToken;
        if (req.isAuthenticated() && req.user) {
            accessToken = req.user[constants.ACCESS_TOKEN];
        } else {
            accessToken = req.headers.authorization && req.headers.authorization.split(' ')[1];
        }

        if (IDP.certificate) {
            ({ valid, scopes } = await verifyWithCertificate(accessToken, IDP.certificate));
        } else if (IDP.jwksUrl) {
            ({ valid, scopes } = await validateWithJwks(accessToken, IDP.jwksUrl, req));
        } else {
            valid = false;
        }

        if (valid) {
            if (String(scopes || '').split(' ').includes(scope)) {
                return next();
            }
            return util.handleError(res, new CustomError(403, constants.ERROR_CODE[403], constants.ERROR_MESSAGE.FORBIDDEN));
        }
        if (req.user) {
            return res.redirect('login');
        }
        return util.handleError(res, new CustomError(401, constants.ERROR_CODE[401], constants.ERROR_MESSAGE.UNAUTHENTICATED));
    }
}

const validateWithJwks = async (token, jwksURL, req) => {
    try {
        const jwks = await createRemoteJWKSet(new URL(jwksURL));
        const jwtVerifyOptions = {};
        if (config.idp?.issuer) jwtVerifyOptions.issuer = config.idp.issuer;
        if (config.idp?.audience) jwtVerifyOptions.audience = config.idp.audience;
        const { payload } = await jwtVerify(token, jwks, jwtVerifyOptions);
        return { valid: true, scopes: payload.scope || '' };
    } catch (err) {
        logger.error("Invalid token", { error: err.message, stack: err.stack, operation: "tokenValidation" });
        if (err.code === 'ERR_JWT_EXPIRED' && req.user && req.user.refreshToken) {
            try {
                logger.info("Access token expired, triggering refresh token flow");
                const response = await refreshAccessToken(req.user.refreshToken);
                req.user[constants.ACCESS_TOKEN] = response.access_token;
                req.user[constants.REFRESH_TOKEN] = response.refresh_token;
                return { valid: true, scopes: response.scope || '' };
            } catch (error) {
                logger.error("Error refreshing access token", { error: error.message, stack: error.stack, operation: "refreshToken" });
                return { valid: false, scopes: '' };
            }
        }
        return { valid: false, scopes: '' };
    }
};

const enforceMTLS = (req, res, next) => {
    const clientCert = req.socket?.getPeerCertificate?.(true);

    if (!clientCert || Object.keys(clientCert).length === 0) {
        return res.status(403).send('Client certificate required');
    }

    if (!req.client.authorized) {
        return res.status(403).send('Client certificate verification failed');
    }

    const now = new Date();
    const validFrom = new Date(clientCert.valid_from);
    const validTo = new Date(clientCert.valid_to);
    if (validFrom > now || validTo < now) {
        return res.status(403).send('Client certificate is expired or not yet valid');
    }

    return next();
};

const enforceAPIKey = (req, res, next) => {
    const keyType = config.security?.serviceApiKey?.headerName;

    if (!keyType || !config.security?.serviceApiKey?.value) {
        return res.status(500).json({ error: "Server configuration error" });
    }

    const apiKey = req.headers[keyType.toLowerCase()];

    if (!apiKey || apiKey !== config.security?.serviceApiKey?.value) {
        return res.status(401).json({ error: "Unauthorized: API key is invalid or not found" });
    }
    return next();
};

module.exports = {
    ensureAuthenticated,
    validateAuthentication,
    enforceSecurity
}
