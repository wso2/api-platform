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

const path = require('path');
const fs = require('fs');
const yaml = require('js-yaml');

// Load .env file if present (silently ignored if absent)
try {
    require('dotenv').config({ path: path.join(process.cwd(), '.env') });
} catch (_) {}

/**
 * Load the base config from config.yaml.
 * Returns an empty object if the file does not exist, so env vars alone can drive the app.
 */
function loadBaseConfig() {
    const yamlPath = path.join(process.cwd(), 'configs', 'config.yaml');

    if (fs.existsSync(yamlPath)) {
        const raw = fs.readFileSync(yamlPath, 'utf8');
        return yaml.load(raw) || {};
    }
    return {};
}

/**
 * Minimal defaults so the app never crashes on missing top-level config sections
 * when no config.yaml is mounted (e.g. in the IT test environment where all
 * values are supplied via DP_* environment variables).
 */
const CONFIG_DEFAULTS = {
    defaultPort: 3000,
    mode: 'production',
    defaultOrgName: '',
    baseUrl: 'http://localhost:3000',
    db: {
        dialect: 'sqlite',
        storage: './devportal.db',
        host: 'localhost',
        port: 5432,
        database: 'devportal',
        username: 'postgres',
        password: '',
    },
    advanced: {
        http: true,
        dbSslDialectOption: false,
        resourceLoadFromBaseUrl: false,
        disabledRoleValidation: true,
        disableOrgCallback: true,
        disableSilentSSO: false,
        encryptionKey: '',
        apiKey: {
            enabled: false,
            keyType: 'x-wso2-api-key',
            keyValue: '',
        },
        tokenExchanger: {
            enabled: false,
        },
        openApiValidator: {
            validateResponses: 'off',
        },
    },
    logging: {
        consoleOnly: true,
    },
    serverCerts: {
        pathToCert: '',
        pathToPk: '',
        pathToCA: '',
    },
    authorizedPages: [],
    authenticatedPages: [],
    designMode: {
        enabled: false,
        apiSamplesPath: './samples/apis/',
        mcpSamplesPath: './samples/mcps/',
        subscriptionPlansPath: './samples/subscriptionPlans.yaml',
        applicationsPath: './samples/applications.yaml',
        pathToLayout: './src/defaultContent/',
    },
    controlPlane: {
        enabled: false,
        url: '',
        graphqlURL: '',
        disableCertValidation: false,
    },
    platformApi: {
        baseUrl: '',
        jwtSecret: '',
        insecure: false,
    },
};

/**
 * Deep-merge src into dst (dst wins on conflicts).
 * Only sets keys from src that are missing in dst.
 */
function mergeDefaults(dst, src) {
    for (const [k, v] of Object.entries(src)) {
        if (dst[k] === undefined || dst[k] === null) {
            dst[k] = v;
        } else if (typeof v === 'object' && !Array.isArray(v) && typeof dst[k] === 'object' && !Array.isArray(dst[k])) {
            mergeDefaults(dst[k], v);
        }
    }
    return dst;
}

/**
 * Deep-set a value in an object given a path expressed as an array of lowercase key tokens.
 * At each level, keys are matched case-insensitively (so "dbsecret" matches "dbSecret").
 * If no matching key exists, the token itself is used as the new key name.
 */
const BLOCKED_KEYS = new Set(['__proto__', 'prototype', 'constructor']);

function deepSet(obj, tokens, value) {
    if (!tokens.length || typeof obj !== 'object' || obj === null) return;

    const token = tokens[0];
    const rest = tokens.slice(1);

    if (BLOCKED_KEYS.has(token)) return;

    // Find an existing own-property key whose lowercase form matches the token
    const existingKey = Object.keys(obj).find(k =>
        Object.prototype.hasOwnProperty.call(obj, k) && k.toLowerCase() === token
    );
    const key = existingKey !== undefined ? existingKey : token;

    if (BLOCKED_KEYS.has(key)) return;

    if (rest.length === 0) {
        obj[key] = coerceValue(value);
    } else {
        if (typeof obj[key] !== 'object' || obj[key] === null) {
            obj[key] = {};
        }
        deepSet(obj[key], rest, value);
    }
}

/**
 * Coerce a string env var value to the most appropriate JS type.
 */
function coerceValue(value) {
    if (value === 'true') return true;
    if (value === 'false') return false;
    if (value !== '' && !isNaN(Number(value))) return Number(value);
    return value;
}

/**
 * Apply DP_* environment variable overrides onto the config object.
 *
 * Convention:
 *   - Prefix: DP_
 *   - _ separates nesting levels (one token per config object level)
 *   - __ represents a literal underscore within a key name
 *   - Tokens are matched case-insensitively against existing config keys
 *
 * Examples:
 *   DP_DB_HOST             → config.db.host
 *   DP_IDENTITYPROVIDER_CLIENTID → config.identityProvider.clientId
 *   DP_DB_PASSWORD                      → config.db.password
 *   DP_ADVANCED_APIKEY_KEYVALUE         → config.advanced.apiKey.keyValue
 *   DP_TELEMETRY_AZUREINSIGHTSCONNECTIONSTRING → config.telemetry.azureInsightsConnectionString
 */
function applyEnvOverrides(config) {
    const PLACEHOLDER = '\x00';
    for (const [key, value] of Object.entries(process.env)) {
        if (!key.startsWith('DP_')) continue;
        const withoutPrefix = key.slice(3); // remove DP_
        // Escape __ → placeholder, split on _, restore placeholder → _
        const tokens = withoutPrefix
            .replace(/__/g, PLACEHOLDER)
            .split('_')
            .map(t => t.replace(new RegExp(PLACEHOLDER, 'g'), '_').toLowerCase());
        deepSet(config, tokens, value);
    }
}

const config = loadBaseConfig();
mergeDefaults(config, CONFIG_DEFAULTS);
applyEnvOverrides(config);

// Webhook subscriber secrets/key paths can be supplied via env vars:
// DP_WEBHOOK_SECRET_<SUBSCRIBER_ID_UPPERCASED_UNDERSCORED>=<secret>
// DP_WEBHOOK_PUBKEY_PATH_<SUBSCRIBER_ID_UPPERCASED_UNDERSCORED>=<path-to-pem-file>
const webhookSubscribers = config.webhooks && config.webhooks.subscribers;
if (Array.isArray(webhookSubscribers)) {
    for (const sub of webhookSubscribers) {
        if (!sub.id) continue;
        const envKey = 'DP_WEBHOOK_SECRET_' + sub.id.toUpperCase().replace(/[^A-Z0-9]/g, '_');
        if (process.env[envKey]) sub.secret = process.env[envKey];
        const pubKeyPathEnv = 'DP_WEBHOOK_PUBKEY_PATH_' + sub.id.toUpperCase().replace(/[^A-Z0-9]/g, '_');
        if (process.env[pubKeyPathEnv]) sub.publicKeyPath = process.env[pubKeyPathEnv];
    }

    for (const sub of webhookSubscribers) {
        if (!sub.publicKeyPath) continue;
        try {
            sub.publicKey = fs.readFileSync(sub.publicKeyPath, 'utf8');
        } catch (err) {
            throw new Error(`[configLoader] Failed to read webhook public key for subscriber '${sub.id}' from '${sub.publicKeyPath}': ${err.message}`);
        }
    }
}

module.exports = { config };
