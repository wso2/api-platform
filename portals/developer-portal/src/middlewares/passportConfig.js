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

const passport = require('passport');
const OAuth2Strategy = require('passport-oauth2');
const jwt = require('jsonwebtoken');
const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const util = require('../utils/util');

function configurePassport(SERVER_ID) {
    const claimNames = {
        [constants.ROLES.ROLE_CLAIM]: config.roleClaim,
        [constants.ROLES.GROUP_CLAIM]: config.groupsClaim,
        [constants.ROLES.ORGANIZATION_CLAIM]: config.orgIDClaim,
    };

    if (config.identityProvider?.clientId) {
        const strategy = new OAuth2Strategy({
            name: 'Asgardeo',
            issuer: config.identityProvider.issuer,
            authorizationURL: config.identityProvider.authorizationURL,
            tokenURL: config.identityProvider.tokenURL,
            userInfoURL: config.identityProvider.userInfoURL,
            clientID: config.identityProvider.clientId,
            callbackURL: config.identityProvider.callbackURL,
            pkce: true,
            state: true,
            logoutURL: process.env.OAUTH2_LOGOUT_ENDPOINT,
            logoutRedirectURI: process.env.OAUTH2_POST_LOGOUT_REDIRECT_URI,
            certificate: '',
            jwksURL: process.env.OAUTH2_JWKS_ENDPOINT,
            passReqToCallback: true,
            scope: ['openid', 'profile', 'email'],
        }, async (req, accessToken, refreshToken, params, profile, done) => {
            if (!accessToken) {
                return done(new Error('Access token missing'));
            }
            let orgList, userOrg;
            let isAdmin = false, isSuperAdmin = false;
            if (config.advanced.tokenExchanger?.enabled) {
                try {
                    const exchangedToken = await util.tokenExchanger(accessToken, req.session.returnTo.split("/")[1]);
                    const decodedExchangedToken = jwt.decode(exchangedToken);
                    orgList = decodedExchangedToken.organizations;
                    userOrg = decodedExchangedToken.organization.uuid;
                    req['exchangedToken'] = exchangedToken;
                    const exchangeTokenScopes = (decodedExchangedToken?.scope || '').split(' ');
                    isAdmin = exchangeTokenScopes.includes(config.advanced.tokenExchanger.admin_scope || "apim:admin");
                } catch (error) {
                    logger.error('Token exchange failed during authentication', {
                        error: error.message,
                        returnTo: req.session.returnTo
                    });
                    return done(error);
                }
            }
            const decodedJWT = jwt.decode(params.id_token) || {};
            const decodedAccessToken = jwt.decode(accessToken);
            const firstName = decodedJWT['given_name'] || decodedJWT['nickname'];
            const lastName = decodedJWT['family_name'];
            const organizationID = decodedJWT[claimNames[constants.ROLES.ORGANIZATION_CLAIM]] ? decodedJWT[config.orgIDClaim] : '';
            const roles = decodedJWT[claimNames[constants.ROLES.ROLE_CLAIM]] ? decodedJWT[config.roleClaim] : '';
            const groups = decodedJWT[claimNames[constants.ROLES.GROUP_CLAIM]] ? decodedJWT[config.groupsClaim] : '';
            if (roles.includes(constants.ROLES.SUPER_ADMIN) || roles.includes(constants.ROLES.ADMIN)) {
                isAdmin = true;
            }
            if (roles.includes(constants.ROLES.SUPER_ADMIN)) {
                isSuperAdmin = true;
            }
            const returnTo = req.session.returnTo;
            let view = '';
            if (returnTo) {
                const startIndex = returnTo.indexOf('/views/') + 7;
                const endIndex = returnTo.indexOf('/', startIndex) !== -1 ? returnTo.indexOf('/', startIndex) : returnTo.length;
                view = returnTo.substring(startIndex, endIndex);
            }
            const imageURL = decodedJWT['google_pic_url'] || decodedJWT['picture'] || constants.DEFAULT_PROFILE_IMAGE_URL;
            profile = {
                firstName: firstName ? (firstName.includes(" ") ? firstName.split(" ")[0] : firstName) : '',
                lastName: lastName ? lastName : (firstName && firstName.includes(" ") ? firstName.split(" ")[1] : ''),
                view,
                idToken: params.id_token,
                email: decodedJWT['email'] || req.session.username,
                [constants.ROLES.ORGANIZATION_CLAIM]: organizationID,
                returnTo: req.session.returnTo,
                accessToken,
                refreshToken,
                authorizedOrgs: orgList,
                exchangeToken: req.exchangedToken,
                [constants.ROLES.ROLE_CLAIM]: roles,
                [constants.ROLES.GROUP_CLAIM]: groups,
                isAdmin,
                isSuperAdmin,
                [constants.USER_ID]: decodedAccessToken[constants.USER_ID],
                serverId: SERVER_ID,
                imageURL,
                userOrg,
            };
            req.session.regenerate((err) => {
                if (err) {
                    logger.error('Session regeneration failed', {
                        error: err.message,
                        stack: err.stack,
                        operation: 'sessionRegeneration'
                    });
                    return done(err);
                }
                req.login(profile, (err) => {
                    if (err) {
                        logger.error('Login failed after session regeneration', {
                            error: err.message,
                            stack: err.stack,
                            operation: 'loginAfterSessionRegen'
                        });
                        return done(err);
                    }
                    logger.debug('Returning profile', { userId: profile.sub, organization: userOrg });
                    return done(null, profile);
                });
            });
        });

        strategy.authorizationParams = function (options) {
            const params = {};
            if (options.prompt) params.prompt = options.prompt;
            if (options.fidp) params.fidp = options.fidp;
            if (options.username) params.username = options.username;
            return params;
        };

        passport.use(strategy);
    }

    passport.serializeUser((user, done) => {
        logger.debug('Serializing user', { userId: user.sub });
        const profile = {
            firstName: user.firstName,
            lastName: user.lastName,
            email: user.email,
            imageURL: user.imageURL,
            view: user.view,
            idToken: user.idToken,
            [constants.ROLES.ORGANIZATION_CLAIM]: user[constants.ROLES.ORGANIZATION_CLAIM],
            returnTo: user.returnTo,
            accessToken: user.accessToken,
            refreshToken: user.refreshToken,
            exchangeToken: user.exchangeToken,
            authorizedOrgs: user.authorizedOrgs,
            [constants.ROLES.ROLE_CLAIM]: user.roles,
            [constants.ROLES.GROUP_CLAIM]: user.groups,
            isAdmin: user.isAdmin,
            isSuperAdmin: user.isSuperAdmin,
            [constants.USER_ID]: user[constants.USER_ID],
            userOrg: user.userOrg,
            isLocalAuth: user.isLocalAuth || false,
            serverId: user.serverId,
        };
        done(null, profile);
    });

    passport.deserializeUser((sessionData, done) => {
        done(null, sessionData);
    });
}

module.exports = { configurePassport };

