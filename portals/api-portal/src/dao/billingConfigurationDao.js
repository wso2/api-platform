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
 * Data access for `billing_configurations`: the billing connector (and payment provider) one
 * organization sells paid plans through, with their settings: the Moesif keys and the Stripe key.
 *
 * The keys (the Moesif Collector Application Id among them) and the webhook signing secret are
 * encrypted at rest with the portal's own key (AES-256-GCM, security.encryption_key), as key
 * manager credentials are. Reads that only render the connection go through `get`, which never
 * selects an encrypted column. Two reads decrypt: `getWithSecrets`, from which the billing provider
 * interface builds the connector, and `getWebhookSecret`, which verifies a payment event and
 * returns only the signing secret.
 */

const db = require('../db/driver');
const logger = require('../config/logger');
const { config } = require('../config/configLoader');
const { createCryptoUtil, bufferToUtf8 } = require('../utils/cryptoUtil');
const { getPortalId } = require('../utils/orgContext');

const TABLE = 'billing_configurations';

const billingCrypto = createCryptoUtil(config.security && config.security.encryptionKey);

const PUBLIC_COLUMNS = [
    'uuid', 'org_uuid', 'provider', 'payment_provider',
    'stripe_publishable_key', 'stripe_webhook_endpoint_id', 'stripe_webhook_base_url', 'moesif_webhook_endpoint_id',
    'moesif_stripe_connected_by_portal',
    'last_event_at', 'created_by', 'created_at', 'updated_by', 'updated_at',
    'CASE WHEN stripe_webhook_secret_enc IS NULL THEN 0 ELSE 1 END AS has_webhook_secret',
].join(', ');

const SECRET_COLUMNS = `${PUBLIC_COLUMNS}, moesif_collector_application_id_enc, moesif_management_key_enc, stripe_api_key_enc, `
    + 'stripe_webhook_secret_enc';

function requireCrypto() {
    if (!billingCrypto.enabled) {
        throw new Error(
            'Billing keys cannot be stored: security.encryption_key is not configured. '
            + 'Set it to a 64-character hex string (openssl rand -hex 32).'
        );
    }
}

function encrypted(value) {
    return db.binaryParam(value ? Buffer.from(billingCrypto.encrypt(value), 'utf8') : null);
}

/**
 * Decrypts one stored key. Any failure (a changed encryption key, a damaged column) means the same
 * thing to an administrator: the key must be entered again. The cause is logged, not returned.
 */
function decryptField(value, field, configurationUuid) {
    const payload = bufferToUtf8(value);
    if (!payload) return '';
    try {
        return billingCrypto.decrypt(payload);
    } catch (error) {
        logger.error('Failed to decrypt a stored billing key', { field, configurationUuid, error: error.message });
        throw new Error('A stored billing key could not be decrypted. Enter the keys again in Settings › Billing; '
            + 'the portal\'s encryption key may have changed.');
    }
}

/** The organization's billing configuration without any key, or null. */
const get = async (orgId, t) => {
    const exec = t || db;
    return (await exec.queryOne(
        `SELECT ${PUBLIC_COLUMNS} FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]
    )) || null;
};

/** The organization's billing configuration with its keys decrypted, or null. */
const getWithSecrets = async (orgId, t) => {
    const exec = t || db;
    const row = await exec.queryOne(
        `SELECT ${SECRET_COLUMNS} FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]
    );
    if (!row) return null;
    requireCrypto();
    const { moesif_collector_application_id_enc: appEnc, moesif_management_key_enc: moesifEnc, stripe_api_key_enc: stripeEnc,
        stripe_webhook_secret_enc: secretEnc, ...publicRow } = row;
    return {
        ...publicRow,
        // The connector's names for the settings (as the service passes them when setting up).
        moesifCollectorApplicationId: decryptField(appEnc, 'moesif_collector_application_id', row.uuid),
        stripePublishableKey: row.stripe_publishable_key,
        moesifManagementKey: decryptField(moesifEnc, 'moesif_management_key', row.uuid),
        stripeApiKey: decryptField(stripeEnc, 'stripe_api_key', row.uuid),
        stripeWebhookSecret: decryptField(secretEnc, 'stripe_webhook_secret', row.uuid) || null,
    };
};

/**
 * What verifying a payment event needs, by configuration uuid, or null: the org, the connector and
 * payment provider, and the event endpoint's signing secret.
 */
