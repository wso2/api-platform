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
const whDao = require('../../dao/webhookSubscriberDao');
const logger = require('../../config/logger');

/**
 * Maps a WEBHOOK_SUBSCRIBER record to the shape consumed by the dispatcher
 * and delivery worker: {id, url, secret, events, timeoutMs}.
 *
 * `secret` serves double duty: HMAC request signing (signer.js) and deriving the
 * field-encryption key for sensitive event fields (envelopeCrypto.js).
 */
function toRuntimeSubscriber(record) {
    return {
        id: record.uuid,
        url: record.target_url,
        secret: whDao.decryptSecret(record),
        events: record.event_patterns || [],
        timeoutMs: record.timeout_ms,
    };
}

/**
 * Returns all enabled subscribers for the given org that should receive an
 * event of the given type.
 *
 * A subscriber whose stored secret cannot be decrypted (e.g. it was encrypted under
 * a different security.encryption_key) is logged and left out rather than thrown:
 * this runs inside the caller's transaction (eventPublisher.publish), so one broken
 * subscriber would otherwise fail the user's own request — an API key generation,
 * say — instead of just that subscriber's delivery.
 *
 * @param {string} orgId
 * @param {string} eventType      — e.g. "apikey.generated"
 * @returns {Promise<Array<{id,url,secret,events,timeoutMs}>>}
 */
async function matchSubscribers(orgId, eventType) {
    const records = await whDao.matchSubscribers(orgId, eventType);
    const subscribers = [];
    for (const record of records) {
        try {
            subscribers.push(toRuntimeSubscriber(record));
        } catch (err) {
            logger.error('Skipping webhook subscriber whose secret could not be decrypted', {
                subscriberId: record.uuid, eventType, error: err.message,
            });
        }
    }
    return subscribers;
}

/**
 * Returns subscriber by id, or null.
 */
async function getSubscriber(id) {
    try {
        const record = await whDao.getById(id);
        return toRuntimeSubscriber(record);
    } catch (err) {
        return null;
    }
}

module.exports = { matchSubscribers, getSubscriber };
