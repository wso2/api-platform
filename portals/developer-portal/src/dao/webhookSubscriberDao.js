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
const { Sequelize } = require('sequelize');
const { WebhookSubscriber } = require('../models/webhookSubscriber');
const { createCryptoUtil } = require('../utils/cryptoUtil');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');

const whCrypto = createCryptoUtil(config.advanced.encryptionKey);

/**
 * Create a new webhook subscriber for an organization.
 * The secret is encrypted before storage.
 */
const create = async (orgId, subData) => {
    try {
        if (!whCrypto.enabled) {
            throw new Error('Webhook subscriber encryption key is not configured. ' +
                'Set config.advanced.encryptionKey to a 64-char hex string.');
        }
        const record = await WebhookSubscriber.create({
            ORG_ID: orgId,
            NAME: subData.name,
            TARGET_URL: subData.url,
            SECRET_ENC: whCrypto.encrypt(subData.secret),
            ...(subData.publicKey && { PUBLIC_KEY: subData.publicKey }),
            ...(subData.gatewayType && { GATEWAY_TYPE: subData.gatewayType }),
            ...(subData.events && { EVENT_PATTERNS: subData.events }),
            ...(subData.enabled !== undefined && { ENABLED: subData.enabled }),
            ...(subData.timeoutMs && { TIMEOUT_MS: subData.timeoutMs }),
        });
        return record;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        logger.error('Error creating webhook subscriber', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Update an existing webhook subscriber.
 * Re-encrypts the secret if it is provided.
 */
const update = async (orgId, subscriberId, subData) => {
    try {
        const updatePayload = {
            ...(subData.name && { NAME: subData.name }),
            ...(subData.url && { TARGET_URL: subData.url }),
            ...(subData.publicKey !== undefined && { PUBLIC_KEY: subData.publicKey }),
            ...(subData.gatewayType !== undefined && { GATEWAY_TYPE: subData.gatewayType }),
            ...(subData.events && { EVENT_PATTERNS: subData.events }),
            ...(subData.enabled !== undefined && { ENABLED: subData.enabled }),
            ...(subData.timeoutMs && { TIMEOUT_MS: subData.timeoutMs }),
        };

        if (subData.secret) {
            if (!whCrypto.enabled) {
                throw new Error('Webhook subscriber encryption key is not configured.');
            }
            updatePayload.SECRET_ENC = whCrypto.encrypt(subData.secret);
        }

        const [updatedRowsCount] = await WebhookSubscriber.update(updatePayload, {
            where: { SUBSCRIBER_ID: subscriberId, ORG_ID: orgId }
        });
        if (updatedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('Webhook subscriber not found');
        }
        // `returning: true` only yields row instances on Postgres; re-fetch
        // explicitly so the result is reliable on SQLite too.
        const updated = await WebhookSubscriber.findByPk(subscriberId);
        return [updatedRowsCount, [updated]];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        logger.error('Error updating webhook subscriber', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * List all webhook subscribers for an organization.
 */
const list = async (orgId) => {
    try {
        return await WebhookSubscriber.findAll({
            where: { ORG_ID: orgId }
        });
    } catch (error) {
        logger.error('Error fetching webhook subscribers', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * List enabled webhook subscribers across all organizations that match the
 * given event type and gateway type. Used by the dispatcher fan-out.
 */
const matchSubscribers = async (orgId, eventType, gatewayType) => {
    try {
        const subscribers = await WebhookSubscriber.findAll({
            where: { ORG_ID: orgId, ENABLED: true }
        });
        return subscribers.filter(sub => {
            if (sub.GATEWAY_TYPE && sub.GATEWAY_TYPE !== '*') {
                if (sub.GATEWAY_TYPE !== gatewayType) return false;
            }
            const patterns = sub.EVENT_PATTERNS;
            if (Array.isArray(patterns) && patterns.length > 0) {
                const matches = patterns.some(pattern => {
                    if (pattern.endsWith('.*')) {
                        return eventType.startsWith(pattern.slice(0, -1));
                    }
                    return pattern === eventType;
                });
                if (!matches) return false;
            }
            return true;
        });
    } catch (error) {
        logger.error('Error matching webhook subscribers', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Get a single webhook subscriber by ID.
 */
const get = async (orgId, subscriberId) => {
    try {
        const sub = await WebhookSubscriber.findOne({ where: { SUBSCRIBER_ID: subscriberId, ORG_ID: orgId } });
        if (!sub) {
            throw new Sequelize.EmptyResultError('Webhook subscriber not found');
        }
        return sub;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        logger.error('Error fetching webhook subscriber', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Delete a webhook subscriber.
 */
const deleteSubscriber = async (orgId, subscriberId) => {
    try {
        const deleted = await WebhookSubscriber.destroy({
            where: { SUBSCRIBER_ID: subscriberId, ORG_ID: orgId }
        });
        if (deleted < 1) {
            throw new Sequelize.EmptyResultError('Webhook subscriber not found');
        }
        return deleted;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        logger.error('Error deleting webhook subscriber', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Decrypt the secret for a webhook subscriber record.
 * Used internally by the delivery worker to sign outgoing requests.
 */
const decryptSecret = (subRecord) => {
    if (!whCrypto.enabled) {
        throw new Error('Webhook subscriber encryption key is not configured.');
    }
    return whCrypto.decrypt(subRecord.SECRET_ENC);
};

module.exports = {
    create,
    update,
    list,
    matchSubscribers,
    get,
    delete: deleteSubscriber,
    decryptSecret,
};
