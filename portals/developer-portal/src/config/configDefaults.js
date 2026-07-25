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
 * no separate, automatic APIP_DP_* env-var override layer; an env var only
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
        path: './devportal.db',  // SQLite only
        host: 'localhost',       // PostgreSQL only
        port: 5432,              // PostgreSQL only
        name: 'devportal',       // PostgreSQL only
        user: 'postgres',        // PostgreSQL only
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
        serviceApiKey: {
            enabled: true,
            headerName: 'x-wso2-api-key',
            value: '',
        },
    },
    // Authentication: a mode gate plus the two backends it selects between —
    // local (default) and idp.
    auth: {
        // "local" — username/password validated against the Platform API control
        // plane (auth.local below). "idp" — external OIDC IDP (auth.idp below).
        mode: 'local',   // local | idp
        // Enforce per-operation role validation.
        roleValidation: false,   // was: advanced.disabledRoleValidation, inverted
        // JWT claim name mappings — which token claim carries each field.
        // Dot-notation supported for nested claims (e.g. "realm_access.roles").
        claimMappings: {
            organization: 'org_name',   // claim carrying the org ID
            roles: 'roles',             // claim carrying the user's roles
            groups: 'groups',
        },
        // Local auth backend (the Platform API control plane) — used when
        // mode = "local". Validates username/password and verifies its JWTs.
        local: {
            platformApiUrl: '',
            // Filesystem path to the Platform API's RS256 public key PEM
            // ([platform_api.auth.jwt].public_key) — the devportal reads this file
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
            roles: {
                admin: 'admin',
                subscriber: 'Internal/subscriber',
                superAdmin: 'superAdmin',
            },
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
    organization: {
        defaultName: 'default',
        autoCreateSubscriptionPlans: true,
    },
    features: {
        // API Workflows is a core, always-on feature — not meant to be toggled off via
        // config. Kept as a struct default (not documented in config-template.toml) only
        // because src/utils/util.js and viewConfigureController.js read it defensively.
        apiWorkflows: true,
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
        // Internal/debug knob for the /devportal REST router's response validation
        // strictness (express-openapi-validator) — 'off' | 'strict' | 'log-only'. Not
        // meant for typical deployment config, so kept out of config-template.toml.
        // See src/routes/api/devportalApiRouter.js#resolveValidateResponsesOpt.
        openApiResponseValidation: 'off',
    },
};

module.exports = { DEFAULTS };
