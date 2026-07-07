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
 * Effective config precedence: DEFAULTS  →  configs/config.toml  →  APIP_DP_* env vars.
 */
const DEFAULTS = {
    server: {
        baseUrl: 'http://localhost:3000',
        port: 3000,
        readOnlyMode: false,
    },
    tls: {
        enabled: false,   // was: advanced.http, inverted (http:true by default → tls disabled)
        certFile: './resources/security/client-truststore.pem',
        keyFile: './resources/security/private-key.pem',
        caFile: './resources/security/client-truststore.pem',
    },
    logging: {
        consoleOnly: false,
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
        fidp: {
            google: 'google',
            github: 'github',
            microsoft: 'microsoft',
            enterprise: 'EnterpriseIDP',
            email: 'LOCAL',
        },
    },
    platformApi: {
        baseUrl: '',
        jwtSecret: '',
        insecure: false,
    },
    pageAccessRules: {
        authenticated: [
            '/portal/*/edit',
            '/portal',
            '/*/configure',
            '**/applications',
            '**/applications/**',
            '**/myapis',
            '**/myapis/**',
            '**/myapis?**',
            '**/api-keys',
            '**/api-keys?**',
        ],
        authorized: [
            '**/applications',
            '**/applications/**',
            '/*/configure',
            '/portal/*/edit',
            '/portal',
            '**/myapis',
            '**/myapis/**',
            '**/myapis?**',
            '**/api-keys',
            '**/api-keys?**',
        ],
    },
    telemetry: {
        enabled: false,
        azureInsightsConnectionString: '',
    },
    organization: {
        defaultName: 'default',
        autoCreateSubscriptionPlans: true,
    },
    features: {
        apiWorkflows: true,
    },
    demo: {
        enabled: false,
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
    developer: {
        openApiResponseValidation: 'off',
    },
};

module.exports = { DEFAULTS };