const getWebhookSecret = async (configurationUuid) => {
    const row = await db.queryOne(
        `SELECT uuid, org_uuid, provider, payment_provider, stripe_webhook_secret_enc FROM ${TABLE}
         WHERE uuid = ? AND portal_id = ?`,
        [configurationUuid, getPortalId()]
    );
    if (!row) return null;
    requireCrypto();
    return {
        uuid: row.uuid,
        orgId: row.org_uuid,
        provider: row.provider,
        payment_provider: row.payment_provider,
        secret: decryptField(row.stripe_webhook_secret_enc, 'stripe_webhook_secret', row.uuid) || null,
    };
};

/**
 * Inserts the configuration. `conn` carries the plain keys; they are encrypted here.
 * @param {object} conn { uuid, provider, paymentProvider, moesifCollectorApplicationId, moesifManagementKey,
 *   stripeApiKey, stripePublishableKey, stripeWebhookEndpointId, stripeWebhookSecret, stripeWebhookBaseUrl,
 *   moesifWebhookEndpointId (only when setup created Moesif's endpoint), moesifStripeConnectedByPortal }
 */
const create = async (orgId, conn, userId, t) => {
    requireCrypto();
    const exec = t || db;
    const now = new Date();
    await exec.execute(
        `INSERT INTO ${TABLE} (uuid, org_uuid, portal_id, provider, payment_provider, moesif_collector_application_id_enc,
            moesif_management_key_enc, stripe_api_key_enc, stripe_publishable_key,
            stripe_webhook_endpoint_id, stripe_webhook_secret_enc, stripe_webhook_base_url, moesif_webhook_endpoint_id,
            moesif_stripe_connected_by_portal,
            created_by, created_at, updated_by, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [conn.uuid, orgId, getPortalId(), conn.provider, conn.paymentProvider, encrypted(conn.moesifCollectorApplicationId), encrypted(conn.moesifManagementKey),
            encrypted(conn.stripeApiKey), conn.stripePublishableKey,
            conn.stripeWebhookEndpointId || null, encrypted(conn.stripeWebhookSecret), conn.stripeWebhookBaseUrl || null,
            conn.moesifWebhookEndpointId || null, conn.moesifStripeConnectedByPortal ? 1 : 0,
            userId, now, userId, now]
    );
    return get(orgId, t);
};

/**
 * Records an endpoint registered again in Stripe (a removal that failed puts back what it had
 * removed). `ends` carries { stripeWebhookEndpointId, stripeWebhookSecret } for the portal's
 * endpoint, or { moesifWebhookEndpointId } for Moesif's.
 */
const setEndpoint = async (orgId, ends) => {
    if (ends.stripeWebhookEndpointId) {
        requireCrypto();
        await db.execute(
            `UPDATE ${TABLE} SET stripe_webhook_endpoint_id = ?, stripe_webhook_secret_enc = ?, updated_at = ?
             WHERE org_uuid = ? AND portal_id = ?`,
            [ends.stripeWebhookEndpointId, encrypted(ends.stripeWebhookSecret), new Date(), orgId, getPortalId()]
        );
    }
    if (ends.moesifWebhookEndpointId) {
        await db.execute(
            `UPDATE ${TABLE} SET moesif_webhook_endpoint_id = ?, updated_at = ? WHERE org_uuid = ? AND portal_id = ?`,
            [ends.moesifWebhookEndpointId, new Date(), orgId, getPortalId()]
        );
    }
};

/**
 * Records the portal's endpoint moved to another public address: { baseUrl, endpointId, secret }.
 * Resolves to the configuration, or null when there is none.
 */
const setWebhookEndpoint = async (orgId, { baseUrl, endpointId, secret }, userId) => {
    requireCrypto();
    await db.execute(
        `UPDATE ${TABLE} SET stripe_webhook_base_url = ?, stripe_webhook_endpoint_id = ?, stripe_webhook_secret_enc = ?,
            updated_by = ?, updated_at = ? WHERE org_uuid = ? AND portal_id = ?`,
        [baseUrl, endpointId, encrypted(secret), userId, new Date(), orgId, getPortalId()]
    );
    return get(orgId);
};

const deleteConfiguration = async (orgId, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `DELETE FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ?`,
        [orgId, getPortalId()]
    );
    return rowCount;
};

/** Records when the payment provider last sent an event, shown as the webhook endpoint's "Last event". */
const touchEvent = async (configurationUuid, at) => {
    await db.execute(
        `UPDATE ${TABLE} SET last_event_at = ? WHERE uuid = ? AND portal_id = ?`,
        [at, configurationUuid, getPortalId()]
    );
};

module.exports = {
    get,
    getWithSecrets,
    getWebhookSecret,
    create,
    setEndpoint,
    setWebhookEndpoint,
    delete: deleteConfiguration,
    touchEvent,
};
