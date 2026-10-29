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
 * Data access for `billing_configurations`: the billing provider (and payment provider) one
 * organization sells paid plans through, with their settings, one configuration per organization.
 *
 * Settings are kept per provider: public values in provider_settings as plain JSON, and keys in
 * provider_secrets_enc, each encrypted on its own (AES-256-GCM, security.encryption_key) and bound to
 * its path, the organization and the portal. `get` never reads the keys; `getWithSecrets` decrypts
 * them for the connector, and `getWebhookSecret` decrypts only the signing secret.
 *
 * Rows read here carry each setting as the connector names it: { stripe: { publishableKey } } is
 * `stripePublishableKey`.
 */

const db = require('../db/driver');
const logger = require('../config/logger');
const { config } = require('../config/configLoader');
const { createCryptoUtil, bufferToUtf8 } = require('../utils/cryptoUtil');
const { getPortalId } = require('../utils/orgContext');

const TABLE = 'billing_configurations';
const PUBLIC_COLUMNS = 'org_uuid, billing_provider, payment_provider, provider_settings, created_by, created_at';

// Where the webhook signing secret is kept.
const WEBHOOK_SECRET_PATH = ['stripe', 'webhookSecret'];

const billingCrypto = createCryptoUtil(config.security && config.security.encryptionKey);

function requireCrypto() {
    if (!billingCrypto.enabled) {
        throw new Error(
            'Billing keys cannot be stored: security.encryption_key is not configured. '
            + 'Set it to a 64-character hex string (openssl rand -hex 32).'
        );
    }
}

/** A JSON object stored as binary. */
function parseObject(value) {
    try {
        const parsed = JSON.parse(bufferToUtf8(value) || '{}');
        return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
    } catch (_err) {
        return {};
    }
}

function jsonParam(value) {
    return db.binaryParam(Buffer.from(JSON.stringify(value), 'utf8'));
}

/** The settings of every provider as the connector names them: { stripe: { apiKey } } -> { stripeApiKey }. */
function flatten(byProvider) {
    const flat = {};
    for (const [provider, values] of Object.entries(byProvider || {})) {
        for (const [key, value] of Object.entries(values || {})) {
            flat[`${provider}${key.charAt(0).toUpperCase()}${key.slice(1)}`] = value;
        }
    }
    return flat;
}

/** The context a secret is encrypted with: its path, the organization and the portal. */
function secretContext(orgId, provider, key) {
    return `${getPortalId()}/${orgId}/${provider}.${key}`;
}

/** The secrets ({ provider: { key: value } }) with each value encrypted; empty values are left out. */
function encryptSecrets(orgId, secrets) {
    const encrypted = {};
    for (const [provider, values] of Object.entries(secrets || {})) {
        for (const [key, value] of Object.entries(values || {})) {
            if (value === undefined || value === null || value === '') continue;
            encrypted[provider] = encrypted[provider] || {};
            encrypted[provider][key] = billingCrypto.encrypt(String(value), secretContext(orgId, provider, key));
        }
    }
    return encrypted;
}

/** Decrypts one stored secret. */
function decryptSecret(orgId, provider, key, payload) {
    if (!payload) return null;
    try {
        return billingCrypto.decrypt(payload, secretContext(orgId, provider, key));
    } catch (error) {
        logger.error('Failed to decrypt a stored billing key', { orgId, key: `${provider}.${key}`, error: error.message });
        throw new Error('A stored billing key could not be decrypted. Remove billing and set it up again in '
            + 'Settings › Billing; the portal\'s encryption key may have changed.');
    }
}

function toConfiguration(row) {
    if (!row) return null;
    const settings = parseObject(row.provider_settings);
    return {
        org_uuid: row.org_uuid,
        billing_provider: row.billing_provider,
        payment_provider: row.payment_provider,
        created_by: row.created_by,
        created_at: row.created_at,
        settings,
        ...flatten(settings),
    };
}

