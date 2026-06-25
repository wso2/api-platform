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
    'subscription.deleted',
    'subscription.plan_changed',
    'apikey.generated',
    'apikey.regenerated',
    'apikey.revoked',
    'apikey.application_updated',
    'application.created',
    'application.updated',
    'application.deleted'
]);

const SECRET_EVENT_TYPES = new Set([
    'apikey.generated', 'apikey.regenerated',
    'subscription.created',
]);

/**
 * Publish a domain event inside an existing Sequelize transaction.
 *
 * For SECRET_EVENT_TYPES (apikey.* and subscription.*): pass `opts.plaintextKey` (string).
 * The value is encrypted per-subscriber into DP_EVENT_DELIVERY.ENCRYPTED_FIELDS and
 * is NOT written to DP_EVENT.PAYLOAD.
 *
 * @param {string} eventType
 * @param {object} payload          — event data (no plaintext keys here)
 * @param {object} opts
 * @param {import('sequelize').Transaction} opts.transaction — required; caller owns the TX
 * @param {string} opts.orgId
 * @param {string} [opts.aggregateType]
 * @param {string} opts.aggregateId  — PK of the primary entity (keyId, subId, etc.)
 * @param {string} [opts.plaintextKey] — required for SECRET_EVENT_TYPES; zeroized after use for apikey.* events
 * @returns {Promise<string>} eventId
 */
async function publish(eventType, payload, opts) {
    if (!VALID_EVENT_TYPES.has(eventType)) {
        throw new Error(`Unknown event type: ${eventType}`);
    }
    const { transaction, orgId, aggregateType, aggregateId, plaintextKey } = opts;
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

    if (SECRET_EVENT_TYPES.has(eventType) && !plaintextKey) {
        logger.error('[eventPublisher] key event missing plaintextKey — rejecting', {
            eventType, orgId, aggregateId
        });
        event.STATUS = 'REJECTED';
        await event.save({ transaction });
        return event.EVENT_ID;
    }

    // For key events, encrypt the plaintext per subscriber and write delivery rows now
    // (inside the same TX) so the plaintext never leaves this call's stack.
    if (SECRET_EVENT_TYPES.has(eventType) && plaintextKey) {
        const subscribers = await matchSubscribers(orgId, eventType);
        const perSubscriberEncrypted = {};

        for (const sub of subscribers) {
            if (!sub.publicKey) {
                logger.warn('[eventPublisher] subscriber has no publicKey — key event will be delivered WITHOUT encrypted_key', {
                    subscriberId: sub.id, eventType
                });
                continue;
            }
            try {
                perSubscriberEncrypted[sub.id] = encryptToSubscriber(sub.publicKey, plaintextKey);
            } catch (err) {
                logger.error('[eventPublisher] failed to encrypt key for subscriber', {
                    subscriberId: sub.id, error: err.message
                });
            }
        }

        if (subscribers.length > 0) {
            await eventDao.createDeliveries(event.EVENT_ID, subscribers, perSubscriberEncrypted, transaction);
            // Mark as dispatched immediately — deliveries already created.
            event.STATUS = 'DISPATCHED';
            await event.save({ transaction });
        }

        bus.emit('key_event_published');
    } else {
        // Non-sensitive events: dispatcher will fan-out and create delivery rows.
        bus.emit('event_published');
    }
    logger.info('[eventPublisher] publishing event', {
        eventId: event.EVENT_ID, eventType, orgId, aggregateType, aggregateId,
        hasPlaintextKey: !!plaintextKey
    });

    return event.EVENT_ID;
}

/** Subscribe to wake signals from publish(). */
function onPublished(listener) {
    bus.on('event_published', listener);
    bus.on('key_event_published', listener);
}

module.exports = { publish, onPublished, VALID_EVENT_TYPES };
