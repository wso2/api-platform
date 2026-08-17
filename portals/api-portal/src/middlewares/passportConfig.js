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
const { jwtVerify, createRemoteJWKSet } = require('jose');
const { getNestedClaim } = require('../utils/jwtDecode');
const { config } = require('../config/configLoader');
const { portalRoles } = require('./authorization');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const orgContext = require('../utils/orgContext');
const { CustomError } = require('../utils/errors/customErrors');

/**
 * Verifies an IDP-issued JWT against the configured JWKS. Returns the parsed
 * payload on success, or throws when signature, algorithm, issuer, audience,
 * or expiry checks fail.
 *
 * The passport-oauth2 callback previously decoded the id_token and access_token
 * without verification, so a tampered token — or one issued for a different
 * audience by the same IDP — would still be accepted as the login identity.
 * This helper closes that gap by using jose's jwtVerify against the same JWKS
 * URL the OAuth strategy is configured with.
 *
 * `audience` is optional so the caller can decide the appropriate audience
 * per token type (id_token → clientId per OpenID Connect Core §3.1.3.7; access
 * token → whatever the IDP is configured to stamp for this deployment).
 */
async function verifyIdpJwt(token, audience) {
    if (!token) {
        return {};
    }
    const jwksURL = config.auth.idp?.jwksUrl;
    if (!jwksURL) {
        throw new Error('IDP jwksUrl is not configured; cannot verify token');
    }
    const jwks = createRemoteJWKSet(new URL(jwksURL));
    const options = { algorithms: constants.JWT_ASYMMETRIC_ALGORITHMS };
    if (config.auth.idp?.issuer) options.issuer = config.auth.idp.issuer;
    if (audience) options.audience = audience;
    const { payload } = await jwtVerify(token, jwks, options);
    return payload;
}

/**
 * Checks an IDP-asserted organization claim against the organization this instance
 * serves.
 *
 * The decision itself is orgContext.requirePinnedOrg's — deliberately, rather than
 * a second resolve-and-compare written out here. There is no IDP in the integration
 * fixture, so this path can only be reached in a real IDP deployment; sharing the
 * helper means the rule being applied is the one the REST-API suite already
 * exercises end-to-end (organizations.spec.js's 403s), and this function is reduced
 * to translating its outcome into a login failure. It also resolves the claim
 * before comparing, so a handle, display name, or idp_ref_id spelling of the same
 * organization all match — which matters here because the flavour of the mapped
 * claim is IDP-specific.
 *
 * A rejection is a flat 403 whether the organization is unknown or merely someone
 * else's (requirePinnedOrg collapses the two), so a login attempt can't be used to
 * probe which organizations exist in the shared database.
 *
 * @param {string} organizationId the mapped organization claim from the ID token
 * @returns {Promise<Error|null>} null when the login may proceed, else an Error
 *   carrying the status the callback route should render
 */
async function assertLoginOrgAllowed(organizationId) {
    try {
        await orgContext.requirePinnedOrg(organizationId);
        return null;
    } catch (err) {
        if (err instanceof CustomError && err.statusCode === 403) {
            logger.warn('Rejected login: token organization is not this portal\'s', {
                expected: orgContext.getHandle(),
                asserted: organizationId,
            });
            const failure = new Error('Forbidden');
            failure.status = 403;
            return failure;
        }
        // A database/lookup fault, not a verdict about the organization — don't let
        // it read as "your organization is wrong".
        logger.error('Organization lookup failed during login', { error: err.message });
        const failure = new Error('Login failed');
        failure.status = 500;
        return failure;
    }
}

