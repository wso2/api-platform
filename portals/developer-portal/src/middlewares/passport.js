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
const passport = require('passport');
const OAuth2Strategy = require('passport-oauth2');
const jwt = require('jsonwebtoken');
const https = require('https');
const constants = require('../utils/constants');
const util = require('../utils/util');
const config = require(process.cwd() + '/config.json');
const secret = require(process.cwd() + '/secret.json');


async function configurePassport(authJsonContent, claimNames) {
  
    //set scopes to call API Manager REST apis
    // const requestedScopes = "openid profile apim:subscribe admin dev";
    let scope = requestedScopes.split(" ");
    scope.push(...(authJsonContent.scope ? authJsonContent.scope.split(" ") : ""));
    const strategy = new OAuth2Strategy({
        name: 'Asgardeo',
        issuer: authJsonContent.issuer,
        authorizationURL: authJsonContent.authorizationURL,
        tokenURL: authJsonContent.tokenURL,
        userInfoURL: authJsonContent.userInfoURL,
        clientID: authJsonContent.clientId,
        callbackURL: authJsonContent.callbackURL,
        scope: scope,
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
        // let orgList;
        // if (config.advanced.tokenExchanger.enabled) {
        //     const exchangedToken = await util.tokenExchanger(accessToken, req.session.returnTo.split("/")[1]);
        //     const decodedExchangedToken = jwt.decode(exchangedToken);
        //     orgList = decodedExchangedToken.organizations;
        //     req['exchangedToken'] = exchangedToken;
        // }
        // const decodedJWT = jwt.decode(params.id_token);
        // const decodedAccessToken = jwt.decode(accessToken);
        // const firstName = decodedJWT['given_name'] || decodedJWT['nickname'];
        // const lastName = decodedJWT['family_name'];
        // const organizationID = decodedJWT[claimNames[constants.ROLES.ORGANIZATION_CLAIM]] ? decodedJWT[config.orgIDClaim] : '';
        // const roles = decodedJWT[claimNames[constants.ROLES.ROLE_CLAIM]] ? decodedJWT[config.roleClaim] : '';
        // const groups = decodedJWT[claimNames[constants.ROLES.GROUP_CLAIM]] ? decodedJWT[config.groupsClaim] : '';
        // let isAdmin, isSuperAdmin = false;
        // if (roles.includes(constants.ROLES.SUPER_ADMIN) || roles.includes(constants.ROLES.ADMIN)) {
        //     isAdmin = true;
        // }
        // if (roles.includes(constants.ROLES.SUPER_ADMIN)) {
        //     isSuperAdmin = true;
        // }
        // const returnTo = req.session.returnTo;
        // let view = '';
        // if (returnTo) {
        //     const startIndex = returnTo.indexOf('/views/') + 7;
        //     const endIndex = returnTo.indexOf('/', startIndex) !== -1 ? returnTo.indexOf('/', startIndex) : returnTo.length;
        //     view = returnTo.substring(startIndex, endIndex);
        // }
        // profile = {
        //     'firstName': firstName ? (firstName.includes(" ") ? firstName.split(" ")[0] : firstName) : '',
        //     'lastName': lastName ? lastName : (firstName && firstName.includes(" ") ? firstName.split(" ")[1] : ''),
        //     'view': view,
        //     'idToken': params.id_token,
        //     'email': decodedJWT['email'],
        //     [constants.ROLES.ORGANIZATION_CLAIM]: organizationID,
        //     'returnTo': req.session.returnTo,
        //     accessToken,
        //     'authorizedOrgs': orgList,
        //     'exchangeToken': req.exchangedToken,
        //     [constants.ROLES.ROLE_CLAIM]: roles,
        //     [constants.ROLES.GROUP_CLAIM]: groups,
        //     'isAdmin': isAdmin,
        //     'isSuperAdmin': isSuperAdmin,
        //     [constants.USER_ID]: decodedAccessToken[constants.USER_ID]
        // };

        profile = {
            accessToken,
        };

        return done(null, profile);
    });

    strategy.authorizationParams = function(options) {
        const params = {};
        if (options.prompt) {
            params.prompt = options.prompt;
        }
        return params;
    };

    passport.use(strategy);
}

module.exports = configurePassport;
