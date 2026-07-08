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
const crypto = require('crypto');
const toml = require('smol-toml');
const { DEFAULTS } = require('./configDefaults');

// Load .env file if present (silently ignored if absent)
try {
    require('dotenv').config({ path: path.join(process.cwd(), '.env') });
} catch (_) {}

/**
 * Recursively convert snake_case keys to camelCase (e.g. "base_url" -> "baseUrl").
 * Applied to the parsed TOML tree so config.toml can use snake_case while the
 * in-code struct and every consumer use camelCase.
 */
function snakeToCamel(key) {
    return key.replace(/_([a-z0-9])/g, (_, c) => c.toUpperCase());
}

function snakeToCamelDeep(value) {
    if (Array.isArray(value)) {
        return value.map(snakeToCamelDeep);
    }
    if (value !== null && typeof value === 'object') {
        const out = {};
        for (const [k, v] of Object.entries(value)) {
            out[snakeToCamel(k)] = snakeToCamelDeep(v);
        }
        return out;
    }
    return value;
}

/**
 * Load configs/config.toml (snake_case), converted to camelCase.
 * Returns an empty object if the file does not exist, so DEFAULTS + env vars
 * alone can drive the app.
 */
function loadTomlConfig() {
    const tomlPath = path.join(process.cwd(), 'configs', 'config.toml');

    if (fs.existsSync(tomlPath)) {
        const raw = fs.readFileSync(tomlPath, 'utf8');
        return snakeToCamelDeep(toml.parse(raw));
    }
    return {};
}

/**
 * Deep-merge src into dst (src wins on conflicts) — used to layer config.toml
 * over a clone of DEFAULTS.
 */
function mergeOver(dst, src) {
    for (const [k, v] of Object.entries(src)) {
        if (v !== null && typeof v === 'object' && !Array.isArray(v) &&
            dst[k] !== null && typeof dst[k] === 'object' && !Array.isArray(dst[k])) {
            mergeOver(dst[k], v);
        } else {
            dst[k] = v;
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
 * Apply APIP_DP_* environment variable overrides onto the config object.
 *
 * Convention:
 *   - Prefix: APIP_DP_
 *   - _ separates nesting levels (one token per config object level)
 *   - __ represents a literal underscore within a key name
 *   - Tokens are matched case-insensitively against existing config keys
 *
 * Examples:
 *   APIP_DP_DATABASE_HOST                        → config.database.host
 *   APIP_DP_IDP_CLIENTID                         → config.idp.clientId
 *   APIP_DP_DATABASE_PASSWORD                    → config.database.password
 *   APIP_DP_SECURITY_SERVICEAPIKEY_VALUE         → config.security.serviceApiKey.value
 *   APIP_DP_WEBHOOKS_DELIVERY_SIGNATURETOLERANCESEC → config.webhooks.delivery.signatureToleranceSec
 */
const ENV_PREFIX = 'APIP_DP_';

function applyEnvOverrides(config) {
    const PLACEHOLDER = '\x00';
    for (const [key, value] of Object.entries(process.env)) {
        if (!key.startsWith(ENV_PREFIX)) continue;
        const withoutPrefix = key.slice(ENV_PREFIX.length);
        // Escape __ → placeholder, split on _, restore placeholder → _
        const tokens = withoutPrefix
            .replace(/__/g, PLACEHOLDER)
            .split('_')
            .map(t => t.replace(new RegExp(PLACEHOLDER, 'g'), '_').toLowerCase());
        deepSet(config, tokens, value);
    }
}

// Precedence: DEFAULTS (source of truth) → configs/config.toml → APIP_DP_* env vars.
const config = mergeOver(JSON.parse(JSON.stringify(DEFAULTS)), loadTomlConfig());
applyEnvOverrides(config);

if (!config.security.encryptionKey || !/^[0-9a-fA-F]{64}$/.test(config.security.encryptionKey)) {
    config.security.encryptionKey = crypto.randomBytes(32).toString('hex');
    // Use process.stderr directly — logger is not yet initialised at this point
    process.stderr.write(
        '[WARN] security.encryptionKey is not set — generated an ephemeral key. ' +
        'Encrypted data (subscription tokens, webhook secrets) will be unreadable after restart. ' +
        'Set APIP_DP_SECURITY_ENCRYPTIONKEY in your .env file to persist it.\n'
    );
}

module.exports = { config };
