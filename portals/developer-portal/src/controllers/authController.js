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
/* eslint-disable no-undef */
const passport = require('passport');
const axios = require('axios');
const https = require('https');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const util = require('../utils/util');
const adminDao = require('../dao/admin');
const IdentityProviderDTO = require("../dto/identityProvider");
const minimatch = require('minimatch');
const { validationResult } = require('express-validator');
const { renderGivenTemplate } = require('../utils/util');
const { trackLoginTrigger, trackLogoutTrigger } = require('../utils/telemetry');


const fetchAuthJsonContent = async (req, orgName) => {

    //use super admin for org creation page login
    if (req.session.returnTo) {
        if (constants.ROUTE.DEVPORTAL_ROOT.some(pattern => minimatch.minimatch(req.session.returnTo, pattern))) {
            return config.identityProvider;
        }
    }
    //if no idp per org, use super IDP
    try {
        const orgId = await adminDao.getOrgId(orgName);
        const response = await adminDao.getIdentityProvider(orgId);
        if (response.length === 0) {
            //login from super IDP
            return config.identityProvider;
        }
        return new IdentityProviderDTO(response[0].dataValues);
    } catch (error) {
        logger.error("Failed to fetch identity provider details", {
            orgName: orgName,
            error: error.message,
            stack: error.stack
        });
        return config.identityProvider;
    }
};

const login = async (req, res, next) => {

    let claimNames = {
        [constants.ROLES.ROLE_CLAIM]: config.roleClaim,
        [constants.ROLES.GROUP_CLAIM]: config.groupsClaim,
        [constants.ROLES.ORGANIZATION_CLAIM]: config.orgIDClaim
    };  
    const orgName = req.params.orgName;
    const baseUrl = '/' + orgName + constants.ROUTE.VIEWS_PATH + req.params.viewName;
    const orgDetails = await adminDao.getOrganization(orgName);
    if (orgDetails) {
        claimNames[constants.ROLES.ROLE_CLAIM] = orgDetails.ROLE_CLAIM_NAME || config.roleClaim;
        claimNames[constants.ROLES.GROUP_CLAIM] = orgDetails.GROUPS_CLAIM_NAME || config.groupsClaim;
        claimNames[constants.ROLES.ORGANIZATION_CLAIM] = orgDetails.ORGANIZATION_CLAIM_NAME || config.orgIDClaim;
    }
    if (!req.isAuthenticated()) {
        const fidp = req.query.fidp;
        if (config.identityProvider?.clientId && fidp && config.fidp[fidp]) {
            if (fidp == 'enterprise' && req.query.username) {
                req.session.username = req.query.username;
                await passport.authenticate('oauth2', { fidp: config.fidp[fidp], username: req.query.username })(req, res, next);
            } else {
                await passport.authenticate('oauth2', { fidp: config.fidp[fidp] })(req, res, next);
            }
            trackLoginTrigger({ orgName }, req);
        } else if (config.identityProvider?.clientId && fidp && fidp == 'default') {
            await passport.authenticate('oauth2')(req, res, next);
        } else {
            const localAuthEnabled = !config.identityProvider?.clientId;
            const templateContent = {
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + req.params.viewName,
                localAuthEnabled,
                loginError: req.query.error || null,
            };
            const html = util.renderTemplate('../pages/login-page/page.hbs',
                'src/pages/login-page/layout.hbs', templateContent, true);
            res.send(html);
            trackLoginTrigger({ orgName }, req);
        }
    } else {
        res.redirect(baseUrl);
    }     
};

const handleCallback = async (req, res, next) => {
    if (!config.identityProvider?.clientId) return next();
    const rules = util.validateRequestParameters();
    const validationPromises = rules.map(validation => validation.run(req));
    Promise.all(validationPromises)
        .then(() => {
            const errors = validationResult(req);
            if (!errors.isEmpty()) {
                return res.status(400).json(util.getErrors(errors));
            }
        })
        .catch(error => {
            logger.error("Error validating request parameters", {
                error: error.message,
                path: req.path,
                method: req.method,
                params: req.params
            });
            return res.status(500).json({ message: 'Internal Server Error' });
        });
    await passport.authenticate(
        'oauth2',
        {
            failureRedirect: '/login'
        },
        (err, user) => {
            if (err || !user) {
                if (err.name === 'AuthorizationError' && err.code === 'login_required') {
                    return res.redirect(req.session.returnTo);
                } else {
                    return next(err || new Error('Authentication failed'));
                }
            }
            req.logIn(user, (err) => {
                if (err) {
                    return next(err);
                }
                res.set('Cache-Control', 'no-store');
                let returnTo = req.user.returnTo;
                    if (!config.advanced.disableOrgCallback && returnTo == null) {
                        returnTo = `/${req.params.orgName}`;
                    }
                    delete req.session.returnTo;
                    // todo: track login success
                    req.session.save(() => {
                        res.redirect(returnTo);
                    })
            });
        })(req, res, next);
};

