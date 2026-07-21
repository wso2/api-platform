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
const orgDao = require('../dao/organizationDao');
const { validationResult } = require('express-validator');
const { decodePlatformJwtClaims } = require('../utils/platformJwt');



const login = async (req, res, next) => {
    const orgName = req.params.orgName;
    const baseUrl = '/' + orgName + constants.ROUTE.VIEWS_PATH + req.params.viewName;
    if (!req.isAuthenticated()) {
        const fidp = req.query.fidp;
        const fidpMap = config.idp?.fidp || {};
        if (config.idp?.clientId) {
            // IDP mode: redirect directly to the IDP, no intermediate login page
            const orgDetails = await orgDao.get(orgName);
            const orgIdentifier = orgDetails?.idp_ref_id;
            if (fidp && fidpMap[fidp]) {
                if (fidp === 'enterprise' && req.query.username) {
                    req.session.username = req.query.username;
                    await passport.authenticate('oauth2', { fidp: fidpMap[fidp], username: req.query.username, ...(orgIdentifier && { org: orgIdentifier }) })(req, res, next);
                } else {
                    await passport.authenticate('oauth2', { fidp: fidpMap[fidp], ...(orgIdentifier && { org: orgIdentifier }) })(req, res, next);
                }
            } else {
                await passport.authenticate('oauth2', { ...(orgIdentifier && { org: orgIdentifier }) })(req, res, next);
            }
        } else {
            // Local auth mode: show username/password form, themed with the active
            // view's uploaded layout (falls back to the default styles otherwise).
            const orgDetails = await orgDao.get(orgName);
            const templateContent = {
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + req.params.viewName,
                localAuthEnabled: true,
                loginError: req.query.error || null,
            };
            const html = await util.renderTemplateWithView('../pages/login-page/page.hbs',
                'src/pages/login-page/layout.hbs', templateContent, true, orgDetails?.uuid, req.params.viewName);
            res.send(html);
        }
    } else {
        res.redirect(baseUrl);
    }     
};

const handleCallback = async (req, res, next) => {
    if (!config.idp?.clientId) return next();
    const rules = util.validateRequestParameters();
    for (const validation of rules) {
        await validation.run(req);
    }
    const errors = validationResult(req);
    if (!errors.isEmpty()) {
        return res.status(400).json(util.getErrors(errors));
    }
    await passport.authenticate(
        'oauth2',
        {
            failureRedirect: '/login'
        },
        (err, user) => {
            if (err || !user) {
                if (err?.name === 'AuthorizationError' && err?.code === 'login_required') {
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
                if (config.idp?.orgCallback && returnTo == null) {
                    returnTo = `/${req.params.orgName}`;
                }
                returnTo = returnTo || `/${req.params.orgName}`;
                delete req.session.returnTo;
                logUserAction('USER_LOGIN', req, { orgName: req.params.orgName });
                req.session.save((saveErr) => {
                    if (saveErr) {
                        logger.error('Session save failed after login', { error: saveErr.message });
                    }
                    res.redirect(returnTo);
                });
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
    const authJsonContent = config.idp;
    if (authJsonContent?.signUpUrl) {
        res.redirect(authJsonContent.signUpUrl);
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
    const authJsonContent = config.idp;
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
                    userId: req.user?.id || 'unknown',
                    orgName: req.params.orgName,
                    error: err.message,
                });
            }
            logUserAction('USER_LOGOUT', req, { orgName: req.params.orgName });
            req.session.destroy((destroyErr) => {
                if (destroyErr) {
                    logger.error('Session destroy failed on local-auth logout', { error: destroyErr.message });
                }
                res.redirect(req.originalUrl.replace('/logout', '/login'));
            });
        });
    } else if (req.user && req.user.accessToken) {
        const referer = req.get('referer');
        const regex = /(.+\/views\/[^\/]+)\/?/;
        const match = referer ? referer.match(regex) : null;
        const logoutURL = match ? match[1] : null;
        req.logout((err) => {
            if (err) {
                logger.error("Logout error", {
                    userId: req.user?.id || 'unknown',
                    orgName: req.params.orgName,
                    error: err.message,
                    stack: err.stack
                });
            }
            logUserAction('USER_LOGOUT', req, {
                orgName: req.params.orgName,
                logoutURL: logoutURL
            });
            req.session.currentPathURI = currentPathURI;
            req.session.save((saveErr) => {
                if (saveErr) {
                    logger.error('Session save failed before IDP logout redirect', { error: saveErr.message });
                }
                res.redirect(`${authJsonContent.logoutUrl}?post_logout_redirect_uri=${authJsonContent.logoutRedirectUri}&id_token_hint=${idToken}`);
            });
        });
    } else {
        // Unauthenticated or session already gone — original behaviour
        res.redirect(req.originalUrl.replace('/logout', ''));
    }
};

const handleLogOutLanding = async (req, res) => {
    const currentPathURI = req.session.currentPathURI || '/';
    req.session.destroy((err) => {
        if (err) {
            logger.error('Session destroy failed on logout landing', { error: err.message });
        }
        res.redirect(currentPathURI);
    });
}

const handleSilentSSO = async (req, res, next) => {
    // Skip if no IDP configured or silent SSO is disabled
    if (!config.idp?.clientId || !config.idp?.silentSso) return next();

    if (req.isAuthenticated() || req.session.silentAuthRedirected) {
        return next();
    }

    req.session.returnTo = req.originalUrl;
    req.session.silentAuthRedirected = true;
    req.session.save((err) => {
        if (err) {
            logger.error('Session save failed during silent SSO', { error: err.message });
            return next();
        }
        passport.authenticate('oauth2', { prompt: 'none' })(req, res, next);
    });
};

const handleLocalLogin = async (req, res) => {
    const { username, password } = req.body;
    const orgName = req.params.orgName;
    const viewName = req.params.viewName;
    const baseUrl = `/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}`;

    if (config.idp?.clientId) {
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
            `${platformApiUrl}/api/portal/v0.9/auth/login`,
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
            logUserAction('USER_LOGIN_FAILED', req, { orgName, reason: 'invalid_credentials' });
            return res.redirect(`${baseUrl}/login?error=Invalid+username+or+password`);
        }
        logger.error('Platform API login request failed', { error: error.message, orgName });
        return res.redirect(`${baseUrl}/login?error=Login+failed%2C+please+try+again`);
    }

    // Decode JWT claims (token is already verified by the platform API)
    const claims = decodePlatformJwtClaims(platformToken);
    if (!claims) {
        logger.error('Failed to decode platform API token');
        return res.redirect(`${baseUrl}/login?error=Login+failed%2C+please+try+again`);
    }

    const adminRole = config.idp?.roles?.admin || 'admin';
    const superAdminRole = config.idp?.roles?.superAdmin || 'superAdmin';
    const subscriberRole = config.idp?.roles?.subscriber || 'Internal/subscriber';
    // Users with any _manage scope are treated as admins in the devportal
    const isAdmin = claims.scopes.some(s => s.endsWith('_manage'));
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
            logUserAction('USER_LOGIN', req, { orgName, isLocalAuth: true });
            res.set('Cache-Control', 'no-store');
            const redirectTo = returnTo || baseUrl;
            req.session.save((saveErr) => {
                if (saveErr) {
                    logger.error('Session save failed after local login', { error: saveErr.message });
                }
                res.redirect(redirectTo);
            });
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
