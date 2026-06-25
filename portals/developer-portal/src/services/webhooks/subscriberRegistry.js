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

/**
 * Maps a DP_WEBHOOK_SUBSCRIBER record to the shape consumed by the dispatcher
 * and delivery worker: {id, url, secret, publicKey, events, timeoutMs}.
 */
function toRuntimeSubscriber(record) {
    return {
        id: record.SUBSCRIBER_ID,
        url: record.TARGET_URL,
        secret: whDao.decryptSecret(record),
        publicKey: record.PUBLIC_KEY || null,
        events: record.EVENT_PATTERNS || [],
        timeoutMs: record.TIMEOUT_MS,
    };
}

/**
 * Returns all enabled subscribers for the given org that should receive an
 * event of the given type.
 *
 * @param {string} orgId
 * @param {string} eventType      — e.g. "apikey.generated"
 * @returns {Promise<Array<{id,url,secret,publicKey,events,timeoutMs}>>}
 */
async function matchSubscribers(orgId, eventType) {
    const records = await whDao.matchSubscribers(orgId, eventType);
    return records.map(toRuntimeSubscriber);
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
