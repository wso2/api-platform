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
const create = async (orgId, subData, createdBy) => {
    try {
        if (subData.secret && !whCrypto.enabled) {
            throw new Error('Webhook subscriber encryption key is not configured. ' +
                'Set config.advanced.encryptionKey to a 64-char hex string.');
        }
        const record = await WebhookSubscriber.create({
            ORG_UUID: orgId,
            NAME: subData.name,
            TARGET_URL: subData.targetUrl,
            ...(subData.secret && { SECRET_ENC: whCrypto.encrypt(subData.secret) }),
            ...(subData.publicKey && { PUBLIC_KEY: subData.publicKey }),
            ...(subData.events && { EVENT_PATTERNS: subData.events }),
            ...(subData.enabled !== undefined && { ENABLED: subData.enabled ? 1 : 0 }),
            ...(subData.timeoutMs && { TIMEOUT_MS: subData.timeoutMs }),
            CREATED_BY: createdBy,
            UPDATED_BY: createdBy,
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
const update = async (orgId, subscriberId, subData, updatedBy) => {
    try {
        const updatePayload = {
            ...(subData.name && { NAME: subData.name }),
            ...(subData.targetUrl && { TARGET_URL: subData.targetUrl }),
            ...(subData.publicKey !== undefined && { PUBLIC_KEY: subData.publicKey }),
            ...(subData.events && { EVENT_PATTERNS: subData.events }),
            ...(subData.enabled !== undefined && { ENABLED: subData.enabled ? 1 : 0 }),
            ...(subData.timeoutMs && { TIMEOUT_MS: subData.timeoutMs }),
            UPDATED_BY: updatedBy,
            UPDATED_AT: new Date(),
        };

        if (subData.secret) {
            if (!whCrypto.enabled) {
                throw new Error('Webhook subscriber encryption key is not configured.');
            }
            updatePayload.SECRET_ENC = whCrypto.encrypt(subData.secret);
        }

        const [updatedRowsCount] = await WebhookSubscriber.update(updatePayload, {
            where: { UUID: subscriberId, ORG_UUID: orgId }
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
            where: { ORG_UUID: orgId }
        });
    } catch (error) {
        logger.error('Error fetching webhook subscribers', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * List enabled webhook subscribers across all organizations that match the
 * given event type. Used by the dispatcher fan-out.
 */
const matchSubscribers = async (orgId, eventType) => {
    try {
        const subscribers = await WebhookSubscriber.findAll({
            where: { ORG_UUID: orgId, ENABLED: 1 }
        });
        return subscribers.filter(sub => {
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
 * Get a single webhook subscriber by UUID.
 */
const get = async (orgId, subscriberId) => {
    try {
        const sub = await WebhookSubscriber.findOne({ where: { UUID: subscriberId, ORG_UUID: orgId } });
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
 * Get a single webhook subscriber by UUID only, without scoping to an org.
 * UUID is a globally unique UUID primary key, so this is safe.
 * Used by the delivery worker, which only has the subscriber UUID (from the
 * delivery row) and not the org UUID in scope.
 */
const getById = async (subscriberId) => {
    try {
        const sub = await WebhookSubscriber.findOne({ where: { UUID: subscriberId } });
        if (!sub) {
            throw new Sequelize.EmptyResultError('Webhook subscriber not found');
        }
        return sub;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        logger.error('Error fetching webhook subscriber by id', { error });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Delete a webhook subscriber.
 */
const deleteSubscriber = async (orgId, subscriberId) => {
    try {
        const deleted = await WebhookSubscriber.destroy({
            where: { UUID: subscriberId, ORG_UUID: orgId }
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
    if (!subRecord.SECRET_ENC) return null;
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
    getById,
    delete: deleteSubscriber,
    decryptSecret,
};
