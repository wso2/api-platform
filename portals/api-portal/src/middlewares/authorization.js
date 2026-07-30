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
 */

'use strict';

/*
 * Authorization decisions shared by every credential path (auth.authorization).
 *
 * Deliberately one module rather than a rule spelled out per call site: the effective
 * scopes of a request are computed identically whether it arrived as a local-auth
 * session, an IDP session, or a bearer token, and whether it is heading for the
 * spec-driven /api/v0.9 router (authMiddleware.js) or an enforceSecurity-gated route
 * (ensureAuthenticated.js). A second implementation is how one of those paths ends up
 * still reading the raw scope claim after the portal has been switched to role mode.
 */

const { config } = require('../config/configLoader');
const constants = require('../utils/constants');
const roleScopeMap = require('../config/roleScopeMap');
const { getNestedClaim } = require('../utils/jwtDecode');

function authorizationConfig() {
    return config.auth?.authorization || {};
}

/** True when per-operation scope enforcement applies to the REST surface. */
function isAuthorizationEnabled() {
    return authorizationConfig().enabled !== false;
}

/** True when effective scopes come from expanding the roles claim, not the scope claim. */
function isRoleMode() {
    return authorizationConfig().mode === 'role';
}

/**
 * Reads the roles claim out of a decoded token, honouring the configured claim path
 * (dot-notation supported, e.g. Keycloak's "realm_access.roles").
 */
function rolesFromClaims(claims) {
    if (!claims) return undefined;
    const claimPath = config.auth?.claimMappings?.roles;
    return getNestedClaim(claims, claimPath) ?? claims[constants.ROLES.ROLE_CLAIM];
}

/**
 * The scopes a request is authorized against.
 *
 * In "scope" mode this is the token's own scope claim, unchanged. In "role" mode the
 * scope claim is ignored entirely and the roles claim is expanded through the grant
 * table instead — an external IDP emits the roles its estate is organized around and
 * has no reason to mint dp:* scopes, so the portal decides what each role may do.
 * Ignoring rather than merging the scope claim is deliberate: a caller must not be
 * able to widen a role's grant by asking their IDP for extra scope values.
 *
 * @param {string[]|string} tokenScopes scope claim, as an array or space-separated string
 * @param {object} claims decoded token payload (or session profile) carrying the roles claim
 * @returns {string[]}
 */
function effectiveScopes(tokenScopes, claims) {
    if (isRoleMode()) {
        return roleScopeMap.expandRoles(rolesFromClaims(claims));
    }
    if (Array.isArray(tokenScopes)) return tokenScopes.filter(Boolean);
    return String(tokenScopes || '').split(' ').filter(Boolean);
}

/**
 * Which role name grants each of the portal's three access tiers. Read from the
 * mode-independent authorization section — the local-auth login path and the IDP
 * login path both need it, which is why it no longer lives under auth.idp.
 */
function portalRoles() {
    return authorizationConfig().portalRoles || {};
}

/** True when a page's required role tier must be present in the caller's roles claim. */
function isPageRoleValidationEnabled() {
    return authorizationConfig().pageRoleValidation === true;
}

module.exports = {
    isAuthorizationEnabled,
    isRoleMode,
    effectiveScopes,
    rolesFromClaims,
    portalRoles,
    isPageRoleValidationEnabled,
};
