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
const EventEmitter = require('events');
const eventDao = require('../../dao/eventDao');
const { matchSubscribers } = require('./subscriberRegistry');
const { encryptToSubscriber } = require('./envelopeCrypto');
const logger = require('../../config/logger');

// Internal bus used to wake the dispatcher without waiting for the next poll tick.
const bus = new EventEmitter();
bus.setMaxListeners(50);

const VALID_EVENT_TYPES = new Set([
    'subscription.created',
    'subscription.updated',
    'subscription.deleted',
    'apikey.generated',
    'apikey.regenerated',
    'apikey.revoked',
    'apikey.application_updated',
    'application.created',
    'application.updated',
    'application.deleted'
]);

/**
 * Publish a domain event inside an existing Sequelize transaction.
 *
 * Pass `opts.secretFields` ({ [fieldName]: plaintextValue }) whenever the event carries
 * sensitive values. Each field is encrypted per-subscriber (RSA+AES-256-GCM) and stored
 * as { [fieldName]: envelope } in DP_EVENT_DELIVERY.ENCRYPTED_FIELDS — delivery rows are
 * created immediately inside the caller's TX so plaintext never leaves this call's stack.
 * Secret values are NOT written to DP_EVENT.PAYLOAD.
 *
 * @param {string} eventType
 * @param {object} payload          — event data (no secret values here)
 * @param {object} opts
 * @param {import('sequelize').Transaction} opts.transaction — required; caller owns the TX
 * @param {string} opts.orgId
 * @param {string} [opts.aggregateType]
 * @param {string} opts.aggregateId  — PK of the primary entity (keyId, subId, etc.)
 * @param {Object.<string,string>} [opts.secretFields] — fields to encrypt per-subscriber
 * @returns {Promise<string>} eventId
 */
async function publish(eventType, payload, opts) {
    if (!VALID_EVENT_TYPES.has(eventType)) {
        throw new Error(`Unknown event type: ${eventType}`);
    }
    const { transaction, orgId, aggregateType, aggregateId, secretFields } = opts;
    if (!transaction) throw new Error('publish() requires a Sequelize transaction');
    if (!orgId) throw new Error('publish() requires orgId');
    if (!aggregateId) throw new Error('publish() requires aggregateId');

    const event = await eventDao.create({
        eventType,
        orgId,
        aggregateType: aggregateType || eventType.split('.')[0],
        aggregateId,
        payload
    }, transaction);

    // When secretFields is provided, encrypt each field per subscriber and write delivery
    // rows inside the same TX so plaintext never leaves this call's stack.
    if (secretFields) {
        const subscribers = await matchSubscribers(orgId, eventType);
        const perSubscriberEncrypted = {};

        for (const sub of subscribers) {
            if (!sub.publicKey) {
                logger.warn('Subscriber has no publicKey — secret event will be delivered without encrypted fields', {
                    subscriberId: sub.id, eventType
                });
                continue;
            }
            const encryptedForSub = {};
            for (const [fieldName, plaintextValue] of Object.entries(secretFields)) {
                try {
                    encryptedForSub[fieldName] = encryptToSubscriber(sub.publicKey, plaintextValue);
                } catch (err) {
                    logger.error('Failed to encrypt field for subscriber', {
                        subscriberId: sub.id, fieldName, error: err.message
                    });
                }
            }
            if (Object.keys(encryptedForSub).length > 0) {
                perSubscriberEncrypted[sub.id] = encryptedForSub;
            }
        }

        if (subscribers.length > 0) {
            await eventDao.createDeliveries(event.UUID, subscribers, perSubscriberEncrypted, transaction);
            event.STATUS = 'DISPATCHED';
        } else {
            event.STATUS = 'ALL_DELIVERED';
        }
        await event.save({ transaction });

        bus.emit('key_event_published');
    } else {
        // Non-sensitive events: dispatcher will fan-out and create delivery rows.
        bus.emit('event_published');
    }
    logger.info('Publishing event', {
        eventId: event.UUID, eventType, orgId, aggregateType, aggregateId,
        hasSecretFields: !!secretFields
    });

    return event.UUID;
}

/** Subscribe to wake signals from publish(). */
function onPublished(listener) {
    bus.on('event_published', listener);
    bus.on('key_event_published', listener);
}

module.exports = { publish, onPublished, VALID_EVENT_TYPES };