const handleSignUp = async (req, res) => {
    const rules = util.validateRequestParameters();
    for (let validation of rules) {
        await validation.run(req);
    }
    const errors = validationResult(req);
    if (!errors.isEmpty()) {
        return res.status(400).json(util.getErrors(errors));
    }
    const authJsonContent = await fetchAuthJsonContent(req.params.orgName);
    if (authJsonContent.signUpURL) {
        res.redirect(authJsonContent.signUpURL);
    } else {
        const returnTo = req.session.returnTo || `/${req.params.orgName}`;
        delete req.session.returnTo;
        res.redirect(returnTo);
    }
};

const handleLogOut = async (req, res) => {
    const rules = util.validateRequestParameters();
    for (let validation of rules) {
        await validation.run(req);
    }
    const errors = validationResult(req);
    if (!errors.isEmpty()) {
        return res.status(400).json(util.getErrors(errors));
    }
    const authJsonContent = await fetchAuthJsonContent(req, req.params.orgName);
    let idToken = ''
    if (req.user != null) {
        idToken = req.user.idToken;
    }
    const currentPathURI = req.originalUrl.replace('/logout', '');
    res.set('Cache-Control', 'no-store');
    if (req.user?.isLocalAuth) {
        // Local-auth users have no IDP — destroy session and redirect to login
        req.logout((err) => {
            if (err) {
                logger.error('Logout error (local-auth)', {
                    userId: req.user?.username || 'unknown',
                    orgName: req.params.orgName,
                    error: err.message,
                });
            }
            logUserAction('USER_LOGOUT', req, { orgName: req.params.orgName });
            req.session.destroy(() => {
                res.set('Cache-Control', 'no-store');
                res.redirect(req.originalUrl.replace('/logout', '/login'));
            });
        });
    } else if (req.user && req.user.accessToken) {
        const referer = req.get('referer');
        const regex = /(.+\/views\/[^\/]+)\/?/;
        const match = referer.match(regex);
        const logoutURL = match ? match[1] : null;
        req.logout((err) => {
            if (err) {
                logger.error("Logout error", {
                    userId: req.user?.id || req.user?.username || 'unknown',
                    orgName: req.params.orgName,
                    error: err.message,
                    stack: err.stack
                });
            }
            logUserAction('USER_LOGOUT', req, {
                orgName: req.params.orgName,
                logoutURL: logoutURL
            });
            trackLogoutTrigger({ orgName: req.params.orgName }, req);
            req.session.currentPathURI = currentPathURI;
            res.redirect(`${authJsonContent.logoutURL}?post_logout_redirect_uri=${authJsonContent.logoutRedirectURI}&id_token_hint=${idToken}`);
        });
    } else {
        // Unauthenticated or session already gone — original behaviour
        res.redirect(req.originalUrl.replace('/logout', ''));
    }
};

const handleLogOutLanding = async (req, res) => {
    const currentPathURI = req.session.currentPathURI;
    req.session.destroy();
    res.redirect(currentPathURI);
}

const handleSilentSSO = async (req, res, next) => {

    // Skip if no IDP configured or silent SSO is disabled
    if (!config.identityProvider?.clientId || config.advanced?.disableSilentSSO) return next();

    await req.session.save((err) => {
        req.session.returnTo = req.originalUrl;

        if (req.isAuthenticated() || req.session.silentAuthRedirected) {
            return next();
        } else {
            passport.authenticate('oauth2', { prompt: 'none' })(req, res, () => { });
            req.session.silentAuthRedirected = true;
        }
    });
};

