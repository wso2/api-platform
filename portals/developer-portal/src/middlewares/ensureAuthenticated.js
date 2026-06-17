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
const { accessTokenPresent, refreshAccessToken, verifyWithCertificate, resolveOrgIdp } = require('../utils/tokenUtil');

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
            if (req.isAuthenticated() && req.user && req.user.isLocalAuth && !config.identityProvider?.clientId) {
                const platformToken = req.user[constants.ACCESS_TOKEN];
                if (!platformToken) return util.handleError(res, new CustomError(401, constants.ERROR_CODE[401], constants.ERROR_MESSAGE.UNAUTHENTICATED));
                const tokenScopes = extractPlatformJwtClaims(platformToken, null)?.scopes ?? [];
                if (!scope || tokenScopes.includes(scope)) return next();
                return util.handleError(res, new CustomError(403, constants.ERROR_CODE[403], constants.ERROR_MESSAGE.FORBIDDEN));
            }
            const token = accessTokenPresent(req);
            if (token) {
                if (req.user && req.user[constants.ROLES.ORGANIZATION_CLAIM] !== req.user[constants.ORG_IDENTIFIER]) {
                    const authorizedOrgs = req.user.authorizedOrgs;
                    if ((authorizedOrgs && !(authorizedOrgs.includes(req.user[constants.ORG_IDENTIFIER]))) || !authorizedOrgs) {
                        const err = new Error('Authentication required');
                        err.status = 401;
                        return next(err);
                    }
                }
                const decodedAccessToken = jwt.decode(token);
                req[constants.USER_ID] = decodedAccessToken?.[constants.USER_ID];
                return validateAuthentication(scope)(req, res, next);
            } else if (config.advanced.apiKey.enabled) {
                if (req.headers.organization) {
                    req.params.orgId = req.headers.organization;
                }
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

const ensurePermission = (currentPage, role, req) => {
    let adminRole, superAdminRole, subscriberRole;
    if (req.user) {
        adminRole = req.user[constants.ROLES.ADMIN];
        superAdminRole = req.user[constants.ROLES.SUPER_ADMIN];
        subscriberRole = req.user[constants.ROLES.SUBSCRIBER];
        if (constants.ROUTE.DEVPORTAL_CONFIGURE.some(pattern => minimatch.minimatch(currentPage, pattern))) {
            return role.includes(superAdminRole) || role.includes(adminRole);
        } else if (constants.ROUTE.DEVPORTAL_ROOT.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
            return role.includes(superAdminRole);
        } else if (config.authorizedPages.some(pattern => minimatch.minimatch(currentPage, pattern))) {
            return role.includes(subscriberRole) || role.includes(adminRole) || role.includes(superAdminRole);
        }
    }
    return false;
}

const ensureAuthenticated = async (req, res, next) => {
    let adminRole = config.adminRole;
    let superAdminRole = config.superAdminRole;
    let subscriberRole = config.subscriberRole;
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
    if (req.originalUrl !== '/favicon.ico' && req.originalUrl !== '/images' &&
        config.authenticatedPages.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
        let orgID;
        if (req.params.orgName) {
            orgID = req.params.orgName;
        } else {
            orgID = req.params.orgId;
        }
        let orgDetails;
        if (orgID !== undefined) {
            orgDetails = await orgDao.get(orgID);
            adminRole = orgDetails.ADMIN_ROLE || adminRole;
            superAdminRole = orgDetails.SUPER_ADMIN_ROLE || superAdminRole;
            subscriberRole = orgDetails.SUBSCRIBER_ROLE || subscriberRole;
        }
        let role;
        logger.debug("Request authentication status", { isAuthenticated: req.isAuthenticated() });
        if (req.isAuthenticated()) {
            // Config-auth: skip all token/exchange checks; roles already in session
            if (req.user && req.user.isLocalAuth && !config.identityProvider?.clientId) {
                req[constants.USER_ID] = req.user[constants.USER_ID];
                if (config.authorizedPages.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
                    if (req.user) {
                        req.user[constants.ROLES.ADMIN] = adminRole;
                        req.user[constants.ROLES.SUPER_ADMIN] = superAdminRole;
                        req.user[constants.ROLES.SUBSCRIBER] = subscriberRole;
                        if (orgDetails) {
                            req.user[constants.ORG_ID] = orgDetails.ORG_ID;
                            req.user[constants.ORG_IDENTIFIER] = orgDetails.ORGANIZATION_IDENTIFIER;
                        }
                    }
                    if (!config.advanced.disabledRoleValidation) {
                        role = req.user[constants.ROLES.ROLE_CLAIM];
                        if (ensurePermission(req.originalUrl, role, req)) {
                            return next();
                        } else {
                            return res.status(403).send("User unauthorized");
                        }
                    }
                }
                return next();
            }
            const token = accessTokenPresent(req);
            if (token) {
                const decodedAccessToken = jwt.decode(token);
                req[constants.USER_ID] = decodedAccessToken[constants.USER_ID];
            }
            if (config.authorizedPages.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
                role = req.user[constants.ROLES.ROLE_CLAIM];
                if (req.user) {
                    req.user[constants.ROLES.ADMIN] = adminRole;
                    req.user[constants.ROLES.SUPER_ADMIN] = superAdminRole;
                    req.user[constants.ROLES.SUBSCRIBER] = subscriberRole;
                    if (orgDetails) {
                        req.user[constants.ORG_ID] = orgDetails.ORG_ID;
                        req.user[constants.ORG_IDENTIFIER] = orgDetails.ORGANIZATION_IDENTIFIER;
                    }
                }
                const isMatch = constants.ROUTE.DEVPORTAL_ROOT.some(pattern => minimatch.minimatch(req.originalUrl, pattern));

                if (!isMatch) {
                    if (req.user && req.user[constants.ROLES.ORGANIZATION_CLAIM] !== req.user[constants.ORG_IDENTIFIER]) {
                        const allowedOrgs = req.user.authorizedOrgs;
                        logger.debug("User authorized organization", { userOrg: req.user.userOrg });
                        if (req.user.userOrg !== req.user[constants.ORG_IDENTIFIER]) {
                            if (allowedOrgs && (allowedOrgs.includes(req.user[constants.ORG_IDENTIFIER]))) {
                                try {
                                    const exchangedToken = await util.tokenExchanger(req.user[constants.EXCHANGE_TOKEN], req.user[constants.ORG_IDENTIFIER]);
                                    const decodedExchangedToken = jwt.decode(exchangedToken);
                                    const userOrg = decodedExchangedToken.organization.uuid;

                                    req.user[constants.EXCHANGE_TOKEN] = exchangedToken;
                                    req.user['userOrg'] = userOrg;
                                    req.user[constants.ROLES.ORGANIZATION_CLAIM] = userOrg;
                                    req.user[constants.ORG_IDENTIFIER] = userOrg;

                                    const freshScopes = (decodedExchangedToken?.scope || '').split(' ');
                                    const freshScopeHasAdmin = freshScopes.includes(config.advanced.tokenExchanger.admin_scope || "apim:admin");
                                    const roleBasedAdmin = role && (role.includes(adminRole) || role.includes(superAdminRole));
                                    req.user.isAdmin = freshScopeHasAdmin || roleBasedAdmin;
                                } catch (error) {
                                    logger.error("Error during token exchange", { error: error.message, stack: error.stack, operation: "tokenExchange" });
                                    const err = new Error('Authentication required');
                                    err.status = 401;
                                    return next(err);
                                }
                            } else {
                                const err = new Error('Authentication required');
                                err.status = 401;
                                return next(err);
                            }
                        }
                    }
                }
                if (!config.advanced.disabledRoleValidation) {
                    if (ensurePermission(req.originalUrl, role, req)) {
                        return next();
                    } else {
                        return res.send("User unauthorized");
                    }
                }
            }
            return next();
        } else {
            await req.session.save(async (err) => {
                if (err) {
                    return res.status(500).send('Internal Server Error');
                }
                req.session.returnTo = req.originalUrl || `/${req.params.orgName}`;
                if (req.params.orgName) {
                    res.redirect(`/${req.params.orgName}/views/${req.params.viewName}/login`);
                } else {
                    res.redirect(303, `/portal/login`);
                }
            });
        }
    } else {
        if (req.isAuthenticated()) {
            const token = accessTokenPresent(req);
            if (token && config.identityProvider.jwksURL) {
                await validateWithJwks(token, config.identityProvider.jwksURL, req);
            }
        }
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

        const IDP = await resolveOrgIdp(req);

        let accessToken;
        if (req.isAuthenticated() && req.user) {
            accessToken = req.user[constants.ACCESS_TOKEN];
        } else {
            accessToken = req.headers.authorization && req.headers.authorization.split(' ')[1];
        }

        let valid, scopes;
        if (IDP.certificate) {
            ({ valid, scopes } = await verifyWithCertificate(accessToken, IDP.certificate));
        } else if (IDP.jwksURL) {
            ({ valid, scopes } = await validateWithJwks(accessToken, IDP.jwksURL, req));
        } else {
            valid = false;
        }

        if (valid) {
            if (String(scopes || '').split(' ').includes(scope)) {
                return next();
            }
            if (req.user) {
                return res.redirect('login');
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
        const { payload } = await jwtVerify(token, jwks);
        return { valid: true, scopes: payload.scope || '' };
    } catch (err) {
        logger.error("Invalid token", { error: err.message, stack: err.stack, operation: "tokenValidation" });
        if (err.code === 'ERR_JWT_EXPIRED' && req.user && req.user.refreshToken) {
            try {
                logger.info("Access token expired, triggering refresh token flow");
                const response = await refreshAccessToken(req.user.refreshToken);
                req.user[constants.ACCESS_TOKEN] = response.access_token;
                req.user[constants.REFRESH_TOKEN] = response.refresh_token;
                req.user[constants.EXCHANGE_TOKEN] = await util.tokenExchanger(response.access_token, req.user.returnTo.split('/')[1]);
                return { valid: true, scopes: response.scope || '' };
            } catch (error) {
                req.user = null;
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
    const keyType = config.advanced?.apiKey?.keyType;

    if (!keyType || !config.advanced?.apiKey?.keyValue) {
        return res.status(500).json({ error: "Server configuration error" });
    }

    const apiKey = req.headers[keyType.toLowerCase()];

    if (!apiKey || apiKey !== config.advanced?.apiKey?.keyValue) {
        return res.status(401).json({ error: "Unauthorized: API key is invalid or not found" });
    }
    return next();
};

module.exports = {
    ensureAuthenticated,
    validateAuthentication,
    enforceSecurity
}
