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
        port: 3000,
    },
    tls: {
        enabled: false,   // was: advanced.http, inverted (http:true by default → tls disabled)
        certFile: './resources/security/client-truststore.pem',
        keyFile: './resources/security/private-key.pem',
        caFile: './resources/security/client-truststore.pem',
    },
    logging: {
        consoleOnly: true,
    },
    database: {
        type: 'sqlite',
        file: './devportal.db',
        host: 'localhost',
        port: 5432,
        name: 'devportal',
        username: 'postgres',
        password: '',
        ssl: {
            enabled: false,
            caFile: './resources/security/ca.pem',
        },
    },
    security: {
        encryptionKey: '',
        sessionSecret: '',
        roleValidation: false,   // was: advanced.disabledRoleValidation, inverted
        serviceApiKey: {
            enabled: true,
            headerName: 'x-wso2-api-key',
            value: '',
        },
    },
    idp: {
        name: 'IS',
        issuer: 'https://localhost:9443/oauth2/token',
        authorizationUrl: 'https://localhost:9443/oauth2/authorize',
        tokenUrl: 'https://localhost:9443/oauth2/token',
        userInfoUrl: 'https://localhost:9443/oauth2/userinfo',
        clientId: '',
        clientSecret: '',
        audience: '',
        callbackUrl: 'http://localhost:3000/default/callback',
        scope: 'openid profile email',
        signUpUrl: '',
        logoutUrl: 'https://localhost:9443/oidc/logout',
        logoutRedirectUri: 'http://localhost:3000/default',
        certificate: '',
        jwksUrl: 'https://localhost:9443/oauth2/jwks',
        tokenRefreshTimeoutMs: 10000,
        silentSso: true,     // was: advanced.disableSilentSSO, inverted
        orgCallback: false,  // was: advanced.disableOrgCallback, inverted
        claims: {
            role: 'roles',
            orgId: 'org_name',
            groups: 'groups',
        },
        roles: {
            admin: 'admin',
            subscriber: 'Internal/subscriber',
            superAdmin: 'superAdmin',
        },
        // Maps ?fidp=<key> query param to IDP identifier for federated login hints
        // (authController.js#login -> passportConfig.js's authorizationParams). Only
        // takes effect in OIDC mode (idp.clientId set) — the default local-auth login
        // screen never renders the social/enterprise buttons that trigger this. Kept
        // out of config-template.toml since it's not part of the default experience.
        fidp: {
            google: 'google',
            github: 'github',
            microsoft: 'microsoft',
            enterprise: 'EnterpriseIDP',
            email: 'LOCAL',
        },
    },
    // Upstream Platform API. Used for local auth credential validation and
    // Platform API JWT verification when idp.clientId is empty.
    platformApi: {
        url: '',
        jwtSecret: '',
        tlsSkipVerify: false,
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
    developer: {
        // Internal/debug knob for the /devportal REST router's response validation
        // strictness (express-openapi-validator) — 'off' | 'strict' | 'log-only'. Not
        // meant for typical deployment config, so kept out of config-template.toml.
        // See src/routes/api/devportalApiRouter.js#resolveValidateResponsesOpt.
        openApiResponseValidation: 'off',
    },
};

module.exports = { DEFAULTS };