const handleLocalLogin = async (req, res) => {
    const { username, password } = req.body;
    const orgName = req.params.orgName;
    const viewName = req.params.viewName;
    const baseUrl = `/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}`;

    if (config.identityProvider?.clientId) {
        return res.status(404).send('Not found');
    }
    if (!username || !password) {
        return res.redirect(`${baseUrl}/login?error=Username+and+password+are+required`);
    }

    const platformApiUrl = config.platformApi?.baseUrl;
    if (!platformApiUrl) {
        logger.error('Local auth attempted but platformApi.baseUrl is not configured');
        return res.redirect(`${baseUrl}/login?error=Authentication+service+not+configured`);
    }

    let platformToken;
    try {
        const response = await axios.post(
            `${platformApiUrl}/api/portal/v1/auth/login`,
            new URLSearchParams({ username, password }).toString(),
            {
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                httpsAgent: new https.Agent({ rejectUnauthorized: !config.platformApi?.insecure }),
                timeout: 10000,
            }
        );
        platformToken = response.data.token;
    } catch (error) {
        if (error.response?.status === 401) {
            logger.warn('Platform API login failed: invalid credentials', { orgName });
            return res.redirect(`${baseUrl}/login?error=Invalid+username+or+password`);
        }
        logger.error('Platform API login request failed', { error: error.message, orgName });
        return res.redirect(`${baseUrl}/login?error=Login+failed%2C+please+try+again`);
    }

    // Decode JWT claims (token is already verified by the platform API)
    let claims;
    try {
        claims = JSON.parse(Buffer.from(platformToken.split('.')[1], 'base64url').toString('utf8'));
    } catch (err) {
        logger.error('Failed to decode platform API token', { error: err.message });
        return res.redirect(`${baseUrl}/login?error=Login+failed%2C+please+try+again`);
    }

    const adminRole = config.adminRole || 'admin';
    const superAdminRole = config.superAdminRole || 'superAdmin';
    const subscriberRole = config.subscriberRole || 'Internal/subscriber';
    const scopes = (claims.scope || '').split(' ');
    // Users with any _manage scope are treated as admins in the devportal
    const isAdmin = scopes.some(s => s.endsWith('_manage'));
    const roles = isAdmin ? [adminRole] : [subscriberRole];

    const returnTo = req.session.returnTo;
    let view = viewName;
    if (returnTo) {
        const startIndex = returnTo.indexOf('/views/') + 7;
        const endIndex = returnTo.indexOf('/', startIndex) !== -1
            ? returnTo.indexOf('/', startIndex)
            : returnTo.length;
        view = returnTo.substring(startIndex, endIndex) || viewName;
    }

    const profile = {
        firstName: claims.username || username,
        lastName: '',
        email: claims.email || username,
        imageURL: 'https://raw.githubusercontent.com/wso2/docs-bijira/refs/heads/main/en/devportal-theming/profile.svg',
        view,
        idToken: null,
        [constants.ROLES.ORGANIZATION_CLAIM]: claims.org_handle || orgName,
        returnTo: returnTo || baseUrl,
        accessToken: platformToken,
        refreshToken: null,
        exchangeToken: null,
        authorizedOrgs: [claims.org_handle || orgName],
        [constants.ROLES.ROLE_CLAIM]: roles,
        [constants.ROLES.GROUP_CLAIM]: [],
        isAdmin,
        isSuperAdmin: false,
        [constants.USER_ID]: claims.sub || username,
        userOrg: claims.org_handle || orgName,
        isLocalAuth: true,
    };

    req.session.regenerate((err) => {
        if (err) {
            logger.error('Session regeneration failed (platform-auth)', { error: err.message, stack: err.stack });
            return res.redirect(`${baseUrl}/login?error=Login+failed%2C+please+try+again`);
        }
        req.logIn(profile, (loginErr) => {
            if (loginErr) {
                logger.error('Platform-auth login session error', { error: loginErr.message, stack: loginErr.stack });
                return res.redirect(`${baseUrl}/login?error=Login+failed%2C+please+try+again`);
            }
            res.set('Cache-Control', 'no-store');
            const redirectTo = returnTo || baseUrl;
            req.session.save(() => res.redirect(redirectTo));
        });
    });
};

module.exports = {
    login,
    handleCallback,
    handleSignUp,
    handleLogOut,
    handleLogOutLanding,
    handleSilentSSO,
    handleLocalLogin,
};