/** The organization's billing configuration without any key, or null. */
const get = async (orgId, t) => {
    const exec = t || db;
    return toConfiguration(await exec.queryOne(
        `SELECT ${PUBLIC_COLUMNS} FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]
    ));
};

/** The organization's billing configuration with its keys decrypted, or null. */
const getWithSecrets = async (orgId, t) => {
    const exec = t || db;
    const row = await exec.queryOne(
        `SELECT ${PUBLIC_COLUMNS}, provider_secrets_enc FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]
    );
    if (!row) return null;
    requireCrypto();
    const stored = parseObject(row.provider_secrets_enc);
    const secrets = {};
    for (const [provider, values] of Object.entries(stored)) {
        secrets[provider] = {};
        for (const [key, payload] of Object.entries(values || {})) {
            secrets[provider][key] = decryptSecret(orgId, provider, key, payload);
        }
    }
    return { ...toConfiguration(row), ...flatten(secrets) };
};

/**
 * What verifying a payment event needs, or null: the providers and the webhook signing secret.
 */
const getWebhookSecret = async (orgId) => {
    const row = await db.queryOne(
        `SELECT org_uuid, billing_provider, payment_provider, provider_secrets_enc FROM ${TABLE}
         WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]
    );
    if (!row) return null;
    requireCrypto();
    const [provider, key] = WEBHOOK_SECRET_PATH;
    const payload = (parseObject(row.provider_secrets_enc)[provider] || {})[key];
    return {
        orgId: row.org_uuid,
        billing_provider: row.billing_provider,
        payment_provider: row.payment_provider,
        secret: decryptSecret(orgId, provider, key, payload),
    };
};

/**
 * Inserts the configuration: `conn` is { billing_provider, payment_provider, settings, secrets }, the
 * secrets in plain text (they are encrypted here).
 */
const create = async (orgId, conn, userId, t) => {
    requireCrypto();
    const exec = t || db;
    await exec.execute(
        `INSERT INTO ${TABLE} (org_uuid, portal_id, billing_provider, payment_provider, provider_settings,
            provider_secrets_enc, created_by, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        [orgId, getPortalId(), conn.billing_provider, conn.payment_provider, jsonParam(conn.settings || {}),
            jsonParam(encryptSecrets(orgId, conn.secrets)), userId, new Date()]
    );
    return get(orgId, t);
};

/**
 * Records the payment provider's event endpoint: `endpoint` is { baseUrl, endpointId, secret }.
 * Resolves to the configuration, or null when there is none.
 */
const setEventEndpoint = async (orgId, { baseUrl, endpointId, secret }, t) => {
    requireCrypto();
    const exec = t || db;
    const row = await exec.queryOne(
        `SELECT provider_settings, provider_secrets_enc FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]
    );
    if (!row) return null;
    const settings = parseObject(row.provider_settings);
    settings.stripe = { ...(settings.stripe || {}), webhookEndpointId: endpointId };
    if (baseUrl) settings.stripe.webhookBaseUrl = baseUrl;
    const secrets = parseObject(row.provider_secrets_enc);
    const [provider, key] = WEBHOOK_SECRET_PATH;
    secrets[provider] = { ...(secrets[provider] || {}), ...encryptSecrets(orgId, { [provider]: { [key]: secret } })[provider] };
    await exec.execute(
        `UPDATE ${TABLE} SET provider_settings = ?, provider_secrets_enc = ? WHERE org_uuid = ? AND portal_id = ?`,
        [jsonParam(settings), jsonParam(secrets), orgId, getPortalId()]
    );
    return get(orgId, t);
};

/** Deletes the configuration, and with it its customers, their checkouts, and its API plans. */
const deleteConfiguration = async (orgId, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `DELETE FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]
    );
    return rowCount;
};

module.exports = {
    get,
    getWithSecrets,
    getWebhookSecret,
    create,
    setEventEndpoint,
    delete: deleteConfiguration,
};