function configurePassport(SERVER_ID) {
    if (config.auth.mode === 'idp') {
        const idpScope = config.auth.idp.scope;
        const strategy = new OAuth2Strategy({
            issuer: config.auth.idp.issuer,
            authorizationURL: config.auth.idp.authorizationUrl,
            tokenURL: config.auth.idp.tokenUrl,
            userInfoURL: config.auth.idp.userInfoUrl,
            clientID: config.auth.idp.clientId,
            clientSecret: config.auth.idp.clientSecret || undefined,
            callbackURL: config.auth.idp.callbackUrl,
            pkce: true,
            state: true,
            logoutURL: config.auth.idp.logoutUrl,
            logoutRedirectURI: config.auth.idp.logoutRedirectUri,
            certificate: '',
            jwksURL: config.auth.idp.jwksUrl,
            passReqToCallback: true,
            scope: typeof idpScope === 'string'
                ? idpScope.split(/\s+/).filter(Boolean)
                : (Array.isArray(idpScope) ? idpScope : ['openid', 'profile', 'email']),
        }, async (req, accessToken, refreshToken, params, profile, done) => {
            if (!accessToken) {
                return done(new Error('Access token missing'));
            }
            let isAdmin = false;
            // Verify the id_token and access_token against the IDP's JWKS
            // before trusting any claim in them. Prior code called safeDecodeJwt
            // which only decoded the payload, leaving signature / issuer /
            // audience / expiry checks entirely unenforced.
            //
            // id_token: audience is the client_id per OIDC Core §3.1.3.7.
            // access_token: audience defaults to the IDP-configured value when
            // present; some IDPs (e.g. Asgardeo default) stamp the client_id
            // there too. When not configured, skip aud validation for the
            // access_token — the signature + issuer + expiry checks still run.
            let decodedJWT = {};
            let decodedAccessToken = {};
            try {
                decodedJWT = await verifyIdpJwt(params.id_token, config.auth.idp?.clientId);
                decodedAccessToken = await verifyIdpJwt(accessToken, config.auth.idp?.audience);
            } catch (err) {
                logger.error('IDP token verification failed during login', {
                    error: err.message,
                    code: err.code,
                });
                return done(new Error(`IDP token verification failed: ${err.message}`));
            }
            const firstName = decodedJWT['given_name'] || decodedJWT['nickname'];
            const lastName = decodedJWT['family_name'];
            const organizationId = getNestedClaim(decodedJWT, config.auth.claimMappings.organization) ?? '';
            const rawRoles = getNestedClaim(decodedJWT, config.auth.claimMappings.roles) ?? '';
            const roles = Array.isArray(rawRoles)
                ? rawRoles
                : String(rawRoles).split(/[\s,]+/).filter(Boolean);
            const rawGroups = getNestedClaim(decodedJWT, config.auth.claimMappings.groups) ?? '';
            const groups = Array.isArray(rawGroups)
                ? rawGroups
                : String(rawGroups).split(/[\s,]+/).filter(Boolean);
            const { admin: adminRole } = portalRoles();
            if (roles.includes(adminRole)) {
                isAdmin = true;
            }
            // The IDP is trusted to say who the user is, not which organization this
            // portal serves. A token it correctly signed for a *different*
            // organization passes every signature/expiry/audience check, so compare
            // the mapped organization claim against the one this instance is pinned
            // to and refuse the login otherwise — authResolver would reject each
            // subsequent request anyway, leaving the user with a session that 403s
            // on every page. Skipped when the claim is absent: that case is already
            // failed closed by authResolver, which needs an organization claim in
            // IDP mode.
            if (organizationId) {
                const orgErr = await assertLoginOrgAllowed(organizationId);
                if (orgErr) return done(orgErr);
            }

            const returnTo = req.session.returnTo;
            let view = '';
            if (returnTo) {
                const startIndex = returnTo.indexOf('/views/') + 7;
                const endIndex = returnTo.indexOf('/', startIndex) !== -1 ? returnTo.indexOf('/', startIndex) : returnTo.length;
                view = returnTo.substring(startIndex, endIndex);
            }
            const imageURL = decodedJWT['google_pic_url'] || decodedJWT['picture'] || constants.DEFAULT_PROFILE_IMAGE_URL;
            // Capture scopes from access token — supports both 'scope' (string) and 'scp' (array) variants
            const rawScope = decodedAccessToken?.scope ?? decodedAccessToken?.scp;
            const grantedScopes = Array.isArray(rawScope)
                ? rawScope.join(' ')
                : (typeof rawScope === 'string' ? rawScope : '');
            profile = {
                firstName: firstName ? (firstName.includes(" ") ? firstName.split(" ")[0] : firstName) : '',
                lastName: lastName ? lastName : (firstName && firstName.includes(" ") ? firstName.split(" ")[1] : ''),
                view,
                idToken: params.id_token,
                email: decodedJWT['email'] || req.session.username,
                [constants.ROLES.ORGANIZATION_CLAIM]: organizationId,
                returnTo: req.session.returnTo,
                accessToken,
                refreshToken,
                grantedScopes,
                [constants.ROLES.ROLE_CLAIM]: roles,
                [constants.ROLES.GROUP_CLAIM]: groups,
                isAdmin,
                [constants.USER_ID]: decodedAccessToken?.[constants.USER_ID],
                serverId: SERVER_ID,
                imageURL,
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
                    return done(null, profile);
                });
            });
        });

        strategy.authorizationParams = function (options) {
            const params = {};
            if (options.prompt) params.prompt = options.prompt;
            if (options.fidp) params.fidp = options.fidp;
            if (options.username) params.username = options.username;
            if (options.org) params.org = options.org;
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
            grantedScopes: user.grantedScopes || '',
            [constants.ROLES.ROLE_CLAIM]: user.roles,
            [constants.ROLES.GROUP_CLAIM]: user.groups,
            isAdmin: user.isAdmin,
            [constants.USER_ID]: user[constants.USER_ID],
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

