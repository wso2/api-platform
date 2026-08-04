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

/**
 * DEFAULTS is the source of truth for the config shape and its default values.
 * Every value here mirrors what configs/config-template.toml documents. Keys are
 * camelCase — configs/config.toml uses snake_case and is converted to camelCase
 * on load (see configLoader.js) before being merged over this struct.
 *
 * Effective config precedence: DEFAULTS  →  configs/config.toml (with any
 * {{ env }} / {{ file }} references resolved — see configLoader.js). There is
 * no separate, automatic APIP_AP_* env-var override layer; an env var only
 * takes effect where config.toml explicitly references it.
 */
const DEFAULTS = {
    server: {
        port: 9543,
        // Canonical public origin (scheme://host[:port]) of this portal. Used
        // ONLY to build the absolute URLs embedded in a generated agent prompt,
        // so those URLs don't depend on the request's (forgeable) Host header.
        // Empty = fall back to the request origin.
        baseUrl: 'https://localhost:9543',
        // Single listener on server.port; https.enabled toggles whether it
        // terminates TLS. enabled=false serves plain HTTP on that port — for when
        // a trusted upstream terminates TLS. cert_file/key_file are required only
        // when enabled=true (no self-signed fallback).
        https: {
            enabled: false,
            certFile: './resources/security/client-truststore.pem',
            keyFile: './resources/security/private-key.pem',
        },
    },
    logging: {
        level: 'info',   // debug | info | warn | error
        format: 'text',  // text | json
        consoleOnly: true,
    },
    // driver uses Sequelize's dialect values (sqlite | postgres).
    database: {
        driver: 'sqlite',        // sqlite | postgres
        path: './api-portal.db', // SQLite only
        host: 'localhost',       // PostgreSQL only
        port: 5432,              // PostgreSQL only
        name: 'api_portal',      // PostgreSQL only
        user: '',                // PostgreSQL / MSSQL only
        password: '',            // PostgreSQL only
        // PostgreSQL TLS: disable | verify-full.
        sslMode: 'disable',
        sslRootCert: './resources/security/ca.pem',  // CA cert — used by verify-full
        // Connection pool — PostgreSQL / MSSQL only. Same defaults both adapters
        // used before this was configurable.
        maxOpenConns: 50,  // matches platform-api's max_open_conns naming
        minOpenConns: 2,
        poolIdleTimeoutMs: 10000,
        poolConnectionTimeoutMs: 30000,
        poolRequestTimeoutMs: 30000,  // MSSQL only — per-query execution timeout
    },
    security: {
        encryptionKey: '',
        sessionSecret: '',
    },
    // Authentication — HOW a token is verified: a mode gate plus the two backends it
    // selects between, local (default) and idp. What a verified token may DO is
    // authorization, which lives in its own mode-independent section below.
    auth: {
        // "local" — username/password validated against the Platform API control
        // plane (auth.local below). "idp" — external OIDC IDP (auth.idp below).
        mode: 'local',   // local | idp
        // The org identifier the IdP asserts at SSO login — the expected VALUE of the
        // claim named by claimMappings.organization below, which is what incoming
        // tokens are matched against (ensureAuthenticated.belongsToTargetOrg). Stored
        // on the organization row as idp_ref_id, where token claims are compared
        // against it.
        //
        // Applied at startup only: seeded with the organization and re-applied on
        // every later boot (seederService.reconcileIdpOrgId), which makes this setting
        // the single writer of that field — the admin API refuses to change it. Empty
        // means "use organization.handle" for a fresh organization (the common case,
        // incl. the platform-api-managed mode where the org_handle claim IS the
        // handle) and "leave the stored value alone" for an existing one, so dropping
        // the setting never silently resets it. Matched verbatim against the claim, so
        // it is NOT lowercased, unlike the handle.
        idpOrgId: '',
        // JWT claim name mappings — which token claim carries each field.
        // Dot-notation supported for nested claims (e.g. "realm_access.roles").
        claimMappings: {
            organization: 'org_name',   // claim carrying the org ID
            roles: 'roles',             // claim carrying the user's roles
            groups: 'groups',
        },
        // Authorization — what a VERIFIED token may do. Deliberately outside both
        // auth.local and auth.idp: a token carries the same roles claim whether the
        // portal verified it against a JWKS endpoint or against the Platform API's
        // public key, so these settings apply in every auth mode. (They used to live
        // as auth.role_validation and auth.idp.roles, which made role configuration
        // reachable only in idp mode even though both branches of
        // ensureAuthenticated read it.)
        authorization: {
            // Master switch for REST-API (/api/v0.9) authorization. When false, an
            // authenticated caller satisfies every operation's declared scope list —
            // an explicit development opt-out, never the default.
            enabled: true,
            // How a REST request's effective scopes are derived:
            //   "role"  — (default) by expanding the token's roles claim through
            //             roleToScopeMapping. Works for every issuer: an external IDP
            //             emits the roles its estate is organized around and has no
            //             reason to mint dp:* scopes, and the Platform API mints its
            //             own ap_* role names, which the shipped table aliases.
            //   "scope" — from the token's own scope claim. Use this when the issuer
            //             mints dp:* scopes directly (an Asgardeo tenant set up with
            //             production/scripts/register_asgardeo_scopes.sh, or the
            //             Platform API in local auth mode).
            mode: 'role',   // scope | role
            // Path to the role-to-scope grant table (YAML). Required when
            // mode = "role", so it has a default rather than being empty: the shipped
            // table is baked into the image at ./resources (Dockerfile's COPY . .,
            // WORKDIR /app), and the same relative path resolves for `npm start` from
            // the project root. docker-compose.yaml points this at the mounted,
            // operator-editable copy under /etc/api-portal instead.
            roleToScopeMapping: './resources/role-to-scope-mapping.yaml',
            // Per-page role-tier gating (ensurePermission in ensureAuthenticated.js):
            // requires the caller's roles claim to name the tier a page demands.
            // Distinct from `enabled` above, which governs REST scopes — collapsing
            // the two would mean an operator turning page gating off also silently
            // turned REST scope enforcement off.
            pageRoleValidation: false,   // was: auth.role_validation
            // Which role name, as it appears in the token's roles claim, grants each
            // of the portal's two access tiers. Was auth.idp.roles, despite being read in
            // local mode too (authController.js's login). A third tier, superAdmin, used
            // to gate the earlier devportal's /portal pages; those are not served here, so
            // it guarded nothing and was removed.
            portalRoles: {
                admin: 'admin',
                subscriber: 'Internal/subscriber',
            },
        },
        // Local auth backend (the Platform API control plane) — used when
        // mode = "local". Validates username/password and verifies its JWTs.
        local: {
            platformApiUrl: '',
            // Filesystem path to the Platform API's RS256 public key PEM
            // ([platform_api.auth.jwt].public_key) — the portal reads this file
            // to verify Platform API-issued tokens.
            publicKeyPath: '',
            tlsSkipVerify: false,
        },
        // OIDC identity provider — used when mode = "idp".
        idp: {
            name: 'IS',
            issuer: 'https://localhost:9443/oauth2/token',
            authorizationUrl: 'https://localhost:9443/oauth2/authorize',
            tokenUrl: 'https://localhost:9443/oauth2/token',
            userInfoUrl: 'https://localhost:9443/oauth2/userinfo',
            clientId: '',
            clientSecret: '',
            audience: '',
            callbackUrl: 'http://localhost:9543/default/callback',
            scope: 'openid profile email',
            signUpUrl: '',
            logoutUrl: 'https://localhost:9443/oidc/logout',
            logoutRedirectUri: 'http://localhost:9543/default',
            certificate: '',
            jwksUrl: 'https://localhost:9443/oauth2/jwks',
            tokenRefreshTimeoutMs: 10000,
            silentSso: true,     // was: advanced.disableSilentSSO, inverted
            orgCallback: false,  // was: advanced.disableOrgCallback, inverted
            // Maps ?fidp=<key> query param to IDP identifier for federated login hints
            // (authController.js#login -> passportConfig.js's authorizationParams). Only
            // takes effect in OIDC mode. Kept out of config-template.toml since it's not
            // part of the default experience.
            fidp: {
                google: 'google',
                github: 'github',
                microsoft: 'microsoft',
                enterprise: 'EnterpriseIDP',
                email: 'LOCAL',
            },
        },
    },
    // Deployer-supplied ADDITIONS to the fixed system page-access lists — merged on top
    // of constants.js's ROUTE.SYSTEM_AUTHENTICATED_PAGES/SYSTEM_AUTHORIZED_PAGES by
    // ensureAuthenticated.js, never a replacement for them. Empty by default.
    pageAccessRules: {
        authenticated: [],
        authorized: [],
    },
    // The single organization this portal instance serves. The database schema is
    // still multi-org (one shared database can hold many organizations, each served
    // by its own portal instance), but a given instance is pinned to exactly one:
    // every page route, REST request, and background worker is scoped to `handle`,
    // and anything resolving to a different organization is rejected. See
    // src/utils/orgContext.js. The organization is seeded on first startup if it
    // doesn't exist yet (src/services/seederService.js).
    organization: {
        // Handle (URL slug) of this instance's organization — the {orgHandle}
        // segment of /{orgHandle}/views/{viewName}. Mirrors platform-api's
        // [platform_api.auth.file.organization] id/display_name pair; in local-auth
        // mode this MUST match that `id`, since it is what the Platform API puts in
        // the org_handle claim of the tokens this portal verifies.
        //
        // Deliberately EMPTY rather than 'default': configLoader.js refuses to start
        // when it is unset (design mode excepted). A shipped default would make that
        // check unreachable and turn a deployment that forgot to configure its
        // organization — the shared-database case this pin exists for — into one that
        // silently adopts, and seeds, an organization named 'default'. The packaged
        // configs/config.toml supplies the value for a normal single-tenant install.
        handle: '',
        // Display name used only when seeding the organization for the first time.
        // Never overwrites an existing organization's name, so an admin's later edit
        // via the settings UI survives restarts. Empty means "use the handle"
        // (orgContext.getDisplayName) — a fixed default here would seed an
        // organization named 'Default' whatever its handle is.
        displayName: '',
        // Deprecated alias for `handle`, kept so an existing config.toml setting
        // default_name keeps working. Resolved (with a warning) in configLoader.js.
        defaultName: '',
        autoCreateSubscriptionPlans: true,
    },
    // Which artifact types this portal serves. An allowlist: a type not listed here
    // gets no nav entry, no landing-page section, and 404s on its routes. Any
    // combination is valid, and a new artifact type is a new entry rather than a
    // new enum value. Defaults to all of them — an operator narrows, never widens.
    // Unknown entries are rejected at startup (see configLoader) so a typo can't
    // silently drop a type.
    artifacts: {
        enabledTypes: ['apis', 'mcp-servers', 'api-workflows'],
    },
    designMode: {
        enabled: false,
        pathToLayout: './src/defaultContent/',
        apiSamplesPath: './samples/apis/',
        mcpSamplesPath: './samples/mcps/',
        subscriptionPlansPath: './samples/subscription-plans.yaml',
        applicationsPath: './samples/applications.yaml',
    },
    webhooks: {
        delivery: {
            pollIntervalMs: 2000,
            batchSize: 50,
            signatureToleranceSec: 300,
        },
    },
    // Upload and archive-extraction limits.
    uploads: {
        maxBytes: 10485760,     // 10 MiB — single upload / single archive entry
        maxTotalBytes: 52428800, // 50 MiB — total extracted per archive
        maxZipEntries: 500,
        maxDepth: 10,
    },
    // API "Try It" proxy — see src/services/tryoutProxyService.js.
    tryout: {
        enabled: true,
        // Whether http:// (not just https://) registered endpoints may be called.
        // Registered endpoints are operator-controlled, and self-hosted gateways
        // are commonly plain http behind a TLS-terminating ingress.
        allowHttpEndpoints: true,
        // Whether an endpoint resolving into a private/loopback range may be
        // called. OFF by default — deny-by-default: the registered-endpoint
        // allowlist cannot protect against an endpoint that was itself
        // registered to point at an internal service, so the IP denylist is the
        // only control for that case and an operator must opt in explicitly.
        //
        // Self-hosted gateways commonly do live on a private address
        // (docker-compose service name, cluster IP, localhost); such a
        // deployment sets allow_private_endpoints = true after confirming that
        // only intended services are reachable from the portal.
        //
        // Link-local and cloud-metadata addresses stay blocked either way.
        allowPrivateEndpoints: false,
        // Skip TLS verification for the upstream endpoint. Development only.
        tlsSkipVerify: false,
        timeoutMs: 15000,
        maxRequestBytes: 1048576,   // 1 MiB
        maxResponseBytes: 5242880,  // 5 MiB
    },
    developer: {
        // Internal/debug knob for the /portal REST router's response validation
        // strictness (express-openapi-validator) — 'off' | 'strict' | 'log-only'. Not
        // meant for typical deployment config, so kept out of config-template.toml.
        // See src/routes/api/apiPortalRouter.js#resolveValidateResponsesOpt.
        openApiResponseValidation: 'off',
    },
};

module.exports = { DEFAULTS };
