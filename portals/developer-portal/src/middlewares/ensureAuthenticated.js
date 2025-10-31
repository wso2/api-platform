/* eslint-disable no-undef */
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
const config = require(process.cwd() + '/config.json');
const secret = require(process.cwd() + '/secret.json');
const adminDao = require('../dao/admin');
const { validationResult } = require('express-validator');
const { jwtVerify, createRemoteJWKSet, importX509 } = require('jose');
const util = require('../utils/util');
const { CustomError } = require('../utils/errors/customErrors');
const IdentityProviderDTO = require("../dto/identityProvider");
const jwt = require('jsonwebtoken');
const axios = require('axios');
const logger = require('../config/logger');
const qs = require('qs');

function enforceSecuirty(scope) {
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
            const token = accessTokenPresent(req);
            if (token) {
                //check user belongs to organization
                if (req.user && req.user[constants.ROLES.ORGANIZATION_CLAIM] !== req.user[constants.ORG_IDENTIFIER]) {
                    //check if exchanged token has organization identifier
                    //const decodedToken = req.user.exchangeToken ? jwt.decode(req.user.exchangeToken) : null;
                    const authorizedOrgs = req.user.authorizedOrgs;
                    if ((authorizedOrgs && !(authorizedOrgs.includes(req.user[constants.ORG_IDENTIFIER]))) || !authorizedOrgs) {
                        const err = new Error('Authentication required');
                        err.status = 401; // Unauthorized
                        return next(err);
                    }
                }
                // TODO: Implement organization extraction logic
                validateAuthentication(scope)(req, res, next);
                //set user ID
                const decodedAccessToken = jwt.decode(token);
                req[constants.USER_ID] = decodedAccessToken[constants.USER_ID];
            } else if (config.advanced.apiKey.enabled) {
                // Communcation with API KEY
                if (req.headers.organization) {
                    const organization = req.headers.organization;
                    if (organization) {
                        req.params.orgId = organization;
                    }
                }
                enforceAPIKey(req, res, next);
            } else if (req.connection.getPeerCertificate(true)) {
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

function accessTokenPresent(req) {
    if (req.user) {
        return req.user[constants.ACCESS_TOKEN];
    }
    if (req.headers.authorization) {
        return req.headers.authorization.split(' ')[1];
    }
    return null;
}

const ensurePermission = (currentPage, role, req) => {
    let adminRole, superAdminRole, subscriberRole;
    if (req.user) {
        adminRole = req.user[constants.ROLES.ADMIN];
        superAdminRole = req.user[constants.ROLES.SUPER_ADMIN];
        subscriberRole = req.user[constants.ROLES.SUBSCRIBER];
        if (minimatch.minimatch(currentPage, constants.ROUTE.DEVPORTAL_CONFIGURE)) {
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
    if ((req.originalUrl != '/favicon.ico' | req.originalUrl != '/images') &&
        config.authenticatedPages.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
        //fetch role details from DB
        let orgID;
        if (req.params.orgName) {
            orgID = req.params.orgName;
        } else {
            orgID = req.params.orgId;
        }
        let orgDetails;
        if (!(orgID === undefined)) {
            orgDetails = await adminDao.getOrganization(orgID);
            adminRole = orgDetails.ADMIN_ROLE || adminRole;
            superAdminRole = orgDetails.SUPER_ADMIN_ROLE || superAdminRole;
            subscriberRole = orgDetails.SUBSCRIBER_ROLE || subscriberRole;
            organizationClaimName = orgDetails.ORGANIZATION_CLAIM_NAME || config.orgIDClaim;
        }
        let role;
        logger.debug("Request authentication status", { isAuthenticated: req.isAuthenticated() });
        if (req.isAuthenticated()) {
            const token = accessTokenPresent(req);
            if (token) {
                const decodedAccessToken = jwt.decode(token);
                req[constants.USER_ID] = decodedAccessToken[constants.USER_ID];
            }
            if (config.authorizedPages.some(pattern => minimatch.minimatch(req.originalUrl, pattern))) {
                role = req.user[constants.ROLES.ROLE_CLAIM];
                //add organization ID to request
                if (req.user) {
                    //add details to session
                    req.user[constants.ROLES.ADMIN] = adminRole;
                    req.user[constants.ROLES.SUPER_ADMIN] = superAdminRole;
                    req.user[constants.ROLES.SUBSCRIBER] = subscriberRole;
                    if (orgDetails) {
                        req.user[constants.ORG_ID] = orgDetails.ORG_ID;
                        req.user[constants.ORG_IDENTIFIER] = orgDetails.ORGANIZATION_IDENTIFIER;
                    }
                }
                //verify user belongs to organization
                const isMatch = constants.ROUTE.DEVPORTAL_ROOT.some(pattern => minimatch.minimatch(req.originalUrl, pattern));

                if (!isMatch) {
                    if (req.user && req.user[constants.ROLES.ORGANIZATION_CLAIM] !== req.user[constants.ORG_IDENTIFIER]) {
                        //check if exchanged token has organization identifier
                        //const decodedToken = req.user.exchangeToken ? jwt.decode(req.user.exchangeToken) : null;
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
                                } catch (error) {
                                    logger.error("Error during token exchange", { error: error.message, stack: error.stack, operation: "tokenExchange" });
                                    const err = new Error('Authentication required');
                                    err.status = 401; // Unauthorized
                                    return next(err);
                                }
                            } else {
                                const err = new Error('Authentication required');
                                err.status = 401; // Unauthorized
                                return next(err);
                            }
                        }

                    }
                }
                if (!config.advanced.disabledRoleValidation) {
                    if (ensurePermission(req.originalUrl, role, req)) {
                        return next();
                    } else {
                        if (req.params.orgName === undefined) {
                            return res.send("User unauthorized");
                        } else {
                            return res.send("User unauthorized");
                        }
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
                await validateWithJWKS(token, config.identityProvider.jwksURL, req);
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
        let IDP, valid, scopes, orgId, response;
        if (req.params.orgName) {
            orgId = await adminDao.getOrgId(orgName);
        } else {
            orgId = req.params.orgId;
        }
        if (orgId) {
            response = await adminDao.getIdentityProvider(orgId);
            if (response.length !== 0) {
                //login from super IDP
                IDP = new IdentityProviderDTO(response[0].dataValues);
            } else {
                IDP = config.identityProvider;
            }
        } else {
            IDP = config.identityProvider;
        }

        let accessToken, basicHeader;
        //if IDP present, fetch bearer token else use basic header
        if (IDP.clientId !== "") {
            if (req.isAuthenticated() && req.user) {
                accessToken = req.user[constants.ACCESS_TOKEN];
            } else {
                accessToken = req.headers.authorization && req.headers.authorization.split(' ')[1];
            }
            //fetch certificate or JWKS URL
            if (IDP.certificate) {
                const pemKey = IDP.certificate;
                const publicKey = await importX509(pemKey, 'RS256');
                [valid, scopes] = await validateWithCert(accessToken, publicKey);
            } else {
                if (IDP.jwksURL) {
                    [valid, scopes] = await validateWithJWKS(accessToken, IDP.jwksURL, req);
                } else {
                    valid = false;
                }
            }
            if (!config.advanced.disableScopeValidation && valid) {
                if (scopes.split(" ").includes(scope)) {
                    return next();
                } else {
                    if (req.user) {
                        return res.redirect('login');
                    } else {
                        return util.handleError(res, new CustomError(403, constants.ERROR_CODE[403], constants.ERROR_MESSAGE.FORBIDDEN));
                    }
                }
            } else {
                if (req.user) {
                    return next();
                } else {
                    return util.handleError(res, new CustomError(401, constants.ERROR_CODE[401], constants.ERROR_MESSAGE.UNAUTHENTICATED));
                }
            }
        } else {
            if (req.isAuthenticated() && req.user) {
                basicHeader = req.user[constants.BASIC_HEADER];
            } else {
                basicHeader = req.headers.authorization && req.headers.authorization.split(' ')[1];
            }
            valid = await validateBasicAuth(basicHeader);
            if (valid) {
                return next();
            } else {
                if (req.user) {
                    return res.redirect('login');
                } else {
                    return util.handleError(res, new CustomError(401, constants.ERROR_CODE[401], constants.ERROR_MESSAGE.UNAUTHENTICATED));
                }
            }
        }
    }
}

const validateWithCert = async (token, publicKey) => {

    try {
        const { payload } = await jwtVerify(token, publicKey);
        return [true, payload.scope];
    } catch (err) {
        logger.error("Invalid token", { error: err.message, operation: "tokenValidation" });
        return [false, ""];
    }
}

const validateWithJWKS = async (token, jwksURL, req) => {

    try {
        const jwks = await createRemoteJWKSet(new URL(jwksURL));
        const { payload } = await jwtVerify(token, jwks);
        return [true, payload.scope];
    } catch (err) {
        logger.error("Invalid token", { error: err.message, stack: err.stack, operation: "tokenValidation" });
        if (err.code === 'ERR_JWT_EXPIRED' && req.user && req.user.refreshToken) {
            // Token expired, refresh it
            try {
                logger.info("Access token expired, triggering refresh token flow");
                const response = await refreshAccessToken(req.user.refreshToken);
                req.user[constants.ACCESS_TOKEN] = response.access_token;
                req.user[constants.REFRESH_TOKEN] = response.refresh_token;
                req.user[constants.EXCHANGE_TOKEN] = await util.tokenExchanger(response.access_token, req.user.returnTo.split("/")[1]);
                
                return [true, response.scope || ""];
            } catch (error) {
                req.user =  null;
                logger.error("Error refreshing access token", { error: error.message, stack: error.stack, operation: "refreshToken" });
                return [false, ""];           
            }
        } else {
            logger.error("Token validation error", { error: err.message, operation: "tokenValidation" });
            return [false, ""];
        }
    }
}

async function refreshAccessToken(refreshToken) {
    try {
        const data = qs.stringify({
            grant_type: 'refresh_token',
            refresh_token: refreshToken,
            client_id: config.identityProvider.clientId,
        });
        const response = await axios.post(config.identityProvider.tokenURL, data, {
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded'
            }
        });
        return response.data

    } catch (err) {
        logger.error('Token refresh error', { 
            error: err.response?.data || err.message, 
            operation: 'tokenRefresh'
        });
        throw err;
    }
}

const validateBasicAuth = async (basicHeader) => {

    let valid = false;
    const base64Decoded = Buffer.from(basicHeader, 'base64').toString('utf-8');
    const [username, password] = base64Decoded.split(':');
    const users = config.defaultAuth.users;
    for (let user of users) {
        if (username === user.username && password === user.password) {
            valid = true;
            break;
        }
    }
    return valid;
}

const enforceMTLS = (req, res, next) => {
    const clientCert = req.connection.getPeerCertificate(true);

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

    if (!keyType || !secret.apiKeySecret) {
        return res.status(500).json({ error: "Server configuration error" });
    }

    const apiKey = req.headers[keyType.toLowerCase()];

    if (!apiKey || apiKey !== secret.apiKeySecret) {
        return res.status(401).json({ error: "Unauthorized: API key is invalid or not found" });
    }
    return next();
};

function getNestedValue(obj, path) {
    return path.split('.').reduce((acc, key) => acc?.[key], obj);
}


module.exports = {
    ensureAuthenticated,
    validateAuthentication,
    enforceSecuirty
}
